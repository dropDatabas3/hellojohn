package admin

// TenantSettingsResponse represents tenant settings in API responses.
// This DTO mirrors repository.TenantSettings but provides API stability.
type TenantSettingsResponse struct {
	// Core Settings
	IssuerMode     string  `json:"issuer_mode"`               // "path" | "subdomain" | "global"
	IssuerOverride *string `json:"issuer_override,omitempty"` // Custom issuer URL

	// Session Configuration
	SessionLifetimeSeconds      int `json:"session_lifetime_seconds,omitempty"`
	RefreshTokenLifetimeSeconds int `json:"refresh_token_lifetime_seconds,omitempty"`

	// Feature Flags
	MFAEnabled         bool `json:"mfa_enabled"`
	SocialLoginEnabled bool `json:"social_login_enabled"`

	// Infrastructure Settings
	UserDB   *UserDBSettings   `json:"user_db,omitempty"`
	SMTP     *SMTPSettings     `json:"smtp,omitempty"`
	Cache    *CacheSettings    `json:"cache,omitempty"`
	Security *SecuritySettings `json:"security,omitempty"`

	// Branding
	LogoURL    string `json:"logo_url,omitempty"`
	BrandColor string `json:"brand_color,omitempty"`

	// Social Providers
	SocialProviders *SocialProvidersConfig `json:"social_providers,omitempty"`

	// Custom User Fields
	UserFields []UserFieldDefinition `json:"user_fields,omitempty"`
}

// UserDBSettings configures the tenant's user database.
type UserDBSettings struct {
	Driver string `json:"driver"`           // "postgres" | "mysql" | "mongo"
	DSN    string `json:"dsn,omitempty"`    // Plain DSN (only in requests)
	DSNEnc string `json:"dsn_enc,omitempty"` // Encrypted DSN (in responses)
	Schema string `json:"schema,omitempty"` // Database schema name
}

// SMTPSettings configures email sending for the tenant.
type SMTPSettings struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`     // Plain password (only in requests)
	PasswordEnc string `json:"password_enc,omitempty"` // Encrypted password (in responses)
	FromEmail   string `json:"from_email"`
	UseTLS      bool   `json:"use_tls"`
}

// CacheSettings configures caching for the tenant.
type CacheSettings struct {
	Enabled  bool   `json:"enabled"`
	Driver   string `json:"driver"`           // "memory" | "redis"
	Host     string `json:"host,omitempty"`   // Redis host
	Port     int    `json:"port,omitempty"`   // Redis port
	Password string `json:"password,omitempty"` // Plain (only in requests)
	PassEnc  string `json:"pass_enc,omitempty"` // Encrypted (in responses)
	DB       int    `json:"db,omitempty"`     // Redis DB number
	Prefix   string `json:"prefix,omitempty"` // Key prefix
}

// SecuritySettings defines security policies.
type SecuritySettings struct {
	PasswordMinLength int  `json:"password_min_length,omitempty"`
	MFARequired       bool `json:"mfa_required"`
}

// SocialProvidersConfig configures social login providers.
type SocialProvidersConfig struct {
	GoogleEnabled   bool   `json:"google_enabled"`
	GoogleClient    string `json:"google_client,omitempty"`
	GoogleSecret    string `json:"google_secret,omitempty"`     // Plain (only in requests)
	GoogleSecretEnc string `json:"google_secret_enc,omitempty"` // Encrypted (in responses)
}

// UserFieldDefinition defines a custom user field.
type UserFieldDefinition struct {
	Name        string `json:"name"`
	Type        string `json:"type"`         // "string" | "number" | "boolean" | "date"
	Required    bool   `json:"required"`
	Unique      bool   `json:"unique"`
	Indexed     bool   `json:"indexed"`
	Description string `json:"description,omitempty"`
}

// UpdateTenantSettingsRequest represents a partial update to tenant settings.
// All fields are optional to support partial updates.
type UpdateTenantSettingsRequest struct {
	// Core Settings
	IssuerMode     *string `json:"issuer_mode,omitempty"`
	IssuerOverride *string `json:"issuer_override,omitempty"`

	// Session Configuration
	SessionLifetimeSeconds      *int `json:"session_lifetime_seconds,omitempty"`
	RefreshTokenLifetimeSeconds *int `json:"refresh_token_lifetime_seconds,omitempty"`

	// Feature Flags
	MFAEnabled         *bool `json:"mfa_enabled,omitempty"`
	SocialLoginEnabled *bool `json:"social_login_enabled,omitempty"`

	// Infrastructure Settings
	UserDB   *UserDBSettings        `json:"user_db,omitempty"`
	SMTP     *SMTPSettings          `json:"smtp,omitempty"`
	Cache    *CacheSettings         `json:"cache,omitempty"`
	Security *SecuritySettings      `json:"security,omitempty"`

	// Branding
	LogoURL    *string `json:"logo_url,omitempty"`
	BrandColor *string `json:"brand_color,omitempty"`

	// Social Providers
	SocialProviders *SocialProvidersConfig `json:"social_providers,omitempty"`

	// Custom User Fields
	UserFields []UserFieldDefinition `json:"user_fields,omitempty"`
}
