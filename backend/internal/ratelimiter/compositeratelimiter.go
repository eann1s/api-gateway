package ratelimiter

import (
	"context"
	"errors"
)


var (
	ErrInvalidPrimaryRateLimiter = errors.New("invalid primary rate limiter")
	ErrInvalidFallbackRateLimiter = errors.New("invalid fallback rate limiter")
)

type CompositeRateLimiter struct {
	primary RateLimiter
	fallback RateLimiter
}

func NewCompositeRateLimiter(primary RateLimiter, fallback RateLimiter) (*CompositeRateLimiter, error) {
	if primary == nil {
		return nil, ErrInvalidPrimaryRateLimiter
	}
	if fallback == nil {
		return nil, ErrInvalidFallbackRateLimiter
	}
	return &CompositeRateLimiter{
		primary: primary,
		fallback: fallback,
	}, nil
}

func (rl *CompositeRateLimiter) Allow(ctx context.Context, key string) (Decision, error) {
	d, err := rl.primary.Allow(ctx, key)
	if err != nil && !errors.Is(err, ErrInvalidIdentityKey) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		d, errr := rl.fallback.Allow(ctx, key)
		if errr != nil {
			return Decision{}, errors.Join(err, errr)
		}
		return d, nil
	}
	return d, err
}
