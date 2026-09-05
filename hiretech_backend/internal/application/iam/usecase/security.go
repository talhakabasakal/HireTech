package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/audit/model"
	auditRepo "github.com/masterfabric-go/masterfabric/internal/domain/audit/repository"
	iamModel "github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	iamRepo "github.com/masterfabric-go/masterfabric/internal/domain/iam/repository"
	iamService "github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

const (
	AuditOTPRequested           = "otp.requested"
	AuditOTPSucceeded           = "otp.succeeded"
	AuditOTPFailed              = "otp.failed"
	AuditLoginSucceeded         = "login.succeeded"
	AuditLoginFailed            = "login.failed"
	AuditRefreshRotated         = "refresh.rotated"
	AuditRefreshReused          = "refresh.reused"
	AuditLogout                 = "logout"
	AuditLogoutAll              = "logout.all"
	AuditDeviceRegistered       = "device.registered"
	AuditDeviceRevoked          = "device.revoked"
	AuditPasswordReset          = "password.reset"
	AuditRecentAuthFailed       = "recent_auth.failed"
	AuditSecuritySettingChanged = "security_setting.changed"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

type AuditRecorder interface {
	Record(ctx context.Context, event SecurityAuditEvent) error
}

type SecurityAuditEvent struct {
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	EventType      string
	Result         string
	CorrelationID  string
	Timestamp      time.Time
}

// RepositoryAuditRecorder adapts the existing audit persistence abstraction.
type RepositoryAuditRecorder struct{ Repo auditRepo.AuditRepository }

func (r RepositoryAuditRecorder) Record(ctx context.Context, event SecurityAuditEvent) error {
	if r.Repo == nil {
		return nil
	}
	metadata := []byte(fmt.Sprintf(`{"result":%q,"event_type":%q}`, event.Result, event.EventType))
	var actor *uuid.UUID
	if event.ActorID != uuid.Nil {
		id := event.ActorID
		actor = &id
	}
	return r.Repo.Create(ctx, &model.AuditLog{
		OrganizationID: event.OrganizationID, UserID: actor, RequestID: event.CorrelationID,
		Action: event.EventType, ResourceType: "security", ResourceID: event.ActorID.String(), Metadata: metadata,
		CreatedAt: event.Timestamp,
	})
}

type SecurityService struct {
	otpRepo  iamRepo.OTPChallengeRepository
	sessions iamRepo.SessionRepository
	devices  iamRepo.DeviceRepository
	users    iamRepo.UserRepository
	auth     iamService.AuthService
	email    iamService.EmailSender
	rate     RateLimiter
	audit    AuditRecorder
	secret   string
	cfg      config.SecurityConfig
	now      func() time.Time
}

func NewSecurityService(otpRepo iamRepo.OTPChallengeRepository, sessions iamRepo.SessionRepository, devices iamRepo.DeviceRepository, users iamRepo.UserRepository, auth iamService.AuthService, email iamService.EmailSender, limiter RateLimiter, audit AuditRecorder, secret string, cfg config.SecurityConfig) *SecurityService {
	if cfg.OTPExpirationMinutes <= 0 {
		cfg.OTPExpirationMinutes = 10
	}
	if cfg.OTPMaxAttempts <= 0 {
		cfg.OTPMaxAttempts = 5
	}
	if cfg.OTPRequestsPerHour <= 0 {
		cfg.OTPRequestsPerHour = 5
	}
	if cfg.OTPVerificationsPerHour <= 0 {
		cfg.OTPVerificationsPerHour = 10
	}
	if cfg.AccessTokenMinutes <= 0 {
		cfg.AccessTokenMinutes = 15
	}
	if cfg.RefreshTokenDays <= 0 {
		cfg.RefreshTokenDays = 30
	}
	if cfg.RecentAuthenticationMinutes <= 0 {
		cfg.RecentAuthenticationMinutes = 10
	}
	return &SecurityService{otpRepo: otpRepo, sessions: sessions, devices: devices, users: users, auth: auth, email: email, rate: limiter, audit: audit, secret: secret, cfg: cfg, now: func() time.Time { return time.Now().UTC() }}
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
func (s *SecurityService) record(ctx context.Context, event, result string, actor uuid.UUID) {
	if s.audit != nil {
		_ = s.audit.Record(ctx, SecurityAuditEvent{ActorID: actor, EventType: event, Result: result, CorrelationID: correlationID(ctx), Timestamp: s.now()})
	}
}
func correlationID(ctx context.Context) string {
	if actor, ok := authcontext.FromContext(ctx); ok {
		return actor.RequestID
	}
	return ""
}

func (s *SecurityService) RequestOTP(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	if email == "" {
		return domainErr.New(domainErr.ErrValidation, "email is required", nil)
	}
	if s.rate != nil {
		allowed, err := s.rate.Allow(ctx, "otp:request:"+email, s.cfg.OTPRequestsPerHour, time.Hour)
		if err != nil {
			return domainErr.New(domainErr.ErrInternal, "unable to process verification request", err)
		}
		if !allowed {
			s.record(ctx, AuditOTPRequested, "RATE_LIMITED", uuid.Nil)
			return domainErr.New(domainErr.ErrRateLimited, "verification request limit exceeded", nil)
		}
	}
	var user *iamModel.User
	if s.users != nil {
		user, _ = s.users.GetByEmail(ctx, email)
	}
	code, err := iamService.GenerateOTP()
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "unable to process verification request", err)
	}
	challenge := &iamModel.OTPChallenge{ID: uuid.New(), Email: email, CodeHash: iamService.HashOTP(s.secret, code), ExpiresAt: s.now().Add(time.Duration(s.cfg.OTPExpirationMinutes) * time.Minute), MaxAttempts: s.cfg.OTPMaxAttempts}
	if user != nil {
		challenge.UserID = user.ID
	}
	var persistErr error
	if replacer, ok := s.otpRepo.(iamRepo.OTPChallengeReplacer); ok {
		persistErr = replacer.Replace(ctx, email, challenge)
	} else {
		persistErr = s.otpRepo.InvalidateActive(ctx, email)
		if persistErr == nil {
			persistErr = s.otpRepo.Create(ctx, challenge)
		}
	}
	if persistErr != nil {
		return domainErr.New(domainErr.ErrInternal, "unable to process verification request", persistErr)
	}
	if user != nil && s.email != nil {
		if err := s.email.SendOTP(ctx, email, code); err != nil {
			return domainErr.New(domainErr.ErrInternal, "unable to process verification request", err)
		}
	}
	// Deliberately do not reveal whether the account exists.
	s.record(ctx, AuditOTPRequested, "SUCCESS", func() uuid.UUID {
		if user != nil {
			return user.ID
		}
		return uuid.Nil
	}())
	return nil
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	SessionID    uuid.UUID `json:"session_id"`
	DeviceID     uuid.UUID `json:"device_id"`
}

func (s *SecurityService) VerifyOTP(ctx context.Context, email, code, deviceName string) (*TokenPair, error) {
	email = normalizeEmail(email)
	if s.rate != nil {
		allowed, err := s.rate.Allow(ctx, "otp:verify:"+email, s.cfg.OTPVerificationsPerHour, time.Hour)
		if err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "verification failed", err)
		}
		if !allowed {
			s.record(ctx, AuditOTPFailed, "RATE_LIMITED", uuid.Nil)
			return nil, domainErr.New(domainErr.ErrRateLimited, "verification limit exceeded", nil)
		}
	}
	challenge, err := s.otpRepo.GetActiveByEmail(ctx, email, s.now())
	if err != nil || challenge == nil || challenge.IsUsed() {
		s.record(ctx, AuditOTPFailed, "FAILURE", uuid.Nil)
		return nil, domainErr.New(domainErr.ErrOTPInvalid, "invalid or expired verification code", nil)
	}
	if challenge.IsExpired(s.now()) {
		s.record(ctx, AuditOTPFailed, "EXPIRED", challenge.UserID)
		return nil, domainErr.New(domainErr.ErrOTPExpired, "invalid or expired verification code", nil)
	}
	if challenge.Attempts >= challenge.MaxAttempts {
		s.record(ctx, AuditOTPFailed, "ATTEMPTS_EXCEEDED", challenge.UserID)
		return nil, domainErr.New(domainErr.ErrOTPAttemptsExceeded, "invalid or expired verification code", nil)
	}
	if !constantTimeEqual(iamService.HashOTP(s.secret, strings.TrimSpace(code)), challenge.CodeHash) {
		attempts, incErr := s.otpRepo.IncrementAttempts(ctx, challenge.ID, challenge.MaxAttempts)
		if incErr != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "verification failed", incErr)
		}
		s.record(ctx, AuditOTPFailed, "FAILURE", challenge.UserID)
		if attempts >= challenge.MaxAttempts {
			return nil, domainErr.New(domainErr.ErrOTPAttemptsExceeded, "invalid or expired verification code", nil)
		}
		return nil, domainErr.New(domainErr.ErrOTPInvalid, "invalid or expired verification code", nil)
	}
	consumed, err := s.otpRepo.Consume(ctx, challenge.ID, challenge.CodeHash, s.now())
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "verification failed", err)
	}
	if !consumed || challenge.UserID == uuid.Nil {
		s.record(ctx, AuditOTPFailed, "FAILURE", challenge.UserID)
		return nil, domainErr.New(domainErr.ErrOTPInvalid, "invalid or expired verification code", nil)
	}
	device := &iamModel.Device{ID: uuid.New(), UserID: challenge.UserID, Name: strings.TrimSpace(deviceName), State: iamModel.DeviceStateCurrent, CreatedAt: s.now(), LastSeenAt: s.now()}
	if err := s.devices.Create(ctx, device); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "verification failed", err)
	}
	pair, err := s.createSession(ctx, challenge.UserID, device.ID, s.now())
	if err != nil {
		return nil, err
	}
	s.record(ctx, AuditOTPSucceeded, "SUCCESS", challenge.UserID)
	s.record(ctx, AuditDeviceRegistered, "SUCCESS", challenge.UserID)
	return pair, nil
}

// ResetPassword verifies the one-time code issued by RequestOTP and replaces
// the user's password. The challenge is consumed before the password update so
// the same code cannot be reused.
func (s *SecurityService) ResetPassword(ctx context.Context, email, code, newPassword string) error {
	email = normalizeEmail(email)
	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < 8 {
		return domainErr.New(domainErr.ErrValidation, "password must contain at least 8 characters", nil)
	}
	if s.rate != nil {
		allowed, err := s.rate.Allow(ctx, "otp:verify:"+email, s.cfg.OTPVerificationsPerHour, time.Hour)
		if err != nil {
			return domainErr.New(domainErr.ErrInternal, "password reset failed", err)
		}
		if !allowed {
			return domainErr.New(domainErr.ErrRateLimited, "verification limit exceeded", nil)
		}
	}
	challenge, err := s.otpRepo.GetActiveByEmail(ctx, email, s.now())
	if err != nil || challenge == nil || challenge.IsUsed() || challenge.UserID == uuid.Nil {
		return domainErr.New(domainErr.ErrOTPInvalid, "invalid or expired verification code", nil)
	}
	if challenge.IsExpired(s.now()) {
		return domainErr.New(domainErr.ErrOTPExpired, "invalid or expired verification code", nil)
	}
	if challenge.Attempts >= challenge.MaxAttempts {
		return domainErr.New(domainErr.ErrOTPAttemptsExceeded, "invalid or expired verification code", nil)
	}
	if !constantTimeEqual(iamService.HashOTP(s.secret, strings.TrimSpace(code)), challenge.CodeHash) {
		attempts, incErr := s.otpRepo.IncrementAttempts(ctx, challenge.ID, challenge.MaxAttempts)
		if incErr != nil {
			return domainErr.New(domainErr.ErrInternal, "password reset failed", incErr)
		}
		if attempts >= challenge.MaxAttempts {
			return domainErr.New(domainErr.ErrOTPAttemptsExceeded, "invalid or expired verification code", nil)
		}
		return domainErr.New(domainErr.ErrOTPInvalid, "invalid or expired verification code", nil)
	}
	consumed, err := s.otpRepo.Consume(ctx, challenge.ID, challenge.CodeHash, s.now())
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "password reset failed", err)
	}
	if !consumed {
		return domainErr.New(domainErr.ErrOTPInvalid, "invalid or expired verification code", nil)
	}
	user, err := s.users.GetByID(ctx, challenge.UserID)
	if err != nil || user == nil {
		return domainErr.New(domainErr.ErrOTPInvalid, "invalid or expired verification code", nil)
	}
	hash, err := s.auth.HashPassword(newPassword)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to hash password", err)
	}
	user.PasswordHash = hash
	if err := s.users.Update(ctx, user); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to update password", err)
	}
	if s.sessions != nil {
		_ = s.sessions.RevokeAllByUser(ctx, user.ID, s.now())
	}
	s.record(ctx, AuditPasswordReset, "SUCCESS", user.ID)
	return nil
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func randomOpaque(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func hashOpaque(secret, value string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(secret))
	_, _ = h.Write([]byte(":"))
	_, _ = h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *SecurityService) createSession(ctx context.Context, userID, deviceID uuid.UUID, authTime time.Time) (*TokenPair, error) {
	sessionID, err := uuid.NewRandom()
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to create session", err)
	}
	familyID, err := uuid.NewRandom()
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to create session", err)
	}
	refreshRandom, err := randomOpaque(32)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to create session", err)
	}
	refresh := sessionID.String() + "." + refreshRandom
	now := s.now()
	session := &iamModel.Session{ID: sessionID, UserID: userID, DeviceID: deviceID, FamilyID: familyID, RefreshTokenHash: hashOpaque(s.secret, refresh), RefreshTokenExpiresAt: now.Add(time.Duration(s.cfg.RefreshTokenDays) * 24 * time.Hour), CreatedAt: now, LastUsedAt: now, AuthenticationTime: authTime}
	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to create session", err)
	}
	access, err := s.issueAccessToken(ctx, session, authTime)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh, SessionID: sessionID, DeviceID: deviceID}, nil
}

func (s *SecurityService) issueAccessToken(ctx context.Context, session *iamModel.Session, authTime time.Time) (string, error) {
	user, err := s.users.GetByID(ctx, session.UserID)
	if err != nil {
		return "", domainErr.New(domainErr.ErrInternal, "failed to issue access token", err)
	}
	return s.auth.GenerateToken(ctx, iamService.TokenClaims{UserID: user.ID, Email: user.Email, SessionID: session.ID.String(), DeviceID: session.DeviceID, TokenClass: string(authcontext.TokenClassBootstrap), AuthenticationMethods: []string{"otp"}, AuthenticationTime: authTime})
}

func (s *SecurityService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	parts := strings.SplitN(strings.TrimSpace(refreshToken), ".", 2)
	if len(parts) != 2 {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "invalid refresh token", nil)
	}
	sessionID, err := uuid.Parse(parts[0])
	if err != nil {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "invalid refresh token", nil)
	}
	session, err := s.sessions.Get(ctx, sessionID)
	if err != nil || session == nil {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "invalid refresh token", nil)
	}
	if s.devices != nil {
		device, getErr := s.devices.GetByID(ctx, session.DeviceID)
		if getErr != nil || device == nil || device.IsRevoked() {
			return nil, domainErr.New(domainErr.ErrDeviceRevoked, "device is no longer active", nil)
		}
	}
	if session.IsRevoked() || !s.now().Before(session.RefreshTokenExpiresAt) {
		return nil, domainErr.New(domainErr.ErrSessionRevoked, "session is no longer active", nil)
	}
	replacementRandom, err := randomOpaque(32)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to rotate refresh token", err)
	}
	replacement := sessionID.String() + "." + replacementRandom
	rotated, reused, err := s.sessions.Rotate(ctx, sessionID, hashOpaque(s.secret, refreshToken), hashOpaque(s.secret, replacement), s.now().Add(time.Duration(s.cfg.RefreshTokenDays)*24*time.Hour), s.now())
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to rotate refresh token", err)
	}
	if reused {
		_ = s.sessions.RevokeFamily(ctx, session.FamilyID, s.now())
		s.record(ctx, AuditRefreshReused, "FAILURE", session.UserID)
		return nil, domainErr.New(domainErr.ErrRefreshTokenReuse, "refresh token reuse detected", nil)
	}
	if rotated == nil {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "invalid refresh token", nil)
	}
	access, err := s.issueAccessToken(ctx, rotated, rotated.AuthenticationTime)
	if err != nil {
		return nil, err
	}
	s.record(ctx, AuditRefreshRotated, "SUCCESS", rotated.UserID)
	return &TokenPair{AccessToken: access, RefreshToken: replacement, SessionID: rotated.ID, DeviceID: rotated.DeviceID}, nil
}

func (s *SecurityService) ValidateAccess(ctx context.Context, claims *iamService.TokenClaims) error {
	if claims == nil || claims.UserID == uuid.Nil {
		return domainErr.New(domainErr.ErrUnauthorized, "invalid session", nil)
	}
	if claims.SessionID == "" {
		return nil
	}
	id, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return domainErr.New(domainErr.ErrUnauthorized, "invalid session", nil)
	}
	session, err := s.sessions.Get(ctx, id)
	if err != nil || session == nil || session.IsRevoked() || session.UserID != claims.UserID {
		return domainErr.New(domainErr.ErrSessionRevoked, "session is no longer active", nil)
	}
	if claims.DeviceID == uuid.Nil || session.DeviceID != claims.DeviceID {
		return domainErr.New(domainErr.ErrUnauthorized, "invalid session", nil)
	}
	device, err := s.devices.GetByID(ctx, claims.DeviceID)
	if err != nil || device == nil || device.IsRevoked() {
		return domainErr.New(domainErr.ErrDeviceRevoked, "device is no longer active", nil)
	}
	return nil
}

func (s *SecurityService) Logout(ctx context.Context, actor authcontext.ActorContext) error {
	if actor.SessionID == "" {
		return nil
	}
	id, err := uuid.Parse(actor.SessionID)
	if err != nil {
		return nil
	}
	if err := s.sessions.Revoke(ctx, id, s.now()); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to log out", err)
	}
	s.record(ctx, AuditLogout, "SUCCESS", actor.UserID)
	return nil
}
func (s *SecurityService) LogoutAll(ctx context.Context, actor authcontext.ActorContext) error {
	if err := s.RequireRecentAuthentication(actor); err != nil {
		s.record(ctx, AuditRecentAuthFailed, "FAILURE", actor.UserID)
		return err
	}
	if err := s.sessions.RevokeAllByUser(ctx, actor.UserID, s.now()); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to log out", err)
	}
	s.record(ctx, AuditLogoutAll, "SUCCESS", actor.UserID)
	return nil
}
func (s *SecurityService) ListDevices(ctx context.Context, actor authcontext.ActorContext) ([]*iamModel.Device, error) {
	if !actor.IsAuthenticated() {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "authentication required", nil)
	}
	return s.devices.ListByUser(ctx, actor.UserID)
}
func (s *SecurityService) RevokeDevice(ctx context.Context, actor authcontext.ActorContext, id uuid.UUID) error {
	if err := s.RequireRecentAuthentication(actor); err != nil {
		s.record(ctx, AuditRecentAuthFailed, "FAILURE", actor.UserID)
		return err
	}
	device, err := s.devices.GetByID(ctx, id)
	if err != nil || device == nil || device.UserID != actor.UserID {
		return domainErr.New(domainErr.ErrNotFound, "device not found", nil)
	}
	if err := s.devices.Revoke(ctx, id, actor.UserID, s.now()); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to revoke device", err)
	}
	sessions, err := s.sessions.ListByUser(ctx, actor.UserID)
	if err == nil {
		for _, session := range sessions {
			if session != nil && session.DeviceID == id {
				_ = s.sessions.Revoke(ctx, session.ID, s.now())
			}
		}
	}
	s.record(ctx, AuditDeviceRevoked, "SUCCESS", actor.UserID)
	return nil
}
func (s *SecurityService) RequireRecentAuthentication(actor authcontext.ActorContext) error {
	if actor.AuthenticationTime.IsZero() || s.now().Sub(actor.AuthenticationTime) > time.Duration(s.cfg.RecentAuthenticationMinutes)*time.Minute {
		return domainErr.New(domainErr.ErrRecentAuthenticationRequired, "recent authentication required", nil)
	}
	return nil
}

// MemoryRateLimiter is deterministic, bounded, and intended for tests or single-node development only.
type MemoryRateLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func NewMemoryRateLimiter() *MemoryRateLimiter {
	return &MemoryRateLimiter{hits: make(map[string][]time.Time)}
}
func (l *MemoryRateLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	list := l.hits[key]
	cutoff := now.Add(-window)
	kept := list[:0]
	for _, t := range list {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= limit {
		l.hits[key] = kept
		return false, nil
	}
	l.hits[key] = append(kept, now)
	return true, nil
}
