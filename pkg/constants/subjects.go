// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

const (
	// InviteServiceQueue is the NATS queue group name for this service.
	InviteServiceQueue = "lfx.invite-service.queue"

	// ProjectSettingsUpdatedSubject is the subject published by project-service
	// when project settings (writers, auditors, etc.) change.
	ProjectSettingsUpdatedSubject = "lfx.projects-api.project_settings.updated"

	// ProjectGetNameSubject is the subject for request/reply to retrieve a project name.
	ProjectGetNameSubject = "lfx.projects-api.get_name"

	// InviteCreatedSubject is published when an invite is issued by this service.
	InviteCreatedSubject = "lfx.invite-service.invite.created"

	// InviteAcceptedSubject is published when an invite is accepted.
	InviteAcceptedSubject = "lfx.invite-service.invite.accepted"

	// InviteRevokedSubject is published when an invite is revoked.
	InviteRevokedSubject = "lfx.invite-service.invite.revoked"

	// StreamNameProjectSettingsEvents is the JetStream stream that captures project
	// settings change events for durable delivery to the invite service.
	StreamNameProjectSettingsEvents = "project-settings-events"

	// ConsumerNameProjectSettingsNotify is the durable JetStream consumer for
	// sending "you were added" notifications on project settings changes.
	ConsumerNameProjectSettingsNotify = "invite-service-project-settings-notify"
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

	// EmailSMTPHostEnvKey is the SMTP server host.
	EmailSMTPHostEnvKey = "EMAIL_SMTP_HOST"

	// EmailSMTPPortEnvKey is the SMTP server port.
	EmailSMTPPortEnvKey = "EMAIL_SMTP_PORT"

	// EmailSMTPUsernameEnvKey is the SMTP username.
	EmailSMTPUsernameEnvKey = "EMAIL_SMTP_USERNAME"

	// EmailSMTPPasswordEnvKey is the SMTP password.
	EmailSMTPPasswordEnvKey = "EMAIL_SMTP_PASSWORD"

	// LFXBaseURLEnvKey is the base URL for LFX deep links.
	LFXBaseURLEnvKey = "LFX_BASE_URL"
)
