package social

import (
	"context"
	"errors"

	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane/v1"
)

// TenantProvider abstracts control plane access for testing.
type TenantProvider interface {
	GetTenantBySlug(ctx context.Context, slug string) (*controlplane.Tenant, error)
}

// ClientConfigService validates client configuration from control plane.
type ClientConfigService interface {
	// GetClient returns the client configuration for a tenant/clientID pair.
	GetClient(ctx context.Context, tenantSlug, clientID string) (*controlplane.OIDCClient, error)
	// ValidateRedirectURI validates that a redirect URI is allowed for a client.
	ValidateRedirectURI(ctx context.Context, tenantSlug, clientID, redirectURI string) error
	// IsProviderAllowed checks if a social provider is allowed for a client.
	// Returns nil if allowed, error otherwise.
	IsProviderAllowed(ctx context.Context, tenantSlug, clientID, provider string) error
	// GetSocialConfig returns the effective social config for a client (client override or tenant default).
	GetSocialConfig(ctx context.Context, tenantSlug, clientID string) (*controlplane.SocialConfig, error)
}

// Errors for client config service.
var (
	ErrTenantRequired        = errors.New("tenant_slug required")
	ErrClientRequired        = errors.New("client_id required")
	ErrClientNotFound        = errors.New("client not found")
	ErrRedirectInvalid       = errors.New("redirect_uri invalid")
	ErrRedirectNotAllowed    = errors.New("redirect_uri not allowed")
	ErrProviderNotAllowed    = errors.New("provider not allowed")
	ErrProviderMisconfigured = errors.New("provider misconfigured")
	ErrSocialLoginDisabled   = errors.New("social login disabled for tenant")
)
