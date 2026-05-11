// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package smtp

import (
	"fmt"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
)

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

func renderProjectAddedHTML(n *model.ProjectAddedNotification) string {
	inviterLine := ""
	if n.InviterName != "" {
		inviterLine = fmt.Sprintf("<strong>%s</strong> has added you to ", n.InviterName)
	} else {
		inviterLine = "You have been added to "
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family:sans-serif;color:#333;max-width:600px;margin:0 auto;padding:24px">
  <img src="https://lfx.linuxfoundation.org/images/lfx-logo.svg" alt="LFX" height="32" style="margin-bottom:24px">
  <h2 style="margin-bottom:8px">You've been added to %s</h2>
  <p>Hello %s,</p>
  <p>%s<strong>%s</strong> with <strong>%s</strong> access.</p>
  <p style="margin-top:32px">
    <a href="%s"
       style="background:#0066cc;color:#fff;padding:12px 24px;border-radius:4px;text-decoration:none;font-weight:bold">
      View Project
    </a>
  </p>
  <p style="margin-top:32px;font-size:12px;color:#888">
    If you did not expect this email, you can safely ignore it.
  </p>
</body>
</html>`,
		n.ProjectName,
		n.RecipientName,
		inviterLine,
		n.ProjectName,
		n.Role,
		n.DeepLinkURL,
	)
}
