package usecase

import (
	"context"
	"sync"
)

// FakeEmailSender captures delivery for tests without logging OTP values.
type FakeEmailSender struct {
	mu     sync.Mutex
	Emails []string
	Codes  []string
}

func (f *FakeEmailSender) SendOTP(_ context.Context, email, code string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Emails = append(f.Emails, email)
	f.Codes = append(f.Codes, code)
	return nil
}
func (f *FakeEmailSender) LastCode() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Codes) == 0 {
		return ""
	}
	return f.Codes[len(f.Codes)-1]
}
