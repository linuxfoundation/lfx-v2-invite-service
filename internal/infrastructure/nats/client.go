// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

)

// Client wraps the NATS connection and provides infrastructure operations.
type Client struct {
	conn *nats.Conn
}

// New creates a NATS client connected to the given URL.
func New(ctx context.Context, url string) (*Client, error) {
	conn, err := nats.Connect(url,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.ErrorContext(ctx, "NATS disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.InfoContext(ctx, "NATS reconnected", "url", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, newServiceUnavailable("failed to connect to NATS", err)
	}

	slog.InfoContext(ctx, "NATS connected", "url", conn.ConnectedUrl())
	return &Client{conn: conn}, nil
}

// Close drains and closes the NATS connection.
func (c *Client) Close() {
	if c.conn != nil {
		_ = c.conn.Drain()
	}
}

// IsReady returns an error if the connection is not usable.
func (c *Client) IsReady() error {
	if c.conn == nil || !c.conn.IsConnected() || c.conn.IsDraining() {
		return newServiceUnavailable("NATS client is not ready")
	}
	return nil
}

// Request sends a synchronous NATS request and returns the raw response bytes.
func (c *Client) Request(ctx context.Context, subject string, data []byte) ([]byte, error) {
	msg, err := c.conn.RequestWithContext(ctx, subject, data)
	if err != nil {
		return nil, newServiceUnavailable("NATS request failed", err)
	}
	return msg.Data, nil
}

// ConsumeWithJetStream binds a durable JetStream consumer and delivers messages to handler.
// Messages are ACKed on success and NAKed on handler error; redelivery timing is governed
// by the ConsumerConfig (MaxDeliver + AckWait). The returned stop function must be called on shutdown.
func (c *Client) ConsumeWithJetStream(
	ctx context.Context,
	streamName string,
	cfg jetstream.ConsumerConfig,
	handler func(ctx context.Context, subject string, data []byte) error,
) (func(), error) {
	js, err := jetstream.New(c.conn)
	if err != nil {
		return nil, newServiceUnavailable("failed to create JetStream client", err)
	}

	consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, cfg)
	if err != nil {
		return nil, newServiceUnavailable("failed to create JetStream consumer", err)
	}

	consumeCtx, err := consumer.Consume(func(msg jetstream.Msg) {
		if handlerErr := handler(ctx, msg.Subject(), msg.Data()); handlerErr != nil {
			slog.ErrorContext(ctx, "stream message handler error — NAKing",
				"error", handlerErr,
				"subject", msg.Subject(),
				"consumer", cfg.Name,
			)
			if nakErr := msg.Nak(); nakErr != nil {
				slog.ErrorContext(ctx, "failed to NAK message", "error", nakErr)
			}
			return
		}
		if ackErr := msg.Ack(); ackErr != nil {
			slog.ErrorContext(ctx, "failed to ACK message", "error", ackErr)
		}
	})
	if err != nil {
		return nil, newServiceUnavailable("failed to start JetStream consume loop", err)
	}

	slog.InfoContext(ctx, "JetStream durable consumer started",
		"stream", streamName,
		"consumer", cfg.Name,
		"filter_subjects", cfg.FilterSubjects,
	)

	return consumeCtx.Stop, nil
}
