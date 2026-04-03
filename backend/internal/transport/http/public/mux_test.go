package http_public

import (
	"net/http"
	"net/http/httptest"
	"testing"
)


func MuxTest(t *testing.T) {
	t.Parallel()

	deps := Deps{Next: func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}}
	h, err := NewHandlers(deps)
	if err != nil {
		t.Fatal(err)
	}
	mux := NewPublicMux(h)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
