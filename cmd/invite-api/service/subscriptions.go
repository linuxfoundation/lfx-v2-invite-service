// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	intsvc "github.com/linuxfoundation/lfx-v2-invite-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

const (
	sendInviteQueueGroup = "invite-service-workers"
	// msgHandlerTimeout caps the total wall time for one send_invite message, covering
	// invite link generation and the email service request/reply round-trip.
	msgHandlerTimeout = 30 * time.Second
)

// StartSubscriptions binds all NATS subscribers and returns their stop functions.
func StartSubscriptions(ctx context.Context) ([]func(), error) {
	stop, err := NATSClient.QueueSubscribe(api.SendInviteSubject, sendInviteQueueGroup, func(msg *nats.Msg) {
		// Derive a per-message context so tracing spans, deadlines, and cancellation
		// are scoped to this message — not shared across all messages for the process lifetime.
		msgCtx, cancel := context.WithTimeout(context.Background(), msgHandlerTimeout)
		defer cancel()

		var req model.SendInviteRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			slog.ErrorContext(msgCtx, "send_invite: failed to unmarshal payload",
				"subject", msg.Subject,
				"error", err,
			)
			replyError(msgCtx, msg, "malformed_request")
			return
		}

		result, handlerErr := NotificationSvc.HandleSendInvite(msgCtx, &req)

		var resp api.SendInviteResponse
		if handlerErr != nil {
			slog.ErrorContext(msgCtx, "send_invite: handler error",
				"resource_uid", req.ResourceUID,
				"error", handlerErr,
			)
			resp.Error = sendInviteErrorCode(handlerErr)
		} else {
			resp.Invite = &api.InviteData{
				UID:       result.InviteUID,
				Email:     result.RecipientEmail,
				ExpiresAt: result.ExpiresAt,
			}
		}

		data, marshalErr := json.Marshal(resp)
		if marshalErr != nil {
			slog.ErrorContext(msgCtx, "send_invite: failed to marshal response", "error", marshalErr)
			replyError(msgCtx, msg, "internal_error")
			return
		}
		if replyErr := msg.Respond(data); replyErr != nil {
			slog.ErrorContext(msgCtx, "send_invite: failed to send reply", "error", replyErr)
			return
		}
		logArgs := []any{"resource_uid", req.ResourceUID, "error", resp.Error}
		if resp.Invite != nil {
			logArgs = append(logArgs, "invite_uid", resp.Invite.UID, "expires_at", resp.Invite.ExpiresAt)
		}
		slog.InfoContext(msgCtx, "send_invite reply sent", logArgs...)
	})
	if err != nil {
		return nil, fmt.Errorf("start subscription %q: %w", "send-invite", err)
	}
	slog.InfoContext(ctx, "subscription started", "name", "send-invite")
	return []func(){stop}, nil
}

// sendInviteErrorCode maps a handler error to a stable, opaque error code for the
// caller. Full error details are logged server-side and never forwarded to callers.
func sendInviteErrorCode(err error) string {
	switch {
	case errors.Is(err, intsvc.ErrInvalidRequest):
		return "invalid_request"
	case errors.Is(err, intsvc.ErrEmailDispatchFailed):
		return "email_dispatch_failed"
	default:
		return "internal_error"
	}
}

// replyError sends a SendInviteResponse with only the Error field set.
func replyError(ctx context.Context, msg *nats.Msg, errCode string) {
	if msg.Reply == "" {
		return
	}
	data, _ := json.Marshal(api.SendInviteResponse{Error: errCode})
	if err := msg.Respond(data); err != nil {
		slog.ErrorContext(ctx, "send_invite: failed to send error reply", "error", err)
	}
}
