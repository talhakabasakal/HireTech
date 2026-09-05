package usecase

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
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
	repo               OutboxRepository
	logger             *slog.Logger
	batch              int
	interval           time.Duration
	maxBatchesPerCycle int
	metrics            relayMetrics
}

type OutboxRelayConfig struct {
	BatchSize          int
	Interval           time.Duration
	MaxBatchesPerCycle int
}

type relayMetrics struct {
	batches  metric.Int64Counter
	events   metric.Int64Counter
	errors   metric.Int64Counter
	duration metric.Float64Histogram
}

func NewOutboxRelay(repo OutboxRepository, logger *slog.Logger) *OutboxRelay {
	return NewOutboxRelayWithConfig(repo, logger, OutboxRelayConfig{})
}

func NewOutboxRelayWithConfig(repo OutboxRepository, logger *slog.Logger, cfg OutboxRelayConfig) *OutboxRelay {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}
	if cfg.MaxBatchesPerCycle <= 0 {
		cfg.MaxBatchesPerCycle = 10
	}
	meter := otel.Meter("hiretech/audit")
	return &OutboxRelay{
		repo:               repo,
		logger:             logger,
		batch:              cfg.BatchSize,
		interval:           cfg.Interval,
		maxBatchesPerCycle: cfg.MaxBatchesPerCycle,
		metrics: relayMetrics{
			batches:  newCounter(meter, "hiretech.audit.outbox.relay.batches", "Number of audit outbox relay attempts."),
			events:   newCounter(meter, "hiretech.audit.outbox.relay.events", "Number of audit events projected by the relay."),
			errors:   newCounter(meter, "hiretech.audit.outbox.relay.errors", "Number of failed audit outbox relay attempts."),
			duration: newHistogram(meter, "hiretech.audit.outbox.relay.duration", "Audit outbox relay duration in seconds."),
		},
	}
}

func (r *OutboxRelay) relayCycle(ctx context.Context) (int, error) {
	total := 0
	for batch := 0; batch < r.maxBatchesPerCycle; batch++ {
		count, err := r.RelayOnce(ctx)
		total += count
		if err != nil || count < r.batch {
			return total, err
		}
		if err := ctx.Err(); err != nil {
			return total, err
		}
	}
	return total, nil
}

func newCounter(meter metric.Meter, name, description string) metric.Int64Counter {
	counter, err := meter.Int64Counter(name, metric.WithDescription(description))
	if err != nil {
		return nil
	}
	return counter
}

func newHistogram(meter metric.Meter, name, description string) metric.Float64Histogram {
	histogram, err := meter.Float64Histogram(name, metric.WithDescription(description), metric.WithUnit("s"))
	if err != nil {
		return nil
	}
	return histogram
}

// RelayOnce drains one bounded batch. It is exposed for deterministic tests
// and for operators that want to run a single controlled pass.
func (r *OutboxRelay) RelayOnce(ctx context.Context) (int, error) {
	if r == nil || r.repo == nil {
		return 0, nil
	}
	started := time.Now()
	if r.metrics.batches != nil {
		r.metrics.batches.Add(ctx, 1)
	}

	count, err := r.repo.RelayPending(ctx, r.batch)
	if err != nil {
		if r.metrics.errors != nil {
			r.metrics.errors.Add(ctx, 1)
		}
	} else if r.metrics.events != nil && count > 0 {
		r.metrics.events.Add(ctx, int64(count))
	}
	if r.metrics.duration != nil {
		r.metrics.duration.Record(ctx, time.Since(started).Seconds())
	}
	return count, err
}

// Run starts the small in-process relay loop and exits with ctx.
func (r *OutboxRelay) Run(ctx context.Context) {
	if r == nil || r.repo == nil {
		return
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		if _, err := r.relayCycle(ctx); err != nil && r.logger != nil && ctx.Err() == nil {
			r.logger.Error("audit outbox relay failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
