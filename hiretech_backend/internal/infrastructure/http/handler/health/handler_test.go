package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

type failingPinger struct{}

func (failingPinger) Ping(context.Context) error {
	return errors.New("connection refused host=secret-db:5432")
}

type healthyPinger struct{}

func (healthyPinger) Ping(context.Context) error { return nil }

type healthyRedisPinger struct{}

func (healthyRedisPinger) Ping(context.Context) *redis.StatusCmd {
	return redis.NewStatusResult("PONG", nil)
}

func TestReadinessRequiresBothSecurityDependencies(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler *Handler
		status  int
	}{
		{"neither configured", NewHandler(nil, nil), http.StatusServiceUnavailable},
		{"postgres absent", &Handler{redis: healthyRedisPinger{}}, http.StatusServiceUnavailable},
		{"redis absent", &Handler{db: healthyPinger{}}, http.StatusServiceUnavailable},
		{"both healthy", &Handler{db: healthyPinger{}, redis: healthyRedisPinger{}}, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			assert.NotPanics(t, func() { tc.handler.Readiness(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil)) })
			assert.Equal(t, tc.status, recorder.Code)
		})
	}
}

func TestReadiness_DoesNotExposeInternalErrors(t *testing.T) {
	handler := &Handler{db: failingPinger{}}

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	handler.Readiness(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.NotContains(t, rec.Body.String(), "secret-db")
	assert.Contains(t, rec.Body.String(), "unhealthy")
}
