package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type outboxRepositoryStub struct {
	limit  int
	count  int
	counts []int
	calls  int
	err    error
}

func (s *outboxRepositoryStub) RelayPending(_ context.Context, limit int) (int, error) {
	s.limit = limit
	s.calls++
	if len(s.counts) > 0 {
		count := s.counts[0]
		s.counts = s.counts[1:]
		return count, s.err
	}
	return s.count, s.err
}

func TestOutboxRelay_RelayOnceUsesBoundedBatch(t *testing.T) {
	repo := &outboxRepositoryStub{count: 3}
	relay := NewOutboxRelay(repo, nil)

	count, err := relay.RelayOnce(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.Equal(t, defaultBatchSize, repo.limit)
}

func TestOutboxRelay_RelayCycleDrainsFullBatchesWithinBound(t *testing.T) {
	repo := &outboxRepositoryStub{counts: []int{2, 2, 1}}
	relay := NewOutboxRelayWithConfig(repo, nil, OutboxRelayConfig{BatchSize: 2, MaxBatchesPerCycle: 3})

	count, err := relay.relayCycle(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 5, count)
	assert.Equal(t, 3, repo.calls)
}

func TestOutboxRelay_RelayCycleStopsAtConfiguredBatchBound(t *testing.T) {
	repo := &outboxRepositoryStub{count: 2}
	relay := NewOutboxRelayWithConfig(repo, nil, OutboxRelayConfig{BatchSize: 2, MaxBatchesPerCycle: 2})

	count, err := relay.relayCycle(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 4, count)
	assert.Equal(t, 2, repo.calls)
}

func TestOutboxRelay_RelayOncePropagatesRepositoryError(t *testing.T) {
	repo := &outboxRepositoryStub{err: assert.AnError}
	relay := NewOutboxRelay(repo, nil)

	count, err := relay.RelayOnce(context.Background())

	assert.ErrorIs(t, err, assert.AnError)
	assert.Zero(t, count)
}
