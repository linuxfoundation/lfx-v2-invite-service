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

//go:embed invite_subject.gotemplate
var subjectTmplSrc string

//go:embed invite_body.gohtml
var htmlTmplSrc string

//go:embed invite_text.gotemplate
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
	Role               string
	DeepLinkURL        string
	OrgName            string
	HasInviter         bool
}

func buildTemplateData(req *model.SendInviteRequest) inviteEmailData {
	orgName := req.OrgName
	if orgName == "" {
		orgName = "LFX"
	}
	return inviteEmailData{
		RecipientFirstName: firstName(req.RecipientName),
		InviterFirstName:   firstName(req.InviterName),
		InviterFullName:    req.InviterName,
		ResourceName:       req.ResourceName,
		Role:               req.Role,
		DeepLinkURL:        req.DeepLinkURL,
		OrgName:            orgName,
		HasInviter:         req.InviterName != "",
	}
}

// firstName returns the first word of a full name, or the whole string if no space.
func firstName(fullName string) string {
	if idx := strings.Index(fullName, " "); idx > 0 {
		return fullName[:idx]
	}
	return fullName
}

// InviteEmailSubject renders the email subject line for an invite request.
func InviteEmailSubject(req *model.SendInviteRequest) string {
	data := buildTemplateData(req)
	var buf bytes.Buffer
	if err := subjectTmpl.Execute(&buf, data); err != nil {
		if req.InviterName != "" {
			return fmt.Sprintf("%s invited you to join %s", firstName(req.InviterName), req.ResourceName)
		}
		return fmt.Sprintf("You've been invited to join %s", req.ResourceName)
	}
	return buf.String()
}

// RenderInviteHTML renders the HTML body for an invite notification.
func RenderInviteHTML(req *model.SendInviteRequest) string {
	data := buildTemplateData(req)
	var buf bytes.Buffer
	if err := htmlTmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("<p>You have been invited to join %s.</p>", htmltmpl.HTMLEscapeString(req.ResourceName))
	}
	return buf.String()
}

// RenderInvitePlain renders the plain-text body for an invite notification.
func RenderInvitePlain(req *model.SendInviteRequest) string {
	data := buildTemplateData(req)
	var buf bytes.Buffer
	if err := plainTmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("You have been invited to join %s.\n\n%s", req.ResourceName, req.DeepLinkURL)
	}
	return buf.String()
}
