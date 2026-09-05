package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/aiadmin/model"
)

type Repository interface {
	GetWorkspace(ctx context.Context, organizationID uuid.UUID) (*model.Workspace, error)
	GetActiveWorkspace(ctx context.Context, organizationID uuid.UUID) (*model.Workspace, error)
	GetModel(ctx context.Context, organizationID uuid.UUID, modelID string) (*model.Model, error)
	Approve(ctx context.Context, organizationID, actorID uuid.UUID, resource, key string, version int) (*model.ConfigurationVersion, error)
	Rollback(ctx context.Context, organizationID, actorID uuid.UUID, resource, key string, version int) (*model.ConfigurationVersion, error)
	RegisterModel(ctx context.Context, organizationID, actorID uuid.UUID, input model.RegisterModelInput) (*model.Model, error)
	CreatePromptVersion(ctx context.Context, organizationID, actorID uuid.UUID, input model.CreatePromptInput) (*model.Prompt, error)
	UpdateRouting(ctx context.Context, organizationID, actorID uuid.UUID, input model.UpdateRoutingInput) (*model.RoutingRule, error)
	PublishRubric(ctx context.Context, organizationID, actorID uuid.UUID, input model.PublishRubricInput) (*model.Rubric, error)
}
