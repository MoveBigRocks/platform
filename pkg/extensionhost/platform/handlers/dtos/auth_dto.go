package dtos

import (
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

type ContextResponse struct {
	Type          string  `json:"type"`
	WorkspaceID   *string `json:"workspace_id,omitempty"`
	WorkspaceName *string `json:"workspace_name,omitempty"`
	WorkspaceSlug *string `json:"workspace_slug,omitempty"`
	Role          string  `json:"role"`
}

func ToContextResponse(c platformdomain.Context) ContextResponse {
	return ContextResponse{
		Type:          string(c.Type),
		WorkspaceID:   c.WorkspaceID,
		WorkspaceName: c.WorkspaceName,
		WorkspaceSlug: c.WorkspaceSlug,
		Role:          c.Role,
	}
}

func ToContextResponseList(contexts []platformdomain.Context) []ContextResponse {
	responses := make([]ContextResponse, len(contexts))
	for i, ctx := range contexts {
		responses[i] = ToContextResponse(ctx)
	}
	return responses
}

type AuthRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Honeypot string `json:"honeypot"`
}

type TokenExchangeRequest struct {
	Token string `json:"token" binding:"required"`
}
