package ratelimiter

import (
	"context"
	"errors"
	"testing"
)


type FakeRateLimiter struct {
	allowFunc func(ctx context.Context, key string) (Decision, error)
	wasCalled bool
}

func (f *FakeRateLimiter) Allow(ctx context.Context, key string) (Decision, error) {
	f.wasCalled = true
	if f.allowFunc != nil {
		return f.allowFunc(ctx, key)
	}
	return Decision{}, nil
}

func TestNewCompositeRateLimiter(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		primary RateLimiter
		fallback RateLimiter
		wantErr error
	} {
		{
			name: "success",
			primary: &FakeRateLimiter{},
			fallback: &FakeRateLimiter{},
			wantErr: nil,
		},
		{
			name: "invalid primary rate limiter",
			primary: nil,
			fallback: &FakeRateLimiter{},
			wantErr: ErrInvalidPrimaryRateLimiter,
		},
		{
			name: "invalid fallback rate limiter",
			primary: &FakeRateLimiter{},
			fallback: nil,
			wantErr: ErrInvalidFallbackRateLimiter,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewCompositeRateLimiter(tt.primary, tt.fallback)
			if err != tt.wantErr {
				t.Errorf("NewCompositeRateLimiter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompositeRateLimiterAllow(t *testing.T) {
	t.Parallel()

	sentinelErr := errors.New("boom")

	tests := []struct{
		name string
		primary *FakeRateLimiter
		fallback *FakeRateLimiter
		wantPrimaryCalled bool
		wantFallbackCalled bool
		wantErr error
	} {
		{
			name: "route to primary when it's ok",
			primary: &FakeRateLimiter{
				allowFunc: func(ctx context.Context, key string) (Decision, error) {
					return Decision{}, nil
				},
			},
			fallback: &FakeRateLimiter{},
			wantPrimaryCalled: true,
			wantFallbackCalled: false,
		},
		{
			name: "route to fallback when primary is not ok",
			primary: &FakeRateLimiter{
				allowFunc: func(ctx context.Context, key string) (Decision, error) {
					return Decision{}, sentinelErr
				},
			},
			fallback: &FakeRateLimiter{
				allowFunc: func(ctx context.Context, key string) (Decision, error) {
					return Decision{}, nil
				},
			},
			wantPrimaryCalled: true,
			wantFallbackCalled: true,
		},
		{
			name: "returns error when both are not ok",
			primary: &FakeRateLimiter{
				allowFunc: func(ctx context.Context, key string) (Decision, error) {
					return Decision{}, sentinelErr
				},
			},
			fallback: &FakeRateLimiter{
				allowFunc: func(ctx context.Context, key string) (Decision, error) {
					return Decision{}, sentinelErr
				},
			},
			wantPrimaryCalled: true,
			wantFallbackCalled: true,
			wantErr: sentinelErr,
		},
		{
			name: "fallback is not called when primary returns err invalid identity key",
			primary: &FakeRateLimiter{
				allowFunc: func(ctx context.Context, key string) (Decision, error) {
					return Decision{}, ErrInvalidIdentityKey
				},
			},
			fallback: &FakeRateLimiter{
				allowFunc: func(ctx context.Context, key string) (Decision, error) {
					return Decision{}, nil
				},
			},
			wantPrimaryCalled: true,
			wantFallbackCalled: false,
			wantErr: ErrInvalidIdentityKey,
		},
		{
			name: "fallback is not called when primary returns context canceled",
			primary: &FakeRateLimiter{
				allowFunc: func(ctx context.Context, key string) (Decision, error) {
					return Decision{}, context.Canceled
				},
			},
			fallback: &FakeRateLimiter{
				allowFunc: func(ctx context.Context, key string) (Decision, error) {
					return Decision{}, nil
				},
			},
			wantPrimaryCalled: true,
			wantFallbackCalled: false,
			wantErr: context.Canceled,
		},
		{
			name: "fallback is not called when primary returns context deadline exceeded",
			primary: &FakeRateLimiter{
				allowFunc: func(ctx context.Context, key string) (Decision, error) {
					return Decision{}, context.DeadlineExceeded
				},
			},
			fallback: &FakeRateLimiter{
				allowFunc: func(ctx context.Context, key string) (Decision, error) {
					return Decision{}, nil
				},
			},
			wantPrimaryCalled: true,
			wantFallbackCalled: false,
			wantErr: context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rl, err := NewCompositeRateLimiter(tt.primary, tt.fallback)
			if err != nil {
				t.Fatal(err)
			}

			_, err = rl.Allow(context.Background(), "123")
			if tt.wantErr == nil && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
			if tt.wantPrimaryCalled != tt.primary.wasCalled {
				t.Errorf("expected primary to be called %v, got %v", tt.wantPrimaryCalled, tt.primary.wasCalled)
			}
			if tt.wantFallbackCalled != tt.fallback.wasCalled {
				t.Errorf("expected fallback to be called %v, got %v", tt.wantFallbackCalled, tt.fallback.wasCalled)
			}
		})
	}
}
