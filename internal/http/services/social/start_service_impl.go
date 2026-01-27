package social

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
)

// StartDeps contains dependencies for start service.
type StartDeps struct {
	Providers      ProvidersService
	AuthURLBuilder AuthURLBuilder      // Deprecated: use OIDCFactory instead
	StateSigner    StateSigner         // Interface to sign state JWTs
	OIDCFactory    OIDCFactory         // Factory to create OIDC clients
	ClientConfig   ClientConfigService // Client configuration validation
}

// AuthURLBuilder builds authorization URLs for OAuth providers.
type AuthURLBuilder interface {
	// BuildAuthURL returns the authorization URL for the given provider.
	BuildAuthURL(ctx context.Context, provider, state, nonce, callbackURL string) (string, error)
}

// startService implements StartService.
type startService struct {
	providers      ProvidersService
	authURLBuilder AuthURLBuilder
	stateSigner    StateSigner
	oidcFactory    OIDCFactory
	clientConfig   ClientConfigService
}

// NewStartService creates a new StartService.
func NewStartService(d StartDeps) StartService {
	return &startService{
		providers:      d.Providers,
		authURLBuilder: d.AuthURLBuilder,
		stateSigner:    d.StateSigner,
		oidcFactory:    d.OIDCFactory,
		clientConfig:   d.ClientConfig,
	}
}

// Start initiates social login flow and returns the redirect URL.
func (s *startService) Start(ctx context.Context, req StartRequest) (*StartResult, error) {
	log := logger.From(ctx).With(logger.Layer("service"), logger.Component("social.start"))

	// Validate required fields
	if req.TenantSlug == "" {
		return nil, ErrStartMissingTenant
	}
	if req.ClientID == "" {
		return nil, ErrStartMissingClientID
	}
	if req.Provider == "" {
		return nil, ErrStartProviderUnknown
	}

	// Validate client exists and provider is allowed using ClientConfigService
	if s.clientConfig != nil {
		// Validate client exists
		_, err := s.clientConfig.GetClient(ctx, req.TenantSlug, req.ClientID)
		if err != nil {
			if errors.Is(err, ErrClientNotFound) {
				return nil, ErrStartInvalidClient
			}
			log.Error("failed to get client", logger.Err(err))
			return nil, fmt.Errorf("%w: %v", ErrStartInvalidClient, err)
		}

		// Validate provider is allowed for this client
		if err := s.clientConfig.IsProviderAllowed(ctx, req.TenantSlug, req.ClientID, req.Provider); err != nil {
			if errors.Is(err, ErrProviderMisconfigured) {
				log.Error("provider misconfigured", logger.Err(err))
				return nil, fmt.Errorf("%w: %v", ErrStartProviderMisconfigured, err)
			}
			log.Warn("provider not allowed",
				logger.String("provider", req.Provider),
				logger.TenantID(req.TenantSlug),
				logger.String("client_id", req.ClientID),
				logger.Err(err),
			)
			return nil, ErrStartProviderDisabled
		}

		// Validate redirect_uri if provided
		if req.RedirectURI != "" {
			if err := s.clientConfig.ValidateRedirectURI(ctx, req.TenantSlug, req.ClientID, req.RedirectURI); err != nil {
				if errors.Is(err, ErrRedirectInvalid) {
					return nil, ErrStartInvalidRedirect
				}
				if errors.Is(err, ErrRedirectNotAllowed) {
					return nil, ErrStartRedirectNotAllowed
				}
				log.Warn("redirect_uri validation failed", logger.Err(err))
				return nil, ErrStartInvalidRedirect
			}
		}
	} else {
		// Fallback to legacy ProvidersService check (for backwards compatibility)
		providers, err := s.providers.List(ctx, req.TenantSlug)
		if err != nil {
			log.Error("failed to list providers", logger.Err(err))
			return nil, fmt.Errorf("%w: %v", ErrStartProviderDisabled, err)
		}

		providerEnabled := false
		for _, p := range providers {
			if strings.EqualFold(p, req.Provider) {
				providerEnabled = true
				break
			}
		}
		if !providerEnabled {
			log.Warn("provider not enabled",
				logger.String("provider", req.Provider),
				logger.TenantID(req.TenantSlug),
			)
			return nil, ErrStartProviderDisabled
		}
	}

	// Generate nonce for OIDC
	nonce, err := generateNonce(16)
	if err != nil {
		log.Error("failed to generate nonce", logger.Err(err))
		return nil, ErrStartAuthURLFailed
	}

	// Generate signed state JWT if StateSigner is available
	var state string
	if s.stateSigner != nil {
		state, err = s.stateSigner.SignState(StateClaims{
			Provider:    req.Provider,
			TenantSlug:  req.TenantSlug,
			ClientID:    req.ClientID,
			RedirectURI: req.RedirectURI,
			Nonce:       nonce,
		})
		if err != nil {
			log.Error("failed to sign state", logger.Err(err))
			return nil, ErrStartAuthURLFailed
		}
	} else {
		// Fallback to random state (less secure, for dev)
		state, err = generateNonce(32)
		if err != nil {
			log.Error("failed to generate state", logger.Err(err))
			return nil, ErrStartAuthURLFailed
		}
	}

	// Use OIDCFactory for real OIDC client
	if s.oidcFactory != nil && strings.EqualFold(req.Provider, "google") {
		oidc, err := s.oidcFactory.Google(ctx, req.TenantSlug, req.BaseURL)
		if err != nil {
			log.Error("failed to create OIDC client",
				logger.String("provider", req.Provider),
				logger.TenantID(req.TenantSlug),
				logger.Err(err),
			)
			return nil, fmt.Errorf("%w: %v", ErrStartAuthURLFailed, err)
		}

		authURL, err := oidc.AuthURL(ctx, state, nonce)
		if err != nil {
			log.Error("failed to build auth URL",
				logger.String("provider", req.Provider),
				logger.Err(err),
			)
			return nil, fmt.Errorf("%w: %v", ErrStartAuthURLFailed, err)
		}

		log.Info("social login started",
			logger.String("provider", req.Provider),
			logger.TenantID(req.TenantSlug),
			logger.String("client_id", req.ClientID),
		)

		return &StartResult{
			RedirectURL: authURL,
		}, nil
	}

	// Fallback to AuthURLBuilder (legacy)
	callbackURL := fmt.Sprintf("%s/v2/auth/social/%s/callback", strings.TrimRight(req.BaseURL, "/"), req.Provider)

	if s.authURLBuilder == nil {
		// Final fallback: stub URL (dev only)
		log.Warn("no OIDC factory or authURLBuilder configured, returning stub URL")
		return &StartResult{
			RedirectURL: fmt.Sprintf("https://accounts.google.com/o/oauth2/v2/auth?state=%s&nonce=%s&redirect_uri=%s",
				state, nonce, callbackURL),
		}, nil
	}

	authURL, err := s.authURLBuilder.BuildAuthURL(ctx, req.Provider, state, nonce, callbackURL)
	if err != nil {
		log.Error("failed to build auth URL",
			logger.String("provider", req.Provider),
			logger.Err(err),
		)
		return nil, fmt.Errorf("%w: %v", ErrStartAuthURLFailed, err)
	}

	log.Info("social login started",
		logger.String("provider", req.Provider),
		logger.TenantID(req.TenantSlug),
		logger.String("client_id", req.ClientID),
	)

	return &StartResult{
		RedirectURL: authURL,
	}, nil
}

// generateNonce generates a random base64url-encoded string.
func generateNonce(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
