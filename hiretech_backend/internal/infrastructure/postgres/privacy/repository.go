package privacy

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	privacyModel "github.com/masterfabric-go/masterfabric/internal/domain/privacy/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) CreateRequest(ctx context.Context, request *privacyModel.Request) error {
	_, err := r.db.Exec(ctx, `INSERT INTO privacy_requests (id,user_id,organization_id,kind,status,reason,manifest,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, request.ID, request.UserID, request.OrganizationID, request.Kind, request.Status, request.Reason, request.Manifest, request.CreatedAt)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to create privacy request", err)
	}
	return nil
}

func (r *Repository) GetRequest(ctx context.Context, userID, requestID uuid.UUID) (*privacyModel.Request, error) {
	var request privacyModel.Request
	err := r.db.QueryRow(ctx, `SELECT id,user_id,organization_id,kind,status,reason,manifest,created_at,completed_at FROM privacy_requests WHERE id=$1 AND user_id=$2`, requestID, userID).Scan(&request.ID, &request.UserID, &request.OrganizationID, &request.Kind, &request.Status, &request.Reason, &request.Manifest, &request.CreatedAt, &request.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "privacy request not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get privacy request", err)
	}
	return &request, nil
}

func (r *Repository) HasActiveHold(ctx context.Context, userID, organizationID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM privacy_legal_holds WHERE released_at IS NULL AND organization_id=$1 AND (user_id=$2 OR user_id IS NULL))`, organizationID, userID).Scan(&exists)
	return exists, err
}

func (r *Repository) CreateLegalHold(ctx context.Context, hold *privacyModel.LegalHold) error {
	if hold.ID == uuid.Nil {
		hold.ID = uuid.New()
	}
	if hold.CreatedAt.IsZero() {
		hold.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(ctx, `INSERT INTO privacy_legal_holds (id,organization_id,user_id,resource_type,resource_id,reason,created_by,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, hold.ID, hold.OrganizationID, hold.UserID, hold.ResourceType, hold.ResourceID, hold.Reason, hold.CreatedBy, hold.CreatedAt)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to create legal hold", err)
	}
	return nil
}

func (r *Repository) ReleaseLegalHold(ctx context.Context, organizationID, holdID uuid.UUID) error {
	result, err := r.db.Exec(ctx, `UPDATE privacy_legal_holds SET released_at=NOW() WHERE id=$1 AND organization_id=$2 AND released_at IS NULL`, holdID, organizationID)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to release legal hold", err)
	}
	if result.RowsAffected() == 0 {
		return domainErr.New(domainErr.ErrNotFound, "legal hold not found", nil)
	}
	return nil
}
