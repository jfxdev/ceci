package dto

type CreateEnvironmentRequest struct {
	Key  string `json:"key" binding:"required"`
	Name string `json:"name" binding:"required"`
}

type UpdateEnvironmentRequest struct {
	Name string `json:"name" binding:"required"`
}

type EnvironmentDTO struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}
