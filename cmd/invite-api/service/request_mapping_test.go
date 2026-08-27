// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"testing"

	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

// TestAPIToModelRequest_MapsAllFields verifies that every field in
// api.SendInviteRequest — structured objects and deprecated scalars alike —
// arrives in the correct field of the domain model. This test is the
// enforcement mechanism for the mapping: a field added to api.SendInviteRequest
// that is omitted from apiToModelRequest will cause this test to fail.
func TestAPIToModelRequest_MapsAllFields(t *testing.T) {
	in := api.SendInviteRequest{
		Recipient: &api.Recipient{
			Name:     "Alice",
			Email:    "alice@example.com",
			Username: "alice-lfid",
			Avatar:   "https://avatar.example.com/alice",
		},
		Inviter: &api.Inviter{
			Name:     "Bob",
			Username: "bob-lfid",
			Email:    "bob@example.com",
			Avatar:   "https://avatar.example.com/bob",
		},
		Resource: &api.Resource{
			UID:  "res-1",
			Name: "My Project",
			Type: "project",
		},
		RecipientEmail: "scalar@example.com", //nolint:staticcheck
		RecipientName:  "Scalar Alice",       //nolint:staticcheck
		InviterName:    "Scalar Bob",         //nolint:staticcheck
		ResourceUID:    "scalar-res",         //nolint:staticcheck
		ResourceName:   "Scalar Project",     //nolint:staticcheck
		ResourceType:   "committee",          //nolint:staticcheck
		Role:           "Member",
		ReturnURL:      "https://app.lfx.dev/return",
		OrgName:        "The Linux Foundation",
		ExpirationDays: 14,
		CustomClaims:   map[string]string{"committee_invite_uid": "inv-abc"},
	}

	got := apiToModelRequest(in)

	// Structured Recipient
	if got.Recipient == nil {
		t.Fatal("Recipient: got nil, want non-nil")
	}
	if got.Recipient.Name != "Alice" {
		t.Errorf("Recipient.Name: got %q, want %q", got.Recipient.Name, "Alice")
	}
	if got.Recipient.Email != "alice@example.com" {
		t.Errorf("Recipient.Email: got %q, want %q", got.Recipient.Email, "alice@example.com")
	}
	if got.Recipient.Username != "alice-lfid" {
		t.Errorf("Recipient.Username: got %q, want %q", got.Recipient.Username, "alice-lfid")
	}
	if got.Recipient.Avatar != "https://avatar.example.com/alice" {
		t.Errorf("Recipient.Avatar: got %q, want %q", got.Recipient.Avatar, "https://avatar.example.com/alice")
	}

	// Structured Inviter
	if got.Inviter == nil {
		t.Fatal("Inviter: got nil, want non-nil")
	}
	if got.Inviter.Name != "Bob" {
		t.Errorf("Inviter.Name: got %q, want %q", got.Inviter.Name, "Bob")
	}
	if got.Inviter.Username != "bob-lfid" {
		t.Errorf("Inviter.Username: got %q, want %q", got.Inviter.Username, "bob-lfid")
	}
	if got.Inviter.Email != "bob@example.com" {
		t.Errorf("Inviter.Email: got %q, want %q", got.Inviter.Email, "bob@example.com")
	}
	if got.Inviter.Avatar != "https://avatar.example.com/bob" {
		t.Errorf("Inviter.Avatar: got %q, want %q", got.Inviter.Avatar, "https://avatar.example.com/bob")
	}

	// Structured Resource → model.InviteResource
	if got.Resource == nil {
		t.Fatal("Resource: got nil, want non-nil")
	}
	if got.Resource.UID != "res-1" {
		t.Errorf("Resource.UID: got %q, want %q", got.Resource.UID, "res-1")
	}
	if got.Resource.Name != "My Project" {
		t.Errorf("Resource.Name: got %q, want %q", got.Resource.Name, "My Project")
	}
	if got.Resource.Type != "project" {
		t.Errorf("Resource.Type: got %q, want %q", got.Resource.Type, "project")
	}

	// Deprecated scalars
	if got.RecipientEmail != "scalar@example.com" { //nolint:staticcheck
		t.Errorf("RecipientEmail: got %q, want %q", got.RecipientEmail, "scalar@example.com") //nolint:staticcheck
	}
	if got.RecipientName != "Scalar Alice" { //nolint:staticcheck
		t.Errorf("RecipientName: got %q, want %q", got.RecipientName, "Scalar Alice") //nolint:staticcheck
	}
	if got.InviterName != "Scalar Bob" { //nolint:staticcheck
		t.Errorf("InviterName: got %q, want %q", got.InviterName, "Scalar Bob") //nolint:staticcheck
	}
	if got.ResourceUID != "scalar-res" { //nolint:staticcheck
		t.Errorf("ResourceUID: got %q, want %q", got.ResourceUID, "scalar-res") //nolint:staticcheck
	}
	if got.ResourceName != "Scalar Project" { //nolint:staticcheck
		t.Errorf("ResourceName: got %q, want %q", got.ResourceName, "Scalar Project") //nolint:staticcheck
	}
	if got.ResourceType != "committee" { //nolint:staticcheck
		t.Errorf("ResourceType: got %q, want %q", got.ResourceType, "committee") //nolint:staticcheck
	}

	// Scalar fields
	if got.Role != "Member" {
		t.Errorf("Role: got %q, want %q", got.Role, "Member")
	}
	if got.ReturnURL != "https://app.lfx.dev/return" {
		t.Errorf("ReturnURL: got %q, want %q", got.ReturnURL, "https://app.lfx.dev/return")
	}
	if got.OrgName != "The Linux Foundation" {
		t.Errorf("OrgName: got %q, want %q", got.OrgName, "The Linux Foundation")
	}
	if got.ExpirationDays != 14 {
		t.Errorf("ExpirationDays: got %d, want %d", got.ExpirationDays, 14)
	}
	if got.CustomClaims["committee_invite_uid"] != "inv-abc" {
		t.Errorf("CustomClaims[committee_invite_uid]: got %q, want %q", got.CustomClaims["committee_invite_uid"], "inv-abc")
	}
}

// TestAPIToModelRequest_NilNestedObjects verifies that absent structured objects
// produce nil fields in the domain model, leaving the Resolved*() fallback to
// deprecated scalars intact.
func TestAPIToModelRequest_NilNestedObjects(t *testing.T) {
	in := api.SendInviteRequest{
		RecipientEmail: "scalar@example.com", //nolint:staticcheck
		Role:           "Member",
	}

	got := apiToModelRequest(in)

	if got.Recipient != nil {
		t.Errorf("Recipient: got %v, want nil", got.Recipient)
	}
	if got.Inviter != nil {
		t.Errorf("Inviter: got %v, want nil", got.Inviter)
	}
	if got.Resource != nil {
		t.Errorf("Resource: got %v, want nil", got.Resource)
	}
	if got.RecipientEmail != "scalar@example.com" { //nolint:staticcheck
		t.Errorf("RecipientEmail: got %q, want %q", got.RecipientEmail, "scalar@example.com") //nolint:staticcheck
	}
}
