// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
)

// EmailSender is a test double for port.EmailSender.
type EmailSender struct {
	SendFunc func(ctx context.Context, req *model.SendInviteRequest) error
	Calls    []*model.SendInviteRequest
}

func (m *EmailSender) SendNotification(ctx context.Context, req *model.SendInviteRequest) error {
	m.Calls = append(m.Calls, req)
	if m.SendFunc != nil {
		return m.SendFunc(ctx, req)
	}
	return nil
}
