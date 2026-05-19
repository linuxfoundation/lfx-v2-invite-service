// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// SendInviteRequest is the internal domain representation of the invite payload.
// It mirrors pkg/api.SendInviteRequest, which is the public contract published
// by resource services over NATS. Keep both in sync when adding fields —
// the split is intentional: pkg/api is the inter-service contract, this package
// is the internal domain model owned by the invite service.
type SendInviteRequest struct {
	RecipientEmail string `json:"recipient_email"`
	RecipientName  string `json:"recipient_name"`
	InviterName    string `json:"inviter_name,omitempty"`
	ResourceUID    string `json:"resource_uid"`
	ResourceName   string `json:"resource_name"`
	Role           string `json:"role"`
	ReturnURL      string `json:"return_url,omitempty"`
	// ResourceType is the kind of resource (e.g. "project", "group", "meeting").
	// Defaults to "resource" when empty.
	ResourceType string `json:"resource_type,omitempty"`
	// OrgName is the foundation or project name used in the email signature
	// ("The X Team"). Defaults to "LFX" when empty.
	OrgName string `json:"org_name,omitempty"`
	// ExpirationDays is the number of days the invite token should be valid.
	// If 0 or omitted, defaults to 7 days.
	ExpirationDays int `json:"expiration_days,omitempty"`
}
