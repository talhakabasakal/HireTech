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

// OrganizationScopedUserAuditRepository is the safe form of user audit reads.
// Implementations must apply the organization predicate in storage.
type OrganizationScopedUserAuditRepository interface {
	ListByUserInOrg(ctx context.Context, orgID, userID uuid.UUID, offset, limit int) ([]*model.AuditLog, int, error)
}
