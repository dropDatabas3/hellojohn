// Package social contiene los services del dominio social login.
package social

import (
	"time"

	"github.com/dropDatabas3/hellojohn/internal/jwt"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// Deps contiene las dependencias para crear los services social.
type Deps struct {
	DAL                 store.DataAccessLayer // V2 data access layer
	Cache               CacheWriter           // Cache with write capabilities (Get/Delete/Set)
	DebugPeek           bool                  // Enable peek mode for result viewer (should be false in production)
	ConfiguredProviders []string              // List of enabled provider names (optional, reads from env if empty)
	AuthURLBuilder      AuthURLBuilder        // Deprecated: use OIDCFactory instead
	StateSigner         StateSigner           // Optional: signer for state JWTs
	LoginCodeTTL        time.Duration         // TTL for login codes (default 60s)
	OIDCFactory         OIDCFactory           // Factory for OIDC clients (Google, etc.)
	Issuer              *jwt.Issuer           // JWT issuer for token signing
	BaseURL             string                // Base URL for issuer resolution
	RefreshTTL          time.Duration         // TTL for refresh tokens
	TenantProvider      TenantProvider        // Control plane tenant provider
}

// Services agrupa todos los services del dominio social.
type Services struct {
	Exchange     ExchangeService
	Result       ResultService
	Providers    ProvidersService
	Start        StartService
	Callback     CallbackService
	Provisioning ProvisioningService
	Token        TokenService
	ClientConfig ClientConfigService
}

// NewServices crea el agregador de services social.
func NewServices(d Deps) Services {
	providers := NewProvidersService(ProvidersDeps{
		ConfiguredProviders: d.ConfiguredProviders,
	})

	provisioning := NewProvisioningService(ProvisioningDeps{
		DAL: d.DAL,
	})

	tokenSvc := NewTokenService(TokenDeps{
		DAL:        d.DAL,
		Issuer:     d.Issuer,
		BaseURL:    d.BaseURL,
		RefreshTTL: d.RefreshTTL,
	})

	// ClientConfigService for validating clients/redirects/providers
	var clientConfig ClientConfigService
	if d.TenantProvider != nil {
		clientConfig = NewClientConfigService(ClientConfigDeps{
			TenantProvider: d.TenantProvider,
		})
	}

	return Services{
		Exchange: NewExchangeService(ExchangeDeps{
			Cache:        d.Cache, // CacheWriter implements Cache
			ClientConfig: clientConfig,
		}),

		Result: NewResultService(ResultDeps{
			Cache:     d.Cache, // CacheWriter implements Cache
			DebugPeek: d.DebugPeek,
		}),
		Providers:    providers,
		Provisioning: provisioning,
		Token:        tokenSvc,
		ClientConfig: clientConfig,
		Start: NewStartService(StartDeps{
			Providers:      providers,
			AuthURLBuilder: d.AuthURLBuilder,
			StateSigner:    d.StateSigner,
			OIDCFactory:    d.OIDCFactory,
			ClientConfig:   clientConfig,
		}),
		Callback: NewCallbackService(CallbackDeps{
			Providers:    providers,
			StateSigner:  d.StateSigner,
			Cache:        d.Cache,
			LoginCodeTTL: d.LoginCodeTTL,
			OIDCFactory:  d.OIDCFactory,
			Provisioning: provisioning,
			TokenService: tokenSvc,
			ClientConfig: clientConfig,
		}),
	}
}
