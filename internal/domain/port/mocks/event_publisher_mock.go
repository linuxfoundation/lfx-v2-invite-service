// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

// EventPublisher is a test double for port.EventPublisher.
type EventPublisher struct {
	PublishFunc func(subject string, data []byte) error

	PublishCalls []PublishCall
}

// PublishCall records the arguments of a single Publish call.
type PublishCall struct {
	Subject string
	Data    []byte
}

func (m *EventPublisher) Publish(subject string, data []byte) error {
	m.PublishCalls = append(m.PublishCalls, PublishCall{Subject: subject, Data: data})
	if m.PublishFunc != nil {
		return m.PublishFunc(subject, data)
	}
	return nil
}
