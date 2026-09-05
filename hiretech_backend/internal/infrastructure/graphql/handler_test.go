package graphql_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	"github.com/masterfabric-go/masterfabric/internal/domain/audit/model"
	auditRepo "github.com/masterfabric-go/masterfabric/internal/domain/audit/repository"
	iamModel "github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	iamRepo "github.com/masterfabric-go/masterfabric/internal/domain/iam/repository"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	orgModel "github.com/masterfabric-go/masterfabric/internal/domain/tenant/model"
	orgRepo "github.com/masterfabric-go/masterfabric/internal/domain/tenant/repository"
	infraAuth "github.com/masterfabric-go/masterfabric/internal/infrastructure/auth"
	graphHTTP "github.com/masterfabric-go/masterfabric/internal/infrastructure/graphql"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
)

type graphResponse struct {
	Data   map[string]json.RawMessage `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
}

type fakeUserRepo struct{ user *iamModel.User }

func (r *fakeUserRepo) Create(context.Context, *iamModel.User) error { return nil }
func (r *fakeUserRepo) GetByID(context.Context, uuid.UUID) (*iamModel.User, error) {
	if r.user == nil {
		return nil, domainErr.New(domainErr.ErrNotFound, "user not found", nil)
	}
	return r.user, nil
}
func (r *fakeUserRepo) GetByEmail(context.Context, string) (*iamModel.User, error) {
	return r.user, nil
}
func (r *fakeUserRepo) Update(context.Context, *iamModel.User) error { return nil }
func (r *fakeUserRepo) Delete(context.Context, uuid.UUID) error      { return nil }
func (r *fakeUserRepo) List(context.Context, int, int) ([]*iamModel.User, int, error) {
	return nil, 0, nil
}

var _ iamRepo.UserRepository = (*fakeUserRepo)(nil)

type fakeOrgRepo struct {
	organizations map[uuid.UUID]*orgModel.Organization
}

func (r *fakeOrgRepo) Create(context.Context, *orgModel.Organization) error { return nil }
func (r *fakeOrgRepo) GetByID(_ context.Context, id uuid.UUID) (*orgModel.Organization, error) {
	org, ok := r.organizations[id]
	if !ok {
		return nil, domainErr.New(domainErr.ErrNotFound, "organization not found", nil)
	}
	return org, nil
}
func (r *fakeOrgRepo) GetBySlug(context.Context, string) (*orgModel.Organization, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeOrgRepo) Update(context.Context, *orgModel.Organization) error { return nil }
func (r *fakeOrgRepo) Delete(context.Context, uuid.UUID) error              { return nil }
func (r *fakeOrgRepo) List(context.Context, int, int) ([]*orgModel.Organization, int, error) {
	return nil, 0, nil
}

var _ orgRepo.OrgRepository = (*fakeOrgRepo)(nil)

type fakeMembershipRepo struct {
	memberships map[uuid.UUID]*iamModel.OrganizationUser
}

func (r *fakeMembershipRepo) Add(context.Context, *iamModel.OrganizationUser) error { return nil }
func (r *fakeMembershipRepo) Remove(context.Context, uuid.UUID, uuid.UUID) error    { return nil }
func (r *fakeMembershipRepo) GetByOrgAndUser(_ context.Context, orgID, userID uuid.UUID) (*iamModel.OrganizationUser, error) {
	membership, ok := r.memberships[orgID]
	if !ok || membership.UserID != userID {
		return nil, domainErr.New(domainErr.ErrNotFound, "organization user not found", nil)
	}
	return membership, nil
}
func (r *fakeMembershipRepo) ListByOrg(context.Context, uuid.UUID, int, int) ([]*iamModel.OrganizationUser, int, error) {
	return nil, 0, nil
}
func (r *fakeMembershipRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]*iamModel.OrganizationUser, error) {
	result := make([]*iamModel.OrganizationUser, 0)
	for _, membership := range r.memberships {
		if membership.UserID == userID {
			result = append(result, membership)
		}
	}
	return result, nil
}

var _ iamRepo.OrgUserRepository = (*fakeMembershipRepo)(nil)

type fakeAuditRepo struct{ entries []*model.AuditLog }

func (r *fakeAuditRepo) Create(_ context.Context, entry *model.AuditLog) error {
	r.entries = append(r.entries, entry)
	return nil
}
func (r *fakeAuditRepo) ListByOrg(context.Context, uuid.UUID, int, int) ([]*model.AuditLog, int, error) {
	return nil, 0, nil
}
func (r *fakeAuditRepo) ListByUser(context.Context, uuid.UUID, int, int) ([]*model.AuditLog, int, error) {
	return nil, 0, nil
}
func (r *fakeAuditRepo) ListByResource(context.Context, string, string, int, int) ([]*model.AuditLog, int, error) {
	return nil, 0, nil
}

var _ auditRepo.AuditRepository = (*fakeAuditRepo)(nil)

func newGraphHandler(t *testing.T) (*graphHTTP.Handler, *infraAuth.JWTService, uuid.UUID, uuid.UUID, uuid.UUID, *fakeAuditRepo) {
	t.Helper()
	userID := uuid.New()
	orgA, orgB := uuid.New(), uuid.New()
	now := time.Now().UTC()
	users := &fakeUserRepo{user: &iamModel.User{ID: userID, Email: "actor@example.com", FirstName: "Act", LastName: "Or", Status: iamModel.UserStatusActive, CreatedAt: now}}
	organizations := &fakeOrgRepo{organizations: map[uuid.UUID]*orgModel.Organization{
		orgA: {ID: orgA, Name: "Alpha", Slug: "alpha", Status: orgModel.OrgStatusActive, CreatedAt: now},
		orgB: {ID: orgB, Name: "Beta", Slug: "beta", Status: orgModel.OrgStatusActive, CreatedAt: now},
	}}
	memberships := &fakeMembershipRepo{memberships: map[uuid.UUID]*iamModel.OrganizationUser{
		orgA: {OrganizationID: orgA, UserID: userID, Status: iamModel.OrgUserStatusActive},
	}}
	jwtService := infraAuth.NewJWTService(config.JWTConfig{Secret: "graph-test-secret", ExpirationHours: 1, Issuer: "test"})
	uc := usecase.NewOrganizationContextUseCase(users, organizations, memberships, jwtService, nil)
	audit := &fakeAuditRepo{}
	return graphHTTP.NewHandler(graphHTTP.Dependencies{AuthService: jwtService, UseCase: uc}), jwtService, userID, orgA, orgB, audit
}

func token(t *testing.T, jwtService *infraAuth.JWTService, userID uuid.UUID, class authcontext.TokenClass, orgID uuid.UUID) string {
	t.Helper()
	signed, err := jwtService.GenerateToken(context.Background(), service.TokenClaims{UserID: userID, Email: "actor@example.com", TokenClass: string(class), OrganizationID: orgID})
	require.NoError(t, err)
	return signed
}

func perform(t *testing.T, h http.Handler, bearer, query string, headers map[string]string) graphResponse {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"query": query})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	var decoded graphResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &decoded), res.Body.String())
	return decoded
}

func protectedHandler(t *testing.T) (http.Handler, *infraAuth.JWTService, uuid.UUID, uuid.UUID, uuid.UUID, *fakeAuditRepo) {
	h, jwtService, userID, orgA, orgB, audit := newGraphHandler(t)
	return middleware.RequestID(middleware.AuditLog(audit)(graphHTTP.AuthMiddleware(jwtService)(h))), jwtService, userID, orgA, orgB, audit
}

func TestGraphQLRequiresAuthenticationAndRejectsTenantHeaders(t *testing.T) {
	h, jwtService, userID, _, _, _ := protectedHandler(t)

	unauthenticated := perform(t, h, "", "query { me { id } }", nil)
	require.Len(t, unauthenticated.Errors, 1)
	assert.Equal(t, "UNAUTHENTICATED", unauthenticated.Errors[0].Extensions["code"])

	bootstrap := token(t, jwtService, userID, authcontext.TokenClassBootstrap, uuid.Nil)
	forgedHeader := perform(t, h, bootstrap, "query { organizations { organization { id } } }", map[string]string{"X-Organization-ID": uuid.NewString()})
	assert.Equal(t, "VALIDATION_FAILED", forgedHeader.Errors[0].Extensions["code"])
}

func TestGraphQLOrganizationsAreMembershipScopedAndSelectionIssuesTenantToken(t *testing.T) {
	h, jwtService, userID, orgA, orgB, audit := protectedHandler(t)
	bootstrap := token(t, jwtService, userID, authcontext.TokenClassBootstrap, uuid.Nil)

	organizations := perform(t, h, bootstrap, "query { organizations { organization { id name } status } }", nil)
	require.Empty(t, organizations.Errors)
	var rows []struct {
		Organization struct {
			ID string `json:"id"`
		} `json:"organization"`
	}
	require.NoError(t, json.Unmarshal(organizations.Data["organizations"], &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, orgA.String(), rows[0].Organization.ID)

	selected := perform(t, h, bootstrap, "mutation { selectOrganization(organizationId: \""+orgA.String()+"\") { accessToken tokenClass organization { id } } }", nil)
	require.Empty(t, selected.Errors)
	var payload struct{ AccessToken, TokenClass string }
	require.NoError(t, json.Unmarshal(selected.Data["selectOrganization"], &payload))
	assert.Equal(t, "TENANT", payload.TokenClass)
	claims, err := jwtService.ValidateToken(context.Background(), payload.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, orgA, claims.OrganizationID)
	assert.Equal(t, "tenant", claims.TokenClass)
	tenantRead := perform(t, h, payload.AccessToken, "query { me { id } }", nil)
	require.Empty(t, tenantRead.Errors)

	crossTenant := perform(t, h, bootstrap, "mutation { selectOrganization(organizationId: \""+orgB.String()+"\") { accessToken } }", nil)
	require.Len(t, crossTenant.Errors, 1)
	assert.Equal(t, "FORBIDDEN", crossTenant.Errors[0].Extensions["code"])

	candidate := token(t, jwtService, userID, authcontext.TokenClassCandidateInterview, orgA)
	forbidden := perform(t, h, candidate, "query { organizations { organization { id } } }", nil)
	require.Len(t, forbidden.Errors, 1)
	assert.Equal(t, "FORBIDDEN", forbidden.Errors[0].Extensions["code"])

	require.Len(t, audit.entries, 5)
	assert.Equal(t, "graphql_operation", audit.entries[0].ResourceType)
	assert.Equal(t, userID, *audit.entries[1].UserID)
	assert.Equal(t, orgA, audit.entries[4].OrganizationID)
	var metadata map[string]any
	require.NoError(t, json.Unmarshal(audit.entries[1].Metadata, &metadata))
	assert.Contains(t, metadata, "operation_name")
	assert.Contains(t, metadata, "duration_ms")
	assert.NotContains(t, string(audit.entries[1].Metadata), "accessToken")
}

func TestGraphQLLimitsAliasesAndBatches(t *testing.T) {
	h, jwtService, userID, _, _, _ := protectedHandler(t)
	bootstrap := token(t, jwtService, userID, authcontext.TokenClassBootstrap, uuid.Nil)

	aliased := perform(t, h, bootstrap, "query { first: me { id } }", nil)
	require.Len(t, aliased.Errors, 1)
	assert.Equal(t, "VALIDATION_FAILED", aliased.Errors[0].Extensions["code"])

	batched := perform(t, h, bootstrap, "query One { me { id } } query Two { me { id } }", nil)
	require.Len(t, batched.Errors, 1)
	assert.Equal(t, "VALIDATION_FAILED", batched.Errors[0].Extensions["code"])
}

func TestGraphQLAuthorizesEveryRootField(t *testing.T) {
	h, jwtService, userID, _, _, _ := protectedHandler(t)
	bootstrap := token(t, jwtService, userID, authcontext.TokenClassBootstrap, uuid.Nil)

	response := perform(t, h, bootstrap, "query { me { id } interviews { id } }", nil)
	require.Len(t, response.Errors, 1)
	assert.Equal(t, "FORBIDDEN", response.Errors[0].Extensions["code"])
}

func TestGraphQLAllowsTenantAdminBoundaryToReachResolver(t *testing.T) {
	h, jwtService, userID, orgID, _, _ := protectedHandler(t)
	tenant := token(t, jwtService, userID, authcontext.TokenClassTenant, orgID)

	response := perform(t, h, tenant, "query { adminWorkspace { models { id } } }", nil)
	require.Len(t, response.Errors, 1)
	assert.NotEqual(t, "FORBIDDEN", response.Errors[0].Extensions["code"])
}
