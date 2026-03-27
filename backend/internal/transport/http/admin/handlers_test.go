package http_admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)


func TestHandlersHealthz(t *testing.T) {
	t.Parallel()
	wantStatus := http.StatusOK

	h := NewHandlers(Deps{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	h.Healthz(rr, req)

	if rr.Code != wantStatus {
		t.Fatalf("status = %d, want %d", rr.Code, wantStatus)
	}
}

func TestHandlersReadyz(t *testing.T) {
	t.Parallel()
	
	tests := []struct {
		name string
		ready bool
		wantStatus int
	}{
		{"ready", true, http.StatusOK},
		{"not ready", false, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewHandlers(Deps{
				Ready: func() bool {
					return tt.ready
				},
			})

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

			h.Readyz(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

type fakeMetrics struct { called bool }
func (f *fakeMetrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.called = true
	w.WriteHeader(http.StatusOK)
}

func TestHandlersMetrics(t *testing.T) {
	t.Parallel()
	f := &fakeMetrics{}

	h := NewHandlers(Deps{
		Metrics: f,
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	h.Metrics(rr, req)

	if !f.called {
		t.Fatal("metrics handler not called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	
}
