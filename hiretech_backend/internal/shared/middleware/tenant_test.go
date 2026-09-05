package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTenantResolverRejectsHeaderThatConflictsWithJWTClaim(t *testing.T) {
	claimed := uuid.New()
	requested := uuid.New()
	ctx := context.WithValue(context.Background(), ContextKeyOrganizationID, claimed)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	req.Header.Set("X-Organization-ID", requested.String())
	rec := httptest.NewRecorder()

	called := false
	h := TenantResolverWithWorkspace(nil, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.False(t, called)
}

func TestRequireOrganizationPathRejectsCrossTenantPath(t *testing.T) {
	claimed := uuid.New()
	requested := uuid.New()
	route := chi.NewRouteContext()
	route.URLParams.Add("orgId", requested.String())
	ctx := context.WithValue(context.Background(), ContextKeyTenantID, claimed)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, route)
	req := httptest.NewRequest(http.MethodGet, "/organizations/"+requested.String(), nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	called := false
	RequireOrganizationPath(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.False(t, called)
}
