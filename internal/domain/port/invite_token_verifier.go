// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"
	"errors"
)

// ErrInvalidInviteToken is returned by InviteTokenVerifier.Verify when the
// presented token is not a valid, unexpired invite token issued by this service.
var ErrInvalidInviteToken = errors.New("invalid_invite_token")

// InviteTokenClaims carries the verified claims of an invite token that the
// acceptance flow binds against the stored invite record.
type InviteTokenClaims struct {
	// InviteUID is the invite UUID (the token's jti and invite_uid claims).
	InviteUID string
	// Email is the canonical recipient email the token was issued to.
	Email string
}

// InviteTokenVerifier verifies signed invite tokens minted by LinkGenerator.
// Implementations must check the signature, signing algorithm, issuer, audience
// and expiry, and return ErrInvalidInviteToken (wrapped) on any failure.
type InviteTokenVerifier interface {
	Verify(ctx context.Context, token string) (InviteTokenClaims, error)
}
