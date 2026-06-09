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

func sampleRecord(uid string) *model.InviteRecord {
	return &model.InviteRecord{
		UID:    uid,
		Status: model.InviteStatusPending,
		Recipient: model.Recipient{
			Name:  "Alice",
			Email: "alice@example.com",
		},
		Inviter: model.Inviter{
			Name:     "Bob",
			Username: "bob",
		},
		Resource: model.InviteResource{
			UID:  "res-1",
			Name: "My Project",
			Type: "project",
		},
		Role:      "Member",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}
}

func TestInviteReadService_GetInvite(t *testing.T) {
	tests := []struct {
		name       string
		uid        string
		storeFunc  func(_ context.Context, uid string) (*model.InviteRecord, error)
		wantInvite *api.Invite
		wantErr    error
	}{
		{
			name: "returns invite for known UID",
			uid:  "known-uid",
			storeFunc: func(_ context.Context, uid string) (*model.InviteRecord, error) {
				return sampleRecord(uid), nil
			},
			wantInvite: &api.Invite{UID: "known-uid"},
		},
		{
			name: "returns ErrInviteNotFound for unknown UID",
			uid:  "unknown-uid",
			storeFunc: func(_ context.Context, _ string) (*model.InviteRecord, error) {
				return nil, port.ErrInviteNotFound
			},
			wantErr: port.ErrInviteNotFound,
		},
		{
			name: "propagates store error",
			uid:  "any-uid",
			storeFunc: func(_ context.Context, _ string) (*model.InviteRecord, error) {
				return nil, errors.New("store down")
			},
			wantErr: errors.New("store down"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mocks.InviteStore{GetByUIDFunc: tt.storeFunc}
			svc := NewInviteReadService(store)

			inv, err := svc.GetInvite(context.Background(), tt.uid)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
					t.Errorf("error: got %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if inv == nil {
				t.Fatal("expected invite, got nil")
			}
			if tt.wantInvite != nil && inv.UID != tt.uid {
				t.Errorf("invite UID: got %q, want %q", inv.UID, tt.uid)
			}
		})
	}
}

func TestInviteReadService_GetInvitesByEmail(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		storeFunc func(_ context.Context, email string) ([]*model.InviteRecord, error)
		wantLen   int
		wantErr   bool
	}{
		{
			name:  "returns all invites for email",
			email: "alice@example.com",
			storeFunc: func(_ context.Context, _ string) ([]*model.InviteRecord, error) {
				return []*model.InviteRecord{
					sampleRecord("uid-1"),
					sampleRecord("uid-2"),
				}, nil
			},
			wantLen: 2,
		},
		{
			name:  "returns empty slice when no invites exist",
			email: "nobody@example.com",
			storeFunc: func(_ context.Context, _ string) ([]*model.InviteRecord, error) {
				return nil, nil
			},
			wantLen: 0,
		},
		{
			name:  "propagates store errors",
			email: "alice@example.com",
			storeFunc: func(_ context.Context, _ string) ([]*model.InviteRecord, error) {
				return nil, errors.New("store down")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mocks.InviteStore{GetByEmailFunc: tt.storeFunc}
			svc := NewInviteReadService(store)

			invites, err := svc.GetInvitesByEmail(context.Background(), tt.email)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(invites) != tt.wantLen {
				t.Errorf("invite count: got %d, want %d", len(invites), tt.wantLen)
			}
		})
	}
}

func TestInviteReadService_GetInvite_ConvertsAllFields(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	accepted := now.Add(time.Hour)
	record := &model.InviteRecord{
		UID:    "full-uid",
		Status: model.InviteStatusAccepted,
		Recipient: model.Recipient{
			Name:     "Alice",
			Email:    "alice@example.com",
			Username: "alice-lfid",
			Avatar:   "https://avatar.example.com/alice",
		},
		Inviter: model.Inviter{
			Name:     "Bob",
			Username: "bob",
			Email:    "bob@example.com",
			Avatar:   "https://avatar.example.com/bob",
		},
		Resource: model.InviteResource{
			UID:  "res-1",
			Name: "My Project",
			Type: "project",
		},
		Role:           "Manage",
		OrgName:        "The Linux Foundation",
		ReturnURL:      "https://app.lfx.dev/project",
		ExpirationDays: 30,
		CreatedAt:      now,
		ExpiresAt:      now.Add(30 * 24 * time.Hour),
		AcceptedAt:     &accepted,
		AcceptedBy:     "alice-lfid",
	}

	store := &mocks.InviteStore{
		GetByUIDFunc: func(_ context.Context, _ string) (*model.InviteRecord, error) {
			return record, nil
		},
	}
	svc := NewInviteReadService(store)
	inv, err := svc.GetInvite(context.Background(), "full-uid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inv.Status != api.InviteStatusAccepted {
		t.Errorf("Status: got %q, want %q", inv.Status, api.InviteStatusAccepted)
	}
	if inv.Recipient.Username != "alice-lfid" {
		t.Errorf("Recipient.Username: got %q, want %q", inv.Recipient.Username, "alice-lfid")
	}
	if inv.Inviter.Email != "bob@example.com" {
		t.Errorf("Inviter.Email: got %q, want %q", inv.Inviter.Email, "bob@example.com")
	}
	if inv.Resource.Type != "project" {
		t.Errorf("Resource.Type: got %q, want %q", inv.Resource.Type, "project")
	}
	if inv.AcceptedBy != "alice-lfid" {
		t.Errorf("AcceptedBy: got %q, want %q", inv.AcceptedBy, "alice-lfid")
	}
	if inv.AcceptedAt == nil || !inv.AcceptedAt.Equal(accepted) {
		t.Errorf("AcceptedAt: got %v, want %v", inv.AcceptedAt, accepted)
	}
}
