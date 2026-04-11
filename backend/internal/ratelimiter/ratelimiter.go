package ratelimiter

import (
	"context"
	"errors"
	"math"
	"time"
)

var (
	ErrInvalidDeps        = errors.New("invalid deps")
	ErrInvalidIdentityKey = errors.New("invalid identityKey")
)

type Decision struct {
	Allowed    bool
	Remaining  int64
	RetryAfter time.Duration
}

type RateLimiter interface {
	Allow(ctx context.Context, key string) (Decision, error)
}

type TokenBucketConfig struct {
	RefillRatePerSec int64
	Capacity         int64
}

func getTtl(capacity int64, refillRatePerSec int64) int64 {
	fullRefillMs := math.Ceil(float64(capacity) / float64(refillRatePerSec) * 1000)
	ttl := int64(fullRefillMs * 2)
	return ttl
}
