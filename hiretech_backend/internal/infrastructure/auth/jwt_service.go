package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"golang.org/x/crypto/bcrypt"
)

// JWTService implements service.AuthService using JWT and bcrypt.
type JWTService struct {
	keys       map[string][]byte
	activeKey  string
	algorithm  string
	privateKey *rsa.PrivateKey
	publicKeys map[string]*rsa.PublicKey
	initErr    error
	expiration time.Duration
	issuer     string
}

// NewJWTService creates a new JWTService from config.
func NewJWTService(cfg config.JWTConfig) *JWTService {
	algorithm := strings.ToUpper(strings.TrimSpace(cfg.Algorithm))
	if algorithm == "" {
		algorithm = "HS256"
	}
	keys := make(map[string][]byte)
	for id, secret := range cfg.Keys {
		if id != "" && secret != "" {
			keys[id] = []byte(secret)
		}
	}
	if algorithm == "HS256" && cfg.Secret != "" {
		keys["legacy"] = []byte(cfg.Secret)
	}
	activeKey := cfg.ActiveKeyID
	if algorithm != "RS256" && (activeKey == "" || len(keys[activeKey]) == 0) {
		activeKey = "legacy"
	}
	service := &JWTService{
		keys: keys, activeKey: activeKey, algorithm: algorithm,
		expiration: func() time.Duration {
			if cfg.AccessTokenMinutes > 0 {
				return time.Duration(cfg.AccessTokenMinutes) * time.Minute
			}
			return time.Duration(cfg.ExpirationHours) * time.Hour
		}(),
		issuer: cfg.Issuer,
	}
	if service.algorithm == "RS256" {
		service.privateKey, service.initErr = parseRSAPrivateKey(cfg.PrivateKeyPEM)
		if service.initErr == nil {
			service.publicKeys, service.initErr = parseRSAPublicKeys(cfg.PublicKeys)
		}
	} else if service.algorithm != "HS256" {
		service.initErr = fmt.Errorf("unsupported JWT algorithm %q", service.algorithm)
	}
	return service
}

// ValidateConfiguration reports key parsing errors before the HTTP listener is
// started. It never includes key material in the returned error.
func (s *JWTService) ValidateConfiguration() error {
	if s == nil {
		return fmt.Errorf("JWT service is nil")
	}
	return s.initErr
}

func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("invalid RSA private key PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse RSA private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}
	return key, nil
}

func parseRSAPublicKeys(raw map[string]string) (map[string]*rsa.PublicKey, error) {
	result := make(map[string]*rsa.PublicKey, len(raw))
	for id, encoded := range raw {
		block, _ := pem.Decode([]byte(encoded))
		if block == nil {
			return nil, fmt.Errorf("invalid RSA public key PEM for kid %q", id)
		}
		if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
			publicKey, ok := key.(*rsa.PublicKey)
			if !ok {
				return nil, fmt.Errorf("public key for kid %q is not RSA", id)
			}
			result[id] = publicKey
			continue
		}
		publicKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse RSA public key for kid %q: %w", id, err)
		}
		result[id] = publicKey
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no RSA public keys configured")
	}
	return result, nil
}

// customClaims extends JWT standard claims with our domain data.
type customClaims struct {
	jwt.RegisteredClaims
	UserID                string           `json:"user_id"`
	Email                 string           `json:"email"`
	OrganizationID        string           `json:"organization_id,omitempty"`
	MembershipID          string           `json:"membership_id,omitempty"`
	SessionID             string           `json:"session_id,omitempty"`
	DeviceID              string           `json:"device_id,omitempty"`
	InterviewID           string           `json:"interview_id,omitempty"`
	TokenClass            string           `json:"token_class,omitempty"`
	AuthenticationTime    *jwt.NumericDate `json:"auth_time,omitempty"`
	Roles                 []string         `json:"roles,omitempty"`
	Permissions           []string         `json:"permissions,omitempty"`
	AuthenticationMethods []string         `json:"authentication_methods,omitempty"`
}

func (s *JWTService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func (s *JWTService) VerifyPassword(hashedPassword, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
		return domainErr.New(domainErr.ErrUnauthorized, "invalid credentials", nil)
	}
	return nil
}

func (s *JWTService) GenerateToken(_ context.Context, claims service.TokenClaims) (string, error) {
	if err := s.ValidateConfiguration(); err != nil {
		return "", err
	}
	now := time.Now().UTC()
	authTime := claims.AuthenticationTime
	if authTime.IsZero() {
		authTime = now
	}
	c := customClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Audience:  []string{"hiretech-graphql"},
			Subject:   claims.UserID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expiration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
		UserID:                claims.UserID.String(),
		Email:                 claims.Email,
		OrganizationID:        claims.OrganizationID.String(),
		MembershipID:          claims.MembershipID.String(),
		SessionID:             claims.SessionID,
		DeviceID:              claims.DeviceID.String(),
		InterviewID:           claims.InterviewID.String(),
		TokenClass:            claims.TokenClass,
		Roles:                 claims.Roles,
		Permissions:           claims.Permissions,
		AuthenticationMethods: claims.AuthenticationMethods,
		AuthenticationTime:    jwt.NewNumericDate(authTime),
	}

	var method jwt.SigningMethod = jwt.SigningMethodHS256
	var signingKey interface{} = s.keys[s.activeKey]
	if s.algorithm == "RS256" {
		method = jwt.SigningMethodRS256
		signingKey = s.privateKey
	}
	token := jwt.NewWithClaims(method, c)
	token.Header["kid"] = s.activeKey
	signed, err := token.SignedString(signingKey)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func containsAudience(audience []string, expected string) bool {
	for _, value := range audience {
		if value == expected {
			return true
		}
	}
	return false
}

func firstAudience(audience []string) string {
	if len(audience) == 0 {
		return ""
	}
	return audience[0]
}

func (s *JWTService) ValidateToken(_ context.Context, tokenStr string) (*service.TokenClaims, error) {
	if err := s.ValidateConfiguration(); err != nil {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "invalid token configuration", err)
	}
	token, err := jwt.ParseWithClaims(tokenStr, &customClaims{}, func(t *jwt.Token) (interface{}, error) {
		keyID, _ := t.Header["kid"].(string)
		if s.algorithm == "RS256" {
			if t.Method != jwt.SigningMethodRS256 {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			key, ok := s.publicKeys[keyID]
			if !ok {
				return nil, fmt.Errorf("unknown signing key")
			}
			return key, nil
		}
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		if keyID == "" {
			keyID = "legacy"
		}
		key, ok := s.keys[keyID]
		if !ok {
			return nil, fmt.Errorf("unknown signing key")
		}
		return key, nil
	})
	if err != nil {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "invalid token", err)
	}

	claims, ok := token.Claims.(*customClaims)
	if !ok || !token.Valid {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "invalid token claims", nil)
	}

	userID, _ := uuid.Parse(claims.UserID)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	membershipID, _ := uuid.Parse(claims.MembershipID)
	deviceID, _ := uuid.Parse(claims.DeviceID)
	interviewID, _ := uuid.Parse(claims.InterviewID)
	authTime := time.Time{}
	if claims.AuthenticationTime != nil {
		authTime = claims.AuthenticationTime.Time.UTC()
	}
	if claims.Issuer != s.issuer || !containsAudience(claims.Audience, "hiretech-graphql") {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "invalid token issuer", nil)
	}
	if len(claims.Audience) == 0 {
		claims.Audience = nil
	}

	return &service.TokenClaims{
		UserID:                userID,
		Email:                 claims.Email,
		OrganizationID:        orgID,
		MembershipID:          membershipID,
		SessionID:             claims.SessionID,
		DeviceID:              deviceID,
		InterviewID:           interviewID,
		Audience:              firstAudience(claims.Audience),
		Issuer:                claims.Issuer,
		TokenClass:            claims.TokenClass,
		Roles:                 claims.Roles,
		Permissions:           claims.Permissions,
		AuthenticationMethods: claims.AuthenticationMethods,
		AuthenticationTime:    authTime,
	}, nil
}
