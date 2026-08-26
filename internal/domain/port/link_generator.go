// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"
	"errors"
	"time"
)

// ErrInvalidCustomClaims is returned by LinkGenerator.Generate when the
// caller-supplied customClaims map fails size or content validation. It is
// distinct from signing errors so the service layer can surface it as an
// "invalid_request" response code rather than "internal_error".
var ErrInvalidCustomClaims = errors.New("invalid custom claims")

// LinkPayload carries all inputs needed to generate a signed invite link.
// Using a typed struct keeps the Generate interface small and named, and
// mirrors the InviteEmailPayload pattern used at the EmailSender seam.
type LinkPayload struct {
	// RecipientEmail is the invitee's address; becomes the "email" JWT claim.
	RecipientEmail string
	// DestinationURL is where the user lands after accepting; becomes the
	// "return_url" JWT claim. Use the caller-supplied URL or the service
	// default; never the signed JWT link itself.
	DestinationURL string
	// ResourceUID is the opaque identifier of the resource being joined;
	// becomes the "resource_uid" JWT claim.
	ResourceUID string
	// ResourceType is the kind of resource (e.g. "group", "project");
	// becomes the "resource_type" JWT claim. An empty string omits the claim.
	ResourceType string
	// Role is the role being granted (e.g. "Member", "Manage").
	Role string
	// ExpirationDays is the token TTL in days. 0 uses the default (30 days);
	// values above the implementation maximum are clamped.
	ExpirationDays int
	// CustomClaims are additional string claims to embed in the JWT.
	// Keys that collide with reserved claims are skipped and a warning is logged.
	CustomClaims map[string]string
}

// LinkGenerator generates a signed invite link for a given recipient and
// destination. It returns the full invite URL and the invite UUID (jti) so
// callers can store the UUID and correlate it with the KV record.
type LinkGenerator interface {
	Generate(ctx context.Context, p LinkPayload) (link, inviteUID string, expiresAt time.Time, err error)
}
