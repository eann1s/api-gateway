package ratelimiter

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)


type bucketState struct {
	remaining int64
	lastRefillTs time.Time
	expiredAt time.Time
}

type LocalRateLimiterDeps struct {
	Config TokenBucketConfig
}

type LocalRateLimiter struct {
	deps LocalRateLimiterDeps
	buckets map[string]*bucketState
	lock sync.Mutex
}

func NewLocalRateLimiter(deps LocalRateLimiterDeps) (*LocalRateLimiter, error) {
	if deps.Config.RefillRatePerSec <= 0 {
		return nil, fmt.Errorf("%w, refill rate per sec should be greater than 0", ErrInvalidDeps)
	}
	if deps.Config.Capacity <= 0 {
		return nil, fmt.Errorf("%w, capacity should be greater than 0", ErrInvalidDeps)
	}
	buckets := make(map[string]*bucketState)
	return &LocalRateLimiter{deps: deps, buckets: buckets}, nil
}

func (lr *LocalRateLimiter) Allow(ctx context.Context, key string) (Decision, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Decision{}, fmt.Errorf("%w, identityKey should not be empty", ErrInvalidIdentityKey)
	}
	now := time.Now()
	ttl := getTtl(lr.deps.Config.Capacity, lr.deps.Config.RefillRatePerSec)
	expiredAt := now.Add(time.Millisecond * time.Duration(ttl))
	lr.lock.Lock()
	defer lr.lock.Unlock()

	bucket, ok := lr.buckets[key]
	if !ok {
		bucket = &bucketState{
			remaining: lr.deps.Config.Capacity,
			lastRefillTs: now,
			expiredAt: expiredAt,
		}
		lr.buckets[key] = bucket
	}

	elapsedTime := now.Sub(bucket.lastRefillTs)
	refill := int64(math.Floor(elapsedTime.Seconds() * float64(lr.deps.Config.RefillRatePerSec)))
	tokens := int64(math.Min(float64(refill + bucket.remaining), float64(lr.deps.Config.Capacity)))

	bucket.lastRefillTs = now
	bucket.expiredAt = expiredAt
	if tokens >= 1 {
		bucket.remaining = tokens - 1
		return Decision{Allowed: true, Remaining: bucket.remaining, RetryAfter: 0}, nil
	}
	bucket.remaining = tokens
	retryAfter := time.Second * time.Duration(math.Ceil(float64(1 - tokens) / float64(lr.deps.Config.RefillRatePerSec)))
	return Decision{Allowed: false, Remaining: tokens, RetryAfter: retryAfter}, nil
}

func (lr *LocalRateLimiter) CleanupExpired() {
	now := time.Now()
	lr.lock.Lock()
	defer lr.lock.Unlock()
	for key, bucket := range lr.buckets {
		if bucket.expiredAt.Before(now) {
			delete(lr.buckets, key)
		}
	}
}
