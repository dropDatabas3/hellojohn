package social

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane/v1"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
)

// ClientConfigDeps contains dependencies for client config service.
type ClientConfigDeps struct {
	TenantProvider TenantProvider
}

// clientConfigService implements ClientConfigService.
type clientConfigService struct {
	tenantProvider TenantProvider
}

// NewClientConfigService creates a new ClientConfigService.
func NewClientConfigService(d ClientConfigDeps) ClientConfigService {
	return &clientConfigService{
		tenantProvider: d.TenantProvider,
	}
}

// GetClient returns the client configuration for a tenant/clientID pair.
func (s *clientConfigService) GetClient(ctx context.Context, tenantSlug, clientID string) (*controlplane.OIDCClient, error) {
	log := logger.From(ctx).With(logger.Layer("service"), logger.Component("social.clientconfig"))

	if tenantSlug == "" {
		return nil, ErrTenantRequired
	}
	if clientID == "" {
		return nil, ErrClientRequired
	}

	tenant, err := s.tenantProvider.GetTenantBySlug(ctx, tenantSlug)
	if err != nil {
		log.Warn("tenant not found", logger.TenantID(tenantSlug), logger.Err(err))
		return nil, fmt.Errorf("%w: tenant not found", ErrTenantRequired)
	}

	for i := range tenant.Clients {
		if tenant.Clients[i].ClientID == clientID {
			return &tenant.Clients[i], nil
		}
	}

	log.Warn("client not found", logger.TenantID(tenantSlug), logger.String("client_id", clientID))
	return nil, ErrClientNotFound
}

// ValidateRedirectURI validates that a redirect URI is allowed for a client.
func (s *clientConfigService) ValidateRedirectURI(ctx context.Context, tenantSlug, clientID, redirectURI string) error {
	log := logger.From(ctx).With(logger.Layer("service"), logger.Component("social.clientconfig"))

	if redirectURI == "" {
		return nil // Empty redirect is allowed (optional param)
	}

	client, err := s.GetClient(ctx, tenantSlug, clientID)
	if err != nil {
		return err
	}

	// Canonicalize the input URI
	canonical, err := canonicalizeRedirect(redirectURI)
	if err != nil {
		log.Warn("redirect_uri invalid", logger.String("redirect_uri", redirectURI), logger.Err(err))
		return fmt.Errorf("%w: %v", ErrRedirectInvalid, err)
	}

	// Check against allowlist
	for _, allowed := range client.RedirectURIs {
		allowedCanonical, err := canonicalizeRedirect(allowed)
		if err != nil {
			continue // Skip malformed entries
		}
		if canonical == allowedCanonical {
			return nil // Match found
		}
	}

	log.Warn("redirect_uri not allowed",
		logger.String("redirect_uri", redirectURI),
		logger.TenantID(tenantSlug),
		logger.String("client_id", clientID),
	)
	return ErrRedirectNotAllowed
}

// IsProviderAllowed checks if a social provider is allowed for a client.
func (s *clientConfigService) IsProviderAllowed(ctx context.Context, tenantSlug, clientID, provider string) error {
	log := logger.From(ctx).With(logger.Layer("service"), logger.Component("social.clientconfig"))

	if tenantSlug == "" {
		return ErrTenantRequired
	}

	tenant, err := s.tenantProvider.GetTenantBySlug(ctx, tenantSlug)
	if err != nil {
		return fmt.Errorf("%w: tenant not found", ErrTenantRequired)
	}

	// Check global social login enabled
	if !tenant.Settings.SocialLoginEnabled {
		log.Warn("social login disabled for tenant", logger.TenantID(tenantSlug))
		return ErrSocialLoginDisabled
	}

	// Get client
	client, err := s.GetClient(ctx, tenantSlug, clientID)
	if err != nil {
		return err
	}

	// Check provider in client.Providers list
	providerFound := false
	for _, p := range client.Providers {
		if strings.EqualFold(p, provider) {
			providerFound = true
			break
		}
	}
	if !providerFound {
		log.Warn("provider not in client.Providers",
			logger.String("provider", provider),
			logger.TenantID(tenantSlug),
			logger.String("client_id", clientID),
		)
		return ErrProviderNotAllowed
	}

	// Get effective social config (client override or tenant default)
	cfg := tenant.Settings.SocialProviders
	if client.SocialProviders != nil {
		cfg = client.SocialProviders
	}

	// Validate provider-specific config
	switch strings.ToLower(provider) {
	case "google":
		if cfg == nil || !cfg.GoogleEnabled {
			log.Warn("google not enabled in social config",
				logger.TenantID(tenantSlug),
				logger.String("client_id", clientID),
			)
			return ErrProviderNotAllowed
		}
		if cfg.GoogleClient == "" || cfg.GoogleSecret == "" {
			log.Error("google misconfigured (missing client_id or secret)",
				logger.TenantID(tenantSlug),
				logger.String("client_id", clientID),
			)
			return ErrProviderMisconfigured
		}
	default:
		log.Warn("unknown provider", logger.String("provider", provider))
		return ErrProviderNotAllowed
	}

	return nil
}

// GetSocialConfig returns the effective social config for a client.
func (s *clientConfigService) GetSocialConfig(ctx context.Context, tenantSlug, clientID string) (*controlplane.SocialConfig, error) {
	tenant, err := s.tenantProvider.GetTenantBySlug(ctx, tenantSlug)
	if err != nil {
		return nil, fmt.Errorf("%w: tenant not found", ErrTenantRequired)
	}

	client, err := s.GetClient(ctx, tenantSlug, clientID)
	if err != nil {
		return nil, err
	}

	// Client override takes precedence
	if client.SocialProviders != nil {
		return client.SocialProviders, nil
	}

	return tenant.Settings.SocialProviders, nil
}

// canonicalizeRedirect canonicalizes a redirect URI for comparison.
func canonicalizeRedirect(uri string) (string, error) {
	if uri == "" {
		return "", fmt.Errorf("empty URI")
	}

	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Must be absolute URL
	if !u.IsAbs() {
		return "", fmt.Errorf("must be absolute URL")
	}

	// Fragment prohibited
	if u.Fragment != "" {
		return "", fmt.Errorf("fragment not allowed")
	}

	// Scheme must be http or https
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}

	// Require https except for localhost
	host := strings.ToLower(u.Hostname())
	if scheme == "http" && !isLocalhost(host) {
		return "", fmt.Errorf("https required for non-localhost")
	}

	// Normalize host to lowercase
	u.Host = strings.ToLower(u.Host)
	u.Scheme = scheme

	// Normalize empty path to "/"
	if u.Path == "" {
		u.Path = "/"
	}

	// Remove default ports
	port := u.Port()
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		u.Host = u.Hostname()
	}

	// Rebuild canonical string (preserves query, removes fragment)
	u.Fragment = ""
	return u.String(), nil
}

// isLocalhost checks if a host is localhost or loopback.
func isLocalhost(host string) bool {
	host = strings.ToLower(host)
	return host == "localhost" ||
		host == "127.0.0.1" ||
		host == "::1" ||
		host == "[::1]" ||
		strings.HasPrefix(host, "127.")
}
