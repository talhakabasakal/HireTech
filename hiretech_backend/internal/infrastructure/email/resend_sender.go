package email

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	iamService "github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
)

const defaultResendAPIURL = "https://api.resend.com/emails"

// ResendSender delivers OTP messages through Resend's HTTPS API. It avoids
// outbound SMTP restrictions on hosted free-tier services.
type ResendSender struct {
	apiKey string
	apiURL string
	from   *mail.Address
	client *http.Client
}

type resendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

// NewResendSender validates the provider configuration without making a
// network request. The API key is intentionally never included in errors.
func NewResendSender(cfg config.EmailConfig) (*ResendSender, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, errors.New("resend api key is required")
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("invalid email from address: %w", err)
	}
	apiURL := strings.TrimSpace(cfg.APIURL)
	if apiURL == "" {
		apiURL = defaultResendAPIURL
	}
	parsed, err := url.Parse(apiURL)
	isLocalHTTP := parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "::1")
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && !isLocalHTTP) {
		return nil, errors.New("resend api url must be an absolute HTTPS URL")
	}
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &ResendSender{
		apiKey: apiKey,
		apiURL: apiURL,
		from:   from,
		client: &http.Client{Timeout: timeout},
	}, nil
}

var _ iamService.EmailSender = (*ResendSender)(nil)

func (s *ResendSender) SendOTP(ctx context.Context, recipient, code string) error {
	to, err := mail.ParseAddress(recipient)
	if err != nil || strings.ContainsAny(recipient, "\r\n") {
		return errors.New("invalid recipient address")
	}
	if len(code) != 6 {
		return errors.New("invalid verification code")
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return errors.New("invalid verification code")
		}
	}

	payload, err := json.Marshal(resendEmailRequest{
		From:    s.from.String(),
		To:      []string{to.Address},
		Subject: "HireTech doğrulama kodu",
		Text:    "HireTech doğrulama kodunuz: " + code + "\n\nBu kod kısa süre içinde geçerliliğini yitirir. Kodu kimseyle paylaşmayın.\n",
	})
	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "HireTech/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("resend request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("resend request rejected with status %d", resp.StatusCode)
	}
	return nil
}
