// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// InboundMessage is the minimal surface a subscription handler needs from an
// incoming message. Keeping the interface small means handlers are testable with
// a simple struct — no real NATS broker, no *nats.Msg, no transport dependency.
//
// Reply sends a response back to the caller. When the message has no reply
// address (fire-and-forget), Reply is a no-op and returns nil. An error is
// returned only when the underlying transport fails to deliver the response.
type InboundMessage interface {
	// Data returns the raw message payload.
	Data() []byte
	// Subject returns the NATS subject the message was published on.
	Subject() string
	// Reply sends data back to the requester. It is a no-op for fire-and-forget
	// messages (those without a reply address) and returns nil in that case.
	Reply(ctx context.Context, data []byte) error
}
