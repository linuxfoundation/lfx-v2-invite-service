// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// InviteEmailPayload carries all resolved, pre-validated data required to render
// and send an invite email. Every field is populated by the service layer before
// crossing the EmailSender seam, so adapters receive unambiguous, ready-to-use
// values without needing to resolve deprecated scalars or reinterpret overloaded fields.
//
// InviteLink is always the signed JWT invite URL — never the raw destination URL.
// The destination URL (stored in the KV record as ReturnURL) is not part of this
// payload; the email adapter has no need for it.
type InviteEmailPayload struct {
	// RecipientEmail is the canonical email address, the output of mail.ParseAddress.
	RecipientEmail string
	// RecipientName is the resolved display name of the recipient.
	RecipientName string
	// InviterName is the resolved display name of the inviter; empty when no inviter.
	InviterName string
	// ResourceName is the resolved resource display name.
	ResourceName string
	// ResourceType is the resolved resource type (e.g. "project", "committee").
	ResourceType string
	// ResourceUID is the resolved resource UID, used for logging inside the adapter.
	ResourceUID string
	// Role is the trimmed, non-empty role string.
	Role string
	// OrgName is the organization name. Adapters should default to "LFX" when empty.
	OrgName string
	// InviteLink is the signed JWT invite URL used as the call-to-action in the email.
	InviteLink string
	// RecipientHasAccount indicates whether the recipient already has an LFX account.
	// When true the template renders the existing-user variant (no "create account" CTA).
	RecipientHasAccount bool
}
