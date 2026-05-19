// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package observability

import (
	"context"
	"log"
	"log/slog"
	"os"

	slogotel "github.com/remychantenay/slog-otel"
)

type ctxKey string

const (
	slogFields      ctxKey = "slog_fields"
	logLevelDefault        = slog.LevelDebug
	debug                  = "debug"
	warn                   = "warn"
	info                   = "info"
)

type contextHandler struct {
	slog.Handler
}

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if attrs, ok := ctx.Value(slogFields).([]slog.Attr); ok {
		for _, v := range attrs {
			r.AddAttrs(v)
		}
	}
	return h.Handler.Handle(ctx, r)
}

// InitStructureLogConfig initializes the structured log configuration.
func InitStructureLogConfig() {
	logLevel := new(slog.LevelVar)
	logLevel.Set(logLevelDefault)

	switch os.Getenv("LOG_LEVEL") {
	case debug:
		logLevel.Set(slog.LevelDebug)
	case info:
		logLevel.Set(slog.LevelInfo)
	case warn:
		logLevel.Set(slog.LevelWarn)
	default:
		logLevel.Set(logLevelDefault)
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := contextHandler{slogotel.OtelHandler{
		Next: slog.NewJSONHandler(os.Stderr, opts),
	}}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	log.SetOutput(os.Stderr)
}
