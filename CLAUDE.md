# Claude Development Guide for LFX V2 Invite Service

## Project Overview

The LFX V2 Invite Service is a Go microservice in the LFX v2 platform. It handles:
- **Phase 1**: "You were added" transactional email notifications to existing-LFID users when they are granted project/foundation access
- **Future**: Invite token issuance (NATS KV), `/invite/:uuid` acceptance endpoint, and acceptance broadcast for non-LFID users

## Key Technologies

- **Language**: Go 1.25+
- **Messaging**: NATS with JetStream for durable event consumption
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
├── model/                    # Pure data: ProjectSettingsUpdatedMessage, AddedUser, Role, etc.
└── port/                     # Interfaces: EmailSender, ProjectNameReader

internal/service/
└── notification.go           # Business logic — receives config via NotificationConfig struct

internal/infrastructure/
├── nats/
│   ├── client.go             # NATS connection + ConsumeWithJetStream helper
│   ├── consumer.go           # StartProjectSettingsConsumer (binds durable JetStream consumer)
│   └── project_reader.go     # GetProjectName via NATS request/reply to project-service
└── smtp/
    ├── sender.go             # SMTP sender implementing port.EmailSender
    └── templates.go          # HTML + plain-text email templates

pkg/
├── constants/subjects.go     # NATS subjects, env var keys, stream/consumer names
├── errors/errors.go          # ServiceUnavailable, Unexpected error types
├── log/log.go                # slog + OTel handler init
└── utils/otel.go             # OTel SDK bootstrap (copied from committee-service pattern)
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
- Infrastructure errors → `pkg/errors.NewServiceUnavailable` / `pkg/errors.NewUnexpected`
- Return errors up; log at the point where you have the most context
- Malformed NATS payloads: ACK and skip (they will never parse successfully on retry)

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
| `lfx.projects-api.project_settings.updated` | Consumed | Project permission changes |
| `lfx.projects-api.get_name` | Request/reply | Look up project display name |
| `lfx.invite-service.invite.created` | Published (future) | Invite issued |
| `lfx.invite-service.invite.accepted` | Published (future) | Invite accepted |
| `lfx.invite-service.invite.revoked` | Published (future) | Invite revoked |

## JetStream Streams

| Stream | Subjects | Owner |
|---|---|---|
| `project-settings-events` | `lfx.projects-api.project_settings.updated` | This service (Helm chart) |

## Related Services

| Service | Relationship |
|---|---|
| `lfx-v2-project-service` | Publishes `project_settings.updated`; answers `get_name` requests |
| `lfx-v2-auth-service` | Pattern reference for SMTP sender implementation |
| `lfx-v2-committee-service` | Pattern reference for JetStream consumer and service structure |
