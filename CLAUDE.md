# Claude Development Guide for LFX V2 Invite Service

## Project Overview

The LFX V2 Invite Service is a Go microservice in the LFX v2 platform. It handles:
- **Current**: Receiving `send_invite` requests via NATS request/reply, rendering the invite email template, forwarding to the email service for delivery, and returning the invite UUID to the caller.
- **Future**: LFID invite token issuance (NATS KV), `/invite?token=<jwt>` acceptance endpoint (served by the self-serve web app), and acceptance broadcast for non-LFID users.

## Key Technologies

- **Language**: Go 1.25+
- **Messaging**: NATS core (request/reply queue groups)
- **Email**: SMTP (same Mailpit-backed sender as auth-service in development)
- **Observability**: OpenTelemetry (traces, metrics, logs) + slog structured logging
- **Container**: Chainguard distroless images
- **Orchestration**: Kubernetes with Helm charts

## Architecture

```
cmd/invite-api/
├── main.go                   # OTel bootstrap, signal handling, graceful shutdown
└── service/
    ├── config.go             # ALL env var reads live here — no os.Getenv in other layers
    ├── implementations.go    # Wires infrastructure into service structs
    └── subscriptions.go      # NATS subscriber registration (one QueueSubscribe per subject)

internal/domain/
├── model/                    # Pure data: SendInviteRequest, Role, DeliveryState, etc.
└── port/                     # Interfaces: EmailSender

internal/service/
└── notification.go           # Business logic — receives config via NotificationConfig struct

internal/infrastructure/
├── auth/
│   └── generator.go          # JWT link generator (HS256) — signs the invite return URL
├── nats/
│   ├── client.go             # NATS connection
│   ├── email_sender.go       # NATSEmailSender — renders template, forwards to email service
│   └── errors.go             # ServiceUnavailable, Unexpected error types (unexported)
├── observability/
│   ├── log.go                # slog + OTel handler init
│   └── otel.go               # OTel SDK bootstrap
└── smtp/
    ├── templates.go          # Template rendering functions
    └── templates/            # Embedded template files
        ├── invite_body.gohtml
        ├── invite_subject.gotemplate
        └── invite_text.gotemplate

pkg/
└── api/
    └── invite.go             # Public contract: NATS subjects, SendInviteRequest, InviteRole
```

## Build Commands

```bash
make build       # Compile binary to bin/invite-api
make test        # Run tests with race detector
make check       # fmt + lint + license-check + go vet
make lint        # golangci-lint
```

## Conventions

### Config injection
All `os.Getenv` calls belong in `cmd/invite-api/service/config.go` → `AppConfigFromEnv()`. Services receive a typed config struct (e.g., `NotificationConfig`), never call `os.Getenv` themselves.

### Adding a new NATS consumer
1. Add the handler method to the relevant service in `internal/service/`
2. Add a new `QueueSubscribe` call in `StartSubscriptions` in `cmd/invite-api/service/subscriptions.go`, following the pattern of the existing `send_invite` subscriber
3. Wire any new infrastructure via `cmd/invite-api/service/implementations.go` and shut it down in `Shutdown()`

### Error handling
- Infrastructure errors → unexported `newServiceUnavailable` / `newUnexpected` in `internal/infrastructure/nats/errors.go`
- Return errors up; log at the point where you have the most context
- Malformed NATS payloads: reply with error and discard (they will never parse successfully on retry)

### Logging
- Use `slog.DebugContext`, `slog.InfoContext`, `slog.WarnContext`, `slog.ErrorContext`
- Always pass `ctx` so OTel trace correlation works
- Log notification outcomes via `auditNotification` (structured `notification_audit` INFO line)

### License headers
Every `.go` file must start with:
```go
// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT
```

## NATS Subjects

| Subject | Direction | Description |
|---|---|---|
| `lfx.invite-service.send_invite` | Request/reply | Callers send `SendInviteRequest`; invite service replies with `SendInviteResponse{Invite}` |
| `lfx.email-service.send_email` | Request/reply | Forward pre-rendered email to the email service for delivery |
| `lfx.invite.accepted` | Published (future) | Invite accepted — published by the self-serve web app, not this service; `lfx.invite.*` namespace intentional |

## Related Services

| Service | Relationship |
|---|---|
| `lfx-v2-email-service` | Handles SMTP delivery; this service forwards pre-rendered email bodies to it |
