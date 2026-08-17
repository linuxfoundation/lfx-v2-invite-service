// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"net/mail"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

// ErrInviteNotFound is the service-layer sentinel for a missing invite record.
// It wraps port.ErrInviteNotFound so callers can use errors.Is without importing
// the port package directly.
var ErrInviteNotFound = port.ErrInviteNotFound

// InviteReadService provides query operations over the invite KV store.
type InviteReadService struct {
	inviteStore port.InviteStore
}

// NewInviteReadService creates an InviteReadService backed by the given store.
func NewInviteReadService(store port.InviteStore) *InviteReadService {
	return &InviteReadService{inviteStore: store}
}

// GetInvite returns the api.Invite view for the given UID.
// Returns ErrInviteNotFound when no record exists.
func (s *InviteReadService) GetInvite(ctx context.Context, uid string) (*api.Invite, error) {
	record, err := s.inviteStore.GetByUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	inv := domainToAPIInvite(record)
	return &inv, nil
}

// GetInvitesByEmail returns all api.Invite records for the given email address.
// The email is canonicalized via mail.ParseAddress before the store lookup so
// that display-name forms (e.g. "Alice <alice@example.com>") resolve to the same
// index key as the stored canonical address. If parsing fails the raw email is
// used as-is (best-effort, avoids blocking valid bare-address queries on malformed input).
// Returns an empty non-nil slice when no records exist — never returns an error in that case.
func (s *InviteReadService) GetInvitesByEmail(ctx context.Context, email string) ([]api.Invite, error) {
	if addr, err := mail.ParseAddress(email); err == nil {
		email = addr.Address
	}
	records, err := s.inviteStore.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	invites := make([]api.Invite, 0, len(records))
	for _, r := range records {
		invites = append(invites, domainToAPIInvite(r))
	}
	return invites, nil
}

// domainToAPIInvite converts a domain InviteRecord to the public api.Invite view.
func domainToAPIInvite(r *model.InviteRecord) api.Invite {
	return api.Invite{
		UID:    r.UID,
		Status: api.InviteStatus(r.Status),
		Recipient: api.Recipient{
			Name:     r.Recipient.Name,
			Email:    r.Recipient.Email,
			Username: r.Recipient.Username,
			Avatar:   r.Recipient.Avatar,
		},
		Inviter: api.Inviter{
			Name:     r.Inviter.Name,
			Username: r.Inviter.Username,
			Email:    r.Inviter.Email,
			Avatar:   r.Inviter.Avatar,
		},
		Resource: api.Resource{
			UID:  r.Resource.UID,
			Name: r.Resource.Name,
			Type: r.Resource.Type,
		},
		Role:           r.Role,
		OrgName:        r.OrgName,
		ReturnURL:      r.ReturnURL,
		ExpirationDays: r.ExpirationDays,
		CreatedAt:      r.CreatedAt,
		ExpiresAt:      r.ExpiresAt,
		AcceptedAt:     r.AcceptedAt,
		AcceptedBy:     r.AcceptedBy,
	}
}
