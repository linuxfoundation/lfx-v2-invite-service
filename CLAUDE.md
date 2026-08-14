# CLAUDE.md — lfx-v2-invite-service

Development guide for Claude instances working on this service.

## Service Overview

NATS request/reply invite broker. Receives `send_invite` requests from resource services, renders the invite email template (HS256-signed JWT link), forwards the pre-rendered email to the email service for delivery, persists the invite record in a NATS JetStream KV bucket, and exposes lookup subjects so downstream services can query invites by UID or email. Handles `lfx.invite.accepted` events to mark records accepted and publishes enriched `lfx.invite-service.invite_accepted` events for downstream consumers.

**Technologies:** Go 1.25, NATS JetStream KeyValue, `golang-jwt/jwt`, Kubernetes/Helm

## Repo Role

This repo owns the invite lifecycle: send, persist, accept, and look up invite records. It owns the `invites` KV bucket, the `pkg/api` public contract, invite JWT signing, email template rendering, and the service-local Helm chart. It does **not** own email SMTP delivery (that belongs to `lfx-v2-email-service`), authorization tuple management, or resource membership state.

## Architecture

```
cmd/invite-api/
├── main.go                     # OTel bootstrap, build version injection, signal handling, graceful shutdown
└── service/
    ├── config.go               # ALL env var reads live here — no os.Getenv in other layers
    ├── implementations.go      # Wires infrastructure into service structs; global vars NATSClient, NotificationSvc, etc.
    └── subscriptions.go        # NATS subscriber registration (one QueueSubscribe per subject)

internal/domain/
├── model/                      # Pure data: SendInviteRequest (+ Resolved* helpers), InviteRecord, Inviter, Recipient, etc.
└── port/                       # Interfaces: EmailSender, InviteStore, EventPublisher; error sentinels; mocks/

internal/service/
├── notification.go             # HandleSendInvite — validates, generates JWT link, persists, sends; ErrInvalidRequest, ErrEmailDispatchFailed
├── acceptance.go               # HandleInviteAccepted — marks KV record accepted, publishes InviteServiceAcceptedEvent
└── invite_read.go              # GetInvite / GetInvitesByEmail — domain→api converter; ErrInviteNotFound sentinel

internal/infrastructure/
├── auth/
│   └── generator.go            # LinkGenerator — HS256 JWT invite links; 30-day default, 90-day max
├── nats/
│   ├── client.go               # NATS connection; QueueSubscribe (auto OTel trace propagation); Request/Publish; KeyValue bind; ConsumeWithJetStream
│   ├── email_sender.go         # NATSEmailSender — renders template, forwards to email service via NATS
│   ├── invite_repository.go    # NATSInviteRepository — KV-backed InviteStore; email index base64url-encoded
│   ├── tracing.go              # natsHeaderCarrier — adapts nats.Header to OTel TextMapCarrier
│   └── errors.go               # Unexported newServiceUnavailable / newUnexpected error constructors
├── observability/
│   ├── log.go                  # slog + OTel handler init; InitStructureLogConfig
│   └── otel.go                 # OTel SDK bootstrap (traces, metrics, logs via autoexport)
└── smtp/
    ├── templates.go            # InviteEmailSubject / RenderInviteHTML / RenderInvitePlain
    └── templates/              # Embedded: invite_body.gohtml, invite_subject.gotemplate, invite_text.gotemplate

pkg/
└── api/
    └── invite.go               # PUBLIC contract: NATS subjects, SendInviteRequest, InviteRole, Invite, response types
```

### Key design decisions

- **OTel trace propagation is automatic.** `Client.QueueSubscribe` in `internal/infrastructure/nats/client.go` extracts the incoming trace context from NATS message headers and starts a consumer span before calling the handler. Handlers do not need to start their own spans — just use the `ctx` they receive.
- **pkg/api is the public contract.** Any service that wants to interact with the invite service imports `github.com/linuxfoundation/lfx-v2-invite-service/pkg/api` for subject constants and payload types. Never expose `internal/` packages to callers.
- **`pkg/constants` must not be used or extended.** The three files `pkg/constants/nats.go`, `pkg/constants/env.go`, and `pkg/constants/email.go` are aspirational stubs that predate the real implementation and contain stale or unused values (e.g. `InviteAcceptedSubject = "lfx.invite-service.invite.accepted"` conflicts with the live subject `"lfx.invite.accepted"` in `pkg/api`). Do not add constants here; always use `pkg/api` as the source of truth.
- **Persist before send; rollback on failure.** `HandleSendInvite` writes the invite record to KV before calling the email service. If the email dispatch fails, it attempts a best-effort KV delete to avoid phantom invites.
- **JWT signed invite links are never stored.** The KV record stores the *destination URL*, not the signed token. The token is generated on the fly and included in the email body only.
- **Email index uses base64url encoding.** Raw email addresses contain characters (`@`, `+`) that are not valid NATS KV key segments. The repository encodes emails with `base64.RawURLEncoding` before using them as key prefixes. Both read and write paths use the same encoding so prefix scans stay correct.
- **Optimistic concurrency on `MarkAccepted`.** The repository retries `kv.Update` up to 3 times on revision mismatch before failing. `ErrAlreadyAccepted` is returned when the record is already in the accepted state so callers can skip duplicate side-effects (e.g. avoid re-publishing `InviteServiceAcceptedEvent`).
- **Handlers always respond.** Every request/reply handler calls `msg.Respond` on every path (success or error) so callers' `RequestWithContext` never hangs.
- **30-second handler timeout for send_invite; 10 seconds for KV read/write handlers.** These constants are defined in `cmd/invite-api/service/subscriptions.go`.

## Development Workflow

### Build version injection

`make build` and `make run` inject two variables at link time:

| Variable | Source | Default (no build) |
|---|---|---|
| `main.Version` | `git describe --tags --always` | `"dev"` |
| `main.GitCommit` | `git rev-parse HEAD` | `"unknown"` |

These are injected via `-ldflags` only. Do not add runtime version-detection logic — the injected values are the canonical source. Do not strip `LDFLAGS` from the Makefile.

### Prerequisites

- Go 1.25+
- `nats` CLI (`brew install nats-io/nats-tools/nats`)
- Docker (for local NATS + JetStream KV)

### Common tasks

```bash
make build            # compile binary to bin/invite-api
make run              # build and run with env vars from shell
make test             # go test -v -race -coverprofile=coverage.out ./...
make fmt              # go fmt + gofmt -s (no goimports)
make lint             # golangci-lint run
make check            # fmt + lint + license-check + go vet
make license-check    # standalone license-header check for all .go files
make docker-build     # build Docker image (ghcr.io/linuxfoundation/lfx-v2-invite-service/invite-service)
make helm-install-local  # helm upgrade --install with values.local.yaml overlay
make helm-templates      # render chart templates to stdout (no cluster needed)
```

## Local dev loop

```bash
# Terminal 1: NATS with JetStream enabled
docker run --rm -p 4222:4222 nats:latest -js

# Terminal 2: create the KV bucket (one-time)
nats kv add invites --history=20 --storage=file

# Terminal 3: service
NATS_URL=nats://localhost:4222 \
  INVITE_JWT_SECRET=dev-secret-at-least-32-bytes-long \
  LFX_SELF_SERVE_BASE_URL=https://app.dev.lfx.dev \
  make run
```

### Send a test invite

```bash
nats req lfx.invite-service.send_invite \
  '{"recipient":{"email":"alice@example.com","name":"Alice"},"inviter":{"name":"Bob"},"resource":{"uid":"res-123","name":"My Project","type":"project"},"role":"Member"}'
```

### Look up an invite by UID

```bash
nats req lfx.invite-service.get_invite '{"uid":"<invite-uid>"}'
```

### Look up invites by email

```bash
nats req lfx.invite-service.get_invites_by_email '{"email":"alice@example.com"}'
```

## NATS Subjects

Authoritative subject constants and payload types live in `pkg/api/invite.go`.

| Constant | Value | Direction | Description |
|---|---|---|---|
| `api.SendInviteSubject` | `lfx.invite-service.send_invite` | Request/reply (consumed) | Resource services send `SendInviteRequest`; reply is `SendInviteResponse` (`uid`, `email`, `expires_at` or `error`) |
| `api.InviteAcceptedSubject` | `lfx.invite.accepted` | Event (consumed) | Published by the self-serve web app; invite service marks the KV record accepted. Queue group `invite-service-acceptance` — co-consumed with project-service |
| `api.GetInviteSubject` | `lfx.invite-service.get_invite` | Request/reply (consumed) | Callers send `GetInviteRequest{UID}`; reply is `GetInviteResponse` |
| `api.GetInvitesByEmailSubject` | `lfx.invite-service.get_invites_by_email` | Request/reply (consumed) | Callers send `GetInvitesByEmailRequest{Email}`; on success reply is a bare `[]Invite` JSON array; on failure reply is `GetInvitesByEmailResponse{Error}` |
| `api.InviteServiceAcceptedSubject` | `lfx.invite-service.invite_accepted` | Published (outbound) | Published after KV record is marked accepted; carries `InviteServiceAcceptedEvent` (full `Invite`) for downstream consumers. Best-effort — publish failure is logged but does not block the acceptance flow |
| _(email service)_ | `lfx.email-service.send_email` | Request/reply (outbound) | Forward pre-rendered email to the email service for delivery |
| `api.InviteCreatedSubject` | `lfx.invite-service.invite.created` | Published (future) | Invite issued |
| `api.InviteRevokedSubject` | `lfx.invite-service.invite.revoked` | Published (future) | Invite revoked |

> **Do not use `pkg/constants`.** `pkg/constants/nats.go` defines a stale `InviteAcceptedSubject = "lfx.invite-service.invite.accepted"` that conflicts with the live subject `"lfx.invite.accepted"` in `pkg/api`. The entire `pkg/constants` package is aspirational legacy and must not be extended or used in new code. Always use `pkg/api` constants as the source of truth.

## NATS KV Storage

The service owns the `invites` NATS JetStream KeyValue bucket:

- **Primary key**: `<inviteUID>` → JSON `InviteRecord`
- **Email index**: `index/email/<base64url(normalizedEmail)>/<inviteUID>` → inviteUID
- Records are kept indefinitely (no TTL) as a permanent audit trail.
- Bucket is provisioned by the Helm chart via the nack `KeyValue` CRD (`charts/lfx-v2-invite-service/templates/nats-kv-buckets.yaml`).
- The email key segment uses `base64.RawURLEncoding` (no padding) to keep raw email characters (`@`, `+`) out of the key. Both write and read paths use `encodeEmailForKey` — do not change the encoding without migrating existing keys.
- **`GetByEmail` scans all bucket keys client-side (O(all-keys)).** The implementation calls `kv.ListKeys` and prefix-filters in memory. Acceptable at current scale; revisit if the bucket grows large.

### Local development (no Kubernetes)

```bash
docker run -d -p 4222:4222 nats:latest -js
nats kv add invites --history=20 --storage=file
```

Set `INVITES_KV_BUCKET=invites` (or leave unset — defaults to `invites`).

## Environment Variables

All reads are centralized in `cmd/invite-api/service/config.go` → `AppConfigFromEnv()`. No other layer calls `os.Getenv`.

| Variable | Default | Notes |
|---|---|---|
| `NATS_URL` | `nats://lfx-platform-nats.lfx.svc.cluster.local:4222` | Use `nats://localhost:4222` for local dev |
| `INVITE_JWT_SECRET` | _(required)_ | HS256 signing key; must be ≥ 32 bytes; fatal at startup if unset or too short |
| `LFX_SELF_SERVE_BASE_URL` | _(env-aware; see below)_ | Base URL for the invite link (`{base}/invite?token=…`). Overrides the `LFX_ENVIRONMENT` default when set |
| `LFX_ENVIRONMENT` | `""` (dev) | `prod` → `https://app.lfx.dev`; `staging`/`stg` → `https://app.staging.lfx.dev`; otherwise → `https://app.dev.lfx.dev` |
| `DEFAULT_INVITE_LINK_RETURN_URL` | _(value of `LFX_SELF_SERVE_BASE_URL`)_ | Fallback destination URL embedded in the invite JWT when callers omit `return_url` |
| `ALLOWED_RETURN_URL_HOSTS` | `*.lfx.dev,*.linuxfoundation.org` | Comma-separated host patterns (wildcard supported) that caller-supplied `return_url` values must match |
| `INVITES_KV_BUCKET` | `invites` | Name of the NATS JetStream KV bucket; bucket must already exist |
| `LOG_LEVEL` | `""` (info) | `debug`, `info`, `warn`, `error` |
| `OTEL_TRACES_EXPORTER` | `otlp` | OTel span exporter; `none` disables tracing |
| `OTEL_METRICS_EXPORTER` | `otlp` | OTel metrics exporter; `none` disables metrics |
| `OTEL_LOGS_EXPORTER` | `otlp` | OTel log exporter; `none` disables OTel log bridge |
| `OTEL_SERVICE_NAME` | `lfx-v2-invite-service` | Service name in trace/metric metadata |
| `OTEL_SERVICE_VERSION` | _(injected from `main.Version` at startup)_ | Auto-set from build version if unset in the environment |
| `OTEL_TRACES_SAMPLER` | `parentbased_traceidratio` | Trace sampler; supports `always_on`, `always_off`, `traceidratio`, `parentbased_always_on`, `parentbased_always_off`, `parentbased_traceidratio` |
| `OTEL_TRACES_SAMPLER_ARG` | `1.0` | Sampling ratio for `traceidratio` / `parentbased_traceidratio` samplers (0.0–1.0) |

## Conventions

### Config injection

All `os.Getenv` calls belong in `cmd/invite-api/service/config.go` → `AppConfigFromEnv()`. Services receive a typed config struct (e.g., `NotificationConfig`), never call `os.Getenv` themselves.

### Adding a new NATS subject

1. Add the subject constant and any new payload types to `pkg/api/invite.go`.
2. Add the handler method to the relevant service in `internal/service/`.
3. Add a queue-subscribe block in `cmd/invite-api/service/subscriptions.go` and append the stop func. OTel trace context extraction is handled automatically by `Client.QueueSubscribe` — handlers just use the `ctx` they receive.
4. Wire any new infrastructure (e.g. a new KV binding) in `cmd/invite-api/service/implementations.go`.

For a **JetStream durable consumer** (ACK/NAK semantics), use `Client.ConsumeWithJetStream` instead of `QueueSubscribe`. Messages are ACKed on handler success and NAKed on handler error; configure `ConsumerConfig.MaxDeliver` and `AckWait` to control redelivery.

### Error handling

- Infrastructure errors → unexported `newServiceUnavailable` / `newUnexpected` in `internal/infrastructure/nats/errors.go`.
- Service-layer sentinels: `ErrInvalidRequest`, `ErrEmailDispatchFailed` (in `notification.go`); `ErrInviteNotFound`, `ErrAlreadyAccepted` (in `port/invite_store.go`).
- Return errors up; log at the point where you have the most context.
- Malformed NATS payloads: reply with error code and discard — they will never parse successfully on retry.
- Callers receive opaque error codes only (e.g. `"invalid_request"`, `"internal_error"`); full details are logged server-side.

### Logging

- Use `slog.DebugContext` for success paths, `slog.WarnContext` for recoverable issues, `slog.ErrorContext` for unexpected failures.
- Always pass `ctx` as the first argument so OTel trace correlation works.
- Redact email addresses in all log fields using the local `redactEmail` helper (defined in both `notification.go` and `email_sender.go`): `"alice@example.com"` → `"a***@example.com"`.
- Log send outcomes via `auditNotification` (structured `notification_audit` INFO line in `NotificationService`).

### License headers

Every `.go` file must start with:

```go
// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT
```

## Testing Patterns

- **Table-driven tests** in `_test.go` files co-located with the source.
- **All tests run with `-race`** (`go test -v -race ./...` via `make test`). New tests must be safe under the race detector.
- **Port mocks** live in `internal/domain/port/mocks/`:
  - `mocks.EmailSender` — satisfies `port.EmailSender`
  - `mocks.InviteStore` — satisfies `port.InviteStore`; inject behavior via `*Func` fields (e.g. `CreateFunc`, `GetByUIDFunc`, `MarkAcceptedFunc`, `DeleteFunc`); inspect calls via `*Calls` slices (e.g. `CreateCalls`, `DeleteCalls`, `MarkAcceptedCalls`)
  - `mocks.EventPublisher` — satisfies `port.EventPublisher`
- **`noopLinkGenerator`** — test double for `service.LinkGenerator` that returns a fixed invite link without JWT signing. Used in `notification_test.go` as the canonical test pattern; copy this approach for new tests involving `NotificationService`.
- **`captureLogs`** — redirects `slog.Default()` to a buffer for the test duration; use to assert on structured log output.
- Do not embed a real NATS server in unit tests. Use the port mocks instead.

## Helm Chart

`charts/lfx-v2-invite-service/` ships with the Go code in the same PR.

- `nats-kv-buckets.yaml` — provisions the `invites` KV bucket via the nack `KeyValue` CRD.
- `externalsecret.yaml` + `secretstore.yaml` — External Secrets Operator integration for `INVITE_JWT_SECRET` from AWS Secrets Manager.
- The `INVITE_JWT_SECRET` is never baked into the chart; it is injected at runtime from the Kubernetes Secret created by the External Secrets Operator.

## Related Services

| Service | Relationship |
|---|---|
| `lfx-v2-email-service` | Handles SMTP delivery; this service forwards pre-rendered email bodies to it via `lfx.email-service.send_email` |
| `lfx-v2-project-service` | Resource service that publishes `send_invite` requests; also co-subscribes to `lfx.invite.accepted` |
| `lfx-v2-committee-service` | Resource service that publishes `send_invite` requests |

## PR Title Convention

```
<type>(<scope>): <summary> [<ticket>]
```

Types: `feat` | `fix` | `refactor` | `docs` | `chore`. Scope is optional but recommended. Ticket reference is required — include `[LFXV2-XXXX]` when a ticket exists.

Examples:
```
feat(acceptance): publish enriched invite_accepted event on accept [LFXV2-1234]
fix(repository): handle stale email index entries on get_by_email [LFXV2-2345]
refactor(notification): extract return_url validation into service layer [LFXV2-3012]
```
