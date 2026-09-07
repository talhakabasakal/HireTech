package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/stretchr/testify/require"
)

func TestNewResendSenderRejectsInvalidConfiguration(t *testing.T) {
	_, err := NewResendSender(config.EmailConfig{From: "noreply@example.test", APIURL: "https://api.resend.com/emails"})
	require.Error(t, err)
	_, err = NewResendSender(config.EmailConfig{APIKey: "re_test", From: "noreply@example.test", APIURL: "http://api.example.test/emails"})
	require.Error(t, err)
}

func TestResendSenderSendsOTPOverHTTPS(t *testing.T) {
	var received resendEmailRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer re_test", r.Header.Get("Authorization"))
		require.Equal(t, "HireTech/1.0", r.Header.Get("User-Agent"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender, err := NewResendSender(config.EmailConfig{
		APIKey:     "re_test",
		APIURL:     server.URL,
		From:       "HireTech <noreply@example.test>",
		TimeoutSec: 2,
	})
	require.NoError(t, err)
	require.NoError(t, sender.SendOTP(context.Background(), "candidate@example.test", "123456"))
	require.Equal(t, `"HireTech" <noreply@example.test>`, received.From)
	require.Equal(t, []string{"candidate@example.test"}, received.To)
	require.Equal(t, "HireTech doğrulama kodu", received.Subject)
	require.Contains(t, received.Text, "123456")
}
