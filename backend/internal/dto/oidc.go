package dto

type OIDCConfigurationDTO struct {
	Enabled         bool   `json:"enabled"`
	IssuerURL       string `json:"issuerUrl"`
	ClientID        string `json:"clientId"`
	RedirectURL     string `json:"redirectUrl"`
	JITEnabled      bool   `json:"jitEnabled"`
	HasClientSecret bool   `json:"hasClientSecret"`
}
type UpdateOIDCConfigurationRequest struct {
	Enabled      bool   `json:"enabled"`
	IssuerURL    string `json:"issuerUrl"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RedirectURL  string `json:"redirectUrl"`
	JITEnabled   bool   `json:"jitEnabled"`
}
