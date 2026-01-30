package repository

import (
	"context"
	"time"
)

// Tenant representa un arrendatario del sistema.
type Tenant struct {
	ID          string
	Slug        string
	Name        string
	DisplayName string
	Language    string // Idioma por defecto del tenant ("es", "en")
	Settings    TenantSettings
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TenantSettings contiene la configuración de un tenant.
type TenantSettings struct {
	LogoURL                     string                `json:"logoUrl" yaml:"logoUrl"`
	BrandColor                  string                `json:"brandColor" yaml:"brandColor"`
	SecondaryColor              string                `json:"secondaryColor" yaml:"secondaryColor"`
	FaviconURL                  string                `json:"faviconUrl" yaml:"faviconUrl"`
	SessionLifetimeSeconds      int                   `json:"sessionLifetimeSeconds" yaml:"sessionLifetimeSeconds"`
	RefreshTokenLifetimeSeconds int                   `json:"refreshTokenLifetimeSeconds" yaml:"refreshTokenLifetimeSeconds"`
	MFAEnabled                  bool                  `json:"mfaEnabled" yaml:"mfaEnabled"`
	SocialLoginEnabled          bool                  `json:"social_login_enabled" yaml:"social_login_enabled"`
	SMTP                        *SMTPSettings         `json:"smtp,omitempty" yaml:"smtp,omitempty"`
	UserDB                      *UserDBSettings       `json:"userDb,omitempty" yaml:"userDb,omitempty"`
	Cache                       *CacheSettings        `json:"cache,omitempty" yaml:"cache,omitempty"`
	Security                    *SecurityPolicy       `json:"security,omitempty" yaml:"security,omitempty"`
	UserFields                  []UserFieldDefinition `json:"userFields,omitempty" yaml:"userFields,omitempty"`
	Mailing                     *MailingSettings      `json:"mailing,omitempty" yaml:"mailing,omitempty"`
	// IssuerMode configura cómo se construye el issuer/JWKS por tenant.
	IssuerMode      string                 `json:"issuerMode,omitempty" yaml:"issuerMode,omitempty"`
	IssuerOverride  string                 `json:"issuerOverride,omitempty" yaml:"issuerOverride,omitempty"`
	SocialProviders *SocialConfig          `json:"socialProviders,omitempty" yaml:"socialProviders,omitempty"`
	ConsentPolicy   *ConsentPolicySettings `json:"consentPolicy,omitempty" yaml:"consentPolicy,omitempty"`
}

// SMTPSettings configuración de email.
type SMTPSettings struct {
	Host        string `json:"host" yaml:"host"`
	Port        int    `json:"port" yaml:"port"`
	Username    string `json:"username" yaml:"username"`
	Password    string `json:"password,omitempty" yaml:"-"`    // Plain (no persiste)
	PasswordEnc string `json:"-" yaml:"passwordEnc,omitempty"` // Encrypted
	FromEmail   string `json:"fromEmail" yaml:"fromEmail"`
	UseTLS      bool   `json:"useTLS" yaml:"useTLS"`
}

// UserDBSettings configuración de DB por tenant.
type UserDBSettings struct {
	Driver     string `json:"driver" yaml:"driver"`
	DSN        string `json:"dsn,omitempty" yaml:"-"`    // Plain (no persiste)
	DSNEnc     string `json:"-" yaml:"dsnEnc,omitempty"` // Encrypted
	Schema     string `json:"schema,omitempty" yaml:"schema,omitempty"`
	ManualMode bool   `json:"manualMode,omitempty" yaml:"manualMode,omitempty"`
}

// CacheSettings configuración de cache por tenant.
type CacheSettings struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Driver   string `json:"driver" yaml:"driver"`
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Password string `json:"password,omitempty" yaml:"-"` // Plain (no persiste)
	PassEnc  string `json:"-" yaml:"passEnc,omitempty"`  // Encrypted
	DB       int    `json:"db" yaml:"db"`
	Prefix   string `json:"prefix" yaml:"prefix"`
}

// SecurityPolicy políticas de seguridad.
type SecurityPolicy struct {
	PasswordMinLength      int  `json:"passwordMinLength" yaml:"passwordMinLength"`
	RequireUppercase       bool `json:"requireUppercase" yaml:"requireUppercase"`
	RequireNumbers         bool `json:"requireNumbers" yaml:"requireNumbers"`
	RequireSpecialChars    bool `json:"requireSpecialChars" yaml:"requireSpecialChars"`
	MFARequired            bool `json:"mfaRequired" yaml:"mfaRequired"`
	MaxLoginAttempts       int  `json:"maxLoginAttempts" yaml:"maxLoginAttempts"`
	LockoutDurationMinutes int  `json:"lockoutDurationMinutes" yaml:"lockoutDurationMinutes"`
}

// UserFieldDefinition define un campo custom de usuario.
type UserFieldDefinition struct {
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"`
	Required    bool   `json:"required" yaml:"required"`
	Unique      bool   `json:"unique" yaml:"unique"`
	Indexed     bool   `json:"indexed" yaml:"indexed"`
	Description string `json:"description" yaml:"description"`
}

// MailingSettings configuración de templates de email.
type MailingSettings struct {
	// Templates organizados por idioma: map[lang]map[templateID]EmailTemplate
	// Ejemplo: Templates["es"]["verify_email"] = EmailTemplate{...}
	Templates map[string]map[string]EmailTemplate `json:"templates" yaml:"templates"`
}

// EmailTemplate un template de email.
type EmailTemplate struct {
	Subject string `json:"subject" yaml:"subject"`
	Body    string `json:"body" yaml:"body"`
}

// TenantRepository define operaciones sobre tenants (Control Plane).
// Este repositorio opera sobre la configuración global, no sobre datos de usuarios.
type TenantRepository interface {
	// List retorna todos los tenants.
	List(ctx context.Context) ([]Tenant, error)

	// GetBySlug busca un tenant por su slug.
	GetBySlug(ctx context.Context, slug string) (*Tenant, error)

	// GetByID busca un tenant por su UUID.
	GetByID(ctx context.Context, id string) (*Tenant, error)

	// Create crea un nuevo tenant.
	// Retorna ErrConflict si el slug ya existe.
	Create(ctx context.Context, tenant *Tenant) error

	// Update actualiza un tenant existente.
	Update(ctx context.Context, tenant *Tenant) error

	// Delete elimina un tenant y toda su configuración.
	Delete(ctx context.Context, slug string) error

	// UpdateSettings actualiza solo los settings de un tenant.
	// Cifra automáticamente campos sensibles.
	UpdateSettings(ctx context.Context, slug string, settings *TenantSettings) error
}

// ConsentPolicySettings configuración de políticas de consentimiento.
type ConsentPolicySettings struct {
	ConsentMode                   string `json:"consent_mode" yaml:"consentMode"`                           // "per_scope" | "single"
	ExpirationDays                *int   `json:"expiration_days,omitempty" yaml:"expirationDays,omitempty"` // null = never expires
	RepromptDays                  *int   `json:"reprompt_days,omitempty" yaml:"repromptDays,omitempty"`     // null = never reprompt
	RememberScopeDecisions        bool   `json:"remember_scope_decisions" yaml:"rememberScopeDecisions"`
	ShowConsentScreen             bool   `json:"show_consent_screen" yaml:"showConsentScreen"`
	AllowSkipConsentForFirstParty bool   `json:"allow_skip_consent_for_first_party" yaml:"allowSkipConsentForFirstParty"`
}

// SocialConfig: habilitación/config de IdPs sociales.
type SocialConfig struct {
	// Google OAuth
	GoogleEnabled   bool   `json:"googleEnabled" yaml:"googleEnabled"`
	GoogleClient    string `json:"googleClient" yaml:"googleClient"`
	GoogleSecret    string `json:"googleSecret,omitempty" yaml:"-"`                            // Plain (input)
	GoogleSecretEnc string `json:"googleSecretEnc,omitempty" yaml:"googleSecretEnc,omitempty"` // Encrypted (persisted)

	// GitHub OAuth
	GitHubEnabled   bool   `json:"githubEnabled" yaml:"githubEnabled"`
	GitHubClient    string `json:"githubClient" yaml:"githubClient"`
	GitHubSecret    string `json:"githubSecret,omitempty" yaml:"-"`                            // Plain (input)
	GitHubSecretEnc string `json:"githubSecretEnc,omitempty" yaml:"githubSecretEnc,omitempty"` // Encrypted (persisted)
}
