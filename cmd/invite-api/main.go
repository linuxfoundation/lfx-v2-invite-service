// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	appsvc "github.com/linuxfoundation/lfx-v2-invite-service/cmd/invite-api/service"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/infrastructure/observability"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
)

const gracefulShutdownSeconds = 25

// subscriptionFlushTimeout bounds the NATS round trip that confirms the broker
// has registered all subscriptions before the service reports ready.
const subscriptionFlushTimeout = 5 * time.Second

// errNotReady is returned by the readiness check until the server has finished
// starting (NATS connected, KV bound, subscriptions registered).
var errNotReady = errors.New("invite service not ready")

func init() {
	// Bootstrap with default log level; reconfigured in run() once AppConfigFromEnv loads LOG_LEVEL.
	observability.InitStructureLogConfig("")
}

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// Seed OTEL_SERVICE_VERSION before SDK init so autoexport picks it up via env.
	if os.Getenv("OTEL_SERVICE_VERSION") == "" {
		_ = os.Setenv("OTEL_SERVICE_VERSION", Version)
	}
	otelShutdown, err := observability.SetupOTelSDK(ctx)
	if err != nil {
		return err
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownSeconds*time.Second)
		defer cancel()
		if shutErr := otelShutdown(shutCtx); shutErr != nil {
			slog.ErrorContext(ctx, "error shutting down OpenTelemetry SDK", "error", shutErr)
		}
	}()

	slog.InfoContext(ctx, "starting invite service",
		"version", Version,
		"git_commit", GitCommit,
	)

	cfg := appsvc.AppConfigFromEnv()
	observability.InitStructureLogConfig(cfg.LogLevel)

	// Start the health server first so /livez answers while NATS connects.
	// /readyz reports not-ready until the server is wired and subscribed, then
	// tracks the NATS connection state. Bind the port up front so a busy or
	// invalid port fails startup immediately.
	healthListener, err := appsvc.ListenHealth(cfg.Port)
	if err != nil {
		return err
	}
	var readySrv atomic.Pointer[appsvc.Server]
	healthServer := &http.Server{
		Handler: appsvc.HealthHandler(func() error {
			s := readySrv.Load()
			if s == nil {
				return errNotReady
			}
			return s.Ready()
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	healthErr := make(chan error, 1)
	go func() {
		slog.InfoContext(ctx, "health server listening", "port", cfg.Port)
		if err := healthServer.Serve(healthListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			healthErr <- err
		}
	}()
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutErr := healthServer.Shutdown(shutCtx); shutErr != nil {
			slog.ErrorContext(ctx, "error shutting down health server", "error", shutErr)
		}
	}()

	srv, err := appsvc.NewServerFromConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer srv.Close() //nolint:errcheck

	stops, err := srv.Start(ctx)
	if err != nil {
		return err
	}
	defer func() {
		for _, stop := range stops {
			stop()
		}
	}()

	// QueueSubscribe is asynchronous: confirm the broker has registered every
	// subscription before /readyz can report ready.
	if err := srv.Flush(subscriptionFlushTimeout); err != nil {
		return fmt.Errorf("flush NATS subscriptions: %w", err)
	}

	readySrv.Store(srv)
	slog.InfoContext(ctx, "invite service ready")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-quit:
		slog.InfoContext(ctx, "received shutdown signal, stopping", "signal", sig.String())
		return nil
	case err := <-healthErr:
		return fmt.Errorf("health server failed: %w", err)
	}
}
