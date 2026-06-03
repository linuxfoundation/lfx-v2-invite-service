// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

// AcceptanceService processes invite-acceptance events published by the LFX
// self-serve web app and updates the invite record in the KV store.
type AcceptanceService struct {
	inviteStore port.InviteStore
	publisher   port.EventPublisher
}

// NewAcceptanceService creates an AcceptanceService backed by the given store and publisher.
func NewAcceptanceService(store port.InviteStore, publisher port.EventPublisher) *AcceptanceService {
	return &AcceptanceService{inviteStore: store, publisher: publisher}
}

// HandleInviteAccepted processes an api.InviteAcceptedEvent, updating the invite
// record to status=accepted. If the event is malformed or the invite is not tracked
// by this service, the call is silently ignored (no error returned) to avoid
// poisoning the queue and blocking delivery to other subscribers (e.g. project-service).
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

	if err := s.inviteStore.MarkAccepted(ctx, evt.InviteUID, evt.Username, time.Now()); err != nil {
		if errors.Is(err, port.ErrInviteNotFound) {
			// Not tracked by this service — this invite belongs to another flow.
			// Silently ignore so as not to interfere with other subscribers.
			slog.DebugContext(ctx, "acceptance: invite not tracked by invite-service — ignoring",
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

	evt := api.InviteServiceAcceptedEvent{
		Invite: api.Invite{
			UID:        record.UID,
			Status:     api.InviteStatusAccepted,
			Recipient:  api.Recipient{Name: record.Recipient.Name, Email: record.Recipient.Email, Username: record.Recipient.Username, Avatar: record.Recipient.Avatar},
			Inviter:    api.Inviter{Name: record.Inviter.Name, Username: record.Inviter.Username, Email: record.Inviter.Email, Avatar: record.Inviter.Avatar},
			Resource:   api.Resource{UID: record.Resource.UID, Name: record.Resource.Name, Type: record.Resource.Type},
			Role:       record.Role,
			OrgName:    record.OrgName,
			ReturnURL:  record.ReturnURL,
			CreatedAt:  record.CreatedAt,
			ExpiresAt:  record.ExpiresAt,
			AcceptedAt: record.AcceptedAt,
			AcceptedBy: record.AcceptedBy,
		},
	}

	data, err := json.Marshal(evt)
	if err != nil {
		slog.WarnContext(ctx, "acceptance: failed to marshal enriched event — skipping",
			"invite_uid", inviteUID,
			"error", err,
		)
		return
	}

	if err := s.publisher.Publish(api.InviteServiceAcceptedSubject, data); err != nil {
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
