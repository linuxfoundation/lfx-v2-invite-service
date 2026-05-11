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
	testBaseURL    = "https://lfx.example.com"
	testProjectUID = "proj-abc123"
	testProjectName = "Test Project"
)

func newService(email *mocks.EmailSender, reader *mocks.ProjectNameReader) *NotificationService {
	return NewNotificationService(email, reader, NotificationConfig{LFXBaseURL: testBaseURL})
}

func baseMsg(old, new model.ProjectSettings) *model.ProjectSettingsUpdatedMessage {
	return &model.ProjectSettingsUpdatedMessage{
		ProjectUID:  testProjectUID,
		OldSettings: old,
		NewSettings: new,
	}
}

func TestHandleProjectSettingsUpdated_NoAddedUsers(t *testing.T) {
	email := &mocks.EmailSender{}
	reader := &mocks.ProjectNameReader{}
	svc := newService(email, reader)

	alice := model.UserInfo{Username: "alice", Email: "alice@example.com", Name: "Alice"}
	msg := baseMsg(
		model.ProjectSettings{Writers: []model.UserInfo{alice}},
		model.ProjectSettings{Writers: []model.UserInfo{alice}},
	)

	if err := svc.HandleProjectSettingsUpdated(context.Background(), msg); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(reader.Calls) != 0 {
		t.Error("expected GetProjectName not to be called")
	}
	if len(email.Calls) != 0 {
		t.Error("expected SendProjectAddedNotification not to be called")
	}
}

func TestHandleProjectSettingsUpdated_GetProjectNameError(t *testing.T) {
	email := &mocks.EmailSender{}
	reader := &mocks.ProjectNameReader{
		GetProjectNameFunc: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("NATS timeout")
		},
	}
	svc := newService(email, reader)

	alice := model.UserInfo{Username: "alice", Email: "alice@example.com", Name: "Alice"}
	msg := baseMsg(
		model.ProjectSettings{},
		model.ProjectSettings{Writers: []model.UserInfo{alice}},
	)

	err := svc.HandleProjectSettingsUpdated(context.Background(), msg)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if len(email.Calls) != 0 {
		t.Error("expected no emails when project name lookup fails")
	}
}

func TestHandleProjectSettingsUpdated_HappyPath_SingleUser(t *testing.T) {
	email := &mocks.EmailSender{}
	reader := &mocks.ProjectNameReader{
		GetProjectNameFunc: func(_ context.Context, _ string) (string, error) {
			return testProjectName, nil
		},
	}
	svc := newService(email, reader)

	alice := model.UserInfo{Username: "alice", Email: "alice@example.com", Name: "Alice"}
	msg := baseMsg(
		model.ProjectSettings{},
		model.ProjectSettings{Writers: []model.UserInfo{alice}},
	)

	if err := svc.HandleProjectSettingsUpdated(context.Background(), msg); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(email.Calls) != 1 {
		t.Fatalf("expected 1 email, got %d", len(email.Calls))
	}

	n := email.Calls[0]
	if n.RecipientEmail != alice.Email {
		t.Errorf("recipient email: got %q, want %q", n.RecipientEmail, alice.Email)
	}
	if n.RecipientName != alice.Name {
		t.Errorf("recipient name: got %q, want %q", n.RecipientName, alice.Name)
	}
	if n.ProjectName != testProjectName {
		t.Errorf("project name: got %q, want %q", n.ProjectName, testProjectName)
	}
	if n.Role != model.RoleManage {
		t.Errorf("role: got %q, want %q", n.Role, model.RoleManage)
	}
	wantURL := testBaseURL + "/projects/" + testProjectUID
	if n.DeepLinkURL != wantURL {
		t.Errorf("deep link: got %q, want %q", n.DeepLinkURL, wantURL)
	}
}

func TestHandleProjectSettingsUpdated_HappyPath_MultipleUsers(t *testing.T) {
	email := &mocks.EmailSender{}
	reader := &mocks.ProjectNameReader{
		GetProjectNameFunc: func(_ context.Context, _ string) (string, error) {
			return testProjectName, nil
		},
	}
	svc := newService(email, reader)

	alice := model.UserInfo{Username: "alice", Email: "alice@example.com", Name: "Alice"}
	bob := model.UserInfo{Username: "bob", Email: "bob@example.com", Name: "Bob"}
	msg := baseMsg(
		model.ProjectSettings{},
		model.ProjectSettings{Writers: []model.UserInfo{alice, bob}},
	)

	if err := svc.HandleProjectSettingsUpdated(context.Background(), msg); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(email.Calls) != 2 {
		t.Errorf("expected 2 emails, got %d", len(email.Calls))
	}
	// GetProjectName called exactly once regardless of user count.
	if len(reader.Calls) != 1 {
		t.Errorf("expected GetProjectName called once, got %d", len(reader.Calls))
	}
}

func TestHandleProjectSettingsUpdated_SkipsUserWithNoEmail(t *testing.T) {
	email := &mocks.EmailSender{}
	reader := &mocks.ProjectNameReader{
		GetProjectNameFunc: func(_ context.Context, _ string) (string, error) {
			return testProjectName, nil
		},
	}
	svc := newService(email, reader)

	noEmail := model.UserInfo{Username: "ghost", Name: "Ghost"}
	msg := baseMsg(
		model.ProjectSettings{},
		model.ProjectSettings{Writers: []model.UserInfo{noEmail}},
	)

	if err := svc.HandleProjectSettingsUpdated(context.Background(), msg); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(email.Calls) != 0 {
		t.Error("expected no email sent for user with no email address")
	}
}

func TestHandleProjectSettingsUpdated_EmailSendError_ReturnsFirstError(t *testing.T) {
	sendErr := errors.New("SMTP unavailable")
	email := &mocks.EmailSender{
		SendFunc: func(_ context.Context, _ *model.ProjectAddedNotification) error {
			return sendErr
		},
	}
	reader := &mocks.ProjectNameReader{
		GetProjectNameFunc: func(_ context.Context, _ string) (string, error) {
			return testProjectName, nil
		},
	}
	svc := newService(email, reader)

	alice := model.UserInfo{Username: "alice", Email: "alice@example.com", Name: "Alice"}
	msg := baseMsg(
		model.ProjectSettings{},
		model.ProjectSettings{Writers: []model.UserInfo{alice}},
	)

	err := svc.HandleProjectSettingsUpdated(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sendErr) {
		t.Errorf("expected wrapped sendErr, got %v", err)
	}
}

func TestHandleProjectSettingsUpdated_PartialEmailFailure_ProcessesAllUsers(t *testing.T) {
	sendErr := errors.New("SMTP unavailable")
	callCount := 0
	email := &mocks.EmailSender{
		SendFunc: func(_ context.Context, _ *model.ProjectAddedNotification) error {
			callCount++
			if callCount == 1 {
				return sendErr
			}
			return nil
		},
	}
	reader := &mocks.ProjectNameReader{
		GetProjectNameFunc: func(_ context.Context, _ string) (string, error) {
			return testProjectName, nil
		},
	}
	svc := newService(email, reader)

	alice := model.UserInfo{Username: "alice", Email: "alice@example.com", Name: "Alice"}
	bob := model.UserInfo{Username: "bob", Email: "bob@example.com", Name: "Bob"}
	msg := baseMsg(
		model.ProjectSettings{},
		model.ProjectSettings{Writers: []model.UserInfo{alice, bob}},
	)

	err := svc.HandleProjectSettingsUpdated(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error for partial failure, got nil")
	}
	// Both users must be attempted despite the first failure.
	if len(email.Calls) != 2 {
		t.Errorf("expected 2 email attempts, got %d", len(email.Calls))
	}
}

func TestHandleProjectSettingsUpdated_MixedRoles(t *testing.T) {
	email := &mocks.EmailSender{}
	reader := &mocks.ProjectNameReader{
		GetProjectNameFunc: func(_ context.Context, _ string) (string, error) {
			return testProjectName, nil
		},
	}
	svc := newService(email, reader)

	writer := model.UserInfo{Username: "alice", Email: "alice@example.com", Name: "Alice"}
	auditor := model.UserInfo{Username: "bob", Email: "bob@example.com", Name: "Bob"}
	msg := baseMsg(
		model.ProjectSettings{},
		model.ProjectSettings{
			Writers:  []model.UserInfo{writer},
			Auditors: []model.UserInfo{auditor},
		},
	)

	if err := svc.HandleProjectSettingsUpdated(context.Background(), msg); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(email.Calls) != 2 {
		t.Fatalf("expected 2 emails, got %d", len(email.Calls))
	}

	roleByUser := map[string]model.Role{}
	for _, n := range email.Calls {
		roleByUser[n.RecipientEmail] = n.Role
	}
	if roleByUser[writer.Email] != model.RoleManage {
		t.Errorf("writer role: got %q, want %q", roleByUser[writer.Email], model.RoleManage)
	}
	if roleByUser[auditor.Email] != model.RoleView {
		t.Errorf("auditor role: got %q, want %q", roleByUser[auditor.Email], model.RoleView)
	}
}

func TestHandleProjectSettingsUpdated_SkipAndSendMixed(t *testing.T) {
	email := &mocks.EmailSender{}
	reader := &mocks.ProjectNameReader{
		GetProjectNameFunc: func(_ context.Context, _ string) (string, error) {
			return testProjectName, nil
		},
	}
	svc := newService(email, reader)

	noEmail := model.UserInfo{Username: "ghost", Name: "Ghost"}
	withEmail := model.UserInfo{Username: "alice", Email: "alice@example.com", Name: "Alice"}
	msg := baseMsg(
		model.ProjectSettings{},
		model.ProjectSettings{Writers: []model.UserInfo{noEmail, withEmail}},
	)

	if err := svc.HandleProjectSettingsUpdated(context.Background(), msg); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(email.Calls) != 1 {
		t.Errorf("expected 1 email (skipping user without email), got %d", len(email.Calls))
	}
	if email.Calls[0].RecipientEmail != withEmail.Email {
		t.Errorf("wrong recipient: got %q, want %q", email.Calls[0].RecipientEmail, withEmail.Email)
	}
}
