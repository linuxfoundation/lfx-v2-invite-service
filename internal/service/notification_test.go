// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

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

// errorLinkGenerator always returns an error from Generate.
type errorLinkGenerator struct{ err error }

func (e *errorLinkGenerator) Generate(_, _, _, _ string, _ int) (string, string, time.Time, error) {
	return "", "", time.Time{}, e.err
}

// captureLogs redirects the slog default logger to a buffer for the duration of the test
// and restores it on cleanup. Returns a pointer to the buffer so callers can inspect output.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	orig := slog.Default()
	buf := &bytes.Buffer{}
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(orig) })
	return buf
}

func (n *noopLinkGenerator) Generate(recipientEmail, destinationURL, resourceUID, role string, expirationDays int) (string, string, time.Time, error) {
	return testBaseURL + "/invite?token=test-token-for-" + recipientEmail, "test-invite-uid", time.Now().Add(7 * 24 * time.Hour), nil
}

func newService(email *mocks.EmailSender) *NotificationService {
	return NewNotificationService(email, &noopLinkGenerator{}, NotificationConfig{DefaultReturnURL: testBaseURL})
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
	svc := newService(email)

	req := baseInviteRequest()
	result, err := svc.HandleSendInvite(context.Background(), req)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.InviteUID != "test-invite-uid" {
		t.Errorf("invite_uid: got %q, want %q", result.InviteUID, "test-invite-uid")
	}
	if result.RecipientEmail != req.RecipientEmail {
		t.Errorf("recipient_email: got %q, want %q", result.RecipientEmail, req.RecipientEmail)
	}
	if result.ExpiresAt.IsZero() {
		t.Error("expires_at should not be zero")
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
}

func TestHandleSendInvite_MissingRecipientEmail_ReturnsError(t *testing.T) {
	email := &mocks.EmailSender{}
	svc := newService(email)

	req := baseInviteRequest()
	req.RecipientEmail = ""
	_, err := svc.HandleSendInvite(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing recipient email, got nil")
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

	_, err := svc.HandleSendInvite(context.Background(), baseInviteRequest())
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
	if _, err := svc.HandleSendInvite(context.Background(), req); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(email.Calls) != 1 {
		t.Fatalf("expected 1 email, got %d", len(email.Calls))
	}
	if email.Calls[0].InviterName != "" {
		t.Errorf("expected empty inviter name, got %q", email.Calls[0].InviterName)
	}
}

func TestHandleSendInvite_UnrecognisedRole_ReturnsError(t *testing.T) {
	email := &mocks.EmailSender{}
	svc := newService(email)

	req := baseInviteRequest()
	req.Role = "superadmin"
	_, err := svc.HandleSendInvite(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unrecognised role, got nil")
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
	if _, err := svc.HandleSendInvite(context.Background(), req); err != nil {
		t.Fatalf("expected nil error for View role, got %v", err)
	}
	if len(email.Calls) != 1 {
		t.Fatalf("expected 1 email for View role, got %d", len(email.Calls))
	}
	if email.Calls[0].Role != string(model.RoleView) {
		t.Errorf("role: got %q, want %q", email.Calls[0].Role, model.RoleView)
	}
}

func TestHandleSendInvite_MemberRole_Accepted(t *testing.T) {
	email := &mocks.EmailSender{}
	svc := newService(email)

	req := baseInviteRequest()
	req.Role = string(model.RoleMember)
	if _, err := svc.HandleSendInvite(context.Background(), req); err != nil {
		t.Fatalf("expected nil error for Member role, got %v", err)
	}
	if len(email.Calls) != 1 {
		t.Fatalf("expected 1 email for Member role, got %d", len(email.Calls))
	}
	if email.Calls[0].Role != string(model.RoleMember) {
		t.Errorf("role: got %q, want %q", email.Calls[0].Role, model.RoleMember)
	}
}

// M18.1: a LinkGenerator failure returns an error and never calls SendNotification.
func TestHandleSendInvite_LinkGeneratorFailure_NoEmailSent(t *testing.T) {
	linkErr := errors.New("signing key unavailable")
	email := &mocks.EmailSender{}
	svc := NewNotificationService(email, &errorLinkGenerator{err: linkErr}, NotificationConfig{DefaultReturnURL: testBaseURL})

	_, err := svc.HandleSendInvite(context.Background(), baseInviteRequest())
	if err == nil {
		t.Fatal("expected error when link generator fails, got nil")
	}
	if len(email.Calls) != 0 {
		t.Errorf("expected no email sent when link generation fails, got %d call(s)", len(email.Calls))
	}
}

// M18.2: the ReturnURL passed to SendNotification is the signed link, not the original.
func TestHandleSendInvite_ReturnURLReplacedWithSignedLink(t *testing.T) {
	email := &mocks.EmailSender{}
	svc := newService(email)

	req := baseInviteRequest()
	originalURL := req.ReturnURL
	if _, err := svc.HandleSendInvite(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(email.Calls) != 1 {
		t.Fatalf("expected 1 email, got %d", len(email.Calls))
	}
	got := email.Calls[0].ReturnURL
	if got == originalURL {
		t.Errorf("ReturnURL was not replaced: still %q (expected a signed invite link)", got)
	}
	if !strings.Contains(got, "/invite?token=") {
		t.Errorf("ReturnURL %q does not look like a signed invite link", got)
	}
}

// M18.3: when SendNotification fails, a DeliveryStateFailed audit entry is emitted.
func TestHandleSendInvite_EmailSendError_AuditsFailed(t *testing.T) {
	buf := captureLogs(t)

	sendErr := errors.New("smtp timeout")
	email := &mocks.EmailSender{
		SendFunc: func(_ context.Context, _ *model.SendInviteRequest) error { return sendErr },
	}
	svc := newService(email)

	_, err := svc.HandleSendInvite(context.Background(), baseInviteRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrEmailDispatchFailed) {
		t.Errorf("expected ErrEmailDispatchFailed in error chain, got %v", err)
	}

	logs := buf.String()
	if !strings.Contains(logs, "notification_audit") {
		t.Error("expected a notification_audit log entry, found none")
	}
	if !strings.Contains(logs, string(model.DeliveryStateFailed)) {
		t.Errorf("expected delivery_state %q in audit log, got:\n%s", model.DeliveryStateFailed, logs)
	}
}
