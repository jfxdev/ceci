package dto

import (
	"encoding/json"
	"strings"
	"unicode"
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"required"`
}

type UpdateUserPreferencesRequest struct {
	Locale string `json:"locale" binding:"required,oneof=en pt-BR"`
}

type LoginResponse struct {
	AccessToken string  `json:"accessToken"`
	User        UserDTO `json:"user"`
}

type UserDTO struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"isAdmin"`
	Locale  string `json:"locale"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// MarshalJSON keeps the legacy error text while ensuring control-plane errors
// always expose a deterministic code clients can localize.
func (e ErrorResponse) MarshalJSON() ([]byte, error) {
	code := e.Code
	if code == "" {
		var b strings.Builder
		for _, r := range strings.ToLower(e.Error) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteRune(r)
			} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "_") {
				b.WriteByte('_')
			}
		}
		code = "error." + strings.Trim(b.String(), "_")
	}
	return json.Marshal(struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}{Error: e.Error, Code: code})
}
