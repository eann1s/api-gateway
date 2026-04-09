package ratelimiter

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)


func TestNewRedisRateLimiter(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		deps RedisRateLimiterDeps
		wantErr error
	} {
		{
			name: "success",
			deps: RedisRateLimiterDeps{
				Redis: &redis.Client{},
				Config: TokenBucketConfig{
					RefillRatePerSec: 10,
					Capacity: 10,
				},
				KeyPrefix: "rl",
			},
			wantErr: nil,
		},
		{
			name: "no redis client",
			deps: RedisRateLimiterDeps{
				Config: TokenBucketConfig{
					RefillRatePerSec: 10,
					Capacity: 10,
				},
				KeyPrefix: "rl",
			},
			wantErr: ErrInvalidDeps,
		},
		{
			name: "invalid refill rate per sec",
			deps: RedisRateLimiterDeps{
				Redis: &redis.Client{},
				Config: TokenBucketConfig{
					RefillRatePerSec: 0,
					Capacity: 10,
				},
				KeyPrefix: "rl",
			},
			wantErr: ErrInvalidDeps,
		},
		{
			name: "invalid capacity",
			deps: RedisRateLimiterDeps{
				Redis: &redis.Client{},
				Config: TokenBucketConfig{
					RefillRatePerSec: 10,
					Capacity: 0,
				},
				KeyPrefix: "rl",
			},
			wantErr: ErrInvalidDeps,
		},
		{
			name: "invalid key prefix",
			deps: RedisRateLimiterDeps{
				Redis: &redis.Client{},
				Config: TokenBucketConfig{
					RefillRatePerSec: 10,
					Capacity: 10,
				},
				KeyPrefix: "  ",
			},
			wantErr: ErrInvalidDeps,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewRedisRateLimiter(tt.deps)
			if tt.wantErr != nil && err == nil {
				t.Errorf("wanted error %v, got nil", tt.wantErr)
			}
			if tt.wantErr == nil && err != nil {
				t.Errorf("error %v not expected", err)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error %v does not match expected error %v", err, tt.wantErr)
			}
		})
	}
}

func TestRedisRateLimiterAllow_EmptyIdentityReturnsError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r, _ := setupLimiter(t)

	_, err := r.Allow(ctx, "  ")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidIdentityKey) {
		t.Errorf("error %v does not match expected error %v", err, ErrInvalidIdentityKey)
	}
}

func TestRedisRateLimiterAllow_FirstAllowedRemainingOne(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r, _ := setupLimiter(t)

	decision, err := r.Allow(ctx, "foo")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed != true {
		t.Errorf("expected allowed to be true, got %v", decision.Allowed)
	}
	if decision.Remaining != 1 {
		t.Errorf("expected remaining to be 1, got %v", decision.Remaining)
	}
}

func TestRedisRateLimiterAllow_TwoAllowedRemainingZero(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r, _ := setupLimiter(t)

	_, err := r.Allow(ctx, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decision, err := r.Allow(ctx, "foo")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed != true {
		t.Errorf("expected allowed to be true, got %v", decision.Allowed)
	}
	if decision.Remaining != 0 {
		t.Errorf("expected remaining to be 0, got %v", decision.Remaining)
	}
}

func TestRedisRateLimiterAllow_ThirdDenied(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r, _ := setupLimiter(t)

	_, err := r.Allow(ctx, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = r.Allow(ctx, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decision, err := r.Allow(ctx, "foo")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed != false {
		t.Errorf("expected allowed to be false, got %v", decision.Allowed)
	}
	if decision.Remaining != 0 {
		t.Errorf("expected remaining to be 0, got %v", decision.Remaining)
	}
	if decision.RetryAfter == 0 {
		t.Errorf("expected retryAfter to be > 0, got %v", decision.RetryAfter)
	}
}

func TestRedisRateLimiterAllow_AfterSomeTimeAllowedAgain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r, mr := setupLimiter(t)

	now := time.Now()
	mr.SetTime(now)

	_, err := r.Allow(ctx, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = r.Allow(ctx, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = r.Allow(ctx, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr.SetTime(now.Add(1 * time.Second))
	decision, err := r.Allow(ctx, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed != true {
		t.Errorf("expected allowed to be true, got %v", decision.Allowed)
	}
	if decision.Remaining != 0 {
		t.Errorf("expected remaining to be 0, got %v", decision.Remaining)
	}
}

func TestRedisRateLimiterAllow_Concurrent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r, _ := setupLimiter(t)

	var wg sync.WaitGroup
	var allowed atomic.Int32
	var denied atomic.Int32
	workers := 50
	identityKey := "foo"
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func ()  {
			defer wg.Done()
			dec, err := r.Allow(ctx, identityKey)
			if err != nil {
				errCh <- err
				return
			}
			if dec.Allowed {
				allowed.Add(1)
			} else {
				denied.Add(1)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	errs := []error{}
	for err := range errCh {
		errs = append(errs, err)
	}
	if errs != nil && len(errs) > 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	if allowed.Load() != 2 {
		t.Errorf("expected allowed to be 2, got %v", allowed.Load())
	}
	if denied.Load() != 48 {
		t.Errorf("expected denied to be 48, got %v", denied.Load())
	}
}

func setupLimiter(t *testing.T) (*RedisRateLimiter, *miniredis.Miniredis) {
	t.Helper()

	mr, _ := miniredis.Run()
	t.Cleanup(func() {mr.Close()})
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {_ = client.Close()})

	deps := RedisRateLimiterDeps{
		Redis: client,
		Config: TokenBucketConfig{
			Capacity: 2,
			RefillRatePerSec: 1,
		},
		KeyPrefix: "rl",
	}

	limiter, err := NewRedisRateLimiter(deps)
	if err != nil {
		t.Fatal(err)
	}
	return limiter, mr
}
