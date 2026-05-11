// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package smtp

import (
	"strings"
	"testing"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
)

func baseNotification() *model.ProjectAddedNotification {
	return &model.ProjectAddedNotification{
		RecipientName:  "Alice",
		RecipientEmail: "alice@example.com",
		ProjectUID:     "proj-123",
		ProjectName:    "My Project",
		Role:           model.RoleManage,
		DeepLinkURL:    "https://lfx.example.com/projects/proj-123",
	}
}

func TestRenderProjectAddedHTML_ContainsProjectName(t *testing.T) {
	n := baseNotification()
	out := renderProjectAddedHTML(n)
	if !strings.Contains(out, n.ProjectName) {
		t.Errorf("HTML output missing project name %q", n.ProjectName)
	}
}

func TestRenderProjectAddedHTML_ContainsRecipientName(t *testing.T) {
	n := baseNotification()
	out := renderProjectAddedHTML(n)
	if !strings.Contains(out, n.RecipientName) {
		t.Errorf("HTML output missing recipient name %q", n.RecipientName)
	}
}

func TestRenderProjectAddedHTML_ContainsDeepLink(t *testing.T) {
	n := baseNotification()
	out := renderProjectAddedHTML(n)
	if !strings.Contains(out, n.DeepLinkURL) {
		t.Errorf("HTML output missing deep link %q", n.DeepLinkURL)
	}
}

func TestRenderProjectAddedHTML_WithInviter(t *testing.T) {
	n := baseNotification()
	n.InviterName = "Bob"
	out := renderProjectAddedHTML(n)
	if !strings.Contains(out, "Bob") {
		t.Errorf("HTML output missing inviter name %q", "Bob")
	}
}

func TestRenderProjectAddedHTML_WithoutInviter(t *testing.T) {
	n := baseNotification()
	n.InviterName = ""
	out := renderProjectAddedHTML(n)
	// The no-inviter branch should render generic text, not include an empty name.
	if strings.Contains(out, "has added you") && strings.Contains(out, " <strong></strong>") {
		t.Error("HTML output should not render empty inviter name")
	}
	if !strings.Contains(out, "You have been added") {
		t.Error("HTML output missing generic 'You have been added' text when no inviter")
	}
}

func TestRenderProjectAddedHTML_EscapesSpecialCharacters(t *testing.T) {
	n := baseNotification()
	n.ProjectName = "<script>alert('xss')</script>"
	out := renderProjectAddedHTML(n)
	if strings.Contains(out, "<script>") {
		t.Error("HTML output must escape project name containing script tags")
	}
}

func TestRenderProjectAddedPlain_ContainsProjectName(t *testing.T) {
	n := baseNotification()
	out := renderProjectAddedPlain(n)
	if !strings.Contains(out, n.ProjectName) {
		t.Errorf("plain text output missing project name %q", n.ProjectName)
	}
}

func TestRenderProjectAddedPlain_ContainsDeepLink(t *testing.T) {
	n := baseNotification()
	out := renderProjectAddedPlain(n)
	if !strings.Contains(out, n.DeepLinkURL) {
		t.Errorf("plain text output missing deep link %q", n.DeepLinkURL)
	}
}

func TestRenderProjectAddedPlain_ContainsRecipientName(t *testing.T) {
	n := baseNotification()
	out := renderProjectAddedPlain(n)
	if !strings.Contains(out, n.RecipientName) {
		t.Errorf("plain text output missing recipient name %q", n.RecipientName)
	}
}

func TestRenderProjectAddedPlain_WithInviter(t *testing.T) {
	n := baseNotification()
	n.InviterName = "Bob"
	out := renderProjectAddedPlain(n)
	if !strings.Contains(out, "Bob has added you") {
		t.Errorf("plain text output missing inviter line, got:\n%s", out)
	}
}

func TestRenderProjectAddedPlain_WithoutInviter(t *testing.T) {
	n := baseNotification()
	n.InviterName = ""
	out := renderProjectAddedPlain(n)
	if !strings.Contains(out, "You have been added to") {
		t.Errorf("plain text output missing generic added line, got:\n%s", out)
	}
}

func TestRenderProjectAddedPlain_ContainsRole(t *testing.T) {
	n := baseNotification()
	out := renderProjectAddedPlain(n)
	if !strings.Contains(out, string(n.Role)) {
		t.Errorf("plain text output missing role %q", n.Role)
	}
}

func TestRenderProjectAddedPlain_ContainsLFXSignature(t *testing.T) {
	n := baseNotification()
	out := renderProjectAddedPlain(n)
	if !strings.Contains(out, "The LFX Team") {
		t.Error("plain text output missing LFX Team signature")
	}
}
