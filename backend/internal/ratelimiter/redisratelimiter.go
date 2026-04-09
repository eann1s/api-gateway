package ratelimiter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var luaScript = `
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)

local key = KEYS[1]
local ttl = tonumber(ARGV[1])
local rate_per_sec = tonumber(ARGV[2])
local capacity = tonumber(ARGV[3])
local tokens = tonumber(redis.call('HGET', key, 'tokens') or tostring(capacity))
local last_refill_ts = tonumber(redis.call('HGET', key, 'last_refill_ts') or tostring(now))

local elapsed_time = now - last_refill_ts
local refill = math.floor(elapsed_time * rate_per_sec / 1000)
tokens = math.min(tokens + refill, capacity)

if tokens >= 1 then
	redis.call('HSET', key, 'tokens', tokens - 1)
	redis.call('HSET', key, 'last_refill_ts', now)
	redis.call('PEXPIRE', key, ttl)
	return {1, tokens - 1, 0}
end
redis.call('HSET', key, 'tokens', tokens)
redis.call('HSET', key, 'last_refill_ts', now)
redis.call('PEXPIRE', key, ttl)
local retry_after = math.ceil((1 - tokens) / rate_per_sec) * 1000
return {0, tokens, retry_after}
`

var (
	ErrInvalidDeps        = errors.New("invalid deps")
	ErrInvalidIdentityKey = errors.New("invalid identityKey")
	ErrInvalidLuaResponse = errors.New("invalid lua response")
	ErrRedisFailure       = errors.New("redis failure")
)

type RedisRateLimiterDeps struct {
	Redis     *redis.Client
	Config    TokenBucketConfig
	KeyPrefix string
}

type RedisRateLimiter struct {
	deps RedisRateLimiterDeps
}

func NewRedisRateLimiter(deps RedisRateLimiterDeps) (*RedisRateLimiter, error) {
	if deps.Redis == nil {
		return nil, fmt.Errorf("%w, redis client is required", ErrInvalidDeps)
	}
	if deps.Config.Capacity <= 0 {
		return nil, fmt.Errorf("%w, capacity should be > 0", ErrInvalidDeps)
	}
	if deps.Config.RefillRatePerSec <= 0 {
		return nil, fmt.Errorf("%w, refillRatePerSec should be > 0", ErrInvalidDeps)
	}
	deps.KeyPrefix = strings.TrimSpace(deps.KeyPrefix)
	if deps.KeyPrefix == "" {
		return nil, fmt.Errorf("%w, keyPrefix is required", ErrInvalidDeps)
	}
	deps.KeyPrefix = strings.TrimSuffix(deps.KeyPrefix, ":")
	return &RedisRateLimiter{deps: deps}, nil
}

func (r *RedisRateLimiter) Allow(ctx context.Context, identityKey string) (Decision, error) {
	identityKey = strings.TrimSpace(identityKey)
	if identityKey == "" {
		return Decision{}, fmt.Errorf("%w, identityKey is required", ErrInvalidIdentityKey)
	}
	key := r.deps.KeyPrefix + ":" + identityKey
	fullRefillMs := math.Ceil(float64(r.deps.Config.Capacity) / float64(r.deps.Config.RefillRatePerSec) * 1000)
	ttl := int64(fullRefillMs * 2)

	raw := r.deps.Redis.Eval(ctx, luaScript, []string{key}, []any{ttl, r.deps.Config.RefillRatePerSec, r.deps.Config.Capacity})
	res, err := raw.Int64Slice()
	if err != nil {
		return Decision{}, fmt.Errorf("%w: %w", ErrRedisFailure, err)
	}
	if len(res) != 3 {
		return Decision{}, fmt.Errorf("%w: %v", ErrInvalidLuaResponse, res)
	}

	allowed := res[0] == 1
	remaining := res[1]
	retryAfterMs := time.Millisecond * time.Duration(res[2])
	return Decision{Allowed: allowed, Remaining: remaining, RetryAfter: retryAfterMs}, nil
}
