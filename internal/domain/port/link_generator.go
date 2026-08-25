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
	RecipientEmail string
	DestinationURL string
	ResourceUID    string
	ResourceType   string
	Role           string
	ExpirationDays int
	CustomClaims   map[string]string
}

// LinkGenerator generates a signed invite link for a given recipient and
// destination. It returns the full invite URL and the invite UUID (jti) so
// callers can store the UUID and correlate it with the KV record.
type LinkGenerator interface {
	Generate(ctx context.Context, p LinkPayload) (link, inviteUID string, expiresAt time.Time, err error)
}
