// Package auth contiene DTOs para endpoints de autenticación.
package auth

// LoginRequest representa la solicitud de login por password.
type LoginRequest struct {
	TenantID string `json:"tenant_id"`
	ClientID string `json:"client_id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse representa la respuesta exitosa de login.
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"` // "Bearer"
	ExpiresIn    int64  `json:"expires_in"` // segundos
	RefreshToken string `json:"refresh_token"`
}

// MFARequiredResponse representa la respuesta cuando MFA es requerido.
type MFARequiredResponse struct {
	MFARequired bool     `json:"mfa_required"`
	MFAToken    string   `json:"mfa_token"`
	AMR         []string `json:"amr"`
}

// LoginResult es el resultado interno del service (tokens o MFA).
type LoginResult struct {
	// Si Success=true, los tokens están disponibles
	Success      bool
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64

	// Si MFARequired=true, hay un challenge pendiente
	MFARequired bool
	MFAToken    string
	AMR         []string
}
