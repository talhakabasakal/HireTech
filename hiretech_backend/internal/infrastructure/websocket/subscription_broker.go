package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	interviewEvent "github.com/masterfabric-go/masterfabric/internal/domain/interview/event"
	"github.com/masterfabric-go/masterfabric/internal/domain/realtime/model"
	"github.com/masterfabric-go/masterfabric/internal/shared/events"
)

const defaultMaxSubscriptions = 1000

// SubscriptionBroker routes candidate-safe interview updates to individual
// GraphQL subscription streams. Keys include the organization to preserve a
// tenant boundary even if an identifier is ever reused across stores.
type SubscriptionBroker struct {
	logger           *slog.Logger
	maxSubscriptions int
	mu               sync.RWMutex
	subscribers      map[string]map[chan model.InterviewUpdate]struct{}
	closed           bool
}

func NewSubscriptionBroker(logger *slog.Logger, maxSubscriptions int) *SubscriptionBroker {
	if maxSubscriptions <= 0 {
		maxSubscriptions = defaultMaxSubscriptions
	}
	return &SubscriptionBroker{logger: logger, maxSubscriptions: maxSubscriptions, subscribers: make(map[string]map[chan model.InterviewUpdate]struct{})}
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
// ends. Authorization is deliberately performed by the GraphQL resolver.
func (b *SubscriptionBroker) Subscribe(ctx context.Context, organizationID, interviewID uuid.UUID) (<-chan model.InterviewUpdate, error) {
	if b == nil || organizationID == uuid.Nil || interviewID == uuid.Nil {
		return nil, fmt.Errorf("subscription scope is required")
	}
	ch := make(chan model.InterviewUpdate, 16)
	key := subscriptionKey(organizationID, interviewID)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, fmt.Errorf("subscription broker is closed")
	}
	if b.subscriptionCountLocked() >= b.maxSubscriptions {
		return nil, fmt.Errorf("subscription limit reached")
	}
	if b.subscribers[key] == nil {
		b.subscribers[key] = make(map[chan model.InterviewUpdate]struct{})
	}
	b.subscribers[key][ch] = struct{}{}
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
	b.mu.RLock()
	defer b.mu.RUnlock()
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
