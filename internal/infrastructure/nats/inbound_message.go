// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"log/slog"

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
func (m *natsMsg) Reply(ctx context.Context, data []byte) error {
	if m.msg.Reply == "" {
		return nil
	}
	if err := m.msg.Respond(data); err != nil {
		slog.ErrorContext(ctx, "natsMsg: failed to send reply",
			"subject", m.msg.Subject,
			"error", err,
		)
		return err
	}
	return nil
}
