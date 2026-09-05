package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	iamService "github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
)

// NewSender returns the configured OTP sender and rejects unsafe/unknown modes.
func NewSender(cfg config.EmailConfig) (iamService.EmailSender, error) {
	switch cfg.Provider {
	case "noop":
		return NoopSender{}, nil
	case "smtp":
		return NewSMTPSender(cfg)
	default:
		return nil, fmt.Errorf("unsupported email provider %q", cfg.Provider)
	}
}

// SMTPSender sends short-lived verification codes without logging their value.
type SMTPSender struct {
	host     string
	port     int
	username string
	password string
	from     *mail.Address
	tlsMode  string
	timeout  time.Duration
}

func NewSMTPSender(cfg config.EmailConfig) (*SMTPSender, error) {
	if strings.TrimSpace(cfg.SMTPHost) == "" || cfg.SMTPPort < 1 || cfg.SMTPPort > 65535 {
		return nil, errors.New("smtp host and port are required")
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("invalid email from address: %w", err)
	}
	if cfg.TLSMode != "none" && cfg.TLSMode != "starttls" && cfg.TLSMode != "implicit" {
		return nil, errors.New("smtp tls mode must be none, starttls, or implicit")
	}
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &SMTPSender{host: cfg.SMTPHost, port: cfg.SMTPPort, username: cfg.Username, password: cfg.Password, from: from, tlsMode: cfg.TLSMode, timeout: timeout}, nil
}

func (s *SMTPSender) SendOTP(ctx context.Context, recipient, code string) error {
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

	address := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	dialer := &net.Dialer{Timeout: s.timeout}
	var conn net.Conn
	if s.tlsMode == "implicit" {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.host})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("smtp connection failed: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(s.timeout))

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp handshake failed: %w", err)
	}
	defer client.Close()
	if s.tlsMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("smtp server does not support STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.host}); err != nil {
			return fmt.Errorf("smtp STARTTLS failed: %w", err)
		}
	}
	if s.username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
			return fmt.Errorf("smtp authentication failed: %w", err)
		}
	}
	if err := client.Mail(s.from.Address); err != nil {
		return fmt.Errorf("smtp sender rejected: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("smtp recipient rejected: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp message rejected: %w", err)
	}
	if err := writeOTPMessage(w, s.from, to, code); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp delivery failed: %w", err)
	}
	return client.Quit()
}

func writeOTPMessage(w io.Writer, from, to *mail.Address, code string) error {
	subject := mime.QEncoding.Encode("UTF-8", "HireTech doğrulama kodu")
	body := "HireTech doğrulama kodunuz: " + code + "\r\n\r\nBu kod kısa süre içinde geçerliliğini yitirir. Kodu kimseyle paylaşmayın.\r\n"
	message := "From: " + from.String() + "\r\nTo: " + to.String() + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" + body
	if _, err := io.WriteString(w, message); err != nil {
		return fmt.Errorf("write smtp message: %w", err)
	}
	return nil
}
