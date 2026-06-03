// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"

	emailapi "github.com/linuxfoundation/lfx-v2-email-service/pkg/api"
	authinfra "github.com/linuxfoundation/lfx-v2-invite-service/internal/infrastructure/auth"
	natsinfra "github.com/linuxfoundation/lfx-v2-invite-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/service"
)

// NATSClient is the shared NATS connection used by all infrastructure adapters.
var NATSClient *natsinfra.Client

// NotificationSvc is the wired notification service.
var NotificationSvc *service.NotificationService

// AcceptanceSvc is the wired invite-acceptance handler.
var AcceptanceSvc *service.AcceptanceService

// InviteReadSvc is the wired invite read service.
var InviteReadSvc *service.InviteReadService

// InitInfrastructure initialises all infrastructure dependencies from cfg.
// Must be called once during startup before StartSubscriptions.
func InitInfrastructure(ctx context.Context, cfg AppConfig) error {
	nc, err := natsinfra.New(ctx, cfg.NATSURL)
	if err != nil {
		return err
	}
	NATSClient = nc

	if cfg.InviteJWTSecret == "" {
		return fmt.Errorf("INVITE_JWT_SECRET is required but not set")
	}
	if len(cfg.InviteJWTSecret) < 32 {
		return fmt.Errorf("INVITE_JWT_SECRET must be at least 32 bytes for HS256 (got %d)", len(cfg.InviteJWTSecret))
	}

	// Bind to the invites KV bucket. The bucket must already exist (provisioned by
	// Helm via the nack KeyValue CRD, or with `nats kv add invites` for local dev).
	invitesKV, err := nc.KeyValue(ctx, cfg.InvitesKVBucket)
	if err != nil {
		return fmt.Errorf("bind invites KV bucket %q: %w", cfg.InvitesKVBucket, err)
	}
	inviteStore := natsinfra.NewNATSInviteRepository(invitesKV)

	linkGen := authinfra.NewLinkGenerator([]byte(cfg.InviteJWTSecret), cfg.SelfServeBaseURL)
	emailSender := natsinfra.NewNATSEmailSender(nc, emailapi.SendEmailSubject)

	NotificationSvc = service.NewNotificationService(
		emailSender,
		linkGen,
		inviteStore,
		service.NotificationConfig{
			DefaultReturnURL:      cfg.DefaultReturnURL,
			AllowedReturnURLHosts: cfg.AllowedReturnURLHosts,
		},
	)

	AcceptanceSvc = service.NewAcceptanceService(inviteStore)
	InviteReadSvc = service.NewInviteReadService(inviteStore)

	slog.InfoContext(ctx, "infrastructure initialised",
		"invites_kv_bucket", cfg.InvitesKVBucket,
	)
	return nil
}

// Shutdown gracefully closes all infrastructure connections.
func Shutdown() {
	if NATSClient != nil {
		NATSClient.Close()
	}
}
