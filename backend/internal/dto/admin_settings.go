package dto

type EnvironmentTemplateDTO struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	Name       string `json:"name"`
	IsRequired bool   `json:"isRequired"`
}

type CreateEnvironmentTemplateRequest struct {
	Key        string `json:"key" binding:"required"`
	Name       string `json:"name" binding:"required"`
	IsRequired bool   `json:"isRequired"`
}

type UpdateEnvironmentTemplateRequest struct {
	Name       string `json:"name" binding:"required"`
	IsRequired bool   `json:"isRequired"`
}
