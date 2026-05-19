// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"

	natsinfra "github.com/linuxfoundation/lfx-v2-invite-service/internal/infrastructure/nats"
)

// subscription pairs a human-readable name with a function that starts a consumer
// and returns a stop function.
type subscription struct {
	name  string
	start func(ctx context.Context, nc *natsinfra.Client) (func(), error)
}

// subscriptions lists every NATS consumer the invite-service manages.
// To add a new consumer, append an entry here — no other wiring required.
var subscriptions = []subscription{
	{
		name: "send-invite",
		start: func(ctx context.Context, nc *natsinfra.Client) (func(), error) {
			return nc.StartSendInviteConsumer(ctx, NotificationSvc.HandleSendInvite)
		},
	},
}

// StartSubscriptions binds all NATS subscribers and returns their stop functions.
func StartSubscriptions(ctx context.Context) ([]func(), error) {
	var stops []func()

	for _, sub := range subscriptions {
		stop, err := sub.start(ctx, NATSClient)
		if err != nil {
			return stops, fmt.Errorf("start subscription %q: %w", sub.name, err)
		}
		stops = append(stops, stop)
		slog.InfoContext(ctx, "subscription started", "name", sub.name)
	}

	return stops, nil
}
