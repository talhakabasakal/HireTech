package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	aiadminModel "github.com/masterfabric-go/masterfabric/internal/domain/aiadmin/model"
	aiadminRepo "github.com/masterfabric-go/masterfabric/internal/domain/aiadmin/repository"
	auditRepo "github.com/masterfabric-go/masterfabric/internal/domain/audit/repository"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/permissions"
)

const permission = "ai_config:manage"

type Service struct {
	repo       aiadminRepo.Repository
	audit      auditRepo.AuditRepository
	recentAuth time.Duration
}

func NewService(repo aiadminRepo.Repository, audit auditRepo.AuditRepository, security ...config.SecurityConfig) *Service {
	recent := 10 * time.Minute
	if len(security) > 0 && security[0].RecentAuthenticationMinutes > 0 {
		recent = time.Duration(security[0].RecentAuthenticationMinutes) * time.Minute
	}
	return &Service{repo: repo, audit: audit, recentAuth: recent}
}

func (s *Service) Workspace(ctx context.Context, actor authcontext.ActorContext) (*aiadminModel.Workspace, error) {
	if err := require(actor, "ai_config:read"); err != nil {
		return nil, err
	}
	// The aggregate includes the audit projection used by the audit screen.
	// Reading configuration must not implicitly grant access to security history.
	if err := require(actor, "audit:read"); err != nil {
		return nil, err
	}
	if err := requireMFA(actor, s.recentAuth); err != nil {
		return nil, err
	}
	if s.audit == nil {
		return nil, domainErr.New(domainErr.ErrNotImplemented, "audit administration is not configured", nil)
	}
	workspace, err := s.repo.GetWorkspace(ctx, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	logs, _, auditErr := s.audit.ListByOrg(ctx, actor.OrganizationID, 0, 100)
	if auditErr != nil {
		return nil, auditErr
	}
	for _, log := range logs {
		actorID := uuid.Nil
		if log.UserID != nil {
			actorID = *log.UserID
		}
		result := "success"
		var metadata struct {
			Outcome string `json:"outcome"`
		}
		if json.Unmarshal(log.Metadata, &metadata) == nil && !strings.EqualFold(metadata.Outcome, "SUCCESS") && metadata.Outcome != "" {
			result = "denied"
		}
		workspace.AuditEvents = append(workspace.AuditEvents, &aiadminModel.AuditEvent{ID: log.ID, Action: log.Action, Actor: actorID, Target: log.ResourceType + ":" + log.ResourceID, Result: result, OccurredAt: log.CreatedAt})
	}
	return workspace, nil
}

func (s *Service) RegisterModel(ctx context.Context, actor authcontext.ActorContext, input aiadminModel.RegisterModelInput) (*aiadminModel.Model, error) {
	if err := require(actor, permission); err != nil {
		return nil, err
	}
	if err := requireMFA(actor, s.recentAuth); err != nil {
		return nil, err
	}
	input.ModelID, input.DisplayName, input.ProviderLabel = strings.TrimSpace(input.ModelID), strings.TrimSpace(input.DisplayName), strings.TrimSpace(input.ProviderLabel)
	if input.ModelID == "" || input.DisplayName == "" || input.ProviderLabel == "" || len(input.Roles) == 0 {
		return nil, domainErr.New(domainErr.ErrValidation, "model identity, provider, and at least one role are required", nil)
	}
	if input.Status == "" {
		input.Status = aiadminModel.ModelActive
	}
	if input.LatencyClass == "" {
		input.LatencyClass = aiadminModel.LatencyBalanced
	}
	if !validRoles(input.Roles) {
		return nil, domainErr.New(domainErr.ErrValidation, "invalid model role", nil)
	}
	if !validModelStatus(input.Status) || !validLatencyClass(input.LatencyClass) {
		return nil, domainErr.New(domainErr.ErrValidation, "invalid model status or latency class", nil)
	}
	return s.repo.RegisterModel(ctx, actor.OrganizationID, actor.UserID, input)
}

func (s *Service) CreatePromptVersion(ctx context.Context, actor authcontext.ActorContext, input aiadminModel.CreatePromptInput) (*aiadminModel.Prompt, error) {
	if err := require(actor, permission); err != nil {
		return nil, err
	}
	if err := requireMFA(actor, s.recentAuth); err != nil {
		return nil, err
	}
	input.Name, input.Prompt = strings.TrimSpace(input.Name), strings.TrimSpace(input.Prompt)
	if !validRole(input.Role) || input.Name == "" || len(input.Prompt) < 20 || len(input.Prompt) > 20000 {
		return nil, domainErr.New(domainErr.ErrValidation, "prompt role, name, and a prompt between 20 and 20000 characters are required", nil)
	}
	return s.repo.CreatePromptVersion(ctx, actor.OrganizationID, actor.UserID, input)
}

func (s *Service) UpdateRouting(ctx context.Context, actor authcontext.ActorContext, input aiadminModel.UpdateRoutingInput) (*aiadminModel.RoutingRule, error) {
	if err := require(actor, permission); err != nil {
		return nil, err
	}
	if err := requireMFA(actor, s.recentAuth); err != nil {
		return nil, err
	}
	input.PrimaryModelID, input.FallbackModelID = strings.TrimSpace(input.PrimaryModelID), strings.TrimSpace(input.FallbackModelID)
	if !validRole(input.Role) || input.PrimaryModelID == "" || input.FallbackModelID == "" || input.TimeoutSeconds < 1 || input.TimeoutSeconds > 300 {
		return nil, domainErr.New(domainErr.ErrValidation, "invalid routing configuration", nil)
	}
	if input.PrimaryModelID == input.FallbackModelID {
		return nil, domainErr.New(domainErr.ErrValidation, "primary and fallback models must differ", nil)
	}
	for _, modelID := range []string{input.PrimaryModelID, input.FallbackModelID} {
		registered, err := s.repo.GetModel(ctx, actor.OrganizationID, modelID)
		if err != nil {
			return nil, err
		}
		if registered == nil || registered.Status == aiadminModel.ModelDisabled || !containsRole(registered.Roles, input.Role) {
			return nil, domainErr.New(domainErr.ErrValidation, "routing model is not registered for the selected role", nil)
		}
	}
	return s.repo.UpdateRouting(ctx, actor.OrganizationID, actor.UserID, input)
}

func (s *Service) PublishRubric(ctx context.Context, actor authcontext.ActorContext, input aiadminModel.PublishRubricInput) (*aiadminModel.Rubric, error) {
	if err := require(actor, permission); err != nil {
		return nil, err
	}
	if err := requireMFA(actor, s.recentAuth); err != nil {
		return nil, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Criteria) == 0 || len(input.Criteria) > 32 {
		return nil, domainErr.New(domainErr.ErrValidation, "rubric name and criteria are required", nil)
	}
	var total float64
	for i := range input.Criteria {
		input.Criteria[i].Name = strings.TrimSpace(input.Criteria[i].Name)
		if input.Criteria[i].Name == "" || input.Criteria[i].Weight <= 0 {
			return nil, domainErr.New(domainErr.ErrValidation, "rubric criteria must have positive names and weights", nil)
		}
		total += input.Criteria[i].Weight
	}
	if total < 0.999 || total > 1.001 {
		return nil, domainErr.New(domainErr.ErrValidation, "rubric weights must sum to 1", nil)
	}
	return s.repo.PublishRubric(ctx, actor.OrganizationID, actor.UserID, input)
}

func (s *Service) ApproveConfiguration(ctx context.Context, actor authcontext.ActorContext, resource, key string, version int) (*aiadminModel.ConfigurationVersion, error) {
	if err := require(actor, permission); err != nil {
		return nil, err
	}
	if err := requireMFA(actor, s.recentAuth); err != nil {
		return nil, err
	}
	resource, key = strings.ToLower(strings.TrimSpace(resource)), strings.TrimSpace(key)
	if !validConfigurationResource(resource) || key == "" || version < 1 {
		return nil, domainErr.New(domainErr.ErrValidation, "invalid AI configuration reference", nil)
	}
	return s.repo.Approve(ctx, actor.OrganizationID, actor.UserID, resource, key, version)
}

func (s *Service) RollbackConfiguration(ctx context.Context, actor authcontext.ActorContext, resource, key string, version int) (*aiadminModel.ConfigurationVersion, error) {
	if err := require(actor, permission); err != nil {
		return nil, err
	}
	if err := requireMFA(actor, s.recentAuth); err != nil {
		return nil, err
	}
	resource, key = strings.ToLower(strings.TrimSpace(resource)), strings.TrimSpace(key)
	if !validConfigurationResource(resource) || key == "" || version < 1 {
		return nil, domainErr.New(domainErr.ErrValidation, "invalid AI configuration reference", nil)
	}
	return s.repo.Rollback(ctx, actor.OrganizationID, actor.UserID, resource, key, version)
}

func require(actor authcontext.ActorContext, needed string) error {
	if actor.TokenClass != authcontext.TokenClassTenant || actor.OrganizationID == uuid.Nil {
		return domainErr.New(domainErr.ErrForbidden, "tenant access required", nil)
	}
	for _, granted := range actor.Permissions {
		if permissions.Matches(granted, needed) || permissions.Matches(granted, permission) {
			return nil
		}
	}
	return domainErr.New(domainErr.ErrForbidden, "permission denied", nil)
}

func requireMFA(actor authcontext.ActorContext, recent time.Duration) error {
	if !(actor.HasAuthenticationMethod("otp") || actor.HasAuthenticationMethod("totp") || actor.HasAuthenticationMethod("webauthn")) {
		return domainErr.New(domainErr.ErrRecentAuthenticationRequired, "MFA is required for AI administration", nil)
	}
	if actor.AuthenticationTime.IsZero() || recent <= 0 || time.Now().UTC().Sub(actor.AuthenticationTime) > recent || actor.AuthenticationTime.After(time.Now().UTC().Add(time.Minute)) {
		return domainErr.New(domainErr.ErrRecentAuthenticationRequired, "recent MFA authentication is required", nil)
	}
	return nil
}
func validRole(role aiadminModel.Role) bool {
	return role == aiadminModel.RoleInterviewer || role == aiadminModel.RoleEvaluator
}
func validRoles(roles []aiadminModel.Role) bool {
	for _, role := range roles {
		if !validRole(role) {
			return false
		}
	}
	return true
}

func validModelStatus(status aiadminModel.ModelStatus) bool {
	return status == aiadminModel.ModelActive || status == aiadminModel.ModelFallback || status == aiadminModel.ModelDisabled
}

func validLatencyClass(value aiadminModel.LatencyClass) bool {
	return value == aiadminModel.LatencyFast || value == aiadminModel.LatencyBalanced || value == aiadminModel.LatencyDeep
}

func containsRole(roles []aiadminModel.Role, wanted aiadminModel.Role) bool {
	for _, role := range roles {
		if role == wanted {
			return true
		}
	}
	return false
}

func validConfigurationResource(resource string) bool {
	return resource == "model" || resource == "prompt" || resource == "routing" || resource == "rubric"
}
