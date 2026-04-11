package middleware

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/eann1s/rate-limiter/backend/internal/ratelimiter"
	"github.com/rs/zerolog"
)


var apikeyHeaders = []string{
	http.CanonicalHeaderKey("X-API-Key"),
	http.CanonicalHeaderKey("Authorization"),
}

func RateLimit(logger zerolog.Logger, rl ratelimiter.RateLimiter) Middleware {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identityKey := ""
			apiKey, ok := getApiKey(r.Header)
			if ok && apiKey != "" {
				identityKey = "api_key:" + apiKey
			} else {
				ip, ok := getIp(r)
				if ok && ip != "" {
					identityKey = "ip:" + ip
				}
			}
			if identityKey == "" {
				logger.Debug().Msg("missing identity key")
				identityKey = "anonymous"
			}

			decision, err := rl.Allow(r.Context(), identityKey)
			if err != nil {
				logger.Error().Err(err).Msg("failed to check rate limit")
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if !decision.Allowed {
				logger.
					Trace().
					Interface("decision", decision).
					Msg("rate limit exceeded")
				w.Header().Set(
					http.CanonicalHeaderKey("Retry-After"), 
					strconv.FormatInt(int64(max(math.Ceil(decision.RetryAfter.Seconds()), 1)), 10),
				)
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			h.ServeHTTP(w, r)
		})
	}
}

func getApiKey(header http.Header) (string, bool) {
	for _, key := range apikeyHeaders {
		val := header.Get(key)
		val = strings.TrimSpace(val)
		if val != "" {
			if key == http.CanonicalHeaderKey("Authorization") {
				parts := strings.SplitN(val, " ", 2)
				if len(parts) != 2 {
					return "", false
				}
				if !strings.EqualFold(parts[0], "Bearer") {
					continue
				}
				return strings.TrimSpace(parts[1]), true
			}
			return val, true
		}
	}
	return "", false
}

func getIp(r *http.Request) (string, bool) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || ip == "" {
		res := net.ParseIP(r.RemoteAddr)
		if res == nil {
			return "", false
		}
		ip = res.String()
	}
	return ip, true
}
