package router

import (
	"errors"
	"testing"
)


var route = func(id, host, prefix string) Route {
		return Route{
			ID:           id,
			Host:         host,
			PathPrefix:   prefix,
			UpstreamPool: "pool-main",
		}
	}

func TestRouterValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		routes   []Route
		wantErr   error
	} {
		{
			name: "success",
			routes: []Route{
				route("root", "api.example.com", "/"),
				route("v1", "api.example.com", "/api/v1"),
				route("v1-users", "api.example.com", "/api/v1/users"),
			},
		},
		{
			name: "duplicate route",
			routes: []Route{
				route("root", "api.example.com", "/"),
				route("v1", "api.example.com", "/api/v1"),
				route("v1-users", "api.example.com", "/api/v1/users"),
				route("v1-users", "api.example.com", "/api/v1/users"),
			},
			wantErr: ErrDuplicateRoute,
		},
		{
			name: "empty id",
			routes: []Route{
				{ ID: "  ", Host: "api.example.com", PathPrefix: "/", UpstreamPool: "main-pool" },
			},
			wantErr: ErrInvalidRoute,
		},
		{
			name: "empty upstream pool",
			routes: []Route{
				{ ID: "root", Host: "api.example.com", PathPrefix: "/", UpstreamPool: "  " },
			},
			wantErr: ErrInvalidRoute,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewRouter(tt.routes)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("error is expected")
				} else {
					if !errors.Is(err, tt.wantErr) {
						t.Fatalf("error = %v, want %v", err, tt.wantErr)
					}
				}
			}
		})
	}
}

func TestRouterMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		host     string
		path     string
		routes   []Route
		wantBest Route
		wantOk   bool
	}{
		{
			name:     "empty routes",
			host:     "api.example.com",
			path:     "/api/v1",
			routes:   []Route{},
			wantBest: Route{},
			wantOk:   false,
		},
		{
			name:     "match longest",
			host:     "api.example.com",
			path:     "/api/v1/users",
			routes: []Route{
				route("root", "api.example.com", "/"),
				route("api", "api.example.com", "/api"),
				route("v1", "api.example.com", "/api/v1"),
				route("other-host", "admin.example.com", "/api/v1/users"),
			},
			wantBest: route("v1", "api.example.com", "/api/v1"),
			wantOk:   true,
		},
		{
			name:     "no match",
			host:     "api.example.com",
			path:     "/x",
			routes: []Route{
				route("api", "api.example.com", "/api"),
				route("v1", "api.example.com", "/api/v1"),
			},
			wantBest: Route{},
			wantOk:   false,
		},
		{
			name:     "no match for unknown host",
			host:     "unknown.example.com",
			path:     "/api/v1/users",
			routes: []Route{
				route("api", "api.example.com", "/api"),
				route("v1", "api.example.com", "/api/v1"),
			},
			wantBest: Route{},
			wantOk:   false,
		},
		{
			name:     "match only exact paths not just prefixes",
			host:     "api.example.com",
			path:     "/api2",
			routes: []Route{
				route("api", "api.example.com", "/api"),
				route("api2", "api.example.com", "/api2"),
			},
			wantBest: route("api2", "api.example.com", "/api2"),
			wantOk:   true,
		},
		{
			name:     "return false when no matches even when prefixes match",
			host:     "api.example.com",
			path:     "/api2",
			routes: []Route{
				route("api", "api.example.com", "/api"),
			},
			wantBest: Route{},
			wantOk:   false,
		},
		{
			name:     "root fallback",
			host:     "api.example.com",
			path:     "/x",
			routes: []Route{
				route("root", "api.example.com", "/"),
				route("api", "api.example.com", "/api"),
			},
			wantBest: route("root", "api.example.com", "/"),
			wantOk:   true,
		},
		{
			name:     "boundary with trailing slash path",
			host:     "api.example.com",
			path:     "/api/",
			routes: []Route{
				route("api", "api.example.com", "/api"),
				route("root", "api.example.com", "/"),
			},
			wantBest: route("api", "api.example.com", "/api"),
			wantOk:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router, err := NewRouter(tt.routes)
			if err != nil {
				t.Fatal(err)
			}

			best, ok := router.Match(tt.host, tt.path)
			if best != tt.wantBest || ok != tt.wantOk {
				t.Fatalf("want best %v, got %v, want ok %v, got %v", tt.wantBest, best, tt.wantOk, ok)
			}
		})
	}
}
