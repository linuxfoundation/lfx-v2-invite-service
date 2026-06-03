// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

// EventPublisher publishes fire-and-forget NATS messages.
type EventPublisher interface {
	Publish(subject string, data []byte) error
}
