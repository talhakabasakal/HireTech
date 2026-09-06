package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/audit/model"
	"github.com/masterfabric-go/masterfabric/internal/shared/pagination"
)

// AuditRepository defines the interface for audit log persistence.
type AuditRepository interface {
	Create(ctx context.Context, log *model.AuditLog) error
	ListByOrg(ctx context.Context, orgID uuid.UUID, offset, limit int) ([]*model.AuditLog, int, error)
	ListByUser(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*model.AuditLog, int, error)
	ListByResource(ctx context.Context, resourceType, resourceID string, offset, limit int) ([]*model.AuditLog, int, error)
}

// OutboxRepository projects transactionally stored audit events into the
// durable audit log store. Implementations must be safe to retry.
type OutboxRepository interface {
	RelayPending(ctx context.Context, limit int) (int, error)
}

// OrganizationScopedUserAuditRepository is the safe form of user audit reads.
// Implementations must apply the organization predicate in storage.
type OrganizationScopedUserAuditRepository interface {
	ListByUserInOrg(ctx context.Context, orgID, userID uuid.UUID, offset, limit int) ([]*model.AuditLog, int, error)
}

// CursorAuditRepository is the bounded, organization-scoped audit read
// contract used by cursor-based API connections.
type CursorAuditRepository interface {
	ListByOrgPage(ctx context.Context, orgID uuid.UUID, after *pagination.Cursor, limit int) ([]*model.AuditLog, error)
}

// IntegrityRepository verifies the per-organization hash chain. A production
// monitor must treat any error as an alert and stop retention cleanup.
type IntegrityRepository interface {
	VerifyIntegrity(ctx context.Context, organizationID uuid.UUID) error
}

// RetentionRepository exposes bounded cleanup and legal-hold mutation. Holds
// are evaluated in storage so cleanup cannot race with an application-only flag.
type RetentionRepository interface {
	PurgeExpired(ctx context.Context, now time.Time, limit int) (int, error)
	SetLegalHold(ctx context.Context, resourceType, resourceID string, held bool) error
}
