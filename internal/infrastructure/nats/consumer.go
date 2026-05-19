// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

const sendInviteQueueGroup = "invite-service-workers"

// SendInviteHandler is the function signature for handling a decoded send-invite request.
// Returns the invite UUID on success, or an empty string and an error on failure.
type SendInviteHandler func(ctx context.Context, req *model.SendInviteRequest) (inviteUID string, err error)

// StartSendInviteConsumer binds a queue-group subscriber on SendInviteSubject and
// replies to each request with a SendInviteResponse containing the invite UUID or
// an error description. The queue group distributes load across replicas.
// Returns a stop function the caller must invoke on shutdown.
func (c *Client) StartSendInviteConsumer(
	ctx context.Context,
	handler SendInviteHandler,
) (func(), error) {
	sub, err := c.conn.QueueSubscribe(api.SendInviteSubject, sendInviteQueueGroup, func(msg *nats.Msg) {
		var req model.SendInviteRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			slog.ErrorContext(ctx, "failed to unmarshal send_invite payload",
				"subject", msg.Subject,
				"error", err,
			)
			// ACK and discard: malformed messages will never parse successfully
			// on re-delivery, so retrying would only exhaust MaxDeliver.
			c.replyError(ctx, msg, "malformed request payload")
			return
		}

		inviteUID, handlerErr := handler(ctx, &req)

		var resp api.SendInviteResponse
		if handlerErr != nil {
			slog.ErrorContext(ctx, "send_invite handler error",
				"resource_uid", req.ResourceUID,
				"error", handlerErr,
			)
			resp.Error = handlerErr.Error()
		} else {
			resp.InviteUID = inviteUID
		}

		data, err := json.Marshal(resp)
		if err != nil {
			slog.ErrorContext(ctx, "failed to marshal send_invite response", "error", err)
			c.replyError(ctx, msg, "internal error marshalling response")
			return
		}
		if err := msg.Respond(data); err != nil {
			slog.ErrorContext(ctx, "failed to send send_invite reply", "error", err)
			return
		}
		slog.InfoContext(ctx, "send_invite reply sent",
			"resource_uid", req.ResourceUID,
			"invite_uid", resp.InviteUID,
			"error", resp.Error,
		)
	})
	if err != nil {
		return nil, newServiceUnavailable("failed to subscribe to send_invite subject", err)
	}

	slog.InfoContext(ctx, "send_invite subscriber started",
		"subject", api.SendInviteSubject,
		"queue_group", sendInviteQueueGroup,
	)

	return func() { _ = sub.Unsubscribe() }, nil
}

// replyError sends a SendInviteResponse with only the Error field set.
func (c *Client) replyError(ctx context.Context, msg *nats.Msg, errMsg string) {
	if msg.Reply == "" {
		return
	}
	data, _ := json.Marshal(api.SendInviteResponse{Error: errMsg})
	if err := msg.Respond(data); err != nil {
		slog.ErrorContext(ctx, "failed to send error reply", "error", err)
	}
}
