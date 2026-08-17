// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
)

func TestNatsMsg_Data(t *testing.T) {
	payload := []byte(`{"key":"value"}`)
	m := &natsMsg{msg: &nats.Msg{Data: payload}}
	if got := m.Data(); string(got) != string(payload) {
		t.Fatalf("Data() = %q, want %q", got, payload)
	}
}

func TestNatsMsg_Subject(t *testing.T) {
	m := &natsMsg{msg: &nats.Msg{Subject: "lfx.invite-service.send_invite"}}
	if got := m.Subject(); got != "lfx.invite-service.send_invite" {
		t.Fatalf("Subject() = %q, want %q", got, "lfx.invite-service.send_invite")
	}
}

// TestNatsMsg_Reply_NoReplyAddress verifies the fire-and-forget no-op: when the
// NATS message has no reply address, Reply returns nil without publishing anything.
func TestNatsMsg_Reply_NoReplyAddress(t *testing.T) {
	m := &natsMsg{msg: &nats.Msg{Subject: "lfx.invite.accepted", Reply: ""}}
	if err := m.Reply(context.Background(), []byte(`ok`)); err != nil {
		t.Fatalf("Reply with no reply address should be a no-op, got error: %v", err)
	}
}

// TestNatsMsg_Reply_TransportFailure verifies that a transport error from
// msg.Respond is propagated to the caller. A *nats.Msg with a populated Reply
// field but a nil connection returns nats.ErrInvalidConnection from Respond.
func TestNatsMsg_Reply_TransportFailure(t *testing.T) {
	// nc (the unexported connection field) is nil on a zero-value *nats.Msg, so
	// Respond returns ErrInvalidConnection — an easy way to inject a transport error
	// without a real broker.
	m := &natsMsg{msg: &nats.Msg{Subject: "lfx.invite-service.send_invite", Reply: "INBOX.test"}}
	err := m.Reply(context.Background(), []byte(`{"uid":"x"}`))
	if err == nil {
		t.Fatal("Reply with nil connection should return an error, got nil")
	}
}
