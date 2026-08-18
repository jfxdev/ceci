package dto

import "leaflag/backend/internal/constants"

type AccessGroupDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CreateAccessGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type AddAccessGroupMemberRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type OIDCAccessGroupMappingDTO struct { ExternalGroup string `json:"externalGroup"` }

type AddOIDCAccessGroupMappingRequest struct { ExternalGroup string `json:"externalGroup" binding:"required"` }

type ProjectAccessGroupDTO struct {
	GroupID     string `json:"groupId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Role        string `json:"role"`
}

type GrantProjectAccessGroupRequest struct {
	GroupID string                `json:"groupId" binding:"required,uuid"`
	Role    constants.ProjectRole `json:"role" binding:"required"`
}

type UpdateProjectAccessGroupRequest struct {
	Role constants.ProjectRole `json:"role" binding:"required"`
}
