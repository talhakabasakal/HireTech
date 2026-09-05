package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestJWTService() *JWTService {
	return NewJWTService(config.JWTConfig{
		Secret:          "test-secret-key-for-testing-only",
		ExpirationHours: 1,
		Issuer:          "test",
	})
}

func newRSATestJWTService(t *testing.T) *JWTService {
	t.Helper()
	privatePEM, publicPEM := generateRSAKeyPair(t)
	return NewJWTService(config.JWTConfig{
		Algorithm:     "RS256",
		PrivateKeyPEM: privatePEM,
		PublicKeys: map[string]string{
			"rsa-test": publicPEM,
		},
		ActiveKeyID:        "rsa-test",
		AccessTokenMinutes: 15,
		Issuer:             "test",
	})
}

func generateRSAKeyPair(t *testing.T) (string, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes})), string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyBytes}))
}

func TestJWTService_HashAndVerifyPassword(t *testing.T) {
	svc := newTestJWTService()

	hash, err := svc.HashPassword("mypassword123")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "mypassword123", hash)

	// Verify correct password
	err = svc.VerifyPassword(hash, "mypassword123")
	assert.NoError(t, err)

	// Verify wrong password
	err = svc.VerifyPassword(hash, "wrongpassword")
	assert.Error(t, err)
}

func TestJWTService_GenerateAndValidateToken(t *testing.T) {
	svc := newTestJWTService()
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()
	interviewID := uuid.New()
	deviceID := uuid.New()
	authTime := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)

	claims := service.TokenClaims{
		UserID:             userID,
		Email:              "test@example.com",
		OrganizationID:     orgID,
		InterviewID:        interviewID,
		DeviceID:           deviceID,
		SessionID:          uuid.NewString(),
		TokenClass:         "candidate-interview",
		AuthenticationTime: authTime,
		Roles:              []string{"org.admin"},
		Permissions:        []string{"org.manage_users"},
	}

	token, err := svc.GenerateToken(ctx, claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate token
	parsed, err := svc.ValidateToken(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, userID, parsed.UserID)
	assert.Equal(t, "test@example.com", parsed.Email)
	assert.Equal(t, orgID, parsed.OrganizationID)
	assert.Equal(t, interviewID, parsed.InterviewID)
	assert.Equal(t, deviceID, parsed.DeviceID)
	assert.Equal(t, claims.SessionID, parsed.SessionID)
	assert.Equal(t, "candidate-interview", parsed.TokenClass)
	assert.Equal(t, authTime, parsed.AuthenticationTime)
	assert.Equal(t, []string{"org.admin"}, parsed.Roles)
	assert.Equal(t, []string{"org.manage_users"}, parsed.Permissions)
}

func TestJWTService_InvalidToken(t *testing.T) {
	svc := newTestJWTService()
	ctx := context.Background()

	_, err := svc.ValidateToken(ctx, "invalid-token-string")
	assert.Error(t, err)
}

func TestJWTService_RSA256GenerateAndValidateToken(t *testing.T) {
	svc := newRSATestJWTService(t)
	ctx := context.Background()

	token, err := svc.GenerateToken(ctx, service.TokenClaims{UserID: uuid.New(), Email: "rsa@example.com"})
	require.NoError(t, err)
	parsedToken, parseErr := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return &rsa.PublicKey{}, nil
	})
	require.Error(t, parseErr)
	require.NotNil(t, parsedToken)
	assert.Equal(t, "RS256", parsedToken.Method.Alg())

	claims, err := svc.ValidateToken(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, "rsa@example.com", claims.Email)
}

func TestJWTService_RSA256KeyRotationUsesKIDOverlap(t *testing.T) {
	oldPrivate, oldPublic := generateRSAKeyPair(t)
	newPrivate, newPublic := generateRSAKeyPair(t)
	oldService := NewJWTService(config.JWTConfig{
		Algorithm:          "RS256",
		PrivateKeyPEM:      oldPrivate,
		PublicKeys:         map[string]string{"old": oldPublic},
		ActiveKeyID:        "old",
		AccessTokenMinutes: 15,
		Issuer:             "test",
	})
	rotatedService := NewJWTService(config.JWTConfig{
		Algorithm:          "RS256",
		PrivateKeyPEM:      newPrivate,
		PublicKeys:         map[string]string{"old": oldPublic, "new": newPublic},
		ActiveKeyID:        "new",
		AccessTokenMinutes: 15,
		Issuer:             "test",
	})

	oldToken, err := oldService.GenerateToken(context.Background(), service.TokenClaims{UserID: uuid.New()})
	require.NoError(t, err)
	newToken, err := rotatedService.GenerateToken(context.Background(), service.TokenClaims{UserID: uuid.New()})
	require.NoError(t, err)
	assert.NoError(t, func() error { _, err := rotatedService.ValidateToken(context.Background(), oldToken); return err }())
	assert.NoError(t, func() error { _, err := rotatedService.ValidateToken(context.Background(), newToken); return err }())

	retiredService := NewJWTService(config.JWTConfig{
		Algorithm:          "RS256",
		PrivateKeyPEM:      newPrivate,
		PublicKeys:         map[string]string{"new": newPublic},
		ActiveKeyID:        "new",
		AccessTokenMinutes: 15,
		Issuer:             "test",
	})
	_, err = retiredService.ValidateToken(context.Background(), oldToken)
	assert.Error(t, err)
}

func TestJWTService_RSA256RejectsHMACToken(t *testing.T) {
	hmacService := newTestJWTService()
	rsaService := newRSATestJWTService(t)
	token, err := hmacService.GenerateToken(context.Background(), service.TokenClaims{UserID: uuid.New()})
	require.NoError(t, err)

	_, err = rsaService.ValidateToken(context.Background(), token)
	assert.Error(t, err)
}

func TestJWTService_RSA256RejectsInvalidKeyConfiguration(t *testing.T) {
	service := NewJWTService(config.JWTConfig{
		Algorithm:     "RS256",
		PrivateKeyPEM: "not-a-key",
		PublicKeys:    map[string]string{"rsa-test": "not-a-key"},
		ActiveKeyID:   "rsa-test",
	})

	assert.Error(t, service.ValidateConfiguration())
}

func TestJWTService_WrongSecret(t *testing.T) {
	svc1 := newTestJWTService()
	svc2 := NewJWTService(config.JWTConfig{
		Secret:          "different-secret",
		ExpirationHours: 1,
		Issuer:          "test",
	})
	ctx := context.Background()

	token, err := svc1.GenerateToken(ctx, service.TokenClaims{
		UserID: uuid.New(),
		Email:  "test@example.com",
	})
	require.NoError(t, err)

	_, err = svc2.ValidateToken(ctx, token)
	assert.Error(t, err)
}

func TestJWTServiceKeyRotationAcceptsRetainedKeysAndSignsWithActiveKey(t *testing.T) {
	ctx := context.Background()
	old := NewJWTService(config.JWTConfig{Secret: "old-secret", ActiveKeyID: "old", Keys: map[string]string{"old": "old-secret", "new": "new-secret"}, Issuer: "test", AccessTokenMinutes: 15})
	newRing := NewJWTService(config.JWTConfig{Secret: "old-secret", ActiveKeyID: "new", Keys: map[string]string{"old": "old-secret", "new": "new-secret"}, Issuer: "test", AccessTokenMinutes: 15})
	claims := service.TokenClaims{UserID: uuid.New(), Email: "rotate@example.com", AuthenticationMethods: []string{"otp"}}

	oldToken, err := old.GenerateToken(ctx, claims)
	require.NoError(t, err)
	_, err = newRing.ValidateToken(ctx, oldToken)
	require.NoError(t, err)

	newToken, err := newRing.GenerateToken(ctx, claims)
	require.NoError(t, err)
	_, err = old.ValidateToken(ctx, newToken)
	require.NoError(t, err)

	rotatedOnly := NewJWTService(config.JWTConfig{ActiveKeyID: "new", Keys: map[string]string{"new": "new-secret"}, Issuer: "test", AccessTokenMinutes: 15})
	_, err = rotatedOnly.ValidateToken(ctx, oldToken)
	assert.Error(t, err)
	parsed, err := newRing.ValidateToken(ctx, newToken)
	require.NoError(t, err)
	assert.Equal(t, []string{"otp"}, parsed.AuthenticationMethods)
}
