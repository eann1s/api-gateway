package middleware

import (
	"net/http"
	"time"

	"github.com/eann1s/rate-limiter/backend/internal/obs/metrics"
	"github.com/eann1s/rate-limiter/backend/internal/requestid"
	"github.com/rs/zerolog"
)


type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID, ok := requestid.FromHeaders(r)
		if !ok {
			reqID = requestid.New()
			r.Header.Set("X-Request-ID", reqID)
		}
		w.Header().Set("X-Request-ID", reqID)
		r = r.WithContext(requestid.WithContext(r.Context(), reqID))
		next.ServeHTTP(w, r)
	})
}

func AccessLog(log zerolog.Logger, m *metrics.Metrics) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()
			wrapper := &responseMetaWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(wrapper, r)
			duration := time.Since(startTime)
			status := wrapper.status
			statusClass := getStatusClass(status)

			var level zerolog.Level
			var msg string
			if status >= http.StatusBadRequest {
				msg = "request failed"
				level = zerolog.ErrorLevel
			} else {
				msg = "request successful"
				level = zerolog.InfoLevel
			}
			ev := log.
				WithLevel(level).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapper.status).
				Dur("duration", duration)


			reqID, ok := requestid.FromContext(r.Context()) 
			if ok && reqID != "" {
				ev = ev.Str("request_id", reqID)
			}

			if wrapper.routeID == "" {
				wrapper.routeID = "unknown"
			}
			if wrapper.upstreamPool == "" {
				wrapper.upstreamPool = "unknown"
			}

			ev = ev.Str("route_id", wrapper.routeID).Str("upstream_pool", wrapper.upstreamPool)

			ev.Msg(msg)

			m.RequestsTotal.WithLabelValues(wrapper.routeID, r.Method, statusClass).Inc()
			m.RequestDuration.WithLabelValues(wrapper.routeID, r.Method, statusClass).Observe(float64(duration.Seconds()))
		})
	}
}

func getStatusClass(status int) string {
	if status < 200 {
		return "1xx"
	} else if status >= 200 && status < 300 {
		return "2xx"
	} else if status >= 300 && status < 400 {
		return "3xx"
	} else if status >= 400 && status < 500 {
		return "4xx"
	} else {
		return "5xx"
	}
}
