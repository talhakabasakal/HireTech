package email

import "context"

// NoopSender is available only when EMAIL_PROVIDER=noop is explicitly chosen
// for isolated tests or UI demos. It must not be the production default.
// It intentionally does not log or expose OTP values.
type NoopSender struct{}

func (NoopSender) SendOTP(context.Context, string, string) error { return nil }
