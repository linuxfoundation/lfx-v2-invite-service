// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
)

// EmailSender is a test double for port.EmailSender.
type EmailSender struct {
	SendFunc func(ctx context.Context, payload model.InviteEmailPayload) error
	Calls    []model.InviteEmailPayload
}

func (m *EmailSender) SendNotification(ctx context.Context, payload model.InviteEmailPayload) error {
	m.Calls = append(m.Calls, payload)
	if m.SendFunc != nil {
		return m.SendFunc(ctx, payload)
	}
	return nil
}
