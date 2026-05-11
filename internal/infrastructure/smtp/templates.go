// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package smtp

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
)

var projectAddedHTMLTmpl = template.Must(template.New("project-added-html").Parse(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family:sans-serif;color:#333;max-width:600px;margin:0 auto;padding:24px">
  <img src="https://lfx.linuxfoundation.org/images/lfx-logo.svg" alt="LFX" height="32" style="margin-bottom:24px">
  <h2 style="margin-bottom:8px">You&#39;ve been added to {{.ProjectName}}</h2>
  <p>Hello {{.RecipientName}},</p>
  {{if .InviterName}}
  <p><strong>{{.InviterName}}</strong> has added you to <strong>{{.ProjectName}}</strong> with <strong>{{.Role}}</strong> access.</p>
  {{else}}
  <p>You have been added to <strong>{{.ProjectName}}</strong> with <strong>{{.Role}}</strong> access.</p>
  {{end}}
  <p style="margin-top:32px">
    <a href="{{.DeepLinkURL}}"
       style="background:#0066cc;color:#fff;padding:12px 24px;border-radius:4px;text-decoration:none;font-weight:bold">
      View Project
    </a>
  </p>
  <p style="margin-top:32px;font-size:12px;color:#888">
    If you did not expect this email, you can safely ignore it.
  </p>
</body>
</html>`))

func renderProjectAddedHTML(n *model.ProjectAddedNotification) string {
	var buf bytes.Buffer
	if err := projectAddedHTMLTmpl.Execute(&buf, n); err != nil {
		// Fall back to plain text indication — template execution should never fail
		// for a well-formed notification struct.
		return fmt.Sprintf("<p>You have been added to %s.</p>", template.HTMLEscapeString(n.ProjectName))
	}
	return buf.String()
}

func renderProjectAddedPlain(n *model.ProjectAddedNotification) string {
	inviterLine := ""
	if n.InviterName != "" {
		inviterLine = fmt.Sprintf("%s has added you to ", n.InviterName)
	} else {
		inviterLine = "You have been added to "
	}

	return fmt.Sprintf(
		"Hello %s,\n\n"+
			"%s%s with %s access.\n\n"+
			"Sign in to view the project:\n%s\n\n"+
			"If you did not expect this, you can safely ignore this email.\n\n"+
			"— The LFX Team",
		n.RecipientName,
		inviterLine,
		n.ProjectName,
		n.Role,
		n.DeepLinkURL,
	)
}
