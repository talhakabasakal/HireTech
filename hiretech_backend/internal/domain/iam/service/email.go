package service

import "context"

// EmailSender delivers verification mail. Implementations must never log the code.
type EmailSender interface {
	SendOTP(ctx context.Context, email, code string) error
}
