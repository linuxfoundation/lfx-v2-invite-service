// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package api contains the public contract types and NATS subjects that
// resource services (project-service, committee-service, etc.) use to
// interact with the invite service. These are the only exported types intended
// for inter-service use; all other types remain internal.
package api

// Subjects consumed by the invite service.
const (
	// SendInviteSubject is published by resource services when a non-LFID user is
	// added to a resource. The invite service consumes this from the invite-requests
	// JetStream stream, renders the email template, and forwards to the email service.
	SendInviteSubject = "lfx.invite-service.send_invite"
)

// Subjects published by the invite service.
const (
	// InviteCreatedSubject is published when the invite service issues an invite token.
	InviteCreatedSubject = "lfx.invite-service.invite.created"
	// InviteAcceptedSubject is published when an invited user accepts their invite.
	InviteAcceptedSubject = "lfx.invite-service.invite.accepted"
	// InviteRevokedSubject is published when an invite is revoked.
	InviteRevokedSubject = "lfx.invite-service.invite.revoked"
)

// InviteRole represents the access level to communicate to an invited user.
type InviteRole string

const (
	// InviteRoleManage maps to the writers/meeting-coordinators permission set.
	InviteRoleManage InviteRole = "Manage"
	// InviteRoleView maps to the auditors permission set.
	InviteRoleView InviteRole = "View"
)

// InviteCreatedEvent is published on InviteCreatedSubject after the invite
// service issues an invite. Resource services subscribe to this subject to
// persist the invite UUID alongside their own invite record.
type InviteCreatedEvent struct {
	InviteUID      string `json:"invite_uid"`
	ResourceUID    string `json:"resource_uid"`
	RecipientEmail string `json:"recipient_email"`
	Role           string `json:"role"`
	ExpiresAt      int64  `json:"expires_at"` // Unix timestamp
}

// SendInviteRequest is the NATS payload published on SendInviteSubject by
// resource services to request that the invite service sends an invite email
// to a user who does not yet have an LFID.
type SendInviteRequest struct {
	RecipientEmail string `json:"recipient_email"`
	RecipientName  string `json:"recipient_name"`
	InviterName    string `json:"inviter_name,omitempty"`
	ResourceUID    string `json:"resource_uid"`
	ResourceName   string `json:"resource_name"`
	Role           string `json:"role"`
	ReturnURL      string `json:"return_url,omitempty"`
	// ResourceType is the kind of resource the recipient is being invited to
	// (e.g. "project", "group", "meeting"). Used in the invite email body.
	// Defaults to "resource" when empty.
	ResourceType string `json:"resource_type,omitempty"`
	// OrgName is the foundation or project name used in the email signature
	// ("The X Team"). Defaults to "LFX" when empty.
	OrgName string `json:"org_name,omitempty"`
}
