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
	return NewNotificationService(email, &noopLinkGenerator{}, nil, NotificationConfig{DefaultReturnURL: testBaseURL})
}

func newServiceWithStore(email *mocks.EmailSender, store *mocks.InviteStore) *NotificationService {
	return NewNotificationService(email, &noopLinkGenerator{}, store, NotificationConfig{DefaultReturnURL: testBaseURL})
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
	if result.RecipientEmail != req.ResolvedRecipientEmail() {
		t.Errorf("recipient_email: got %q, want %q", result.RecipientEmail, req.ResolvedRecipientEmail())
	}
	if result.ExpiresAt.IsZero() {
		t.Error("expires_at should not be zero")
	}
	if len(email.Calls) != 1 {
		t.Fatalf("expected 1 email, got %d", len(email.Calls))
	}
	n := email.Calls[0]
	if n.ResolvedRecipientEmail() != req.ResolvedRecipientEmail() {
		t.Errorf("recipient email: got %q, want %q", n.ResolvedRecipientEmail(), req.ResolvedRecipientEmail())
	}
	if n.ResolvedInviterName() != req.ResolvedInviterName() {
		t.Errorf("inviter name: got %q, want %q", n.ResolvedInviterName(), req.ResolvedInviterName())
	}
	if n.ResolvedResourceName() != req.ResolvedResourceName() {
		t.Errorf("resource name: got %q, want %q", n.ResolvedResourceName(), req.ResolvedResourceName())
	}
	if n.Role != req.Role {
		t.Errorf("role: got %q, want %q", n.Role, req.Role)
	}
}

func TestHandleSendInvite_MissingRecipientEmail_ReturnsError(t *testing.T) {
	email := &mocks.EmailSender{}
	svc := newService(email)

	req := baseInviteRequest()
	req.RecipientEmail = "" //nolint:staticcheck // testing deprecated scalar fallback: no Recipient object, empty scalar → error
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
	req.InviterName = "" //nolint:staticcheck // testing deprecated scalar fallback: no Inviter object, empty scalar → no inviter in email
	if _, err := svc.HandleSendInvite(context.Background(), req); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(email.Calls) != 1 {
		t.Fatalf("expected 1 email, got %d", len(email.Calls))
	}
	if email.Calls[0].ResolvedInviterName() != "" {
		t.Errorf("expected empty inviter name, got %q", email.Calls[0].ResolvedInviterName())
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
	svc := NewNotificationService(email, &errorLinkGenerator{err: linkErr}, nil, NotificationConfig{DefaultReturnURL: testBaseURL})

	_, err := svc.HandleSendInvite(context.Background(), baseInviteRequest())
	if err == nil {
		t.Fatal("expected error when link generator fails, got nil")
	}
	if len(email.Calls) != 0 {
		t.Errorf("expected no email sent when link generation fails, got %d call(s)", len(email.Calls))
	}
}

// TestHandleSendInvite_InviteStorePersistsPending verifies that a successful send
// creates a pending InviteRecord in the store with the destination URL (not the JWT link).
func TestHandleSendInvite_InviteStorePersistsPending(t *testing.T) {
	email := &mocks.EmailSender{}
	store := &mocks.InviteStore{}
	svc := newServiceWithStore(email, store)

	req := baseInviteRequest()
	originalURL := req.ReturnURL
	_, err := svc.HandleSendInvite(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.CreateCalls) != 1 {
		t.Fatalf("expected 1 store.Create call, got %d", len(store.CreateCalls))
	}
	record := store.CreateCalls[0]
	if record.Status != model.InviteStatusPending {
		t.Errorf("status: got %q, want %q", record.Status, model.InviteStatusPending)
	}
	if record.Recipient.Email != "alice@example.com" {
		t.Errorf("recipient.email: got %q, want %q", record.Recipient.Email, "alice@example.com")
	}
	if record.ReturnURL != originalURL {
		t.Errorf("return_url should be original destination URL %q, got %q (JWT link was stored instead)", originalURL, record.ReturnURL)
	}
	if record.UID == "" {
		t.Error("record.UID should not be empty")
	}
	if record.ExpiresAt.IsZero() {
		t.Error("record.ExpiresAt should not be zero")
	}
}

// TestHandleSendInvite_StoreFailureAbortsSend verifies that a KV write failure
// returns an error and does not dispatch the email — we never send an invite we
// cannot track.
func TestHandleSendInvite_StoreFailureAbortsSend(t *testing.T) {
	buf := captureLogs(t)

	email := &mocks.EmailSender{}
	store := &mocks.InviteStore{
		CreateFunc: func(_ context.Context, _ *model.InviteRecord) error {
			return errors.New("kv unavailable")
		},
	}
	svc := newServiceWithStore(email, store)

	_, err := svc.HandleSendInvite(context.Background(), baseInviteRequest())
	if err == nil {
		t.Fatal("expected error when store fails, got nil")
	}
	if len(email.Calls) != 0 {
		t.Errorf("expected no email sent when store fails, got %d", len(email.Calls))
	}
	if !strings.Contains(buf.String(), "invite_store") {
		t.Error("expected invite_store error log entry, found none")
	}
}

// TestHandleSendInvite_StructuredObjectsPreferred verifies that when structured
// Recipient/Inviter/Resource objects are provided, they take precedence over deprecated scalars.
func TestHandleSendInvite_StructuredObjectsPreferred(t *testing.T) {
	email := &mocks.EmailSender{}
	store := &mocks.InviteStore{}
	svc := newServiceWithStore(email, store)

	req := &model.SendInviteRequest{
		// Structured objects (preferred).
		Recipient: &model.Recipient{Name: "Alice Structured", Email: "alice-structured@example.com"},
		Inviter:   &model.Inviter{Name: "Bob Structured", Username: "bob-s", Email: "bob@example.com"},
		Resource:  &model.InviteResource{UID: "structured-res", Name: "Structured Project", Type: "project"},
		// Deprecated scalars — should be ignored when structured objects are present.
		RecipientEmail: "alice-scalar@example.com",
		RecipientName:  "Alice Scalar",
		InviterName:    "Bob Scalar",
		ResourceUID:    "scalar-res",
		ResourceName:   "Scalar Project",
		Role:           string(model.RoleManage),
	}

	result, err := svc.HandleSendInvite(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RecipientEmail != "alice-structured@example.com" {
		t.Errorf("result.RecipientEmail: got %q, want structured email", result.RecipientEmail)
	}
	if len(store.CreateCalls) != 1 {
		t.Fatalf("expected 1 store.Create call, got %d", len(store.CreateCalls))
	}
	record := store.CreateCalls[0]
	if record.Recipient.Email != "alice-structured@example.com" {
		t.Errorf("record.Recipient.Email: got %q, want structured email", record.Recipient.Email)
	}
	if record.Inviter.Username != "bob-s" {
		t.Errorf("record.Inviter.Username: got %q, want %q", record.Inviter.Username, "bob-s")
	}
	if record.Resource.UID != "structured-res" {
		t.Errorf("record.Resource.UID: got %q, want %q", record.Resource.UID, "structured-res")
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
