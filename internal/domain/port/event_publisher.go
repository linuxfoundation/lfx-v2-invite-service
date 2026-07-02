// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// EventPublisher publishes fire-and-forget NATS messages.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}
