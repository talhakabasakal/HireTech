package usecase_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	iamUsecase "github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
)

func TestRedisRateLimiterIntegration(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("set REDIS_TEST_ADDR to run Redis integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Ping(ctx).Err())

	key := "hiretech:integration:rate-limit:" + uuid.NewString()
	t.Cleanup(func() { _ = client.Del(context.Background(), key).Err() })
	limiter := iamUsecase.NewRedisRateLimiter(client)

	allowed, err := limiter.Allow(ctx, key, 2, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)
	allowed, err = limiter.Allow(ctx, key, 2, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)
	allowed, err = limiter.Allow(ctx, key, 2, time.Minute)
	require.NoError(t, err)
	assert.False(t, allowed)

	count, err := client.Get(ctx, key).Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
	ttl, err := client.TTL(ctx, key).Result()
	require.NoError(t, err)
	assert.Positive(t, ttl)
}
