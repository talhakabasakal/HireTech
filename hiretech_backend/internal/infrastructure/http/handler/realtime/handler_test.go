package realtime

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/stretchr/testify/assert"
)

func TestResolveOrgIDUsesJWTClaimAndRejectsConflictingHeader(t *testing.T) {
	claimOrgID := uuid.New()
	request := httptest.NewRequest("GET", "http://localhost/api/v1/ws", nil)
	request.Header.Set("X-Organization-ID", uuid.NewString())

	resolved, err := resolveOrgID(request, claimOrgID)

	assert.ErrorIs(t, err, domainErr.ErrForbidden)
	assert.Equal(t, uuid.Nil, resolved)
}

func TestResolveOrgIDAcceptsMatchingHeader(t *testing.T) {
	claimOrgID := uuid.New()
	request := httptest.NewRequest("GET", "http://localhost/api/v1/ws", nil)
	request.Header.Set("X-Organization-ID", claimOrgID.String())

	resolved, err := resolveOrgID(request, claimOrgID)

	assert.NoError(t, err)
	assert.Equal(t, claimOrgID, resolved)
}

func TestResolveOrgIDRequiresTenantClaim(t *testing.T) {
	request := httptest.NewRequest("GET", "http://localhost/api/v1/ws", nil)
	request.Header.Set("X-Organization-ID", uuid.NewString())

	resolved, err := resolveOrgID(request, uuid.Nil)

	assert.True(t, errors.Is(err, domainErr.ErrForbidden))
	assert.Equal(t, uuid.Nil, resolved)
}
