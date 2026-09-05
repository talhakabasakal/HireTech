package model

import (
	"time"

	"github.com/google/uuid"
)

type OTPChallenge struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Email       string
	CodeHash    string
	ExpiresAt   time.Time
	Attempts    int
	MaxAttempts int
	UsedAt      *time.Time
	CreatedAt   time.Time
}

func (c *OTPChallenge) IsExpired(now time.Time) bool { return !now.Before(c.ExpiresAt) }
func (c *OTPChallenge) IsUsed() bool                 { return c.UsedAt != nil }

type DeviceState string

const (
	DeviceStateCurrent    DeviceState = "current"
	DeviceStateTrusted    DeviceState = "trusted"
	DeviceStateSuspicious DeviceState = "suspicious"
	DeviceStateRevoked    DeviceState = "revoked"
)

type Device struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Name            string
	FingerprintHash string
	State           DeviceState
	LastSeenAt      time.Time
	CreatedAt       time.Time
	RevokedAt       *time.Time
}

func (d *Device) IsRevoked() bool { return d == nil || d.State == DeviceStateRevoked }

type Session struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	OrganizationID        uuid.UUID
	DeviceID              uuid.UUID
	FamilyID              uuid.UUID
	RefreshTokenHash      string
	RefreshTokenExpiresAt time.Time
	CreatedAt             time.Time
	LastUsedAt            time.Time
	RevokedAt             *time.Time
	AuthenticationTime    time.Time
}

func (s *Session) IsRevoked() bool { return s == nil || s.RevokedAt != nil }
