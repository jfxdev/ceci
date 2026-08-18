package dto

type ContextFieldValueInput struct {
	Value       string `json:"value" binding:"required"`
	Description string `json:"description"`
}

type CreateContextFieldRequest struct {
	Key         string                   `json:"key" binding:"required"`
	Description string                   `json:"description"`
	Values      []ContextFieldValueInput `json:"values"`
}

type UpdateContextFieldRequest struct {
	Description *string                   `json:"description"`
	Values      *[]ContextFieldValueInput `json:"values"`
}

type ContextFieldValueDTO struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

type ContextFieldDTO struct {
	ID          string                 `json:"id"`
	Key         string                 `json:"key"`
	Description string                 `json:"description,omitempty"`
	Values      []ContextFieldValueDTO `json:"values"`
}
