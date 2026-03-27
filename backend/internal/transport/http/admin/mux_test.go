package http_admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)


func TestNewAdminMux(t *testing.T) {
	t.Parallel()

	h := NewHandlers(Deps{
		Ready: func() bool {return true},
		Metrics: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("metrics"))
		}),
	})
	mux := NewAdminMux(h)

	tests := []struct{
		name string
		path string
		method string
		wantStatus int
	} {
		{"healthz", "/healthz", http.MethodGet, http.StatusOK},
		{"readyz", "/readyz", http.MethodGet, http.StatusOK},
		{"metrics", "/metrics", http.MethodGet, http.StatusOK},
		{"not found", "/not-found", http.MethodGet, http.StatusNotFound},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)

			mux.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}
