package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/masterfabric-go/masterfabric/internal/application/iam/dto"
	iamModel "github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	iamRepo "github.com/masterfabric-go/masterfabric/internal/domain/iam/repository"
	iamService "github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	tenantModel "github.com/masterfabric-go/masterfabric/internal/domain/tenant/model"
	tenantRepo "github.com/masterfabric-go/masterfabric/internal/domain/tenant/repository"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/events"
)

type assignRoleRepoStub struct {
	role     *iamModel.Role
	assigned bool
}

func (s *assignRoleRepoStub) Create(context.Context, *iamModel.Role) error { return nil }
func (s *assignRoleRepoStub) GetByID(context.Context, uuid.UUID) (*iamModel.Role, error) {
	return s.role, nil
}
func (s *assignRoleRepoStub) ListByScope(context.Context, iamModel.ScopeType, uuid.UUID) ([]*iamModel.Role, error) {
	return nil, nil
}
func (s *assignRoleRepoStub) Update(context.Context, *iamModel.Role) error { return nil }
func (s *assignRoleRepoStub) Delete(context.Context, uuid.UUID) error      { return nil }
func (s *assignRoleRepoStub) AddPermission(context.Context, uuid.UUID, string) error {
	return nil
}
func (s *assignRoleRepoStub) RemovePermission(context.Context, uuid.UUID, string) error {
	return nil
}
func (s *assignRoleRepoStub) GetPermissions(context.Context, uuid.UUID) ([]string, error) {
	return nil, nil
}
func (s *assignRoleRepoStub) AssignRoleToUser(context.Context, *iamModel.UserRole) error {
	s.assigned = true
	return nil
}
func (s *assignRoleRepoStub) RemoveRoleFromUser(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}
func (s *assignRoleRepoStub) GetUserRoles(context.Context, uuid.UUID, uuid.UUID) ([]*iamModel.UserRole, error) {
	return nil, nil
}
func (s *assignRoleRepoStub) GetUserPermissions(context.Context, uuid.UUID, uuid.UUID) ([]string, error) {
	return nil, nil
}

type assignOrgUserRepoStub struct{ membership *iamModel.OrganizationUser }

func (s *assignOrgUserRepoStub) Add(context.Context, *iamModel.OrganizationUser) error { return nil }
func (s *assignOrgUserRepoStub) Remove(context.Context, uuid.UUID, uuid.UUID) error    { return nil }
func (s *assignOrgUserRepoStub) GetByOrgAndUser(context.Context, uuid.UUID, uuid.UUID) (*iamModel.OrganizationUser, error) {
	return s.membership, nil
}
func (s *assignOrgUserRepoStub) ListByOrg(context.Context, uuid.UUID, int, int) ([]*iamModel.OrganizationUser, int, error) {
	return nil, 0, nil
}
func (s *assignOrgUserRepoStub) ListByUser(context.Context, uuid.UUID) ([]*iamModel.OrganizationUser, error) {
	return nil, nil
}

type assignAppRepoStub struct{ app *tenantModel.App }

func (s *assignAppRepoStub) Create(context.Context, *tenantModel.App) error { return nil }
func (s *assignAppRepoStub) GetByID(context.Context, uuid.UUID) (*tenantModel.App, error) {
	return s.app, nil
}
func (s *assignAppRepoStub) GetBySlug(context.Context, uuid.UUID, string) (*tenantModel.App, error) {
	return nil, nil
}
func (s *assignAppRepoStub) Update(context.Context, *tenantModel.App) error { return nil }
func (s *assignAppRepoStub) Delete(context.Context, uuid.UUID) error        { return nil }
func (s *assignAppRepoStub) ListByOrg(context.Context, uuid.UUID, int, int) ([]*tenantModel.App, int, error) {
	return nil, 0, nil
}

type assignRBACStub struct{}

func (assignRBACStub) HasPermission(context.Context, uuid.UUID, uuid.UUID, string) (bool, error) {
	return true, nil
}
func (assignRBACStub) HasAnyPermission(context.Context, uuid.UUID, uuid.UUID, []string) (bool, error) {
	return true, nil
}
func (assignRBACStub) GetUserPermissions(context.Context, uuid.UUID, uuid.UUID) ([]string, error) {
	return nil, nil
}
func (assignRBACStub) InvalidateCache(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type assignEventBusStub struct{}

func (assignEventBusStub) Publish(context.Context, string, events.Event) error { return nil }
func (assignEventBusStub) Subscribe(string, events.Handler)                    {}
func (assignEventBusStub) Close() error                                        { return nil }

var _ iamRepo.RoleRepository = (*assignRoleRepoStub)(nil)
var _ iamRepo.OrgUserRepository = (*assignOrgUserRepoStub)(nil)
var _ tenantRepo.AppRepository = (*assignAppRepoStub)(nil)
var _ iamService.RBACService = assignRBACStub{}

func TestAssignRoleRejectsOrganizationOutsideAuthenticatedTenant(t *testing.T) {
	actorOrg := uuid.New()
	reqOrg := uuid.New()
	roleRepo := &assignRoleRepoStub{role: &iamModel.Role{ScopeType: iamModel.ScopeTypeOrganization, ScopeID: reqOrg}}
	uc := NewAssignRoleUseCase(roleRepo, &assignOrgUserRepoStub{membership: &iamModel.OrganizationUser{Status: iamModel.OrgUserStatusActive}}, nil, assignRBACStub{}, assignEventBusStub{})
	ctx := authcontext.WithActor(context.Background(), authcontext.ActorContext{UserID: uuid.New(), OrganizationID: actorOrg, TokenClass: authcontext.TokenClassTenant})

	err := uc.Execute(ctx, dto.AssignRoleRequest{UserID: uuid.New(), RoleID: uuid.New(), OrganizationID: reqOrg})

	require.ErrorIs(t, err, domainErr.ErrForbidden)
	require.False(t, roleRepo.assigned)
}
