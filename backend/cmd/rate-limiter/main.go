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
	"github.com/eann1s/rate-limiter/backend/internal/obs"
	"github.com/eann1s/rate-limiter/backend/internal/readiness"
	http_admin "github.com/eann1s/rate-limiter/backend/internal/transport/http/admin"
	http_public "github.com/eann1s/rate-limiter/backend/internal/transport/http/public"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	readiness := &readiness.AtomicReadiness{}
	publicSrv := newPublicSrv(cfg)
	adminSrv := newAdminSrv(cfg, readiness, reg)
	app := app.NewApp(readiness, cfg, log, publicSrv, adminSrv)
	
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil {
		return err
	}

	return nil
}

func newPublicSrv(cfg config.Config) *http.Server {
	deps := http_public.Deps{}
	h := http_public.NewHandlers(deps)
	mux := http_public.NewPublicMux(h)
	return &http.Server{
		Handler: mux,
		Addr: cfg.Listeners.Public.Addr,
		ReadTimeout: cfg.Defaults.Timeouts.Request,
		ReadHeaderTimeout: cfg.Defaults.Timeouts.UpstreamResponseHeader,
	}
}

func newAdminSrv(cfg config.Config, readiness readiness.Readiness, reg *prometheus.Registry) *http.Server {
	deps := http_admin.Deps{
		Ready: readiness.IsReady,
		Metrics: promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
	}
	h := http_admin.NewHandlers(deps)
	mux := http_admin.NewAdminMux(h)
	return &http.Server{
		Handler: mux,
		Addr: cfg.Listeners.Admin.Addr,
		ReadTimeout: cfg.Defaults.Timeouts.Request,
		ReadHeaderTimeout: cfg.Defaults.Timeouts.UpstreamResponseHeader,
	}
}

