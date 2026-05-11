// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package smtp

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	pkgerrors "github.com/linuxfoundation/lfx-v2-invite-service/pkg/errors"
)

// Config holds SMTP connection parameters.
type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	FromAddr string
	FromName string
}

// Sender implements port.EmailSender via SMTP.
type Sender struct {
	config Config
}

// NewSender creates an SMTP Sender with the provided config.
func NewSender(cfg Config) *Sender {
	return &Sender{config: cfg}
}

// SendProjectAddedNotification sends a "you were added to a project" notification email.
func (s *Sender) SendProjectAddedNotification(ctx context.Context, n *model.ProjectAddedNotification) error {
	subject := fmt.Sprintf("You've been added to %s", n.ProjectName)
	htmlBody := renderProjectAddedHTML(n)
	plainBody := renderProjectAddedPlain(n)

	if err := s.send(ctx, n.RecipientEmail, subject, htmlBody, plainBody); err != nil {
		return err
	}

	slog.DebugContext(ctx, "project-added email sent via SMTP",
		"recipient", n.RecipientEmail,
		"project_uid", n.ProjectUID,
	)
	return nil
}

func (s *Sender) send(ctx context.Context, to, subject, htmlBody, plainBody string) error {
	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)
	from := s.config.FromAddr
	fromHeader := from
	if s.config.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", s.config.FromName, from)
	}

	boundary := "==LFX_MIME_BOUNDARY=="
	body := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n"+
			"--%s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n"+
			"--%s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n"+
			"--%s--",
		fromHeader, to, subject,
		boundary,
		boundary, plainBody,
		boundary, htmlBody,
		boundary,
	)

	var auth smtp.Auth
	if s.config.Username != "" && s.config.Password != "" {
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	}

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(body)); err != nil {
		slog.ErrorContext(ctx, "SMTP send failed",
			"error", err,
			"host", s.config.Host,
			"to", to,
		)
		return pkgerrors.NewUnexpected("failed to send email via SMTP", err)
	}
	return nil
}
