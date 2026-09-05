package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/iam/dto"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	iamRepo "github.com/masterfabric-go/masterfabric/internal/domain/iam/repository"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	orgModel "github.com/masterfabric-go/masterfabric/internal/domain/tenant/model"
	tenantRepo "github.com/masterfabric-go/masterfabric/internal/domain/tenant/repository"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

// OrganizationMembership is the application result used by the GraphQL transport.
type OrganizationMembership struct {
	Organization *orgModel.Organization
	Membership   *model.OrganizationUser
}

// OrganizationContextUseCase provides the trusted identity and organization switch path.
type OrganizationContextUseCase struct {
	userRepo       iamRepo.UserRepository
	orgRepo        tenantRepo.OrgRepository
	membershipRepo iamRepo.OrgUserRepository
	auth           service.AuthService
	rbac           service.RBACService
}

func NewOrganizationContextUseCase(
	userRepo iamRepo.UserRepository,
	orgRepo tenantRepo.OrgRepository,
	membershipRepo iamRepo.OrgUserRepository,
	auth service.AuthService,
	rbac service.RBACService,
) *OrganizationContextUseCase {
	return &OrganizationContextUseCase{
		userRepo:       userRepo,
		orgRepo:        orgRepo,
		membershipRepo: membershipRepo,
		auth:           auth,
		rbac:           rbac,
	}
}

func (uc *OrganizationContextUseCase) GetMe(ctx context.Context, actor authcontext.ActorContext) (*dto.UserInfo, error) {
	if !actor.IsAuthenticated() {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "authentication required", nil)
	}

	user, err := uc.userRepo.GetByID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}

	return &dto.UserInfo{
		ID: user.ID, Email: user.Email, FirstName: user.FirstName, LastName: user.LastName,
		Status: string(user.Status), CreatedAt: user.CreatedAt,
	}, nil
}

func (uc *OrganizationContextUseCase) ListOrganizations(ctx context.Context, actor authcontext.ActorContext) ([]*OrganizationMembership, error) {
	if !actor.IsAuthenticated() {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "authentication required", nil)
	}

	memberships, err := uc.membershipRepo.ListByUser(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}

	result := make([]*OrganizationMembership, 0, len(memberships))
	for _, membership := range memberships {
		if membership == nil || membership.Status != model.OrgUserStatusActive {
			continue
		}
		org, err := uc.orgRepo.GetByID(ctx, membership.OrganizationID)
		if err != nil {
			if errors.Is(err, domainErr.ErrNotFound) {
				continue
			}
			return nil, err
		}
		if org == nil || !org.IsActive() {
			continue
		}
		result = append(result, &OrganizationMembership{Organization: org, Membership: membership})
	}
	return result, nil
}

func (uc *OrganizationContextUseCase) SelectOrganization(ctx context.Context, actor authcontext.ActorContext, organizationID uuid.UUID) (string, *OrganizationMembership, error) {
	if !actor.IsAuthenticated() {
		return "", nil, domainErr.New(domainErr.ErrUnauthorized, "authentication required", nil)
	}
	if !actor.IsBootstrap() {
		return "", nil, domainErr.New(domainErr.ErrForbidden, "organization selection requires a bootstrap token", nil)
	}
	if organizationID == uuid.Nil {
		return "", nil, domainErr.New(domainErr.ErrValidation, "organization id is required", nil)
	}

	org, err := uc.orgRepo.GetByID(ctx, organizationID)
	if err != nil {
		return "", nil, domainErr.New(domainErr.ErrForbidden, "organization access denied", nil)
	}
	if org == nil || !org.IsActive() {
		return "", nil, domainErr.New(domainErr.ErrForbidden, "organization access denied", nil)
	}

	membership, err := uc.membershipRepo.GetByOrgAndUser(ctx, organizationID, actor.UserID)
	if err != nil || membership == nil || membership.Status != model.OrgUserStatusActive {
		return "", nil, domainErr.New(domainErr.ErrForbidden, "organization access denied", nil)
	}

	permissions := actor.Permissions
	if uc.rbac != nil {
		permissions, err = uc.rbac.GetUserPermissions(ctx, actor.UserID, organizationID)
		if err != nil {
			return "", nil, domainErr.New(domainErr.ErrInternal, "failed to resolve organization permissions", err)
		}
	}

	token, err := uc.auth.GenerateToken(ctx, service.TokenClaims{
		UserID: actor.UserID, Email: actor.Email, OrganizationID: organizationID,
		TokenClass: string(authcontext.TokenClassTenant),
		SessionID:  actor.SessionID, DeviceID: actor.DeviceID,
		Permissions: permissions, AuthenticationMethods: actor.AuthenticationMethods, AuthenticationTime: actor.AuthenticationTime,
	})
	if err != nil {
		return "", nil, domainErr.New(domainErr.ErrInternal, "failed to issue organization token", err)
	}

	return token, &OrganizationMembership{Organization: org, Membership: membership}, nil
}
