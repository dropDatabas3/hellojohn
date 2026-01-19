package repository

import (
	"context"
)

const (
	ClientTypePublic       = "public"
	ClientTypeConfidential = "confidential"
)

// Client representa un cliente OIDC/OAuth.
type Client struct {
	ID                       string
	TenantID                 string
	ClientID                 string // identificador público
	Name                     string
	Type                     string // "public" | "confidential"
	RedirectURIs             []string
	AllowedOrigins           []string
	Providers                []string
	Scopes                   []string
	SecretEnc                string // Secret cifrado
	RequireEmailVerification bool
	ResetPasswordURL         string
	VerifyEmailURL           string
	ClaimSchema              map[string]any
	ClaimMapping             map[string]any
	SocialProviders          *SocialConfig // Override de configuración social por cliente
}

// ClientVersion representa una versión de configuración de un client.
type ClientVersion struct {
	ID               string
	ClientID         string
	Version          string
	Status           string // "draft" | "active" | "retired"
	ClaimSchemaJSON  []byte
	ClaimMappingJSON []byte
	CryptoConfigJSON []byte
}

// ClientInput contiene los datos para crear/actualizar un client.
type ClientInput struct {
	Name                     string
	ClientID                 string
	Type                     string
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

// ClientRepository define operaciones sobre OIDC clients.
type ClientRepository interface {
	// Get obtiene un client por su client_id público.
	// Retorna ErrNotFound si no existe.
	Get(ctx context.Context, tenantID, clientID string) (*Client, error)

	// GetByUUID obtiene un client por su UUID interno.
	GetByUUID(ctx context.Context, uuid string) (*Client, *ClientVersion, error)

	// List lista todos los clients de un tenant.
	// El parámetro query permite filtrar por nombre/client_id.
	List(ctx context.Context, tenantID string, query string) ([]Client, error)

	// Create crea un nuevo client.
	// Retorna ErrConflict si el client_id ya existe.
	Create(ctx context.Context, tenantID string, input ClientInput) (*Client, error)

	// Update actualiza un client existente.
	Update(ctx context.Context, tenantID string, input ClientInput) (*Client, error)

	// Delete elimina un client.
	Delete(ctx context.Context, tenantID, clientID string) error

	// DecryptSecret descifra y retorna el secret de un client confidential.
	DecryptSecret(ctx context.Context, tenantID, clientID string) (string, error)

	// ─── Validaciones ───

	// ValidateClientID verifica que el client_id tenga formato válido.
	ValidateClientID(id string) bool

	// ValidateRedirectURI verifica que la URI sea válida.
	ValidateRedirectURI(uri string) bool

	// IsScopeAllowed verifica si un scope está permitido para el client.
	IsScopeAllowed(client *Client, scope string) bool
}
