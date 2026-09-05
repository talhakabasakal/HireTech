package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
)

type OTPChallengeRepository interface {
	InvalidateActive(ctx context.Context, email string) error
	Create(ctx context.Context, challenge *model.OTPChallenge) error
	GetActiveByEmail(ctx context.Context, email string, now time.Time) (*model.OTPChallenge, error)
	IncrementAttempts(ctx context.Context, id uuid.UUID, maxAttempts int) (int, error)
	Consume(ctx context.Context, id uuid.UUID, codeHash string, now time.Time) (bool, error)
}

// OTPChallengeReplacer atomically invalidates prior challenges and creates one.
// It is optional so lightweight test fakes remain compatible with the base repository.
type OTPChallengeReplacer interface {
	Replace(ctx context.Context, email string, challenge *model.OTPChallenge) error
}

type SessionRepository interface {
	Create(ctx context.Context, session *model.Session) error
	Get(ctx context.Context, id uuid.UUID) (*model.Session, error)
	Rotate(ctx context.Context, id uuid.UUID, presentedHash, replacementHash string, expiresAt, now time.Time) (*model.Session, bool, error)
	Revoke(ctx context.Context, id uuid.UUID, now time.Time) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID, now time.Time) error
	RevokeAllByUser(ctx context.Context, userID uuid.UUID, now time.Time) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*model.Session, error)
}

type DeviceRepository interface {
	Create(ctx context.Context, device *model.Device) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Device, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*model.Device, error)
	Revoke(ctx context.Context, id, userID uuid.UUID, now time.Time) error
}
