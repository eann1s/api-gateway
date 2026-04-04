package readiness

import (
	"sync/atomic"
)


type Readiness interface {
	IsReady() bool
	SetReady(bool)
}

type AtomicReadiness struct {
	v atomic.Bool
}

func (r *AtomicReadiness) IsReady() bool {
	return r.v.Load()
}

func (r *AtomicReadiness) SetReady(b bool) {
	r.v.Store(b)
}
