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
	LogoURL                     string
	BrandColor                  string
	SessionLifetimeSeconds      int
	RefreshTokenLifetimeSeconds int
	MFAEnabled                  bool
	SocialLoginEnabled          bool
	SMTP                        *SMTPSettings
	UserDB                      *UserDBSettings
	Cache                       *CacheSettings
	Security                    *SecurityPolicy
	UserFields                  []UserFieldDefinition
	Mailing                     *MailingSettings
	// IssuerMode configura cómo se construye el issuer/JWKS por tenant.
	IssuerMode     string
	IssuerOverride string
}

// SMTPSettings configuración de email.
type SMTPSettings struct {
	Host        string
	Port        int
	Username    string
	Password    string // Plain (no persiste)
	PasswordEnc string // Encrypted
	FromEmail   string
	UseTLS      bool
}

// UserDBSettings configuración de DB por tenant.
type UserDBSettings struct {
	Driver string
	DSN    string // Plain (no persiste)
	DSNEnc string // Encrypted
	Schema string
}

// CacheSettings configuración de cache por tenant.
type CacheSettings struct {
	Enabled  bool
	Driver   string
	Host     string
	Port     int
	Password string
	PassEnc  string
	DB       int
	Prefix   string
}

// SecurityPolicy políticas de seguridad.
type SecurityPolicy struct {
	PasswordMinLength int
	MFARequired       bool
}

// UserFieldDefinition define un campo custom de usuario.
type UserFieldDefinition struct {
	Name        string
	Type        string
	Required    bool
	Unique      bool
	Indexed     bool
	Description string
}

// MailingSettings configuración de templates de email.
type MailingSettings struct {
	// Templates organizados por idioma: map[lang]map[templateID]EmailTemplate
	// Ejemplo: Templates["es"]["verify_email"] = EmailTemplate{...}
	Templates map[string]map[string]EmailTemplate
}

// EmailTemplate un template de email.
type EmailTemplate struct {
	Subject string
	Body    string
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
