// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/infrastructure/auth"
)

func TestTokenVerifier_Verify_GeneratedToken(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	baseURL := "https://lfx.example.com"

	gen := auth.NewLinkGenerator(secret, baseURL)
	link, inviteUID, _, _, err := gen.Generate(context.Background(), port.LinkPayload{
		RecipientEmail: "user@example.com",
		DestinationURL: "https://example.com",
		ResourceUID:    "res-123",
		Role:           "Manage",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	token := strings.TrimPrefix(link, baseURL+"/invite?token=")

	claims, err := auth.NewTokenVerifier(secret).Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.InviteUID != inviteUID {
		t.Errorf("InviteUID = %q, want %q", claims.InviteUID, inviteUID)
	}
	if claims.Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", claims.Email, "user@example.com")
	}
}

func TestTokenVerifier_Verify_Rejects(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	now := time.Now()

	validClaims := func() jwt.MapClaims {
		return jwt.MapClaims{
			"iss":        "lfx-v2-invite-service",
			"aud":        jwt.ClaimStrings{"lfx-self-serve"},
			"iat":        now.Unix(),
			"nbf":        now.Unix(),
			"exp":        now.Add(time.Hour).Unix(),
			"jti":        "uid-1",
			"invite_uid": "uid-1",
			"email":      "user@example.com",
		}
	}
	sign := func(t *testing.T, method jwt.SigningMethod, key any, claims jwt.MapClaims) string {
		t.Helper()
		s, err := jwt.NewWithClaims(method, claims).SignedString(key)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		return s
	}

	tests := []struct {
		name  string
		token func(t *testing.T) string
	}{
		{
			name:  "empty token",
			token: func(*testing.T) string { return "" },
		},
		{
			name:  "garbage token",
			token: func(*testing.T) string { return "not-a-jwt" },
		},
		{
			name: "wrong secret",
			token: func(t *testing.T) string {
				return sign(t, jwt.SigningMethodHS256, []byte("other-secret-must-be-at-least-32b!"), validClaims())
			},
		},
		{
			name: "alg none",
			token: func(t *testing.T) string {
				return sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, validClaims())
			},
		},
		{
			name: "HS512 instead of HS256",
			token: func(t *testing.T) string {
				return sign(t, jwt.SigningMethodHS512, secret, validClaims())
			},
		},
		{
			name: "expired",
			token: func(t *testing.T) string {
				c := validClaims()
				c["exp"] = now.Add(-time.Hour).Unix()
				return sign(t, jwt.SigningMethodHS256, secret, c)
			},
		},
		{
			name: "missing exp",
			token: func(t *testing.T) string {
				c := validClaims()
				delete(c, "exp")
				return sign(t, jwt.SigningMethodHS256, secret, c)
			},
		},
		{
			name: "wrong issuer",
			token: func(t *testing.T) string {
				c := validClaims()
				c["iss"] = "someone-else"
				return sign(t, jwt.SigningMethodHS256, secret, c)
			},
		},
		{
			name: "wrong audience",
			token: func(t *testing.T) string {
				c := validClaims()
				c["aud"] = jwt.ClaimStrings{"other-app"}
				return sign(t, jwt.SigningMethodHS256, secret, c)
			},
		},
		{
			name: "jti does not match invite_uid",
			token: func(t *testing.T) string {
				c := validClaims()
				c["invite_uid"] = "uid-2"
				return sign(t, jwt.SigningMethodHS256, secret, c)
			},
		},
		{
			name: "non-string jti",
			token: func(t *testing.T) string {
				c := validClaims()
				c["jti"] = 1
				c["invite_uid"] = 1
				return sign(t, jwt.SigningMethodHS256, secret, c)
			},
		},
		{
			name: "not yet valid (nbf in the future)",
			token: func(t *testing.T) string {
				c := validClaims()
				c["nbf"] = now.Add(time.Hour).Unix()
				return sign(t, jwt.SigningMethodHS256, secret, c)
			},
		},
		{
			name: "missing jti",
			token: func(t *testing.T) string {
				c := validClaims()
				delete(c, "jti")
				delete(c, "invite_uid")
				return sign(t, jwt.SigningMethodHS256, secret, c)
			},
		},
		{
			name: "missing email",
			token: func(t *testing.T) string {
				c := validClaims()
				delete(c, "email")
				return sign(t, jwt.SigningMethodHS256, secret, c)
			},
		},
	}

	v := auth.NewTokenVerifier(secret)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := v.Verify(context.Background(), tt.token(t))
			if !errors.Is(err, port.ErrInvalidInviteToken) {
				t.Errorf("Verify() error = %v, want ErrInvalidInviteToken", err)
			}
		})
	}
}
