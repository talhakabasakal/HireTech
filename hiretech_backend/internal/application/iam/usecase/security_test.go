package usecase

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	infraAuth "github.com/masterfabric-go/masterfabric/internal/infrastructure/auth"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/stretchr/testify/require"
)

type memoryOTP struct {
	mu      sync.Mutex
	current *model.OTPChallenge
}

func (r *memoryOTP) InvalidateActive(context.Context, string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current != nil {
		now := time.Now()
		r.current.UsedAt = &now
	}
	return nil
}
func (r *memoryOTP) Create(_ context.Context, c *model.OTPChallenge) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *c
	r.current = &cp
	return nil
}
func (r *memoryOTP) GetActiveByEmail(_ context.Context, email string, _ time.Time) (*model.OTPChallenge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil || r.current.Email != email {
		return nil, errors.New("not found")
	}
	cp := *r.current
	return &cp, nil
}
func (r *memoryOTP) IncrementAttempts(_ context.Context, id uuid.UUID, max int) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil || r.current.ID != id {
		return max, nil
	}
	if r.current.Attempts < max {
		r.current.Attempts++
	}
	return r.current.Attempts, nil
}
func (r *memoryOTP) Consume(_ context.Context, id uuid.UUID, hash string, now time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil || r.current.ID != id || r.current.CodeHash != hash || r.current.UsedAt != nil || !now.Before(r.current.ExpiresAt) || r.current.Attempts >= r.current.MaxAttempts {
		return false, nil
	}
	r.current.UsedAt = &now
	return true, nil
}

type memoryUsers struct{ user *model.User }

func (r memoryUsers) Create(context.Context, *model.User) error { return nil }
func (r memoryUsers) GetByID(context.Context, uuid.UUID) (*model.User, error) {
	if r.user == nil {
		return nil, errors.New("not found")
	}
	return r.user, nil
}
func (r memoryUsers) GetByEmail(_ context.Context, email string) (*model.User, error) {
	if r.user != nil && r.user.Email == email {
		return r.user, nil
	}
	return nil, errors.New("not found")
}
func (r memoryUsers) Update(context.Context, *model.User) error                  { return nil }
func (r memoryUsers) Delete(context.Context, uuid.UUID) error                    { return nil }
func (r memoryUsers) List(context.Context, int, int) ([]*model.User, int, error) { return nil, 0, nil }

type memoryDevices struct {
	mu      sync.Mutex
	devices map[uuid.UUID]*model.Device
}

func newMemoryDevices() *memoryDevices { return &memoryDevices{devices: map[uuid.UUID]*model.Device{}} }
func (r *memoryDevices) Create(_ context.Context, d *model.Device) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *d
	r.devices[d.ID] = &cp
	return nil
}
func (r *memoryDevices) GetByID(_ context.Context, id uuid.UUID) (*model.Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := r.devices[id]
	if d == nil {
		return nil, errors.New("not found")
	}
	cp := *d
	return &cp, nil
}
func (r *memoryDevices) ListByUser(_ context.Context, user uuid.UUID) ([]*model.Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*model.Device
	for _, d := range r.devices {
		if d.UserID == user {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (r *memoryDevices) Revoke(_ context.Context, id, user uuid.UUID, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := r.devices[id]
	if d == nil || d.UserID != user {
		return errors.New("not found")
	}
	d.State = model.DeviceStateRevoked
	d.RevokedAt = &now
	return nil
}

type memorySessions struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]*model.Session
	tokens   map[string]bool
}

func newMemorySessions() *memorySessions {
	return &memorySessions{sessions: map[uuid.UUID]*model.Session{}, tokens: map[string]bool{}}
}
func (r *memorySessions) Create(_ context.Context, s *model.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *s
	r.sessions[s.ID] = &cp
	r.tokens[s.RefreshTokenHash] = false
	return nil
}
func (r *memorySessions) Get(_ context.Context, id uuid.UUID) (*model.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.sessions[id]
	if s == nil {
		return nil, errors.New("not found")
	}
	cp := *s
	return &cp, nil
}
func (r *memorySessions) Rotate(_ context.Context, id uuid.UUID, presented, replacement string, expires, now time.Time) (*model.Session, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.sessions[id]
	if s == nil {
		return nil, false, nil
	}
	used, ok := r.tokens[presented]
	if !ok {
		return nil, false, nil
	}
	if used {
		return s, true, nil
	}
	if s.RevokedAt != nil {
		return s, false, nil
	}
	r.tokens[presented] = true
	r.tokens[replacement] = false
	s.RefreshTokenHash = replacement
	s.RefreshTokenExpiresAt = expires
	s.LastUsedAt = now
	cp := *s
	return &cp, false, nil
}
func (r *memorySessions) Revoke(_ context.Context, id uuid.UUID, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s := r.sessions[id]; s != nil {
		s.RevokedAt = &now
	}
	return nil
}
func (r *memorySessions) RevokeFamily(_ context.Context, family uuid.UUID, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.sessions {
		if s.FamilyID == family {
			s.RevokedAt = &now
		}
	}
	return nil
}
func (r *memorySessions) RevokeAllByUser(_ context.Context, user uuid.UUID, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.sessions {
		if s.UserID == user {
			s.RevokedAt = &now
		}
	}
	return nil
}
func (r *memorySessions) ListByUser(_ context.Context, user uuid.UUID) ([]*model.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*model.Session
	for _, s := range r.sessions {
		if s.UserID == user {
			cp := *s
			out = append(out, &cp)
		}
	}
	return out, nil
}

type auditCapture struct {
	mu     sync.Mutex
	events []SecurityAuditEvent
}

func (a *auditCapture) Record(_ context.Context, e SecurityAuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, e)
	return nil
}

type testSecurity struct {
	svc      *SecurityService
	email    *FakeEmailSender
	audit    *auditCapture
	sessions *memorySessions
	devices  *memoryDevices
	user     *model.User
}

func newTestSecurity(t *testing.T) *testSecurity {
	t.Helper()
	user := &model.User{ID: uuid.New(), Email: "person@example.com", Status: model.UserStatusActive}
	email := &FakeEmailSender{}
	audit := &auditCapture{}
	sessions := newMemorySessions()
	devices := newMemoryDevices()
	jwt := infraAuth.NewJWTService(config.JWTConfig{Secret: "security-test-secret", ExpirationHours: 1, AccessTokenMinutes: 15, Issuer: "test"})
	svc := NewSecurityService(&memoryOTP{}, sessions, devices, memoryUsers{user: user}, jwt, email, NewMemoryRateLimiter(), audit, "hash-secret", config.SecurityConfig{OTPExpirationMinutes: 10, OTPMaxAttempts: 3, OTPRequestsPerHour: 5, OTPVerificationsPerHour: 10, AccessTokenMinutes: 15, RefreshTokenDays: 30, RecentAuthenticationMinutes: 10})
	return &testSecurity{svc: svc, email: email, audit: audit, sessions: sessions, devices: devices, user: user}
}

func TestOTPGenerationAndHashing(t *testing.T) {
	for i := 0; i < 20; i++ {
		code, err := service.GenerateOTP()
		require.NoError(t, err)
		require.Regexp(t, regexp.MustCompile(`^[0-9]{6}$`), code)
		require.NotEqual(t, code, service.HashOTP("secret", code))
		require.Equal(t, service.HashOTP("secret", code), service.HashOTP("secret", code))
	}
}
func TestSecurityOTPReplayAttemptsRateAndUnknownAccount(t *testing.T) {
	s := newTestSecurity(t)
	ctx := context.Background()
	require.NoError(t, s.svc.RequestOTP(ctx, s.user.Email))
	pair, err := s.svc.VerifyOTP(ctx, s.user.Email, s.email.LastCode(), "laptop")
	require.NoError(t, err)
	require.NotEmpty(t, pair.RefreshToken)
	_, err = s.svc.VerifyOTP(ctx, s.user.Email, s.email.LastCode(), "laptop")
	require.ErrorIs(t, err, domainErr.ErrOTPInvalid)
	require.NoError(t, s.svc.RequestOTP(ctx, s.user.Email))
	for i := 0; i < 3; i++ {
		_, err = s.svc.VerifyOTP(ctx, s.user.Email, "000000", "laptop")
	}
	require.ErrorIs(t, err, domainErr.ErrOTPAttemptsExceeded)
	require.NoError(t, s.svc.RequestOTP(ctx, "unknown@example.com"))
	require.Len(t, s.email.Codes, 2)
	_, err = s.svc.VerifyOTP(ctx, "unknown@example.com", "000000", "new")
	require.ErrorIs(t, err, domainErr.ErrOTPInvalid)
}
func TestSecurityOTPExpirationAndRateLimit(t *testing.T) {
	s := newTestSecurity(t)
	s.svc.cfg.OTPExpirationMinutes = 1
	s.svc.now = func() time.Time { return time.Now().Add(-2 * time.Minute) }
	require.NoError(t, s.svc.RequestOTP(context.Background(), s.user.Email))
	s.svc.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	_, err := s.svc.VerifyOTP(context.Background(), s.user.Email, s.email.LastCode(), "x")
	require.ErrorIs(t, err, domainErr.ErrOTPExpired)
	s.svc = NewSecurityService(&memoryOTP{}, s.sessions, s.devices, memoryUsers{user: s.user}, s.svc.auth, s.email, NewMemoryRateLimiter(), s.audit, "hash-secret", config.SecurityConfig{OTPRequestsPerHour: 1, OTPMaxAttempts: 3})
	require.NoError(t, s.svc.RequestOTP(context.Background(), s.user.Email))
	require.ErrorIs(t, s.svc.RequestOTP(context.Background(), s.user.Email), domainErr.ErrRateLimited)
}
func TestRefreshRotationReuseFamilyLogoutAndDeviceRevocation(t *testing.T) {
	s := newTestSecurity(t)
	ctx := context.Background()
	require.NoError(t, s.svc.RequestOTP(ctx, s.user.Email))
	pair, err := s.svc.VerifyOTP(ctx, s.user.Email, s.email.LastCode(), "phone")
	require.NoError(t, err)
	rotated, err := s.svc.Refresh(ctx, pair.RefreshToken)
	require.NoError(t, err)
	require.NotEqual(t, pair.RefreshToken, rotated.RefreshToken)
	_, err = s.svc.Refresh(ctx, pair.RefreshToken)
	require.ErrorIs(t, err, domainErr.ErrRefreshTokenReuse)
	session, _ := s.sessions.Get(ctx, pair.SessionID)
	require.NotNil(t, session.RevokedAt)
	actor := authcontext.ActorContext{UserID: s.user.ID, SessionID: rotated.SessionID.String(), DeviceID: rotated.DeviceID, AuthenticationTime: time.Now()}
	require.NoError(t, s.svc.Logout(ctx, actor))
	session, _ = s.sessions.Get(ctx, rotated.SessionID)
	require.NotNil(t, session.RevokedAt)
	require.NoError(t, s.svc.RequestOTP(ctx, s.user.Email))
	newPair, err := s.svc.VerifyOTP(ctx, s.user.Email, s.email.LastCode(), "tablet")
	require.NoError(t, err)
	actor = authcontext.ActorContext{UserID: s.user.ID, AuthenticationTime: time.Now()}
	require.NoError(t, s.svc.RevokeDevice(ctx, actor, newPair.DeviceID))
	_, err = s.svc.Refresh(ctx, newPair.RefreshToken)
	require.ErrorIs(t, err, domainErr.ErrDeviceRevoked)
}
func TestRecentAuthenticationPolicyAndAuditRedaction(t *testing.T) {
	s := newTestSecurity(t)
	actor := authcontext.ActorContext{UserID: s.user.ID, AuthenticationTime: time.Now().Add(-time.Hour)}
	require.ErrorIs(t, s.svc.RequireRecentAuthentication(actor), domainErr.ErrRecentAuthenticationRequired)
	actor.AuthenticationTime = time.Now()
	require.NoError(t, s.svc.RequireRecentAuthentication(actor))
	require.NoError(t, s.svc.RequestOTP(context.Background(), s.user.Email))
	_, err := s.svc.VerifyOTP(context.Background(), s.user.Email, "bad", "x")
	require.Error(t, err)
	s.audit.mu.Lock()
	defer s.audit.mu.Unlock()
	for _, e := range s.audit.events {
		require.NotContains(t, e.EventType, "000000")
		require.NotContains(t, e.Result, "secret")
	}
}
