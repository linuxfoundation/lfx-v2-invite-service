// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	emailapi "github.com/linuxfoundation/lfx-v2-email-service/pkg/api"
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	smtptmpl "github.com/linuxfoundation/lfx-v2-invite-service/internal/infrastructure/smtp"
)

// emailServiceTimeout is the maximum time to wait for the email service to accept a message.
const emailServiceTimeout = 10 * time.Second

// NATSEmailSender implements port.EmailSender by forwarding rendered email bodies
// to the email service via NATS request/reply. The invite service owns and renders
// the templates; the email service handles SMTP delivery.
type NATSEmailSender struct {
	client  *Client
	subject string
}

// NewNATSEmailSender creates a NATSEmailSender that publishes to the given NATS subject.
func NewNATSEmailSender(client *Client, subject string) *NATSEmailSender {
	return &NATSEmailSender{client: client, subject: subject}
}

// SendNotification renders the invite template and publishes to the email service
// via NATS request/reply. An empty reply means success.
func (s *NATSEmailSender) SendNotification(ctx context.Context, req *model.SendInviteRequest) error {
	envelope := emailapi.SendEmailRequest{
		To:      req.RecipientEmail,
		Subject: smtptmpl.InviteEmailSubject(req),
		HTML:    smtptmpl.RenderInviteHTML(req),
		Text:    smtptmpl.RenderInvitePlain(req),
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return newUnexpected("failed to marshal email request", err)
	}

	// Enforce a tighter deadline on the email service request independently of the
	// outer per-message context so email latency stays bounded even under slow replies.
	reqCtx, cancel := context.WithTimeout(ctx, emailServiceTimeout)
	defer cancel()

	reply, err := s.client.Request(reqCtx, s.subject, data)
	if err != nil {
		return err
	}

	if len(reply) == 0 {
		slog.DebugContext(ctx, "email service accepted message",
			"recipient", req.RecipientEmail,
			"resource_uid", req.ResourceUID,
		)
		return nil
	}

	// Treat any non-empty reply that fails to parse as an error — a corrupt reply
	// must not be silently treated as success.
	var errResp emailapi.SendEmailErrorResponse
	if jsonErr := json.Unmarshal(reply, &errResp); jsonErr != nil {
		return newUnexpected("malformed email-service reply", jsonErr)
	}
	if errResp.Error != "" {
		return newServiceUnavailable("email service returned error", errors.New(errResp.Error))
	}

	return nil
}
