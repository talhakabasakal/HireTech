package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	privacyModel "github.com/masterfabric-go/masterfabric/internal/domain/privacy/model"
)

type privacyRepositoryStub struct{}

func (privacyRepositoryStub) CreateRequest(context.Context, *privacyModel.Request) error { return nil }
func (privacyRepositoryStub) GetRequest(context.Context, uuid.UUID, uuid.UUID) (*privacyModel.Request, error) {
	return nil, nil
}
func (privacyRepositoryStub) HasActiveHold(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (privacyRepositoryStub) CreateLegalHold(context.Context, *privacyModel.LegalHold) error {
	return nil
}
func (privacyRepositoryStub) ReleaseLegalHold(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func TestServiceRejectsUnscopedRequest(t *testing.T) {
	service := NewService(privacyRepositoryStub{})
	_, err := service.RequestExport(context.Background(), uuid.New(), uuid.Nil)
	if err != ErrOrganizationRequired {
		t.Fatalf("expected organization scope error, got %v", err)
	}
}

func TestServiceRequiresRecentAuthenticationForDeletion(t *testing.T) {
	service := NewService(privacyRepositoryStub{})
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	_, err := service.RequestDeletion(context.Background(), uuid.New(), uuid.New(), now.Add(-16*time.Minute))
	if err != ErrRecentAuthenticationRequired {
		t.Fatalf("expected recent authentication error, got %v", err)
	}
}
