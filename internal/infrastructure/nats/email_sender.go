// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	smtptmpl "github.com/linuxfoundation/lfx-v2-invite-service/internal/infrastructure/smtp"
	pkgerrors "github.com/linuxfoundation/lfx-v2-invite-service/pkg/errors"
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

// sendEmailRequest matches the email service's SendEmailRequest payload shape.
type sendEmailRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	Text    string `json:"text"`
}

// sendEmailErrorResponse matches the email service's SendEmailErrorResponse payload shape.
type sendEmailErrorResponse struct {
	Error string `json:"error"`
}

// SendNotification renders the invite template and publishes to the email service
// via NATS request/reply. An empty reply means success.
func (s *NATSEmailSender) SendNotification(ctx context.Context, req *model.SendInviteRequest) error {
	envelope := sendEmailRequest{
		To:      req.RecipientEmail,
		Subject: fmt.Sprintf("You've been added to %s", req.ResourceName),
		HTML:    smtptmpl.RenderInviteHTML(req),
		Text:    smtptmpl.RenderInvitePlain(req),
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return pkgerrors.NewUnexpected("failed to marshal email request", err)
	}

	// Enforce a hard deadline so the caller is never blocked indefinitely if the
	// email service is down or slow. The JetStream message handler context has no
	// deadline by default.
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

	var errResp sendEmailErrorResponse
	if jsonErr := json.Unmarshal(reply, &errResp); jsonErr == nil && errResp.Error != "" {
		return pkgerrors.NewServiceUnavailable("email service returned error",
			errors.New(errResp.Error))
	}

	return nil
}
