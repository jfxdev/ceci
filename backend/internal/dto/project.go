package dto

import "ceci/backend/internal/constants"

type CreateProjectRequest struct {
	Name string `json:"name" binding:"required"`
	Slug string `json:"slug" binding:"required"`
}

type UpdateProjectRequest struct {
	Name string `json:"name" binding:"required"`
}

type ProjectDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Role string `json:"role,omitempty"`
}

type AddMemberRequest struct {
	Email string                `json:"email" binding:"required,email"`
	Role  constants.ProjectRole `json:"role" binding:"required"`
}

type UpdateMemberRequest struct {
	Role constants.ProjectRole `json:"role" binding:"required"`
}

type MemberDTO struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}
