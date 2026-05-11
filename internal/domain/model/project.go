// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// UserInfo represents a user's profile as stored in project settings.
type UserInfo struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// ProjectSettings mirrors the project-service NATS KV representation of project settings.
type ProjectSettings struct {
	UID      string     `json:"uid"`
	Auditors []UserInfo `json:"auditors"`
	Writers  []UserInfo `json:"writers"`
}

// ProjectSettingsUpdatedMessage is the payload published on
// lfx.projects-api.project_settings.updated.
type ProjectSettingsUpdatedMessage struct {
	ProjectUID  string          `json:"project_uid"`
	OldSettings ProjectSettings `json:"old_settings"`
	NewSettings ProjectSettings `json:"new_settings"`
}

// AddedUsers returns users present in new but absent from old for writers and auditors,
// paired with the role they were granted.
func (m *ProjectSettingsUpdatedMessage) AddedUsers() []AddedUser {
	var added []AddedUser
	added = append(added, diffUserLists(m.OldSettings.Writers, m.NewSettings.Writers, RoleManage)...)
	added = append(added, diffUserLists(m.OldSettings.Auditors, m.NewSettings.Auditors, RoleView)...)
	return added
}

func diffUserLists(old, newList []UserInfo, role Role) []AddedUser {
	oldSet := make(map[string]struct{}, len(old))
	for _, u := range old {
		oldSet[u.Username] = struct{}{}
	}

	var added []AddedUser
	for _, u := range newList {
		if u.Username == "" {
			// pending invite (no LFID yet) — not handled in this phase
			continue
		}
		if _, exists := oldSet[u.Username]; !exists {
			added = append(added, AddedUser{User: u, Role: role})
		}
	}
	return added
}

// Role represents the access level granted.
type Role string

const (
	// RoleManage maps to the writers permission set.
	RoleManage Role = "Manage"
	// RoleView maps to the auditors permission set.
	RoleView Role = "View"
)

// AddedUser pairs a user with the role they received.
type AddedUser struct {
	User UserInfo
	Role Role
}
