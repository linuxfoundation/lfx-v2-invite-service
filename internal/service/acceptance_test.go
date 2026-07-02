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

func TestAcceptanceService_HandleInviteAccepted(t *testing.T) {
	validRecord := &model.InviteRecord{
		UID:       "uid-123",
		Status:    model.InviteStatusAccepted,
		Recipient: model.Recipient{Email: "alice@example.com"},
		Inviter:   model.Inviter{Username: "bob"},
		Resource:  model.InviteResource{UID: "proj-1", Type: "project"},
		Role:      "Member",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	tests := []struct {
		name              string
		event             api.InviteAcceptedEvent
		markAcceptedErr   error
		getByUIDRecord    *model.InviteRecord
		getByUIDErr       error
		publishErr        error
		wantMarkCalled    bool
		wantUID           string
		wantUsername      string
		wantGetByUIDUID   string
		wantPublishCalled bool
		wantPublishSubj   string
	}{
		{
			name:              "marks invite accepted and publishes enriched event",
			event:             api.InviteAcceptedEvent{InviteUID: "uid-123", Username: "alice"},
			getByUIDRecord:    validRecord,
			wantMarkCalled:    true,
			wantUID:           "uid-123",
			wantUsername:      "alice",
			wantGetByUIDUID:   "uid-123",
			wantPublishCalled: true,
			wantPublishSubj:   api.InviteServiceAcceptedSubject,
		},
		{
			name:           "silently ignores event with missing invite_uid",
			event:          api.InviteAcceptedEvent{InviteUID: "", Username: "alice"},
			wantMarkCalled: false,
		},
		{
			name:           "silently ignores event with missing username",
			event:          api.InviteAcceptedEvent{InviteUID: "uid-123", Username: ""},
			wantMarkCalled: false,
		},
		{
			name:            "silently ignores not-found error (invite belongs to another service)",
			event:           api.InviteAcceptedEvent{InviteUID: "foreign-uid", Username: "bob"},
			markAcceptedErr: port.ErrInviteNotFound,
			wantMarkCalled:  true,
		},
		{
			name:              "skips publish on duplicate/redelivered event (ErrAlreadyAccepted)",
			event:             api.InviteAcceptedEvent{InviteUID: "uid-already", Username: "alice"},
			markAcceptedErr:   port.ErrAlreadyAccepted,
			wantMarkCalled:    true,
			wantPublishCalled: false,
		},
		{
			name:            "logs but does not panic on transient store error",
			event:           api.InviteAcceptedEvent{InviteUID: "uid-456", Username: "carol"},
			markAcceptedErr: errTransient,
			wantMarkCalled:  true,
		},
		{
			name:              "skips publish but does not fail when GetByUID errors",
			event:             api.InviteAcceptedEvent{InviteUID: "uid-123", Username: "alice"},
			getByUIDErr:       errTransient,
			wantMarkCalled:    true,
			wantGetByUIDUID:   "uid-123",
			wantPublishCalled: false,
		},
		{
			name:              "skips publish but does not fail when publish errors",
			event:             api.InviteAcceptedEvent{InviteUID: "uid-123", Username: "alice"},
			getByUIDRecord:    validRecord,
			publishErr:        errPublish,
			wantMarkCalled:    true,
			wantGetByUIDUID:   "uid-123",
			wantPublishCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mocks.InviteStore{
				MarkAcceptedFunc: func(_ context.Context, _, _ string, _ time.Time) error {
					return tt.markAcceptedErr
				},
				GetByUIDFunc: func(_ context.Context, _ string) (*model.InviteRecord, error) {
					return tt.getByUIDRecord, tt.getByUIDErr
				},
			}
			publisher := &mocks.EventPublisher{
				PublishFunc: func(_ context.Context, _ string, _ []byte) error {
					return tt.publishErr
				},
			}
			svc := NewAcceptanceService(store, publisher)
			svc.HandleInviteAccepted(context.Background(), tt.event)

			if tt.wantMarkCalled && len(store.MarkAcceptedCalls) == 0 {
				t.Fatal("expected MarkAccepted to be called, but it was not")
			}
			if !tt.wantMarkCalled && len(store.MarkAcceptedCalls) != 0 {
				t.Fatalf("expected MarkAccepted NOT to be called, but got %d calls", len(store.MarkAcceptedCalls))
			}
			if tt.wantMarkCalled && len(store.MarkAcceptedCalls) > 0 {
				call := store.MarkAcceptedCalls[0]
				if tt.wantUID != "" && call.UID != tt.wantUID {
					t.Errorf("UID: got %q, want %q", call.UID, tt.wantUID)
				}
				if tt.wantUsername != "" && call.Username != tt.wantUsername {
					t.Errorf("Username: got %q, want %q", call.Username, tt.wantUsername)
				}
			}

			if tt.wantGetByUIDUID != "" && len(store.GetByUIDCalls) == 0 {
				t.Fatal("expected GetByUID to be called, but it was not")
			}
			if tt.wantGetByUIDUID != "" && len(store.GetByUIDCalls) > 0 && store.GetByUIDCalls[0] != tt.wantGetByUIDUID {
				t.Errorf("GetByUID UID: got %q, want %q", store.GetByUIDCalls[0], tt.wantGetByUIDUID)
			}

			if tt.wantPublishCalled && len(publisher.PublishCalls) == 0 {
				t.Fatal("expected Publish to be called, but it was not")
			}
			if !tt.wantPublishCalled && len(publisher.PublishCalls) != 0 {
				t.Fatalf("expected Publish NOT to be called, but got %d calls", len(publisher.PublishCalls))
			}
			if tt.wantPublishSubj != "" && len(publisher.PublishCalls) > 0 && publisher.PublishCalls[0].Subject != tt.wantPublishSubj {
				t.Errorf("Publish subject: got %q, want %q", publisher.PublishCalls[0].Subject, tt.wantPublishSubj)
			}
		})
	}
}
