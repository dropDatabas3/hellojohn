package core

import "time"

type Tenant struct {
	ID, Name, Slug string
	Settings       map[string]any
	CreatedAt      time.Time
}

type Client struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenant_id"`
	Name            string    `json:"name"`
	ClientID        string    `json:"client_id"`
	ClientType      string    `json:"client_type"`
	RedirectURIs    []string  `json:"redirect_uris"`
	AllowedOrigins  []string  `json:"allowed_origins"`
	Providers       []string  `json:"providers"`
	Scopes          []string  `json:"scopes"`
	ActiveVersionID *string   `json:"active_version_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type ClientVersion struct {
	ID               string     `json:"id"`
	ClientID         string     `json:"client_id"`
	Version          string     `json:"version"`
	ClaimSchemaJSON  []byte     `json:"claim_schema_json"`
	ClaimMappingJSON []byte     `json:"claim_mapping_json"`
	CryptoConfigJSON []byte     `json:"crypto_config_json"`
	Status           string     `json:"status"` // draft|active|retired
	CreatedAt        time.Time  `json:"created_at"`
	PromotedAt       *time.Time `json:"promoted_at,omitempty"`
}

type User struct {
	ID, TenantID, Email, Status string
	EmailVerified               bool
	Metadata                    map[string]any
	CreatedAt                   time.Time
}

type Identity struct {
	ID, UserID, Provider, ProviderUserID, Email string
	EmailVerified                               bool
	PasswordHash                                *string
	CreatedAt                                   time.Time
}

type RefreshToken struct {
	ID          string
	UserID      string
	ClientID    string
	TokenHash   string
	IssuedAt    time.Time
	ExpiresAt   time.Time
	RotatedFrom *string
	RevokedAt   *time.Time
}

// MFA / TOTP secret metadata
type MFATOTP struct {
	UserID          string
	SecretEncrypted string
	ConfirmedAt     *time.Time
	LastUsedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
