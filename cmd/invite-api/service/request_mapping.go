// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

// apiToModelRequest converts a pkg/api.SendInviteRequest (the public NATS wire
// contract) to the internal domain model. This is the single place the two types
// are mapped — field additions to api.SendInviteRequest must be reflected here
// so omissions are caught at review rather than silently passing empty values.
func apiToModelRequest(r api.SendInviteRequest) model.SendInviteRequest {
	m := model.SendInviteRequest{
		// Deprecated scalars — copied verbatim; resolution logic lives in the
		// Resolved*() methods on model.SendInviteRequest and is unchanged.
		RecipientEmail: r.RecipientEmail, //nolint:staticcheck
		RecipientName:  r.RecipientName,  //nolint:staticcheck
		InviterName:    r.InviterName,    //nolint:staticcheck
		ResourceUID:    r.ResourceUID,    //nolint:staticcheck
		ResourceName:   r.ResourceName,   //nolint:staticcheck
		ResourceType:   r.ResourceType,   //nolint:staticcheck
		Role:           r.Role,
		ReturnURL:      r.ReturnURL,
		OrgName:        r.OrgName,
		ExpirationDays: r.ExpirationDays,
		CustomClaims:   r.CustomClaims,
	}
	if r.Recipient != nil {
		m.Recipient = &model.Recipient{
			Name:     r.Recipient.Name,
			Email:    r.Recipient.Email,
			Username: r.Recipient.Username,
			Avatar:   r.Recipient.Avatar,
		}
	}
	if r.Inviter != nil {
		m.Inviter = &model.Inviter{
			Name:     r.Inviter.Name,
			Username: r.Inviter.Username,
			Email:    r.Inviter.Email,
			Avatar:   r.Inviter.Avatar,
		}
	}
	if r.Resource != nil {
		m.Resource = &model.InviteResource{
			UID:  r.Resource.UID,
			Name: r.Resource.Name,
			Type: r.Resource.Type,
		}
	}
	return m
}
