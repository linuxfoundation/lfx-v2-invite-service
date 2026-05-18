// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package api contains the public contract types and NATS subjects that
// resource services (project-service, committee-service, etc.) use to
// interact with the invite service. These are the only exported types intended
// for inter-service use; all other types remain internal.
package api

// SendInviteSubject is the NATS subject resource services publish to when a
// non-LFID user is added to a resource. The invite service consumes this
// subject from the invite-requests JetStream stream, renders the invite email
// template, and forwards to the email service for delivery.
const SendInviteSubject = "lfx.invite-service.send_invite"

// InviteRole represents the access level to communicate to an invited user.
type InviteRole string

const (
	// InviteRoleManage maps to the writers/meeting-coordinators permission set.
	InviteRoleManage InviteRole = "Manage"
	// InviteRoleView maps to the auditors permission set.
	InviteRoleView InviteRole = "View"
)

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
	DeepLinkURL    string `json:"deep_link_url,omitempty"`
}
