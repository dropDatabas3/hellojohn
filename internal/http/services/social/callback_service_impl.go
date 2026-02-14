package social

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	dtoa "github.com/dropDatabas3/hellojohn/internal/http/dto/auth"
	dtos "github.com/dropDatabas3/hellojohn/internal/http/dto/social"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
)

// CacheWriter extends Cache with write capabilities for callback service.
type CacheWriter interface {
	Get(key string) ([]byte, bool)
	Delete(key string) error
	Set(key string, value []byte, ttl time.Duration)
}

// CallbackDeps contains dependencies for callback service.
type CallbackDeps struct {
	Providers    ProvidersService
	StateSigner  StateSigner
	Cache        CacheWriter // Use CacheWriter for write capabilities
	LoginCodeTTL time.Duration
	OIDCFactory  OIDCFactory         // Factory to create OIDC clients
	Provisioning ProvisioningService // User provisioning service
	TokenService TokenService        // Token issuance service
	ClientConfig ClientConfigService // Client configuration validation
}

// callbackService implements CallbackService.
type callbackService struct {
	providers    ProvidersService
	stateSigner  StateSigner
	cache        CacheWriter
	loginCodeTTL time.Duration
	oidcFactory  OIDCFactory
	provisioning ProvisioningService
	tokenService TokenService
	clientConfig ClientConfigService
}

// NewCallbackService creates a new CallbackService.
func NewCallbackService(d CallbackDeps) CallbackService {
	ttl := d.LoginCodeTTL
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &callbackService{
		providers:    d.Providers,
		stateSigner:  d.StateSigner,
		cache:        d.Cache,
		loginCodeTTL: ttl,
		oidcFactory:  d.OIDCFactory,
		provisioning: d.Provisioning,
		tokenService: d.TokenService,
		clientConfig: d.ClientConfig,
	}
}

// Callback processes the OAuth callback.
func (s *callbackService) Callback(ctx context.Context, req CallbackRequest) (*CallbackResult, error) {
	log := logger.From(ctx).With(logger.Layer("service"), logger.Component("social.callback"))

	// Validate required fields
	if req.State == "" {
		return nil, ErrCallbackMissingState
	}
	if req.Code == "" {
		return nil, ErrCallbackMissingCode
	}

	// Parse and validate state
	if s.stateSigner == nil {
		log.Error("stateSigner not configured")
		return nil, ErrCallbackInvalidState
	}

	stateClaims, err := s.stateSigner.ParseState(req.State)
	if err != nil {
		log.Warn("state validation failed", logger.Err(err))
		return nil, fmt.Errorf("%w: %v", ErrCallbackInvalidState, err)
	}

	// Validate provider matches path
	if !strings.EqualFold(stateClaims.Provider, req.Provider) {
		log.Warn("provider mismatch",
			logger.String("path_provider", req.Provider),
			logger.String("state_provider", stateClaims.Provider),
		)
		return nil, ErrCallbackProviderMismatch
	}

	// Validate required claims from state
	if stateClaims.TenantSlug == "" {
		log.Warn("state missing tenant_slug")
		return nil, ErrCallbackInvalidState
	}
	if stateClaims.ClientID == "" {
		log.Warn("state missing client_id")
		return nil, ErrCallbackInvalidState
	}
	if stateClaims.Nonce == "" {
		log.Warn("state missing nonce")
		return nil, ErrCallbackInvalidState
	}

	// Use ClientConfigService for strict validation if available
	if s.clientConfig != nil {
		// Validate client exists
		if _, err := s.clientConfig.GetClient(ctx, stateClaims.TenantSlug, stateClaims.ClientID); err != nil {
			if errors.Is(err, ErrClientNotFound) {
				log.Warn("client not found in control plane",
					logger.TenantID(stateClaims.TenantSlug),
					logger.String("client_id", stateClaims.ClientID),
				)
				return nil, ErrCallbackInvalidClient
			}
			log.Error("failed to get client", logger.Err(err))
			return nil, fmt.Errorf("%w: %v", ErrCallbackInvalidClient, err)
		}

		// Validate provider is allowed for this client
		if err := s.clientConfig.IsProviderAllowed(ctx, stateClaims.TenantSlug, stateClaims.ClientID, req.Provider); err != nil {
			if errors.Is(err, ErrProviderMisconfigured) {
				log.Error("provider misconfigured", logger.Err(err))
				return nil, ErrCallbackProviderMisconfigured
			}
			if errors.Is(err, ErrSocialLoginDisabled) {
				log.Warn("social login disabled for tenant", logger.TenantID(stateClaims.TenantSlug))
				return nil, ErrCallbackProviderDisabled
			}
			log.Warn("provider not allowed",
				logger.String("provider", req.Provider),
				logger.TenantID(stateClaims.TenantSlug),
				logger.String("client_id", stateClaims.ClientID),
				logger.Err(err),
			)
			return nil, ErrCallbackProviderDisabled
		}

		// Validate redirect_uri if present in state
		if stateClaims.RedirectURI != "" {
			if err := s.clientConfig.ValidateRedirectURI(ctx, stateClaims.TenantSlug, stateClaims.ClientID, stateClaims.RedirectURI); err != nil {
				if errors.Is(err, ErrRedirectInvalid) || errors.Is(err, ErrRedirectNotAllowed) {
					log.Warn("redirect_uri validation failed",
						logger.String("redirect_uri", stateClaims.RedirectURI),
						logger.Err(err),
					)
					return nil, ErrCallbackInvalidRedirect
				}
				log.Warn("redirect_uri validation error", logger.Err(err))
				return nil, ErrCallbackInvalidRedirect
			}
		}
	} else {
		// Fallback to legacy ProvidersService (backwards compatibility)
		providers, err := s.providers.List(ctx, stateClaims.TenantSlug)
		if err != nil {
			log.Error("failed to list providers", logger.Err(err))
			return nil, fmt.Errorf("%w: %v", ErrCallbackProviderDisabled, err)
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
				logger.TenantID(stateClaims.TenantSlug),
			)
			return nil, ErrCallbackProviderDisabled
		}
	}

	log.Info("callback validated",
		logger.String("provider", req.Provider),
		logger.TenantID(stateClaims.TenantSlug),
		logger.String("client_id", stateClaims.ClientID),
	)

	// Exchange code with provider using OIDC client
	var idClaims *OIDCClaims
	if s.oidcFactory != nil && strings.EqualFold(req.Provider, "google") {
		oidc, err := s.oidcFactory.Google(ctx, stateClaims.TenantSlug, req.BaseURL)
		if err != nil {
			log.Error("failed to create OIDC client",
				logger.String("provider", req.Provider),
				logger.TenantID(stateClaims.TenantSlug),
				logger.Err(err),
			)
			return nil, fmt.Errorf("%w: %v", ErrCallbackOIDCExchangeFailed, err)
		}

		// Exchange authorization code for tokens
		tokens, err := oidc.ExchangeCode(ctx, req.Code)
		if err != nil {
			log.Error("code exchange failed",
				logger.String("provider", req.Provider),
				logger.Err(err),
			)
			return nil, fmt.Errorf("%w: %v", ErrCallbackOIDCExchangeFailed, err)
		}

		// Verify ID token with nonce from state
		idClaims, err = oidc.VerifyIDToken(ctx, tokens.IDToken, stateClaims.Nonce)
		if err != nil {
			log.Error("ID token verification failed",
				logger.String("provider", req.Provider),
				logger.Err(err),
			)
			return nil, fmt.Errorf("%w: %v", ErrCallbackIDTokenInvalid, err)
		}

		// Validate email is present
		if idClaims.Email == "" {
			log.Error("email missing in ID token",
				logger.String("provider", req.Provider),
				logger.String("sub", idClaims.Sub),
			)
			return nil, ErrCallbackEmailMissing
		}

		log.Info("OIDC exchange successful",
			logger.String("provider", req.Provider),
			logger.String("email", idClaims.Email),
			logger.Bool("email_verified", idClaims.EmailVerified),
			logger.String("name", idClaims.Name),
		)
	}

	// GitHub OAuth (non-OIDC)
	if s.oidcFactory != nil && strings.EqualFold(req.Provider, "github") {
		oauth, err := s.oidcFactory.GitHub(ctx, stateClaims.TenantSlug, req.BaseURL)
		if err != nil {
			log.Error("failed to create GitHub OAuth client",
				logger.String("provider", req.Provider),
				logger.TenantID(stateClaims.TenantSlug),
				logger.Err(err),
			)
			return nil, fmt.Errorf("%w: %v", ErrCallbackOIDCExchangeFailed, err)
		}

		// Exchange authorization code for access token
		tokens, err := oauth.ExchangeCode(ctx, req.Code)
		if err != nil {
			log.Error("GitHub code exchange failed",
				logger.String("provider", req.Provider),
				logger.Err(err),
			)
			return nil, fmt.Errorf("%w: %v", ErrCallbackOIDCExchangeFailed, err)
		}

		// For GitHub, "VerifyIDToken" actually fetches user info via API
		// We pass the access token as the "idToken" parameter
		idClaims, err = oauth.VerifyIDToken(ctx, tokens.AccessToken, stateClaims.Nonce)
		if err != nil {
			log.Error("GitHub user info fetch failed",
				logger.String("provider", req.Provider),
				logger.Err(err),
			)
			return nil, fmt.Errorf("%w: %v", ErrCallbackIDTokenInvalid, err)
		}

		// Validate email is present
		if idClaims.Email == "" {
			log.Error("email missing from GitHub",
				logger.String("provider", req.Provider),
				logger.String("sub", idClaims.Sub),
			)
			return nil, ErrCallbackEmailMissing
		}

		log.Info("GitHub OAuth exchange successful",
			logger.String("provider", req.Provider),
			logger.String("email", idClaims.Email),
			logger.Bool("email_verified", idClaims.EmailVerified),
			logger.String("name", idClaims.Name),
		)
	}

	// Run user provisioning if we have claims and provisioning service
	var userID string
	if idClaims != nil && s.provisioning != nil {
		var err error
		userID, err = s.provisioning.EnsureUserAndIdentity(ctx, stateClaims.TenantSlug, req.Provider, idClaims)
		if err != nil {
			log.Error("user provisioning failed",
				logger.String("provider", req.Provider),
				logger.TenantID(stateClaims.TenantSlug),
				logger.Err(err),
			)
			return nil, fmt.Errorf("%w: %v", ErrCallbackProvisionFailed, err)
		}

		log.Info("user provisioned",
			logger.String("provider", req.Provider),
			logger.TenantID(stateClaims.TenantSlug),
			logger.String("user_id", userID),
		)
	}

	// Issue real tokens using TokenService
	var tokenResponse *dtoa.LoginResponse
	if s.tokenService != nil && userID != "" {
		var err error
		tokenResponse, err = s.tokenService.IssueSocialTokens(ctx, stateClaims.TenantSlug, stateClaims.ClientID, userID, []string{req.Provider})
		if err != nil {
			log.Error("token issuance failed",
				logger.String("provider", req.Provider),
				logger.TenantID(stateClaims.TenantSlug),
				logger.Err(err),
			)
			return nil, fmt.Errorf("%w: %v", ErrCallbackTokenIssueFailed, err)
		}
	} else {
		// Fallback: stub tokens if TokenService not configured or no userID
		codePrefix := req.Code
		if len(codePrefix) > 8 {
			codePrefix = codePrefix[:8]
		}
		tokenResponse = &dtoa.LoginResponse{
			AccessToken:  "stub_access_" + stateClaims.TenantSlug + "_" + codePrefix,
			TokenType:    "Bearer",
			ExpiresIn:    3600,
			RefreshToken: "stub_refresh_" + stateClaims.TenantSlug + "_" + codePrefix,
		}
	}

	// If redirect_uri was provided, use login_code flow
	if stateClaims.RedirectURI != "" {
		// Generate login code
		loginCode, err := generateNonce(32)
		if err != nil {
			log.Error("failed to generate login code", logger.Err(err))
			return nil, ErrCallbackTokenIssueFailed
		}

		// Store payload in cache
		payload := dtos.ExchangePayload{
			ClientID:   stateClaims.ClientID,
			TenantID:   stateClaims.TenantSlug, // Using slug as ID for backwards compat
			TenantSlug: stateClaims.TenantSlug,
			Provider:   req.Provider,
			Response:   *tokenResponse,
		}
		payloadBytes, _ := json.Marshal(payload)

		cacheKey := "social:code:" + loginCode
		if s.cache != nil {
			s.cache.Set(cacheKey, payloadBytes, s.loginCodeTTL)
		}

		log.Info("login code stored",
			logger.String("code_prefix", loginCode[:8]),
			logger.TenantID(stateClaims.TenantSlug),
		)

		// Build redirect URL with code and social=true marker
		redirectURL := stateClaims.RedirectURI
		if u, err := url.Parse(redirectURL); err == nil {
			q := u.Query()
			q.Set("code", loginCode)
			q.Set("social", "true") // Marker for SDK to identify social login callback
			u.RawQuery = q.Encode()
			redirectURL = u.String()
		} else {
			sep := "?"
			if strings.Contains(redirectURL, "?") {
				sep = "&"
			}
			redirectURL = redirectURL + sep + "code=" + loginCode + "&social=true"
		}

		return &CallbackResult{
			RedirectURL: redirectURL,
		}, nil
	}

	// Direct JSON response (no redirect)
	respBytes, _ := json.Marshal(tokenResponse)
	return &CallbackResult{
		JSONResponse: respBytes,
	}, nil
}
