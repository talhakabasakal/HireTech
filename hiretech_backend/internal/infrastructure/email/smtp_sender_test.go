package email

import (
	"bytes"
	"net/mail"
	"strings"
	"testing"

	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/stretchr/testify/require"
)

func TestNewSMTPSenderRejectsInvalidConfiguration(t *testing.T) {
	_, err := NewSMTPSender(config.EmailConfig{SMTPPort: 1025, From: "bad address", TLSMode: "none"})
	require.Error(t, err)
	_, err = NewSMTPSender(config.EmailConfig{SMTPHost: "localhost", SMTPPort: 1025, From: "noreply@example.test", TLSMode: "invalid"})
	require.Error(t, err)
}

func TestWriteOTPMessage(t *testing.T) {
	from, _ := mail.ParseAddress("HireTech <noreply@example.test>")
	to, _ := mail.ParseAddress("candidate@example.test")
	var output bytes.Buffer
	require.NoError(t, writeOTPMessage(&output, from, to, "123456"))
	require.Contains(t, output.String(), "123456")
	require.True(t, strings.Contains(output.String(), "Content-Type: text/plain; charset=UTF-8"))
}
