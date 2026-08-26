// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/infrastructure/auth"
)

func TestLinkGenerator_Generate(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	baseURL := "https://lfx.example.com"
	p := port.LinkPayload{
		RecipientEmail: "user@example.com",
		DestinationURL: "https://lfx.example.com/project/overview?project=my-project",
		ResourceUID:    "proj-abc123",
		ResourceType:   "group",
		Role:           "Manage",
	}

	gen := auth.NewLinkGenerator(secret, baseURL)
	link, inviteUID, expiresAt, err := gen.Generate(context.Background(), p)
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
	if got := claims["email"]; got != p.RecipientEmail {
		t.Errorf("email claim = %v, want %v", got, p.RecipientEmail)
	}
	if got := claims["return_url"]; got != p.DestinationURL {
		t.Errorf("return_url claim = %v, want %v", got, p.DestinationURL)
	}
	if got := claims["resource_uid"]; got != p.ResourceUID {
		t.Errorf("resource_uid claim = %v, want %v", got, p.ResourceUID)
	}
	if got := claims["resource_type"]; got != p.ResourceType {
		t.Errorf("resource_type claim = %v, want %v", got, p.ResourceType)
	}
	if got := claims["role"]; got != p.Role {
		t.Errorf("role claim = %v, want %v", got, p.Role)
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
	link, _, expiresAt, err := gen.Generate(context.Background(), port.LinkPayload{
		RecipientEmail: "user@example.com",
		DestinationURL: "https://example.com",
		ResourceUID:    "res-123",
		ResourceType:   "project",
		Role:           "Manage",
		ExpirationDays: 30,
	})
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
	link, _, _, err := gen.Generate(context.Background(), port.LinkPayload{
		RecipientEmail: "user@example.com",
		DestinationURL: "https://example.com/dest",
		ResourceUID:    "res-123",
		Role:           "Manage",
	})
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
	p := port.LinkPayload{RecipientEmail: "user@example.com", DestinationURL: "https://example.com", ResourceUID: "res-123", Role: "Manage"}

	link1, _, _, _ := gen.Generate(context.Background(), p)
	link2, _, _, _ := gen.Generate(context.Background(), p)

	if link1 == link2 {
		t.Error("two Generate() calls for the same input produced identical links (jti must be unique)")
	}
}

func TestLinkGenerator_Generate_EmptyResourceType_ClaimOmitted(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	gen := auth.NewLinkGenerator(secret, "https://lfx.example.com")

	link, _, _, err := gen.Generate(context.Background(), port.LinkPayload{
		RecipientEmail: "user@example.com",
		DestinationURL: "https://example.com",
		ResourceUID:    "res-123",
		Role:           "Manage",
		// ResourceType intentionally empty
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	tokenStr := strings.TrimPrefix(link, "https://lfx.example.com/invite?token=")
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
	if _, present := claims["resource_type"]; present {
		t.Error("resource_type claim should be absent when resourceType is empty, but it was present")
	}
}

func TestLinkGenerator_Generate_CustomClaims_Embedded(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	gen := auth.NewLinkGenerator(secret, "https://lfx.example.com")

	link, _, _, err := gen.Generate(context.Background(), port.LinkPayload{
		RecipientEmail: "user@example.com",
		DestinationURL: "https://example.com",
		ResourceUID:    "res-123",
		ResourceType:   "group",
		Role:           "Member",
		CustomClaims: map[string]string{
			"committee_invite_uid": "inv-abc123",
			"tenant_id":            "tenant-xyz",
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	tokenStr := strings.TrimPrefix(link, "https://lfx.example.com/invite?token=")
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
	if got := claims["committee_invite_uid"]; got != "inv-abc123" {
		t.Errorf("committee_invite_uid claim = %v, want %v", got, "inv-abc123")
	}
	if got := claims["tenant_id"]; got != "tenant-xyz" {
		t.Errorf("tenant_id claim = %v, want %v", got, "tenant-xyz")
	}
}

func TestLinkGenerator_Generate_CustomClaims_ReservedKeysIgnored(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	gen := auth.NewLinkGenerator(secret, "https://lfx.example.com")

	// Attempt to override a reserved claim via CustomClaims — must be ignored (with a warning log).
	link, _, _, err := gen.Generate(context.Background(), port.LinkPayload{
		RecipientEmail: "user@example.com",
		DestinationURL: "https://example.com",
		ResourceUID:    "res-123",
		ResourceType:   "group",
		Role:           "Member",
		CustomClaims: map[string]string{
			"email":                "attacker@evil.com",
			"committee_invite_uid": "inv-safe",
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	tokenStr := strings.TrimPrefix(link, "https://lfx.example.com/invite?token=")
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
	// The reserved email claim must retain the original value, not the attacker's override.
	if got := claims["email"]; got != "user@example.com" {
		t.Errorf("email claim = %v, want original %v (reserved key must not be overridable)", got, "user@example.com")
	}
	// Non-reserved custom claim must still be present.
	if got := claims["committee_invite_uid"]; got != "inv-safe" {
		t.Errorf("committee_invite_uid claim = %v, want %v", got, "inv-safe")
	}
}

func TestLinkGenerator_Generate_CustomClaims_SubReserved(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	gen := auth.NewLinkGenerator(secret, "https://lfx.example.com")

	// sub is an RFC 7519 registered claim and must be reserved.
	link, _, _, err := gen.Generate(context.Background(), port.LinkPayload{
		RecipientEmail: "user@example.com",
		DestinationURL: "https://example.com",
		ResourceUID:    "res-123",
		ResourceType:   "group",
		Role:           "Member",
		CustomClaims: map[string]string{
			"sub":                  "attacker",
			"committee_invite_uid": "inv-safe",
		},
	})
	if err != nil {
		t.Fatalf("Generate() unexpected error = %v", err)
	}

	tokenStr := strings.TrimPrefix(link, "https://lfx.example.com/invite?token=")
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
	// sub must be absent — a caller must not be able to set the subject claim.
	if _, present := claims["sub"]; present {
		t.Errorf("sub claim is present in token, want it absent (reserved key must not be overridable)")
	}
	// Non-reserved custom claim must still be present.
	if got := claims["committee_invite_uid"]; got != "inv-safe" {
		t.Errorf("committee_invite_uid claim = %v, want %v", got, "inv-safe")
	}
}

func TestLinkGenerator_Generate_CustomClaims_TooMany(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	gen := auth.NewLinkGenerator(secret, "https://lfx.example.com")

	tooMany := make(map[string]string, 17)
	for i := 0; i < 17; i++ {
		tooMany[fmt.Sprintf("key%d", i)] = "value"
	}
	_, _, _, err := gen.Generate(context.Background(), port.LinkPayload{
		RecipientEmail: "user@example.com",
		DestinationURL: "https://example.com",
		ResourceUID:    "res-123",
		ResourceType:   "group",
		Role:           "Member",
		CustomClaims:   tooMany,
	})
	if err == nil {
		t.Fatal("Generate() expected error for too many custom claims, got nil")
	}
	if !errors.Is(err, port.ErrInvalidCustomClaims) {
		t.Errorf("expected ErrInvalidCustomClaims, got %v", err)
	}
}

func TestLinkGenerator_Generate_CustomClaims_KeyTooLong(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	gen := auth.NewLinkGenerator(secret, "https://lfx.example.com")

	longKey := strings.Repeat("k", 65)
	_, _, _, err := gen.Generate(context.Background(), port.LinkPayload{
		RecipientEmail: "user@example.com",
		DestinationURL: "https://example.com",
		ResourceUID:    "res-123",
		ResourceType:   "group",
		Role:           "Member",
		CustomClaims:   map[string]string{longKey: "value"},
	})
	if err == nil {
		t.Fatal("Generate() expected error for oversized key, got nil")
	}
	if !errors.Is(err, port.ErrInvalidCustomClaims) {
		t.Errorf("expected ErrInvalidCustomClaims, got %v", err)
	}
}

func TestLinkGenerator_Generate_CustomClaims_ValueTooLong(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32bytes!")
	gen := auth.NewLinkGenerator(secret, "https://lfx.example.com")

	longValue := strings.Repeat("v", 1025)
	_, _, _, err := gen.Generate(context.Background(), port.LinkPayload{
		RecipientEmail: "user@example.com",
		DestinationURL: "https://example.com",
		ResourceUID:    "res-123",
		ResourceType:   "group",
		Role:           "Member",
		CustomClaims:   map[string]string{"committee_invite_uid": longValue},
	})
	if err == nil {
		t.Fatal("Generate() expected error for oversized value, got nil")
	}
	if !errors.Is(err, port.ErrInvalidCustomClaims) {
		t.Errorf("expected ErrInvalidCustomClaims, got %v", err)
	}
}
