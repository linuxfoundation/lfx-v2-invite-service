# Claude Development Guide for LFX V2 Invite Service

## Project Overview

The LFX V2 Invite Service is a Go microservice in the LFX v2 platform. It handles:
- **Current**: Receiving `send_invite` requests from resource services via NATS request/reply, rendering the invite email template, forwarding to the email service for delivery, returning the invite UUID to the caller, **persisting invite records to a NATS JetStream KV bucket (`invites`)**, handling `lfx.invite.accepted` events from the self-serve app to mark records as accepted, and exposing NATS request/reply subjects to look up invite data by UID or email.
- **Future**: `/invite/:uuid` acceptance endpoint enhancements; invite revocation flow.

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
├── model/                    # Pure data: SendInviteRequest, InviteRecord, Role, DeliveryState, etc.
└── port/                     # Interfaces: EmailSender, InviteStore; mocks/

internal/service/
├── notification.go           # Business logic — receives config via NotificationConfig struct; persists invite on send
├── acceptance.go             # Handles lfx.invite.accepted events → marks KV record accepted
└── invite_read.go            # GetInvite / GetInvitesByEmail — domain→api converter

internal/infrastructure/
├── nats/
│   ├── client.go             # NATS connection + KeyValue() bind helper
│   ├── email_sender.go       # NATSEmailSender — renders template, forwards to email service
│   ├── invite_repository.go  # NATSInviteRepository — KV-backed InviteStore implementation
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
1. Add the handler method to the relevant service in `internal/service/`
2. Add a queue-subscribe block in `cmd/invite-api/service/subscriptions.go` and append the stop func
3. Add subject constant + payload types to `pkg/api/invite.go`
4. Wire any new infrastructure (e.g. a new KV binding) in `cmd/invite-api/service/implementations.go`

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

Authoritative subject constants and payload types live in `pkg/api/invite.go`.

| Subject | Direction | Description |
|---|---|---|
| `lfx.invite-service.send_invite` | Request/reply (consumed) | Resource services send `SendInviteRequest`; invite service replies with `SendInviteResponse` (flat: `uid`, `email`, `expires_at` at top level) and persists the invite record |
| `lfx.invite.accepted` | Event (consumed) | Published by the self-serve web app once a user accepts; invite service marks the KV record as accepted. Own queue group `invite-service-acceptance` — consumed alongside project-service |
| `lfx.invite-service.get_invite` | Request/reply (consumed) | Callers send `GetInviteRequest{UID}`; invite service replies with `GetInviteResponse` |
| `lfx.invite-service.get_invites_by_email` | Request/reply (consumed) | Callers send `GetInvitesByEmailRequest{Email}`; on success invite service replies with a bare `[]Invite` JSON array; on error replies with `GetInvitesByEmailResponse{Error}` |
| `lfx.email-service.send_email` | Request/reply (outbound) | Forward pre-rendered email to the email service for delivery |
| `lfx.invite-service.invite_accepted` | Published (outbound) | Published after KV record is marked accepted; carries enriched invite context (recipient, inviter, resource, role) for downstream services. Best-effort — publish failure is logged but does not block the acceptance flow. TODO: switch upstream consumer to JetStream for retry semantics. |
| `lfx.invite-service.invite.created` | Published (future) | Invite issued |
| `lfx.invite-service.invite.revoked` | Published (future) | Invite revoked |

> **Note on `pkg/constants/nats.go`:** this file defines a stale `InviteAcceptedSubject = "lfx.invite-service.invite.accepted"` (different namespace than the authoritative `"lfx.invite.accepted"` in `pkg/api`). The constants file is largely aspirational and may be removed in a future cleanup; always use `pkg/api` constants as the source of truth.

## NATS KV Storage

The service owns the `invites` NATS JetStream KeyValue bucket:
- **Primary key**: `<inviteUID>` → JSON `InviteRecord`
- **Email index**: `index/email/<normalizedEmail>/<inviteUID>` → inviteUID
- Records are kept indefinitely (no TTL) as a permanent audit trail.
- Bucket is provisioned by the Helm chart via the nack `KeyValue` CRD (see `charts/lfx-v2-invite-service/templates/nats-kv-buckets.yaml`).

### Local development (no Kubernetes)

```bash
# Start NATS with JetStream enabled
docker run -d -p 4222:4222 nats:latest -js

# Create the invites KV bucket
nats kv add invites --history=20 --storage=file
```

Set `INVITES_KV_BUCKET=invites` (or leave unset — defaults to `invites`).

## Related Services

| Service | Relationship |
|---|---|
| `lfx-v2-email-service` | Handles SMTP delivery; this service forwards pre-rendered email bodies to it |
| `lfx-v2-project-service` | Example resource service that will publish `send_invite` requests |
| `lfx-v2-committee-service` | Example resource service that publishes `send_invite` requests |
