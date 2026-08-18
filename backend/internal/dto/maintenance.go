package dto

type MaintenanceStatusDTO struct {
	Enabled   bool    `json:"enabled"`
	Message   string  `json:"message"`
	StartedAt *string `json:"startedAt,omitempty"`
	StartedBy string  `json:"startedBy,omitempty"`
}

type UpdateMaintenanceRequest struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
}
