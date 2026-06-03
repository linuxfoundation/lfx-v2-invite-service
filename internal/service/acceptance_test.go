// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port/mocks"
	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

var errTransient = errors.New("transient kv error")

func TestAcceptanceService_HandleInviteAccepted(t *testing.T) {
	tests := []struct {
		name            string
		event           api.InviteAcceptedEvent
		markAcceptedErr error
		wantCalled      bool
		wantUID         string
		wantUsername    string
	}{
		{
			name:         "marks invite accepted on valid event",
			event:        api.InviteAcceptedEvent{InviteUID: "uid-123", Username: "alice"},
			wantCalled:   true,
			wantUID:      "uid-123",
			wantUsername: "alice",
		},
		{
			name:       "silently ignores event with missing invite_uid",
			event:      api.InviteAcceptedEvent{InviteUID: "", Username: "alice"},
			wantCalled: false,
		},
		{
			name:       "silently ignores event with missing username",
			event:      api.InviteAcceptedEvent{InviteUID: "uid-123", Username: ""},
			wantCalled: false,
		},
		{
			name:            "silently ignores not-found error (invite belongs to another service)",
			event:           api.InviteAcceptedEvent{InviteUID: "foreign-uid", Username: "bob"},
			markAcceptedErr: port.ErrInviteNotFound,
			wantCalled:      true,
		},
		{
			name:            "logs but does not panic on transient store error",
			event:           api.InviteAcceptedEvent{InviteUID: "uid-456", Username: "carol"},
			markAcceptedErr: errTransient,
			wantCalled:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mocks.InviteStore{
				MarkAcceptedFunc: func(_ context.Context, _, _ string, _ time.Time) error {
					return tt.markAcceptedErr
				},
			}
			svc := NewAcceptanceService(store)
			svc.HandleInviteAccepted(context.Background(), tt.event)

			if tt.wantCalled && len(store.MarkAcceptedCalls) == 0 {
				t.Fatal("expected MarkAccepted to be called, but it was not")
			}
			if !tt.wantCalled && len(store.MarkAcceptedCalls) != 0 {
				t.Fatalf("expected MarkAccepted NOT to be called, but got %d calls", len(store.MarkAcceptedCalls))
			}
			if tt.wantCalled && len(store.MarkAcceptedCalls) > 0 {
				call := store.MarkAcceptedCalls[0]
				if tt.wantUID != "" && call.UID != tt.wantUID {
					t.Errorf("UID: got %q, want %q", call.UID, tt.wantUID)
				}
				if tt.wantUsername != "" && call.Username != tt.wantUsername {
					t.Errorf("Username: got %q, want %q", call.Username, tt.wantUsername)
				}
			}
		})
	}
}
