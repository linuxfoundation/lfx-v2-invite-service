// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/infrastructure/auth"
)

func TestLinkGenerator_Generate(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	baseURL := "https://lfx.example.com"
	recipientEmail := "user@example.com"
	returnURL := "https://lfx.example.com/project/overview?project=my-project"
	resourceUID := "proj-abc123"
	role := "Manage"

	gen := auth.NewLinkGenerator(secret, baseURL)
	link, inviteUID, expiresAt, err := gen.Generate(recipientEmail, returnURL, resourceUID, role, 0)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if inviteUID == "" {
		t.Fatal("Generate() returned empty inviteUID")
	}

	// Link must start with the expected prefix.
	prefix := baseURL + "/invite?token="
	if !strings.HasPrefix(link, prefix) {
		t.Fatalf("link %q does not start with %q", link, prefix)
	}

	tokenStr := strings.TrimPrefix(link, prefix)

	// Parse and verify the token.
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return secret, nil
	})
	if err != nil {
		t.Fatalf("jwt.Parse() error = %v", err)
	}
	if !token.Valid {
		t.Fatal("token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("unexpected claims type")
	}

	// Verify required claims are present and correct.
	if got := claims["email"]; got != recipientEmail {
		t.Errorf("email claim = %v, want %v", got, recipientEmail)
	}
	if got := claims["return_url"]; got != returnURL {
		t.Errorf("return_url claim = %v, want %v", got, returnURL)
	}
	if got := claims["resource_uid"]; got != resourceUID {
		t.Errorf("resource_uid claim = %v, want %v", got, resourceUID)
	}
	if got := claims["role"]; got != role {
		t.Errorf("role claim = %v, want %v", got, role)
	}
	if claims["jti"] == "" || claims["jti"] == nil {
		t.Error("jti claim is missing or empty")
	}
	// The returned inviteUID must match the jti and invite_uid embedded in the token.
	if got := claims["jti"]; got != inviteUID {
		t.Errorf("jti claim = %v, want returned inviteUID %v", got, inviteUID)
	}
	if got := claims["invite_uid"]; got != inviteUID {
		t.Errorf("invite_uid claim = %v, want returned inviteUID %v", got, inviteUID)
	}

	// exp should be ~30 days from now (default TTL).
	expFloat, ok := claims["exp"].(float64)
	if !ok {
		t.Fatal("exp claim is not a number")
	}
	exp := time.Unix(int64(expFloat), 0)
	wantExpMin := time.Now().Add(29*24*time.Hour + 23*time.Hour)
	wantExpMax := time.Now().Add(30*24*time.Hour + time.Minute)
	if exp.Before(wantExpMin) || exp.After(wantExpMax) {
		t.Errorf("exp %v is outside expected range [%v, %v]", exp, wantExpMin, wantExpMax)
	}
	// Returned expiresAt should match the JWT exp claim within 1 second.
	if expiresAt.Unix() != exp.Unix() {
		t.Errorf("returned expiresAt %v does not match JWT exp %v", expiresAt, exp)
	}
}

func TestLinkGenerator_Generate_Custom_ExpirationDays(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	baseURL := "https://lfx.example.com"

	gen := auth.NewLinkGenerator(secret, baseURL)
	link, _, expiresAt, err := gen.Generate("user@example.com", "https://example.com", "res-123", "Manage", 30)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	tokenStr := strings.TrimPrefix(link, baseURL+"/invite?token=")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return secret, nil
	})
	if err != nil {
		t.Fatalf("jwt.Parse() error = %v", err)
	}

	claims, _ := token.Claims.(jwt.MapClaims)
	expFloat, _ := claims["exp"].(float64)
	exp := time.Unix(int64(expFloat), 0)

	// exp should be ~30 days from now.
	wantExpMin := time.Now().Add(29*24*time.Hour + 23*time.Hour)
	wantExpMax := time.Now().Add(30*24*time.Hour + time.Minute)
	if exp.Before(wantExpMin) || exp.After(wantExpMax) {
		t.Errorf("exp %v is outside expected 30-day range [%v, %v]", exp, wantExpMin, wantExpMax)
	}
	// Returned expiresAt should match the JWT exp claim within 1 second.
	if expiresAt.Unix() != exp.Unix() {
		t.Errorf("returned expiresAt %v does not match JWT exp %v", expiresAt, exp)
	}
}

func TestLinkGenerator_Generate_WrongSecret(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	otherSecret := []byte("other-secret-must-be-at-least-32b!")
	baseURL := "https://lfx.example.com"

	gen := auth.NewLinkGenerator(secret, baseURL)
	link, _, _, err := gen.Generate("user@example.com", "https://example.com/dest", "res-123", "Manage", 0)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	tokenStr := strings.TrimPrefix(link, baseURL+"/invite?token=")

	_, err = jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return otherSecret, nil
	})
	if err == nil {
		t.Error("expected signature verification to fail with wrong secret, but it succeeded")
	}
}

func TestLinkGenerator_Generate_UniqueJTI(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	gen := auth.NewLinkGenerator(secret, "https://lfx.example.com")

	link1, _, _, _ := gen.Generate("user@example.com", "https://example.com", "res-123", "Manage", 0)
	link2, _, _, _ := gen.Generate("user@example.com", "https://example.com", "res-123", "Manage", 0)

	if link1 == link2 {
		t.Error("two Generate() calls for the same input produced identical links (jti must be unique)")
	}
}
