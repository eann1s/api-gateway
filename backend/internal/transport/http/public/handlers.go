package http_public

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/eann1s/rate-limiter/backend/internal/routectx"
	"github.com/eann1s/rate-limiter/backend/internal/router"
)


type Deps struct {
	Router *router.Router
	Next http.HandlerFunc
}

type Handlers struct {
	deps Deps
}

var ErrNoRouter = errors.New("router is required")
var ErrNoNextHandler = errors.New("next handler is required")

func NewHandlers(deps Deps) (*Handlers, error) {
	if deps.Router == nil {
		return nil, ErrNoRouter
	}
	if deps.Next == nil {
		return nil, ErrNoNextHandler
	}
	return &Handlers{
		deps: deps,
	}, nil
}

func (h *Handlers) Root(w http.ResponseWriter, r *http.Request) {
	host := strings.TrimSpace(strings.ToLower(r.Host))
	res, _, err := net.SplitHostPort(host)
	if err == nil && res != "" {
		host = res
	}
	path := r.URL.Path
	route, ok := h.deps.Router.Match(host, path)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	rc := routectx.RouteContext{
		RouteID: route.ID,
		UpstreamPool: route.UpstreamPool,
	}
	r = r.WithContext(routectx.WithRoute(r.Context(), rc))
	h.deps.Next(w, r)
}
