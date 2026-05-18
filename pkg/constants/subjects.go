// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

const (
	// InviteServiceQueue is the NATS queue group name for this service.
	InviteServiceQueue = "lfx.invite-service.queue"

	// InviteCreatedSubject is published when an invite is issued by this service.
	InviteCreatedSubject = "lfx.invite-service.invite.created"

	// InviteAcceptedSubject is published when an invite is accepted.
	InviteAcceptedSubject = "lfx.invite-service.invite.accepted"

	// InviteRevokedSubject is published when an invite is revoked.
	InviteRevokedSubject = "lfx.invite-service.invite.revoked"

	// SendInviteSubject is published by resource services to request an invite
	// notification email. The invite service consumes this subject, renders the
	// template, and forwards to the email service for delivery.
	SendInviteSubject = "lfx.invite-service.send_invite"

	// EmailServiceSendSubject is the NATS request/reply subject for the email service.
	// The invite service publishes pre-rendered HTML/text email bodies here.
	EmailServiceSendSubject = "lfx.email-service.send_email"

	// StreamNameInviteRequests is the JetStream stream that captures send-invite
	// requests published by resource services.
	StreamNameInviteRequests = "invite-requests"

	// ConsumerNameInviteRequestsHandler is the durable JetStream consumer that
	// processes send-invite requests and dispatches notification emails.
	ConsumerNameInviteRequestsHandler = "invite-service-send-invite"
)

// Email configuration constants
const (
	// EmailFromAddress is the sender address for notification emails.
	EmailFromAddress = "noreply@lfx.linuxfoundation.org"

	// EmailFromName is the display name for notification emails.
	EmailFromName = "LFX Platform"
)

// Environment variable keys
const (
	// NATSURLEnvKey is the NATS server URL.
	NATSURLEnvKey = "NATS_URL"

	// LFXBaseURLEnvKey is the base URL for LFX deep links.
	LFXBaseURLEnvKey = "LFX_BASE_URL"
)
