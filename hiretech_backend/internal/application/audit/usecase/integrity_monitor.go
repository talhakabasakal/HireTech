package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	auditRepo "github.com/masterfabric-go/masterfabric/internal/domain/audit/repository"
)

// IntegrityAlertSink is deliberately tiny so alert delivery can be connected
// to the existing logger/metrics system without introducing a daemon.
type IntegrityAlertSink interface {
	Alert(ctx context.Context, alert IntegrityAlert) error
}

type IntegrityAlert struct {
	OrganizationID uuid.UUID
	CheckedAt      time.Time
	Reason         string
}

type IntegrityMonitor struct {
	repo     auditRepo.IntegrityRepository
	alert    IntegrityAlertSink
	now      func() time.Time
	interval time.Duration
	pageSize int
	maxOrgs  int
}

func NewIntegrityMonitor(repo auditRepo.IntegrityRepository, alert IntegrityAlertSink) *IntegrityMonitor {
	return NewIntegrityMonitorWithConfig(repo, alert, IntegrityMonitorConfig{})
}

type IntegrityMonitorConfig struct {
	Interval             time.Duration
	OrganizationPageSize int
	MaxOrganizations     int
}

// NewIntegrityMonitorWithConfig creates a bounded, fail-closed integrity
// monitor. It performs reads and verification only; retention cleanup is a
// separate operation and must not run after an integrity failure.
func NewIntegrityMonitorWithConfig(repo auditRepo.IntegrityRepository, alert IntegrityAlertSink, cfg IntegrityMonitorConfig) *IntegrityMonitor {
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.OrganizationPageSize <= 0 {
		cfg.OrganizationPageSize = 100
	}
	if cfg.MaxOrganizations <= 0 {
		cfg.MaxOrganizations = 1000
	}
	return &IntegrityMonitor{
		repo: repo, alert: alert, now: time.Now, interval: cfg.Interval,
		pageSize: cfg.OrganizationPageSize, maxOrgs: cfg.MaxOrganizations,
	}
}

// OrganizationIDLister supplies only organization IDs, keeping the monitor
// independent from tenant persistence while preserving bounded pagination.
type OrganizationIDLister func(ctx context.Context, offset, limit int) ([]uuid.UUID, int, error)

func (m *IntegrityMonitor) Check(ctx context.Context, organizationID uuid.UUID) error {
	if m == nil || m.repo == nil || organizationID == uuid.Nil {
		return fmt.Errorf("audit integrity monitor is not configured")
	}
	if err := m.repo.VerifyIntegrity(ctx, organizationID); err != nil {
		return m.alertFailure(ctx, organizationID, err)
	}
	return nil
}

func (m *IntegrityMonitor) alertFailure(ctx context.Context, organizationID uuid.UUID, reason error) error {
	if m.alert == nil {
		return reason
	}
	alertErr := m.alert.Alert(ctx, IntegrityAlert{OrganizationID: organizationID, CheckedAt: m.now().UTC(), Reason: reason.Error()})
	return errors.Join(reason, alertErr)
}

// CheckAll verifies every organization returned by the bounded lister. If the
// configured ceiling would omit organizations, the cycle fails and emits an
// alert instead of silently declaring a partial verification successful.
func (m *IntegrityMonitor) CheckAll(ctx context.Context, list OrganizationIDLister) error {
	if m == nil || m.repo == nil || list == nil {
		return fmt.Errorf("audit integrity monitor is not configured")
	}
	totalChecked := 0
	for offset := 0; ; {
		ids, total, err := list(ctx, offset, m.pageSize)
		if err != nil {
			return m.alertFailure(ctx, uuid.Nil, fmt.Errorf("list organizations for audit integrity: %w", err))
		}
		if totalChecked+len(ids) > m.maxOrgs || total > m.maxOrgs {
			return m.alertFailure(ctx, uuid.Nil, fmt.Errorf("audit integrity organization scan exceeds configured limit: total=%d limit=%d", total, m.maxOrgs))
		}
		for _, organizationID := range ids {
			if err := m.Check(ctx, organizationID); err != nil {
				return err
			}
			totalChecked++
		}
		if len(ids) == 0 || offset+len(ids) >= total {
			return nil
		}
		offset += len(ids)
	}
}

// Run performs one verification immediately and then repeats it on the
// configured interval. A failed cycle is logged and retried; it never becomes
// a successful cycle and never enables retention cleanup.
func (m *IntegrityMonitor) Run(ctx context.Context, list OrganizationIDLister, logger *slog.Logger) {
	if m == nil || m.repo == nil || list == nil {
		return
	}
	run := func() {
		if err := m.CheckAll(ctx, list); err != nil && ctx.Err() == nil && logger != nil {
			logger.ErrorContext(ctx, "audit integrity verification failed", "error", err)
		}
	}
	run()
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

// SlogIntegrityAlertSink emits a stable structured event consumed by the
// deployment's log/alert pipeline. No secret or audit payload is included.
type SlogIntegrityAlertSink struct{ logger *slog.Logger }

func NewSlogIntegrityAlertSink(logger *slog.Logger) *SlogIntegrityAlertSink {
	return &SlogIntegrityAlertSink{logger: logger}
}

func (s *SlogIntegrityAlertSink) Alert(ctx context.Context, alert IntegrityAlert) error {
	if s == nil || s.logger == nil {
		return errors.New("audit integrity alert logger is not configured")
	}
	s.logger.ErrorContext(ctx, "audit integrity alert",
		"alert_type", "audit_integrity_violation",
		"organization_id", alert.OrganizationID.String(),
		"checked_at", alert.CheckedAt.UTC().Format(time.RFC3339Nano),
		"reason", alert.Reason,
	)
	return nil
}
