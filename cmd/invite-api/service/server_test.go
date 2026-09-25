// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port/mocks"
	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

// TestNewServer_Construction verifies the injection constructor returns a
// non-nil Server regardless of the dependency values passed.
func TestNewServer_Construction(t *testing.T) {
	srv := NewServer(nil, nil, nil, nil, nil)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
}

// TestServer_Start_RegistersAllSubscriptions verifies that Start registers a
// queue-subscribe for each of the four expected NATS subjects and returns one
// stop func per subscription. No real broker is required — the mock subscriber
// records calls and returns no-op stop funcs.
func TestServer_Start_RegistersAllSubscriptions(t *testing.T) {
	sub := &mocks.Subscriber{}
	srv := NewServer(sub, nil, nil, nil, nil)

	stops, err := srv.Start(context.Background())
	if err != nil {
		t.Fatalf("Start returned unexpected error: %v", err)
	}
	if len(stops) != 4 {
		t.Errorf("Start: got %d stop funcs, want 4", len(stops))
	}

	wantSubjects := []string{
		api.SendInviteSubject,
		api.InviteAcceptedSubject,
		api.GetInviteSubject,
		api.GetInvitesByEmailSubject,
	}
	if len(sub.Calls) != len(wantSubjects) {
		t.Fatalf("QueueSubscribe called %d times, want %d", len(sub.Calls), len(wantSubjects))
	}
	for i, want := range wantSubjects {
		if sub.Calls[i].Subject != want {
			t.Errorf("subscription[%d]: got subject %q, want %q", i, sub.Calls[i].Subject, want)
		}
	}
}

// TestServer_Start_RollsBackOnFailure verifies that when one QueueSubscribe call
// fails mid-sequence, Start stops all previously registered subscriptions before
// returning the error, leaving no dangling consumers.
func TestServer_Start_RollsBackOnFailure(t *testing.T) {
	// Run once per subject: fail that subject and check that no stops leak.
	subjects := []string{
		api.SendInviteSubject,
		api.InviteAcceptedSubject,
		api.GetInviteSubject,
		api.GetInvitesByEmailSubject,
	}

	for _, failSubject := range subjects {
		t.Run("fail_on_"+failSubject, func(t *testing.T) {
			stopCount := 0
			sub := &mocks.Subscriber{
				QueueSubscribeFunc: func(s, q string, _ func(context.Context, port.InboundMessage)) (func(), error) {
					if s == failSubject {
						return nil, fmt.Errorf("injected failure")
					}
					return func() { stopCount++ }, nil
				},
			}
			srv := NewServer(sub, nil, nil, nil, nil)
			stops, err := srv.Start(context.Background())
			if err == nil {
				t.Fatal("Start: expected error, got nil")
			}
			if stops != nil {
				t.Errorf("Start: expected nil stops on error, got %v", stops)
			}
			// All stop funcs registered before the failure must have been called.
			want := 0
			for _, c := range sub.Calls {
				if c.Subject == failSubject {
					break
				}
				want++
			}
			if stopCount != want {
				t.Errorf("rollback: %d stop funcs called, want %d", stopCount, want)
			}
		})
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
