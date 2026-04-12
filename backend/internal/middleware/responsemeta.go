package middleware

import "net/http"


type RouteMetaWriter interface {
	SetRouteMeta(routeID string, upstreamPool string)
}

type responseMetaWriter struct {
	http.ResponseWriter
	status int
	routeID string
	upstreamPool string
}

func (w *responseMetaWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseMetaWriter) SetRouteMeta(routeID string, upstreamPool string) {
	w.routeID = routeID
	w.upstreamPool = upstreamPool
}