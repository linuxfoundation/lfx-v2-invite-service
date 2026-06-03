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
	acceptanceQueueGroup = "invite-service-acceptance"
	getInviteQueueGroup  = "invite-service-get-invite"
	getByEmailQueueGroup = "invite-service-get-by-email"

	// msgHandlerTimeout caps the total wall time for one send_invite message, covering
	// invite link generation and the email service request/reply round-trip.
	msgHandlerTimeout = 30 * time.Second
	// kvHandlerTimeout caps the total wall time for KV read/write handlers.
	kvHandlerTimeout = 10 * time.Second
)

// StartSubscriptions binds all NATS subscribers and returns their stop functions.
// If any subscription fails to start, all previously started subscriptions are
// stopped before the error is returned so the process is never left in a
// partially-initialized state with live consumers.
func StartSubscriptions(ctx context.Context) ([]func(), error) {
	stopFuncs := make([]func(), 0, 4)

	// stopAll is called on any startup error to unsubscribe consumers already registered.
	stopAll := func() {
		for _, stop := range stopFuncs {
			stop()
		}
	}

	// --- send_invite: request/reply from resource services ---
	stopSend, err := NATSClient.QueueSubscribe(api.SendInviteSubject, sendInviteQueueGroup, func(msg *nats.Msg) {
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
				"resource_uid", req.ResolvedResourceUID(),
				"error", handlerErr,
			)
			resp.Error = sendInviteErrorCode(handlerErr)
		} else {
			resp.InviteData = &api.InviteData{
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
		logArgs := []any{"resource_uid", req.ResolvedResourceUID(), "error", resp.Error}
		if resp.InviteData != nil {
			logArgs = append(logArgs, "invite_uid", resp.UID, "expires_at", resp.ExpiresAt)
		}
		slog.InfoContext(msgCtx, "send_invite reply sent", logArgs...)
	})
	if err != nil {
		return nil, fmt.Errorf("start subscription %q: %w", "send-invite", err)
	}
	stopFuncs = append(stopFuncs, stopSend)
	slog.InfoContext(ctx, "subscription started", "name", "send-invite")

	// --- invite.accepted: fire-and-forget event from self-serve web app ---
	stopAccepted, err := NATSClient.QueueSubscribe(api.InviteAcceptedSubject, acceptanceQueueGroup, func(msg *nats.Msg) {
		msgCtx, cancel := context.WithTimeout(context.Background(), kvHandlerTimeout)
		defer cancel()

		var evt api.InviteAcceptedEvent
		if err := json.Unmarshal(msg.Data, &evt); err != nil {
			slog.WarnContext(msgCtx, "invite_accepted: failed to unmarshal payload",
				"subject", msg.Subject,
				"error", err,
			)
			// No reply expected — fire-and-forget. Discard malformed messages.
			return
		}

		AcceptanceSvc.HandleInviteAccepted(msgCtx, evt)
	})
	if err != nil {
		stopAll()
		return nil, fmt.Errorf("start subscription %q: %w", "invite-accepted", err)
	}
	stopFuncs = append(stopFuncs, stopAccepted)
	slog.InfoContext(ctx, "subscription started", "name", "invite-accepted")

	// --- get_invite: request/reply — fetch invite record by UID ---
	stopGetInvite, err := NATSClient.QueueSubscribe(api.GetInviteSubject, getInviteQueueGroup, func(msg *nats.Msg) {
		msgCtx, cancel := context.WithTimeout(context.Background(), kvHandlerTimeout)
		defer cancel()

		var req api.GetInviteRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			slog.ErrorContext(msgCtx, "get_invite: failed to unmarshal payload", "error", err)
			replyGetInviteError(msgCtx, msg, "malformed_request")
			return
		}

		if req.UID == "" {
			replyGetInviteError(msgCtx, msg, "invalid_request")
			return
		}

		invite, err := InviteReadSvc.GetInvite(msgCtx, req.UID)
		var resp api.GetInviteResponse
		if err != nil {
			if errors.Is(err, intsvc.ErrInviteNotFound) {
				resp.Error = "not_found"
			} else {
				slog.ErrorContext(msgCtx, "get_invite: store error", "invite_uid", req.UID, "error", err)
				resp.Error = "internal_error"
			}
		} else {
			resp.Invite = invite
		}

		data, marshalErr := json.Marshal(resp)
		if marshalErr != nil {
			slog.ErrorContext(msgCtx, "get_invite: failed to marshal response", "error", marshalErr)
			replyGetInviteError(msgCtx, msg, "internal_error")
			return
		}
		if replyErr := msg.Respond(data); replyErr != nil {
			slog.ErrorContext(msgCtx, "get_invite: failed to send reply", "error", replyErr)
		}
	})
	if err != nil {
		stopAll()
		return nil, fmt.Errorf("start subscription %q: %w", "get-invite", err)
	}
	stopFuncs = append(stopFuncs, stopGetInvite)
	slog.InfoContext(ctx, "subscription started", "name", "get-invite")

	// --- get_invites_by_email: request/reply — fetch invite records by email ---
	stopGetByEmail, err := NATSClient.QueueSubscribe(api.GetInvitesByEmailSubject, getByEmailQueueGroup, func(msg *nats.Msg) {
		msgCtx, cancel := context.WithTimeout(context.Background(), kvHandlerTimeout)
		defer cancel()

		var req api.GetInvitesByEmailRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			slog.ErrorContext(msgCtx, "get_invites_by_email: failed to unmarshal payload", "error", err)
			replyGetByEmailError(msgCtx, msg, "malformed_request")
			return
		}

		if req.Email == "" {
			replyGetByEmailError(msgCtx, msg, "invalid_request")
			return
		}

		invites, err := InviteReadSvc.GetInvitesByEmail(msgCtx, req.Email)
		if err != nil {
			slog.ErrorContext(msgCtx, "get_invites_by_email: store error", "error", err)
			replyGetByEmailError(msgCtx, msg, "internal_error")
			return
		}

		data, marshalErr := json.Marshal(api.GetInvitesByEmailResponse{Invites: invites})
		if marshalErr != nil {
			slog.ErrorContext(msgCtx, "get_invites_by_email: failed to marshal response", "error", marshalErr)
			replyGetByEmailError(msgCtx, msg, "internal_error")
			return
		}
		if replyErr := msg.Respond(data); replyErr != nil {
			slog.ErrorContext(msgCtx, "get_invites_by_email: failed to send reply", "error", replyErr)
		}
	})
	if err != nil {
		stopAll()
		return nil, fmt.Errorf("start subscription %q: %w", "get-invites-by-email", err)
	}
	stopFuncs = append(stopFuncs, stopGetByEmail)
	slog.InfoContext(ctx, "subscription started", "name", "get-invites-by-email")

	return stopFuncs, nil
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

// replyGetInviteError sends a GetInviteResponse with only the Error field set.
func replyGetInviteError(ctx context.Context, msg *nats.Msg, errCode string) {
	if msg.Reply == "" {
		return
	}
	data, _ := json.Marshal(api.GetInviteResponse{Error: errCode})
	if err := msg.Respond(data); err != nil {
		slog.ErrorContext(ctx, "get_invite: failed to send error reply", "error", err)
	}
}

// replyGetByEmailError sends a GetInvitesByEmailResponse with an empty Invites slice and the Error field set.
func replyGetByEmailError(ctx context.Context, msg *nats.Msg, errCode string) {
	if msg.Reply == "" {
		return
	}
	data, _ := json.Marshal(api.GetInvitesByEmailResponse{Invites: []api.Invite{}, Error: errCode})
	if err := msg.Respond(data); err != nil {
		slog.ErrorContext(ctx, "get_invites_by_email: failed to send error reply", "error", err)
	}
}
