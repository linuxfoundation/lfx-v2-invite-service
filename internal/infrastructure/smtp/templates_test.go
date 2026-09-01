// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package smtp

import (
	"strings"
	"testing"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
)

func basePayload() model.InviteEmailPayload {
	return model.InviteEmailPayload{
		RecipientEmail: "alice@example.com",
		RecipientName:  "Alice Smith",
		InviterName:    "Bob Jones",
		ResourceName:   "My Project",
		ResourceType:   "",
		ResourceUID:    "res-123",
		Role:           string(model.RoleManage),
		OrgName:        "Linux Foundation",
		InviteLink:     "https://lfx.example.com/resources/res-123",
	}
}

// --- Subject ---

func TestRenderInviteEmail_Subject_WithInviter(t *testing.T) {
	p := basePayload()
	subject := RenderInviteEmail(p).Subject
	if !strings.Contains(subject, "Bob") {
		t.Errorf("subject missing inviter first name, got %q", subject)
	}
	if !strings.Contains(subject, p.ResourceName) {
		t.Errorf("subject missing resource name, got %q", subject)
	}
}

func TestRenderInviteEmail_Subject_WithoutInviter(t *testing.T) {
	p := basePayload()
	p.InviterName = ""
	subject := RenderInviteEmail(p).Subject
	if strings.Contains(subject, "Bob") {
		t.Errorf("subject should not contain inviter name when absent, got %q", subject)
	}
	if !strings.Contains(subject, p.ResourceName) {
		t.Errorf("subject missing resource name, got %q", subject)
	}
}

func TestRenderInviteEmail_Subject_UsesFirstNameOnly(t *testing.T) {
	p := basePayload()
	subject := RenderInviteEmail(p).Subject
	if strings.Contains(subject, "Jones") {
		t.Errorf("subject should use first name only, not full name; got %q", subject)
	}
}

// --- HTML body ---

func TestRenderInviteEmail_HTML_ContainsRecipientFirstName(t *testing.T) {
	p := basePayload()
	out := RenderInviteEmail(p).HTML
	if !strings.Contains(out, "Alice") {
		t.Errorf("HTML missing recipient first name, got output length %d", len(out))
	}
	if strings.Contains(out, "Alice Smith") {
		t.Error("HTML should use first name only for greeting, not full name")
	}
}

func TestRenderInviteEmail_HTML_ContainsInviterFullName(t *testing.T) {
	p := basePayload()
	out := RenderInviteEmail(p).HTML
	if !strings.Contains(out, "Bob Jones") {
		t.Errorf("HTML missing inviter full name %q", "Bob Jones")
	}
}

func TestRenderInviteEmail_HTML_ContainsResourceName(t *testing.T) {
	p := basePayload()
	out := RenderInviteEmail(p).HTML
	if !strings.Contains(out, p.ResourceName) {
		t.Errorf("HTML missing resource name %q", p.ResourceName)
	}
}

func TestRenderInviteEmail_HTML_ContainsDeepLink(t *testing.T) {
	p := basePayload()
	out := RenderInviteEmail(p).HTML
	if !strings.Contains(out, p.InviteLink) {
		t.Errorf("HTML missing invite link %q", p.InviteLink)
	}
}

func TestRenderInviteEmail_HTML_ContainsOrgName(t *testing.T) {
	p := basePayload()
	out := RenderInviteEmail(p).HTML
	if !strings.Contains(out, "Linux Foundation") {
		t.Error("HTML missing org name in signature")
	}
}

func TestRenderInviteEmail_HTML_DefaultsOrgNameToLFX(t *testing.T) {
	p := basePayload()
	p.OrgName = ""
	out := RenderInviteEmail(p).HTML
	if !strings.Contains(out, "LFX Team") {
		t.Error("HTML should fall back to 'LFX Team' when OrgName is empty")
	}
}

func TestRenderInviteEmail_HTML_WithoutInviter(t *testing.T) {
	p := basePayload()
	p.InviterName = ""
	out := RenderInviteEmail(p).HTML
	if strings.Contains(out, "Bob") {
		t.Error("HTML should not contain inviter name when absent")
	}
	if !strings.Contains(out, "You have been invited") {
		t.Error("HTML missing generic invite text when no inviter")
	}
}

func TestRenderInviteEmail_HTML_ContainsCTA(t *testing.T) {
	p := basePayload()
	out := RenderInviteEmail(p).HTML
	if !strings.Contains(out, "Accept invitation") {
		t.Error("HTML missing CTA button text")
	}
}

func TestRenderInviteEmail_HTML_EscapesSpecialCharacters(t *testing.T) {
	p := basePayload()
	p.ResourceName = "<script>alert('xss')</script>"
	out := RenderInviteEmail(p).HTML
	if strings.Contains(out, "<script>") {
		t.Error("HTML must escape resource name containing script tags")
	}
}

func TestRenderInviteEmail_HTML_EmptyRecipientName_GreetingHasNoTrailingSpace(t *testing.T) {
	p := basePayload()
	p.RecipientName = ""
	out := RenderInviteEmail(p).HTML
	if strings.Contains(out, "Hi ,") {
		t.Error("HTML greeting should be 'Hi,' not 'Hi ,' when recipient name is empty")
	}
	if !strings.Contains(out, "Hi,") {
		t.Errorf("HTML should contain 'Hi,' greeting when recipient name is empty, got:\n%s", out)
	}
}

// --- Plain-text body ---

func TestRenderInviteEmail_Plain_ContainsRecipientFirstName(t *testing.T) {
	p := basePayload()
	out := RenderInviteEmail(p).Plain
	if !strings.Contains(out, "Alice") {
		t.Errorf("plain text missing recipient first name")
	}
}

func TestRenderInviteEmail_Plain_ContainsInviterFullName(t *testing.T) {
	p := basePayload()
	out := RenderInviteEmail(p).Plain
	if !strings.Contains(out, "Bob Jones") {
		t.Errorf("plain text missing inviter full name")
	}
}

func TestRenderInviteEmail_Plain_ContainsResourceName(t *testing.T) {
	p := basePayload()
	out := RenderInviteEmail(p).Plain
	if !strings.Contains(out, p.ResourceName) {
		t.Errorf("plain text missing resource name %q", p.ResourceName)
	}
}

func TestRenderInviteEmail_Plain_ContainsDeepLink(t *testing.T) {
	p := basePayload()
	out := RenderInviteEmail(p).Plain
	if !strings.Contains(out, p.InviteLink) {
		t.Errorf("plain text missing invite link %q", p.InviteLink)
	}
}

func TestRenderInviteEmail_Plain_ContainsRole(t *testing.T) {
	p := basePayload()
	out := RenderInviteEmail(p).Plain
	if !strings.Contains(out, p.Role) {
		t.Errorf("plain text missing role %q", p.Role)
	}
}

func TestRenderInviteEmail_Plain_WithoutInviter(t *testing.T) {
	p := basePayload()
	p.InviterName = ""
	out := RenderInviteEmail(p).Plain
	if strings.Contains(out, "Bob") {
		t.Error("plain text should not contain inviter name when absent")
	}
	if !strings.Contains(out, "You have been invited") {
		t.Errorf("plain text missing generic invite line, got:\n%s", out)
	}
}

func TestRenderInviteEmail_Plain_ContainsOrgTeamSignature(t *testing.T) {
	p := basePayload()
	out := RenderInviteEmail(p).Plain
	if !strings.Contains(out, "Linux Foundation Team") {
		t.Errorf("plain text missing org team signature, got:\n%s", out)
	}
}

func TestRenderInviteEmail_Plain_DefaultsOrgNameToLFX(t *testing.T) {
	p := basePayload()
	p.OrgName = ""
	out := RenderInviteEmail(p).Plain
	if !strings.Contains(out, "LFX Team") {
		t.Error("plain text should fall back to 'LFX Team' when OrgName is empty")
	}
}

func TestRenderInviteEmail_Plain_EmptyRecipientName_GreetingHasNoTrailingSpace(t *testing.T) {
	p := basePayload()
	p.RecipientName = ""
	out := RenderInviteEmail(p).Plain
	if strings.Contains(out, "Hi ,") {
		t.Error("plain text greeting should be 'Hi,' not 'Hi ,' when recipient name is empty")
	}
	if !strings.Contains(out, "Hi,") {
		t.Errorf("plain text should contain 'Hi,' greeting when recipient name is empty, got:\n%s", out)
	}
}

func TestRenderInviteEmail_Plain_ContainsSteps(t *testing.T) {
	p := basePayload()
	out := RenderInviteEmail(p).Plain
	if !strings.Contains(out, "1.") || !strings.Contains(out, "2.") || !strings.Contains(out, "3.") {
		t.Error("plain text missing numbered steps")
	}
}

// --- RecipientHasAccount template branching ---

func TestRenderInviteEmail_HTML_RecipientHasAccount(t *testing.T) {
	tests := []struct {
		name                string
		recipientHasAccount bool
		wantCTA             string
		wantNoSteps         bool
	}{
		{
			name:                "existing user: simple accept CTA, no create-account steps",
			recipientHasAccount: true,
			wantCTA:             "Accept invitation</a>",
			wantNoSteps:         true,
		},
		{
			name:                "new user: create-account CTA with setup steps",
			recipientHasAccount: false,
			wantCTA:             "Accept invitation &amp; create account</a>",
			wantNoSteps:         false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := basePayload()
			p.RecipientHasAccount = tc.recipientHasAccount
			html := RenderInviteEmail(p).HTML
			if !strings.Contains(html, tc.wantCTA) {
				t.Errorf("HTML missing expected CTA %q", tc.wantCTA)
			}
			hasSteps := strings.Contains(html, "Create your LFX account")
			if tc.wantNoSteps && hasSteps {
				t.Error("HTML must not contain create-account steps for existing user")
			}
			if !tc.wantNoSteps && !hasSteps {
				t.Error("HTML must contain create-account steps for new user")
			}
		})
	}
}

func TestRenderInviteEmail_Plain_RecipientHasAccount(t *testing.T) {
	tests := []struct {
		name                string
		recipientHasAccount bool
		wantCTA             string
		wantNoSteps         bool
	}{
		{
			name:                "existing user: simple accept CTA, no create-account steps",
			recipientHasAccount: true,
			wantCTA:             "Accept invitation:",
			wantNoSteps:         true,
		},
		{
			name:                "new user: create-account CTA with setup steps",
			recipientHasAccount: false,
			wantCTA:             "Accept invitation & create account:",
			wantNoSteps:         false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := basePayload()
			p.RecipientHasAccount = tc.recipientHasAccount
			plain := RenderInviteEmail(p).Plain
			if !strings.Contains(plain, tc.wantCTA) {
				t.Errorf("plain text missing expected CTA %q", tc.wantCTA)
			}
			hasSteps := strings.Contains(plain, "Create your LFX account")
			if tc.wantNoSteps && hasSteps {
				t.Error("plain text must not contain create-account steps for existing user")
			}
			if !tc.wantNoSteps && !hasSteps {
				t.Error("plain text must contain create-account steps for new user")
			}
		})
	}
}

// --- Fallback internals (white-box, same package) ---

func TestFallbackInviteSubject_WithInviter(t *testing.T) {
	subject := fallbackInviteSubject(inviteEmailData{
		InviterFirstName: "Bob",
		ResourceName:     "My Project",
		ResourceType:     "meeting",
		HasInviter:       true,
	})
	if !strings.Contains(subject, "Bob invited you to join My Project meeting") {
		t.Errorf("unexpected fallback subject: %q", subject)
	}
}

func TestFallbackInviteSubject_SanitizesResourceType(t *testing.T) {
	subject := fallbackInviteSubject(inviteEmailData{
		ResourceName: "My Project",
		ResourceType: "meet\r\ning",
	})
	if strings.Contains(subject, "\r") || strings.Contains(subject, "\n") {
		t.Errorf("fallback subject must sanitize resource type, got %q", subject)
	}
}

func TestFallbackInviteHTML_EscapesResourceName(t *testing.T) {
	out := fallbackInviteHTML(inviteEmailData{
		ResourceName: "<script>alert('xss')</script>",
		ResourceType: "meeting",
	})
	if strings.Contains(out, "<script>") {
		t.Error("fallback HTML must escape resource name")
	}
	if !strings.Contains(out, "meeting") {
		t.Errorf("fallback HTML missing resource type, got %q", out)
	}
}

func TestFallbackInvitePlain_SanitizesReturnURL(t *testing.T) {
	out := fallbackInvitePlain(inviteEmailData{
		ResourceName: "My Project",
		ReturnURL:    "https://lfx.example.com/invite\nInjected: header",
	})
	if strings.Contains(out, "\nInjected") {
		t.Errorf("fallback plain text must sanitize return URL, got %q", out)
	}
	if !strings.Contains(out, "https://lfx.example.com/invite") {
		t.Errorf("fallback plain text missing return URL, got %q", out)
	}
}
