package dto

type CreateAPIKeyRequest struct {
	Label string `json:"label" binding:"required"`
}

type APIKeyCreatedDTO struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Prefix string `json:"prefix"`
	RawKey string `json:"rawKey"`
}

type APIKeyDTO struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	Prefix        string `json:"prefix"`
	EnvironmentID string `json:"environmentId"`
	Revoked       bool   `json:"revoked"`
}
