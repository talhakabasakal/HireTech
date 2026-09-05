package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	interviewEvent "github.com/masterfabric-go/masterfabric/internal/domain/interview/event"
	"github.com/masterfabric-go/masterfabric/internal/domain/realtime/model"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionBrokerPublishesOnlyMatchingTenantAndInterview(t *testing.T) {
	broker := NewSubscriptionBroker(nil, 2)
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
	broker := NewSubscriptionBroker(nil, 1)
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
	broker := NewSubscriptionBroker(nil, 1)
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
