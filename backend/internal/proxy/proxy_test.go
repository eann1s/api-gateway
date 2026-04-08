package proxy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/eann1s/rate-limiter/backend/internal/config"
	"github.com/eann1s/rate-limiter/backend/internal/requestid"
	"github.com/eann1s/rate-limiter/backend/internal/routectx"
	"github.com/rs/zerolog"
)


type fakeRoundTripper struct {
	gotReq *http.Request
	response *http.Response
	roundTripFn func(*http.Request) (*http.Response, error)
}

func (f *fakeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	f.gotReq = r
	if f.roundTripFn != nil {
		return f.roundTripFn(r)
	}
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	reader := io.NopCloser(strings.NewReader(string(bytes)))
	r.Body = reader
	return f.response, nil
}

func TestProxy_NewProxy(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		config config.Config
		logger zerolog.Logger
		client *http.Client
		targetResolver func(string) ([]string, error)
		targetSelector func(string, []string) (string, error)
		wantErr error
	} {
		{
			name: "valid",
			config: config.Config{
				UpstreamPools: []config.UpstreamPoolConfig{
					{
						ID: "pool1",
						Targets: []string{
							"http://localhost:8080",
							"http://localhost:8081",
						},
					},
				},
			},
			client: http.DefaultClient,
			logger: zerolog.Nop(),
			targetResolver: func(string) ([]string, error) {
				return []string{
					"http://localhost:8080",
					"http://localhost:8081",
				}, nil
			},
			targetSelector: func(string, []string) (string, error) {
				return "http://localhost:8080", nil
			},
		},
		{
			name: "invalid client",
			config: config.Config{},
			logger: zerolog.Nop(),
			targetResolver: func(string) ([]string, error) {
				return nil, nil
			},
			targetSelector: func(string, []string) (string, error) {
				return "", nil
			},
			wantErr: ErrInvalidDeps,
		},
		{
			name: "invalid target resolver",
			config: config.Config{},
			client: http.DefaultClient,
			targetSelector: func(string, []string) (string, error) {
				return "", nil
			},
			wantErr: ErrInvalidDeps,
		},
		{
			name: "invalid target selector",
			config: config.Config{},
			client: http.DefaultClient,
			targetResolver: func(string) ([]string, error) {
				return nil, nil
			},
			wantErr: ErrInvalidDeps,
		},

	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewProxy(Deps{
				Config: tt.config,
				Logger: tt.logger,
				Client: tt.client,
				TargetResolver: tt.targetResolver,
				TargetSelector: tt.targetSelector,
			})

			if tt.wantErr != nil && err == nil {
				t.Errorf("expected error, got nil")
			}
			if tt.wantErr == nil && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestProxyNext(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		routeID string
		upstreamPool string
		reqID string
		reqHost string
		reqRemoteAddr string
		reqMethod string
		reqPath string
		reqHeader http.Header
		reqBody string
		respHeader http.Header
		respBody string
		reqContentLength int
		respStatusCode int
		config config.Config
		targetResolver func(string) ([]string, error)
		targetSelector func(string, []string) (string, error)
		roundTripFn func(*http.Request) (*http.Response, error)
		wantUpstreamResponse bool
		wantRespStatusCode int
		wantUrl string
		wantXForwardedFor string
		wantXForwardedHost string
		wantXForwardedProto string
	} {
		{
			name: "success",
			routeID: "route1",
			upstreamPool: "pool1",
			reqID: "123",
			reqHost: "localhost",
			reqRemoteAddr: "127.0.0.1",
			reqMethod: http.MethodGet,
			reqPath: "/api/v1?foo=bar",
			reqHeader: map[string][]string{
				"Accept": {"application/json"},
				"X-Forwarded-For": {"localhost"},
				"Connection": {"close"},
			},
			reqBody: "request body",
			respHeader: map[string][]string{
				"Content-Type": {"application/json"},
				"Connection": {"close"},
			},
			respBody: "response body",
			respStatusCode: http.StatusOK,
			config: config.Config{
				Defaults: config.DefaultsConfig{
					BodyLimit: 100,
				},
				UpstreamPools: []config.UpstreamPoolConfig{
						{
							ID: "pool1",
							Targets: []string{
								"http://localhost:8080",
								"http://localhost:8081",
							},
						},
					},
			},
			targetResolver: func(string) ([]string, error) {
				return []string{
					"http://localhost:8080",
					"http://localhost:8081",
				}, nil
			},
			targetSelector: func(string, []string) (string, error) {
				return "http://localhost:8080", nil
			},
			wantUpstreamResponse: true,
			wantRespStatusCode: http.StatusOK,
			wantUrl: "http://localhost:8080/api/v1?foo=bar",
			wantXForwardedFor: "localhost,127.0.0.1",
			wantXForwardedHost: "localhost",
			wantXForwardedProto: "http",
		},
		{
			name: "fail when missing upstream pool in context",
			routeID: "route1",
			reqID: "123",
			reqHost: "localhost",
			reqRemoteAddr: "127.0.0.1",
			reqMethod: http.MethodGet,
			reqPath: "/api/v1?foo=bar",
			reqHeader: map[string][]string{},
			reqBody: "request body",
			config: config.Config{
				Defaults: config.DefaultsConfig{
					BodyLimit: 100,
				},
				UpstreamPools: []config.UpstreamPoolConfig{},
			},
			targetResolver: func(string) ([]string, error) {
				return []string{
					"http://localhost:8080",
					"http://localhost:8081",
				}, nil
			},
			targetSelector: func(string, []string) (string, error) {
				return "http://localhost:8080", nil
			},
			wantUpstreamResponse: false,
			wantRespStatusCode: http.StatusInternalServerError,
		},
		{
			name: "fail when target resolver could not resolve targets",
			routeID: "route1",
			upstreamPool: "pool1",
			reqID: "123",
			reqHost: "localhost",
			reqRemoteAddr: "127.0.0.1",
			reqMethod: http.MethodGet,
			reqPath: "/api/v1?foo=bar",
			reqHeader: map[string][]string{},
			reqBody: "request body",
			config: config.Config{
				Defaults: config.DefaultsConfig{
					BodyLimit: 100,
				},
				UpstreamPools: []config.UpstreamPoolConfig{
						{
							ID: "pool1",
							Targets: []string{
								"http://localhost:8080",
								"http://localhost:8081",
							},
						},
					},
			},
			targetResolver: func(s string) ([]string, error) {
				return []string{}, errors.New("boom")
			},
			targetSelector: func(string, []string) (string, error) {
				return "http://localhost:8080", nil
			},
			wantUpstreamResponse: false,
			wantRespStatusCode: http.StatusBadGateway,
		},
		{
			name: "fail when target selector could not select target",
			routeID: "route1",
			upstreamPool: "pool1",
			reqID: "123",
			reqHost: "localhost",
			reqRemoteAddr: "127.0.0.1",
			reqMethod: http.MethodGet,
			reqPath: "/api/v1?foo=bar",
			reqHeader: map[string][]string{},
			reqBody: "request body",
			config: config.Config{
				Defaults: config.DefaultsConfig{
					BodyLimit: 100,
				},
				UpstreamPools: []config.UpstreamPoolConfig{
						{
							ID: "pool1",
							Targets: []string{
								"http://localhost:8080",
								"http://localhost:8081",
							},
						},
					},
			},
			targetResolver: func(string) ([]string, error) {
				return []string{
					"http://localhost:8080",
					"http://localhost:8081",
				}, nil
			},
			targetSelector: func(s1 string, s2 []string) (string, error) {
				return "", errors.New("boom")
			},
			wantUpstreamResponse: false,
			wantRespStatusCode: http.StatusBadGateway,
		},
		{
			name: "fail when url parse failed",
			routeID: "route1",
			upstreamPool: "pool1",
			reqID: "123",
			reqHost: "localhost",
			reqRemoteAddr: "127.0.0.1",
			reqMethod: http.MethodGet,
			reqPath: "/api/v1?foo=bar",
			reqHeader: map[string][]string{},
			reqBody: "request body",
			config: config.Config{
				Defaults: config.DefaultsConfig{
					BodyLimit: 100,
				},
				UpstreamPools: []config.UpstreamPoolConfig{
						{
							ID: "pool1",
							Targets: []string{
								"http://localhost:8080",
								"http://localhost:8081",
							},
						},
					},
			},
			targetResolver: func(string) ([]string, error) {
				return []string{
					"http://localhost:8080",
					"http://localhost:8081",
				}, nil
			},
			targetSelector: func(s1 string, s2 []string) (string, error) {
				return "http://[::1", nil
			},
			wantUpstreamResponse: false,
			wantRespStatusCode: http.StatusInternalServerError,
		},
		{
			name: "fail when content length too large",
			routeID: "route1",
			upstreamPool: "pool1",
			reqID: "123",
			reqHost: "localhost",
			reqRemoteAddr: "127.0.0.1",
			reqMethod: http.MethodGet,
			reqPath: "/api/v1?foo=bar",
			reqHeader: map[string][]string{},
			reqBody: "request body",
			reqContentLength: 101,
			config: config.Config{
				Defaults: config.DefaultsConfig{
					BodyLimit: 100,
				},
				UpstreamPools: []config.UpstreamPoolConfig{
						{
							ID: "pool1",
							Targets: []string{
								"http://localhost:8080",
								"http://localhost:8081",
							},
						},
					},
			},
			targetResolver: func(string) ([]string, error) {
				return []string{
					"http://localhost:8080",
					"http://localhost:8081",
				}, nil
			},
			targetSelector: func(s1 string, s2 []string) (string, error) {
				return "http://localhost:8080", nil
			},
			wantUpstreamResponse: false,
			wantRespStatusCode: http.StatusRequestEntityTooLarge,
		},
		{
			name: "fail when request body too large",
			routeID: "route1",
			upstreamPool: "pool1",
			reqID: "123",
			reqHost: "localhost",
			reqRemoteAddr: "127.0.0.1",
			reqMethod: http.MethodGet,
			reqPath: "/api/v1?foo=bar",
			reqHeader: map[string][]string{},
			reqBody: "super big request body",
			reqContentLength: -1,
			respStatusCode: 200,
			config: config.Config{
				Defaults: config.DefaultsConfig{
					BodyLimit: 10,
				},
				UpstreamPools: []config.UpstreamPoolConfig{
						{
							ID: "pool1",
							Targets: []string{
								"http://localhost:8080",
								"http://localhost:8081",
							},
						},
					},
			},
			targetResolver: func(string) ([]string, error) {
				return []string{
					"http://localhost:8080",
					"http://localhost:8081",
				}, nil
			},
			targetSelector: func(s1 string, s2 []string) (string, error) {
				return "http://localhost:8080", nil
			},
			wantUpstreamResponse: false,
			wantRespStatusCode: http.StatusRequestEntityTooLarge,
		},
		{
			name: "fail early when content length too large",
			routeID: "route1",
			upstreamPool: "pool1",
			reqID: "123",
			reqHost: "localhost",
			reqRemoteAddr: "127.0.0.1",
			reqMethod: http.MethodGet,
			reqPath: "/api/v1?foo=bar",
			reqHeader: map[string][]string{},
			reqBody: "request body",
			reqContentLength: 100,
			respStatusCode: 200,
			config: config.Config{
				Defaults: config.DefaultsConfig{
					BodyLimit: 10,
				},
				UpstreamPools: []config.UpstreamPoolConfig{
						{
							ID: "pool1",
							Targets: []string{
								"http://localhost:8080",
								"http://localhost:8081",
							},
						},
					},
			},
			targetResolver: func(string) ([]string, error) {
				return []string{
					"http://localhost:8080",
					"http://localhost:8081",
				}, nil
			},
			targetSelector: func(s1 string, s2 []string) (string, error) {
				return "http://localhost:8080", nil
			},
			wantUpstreamResponse: false,
			wantRespStatusCode: http.StatusRequestEntityTooLarge,
		},
		{
			name: "fail when upstream deadline exceeded",
			routeID: "route1",
			upstreamPool: "pool1",
			reqID: "123",
			reqHost: "localhost",
			reqRemoteAddr: "127.0.0.1",
			reqMethod: http.MethodGet,
			reqPath: "/api/v1?foo=bar",
			reqHeader: map[string][]string{},
			reqBody: "request body",
			reqContentLength: 10,
			respStatusCode: 200,
			config: config.Config{
				Defaults: config.DefaultsConfig{
					BodyLimit: 100,
				},
				UpstreamPools: []config.UpstreamPoolConfig{
						{
							ID: "pool1",
							Targets: []string{
								"http://localhost:8080",
								"http://localhost:8081",
							},
						},
					},
			},
			targetResolver: func(string) ([]string, error) {
				return []string{
					"http://localhost:8080",
					"http://localhost:8081",
				}, nil
			},
			targetSelector: func(s1 string, s2 []string) (string, error) {
				return "http://localhost:8080", nil
			},
			roundTripFn: func(r *http.Request) (*http.Response, error) {
				return nil, context.DeadlineExceeded
			},
			wantUpstreamResponse: false,
			wantRespStatusCode: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeRT := &fakeRoundTripper{
				response: &http.Response{
					StatusCode: tt.respStatusCode,
					Body: io.NopCloser(strings.NewReader(tt.respBody)),
					Header: tt.respHeader,
				},
				roundTripFn: tt.roundTripFn,
			}

			deps := Deps{
				Config: tt.config,
				Logger: zerolog.Nop(),
				Client: &http.Client{
					Transport: fakeRT,
				},
				TargetResolver: tt.targetResolver,
				TargetSelector: tt.targetSelector,
			}
			proxy, err := NewProxy(deps)
			if err != nil {
				t.Fatal(err)
			}

			writer := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(
				routectx.WithRoute(
					requestid.WithContext(context.Background(), tt.reqID), 
					routectx.RouteContext{
						RouteID: tt.routeID,
						UpstreamPool: tt.upstreamPool,
					}),
				tt.reqMethod, 
				tt.reqPath, 
				strings.NewReader(tt.reqBody),
			)
			req.Host = tt.reqHost
			req.RemoteAddr = tt.reqRemoteAddr
			req.Header = tt.reqHeader
			req.ContentLength = int64(tt.reqContentLength)

			proxy.Next()(writer, req)

			if tt.wantUpstreamResponse {
				gotReq := fakeRT.gotReq
				if gotReq == nil {
					t.Error("expected request to be sent")
				}
				if gotReq.Method != tt.reqMethod {
					t.Errorf("expected method to be %v, got %v", tt.reqMethod, fakeRT.gotReq.Method)
				}
				if gotReq.URL.String() != tt.wantUrl {
					t.Errorf("expected URL to be %v, got %v", tt.wantUrl, fakeRT.gotReq.URL.String())
				}
				for key, values := range tt.reqHeader {
					if slices.Contains(hopHeaders, key) || key == "X-Forwarded-For" {
						continue
					}
					for _, value := range values {
						if gotReq.Header.Get(key) != value {
							t.Errorf("expected request header %v to be %v, got %v", key, tt.reqHeader.Get(key), gotReq.Header.Get(key))
						}
					}
				}
				if gotReq.Header.Get("X-Forwarded-For") != tt.wantXForwardedFor {
					t.Errorf("expected X-Forwarded-For to be %v, got %v", tt.wantXForwardedFor, fakeRT.gotReq.Header.Get("X-Forwarded-For"))
				}
				if gotReq.Header.Get("X-Forwarded-Host") != tt.wantXForwardedHost {
					t.Errorf("expected X-Forwarded-Host to be %v, got %v", tt.wantXForwardedHost, fakeRT.gotReq.Header.Get("X-Forwarded-Host"))
				}
				if gotReq.Header.Get("X-Forwarded-Proto") != tt.wantXForwardedProto {
					t.Errorf("expected X-Forwarded-Proto to be %v, got %v", tt.wantXForwardedProto, fakeRT.gotReq.Header.Get("X-Forwarded-Proto"))
				}
				if gotReq.Header.Get("X-Request-ID") != tt.reqID {
					t.Errorf("expected X-Request-ID to be %v, got %v", tt.reqID, fakeRT.gotReq.Header.Get("X-Request-ID"))
				}
				for _, h := range hopHeaders {
					if gotReq.Header.Get(h) != "" {
						t.Errorf("expected request header %v to be removed, got %v", h, fakeRT.gotReq.Header.Get(h))
					}
				}
				gotReqBody, err := io.ReadAll(gotReq.Body)
				if err != nil {
					t.Fatal(err)
				}
				if string(gotReqBody) != tt.reqBody {
					t.Errorf("expected request body to be %v, got %v", tt.reqBody, string(gotReqBody))
				}
			}

			gotResp := writer.Result()
			if gotResp.StatusCode != tt.wantRespStatusCode {
				t.Errorf("expected status code %v, got %v", tt.wantRespStatusCode, gotResp.StatusCode)
			}
			if tt.wantUpstreamResponse {
				for key, values := range tt.respHeader {
					if slices.Contains(hopHeaders, http.CanonicalHeaderKey(key)) {
						continue
					}
					for _, value := range values {
						if gotResp.Header.Get(key) != value {
							t.Errorf("expected response header %v to be %v, got %v", key, tt.respHeader.Get(key), gotResp.Header.Get(key))
						}
					}
				}
				for _, h := range hopHeaders {
					if gotResp.Header.Get(h) != "" {
						t.Errorf("expected response header %v to be removed, got %v", h, gotResp.Header.Get(h))
					}
				}
				gotRespBody, err := io.ReadAll(gotResp.Body)
				if err != nil {
					t.Fatal(err)
				}
				if string(gotRespBody) != tt.respBody {
					t.Errorf("expected response body to be %v, got %v", tt.respBody, string(gotRespBody))
				}
			}
		})
	}
}

