package kafka_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	interviewEvent "github.com/masterfabric-go/masterfabric/internal/domain/interview/event"
	infraKafka "github.com/masterfabric-go/masterfabric/internal/infrastructure/kafka"
	"github.com/masterfabric-go/masterfabric/internal/shared/events"
)

func TestKafkaRealtimeFanoutIntegration(t *testing.T) {
	broker := os.Getenv("KAFKA_TEST_BROKER")
	if broker == "" {
		t.Skip("set KAFKA_TEST_BROKER to run Kafka fan-out integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, infraKafka.EnsureTopics(ctx, broker, []string{events.TopicInterview}, 1, 1, slog.Default()))

	interviewID := uuid.New()
	groups := []string{"fanout-a-" + uuid.NewString(), "fanout-b-" + uuid.NewString()}
	received := []chan struct{}{make(chan struct{}, 1), make(chan struct{}, 1)}
	buses := make([]*infraKafka.Bus, 0, len(groups))
	for index, group := range groups {
		bus := infraKafka.NewBus([]string{broker}, group, slog.Default())
		out := received[index]
		bus.Subscribe(events.TopicInterview, func(_ context.Context, raw events.Event) error {
			event, ok := raw.(*events.Envelope)
			if !ok || event == nil {
				return nil
			}
			var changed interviewEvent.Changed
			if event.Type == events.EventTypeInterviewChanged && json.Unmarshal(event.Data, &changed) == nil && changed.InterviewID == interviewID {
				select {
				case out <- struct{}{}:
				default:
				}
			}
			return nil
		})
		bus.Start(ctx)
		buses = append(buses, bus)
	}
	t.Cleanup(func() {
		for _, bus := range buses {
			_ = bus.Close()
		}
	})

	publisher := infraKafka.NewBus([]string{broker}, "fanout-publisher-"+uuid.NewString(), slog.Default())
	t.Cleanup(func() { _ = publisher.Close() })
	update := interviewEvent.Changed{OrganizationID: uuid.New(), InterviewID: interviewID, EventType: events.EventTypeInterviewChanged, Status: "in_progress", Version: 3, Timestamp: time.Now().UTC()}

	seen := make([]bool, len(received))
	for attempts := 0; attempts < 40; attempts++ {
		require.NoError(t, publisher.Publish(ctx, events.TopicInterview, update))
		if waitForBoth(ctx, received, seen, 250*time.Millisecond) {
			return
		}
	}
	t.Fatal("both instance-scoped consumer groups did not receive the interview update")
}

func waitForBoth(ctx context.Context, channels []chan struct{}, seen []bool, timeout time.Duration) bool {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		complete := true
		for index, channel := range channels {
			if seen[index] {
				continue
			}
			select {
			case <-channel:
				seen[index] = true
			default:
				complete = false
			}
		}
		if complete {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-time.After(10 * time.Millisecond):
		}
	}
}
