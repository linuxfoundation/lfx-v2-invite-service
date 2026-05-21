// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"time"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
)

// Stable error sentinels exposed to the transport layer so it can map handler
// errors to opaque codes without leaking internal details to callers.
var (
	// ErrInvalidRequest is returned when the caller's SendInviteRequest fails validation.
	ErrInvalidRequest = errors.New("invalid_request")
	// ErrEmailDispatchFailed is returned when the email service cannot deliver the invite.
	ErrEmailDispatchFailed = errors.New("email_dispatch_failed")
)

// LinkGenerator generates a signed invite link for a given recipient and destination.
// Returns the full invite URL and the invite UUID (jti) so the service can
// publish the UUID to resource services via the InviteCreatedEvent.
type LinkGenerator interface {
	Generate(recipientEmail, destinationURL, resourceUID, role string, expirationDays int) (link, inviteUID string, expiresAt time.Time, err error)
}

// NotificationConfig holds configuration for the NotificationService.
type NotificationConfig struct {
	DefaultReturnURL string
}

// SendInviteResult carries the data returned by the invite service to the caller.
type SendInviteResult struct {
	InviteUID      string
	RecipientEmail string
	ExpiresAt      time.Time
}

// NotificationService dispatches invite notification emails via the email service.
type NotificationService struct {
	emailSender   port.EmailSender
	linkGenerator LinkGenerator
	config        NotificationConfig
}

// NewNotificationService creates a new NotificationService.
func NewNotificationService(email port.EmailSender, linkGen LinkGenerator, cfg NotificationConfig) *NotificationService {
	return &NotificationService{
		emailSender:   email,
		linkGenerator: linkGen,
		config:        cfg,
	}
}

// HandleSendInvite processes a send-invite request from a resource service,
// dispatches the invite notification email, and returns the invite UUID so the
// caller can store it. Returns an error if the email could not be sent.
func (s *NotificationService) HandleSendInvite(ctx context.Context, req *model.SendInviteRequest) (SendInviteResult, error) {
	if req.RecipientEmail == "" {
		return SendInviteResult{}, fmt.Errorf("%w: no recipient email for resource %s", ErrInvalidRequest, req.ResourceUID)
	}

	// Validate and canonicalize the recipient email to prevent header injection /
	// multiple-address smuggling before the address flows into the email envelope.
	addr, mailErr := mail.ParseAddress(req.RecipientEmail)
	if mailErr != nil {
		return SendInviteResult{}, fmt.Errorf("%w: invalid recipient_email %q: %w", ErrInvalidRequest, req.RecipientEmail, mailErr)
	}

	role := model.Role(req.Role)
	if role != model.RoleManage && role != model.RoleView {
		return SendInviteResult{}, fmt.Errorf("%w: unrecognised role %q for resource %s", ErrInvalidRequest, req.Role, req.ResourceUID)
	}

	// Determine destination URL — use DefaultReturnURL as fallback when not supplied.
	destURL := req.ReturnURL
	if destURL == "" && s.config.DefaultReturnURL != "" {
		destURL = s.config.DefaultReturnURL
	}

	// Generate a signed JWT invite link wrapping the destination URL.
	// Fail closed: JWT signing failure is a hard error — silently falling back to a
	// plain URL would deliver an LFX-branded email pointing to an unsigned, unrevokable link.
	inviteLink, inviteUID, expiresAt, linkErr := s.linkGenerator.Generate(addr.Address, destURL, req.ResourceUID, req.Role, req.ExpirationDays)
	if linkErr != nil {
		return SendInviteResult{}, fmt.Errorf("generate invite link for resource %s: %w", req.ResourceUID, linkErr)
	}

	// Shallow-copy with canonical email and signed link so we don't mutate the caller's struct.
	reqCopy := *req
	reqCopy.RecipientEmail = addr.Address
	reqCopy.ReturnURL = inviteLink
	req = &reqCopy

	if err := s.emailSender.SendNotification(ctx, req); err != nil {
		slog.ErrorContext(ctx, "failed to send invite notification",
			"resource_uid", req.ResourceUID,
			"recipient_email", req.RecipientEmail,
			"error", err,
		)
		s.auditNotification(ctx, &model.NotificationAuditEntry{
			ResourceUID:    req.ResourceUID,
			RecipientEmail: req.RecipientEmail,
			Role:           role,
			DeliveryState:  model.DeliveryStateFailed,
			ErrorMessage:   err.Error(),
		})
		return SendInviteResult{}, fmt.Errorf("%w: send invite notification for resource %s: %w", ErrEmailDispatchFailed, req.ResourceUID, err)
	}

	slog.InfoContext(ctx, "invite notification sent",
		"resource_uid", req.ResourceUID,
		"recipient_email", req.RecipientEmail,
		"invite_uid", inviteUID,
		"expires_at", expiresAt,
	)
	s.auditNotification(ctx, &model.NotificationAuditEntry{
		ResourceUID:    req.ResourceUID,
		RecipientEmail: req.RecipientEmail,
		Role:           role,
		DeliveryState:  model.DeliveryStateSent,
	})

	return SendInviteResult{
		InviteUID:      inviteUID,
		RecipientEmail: req.RecipientEmail,
		ExpiresAt:      expiresAt,
	}, nil
}

// auditNotification writes a structured audit log entry. In Phase 1 this is a structured
// log line; a persistent audit store will be wired in a later ticket.
func (s *NotificationService) auditNotification(ctx context.Context, entry *model.NotificationAuditEntry) {
	slog.InfoContext(ctx, "notification_audit",
		"resource_uid", entry.ResourceUID,
		"recipient_lfid", entry.RecipientLFID,
		"recipient_email", entry.RecipientEmail,
		"role", entry.Role,
		"delivery_state", entry.DeliveryState,
		"error_message", entry.ErrorMessage,
	)
}
