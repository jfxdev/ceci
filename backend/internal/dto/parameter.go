package dto

type UpsertParameterRequest struct {
	Value string `json:"value" binding:"required"`
}

type ParameterDTO struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Version   int    `json:"version"`
	UpdatedAt string `json:"updatedAt"`
}

type ParameterVersionDTO struct {
	Version    int    `json:"version"`
	Value      string `json:"value"`
	ChangeType string `json:"changeType"`
	ChangedAt  string `json:"changedAt"`
}
