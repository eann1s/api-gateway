package ratelimiter

import (
	"context"
	"time"
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
