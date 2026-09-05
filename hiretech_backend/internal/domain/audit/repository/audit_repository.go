package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/audit/model"
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
