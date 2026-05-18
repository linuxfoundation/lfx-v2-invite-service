// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
)

// EmailSender sends transactional notification emails.
type EmailSender interface {
	SendNotification(ctx context.Context, req *model.SendInviteRequest) error
}
