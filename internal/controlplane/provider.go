package controlplane

import (
	"context"
	"regexp"
	"strings"
)

var (
	reClientID = regexp.MustCompile(`^[a-z0-9\-_]+$`)
)

// ControlPlane define el contrato para cargar/escribir config de tenants/clients/scopes.
// MVP: sólo FS. Más adelante se puede agregar otro provider (BD, etc.).
type ControlPlane interface {
	// Tenants
	ListTenants(ctx context.Context) ([]Tenant, error)
	GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error)
	UpsertTenant(ctx context.Context, t *Tenant) error
	DeleteTenant(ctx context.Context, slug string) error

	// Scopes por tenant
	ListScopes(ctx context.Context, slug string) ([]Scope, error)
	UpsertScope(ctx context.Context, slug string, s Scope) error
	DeleteScope(ctx context.Context, slug, name string) error

	// Clients por tenant
	GetClient(ctx context.Context, slug, clientID string) (*OIDCClient, error)
	ListClients(ctx context.Context, slug string) ([]OIDCClient, error)
	UpsertClient(ctx context.Context, slug string, in ClientInput) (*OIDCClient, error)
	DeleteClient(ctx context.Context, slug, clientID string) error

	// Secretos (descifrado on-demand)
	DecryptClientSecret(ctx context.Context, slug, clientID string) (string, error)

	// Validaciones
	ValidateClientID(id string) bool
	ValidateRedirectURI(uri string) bool
	IsScopeAllowed(c *OIDCClient, scope string) bool

	// GetTenantByID looks up a tenant by its UUID
	GetTenantByID(ctx context.Context, id string) (*Tenant, error)
}

// Helpers default (pueden ser usados por FS provider)
func DefaultValidateClientID(id string) bool {
	return reClientID.MatchString(id)
}

// FSProviderInterface define métodos específicos del FSProvider que no están en ControlPlane
type FSProviderInterface interface {
	ControlPlane
	GetTenantRaw(ctx context.Context, slug string) (*Tenant, []byte, error)
	GetTenantSettingsRaw(ctx context.Context, slug string) (*TenantSettings, []byte, error)
	UpdateTenantSettings(ctx context.Context, slug string, settings *TenantSettings) error
	// FSRoot returns the root directory path used by the FS provider
	// FSRoot returns the root directory path used by the FS provider
	FSRoot() string
}

// AsFSProvider helper para convertir ControlPlane a FSProvider si es posible
func AsFSProvider(provider ControlPlane) (FSProviderInterface, bool) {
	if fs, ok := provider.(FSProviderInterface); ok {
		return fs, true
	}
	return nil, false
}

// Regla: https obligatorio salvo localhost (http://localhost o http://127.0.0.1)
func DefaultValidateRedirectURI(uri string) bool {
	u := strings.TrimSpace(strings.ToLower(uri))
	if strings.HasPrefix(u, "https://") {
		return true
	}
	if strings.HasPrefix(u, "http://localhost") || strings.HasPrefix(u, "http://127.0.0.1") {
		return true
	}
	return false
}

func DefaultIsScopeAllowed(c *OIDCClient, scope string) bool {
	s := strings.TrimSpace(scope)
	if s == "" {
		return false
	}
	for _, v := range c.Scopes {
		if v == s {
			return true
		}
	}
	return false
}
