# LFX V2 Invite Service

Handles invite email delivery for the LFX platform. When a non-LFID user is added to a resource, a `SendInviteRequest` is published via NATS; the invite service renders the email template, generates a signed JWT return link, and forwards to the email service for SMTP delivery.

## Overview

**Current scope** — pure NATS subscriber. Accepts `send_invite` requests, renders invite emails with embedded signed return URLs, and forwards to `lfx-v2-email-service` for delivery.

**Planned scope** — LFID invite token issuance, `/invite?token=<jwt>` HTTP acceptance endpoint (served by the self-serve web app), and acceptance broadcast (`lfx.invite.accepted`) consumed by resource services to grant access.

## Usage

### Send an invite

**Subject:** `lfx.invite-service.send_invite`  
**Queue group:** `invite-service-workers`

**Request payload fields:**

| Field | Type | Required | Description |
| ----- | ---- | :------: | ----------- |
| `recipient_email` | string | yes | Email address of the person being invited |
| `recipient_name` | string | | Display name of the recipient |
| `inviter_name` | string | | Display name of the person triggering the invite |
| `resource_uid` | string | yes | UID of the resource the recipient is being invited to |
| `resource_name` | string | | Human-readable resource name used in the email body |
| `role` | string | yes | `"Manage"` (writers/coordinators), `"View"` (auditors), or `"Member"` (plain membership) |
| `return_url` | string | | Override return URL after invite acceptance. Must be HTTPS and match `ALLOWED_RETURN_URL_HOSTS`. Defaults to `DEFAULT_INVITE_LINK_RETURN_URL`. |
| `resource_type` | string | | Kind of resource (e.g. `"project"`, `"group"`). Defaults to `"resource"`. |
| `org_name` | string | | Foundation or project name for the email signature. Defaults to `"LFX"`. |
| `expiration_days` | int | | Days the invite link is valid. Defaults to 30, max 90. |

```json
{
  "recipient_email": "user@example.com",
  "recipient_name": "Jane Doe",
  "inviter_name": "John Smith",
  "resource_uid": "resource-123",
  "resource_name": "My Project",
  "role": "Manage",
  "resource_type": "project"
}
```

**Success response:**
```json
{ "invite": { "uid": "abc123", "email": "user@example.com", "expires_at": "2026-06-20T00:00:00Z" } }
```

**Error response:**
```json
{ "error": "<reason>" }
```

| `error` value | Cause |
| ------------- | ----- |
| `malformed_request` | Payload could not be JSON-decoded |
| `invalid_request` | Missing required field, invalid email, unsupported role, or disallowed return URL |
| `email_dispatch_failed` | Email service returned an error or was unreachable |
| `internal_error` | Unexpected server-side error |

**Example (NATS CLI):**
```bash
nats req lfx.invite-service.send_invite \
  '{"recipient_email":"user@example.com","recipient_name":"Jane Doe","resource_uid":"resource-123","resource_name":"My Project","role":"Manage","resource_type":"project"}'
```

### Use with Go

```bash
go get github.com/linuxfoundation/lfx-v2-invite-service/pkg/api
```

```go
package main

import (
    "encoding/json"
    "fmt"
    "time"

    "github.com/nats-io/nats.go"
    inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

func main() {
    nc, err := nats.Connect(nats.DefaultURL)
    if err != nil {
        panic(err)
    }
    defer nc.Close()

    req := inviteapi.SendInviteRequest{
        RecipientEmail: "user@example.com",
        RecipientName:  "Jane Doe",
        InviterName:    "John Smith",
        ResourceUID:    "resource-123",
        ResourceName:   "My Project",
        Role:           string(inviteapi.InviteRoleManage),
        ResourceType:   "project",
    }
    data, err := json.Marshal(req)
    if err != nil {
        panic(err)
    }

    msg, err := nc.Request(inviteapi.SendInviteSubject, data, 10*time.Second)
    if err != nil {
        panic(err)
    }

    var resp inviteapi.SendInviteResponse
    if err := json.Unmarshal(msg.Data, &resp); err != nil {
        panic(err)
    }
    if resp.Error != "" {
        fmt.Println("invite failed:", resp.Error)
        return
    }
    fmt.Println("invite sent, uid:", resp.Invite.UID)
}
```

## Quick Start

### Option 1 — Run directly

```bash
# Prerequisites: Go 1.25+, a local NATS server
make build

NATS_URL=nats://localhost:4222 \
INVITE_JWT_SECRET="change-me-local-dev-secret-32b!!" \
./bin/invite-api
```

### Option 2 — Deploy to a local cluster with Helm

```bash
cp charts/lfx-v2-invite-service/values.local.yaml.example charts/lfx-v2-invite-service/values.local.yaml
# Edit values.local.yaml to set the NATS URL and JWT secret
make helm-install-local
```

## Environment Variables

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `NATS_URL` | `nats://lfx-platform-nats.lfx.svc.cluster.local:4222` | NATS server URL |
| `INVITE_JWT_SECRET` | *required* | HMAC-SHA256 key for signing invite return-URL JWTs. **Minimum 32 bytes.** |
| `LFX_ENVIRONMENT` | unset → dev | Controls `LFX_SELF_SERVE_BASE_URL` default: `prod` → `https://app.lfx.dev`, `staging`/`stg` → `https://app.staging.lfx.dev`, else `https://app.dev.lfx.dev` |
| `LFX_SELF_SERVE_BASE_URL` | derived from `LFX_ENVIRONMENT` | Explicit override for the self-serve base URL |
| `DEFAULT_INVITE_LINK_RETURN_URL` | falls back to `LFX_SELF_SERVE_BASE_URL` | Default return URL embedded in invite JWTs when the caller omits `return_url` |
| `ALLOWED_RETURN_URL_HOSTS` | `*.lfx.dev,*.linuxfoundation.org` | Comma-separated list of allowed `return_url` host patterns. Only the `*.` subdomain wildcard is supported (e.g. `*.lfx.dev` matches `app.lfx.dev`). Must be HTTPS. |
| `LOG_LEVEL` | `""` (logger default applies) | Log level: `debug`, `info`, `warn` |

Standard `OTEL_*` SDK env vars (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`, etc.) are read by the OTel SDK in `internal/infrastructure/observability/otel.go`.

## File Structure

```
├── charts/
│   └── lfx-v2-invite-service/           # Helm chart for Kubernetes deployment
├── cmd/
│   └── invite-api/
│       ├── main.go                      # OTel bootstrap, signal handling, graceful shutdown
│       └── service/
│           ├── config.go                # All env-var reads — no os.Getenv outside this file
│           ├── implementations.go       # Wires infrastructure into service structs
│           └── subscriptions.go         # NATS subscriber registration
├── internal/
│   ├── domain/
│   │   ├── model/                       # Pure data: SendInviteRequest, Role, DeliveryState
│   │   └── port/                        # Interfaces: EmailSender
│   ├── infrastructure/
│   │   ├── auth/                        # JWT link generator (HS256)
│   │   ├── nats/                        # NATS client, NATSEmailSender, error types
│   │   ├── observability/               # slog setup and OTel SDK bootstrap
│   │   └── smtp/                        # Template rendering + embedded templates
│   └── service/                         # Business logic: NotificationService
└── pkg/
    └── api/                             # Public inter-service contract: subjects, request/response types
```

## Architecture Notes

- **NATS-only today** — no HTTP server. An HTTP server will be added when the `/invite/:uuid` acceptance endpoint is needed.
- **Template ownership** — the invite service owns and renders the email template (HTML + plaintext + subject line). The email service (`lfx-v2-email-service`) handles SMTP delivery only. Callers publish a structured `SendInviteRequest` — no pre-rendered HTML required.
- **Signed return URLs** — `INVITE_JWT_SECRET` signs the return URL embedded in the invite email. JWT signing failure fails the entire request rather than falling back to an unsigned URL; emailing an LFX-branded link to an unsigned, unrevokable destination would be a security regression.
- **Planned: invite acceptance** — once the LFID flow is built, the self-serve web app will serve `/invite?token=<jwt>` and publish `lfx.invite.accepted` after a user completes the acceptance flow. Resource services subscribe to this subject to grant access.

## Development

### Prerequisites

- Go 1.25+
- Access to a NATS server (local or cluster)

### Make targets

| Target | Description |
| ------ | ----------- |
| `make build` | Compile binary → `bin/invite-api` |
| `make test` | Run tests with race detector |
| `make check` | fmt + lint + license-check + go vet |
| `make lint` | golangci-lint only |
| `make helm-templates` | Render the Helm chart with default values (dry-run) |
| `make helm-install-local` | Install/upgrade the chart into the current kube context |

## Adding a New NATS Subscription

1. Add the handler method to the appropriate service in `internal/service/`.
2. Add a new `QueueSubscribe` call in `StartSubscriptions` in `cmd/invite-api/service/subscriptions.go`, following the pattern of the existing `send_invite` subscriber.
3. Wire any new infrastructure (e.g. JetStream consumers) via `cmd/invite-api/service/implementations.go` and shut them down in `Shutdown()`.

## Releases

1. Merge the PR, then create a GitHub release with a `v{version}` tag.
2. CI builds and publishes the container image and Helm chart automatically.

## Related Services

| Service | Relationship |
| ------- | ------------ |
| `lfx-v2-email-service` | Handles SMTP delivery; this service forwards pre-rendered emails to it |

## License

Copyright The Linux Foundation and each contributor to LFX.

Source code is licensed under the MIT License. See `LICENSE`.
Documentation is licensed under CC-BY-4.0. See `LICENSE-docs`.
