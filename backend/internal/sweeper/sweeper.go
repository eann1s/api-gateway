package sweeper

import (
	"context"
	"errors"
	"time"
)


var (
	ErrInvalidInterval = errors.New("invalid interval")
)

type Sweeper struct {
	interval time.Duration
}

func NewSweeper(interval time.Duration) (*Sweeper, error) {
	if interval <= 0 {
		return nil, ErrInvalidInterval
	}
	return &Sweeper{
		interval: interval,
	}, nil
}

func (s *Sweeper) Run(ctx context.Context, cleanup func()) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cleanup()
		case <-ctx.Done():
			return
		}
	}
}
