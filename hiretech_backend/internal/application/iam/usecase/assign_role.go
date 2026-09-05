package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/iam/dto"
	iamEvent "github.com/masterfabric-go/masterfabric/internal/domain/iam/event"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/repository"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	tenantRepo "github.com/masterfabric-go/masterfabric/internal/domain/tenant/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/events"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
)

// AssignRoleUseCase handles role assignment.
type AssignRoleUseCase struct {
	roleRepo repository.RoleRepository
	orgUsers repository.OrgUserRepository
	apps     tenantRepo.AppRepository
	rbac     service.RBACService
	eventBus events.EventBus
}

// NewAssignRoleUseCase creates a new AssignRoleUseCase.
func NewAssignRoleUseCase(roleRepo repository.RoleRepository, orgUsers repository.OrgUserRepository, apps tenantRepo.AppRepository, rbac service.RBACService, eventBus events.EventBus) *AssignRoleUseCase {
	return &AssignRoleUseCase{roleRepo: roleRepo, orgUsers: orgUsers, apps: apps, rbac: rbac, eventBus: eventBus}
}

// Execute assigns a role to a user.
func (uc *AssignRoleUseCase) Execute(ctx context.Context, req dto.AssignRoleRequest) error {
	actor, ok := middleware.ActorFromContext(ctx)
	if !ok || actor.TokenClass != "tenant" || actor.OrganizationID == uuid.Nil || actor.OrganizationID != req.OrganizationID {
		return domainErr.New(domainErr.ErrForbidden, "role assignment must stay within the authenticated organization", nil)
	}
	if uc.roleRepo == nil || uc.orgUsers == nil || uc.rbac == nil || uc.eventBus == nil {
		return domainErr.New(domainErr.ErrNotImplemented, "role assignment authorization is not configured", nil)
	}
	role, err := uc.roleRepo.GetByID(ctx, req.RoleID)
	if err != nil || role == nil {
		return domainErr.New(domainErr.ErrForbidden, "role is not valid for this organization", nil)
	}
	if req.AppID == nil {
		if role.ScopeType != model.ScopeTypeOrganization || role.ScopeID != req.OrganizationID {
			return domainErr.New(domainErr.ErrForbidden, "role is not valid for this organization", nil)
		}
	} else {
		if role.ScopeType != model.ScopeTypeApp || role.ScopeID != *req.AppID || uc.apps == nil {
			return domainErr.New(domainErr.ErrForbidden, "role is not valid for this application", nil)
		}
		app, appErr := uc.apps.GetByID(ctx, *req.AppID)
		if appErr != nil || app == nil || app.OrganizationID != req.OrganizationID {
			return domainErr.New(domainErr.ErrForbidden, "application is not in the authenticated organization", nil)
		}
	}
	membership, err := uc.orgUsers.GetByOrgAndUser(ctx, req.OrganizationID, req.UserID)
	if err != nil || membership == nil || membership.Status != model.OrgUserStatusActive {
		return domainErr.New(domainErr.ErrForbidden, "user is not an active organization member", nil)
	}
	userRole := &model.UserRole{
		UserID:         req.UserID,
		RoleID:         req.RoleID,
		OrganizationID: req.OrganizationID,
		AppID:          req.AppID,
	}

	if err := uc.roleRepo.AssignRoleToUser(ctx, userRole); err != nil {
		return err
	}

	// Invalidate permission cache
	_ = uc.rbac.InvalidateCache(ctx, req.UserID, req.OrganizationID)

	// Publish domain event to Kafka
	_ = uc.eventBus.Publish(ctx, events.TopicIAM, iamEvent.RoleAssigned{
		UserID:         req.UserID,
		RoleID:         req.RoleID,
		OrganizationID: req.OrganizationID,
		Timestamp:      time.Now().UTC(),
	})

	return nil
}
