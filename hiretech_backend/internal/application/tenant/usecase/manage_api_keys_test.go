package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/masterfabric-go/masterfabric/internal/domain/tenant/model"
	tenantRepo "github.com/masterfabric-go/masterfabric/internal/domain/tenant/repository"
	"github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type apiKeyRepositoryStub struct {
	current     *model.AppAPIKey
	replacement *model.AppAPIKey
	revoked     uuid.UUID
}

func (r *apiKeyRepositoryStub) Create(context.Context, *model.AppAPIKey) error { return nil }

func (r *apiKeyRepositoryStub) GetByID(context.Context, uuid.UUID) (*model.AppAPIKey, error) {
	return r.current, nil
}

func (r *apiKeyRepositoryStub) GetByHash(context.Context, string) (*model.AppAPIKey, error) {
	return nil, errors.New(errors.ErrNotFound, "not found", nil)
}

func (r *apiKeyRepositoryStub) Rotate(_ context.Context, oldID uuid.UUID, replacement *model.AppAPIKey) error {
	r.revoked = oldID
	r.replacement = replacement
	replacement.CreatedAt = time.Now().UTC()
	return nil
}

func (r *apiKeyRepositoryStub) Revoke(context.Context, uuid.UUID) error { return nil }

func (r *apiKeyRepositoryStub) ListByApp(context.Context, uuid.UUID) ([]*model.AppAPIKey, error) {
	return nil, nil
}

var _ tenantRepo.APIKeyRepository = (*apiKeyRepositoryStub)(nil)

func TestRotateKeyReturnsOneTimeReplacementAndPreservesMetadata(t *testing.T) {
	oldID := uuid.New()
	repo := &apiKeyRepositoryStub{current: &model.AppAPIKey{ID: oldID, AppID: uuid.New(), Name: "production", Scopes: []byte(`{"read":true}`), IsActive: true}}
	service := NewManageAPIKeysUseCase(repo)

	rotated, err := service.RotateKey(context.Background(), oldID)
	require.NoError(t, err)
	require.NotEmpty(t, rotated.Key)
	require.True(t, len(rotated.Key) > 3)
	require.Equal(t, oldID, repo.revoked)
	require.Equal(t, repo.current.AppID, repo.replacement.AppID)
	require.Equal(t, repo.current.Name, repo.replacement.Name)
	require.Equal(t, repo.replacement.ID, rotated.ID)
	require.NotEqual(t, repo.current.KeyHash, repo.replacement.KeyHash)

	// The returned raw key is represented only by its hash in the replacement.
	require.Equal(t, hashAPIKey(rotated.Key), repo.replacement.KeyHash)
}
