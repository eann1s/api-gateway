package app

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/eann1s/rate-limiter/backend/internal/config"
	"github.com/eann1s/rate-limiter/backend/internal/readiness"
	"github.com/rs/zerolog"
)


type App struct {
	cfg config.Config
	logger zerolog.Logger
	public *http.Server
	admin *http.Server
	readiness readiness.Readiness
}

func NewApp(readiness readiness.Readiness, cfg config.Config, logger zerolog.Logger, publicSrv *http.Server, adminSrv *http.Server) *App {
	return &App{
		cfg: cfg,
		logger: logger,
		readiness: readiness,
		public: publicSrv,
		admin: adminSrv,
	}
}

func (app *App) Run(ctx context.Context) error {
	errCh := make(chan error, 2)

	publicLn, err := net.Listen("tcp", app.cfg.Listeners.Public.Addr)
	if err != nil {
		return err
	}

	adminLn, err := net.Listen("tcp", app.cfg.Listeners.Admin.Addr)
	if err != nil {
		errr := publicLn.Close()
		if errr != nil {
			return errors.Join(err, errr)
		}
		return err
	}

	app.readiness.SetReady(true)

	go func() {
		err := app.public.Serve(publicLn)
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	go func() {
		err := app.admin.Serve(adminLn)
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		if errr := app.shutdownWithTimeout(); errr != nil {
			return errors.Join(err, errr)
		}
		return err
	case <-ctx.Done():
		if err := app.shutdownWithTimeout(); err != nil {
			return err
		}
	}

	return nil
}

func (app *App) Shutdown(ctx context.Context) error {
	app.readiness.SetReady(false)

	var publicErr error
	var adminErr error
	if err := app.public.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
		publicErr = err
	}
	if err := app.admin.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
		adminErr = err
	}
	return errors.Join(publicErr, adminErr)
}

func (app *App) shutdownWithTimeout() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), app.cfg.Shutdown.Timeout)
	defer cancel()
	return app.Shutdown(shutdownCtx)
}


