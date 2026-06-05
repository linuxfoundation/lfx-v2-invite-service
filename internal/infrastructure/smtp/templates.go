// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package smtp

import (
	"bytes"
	_ "embed"
	"fmt"
	htmltmpl "html/template"
	"strings"
	texttmpl "text/template"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
)

//go:embed templates/invite_subject.gotemplate
var subjectTmplSrc string

//go:embed templates/invite_body.gohtml
var htmlTmplSrc string

//go:embed templates/invite_text.gotemplate
var plainTmplSrc string

var (
	subjectTmpl = texttmpl.Must(texttmpl.New("invite-subject").Parse(subjectTmplSrc))
	htmlTmpl    = htmltmpl.Must(htmltmpl.New("invite-body").Parse(htmlTmplSrc))
	plainTmpl   = texttmpl.Must(texttmpl.New("invite-text").Parse(plainTmplSrc))
)

// inviteEmailData is the template execution context built from a SendInviteRequest.
type inviteEmailData struct {
	RecipientFirstName string
	InviterFirstName   string
	InviterFullName    string
	ResourceName       string
	ResourceType       string
	Role               string
	ReturnURL          string
	OrgName            string
	HasInviter         bool
}

func buildTemplateData(req *model.SendInviteRequest) inviteEmailData {
	orgName := req.OrgName
	if orgName == "" {
		orgName = "LFX"
	}
	resourceType := req.ResolvedResourceType()
	if resourceType == "" {
		resourceType = "resource"
	}
	inviterName := req.ResolvedInviterName()
	return inviteEmailData{
		RecipientFirstName: firstName(req.ResolvedRecipientName()),
		InviterFirstName:   firstName(inviterName),
		InviterFullName:    inviterName,
		ResourceName:       req.ResolvedResourceName(),
		ResourceType:       resourceType,
		Role:               req.Role,
		ReturnURL:          req.ReturnURL,
		OrgName:            orgName,
		HasInviter:         inviterName != "",
	}
}

// firstName returns the first word of a full name, or the whole string if no space.
func firstName(fullName string) string {
	if idx := strings.Index(fullName, " "); idx > 0 {
		return fullName[:idx]
	}
	return fullName
}

// sanitizeSingleLine strips CR, LF, and NUL bytes and caps length to guard against
// email header injection when the result is used as a mail Subject.
func sanitizeSingleLine(s string) string {
	s = strings.NewReplacer("\r", " ", "\n", " ", "\x00", "").Replace(s)
	if len(s) > 256 {
		s = s[:256]
	}
	return s
}

// InviteEmailSubject renders the email subject line for an invite request.
func InviteEmailSubject(req *model.SendInviteRequest) string {
	data := buildTemplateData(req)
	var buf bytes.Buffer
	if err := subjectTmpl.Execute(&buf, data); err != nil {
		resourceType := req.ResolvedResourceType()
		if resourceType == "" {
			resourceType = "resource"
		}
		if inviterName := req.ResolvedInviterName(); inviterName != "" {
			return sanitizeSingleLine(fmt.Sprintf("%s invited you to join %s %s", firstName(inviterName), req.ResolvedResourceName(), resourceType))
		}
		return sanitizeSingleLine(fmt.Sprintf("You've been invited to join %s %s", req.ResolvedResourceName(), resourceType))
	}
	return sanitizeSingleLine(buf.String())
}

// RenderInviteHTML renders the HTML body for an invite notification.
func RenderInviteHTML(req *model.SendInviteRequest) string {
	data := buildTemplateData(req)
	var buf bytes.Buffer
	if err := htmlTmpl.Execute(&buf, data); err != nil {
		resourceType := req.ResolvedResourceType()
		if resourceType == "" {
			resourceType = "resource"
		}
		return fmt.Sprintf("<p>You have been invited to join the %s %s.</p>", htmltmpl.HTMLEscapeString(req.ResolvedResourceName()), htmltmpl.HTMLEscapeString(resourceType))
	}
	return buf.String()
}

// RenderInvitePlain renders the plain-text body for an invite notification.
func RenderInvitePlain(req *model.SendInviteRequest) string {
	data := buildTemplateData(req)
	var buf bytes.Buffer
	if err := plainTmpl.Execute(&buf, data); err != nil {
		resourceType := req.ResolvedResourceType()
		if resourceType == "" {
			resourceType = "resource"
		}
		return fmt.Sprintf("You have been invited to join the %s %s.\n\n%s", req.ResolvedResourceName(), resourceType, req.ReturnURL)
	}
	return buf.String()
}
