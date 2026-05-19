// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"

	emailapi "github.com/linuxfoundation/lfx-v2-email-service/pkg/api"
	natsinfra "github.com/linuxfoundation/lfx-v2-invite-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/service"
)

// NATSClient is the shared NATS connection used by all infrastructure adapters.
var NATSClient *natsinfra.Client

// NotificationSvc is the wired notification service.
var NotificationSvc *service.NotificationService

// InitInfrastructure initialises all infrastructure dependencies from cfg.
// Must be called once during startup before StartSubscriptions.
func InitInfrastructure(ctx context.Context, cfg AppConfig) error {
	nc, err := natsinfra.New(ctx, cfg.NATSURL)
	if err != nil {
		return err
	}
	NATSClient = nc

	emailSender := natsinfra.NewNATSEmailSender(nc, emailapi.SendEmailSubject)

	NotificationSvc = service.NewNotificationService(
		emailSender,
		service.NotificationConfig{
			LFXBaseURL: cfg.LFXBaseURL,
		},
	)

	slog.InfoContext(ctx, "infrastructure initialised")
	return nil
}

// Shutdown gracefully closes all infrastructure connections.
func Shutdown() {
	if NATSClient != nil {
		NATSClient.Close()
	}
}
