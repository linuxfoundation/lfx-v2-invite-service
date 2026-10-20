// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port/mocks"
	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

var errTransient = errors.New("transient kv error")
var errPublish = errors.New("nats publish error")

// stubTokenVerifier is a test double for port.InviteTokenVerifier. It returns
// claims for the token "valid-token" and ErrInvalidInviteToken for anything else,
// unless claims/err are overridden.
type stubTokenVerifier struct {
	claims *port.InviteTokenClaims
	err    error
	calls  []string
}

func (v *stubTokenVerifier) Verify(_ context.Context, token string) (port.InviteTokenClaims, error) {
	v.calls = append(v.calls, token)
	if v.err != nil {
		return port.InviteTokenClaims{}, v.err
	}
	if v.claims != nil {
		return *v.claims, nil
	}
	if token != "valid-token" {
		return port.InviteTokenClaims{}, port.ErrInvalidInviteToken
	}
	return port.InviteTokenClaims{InviteUID: "uid-123", Email: "alice@example.com"}, nil
}

func pendingRecord(mutate ...func(r *model.InviteRecord)) *model.InviteRecord {
	r := &model.InviteRecord{
		UID:       "uid-123",
		Status:    model.InviteStatusPending,
		Recipient: model.Recipient{Email: "alice@example.com"},
		Inviter:   model.Inviter{Username: "bob"},
		Resource:  model.InviteResource{UID: "proj-1", Type: "project"},
		Role:      "Member",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	for _, m := range mutate {
		m(r)
	}
	return r
}

func TestAcceptanceService_HandleInviteAccepted(t *testing.T) {
	validEvent := api.InviteAcceptedEvent{InviteUID: "uid-123", Username: "alice", Token: "valid-token"}

	tests := []struct {
		name            string
		event           api.InviteAcceptedEvent
		record          *model.InviteRecord // returned by every GetByUID call
		getErr          error               // returned by the first (verification) GetByUID call
		publishGetErr   error               // returned by the second (enrichment) GetByUID call
		verifierClaims  *port.InviteTokenClaims
		verifierErr     error
		markAcceptedErr error
		publishErr      error

		wantGetByUID       bool
		wantVerifierCalled bool
		wantMarkCalled     bool
		wantUsername       string
		wantPublishCalled  bool
	}{
		{
			name:               "marks invite accepted and publishes enriched event",
			event:              validEvent,
			record:             pendingRecord(),
			wantGetByUID:       true,
			wantVerifierCalled: true,
			wantMarkCalled:     true,
			wantUsername:       "alice",
			wantPublishCalled:  true,
		},
		{
			name:  "accepts when username matches the recipient username",
			event: validEvent,
			record: pendingRecord(func(r *model.InviteRecord) {
				r.Recipient.Username = "alice"
			}),
			wantGetByUID:       true,
			wantVerifierCalled: true,
			wantMarkCalled:     true,
			wantUsername:       "alice",
			wantPublishCalled:  true,
		},
		{
			name:  "rejects username that differs from the recipient username only by case",
			event: api.InviteAcceptedEvent{InviteUID: "uid-123", Username: "Alice", Token: "valid-token"},
			record: pendingRecord(func(r *model.InviteRecord) {
				r.Recipient.Username = "alice"
			}),
			wantGetByUID:       true,
			wantVerifierCalled: true,
		},
		{
			name:  "discards event with missing invite_uid",
			event: api.InviteAcceptedEvent{InviteUID: "", Username: "alice", Token: "valid-token"},
		},
		{
			name:  "discards event with missing username",
			event: api.InviteAcceptedEvent{InviteUID: "uid-123", Username: "", Token: "valid-token"},
		},
		{
			name:         "discards forged event carrying only invite_uid and username",
			event:        api.InviteAcceptedEvent{InviteUID: "uid-123", Username: "mallory"},
			record:       pendingRecord(),
			wantGetByUID: true,
		},
		{
			name:               "rejects event whose token fails verification",
			event:              api.InviteAcceptedEvent{InviteUID: "uid-123", Username: "mallory", Token: "forged-token"},
			record:             pendingRecord(),
			wantGetByUID:       true,
			wantVerifierCalled: true,
		},
		{
			name:               "discards event when verifier returns an unexpected error",
			event:              validEvent,
			record:             pendingRecord(),
			verifierErr:        errTransient,
			wantGetByUID:       true,
			wantVerifierCalled: true,
		},
		{
			name:               "rejects token issued for a different invite",
			event:              validEvent,
			record:             pendingRecord(),
			verifierClaims:     &port.InviteTokenClaims{InviteUID: "uid-other", Email: "alice@example.com"},
			wantGetByUID:       true,
			wantVerifierCalled: true,
		},
		{
			name:               "rejects token whose email does not match the recipient",
			event:              validEvent,
			record:             pendingRecord(),
			verifierClaims:     &port.InviteTokenClaims{InviteUID: "uid-123", Email: "mallory@example.com"},
			wantGetByUID:       true,
			wantVerifierCalled: true,
		},
		{
			name:  "rejects username that does not match the recipient username",
			event: api.InviteAcceptedEvent{InviteUID: "uid-123", Username: "mallory", Token: "valid-token"},
			record: pendingRecord(func(r *model.InviteRecord) {
				r.Recipient.Username = "alice"
			}),
			wantGetByUID:       true,
			wantVerifierCalled: true,
		},
		{
			name:  "rejects expired invite before any write or publish",
			event: validEvent,
			record: pendingRecord(func(r *model.InviteRecord) {
				r.ExpiresAt = time.Now().Add(-time.Hour)
			}),
			wantGetByUID: true,
		},
		{
			name:  "rejects invite with zero ExpiresAt (fails closed)",
			event: validEvent,
			record: pendingRecord(func(r *model.InviteRecord) {
				r.ExpiresAt = time.Time{}
			}),
			wantGetByUID: true,
		},
		{
			name:         "silently ignores not-found invite (belongs to another service)",
			event:        api.InviteAcceptedEvent{InviteUID: "foreign-uid", Username: "bob", Token: "some-token"},
			getErr:       port.ErrInviteNotFound,
			wantGetByUID: true,
		},
		{
			name:         "logs but does not panic on transient error loading the record",
			event:        validEvent,
			getErr:       errTransient,
			wantGetByUID: true,
		},
		{
			name:  "skips already-accepted record without verifying or publishing",
			event: validEvent,
			record: pendingRecord(func(r *model.InviteRecord) {
				r.Status = model.InviteStatusAccepted
			}),
			wantGetByUID: true,
		},
		{
			name:               "skips publish when MarkAccepted reports ErrAlreadyAccepted (race)",
			event:              validEvent,
			record:             pendingRecord(),
			markAcceptedErr:    port.ErrAlreadyAccepted,
			wantGetByUID:       true,
			wantVerifierCalled: true,
			wantMarkCalled:     true,
		},
		{
			name:               "skips publish when MarkAccepted reports ErrInviteExpired",
			event:              validEvent,
			record:             pendingRecord(),
			markAcceptedErr:    port.ErrInviteExpired,
			wantGetByUID:       true,
			wantVerifierCalled: true,
			wantMarkCalled:     true,
		},
		{
			name:               "skips publish when MarkAccepted reports not found (race with delete)",
			event:              validEvent,
			record:             pendingRecord(),
			markAcceptedErr:    port.ErrInviteNotFound,
			wantGetByUID:       true,
			wantVerifierCalled: true,
			wantMarkCalled:     true,
		},
		{
			name:               "logs but does not panic on transient MarkAccepted error",
			event:              validEvent,
			record:             pendingRecord(),
			markAcceptedErr:    errTransient,
			wantGetByUID:       true,
			wantVerifierCalled: true,
			wantMarkCalled:     true,
		},
		{
			name:               "skips publish but does not fail when enrichment GetByUID errors",
			event:              validEvent,
			record:             pendingRecord(),
			publishGetErr:      errTransient,
			wantGetByUID:       true,
			wantVerifierCalled: true,
			wantMarkCalled:     true,
		},
		{
			name:               "does not fail when publish errors",
			event:              validEvent,
			record:             pendingRecord(),
			publishErr:         errPublish,
			wantGetByUID:       true,
			wantVerifierCalled: true,
			wantMarkCalled:     true,
			wantPublishCalled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getCalls := 0
			store := &mocks.InviteStore{
				MarkAcceptedFunc: func(_ context.Context, _, _ string, _ time.Time) error {
					return tt.markAcceptedErr
				},
				GetByUIDFunc: func(_ context.Context, _ string) (*model.InviteRecord, error) {
					getCalls++
					if getCalls == 1 && tt.getErr != nil {
						return nil, tt.getErr
					}
					if getCalls > 1 && tt.publishGetErr != nil {
						return nil, tt.publishGetErr
					}
					return tt.record, nil
				},
			}
			publisher := &mocks.EventPublisher{
				PublishFunc: func(_ context.Context, _ string, _ []byte) error {
					return tt.publishErr
				},
			}
			verifier := &stubTokenVerifier{claims: tt.verifierClaims, err: tt.verifierErr}

			svc := NewAcceptanceService(store, publisher, verifier)
			svc.HandleInviteAccepted(context.Background(), tt.event)

			if tt.wantGetByUID != (len(store.GetByUIDCalls) > 0) {
				t.Fatalf("GetByUID called = %v, want %v", len(store.GetByUIDCalls) > 0, tt.wantGetByUID)
			}
			if tt.wantGetByUID && store.GetByUIDCalls[0] != tt.event.InviteUID {
				t.Errorf("GetByUID UID: got %q, want %q", store.GetByUIDCalls[0], tt.event.InviteUID)
			}

			if tt.wantVerifierCalled != (len(verifier.calls) > 0) {
				t.Fatalf("Verify called = %v, want %v", len(verifier.calls) > 0, tt.wantVerifierCalled)
			}
			if tt.wantVerifierCalled && verifier.calls[0] != tt.event.Token {
				t.Errorf("Verify token: got %q, want %q", verifier.calls[0], tt.event.Token)
			}

			if tt.wantMarkCalled && len(store.MarkAcceptedCalls) == 0 {
				t.Fatal("expected MarkAccepted to be called, but it was not")
			}
			if !tt.wantMarkCalled && len(store.MarkAcceptedCalls) != 0 {
				t.Fatalf("expected MarkAccepted NOT to be called, but got %d calls", len(store.MarkAcceptedCalls))
			}
			if tt.wantMarkCalled {
				call := store.MarkAcceptedCalls[0]
				if call.UID != tt.event.InviteUID {
					t.Errorf("MarkAccepted UID: got %q, want %q", call.UID, tt.event.InviteUID)
				}
				if tt.wantUsername != "" && call.Username != tt.wantUsername {
					t.Errorf("MarkAccepted Username: got %q, want %q", call.Username, tt.wantUsername)
				}
			}

			if tt.wantPublishCalled && len(publisher.PublishCalls) == 0 {
				t.Fatal("expected Publish to be called, but it was not")
			}
			if !tt.wantPublishCalled && len(publisher.PublishCalls) != 0 {
				t.Fatalf("expected Publish NOT to be called, but got %d calls", len(publisher.PublishCalls))
			}
			if tt.wantPublishCalled && publisher.PublishCalls[0].Subject != api.InviteServiceAcceptedSubject {
				t.Errorf("Publish subject: got %q, want %q", publisher.PublishCalls[0].Subject, api.InviteServiceAcceptedSubject)
			}
		})
	}
}
