// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
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

	if err := appsvc.InitInfrastructure(ctx, cfg); err != nil {
		return err
	}
	defer appsvc.Shutdown()

	stops, err := appsvc.StartSubscriptions(ctx)
	if err != nil {
		return err
	}
	defer func() {
		for _, stop := range stops {
			stop()
		}
	}()

	slog.InfoContext(ctx, "invite service ready")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.InfoContext(ctx, "received shutdown signal, stopping", "signal", sig.String())
	return nil
}
