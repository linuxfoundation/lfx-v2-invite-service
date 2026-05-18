// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port"
)

// NotificationConfig holds configuration for the NotificationService.
type NotificationConfig struct {
	LFXBaseURL string
}

// NotificationService dispatches invite notification emails via the email service.
type NotificationService struct {
	emailSender port.EmailSender
	config      NotificationConfig
}

// NewNotificationService creates a new NotificationService.
func NewNotificationService(email port.EmailSender, cfg NotificationConfig) *NotificationService {
	return &NotificationService{
		emailSender: email,
		config:      cfg,
	}
}

// HandleSendInvite processes a send-invite request from a resource service and
// dispatches the invite notification email via the email service.
func (s *NotificationService) HandleSendInvite(ctx context.Context, req *model.SendInviteRequest) error {
	if req.RecipientEmail == "" {
		slog.WarnContext(ctx, "send_invite request has no recipient email, skipping",
			"resource_uid", req.ResourceUID,
		)
		return nil
	}

	role := model.Role(req.Role)
	if role != model.RoleManage && role != model.RoleView {
		slog.WarnContext(ctx, "send_invite request has unrecognised role, skipping",
			"resource_uid", req.ResourceUID,
			"role", req.Role,
		)
		s.auditNotification(ctx, &model.NotificationAuditEntry{
			ResourceUID:    req.ResourceUID,
			RecipientEmail: req.RecipientEmail,
			Role:           role,
			DeliveryState:  model.DeliveryStateSkipped,
			ErrorMessage:   "unrecognised role value: " + req.Role,
		})
		return nil
	}

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
		return fmt.Errorf("send invite notification for resource %s: %w", req.ResourceUID, err)
	}

	slog.InfoContext(ctx, "invite notification sent",
		"resource_uid", req.ResourceUID,
		"recipient_email", req.RecipientEmail,
	)
	s.auditNotification(ctx, &model.NotificationAuditEntry{
		ResourceUID:    req.ResourceUID,
		RecipientEmail: req.RecipientEmail,
		Role:           role,
		DeliveryState:  model.DeliveryStateSent,
	})
	return nil
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
