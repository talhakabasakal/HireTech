package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	privacyModel "github.com/masterfabric-go/masterfabric/internal/domain/privacy/model"
	privacyRepo "github.com/masterfabric-go/masterfabric/internal/domain/privacy/repository"
)

var (
	ErrRecentAuthenticationRequired = errors.New("recent authentication is required")
	ErrLegalHoldBlocksDeletion      = errors.New("active legal hold blocks account deletion")
	ErrOrganizationRequired         = errors.New("organization scope is required")
)

type Service struct {
	repo privacyRepo.Repository
	now  func() time.Time
}

func NewService(repo privacyRepo.Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) RequestExport(ctx context.Context, userID, organizationID uuid.UUID) (*privacyModel.Request, error) {
	return s.create(ctx, userID, organizationID, privacyModel.RequestExport, false, time.Time{})
}

func (s *Service) RequestDeletion(ctx context.Context, userID, organizationID uuid.UUID, authenticatedAt time.Time) (*privacyModel.Request, error) {
	return s.create(ctx, userID, organizationID, privacyModel.RequestDeletion, true, authenticatedAt)
}

func (s *Service) GetRequest(ctx context.Context, userID, requestID uuid.UUID) (*privacyModel.Request, error) {
	return s.repo.GetRequest(ctx, userID, requestID)
}

func (s *Service) CreateLegalHold(ctx context.Context, hold *privacyModel.LegalHold) error {
	if hold == nil || hold.OrganizationID == uuid.Nil || hold.CreatedBy == uuid.Nil || strings.TrimSpace(hold.ResourceType) == "" || strings.TrimSpace(hold.ResourceID) == "" || strings.TrimSpace(hold.Reason) == "" {
		return errors.New("legal hold scope, reason, and actor are required")
	}
	return s.repo.CreateLegalHold(ctx, hold)
}

func (s *Service) ReleaseLegalHold(ctx context.Context, organizationID, holdID uuid.UUID) error {
	return s.repo.ReleaseLegalHold(ctx, organizationID, holdID)
}

func (s *Service) create(ctx context.Context, userID, organizationID uuid.UUID, kind privacyModel.RequestKind, requireRecentAuth bool, authenticatedAt time.Time) (*privacyModel.Request, error) {
	if s == nil || s.repo == nil || userID == uuid.Nil {
		return nil, errors.New("privacy service is not configured")
	}
	if organizationID == uuid.Nil {
		return nil, ErrOrganizationRequired
	}
	if requireRecentAuth && (authenticatedAt.IsZero() || s.now().Sub(authenticatedAt) < 0 || s.now().Sub(authenticatedAt) > 15*time.Minute) {
		return nil, ErrRecentAuthenticationRequired
	}
	if requireRecentAuth {
		hold, err := s.repo.HasActiveHold(ctx, userID, organizationID)
		if err != nil {
			return nil, err
		}
		if hold {
			return nil, ErrLegalHoldBlocksDeletion
		}
	}
	now := s.now().UTC()
	request := &privacyModel.Request{ID: uuid.New(), UserID: userID, OrganizationID: organizationID, Kind: kind, Status: privacyModel.StatusPending, CreatedAt: now}
	if err := s.repo.CreateRequest(ctx, request); err != nil {
		return nil, err
	}
	return request, nil
}
