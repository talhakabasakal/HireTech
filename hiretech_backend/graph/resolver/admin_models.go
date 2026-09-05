package resolver

import (
	"strings"

	"github.com/masterfabric-go/masterfabric/graph/model"
	aiadminModel "github.com/masterfabric-go/masterfabric/internal/domain/aiadmin/model"
)

func adminModelGraphQL(value *aiadminModel.Model) *model.AdminModel {
	roles := make([]model.AdminLLMRole, 0, len(value.Roles))
	for _, role := range value.Roles {
		roles = append(roles, model.AdminLLMRole(strings.ToUpper(string(role))))
	}
	return &model.AdminModel{ID: value.ID, ModelID: value.ModelID, DisplayName: value.DisplayName, ProviderLabel: value.ProviderLabel, Roles: roles, Status: model.AdminModelStatus(strings.ToUpper(string(value.Status))), LatencyClass: model.AdminLatencyClass(strings.ToUpper(string(value.LatencyClass))), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func adminPromptGraphQL(value *aiadminModel.Prompt) *model.AdminPromptConfiguration {
	return &model.AdminPromptConfiguration{ID: value.ID, Role: model.AdminLLMRole(strings.ToUpper(string(value.Role))), Name: value.Name, Version: value.Version, PromptTemplate: value.Prompt, Status: model.AdminPromptStatus(strings.ToUpper(string(value.Status))), UpdatedAt: value.UpdatedAt, UpdatedBy: value.UpdatedBy}
}

func adminRubricGraphQL(value *aiadminModel.Rubric) *model.AdminEvaluationRubric {
	criteria := make([]*model.AdminRubricCriterion, 0, len(value.Criteria))
	for _, criterion := range value.Criteria {
		criteria = append(criteria, &model.AdminRubricCriterion{Name: criterion.Name, Weight: criterion.Weight})
	}
	return &model.AdminEvaluationRubric{ID: value.ID, Name: value.Name, Version: value.Version, Criteria: criteria, Status: model.AdminPromptStatus(strings.ToUpper(value.Status)), UpdatedAt: value.UpdatedAt, UpdatedBy: value.UpdatedBy}
}

func adminRoutingGraphQL(value *aiadminModel.RoutingRule) *model.AdminRoutingRule {
	return &model.AdminRoutingRule{ID: value.ID, Role: model.AdminLLMRole(strings.ToUpper(string(value.Role))), PrimaryModelID: value.PrimaryModelID, FallbackModelID: value.FallbackModelID, TimeoutSeconds: value.TimeoutSeconds, Enabled: value.Enabled, UpdatedAt: value.UpdatedAt, UpdatedBy: value.UpdatedBy}
}

func adminVersionGraphQL(value *aiadminModel.ConfigurationVersion) *model.AdminConfigurationVersion {
	if value == nil {
		return nil
	}
	return &model.AdminConfigurationVersion{ID: value.ID, Resource: value.Resource, Version: value.Version, Action: value.Action, Status: value.Status, Actor: value.Actor, CreatedAt: value.CreatedAt}
}

func adminWorkspaceGraphQL(value *aiadminModel.Workspace) *model.AdminWorkspace {
	result := &model.AdminWorkspace{Models: []*model.AdminModel{}, Prompts: []*model.AdminPromptConfiguration{}, Routing: []*model.AdminRoutingRule{}, Versions: []*model.AdminConfigurationVersion{}, AuditEvents: []*model.AdminAuditEvent{}}
	for _, item := range value.Models {
		result.Models = append(result.Models, adminModelGraphQL(item))
	}
	for _, item := range value.Prompts {
		result.Prompts = append(result.Prompts, adminPromptGraphQL(item))
	}
	for _, item := range value.Routing {
		result.Routing = append(result.Routing, adminRoutingGraphQL(item))
	}
	if value.Rubric != nil {
		result.Rubric = adminRubricGraphQL(value.Rubric)
	}
	for _, item := range value.Versions {
		result.Versions = append(result.Versions, &model.AdminConfigurationVersion{ID: item.ID, Resource: item.Resource, Version: item.Version, Action: item.Action, Status: item.Status, Actor: item.Actor, CreatedAt: item.CreatedAt})
	}
	for _, item := range value.AuditEvents {
		result.AuditEvents = append(result.AuditEvents, &model.AdminAuditEvent{ID: item.ID, Action: item.Action, Actor: item.Actor, Target: item.Target, Result: item.Result, OccurredAt: item.OccurredAt})
	}
	return result
}
