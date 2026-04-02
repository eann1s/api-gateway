package routectx

import "context"

type routeContextKey struct {}
type RouteContext struct {
	RouteID string
	UpstreamPool string
}

func WithRoute(ctx context.Context, rc RouteContext) context.Context {
	return context.WithValue(ctx, routeContextKey{}, rc)
}

func FromContext(ctx context.Context) (RouteContext, bool) {
	rc, ok := ctx.Value(routeContextKey{}).(RouteContext)
	return rc, ok
}

func RouteIDFromContext(ctx context.Context) (string, bool) {
	rc, ok := FromContext(ctx)
	return rc.RouteID, ok
}

func UpstreamPoolFromContext(ctx context.Context) (string, bool) {
	rc, ok := FromContext(ctx)
	return rc.UpstreamPool, ok
}

