// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

// InvitePublisher publishes invite lifecycle events so that resource services
// can correlate the invite UUID with their own records.
type InvitePublisher interface {
	PublishInviteCreated(ctx context.Context, event inviteapi.InviteCreatedEvent) error
}
