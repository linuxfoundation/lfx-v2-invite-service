// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
)

const (
	// emailIndexPrefix is the KV key prefix for email→inviteUID secondary index entries.
	// Full key: index/email/<normalizedEmail>/<inviteUID>
	emailIndexPrefix = "index/email/"

	// maxMarkAcceptedRetries is the maximum number of optimistic-concurrency retries
	// when updating an invite to accepted status.
	maxMarkAcceptedRetries = 3
)

// NATSInviteRepository implements port.InviteStore using a NATS JetStream KeyValue bucket.
// Primary records are keyed by invite UID; a secondary index under "index/email/<email>/<uid>"
// maps each email to its invite UIDs. Email lookups scan all bucket keys client-side
// (O(all-keys)) — this is acceptable for current scale but should be revisited if the
// bucket grows large.
type NATSInviteRepository struct {
	kv jetstream.KeyValue
}

// NewNATSInviteRepository creates a repository backed by the given KeyValue bucket.
func NewNATSInviteRepository(kv jetstream.KeyValue) *NATSInviteRepository {
	return &NATSInviteRepository{kv: kv}
}

// Create persists a new InviteRecord (primary key = UID) and writes the email
// secondary-index entry. If either write fails, the error is returned and any
// partially written state is left as-is (the index entry is idempotent on retry).
func (r *NATSInviteRepository) Create(ctx context.Context, record *model.InviteRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return newUnexpected("invite_repository: marshal invite record", err)
	}

	if _, err := r.kv.Put(ctx, record.UID, data); err != nil {
		return newServiceUnavailable("invite_repository: put invite record", err)
	}

	// Write email secondary index: key = "index/email/<email>/<uid>", value = uid.
	// This is a separate key so concurrent creates for different invites to the same
	// email don't conflict (no read-modify-write needed).
	indexKey := emailIndexKey(record.Recipient.Email, record.UID)
	if _, err := r.kv.Put(ctx, indexKey, []byte(record.UID)); err != nil {
		slog.ErrorContext(ctx, "invite_repository: failed to write email index — record stored but index may be inconsistent",
			"invite_uid", record.UID,
			"error", err,
		)
		return newServiceUnavailable("invite_repository: put email index", err)
	}

	return nil
}

// GetByUID retrieves the invite record for the given UID.
// Returns port.ErrInviteNotFound when no record exists.
func (r *NATSInviteRepository) GetByUID(ctx context.Context, uid string) (*model.InviteRecord, error) {
	entry, err := r.kv.Get(ctx, uid)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, port.ErrInviteNotFound
		}
		return nil, newServiceUnavailable("invite_repository: get invite by uid", err)
	}

	var record model.InviteRecord
	if err := json.Unmarshal(entry.Value(), &record); err != nil {
		return nil, newUnexpected("invite_repository: unmarshal invite record", err)
	}
	return &record, nil
}

// GetByEmail retrieves all invite records for the given email address (case-insensitive).
// Returns an empty slice when no matching records exist.
func (r *NATSInviteRepository) GetByEmail(ctx context.Context, email string) ([]*model.InviteRecord, error) {
	normalized := encodeEmailForKey(email)
	prefix := emailIndexPrefix + normalized + "/"

	keys, err := r.kv.ListKeys(ctx)
	if err != nil {
		return nil, newServiceUnavailable("invite_repository: list keys for email lookup", err)
	}
	defer func() { _ = keys.Stop() }()

	var inviteUIDs []string
	for key := range keys.Keys() {
		if strings.HasPrefix(key, prefix) {
			// The UID is the last path segment.
			inviteUIDs = append(inviteUIDs, strings.TrimPrefix(key, prefix))
		}
	}

	if len(inviteUIDs) == 0 {
		return []*model.InviteRecord{}, nil
	}

	records := make([]*model.InviteRecord, 0, len(inviteUIDs))
	for _, uid := range inviteUIDs {
		record, err := r.GetByUID(ctx, uid)
		if err != nil {
			if errors.Is(err, port.ErrInviteNotFound) {
				// Index entry without a corresponding record — stale; skip.
				slog.WarnContext(ctx, "invite_repository: stale email index entry — no record for uid",
					"uid", uid, "email", normalized)
				continue
			}
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

// MarkAccepted updates the invite record to status=accepted. It uses optimistic
// concurrency (read-with-revision + conditional update) with up to maxMarkAcceptedRetries
// retries on revision mismatch. If the record does not exist, port.ErrInviteNotFound is
// returned so the caller can distinguish "invite not tracked here" from a transient error.
func (r *NATSInviteRepository) MarkAccepted(ctx context.Context, uid, username string, at time.Time) error {
	for attempt := range maxMarkAcceptedRetries {
		entry, err := r.kv.Get(ctx, uid)
		if err != nil {
			if errors.Is(err, jetstream.ErrKeyNotFound) {
				// Not tracked by this service — return ErrInviteNotFound so the service
				// layer can distinguish "not mine" from a transient KV error.
				return port.ErrInviteNotFound
			}
			return newServiceUnavailable("invite_repository: get invite for mark_accepted", err)
		}

		var record model.InviteRecord
		if err := json.Unmarshal(entry.Value(), &record); err != nil {
			return newUnexpected("invite_repository: unmarshal invite for mark_accepted", err)
		}

		// Idempotent: already accepted.
		if record.Status == model.InviteStatusAccepted {
			return nil
		}

		record.Status = model.InviteStatusAccepted
		record.AcceptedAt = &at
		record.AcceptedBy = username

		data, err := json.Marshal(record)
		if err != nil {
			return newUnexpected("invite_repository: marshal invite for mark_accepted", err)
		}

		_, updateErr := r.kv.Update(ctx, uid, data, entry.Revision())
		if updateErr == nil {
			return nil
		}

		// Revision mismatch means a concurrent update; retry.
		if isRevisionMismatch(updateErr) && attempt < maxMarkAcceptedRetries-1 {
			slog.DebugContext(ctx, "invite_repository: revision mismatch on mark_accepted — retrying",
				"attempt", attempt+1, "invite_uid", uid)
			continue
		}

		return newServiceUnavailable(fmt.Sprintf("invite_repository: update invite for mark_accepted (attempt %d)", attempt+1), updateErr)
	}
	return nil
}

// Delete removes the primary invite record and its email secondary-index entry.
// Used to roll back a stored record when the subsequent email dispatch fails.
// Returns port.ErrInviteNotFound when no primary record exists for the given UID.
func (r *NATSInviteRepository) Delete(ctx context.Context, uid string) error {
	// Read the record first so we can derive the email index key.
	entry, err := r.kv.Get(ctx, uid)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return port.ErrInviteNotFound
		}
		return newServiceUnavailable("invite_repository: get invite for delete", err)
	}

	var record model.InviteRecord
	if err := json.Unmarshal(entry.Value(), &record); err != nil {
		return newUnexpected("invite_repository: unmarshal invite for delete", err)
	}

	// Delete the primary record.
	if err := r.kv.Delete(ctx, uid); err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
		return newServiceUnavailable("invite_repository: delete invite record", err)
	}

	// Delete the email index entry. A missing index entry is not an error — it
	// may never have been written if Create failed partway through.
	indexKey := emailIndexKey(record.Recipient.Email, uid)
	if err := r.kv.Delete(ctx, indexKey); err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
		slog.ErrorContext(ctx, "invite_repository: failed to delete email index entry — primary record already removed",
			"invite_uid", uid,
			"error", err,
		)
	}

	return nil
}

// emailIndexKey returns the secondary-index KV key for an email+uid pair.
func emailIndexKey(email, uid string) string {
	return emailIndexPrefix + encodeEmailForKey(email) + "/" + uid
}

// normalizeEmail lowercases and trims an email address for consistent key lookups.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// encodeEmailForKey base64url-encodes the normalized email so it is safe to use
// as a NATS KV key segment. NATS keys only allow [A-Za-z0-9-_=./]; raw email
// addresses contain characters like '@' and '+' that are not permitted.
// RawURLEncoding produces only [A-Za-z0-9-_] (no padding '='), which is a strict
// subset of the allowed set. Both write (emailIndexKey) and read (GetByEmail)
// encode with the same function so prefix scans remain correct.
func encodeEmailForKey(email string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(normalizeEmail(email)))
}

// isRevisionMismatch returns true when err indicates a NATS KV optimistic-concurrency failure.
// NATS returns a "wrong last sequence" error (JS API error code 10071) when the revision
// provided to Update does not match the current revision of the entry. We detect this
// via the error message, mirroring the pattern used in lfx-v2-project-service.
func isRevisionMismatch(err error) bool {
	if err == nil {
		return false
	}
	// Check via the JetStreamError interface so we can inspect the error code directly.
	var jsErr jetstream.JetStreamError
	if errors.As(err, &jsErr) {
		if apiErr := jsErr.APIError(); apiErr != nil && apiErr.ErrorCode == jetstream.JSErrCodeStreamWrongLastSequence {
			return true
		}
	}
	// Belt-and-suspenders: check the error string for cases where the error is wrapped
	// in a way that doesn't surface as a JetStreamError.
	return strings.Contains(err.Error(), "wrong last sequence")
}
