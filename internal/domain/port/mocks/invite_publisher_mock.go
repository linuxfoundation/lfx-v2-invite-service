// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

// InvitePublisher is a test double for port.InvitePublisher.
type InvitePublisher struct {
	PublishFunc func(ctx context.Context, event inviteapi.InviteCreatedEvent) error
	Calls       []inviteapi.InviteCreatedEvent
}

func (m *InvitePublisher) PublishInviteCreated(ctx context.Context, event inviteapi.InviteCreatedEvent) error {
	m.Calls = append(m.Calls, event)
	if m.PublishFunc != nil {
		return m.PublishFunc(ctx, event)
	}
	return nil
}
