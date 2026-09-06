package websocket

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	interviewEvent "github.com/masterfabric-go/masterfabric/internal/domain/interview/event"
	"github.com/masterfabric-go/masterfabric/internal/domain/realtime/model"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionBrokerPublishesOnlyMatchingTenantAndInterview(t *testing.T) {
	broker := NewSubscriptionBroker(nil, 2, nil, "test-secret")
	org, otherOrg, interview, otherInterview := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	matching, err := broker.Subscribe(ctx, org, interview)
	require.NoError(t, err)
	other, err := broker.Subscribe(ctx, otherOrg, otherInterview)
	require.NoError(t, err)

	broker.Publish(model.InterviewUpdate{OrganizationID: org, InterviewID: interview, EventType: "interview.started", OccurredAt: time.Now().UTC()})
	select {
	case <-matching:
	case <-time.After(time.Second):
		t.Fatal("matching subscription did not receive update")
	}
	select {
	case <-other:
		t.Fatal("unrelated subscription received update")
	default:
	}
}

func TestSubscriptionBrokerConvertsTypedInterviewEvent(t *testing.T) {
	broker := NewSubscriptionBroker(nil, 1, nil, "test-secret")
	org, interview := uuid.New(), uuid.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := broker.Subscribe(ctx, org, interview)
	require.NoError(t, err)

	err = broker.handleEvent(context.Background(), interviewEvent.Changed{OrganizationID: org, InterviewID: interview, EventType: "interview.completed", Timestamp: time.Now().UTC()})
	require.NoError(t, err)

	select {
	case update := <-stream:
		require.Equal(t, "interview.completed", update.EventType)
	case <-time.After(time.Second):
		t.Fatal("typed interview event was not delivered")
	}
}

func TestSubscriptionBrokerClosesStreamsWhenContextEnds(t *testing.T) {
	broker := NewSubscriptionBroker(nil, 1, nil, "test-secret")
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := broker.Subscribe(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)
	cancel()
	select {
	case _, ok := <-stream:
		require.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("subscription stream was not closed")
	}

	broker.Close()
	broker.Close()
}

func TestSubscriptionBrokerReplaysOnlyScopedEvents(t *testing.T) {
	broker := NewSubscriptionBroker(nil, 2, nil, "test-secret")
	org, otherOrg, interview := uuid.New(), uuid.New(), uuid.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := broker.Subscribe(ctx, org, interview)
	require.NoError(t, err)

	broker.Publish(model.InterviewUpdate{OrganizationID: org, InterviewID: interview, EventType: "interview.started", OccurredAt: time.Now().UTC()})
	first := <-stream
	broker.Publish(model.InterviewUpdate{OrganizationID: otherOrg, InterviewID: interview, EventType: "interview.completed", OccurredAt: time.Now().UTC()})
	broker.Publish(model.InterviewUpdate{OrganizationID: org, InterviewID: interview, EventType: "interview.completed", OccurredAt: time.Now().UTC()})

	replayed, err := broker.Subscribe(context.Background(), org, interview, first.Cursor)
	require.NoError(t, err)
	defer broker.Close()
	select {
	case update := <-replayed:
		require.Equal(t, org, update.OrganizationID)
		require.Equal(t, "interview.completed", update.EventType)
	case <-time.After(time.Second):
		t.Fatal("scoped replay did not arrive")
	}
	select {
	case update := <-replayed:
		t.Fatalf("replay leaked an unrelated event: %+v", update)
	default:
	}
}

func TestSubscriptionBrokerRejectsInvalidAndExpiredCursors(t *testing.T) {
	broker := NewSubscriptionBroker(nil, 2, nil, "test-secret")
	org, interview := uuid.New(), uuid.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := broker.Subscribe(ctx, org, interview)
	require.NoError(t, err)
	broker.Publish(model.InterviewUpdate{OrganizationID: org, InterviewID: interview, EventType: "interview.started", OccurredAt: time.Now().UTC()})
	first := <-stream

	_, err = broker.Subscribe(context.Background(), org, interview, first.Cursor+"tampered")
	require.ErrorIs(t, err, ErrReplayCursorInvalid)
	_, err = broker.Subscribe(context.Background(), uuid.New(), interview, first.Cursor)
	require.ErrorIs(t, err, ErrReplayCursorInvalid)
	expired := broker.encodeCursor(org, interview, "1-0", time.Now().UTC().Add(-time.Second))
	_, err = broker.Subscribe(context.Background(), org, interview, expired)
	require.ErrorIs(t, err, ErrReplayCursorExpired)
}

func TestSubscriptionBrokerRejectsReplayGapAndBoundedBacklog(t *testing.T) {
	broker := NewSubscriptionBroker(nil, 2, nil, "test-secret")
	broker.replayStore = newMemoryReplayStore(2, defaultReplayTTL)
	broker.replayLimit = 2
	org, interview := uuid.New(), uuid.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := broker.Subscribe(ctx, org, interview)
	require.NoError(t, err)
	broker.Publish(model.InterviewUpdate{OrganizationID: org, InterviewID: interview, EventType: "one", OccurredAt: time.Now().UTC()})
	first := <-stream
	broker.Publish(model.InterviewUpdate{OrganizationID: org, InterviewID: interview, EventType: "two", OccurredAt: time.Now().UTC()})
	broker.Publish(model.InterviewUpdate{OrganizationID: org, InterviewID: interview, EventType: "three", OccurredAt: time.Now().UTC()})

	_, err = broker.Subscribe(context.Background(), org, interview, first.Cursor)
	require.True(t, errors.Is(err, ErrReplayGap) || errors.Is(err, ErrReplayLimit), "expected gap or bound error, got %v", err)
}
