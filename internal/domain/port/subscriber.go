// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// Subscriber registers queue-group message handlers and returns a stop function
// per subscription. Implementations are responsible for OTel trace-context
// extraction and wrapping raw messages in InboundMessage before calling the handler.
type Subscriber interface {
	QueueSubscribe(subject, queue string, handler func(ctx context.Context, msg InboundMessage)) (func(), error)
}
