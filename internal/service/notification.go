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

// NotificationService handles "you were added" notifications for existing-LFID users.
type NotificationService struct {
	emailSender   port.EmailSender
	projectReader port.ProjectNameReader
	config        NotificationConfig
}

// NewNotificationService creates a new NotificationService.
func NewNotificationService(email port.EmailSender, projects port.ProjectNameReader, cfg NotificationConfig) *NotificationService {
	return &NotificationService{
		emailSender:   email,
		projectReader: projects,
		config:        cfg,
	}
}

// HandleProjectSettingsUpdated processes a project_settings.updated event and sends
// notification emails to any newly added existing-LFID users.
func (s *NotificationService) HandleProjectSettingsUpdated(ctx context.Context, msg *model.ProjectSettingsUpdatedMessage) error {
	addedUsers := msg.AddedUsers()
	if len(addedUsers) == 0 {
		slog.DebugContext(ctx, "no new existing-LFID users added, skipping notifications",
			"project_uid", msg.ProjectUID,
		)
		return nil
	}

	projectName, err := s.projectReader.GetProjectName(ctx, msg.ProjectUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to look up project name, cannot send notifications",
			"project_uid", msg.ProjectUID,
			"error", err,
		)
		return fmt.Errorf("get project name for %s: %w", msg.ProjectUID, err)
	}

	var firstErr error
	for _, added := range addedUsers {
		if added.User.Email == "" {
			slog.WarnContext(ctx, "added user has no email address, skipping notification",
				"project_uid", msg.ProjectUID,
				"username", added.User.Username,
			)
			s.auditNotification(ctx, &model.NotificationAuditEntry{
				ProjectUID:    msg.ProjectUID,
				RecipientLFID: added.User.Username,
				Role:          added.Role,
				DeliveryState: model.DeliveryStateSkipped,
				ErrorMessage:  "no email address on user record",
			})
			continue
		}

		n := &model.ProjectAddedNotification{
			RecipientName:  added.User.Name,
			RecipientEmail: added.User.Email,
			ProjectUID:     msg.ProjectUID,
			ProjectName:    projectName,
			Role:           added.Role,
			DeepLinkURL:    fmt.Sprintf("%s/projects/%s", s.config.LFXBaseURL, msg.ProjectUID),
		}

		if sendErr := s.emailSender.SendProjectAddedNotification(ctx, n); sendErr != nil {
			slog.ErrorContext(ctx, "failed to send project-added notification",
				"project_uid", msg.ProjectUID,
				"recipient_lfid", added.User.Username,
				"recipient_email", added.User.Email,
				"role", added.Role,
				"error", sendErr,
			)
			s.auditNotification(ctx, &model.NotificationAuditEntry{
				ProjectUID:     msg.ProjectUID,
				RecipientLFID:  added.User.Username,
				RecipientEmail: added.User.Email,
				Role:           added.Role,
				DeliveryState:  model.DeliveryStateFailed,
				ErrorMessage:   sendErr.Error(),
			})
			if firstErr == nil {
				firstErr = sendErr
			}
			continue
		}

		slog.InfoContext(ctx, "project-added notification sent",
			"project_uid", msg.ProjectUID,
			"project_name", projectName,
			"recipient_lfid", added.User.Username,
			"role", added.Role,
		)
		s.auditNotification(ctx, &model.NotificationAuditEntry{
			ProjectUID:     msg.ProjectUID,
			RecipientLFID:  added.User.Username,
			RecipientEmail: added.User.Email,
			Role:           added.Role,
			DeliveryState:  model.DeliveryStateSent,
		})
	}

	return firstErr
}

// HandleSendInvite processes a send-invite request from a resource service and
// dispatches the invite notification email via the email service.
func (s *NotificationService) HandleSendInvite(ctx context.Context, req *model.SendInviteRequest) error {
	if req.RecipientEmail == "" {
		slog.WarnContext(ctx, "send_invite request has no recipient email, skipping",
			"project_uid", req.ProjectUID,
		)
		return nil
	}

	n := &model.ProjectAddedNotification{
		RecipientName:  req.RecipientName,
		RecipientEmail: req.RecipientEmail,
		InviterName:    req.InviterName,
		ProjectUID:     req.ProjectUID,
		ProjectName:    req.ProjectName,
		Role:           model.Role(req.Role),
		DeepLinkURL:    req.DeepLinkURL,
	}

	if err := s.emailSender.SendProjectAddedNotification(ctx, n); err != nil {
		slog.ErrorContext(ctx, "failed to send invite notification",
			"project_uid", req.ProjectUID,
			"recipient_email", req.RecipientEmail,
			"error", err,
		)
		return fmt.Errorf("send invite notification for project %s: %w", req.ProjectUID, err)
	}

	slog.InfoContext(ctx, "invite notification sent",
		"project_uid", req.ProjectUID,
		"recipient_email", req.RecipientEmail,
	)
	return nil
}

// auditNotification writes a structured audit log entry. In Phase 1 this is a structured
// log line; a persistent audit store will be wired in a later ticket.
func (s *NotificationService) auditNotification(ctx context.Context, entry *model.NotificationAuditEntry) {
	slog.InfoContext(ctx, "notification_audit",
		"project_uid", entry.ProjectUID,
		"recipient_lfid", entry.RecipientLFID,
		"recipient_email", entry.RecipientEmail,
		"role", entry.Role,
		"delivery_state", entry.DeliveryState,
		"error_message", entry.ErrorMessage,
	)
}
