package proxy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/eann1s/rate-limiter/backend/internal/config"
	"github.com/eann1s/rate-limiter/backend/internal/requestid"
	"github.com/eann1s/rate-limiter/backend/internal/routectx"
	"github.com/rs/zerolog"
)

var (
	ErrInvalidDeps = errors.New("invalid dependencies")
)

var hopHeaders = []string{
	http.CanonicalHeaderKey("Connection"),
	http.CanonicalHeaderKey("Proxy-Connection"),
	http.CanonicalHeaderKey("Keep-Alive"),
	http.CanonicalHeaderKey("Proxy-Authenticate"),
	http.CanonicalHeaderKey("Proxy-Authorization"),
	http.CanonicalHeaderKey("Te"),
	http.CanonicalHeaderKey("Trailers"),
	http.CanonicalHeaderKey("Trailer"),
	http.CanonicalHeaderKey("Transfer-Encoding"),
	http.CanonicalHeaderKey("Upgrade"),
}

type Deps struct {
	Config         config.Config
	Logger         zerolog.Logger
	Client         *http.Client
	TargetResolver func(string) ([]string, error)
	TargetSelector func(string, []string) (string, error)
}

type Proxy struct {
	deps Deps
}

func NewProxy(deps Deps) (*Proxy, error) {
	if deps.Client == nil {
		return nil, fmt.Errorf("%w, http client is required", ErrInvalidDeps)
	}
	if deps.TargetResolver == nil {
		return nil, fmt.Errorf("%w, target resolver is required", ErrInvalidDeps)
	}
	if deps.TargetSelector == nil {
		return nil, fmt.Errorf("%w, target selector is required", ErrInvalidDeps)
	}
	return &Proxy{
		deps: deps,
	}, nil
}

func (p *Proxy) Next() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		upstreamPool, ok := routectx.UpstreamPoolFromContext(r.Context())
		if !ok || upstreamPool == "" {
			p.deps.Logger.Error().Msg("missing upstream pool")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		targets, err := p.deps.TargetResolver(upstreamPool)
		if err != nil {
			p.deps.Logger.Error().Err(err).Msg("failed to resolve targets")
			w.WriteHeader(http.StatusBadGateway)
			return
		}

		target, err := p.deps.TargetSelector(upstreamPool, targets)
		if err != nil {
			p.deps.Logger.Error().Err(err).Msg("failed to select target")
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		p.deps.Logger.Debug().Str("target", target).Msg("target selected")

		u, err := url.Parse(target)
		if err != nil || u == nil {
			p.deps.Logger.Error().Err(err).Msg("failed to build target url")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		u = u.JoinPath(r.URL.Path)
		u.RawQuery = r.URL.RawQuery

		if r.ContentLength > 0 && r.ContentLength > int64(p.deps.Config.Defaults.BodyLimit) {
			p.deps.Logger.Error().Msg("request body too large")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}

		reqID, ok := requestid.FromContext(r.Context())
		if !ok || reqID == "" {
			reqID = requestid.New()
		}

		body, err := newLimitedBody(r.Body, int64(p.deps.Config.Defaults.BodyLimit))
		if err != nil {
			p.deps.Logger.Error().Err(err).Msg("failed to limit request body")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		request, err := http.NewRequestWithContext(
			requestid.WithContext(r.Context(), reqID),
			r.Method,
			u.String(),
			body,
		)
		if err != nil {
			p.deps.Logger.Error().Err(err).Msg("failed to compose request")
			w.WriteHeader(http.StatusBadGateway)
			return
		}

		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || ip == "" {
			parsed := net.ParseIP(r.RemoteAddr)
			if parsed != nil {
				ip = parsed.String()
			}
			p.deps.Logger.Warn().Err(err).Msg("failed to parse remote address")
		}

		request.Header = r.Header.Clone()
		existingXForwardedFor := request.Header.Get("X-Forwarded-For")
		if existingXForwardedFor != "" && ip != "" {
			request.Header.Set("X-Forwarded-For", existingXForwardedFor+","+ip)
		} else if ip != "" {
			request.Header.Set("X-Forwarded-For", ip)
		}
		proto := "http"
		if r.TLS != nil {
			proto = "https"
		}
		request.Header.Set("X-Forwarded-Proto", proto)
		request.Header.Set("X-Forwarded-Host", r.Host)
		request.Header.Set("X-Request-ID", reqID)
		removeHopHeaders(request.Header)

		resp, err := p.deps.Client.Do(request)
		if errors.Is(err, ErrRequestBodyTooLarge) {
			p.deps.Logger.Error().Err(err).Msg("request body too large")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		if err != nil || resp == nil {
			p.deps.Logger.Error().Err(err).Msg("failed to forward request")
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(http.CanonicalHeaderKey(key), value)
			}
		}
		removeHopHeaders(w.Header())
		w.WriteHeader(resp.StatusCode)
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			p.deps.Logger.Error().Err(err).Msg("failed to write response body")
			return
		}
	}
}

func removeHopHeaders(header http.Header) {
	connectionHeaderVal := header.Get("Connection")
	parts := strings.Split(connectionHeaderVal, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		header.Del(part)
	}
	for _, h := range hopHeaders {
		header.Del(h)
	}
}
