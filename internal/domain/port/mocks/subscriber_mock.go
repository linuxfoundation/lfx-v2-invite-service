// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"
	"fmt"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
)

// Subscriber is a test double for port.Subscriber.
// By default every QueueSubscribe call succeeds and returns a no-op stop func.
// Inject QueueSubscribeFunc to override the behaviour.
type Subscriber struct {
	QueueSubscribeFunc func(subject, queue string, handler func(context.Context, port.InboundMessage)) (func(), error)
	Calls              []SubscriberCall
}

// SubscriberCall records a single QueueSubscribe invocation.
type SubscriberCall struct {
	Subject string
	Queue   string
}

func (m *Subscriber) QueueSubscribe(subject, queue string, handler func(context.Context, port.InboundMessage)) (func(), error) {
	m.Calls = append(m.Calls, SubscriberCall{Subject: subject, Queue: queue})
	if m.QueueSubscribeFunc != nil {
		return m.QueueSubscribeFunc(subject, queue, handler)
	}
	return func() {}, nil
}

// FailOnSubject returns a QueueSubscribeFunc that returns an error for the
// given subject and succeeds for all others. Useful for testing partial
// subscription failure paths in Server.Start.
func FailOnSubject(subject string) func(string, string, func(context.Context, port.InboundMessage)) (func(), error) {
	return func(s, q string, _ func(context.Context, port.InboundMessage)) (func(), error) {
		if s == subject {
			return nil, fmt.Errorf("injected failure for subject %q", s)
		}
		return func() {}, nil
	}
}
