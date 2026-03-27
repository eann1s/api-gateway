package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/eann1s/rate-limiter/backend/internal/config"
	"github.com/eann1s/rate-limiter/backend/internal/readiness"
	"github.com/rs/zerolog"
)


func TestApp_Run_ReturnsNil_OnContextCancel(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Listeners: config.ListenersConfig{
			Public: config.PublicListenerConfig{ Addr: ":0" },
			Admin: config.AdminListenerConfig{ Addr: ":0" },
		},
		Shutdown: config.ShutdownConfig{ Timeout: 3 * time.Second },
	}
	logger := zerolog.Logger{}
	ready := &readiness.AtomicReadiness{}
	publicSrv := &http.Server{Handler: http.NewServeMux()}
	adminSrv := &http.Server{Handler: http.NewServeMux()}

	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())

	app := NewApp(ready, cfg, logger, publicSrv, adminSrv)

	go func() { errCh <- app.Run(ctx) }()
	cancel()

	select {
	case err := <-errCh:
		if err != nil { t.Fatalf("Run err: %v", err) }
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestApp_Run_SetsReadinessTrue_AndFalseOnShutdown(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Listeners: config.ListenersConfig{
			Public: config.PublicListenerConfig{ Addr: ":0" },
			Admin: config.AdminListenerConfig{ Addr: ":0" },
		},
		Shutdown: config.ShutdownConfig{ Timeout: 3 * time.Second },
	}
	logger := zerolog.Logger{}
	ready := &readiness.AtomicReadiness{}
	publicSrv := &http.Server{Handler: http.NewServeMux()}
	adminSrv := &http.Server{Handler: http.NewServeMux()}

	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())

	app := NewApp(ready, cfg, logger, publicSrv, adminSrv)

	go func() { errCh <- app.Run(ctx) }()
	waitForReadiness(t, ready, 2 * time.Second)
	cancel()

	select {
	case err := <-errCh:
		if err != nil { t.Fatalf("Run err: %v", err) }
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	if ready.IsReady() { t.Fatal("expected readiness to be false") }
}

func TestApp_Run_ReturnsError_OnPublicListenerError(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", ":0")
	if err != nil { t.Fatal(err) }
	busyAddr := listener.Addr().String()
	defer listener.Close()

	cfg := config.Config{
		Listeners: config.ListenersConfig{
			Public: config.PublicListenerConfig{ Addr: busyAddr },
			Admin: config.AdminListenerConfig{ Addr: ":0" },
		},
		Shutdown: config.ShutdownConfig{ Timeout: 3 * time.Second },
	}
	logger := zerolog.Logger{}
	ready := &readiness.AtomicReadiness{}
	publicSrv := &http.Server{Handler: http.NewServeMux()}
	adminSrv := &http.Server{Handler: http.NewServeMux()}

	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ready, cfg, logger, publicSrv, adminSrv)

	go func() { errCh <- app.Run(ctx) }()

	select {
	case err := <-errCh:
		if err != nil {
			var op *net.OpError
			if !errors.As(err, &op) || op.Op != "listen" {
				t.Fatalf("Run err: %v", err)
			}
		}
		if err == nil {
			t.Fatal("error is expected")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout after 3 seconds")
	}
}

func waitForReadiness(t *testing.T, ready readiness.Readiness, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ready.IsReady() { return }
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for readiness after %v", timeout)
}


