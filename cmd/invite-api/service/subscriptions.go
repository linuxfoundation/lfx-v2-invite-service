// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

const sendInviteQueueGroup = "invite-service-workers"

// StartSubscriptions binds all NATS subscribers and returns their stop functions.
func StartSubscriptions(ctx context.Context) ([]func(), error) {
	stop, err := NATSClient.QueueSubscribe(api.SendInviteSubject, sendInviteQueueGroup, func(msg *nats.Msg) {
		var req model.SendInviteRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			slog.ErrorContext(ctx, "send_invite: failed to unmarshal payload",
				"subject", msg.Subject,
				"error", err,
			)
			replyError(ctx, msg, "malformed request payload")
			return
		}

		inviteUID, handlerErr := NotificationSvc.HandleSendInvite(ctx, &req)

		var resp api.SendInviteResponse
		if handlerErr != nil {
			slog.ErrorContext(ctx, "send_invite: handler error",
				"resource_uid", req.ResourceUID,
				"error", handlerErr,
			)
			resp.Error = handlerErr.Error()
		} else {
			resp.InviteUID = inviteUID
		}

		data, marshalErr := json.Marshal(resp)
		if marshalErr != nil {
			slog.ErrorContext(ctx, "send_invite: failed to marshal response", "error", marshalErr)
			replyError(ctx, msg, "internal error marshalling response")
			return
		}
		if replyErr := msg.Respond(data); replyErr != nil {
			slog.ErrorContext(ctx, "send_invite: failed to send reply", "error", replyErr)
			return
		}
		slog.InfoContext(ctx, "send_invite reply sent",
			"resource_uid", req.ResourceUID,
			"invite_uid", resp.InviteUID,
			"error", resp.Error,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("start subscription %q: %w", "send-invite", err)
	}
	slog.InfoContext(ctx, "subscription started", "name", "send-invite")
	return []func(){stop}, nil
}

// replyError sends a SendInviteResponse with only the Error field set.
func replyError(ctx context.Context, msg *nats.Msg, errMsg string) {
	if msg.Reply == "" {
		return
	}
	data, _ := json.Marshal(api.SendInviteResponse{Error: errMsg})
	if err := msg.Respond(data); err != nil {
		slog.ErrorContext(ctx, "send_invite: failed to send error reply", "error", err)
	}
}
