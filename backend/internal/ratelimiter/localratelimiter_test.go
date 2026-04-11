package ratelimiter

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewLocalRateLimiter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		deps    LocalRateLimiterDeps
		wantErr error
	}{
		{
			name: "success",
			deps: LocalRateLimiterDeps{
				Config: TokenBucketConfig{
					RefillRatePerSec: 10,
					Capacity:         10,
				},
			},
			wantErr: nil,
		},
		{
			name: "invalid refill rate per sec",
			deps: LocalRateLimiterDeps{
				Config: TokenBucketConfig{
					RefillRatePerSec: 0,
					Capacity:         10,
				},
			},
			wantErr: ErrInvalidDeps,
		},
		{
			name: "invalid capacity",
			deps: LocalRateLimiterDeps{
				Config: TokenBucketConfig{
					RefillRatePerSec: 10,
					Capacity:         0,
				},
			},
			wantErr: ErrInvalidDeps,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewLocalRateLimiter(tt.deps)
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

func TestLocalRateLimiterAllow_EmptyIdentityReturnsError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := setupLocalLimiter(t)

	_, err := r.Allow(ctx, "  ")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidIdentityKey) {
		t.Errorf("error %v does not match expected error %v", err, ErrInvalidIdentityKey)
	}
}

func TestLocalRateLimiterAllow_FirstAllowedRemainingOne(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := setupLocalLimiter(t)

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

func TestLocalRateLimiterAllow_TwoAllowedRemainingZero(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := setupLocalLimiter(t)

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

func TestLocalRateLimiterAllow_ThirdDenied(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := setupLocalLimiter(t)

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

func TestLocalRateLimiterAllow_AfterSomeTimeAllowedAgain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := setupLocalLimiter(t)

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

	time.Sleep(time.Second * 1)
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

func TestLocalRateLimiterAllow_Concurrent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := setupLocalLimiter(t)

	var wg sync.WaitGroup
	var allowed atomic.Int32
	var denied atomic.Int32
	workers := 50
	identityKey := "foo"
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
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

func TestLocalRateLimiterCleanup(t *testing.T) {
	t.Parallel()

	r := setupLocalLimiter(t)

	r.buckets["foo"] = &bucketState{
		remaining: 1,
		expiredAt: time.Now().Add(-time.Second),
	}
	r.CleanupExpired()

	if _, ok := r.buckets["foo"]; ok {
		t.Errorf("expected bucket to be deleted, got %v", r.buckets["foo"])
		
	}
	if len(r.buckets) != 0 {
		t.Errorf("expected buckets to be empty, got %v", len(r.buckets))
	}
}

func setupLocalLimiter(t *testing.T) *LocalRateLimiter {
	t.Helper()

	deps := LocalRateLimiterDeps{
		Config: TokenBucketConfig{
			Capacity:         2,
			RefillRatePerSec: 1,
		},
	}

	limiter, err := NewLocalRateLimiter(deps)
	if err != nil {
		t.Fatal(err)
	}
	return limiter
}
