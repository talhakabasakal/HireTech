package graphql

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/99designs/gqlgen/graphql"

	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
)

type operationLimits struct{}

type persistedOperationGate struct {
	allowedHashes map[string]struct{}
}

func (persistedOperationGate) ExtensionName() string { return "PersistedOperationGate" }

func (persistedOperationGate) Validate(graphql.ExecutableSchema) error { return nil }

// MutateOperationParameters runs for every transport, including WebSocket
// subscriptions. The HTTP wrapper performs the same check before handing the
// request to gqlgen; keeping this transport-level gate prevents that check
// from being bypassed by connection_init/start messages.
func (g persistedOperationGate) MutateOperationParameters(_ context.Context, params *graphql.RawParams) *gqlerror.Error {
	if err := validatePersistedOperationParams(params.Query, params.Extensions, g.allowedHashes); err != nil {
		return &gqlerror.Error{Message: err.Error(), Extensions: map[string]any{"code": "VALIDATION_FAILED"}}
	}
	return nil
}

func validatePersistedOperationParams(query string, extensions map[string]any, allowedHashes map[string]struct{}) error {
	if strings.TrimSpace(query) == "" {
		return errors.New("a persisted GraphQL operation is required")
	}
	persisted, ok := extensions["persistedQuery"].(map[string]any)
	if !ok {
		return errors.New("a valid persisted GraphQL operation hash is required")
	}
	versionOK := false
	switch version := persisted["version"].(type) {
	case float64:
		versionOK = int(version) == 1
	case int:
		versionOK = version == 1
	case int64:
		versionOK = version == 1
	case json.Number:
		versionOK = version == "1"
	}
	if !versionOK {
		return errors.New("a valid persisted GraphQL operation hash is required")
	}
	hash, ok := persisted["sha256Hash"].(string)
	hash = strings.ToLower(strings.TrimSpace(hash))
	if !ok || len(hash) != sha256.Size*2 {
		return errors.New("a valid persisted GraphQL operation hash is required")
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return errors.New("a valid persisted GraphQL operation hash is required")
	}
	digest := sha256.Sum256([]byte(query))
	if hash != hex.EncodeToString(digest[:]) {
		return errors.New("persisted GraphQL operation hash does not match the query")
	}
	if _, ok := allowedHashes[hash]; !ok {
		return errors.New("persisted GraphQL operation is not allowlisted")
	}
	return nil
}

func (operationLimits) ExtensionName() string { return "RequestLimits" }

func (operationLimits) Validate(graphql.ExecutableSchema) error { return nil }

func (operationLimits) MutateOperationContext(ctx context.Context, opCtx *graphql.OperationContext) *gqlerror.Error {
	if len(opCtx.Doc.Operations) > MaxOperations {
		return limitError(ctx, "only one GraphQL operation is allowed")
	}
	opCtx.DisableIntrospection = true
	if opCtx.Operation == nil {
		return limitError(ctx, "operation could not be selected")
	}

	stats := documentStats(opCtx.Operation.SelectionSet, opCtx.Doc.Fragments, nil, 1)
	if stats.Depth > MaxDepth {
		return limitError(ctx, "query depth exceeds the configured limit")
	}
	if stats.Nodes > MaxNodes {
		return limitError(ctx, "query node count exceeds the configured limit")
	}
	if stats.Aliases > MaxAliases {
		return limitError(ctx, "query aliases are not allowed")
	}
	if audit := middleware.RequestAuditFromContext(ctx); audit != nil {
		audit.SetOperation(operationName(opCtx), string(opCtx.Operation.Operation), stats.Nodes+stats.Depth)
	}
	return nil
}

type documentStat struct{ Depth, Nodes, Aliases int }

func documentStats(selection ast.SelectionSet, fragments ast.FragmentDefinitionList, visited map[string]bool, depth int) documentStat {
	if visited == nil {
		visited = map[string]bool{}
	}
	result := documentStat{Depth: depth}
	for _, selection := range selection {
		switch item := selection.(type) {
		case *ast.Field:
			result.Nodes++
			if item.Alias != "" && item.Alias != item.Name {
				result.Aliases++
			}
			child := documentStats(item.SelectionSet, fragments, visited, depth+1)
			result = mergeStats(result, child)
		case *ast.FragmentSpread:
			if visited[item.Name] {
				continue
			}
			visited[item.Name] = true
			if fragment := fragments.ForName(item.Name); fragment != nil {
				result = mergeStats(result, documentStats(fragment.SelectionSet, fragments, visited, depth))
			}
		case *ast.InlineFragment:
			result = mergeStats(result, documentStats(item.SelectionSet, fragments, visited, depth))
		}
	}
	return result
}

func mergeStats(left, right documentStat) documentStat {
	if right.Depth > left.Depth {
		left.Depth = right.Depth
	}
	left.Nodes += right.Nodes
	left.Aliases += right.Aliases
	return left
}

func operationName(opCtx *graphql.OperationContext) string {
	if opCtx.Operation.Name != "" {
		return opCtx.Operation.Name
	}
	return "anonymous"
}

func limitError(ctx context.Context, message string) *gqlerror.Error {
	if audit := middleware.RequestAuditFromContext(ctx); audit != nil {
		audit.SetResult("DENIED", "VALIDATION_FAILED", 0)
	}
	return &gqlerror.Error{Message: message, Extensions: map[string]any{"code": "VALIDATION_FAILED"}}
}

type authorizationMiddleware struct{}

func (authorizationMiddleware) ExtensionName() string { return "GraphQLAuthorization" }

func (authorizationMiddleware) Validate(graphql.ExecutableSchema) error { return nil }

func (authorizationMiddleware) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	opCtx := graphql.GetOperationContext(ctx)
	actor, authenticated := actorFromContext(ctx)
	allowed := authenticated
	if allowed {
		fields := rootFieldNames(opCtx.Operation.SelectionSet, opCtx.Doc.Fragments, nil)
		allowed = len(fields) > 0
		for _, field := range fields {
			if !rootFieldAllowed(actor, field) {
				allowed = false
				break
			}
		}
	}
	if !allowed {
		if audit := middleware.RequestAuditFromContext(ctx); audit != nil {
			audit.SetResult("DENIED", "FORBIDDEN", 0)
		}
		return func(context.Context) *graphql.Response {
			return &graphql.Response{Errors: gqlerror.List{safeErrorPresenter(ctx, domainErr.New(domainErr.ErrForbidden, "forbidden", nil))}}
		}
	}
	return next(ctx)
}

func operationAuditMiddleware(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	started := graphql.Now()
	responseHandler := next(ctx)
	return func(ctx context.Context) *graphql.Response {
		response := responseHandler(ctx)
		outcome, code := "SUCCESS", ""
		if len(response.Errors) > 0 {
			outcome, code = "FAILURE", "INTERNAL"
			if response.Errors[0].Extensions != nil {
				if c, ok := response.Errors[0].Extensions["code"].(string); ok {
					code = c
				}
			}
		}
		if audit := middleware.RequestAuditFromContext(ctx); audit != nil {
			audit.SetResult(outcome, code, graphql.Now().Sub(started))
		}
		return response
	}
}

func actorFromContext(ctx context.Context) (authcontext.ActorContext, bool) {
	return authcontext.FromContext(ctx)
}

func rootFieldNames(selection ast.SelectionSet, fragments ast.FragmentDefinitionList, visited map[string]bool) []string {
	if visited == nil {
		visited = map[string]bool{}
	}
	result := make([]string, 0, len(selection))
	for _, item := range selection {
		switch value := item.(type) {
		case *ast.Field:
			result = append(result, value.Name)
		case *ast.FragmentSpread:
			if visited[value.Name] {
				continue
			}
			visited[value.Name] = true
			if fragment := fragments.ForName(value.Name); fragment != nil {
				result = append(result, rootFieldNames(fragment.SelectionSet, fragments, visited)...)
			}
		case *ast.InlineFragment:
			result = append(result, rootFieldNames(value.SelectionSet, fragments, visited)...)
		}
	}
	return result
}

func rootFieldAllowed(actor authcontext.ActorContext, field string) bool {
	switch field {
	case "me":
		return true
	case "organizations":
		return actor.TokenClass == "" || actor.IsBootstrap() || actor.TokenClass == authcontext.TokenClassTenant
	case "selectOrganization", "redeemInterviewInvitation":
		return actor.IsBootstrap()
	case "interviews", "interviewConnection", "createInterview", "addQuestion", "publishInterview", "createInterviewInvitation", "cancelInterview":
		return actor.TokenClass == authcontext.TokenClassTenant
	case "interview", "question", "answer":
		return actor.TokenClass == authcontext.TokenClassTenant || actor.TokenClass == authcontext.TokenClassCandidateInterview
	case "evaluationReport", "requestEvaluation", "recordHumanReview", "questionDraft", "requestQuestionDraft", "approveQuestionDraft", "rejectQuestionDraft":
		return actor.TokenClass == authcontext.TokenClassTenant
	case "adminWorkspace", "adminAuditEvents", "registerAdminModel", "createAdminPromptVersion", "updateAdminRouting", "publishAdminRubric":
		return actor.TokenClass == authcontext.TokenClassTenant
	case "startInterview", "submitAnswer", "completeInterview":
		return actor.TokenClass == authcontext.TokenClassCandidateInterview
	case "interviewUpdated":
		return actor.TokenClass == authcontext.TokenClassTenant || actor.TokenClass == authcontext.TokenClassCandidateInterview
	default:
		return false
	}
}
