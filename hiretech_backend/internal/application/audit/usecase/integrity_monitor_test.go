package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type integrityRepositoryStub struct {
	checked []uuid.UUID
	err     error
}

func (s *integrityRepositoryStub) VerifyIntegrity(_ context.Context, organizationID uuid.UUID) error {
	s.checked = append(s.checked, organizationID)
	return s.err
}

type integrityAlertStub struct {
	alerts []IntegrityAlert
}

func (s *integrityAlertStub) Alert(_ context.Context, alert IntegrityAlert) error {
	s.alerts = append(s.alerts, alert)
	return nil
}

func TestIntegrityMonitorCheckAllPaginatesOrganizations(t *testing.T) {
	repo := &integrityRepositoryStub{}
	alerts := &integrityAlertStub{}
	monitor := NewIntegrityMonitorWithConfig(repo, alerts, IntegrityMonitorConfig{OrganizationPageSize: 2, MaxOrganizations: 10})
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	err := monitor.CheckAll(context.Background(), func(_ context.Context, offset, limit int) ([]uuid.UUID, int, error) {
		if limit != 2 {
			t.Fatalf("unexpected page size: %d", limit)
		}
		if offset >= len(ids) {
			return nil, len(ids), nil
		}
		end := offset + limit
		if end > len(ids) {
			end = len(ids)
		}
		return ids[offset:end], len(ids), nil
	})
	if err != nil {
		t.Fatalf("check all failed: %v", err)
	}
	if len(repo.checked) != len(ids) {
		t.Fatalf("checked %d organizations, want %d", len(repo.checked), len(ids))
	}
	if len(alerts.alerts) != 0 {
		t.Fatalf("unexpected alerts: %+v", alerts.alerts)
	}
}

func TestIntegrityMonitorAlertsOnVerificationFailure(t *testing.T) {
	repo := &integrityRepositoryStub{err: errors.New("chain mismatch")}
	alerts := &integrityAlertStub{}
	organizationID := uuid.New()
	monitor := NewIntegrityMonitor(repo, alerts)

	err := monitor.Check(context.Background(), organizationID)
	if err == nil || !errors.Is(err, repo.err) {
		t.Fatalf("expected verification error, got %v", err)
	}
	if len(alerts.alerts) != 1 || alerts.alerts[0].OrganizationID != organizationID {
		t.Fatalf("expected one organization alert, got %+v", alerts.alerts)
	}
}

func TestIntegrityMonitorRejectsPartialScan(t *testing.T) {
	repo := &integrityRepositoryStub{}
	alerts := &integrityAlertStub{}
	monitor := NewIntegrityMonitorWithConfig(repo, alerts, IntegrityMonitorConfig{MaxOrganizations: 1})

	err := monitor.CheckAll(context.Background(), func(_ context.Context, _, _ int) ([]uuid.UUID, int, error) {
		return []uuid.UUID{uuid.New()}, 2, nil
	})
	if err == nil {
		t.Fatal("expected partial scan to fail")
	}
	if len(alerts.alerts) != 1 || alerts.alerts[0].OrganizationID != uuid.Nil {
		t.Fatalf("expected scan alert, got %+v", alerts.alerts)
	}
}

func TestSlogIntegrityAlertSinkRequiresLogger(t *testing.T) {
	sink := NewSlogIntegrityAlertSink(nil)
	err := sink.Alert(context.Background(), IntegrityAlert{CheckedAt: time.Now()})
	if err == nil {
		t.Fatal("expected missing logger error")
	}
}
