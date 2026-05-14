// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/constants"
)

// ProjectSettingsHandler is the function signature for handling a decoded event.
type ProjectSettingsHandler func(ctx context.Context, msg *model.ProjectSettingsUpdatedMessage) error

// SendInviteHandler is the function signature for handling a decoded send-invite request.
type SendInviteHandler func(ctx context.Context, req *model.SendInviteRequest) error

// StartProjectSettingsConsumer binds a durable JetStream consumer on the
// project-settings-events stream and delivers decoded messages to handler.
// Returns a stop function the caller must invoke on shutdown.
func (c *Client) StartProjectSettingsConsumer(
	ctx context.Context,
	handler ProjectSettingsHandler,
) (func(), error) {
	cfg := jetstream.ConsumerConfig{
		Name:    constants.ConsumerNameProjectSettingsNotify,
		Durable: constants.ConsumerNameProjectSettingsNotify,
		FilterSubjects: []string{
			constants.ProjectSettingsUpdatedSubject,
		},
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverNewPolicy,
		MaxDeliver:    5,
		AckWait:       30 * time.Second,
	}

	return c.ConsumeWithJetStream(ctx, constants.StreamNameProjectSettingsEvents, cfg,
		func(ctx context.Context, subject string, data []byte) error {
			var msg model.ProjectSettingsUpdatedMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				slog.ErrorContext(ctx, "failed to unmarshal project_settings.updated payload",
					"subject", subject,
					"error", err,
				)
				// Returning nil so the message is ACKed and not retried — malformed
				// payloads will never parse successfully.
				return nil
			}
			return handler(ctx, &msg)
		},
	)
}

// StartSendInviteConsumer binds a durable JetStream consumer on the invite-requests
// stream and delivers decoded send-invite requests to handler.
// Returns a stop function the caller must invoke on shutdown.
func (c *Client) StartSendInviteConsumer(
	ctx context.Context,
	handler SendInviteHandler,
) (func(), error) {
	cfg := jetstream.ConsumerConfig{
		Name:    constants.ConsumerNameInviteRequestsHandler,
		Durable: constants.ConsumerNameInviteRequestsHandler,
		FilterSubjects: []string{
			constants.SendInviteSubject,
		},
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverNewPolicy,
		MaxDeliver:    5,
		AckWait:       30 * time.Second,
	}

	return c.ConsumeWithJetStream(ctx, constants.StreamNameInviteRequests, cfg,
		func(ctx context.Context, subject string, data []byte) error {
			var req model.SendInviteRequest
			if err := json.Unmarshal(data, &req); err != nil {
				slog.ErrorContext(ctx, "failed to unmarshal send_invite payload",
					"subject", subject,
					"error", err,
				)
				return nil
			}
			return handler(ctx, &req)
		},
	)
}
