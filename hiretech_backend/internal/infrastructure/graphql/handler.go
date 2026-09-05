package graphql

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/google/uuid"
	gorillaws "github.com/gorilla/websocket"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/masterfabric-go/masterfabric/graph/generated"
	"github.com/masterfabric-go/masterfabric/graph/resolver"
	aiadminUsecase "github.com/masterfabric-go/masterfabric/internal/application/aiadmin/usecase"
	evaluationUsecase "github.com/masterfabric-go/masterfabric/internal/application/evaluation/usecase"
	iamUsecase "github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	interviewUsecase "github.com/masterfabric-go/masterfabric/internal/application/interview/usecase"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	infraWS "github.com/masterfabric-go/masterfabric/internal/infrastructure/websocket"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/logger"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
)

const (
	DefaultTimeout  = 5 * time.Second
	MaxParserTokens = 4096
	MaxComplexity   = 50
	MaxDepth        = 8
	MaxNodes        = 64
	MaxAliases      = 0
	MaxOperations   = 1
)

// Handler is the GraphQL transport with request-scoped limits. HTTP queries
// remain POST-only; subscriptions use the separately routed WebSocket method.
type Handler struct {
	server                     *handler.Server
	auth                       service.AuthService
	sessionValidator           service.SessionValidator
	subscriptionBroker         *infraWS.SubscriptionBroker
	websocketEnabled           bool
	timeout                    time.Duration
	maxBodyBytes               int64
	requirePersistedOperations bool
	allowedOperationHashes     map[string]struct{}
}

type Dependencies struct {
	AuthService                service.AuthService
	SessionValidator           service.SessionValidator
	UseCase                    *iamUsecase.OrganizationContextUseCase
	InterviewUseCase           *interviewUsecase.InterviewService
	EvaluationUseCase          *evaluationUsecase.EvaluationService
	QuestionDraftUseCase       *interviewUsecase.QuestionDraftService
	AIAdminUseCase             *aiadminUsecase.Service
	Timeout                    time.Duration
	MaxBodyBytes               int64
	RequirePersistedOperations bool
	AllowedOperationHashes     []string
	SubscriptionBroker         *infraWS.SubscriptionBroker
	WebSocketEnabled           bool
	WebSocketAllowedOrigins    []string
}

func NewHandler(deps Dependencies) *Handler {
	h := &Handler{
		auth: deps.AuthService, sessionValidator: deps.SessionValidator,
		subscriptionBroker: deps.SubscriptionBroker, websocketEnabled: deps.WebSocketEnabled,
	}
	server := handler.New(generated.NewExecutableSchema(generated.Config{
		Resolvers: &resolver.Resolver{UseCase: deps.UseCase, InterviewUseCase: deps.InterviewUseCase, EvaluationUseCase: deps.EvaluationUseCase, QuestionDraftUseCase: deps.QuestionDraftUseCase, AIAdminUseCase: deps.AIAdminUseCase, SubscriptionBroker: deps.SubscriptionBroker},
	}))
	// POST is the only HTTP application transport. WebSocket is added only when
	// the explicitly configured subscription capability is enabled.
	if deps.SubscriptionBroker != nil && deps.WebSocketEnabled {
		server.AddTransport(transport.Websocket{
			Upgrader: gorillaws.Upgrader{
				ReadBufferSize:  1024,
				WriteBufferSize: 1024,
				CheckOrigin:     websocketOriginAllowed(deps.WebSocketAllowedOrigins),
			},
			InitTimeout:           5 * time.Second,
			KeepAlivePingInterval: 30 * time.Second,
			InitFunc:              h.websocketInit,
		})
	}
	server.AddTransport(transport.POST{})
	server.SetParserTokenLimit(MaxParserTokens)
	server.Use(extension.FixedComplexityLimit(MaxComplexity))
	server.Use(operationLimits{})
	server.Use(authorizationMiddleware{})
	server.AroundOperations(operationAuditMiddleware)
	server.SetErrorPresenter(safeErrorPresenter)
	if deps.MaxBodyBytes <= 0 {
		deps.MaxBodyBytes = 1 << 20
	}
	server.SetRecoverFunc(func(context.Context, any) error {
		return domainErr.New(domainErr.ErrInternal, "internal server error", nil)
	})

	timeout := deps.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	allowedHashes := make(map[string]struct{}, len(deps.AllowedOperationHashes))
	for _, hash := range deps.AllowedOperationHashes {
		hash = strings.ToLower(strings.TrimSpace(hash))
		if hash != "" {
			allowedHashes[hash] = struct{}{}
		}
	}
	if deps.RequirePersistedOperations {
		server.Use(persistedOperationGate{allowedHashes: allowedHashes})
	}
	h.server = server
	h.timeout = timeout
	h.maxBodyBytes = deps.MaxBodyBytes
	h.requirePersistedOperations = deps.RequirePersistedOperations
	h.allowedOperationHashes = allowedHashes
	return h
}

// WebsocketHandler is intentionally separate from ServeHTTP so the POST body
// validator cannot be bypassed or accidentally invoked for a WebSocket
// handshake. Authentication is performed from the connection_init payload.
func (h *Handler) WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	if h == nil || !h.websocketEnabled || h.subscriptionBroker == nil {
		writeError(w, http.StatusServiceUnavailable, "INTERNAL", "GraphQL subscriptions are disabled")
		return
	}
	for _, name := range []string{"X-Organization-ID", "X-Workspace-ID", "X-App-ID"} {
		if r.Header.Get(name) != "" {
			writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "tenant headers are not accepted by GraphQL")
			return
		}
	}
	h.server.ServeHTTP(w, r)
}

func (h *Handler) websocketInit(ctx context.Context, payload transport.InitPayload) (context.Context, *transport.InitPayload, error) {
	authorization := payload.Authorization()
	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || strings.TrimSpace(parts[1]) == "" {
		return nil, nil, errors.New("authentication required")
	}
	if h.auth == nil {
		return nil, nil, errors.New("authentication required")
	}
	claims, err := h.auth.ValidateToken(ctx, strings.TrimSpace(parts[1]))
	if err != nil || claims == nil || claims.UserID == uuid.Nil || claims.Audience != "hiretech-graphql" {
		return nil, nil, errors.New("authentication required")
	}
	if h.sessionValidator != nil {
		if err := h.sessionValidator.ValidateAccess(ctx, claims); err != nil {
			return nil, nil, errors.New("authentication required")
		}
	}
	tokenClass := authcontext.TokenClass(claims.TokenClass)
	if tokenClass == "" {
		tokenClass = authcontext.TokenClassBootstrap
	}
	if tokenClass != authcontext.TokenClassBootstrap && tokenClass != authcontext.TokenClassTenant && tokenClass != authcontext.TokenClassCandidateInterview {
		return nil, nil, errors.New("authentication required")
	}
	actor := authcontext.ActorContext{
		UserID: claims.UserID, Email: claims.Email, SessionID: claims.SessionID,
		TokenClass: tokenClass, OrganizationID: claims.OrganizationID,
		MembershipID: claims.MembershipID, DeviceID: claims.DeviceID, InterviewID: claims.InterviewID,
		Permissions: claims.Permissions, AuthenticationMethods: claims.AuthenticationMethods,
		AuthenticationTime: claims.AuthenticationTime,
	}
	ctx = authcontext.WithActor(ctx, actor)
	if actor.OrganizationID != uuid.Nil {
		ctx = logger.ContextWithOrganizationID(ctx, actor.OrganizationID.String())
	}
	ctx = logger.ContextWithUserID(ctx, actor.UserID.String())
	return ctx, nil, nil
}

func websocketOriginAllowed(allowed []string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}
		for _, candidate := range allowed {
			if candidate == "*" || strings.EqualFold(strings.TrimSpace(candidate), origin) {
				return true
			}
		}
		return false
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid GraphQL request")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, h.maxBodyBytes+1))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "VALIDATION_FAILED", "request body is too large")
		return
	}
	if int64(len(body)) > h.maxBodyBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "VALIDATION_FAILED", "request body is too large")
		return
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid GraphQL request")
		return
	}
	if err := h.validatePersistedOperation(body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	h.server.ServeHTTP(w, r.WithContext(ctx))
}

func (h *Handler) validatePersistedOperation(body []byte) error {
	if h == nil || !h.requirePersistedOperations {
		return nil
	}
	var request struct {
		Query      string `json:"query"`
		Extensions struct {
			PersistedQuery struct {
				Version    int    `json:"version"`
				SHA256Hash string `json:"sha256Hash"`
			} `json:"persistedQuery"`
		} `json:"extensions"`
	}
	if err := json.Unmarshal(body, &request); err != nil || strings.TrimSpace(request.Query) == "" {
		return errors.New("a persisted GraphQL operation is required")
	}
	persisted := request.Extensions.PersistedQuery
	if persisted.Version != 1 || len(persisted.SHA256Hash) != sha256.Size*2 {
		return errors.New("a valid persisted GraphQL operation hash is required")
	}
	hash := strings.ToLower(persisted.SHA256Hash)
	if _, err := hex.DecodeString(hash); err != nil {
		return errors.New("a valid persisted GraphQL operation hash is required")
	}
	digest := sha256.Sum256([]byte(request.Query))
	if hash != hex.EncodeToString(digest[:]) {
		return errors.New("persisted GraphQL operation hash does not match the query")
	}
	if _, ok := h.allowedOperationHashes[hash]; !ok {
		return errors.New("persisted GraphQL operation is not allowlisted")
	}
	return nil
}

// AuthMiddleware builds the trusted actor context only from a verified JWT.
// Client tenant headers are rejected rather than used as a fallback authority.
func AuthMiddleware(authService service.AuthService, validators ...service.SessionValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, name := range []string{"X-Organization-ID", "X-Workspace-ID", "X-App-ID"} {
				if r.Header.Get(name) != "" {
					writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "tenant headers are not accepted by GraphQL")
					return
				}
			}
			if authService == nil {
				writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
				return
			}

			parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || strings.TrimSpace(parts[1]) == "" {
				writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
				return
			}
			claims, err := authService.ValidateToken(r.Context(), strings.TrimSpace(parts[1]))
			if err != nil || claims == nil || claims.UserID == uuid.Nil {
				writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
				return
			}
			if claims.Audience != "hiretech-graphql" {
				writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
				return
			}
			for _, validator := range validators {
				if validator != nil {
					if err := validator.ValidateAccess(r.Context(), claims); err != nil {
						writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
						return
					}
				}
			}

			tokenClass := authcontext.TokenClass(claims.TokenClass)
			if tokenClass == "" {
				tokenClass = authcontext.TokenClassBootstrap
			}
			if tokenClass != authcontext.TokenClassBootstrap && tokenClass != authcontext.TokenClassTenant && tokenClass != authcontext.TokenClassCandidateInterview {
				writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
				return
			}

			actor := authcontext.ActorContext{
				RequestID: requestID(r), UserID: claims.UserID, Email: claims.Email,
				SessionID: claims.SessionID, TokenClass: tokenClass, OrganizationID: claims.OrganizationID,
				MembershipID: claims.MembershipID, DeviceID: claims.DeviceID, InterviewID: claims.InterviewID, Permissions: claims.Permissions,
				AuthenticationMethods: claims.AuthenticationMethods,
				AuthenticationTime:    claims.AuthenticationTime,
			}
			ctx := authcontext.WithActor(r.Context(), actor)
			ctx = logger.ContextWithUserID(ctx, actor.UserID.String())
			if actor.OrganizationID != uuid.Nil {
				ctx = logger.ContextWithOrganizationID(ctx, actor.OrganizationID.String())
			}
			if audit := middleware.RequestAuditFromContext(ctx); audit != nil {
				audit.SetRequestID(requestID(r))
				audit.SetActor(actor.UserID, actor.OrganizationID)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func requestID(r *http.Request) string {
	if id := r.Header.Get(middleware.RequestIDHeader); id != "" {
		return id
	}
	if id, ok := logger.RequestIDFromContext(r.Context()); ok {
		return id
	}
	return ""
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]any{{
		"message": message, "extensions": map[string]string{"code": code},
	}}})
}

func safeErrorPresenter(ctx context.Context, err error) *gqlerror.Error {
	code := "INTERNAL"
	message := "internal server error"
	path := graphql.GetPath(ctx)

	var gqlErr *gqlerror.Error
	if errors.As(err, &gqlErr) {
		message = gqlErr.Message
		path = gqlErr.Path
		code = "VALIDATION_FAILED"
		if gqlErr.Extensions != nil {
			if existing, ok := gqlErr.Extensions["code"].(string); ok && existing != "" {
				if existing == "GRAPHQL_VALIDATION_FAILED" {
					code = "VALIDATION_FAILED"
				} else {
					code = existing
				}
			}
		}
	}
	switch {
	case errors.Is(err, domainErr.ErrUnauthorized):
		code, message = "UNAUTHENTICATED", "authentication required"
	case errors.Is(err, domainErr.ErrForbidden):
		code, message = "FORBIDDEN", "forbidden"
	case errors.Is(err, domainErr.ErrValidation), errors.Is(err, domainErr.ErrBadRequest):
		code, message = "VALIDATION_FAILED", "invalid request"
	case errors.Is(err, domainErr.ErrNotFound):
		code, message = "NOT_FOUND", "resource not found"
	case errors.Is(err, domainErr.ErrConflict), errors.Is(err, domainErr.ErrAlreadyExists):
		code, message = "CONFLICT", "request conflicts with current state"
	case errors.Is(err, domainErr.ErrRateLimited):
		code, message = "RATE_LIMITED", "rate limit exceeded"
	}

	extensions := map[string]any{"code": code}
	if audit := middleware.RequestAuditFromContext(ctx); audit != nil && audit.RequestID != "" {
		extensions["requestId"] = audit.RequestID
	}
	return &gqlerror.Error{Message: message, Path: path, Extensions: extensions}
}
