// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	tokenTTL          = 30 * 24 * time.Hour
	maxExpirationDays = 90
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
	"iss": {}, "aud": {}, "iat": {}, "nbf": {}, "exp": {}, "jti": {},
	"invite_uid": {}, "email": {}, "return_url": {}, "resource_uid": {},
	"resource_type": {}, "role": {},
}

// Generate creates a signed JWT invite link for the given recipient and destination.
// The token is HS256-signed and carries: iss, aud, iat, nbf, exp, jti, email,
// invite_uid, return_url, resource_uid, resource_type, role, and any customClaims.
// resourceType is the kind of resource (e.g. "group", "project"); pass an empty string
// when the type is unknown — the claim is omitted from the token in that case.
// customClaims are additional string claims to embed; keys that collide with reserved
// claims are silently ignored to prevent claim hijacking.
// Returns the full invite URL and the invite UUID (jti) so callers can store the UUID.
// The returned URL is: {inviteLinkBaseURL}/invite?token={signedJWT}
//
// Verifier note: the self-serve web app MUST validate with
// jwt.WithValidMethods([]string{"HS256"}) to prevent algorithm-confusion attacks.
func (g *LinkGenerator) Generate(recipientEmail, returnURL, resourceUID, resourceType, role string, expirationDays int, customClaims map[string]string) (link, inviteUID string, expiresAt time.Time, err error) {
	now := time.Now()
	inviteUID = uuid.NewString()
	ttl := tokenTTL
	if expirationDays > 0 {
		if expirationDays > maxExpirationDays {
			slog.Warn("expirationDays exceeds maximum; clamping",
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
		"email":        recipientEmail,
		"return_url":   returnURL,
		"resource_uid": resourceUID,
		"role":         role,
	}
	if resourceType != "" {
		claims["resource_type"] = resourceType
	}
	for k, v := range customClaims {
		if _, reserved := reservedClaims[k]; reserved {
			slog.Warn("custom claim key is reserved and will be ignored", "key", k)
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
