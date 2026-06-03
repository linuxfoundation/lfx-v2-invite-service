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
	// to at, and sets AcceptedBy to username. If the record does not exist (i.e. the
	// invite belongs to another flow), the call is silently ignored — no error returned.
	MarkAccepted(ctx context.Context, uid, username string, at time.Time) error
}
