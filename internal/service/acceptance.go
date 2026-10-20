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

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

// errAcceptanceRejected marks an acceptance event whose evidence does not match
// the stored invite record.
var errAcceptanceRejected = errors.New("acceptance_rejected")

// AcceptanceService processes invite-acceptance events published by the LFX
// self-serve web app and updates the invite record in the KV store.
type AcceptanceService struct {
	inviteStore   port.InviteStore
	publisher     port.EventPublisher
	tokenVerifier port.InviteTokenVerifier
}

// NewAcceptanceService creates an AcceptanceService backed by the given store and
// publisher. verifier checks the signed invite token carried on each acceptance event.
func NewAcceptanceService(store port.InviteStore, publisher port.EventPublisher, verifier port.InviteTokenVerifier) *AcceptanceService {
	return &AcceptanceService{inviteStore: store, publisher: publisher, tokenVerifier: verifier}
}

// HandleInviteAccepted processes an api.InviteAcceptedEvent, updating the invite
// record to status=accepted. If the event is malformed or the invite is not tracked
// by this service, the call is silently ignored (no error returned) to avoid
// poisoning the queue and blocking delivery to other subscribers (e.g. project-service).
//
// The event is only honoured when it carries the signed invite token this service
// issued for the invite: the token must verify, its jti must equal the invite UID,
// and its email must equal the record's recipient email. The invite must not be
// expired, and when the record names a recipient username the event's username
// must match it. Rejected events cause no KV write and no publish.
//
// On success it publishes an InviteServiceAcceptedEvent on InviteServiceAcceptedSubject
// with the full invite record as enriched context for downstream subscribers.
func (s *AcceptanceService) HandleInviteAccepted(ctx context.Context, evt api.InviteAcceptedEvent) {
	if evt.InviteUID == "" || evt.Username == "" {
		slog.WarnContext(ctx, "acceptance: invite_accepted event missing invite_uid or username — discarding",
			"invite_uid", evt.InviteUID,
			"username", evt.Username,
		)
		return
	}

	record, err := s.inviteStore.GetByUID(ctx, evt.InviteUID)
	if err != nil {
		if errors.Is(err, port.ErrInviteNotFound) {
			// Not tracked by this service — this invite belongs to another flow.
			slog.DebugContext(ctx, "acceptance: invite not tracked by invite-service — ignoring",
				"invite_uid", evt.InviteUID)
			return
		}
		slog.WarnContext(ctx, "acceptance: failed to load invite record for verification",
			"invite_uid", evt.InviteUID,
			"error", err,
		)
		return
	}
	if record.Status == model.InviteStatusAccepted {
		slog.DebugContext(ctx, "acceptance: invite already accepted — skipping duplicate publish",
			"invite_uid", evt.InviteUID)
		return
	}

	now := time.Now()
	if err := s.verifyAcceptance(ctx, evt, record, now); err != nil {
		slog.WarnContext(ctx, "acceptance: invite_accepted event rejected — discarding",
			"invite_uid", evt.InviteUID,
			"username", evt.Username,
			"error", err,
		)
		return
	}

	if err := s.inviteStore.MarkAccepted(ctx, evt.InviteUID, evt.Username, now); err != nil {
		if errors.Is(err, port.ErrInviteNotFound) {
			// Not tracked by this service — this invite belongs to another flow.
			// Silently ignore so as not to interfere with other subscribers.
			slog.DebugContext(ctx, "acceptance: invite not tracked by invite-service — ignoring",
				"invite_uid", evt.InviteUID)
			return
		}
		if errors.Is(err, port.ErrAlreadyAccepted) {
			// Redelivered or duplicate event — the record is already in the accepted state.
			// Skip the enriched publish to avoid emitting duplicate InviteServiceAcceptedEvents.
			slog.DebugContext(ctx, "acceptance: invite already accepted — skipping duplicate publish",
				"invite_uid", evt.InviteUID)
			return
		}
		if errors.Is(err, port.ErrInviteExpired) {
			slog.WarnContext(ctx, "acceptance: invite expired — discarding",
				"invite_uid", evt.InviteUID)
			return
		}
		// Transient KV error: log and drop. The event is fire-and-forget so there
		// is no retry mechanism; the record will remain in pending state.
		slog.WarnContext(ctx, "acceptance: failed to mark invite as accepted",
			"invite_uid", evt.InviteUID,
			"username", evt.Username,
			"error", err,
		)
		return
	}

	slog.InfoContext(ctx, "acceptance: invite accepted — record updated",
		"invite_uid", evt.InviteUID,
		"username", evt.Username,
	)

	s.publishAccepted(ctx, evt.InviteUID)
}

// verifyAcceptance checks that evt is backed by evidence this service can verify
// and that it is bound to record. It returns nil only when a token is present, the
// invite is unexpired, the token verifies and matches the record's UID and
// recipient email, and (when the record names one) the recipient username matches
// the event's username.
func (s *AcceptanceService) verifyAcceptance(ctx context.Context, evt api.InviteAcceptedEvent, record *model.InviteRecord, now time.Time) error {
	if evt.Token == "" {
		return fmt.Errorf("%w: event carries no invite token", errAcceptanceRejected)
	}
	// A zero ExpiresAt fails closed.
	if !now.Before(record.ExpiresAt) {
		return port.ErrInviteExpired
	}

	claims, err := s.tokenVerifier.Verify(ctx, evt.Token)
	if err != nil {
		return err
	}
	if claims.InviteUID != record.UID {
		return fmt.Errorf("%w: token invite_uid does not match invite record", errAcceptanceRejected)
	}
	// Both values are the same canonical address from mail.ParseAddress (see
	// HandleSendInvite), so they are compared exactly.
	if claims.Email != record.Recipient.Email {
		return fmt.Errorf("%w: token email does not match invite recipient", errAcceptanceRejected)
	}
	if record.Recipient.Username != "" && evt.Username != record.Recipient.Username {
		return fmt.Errorf("%w: username does not match invite recipient username", errAcceptanceRejected)
	}
	return nil
}

// publishAccepted fetches the full invite record and publishes InviteServiceAcceptedSubject.
// Failures are best-effort: logged but never block the acceptance flow.
func (s *AcceptanceService) publishAccepted(ctx context.Context, inviteUID string) {
	record, err := s.inviteStore.GetByUID(ctx, inviteUID)
	if err != nil {
		slog.WarnContext(ctx, "acceptance: failed to fetch invite record for enriched publish — skipping",
			"invite_uid", inviteUID,
			"error", err,
		)
		return
	}

	inv := domainToAPIInvite(record)
	inv.Status = api.InviteStatusAccepted
	evt := api.InviteServiceAcceptedEvent{Invite: inv}

	data, err := json.Marshal(evt)
	if err != nil {
		slog.WarnContext(ctx, "acceptance: failed to marshal enriched event — skipping",
			"invite_uid", inviteUID,
			"error", err,
		)
		return
	}

	if err := s.publisher.Publish(ctx, api.InviteServiceAcceptedSubject, data); err != nil {
		slog.WarnContext(ctx, "acceptance: failed to publish enriched invite_accepted event — skipping",
			"invite_uid", inviteUID,
			"error", err,
		)
		return
	}

	slog.DebugContext(ctx, "acceptance: published enriched invite_accepted event",
		"invite_uid", inviteUID,
		"subject", api.InviteServiceAcceptedSubject,
	)
}
