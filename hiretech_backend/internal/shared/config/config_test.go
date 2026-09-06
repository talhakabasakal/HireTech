package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	cfg := Load()

	assert.Equal(t, EnvironmentDevelopment, cfg.Environment)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "masterfabric", cfg.Database.User)
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, 6379, cfg.Redis.Port)
	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Format)
}

func TestKafkaConfig_ConsumerGroupIDIsInstanceScoped(t *testing.T) {
	cfg := KafkaConfig{GroupID: "hiretech", InstanceID: "api-2"}

	assert.Equal(t, "hiretech-api-2", cfg.ConsumerGroupID())
}

func TestLoad_PublicKeyRingEnvironment(t *testing.T) {
	t.Setenv("JWT_PUBLIC_KEYS", `{"active":"public-key-pem"}`)

	cfg := Load()

	assert.Equal(t, map[string]string{"active": "public-key-pem"}, cfg.JWT.PublicKeys)
}

func TestLoad_PersistedOperationConfiguration(t *testing.T) {
	t.Setenv("GRAPHQL_REQUIRE_PERSISTED_OPERATIONS", "true")
	t.Setenv("GRAPHQL_ALLOWED_OPERATION_HASHES", "ABC,abc, 123")

	cfg := Load()

	assert.True(t, cfg.GraphQL.RequirePersistedOperations)
	assert.Equal(t, []string{"abc", "123"}, cfg.GraphQL.AllowedOperationHashes)
}

func TestLoad_AuditRelayConfiguration(t *testing.T) {
	t.Setenv("AUDIT_RELAY_ENABLED", "true")
	t.Setenv("AUDIT_RELAY_BATCH_SIZE", "250")
	t.Setenv("AUDIT_RELAY_INTERVAL_MILLISECONDS", "500")
	t.Setenv("AUDIT_RELAY_MAX_BATCHES_PER_CYCLE", "20")

	cfg := Load()

	assert.True(t, cfg.Audit.RelayEnabled)
	assert.Equal(t, 250, cfg.Audit.RelayBatchSize)
	assert.Equal(t, 500*time.Millisecond, cfg.Audit.RelayInterval)
	assert.Equal(t, 20, cfg.Audit.RelayMaxBatchesPerCycle)
}

func TestConfig_ValidateForProductionRejectsDefaultJWTSecret(t *testing.T) {
	cfg := &Config{Environment: EnvironmentProduction, JWT: JWTConfig{Secret: defaultJWTSecret}}

	err := cfg.ValidateForProduction()

	assert.EqualError(t, err, "JWT_SECRET must be explicitly configured in production")
}

func TestConfig_ValidateForProductionRejectsShortJWTSecret(t *testing.T) {
	cfg := &Config{Environment: EnvironmentProduction, JWT: JWTConfig{Secret: "too-short"}}

	err := cfg.ValidateForProduction()

	assert.EqualError(t, err, "JWT_SECRET must contain at least 32 characters in production")
}

func TestConfig_ValidateForProductionAcceptsConfiguredJWTSecret(t *testing.T) {
	cfg := &Config{Environment: EnvironmentProduction, JWT: JWTConfig{
		Secret:        "a-secure-application-hmac-secret-with-32-chars",
		Algorithm:     "RS256",
		PrivateKeyPEM: "configured",
		PublicKeys:    map[string]string{"active": "configured"},
		ActiveKeyID:   "active",
	}}

	assert.NoError(t, cfg.ValidateForProduction())
}

func TestConfig_ValidateForProductionRejectsHS256(t *testing.T) {
	cfg := &Config{Environment: EnvironmentProduction, JWT: JWTConfig{
		Secret:    "a-secure-production-secret-with-32-chars",
		Algorithm: "HS256",
	}}

	assert.EqualError(t, cfg.ValidateForProduction(), "JWT_ALGORITHM must be RS256 in production")
}

func TestConfig_ValidateForProductionRequiresPersistedOperationHashesWhenEnabled(t *testing.T) {
	cfg := &Config{Environment: EnvironmentProduction, JWT: JWTConfig{
		Secret:        "a-secure-application-hmac-secret-with-32-chars",
		Algorithm:     "RS256",
		PrivateKeyPEM: "configured",
		PublicKeys:    map[string]string{"active": "configured"},
		ActiveKeyID:   "active",
	}, GraphQL: GraphQLConfig{RequirePersistedOperations: true}}

	assert.EqualError(t, cfg.ValidateForProduction(), "GRAPHQL_ALLOWED_OPERATION_HASHES must be configured when persisted operations are required")
}

func TestConfig_ValidateForProductionRejectsInvalidAuditRelayBounds(t *testing.T) {
	cfg := &Config{Environment: EnvironmentProduction, JWT: JWTConfig{
		Secret:        "a-secure-application-hmac-secret-with-32-chars",
		Algorithm:     "RS256",
		PrivateKeyPEM: "configured",
		PublicKeys:    map[string]string{"active": "configured"},
		ActiveKeyID:   "active",
	}, Audit: AuditConfig{RelayEnabled: true}}

	assert.EqualError(t, cfg.ValidateForProduction(), "audit relay batch size, interval, and max batches per cycle must be positive in production")
}

func TestAIConfig_ValidateForProductionRejectsSmoketestModel(t *testing.T) {
	cfg := &Config{Environment: EnvironmentProduction, JWT: JWTConfig{
		Secret:        "a-secure-application-hmac-secret-with-32-chars",
		Algorithm:     "RS256",
		PrivateKeyPEM: "configured",
		PublicKeys:    map[string]string{"active": "configured"},
		ActiveKeyID:   "active",
	}, AI: AIConfig{Enabled: true, Interviewer: AIModelConfig{BaseURL: "https://interviewer", ModelID: "model", ModelVersion: "qlora-smoketest-v1"}, Evaluator: AIModelConfig{BaseURL: "https://evaluator", ModelID: "model", ModelVersion: "reviewed-v1"}}}

	assert.EqualError(t, cfg.ValidateForProduction(), "AI interviewer smoketest model cannot be enabled in production")
}

func TestAIConfig_ValidateForProductionAllowsDisabledAI(t *testing.T) {
	assert.NoError(t, AIConfig{}.ValidateForProduction())
}

func TestLoad_EnvironmentOverrides(t *testing.T) {
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("DB_HOST", "db.example.com")
	os.Setenv("EMAIL_PROVIDER", "smtp")
	os.Setenv("SMTP_TLS_MODE", "starttls")
	defer os.Unsetenv("SERVER_PORT")
	defer os.Unsetenv("DB_HOST")
	defer os.Unsetenv("EMAIL_PROVIDER")
	defer os.Unsetenv("SMTP_TLS_MODE")

	cfg := Load()
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, "smtp", cfg.Email.Provider)
	assert.Equal(t, "starttls", cfg.Email.TLSMode)
}

func TestLoad_DBPoolInt32Bounds(t *testing.T) {
	os.Setenv("DB_MAX_CONNS", "50")
	os.Setenv("DB_MIN_CONNS", "2147483648")
	defer os.Unsetenv("DB_MAX_CONNS")
	defer os.Unsetenv("DB_MIN_CONNS")

	cfg := Load()
	assert.Equal(t, int32(50), cfg.Database.MaxConns)
	assert.Equal(t, int32(5), cfg.Database.MinConns)
}

func TestDatabaseConfig_DSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "user",
		Password: "pass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}
	expected := "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
	assert.Equal(t, expected, cfg.DSN())
}

func TestDatabaseConfig_DSN_EscapesSpecialCharacters(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "user@domain",
		Password: "p@ss:w?rd#",
		DBName:   "testdb",
		SSLMode:  "require",
	}
	dsn := cfg.DSN()
	assert.Contains(t, dsn, "postgres://")
	assert.Contains(t, dsn, "sslmode=require")
	assert.NotContains(t, dsn, "p@ss:w?rd#")
}

func TestRedisConfig_Addr(t *testing.T) {
	cfg := RedisConfig{Host: "redis.local", Port: 6380}
	assert.Equal(t, "redis.local:6380", cfg.Addr())
}
