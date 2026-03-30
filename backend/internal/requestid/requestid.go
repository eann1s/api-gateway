package requestid

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)


type contextKey struct {}

func New() string {
	return uuid.NewString()
}

func WithContext(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, contextKey{}, requestID)
}

func FromContext(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(contextKey{}).(string)
	return requestID, ok
}

func FromHeaders(r *http.Request) (string, bool) {
	val := r.Header.Get("X-Request-ID")
	if val == "" {
		return "", false
	}
	return val, true
}