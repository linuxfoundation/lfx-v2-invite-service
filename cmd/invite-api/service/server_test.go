// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"strings"
	"testing"
)

// TestNewServer_Construction verifies the injection constructor returns a
// non-nil Server regardless of the dependency values passed.
func TestNewServer_Construction(t *testing.T) {
	srv := NewServer(nil, nil, nil, nil, nil)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
}

// TestNewServerFromConfig_ConfigValidation verifies that NewServerFromConfig
// rejects invalid configuration before attempting any network connections.
// All cases below should fail on config alone — no NATS broker is required.
func TestNewServerFromConfig_ConfigValidation(t *testing.T) {
	tests := []struct {
		name            string
		cfg             AppConfig
		wantErrContains string
	}{
		{
			name:            "missing JWT secret",
			cfg:             AppConfig{InviteJWTSecret: ""},
			wantErrContains: "INVITE_JWT_SECRET",
		},
		{
			name:            "JWT secret too short",
			cfg:             AppConfig{InviteJWTSecret: "short"},
			wantErrContains: "32 bytes",
		},
		{
			name:            "JWT secret exactly 31 bytes is still too short",
			cfg:             AppConfig{InviteJWTSecret: strings.Repeat("a", 31)},
			wantErrContains: "32 bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewServerFromConfig(context.Background(), tt.cfg)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrContains)
			}
		})
	}
}
