// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// SendInviteRequest is the internal domain representation of the invite payload.
// It mirrors pkg/api.SendInviteRequest, which is the public contract published
// by resource services over NATS. Keep both in sync when adding fields —
// the split is intentional: pkg/api is the inter-service contract, this package
// is the internal domain model owned by the invite service.
//
// Preferred: populate the structured Recipient, Inviter, and Resource objects.
// Deprecated scalar fields are retained for backward-compatibility.
type SendInviteRequest struct {
	// Structured fields (preferred).
	Recipient *Recipient      `json:"recipient,omitempty"`
	Inviter   *Inviter        `json:"inviter,omitempty"`
	Resource  *InviteResource `json:"resource,omitempty"`

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
	// ResourceType is the kind of resource (e.g. "project", "group", "meeting").
	// Defaults to "resource" when empty.
	ResourceType string `json:"resource_type,omitempty"`

	Role      string `json:"role"`
	ReturnURL string `json:"return_url,omitempty"`
	// OrgName is the foundation or project name used in the email signature
	// ("The X Team"). Defaults to "LFX" when empty.
	OrgName string `json:"org_name,omitempty"`
	// ExpirationDays is the number of days the invite token should be valid.
	// If 0 or omitted, defaults to 30 days. Maximum is 90 days.
	ExpirationDays int `json:"expiration_days,omitempty"`
	// CustomClaims are additional string claims to embed in the signed JWT token.
	// See pkg/api.SendInviteRequest.CustomClaims for the full contract.
	CustomClaims map[string]string `json:"custom_claims,omitempty"`
}

// ResolvedRecipientEmail returns the recipient email, preferring the structured
// Recipient object over the deprecated RecipientEmail scalar.
func (r *SendInviteRequest) ResolvedRecipientEmail() string {
	if r.Recipient != nil && r.Recipient.Email != "" {
		return r.Recipient.Email
	}
	return r.RecipientEmail
}

// ResolvedRecipientName returns the recipient name, preferring the structured
// Recipient object over the deprecated RecipientName scalar.
func (r *SendInviteRequest) ResolvedRecipientName() string {
	if r.Recipient != nil && r.Recipient.Name != "" {
		return r.Recipient.Name
	}
	return r.RecipientName
}

// ResolvedResourceUID returns the resource UID, preferring the structured
// Resource object over the deprecated ResourceUID scalar.
func (r *SendInviteRequest) ResolvedResourceUID() string {
	if r.Resource != nil && r.Resource.UID != "" {
		return r.Resource.UID
	}
	return r.ResourceUID
}

// ResolvedResourceName returns the resource name, preferring the structured
// Resource object over the deprecated ResourceName scalar.
func (r *SendInviteRequest) ResolvedResourceName() string {
	if r.Resource != nil && r.Resource.Name != "" {
		return r.Resource.Name
	}
	return r.ResourceName
}

// ResolvedResourceType returns the resource type, preferring the structured
// Resource object over the deprecated ResourceType scalar.
func (r *SendInviteRequest) ResolvedResourceType() string {
	if r.Resource != nil && r.Resource.Type != "" {
		return r.Resource.Type
	}
	return r.ResourceType
}

// ResolvedInviterName returns the inviter display name, preferring the structured
// Inviter object over the deprecated InviterName scalar.
func (r *SendInviteRequest) ResolvedInviterName() string {
	if r.Inviter != nil && r.Inviter.Name != "" {
		return r.Inviter.Name
	}
	return r.InviterName
}
