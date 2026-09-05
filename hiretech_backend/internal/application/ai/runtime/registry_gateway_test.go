package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	aiModel "github.com/masterfabric-go/masterfabric/internal/domain/ai/model"
	aiService "github.com/masterfabric-go/masterfabric/internal/domain/ai/service"
	aiadminModel "github.com/masterfabric-go/masterfabric/internal/domain/aiadmin/model"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
)

type registryRepository struct {
	workspace *aiadminModel.Workspace
}

func (r *registryRepository) GetWorkspace(context.Context, uuid.UUID) (*aiadminModel.Workspace, error) {
	return r.workspace, nil
}

func (r *registryRepository) GetActiveWorkspace(context.Context, uuid.UUID) (*aiadminModel.Workspace, error) {
	return r.workspace, nil
}

func (r *registryRepository) GetModel(context.Context, uuid.UUID, string) (*aiadminModel.Model, error) {
	return nil, errors.New("not used")
}

func (r *registryRepository) Approve(context.Context, uuid.UUID, uuid.UUID, string, string, int) (*aiadminModel.ConfigurationVersion, error) {
	return nil, errors.New("not used")
}

func (r *registryRepository) Rollback(context.Context, uuid.UUID, uuid.UUID, string, string, int) (*aiadminModel.ConfigurationVersion, error) {
	return nil, errors.New("not used")
}

func (r *registryRepository) RegisterModel(context.Context, uuid.UUID, uuid.UUID, aiadminModel.RegisterModelInput) (*aiadminModel.Model, error) {
	return nil, errors.New("not used")
}

func (r *registryRepository) CreatePromptVersion(context.Context, uuid.UUID, uuid.UUID, aiadminModel.CreatePromptInput) (*aiadminModel.Prompt, error) {
	return nil, errors.New("not used")
}

func (r *registryRepository) UpdateRouting(context.Context, uuid.UUID, uuid.UUID, aiadminModel.UpdateRoutingInput) (*aiadminModel.RoutingRule, error) {
	return nil, errors.New("not used")
}

func (r *registryRepository) PublishRubric(context.Context, uuid.UUID, uuid.UUID, aiadminModel.PublishRubricInput) (*aiadminModel.Rubric, error) {
	return nil, errors.New("not used")
}

type recordingGateway struct {
	requests []aiModel.CompletionRequest
	response aiModel.CompletionResponse
	err      error
}

func (g *recordingGateway) Complete(_ context.Context, request aiModel.CompletionRequest) (aiModel.CompletionResponse, error) {
	g.requests = append(g.requests, request)
	return g.response, g.err
}

func TestRegistryGatewayUsesApprovedTenantConfigurationAndFallback(t *testing.T) {
	workspace := &aiadminModel.Workspace{
		Models: []*aiadminModel.Model{
			{ModelID: "primary", ProviderLabel: "primary-provider", Roles: []aiadminModel.Role{aiadminModel.RoleInterviewer}, Status: aiadminModel.ModelActive},
			{ModelID: "fallback", ProviderLabel: "fallback-provider", Roles: []aiadminModel.Role{aiadminModel.RoleInterviewer}, Status: aiadminModel.ModelFallback},
		},
		Prompts: []*aiadminModel.Prompt{{Role: aiadminModel.RoleInterviewer, Prompt: "approved system prompt", Status: aiadminModel.PromptActive}},
		Routing: []*aiadminModel.RoutingRule{{Role: aiadminModel.RoleInterviewer, PrimaryModelID: "primary", FallbackModelID: "fallback", Enabled: true}},
	}
	primary := &recordingGateway{err: errors.New("primary unavailable")}
	fallback := &recordingGateway{response: aiModel.CompletionResponse{Content: "ok", ModelID: "fallback"}}
	gateway := NewRegistryGateway(&registryRepository{workspace: workspace}, map[string]aiService.Gateway{
		"primary-provider":  primary,
		"fallback-provider": fallback,
	}, nil)
	ctx := authcontext.WithActor(context.Background(), authcontext.ActorContext{UserID: uuid.New(), OrganizationID: uuid.New()})

	response, err := gateway.Complete(ctx, aiModel.CompletionRequest{Role: aiModel.RoleInterviewer, Language: aiModel.LanguageTurkish, Messages: []aiModel.Message{{Role: "user", Content: "soru"}}})
	require.NoError(t, err)
	require.Equal(t, "ok", response.Content)
	require.Len(t, primary.requests, 1)
	require.Len(t, fallback.requests, 1)
	require.Equal(t, "primary", primary.requests[0].ModelID)
	require.Equal(t, "fallback", fallback.requests[0].ModelID)
	require.Equal(t, aiModel.Message{Role: "system", Content: "approved system prompt"}, fallback.requests[0].Messages[0])
}

func TestRegistryGatewayFallsBackToStaticGatewayWhenNoApprovedRoute(t *testing.T) {
	static := &recordingGateway{response: aiModel.CompletionResponse{Content: "static"}}
	gateway := NewRegistryGateway(&registryRepository{workspace: &aiadminModel.Workspace{}}, nil, map[aiadminModel.Role]aiService.Gateway{
		aiadminModel.RoleEvaluator: static,
	})
	ctx := authcontext.WithActor(context.Background(), authcontext.ActorContext{UserID: uuid.New(), OrganizationID: uuid.New()})

	response, err := gateway.Complete(ctx, aiModel.CompletionRequest{Role: aiModel.RoleEvaluator, Language: aiModel.LanguageEnglish, Messages: []aiModel.Message{{Role: "user", Content: "evaluate"}}})
	require.NoError(t, err)
	require.Equal(t, "static", response.Content)
	require.Len(t, static.requests, 1)
}
