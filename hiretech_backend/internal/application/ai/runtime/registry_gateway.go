package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	aiModel "github.com/masterfabric-go/masterfabric/internal/domain/ai/model"
	aiService "github.com/masterfabric-go/masterfabric/internal/domain/ai/service"
	aiadminModel "github.com/masterfabric-go/masterfabric/internal/domain/aiadmin/model"
	aiadminRepo "github.com/masterfabric-go/masterfabric/internal/domain/aiadmin/repository"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
)

// RegistryGateway resolves tenant-scoped routing and prompt configuration at
// request time. Provider credentials remain in process configuration; the
// registry stores only approved model identifiers and provider labels.
type RegistryGateway struct {
	source       aiadminRepo.Repository
	providers    map[string]aiService.Gateway
	roleFallback map[aiadminModel.Role]aiService.Gateway
}

func NewRegistryGateway(source aiadminRepo.Repository, providers map[string]aiService.Gateway, roleFallback map[aiadminModel.Role]aiService.Gateway) *RegistryGateway {
	normalized := make(map[string]aiService.Gateway, len(providers))
	for label, gateway := range providers {
		if gateway != nil {
			normalized[strings.ToLower(strings.TrimSpace(label))] = gateway
		}
	}
	return &RegistryGateway{source: source, providers: normalized, roleFallback: roleFallback}
}

func (g *RegistryGateway) Complete(ctx context.Context, request aiModel.CompletionRequest) (aiModel.CompletionResponse, error) {
	actor, hasActor := authcontext.FromContext(ctx)
	if !hasActor || actor.OrganizationID == [16]byte{} || g.source == nil {
		return g.staticFallback(ctx, request)
	}
	workspace, err := g.source.GetActiveWorkspace(ctx, actor.OrganizationID)
	if err != nil {
		return aiModel.CompletionResponse{}, fmt.Errorf("load AI runtime configuration: %w", err)
	}
	role := request.Role
	route := findRoute(workspace, role)
	if route == nil || !route.Enabled {
		return g.staticFallback(ctx, request)
	}

	prompt := findPrompt(workspace, role)
	request.Messages = withConfiguredPrompt(prompt, request.Messages)
	primary, err := g.resolveModel(workspace, route.PrimaryModelID, role)
	if err != nil {
		return aiModel.CompletionResponse{}, err
	}
	primaryRequest := request
	primaryRequest.ModelID = primary.ModelID
	primaryGateway, err := g.providerFor(primary, role)
	if err != nil {
		return aiModel.CompletionResponse{}, err
	}
	response, primaryErr := g.completeWithRouteTimeout(ctx, route.TimeoutSeconds, primaryGateway, primaryRequest)
	if primaryErr == nil {
		return response, nil
	}

	fallback, fallbackErr := g.resolveModel(workspace, route.FallbackModelID, role)
	if fallbackErr != nil {
		return aiModel.CompletionResponse{}, fmt.Errorf("primary AI model failed: %v; fallback unavailable: %w", primaryErr, fallbackErr)
	}
	fallbackGateway, fallbackErr := g.providerFor(fallback, role)
	if fallbackErr != nil {
		return aiModel.CompletionResponse{}, fmt.Errorf("primary AI model failed: %v; fallback unavailable: %w", primaryErr, fallbackErr)
	}
	fallbackRequest := request
	fallbackRequest.ModelID = fallback.ModelID
	response, fallbackErr = g.completeWithRouteTimeout(ctx, route.TimeoutSeconds, fallbackGateway, fallbackRequest)
	if fallbackErr != nil {
		return aiModel.CompletionResponse{}, fmt.Errorf("primary AI model failed: %v; fallback AI model failed: %w", primaryErr, fallbackErr)
	}
	return response, nil
}

func (g *RegistryGateway) completeWithRouteTimeout(ctx context.Context, seconds int, gateway aiService.Gateway, request aiModel.CompletionRequest) (aiModel.CompletionResponse, error) {
	if seconds <= 0 {
		return gateway.Complete(ctx, request)
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()
	return gateway.Complete(requestCtx, request)
}

func (g *RegistryGateway) staticFallback(ctx context.Context, request aiModel.CompletionRequest) (aiModel.CompletionResponse, error) {
	if gateway := g.roleFallback[aiadminModel.Role(request.Role)]; gateway != nil {
		return gateway.Complete(ctx, request)
	}
	return aiModel.CompletionResponse{}, fmt.Errorf("AI routing is not configured for role %s", request.Role)
}

func (g *RegistryGateway) resolveModel(workspace *aiadminModel.Workspace, modelID string, role aiModel.Role) (*aiadminModel.Model, error) {
	for _, value := range workspace.Models {
		if value != nil && value.ModelID == modelID {
			if value.Status == aiadminModel.ModelDisabled || !hasRole(value.Roles, aiadminModel.Role(role)) {
				return nil, fmt.Errorf("AI model %s is disabled or not approved for role %s", modelID, role)
			}
			return value, nil
		}
	}
	return nil, fmt.Errorf("AI model %s is not registered", modelID)
}

func (g *RegistryGateway) providerFor(model *aiadminModel.Model, role aiModel.Role) (aiService.Gateway, error) {
	if gateway := g.providers[strings.ToLower(strings.TrimSpace(model.ProviderLabel))]; gateway != nil {
		return gateway, nil
	}
	// A tenant-approved model must execute only through its registered
	// provider. Falling back here could send the approved model identifier to
	// an unrelated provider, so static role fallbacks are intentionally limited
	// to the no-tenant/no-route path in staticFallback.
	return nil, fmt.Errorf("AI provider %s is not configured for role %s", model.ProviderLabel, role)
}

func findRoute(workspace *aiadminModel.Workspace, role aiModel.Role) *aiadminModel.RoutingRule {
	for _, route := range workspace.Routing {
		if route != nil && route.Role == aiadminModel.Role(role) {
			return route
		}
	}
	return nil
}

func findPrompt(workspace *aiadminModel.Workspace, role aiModel.Role) string {
	for _, prompt := range workspace.Prompts {
		if prompt != nil && prompt.Role == aiadminModel.Role(role) && prompt.Status == aiadminModel.PromptActive {
			return prompt.Prompt
		}
	}
	return ""
}

func withConfiguredPrompt(prompt string, messages []aiModel.Message) []aiModel.Message {
	if strings.TrimSpace(prompt) == "" {
		return messages
	}
	result := make([]aiModel.Message, 0, len(messages)+1)
	result = append(result, aiModel.Message{Role: "system", Content: prompt})
	return append(result, messages...)
}

func hasRole(roles []aiadminModel.Role, wanted aiadminModel.Role) bool {
	for _, role := range roles {
		if role == wanted {
			return true
		}
	}
	return false
}
