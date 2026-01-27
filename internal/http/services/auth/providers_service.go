package auth

import (
	"context"
	"net/url"
	"strings"

	"github.com/google/uuid"

	dto "github.com/dropDatabas3/hellojohn/internal/http/dto/auth"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	store "github.com/dropDatabas3/hellojohn/internal/store"
	"go.uber.org/zap"
)

// ProvidersService defines operations for providers discovery.
type ProvidersService interface {
	GetProviders(ctx context.Context, in dto.ProvidersRequest) (*dto.ProvidersResult, error)
}

// ProviderConfig holds global provider configuration (from config.Config).
type ProviderConfig struct {
	// Google OAuth
	GoogleEnabled      bool
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// JWT issuer for default redirect derivation
	JWTIssuer string
}

// ProvidersDeps contains dependencies for the providers service.
type ProvidersDeps struct {
	DAL       store.DataAccessLayer
	Providers ProviderConfig
}

type providersService struct {
	deps ProvidersDeps
}

// NewProvidersService creates a new ProvidersService.
func NewProvidersService(deps ProvidersDeps) ProvidersService {
	return &providersService{deps: deps}
}

// GetProviders returns available auth providers for the UI.
func (s *providersService) GetProviders(ctx context.Context, in dto.ProvidersRequest) (*dto.ProvidersResult, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("auth.providers"),
		logger.Op("GetProviders"),
	)

	result := &dto.ProvidersResult{
		Providers: make([]dto.ProviderInfo, 0, 3),
	}

	// Password: always enabled/ready (informative only; real gating in auth handlers)
	result.Providers = append(result.Providers, dto.ProviderInfo{
		Name:    "password",
		Enabled: true,
		Ready:   true,
		Popup:   false,
	})

	// Google
	googleInfo := s.buildGoogleProvider(ctx, in, log)
	result.Providers = append(result.Providers, googleInfo)

	log.Debug("providers resolved", zap.Int("count", len(result.Providers)))
	return result, nil
}

// buildGoogleProvider builds the Google provider info.
func (s *providersService) buildGoogleProvider(ctx context.Context, in dto.ProvidersRequest, log *zap.Logger) dto.ProviderInfo {
	pi := dto.ProviderInfo{
		Name:  "google",
		Popup: true,
	}

	if !s.deps.Providers.GoogleEnabled {
		pi.Enabled = false
		pi.Ready = false
		return pi
	}

	pi.Enabled = true

	// Check if ready (config correct)
	googleReady := strings.TrimSpace(s.deps.Providers.GoogleClientID) != "" &&
		strings.TrimSpace(s.deps.Providers.GoogleClientSecret) != ""

	// Need either explicit redirect URL or JWT issuer to derive callback
	if strings.TrimSpace(s.deps.Providers.GoogleRedirectURL) == "" &&
		strings.TrimSpace(s.deps.Providers.JWTIssuer) == "" {
		googleReady = false
	}

	pi.Ready = googleReady

	if !googleReady {
		pi.Reason = "google provider not configured (client_id/secret or redirect_url/jwt.issuer missing)"
		return pi
	}

	// Try to generate start_url if we have enough context
	startURL := s.buildGoogleStartURL(ctx, in, log)
	if startURL != "" {
		pi.StartURL = &startURL
	}

	return pi
}

// buildGoogleStartURL builds the start_url for Google OAuth if valid.
func (s *providersService) buildGoogleStartURL(ctx context.Context, in dto.ProvidersRequest, log *zap.Logger) string {
	tenantID := strings.TrimSpace(in.TenantID)
	clientID := strings.TrimSpace(in.ClientID)
	redirectURI := strings.TrimSpace(in.RedirectURI)

	// Parse tenant UUID
	if tenantID == "" {
		return ""
	}
	tid, err := uuid.Parse(tenantID)
	if err != nil || tid == uuid.Nil {
		return ""
	}

	if clientID == "" {
		return ""
	}

	// Default redirect if not provided
	if redirectURI == "" {
		base := strings.TrimRight(s.deps.Providers.JWTIssuer, "/")
		if base != "" {
			redirectURI = base + "/v1/auth/social/result"
		}
	}

	if redirectURI == "" {
		return ""
	}

	// Validate redirect_uri against client
	if !s.validateRedirectURI(ctx, tid.String(), clientID, redirectURI, log) {
		return ""
	}

	// Build start URL
	v := url.Values{}
	v.Set("tenant_id", tid.String())
	v.Set("client_id", clientID)
	v.Set("redirect_uri", redirectURI)

	return "/v1/auth/social/google/start?" + v.Encode()
}

// validateRedirectURI checks if the redirect_uri is allowed for the client.
func (s *providersService) validateRedirectURI(ctx context.Context, tenantID, clientID, redirectURI string, log *zap.Logger) bool {
	tda, err := s.deps.DAL.ForTenant(ctx, tenantID)
	if err != nil {
		log.Debug("tenant not found for redirect validation", zap.String("tenant_id", tenantID))
		return false
	}

	client, err := tda.Clients().Get(ctx, tenantID, clientID)
	if err != nil || client == nil {
		log.Debug("client not found for redirect validation", zap.String("client_id", clientID))
		return false
	}

	// Check if redirectURI is in allowed list
	for _, allowed := range client.RedirectURIs {
		if allowed == redirectURI {
			return true
		}
		// Also check prefix match for wildcard support
		if strings.HasSuffix(allowed, "*") {
			prefix := strings.TrimSuffix(allowed, "*")
			if strings.HasPrefix(redirectURI, prefix) {
				return true
			}
		}
	}

	log.Debug("redirect_uri not allowed",
		zap.String("client_id", clientID),
		zap.String("redirect_uri", redirectURI),
	)
	return false
}
