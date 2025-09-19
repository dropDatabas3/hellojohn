package core

import "time"

type Tenant struct {
	ID, Name, Slug string
	Settings       map[string]any
	CreatedAt      time.Time
}

type Client struct {
	ID, TenantID, Name, ClientID, ClientType string
	RedirectURIs, AllowedOrigins             []string
	Providers, Scopes                        []string
	ActiveVersionID                          *string
	CreatedAt                                time.Time
}

type ClientVersion struct {
	ID, ClientID, Version                               string
	ClaimSchemaJSON, ClaimMappingJSON, CryptoConfigJSON []byte
	Status                                              string // draft|active|retired
	CreatedAt                                           time.Time
	PromotedAt                                          *time.Time
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
