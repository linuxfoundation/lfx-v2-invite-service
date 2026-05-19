# LFX V2 Invite Service

This repository contains the source code for the LFX v2 invite service.

## Overview

The LFX v2 Invite Service handles the following NATS events:

| Subject | Direction | Description |
| ------- | --------- | ----------- |
| `lfx.invite-service.send_invite` | Consumed | Resource services publish a `SendInviteRequest` payload here. The invite service renders the invite email template and forwards the pre-rendered HTML/text to the email service (`lfx.email-service.send_email`) for delivery. |

An HTTP API for LFID invite issuance and acceptance is coming soon.

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
│   │   ├── model/                   # Domain models (invite request, notification, roles)
│   │   └── port/                    # Interface definitions (EmailSender)
│   ├── infrastructure/
│   │   ├── nats/                    # NATS client, JetStream consumer, NATSEmailSender, error types
│   │   ├── observability/           # slog setup and OTel SDK bootstrap
│   │   └── smtp/                    # Template rendering + embedded templates/
│   └── service/                     # Business logic (NotificationService)
└── pkg/
    └── api/                         # Public inter-service contract: subjects, SendInviteRequest, InviteRole
```

## Key Design Decisions

- **No HTTP API yet** — the service is currently a pure NATS subscriber. An HTTP server will be added when the LFID invite management API is needed.
- **JetStream for durability** — the service owns the `invite-requests` stream; messages are not lost if the service is temporarily down.
- **Template ownership** — the invite service owns and renders the email template; the email service (`lfx.email-service.send_email`) handles SMTP delivery. Callers publish a `SendInviteRequest` with structured fields — no pre-rendered HTML required from them.
- **Config injected via struct** — all env vars are read in `cmd/invite-api/service/config.go` and passed into service constructors; no `os.Getenv` calls in business logic.

## Environment Variables

| Variable            | Default                                            | Description                        |
| ------------------- | -------------------------------------------------- | ---------------------------------- |
| `NATS_URL`              | `nats://lfx-platform-nats.lfx.svc.cluster.local:4222` | NATS server URL                                  |
| `LFX_BASE_URL`          | `https://lfx.linuxfoundation.org`                  | Fallback deep-link URL when request omits one     |
| `INVITE_JWT_SECRET`     | (required)                                         | HMAC-SHA256 key for signing invite JWTs           |
| `INVITE_LINK_BASE_URL`  | `https://lfx.linuxfoundation.org`                  | Base URL of the invite acceptance web app         |
| `LOG_LEVEL`             | `debug`                                            | Log level: debug, info, warn                      |
| `OTEL_SERVICE_NAME`     | `lfx-v2-invite-service`                            | OpenTelemetry service name                        |

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

1. Merge the PR, then create a GitHub release with a `v{version}` tag.
2. CI builds and publishes the container image and Helm chart automatically.

## License

Copyright The Linux Foundation and each contributor to LFX.

Source code is licensed under the MIT License. See `LICENSE`.
Documentation is licensed under CC-BY-4.0. See `LICENSE-docs`.
