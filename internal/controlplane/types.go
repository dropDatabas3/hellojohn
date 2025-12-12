// internal/controlplane/types.go
package controlplane

import "time"

// ClientType define el tipo de cliente OIDC.
type ClientType string

const (
	ClientTypePublic       ClientType = "public"
	ClientTypeConfidential ClientType = "confidential"
)

// TenantSettings: branding, SMTP, base de datos del user-plane, políticas, etc.
type TenantSettings struct {
	LogoURL                     string                `json:"logoUrl,omitempty" yaml:"logoUrl,omitempty"`
	BrandColor                  string                `json:"brandColor,omitempty" yaml:"brandColor,omitempty"`
	SessionLifetimeSeconds      int                   `json:"session_lifetime_seconds,omitempty" yaml:"session_lifetime_seconds,omitempty"`
	RefreshTokenLifetimeSeconds int                   `json:"refresh_token_lifetime_seconds,omitempty" yaml:"refresh_token_lifetime_seconds,omitempty"`
	MFAEnabled                  bool                  `json:"mfa_enabled,omitempty" yaml:"mfa_enabled,omitempty"`
	SocialLoginEnabled          bool                  `json:"social_login_enabled,omitempty" yaml:"social_login_enabled,omitempty"`
	SMTP                        *SMTPSettings         `json:"smtp,omitempty" yaml:"smtp,omitempty"`
	UserDB                      *UserDBSettings       `json:"userDb,omitempty" yaml:"userDb,omitempty"`
	Cache                       *CacheSettings        `json:"cache,omitempty" yaml:"cache,omitempty"`
	Security                    *SecurityPolicy       `json:"security,omitempty" yaml:"security,omitempty"`
	IssuerMode                  IssuerMode            `json:"issuerMode,omitempty" yaml:"issuerMode,omitempty"`
	IssuerOverride              string                `json:"issuerOverride,omitempty" yaml:"issuerOverride,omitempty"`
	SocialProviders             *SocialConfig         `json:"socialProviders,omitempty" yaml:"socialProviders,omitempty"`
	Forms                       *FormsSettings        `json:"forms,omitempty" yaml:"forms,omitempty"`
	UserFields                  []UserFieldDefinition `json:"user_fields,omitempty" yaml:"user_fields,omitempty"`
	Mailing                     *MailingSettings      `json:"mailing,omitempty" yaml:"mailing,omitempty"`
}

type MailingSettings struct {
	Templates map[string]EmailTemplate `json:"templates,omitempty" yaml:"templates,omitempty"`
}

type EmailTemplate struct {
	Subject string `json:"subject,omitempty" yaml:"subject,omitempty"`
	Body    string `json:"body,omitempty" yaml:"body,omitempty"` // HTML support allowed
}

type UserFieldDefinition struct {
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"` // text, int, boolean, date
	Required    bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Unique      bool   `json:"unique,omitempty" yaml:"unique,omitempty"`
	Indexed     bool   `json:"indexed,omitempty" yaml:"indexed,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// FormsSettings stores configuration for login and register forms.
type FormsSettings struct {
	Login    *FormConfig `json:"login,omitempty" yaml:"login,omitempty"`
	Register *FormConfig `json:"register,omitempty" yaml:"register,omitempty"`
}

// FormConfig defines the structure and style of a form.
type FormConfig struct {
	Theme        FormTheme    `json:"theme" yaml:"theme"`
	Steps        []FormStep   `json:"steps" yaml:"steps"`
	SocialLayout SocialLayout `json:"socialLayout,omitempty" yaml:"socialLayout,omitempty"`
}

// SocialLayout defines how social login buttons are displayed.
type SocialLayout struct {
	Position string `json:"position" yaml:"position"` // top, bottom
	Style    string `json:"style" yaml:"style"`       // grid, list
}

// FormStep defines a single step in a multi-step form.
type FormStep struct {
	ID          string      `json:"id" yaml:"id"`
	Title       string      `json:"title" yaml:"title"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	Fields      []FormField `json:"fields" yaml:"fields"`
}

// FormTheme defines colors and styles.
type FormTheme struct {
	// Colors
	PrimaryColor    string `json:"primaryColor" yaml:"primaryColor"`
	BackgroundColor string `json:"backgroundColor" yaml:"backgroundColor"`
	TextColor       string `json:"textColor" yaml:"textColor"`

	// Typography
	FontFamily   string `json:"fontFamily,omitempty" yaml:"fontFamily,omitempty"`
	HeadingColor string `json:"headingColor,omitempty" yaml:"headingColor,omitempty"`

	// Components
	BorderRadius string      `json:"borderRadius" yaml:"borderRadius"`
	InputStyle   InputStyle  `json:"inputStyle,omitempty" yaml:"inputStyle,omitempty"`
	ButtonStyle  ButtonStyle `json:"buttonStyle,omitempty" yaml:"buttonStyle,omitempty"`

	// Layout
	Spacing    string `json:"spacing,omitempty" yaml:"spacing,omitempty"` // compact, normal, relaxed
	ShowLabels bool   `json:"showLabels" yaml:"showLabels"`
	LogoUrl    string `json:"logoUrl,omitempty" yaml:"logoUrl,omitempty"`
}

type InputStyle struct {
	Variant     string `json:"variant" yaml:"variant"` // outlined, filled, underlined
	BorderColor string `json:"borderColor,omitempty" yaml:"borderColor,omitempty"`
}

type ButtonStyle struct {
	Variant   string `json:"variant" yaml:"variant"`                   // solid, outline, ghost
	Shadow    string `json:"shadow,omitempty" yaml:"shadow,omitempty"` // none, sm, md, lg
	FullWidth bool   `json:"fullWidth" yaml:"fullWidth"`
}

// FormField defines a single input field.
type FormField struct {
	ID          string `json:"id" yaml:"id"`
	Type        string `json:"type" yaml:"type"` // email, password, text, number, phone
	Label       string `json:"label" yaml:"label"`
	Placeholder string `json:"placeholder,omitempty" yaml:"placeholder,omitempty"`
	Required    bool   `json:"required" yaml:"required"`
	Name        string `json:"name" yaml:"name"` // Field name for submission
	HelpText    string `json:"helpText,omitempty" yaml:"helpText,omitempty"`

	// Validation
	MinLength int    `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength int    `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Pattern   string `json:"pattern,omitempty" yaml:"pattern,omitempty"` // Regex
}

// IssuerMode configura cómo se construye el issuer/JWKS por tenant.
type IssuerMode string

const (
	IssuerModeGlobal IssuerMode = "global" // default (compat hacia atrás)
	IssuerModePath   IssuerMode = "path"   // MVP F5: iss = {base}/t/{slug}; jwks = /.well-known/jwks/{slug}.json
	IssuerModeDomain IssuerMode = "domain" // futuro (no bloquea F5)
)

// Tenant representa un arrendatario (aislamiento lógico).
type Tenant struct {
	// UUID en string (evita dependencia a libs externas). Validar formato al cargar.
	ID          string         `json:"id" yaml:"id"`
	Name        string         `json:"name" yaml:"name"`
	DisplayName string         `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Slug        string         `json:"slug" yaml:"slug"` // único; usado en paths/URLs
	CreatedAt   time.Time      `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt" yaml:"updatedAt"`
	Settings    TenantSettings `json:"settings" yaml:"settings"`
	Clients     []OIDCClient   `json:"clients,omitempty" yaml:"clients,omitempty"`
	Scopes      []Scope        `json:"scopes,omitempty" yaml:"scopes,omitempty"`
}

// CacheSettings: configuración de caché por tenant (Redis/Mem).
type CacheSettings struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Driver   string `json:"driver,omitempty" yaml:"driver,omitempty"` // redis|memory
	Host     string `json:"host,omitempty" yaml:"host,omitempty"`
	Port     int    `json:"port,omitempty" yaml:"port,omitempty"`
	Password string `json:"password,omitempty" yaml:"-"`                // Plain input
	PassEnc  string `json:"passEnc,omitempty" yaml:"passEnc,omitempty"` // Encrypted
	DB       int    `json:"db,omitempty" yaml:"db,omitempty"`
	Prefix   string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
}

// SMTPSettings: credenciales cifradas con secretbox (passwordEnc).
type SMTPSettings struct {
	Host        string `json:"host" yaml:"host"`
	Port        int    `json:"port" yaml:"port"`
	Username    string `json:"username,omitempty" yaml:"username,omitempty"`
	PasswordEnc string `json:"passwordEnc,omitempty" yaml:"passwordEnc,omitempty"` // Encrypt(...)
	Password    string `json:"password,omitempty" yaml:"-"`                        // Plain input (no persiste)
	FromEmail   string `json:"fromEmail,omitempty" yaml:"fromEmail,omitempty"`
	UseTLS      bool   `json:"useTLS,omitempty" yaml:"useTLS,omitempty"`
}

// UserDBSettings: DSN cifrada para el user-plane por tenant (opcional).
type UserDBSettings struct {
	Driver string `json:"driver,omitempty" yaml:"driver,omitempty"` // pg|mysql|mongo|...
	DSNEnc string `json:"dsnEnc,omitempty" yaml:"dsnEnc,omitempty"` // Encrypt(...)
	DSN    string `json:"dsn,omitempty" yaml:"-"`                   // Plain input (no persiste)
	Schema string `json:"schema,omitempty" yaml:"schema,omitempty"` // p.ej. schema por tenant
}

// SecurityPolicy: políticas base (MVP).
type SecurityPolicy struct {
	PasswordMinLength int  `json:"passwordMinLength,omitempty" yaml:"passwordMinLength,omitempty"`
	MFARequired       bool `json:"mfaRequired,omitempty" yaml:"mfaRequired,omitempty"`
}

// OIDCClient: definición de cliente por tenant.
type OIDCClient struct {
	ClientID        string        `json:"clientId" yaml:"clientId"` // único por tenant
	Name            string        `json:"name" yaml:"name"`
	Type            ClientType    `json:"type" yaml:"type"`                                         // public|confidential
	RedirectURIs    []string      `json:"redirectUris" yaml:"redirectUris"`                         // match exacto; https salvo localhost
	AllowedOrigins  []string      `json:"allowedOrigins,omitempty" yaml:"allowedOrigins,omitempty"` // CORS
	Providers       []string      `json:"providers,omitempty" yaml:"providers,omitempty"`           // ej: ["password","google"]
	Scopes          []string      `json:"scopes,omitempty" yaml:"scopes,omitempty"`                 // p.ej. ["openid","email","profile","admin"]
	SecretEnc       string        `json:"secretEnc,omitempty" yaml:"secretEnc,omitempty"`           // Encrypt(...), solo para confidential
	SocialProviders *SocialConfig `json:"socialProviders,omitempty" yaml:"socialProviders,omitempty"`

	// Email verification & password reset configuration
	RequireEmailVerification bool   `json:"requireEmailVerification,omitempty" yaml:"requireEmailVerification,omitempty"`
	ResetPasswordURL         string `json:"resetPasswordUrl,omitempty" yaml:"resetPasswordUrl,omitempty"`
	VerifyEmailURL           string `json:"verifyEmailUrl,omitempty" yaml:"verifyEmailUrl,omitempty"`

	// MVP: guardamos "versión activa" inline; más adelante se puede expandir a array Versiones. “versión activa” inline; más adelante se puede expandir a array Versiones.
	ClaimSchema  map[string]any `json:"claimSchema,omitempty" yaml:"claimSchema,omitempty"`
	ClaimMapping map[string]any `json:"claimMapping,omitempty" yaml:"claimMapping,omitempty"`
}

// SocialConfig: habilitación/config de IdPs sociales (MVP).
type SocialConfig struct {
	GoogleEnabled bool   `json:"googleEnabled,omitempty" yaml:"googleEnabled,omitempty"`
	GoogleClient  string `json:"googleClient,omitempty" yaml:"googleClient,omitempty"`
	GoogleSecret  string `json:"googleSecretEnc,omitempty" yaml:"googleSecretEnc,omitempty"` // Encrypt(...)
}

// Scope: scopes custom por tenant.
type Scope struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	System      bool   `json:"system,omitempty" yaml:"system,omitempty"` // openid/email/profile => true
}

// ---- DTOs Admin API (payloads crudos) ----

type TenantCreateRequest struct {
	Name string `json:"name" yaml:"name"`
	Slug string `json:"slug" yaml:"slug"`
}

type ClientInput struct {
	Name           string     `json:"name" yaml:"name"`
	ClientID       string     `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	Type           ClientType `json:"type" yaml:"type"`
	RedirectURIs   []string   `json:"redirectUris" yaml:"redirectUris"`
	AllowedOrigins []string   `json:"allowedOrigins,omitempty" yaml:"allowedOrigins,omitempty"`
	Providers      []string   `json:"providers,omitempty" yaml:"providers,omitempty"`
	Scopes         []string   `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	Secret         string     `json:"secret,omitempty" yaml:"secret,omitempty"` // plain entrante; se cifra al persistir

	// Email verification & password reset
	RequireEmailVerification bool   `json:"requireEmailVerification,omitempty" yaml:"requireEmailVerification,omitempty"`
	ResetPasswordURL         string `json:"resetPasswordUrl,omitempty" yaml:"resetPasswordUrl,omitempty"`
	VerifyEmailURL           string `json:"verifyEmailUrl,omitempty" yaml:"verifyEmailUrl,omitempty"`

	// Opcionales: configuración de claims por versión simple en FS (MVP)
	ClaimSchema  map[string]any `json:"claimSchema,omitempty" yaml:"claimSchema,omitempty"`
	ClaimMapping map[string]any `json:"claimMapping,omitempty" yaml:"claimMapping,omitempty"`
}
