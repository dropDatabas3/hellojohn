package admin

// TenantSettingsResponse represents tenant settings in API responses.
// This DTO mirrors repository.TenantSettings but provides API stability.
// Uses camelCase for consistency with the domain model and frontend.
type TenantSettingsResponse struct {
	// Core Settings
	IssuerMode     string  `json:"issuerMode"`               // "path" | "subdomain" | "global"
	IssuerOverride *string `json:"issuerOverride,omitempty"` // Custom issuer URL

	// Session Configuration
	SessionLifetimeSeconds      int `json:"sessionLifetimeSeconds,omitempty"`
	RefreshTokenLifetimeSeconds int `json:"refreshTokenLifetimeSeconds,omitempty"`

	// Feature Flags
	MFAEnabled         bool `json:"mfaEnabled"`
	SocialLoginEnabled bool `json:"socialLoginEnabled"`

	// Infrastructure Settings
	UserDB   *UserDBSettings   `json:"userDb,omitempty"`
	SMTP     *SMTPSettings     `json:"smtp,omitempty"`
	Cache    *CacheSettings    `json:"cache,omitempty"`
	Security *SecuritySettings `json:"security,omitempty"`

	// Branding
	LogoURL    string `json:"logoUrl,omitempty"`
	BrandColor string `json:"brandColor,omitempty"`

	// Social Providers
	SocialProviders *SocialProvidersConfig `json:"socialProviders,omitempty"`

	// Custom User Fields
	UserFields []UserFieldDefinition `json:"userFields,omitempty"`
}

// UserDBSettings configures the tenant's user database.
type UserDBSettings struct {
	Driver string `json:"driver"`            // "postgres" | "mysql" | "mongo"
	DSN    string `json:"dsn,omitempty"`     // Plain DSN (only in requests)
	DSNEnc string `json:"dsnEnc,omitempty"`  // Encrypted DSN (in responses)
	Schema string `json:"schema,omitempty"`  // Database schema name
}

// SMTPSettings configures email sending for the tenant.
type SMTPSettings struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`    // Plain password (only in requests)
	PasswordEnc string `json:"passwordEnc,omitempty"` // Encrypted password (in responses)
	FromEmail   string `json:"fromEmail"`
	UseTLS      bool   `json:"useTLS"`
}

// CacheSettings configures caching for the tenant.
type CacheSettings struct {
	Enabled  bool   `json:"enabled"`
	Driver   string `json:"driver"`             // "memory" | "redis"
	Host     string `json:"host,omitempty"`     // Redis host
	Port     int    `json:"port,omitempty"`     // Redis port
	Password string `json:"password,omitempty"` // Plain (only in requests)
	PassEnc  string `json:"passEnc,omitempty"`  // Encrypted (in responses)
	DB       int    `json:"db,omitempty"`       // Redis DB number
	Prefix   string `json:"prefix,omitempty"`   // Key prefix
}

// SecuritySettings defines security policies.
type SecuritySettings struct {
	PasswordMinLength int  `json:"passwordMinLength,omitempty"`
	MFARequired       bool `json:"mfaRequired"`
}

// SocialProvidersConfig configures social login providers.
type SocialProvidersConfig struct {
	GoogleEnabled   bool   `json:"googleEnabled"`
	GoogleClient    string `json:"googleClient,omitempty"`
	GoogleSecret    string `json:"googleSecret,omitempty"`    // Plain (only in requests)
	GoogleSecretEnc string `json:"googleSecretEnc,omitempty"` // Encrypted (in responses)
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
// Uses camelCase for consistency with the domain model and frontend.
type UpdateTenantSettingsRequest struct {
	// Core Settings
	IssuerMode     *string `json:"issuerMode,omitempty"`
	IssuerOverride *string `json:"issuerOverride,omitempty"`

	// Session Configuration
	SessionLifetimeSeconds      *int `json:"sessionLifetimeSeconds,omitempty"`
	RefreshTokenLifetimeSeconds *int `json:"refreshTokenLifetimeSeconds,omitempty"`

	// Feature Flags
	MFAEnabled         *bool `json:"mfaEnabled,omitempty"`
	SocialLoginEnabled *bool `json:"socialLoginEnabled,omitempty"`

	// Infrastructure Settings
	UserDB   *UserDBSettings        `json:"userDb,omitempty"`
	SMTP     *SMTPSettings          `json:"smtp,omitempty"`
	Cache    *CacheSettings         `json:"cache,omitempty"`
	Security *SecuritySettings      `json:"security,omitempty"`

	// Branding
	LogoURL    *string `json:"logoUrl,omitempty"`
	BrandColor *string `json:"brandColor,omitempty"`

	// Social Providers
	SocialProviders *SocialProvidersConfig `json:"socialProviders,omitempty"`

	// Custom User Fields
	UserFields []UserFieldDefinition `json:"userFields,omitempty"`
}
