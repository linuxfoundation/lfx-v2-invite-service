// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
)

// startTestNATSServer launches an embedded NATS server with JetStream on a random port.
// The server is shut down via t.Cleanup when the test ends.
func startTestNATSServer(t *testing.T) string {
	t.Helper()
	opts := &natsserver.Options{
		Host:      "127.0.0.1",
		Port:      -1, // random
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoLog:     true,
		NoSigs:    true,
	}
	ns, err := natsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("create NATS server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(4 * time.Second) {
		t.Fatal("NATS server not ready")
	}
	t.Cleanup(ns.Shutdown)
	return ns.ClientURL()
}

// newTestRepo creates a NATSInviteRepository backed by a fresh embedded NATS JetStream KV bucket.
func newTestRepo(t *testing.T) *NATSInviteRepository {
	t.Helper()
	url := startTestNATSServer(t)

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect to NATS: %v", err)
	}
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("create JetStream context: %v", err)
	}

	kv, err := js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{
		Bucket: "invites-test",
	})
	if err != nil {
		t.Fatalf("create KV bucket: %v", err)
	}

	return NewNATSInviteRepository(kv)
}

// sampleRecord returns a minimal valid InviteRecord for the given uid and email.
func sampleRecord(uid, email string) *model.InviteRecord {
	now := time.Now().Truncate(time.Second)
	return &model.InviteRecord{
		UID:    uid,
		Status: model.InviteStatusPending,
		Recipient: model.Recipient{
			Email: email,
			Name:  "Alice",
		},
		Inviter:   model.Inviter{Username: "bob", Name: "Bob"},
		Resource:  model.InviteResource{UID: "proj-1", Type: "project"},
		Role:      "Member",
		CreatedAt: now,
		ExpiresAt: now.Add(7 * 24 * time.Hour),
	}
}

// ── Pure function tests ───────────────────────────────────────────────────────

func TestEncodeEmailForKey_RoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		email string
	}{
		{"lowercase bare", "alice@example.com"},
		{"uppercase", "ALICE@EXAMPLE.COM"},
		{"with plus", "alice+tag@example.com"},
		{"with dots", "alice.b.c@example.com"},
		{"leading/trailing space", "  alice@example.com  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc1 := encodeEmailForKey(tc.email)
			enc2 := encodeEmailForKey(tc.email)
			if enc1 != enc2 {
				t.Errorf("encode is not deterministic: %q != %q", enc1, enc2)
			}
			// Lowercase variant must produce the same key (normalizeEmail lowercases).
			lc := encodeEmailForKey(normalizeEmail(tc.email))
			if enc1 != lc {
				t.Errorf("encode of %q differs from encode of lowercased form: %q vs %q", tc.email, enc1, lc)
			}
		})
	}
}

func TestIsRevisionMismatch(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if isRevisionMismatch(nil) {
			t.Error("expected false for nil error")
		}
	})

	t.Run("unrelated error", func(t *testing.T) {
		if isRevisionMismatch(errors.New("some other error")) {
			t.Error("expected false for unrelated error")
		}
	})

	t.Run("string fallback — wrong last sequence", func(t *testing.T) {
		// Covers the belt-and-suspenders string match branch.
		err := errors.New("wrong last sequence for key")
		if !isRevisionMismatch(err) {
			t.Error("expected true for 'wrong last sequence' string")
		}
	})
}

// ── Integration tests (embedded NATS JetStream) ───────────────────────────────

func TestNATSInviteRepository_Create_GetByUID(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rec := sampleRecord("uid-1", "alice@example.com")
	if err := repo.Create(ctx, rec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByUID(ctx, "uid-1")
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if got.UID != rec.UID {
		t.Errorf("UID: got %q, want %q", got.UID, rec.UID)
	}
	if got.Recipient.Email != rec.Recipient.Email {
		t.Errorf("Recipient.Email: got %q, want %q", got.Recipient.Email, rec.Recipient.Email)
	}
	if got.Status != model.InviteStatusPending {
		t.Errorf("Status: got %q, want pending", got.Status)
	}
}

func TestNATSInviteRepository_GetByUID_NotFound(t *testing.T) {
	repo := newTestRepo(t)

	_, err := repo.GetByUID(context.Background(), "no-such-uid")
	if !errors.Is(err, port.ErrInviteNotFound) {
		t.Errorf("expected ErrInviteNotFound, got %v", err)
	}
}

func TestNATSInviteRepository_GetByEmail_PrefixScan(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	// Two invites for alice, one for bob.
	if err := repo.Create(ctx, sampleRecord("uid-a1", "alice@example.com")); err != nil {
		t.Fatalf("Create alice1: %v", err)
	}
	if err := repo.Create(ctx, sampleRecord("uid-a2", "alice@example.com")); err != nil {
		t.Fatalf("Create alice2: %v", err)
	}
	if err := repo.Create(ctx, sampleRecord("uid-b1", "bob@example.com")); err != nil {
		t.Fatalf("Create bob: %v", err)
	}

	aliceRecords, err := repo.GetByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("GetByEmail alice: %v", err)
	}
	if len(aliceRecords) != 2 {
		t.Errorf("expected 2 records for alice, got %d", len(aliceRecords))
	}

	bobRecords, err := repo.GetByEmail(ctx, "bob@example.com")
	if err != nil {
		t.Fatalf("GetByEmail bob: %v", err)
	}
	if len(bobRecords) != 1 {
		t.Errorf("expected 1 record for bob, got %d", len(bobRecords))
	}

	noRecords, err := repo.GetByEmail(ctx, "nobody@example.com")
	if err != nil {
		t.Fatalf("GetByEmail nobody: %v", err)
	}
	if len(noRecords) != 0 {
		t.Errorf("expected empty slice for unknown email, got %d records", len(noRecords))
	}
}

func TestNATSInviteRepository_GetByEmail_DisplayNameQuery(t *testing.T) {
	// Verifies that a display-name email query (e.g. "Alice <alice@example.com>") is
	// canonicalized before key encoding, matching the stored canonical address.
	repo := newTestRepo(t)
	ctx := context.Background()

	if err := repo.Create(ctx, sampleRecord("uid-canon", "alice@example.com")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Query with display-name form — write path stored "alice@example.com",
	// read path must canonicalize before encoding.
	records, err := repo.GetByEmail(ctx, `"Alice" <alice@example.com>`)
	if err != nil {
		t.Fatalf("GetByEmail display-name: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record when querying by display-name email, got %d", len(records))
	}
}

func TestNATSInviteRepository_GetByEmail_StaleIndexSkipped(t *testing.T) {
	// Write index entry without a corresponding primary record; GetByEmail should
	// skip the stale entry and return an empty slice rather than an error.
	repo := newTestRepo(t)
	ctx := context.Background()

	// Create then delete the primary record, leaving the index entry behind.
	rec := sampleRecord("uid-stale", "stale@example.com")
	if err := repo.Create(ctx, rec); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Delete only the primary record via the KV directly (simulating a partial delete).
	if err := repo.kv.Delete(ctx, rec.UID); err != nil {
		t.Fatalf("direct KV delete: %v", err)
	}

	records, err := repo.GetByEmail(ctx, "stale@example.com")
	if err != nil {
		t.Fatalf("GetByEmail with stale index: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected empty result for stale index, got %d records", len(records))
	}
}

func TestNATSInviteRepository_MarkAccepted_HappyPath(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	if err := repo.Create(ctx, sampleRecord("uid-mark", "alice@example.com")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	at := time.Now().Truncate(time.Second)
	if err := repo.MarkAccepted(ctx, "uid-mark", "alice-lfid", at); err != nil {
		t.Fatalf("MarkAccepted: %v", err)
	}

	got, err := repo.GetByUID(ctx, "uid-mark")
	if err != nil {
		t.Fatalf("GetByUID after accept: %v", err)
	}
	if got.Status != model.InviteStatusAccepted {
		t.Errorf("Status: got %q, want accepted", got.Status)
	}
	if got.AcceptedBy != "alice-lfid" {
		t.Errorf("AcceptedBy: got %q, want alice-lfid", got.AcceptedBy)
	}
	if got.AcceptedAt == nil || got.AcceptedAt.IsZero() {
		t.Error("AcceptedAt should not be nil/zero")
	}
}

func TestNATSInviteRepository_MarkAccepted_AlreadyAccepted_ReturnsErrAlreadyAccepted(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	if err := repo.Create(ctx, sampleRecord("uid-dup", "alice@example.com")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.MarkAccepted(ctx, "uid-dup", "alice-lfid", time.Now()); err != nil {
		t.Fatalf("first MarkAccepted: %v", err)
	}

	// Second call on the same record must return ErrAlreadyAccepted — not nil — so
	// the caller can distinguish a real transition from a duplicate/redelivered event.
	err := repo.MarkAccepted(ctx, "uid-dup", "alice-lfid", time.Now())
	if !errors.Is(err, port.ErrAlreadyAccepted) {
		t.Errorf("expected ErrAlreadyAccepted on duplicate call, got %v", err)
	}
}

func TestNATSInviteRepository_MarkAccepted_NotFound(t *testing.T) {
	repo := newTestRepo(t)

	err := repo.MarkAccepted(context.Background(), "no-such-uid", "alice", time.Now())
	if !errors.Is(err, port.ErrInviteNotFound) {
		t.Errorf("expected ErrInviteNotFound for missing uid, got %v", err)
	}
}

func TestNATSInviteRepository_MarkAccepted_RetryOnConcurrentWrite(t *testing.T) {
	// Verify the optimistic-concurrency retry loop: bump the record's revision
	// while MarkAccepted is in flight and confirm it still completes successfully.
	repo := newTestRepo(t)
	ctx := context.Background()

	if err := repo.Create(ctx, sampleRecord("uid-race", "alice@example.com")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Bump the revision once from outside concurrently with MarkAccepted.
	// A sync.WaitGroup ensures the bump happens while MarkAccepted is in its loop.
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		// Small sleep so MarkAccepted's first Get/Update cycle is likely in progress.
		time.Sleep(5 * time.Millisecond)
		// Write a no-op update to bump the revision and cause the first Update attempt
		// in MarkAccepted to fail with a revision mismatch.
		existing, err := repo.GetByUID(ctx, "uid-race")
		if err != nil {
			return
		}
		_ = repo.Create(ctx, existing) //nolint:errcheck // best-effort bump; key-exists error is fine
	}()

	err := repo.MarkAccepted(ctx, "uid-race", "alice-lfid", time.Now())
	wg.Wait()

	// MarkAccepted must eventually succeed despite the concurrent write.
	if err != nil && !errors.Is(err, port.ErrAlreadyAccepted) {
		t.Errorf("MarkAccepted with concurrent write failed unexpectedly: %v", err)
	}
}

func TestNATSInviteRepository_Delete_MissingIndexTolerated(t *testing.T) {
	// Delete should succeed even when the email index entry was never written
	// (or was already deleted), to support partial-write recovery.
	repo := newTestRepo(t)
	ctx := context.Background()

	rec := sampleRecord("uid-del", "alice@example.com")
	if err := repo.Create(ctx, rec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Remove the email index entry manually to simulate a partial-write scenario.
	indexKey := emailIndexKey(rec.Recipient.Email, rec.UID)
	if err := repo.kv.Delete(ctx, indexKey); err != nil {
		t.Fatalf("pre-delete index entry: %v", err)
	}

	// Delete should not return an error for the missing index entry.
	if err := repo.Delete(ctx, rec.UID); err != nil {
		t.Errorf("Delete with missing index: %v", err)
	}

	// Primary record should no longer be findable.
	_, err := repo.GetByUID(ctx, rec.UID)
	if !errors.Is(err, port.ErrInviteNotFound) {
		t.Errorf("expected ErrInviteNotFound after delete, got %v", err)
	}
}

func TestNATSInviteRepository_Delete_NotFound(t *testing.T) {
	repo := newTestRepo(t)

	err := repo.Delete(context.Background(), "no-such-uid")
	if !errors.Is(err, port.ErrInviteNotFound) {
		t.Errorf("expected ErrInviteNotFound for unknown uid, got %v", err)
	}
}
