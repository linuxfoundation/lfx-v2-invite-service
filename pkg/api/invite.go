// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package api contains the public contract types and NATS subjects that
// resource services (project-service, committee-service, etc.) use to
// interact with the invite service. These are the only exported types intended
// for inter-service use; all other types remain internal.
package api

import "time"

// Subjects consumed by the invite service.
const (
	// SendInviteSubject is used for NATS request/reply by resource services when a
	// non-LFID user is added to a resource. The invite service renders the email
	// template, forwards to the email service, and replies with SendInviteResponse.
	SendInviteSubject = "lfx.invite-service.send_invite"

	// GetInviteSubject is used for NATS request/reply to look up an invite record by UID.
	// Callers send GetInviteRequest; the invite service replies with GetInviteResponse.
	GetInviteSubject = "lfx.invite-service.get_invite"

	// GetInvitesByEmailSubject is used for NATS request/reply to look up all invite
	// records for a given email address. Callers send GetInvitesByEmailRequest; the
	// invite service replies with GetInvitesByEmailResponse.
	GetInvitesByEmailSubject = "lfx.invite-service.get_invites_by_email"
)

// Subjects published by the invite service.
const (
	// InviteCreatedSubject is published when the invite service issues an invite token.
	InviteCreatedSubject = "lfx.invite-service.invite.created"
	// InviteAcceptedSubject is published by the LFX self-serve web app once a user
	// completes the invite acceptance flow (JWT validation + login). Backend services
	// subscribe to this subject to grant access and clean up pending invite state.
	// Note: this subject intentionally uses the "lfx.invite.*" namespace rather than
	// "lfx.invite-service.invite.*" because the publisher is the self-serve web app,
	// not the invite service. The constant lives here as the authoritative contract location.
	InviteAcceptedSubject = "lfx.invite.accepted"
	// InviteServiceAcceptedSubject is published by the invite service after it has
	// processed an InviteAcceptedSubject event and updated its KV record. It carries
	// enriched context (recipient, inviter, resource, role) that the original
	// self-serve event does not include, so downstream services can subscribe here
	// instead of performing their own invite lookups.
	//
	// TODO(reprocessing): the upstream lfx.invite.accepted subscription currently uses
	// core NATS QueueSubscribe which has no ACK/NAK — a publish failure here is
	// best-effort and logged but not retried. Switch the upstream consumer to a
	// JetStream durable consumer so a publish failure can NAK the message and trigger
	// redelivery.
	InviteServiceAcceptedSubject = "lfx.invite-service.invite_accepted"
	// InviteRevokedSubject is published when an invite is revoked.
	InviteRevokedSubject = "lfx.invite-service.invite.revoked"
)

// InviteStatus represents the lifecycle state of an invite record.
type InviteStatus string

const (
	// InviteStatusPending means the invite has been sent but not yet accepted.
	InviteStatusPending InviteStatus = "pending"
	// InviteStatusAccepted means the invited user has completed the acceptance flow.
	InviteStatusAccepted InviteStatus = "accepted"
)

// InviteRole represents the access level to communicate to an invited user.
type InviteRole string

const (
	// InviteRoleManage maps to the writers/meeting-coordinators permission set.
	InviteRoleManage InviteRole = "Manage"
	// InviteRoleView maps to the auditors permission set.
	InviteRoleView InviteRole = "View"
	// InviteRoleMember represents a plain committee/group membership with no
	// elevated write or audit access.
	InviteRoleMember InviteRole = "Member"
)

// Inviter holds structured identity for the person who sent the invite.
type Inviter struct {
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
}

// Recipient holds structured identity for the person being invited.
type Recipient struct {
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
}

// Resource holds the structured representation of the resource the invite is for.
type Resource struct {
	UID  string `json:"uid,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

// InviteData holds the invite metadata returned on a successful send_invite reply.
type InviteData struct {
	UID       string    `json:"uid"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Invite is the read-API view of a stored invite record, returned by get_invite
// and get_invites_by_email subjects.
type Invite struct {
	UID            string       `json:"uid"`
	Status         InviteStatus `json:"status"`
	Recipient      Recipient    `json:"recipient"`
	Inviter        Inviter      `json:"inviter"`
	Resource       Resource     `json:"resource"`
	Role           string       `json:"role"`
	OrgName        string       `json:"org_name,omitempty"`
	ReturnURL      string       `json:"return_url,omitempty"`
	ExpirationDays int          `json:"expiration_days,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	ExpiresAt      time.Time    `json:"expires_at"`
	AcceptedAt     *time.Time   `json:"accepted_at,omitempty"`
	AcceptedBy     string       `json:"accepted_by,omitempty"`
}

// SendInviteResponse is the reply payload returned by the invite service on
// SendInviteSubject. On success the InviteData fields are inlined at the top level
// (uid, email, expires_at). On failure only Error is set.
type SendInviteResponse struct {
	*InviteData
	Error string `json:"error,omitempty"`
}

// SendInviteRequest is the NATS payload published on SendInviteSubject by
// resource services to request that the invite service sends an invite email
// to a user who does not yet have an LFID.
//
// Preferred: populate the structured Recipient, Inviter, and Resource objects.
// Deprecated fields (RecipientEmail, RecipientName, InviterName, ResourceUID,
// ResourceName, ResourceType) are retained for backward-compatibility; the
// invite service prefers the structured objects and falls back to the scalar
// fields when the objects are absent.
type SendInviteRequest struct {
	// Structured fields (preferred).
	Recipient *Recipient `json:"recipient,omitempty"`
	Inviter   *Inviter   `json:"inviter,omitempty"`
	Resource  *Resource  `json:"resource,omitempty"`

	// Deprecated: use Recipient.Email instead.
	RecipientEmail string `json:"recipient_email,omitempty"`
	// Deprecated: use Recipient.Name instead.
	RecipientName string `json:"recipient_name,omitempty"`
	// Deprecated: use Inviter.Name instead.
	InviterName string `json:"inviter_name,omitempty"`
	// Deprecated: use Resource.UID instead.
	ResourceUID string `json:"resource_uid,omitempty"`
	// Deprecated: use Resource.Name instead.
	ResourceName string `json:"resource_name,omitempty"`
	// Deprecated: use Resource.Type instead.
	ResourceType string `json:"resource_type,omitempty"`

	Role      string `json:"role"`
	ReturnURL string `json:"return_url,omitempty"`
	// OrgName is the foundation or project name used in the email signature
	// ("The X Team"). Defaults to "LFX" when empty.
	OrgName string `json:"org_name,omitempty"`
	// ExpirationDays is the number of days the invite token should be valid.
	// If 0 or omitted, defaults to 30 days. Maximum is 90 days.
	ExpirationDays int `json:"expiration_days,omitempty"`
}

// InviteAcceptedEvent is the payload published on InviteAcceptedSubject by the
// LFX self-serve web app once a user completes the invite acceptance flow.
type InviteAcceptedEvent struct {
	InviteUID string `json:"invite_uid"`
	Username  string `json:"username"`
}

// GetInviteRequest is the payload for GetInviteSubject.
type GetInviteRequest struct {
	UID string `json:"uid"`
}

// GetInviteResponse is the reply payload for GetInviteSubject.
// On success all Invite fields are inlined at the top level. On failure only Error is set.
type GetInviteResponse struct {
	*Invite
	Error string `json:"error,omitempty"`
}

// GetInvitesByEmailRequest is the payload for GetInvitesByEmailSubject.
type GetInvitesByEmailRequest struct {
	Email string `json:"email"`
}

// GetInvitesByEmailResponse is the error reply payload for GetInvitesByEmailSubject.
// On success the reply is a bare JSON array of Invite objects ([]Invite).
// On failure only Error is set.
type GetInvitesByEmailResponse struct {
	Error string `json:"error,omitempty"`
}

// InviteServiceAcceptedEvent is published on InviteServiceAcceptedSubject by the
// invite service after it has processed an acceptance and updated its KV record.
// It embeds the full Invite so subscribers receive enriched context without needing
// a separate get_invite lookup.
type InviteServiceAcceptedEvent struct {
	Invite
}
