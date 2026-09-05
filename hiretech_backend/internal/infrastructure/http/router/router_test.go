package router_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/repository"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	infraAuth "github.com/masterfabric-go/masterfabric/internal/infrastructure/auth"
	iamHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/iam"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/http/router"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
)

type restUserRepo struct{ user *model.User }

func (r *restUserRepo) Create(context.Context, *model.User) error               { return nil }
func (r *restUserRepo) GetByID(context.Context, uuid.UUID) (*model.User, error) { return r.user, nil }
func (r *restUserRepo) GetByEmail(context.Context, string) (*model.User, error) { return r.user, nil }
func (r *restUserRepo) Update(context.Context, *model.User) error               { return nil }
func (r *restUserRepo) Delete(context.Context, uuid.UUID) error                 { return nil }
func (r *restUserRepo) List(context.Context, int, int) ([]*model.User, int, error) {
	return nil, 0, nil
}

var _ repository.UserRepository = (*restUserRepo)(nil)

func TestExistingRESTRoutesRemainAvailableAlongsideGraphQLWiring(t *testing.T) {
	userID := uuid.New()
	jwtService := infraAuth.NewJWTService(config.JWTConfig{Secret: "router-test-secret", ExpirationHours: 1, Issuer: "test"})
	userRepo := &restUserRepo{user: &model.User{ID: userID, Email: "rest@example.com", Status: model.UserStatusActive, CreatedAt: time.Now().UTC()}}
	iam := iamHandler.NewHandler(nil, nil, nil, userRepo)
	r := router.New(router.Dependencies{
		Logger:       slog.Default(),
		AuthService:  jwtService,
		IAMHandler:   iam,
		MaxBodyBytes: 1 << 20,
	})

	unauthorized := httptest.NewRecorder()
	r.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	assert.Equal(t, http.StatusUnauthorized, unauthorized.Code)

	token, err := jwtService.GenerateToken(context.Background(), service.TokenClaims{UserID: userID, Email: "rest@example.com"})
	require.NoError(t, err)
	authorizedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	authorizedRequest.Header.Set("Authorization", "Bearer "+token)
	authorized := httptest.NewRecorder()
	r.ServeHTTP(authorized, authorizedRequest)
	assert.Equal(t, http.StatusOK, authorized.Code)
	assert.Contains(t, authorized.Body.String(), "rest@example.com")

	protectedWithoutRBAC := httptest.NewRecorder()
	protectedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/users/", nil)
	protectedRequest.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(protectedWithoutRBAC, protectedRequest)
	assert.Equal(t, http.StatusServiceUnavailable, protectedWithoutRBAC.Code)

	health := httptest.NewRecorder()
	r.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	assert.Equal(t, http.StatusOK, health.Code)
}
