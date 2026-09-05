package service

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TokenClaims represents the contents of a JWT token.
type TokenClaims struct {
	UserID                uuid.UUID `json:"user_id"`
	Email                 string    `json:"email"`
	OrganizationID        uuid.UUID `json:"organization_id,omitempty"`
	MembershipID          uuid.UUID `json:"membership_id,omitempty"`
	SessionID             string    `json:"session_id,omitempty"`
	DeviceID              uuid.UUID `json:"device_id,omitempty"`
	InterviewID           uuid.UUID `json:"interview_id,omitempty"`
	Audience              string    `json:"audience,omitempty"`
	Issuer                string    `json:"issuer,omitempty"`
	TokenClass            string    `json:"token_class,omitempty"`
	Roles                 []string  `json:"roles,omitempty"`
	Permissions           []string  `json:"permissions,omitempty"`
	AuthenticationMethods []string  `json:"authentication_methods,omitempty"`
	AuthenticationTime    time.Time `json:"authentication_time,omitempty"`
}

// AuthService defines authentication operations.
type AuthService interface {
	// HashPassword hashes a plaintext password.
	HashPassword(password string) (string, error)
	// VerifyPassword checks a plaintext password against a hash.
	VerifyPassword(hashedPassword, password string) error
	// GenerateToken creates a JWT token from claims.
	GenerateToken(ctx context.Context, claims TokenClaims) (string, error)
	// ValidateToken validates a JWT token and returns its claims.
	ValidateToken(ctx context.Context, token string) (*TokenClaims, error)
}

// SessionValidator validates the server-side state behind an access token.
type SessionValidator interface {
	ValidateAccess(ctx context.Context, claims *TokenClaims) error
}
