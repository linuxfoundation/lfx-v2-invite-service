// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// SendInviteRequest is the NATS payload published by resource services on
// lfx.invite-service.send_invite to request an invite notification email.
// The invite service owns the email template; the email service handles delivery.
type SendInviteRequest struct {
	RecipientEmail string `json:"recipient_email"`
	RecipientName  string `json:"recipient_name"`
	InviterName    string `json:"inviter_name,omitempty"`
	ResourceUID    string `json:"resource_uid"`
	ResourceName   string `json:"resource_name"`
	Role           string `json:"role"`
	DeepLinkURL    string `json:"deep_link_url,omitempty"`
}
