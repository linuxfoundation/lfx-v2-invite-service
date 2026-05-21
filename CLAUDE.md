# Claude Development Guide for LFX V2 Invite Service

## Project Overview

The LFX V2 Invite Service is a Go microservice in the LFX v2 platform. It handles:
- **Current**: Receiving `send_invite` requests from resource services via NATS request/reply, rendering the invite email template, forwarding to the email service for delivery, and returning the invite UUID to the caller.
- **Future**: LFID invite token issuance (NATS KV), `/invite/:uuid` acceptance endpoint, and acceptance broadcast for non-LFID users.

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
    └── subscriptions.go      # Slice of {name, start func} — for-loop starts all consumers

internal/domain/
├── model/                    # Pure data: SendInviteRequest, Role, DeliveryState, etc.
└── port/                     # Interfaces: EmailSender

internal/service/
└── notification.go           # Business logic — receives config via NotificationConfig struct

internal/infrastructure/
├── nats/
│   ├── client.go             # NATS connection
│   ├── consumer.go           # StartSendInviteConsumer (queue-group request/reply subscriber)
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
make build       # Compile binary to bin/lfx-v2-invite-service
make test        # Run tests with race detector
make check       # fmt + lint + license-check + go vet
make lint        # golangci-lint
```

## Conventions

### Config injection
All `os.Getenv` calls belong in `cmd/invite-api/service/config.go` → `AppConfigFromEnv()`. Services receive a typed config struct (e.g., `NotificationConfig`), never call `os.Getenv` themselves.

### Adding a new NATS consumer
1. Add a `Start<Name>Consumer` method on `*nats.Client` in `internal/infrastructure/nats/`
2. Add the handler method to the relevant service in `internal/service/`
3. Append to the `subscriptions` slice in `cmd/invite-api/service/subscriptions.go`

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
| `lfx.invite-service.send_invite` | Request/reply | Resource services send `SendInviteRequest`; invite service replies with `SendInviteResponse{InviteUID}` |
| `lfx.email-service.send_email` | Request/reply | Forward pre-rendered email to the email service for delivery |
| `lfx.invite-service.invite.created` | Published (future) | Invite issued |
| `lfx.invite-service.invite.accepted` | Published (future) | Invite accepted |
| `lfx.invite-service.invite.revoked` | Published (future) | Invite revoked |

## Related Services

| Service | Relationship |
|---|---|
| `lfx-v2-email-service` | Handles SMTP delivery; this service forwards pre-rendered email bodies to it |
| `lfx-v2-project-service` | Example resource service that will publish `send_invite` requests |
| `lfx-v2-committee-service` | Example resource service that publishes `send_invite` requests |
