// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	appsvc "github.com/linuxfoundation/lfx-v2-invite-service/cmd/invite-api/service"
	logging "github.com/linuxfoundation/lfx-v2-invite-service/pkg/log"
	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/utils"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

const gracefulShutdownSeconds = 25

func init() {
	logging.InitStructureLogConfig()
}

func main() {
	ctx := context.Background()

	otelConfig := utils.OTelConfigFromEnv(ctx)
	if otelConfig.ServiceVersion == "" {
		otelConfig.ServiceVersion = Version
	}
	otelShutdown, err := utils.SetupOTelSDKWithConfig(ctx, otelConfig)
	if err != nil {
		slog.ErrorContext(ctx, "error setting up OpenTelemetry SDK", "error", err)
		os.Exit(1)
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
		"build_time", BuildTime,
		"git_commit", GitCommit,
	)

	cfg := appsvc.AppConfigFromEnv()

	if err := appsvc.InitInfrastructure(ctx, cfg); err != nil {
		slog.ErrorContext(ctx, "failed to initialise infrastructure", "error", err)
		os.Exit(1)
	}
	defer appsvc.Shutdown()

	stops, err := appsvc.StartSubscriptions(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to start subscriptions", "error", err)
		os.Exit(1)
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

	slog.InfoContext(ctx, "received shutdown signal, stopping", "signal", fmt.Sprintf("%s", sig))
}
