package http_public

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eann1s/rate-limiter/backend/internal/routectx"
	"github.com/eann1s/rate-limiter/backend/internal/router"
)


func TestHandlers_NewHandlers(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		deps Deps
		wantErr error
	} {
		{
			name: "success",
			deps: Deps{Router: &router.Router{}, Next: func(w http.ResponseWriter, r *http.Request) {}},
			wantErr: nil,
		},
		{
			name: "no router",
			deps: Deps{Next: func(w http.ResponseWriter, r *http.Request) {}},
			wantErr: ErrNoRouter,
		},
		{
			name: "no next handler",
			deps: Deps{Router: &router.Router{}},
			wantErr: ErrNoNextHandler,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewHandlers(tt.deps)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewHandlers() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandlers_Root(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		host string
		path string
		routeID string
		upstreamPool string
		wantStatus int
		wantNextCalled bool
		routes []router.Route
	} {
		{
			name: "success",
			host: "api.example.com",
			path: "/api/v1",
			routeID: "api-v1",
			upstreamPool: "main-pool",
			wantStatus: http.StatusOK,
			wantNextCalled: true,
			routes: []router.Route{
				{ID: "api-v1", Host: "api.example.com", PathPrefix: "/api/v1", UpstreamPool: "main-pool"},
			},
		},
		{
			name: "no match by host",
			host: "api.example.com",
			path: "/api/v1",
			routeID: "api-v1",
			upstreamPool: "main-pool",
			wantStatus: http.StatusNotFound,
			wantNextCalled: false,
			routes: []router.Route{
				{ID: "api-v1", Host: "admin.example.com", PathPrefix: "/api/v1", UpstreamPool: "main-pool"},
			},
		},
		{
			name: "no match by path",
			host: "api.example.com",
			path: "/api/v1",
			routeID: "api-v1",
			upstreamPool: "main-pool",
			wantStatus: http.StatusNotFound,
			wantNextCalled: false,
			routes: []router.Route{
				{ID: "api-v1", Host: "api.example.com", PathPrefix: "/api/v2", UpstreamPool: "main-pool"},
			},
		},
		{
			name: "host normalized",
			host: "  api.examplE.com:1234 ",
			path: "/api/v1",
			routeID: "api-v1",
			upstreamPool: "main-pool",
			wantStatus: http.StatusOK,
			wantNextCalled: true,
			routes: []router.Route{
				{ID: "api-v1", Host: "api.example.com", PathPrefix: "/api/v1", UpstreamPool: "main-pool"},
			},
		},
		{
			name: "path normalized",
			host: "api.example.com",
			path: " /api/v1 ",
			routeID: "api-v1",
			upstreamPool: "main-pool",
			wantStatus: http.StatusOK,
			wantNextCalled: true,
			routes: []router.Route{
				{ID: "api-v1", Host: "api.example.com", PathPrefix: "/api/v1", UpstreamPool: "main-pool"},
			},
		},
		{
			name: "invalid host",
			host: "invalid",
			path: "/api/v1",
			routeID: "api-v1",
			upstreamPool: "main-pool",
			wantStatus: http.StatusNotFound,
			wantNextCalled: false,
			routes: []router.Route{
				{ID: "api-v1", Host: "api.example.com", PathPrefix: "/api/v1", UpstreamPool: "main-pool"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router, err := router.NewRouter(tt.routes)
			if err != nil {
				t.Fatal(err)
			}
			
			var gotRouteID string
			var gotUpstreamPool string
			nextCalled := false
			deps := Deps{
				Router: router,
				Next: func(w http.ResponseWriter, r *http.Request) {
					rc, ok := routectx.FromContext(r.Context())
					if !ok {
						t.Fatal("route context not set")
					}
					gotRouteID = rc.RouteID
					gotUpstreamPool = rc.UpstreamPool
					nextCalled = true
					w.WriteHeader(http.StatusOK)
				},
			}
			h, err := NewHandlers(deps)
			if err != nil {
				t.Fatal(err)
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = tt.host
			req.URL.Path = tt.path

			h.Root(w, req)

			if nextCalled != tt.wantNextCalled {
				t.Fatalf("next called = %t, want %t", nextCalled, tt.wantNextCalled)
			}
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantNextCalled && gotRouteID != tt.routeID {
				t.Fatalf("route id = %s, want %s", gotRouteID, tt.routeID)
			}
			if tt.wantNextCalled && gotUpstreamPool != tt.upstreamPool {
				t.Fatalf("upstream pool = %s, want %s", gotUpstreamPool, tt.upstreamPool)
			}
		})
	}
}