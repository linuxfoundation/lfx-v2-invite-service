// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
)

// tokenLeeway is the clock-skew tolerance applied to exp/nbf/iat checks.
const tokenLeeway = 60 * time.Second

// TokenVerifier verifies HS256 invite tokens minted by LinkGenerator.
type TokenVerifier struct {
	secret []byte
}

// NewTokenVerifier returns a TokenVerifier that checks tokens against secret,
// which must be the same key the LinkGenerator signs with.
func NewTokenVerifier(secret []byte) *TokenVerifier {
	return &TokenVerifier{secret: secret}
}

// Verify parses token and checks the HS256 signature (algorithm pinned), issuer,
// audience, exp (required), nbf and iat. It also requires non-empty jti and email
// claims and that jti equals the invite_uid claim. Any failure is returned wrapped
// in port.ErrInvalidInviteToken.
func (v *TokenVerifier) Verify(_ context.Context, token string) (port.InviteTokenClaims, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return v.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithAudience(tokenAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(tokenLeeway),
	)
	if err != nil {
		return port.InviteTokenClaims{}, fmt.Errorf("%w: %w", port.ErrInvalidInviteToken, err)
	}

	jti, _ := claims["jti"].(string)
	inviteUID, _ := claims["invite_uid"].(string)
	email, _ := claims["email"].(string)
	if jti == "" || jti != inviteUID {
		return port.InviteTokenClaims{}, fmt.Errorf("%w: missing or inconsistent jti/invite_uid claims", port.ErrInvalidInviteToken)
	}
	if email == "" {
		return port.InviteTokenClaims{}, fmt.Errorf("%w: missing email claim", port.ErrInvalidInviteToken)
	}

	return port.InviteTokenClaims{InviteUID: inviteUID, Email: email}, nil
}
