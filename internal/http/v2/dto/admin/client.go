// Package admin contiene DTOs para endpoints administrativos.
package admin

// ClientRequest representa la entrada para crear o actualizar un client.
type ClientRequest struct {
	Name                     string   `json:"name"`
	ClientID                 string   `json:"client_id"`
	Type                     string   `json:"type"` // "public" | "confidential"
	RedirectURIs             []string `json:"redirect_uris,omitempty"`
	AllowedOrigins           []string `json:"allowed_origins,omitempty"`
	Providers                []string `json:"providers,omitempty"`
	Scopes                   []string `json:"scopes,omitempty"`
	Secret                   string   `json:"secret,omitempty"`
	RequireEmailVerification bool     `json:"require_email_verification,omitempty"`
	ResetPasswordURL         string   `json:"reset_password_url,omitempty"`
	VerifyEmailURL           string   `json:"verify_email_url,omitempty"`
}

// ClientResponse representa un client en la respuesta.
type ClientResponse struct {
	ID                       string   `json:"id"`
	Name                     string   `json:"name"`
	ClientID                 string   `json:"client_id"`
	Type                     string   `json:"type"`
	RedirectURIs             []string `json:"redirect_uris,omitempty"`
	AllowedOrigins           []string `json:"allowed_origins,omitempty"`
	Providers                []string `json:"providers,omitempty"`
	Scopes                   []string `json:"scopes,omitempty"`
	SecretHash               string   `json:"secret_hash,omitempty"`
	RequireEmailVerification bool     `json:"require_email_verification,omitempty"`
	ResetPasswordURL         string   `json:"reset_password_url,omitempty"`
	VerifyEmailURL           string   `json:"verify_email_url,omitempty"`
	CreatedAt                string   `json:"created_at,omitempty"`
	UpdatedAt                string   `json:"updated_at,omitempty"`
}

// StatusResponse es una respuesta genérica de estado.
type StatusResponse struct {
	Status string `json:"status"`
}
