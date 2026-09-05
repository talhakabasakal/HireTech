package auth

import (
	"context"
	"fmt"
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
	expiration time.Duration
	issuer     string
}

// NewJWTService creates a new JWTService from config.
func NewJWTService(cfg config.JWTConfig) *JWTService {
	keys := make(map[string][]byte)
	for id, secret := range cfg.Keys {
		if id != "" && secret != "" {
			keys[id] = []byte(secret)
		}
	}
	if cfg.Secret != "" {
		keys["legacy"] = []byte(cfg.Secret)
	}
	activeKey := cfg.ActiveKeyID
	if activeKey == "" || len(keys[activeKey]) == 0 {
		activeKey = "legacy"
	}
	return &JWTService{
		keys: keys, activeKey: activeKey,
		expiration: func() time.Duration {
			if cfg.AccessTokenMinutes > 0 {
				return time.Duration(cfg.AccessTokenMinutes) * time.Minute
			}
			return time.Duration(cfg.ExpirationHours) * time.Hour
		}(),
		issuer: cfg.Issuer,
	}
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

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	token.Header["kid"] = s.activeKey
	signed, err := token.SignedString(s.keys[s.activeKey])
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
	token, err := jwt.ParseWithClaims(tokenStr, &customClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		keyID, _ := t.Header["kid"].(string)
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
