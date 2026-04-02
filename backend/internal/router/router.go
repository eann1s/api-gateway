package router

import (
	"errors"
	"fmt"
	"strings"
)


type Route struct {
	ID string
	Host string
	PathPrefix string
	UpstreamPool string
}

type RouterTable map[string][]Route // host -> routes

type Router struct {
	table RouterTable
}

var ErrDuplicateRoute = errors.New("duplicate route")
var ErrInvalidRoute = errors.New("invalid route")

func NewRouter(routes []Route) (*Router, error) {
	table := make(RouterTable, len(routes))
	dedup := make(map[[2]string]struct{}, len(routes))
	for _, r := range routes {
		err := normalizeAndValidateRoute(&r)
		if err != nil {
			return nil, err
		}
		if _, ok := table[r.Host]; !ok {
			table[r.Host] = make([]Route, 0)
		}

		dedupKey := [2]string{r.Host, r.PathPrefix}
		if _, ok := dedup[dedupKey]; ok {
			return nil, fmt.Errorf("%w for host %q and path prefix %q", ErrDuplicateRoute, r.Host, r.PathPrefix)
		}
		dedup[dedupKey] = struct{}{}
		table[r.Host] = append(table[r.Host], r)
	}
	return &Router{
		table: table,
	}, nil
}

func (r *Router) Match(host string, path string) (Route, bool) {
	routes, ok := r.table[host]
	if !ok {
		return Route{}, false
	}
	return matchLongestPrefix(path, routes)
}

func normalizeAndValidateRoute(r *Route) error {
	r.ID = strings.TrimSpace(r.ID)
	if r.ID == "" {
		return fmt.Errorf("%w: id cannot be empty", ErrInvalidRoute)
	}

	if r.Host == "" {
		return fmt.Errorf("%w: host cannot be empty", ErrInvalidRoute)
	}
	host, err := NormalizeAndValidateHost(r.Host)
	if err != nil {
		return err
	}
	r.Host = host

	if r.PathPrefix == "" {
		return fmt.Errorf("%w: path prefix cannot be empty", ErrInvalidRoute)
	}
	pathPrefix, err := normalizePrefix(r.PathPrefix)
	if err != nil {
		return err
	}
	r.PathPrefix = pathPrefix

	r.UpstreamPool = strings.TrimSpace(r.UpstreamPool)
	if r.UpstreamPool == "" {
		return fmt.Errorf("%w: upstream pool cannot be empty", ErrInvalidRoute)
	}
	return nil
}

func matchLongestPrefix(path string, routes []Route) (Route, bool) {
	best := Route{}
	for _, r := range routes {
		p := r.PathPrefix
		if p == "" {
			continue
		}
		if p == "/" || p == path || (strings.HasPrefix(path, p) && path[len(p)] == '/') {
			if len(best.PathPrefix) < len(p) {
				best = r
			}
		}
	}
	return best, best.PathPrefix != ""
}


