# LFX V2 Invite Service

This repository contains the source code for the LFX v2 invite service.

## Overview

The LFX v2 Invite Service handles two responsibilities:

1. **Phase 1 — "You were added" notifications**: Subscribes to `lfx.projects-api.project_settings.updated` NATS events and sends transactional email notifications to existing-LFID users who are newly granted access to a project or foundation.

2. **Invite issuance (future)**: Will issue time-limited invite tokens (via NATS KV) for non-LFID users, expose an `/invite/:uuid` acceptance endpoint, and broadcast acceptance events so resource-owning services can reconcile member records.

## File Structure

```
├── charts/                          # Helm charts for Kubernetes deployment
│   └── lfx-v2-invite-service/
│       └── templates/
│           └── nats-streams.yaml    # JetStream stream definitions
├── cmd/
│   └── invite-api/                  # Application entry point
│       ├── main.go
│       └── service/
│           ├── config.go            # AppConfig read from env vars
│           ├── implementations.go   # Infrastructure wiring
│           └── subscriptions.go     # NATS consumer registration
├── internal/
│   ├── domain/
│   │   ├── model/                   # Domain models (project events, notifications)
│   │   └── port/                    # Interface definitions (EmailSender, ProjectNameReader)
│   ├── infrastructure/
│   │   ├── nats/                    # NATS client, JetStream consumer, project name lookup
│   │   └── smtp/                    # SMTP email sender + HTML/plain-text templates
│   └── service/                     # Business logic (NotificationService)
└── pkg/
    ├── constants/                   # NATS subjects, env var keys, email defaults
    ├── errors/                      # Error types
    ├── log/                         # Structured logging setup (slog + OTel)
    └── utils/                       # OpenTelemetry SDK bootstrap
```

## Key Design Decisions

- **No HTTP API in Phase 1** — the service is a pure NATS subscriber. An HTTP server will be added when the `/invite/:uuid` acceptance endpoint is needed.
- **JetStream for durability** — the service creates its own `project-settings-events` stream so notifications are not lost if the service is temporarily down.
- **Event-driven, no source service changes** — project-service already publishes `project_settings.updated`; no modifications to upstream services are required for Phase 1.
- **Config injected via struct** — all env vars are read in `cmd/invite-api/service/config.go` and passed into service constructors; no `os.Getenv` calls in business logic.

## Environment Variables

| Variable            | Default                                            | Description                        |
| ------------------- | -------------------------------------------------- | ---------------------------------- |
| `NATS_URL`          | `nats://lfx-platform-nats.lfx.svc.cluster.local:4222` | NATS server URL              |
| `EMAIL_SMTP_HOST`   | `lfx-platform-mailpit-smtp.lfx.svc.cluster.local` | SMTP server host                   |
| `EMAIL_SMTP_PORT`   | `25`                                               | SMTP server port                   |
| `EMAIL_SMTP_USERNAME` | _(empty)_                                        | SMTP username (optional)           |
| `EMAIL_SMTP_PASSWORD` | _(empty)_                                        | SMTP password (optional)           |
| `LFX_BASE_URL`      | `https://lfx.linuxfoundation.org`                  | Base URL for deep links in emails  |
| `LOG_LEVEL`         | `debug`                                            | Log level: debug, info, warn       |
| `OTEL_SERVICE_NAME` | `lfx-v2-invite-service`                            | OpenTelemetry service name         |

## Development

### Prerequisites

- Go 1.25+
- Access to a NATS server (local or cluster)

### Build

```bash
make build
# binary: bin/lfx-v2-invite-service
```

### Run locally

```bash
NATS_URL=nats://localhost:4222 ./bin/lfx-v2-invite-service
```

### Test

```bash
make test
```

### Lint & format

```bash
make check
```

## Adding a New Subscription

1. Implement a `Start<Name>Consumer` method on `nats.Client` in `internal/infrastructure/nats/`.
2. Add a handler method to the appropriate service in `internal/service/`.
3. Append a new `subscription{}` entry to the `subscriptions` slice in `cmd/invite-api/service/subscriptions.go` — the for-loop in `StartSubscriptions` picks it up automatically.

## Releases

1. Update `charts/lfx-v2-invite-service/Chart.yaml` `version` field.
2. Merge the PR, then create a GitHub release with a `v{version}` tag.
3. CI builds and publishes the container image and Helm chart automatically.

## License

Copyright The Linux Foundation and each contributor to LFX.

Source code is licensed under the MIT License. See `LICENSE`.
Documentation is licensed under CC-BY-4.0. See `LICENSE-docs`.
