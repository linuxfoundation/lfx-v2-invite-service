// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"net/url"
	"strings"
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
	// AllowedReturnURLHosts is the list of host patterns (e.g. "*.lfx.dev") that
	// a caller-supplied return_url must match. An empty slice disables the check.
	AllowedReturnURLHosts []string
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
	inviteStore   port.InviteStore
	config        NotificationConfig
}

// NewNotificationService creates a new NotificationService.
// inviteStore may be nil during tests or in deployments that haven't provisioned
// the KV bucket yet; the send flow will log a warning and continue.
func NewNotificationService(email port.EmailSender, linkGen LinkGenerator, store port.InviteStore, cfg NotificationConfig) *NotificationService {
	return &NotificationService{
		emailSender:   email,
		linkGenerator: linkGen,
		inviteStore:   store,
		config:        cfg,
	}
}

// HandleSendInvite processes a send-invite request from a resource service,
// dispatches the invite notification email, persists the invite record to KV,
// and returns the invite UUID so the caller can store it.
// Returns an error if the email could not be sent; KV write failures are logged
// but do not fail the operation (the email has already been dispatched).
func (s *NotificationService) HandleSendInvite(ctx context.Context, req *model.SendInviteRequest) (SendInviteResult, error) {
	// Resolve email and resource UID from structured objects or deprecated scalars.
	rawEmail := req.ResolvedRecipientEmail()
	resourceUID := req.ResolvedResourceUID()

	if rawEmail == "" {
		return SendInviteResult{}, fmt.Errorf("%w: no recipient email for resource %s", ErrInvalidRequest, resourceUID)
	}

	// Validate and canonicalize the recipient email to prevent header injection /
	// multiple-address smuggling before the address flows into the email envelope.
	addr, mailErr := mail.ParseAddress(rawEmail)
	if mailErr != nil {
		return SendInviteResult{}, fmt.Errorf("%w: invalid recipient_email %q: %w", ErrInvalidRequest, rawEmail, mailErr)
	}
	canonicalEmail := addr.Address

	role := model.Role(req.Role)
	if role != model.RoleManage && role != model.RoleView && role != model.RoleMember {
		return SendInviteResult{}, fmt.Errorf("%w: unrecognised role %q for resource %s", ErrInvalidRequest, req.Role, resourceUID)
	}

	// Validate the caller-supplied return_url against the allowlist before using it.
	// The default fallback is always a known-good LFX URL, so only the caller value needs checking.
	if req.ReturnURL != "" {
		if err := validateReturnURL(req.ReturnURL, s.config.AllowedReturnURLHosts); err != nil {
			return SendInviteResult{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
		}
	}

	// Determine destination URL — use DefaultReturnURL as fallback when not supplied.
	// Capture destURL BEFORE it is overwritten in the copy with the signed JWT link;
	// we store the destination URL in the KV record, never the token itself.
	destURL := req.ReturnURL
	if destURL == "" && s.config.DefaultReturnURL != "" {
		destURL = s.config.DefaultReturnURL
	}

	// Generate a signed JWT invite link wrapping the destination URL.
	// Fail closed: JWT signing failure is a hard error — silently falling back to a
	// plain URL would deliver an LFX-branded email pointing to an unsigned, unrevokable link.
	inviteLink, inviteUID, expiresAt, linkErr := s.linkGenerator.Generate(canonicalEmail, destURL, resourceUID, req.Role, req.ExpirationDays)
	if linkErr != nil {
		return SendInviteResult{}, fmt.Errorf("generate invite link for resource %s: %w", resourceUID, linkErr)
	}

	// Shallow-copy with canonical and resolved fields so templates and downstream
	// code see consistent state regardless of whether the caller used the structured
	// objects or the deprecated scalar fields. The nolint directives below are
	// intentional: we are populating the deprecated scalars for backward-compat with
	// infrastructure adapters (email_sender, smtp templates) that still read them.
	reqCopy := *req
	reqCopy.RecipientEmail = canonicalEmail             //nolint:staticcheck
	reqCopy.RecipientName = req.ResolvedRecipientName() //nolint:staticcheck
	reqCopy.InviterName = req.ResolvedInviterName()     //nolint:staticcheck
	reqCopy.ResourceUID = resourceUID                   //nolint:staticcheck
	reqCopy.ResourceName = req.ResolvedResourceName()   //nolint:staticcheck
	reqCopy.ResourceType = req.ResolvedResourceType()   //nolint:staticcheck
	reqCopy.ReturnURL = inviteLink                      // templates need the signed link as the CTA URL
	req = &reqCopy

	if err := s.emailSender.SendNotification(ctx, req); err != nil {
		slog.ErrorContext(ctx, "failed to send invite notification",
			"resource_uid", resourceUID,
			"recipient_email", redactEmail(canonicalEmail),
			"error", err,
		)
		s.auditNotification(ctx, &model.NotificationAuditEntry{
			ResourceUID:    resourceUID,
			RecipientEmail: canonicalEmail,
			Role:           role,
			DeliveryState:  model.DeliveryStateFailed,
			ErrorMessage:   err.Error(),
		})
		return SendInviteResult{}, fmt.Errorf("%w: send invite notification for resource %s: %w", ErrEmailDispatchFailed, resourceUID, err)
	}

	slog.InfoContext(ctx, "invite notification sent",
		"resource_uid", resourceUID,
		"recipient_email", redactEmail(canonicalEmail),
		"invite_uid", inviteUID,
		"expires_at", expiresAt,
	)
	s.auditNotification(ctx, &model.NotificationAuditEntry{
		ResourceUID:    resourceUID,
		RecipientEmail: canonicalEmail,
		Role:           role,
		DeliveryState:  model.DeliveryStateSent,
	})

	// Persist the invite record to KV. This is best-effort: the email has already
	// been sent so we log and continue rather than returning an error on KV failure.
	if s.inviteStore != nil {
		record := buildInviteRecord(inviteUID, req, destURL, expiresAt)
		if storeErr := s.inviteStore.Create(ctx, record); storeErr != nil {
			slog.ErrorContext(ctx, "invite_store: failed to persist invite record — email was sent, record will be missing",
				"invite_uid", inviteUID,
				"resource_uid", resourceUID,
				"error", storeErr,
			)
		}
	} else {
		slog.WarnContext(ctx, "invite_store: no store configured — invite record not persisted",
			"invite_uid", inviteUID)
	}

	return SendInviteResult{
		InviteUID:      inviteUID,
		RecipientEmail: canonicalEmail,
		ExpiresAt:      expiresAt,
	}, nil
}

// buildInviteRecord constructs the InviteRecord to persist from the normalized
// copy of the request. destURL is the destination URL captured before the JWT
// link overwrote ReturnURL on the copy — it is never the signed token.
func buildInviteRecord(inviteUID string, req *model.SendInviteRequest, destURL string, expiresAt time.Time) *model.InviteRecord {
	// Build inviter — prefer the structured object, fall back to the resolved scalar.
	inviterName := req.ResolvedInviterName()
	var inviter model.Inviter
	if req.Inviter != nil {
		inviter = model.Inviter{
			Name:     firstNonEmpty(req.Inviter.Name, inviterName),
			Username: req.Inviter.Username,
			Email:    req.Inviter.Email,
			Avatar:   req.Inviter.Avatar,
		}
	} else {
		inviter = model.Inviter{Name: inviterName}
	}

	// Recipient.Email is always the canonical value from mail.ParseAddress,
	// captured in the inviteUID / expiresAt generation step above.
	recipientName := req.ResolvedRecipientName()
	recipientEmail := req.ResolvedRecipientEmail()
	var recipient model.Recipient
	if req.Recipient != nil {
		recipient = model.Recipient{
			Name:     firstNonEmpty(req.Recipient.Name, recipientName),
			Email:    recipientEmail, // canonical
			Username: req.Recipient.Username,
			Avatar:   req.Recipient.Avatar,
		}
	} else {
		recipient = model.Recipient{
			Name:  recipientName,
			Email: recipientEmail,
		}
	}

	// Resource uses the resolved values (scalars already normalized in the copy).
	resource := model.InviteResource{
		UID:  req.ResolvedResourceUID(),
		Name: req.ResolvedResourceName(),
		Type: req.ResolvedResourceType(),
	}

	return &model.InviteRecord{
		UID:            inviteUID,
		Status:         model.InviteStatusPending,
		Recipient:      recipient,
		Inviter:        inviter,
		Resource:       resource,
		Role:           req.Role,
		OrgName:        req.OrgName,
		ReturnURL:      destURL,
		ExpirationDays: req.ExpirationDays,
		CreatedAt:      time.Now(),
		ExpiresAt:      expiresAt,
	}
}

// firstNonEmpty returns the first non-empty string from the given values.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// validateReturnURL checks that rawURL is an https URL whose host matches at least one
// pattern in allowedHosts. A wildcard pattern "*.example.com" matches any subdomain of
// example.com (including multi-level, e.g. a.b.example.com). An empty allowedHosts
// slice disables enforcement.
func validateReturnURL(rawURL string, allowedHosts []string) error {
	if len(allowedHosts) == 0 {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid return_url %q: %w", rawURL, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("return_url must use https scheme, got %q", u.Scheme)
	}
	host := u.Hostname()
	for _, pattern := range allowedHosts {
		if matchHost(host, pattern) {
			return nil
		}
	}
	return fmt.Errorf("return_url host %q is not in the allowed list", host)
}

// redactEmail masks the local part of an email address for safe logging.
// "alice@example.com" → "a***@example.com"
func redactEmail(email string) string {
	at := strings.Index(email, "@")
	if at <= 0 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}

// matchHost reports whether host matches pattern. A pattern starting with "*."
// matches any subdomain of the remainder (e.g. "*.lfx.dev" matches "app.lfx.dev"
// and "a.b.lfx.dev"). Otherwise an exact match is required.
func matchHost(host, pattern string) bool {
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(host, pattern[1:]) // pattern[1:] == ".lfx.dev"
	}
	return host == pattern
}

// auditNotification writes a structured audit log entry. In Phase 1 this is a structured
// log line; a persistent audit store will be wired in a later ticket.
func (s *NotificationService) auditNotification(ctx context.Context, entry *model.NotificationAuditEntry) {
	slog.InfoContext(ctx, "notification_audit",
		"resource_uid", entry.ResourceUID,
		"recipient_lfid", entry.RecipientLFID,
		"recipient_email", redactEmail(entry.RecipientEmail),
		"role", entry.Role,
		"delivery_state", entry.DeliveryState,
		"error_message", entry.ErrorMessage,
	)
}
