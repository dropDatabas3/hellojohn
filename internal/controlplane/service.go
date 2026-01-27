// Package controlplane proporciona la capa de servicio para operaciones del Control Plane.
// Esta capa encapsula lógica de negocio (validaciones, cifrado, orquestación)
// y delega la persistencia a Store V2.
//
// Arquitectura:
//
//	┌─────────────────────────────────────────────────────────────┐
//	│                       HANDLERS                              │
//	└───────────────────────────┬─────────────────────────────────┘
//	                            │
//	                            ▼
//	┌─────────────────────────────────────────────────────────────┐
//	│                   CONTROLPLANE SERVICE                      │
//	│  • Validaciones (ClientID, RedirectURI, IssuerMode)         │
//	│  • Cifrado de secrets (SMTP, DSN, ClientSecret)             │
//	│  • Reglas de negocio                                        │
//	└───────────────────────────┬─────────────────────────────────┘
//	                            │
//	                            ▼
//	┌─────────────────────────────────────────────────────────────┐
//	│                       STORE V2                              │
//	│  • CRUD puro (sin lógica de negocio)                        │
//	│  • Multi-driver (FS, PG, etc.)                              │
//	└─────────────────────────────────────────────────────────────┘
//
// ─── Service Interface ───
//
//	// ─── Tenants ───
//	ListTenants(ctx context.Context) ([]repository.Tenant, error)
//	GetTenant(ctx context.Context, slug string) (*repository.Tenant, error)
//	GetTenantByID(ctx context.Context, id string) (*repository.Tenant, error)
//	CreateTenant(ctx context.Context, name, slug string) (*repository.Tenant, error)
//	UpdateTenant(ctx context.Context, tenant *repository.Tenant) error
//	DeleteTenant(ctx context.Context, slug string) error
//	UpdateTenantSettings(ctx context.Context, slug string, settings *repository.TenantSettings) error
//
//	// ─── Clients ───
//	ListClients(ctx context.Context, slug string) ([]repository.Client, error)
//	GetClient(ctx context.Context, slug, clientID string) (*repository.Client, error)
//	CreateClient(ctx context.Context, slug string, input ClientInput) (*repository.Client, error)
//	UpdateClient(ctx context.Context, slug string, input ClientInput) (*repository.Client, error)
//	DeleteClient(ctx context.Context, slug, clientID string) error
//	DecryptClientSecret(ctx context.Context, slug, clientID string) (string, error)
//
//	// ─── Scopes ───
//	ListScopes(ctx context.Context, slug string) ([]repository.Scope, error)
//	CreateScope(ctx context.Context, slug, name, description string) (*repository.Scope, error)
//	DeleteScope(ctx context.Context, slug, name string) error
//
//	// ─── Validations ───
//	ValidateClientID(id string) bool
//	ValidateRedirectURI(uri string) bool
//	IsScopeAllowed(client *repository.Client, scope string) bool
package controlplane

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	sec "github.com/dropDatabas3/hellojohn/internal/security/secretbox"
	store "github.com/dropDatabas3/hellojohn/internal/store"
	"github.com/google/uuid"
)

// ─── Errors ───

var (
	ErrBadInput       = errors.New("control plane: bad input")
	ErrTenantNotFound = errors.New("control plane: tenant not found")
	ErrClientNotFound = errors.New("control plane: client not found")
	ErrScopeInUse     = errors.New("control plane: scope in use by client")
)

// ─── Service Interface ───

// Service define las operaciones del Control Plane.
// Usa Store V2 internamente para persistencia.
type Service interface {
	// ─── Tenants ───
	ListTenants(ctx context.Context) ([]repository.Tenant, error)
	GetTenant(ctx context.Context, slug string) (*repository.Tenant, error)
	GetTenantByID(ctx context.Context, id string) (*repository.Tenant, error)
	CreateTenant(ctx context.Context, name, slug string, language string) (*repository.Tenant, error)
	UpdateTenant(ctx context.Context, tenant *repository.Tenant) error
	DeleteTenant(ctx context.Context, slug string) error
	UpdateTenantSettings(ctx context.Context, slug string, settings *repository.TenantSettings) error

	// ─── Clients ───
	ListClients(ctx context.Context, slug string) ([]repository.Client, error)
	GetClient(ctx context.Context, slug, clientID string) (*repository.Client, error)
	CreateClient(ctx context.Context, slug string, input ClientInput) (*repository.Client, error)
	UpdateClient(ctx context.Context, slug string, input ClientInput) (*repository.Client, error)
	DeleteClient(ctx context.Context, slug, clientID string) error
	DecryptClientSecret(ctx context.Context, slug, clientID string) (string, error)

	// ─── Scopes ───
	ListScopes(ctx context.Context, slug string) ([]repository.Scope, error)
	CreateScope(ctx context.Context, slug, name, description string) (*repository.Scope, error)
	DeleteScope(ctx context.Context, slug, name string) error

	// ─── Admins ───
	ListAdmins(ctx context.Context) ([]repository.Admin, error)
	GetAdmin(ctx context.Context, id string) (*repository.Admin, error)
	GetAdminByEmail(ctx context.Context, email string) (*repository.Admin, error)
	CreateAdmin(ctx context.Context, input CreateAdminInput) (*repository.Admin, error)
	UpdateAdmin(ctx context.Context, id string, input UpdateAdminInput) (*repository.Admin, error)
	DeleteAdmin(ctx context.Context, id string) error
	UpdateAdminPassword(ctx context.Context, id string, passwordHash string) error
	CheckAdminPassword(passwordHash, plainPassword string) bool

	// ─── Admin Refresh Tokens ───
	CreateAdminRefreshToken(ctx context.Context, input AdminRefreshTokenInput) error
	GetAdminRefreshToken(ctx context.Context, tokenHash string) (*AdminRefreshToken, error)
	DeleteAdminRefreshToken(ctx context.Context, tokenHash string) error
	CleanupExpiredAdminRefreshTokens(ctx context.Context) (int, error)

	// ─── Validations ───
	ValidateClientID(id string) bool
	ValidateRedirectURI(uri string) bool
	IsScopeAllowed(client *repository.Client, scope string) bool
}

// ClientInput contiene los datos para crear/actualizar un client.
type ClientInput struct {
	Name                     string
	ClientID                 string
	Type                     string // "public" | "confidential"
	RedirectURIs             []string
	AllowedOrigins           []string
	Providers                []string
	Scopes                   []string
	Secret                   string // Plain, se cifra al persistir
	RequireEmailVerification bool
	ResetPasswordURL         string
	VerifyEmailURL           string
	ClaimSchema              map[string]any
	ClaimMapping             map[string]any
}

// CreateAdminInput contiene los datos para crear un admin.
type CreateAdminInput struct {
	Email           string               // Required
	PasswordHash    string               // Required (ya hasheado con Argon2id)
	Name            string               // Optional
	Type            repository.AdminType // Required (global | tenant)
	AssignedTenants []string             // Optional (solo para AdminTypeTenant)
	CreatedBy       *string              // Optional (ID del admin que lo crea)
}

// UpdateAdminInput contiene los datos para actualizar un admin.
type UpdateAdminInput struct {
	Email           *string               // Optional
	Name            *string               // Optional
	Type            *repository.AdminType // Optional
	AssignedTenants *[]string             // Optional
	DisabledAt      *time.Time            // Optional (nil = enable, non-nil = disable)
}

// AdminRefreshTokenInput contiene los datos para crear un refresh token de admin.
type AdminRefreshTokenInput struct {
	AdminID   string    // ID del admin
	TokenHash string    // Hash SHA-256 del token
	ExpiresAt time.Time // Fecha de expiración
}

// AdminRefreshToken representa un refresh token de admin persistido.
type AdminRefreshToken struct {
	TokenHash string
	AdminID   string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// ─── Implementation ───

type service struct {
	store store.DataAccessLayer
}

// NewService crea un nuevo servicio de Control Plane.
func NewService(dal store.DataAccessLayer) Service {
	return &service{store: dal}
}

// ─── Tenants ───

func (s *service) ListTenants(ctx context.Context) ([]repository.Tenant, error) {
	return s.store.ConfigAccess().Tenants().List(ctx)
}

func (s *service) GetTenant(ctx context.Context, slug string) (*repository.Tenant, error) {
	tenant, err := s.store.ConfigAccess().Tenants().GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return tenant, nil
}

func (s *service) GetTenantByID(ctx context.Context, id string) (*repository.Tenant, error) {
	tenant, err := s.store.ConfigAccess().Tenants().GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return tenant, nil
}

func (s *service) CreateTenant(ctx context.Context, name, slug string, language string) (*repository.Tenant, error) {
	// Validaciones
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: name required", ErrBadInput)
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, fmt.Errorf("%w: slug required", ErrBadInput)
	}
	if !isValidSlug(slug) {
		return nil, fmt.Errorf("%w: invalid slug format", ErrBadInput)
	}
	if language == "" {
		language = DefaultLanguage // Idioma por defecto
	}

	now := time.Now().UTC()
	tenant := &repository.Tenant{
		ID:        uuid.NewString(),
		Slug:      slug,
		Name:      name,
		Language:  language,
		CreatedAt: now,
		UpdatedAt: now,
		Settings: repository.TenantSettings{
			Mailing: &repository.MailingSettings{
				Templates: DefaultEmailTemplates(),
			},
		},
	}

	if err := s.store.ConfigAccess().Tenants().Create(ctx, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

func (s *service) UpdateTenant(ctx context.Context, tenant *repository.Tenant) error {
	tenant.UpdatedAt = time.Now().UTC()
	return s.store.ConfigAccess().Tenants().Update(ctx, tenant)
}

func (s *service) DeleteTenant(ctx context.Context, slug string) error {
	return s.store.ConfigAccess().Tenants().Delete(ctx, slug)
}

func (s *service) UpdateTenantSettings(ctx context.Context, slug string, settings *repository.TenantSettings) error {
	// Cifrar campos sensibles
	if settings.SMTP != nil && settings.SMTP.Password != "" {
		enc, err := sec.Encrypt(settings.SMTP.Password)
		if err != nil {
			return fmt.Errorf("encrypt SMTP password: %w", err)
		}
		settings.SMTP.PasswordEnc = enc
		settings.SMTP.Password = "" // Limpiar plain
	}

	if settings.UserDB != nil && settings.UserDB.DSN != "" {
		enc, err := sec.Encrypt(settings.UserDB.DSN)
		if err != nil {
			return fmt.Errorf("encrypt UserDB DSN: %w", err)
		}
		settings.UserDB.DSNEnc = enc
		settings.UserDB.DSN = "" // Limpiar plain
	}

	if settings.Cache != nil && settings.Cache.Password != "" {
		enc, err := sec.Encrypt(settings.Cache.Password)
		if err != nil {
			return fmt.Errorf("encrypt Cache password: %w", err)
		}
		settings.Cache.PassEnc = enc
		settings.Cache.Password = "" // Limpiar plain
	}

	return s.store.ConfigAccess().Tenants().UpdateSettings(ctx, slug, settings)
}

// ─── Clients ───

func (s *service) ListClients(ctx context.Context, slug string) ([]repository.Client, error) {
	return s.store.ConfigAccess().Clients(slug).List(ctx, slug, "")
}

func (s *service) GetClient(ctx context.Context, slug, clientID string) (*repository.Client, error) {
	client, err := s.store.ConfigAccess().Clients(slug).Get(ctx, slug, clientID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrClientNotFound
		}
		return nil, err
	}
	return client, nil
}

func (s *service) CreateClient(ctx context.Context, slug string, input ClientInput) (*repository.Client, error) {
	// Validaciones
	if !s.ValidateClientID(input.ClientID) {
		return nil, fmt.Errorf("%w: invalid clientId", ErrBadInput)
	}
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("%w: name required", ErrBadInput)
	}
	if input.Type != "public" && input.Type != "confidential" {
		return nil, fmt.Errorf("%w: invalid client type", ErrBadInput)
	}
	if len(input.RedirectURIs) == 0 {
		return nil, fmt.Errorf("%w: redirectUris required", ErrBadInput)
	}
	for _, uri := range input.RedirectURIs {
		if !s.ValidateRedirectURI(uri) {
			return nil, fmt.Errorf("%w: invalid redirect uri: %s", ErrBadInput, uri)
		}
	}

	// Cifrar secret para confidential clients
	var secretEnc string
	if input.Type == "confidential" {
		if input.Secret == "" {
			return nil, fmt.Errorf("%w: secret required for confidential client", ErrBadInput)
		}
		enc, err := sec.Encrypt(input.Secret)
		if err != nil {
			return nil, fmt.Errorf("encrypt client secret: %w", err)
		}
		secretEnc = enc
	}

	repoInput := repository.ClientInput{
		Name:                     input.Name,
		ClientID:                 input.ClientID,
		Type:                     input.Type,
		RedirectURIs:             uniqueStrings(input.RedirectURIs),
		AllowedOrigins:           uniqueStrings(input.AllowedOrigins),
		Providers:                uniqueStrings(input.Providers),
		Scopes:                   uniqueStrings(input.Scopes),
		Secret:                   secretEnc, // Ya cifrado
		RequireEmailVerification: input.RequireEmailVerification,
		ResetPasswordURL:         input.ResetPasswordURL,
		VerifyEmailURL:           input.VerifyEmailURL,
		ClaimSchema:              input.ClaimSchema,
		ClaimMapping:             input.ClaimMapping,
	}

	return s.store.ConfigAccess().Clients(slug).Create(ctx, slug, repoInput)
}

func (s *service) UpdateClient(ctx context.Context, slug string, input ClientInput) (*repository.Client, error) {
	// Validaciones similares a Create
	if !s.ValidateClientID(input.ClientID) {
		return nil, fmt.Errorf("%w: invalid clientId", ErrBadInput)
	}
	for _, uri := range input.RedirectURIs {
		if !s.ValidateRedirectURI(uri) {
			return nil, fmt.Errorf("%w: invalid redirect uri: %s", ErrBadInput, uri)
		}
	}

	// Cifrar secret si viene nuevo
	var secretEnc string
	if input.Type == "confidential" && input.Secret != "" {
		enc, err := sec.Encrypt(input.Secret)
		if err != nil {
			return nil, fmt.Errorf("encrypt client secret: %w", err)
		}
		secretEnc = enc
	}

	repoInput := repository.ClientInput{
		Name:                     input.Name,
		ClientID:                 input.ClientID,
		Type:                     input.Type,
		RedirectURIs:             uniqueStrings(input.RedirectURIs),
		AllowedOrigins:           uniqueStrings(input.AllowedOrigins),
		Providers:                uniqueStrings(input.Providers),
		Scopes:                   uniqueStrings(input.Scopes),
		Secret:                   secretEnc,
		RequireEmailVerification: input.RequireEmailVerification,
		ResetPasswordURL:         input.ResetPasswordURL,
		VerifyEmailURL:           input.VerifyEmailURL,
		ClaimSchema:              input.ClaimSchema,
		ClaimMapping:             input.ClaimMapping,
	}

	return s.store.ConfigAccess().Clients(slug).Update(ctx, slug, repoInput)
}

func (s *service) DeleteClient(ctx context.Context, slug, clientID string) error {
	return s.store.ConfigAccess().Clients(slug).Delete(ctx, slug, clientID)
}

func (s *service) DecryptClientSecret(ctx context.Context, slug, clientID string) (string, error) {
	client, err := s.GetClient(ctx, slug, clientID)
	if err != nil {
		return "", err
	}
	if client.SecretEnc == "" {
		return "", nil
	}
	return sec.Decrypt(client.SecretEnc)
}

// ─── Scopes ───

func (s *service) ListScopes(ctx context.Context, slug string) ([]repository.Scope, error) {
	return s.store.ConfigAccess().Scopes(slug).List(ctx, slug)
}

func (s *service) CreateScope(ctx context.Context, slug, name, description string) (*repository.Scope, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: scope name required", ErrBadInput)
	}
	return s.store.ConfigAccess().Scopes(slug).Create(ctx, slug, name, description)
}

func (s *service) DeleteScope(ctx context.Context, slug, name string) error {
	// Verificar que no esté en uso por algún client
	clients, err := s.ListClients(ctx, slug)
	if err != nil {
		return err
	}
	for _, c := range clients {
		for _, sc := range c.Scopes {
			if sc == name {
				return ErrScopeInUse
			}
		}
	}
	return s.store.ConfigAccess().Scopes(slug).Delete(ctx, slug, name)
}

// ─── Validations ───

var reClientID = regexp.MustCompile(`^[a-z0-9\-_]+$`)

func (s *service) ValidateClientID(id string) bool {
	id = strings.TrimSpace(id)
	return len(id) >= 3 && len(id) <= 64 && reClientID.MatchString(id)
}

func (s *service) ValidateRedirectURI(uri string) bool {
	uri = strings.ToLower(strings.TrimSpace(uri))
	if strings.HasPrefix(uri, "https://") {
		return true
	}
	if strings.HasPrefix(uri, "http://localhost") || strings.HasPrefix(uri, "http://127.0.0.1") {
		return true
	}
	return false
}

func (s *service) IsScopeAllowed(client *repository.Client, scope string) bool {
	scope = strings.TrimSpace(scope)
	for _, s := range client.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// ─── Helpers ───

func isValidSlug(s string) bool {
	return regexp.MustCompile(`^[a-z0-9\-]+$`).MatchString(s)
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
