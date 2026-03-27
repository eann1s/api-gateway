package readiness

import (
	"sync/atomic"
	"testing"
)


func TestAtomicReadiness_IsReady(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		ready bool
		want bool
	}{
		{"ready", true, true},
		{"not ready", false, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &AtomicReadiness{v: atomic.Bool{}}
			r.v.Store(tt.ready)

			if got := r.IsReady(); got != tt.want {
				t.Fatalf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAtomicReadiness_SetReady(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		ready bool
		want bool
	}{
		{"ready", true, true},
		{"not ready", false, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &AtomicReadiness{v: atomic.Bool{}}
			r.SetReady(tt.ready)

			if got := r.v.Load(); got != tt.want {
				t.Fatalf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}
