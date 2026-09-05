package usecase

import (
	"context"
	"log/slog"
	"time"
)

const (
	defaultBatchSize = 100
	defaultInterval  = 2 * time.Second
)

// OutboxRepository is the storage boundary for the audit relay.
type OutboxRepository interface {
	RelayPending(ctx context.Context, limit int) (int, error)
}

// OutboxRelay periodically projects committed audit events into the durable
// audit log store. Failed batches remain pending and are retried on the next
// tick.
type OutboxRelay struct {
	repo     OutboxRepository
	logger   *slog.Logger
	batch    int
	interval time.Duration
}

func NewOutboxRelay(repo OutboxRepository, logger *slog.Logger) *OutboxRelay {
	return &OutboxRelay{repo: repo, logger: logger, batch: defaultBatchSize, interval: defaultInterval}
}

// RelayOnce drains one bounded batch. It is exposed for deterministic tests
// and for operators that want to run a single controlled pass.
func (r *OutboxRelay) RelayOnce(ctx context.Context) (int, error) {
	if r == nil || r.repo == nil {
		return 0, nil
	}
	return r.repo.RelayPending(ctx, r.batch)
}

// Run starts the small in-process relay loop and exits with ctx.
func (r *OutboxRelay) Run(ctx context.Context) {
	if r == nil || r.repo == nil {
		return
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		if _, err := r.RelayOnce(ctx); err != nil && r.logger != nil {
			r.logger.Error("audit outbox relay failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
