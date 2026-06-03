// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
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
}

// NewAcceptanceService creates an AcceptanceService backed by the given store.
func NewAcceptanceService(store port.InviteStore) *AcceptanceService {
	return &AcceptanceService{inviteStore: store}
}

// HandleInviteAccepted processes an api.InviteAcceptedEvent, updating the invite
// record to status=accepted. If the event is malformed or the invite is not tracked
// by this service, the call is silently ignored (no error returned) to avoid
// poisoning the queue and blocking delivery to other subscribers (e.g. project-service).
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
}
