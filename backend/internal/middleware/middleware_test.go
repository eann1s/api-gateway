package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/eann1s/rate-limiter/backend/internal/obs/metrics"
	"github.com/eann1s/rate-limiter/backend/internal/ratelimiter"
	"github.com/rs/zerolog"
)


func TestRequestID(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		reqID string
	} {
		{
			name: "request id does not change if already present",
			reqID: "123",
		},
		{
			name: "request id is set if not present",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.reqID != "" {
				req.Header.Set("X-Request-ID", tt.reqID)
			}

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			h := Chain(next, RequestID)
			h.ServeHTTP(w, req)

			got := w.Header().Get("X-Request-ID")
			if tt.reqID != "" && got != tt.reqID {
				t.Fatalf("expected %v, got %v", tt.reqID, got)
			}
			if got == "" {
				t.Fatal("expected req id not to be empty")
			}
		})
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
			m := metrics.NewMetrics()

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
				h = Chain(next, RequestID, AccessLog(log, m))
			} else {
				h = AccessLog(log, m)(next)
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

type MockRateLimiter struct {
	gotKey string
	allowFunc func(ctx context.Context, key string) (ratelimiter.Decision, error)
}

func (rl *MockRateLimiter) Allow(ctx context.Context, key string) (ratelimiter.Decision, error) {
	rl.gotKey = key
	if rl.allowFunc != nil {
		return rl.allowFunc(ctx, key)
	} else {
		return ratelimiter.Decision{}, nil
	}
}

func TestRateLimit(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		header http.Header
		ip string
		allowFunc func(ctx context.Context, key string) (ratelimiter.Decision, error)
		wantNextCalled bool
		wantStatusCode int
		wantIdentityKey string
		wantLogLevel string
		wantLogMsg string
		wantRetryAfter int
	} {
		{
			name: "success api_key in X-Api-Key",
			header: map[string][]string{
				"X-Api-Key": {"foo"},
			},
			allowFunc: func(ctx context.Context, key string) (ratelimiter.Decision, error) {
				return ratelimiter.Decision{
					Allowed: true,
					RetryAfter: 0,
					Remaining: 3,
				}, nil
			},
			wantNextCalled: true,
			wantStatusCode: http.StatusOK,
			wantIdentityKey: "api_key:foo",
		},
		{
			name: "success api_key in Authorization Bearer",
			header: map[string][]string{
				"Authorization": {"Bearer foo"},
			},
			allowFunc: func(ctx context.Context, key string) (ratelimiter.Decision, error) {
				return ratelimiter.Decision{
					Allowed: true,
					RetryAfter: 0,
					Remaining: 3,
				}, nil
			},
			wantNextCalled: true,
			wantStatusCode: http.StatusOK,
			wantIdentityKey: "api_key:foo",
		},
		{
			name: "success ip fallback",
			ip: "1.2.3.4",
			allowFunc: func(ctx context.Context, key string) (ratelimiter.Decision, error) {
				return ratelimiter.Decision{
					Allowed: true,
					RetryAfter: 0,
					Remaining: 3,
				}, nil
			},
			wantNextCalled: true,
			wantStatusCode: http.StatusOK,
			wantIdentityKey: "ip:1.2.3.4",
		},
		{
			name: "success anonymous fallback without ip",
			allowFunc: func(ctx context.Context, key string) (ratelimiter.Decision, error) {
				return ratelimiter.Decision{
					Allowed: true,
					RetryAfter: 0,
					Remaining: 3,
				}, nil
			},
			wantNextCalled: true,
			wantStatusCode: http.StatusOK,
			wantIdentityKey: "anonymous",
			wantLogLevel: zerolog.DebugLevel.String(),
			wantLogMsg: "missing identity key",
		},
		{
			name: "fallback to ip when apikey in wrong header",
			header: map[string][]string{
				"Wrong-Header": {"foo"},
			},
			ip: "1.2.3.4",
			allowFunc: func(ctx context.Context, key string) (ratelimiter.Decision, error) {
				return ratelimiter.Decision{
					Allowed: true,
					RetryAfter: 0,
					Remaining: 3,
				}, nil
			},
			wantNextCalled: true,
			wantStatusCode: http.StatusOK,
			wantIdentityKey: "ip:1.2.3.4",
		},
		{
			name: "service unavailable when rl fails",
			ip: "1.2.3.4",
			allowFunc: func(ctx context.Context, key string) (ratelimiter.Decision, error) {
				return ratelimiter.Decision{}, errors.New("failed to check rate limit")
			},
			wantNextCalled: false,
			wantIdentityKey: "ip:1.2.3.4",
			wantStatusCode: http.StatusServiceUnavailable,
			wantLogLevel: zerolog.ErrorLevel.String(),
			wantLogMsg: "failed to check rate limit",
		},
		{
			name: "429 when decision allowed = false",
			ip: "1.2.3.4",
			allowFunc: func(ctx context.Context, key string) (ratelimiter.Decision, error) {
				return ratelimiter.Decision{
					Allowed: false,
					RetryAfter: time.Second * 1,
					Remaining: 0,
				}, nil
			},
			wantNextCalled: false,
			wantStatusCode: http.StatusTooManyRequests,
			wantIdentityKey: "ip:1.2.3.4",
			wantRetryAfter: 1,
			wantLogLevel: zerolog.TraceLevel.String(),
			wantLogMsg: "rate limit exceeded",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rl := &MockRateLimiter{
				allowFunc: tt.allowFunc,
			}

			var buf bytes.Buffer
			log := zerolog.New(&buf).With().Timestamp().Logger()
			
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header = tt.header
			req.RemoteAddr = tt.ip

			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			middleware := Chain(next, RateLimit(log, rl))
			middleware.ServeHTTP(w, req)

			res := w.Result()
			if tt.wantNextCalled != nextCalled {
				t.Fatalf("expected next called: %v, got %v", tt.wantNextCalled, nextCalled)
			}
			if res.StatusCode != tt.wantStatusCode {
				t.Fatalf("expected status code: %v, got %v", tt.wantStatusCode, res.StatusCode)
			}
			if rl.gotKey != tt.wantIdentityKey {
				t.Fatalf("expected identity key: %v, got %v", tt.wantIdentityKey, rl.gotKey)
			}
			if tt.wantRetryAfter == 0 && res.Header.Get("Retry-After") != "" {
				t.Fatalf("expected retry after: %v, got %v", tt.wantRetryAfter, res.Header.Get("Retry-After"))
			}
			if tt.wantRetryAfter != 0 && res.Header.Get("Retry-After") != strconv.Itoa(tt.wantRetryAfter) {
				t.Fatalf("expected retry after: %v, got %v", tt.wantRetryAfter, res.Header.Get("Retry-After"))
			}

			if tt.wantLogMsg != "" {
				var got map[string]any
				if err := json.NewDecoder(&buf).Decode(&got); err != nil {
					t.Fatal(err)
				}
				if got["message"] != tt.wantLogMsg {
					t.Fatalf("expected log message: %v, got %v", tt.wantLogMsg, buf.String())
				}
				if got["level"] != tt.wantLogLevel {
					t.Fatalf("expected log level: %v, got %v", tt.wantLogLevel, buf.String())
				}
			}
		})
	}
}
