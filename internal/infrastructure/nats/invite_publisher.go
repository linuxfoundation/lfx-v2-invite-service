// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"fmt"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

// NATSInvitePublisher publishes invite lifecycle events over core NATS.
type NATSInvitePublisher struct {
	client *Client
}

// NewNATSInvitePublisher returns an InvitePublisher backed by the given NATS client.
func NewNATSInvitePublisher(client *Client) *NATSInvitePublisher {
	return &NATSInvitePublisher{client: client}
}

// PublishInviteCreated publishes an InviteCreatedEvent on InviteCreatedSubject.
func (p *NATSInvitePublisher) PublishInviteCreated(ctx context.Context, event inviteapi.InviteCreatedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal invite.created event: %w", err)
	}
	if err := p.client.conn.Publish(inviteapi.InviteCreatedSubject, data); err != nil {
		return fmt.Errorf("publish invite.created: %w", err)
	}
	return nil
}
