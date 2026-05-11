// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import "context"

// ProjectNameReader is a test double for port.ProjectNameReader.
type ProjectNameReader struct {
	GetProjectNameFunc func(ctx context.Context, projectUID string) (string, error)
	Calls              []string
}

func (m *ProjectNameReader) GetProjectName(ctx context.Context, projectUID string) (string, error) {
	m.Calls = append(m.Calls, projectUID)
	if m.GetProjectNameFunc != nil {
		return m.GetProjectNameFunc(ctx, projectUID)
	}
	return "", nil
}
