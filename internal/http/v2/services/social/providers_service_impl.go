package social

import (
	"context"
	"os"
	"strings"
)

// ProvidersDeps contains dependencies for ProvidersService.
type ProvidersDeps struct {
	// ConfiguredProviders is a list of enabled provider names.
	// If nil, will read from SOCIAL_PROVIDERS env var.
	ConfiguredProviders []string
}

// providersService implements ProvidersService.
type providersService struct {
	providers []string
}

// NewProvidersService creates a new ProvidersService.
func NewProvidersService(d ProvidersDeps) ProvidersService {
	providers := d.ConfiguredProviders

	// Fallback: read from env var if not configured
	if len(providers) == 0 {
		if envProviders := os.Getenv("SOCIAL_PROVIDERS"); envProviders != "" {
			parts := strings.Split(envProviders, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					providers = append(providers, p)
				}
			}
		}
	}

	// Default: at least google if nothing configured
	if len(providers) == 0 {
		providers = []string{"google"}
	}

	return &providersService{
		providers: providers,
	}
}

// List returns the list of available social providers.
// tenantID is optional - for now we return global providers.
// In the future, this could be extended to return tenant-specific providers.
func (s *providersService) List(ctx context.Context, tenantID string) ([]string, error) {
	// TODO: If tenant-specific provider config is needed,
	// lookup tenant settings here using tenantID.
	// For now, return global configured providers.
	return s.providers, nil
}
