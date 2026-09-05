package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	testrequire "github.com/stretchr/testify/require"

	aiadminModel "github.com/masterfabric-go/masterfabric/internal/domain/aiadmin/model"
	aiadminRepo "github.com/masterfabric-go/masterfabric/internal/domain/aiadmin/repository"
	auditModel "github.com/masterfabric-go/masterfabric/internal/domain/audit/model"
	auditRepo "github.com/masterfabric-go/masterfabric/internal/domain/audit/repository"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type fakeRepository struct {
	modelOrg   uuid.UUID
	modelActor uuid.UUID
	models     map[string]*aiadminModel.Model
}

type fakeAuditRepository struct {
	logs []*auditModel.AuditLog
	err  error
}

func (r *fakeAuditRepository) Create(context.Context, *auditModel.AuditLog) error { return nil }
func (r *fakeAuditRepository) ListByOrg(context.Context, uuid.UUID, int, int) ([]*auditModel.AuditLog, int, error) {
	return r.logs, len(r.logs), r.err
}
func (r *fakeAuditRepository) ListByUser(context.Context, uuid.UUID, int, int) ([]*auditModel.AuditLog, int, error) {
	return nil, 0, nil
}
func (r *fakeAuditRepository) ListByResource(context.Context, string, string, int, int) ([]*auditModel.AuditLog, int, error) {
	return nil, 0, nil
}

func (r *fakeRepository) GetWorkspace(context.Context, uuid.UUID) (*aiadminModel.Workspace, error) {
	return &aiadminModel.Workspace{Models: []*aiadminModel.Model{}, Prompts: []*aiadminModel.Prompt{}, Routing: []*aiadminModel.RoutingRule{}, Versions: []*aiadminModel.ConfigurationVersion{}, AuditEvents: []*aiadminModel.AuditEvent{}}, nil
}

func (r *fakeRepository) GetActiveWorkspace(ctx context.Context, org uuid.UUID) (*aiadminModel.Workspace, error) {
	return r.GetWorkspace(ctx, org)
}

func (r *fakeRepository) GetModel(_ context.Context, _ uuid.UUID, modelID string) (*aiadminModel.Model, error) {
	if r.models == nil {
		return nil, domainErr.New(domainErr.ErrNotFound, "AI model not found", nil)
	}
	return r.models[modelID], nil
}

func (r *fakeRepository) Approve(context.Context, uuid.UUID, uuid.UUID, string, string, int) (*aiadminModel.ConfigurationVersion, error) {
	return &aiadminModel.ConfigurationVersion{Status: aiadminModel.ConfigurationActive}, nil
}

func (r *fakeRepository) Rollback(context.Context, uuid.UUID, uuid.UUID, string, string, int) (*aiadminModel.ConfigurationVersion, error) {
	return &aiadminModel.ConfigurationVersion{Status: aiadminModel.ConfigurationActive}, nil
}

func (r *fakeRepository) RegisterModel(_ context.Context, org, actor uuid.UUID, _ aiadminModel.RegisterModelInput) (*aiadminModel.Model, error) {
	r.modelOrg, r.modelActor = org, actor
	return &aiadminModel.Model{ID: uuid.New(), ModelID: "model/test", DisplayName: "Test", ProviderLabel: "managed", Roles: []aiadminModel.Role{aiadminModel.RoleInterviewer}, Status: aiadminModel.ModelActive, LatencyClass: aiadminModel.LatencyBalanced}, nil
}

func (r *fakeRepository) CreatePromptVersion(context.Context, uuid.UUID, uuid.UUID, aiadminModel.CreatePromptInput) (*aiadminModel.Prompt, error) {
	return &aiadminModel.Prompt{}, nil
}

func (r *fakeRepository) UpdateRouting(context.Context, uuid.UUID, uuid.UUID, aiadminModel.UpdateRoutingInput) (*aiadminModel.RoutingRule, error) {
	return &aiadminModel.RoutingRule{}, nil
}

func (r *fakeRepository) PublishRubric(context.Context, uuid.UUID, uuid.UUID, aiadminModel.PublishRubricInput) (*aiadminModel.Rubric, error) {
	return &aiadminModel.Rubric{}, nil
}

var _ aiadminRepo.Repository = (*fakeRepository)(nil)
var _ auditRepo.AuditRepository = (*fakeAuditRepository)(nil)

func adminActor(org uuid.UUID, permissions ...string) authcontext.ActorContext {
	return authcontext.ActorContext{UserID: uuid.New(), OrganizationID: org, TokenClass: authcontext.TokenClassTenant, Permissions: permissions, AuthenticationMethods: []string{"otp"}, AuthenticationTime: time.Now().UTC()}
}

func requireForbidden(t *testing.T, err error) {
	t.Helper()
	testrequire.Error(t, err)
	testrequire.True(t, errors.Is(err, domainErr.ErrForbidden))
}

func TestWorkspaceAcceptsReadWildcardsAndRequiresAuditRead(t *testing.T) {
	org := uuid.New()
	service := NewService(&fakeRepository{}, &fakeAuditRepository{})

	_, err := service.Workspace(context.Background(), adminActor(org, "*:read"))
	testrequire.NoError(t, err)

	_, err = service.Workspace(context.Background(), adminActor(org, "ai_config:read"))
	requireForbidden(t, err)
}

func TestWorkspacePreservesDeniedAuditOutcomeAndFailsClosedOnAuditErrors(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	denied := &auditModel.AuditLog{ID: uuid.New(), UserID: &user, Action: "graphql.adminWorkspace", ResourceType: "graphql_operation", ResourceID: "AdminWorkspace", Metadata: []byte(`{"outcome":"DENIED"}`), CreatedAt: time.Now().UTC()}
	service := NewService(&fakeRepository{}, &fakeAuditRepository{logs: []*auditModel.AuditLog{denied}})

	workspace, err := service.Workspace(context.Background(), adminActor(org, "*:read"))
	testrequire.NoError(t, err)
	testrequire.Len(t, workspace.AuditEvents, 1)
	testrequire.Equal(t, "denied", workspace.AuditEvents[0].Result)
	testrequire.Equal(t, "graphql_operation:AdminWorkspace", workspace.AuditEvents[0].Target)

	service = NewService(&fakeRepository{}, &fakeAuditRepository{err: errors.New("audit unavailable")})
	_, err = service.Workspace(context.Background(), adminActor(org, "*:read"))
	testrequire.Error(t, err)
}

func TestMutationsUseTenantScopeAndManagePermission(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	repo := &fakeRepository{}
	service := NewService(repo, nil)

	value, err := service.RegisterModel(context.Background(), authcontext.ActorContext{UserID: user, OrganizationID: org, TokenClass: authcontext.TokenClassTenant, Permissions: []string{"ai_config:*"}, AuthenticationMethods: []string{"otp"}, AuthenticationTime: time.Now().UTC()}, aiadminModel.RegisterModelInput{
		ModelID: "model/test", DisplayName: "Test", ProviderLabel: "managed", Roles: []aiadminModel.Role{aiadminModel.RoleInterviewer},
	})
	testrequire.NoError(t, err)
	testrequire.NotEqual(t, uuid.Nil, value.ID)
	testrequire.Equal(t, org, repo.modelOrg)
	testrequire.Equal(t, user, repo.modelActor)

	_, err = service.RegisterModel(context.Background(), adminActor(org, "ai_config:read"), aiadminModel.RegisterModelInput{ModelID: "model/test", DisplayName: "Test", ProviderLabel: "managed", Roles: []aiadminModel.Role{aiadminModel.RoleInterviewer}})
	requireForbidden(t, err)
}

func TestMutationsRequireRecentMFA(t *testing.T) {
	service := NewService(&fakeRepository{}, nil)
	actor := authcontext.ActorContext{
		UserID:                uuid.New(),
		OrganizationID:        uuid.New(),
		TokenClass:            authcontext.TokenClassTenant,
		Permissions:           []string{"ai_config:manage"},
		AuthenticationMethods: []string{"password"},
		AuthenticationTime:    time.Now().UTC(),
	}

	_, err := service.RegisterModel(context.Background(), actor, aiadminModel.RegisterModelInput{
		ModelID: "model/test", DisplayName: "Test", ProviderLabel: "managed", Roles: []aiadminModel.Role{aiadminModel.RoleInterviewer},
	})
	testrequire.ErrorIs(t, err, domainErr.ErrRecentAuthenticationRequired)
}

func TestAdminOperationsRejectNonTenantTokens(t *testing.T) {
	service := NewService(&fakeRepository{}, nil)
	candidate := authcontext.ActorContext{UserID: uuid.New(), OrganizationID: uuid.New(), TokenClass: authcontext.TokenClassCandidateInterview, Permissions: []string{"*"}}

	_, err := service.RegisterModel(context.Background(), candidate, aiadminModel.RegisterModelInput{ModelID: "model/test", DisplayName: "Test", ProviderLabel: "managed", Roles: []aiadminModel.Role{aiadminModel.RoleInterviewer}})
	requireForbidden(t, err)
}

func TestRoutingRequiresDistinctRegisteredModelsForRole(t *testing.T) {
	org := uuid.New()
	repo := &fakeRepository{models: map[string]*aiadminModel.Model{
		"primary":  {ModelID: "primary", Roles: []aiadminModel.Role{aiadminModel.RoleEvaluator}, Status: aiadminModel.ModelActive},
		"fallback": {ModelID: "fallback", Roles: []aiadminModel.Role{aiadminModel.RoleEvaluator}, Status: aiadminModel.ModelFallback},
	}}
	service := NewService(repo, nil)
	actor := adminActor(org, "ai_config:manage")

	_, err := service.UpdateRouting(context.Background(), actor, aiadminModel.UpdateRoutingInput{Role: aiadminModel.RoleEvaluator, PrimaryModelID: "primary", FallbackModelID: "fallback", TimeoutSeconds: 30, Enabled: true})
	testrequire.NoError(t, err)

	_, err = service.UpdateRouting(context.Background(), actor, aiadminModel.UpdateRoutingInput{Role: aiadminModel.RoleEvaluator, PrimaryModelID: "primary", FallbackModelID: "missing", TimeoutSeconds: 30, Enabled: true})
	testrequire.Error(t, err)

	_, err = service.UpdateRouting(context.Background(), actor, aiadminModel.UpdateRoutingInput{Role: aiadminModel.RoleEvaluator, PrimaryModelID: "primary", FallbackModelID: "primary", TimeoutSeconds: 30, Enabled: true})
	testrequire.Error(t, err)
}
