package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/eann1s/rate-limiter/backend/internal/app"
	"github.com/eann1s/rate-limiter/backend/internal/config"
	"github.com/eann1s/rate-limiter/backend/internal/middleware"
	"github.com/eann1s/rate-limiter/backend/internal/obs"
	"github.com/eann1s/rate-limiter/backend/internal/readiness"
	"github.com/eann1s/rate-limiter/backend/internal/router"
	http_admin "github.com/eann1s/rate-limiter/backend/internal/transport/http/admin"
	http_public "github.com/eann1s/rate-limiter/backend/internal/transport/http/public"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

var version = "dev"

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "./config.yml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	log, err := obs.NewLogger(cfg.Observability.Logs.Level, version)
	if err != nil {
		return err
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	routes := toRouterRoutes(cfg.Routes)
	router, err := router.NewRouter(routes)
	if err != nil {
		return err
	}

	readiness := &readiness.AtomicReadiness{}
	publicSrv, err := newPublicSrv(log, cfg, router)
	if err != nil {
		return err
	}
	adminSrv, err := newAdminSrv(log, cfg, readiness, reg)
	if err != nil {
		return err
	}
	app := app.NewApp(readiness, cfg, log, publicSrv, adminSrv)

	log.Info().Msg("starting gateway")
	
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil {
		log.Error().Err(err).Msg("shutdown with error")
		return err
	}

	log.Info().Msg("shutdown complete")
	return nil
}

func newPublicSrv(log zerolog.Logger, cfg config.Config, router *router.Router) (*http.Server, error) {
	deps := http_public.Deps{
		Router: router, 
		Next: func(w http.ResponseWriter, r *http.Request) {},
	}
	h, err := http_public.NewHandlers(deps)
	if err != nil {
		return nil, err
	}
	mux := http_public.NewPublicMux(h)
	handler := middleware.Chain(mux, middleware.RequestID, middleware.AccessLog(log))
	return &http.Server{
		Handler: handler,
		Addr: cfg.Listeners.Public.Addr,
		ReadTimeout: cfg.Defaults.Timeouts.Request,
		ReadHeaderTimeout: cfg.Defaults.Timeouts.UpstreamResponseHeader,
	}, nil
}

func newAdminSrv(log zerolog.Logger, cfg config.Config, readiness readiness.Readiness, reg *prometheus.Registry) (*http.Server, error) {
	deps := http_admin.Deps{
		Ready: readiness.IsReady,
		Metrics: promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
	}
	h := http_admin.NewHandlers(deps)
	mux := http_admin.NewAdminMux(h)
	handler := middleware.Chain(mux, middleware.RequestID, middleware.AccessLog(log))
	return &http.Server{
		Handler: handler,
		Addr: cfg.Listeners.Admin.Addr,
		ReadTimeout: cfg.Defaults.Timeouts.Request,
		ReadHeaderTimeout: cfg.Defaults.Timeouts.UpstreamResponseHeader,
	}, nil
}

func toRouterRoutes(routes []config.RouteConfig) []router.Route {
	rr := make([]router.Route, len(routes))
	for i, r := range routes {
		rr[i] = router.Route{
			ID: r.ID,
			Host: r.Host,
			PathPrefix: r.PathPrefix,
			UpstreamPool: r.UpstreamPool,
		}
	}
	return rr
}

