// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
)

const (
	tokenTTL          = 30 * 24 * time.Hour
	maxExpirationDays = 90

	// Limits applied to caller-supplied CustomClaims to prevent token bloat.
	maxCustomClaims        = 16
	maxCustomClaimKeyLen   = 64
	maxCustomClaimValueLen = 1024
)

// LinkGenerator creates HMAC-SHA256 signed JWT invite links.
type LinkGenerator struct {
	secret            []byte
	inviteLinkBaseURL string
}

// NewLinkGenerator returns a LinkGenerator that signs tokens with secret and
// builds links against inviteLinkBaseURL (e.g. "https://lfx.linuxfoundation.org").
func NewLinkGenerator(secret []byte, inviteLinkBaseURL string) *LinkGenerator {
	return &LinkGenerator{secret: secret, inviteLinkBaseURL: inviteLinkBaseURL}
}

// reservedClaims is the set of JWT claim keys that callers may not override via
// CustomClaims. These are either standard JWT claims or application claims that
// the invite service controls.
var reservedClaims = map[string]struct{}{
	"iss": {}, "aud": {}, "iat": {}, "nbf": {}, "exp": {}, "jti": {}, "sub": {},
	"invite_uid": {}, "email": {}, "return_url": {}, "resource_uid": {},
	"resource_type": {}, "role": {},
}

// Generate creates a signed JWT invite link from the fields in p.
// The token is HS256-signed and carries: iss, aud, iat, nbf, exp, jti, email,
// invite_uid, return_url, resource_uid, resource_type, role, and any p.CustomClaims.
// p.ResourceType is the kind of resource (e.g. "group", "project"); an empty string
// omits the resource_type claim from the token.
// p.CustomClaims are additional string claims to embed; keys that collide with reserved
// claims are ignored (with a warning log) to prevent claim hijacking. Claims that
// exceed the count (maxCustomClaims), key-length (maxCustomClaimKeyLen), or
// value-length (maxCustomClaimValueLen) limits cause Generate to return an error.
// Returns the full invite URL and the invite UUID (jti) so callers can store the UUID.
// The returned URL is: {inviteLinkBaseURL}/invite?token={signedJWT}
//
// Verifier note: the self-serve web app MUST validate with
// jwt.WithValidMethods([]string{"HS256"}) to prevent algorithm-confusion attacks.
func (g *LinkGenerator) Generate(ctx context.Context, p port.LinkPayload) (link, inviteUID string, expiresAt time.Time, err error) {
	now := time.Now()
	inviteUID = uuid.NewString()
	ttl := tokenTTL
	expirationDays := p.ExpirationDays
	if expirationDays > 0 {
		if expirationDays > maxExpirationDays {
			slog.WarnContext(ctx, "expirationDays exceeds maximum; clamping",
				"requested", expirationDays,
				"max", maxExpirationDays,
			)
			expirationDays = maxExpirationDays
		}
		ttl = time.Duration(expirationDays) * 24 * time.Hour
	}
	expiresAt = now.Add(ttl)
	claims := jwt.MapClaims{
		// Standard claims (ASVS V3.5.3 — replay defense, algorithm pinning).
		"iss": "lfx-v2-invite-service",
		"aud": jwt.ClaimStrings{"lfx-self-serve"},
		"iat": now.Unix(),
		"nbf": now.Unix(), // clock-skew: verifiers may allow up to 60 s of tolerance
		"exp": expiresAt.Unix(),
		"jti": inviteUID,
		// Application claims.
		"invite_uid":   inviteUID,
		"email":        p.RecipientEmail,
		"return_url":   p.DestinationURL,
		"resource_uid": p.ResourceUID,
		"role":         p.Role,
	}
	if p.ResourceType != "" {
		claims["resource_type"] = p.ResourceType
	}
	if len(p.CustomClaims) > maxCustomClaims {
		return "", "", time.Time{}, fmt.Errorf("%w: too many entries (%d > %d)", port.ErrInvalidCustomClaims, len(p.CustomClaims), maxCustomClaims)
	}
	for k, v := range p.CustomClaims {
		if len(k) > maxCustomClaimKeyLen {
			return "", "", time.Time{}, fmt.Errorf("%w: key %q exceeds max length (%d > %d)", port.ErrInvalidCustomClaims, k, len(k), maxCustomClaimKeyLen)
		}
		if len(v) > maxCustomClaimValueLen {
			return "", "", time.Time{}, fmt.Errorf("%w: value for key %q exceeds max length (%d > %d)", port.ErrInvalidCustomClaims, k, len(v), maxCustomClaimValueLen)
		}
		if _, reserved := reservedClaims[k]; reserved {
			slog.WarnContext(ctx, "custom claim key is reserved and will be ignored", "key", k)
			continue
		}
		claims[k] = v
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(g.secret)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("sign invite token: %w", err)
	}

	return g.inviteLinkBaseURL + "/invite?token=" + signed, inviteUID, expiresAt, nil
}
