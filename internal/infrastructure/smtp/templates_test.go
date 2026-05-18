// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package smtp

import (
	"strings"
	"testing"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
)

func baseRequest() *model.SendInviteRequest {
	return &model.SendInviteRequest{
		RecipientName:  "Alice",
		RecipientEmail: "alice@example.com",
		ResourceUID:    "res-123",
		ResourceName:   "My Project",
		Role:           string(model.RoleManage),
		DeepLinkURL:    "https://lfx.example.com/resources/res-123",
	}
}

func TestRenderInviteHTML_ContainsResourceName(t *testing.T) {
	req := baseRequest()
	out := RenderInviteHTML(req)
	if !strings.Contains(out, req.ResourceName) {
		t.Errorf("HTML output missing resource name %q", req.ResourceName)
	}
}

func TestRenderInviteHTML_ContainsRecipientName(t *testing.T) {
	req := baseRequest()
	out := RenderInviteHTML(req)
	if !strings.Contains(out, req.RecipientName) {
		t.Errorf("HTML output missing recipient name %q", req.RecipientName)
	}
}

func TestRenderInviteHTML_ContainsDeepLink(t *testing.T) {
	req := baseRequest()
	out := RenderInviteHTML(req)
	if !strings.Contains(out, req.DeepLinkURL) {
		t.Errorf("HTML output missing deep link %q", req.DeepLinkURL)
	}
}

func TestRenderInviteHTML_WithInviter(t *testing.T) {
	req := baseRequest()
	req.InviterName = "Bob"
	out := RenderInviteHTML(req)
	if !strings.Contains(out, "Bob") {
		t.Errorf("HTML output missing inviter name %q", "Bob")
	}
}

func TestRenderInviteHTML_WithoutInviter(t *testing.T) {
	req := baseRequest()
	req.InviterName = ""
	out := RenderInviteHTML(req)
	if strings.Contains(out, "has added you") && strings.Contains(out, " <strong></strong>") {
		t.Error("HTML output should not render empty inviter name")
	}
	if !strings.Contains(out, "You have been added") {
		t.Error("HTML output missing generic 'You have been added' text when no inviter")
	}
}

func TestRenderInviteHTML_EscapesSpecialCharacters(t *testing.T) {
	req := baseRequest()
	req.ResourceName = "<script>alert('xss')</script>"
	out := RenderInviteHTML(req)
	if strings.Contains(out, "<script>") {
		t.Error("HTML output must escape resource name containing script tags")
	}
}

func TestRenderInvitePlain_ContainsResourceName(t *testing.T) {
	req := baseRequest()
	out := RenderInvitePlain(req)
	if !strings.Contains(out, req.ResourceName) {
		t.Errorf("plain text output missing resource name %q", req.ResourceName)
	}
}

func TestRenderInvitePlain_ContainsDeepLink(t *testing.T) {
	req := baseRequest()
	out := RenderInvitePlain(req)
	if !strings.Contains(out, req.DeepLinkURL) {
		t.Errorf("plain text output missing deep link %q", req.DeepLinkURL)
	}
}

func TestRenderInvitePlain_ContainsRecipientName(t *testing.T) {
	req := baseRequest()
	out := RenderInvitePlain(req)
	if !strings.Contains(out, req.RecipientName) {
		t.Errorf("plain text output missing recipient name %q", req.RecipientName)
	}
}

func TestRenderInvitePlain_WithInviter(t *testing.T) {
	req := baseRequest()
	req.InviterName = "Bob"
	out := RenderInvitePlain(req)
	if !strings.Contains(out, "Bob has added you") {
		t.Errorf("plain text output missing inviter line, got:\n%s", out)
	}
}

func TestRenderInvitePlain_WithoutInviter(t *testing.T) {
	req := baseRequest()
	req.InviterName = ""
	out := RenderInvitePlain(req)
	if !strings.Contains(out, "You have been added to") {
		t.Errorf("plain text output missing generic added line, got:\n%s", out)
	}
}

func TestRenderInvitePlain_ContainsRole(t *testing.T) {
	req := baseRequest()
	out := RenderInvitePlain(req)
	if !strings.Contains(out, req.Role) {
		t.Errorf("plain text output missing role %q", req.Role)
	}
}

func TestRenderInvitePlain_ContainsLFXSignature(t *testing.T) {
	req := baseRequest()
	out := RenderInvitePlain(req)
	if !strings.Contains(out, "The LFX Team") {
		t.Error("plain text output missing LFX Team signature")
	}
}
