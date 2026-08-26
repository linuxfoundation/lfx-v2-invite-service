// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package smtp

import (
	"bytes"
	_ "embed"
	"fmt"
	htmltmpl "html/template"
	"io"
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

// inviteEmailData is the template execution context.
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

func buildTemplateData(payload model.InviteEmailPayload) inviteEmailData {
	orgName := payload.OrgName
	if orgName == "" {
		orgName = "LFX"
	}
	return inviteEmailData{
		RecipientFirstName: firstName(payload.RecipientName),
		InviterFirstName:   firstName(payload.InviterName),
		InviterFullName:    payload.InviterName,
		ResourceName:       payload.ResourceName,
		ResourceType:       payload.ResourceType,
		Role:               payload.Role,
		ReturnURL:          payload.InviteLink,
		OrgName:            orgName,
		HasInviter:         payload.InviterName != "",
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

func sanitizedResourceSuffix(resourceType string) string {
	if resourceType == "" {
		return ""
	}
	return " " + sanitizeSingleLine(resourceType)
}

// fallbackInviteSubject returns a minimal subject when template execution fails.
// Intentionally generic so it does not duplicate full template copy.
func fallbackInviteSubject(data inviteEmailData) string {
	suffix := sanitizedResourceSuffix(data.ResourceType)
	name := sanitizeSingleLine(data.ResourceName)
	if data.HasInviter {
		return sanitizeSingleLine(fmt.Sprintf("%s invited you to join %s%s",
			sanitizeSingleLine(data.InviterFirstName), name, suffix))
	}
	return sanitizeSingleLine(fmt.Sprintf("You've been invited to join %s%s", name, suffix))
}

// fallbackInviteHTML returns minimal, safely-escaped HTML when template execution fails.
func fallbackInviteHTML(data inviteEmailData) string {
	suffix := ""
	if data.ResourceType != "" {
		suffix = " " + htmltmpl.HTMLEscapeString(data.ResourceType)
	}
	return fmt.Sprintf("<p>You have been invited to join %s%s.</p>",
		htmltmpl.HTMLEscapeString(data.ResourceName), suffix)
}

// fallbackInvitePlain returns minimal plain text when template execution fails.
func fallbackInvitePlain(data inviteEmailData) string {
	suffix := sanitizedResourceSuffix(data.ResourceType)
	return fmt.Sprintf("You have been invited to join %s%s.\n\n%s",
		sanitizeSingleLine(data.ResourceName), suffix, sanitizeSingleLine(data.ReturnURL))
}

// RenderedInviteEmail holds the rendered subject, HTML body, and plain-text
// body for an invite notification email.
type RenderedInviteEmail struct {
	Subject string
	HTML    string
	Plain   string
}

// templateExecutor is satisfied by both html/template.Template and
// text/template.Template, letting renderPart work with either.
type templateExecutor interface {
	Execute(wr io.Writer, data any) error
}

// renderPart executes tmpl into a buffer and returns the result. If execution
// fails it returns fallback(data) instead.
func renderPart(tmpl templateExecutor, data inviteEmailData, fallback func(inviteEmailData) string) string {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fallback(data)
	}
	return buf.String()
}

// RenderInviteEmail renders all three parts of an invite email in a single
// call. buildTemplateData is invoked once, keeping the rendering cohesive and
// the caller interface small.
func RenderInviteEmail(payload model.InviteEmailPayload) RenderedInviteEmail {
	data := buildTemplateData(payload)
	return RenderedInviteEmail{
		Subject: sanitizeSingleLine(renderPart(subjectTmpl, data, fallbackInviteSubject)),
		HTML:    renderPart(htmlTmpl, data, fallbackInviteHTML),
		Plain:   renderPart(plainTmpl, data, fallbackInvitePlain),
	}
}
