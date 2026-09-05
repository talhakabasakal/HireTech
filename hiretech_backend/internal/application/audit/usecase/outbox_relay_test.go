package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type outboxRepositoryStub struct {
	limit int
	count int
	err   error
}

func (s *outboxRepositoryStub) RelayPending(_ context.Context, limit int) (int, error) {
	s.limit = limit
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

func TestOutboxRelay_RelayOncePropagatesRepositoryError(t *testing.T) {
	repo := &outboxRepositoryStub{err: assert.AnError}
	relay := NewOutboxRelay(repo, nil)

	count, err := relay.RelayOnce(context.Background())

	assert.ErrorIs(t, err, assert.AnError)
	assert.Zero(t, count)
}
