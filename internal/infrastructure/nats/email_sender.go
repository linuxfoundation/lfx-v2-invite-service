// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	smtptmpl "github.com/linuxfoundation/lfx-v2-invite-service/internal/infrastructure/smtp"
	pkgerrors "github.com/linuxfoundation/lfx-v2-invite-service/pkg/errors"
)

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

// SendProjectAddedNotification renders the invite template and publishes to the email
// service via NATS request/reply. An empty reply means success.
func (s *NATSEmailSender) SendProjectAddedNotification(ctx context.Context, n *model.ProjectAddedNotification) error {
	req := sendEmailRequest{
		To:      n.RecipientEmail,
		Subject: fmt.Sprintf("You've been added to %s", n.ProjectName),
		HTML:    smtptmpl.RenderProjectAddedHTML(n),
		Text:    smtptmpl.RenderProjectAddedPlain(n),
	}

	data, err := json.Marshal(req)
	if err != nil {
		return pkgerrors.NewUnexpected("failed to marshal email request", err)
	}

	reply, err := s.client.Request(ctx, s.subject, data)
	if err != nil {
		return err
	}

	if len(reply) == 0 {
		slog.DebugContext(ctx, "email service accepted message",
			"recipient", n.RecipientEmail,
			"project_uid", n.ProjectUID,
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
