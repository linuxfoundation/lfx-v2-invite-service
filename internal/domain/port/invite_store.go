// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"
	"errors"
	"time"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
)

// ErrInviteNotFound is returned when no invite record matches the requested key.
var ErrInviteNotFound = errors.New("invite_not_found")

// ErrAlreadyAccepted is returned by MarkAccepted when the invite record is already
// in the accepted state. Callers can use this to distinguish a no-op idempotent call
// from a real state transition and avoid triggering duplicate side-effects.
var ErrAlreadyAccepted = errors.New("invite_already_accepted")

// InviteStore is the storage interface for invite records.
// Implementations are expected to be backed by NATS JetStream KeyValue.
type InviteStore interface {
	// Create persists a new invite record keyed by record.UID and writes the
	// email secondary-index entry. Returns an error if the write fails.
	Create(ctx context.Context, record *model.InviteRecord) error

	// GetByUID retrieves the invite record for the given invite UID.
	// Returns ErrInviteNotFound when no record exists for that UID.
	GetByUID(ctx context.Context, uid string) (*model.InviteRecord, error)

	// GetByEmail retrieves all invite records for the given email address,
	// across all resources and statuses. Returns an empty slice (not an error)
	// when there are no matching records.
	GetByEmail(ctx context.Context, email string) ([]*model.InviteRecord, error)

	// MarkAccepted updates the invite record to status=accepted, sets AcceptedAt
	// to at, and sets AcceptedBy to username. Returns ErrInviteNotFound when no
	// record exists for the given uid — callers can use this to distinguish "invite
	// belongs to another service's flow" from a transient storage error. Returns
	// ErrAlreadyAccepted when the record was already in the accepted state, so callers
	// can gate downstream side-effects (e.g. event publishing) on a real transition.
	MarkAccepted(ctx context.Context, uid, username string, at time.Time) error

	// Delete removes the invite record and its email secondary-index entry for the
	// given UID. Used to roll back a persisted record when the subsequent email
	// dispatch fails. Returns ErrInviteNotFound when no record exists.
	Delete(ctx context.Context, uid string) error
}
