// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port/mocks"
)

const (
	testBaseURL      = "https://lfx.example.com"
	testResourceUID  = "res-abc123"
	testResourceName = "Test Project"
)

// noopLinkGenerator returns a fixed invite link without signing, for use in tests.
type noopLinkGenerator struct{}

func (n *noopLinkGenerator) Generate(recipientEmail, destinationURL, resourceUID, role string) (string, string, error) {
	return testBaseURL + "/invite?token=test-token-for-" + recipientEmail, "test-invite-uid", nil
}

func newService(email *mocks.EmailSender) *NotificationService {
	return NewNotificationService(email, &noopLinkGenerator{}, &mocks.InvitePublisher{}, NotificationConfig{DefaultReturnURL: testBaseURL})
}

func baseInviteRequest() *model.SendInviteRequest {
	return &model.SendInviteRequest{
		RecipientEmail: "alice@example.com",
		RecipientName:  "Alice",
		InviterName:    "Bob",
		ResourceUID:    testResourceUID,
		ResourceName:   testResourceName,
		Role:           string(model.RoleManage),
		ReturnURL:      testBaseURL + "/resources/" + testResourceUID,
	}
}

func TestHandleSendInvite_HappyPath(t *testing.T) {
	email := &mocks.EmailSender{}
	publisher := &mocks.InvitePublisher{}
	svc := NewNotificationService(email, &noopLinkGenerator{}, publisher, NotificationConfig{DefaultReturnURL: testBaseURL})

	req := baseInviteRequest()
	if err := svc.HandleSendInvite(context.Background(), req); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(email.Calls) != 1 {
		t.Fatalf("expected 1 email, got %d", len(email.Calls))
	}
	n := email.Calls[0]
	if n.RecipientEmail != req.RecipientEmail {
		t.Errorf("recipient email: got %q, want %q", n.RecipientEmail, req.RecipientEmail)
	}
	if n.InviterName != req.InviterName {
		t.Errorf("inviter name: got %q, want %q", n.InviterName, req.InviterName)
	}
	if n.ResourceName != req.ResourceName {
		t.Errorf("resource name: got %q, want %q", n.ResourceName, req.ResourceName)
	}
	if n.Role != req.Role {
		t.Errorf("role: got %q, want %q", n.Role, req.Role)
	}

	// invite.created must be published with the correct fields.
	if len(publisher.Calls) != 1 {
		t.Fatalf("expected 1 invite.created publish, got %d", len(publisher.Calls))
	}
	event := publisher.Calls[0]
	if event.InviteUID != "test-invite-uid" {
		t.Errorf("invite_uid: got %q, want %q", event.InviteUID, "test-invite-uid")
	}
	if event.ResourceUID != req.ResourceUID {
		t.Errorf("resource_uid: got %q, want %q", event.ResourceUID, req.ResourceUID)
	}
	if event.RecipientEmail != req.RecipientEmail {
		t.Errorf("recipient_email: got %q, want %q", event.RecipientEmail, req.RecipientEmail)
	}
	if event.Role != req.Role {
		t.Errorf("role: got %q, want %q", event.Role, req.Role)
	}
	if event.ExpiresAt == 0 {
		t.Error("expires_at must be set")
	}
}

func TestHandleSendInvite_MissingRecipientEmail_Skips(t *testing.T) {
	email := &mocks.EmailSender{}
	svc := newService(email)

	req := baseInviteRequest()
	req.RecipientEmail = ""
	if err := svc.HandleSendInvite(context.Background(), req); err != nil {
		t.Fatalf("expected nil error for missing email, got %v", err)
	}
	if len(email.Calls) != 0 {
		t.Error("expected no email sent when recipient email is empty")
	}
}

func TestHandleSendInvite_EmailSendError_Propagates(t *testing.T) {
	sendErr := errors.New("email service unavailable")
	email := &mocks.EmailSender{
		SendFunc: func(_ context.Context, _ *model.SendInviteRequest) error {
			return sendErr
		},
	}
	svc := newService(email)

	err := svc.HandleSendInvite(context.Background(), baseInviteRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sendErr) {
		t.Errorf("expected wrapped sendErr, got %v", err)
	}
}

func TestHandleSendInvite_NoInviter(t *testing.T) {
	email := &mocks.EmailSender{}
	svc := newService(email)

	req := baseInviteRequest()
	req.InviterName = ""
	if err := svc.HandleSendInvite(context.Background(), req); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(email.Calls) != 1 {
		t.Fatalf("expected 1 email, got %d", len(email.Calls))
	}
	if email.Calls[0].InviterName != "" {
		t.Errorf("expected empty inviter name, got %q", email.Calls[0].InviterName)
	}
}

func TestHandleSendInvite_UnrecognisedRole_Skips(t *testing.T) {
	email := &mocks.EmailSender{}
	svc := newService(email)

	req := baseInviteRequest()
	req.Role = "superadmin"
	if err := svc.HandleSendInvite(context.Background(), req); err != nil {
		t.Fatalf("expected nil error for unrecognised role, got %v", err)
	}
	if len(email.Calls) != 0 {
		t.Error("expected no email sent for unrecognised role")
	}
}

func TestHandleSendInvite_ViewRole_Accepted(t *testing.T) {
	email := &mocks.EmailSender{}
	svc := newService(email)

	req := baseInviteRequest()
	req.Role = string(model.RoleView)
	if err := svc.HandleSendInvite(context.Background(), req); err != nil {
		t.Fatalf("expected nil error for View role, got %v", err)
	}
	if len(email.Calls) != 1 {
		t.Fatalf("expected 1 email for View role, got %d", len(email.Calls))
	}
	if email.Calls[0].Role != string(model.RoleView) {
		t.Errorf("role: got %q, want %q", email.Calls[0].Role, model.RoleView)
	}
}
