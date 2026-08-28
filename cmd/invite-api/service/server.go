// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"

	emailapi "github.com/linuxfoundation/lfx-v2-email-service/pkg/api"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
	authinfra "github.com/linuxfoundation/lfx-v2-invite-service/internal/infrastructure/auth"
	natsinfra "github.com/linuxfoundation/lfx-v2-invite-service/internal/infrastructure/nats"
	intsvc "github.com/linuxfoundation/lfx-v2-invite-service/internal/service"
)

// Server holds all wired dependencies for the invite service and owns the
// subscription lifecycle. Construct with NewServerFromConfig for production use,
// or NewServer to inject dependencies directly (e.g. in tests).
type Server struct {
	subscriber port.Subscriber
	closeFn    func() // shuts down the underlying transport connection
	notifSvc   *intsvc.NotificationService
	acceptSvc  *intsvc.AcceptanceService
	readSvc    *intsvc.InviteReadService
}

// NewServer constructs a Server from pre-built dependencies. The closeFn is
// called by Close to release any underlying connection held by the subscriber.
// Pass a no-op func when the subscriber manages its own lifecycle.
func NewServer(
	subscriber port.Subscriber,
	closeFn func(),
	notifSvc *intsvc.NotificationService,
	acceptSvc *intsvc.AcceptanceService,
	readSvc *intsvc.InviteReadService,
) *Server {
	if closeFn == nil {
		closeFn = func() {}
	}
	return &Server{
		subscriber: subscriber,
		closeFn:    closeFn,
		notifSvc:   notifSvc,
		acceptSvc:  acceptSvc,
		readSvc:    readSvc,
	}
}

// NewServerFromConfig constructs a Server by building all infrastructure from cfg.
// Config validation runs before any network connection is attempted so startup
// fails fast on misconfiguration. Returns an error if required config values are
// missing or invalid, if the NATS connection fails, or if the KV bucket cannot
// be bound.
func NewServerFromConfig(ctx context.Context, cfg AppConfig) (*Server, error) {
	// Validate required config before attempting any network connections.
	if cfg.InviteJWTSecret == "" {
		return nil, fmt.Errorf("INVITE_JWT_SECRET is required but not set")
	}
	if len(cfg.InviteJWTSecret) < 32 {
		return nil, fmt.Errorf("INVITE_JWT_SECRET must be at least 32 bytes for HS256 (got %d)", len(cfg.InviteJWTSecret))
	}

	nc, err := natsinfra.New(ctx, cfg.NATSURL)
	if err != nil {
		return nil, err
	}

	invitesKV, err := nc.KeyValue(ctx, cfg.InvitesKVBucket)
	if err != nil {
		nc.Close() // prevent socket leak — no Server exists yet for the caller to close
		return nil, fmt.Errorf("bind invites KV bucket %q: %w", cfg.InvitesKVBucket, err)
	}
	inviteStore := natsinfra.NewNATSInviteRepository(invitesKV)

	linkGen := authinfra.NewLinkGenerator([]byte(cfg.InviteJWTSecret), cfg.SelfServeBaseURL)
	emailSender := natsinfra.NewNATSEmailSender(nc, emailapi.SendEmailSubject)

	notifSvc := intsvc.NewNotificationService(
		emailSender,
		linkGen,
		inviteStore,
		intsvc.NotificationConfig{
			DefaultReturnURL:      cfg.DefaultReturnURL,
			AllowedReturnURLHosts: cfg.AllowedReturnURLHosts,
		},
	)
	acceptSvc := intsvc.NewAcceptanceService(inviteStore, nc)
	readSvc := intsvc.NewInviteReadService(inviteStore)

	slog.InfoContext(ctx, "infrastructure initialised",
		"invites_kv_bucket", cfg.InvitesKVBucket,
	)

	return NewServer(nc, nc.Close, notifSvc, acceptSvc, readSvc), nil
}

// Close shuts down the underlying transport connection and satisfies io.Closer.
func (s *Server) Close() error {
	s.closeFn()
	return nil
}
