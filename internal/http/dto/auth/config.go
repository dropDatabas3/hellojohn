package auth

// ConfigRequest holds query params for GET /v2/auth/config
type ConfigRequest struct {
	ClientID string `json:"client_id"`
}

// CustomFieldSchema defines a custom field for the UI.
type CustomFieldSchema struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "text", "number", "boolean"
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

// ConfigResponse is the public config for frontend auth UI.
type ConfigResponse struct {
	TenantName      string              `json:"tenant_name"`
	TenantSlug      string              `json:"tenant_slug"`
	ClientName      string              `json:"client_name"`
	LogoURL         string              `json:"logo_url,omitempty"`
	PrimaryColor    string              `json:"primary_color,omitempty"`
	SocialProviders []string            `json:"social_providers"`
	PasswordEnabled bool                `json:"password_enabled"`
	Features        map[string]bool     `json:"features,omitempty"`
	CustomFields    []CustomFieldSchema `json:"custom_fields,omitempty"`

	// Email verification & password reset URLs
	RequireEmailVerification bool   `json:"require_email_verification,omitempty"`
	ResetPasswordURL         string `json:"reset_password_url,omitempty"`
	VerifyEmailURL           string `json:"verify_email_url,omitempty"`
}

// ConfigResult is the internal result from ConfigService.
type ConfigResult struct {
	TenantName      string
	TenantSlug      string
	ClientName      string
	LogoURL         string
	PrimaryColor    string
	SocialProviders []string
	PasswordEnabled bool
	Features        map[string]bool
	CustomFields    []CustomFieldSchema

	RequireEmailVerification bool
	ResetPasswordURL         string
	VerifyEmailURL           string
}
