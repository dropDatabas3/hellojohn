// internal/controlplane/types.go
package controlplane

import "time"

// ClientType define el tipo de cliente OIDC.
type ClientType string

const (
	ClientTypePublic       ClientType = "public"
	ClientTypeConfidential ClientType = "confidential"
)

// Tenant representa un arrendatario (aislamiento lógico).
type Tenant struct {
	// UUID en string (evita dependencia a libs externas). Validar formato al cargar.
	ID        string         `json:"id" yaml:"id"`
	Name      string         `json:"name" yaml:"name"`
	Slug      string         `json:"slug" yaml:"slug"` // único; usado en paths/URLs
	CreatedAt time.Time      `json:"createdAt" yaml:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt" yaml:"updatedAt"`
	Settings  TenantSettings `json:"settings" yaml:"settings"`
	Clients   []OIDCClient   `json:"clients,omitempty" yaml:"clients,omitempty"`
	Scopes    []Scope        `json:"scopes,omitempty" yaml:"scopes,omitempty"`
}

// TenantSettings: branding, SMTP, base de datos del user-plane, políticas, etc.
type TenantSettings struct {
	LogoURL        string          `json:"logoUrl,omitempty" yaml:"logoUrl,omitempty"`
	BrandColor     string          `json:"brandColor,omitempty" yaml:"brandColor,omitempty"`
	SMTP           *SMTPSettings   `json:"smtp,omitempty" yaml:"smtp,omitempty"`
	UserDB         *UserDBSettings `json:"userDb,omitempty" yaml:"userDb,omitempty"`
	Security       *SecurityPolicy `json:"security,omitempty" yaml:"security,omitempty"`
	IssuerMode     IssuerMode      `json:"issuerMode,omitempty" yaml:"issuerMode,omitempty"`
	IssuerOverride string          `json:"issuerOverride,omitempty" yaml:"issuerOverride,omitempty"`
}

// IssuerMode configura cómo se construye el issuer/JWKS por tenant.
type IssuerMode string

const (
	IssuerModeGlobal IssuerMode = "global" // default (compat hacia atrás)
	IssuerModePath   IssuerMode = "path"   // MVP F5: iss = {base}/t/{slug}; jwks = /.well-known/jwks/{slug}.json
	IssuerModeDomain IssuerMode = "domain" // futuro (no bloquea F5)
)

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
	// MVP: guardamos “versión activa” inline; más adelante se puede expandir a array Versiones.
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
}
