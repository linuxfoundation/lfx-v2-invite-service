// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/constants"
)

// ProjectNameReader implements port.ProjectNameReader via NATS request/reply.
type ProjectNameReader struct {
	client *Client
}

// NewProjectNameReader creates a ProjectNameReader backed by the given NATS client.
func NewProjectNameReader(client *Client) *ProjectNameReader {
	return &ProjectNameReader{client: client}
}

// GetProjectName requests the project name from project-service via NATS request/reply.
// The request payload is the raw project UID bytes; the response is the raw project name bytes.
func (r *ProjectNameReader) GetProjectName(ctx context.Context, projectUID string) (string, error) {
	data, err := r.client.Request(ctx, constants.ProjectGetNameSubject, []byte(projectUID))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
