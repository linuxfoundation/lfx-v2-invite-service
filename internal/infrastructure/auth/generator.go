// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const tokenTTL = 7 * 24 * time.Hour

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

// Generate creates a signed JWT invite link for the given recipient and destination.
// The token carries: email, jti (UUID), return_url, resource_uid, role, iat, exp (7 days).
// The returned URL is: {inviteLinkBaseURL}/invite?token={signedJWT}
func (g *LinkGenerator) Generate(recipientEmail, returnURL, resourceUID, role string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"email":        recipientEmail,
		"jti":          uuid.NewString(),
		"return_url":   returnURL,
		"resource_uid": resourceUID,
		"role":         role,
		"iat":          now.Unix(),
		"exp":          now.Add(tokenTTL).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(g.secret)
	if err != nil {
		return "", fmt.Errorf("sign invite token: %w", err)
	}

	return g.inviteLinkBaseURL + "/invite?token=" + signed, nil
}
