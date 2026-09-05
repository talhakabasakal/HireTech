package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration.
type Config struct {
	Environment string
	Server      ServerConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	JWT         JWTConfig
	Security    SecurityConfig
	Email       EmailConfig
	Kafka       KafkaConfig
	WebSocket   WebSocketConfig
	Log         LogConfig
	AI          AIConfig
}

const (
	EnvironmentDevelopment = "development"
	EnvironmentProduction  = "production"
	defaultJWTSecret       = "change-me-in-production"
)

// IsProduction reports whether production-only startup safeguards apply.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(strings.TrimSpace(c.Environment), EnvironmentProduction)
}

// ValidateForProduction rejects insecure defaults before any listener starts.
// Development and test environments intentionally keep their existing defaults.
func (c *Config) ValidateForProduction() error {
	if !c.IsProduction() {
		return nil
	}
	if c.JWT.Secret == defaultJWTSecret {
		return fmt.Errorf("JWT_SECRET must be explicitly configured in production")
	}
	if len(strings.TrimSpace(c.JWT.Secret)) < 32 {
		return fmt.Errorf("JWT_SECRET must contain at least 32 characters in production")
	}
	return nil
}

// EmailConfig controls transactional OTP delivery. Provider may be noop or smtp.
type EmailConfig struct {
	Provider   string
	SMTPHost   string
	SMTPPort   int
	Username   string
	Password   string
	From       string
	TLSMode    string
	TimeoutSec int
}

// WebSocketConfig holds real-time WebSocket settings.
type WebSocketConfig struct {
	Enabled         bool
	MaxConnections  int
	PingIntervalSec int
	ReadBufferSize  int
	WriteBufferSize int
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host               string
	Port               int
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	CORSAllowedOrigins []string
	MaxBodyBytes       int64
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	MaxConns int32
	MinConns int32
}

// DSN returns the PostgreSQL connection string with escaped credentials.
func (d DatabaseConfig) DSN() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(d.User, d.Password),
		Host:   fmt.Sprintf("%s:%d", d.Host, d.Port),
		Path:   "/" + d.DBName,
	}
	u.RawQuery = url.Values{"sslmode": {d.SSLMode}}.Encode()
	return u.String()
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// Addr returns the Redis address string.
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// JWTConfig holds JWT signing settings.
type JWTConfig struct {
	Secret             string
	Keys               map[string]string
	ActiveKeyID        string
	ExpirationHours    int
	AccessTokenMinutes int
	Issuer             string
}

// SecurityConfig controls OTP, session, device, and recent-authentication policy.
type SecurityConfig struct {
	OTPExpirationMinutes        int
	OTPMaxAttempts              int
	OTPRequestsPerHour          int
	OTPVerificationsPerHour     int
	AccessTokenMinutes          int
	RefreshTokenDays            int
	RecentAuthenticationMinutes int
}

// KafkaConfig holds Kafka connection and consumer settings.
type KafkaConfig struct {
	Brokers           []string
	GroupID           string
	Enabled           bool
	NumPartitions     int
	ReplicationFactor int
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level  string // debug, info, warn, error
	Format string // json, text
}

// AIConfig controls the provider-independent interviewer and evaluator endpoints.
type AIConfig struct {
	Enabled          bool
	RequestTimeout   time.Duration
	MaxResponseBytes int64
	Interviewer      AIModelConfig
	Evaluator        AIModelConfig
}

// AIModelConfig references an OpenAI-compatible inference endpoint.
type AIModelConfig struct {
	ProviderLabel string
	BaseURL       string
	APIKey        string
	ModelID       string
	ModelVersion  string
	MaxTokens     int
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Environment: envOrDefault("APP_ENV", EnvironmentDevelopment),
		Server: ServerConfig{
			Host:               envOrDefault("SERVER_HOST", "0.0.0.0"),
			Port:               envOrDefaultInt("SERVER_PORT", 8080),
			ReadTimeout:        time.Duration(envOrDefaultInt("SERVER_READ_TIMEOUT_SECONDS", 15)) * time.Second,
			WriteTimeout:       time.Duration(envOrDefaultInt("SERVER_WRITE_TIMEOUT_SECONDS", 15)) * time.Second,
			IdleTimeout:        time.Duration(envOrDefaultInt("SERVER_IDLE_TIMEOUT_SECONDS", 60)) * time.Second,
			CORSAllowedOrigins: envOrDefaultSlice("CORS_ALLOWED_ORIGINS", nil),
			MaxBodyBytes:       envOrDefaultInt64("MAX_BODY_BYTES", 1<<20),
		},
		Database: DatabaseConfig{
			Host:     envOrDefault("DB_HOST", "localhost"),
			Port:     envOrDefaultInt("DB_PORT", 5432),
			User:     envOrDefault("DB_USER", "masterfabric"),
			Password: envOrDefault("DB_PASSWORD", "masterfabric"),
			DBName:   envOrDefault("DB_NAME", "masterfabric"),
			SSLMode:  envOrDefault("DB_SSLMODE", "disable"),
			MaxConns: envOrDefaultInt32("DB_MAX_CONNS", 25),
			MinConns: envOrDefaultInt32("DB_MIN_CONNS", 5),
		},
		Redis: RedisConfig{
			Host:     envOrDefault("REDIS_HOST", "localhost"),
			Port:     envOrDefaultInt("REDIS_PORT", 6379),
			Password: envOrDefault("REDIS_PASSWORD", ""),
			DB:       envOrDefaultInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:             envOrDefault("JWT_SECRET", defaultJWTSecret),
			Keys:               parseKeyRing(os.Getenv("JWT_KEYS")),
			ActiveKeyID:        envOrDefault("JWT_ACTIVE_KID", "legacy"),
			ExpirationHours:    envOrDefaultInt("JWT_EXPIRATION_HOURS", 24),
			AccessTokenMinutes: envOrDefaultInt("JWT_ACCESS_TOKEN_MINUTES", 15),
			Issuer:             envOrDefault("JWT_ISSUER", "masterfabric"),
		},
		Security: SecurityConfig{
			OTPExpirationMinutes:        envOrDefaultInt("OTP_EXPIRATION_MINUTES", 10),
			OTPMaxAttempts:              envOrDefaultInt("OTP_MAX_ATTEMPTS", 5),
			OTPRequestsPerHour:          envOrDefaultInt("OTP_REQUESTS_PER_HOUR", 5),
			OTPVerificationsPerHour:     envOrDefaultInt("OTP_VERIFICATIONS_PER_HOUR", 10),
			AccessTokenMinutes:          envOrDefaultInt("JWT_ACCESS_TOKEN_MINUTES", 15),
			RefreshTokenDays:            envOrDefaultInt("REFRESH_TOKEN_DAYS", 30),
			RecentAuthenticationMinutes: envOrDefaultInt("RECENT_AUTH_MINUTES", 10),
		},
		Email: EmailConfig{
			// Do not silently accept verification requests without delivery. Local
			// development is wired to Mailpit by dev.sh; noop must be explicit.
			Provider:   strings.ToLower(envOrDefault("EMAIL_PROVIDER", "smtp")),
			SMTPHost:   envOrDefault("SMTP_HOST", "127.0.0.1"),
			SMTPPort:   envOrDefaultInt("SMTP_PORT", 1025),
			Username:   envOrDefault("SMTP_USERNAME", ""),
			Password:   envOrDefault("SMTP_PASSWORD", ""),
			From:       envOrDefault("EMAIL_FROM", "HireTech <noreply@hiretech.com>"),
			TLSMode:    strings.ToLower(envOrDefault("SMTP_TLS_MODE", "none")),
			TimeoutSec: envOrDefaultInt("SMTP_TIMEOUT_SECONDS", 10),
		},
		Kafka: KafkaConfig{
			Brokers:           envOrDefaultSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
			GroupID:           envOrDefault("KAFKA_GROUP_ID", "masterfabric-go"),
			Enabled:           envOrDefault("KAFKA_ENABLED", "false") == "true",
			NumPartitions:     envOrDefaultInt("KAFKA_NUM_PARTITIONS", 3),
			ReplicationFactor: envOrDefaultInt("KAFKA_REPLICATION_FACTOR", 1),
		},
		WebSocket: WebSocketConfig{
			Enabled:         envOrDefault("WS_ENABLED", "true") == "true",
			MaxConnections:  envOrDefaultInt("WS_MAX_CONNECTIONS", 1000),
			PingIntervalSec: envOrDefaultInt("WS_PING_INTERVAL_SECONDS", 30),
			ReadBufferSize:  envOrDefaultInt("WS_READ_BUFFER_SIZE", 1024),
			WriteBufferSize: envOrDefaultInt("WS_WRITE_BUFFER_SIZE", 1024),
		},
		Log: LogConfig{
			Level:  envOrDefault("LOG_LEVEL", "info"),
			Format: envOrDefault("LOG_FORMAT", "json"),
		},
		AI: AIConfig{
			Enabled:          envOrDefault("AI_ENABLED", "false") == "true",
			RequestTimeout:   time.Duration(envOrDefaultInt("AI_REQUEST_TIMEOUT_SECONDS", 30)) * time.Second,
			MaxResponseBytes: envOrDefaultInt64("AI_MAX_RESPONSE_BYTES", 1<<20),
			Interviewer: AIModelConfig{
				ProviderLabel: envOrDefault("AI_INTERVIEWER_PROVIDER_LABEL", "interviewer"),
				BaseURL:       envOrDefault("AI_INTERVIEWER_BASE_URL", "http://localhost:8001/v1"),
				APIKey:        envOrDefault("AI_INTERVIEWER_API_KEY", ""),
				ModelID:       envOrDefault("AI_INTERVIEWER_MODEL_ID", "microsoft/Phi-4-mini-instruct"),
				ModelVersion:  envOrDefault("AI_INTERVIEWER_MODEL_VERSION", "qlora-smoketest-v1"),
				MaxTokens:     envOrDefaultInt("AI_INTERVIEWER_MAX_TOKENS", 1024),
			},
			Evaluator: AIModelConfig{
				ProviderLabel: envOrDefault("AI_EVALUATOR_PROVIDER_LABEL", "evaluator"),
				BaseURL:       envOrDefault("AI_EVALUATOR_BASE_URL", "http://localhost:8002/v1"),
				APIKey:        envOrDefault("AI_EVALUATOR_API_KEY", ""),
				ModelID:       envOrDefault("AI_EVALUATOR_MODEL_ID", "Qwen/Qwen3-4B"),
				ModelVersion:  envOrDefault("AI_EVALUATOR_MODEL_VERSION", "qlora-smoketest-v1"),
				MaxTokens:     envOrDefaultInt("AI_EVALUATOR_MAX_TOKENS", 2048),
			},
		},
	}
}

func parseKeyRing(raw string) map[string]string {
	result := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func envOrDefaultInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func envOrDefaultInt32(key string, defaultVal int32) int32 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			return int32(n)
		}
	}
	return defaultVal
}

func envOrDefaultInt64(key string, defaultVal int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return defaultVal
}

func envOrDefaultSlice(key string, defaultVal []string) []string {
	if val := os.Getenv(key); val != "" {
		parts := strings.Split(val, ",")
		var result []string
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultVal
}
