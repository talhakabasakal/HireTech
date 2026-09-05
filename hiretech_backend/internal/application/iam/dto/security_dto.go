package dto

import (
	"time"

	"github.com/google/uuid"
)

type OTPRequest struct {
	Email string `json:"email" validate:"required,email"`
}
type OTPVerifyRequest struct {
	Email      string `json:"email" validate:"required,email"`
	Code       string `json:"code" validate:"required,len=6,numeric"`
	DeviceName string `json:"device_name"`
}
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}
type SecurityTokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	SessionID    uuid.UUID `json:"session_id"`
	DeviceID     uuid.UUID `json:"device_id"`
}
type DeviceInfo struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	State      string    `json:"state"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
}
