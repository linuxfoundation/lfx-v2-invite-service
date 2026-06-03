# LFX V2 Invite Service

Invite issuance and tracking service for the LFX Self-Service platform. Receives
invite requests from resource services over NATS request/reply, renders the invite
email, delivers it via the email service, and persists every invite record in NATS KV
so its status can be queried and updated when the recipient accepts.

## Usage

### Send an invite

**Subject:** `lfx.invite-service.send_invite`  
**Queue group:** `invite-service-workers`

Sent by resource services (project-service, committee-service, etc.) when a user
without an LFID is added to a resource. The invite service generates a signed JWT
invite link, renders the email template, forwards delivery to the email service, and
persists a `pending` invite record in KV.

**Request payload fields:**

| Field | Type | Required | Description |
|---|---|---|---|
| `recipient` | object | yes* | Structured recipient identity — preferred over the deprecated scalar fields below |
| `recipient.email` | string | yes | Recipient email address |
| `recipient.name` | string | no | Recipient display name, used in the email greeting |
| `recipient.username` | string | no | Recipient LFID username, if already known |
| `recipient.avatar` | string | no | Recipient avatar URL |
| `inviter` | object | no | Structured identity of the person who triggered the invite |
| `inviter.name` | string | no | Inviter display name, used in the email body |
| `inviter.username` | string | no | Inviter LFID username |
| `inviter.email` | string | no | Inviter email address |
| `inviter.avatar` | string | no | Inviter avatar URL |
| `resource` | object | yes* | Structured resource the recipient is being invited to |
| `resource.uid` | string | yes | Resource UID |
| `resource.name` | string | no | Resource display name, used in the email body |
| `resource.type` | string | no | Resource kind — e.g. `"project"`, `"committee"`, `"meeting"`. Defaults to `"resource"` when empty |
| `role` | string | yes | Access level: `"Manage"`, `"View"`, or `"Member"` |
| `return_url` | string | no | URL the invite link redirects to after acceptance. Must use `https` and match the `ALLOWED_RETURN_URL_HOSTS` allowlist. Defaults to `DEFAULT_INVITE_LINK_RETURN_URL` when omitted |
| `org_name` | string | no | Foundation or project name used in the email signature ("The X Team"). Defaults to `"LFX"` when empty |
| `expiration_days` | int | no | Number of days before the invite JWT expires. Defaults to 30, capped at 90 |

> **Deprecated scalar fields** — `recipient_email`, `recipient_name`, `inviter_name`,
> `resource_uid`, `resource_name`, `resource_type` are accepted for backward
> compatibility. The structured `recipient`, `inviter`, and `resource` objects take
> precedence when both are provided.

```json
{
  "recipient": {
    "name": "Alice Smith",
    "email": "alice@example.com"
  },
  "inviter": {
    "name": "Bob Jones",
    "username": "bobjones",
    "email": "bob@linuxfoundation.org"
  },
  "resource": {
    "uid": "proj-abc123",
    "name": "My Project",
    "type": "project"
  },
  "role": "Member",
  "org_name": "The Linux Foundation",
  "expiration_days": 30
}
```

**Success response:**
```json
{
  "uid": "550e8400-e29b-41d4-a716-446655440000",
  "email": "alice@example.com",
  "expires_at": "2025-02-14T10:30:00Z"
}
```

Store `uid` to look up the invite record later or to correlate with the
`lfx.invite.accepted` event.

**Error response:**
```json
{ "error": "<reason>" }
```

| `error` value | Cause |
|---|---|
| `malformed_request` | Request body is not valid JSON |
| `invalid_request` | Missing required field, unrecognised role, or `return_url` failed host validation |
| `email_dispatch_failed` | Invite link was generated but the email service could not deliver it |
| `internal_error` | Unexpected server-side failure |

**Examples (NATS CLI):**
```bash
# Structured objects (preferred)
nats req lfx.invite-service.send_invite \
  '{"recipient":{"name":"Alice","email":"alice@example.com"},"inviter":{"name":"Bob","username":"bob","email":"bob@lf.org"},"resource":{"uid":"proj-1","name":"My Project","type":"project"},"role":"Member"}'

# Deprecated scalars (still accepted for backward-compat)
nats req lfx.invite-service.send_invite \
  '{"recipient_email":"alice@example.com","recipient_name":"Alice","resource_uid":"proj-1","resource_name":"My Project","role":"Member"}'
```

---

### Get an invite by UID

**Subject:** `lfx.invite-service.get_invite`  
**Queue group:** `invite-service-get-invite`

Returns the full invite record for a given invite UID.

**Request:**
```json
{ "uid": "550e8400-e29b-41d4-a716-446655440000" }
```

**Success response:**
```json
{
  "uid": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "recipient": {
    "name": "Alice Smith",
    "email": "alice@example.com"
  },
  "inviter": {
    "name": "Bob Jones",
    "username": "bobjones",
    "email": "bob@linuxfoundation.org"
  },
  "resource": {
    "uid": "proj-abc123",
    "name": "My Project",
    "type": "project"
  },
  "role": "Member",
  "org_name": "The Linux Foundation",
  "return_url": "https://app.lfx.dev",
  "expiration_days": 30,
  "created_at": "2025-01-15T10:30:00Z",
  "expires_at": "2025-02-14T10:30:00Z"
}
```

Once the recipient accepts, `status` changes to `"accepted"` and the response
includes `accepted_at` and `accepted_by`:

```json
{
  "uid": "550e8400-e29b-41d4-a716-446655440000",
  "status": "accepted",
  "recipient": { "name": "Alice Smith", "email": "alice@example.com" },
  "inviter": { "name": "Bob Jones", "username": "bobjones" },
  "resource": { "uid": "proj-abc123", "name": "My Project", "type": "project" },
  "role": "Member",
  "created_at": "2025-01-15T10:30:00Z",
  "expires_at": "2025-02-14T10:30:00Z",
  "accepted_at": "2025-01-20T14:05:00Z",
  "accepted_by": "alice-lfid"
}
```

**Error response:**
```json
{ "error": "<reason>" }
```

| `error` value | Cause |
|---|---|
| `malformed_request` | Request body is not valid JSON |
| `invalid_request` | `uid` field is missing or empty |
| `not_found` | No invite record exists for the given UID |
| `internal_error` | Unexpected server-side failure |

**Example (NATS CLI):**
```bash
nats req lfx.invite-service.get_invite \
  '{"uid":"550e8400-e29b-41d4-a716-446655440000"}'
```

---

### Get invites by email

**Subject:** `lfx.invite-service.get_invites_by_email`  
**Queue group:** `invite-service-get-by-email`

Returns all invite records for a given email address across all resources and
statuses. Useful for checking whether a user already has a pending invite before
sending another.

**Request:**
```json
{ "email": "alice@example.com" }
```

**Success response** — a bare JSON array of invite records (empty array when none exist):
```json
[
  {
    "uid": "550e8400-e29b-41d4-a716-446655440000",
    "status": "pending",
    "recipient": { "name": "Alice Smith", "email": "alice@example.com" },
    "inviter": { "name": "Bob Jones", "username": "bobjones" },
    "resource": { "uid": "proj-abc123", "name": "My Project", "type": "project" },
    "role": "Member",
    "created_at": "2025-01-15T10:30:00Z",
    "expires_at": "2025-02-14T10:30:00Z"
  },
  {
    "uid": "661f9511-f30c-52e5-b827-557766551111",
    "status": "accepted",
    "recipient": { "name": "Alice Smith", "email": "alice@example.com" },
    "resource": { "uid": "comm-xyz789", "name": "My Committee", "type": "committee" },
    "role": "Member",
    "created_at": "2025-01-10T09:00:00Z",
    "expires_at": "2025-02-09T09:00:00Z",
    "accepted_at": "2025-01-11T16:30:00Z",
    "accepted_by": "alice-lfid"
  }
]
```

**Error response:**
```json
{ "error": "<reason>" }
```

| `error` value | Cause |
|---|---|
| `malformed_request` | Request body is not valid JSON |
| `invalid_request` | `email` field is missing or empty |
| `internal_error` | Unexpected server-side failure |

**Example (NATS CLI):**
```bash
nats req lfx.invite-service.get_invites_by_email \
  '{"email":"alice@example.com"}'
```

---

### Invite accepted event (consumed)

**Subject:** `lfx.invite.accepted`  
**Queue group:** `invite-service-acceptance`

Published by the LFX self-serve web app once a user completes the acceptance
flow (JWT validation + login). The invite service subscribes to this event
alongside other services (e.g. project-service) — each uses its own queue group
and receives an independent copy.

On receipt the invite service marks the KV record `status: accepted`, sets
`accepted_at` to now, and records the LFID username in `accepted_by`.
If the `invite_uid` is not found in the invite-service KV store (i.e. the invite
belongs to a different service's flow), the event is silently ignored.

**Event payload:**
```json
{
  "invite_uid": "550e8400-e29b-41d4-a716-446655440000",
  "username": "alice-lfid"
}
```

This subject is consumed, not exposed for direct use — it is published by
the self-serve web app.

---

### Use with Go

The `pkg/api` package exports subject constants and request/response types for
all NATS interactions consumed and published by the invite service.

```bash
go get github.com/linuxfoundation/lfx-v2-invite-service/pkg/api
```

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

func main() {
	nc, _ := nats.Connect(nats.DefaultURL)
	defer nc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send an invite.
	req := inviteapi.SendInviteRequest{
		Recipient: &inviteapi.Recipient{Name: "Alice Smith", Email: "alice@example.com"},
		Inviter:   &inviteapi.Inviter{Name: "Bob Jones", Username: "bobjones"},
		Resource:  &inviteapi.Resource{UID: "proj-abc123", Name: "My Project", Type: "project"},
		Role:      string(inviteapi.InviteRoleMember),
		OrgName:   "The Linux Foundation",
	}
	data, _ := json.Marshal(req)

	reply, err := nc.RequestWithContext(ctx, inviteapi.SendInviteSubject, data)
	if err != nil {
		panic(err)
	}

	var sendResp inviteapi.SendInviteResponse
	_ = json.Unmarshal(reply.Data, &sendResp)
	if sendResp.Error != "" {
		fmt.Println("send failed:", sendResp.Error)
		return
	}
	fmt.Println("invite sent, uid:", sendResp.UID)

	// Look up the invite record by UID.
	getReq, _ := json.Marshal(inviteapi.GetInviteRequest{UID: sendResp.UID})
	getReply, _ := nc.RequestWithContext(ctx, inviteapi.GetInviteSubject, getReq)

	var getResp inviteapi.GetInviteResponse
	_ = json.Unmarshal(getReply.Data, &getResp)
	if getResp.Error != "" {
		fmt.Println("get failed:", getResp.Error)
		return
	}
	fmt.Printf("status: %s, expires: %s\n", getResp.Invite.Status, getResp.Invite.ExpiresAt)

	// List all invites for an email address.
	listReq, _ := json.Marshal(inviteapi.GetInvitesByEmailRequest{Email: "alice@example.com"})
	listReply, _ := nc.RequestWithContext(ctx, inviteapi.GetInvitesByEmailSubject, listReq)

	var invites []inviteapi.Invite
	_ = json.Unmarshal(listReply.Data, &invites)
	fmt.Printf("invites for alice: %d\n", len(invites))
}
```

---

## KV Storage

Invite records are stored in the `invites` NATS JetStream KV bucket:

- **Primary key**: `<inviteUID>` → JSON invite record
- **Email index**: `index/email/<normalizedEmail>/<inviteUID>` → invite UID (enables list-by-email)
- Records are kept indefinitely — no TTL — as a permanent audit trail.
- `return_url` stores the destination URL, never the signed JWT token.

The bucket is provisioned by the Helm chart via the nack `KeyValue` CRD
(`charts/lfx-v2-invite-service/templates/nats-kv-buckets.yaml`). For local
development without Kubernetes, create it with:

```bash
nats kv add invites --history=20 --storage=file
```

---

## File Structure

```
├── charts/                          # Helm charts for Kubernetes deployment
│   └── lfx-v2-invite-service/
│       └── templates/
│           └── nats-kv-buckets.yaml # Declares the invites KV bucket via nack CRD
├── cmd/
│   └── invite-api/                  # Application entry point
│       ├── main.go
│       └── service/
│           ├── config.go            # AppConfig read from env vars
│           ├── implementations.go   # Infrastructure wiring (NATS, KV, services)
│           └── subscriptions.go     # NATS subscriber registration (all 4 subjects)
├── internal/
│   ├── domain/
│   │   ├── model/                   # Domain types: InviteRecord, SendInviteRequest, roles
│   │   └── port/                    # Interfaces: EmailSender, InviteStore; mocks/
│   ├── infrastructure/
│   │   ├── nats/                    # NATS client, NATSEmailSender, NATSInviteRepository
│   │   ├── observability/           # slog setup and OTel SDK bootstrap
│   │   └── smtp/                    # Template rendering + embedded templates/
│   └── service/
│       ├── notification.go          # HandleSendInvite — email dispatch + KV persist
│       ├── acceptance.go            # HandleInviteAccepted — marks KV record accepted
│       └── invite_read.go           # GetInvite / GetInvitesByEmail
└── pkg/
    └── api/                         # Public inter-service contract: subjects, types
```

## Key Design Decisions

- **No HTTP API** — the service is a pure NATS subscriber. All operations use request/reply or fire-and-forget events.
- **Template ownership** — the invite service owns and renders the email template; the email service (`lfx.email-service.send_email`) handles SMTP delivery. Callers publish structured fields — no pre-rendered HTML required.
- **Fail-closed KV persist** — the invite record is written to KV *before* the email is sent. A KV write failure aborts the operation and no email is dispatched. If the email send fails after a successful KV write, a best-effort rollback delete is attempted and `email_dispatch_failed` is returned to the caller.
- **Own queue group for acceptance** — the service uses `invite-service-acceptance` as its queue group for `lfx.invite.accepted`, so it receives an independent copy alongside other subscribers (e.g. project-service).
- **Config injected via struct** — all env vars are read in `cmd/invite-api/service/config.go` and passed into service constructors; no `os.Getenv` calls in business logic.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `NATS_URL` | `nats://lfx-platform-nats.lfx.svc.cluster.local:4222` | NATS server URL |
| `INVITE_JWT_SECRET` | (required) | HMAC-SHA256 key (≥32 bytes) for signing invite JWTs |
| `INVITES_KV_BUCKET` | `invites` | Name of the NATS JetStream KV bucket for invite records |
| `DEFAULT_INVITE_LINK_RETURN_URL` | auto (falls back to `LFX_SELF_SERVE_BASE_URL`) | Fallback return URL when the caller omits `return_url` |
| `LFX_SELF_SERVE_BASE_URL` | auto (see `LFX_ENVIRONMENT`) | Base URL of the invite acceptance web app |
| `LFX_ENVIRONMENT` | (unset → dev) | Sets self-serve URL default: `prod`, `stg`, or dev |
| `ALLOWED_RETURN_URL_HOSTS` | `*.lfx.dev,*.linuxfoundation.org` | Comma-separated host patterns permitted for `return_url`. Wildcard prefix `*.` matches any subdomain |
| `LOG_LEVEL` | `debug` | Log level: `debug`, `info`, `warn`, `error` |
| `OTEL_SERVICE_NAME` | `lfx-v2-invite-service` | OpenTelemetry service name |

## Development

### Prerequisites

- Go 1.25+
- NATS server with JetStream enabled (Docker: `docker run -d -p 4222:4222 nats:latest -js`)

### Build

```bash
make build
# binary: bin/lfx-v2-invite-service/invite-service
```

### Create the Kubernetes secret

The Helm chart expects a secret named `lfx-v2-invite-service` with an
`invite-jwt-secret` key. Create it before deploying (or the pod will fail to
start with `secret "lfx-v2-invite-service" not found`):

```bash
kubectl create secret generic lfx-v2-invite-service \
  --from-literal=invite-jwt-secret=$(openssl rand -base64 48) \
  -n lfx
```

> In production the secret is managed by External Secrets Operator, which
> pulls the value from AWS Secrets Manager at
> `/cloudops/managed-secrets/cloud/invite-service/jwt`. For local Kubernetes
> (e.g. OrbStack) the manual `kubectl create secret` above is sufficient.

### Run locally

```bash
# Create the KV bucket first
nats kv add invites --history=20 --storage=file

export NATS_URL=nats://localhost:4222
export INVITE_JWT_SECRET=$(openssl rand -base64 48)
./bin/lfx-v2-invite-service/invite-service
```

### Test

```bash
make test
```

### Lint & format

```bash
make check
```

## Releases

1. Merge the PR, then create a GitHub release with a `v{version}` tag.
2. CI builds and publishes the container image and Helm chart automatically.

## License

Copyright The Linux Foundation and each contributor to LFX.

Source code is licensed under the MIT License. See `LICENSE`.  
Documentation is licensed under CC-BY-4.0. See `LICENSE-docs`.
