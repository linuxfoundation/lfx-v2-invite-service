// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"testing"
)

func TestAddedUsers(t *testing.T) {
	alice := UserInfo{Username: "alice", Name: "Alice", Email: "alice@example.com"}
	bob := UserInfo{Username: "bob", Name: "Bob", Email: "bob@example.com"}
	carol := UserInfo{Username: "carol", Name: "Carol", Email: "carol@example.com"}

	tests := []struct {
		name     string
		msg      ProjectSettingsUpdatedMessage
		wantLen  int
		validate func(t *testing.T, got []AddedUser)
	}{
		{
			name: "no users in old or new",
			msg: ProjectSettingsUpdatedMessage{
				ProjectUID:  "proj-1",
				OldSettings: ProjectSettings{},
				NewSettings: ProjectSettings{},
			},
			wantLen: 0,
		},
		{
			name: "old empty, new has writers — all added as Manage",
			msg: ProjectSettingsUpdatedMessage{
				OldSettings: ProjectSettings{},
				NewSettings: ProjectSettings{Writers: []UserInfo{alice, bob}},
			},
			wantLen: 2,
			validate: func(t *testing.T, got []AddedUser) {
				t.Helper()
				for _, u := range got {
					if u.Role != RoleManage {
						t.Errorf("expected role %q, got %q", RoleManage, u.Role)
					}
				}
			},
		},
		{
			name: "old empty, new has auditors — all added as View",
			msg: ProjectSettingsUpdatedMessage{
				OldSettings: ProjectSettings{},
				NewSettings: ProjectSettings{Auditors: []UserInfo{alice}},
			},
			wantLen: 1,
			validate: func(t *testing.T, got []AddedUser) {
				t.Helper()
				if got[0].Role != RoleView {
					t.Errorf("expected role %q, got %q", RoleView, got[0].Role)
				}
			},
		},
		{
			name: "no change to either list — nothing added",
			msg: ProjectSettingsUpdatedMessage{
				OldSettings: ProjectSettings{Writers: []UserInfo{alice}, Auditors: []UserInfo{bob}},
				NewSettings: ProjectSettings{Writers: []UserInfo{alice}, Auditors: []UserInfo{bob}},
			},
			wantLen: 0,
		},
		{
			name: "new writer added, auditors unchanged",
			msg: ProjectSettingsUpdatedMessage{
				OldSettings: ProjectSettings{Writers: []UserInfo{alice}},
				NewSettings: ProjectSettings{Writers: []UserInfo{alice, bob}},
			},
			wantLen: 1,
			validate: func(t *testing.T, got []AddedUser) {
				t.Helper()
				if got[0].User.Username != "bob" {
					t.Errorf("expected bob, got %q", got[0].User.Username)
				}
				if got[0].Role != RoleManage {
					t.Errorf("expected Manage, got %q", got[0].Role)
				}
			},
		},
		{
			name: "new auditor added, writers unchanged",
			msg: ProjectSettingsUpdatedMessage{
				OldSettings: ProjectSettings{Auditors: []UserInfo{alice}},
				NewSettings: ProjectSettings{Auditors: []UserInfo{alice, carol}},
			},
			wantLen: 1,
			validate: func(t *testing.T, got []AddedUser) {
				t.Helper()
				if got[0].User.Username != "carol" {
					t.Errorf("expected carol, got %q", got[0].User.Username)
				}
				if got[0].Role != RoleView {
					t.Errorf("expected View, got %q", got[0].Role)
				}
			},
		},
		{
			name: "user promoted from auditor to writer — appears as new writer",
			msg: ProjectSettingsUpdatedMessage{
				OldSettings: ProjectSettings{Auditors: []UserInfo{alice}},
				NewSettings: ProjectSettings{Writers: []UserInfo{alice}},
			},
			wantLen: 1,
			validate: func(t *testing.T, got []AddedUser) {
				t.Helper()
				if got[0].Role != RoleManage {
					t.Errorf("expected Manage, got %q", got[0].Role)
				}
			},
		},
		{
			name: "user demoted from writer to auditor — appears as new auditor",
			msg: ProjectSettingsUpdatedMessage{
				OldSettings: ProjectSettings{Writers: []UserInfo{alice}},
				NewSettings: ProjectSettings{Auditors: []UserInfo{alice}},
			},
			wantLen: 1,
			validate: func(t *testing.T, got []AddedUser) {
				t.Helper()
				if got[0].Role != RoleView {
					t.Errorf("expected View, got %q", got[0].Role)
				}
			},
		},
		{
			name: "user with empty username skipped (pending invite)",
			msg: ProjectSettingsUpdatedMessage{
				OldSettings: ProjectSettings{},
				NewSettings: ProjectSettings{
					Writers: []UserInfo{
						{Username: "", Name: "Pending", Email: "pending@example.com"},
						alice,
					},
				},
			},
			wantLen: 1,
			validate: func(t *testing.T, got []AddedUser) {
				t.Helper()
				if got[0].User.Username != "alice" {
					t.Errorf("expected alice, got %q", got[0].User.Username)
				}
			},
		},
		{
			name: "new users added to both writers and auditors",
			msg: ProjectSettingsUpdatedMessage{
				OldSettings: ProjectSettings{},
				NewSettings: ProjectSettings{
					Writers:  []UserInfo{alice},
					Auditors: []UserInfo{bob},
				},
			},
			wantLen: 2,
		},
		{
			name: "user already a writer, added as auditor too — counted once as auditor",
			msg: ProjectSettingsUpdatedMessage{
				OldSettings: ProjectSettings{Writers: []UserInfo{alice}},
				NewSettings: ProjectSettings{Writers: []UserInfo{alice}, Auditors: []UserInfo{alice}},
			},
			wantLen: 1,
			validate: func(t *testing.T, got []AddedUser) {
				t.Helper()
				if got[0].Role != RoleView {
					t.Errorf("expected View, got %q", got[0].Role)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.msg.AddedUsers()
			if len(got) != tt.wantLen {
				t.Errorf("AddedUsers() returned %d users, want %d", len(got), tt.wantLen)
			}
			if tt.validate != nil {
				tt.validate(t, got)
			}
		})
	}
}
