// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"

	"github.com/nats-io/nats.go"
)

// natsMsg wraps a *nats.Msg to satisfy port.InboundMessage.
// It is created inside QueueSubscribe and never escapes to callers.
type natsMsg struct {
	msg *nats.Msg
}

func (m *natsMsg) Data() []byte    { return m.msg.Data }
func (m *natsMsg) Subject() string { return m.msg.Subject }

// Reply sends a response to the requester. When the message has no reply
// address (fire-and-forget), it returns nil without sending anything.
// Errors are returned to the caller for logging at the point with the most context.
func (m *natsMsg) Reply(_ context.Context, data []byte) error {
	if m.msg.Reply == "" {
		return nil
	}
	return m.msg.Respond(data)
}
