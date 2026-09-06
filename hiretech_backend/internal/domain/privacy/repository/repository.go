package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/privacy/model"
)

type Repository interface {
	CreateRequest(ctx context.Context, request *model.Request) error
	GetRequest(ctx context.Context, userID, requestID uuid.UUID) (*model.Request, error)
	HasActiveHold(ctx context.Context, userID, organizationID uuid.UUID) (bool, error)
	CreateLegalHold(ctx context.Context, hold *model.LegalHold) error
	ReleaseLegalHold(ctx context.Context, organizationID, holdID uuid.UUID) error
}
