package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
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
	cfg := &Config{Environment: EnvironmentProduction, JWT: JWTConfig{Secret: "a-secure-production-secret-with-32-chars"}}

	assert.NoError(t, cfg.ValidateForProduction())
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
