// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import "context"

// EventPublisher is a test double for port.EventPublisher.
type EventPublisher struct {
	PublishFunc func(ctx context.Context, subject string, data []byte) error

	PublishCalls []PublishCall
}

// PublishCall records the arguments of a single Publish call.
type PublishCall struct {
	Ctx     context.Context
	Subject string
	Data    []byte
}

func (m *EventPublisher) Publish(ctx context.Context, subject string, data []byte) error {
	m.PublishCalls = append(m.PublishCalls, PublishCall{Ctx: ctx, Subject: subject, Data: data})
	if m.PublishFunc != nil {
		return m.PublishFunc(ctx, subject, data)
	}
	return nil
}
