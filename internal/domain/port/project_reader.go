// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// ProjectNameReader retrieves a project's display name by UID.
// Implemented via NATS request/reply to the project-service.
type ProjectNameReader interface {
	GetProjectName(ctx context.Context, projectUID string) (string, error)
}
