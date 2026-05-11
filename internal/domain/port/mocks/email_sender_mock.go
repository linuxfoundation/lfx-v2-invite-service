// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
)

// EmailSender is a test double for port.EmailSender.
type EmailSender struct {
	SendFunc func(ctx context.Context, n *model.ProjectAddedNotification) error
	Calls    []*model.ProjectAddedNotification
}

func (m *EmailSender) SendProjectAddedNotification(ctx context.Context, n *model.ProjectAddedNotification) error {
	m.Calls = append(m.Calls, n)
	if m.SendFunc != nil {
		return m.SendFunc(ctx, n)
	}
	return nil
}
