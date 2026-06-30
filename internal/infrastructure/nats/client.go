// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	ctx, span := tracer.Start(ctx, "nats.request",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", subject),
			attribute.String("messaging.operation.type", "send"),
			attribute.Int("messaging.message.body.size", len(data)),
		),
	)
	defer span.End()

	msg := nats.NewMsg(subject)
	msg.Header = make(nats.Header)
	msg.Data = data
	otel.GetTextMapPropagator().Inject(ctx, natsHeaderCarrier(msg.Header))

	reply, err := c.conn.RequestMsgWithContext(ctx, msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, newServiceUnavailable("NATS request failed", err)
	}
	return reply.Data, nil
}

// Publish sends a fire-and-forget NATS message with no reply expected.
func (c *Client) Publish(ctx context.Context, subject string, data []byte) error {
	ctx, span := tracer.Start(ctx, "nats.publish",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", subject),
			attribute.String("messaging.operation.type", "publish"),
			attribute.Int("messaging.message.body.size", len(data)),
		),
	)
	defer span.End()

	msg := nats.NewMsg(subject)
	msg.Header = make(nats.Header)
	msg.Data = data
	otel.GetTextMapPropagator().Inject(ctx, natsHeaderCarrier(msg.Header))

	if err := c.conn.PublishMsg(msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return newServiceUnavailable("NATS publish failed", err)
	}
	return nil
}

// QueueSubscribe registers a core-NATS queue-group subscriber and returns an
// unsubscribe function the caller must invoke on shutdown.
// The handler receives the span context extracted from incoming message headers.
func (c *Client) QueueSubscribe(subject, queue string, handler func(ctx context.Context, msg *nats.Msg)) (func(), error) {
	sub, err := c.conn.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		msgCtx := otel.GetTextMapPropagator().Extract(context.Background(), natsHeaderCarrier(msg.Header))
		msgCtx, span := tracer.Start(msgCtx, "nats.process",
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("messaging.system", "nats"),
				attribute.String("messaging.destination.name", subject),
				attribute.String("messaging.operation.type", "process"),
				attribute.Int("messaging.message.body.size", len(msg.Data)),
			),
		)
		defer span.End()
		handler(msgCtx, msg)
	})
	if err != nil {
		return nil, newServiceUnavailable("failed to subscribe to "+subject, err)
	}
	return func() { _ = sub.Unsubscribe() }, nil
}

// KeyValue binds to an existing NATS JetStream KeyValue bucket by name.
// The bucket must already exist (created externally via Helm / nack CRD or `nats kv add`).
// Returns an error if the bucket cannot be found or the JetStream client cannot be created.
func (c *Client) KeyValue(ctx context.Context, bucket string) (jetstream.KeyValue, error) {
	js, err := jetstream.New(c.conn)
	if err != nil {
		return nil, newServiceUnavailable("failed to create JetStream client", err)
	}
	kv, err := js.KeyValue(ctx, bucket)
	if err != nil {
		return nil, newServiceUnavailable("failed to bind to KV bucket "+bucket, err)
	}
	slog.InfoContext(ctx, "NATS KV bucket bound", "bucket", bucket)
	return kv, nil
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
