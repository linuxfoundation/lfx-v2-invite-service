// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"
	"time"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
)

// InviteStore is a test double for port.InviteStore.
type InviteStore struct {
	CreateFunc       func(ctx context.Context, record *model.InviteRecord) error
	GetByUIDFunc     func(ctx context.Context, uid string) (*model.InviteRecord, error)
	GetByEmailFunc   func(ctx context.Context, email string) ([]*model.InviteRecord, error)
	MarkAcceptedFunc func(ctx context.Context, uid, username string, at time.Time) error

	CreateCalls       []*model.InviteRecord
	GetByUIDCalls     []string
	GetByEmailCalls   []string
	MarkAcceptedCalls []MarkAcceptedCall
}

// MarkAcceptedCall records the arguments of a single MarkAccepted call.
type MarkAcceptedCall struct {
	UID      string
	Username string
	At       time.Time
}

func (m *InviteStore) Create(ctx context.Context, record *model.InviteRecord) error {
	m.CreateCalls = append(m.CreateCalls, record)
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, record)
	}
	return nil
}

func (m *InviteStore) GetByUID(ctx context.Context, uid string) (*model.InviteRecord, error) {
	m.GetByUIDCalls = append(m.GetByUIDCalls, uid)
	if m.GetByUIDFunc != nil {
		return m.GetByUIDFunc(ctx, uid)
	}
	return nil, nil
}

func (m *InviteStore) GetByEmail(ctx context.Context, email string) ([]*model.InviteRecord, error) {
	m.GetByEmailCalls = append(m.GetByEmailCalls, email)
	if m.GetByEmailFunc != nil {
		return m.GetByEmailFunc(ctx, email)
	}
	return nil, nil
}

func (m *InviteStore) MarkAccepted(ctx context.Context, uid, username string, at time.Time) error {
	m.MarkAcceptedCalls = append(m.MarkAcceptedCalls, MarkAcceptedCall{UID: uid, Username: username, At: at})
	if m.MarkAcceptedFunc != nil {
		return m.MarkAcceptedFunc(ctx, uid, username, at)
	}
	return nil
}
