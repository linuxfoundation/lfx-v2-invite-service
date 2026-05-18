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

// SendInviteHandler is the function signature for handling a decoded send-invite request.
type SendInviteHandler func(ctx context.Context, req *model.SendInviteRequest) error

// durableConsumerConfig returns a standard JetStream consumer config for the given
// consumer name and filter subjects. All consumers in this service share the same
// ACK policy, delivery policy, retry limit, and ACK wait.
func durableConsumerConfig(name string, filterSubjects []string) jetstream.ConsumerConfig {
	return jetstream.ConsumerConfig{
		Name:           name,
		Durable:        name,
		FilterSubjects: filterSubjects,
		AckPolicy:      jetstream.AckExplicitPolicy,
		DeliverPolicy:  jetstream.DeliverNewPolicy,
		MaxDeliver:     5,
		AckWait:        30 * time.Second,
	}
}

// StartSendInviteConsumer binds a durable JetStream consumer on the invite-requests
// stream and delivers decoded send-invite requests to handler.
// Returns a stop function the caller must invoke on shutdown.
func (c *Client) StartSendInviteConsumer(
	ctx context.Context,
	handler SendInviteHandler,
) (func(), error) {
	cfg := durableConsumerConfig(
		constants.ConsumerNameInviteRequestsHandler,
		[]string{constants.SendInviteSubject},
	)

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
