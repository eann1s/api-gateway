package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
)

func TestRequestID_ReqIDDoesnotChange_IfAlreadyPresent(t *testing.T) {
	t.Parallel()
			
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "123")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := RequestID(next)

	h.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got == "" || got != "123" {
		t.Fatalf("expected %v, got %v", "123", got)
	}
}

func TestRequestID_ReqIDIsSet_IfNotPresent(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := RequestID(next)

	h.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("expected req id not to be empty")
	}
}

func TestAccessLog(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		reqID string
		status int
		level string
		method string
		path string
		message string
	} {
		{
			name: "info when status lt 400",
			reqID: "123",
			status: http.StatusOK,
			level: "info",
			method: http.MethodGet,
			path: "/",
			message: "request successful",
		},
		{
			name: "error when status gte 400",
			reqID: "123",
			status: http.StatusBadRequest,
			level: "error",
			method: http.MethodGet,
			path: "/",
			message: "request failed",
		},
		{
			name: "no request id when request id is missing",
			status: http.StatusOK,
			level: "info",
			method: http.MethodGet,
			path: "/",
			message: "request successful",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			log := zerolog.New(&buf).With().Timestamp().Logger()

			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.reqID != "" {
				req.Header.Set("X-Request-ID", tt.reqID)
			}

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			})

			var h http.Handler
			if tt.reqID != "" {
				h = Chain(next, RequestID, AccessLog(log))
			} else {
				h = AccessLog(log)(next)
			}

			h.ServeHTTP(w, req)

			if buf.String() == "" {
				t.Fatal("expected log to be non-empty")
			}

			var got map[string]any
			if err := json.NewDecoder(&buf).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if v, ok := got["level"]; !ok || v != tt.level {
				t.Fatalf("expected %v, got %v", tt.level, v)
			}
			if tt.reqID != "" {
				if v, ok := got["request_id"]; !ok || v != tt.reqID {
					t.Fatalf("expected %v, got %v", tt.reqID, v)
				}
			} else {
				if _, ok := got["request_id"]; ok {
					t.Fatal("expected request_id to be empty")
				}
			}
			if v, ok := got["method"]; !ok || v != tt.method {
				t.Fatalf("expected %v, got %v", tt.method, v)
			}
			if v, ok := got["path"]; !ok || v != tt.path {
				t.Fatalf("expected %v, got %v", tt.path, v)
			}
			if v, ok := got["status"]; !ok || int(v.(float64)) != tt.status {
				t.Fatalf("expected %v, got %v", tt.status, v)
			}
			if _, ok := got["duration"]; !ok {
				t.Fatal("expected duration to be not empty")
			}
			if v, ok := got["message"]; !ok || v != tt.message {
				t.Fatalf("expected %v, got %v", tt.message, v)
			}
		})
	}
}
