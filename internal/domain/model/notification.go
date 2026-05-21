// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// Role represents the access level granted to a user.
type Role string

const (
	// RoleManage maps to the writers permission set.
	RoleManage Role = "Manage"
	// RoleView maps to the auditors permission set.
	RoleView Role = "View"
)

// DeliveryState indicates the outcome of an attempted email send.
type DeliveryState string

const (
	// DeliveryStateSent means the send succeeded.
	DeliveryStateSent DeliveryState = "sent"
	// DeliveryStateFailed means the send failed.
	DeliveryStateFailed DeliveryState = "failed"
	// DeliveryStateSkipped means the send was intentionally skipped (e.g., no email address).
	DeliveryStateSkipped DeliveryState = "skipped"
)

// NotificationAuditEntry records the outcome of a notification attempt.
type NotificationAuditEntry struct {
	ResourceUID    string
	RecipientLFID  string
	RecipientEmail string
	Role           Role
	DeliveryState  DeliveryState
	ErrorMessage   string
}
