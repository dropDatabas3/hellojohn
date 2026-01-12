package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	"github.com/dropDatabas3/hellojohn/internal/domain/types"
	dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/auth"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/helpers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// RefreshService defines operations for token refresh.
type RefreshService interface {
	Refresh(ctx context.Context, in dto.RefreshRequest, tenantSlug string) (*dto.RefreshResult, error)
}

// RefreshDeps contains dependencies for the refresh service.
type RefreshDeps struct {
	DAL        store.DataAccessLayer
	Issuer     *jwtx.Issuer
	RefreshTTL time.Duration
	ClaimsHook ClaimsHook
}

type refreshService struct {
	deps RefreshDeps
}

// NewRefreshService creates a new refresh service.
func NewRefreshService(deps RefreshDeps) RefreshService {
	if deps.ClaimsHook == nil {
		deps.ClaimsHook = NoOpClaimsHook{}
	}
	return &refreshService{deps: deps}
}

// Refresh errors
var (
	ErrMissingRefreshFields = fmt.Errorf("missing required fields")
	ErrInvalidRefreshToken  = fmt.Errorf("invalid or expired refresh token")
	ErrRefreshTokenRevoked  = fmt.Errorf("refresh token revoked")
	ErrClientMismatch       = fmt.Errorf("client_id mismatch")
	ErrRefreshUserDisabled  = fmt.Errorf("user disabled")
	ErrRefreshIssueFailed   = fmt.Errorf("failed to issue tokens")
)

func (s *refreshService) Refresh(ctx context.Context, in dto.RefreshRequest, tenantSlug string) (*dto.RefreshResult, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("auth.refresh"),
		logger.Op("Refresh"),
	)

	// Normalize inputs
	in.RefreshToken = strings.TrimSpace(in.RefreshToken)
	in.ClientID = strings.TrimSpace(in.ClientID)
	in.TenantID = strings.TrimSpace(in.TenantID)

	if in.RefreshToken == "" || in.ClientID == "" {
		return nil, ErrMissingRefreshFields
	}

	// Use provided tenant or fallback from context
	if tenantSlug == "" {
		tenantSlug = in.TenantID
	}
	if tenantSlug == "" {
		return nil, ErrMissingRefreshFields
	}

	// Check if it's a JWT refresh token (stateless admin flow)
	if strings.Count(in.RefreshToken, ".") == 2 {
		result, err := s.refreshAdminJWT(ctx, in.RefreshToken, log)
		if err == nil {
			return result, nil
		}
		// If JWT validation failed, fall through to DB-based refresh
		log.Debug("JWT refresh validation failed, trying DB", logger.Err(err))
	}

	// DB-based refresh flow
	return s.refreshFromDB(ctx, in, tenantSlug, log)
}

// refreshAdminJWT handles stateless JWT refresh for FS admins.
func (s *refreshService) refreshAdminJWT(ctx context.Context, tokenStr string, log *zap.Logger) (*dto.RefreshResult, error) {
	// Parse and verify JWT
	token, err := jwtv5.Parse(tokenStr, func(token *jwtv5.Token) (interface{}, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, jwtv5.ErrTokenUnverifiable
		}
		return s.deps.Issuer.Keys.PublicKeyByKID(kid)
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid JWT: %w", err)
	}

	claims, ok := token.Claims.(jwtv5.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}

	// Must be a refresh token
	use, _ := claims["token_use"].(string)
	if use != "refresh" {
		return nil, fmt.Errorf("not a refresh token")
	}

	userID, _ := claims.GetSubject()
	if userID == "" {
		return nil, fmt.Errorf("missing sub claim")
	}

	// Get signing key
	kid, priv, _, err := s.deps.Issuer.Keys.Active()
	if err != nil {
		return nil, fmt.Errorf("no signing key: %w", err)
	}

	now := time.Now().UTC()
	exp := now.Add(s.deps.Issuer.AccessTTL)

	// Build admin access token claims
	amr := []string{"pwd", "refresh"}
	grantedScopes := []string{"openid", "profile", "email"}
	std := map[string]any{
		"tid": "global",
		"amr": amr,
		"acr": "urn:hellojohn:loa:1",
		"scp": strings.Join(grantedScopes, " "),
	}

	// Minimal system claims for admin
	custom := helpers.PutSystemClaimsV2(map[string]any{}, s.deps.Issuer.Iss, map[string]any{"is_admin": true}, []string{"sys:admin"}, nil)

	atClaims := jwtv5.MapClaims{
		"iss": s.deps.Issuer.Iss,
		"sub": userID,
		"aud": "admin",
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": exp.Unix(),
	}
	for k, v := range std {
		atClaims[k] = v
	}
	if custom != nil {
		atClaims["custom"] = custom
	}

	atToken := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, atClaims)
	atToken.Header["kid"] = kid
	atToken.Header["typ"] = "JWT"

	accessToken, err := atToken.SignedString(priv)
	if err != nil {
		return nil, fmt.Errorf("sign access failed: %w", err)
	}

	// Issue new refresh JWT (rotation)
	rtClaims := jwtv5.MapClaims{
		"iss":       s.deps.Issuer.Iss,
		"sub":       userID,
		"aud":       "admin",
		"iat":       now.Unix(),
		"nbf":       now.Unix(),
		"exp":       now.Add(s.deps.RefreshTTL).Unix(),
		"token_use": "refresh",
	}

	rtToken := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, rtClaims)
	rtToken.Header["kid"] = kid
	rtToken.Header["typ"] = "JWT"

	refreshToken, err := rtToken.SignedString(priv)
	if err != nil {
		return nil, fmt.Errorf("sign refresh failed: %w", err)
	}

	log.Info("admin JWT refresh successful")

	return &dto.RefreshResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(time.Until(exp).Seconds()),
	}, nil
}

// refreshFromDB handles stateful DB-based refresh for regular users.
func (s *refreshService) refreshFromDB(ctx context.Context, in dto.RefreshRequest, tenantSlug string, log *zap.Logger) (*dto.RefreshResult, error) {
	// Hash the refresh token (hex encoding, aligned with store)
	sum := sha256.Sum256([]byte(in.RefreshToken))
	hashHex := hex.EncodeToString(sum[:])

	// Get TDA for tenant
	tda, err := s.deps.DAL.ForTenant(ctx, tenantSlug)
	if err != nil {
		log.Debug("tenant resolution failed", logger.Err(err))
		return nil, ErrInvalidClient
	}

	if err := tda.RequireDB(); err != nil {
		log.Debug("tenant DB not available", logger.Err(err))
		return nil, ErrNoDatabase
	}

	tenantID := tda.ID()
	log = log.With(logger.TenantSlug(tda.Slug()))

	// Find refresh token by hash
	rt, err := tda.Tokens().GetByHash(ctx, hashHex)
	if err != nil || rt == nil {
		log.Debug("refresh token not found")
		return nil, ErrInvalidRefreshToken
	}

	// Check if revoked or expired
	now := time.Now()
	if rt.RevokedAt != nil || !now.Before(rt.ExpiresAt) {
		log.Debug("refresh token revoked or expired")
		return nil, ErrRefreshTokenRevoked
	}

	// Validate client_id matches
	if !strings.EqualFold(in.ClientID, rt.ClientID) {
		log.Debug("client_id mismatch")
		return nil, ErrClientMismatch
	}

	// Re-open TDA if token belongs to different tenant
	if rt.TenantID != "" && rt.TenantID != tenantID {
		tda2, err := s.deps.DAL.ForTenant(ctx, rt.TenantID)
		if err != nil {
			return nil, ErrNoDatabase
		}
		if err := tda2.RequireDB(); err != nil {
			return nil, ErrNoDatabase
		}
		tda = tda2
		tenantID = tda.ID()
	}

	// Check user is not disabled
	user, err := tda.Users().GetByID(ctx, rt.UserID)
	if err != nil {
		log.Debug("user not found", logger.Err(err))
		return nil, ErrInvalidRefreshToken
	}

	if helpers.IsUserDisabled(user) {
		log.Info("user disabled")
		return nil, ErrRefreshUserDisabled
	}

	log = log.With(logger.UserID(user.ID))

	// Get scopes from client config
	var grantedScopes []string
	if client, err := tda.Clients().Get(ctx, tenantID, rt.ClientID); err == nil && client != nil {
		grantedScopes = client.Scopes
	} else {
		grantedScopes = []string{"openid"}
	}

	// Build claims
	amr := []string{"refresh"}
	std := map[string]any{
		"tid": tenantID,
		"amr": amr,
		"scp": strings.Join(grantedScopes, " "),
	}
	custom := map[string]any{}

	// Apply claims hook
	std, custom = s.deps.ClaimsHook.ApplyAccess(ctx, tenantID, in.ClientID, user.ID, grantedScopes, amr, std, custom)

	// Resolve effective issuer
	effIss := jwtx.ResolveIssuer(
		s.deps.Issuer.Iss,
		string(tda.Settings().IssuerMode),
		tda.Slug(),
		tda.Settings().IssuerOverride,
	)

	// Add RBAC claims if supported
	custom = helpers.PutSystemClaimsV2(custom, effIss, user.Metadata, nil, nil)

	// Select signing key
	kid, priv, _, err := s.selectSigningKey(tda)
	if err != nil {
		log.Error("failed to get signing key", logger.Err(err))
		return nil, ErrRefreshIssueFailed
	}

	nowUTC := time.Now().UTC()
	exp := nowUTC.Add(s.deps.Issuer.AccessTTL)

	claims := jwtv5.MapClaims{
		"iss": effIss,
		"sub": user.ID,
		"aud": in.ClientID,
		"iat": nowUTC.Unix(),
		"nbf": nowUTC.Unix(),
		"exp": exp.Unix(),
	}
	for k, v := range std {
		claims[k] = v
	}
	if len(custom) > 0 {
		claims["custom"] = custom
	}

	tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
	tk.Header["kid"] = kid
	tk.Header["typ"] = "JWT"

	accessToken, err := tk.SignedString(priv)
	if err != nil {
		log.Error("failed to sign access token", logger.Err(err))
		return nil, ErrRefreshIssueFailed
	}

	// Create new refresh token (rotation)
	rawRefresh, err := tokens.GenerateOpaqueToken(32)
	if err != nil {
		log.Error("failed to generate refresh token", logger.Err(err))
		return nil, ErrRefreshIssueFailed
	}

	newHash := tokens.SHA256Hex(rawRefresh)
	ttlSeconds := int(s.deps.RefreshTTL.Seconds())

	tokenInput := repository.CreateRefreshTokenInput{
		TenantID:   tenantID,
		ClientID:   in.ClientID,
		UserID:     user.ID,
		TokenHash:  newHash,
		TTLSeconds: ttlSeconds,
	}

	if _, err := tda.Tokens().Create(ctx, tokenInput); err != nil {
		log.Error("failed to persist new refresh token", logger.Err(err))
		return nil, ErrRefreshIssueFailed
	}

	// Revoke old token (best effort)
	if err := tda.Tokens().Revoke(ctx, rt.ID); err != nil {
		log.Warn("failed to revoke old refresh token", logger.Err(err))
	}

	log.Info("refresh successful")

	return &dto.RefreshResult{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(time.Until(exp).Seconds()),
	}, nil
}

func (s *refreshService) selectSigningKey(tda store.TenantDataAccess) (kid string, priv any, pub any, err error) {
	settings := tda.Settings()
	if types.IssuerMode(settings.IssuerMode) == types.IssuerModePath {
		return s.deps.Issuer.Keys.ActiveForTenant(tda.Slug())
	}
	return s.deps.Issuer.Keys.Active()
}
