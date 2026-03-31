package middleware

import (
	"net/http"
	"time"

	"github.com/eann1s/rate-limiter/backend/internal/requestid"
	"github.com/rs/zerolog"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

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

func AccessLog(log zerolog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()
			wrapper := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(wrapper, r)
			duration := time.Since(startTime)
			status := wrapper.status

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
			ev.Msg(msg)
		})
	}
}
