// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import "time"

// InviteStatus represents the lifecycle state of an invite record.
type InviteStatus string

const (
	// InviteStatusPending means the invite has been sent but not yet accepted.
	InviteStatusPending InviteStatus = "pending"
	// InviteStatusAccepted means the invited user has completed the acceptance flow.
	InviteStatusAccepted InviteStatus = "accepted"
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

// InviteResource holds the structured representation of the resource the invite is for.
// Named InviteResource to avoid collision with other "Resource" types in the domain.
type InviteResource struct {
	UID  string `json:"uid,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

// InviteRecord is the invite service's persisted record of an invite, stored in
// the NATS JetStream KV bucket. It is keyed by the invite UUID (= JWT jti).
type InviteRecord struct {
	// UID is the invite UUID (= JWT jti). Primary KV key.
	UID    string       `json:"uid"`
	Status InviteStatus `json:"status"`

	Recipient Recipient      `json:"recipient"`
	Inviter   Inviter        `json:"inviter"`
	Resource  InviteResource `json:"resource"`

	Role    string `json:"role"`
	OrgName string `json:"org_name,omitempty"`
	// ReturnURL is the *destination* URL captured before the JWT is generated.
	// The signed JWT link is never stored.
	ReturnURL      string `json:"return_url,omitempty"`
	ExpirationDays int    `json:"expiration_days,omitempty"`

	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	// AcceptedBy is the LFID username of the user who accepted the invite.
	AcceptedBy string `json:"accepted_by,omitempty"`
}
