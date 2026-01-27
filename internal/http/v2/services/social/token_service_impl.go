package social

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	dtoa "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/auth"
	"github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// TokenDeps contains dependencies for token service.
type TokenDeps struct {
	DAL        store.DataAccessLayer // V2 data access layer
	Issuer     *jwt.Issuer           // JWT issuer for signing tokens
	BaseURL    string                // Base URL for issuer resolution
	RefreshTTL time.Duration         // TTL for refresh tokens (default 30 days)
}

// tokenService implements TokenService.
type tokenService struct {
	dal        store.DataAccessLayer
	issuer     *jwt.Issuer
	baseURL    string
	refreshTTL time.Duration
}

// NewTokenService creates a new TokenService.
func NewTokenService(d TokenDeps) TokenService {
	ttl := d.RefreshTTL
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour // 30 days default
	}
	return &tokenService{
		dal:        d.DAL,
		issuer:     d.Issuer,
		baseURL:    d.BaseURL,
		refreshTTL: ttl,
	}
}

// IssueSocialTokens issues access and refresh tokens after social login.
func (s *tokenService) IssueSocialTokens(ctx context.Context, tenantSlug, clientID, userID string, amr []string) (*dtoa.LoginResponse, error) {
	log := logger.From(ctx).With(logger.Layer("service"), logger.Component("social.token"))

	if s.issuer == nil {
		log.Error("issuer not configured")
		return nil, ErrTokenIssuerNotConfigured
	}

	// Get tenant data access via DAL
	if s.dal == nil {
		log.Error("DAL not configured")
		return nil, ErrTokenIssuerNotConfigured
	}

	tda, err := s.dal.ForTenant(ctx, tenantSlug)
	if err != nil {
		log.Error("tenant not found", logger.Err(err), logger.TenantID(tenantSlug))
		return nil, fmt.Errorf("%w: tenant not found", ErrTokenIssueFailed)
	}

	settings := tda.Settings()
	if settings == nil {
		log.Error("tenant settings not available", logger.TenantID(tenantSlug))
		return nil, fmt.Errorf("%w: tenant settings not found", ErrTokenIssueFailed)
	}

	// Resolve effective issuer for tenant
	issMode := settings.IssuerMode
	issOverride := settings.IssuerOverride
	effectiveIss := jwt.ResolveIssuer(s.baseURL, issMode, tenantSlug, issOverride)

	// Build standard claims
	stdClaims := map[string]any{
		"tid": tenantSlug,
		"cid": clientID,
	}
	if len(amr) > 0 {
		stdClaims["amr"] = amr
	}

	// Issue access token for tenant
	accessToken, exp, err := s.issuer.IssueAccessForTenant(tenantSlug, effectiveIss, userID, clientID, stdClaims, nil)
	if err != nil {
		log.Error("failed to issue access token", logger.Err(err))
		return nil, fmt.Errorf("%w: access token: %v", ErrTokenIssueFailed, err)
	}

	// Generate refresh token (opaque)
	refreshToken, err := generateOpaqueToken(32)
	if err != nil {
		log.Error("failed to generate refresh token", logger.Err(err))
		return nil, fmt.Errorf("%w: refresh token generation", ErrTokenIssueFailed)
	}

	// Store refresh token in tenant DB
	if err := s.storeRefreshToken(ctx, settings, userID, clientID, refreshToken); err != nil {
		log.Error("failed to store refresh token", logger.Err(err), logger.TenantID(tenantSlug))
		// Don't fail the flow, just log and continue without refresh
		// In production you might want to fail here
	}

	// Calculate expires_in from exp
	expiresIn := int64(time.Until(exp).Seconds())
	if expiresIn < 0 {
		expiresIn = 900 // 15 minutes default
	}

	log.Info("social tokens issued",
		logger.TenantID(tenantSlug),
		logger.String("client_id", clientID),
		logger.String("user_id", userID),
		logger.Int("expires_in", int(expiresIn)),
	)

	return &dtoa.LoginResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		RefreshToken: refreshToken,
	}, nil
}

// storeRefreshToken stores refresh token hash in tenant DB.
func (s *tokenService) storeRefreshToken(ctx context.Context, settings *repository.TenantSettings, userID, clientID, refreshToken string) error {
	if settings == nil || settings.UserDB == nil || settings.UserDB.DSN == "" {
		return nil // No DB configured, skip storage
	}

	pool, err := DefaultPoolManager.GetPool(ctx, settings.UserDB.DSN)
	if err != nil {
		return fmt.Errorf("db connection: %w", err)
	}
	// DO NOT close pool here, it is managed

	// Hash the refresh token for storage
	hash := sha256Base64(refreshToken)

	// Calculate expiration
	expiresAt := time.Now().UTC().Add(s.refreshTTL)

	// Insert refresh token
	_, err = pool.Exec(ctx, `
		INSERT INTO refresh_token (token_hash, user_id, client_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, hash, userID, clientID, expiresAt)

	if err != nil {
		return fmt.Errorf("insert refresh_token: %w", err)
	}

	return nil
}

// generateOpaqueToken generates a cryptographically secure opaque token.
func generateOpaqueToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// sha256Base64 returns base64url encoded SHA256 hash.
func sha256Base64(s string) string {
	h := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
