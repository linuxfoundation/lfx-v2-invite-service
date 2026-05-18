// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package smtp

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
)

var inviteHTMLTmpl = template.Must(template.New("invite-html").Parse(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family:sans-serif;color:#333;max-width:600px;margin:0 auto;padding:24px">
  <img src="https://lfx.linuxfoundation.org/images/lfx-logo.svg" alt="LFX" height="32" style="margin-bottom:24px">
  <h2 style="margin-bottom:8px">You&#39;ve been added to {{.ResourceName}}</h2>
  <p>Hello {{.RecipientName}},</p>
  {{if .InviterName}}
  <p><strong>{{.InviterName}}</strong> has added you to <strong>{{.ResourceName}}</strong> with <strong>{{.Role}}</strong> access.</p>
  {{else}}
  <p>You have been added to <strong>{{.ResourceName}}</strong> with <strong>{{.Role}}</strong> access.</p>
  {{end}}
  <p style="margin-top:32px">
    <a href="{{.DeepLinkURL}}"
       style="background:#0066cc;color:#fff;padding:12px 24px;border-radius:4px;text-decoration:none;font-weight:bold">
      Get Started
    </a>
  </p>
  <p style="margin-top:32px;font-size:12px;color:#888">
    If you did not expect this email, you can safely ignore it.
  </p>
</body>
</html>`))

// RenderInviteHTML renders the HTML body for an invite notification.
func RenderInviteHTML(req *model.SendInviteRequest) string {
	var buf bytes.Buffer
	if err := inviteHTMLTmpl.Execute(&buf, req); err != nil {
		return fmt.Sprintf("<p>You have been added to %s.</p>", template.HTMLEscapeString(req.ResourceName))
	}
	return buf.String()
}

// RenderInvitePlain renders the plain-text body for an invite notification.
func RenderInvitePlain(req *model.SendInviteRequest) string {
	inviterLine := ""
	if req.InviterName != "" {
		inviterLine = fmt.Sprintf("%s has added you to ", req.InviterName)
	} else {
		inviterLine = "You have been added to "
	}

	return fmt.Sprintf(
		"Hello %s,\n\n"+
			"%s%s with %s access.\n\n"+
			"Get started:\n%s\n\n"+
			"If you did not expect this, you can safely ignore this email.\n\n"+
			"— The LFX Team",
		req.RecipientName,
		inviterLine,
		req.ResourceName,
		req.Role,
		req.DeepLinkURL,
	)
}
