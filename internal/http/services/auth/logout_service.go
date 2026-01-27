package auth

import (
	"context"
	"fmt"
	"strings"

	dto "github.com/dropDatabas3/hellojohn/internal/http/dto/auth"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	store "github.com/dropDatabas3/hellojohn/internal/store"
)

// LogoutService defines operations for logout.
type LogoutService interface {
	// Logout revokes a single refresh token.
	Logout(ctx context.Context, in dto.LogoutRequest, tenantSlug string) error
	// LogoutAll revokes all refresh tokens for a user.
	LogoutAll(ctx context.Context, in dto.LogoutAllRequest, tenantSlug string) error
}

// LogoutDeps contains dependencies for the logout service.
type LogoutDeps struct {
	DAL store.DataAccessLayer
}

type logoutService struct {
	deps LogoutDeps
}

// NewLogoutService creates a new logout service.
func NewLogoutService(deps LogoutDeps) LogoutService {
	return &logoutService{deps: deps}
}

// Logout errors
var (
	ErrLogoutMissingFields = fmt.Errorf("missing required fields")
	ErrLogoutInvalidClient = fmt.Errorf("invalid client")
	ErrLogoutNoDatabase    = fmt.Errorf("no database for tenant")
	ErrLogoutNotSupported  = fmt.Errorf("mass revocation not supported")
	ErrLogoutFailed        = fmt.Errorf("revocation failed")
)

// Logout revokes a single refresh token (idempotent).
func (s *logoutService) Logout(ctx context.Context, in dto.LogoutRequest, tenantSlug string) error {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("auth.logout"),
		logger.Op("Logout"),
	)

	// Normalize
	in.RefreshToken = strings.TrimSpace(in.RefreshToken)
	in.ClientID = strings.TrimSpace(in.ClientID)
	in.TenantID = strings.TrimSpace(in.TenantID)

	if in.RefreshToken == "" || in.ClientID == "" {
		return ErrLogoutMissingFields
	}

	// Use provided tenant or fallback
	if tenantSlug == "" {
		tenantSlug = in.TenantID
	}
	if tenantSlug == "" {
		return ErrLogoutMissingFields
	}

	// Hash the refresh token (hex encoding)
	// Hash the refresh token (base64url encoding)
	hash := tokens.SHA256Base64URL(in.RefreshToken)

	// Get TDA for tenant
	tda, err := s.deps.DAL.ForTenant(ctx, tenantSlug)
	if err != nil {
		log.Debug("tenant resolution failed", logger.Err(err))
		return ErrLogoutInvalidClient
	}

	if err := tda.RequireDB(); err != nil {
		log.Debug("tenant DB not available", logger.Err(err))
		return ErrLogoutNoDatabase
	}

	log = log.With(logger.TenantSlug(tda.Slug()))

	// Find refresh token by hash
	rt, err := tda.Tokens().GetByHash(ctx, hash)
	if err != nil || rt == nil {
		// Idempotent: if not found, consider it already revoked
		log.Debug("refresh token not found, treating as already revoked")
		return nil
	}

	// Validate client_id matches
	if !strings.EqualFold(in.ClientID, rt.ClientID) {
		log.Debug("client_id mismatch")
		return ErrLogoutInvalidClient
	}

	// Re-open TDA if token belongs to different tenant
	if rt.TenantID != "" && rt.TenantID != tda.ID() {
		tda2, err := s.deps.DAL.ForTenant(ctx, rt.TenantID)
		if err != nil {
			return ErrLogoutNoDatabase
		}
		if err := tda2.RequireDB(); err != nil {
			return ErrLogoutNoDatabase
		}
		tda = tda2
	}

	// Revoke the token
	if err := tda.Tokens().Revoke(ctx, rt.ID); err != nil {
		log.Warn("failed to revoke refresh token", logger.Err(err))
		// Still return nil for idempotency
	}

	log.Info("logout successful")
	return nil
}

// LogoutAll revokes all refresh tokens for a user.
func (s *logoutService) LogoutAll(ctx context.Context, in dto.LogoutAllRequest, tenantSlug string) error {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("auth.logout"),
		logger.Op("LogoutAll"),
	)

	// Normalize
	in.UserID = strings.TrimSpace(in.UserID)
	in.ClientID = strings.TrimSpace(in.ClientID)

	if in.UserID == "" {
		return ErrLogoutMissingFields
	}

	if tenantSlug == "" {
		return ErrLogoutMissingFields
	}

	// Get TDA for tenant
	tda, err := s.deps.DAL.ForTenant(ctx, tenantSlug)
	if err != nil {
		log.Debug("tenant resolution failed", logger.Err(err))
		return ErrLogoutInvalidClient
	}

	if err := tda.RequireDB(); err != nil {
		log.Debug("tenant DB not available", logger.Err(err))
		return ErrLogoutNoDatabase
	}

	log = log.With(logger.TenantSlug(tda.Slug()), logger.UserID(in.UserID))

	// Revoke all tokens for user (optionally filtered by client)
	count, err := tda.Tokens().RevokeAllByUser(ctx, in.UserID, in.ClientID)
	if err != nil {
		log.Error("mass revocation failed", logger.Err(err))
		return ErrLogoutFailed
	}

	log.Info("logout-all successful", logger.Int("revoked_count", int(count)))
	return nil
}
