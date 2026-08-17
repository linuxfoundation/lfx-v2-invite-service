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

// LinkGenerator generates a signed invite link for a given recipient and
// destination. It returns the full invite URL and the invite UUID (jti) so
// callers can store the UUID and correlate it with the KV record.
type LinkGenerator interface {
	Generate(
		ctx context.Context,
		recipientEmail, destinationURL, resourceUID, resourceType, role string,
		expirationDays int,
		customClaims map[string]string,
	) (link, inviteUID string, expiresAt time.Time, err error)
}
