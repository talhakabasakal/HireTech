package websocket

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	interviewEvent "github.com/masterfabric-go/masterfabric/internal/domain/interview/event"
	"github.com/masterfabric-go/masterfabric/internal/domain/realtime/model"
	"github.com/masterfabric-go/masterfabric/internal/shared/events"
	"github.com/redis/go-redis/v9"
)

const (
	defaultMaxSubscriptions = 1000
	defaultReplayLimit      = 64
	defaultReplayTTL        = 5 * time.Minute
	replayStreamPrefix      = "hiretech:graphql:interview-replay:"
)

var (
	ErrReplayCursorInvalid = errors.New("replay cursor is invalid")
	ErrReplayCursorExpired = errors.New("replay cursor has expired")
	ErrReplayExpired       = errors.New("replay stream has expired")
	ErrReplayGap           = errors.New("replay cursor is no longer retained")
	ErrReplayLimit         = errors.New("replay window exceeds the bounded limit")
)

var redisStreamIDPattern = regexp.MustCompile(`^[0-9]+-[0-9]+$`)

type replayRecord struct {
	streamID  string
	update    model.InterviewUpdate
	expiresAt time.Time
}

type replayStore interface {
	Append(context.Context, string, model.InterviewUpdate) (replayRecord, error)
	Replay(context.Context, string, string, int) ([]replayRecord, error)
}

// SubscriptionBroker routes candidate-safe interview updates to individual
// GraphQL subscription streams. Keys include the organization to preserve a
// tenant boundary even if an identifier is ever reused across stores.
type SubscriptionBroker struct {
	logger           *slog.Logger
	maxSubscriptions int
	replayLimit      int
	replayTTL        time.Duration
	replayStore      replayStore
	fallbackStore    replayStore
	cursorSecret     []byte
	mu               sync.RWMutex
	subscribers      map[string]map[chan model.InterviewUpdate]struct{}
	closed           bool
}

// NewSubscriptionBroker creates the interview-scoped subscription broker. A
// connected Redis client enables durable replay across API instances; the
// bounded memory store is retained for local development and tests where
// Redis is intentionally unavailable.
func NewSubscriptionBroker(logger *slog.Logger, maxSubscriptions int, redisClient *redis.Client, cursorSecret string) *SubscriptionBroker {
	if maxSubscriptions <= 0 {
		maxSubscriptions = defaultMaxSubscriptions
	}
	var store replayStore = newMemoryReplayStore(defaultReplayLimit, defaultReplayTTL)
	if redisClient != nil {
		store = newRedisReplayStore(redisClient, defaultReplayLimit, defaultReplayTTL)
	}
	return &SubscriptionBroker{
		logger: logger, maxSubscriptions: maxSubscriptions, replayLimit: defaultReplayLimit,
		replayTTL: defaultReplayTTL, replayStore: store, fallbackStore: newMemoryReplayStore(defaultReplayLimit, defaultReplayTTL), cursorSecret: []byte(cursorSecret),
		subscribers: make(map[string]map[chan model.InterviewUpdate]struct{}),
	}
}

// Register connects the broker to the application event bus. It accepts both
// typed in-process events and Kafka envelopes.
func (b *SubscriptionBroker) Register(bus events.EventBus) {
	if b == nil || bus == nil {
		return
	}
	bus.Subscribe(events.TopicInterview, b.handleEvent)
}

func (b *SubscriptionBroker) handleEvent(_ context.Context, raw events.Event) error {
	var update model.InterviewUpdate
	switch value := raw.(type) {
	case interviewEvent.Changed:
		update = model.InterviewUpdate{OrganizationID: value.OrganizationID, InterviewID: value.InterviewID, EventType: value.EventType, Status: value.Status, Version: value.Version, OccurredAt: value.Timestamp}
	case *interviewEvent.Changed:
		if value == nil {
			return nil
		}
		update = model.InterviewUpdate{OrganizationID: value.OrganizationID, InterviewID: value.InterviewID, EventType: value.EventType, Status: value.Status, Version: value.Version, OccurredAt: value.Timestamp}
	case *events.Envelope:
		if value == nil || value.Type != events.EventTypeInterviewChanged {
			return nil
		}
		var changed interviewEvent.Changed
		if err := json.Unmarshal(value.Data, &changed); err != nil {
			return fmt.Errorf("decode interview realtime event: %w", err)
		}
		update = model.InterviewUpdate{OrganizationID: changed.OrganizationID, InterviewID: changed.InterviewID, EventType: changed.EventType, Status: changed.Status, Version: changed.Version, OccurredAt: changed.Timestamp}
	default:
		return nil
	}
	if update.OccurredAt.IsZero() {
		return nil
	}
	b.Publish(update)
	return nil
}

// Subscribe creates a bounded stream and removes it when the request context
// ends. Authorization is deliberately performed by the GraphQL resolver. The
// optional after cursor is scoped and validated before any replay is emitted.
func (b *SubscriptionBroker) Subscribe(ctx context.Context, organizationID, interviewID uuid.UUID, after ...string) (<-chan model.InterviewUpdate, error) {
	if b == nil || organizationID == uuid.Nil || interviewID == uuid.Nil {
		return nil, fmt.Errorf("subscription scope is required")
	}
	afterCursor := ""
	if len(after) > 0 {
		afterCursor = strings.TrimSpace(after[0])
	}
	key := subscriptionKey(organizationID, interviewID)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, fmt.Errorf("subscription broker is closed")
	}
	if b.subscriptionCountLocked() >= b.maxSubscriptions {
		return nil, fmt.Errorf("subscription limit reached")
	}
	replay, err := b.replayLocked(ctx, organizationID, interviewID, afterCursor)
	if err != nil {
		return nil, err
	}
	ch := make(chan model.InterviewUpdate, b.replayLimit+16)
	if b.subscribers[key] == nil {
		b.subscribers[key] = make(map[chan model.InterviewUpdate]struct{})
	}
	b.subscribers[key][ch] = struct{}{}
	for _, record := range replay {
		ch <- record.update
	}
	go func() {
		<-ctx.Done()
		b.remove(key, ch)
	}()
	return ch, nil
}

func (b *SubscriptionBroker) Publish(update model.InterviewUpdate) {
	if b == nil || update.OrganizationID == uuid.Nil || update.InterviewID == uuid.Nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if b.replayStore != nil {
		record, err := b.replayStore.Append(context.Background(), subscriptionKey(update.OrganizationID, update.InterviewID), update)
		if err != nil {
			if b.logger != nil {
				b.logger.Error("failed to persist interview replay event", "error", err, "interview_id", update.InterviewID)
			}
			if b.fallbackStore != nil {
				record, err = b.fallbackStore.Append(context.Background(), subscriptionKey(update.OrganizationID, update.InterviewID), update)
			}
		}
		if err != nil {
			if b.logger != nil {
				b.logger.Error("failed to persist fallback interview replay event", "error", err, "interview_id", update.InterviewID)
			}
		} else {
			update.Cursor = b.encodeCursor(update.OrganizationID, update.InterviewID, record.streamID, record.expiresAt)
		}
	}
	for ch := range b.subscribers[subscriptionKey(update.OrganizationID, update.InterviewID)] {
		select {
		case ch <- update:
		default:
			if b.logger != nil {
				b.logger.Warn("graphQL subscription buffer full, dropping interview update", "interview_id", update.InterviewID)
			}
		}
	}
}

func (b *SubscriptionBroker) replayLocked(ctx context.Context, organizationID, interviewID uuid.UUID, after string) ([]replayRecord, error) {
	if strings.TrimSpace(after) == "" {
		return nil, nil
	}
	streamID, err := b.decodeCursor(organizationID, interviewID, after, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if b.replayStore == nil {
		return nil, ErrReplayGap
	}
	replay, err := b.replayStore.Replay(ctx, subscriptionKey(organizationID, interviewID), streamID, b.replayLimit)
	if err != nil {
		return nil, err
	}
	for index := range replay {
		replay[index].update.Cursor = b.encodeCursor(organizationID, interviewID, replay[index].streamID, replay[index].expiresAt)
	}
	return replay, nil
}

type replayCursor struct {
	Version       int    `json:"v"`
	Organization  string `json:"o"`
	Interview     string `json:"i"`
	StreamID      string `json:"s"`
	ExpiresAtUnix int64  `json:"e"`
	Signature     string `json:"h"`
}

func (b *SubscriptionBroker) encodeCursor(organizationID, interviewID uuid.UUID, streamID string, expiresAt time.Time) string {
	cursor := replayCursor{Version: 1, Organization: organizationID.String(), Interview: interviewID.String(), StreamID: streamID, ExpiresAtUnix: expiresAt.Unix()}
	cursor.Signature = b.signCursor(cursor)
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}

func (b *SubscriptionBroker) decodeCursor(organizationID, interviewID uuid.UUID, encoded string, now time.Time) (string, error) {
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("%w: encoding", ErrReplayCursorInvalid)
	}
	var cursor replayCursor
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.Version != 1 || cursor.Organization != organizationID.String() || cursor.Interview != interviewID.String() || !redisStreamIDPattern.MatchString(cursor.StreamID) || cursor.ExpiresAtUnix <= 0 || cursor.Signature == "" {
		return "", ErrReplayCursorInvalid
	}
	if !hmac.Equal([]byte(cursor.Signature), []byte(b.signCursor(cursor))) {
		return "", ErrReplayCursorInvalid
	}
	if !now.Before(time.Unix(cursor.ExpiresAtUnix, 0)) {
		return "", ErrReplayCursorExpired
	}
	return cursor.StreamID, nil
}

func (b *SubscriptionBroker) signCursor(cursor replayCursor) string {
	unsigned := cursor
	unsigned.Signature = ""
	data, _ := json.Marshal(unsigned)
	hash := hmac.New(sha256.New, b.cursorSecret)
	_, _ = hash.Write(data)
	return base64.RawURLEncoding.EncodeToString(hash.Sum(nil))
}

func replayStreamKey(scope string) string { return replayStreamPrefix + scope }

func parseStreamID(value string) (uint64, uint64, error) {
	if !redisStreamIDPattern.MatchString(value) {
		return 0, 0, ErrReplayCursorInvalid
	}
	parts := strings.SplitN(value, "-", 2)
	first, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, 0, ErrReplayCursorInvalid
	}
	second, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, 0, ErrReplayCursorInvalid
	}
	return first, second, nil
}

func compareStreamID(left, right string) int {
	leftMS, leftSeq, leftErr := parseStreamID(left)
	rightMS, rightSeq, rightErr := parseStreamID(right)
	if leftErr != nil || rightErr != nil {
		return 0
	}
	if leftMS < rightMS || (leftMS == rightMS && leftSeq < rightSeq) {
		return -1
	}
	if leftMS > rightMS || (leftMS == rightMS && leftSeq > rightSeq) {
		return 1
	}
	return 0
}

func (b *SubscriptionBroker) remove(key string, ch chan model.InterviewUpdate) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if subscribers := b.subscribers[key]; subscribers != nil {
		delete(subscribers, ch)
		if len(subscribers) == 0 {
			delete(b.subscribers, key)
		}
	}
	close(ch)
}

func (b *SubscriptionBroker) subscriptionCountLocked() int {
	count := 0
	for _, subscribers := range b.subscribers {
		count += len(subscribers)
	}
	return count
}

func subscriptionKey(organizationID, interviewID uuid.UUID) string {
	return organizationID.String() + ":" + interviewID.String()
}

// Close stops future subscriptions and closes all active streams.
func (b *SubscriptionBroker) Close() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for key, subscribers := range b.subscribers {
		for ch := range subscribers {
			close(ch)
		}
		delete(b.subscribers, key)
	}
}
