package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/masterfabric-go/masterfabric/internal/domain/realtime/model"
)

type memoryReplayStore struct {
	mu       sync.Mutex
	streams  map[string][]replayRecord
	nextID   uint64
	maxItems int
	ttl      time.Duration
}

func newMemoryReplayStore(maxItems int, ttl time.Duration) *memoryReplayStore {
	return &memoryReplayStore{streams: make(map[string][]replayRecord), maxItems: maxItems, ttl: ttl}
}

func (s *memoryReplayStore) Append(_ context.Context, scope string, update model.InterviewUpdate) (replayRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	record := replayRecord{
		streamID:  fmt.Sprintf("%d-0", s.nextID),
		update:    update,
		expiresAt: time.Now().UTC().Add(s.ttl),
	}
	stream := append(s.streams[scope], record)
	if len(stream) > s.maxItems {
		stream = stream[len(stream)-s.maxItems:]
	}
	s.streams[scope] = stream
	return record, nil
}

func (s *memoryReplayStore) Replay(_ context.Context, scope, after string, limit int) ([]replayRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stream := s.streams[scope]
	if len(stream) == 0 {
		return nil, ErrReplayExpired
	}
	if !containsStreamID(stream, after) {
		if compareStreamID(after, stream[0].streamID) < 0 {
			return nil, ErrReplayGap
		}
		return nil, ErrReplayGap
	}
	start := 0
	for index, record := range stream {
		if record.streamID == after {
			start = index + 1
			break
		}
	}
	if len(stream)-start > limit {
		return nil, ErrReplayLimit
	}
	result := append([]replayRecord(nil), stream[start:]...)
	return result, nil
}

func containsStreamID(stream []replayRecord, streamID string) bool {
	for _, record := range stream {
		if record.streamID == streamID {
			return true
		}
	}
	return false
}

type redisReplayStore struct {
	client   *redis.Client
	maxItems int
	ttl      time.Duration
}

func newRedisReplayStore(client *redis.Client, maxItems int, ttl time.Duration) *redisReplayStore {
	return &redisReplayStore{client: client, maxItems: maxItems, ttl: ttl}
}

func (s *redisReplayStore) Append(ctx context.Context, scope string, update model.InterviewUpdate) (replayRecord, error) {
	update.Cursor = ""
	data, err := json.Marshal(update)
	if err != nil {
		return replayRecord{}, fmt.Errorf("marshal interview replay event: %w", err)
	}
	expiresAt := time.Now().UTC().Add(s.ttl)
	streamID, err := s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: replayStreamKey(scope),
		ID:     "*",
		MaxLen: int64(s.maxItems),
		Approx: false,
		Values: map[string]interface{}{
			"data":       string(data),
			"expires_at": expiresAt.Unix(),
		},
	}).Result()
	if err != nil {
		return replayRecord{}, fmt.Errorf("append interview replay event: %w", err)
	}
	if err := s.client.Expire(ctx, replayStreamKey(scope), s.ttl).Err(); err != nil {
		return replayRecord{}, fmt.Errorf("expire interview replay stream: %w", err)
	}
	return replayRecord{streamID: streamID, update: update, expiresAt: expiresAt}, nil
}

func (s *redisReplayStore) Replay(ctx context.Context, scope, after string, limit int) ([]replayRecord, error) {
	key := replayStreamKey(scope)
	oldest, err := s.client.XRangeN(ctx, key, "-", "+", 1).Result()
	if err != nil {
		return nil, fmt.Errorf("read oldest interview replay event: %w", err)
	}
	if len(oldest) == 0 {
		return nil, ErrReplayExpired
	}
	// Requiring the exact cursor entry distinguishes a retained cursor from a
	// cursor that points into a trimmed portion of the stream.
	exact, err := s.client.XRangeN(ctx, key, after, after, 1).Result()
	if err != nil {
		return nil, fmt.Errorf("validate interview replay cursor: %w", err)
	}
	if len(exact) == 0 {
		return nil, ErrReplayGap
	}
	replay, err := s.client.XRangeN(ctx, key, "("+after, "+", int64(limit+1)).Result()
	if err != nil {
		return nil, fmt.Errorf("read interview replay events: %w", err)
	}
	if len(replay) > limit {
		return nil, ErrReplayLimit
	}
	result := make([]replayRecord, 0, len(replay))
	for _, message := range replay {
		data, ok := message.Values["data"].(string)
		if !ok {
			return nil, fmt.Errorf("interview replay event has invalid data")
		}
		var update model.InterviewUpdate
		if err := json.Unmarshal([]byte(data), &update); err != nil {
			return nil, fmt.Errorf("decode interview replay event: %w", err)
		}
		expiresAtValue, ok := message.Values["expires_at"].(string)
		if !ok {
			return nil, fmt.Errorf("interview replay event has invalid expiry")
		}
		expiresAtUnix, err := parseRedisInt(expiresAtValue)
		if err != nil {
			return nil, err
		}
		result = append(result, replayRecord{streamID: message.ID, update: update, expiresAt: time.Unix(expiresAtUnix, 0).UTC()})
	}
	return result, nil
}

func parseRedisInt(value string) (int64, error) {
	var parsed int64
	if _, err := fmt.Sscan(value, &parsed); err != nil {
		return 0, fmt.Errorf("invalid replay expiry: %w", err)
	}
	return parsed, nil
}
