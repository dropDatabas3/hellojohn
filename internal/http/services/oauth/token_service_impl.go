package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane"
	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	store "github.com/dropDatabas3/hellojohn/internal/store"
)

// TokenDeps contains dependencies for token service.
type TokenDeps struct {
	DAL          store.DataAccessLayer
	Issuer       *jwtx.Issuer
	Cache        CacheClient
	ControlPlane controlplane.Service
	RefreshTTL   time.Duration
}

// tokenService implements TokenService.
type tokenService struct {
	dal        store.DataAccessLayer
	issuer     *jwtx.Issuer
	cache      CacheClient
	cp         controlplane.Service
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
		cache:      d.Cache,
		cp:         d.ControlPlane,
		refreshTTL: ttl,
	}
}

// ExchangeAuthorizationCode handles grant_type=authorization_code (PKCE).
func (s *tokenService) ExchangeAuthorizationCode(ctx context.Context, req AuthCodeRequest) (*TokenResponse, error) {
	log := logger.From(ctx).With(logger.Layer("service"), logger.Op("oauth.token.authcode"))

	// Validate required fields
	if req.Code == "" || req.RedirectURI == "" || req.ClientID == "" || req.CodeVerifier == "" {
		return nil, ErrTokenInvalidRequest
	}

	// Lookup client from control plane
	client, tenantSlug, err := s.lookupClient(ctx, req.TenantSlug, req.ClientID)
	if err != nil {
		log.Warn("client not found", logger.String("client_id", req.ClientID), logger.Err(err))
		return nil, ErrTokenInvalidClient
	}

	// Consume authorization code from cache (one-shot)
	// Hardening: check for hashed code first
	codeHash := tokens.SHA256Base64URL(req.Code)
	keyHashed := "code:" + codeHash
	keyPlain := "code:" + req.Code

	key := keyHashed
	data, ok := s.cache.Get(keyHashed)
	if !ok {
		// Fallback: check plain code (legacy/transient)
		key = keyPlain
		data, ok = s.cache.Get(keyPlain)
	}

	if !ok {
		log.Warn("authorization code not found")
		return nil, ErrTokenInvalidGrant
	}
	s.cache.Delete(key)

	var ac AuthCodePayload
	if err := json.Unmarshal(data, &ac); err != nil {
		log.Warn("authorization code corrupted", logger.Err(err))
		return nil, ErrTokenInvalidGrant
	}

	// Validate authorization code
	if time.Now().After(ac.ExpiresAt) {
		log.Warn("authorization code expired")
		return nil, ErrTokenInvalidGrant
	}
	if ac.ClientID != client.ClientID || ac.RedirectURI != req.RedirectURI {
		log.Warn("client/redirect_uri mismatch")
		return nil, ErrTokenInvalidGrant
	}

	// Validate PKCE S256
	verifierHash := tokens.SHA256Base64URL(req.CodeVerifier)
	if !strings.EqualFold(ac.ChallengeMethod, "S256") || !strings.EqualFold(ac.CodeChallenge, verifierHash) {
		log.Warn("PKCE verification failed")
		return nil, ErrTokenInvalidGrant
	}

	// Build access token claims
	reqScopes := strings.Fields(ac.Scope)
	acrVal := "urn:hellojohn:loa:1"
	for _, v := range ac.AMR {
		if v == "mfa" {
			acrVal = "urn:hellojohn:loa:2"
			break
		}
	}

	std := map[string]any{
		"tid":   tenantSlug,
		"amr":   ac.AMR,
		"acr":   acrVal,
		"scope": strings.Join(reqScopes, " "),
		"scp":   reqScopes,
	}
	custom := map[string]any{}

	// Resolve effective issuer for tenant
	effIss := s.resolveEffectiveIssuer(ctx, tenantSlug)

	// Issue access token
	access, exp, err := s.issuer.IssueAccessForTenant(tenantSlug, effIss, ac.UserID, req.ClientID, std, custom)
	if err != nil {
		log.Error("failed to issue access token", logger.Err(err))
		return nil, ErrTokenServerError
	}

	// Create refresh token
	rawRT, err := s.createRefreshToken(ctx, tenantSlug, client.ClientID, ac.UserID)
	if err != nil {
		log.Error("failed to create refresh token", logger.Err(err))
		return nil, ErrTokenServerError
	}

	// Issue ID token
	idStd := map[string]any{
		"tid":     tenantSlug,
		"at_hash": atHash(access),
		"azp":     req.ClientID,
		"acr":     acrVal,
		"amr":     ac.AMR,
	}
	idExtra := map[string]any{}
	if ac.Nonce != "" {
		idExtra["nonce"] = ac.Nonce
	}

	idToken, _, err := s.issuer.IssueIDTokenForTenant(tenantSlug, effIss, ac.UserID, req.ClientID, idStd, idExtra)
	if err != nil {
		log.Error("failed to issue id_token", logger.Err(err))
		return nil, ErrTokenServerError
	}

	log.Info("authorization_code exchanged",
		logger.TenantID(tenantSlug),
		logger.String("client_id", req.ClientID),
	)

	return &TokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(exp).Seconds()),
		RefreshToken: rawRT,
		IDToken:      idToken,
		Scope:        ac.Scope,
	}, nil
}

// ExchangeRefreshToken handles grant_type=refresh_token (rotation).
func (s *tokenService) ExchangeRefreshToken(ctx context.Context, req RefreshTokenRequest) (*TokenResponse, error) {
	log := logger.From(ctx).With(logger.Layer("service"), logger.Op("oauth.token.refresh"))

	if req.ClientID == "" || req.RefreshToken == "" {
		return nil, ErrTokenInvalidRequest
	}

	// Lookup client
	client, tenantSlug, err := s.lookupClient(ctx, req.TenantSlug, req.ClientID)
	if err != nil {
		log.Warn("client not found", logger.String("client_id", req.ClientID))
		return nil, ErrTokenInvalidClient
	}

	// Get tenant data access
	tenantData, err := s.dal.ForTenant(ctx, tenantSlug)
	if err != nil {
		log.Warn("tenant data access not available", logger.TenantID(tenantSlug))
		return nil, ErrTokenDBNotConfigured
	}

	// Lookup refresh token by hash
	tokenHash := tokens.SHA256Base64URL(req.RefreshToken)
	rt, err := tenantData.Tokens().GetByHash(ctx, tokenHash)
	if err != nil {
		log.Warn("refresh token not found or invalid")
		return nil, ErrTokenInvalidGrant
	}

	// Validate refresh token
	now := time.Now()
	// NOTE: Checking clientID match if stored token has one
	if rt.RevokedAt != nil || !now.Before(rt.ExpiresAt) || (rt.ClientID != "" && rt.ClientID != client.ClientID) {
		log.Warn("refresh token revoked/expired/mismatched")
		return nil, ErrTokenInvalidGrant
	}

	// Build access token claims
	std := map[string]any{
		"tid": tenantSlug,
		"amr": []string{"refresh"},
		"acr": "urn:hellojohn:loa:1",
		"scp": []string{},
	}
	custom := map[string]any{}

	// Resolve effective issuer
	effIss := s.resolveEffectiveIssuer(ctx, tenantSlug)

	// Issue new access token
	access, exp, err := s.issuer.IssueAccessForTenant(tenantSlug, effIss, rt.UserID, req.ClientID, std, custom)
	if err != nil {
		log.Error("failed to issue access token", logger.Err(err))
		return nil, ErrTokenServerError
	}

	// Rotate refresh token: revoke old, create new
	_ = tenantData.Tokens().Revoke(ctx, rt.ID)

	newRT, err := s.createRefreshToken(ctx, tenantSlug, client.ClientID, rt.UserID)
	if err != nil {
		log.Error("failed to create new refresh token", logger.Err(err))
		return nil, ErrTokenServerError
	}

	log.Info("refresh_token exchanged",
		logger.TenantID(tenantSlug),
		logger.String("client_id", req.ClientID),
	)

	return &TokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(exp).Seconds()),
		RefreshToken: newRT,
	}, nil
}

// ExchangeClientCredentials handles grant_type=client_credentials (M2M).
func (s *tokenService) ExchangeClientCredentials(ctx context.Context, req ClientCredentialsRequest) (*TokenResponse, error) {
	log := logger.From(ctx).With(logger.Layer("service"), logger.Op("oauth.token.clientcreds"))

	if req.ClientID == "" {
		return nil, ErrTokenInvalidRequest
	}

	// Lookup client
	client, tenantSlug, err := s.lookupClient(ctx, req.TenantSlug, req.ClientID)
	if err != nil {
		log.Warn("client not found", logger.String("client_id", req.ClientID))
		return nil, ErrTokenInvalidClient
	}

	// Must be confidential
	if client.Type != repository.ClientTypeConfidential {
		log.Warn("client_credentials requires confidential client")
		return nil, ErrTokenUnauthorizedClient
	}

	// Validate client secret
	if err := s.validateClientSecret(ctx, tenantSlug, client, req.ClientSecret); err != nil {
		log.Warn("invalid client credentials")
		return nil, ErrTokenInvalidClient
	}

	// Validate requested scopes
	reqScopes := []string{}
	// NOTE: default logic if empty?
	if req.Scope != "" {
		reqScopes = strings.Fields(req.Scope)
	}
	for _, scope := range reqScopes {
		if !s.cp.IsScopeAllowed(client, scope) {
			log.Warn("scope not allowed", logger.String("scope", scope))
			return nil, ErrTokenInvalidScope
		}
	}

	// Determine scope output
	var scopeOut string
	if len(reqScopes) > 0 {
		scopeOut = strings.Join(reqScopes, " ")
	} else {
		scopeOut = strings.Join(client.Scopes, " ")
	}

	// Build claims
	std := map[string]any{
		"tid":   tenantSlug,
		"amr":   []string{"client"},
		"acr":   "urn:hellojohn:loa:1",
		"scp":   scopeOut,
		"scope": scopeOut,
	}
	custom := map[string]any{}

	// Resolve effective issuer
	effIss := s.resolveEffectiveIssuer(ctx, tenantSlug)

	// Issue access token (sub = clientID for M2M)
	access, exp, err := s.issuer.IssueAccessForTenant(tenantSlug, effIss, req.ClientID, req.ClientID, std, custom)
	if err != nil {
		log.Error("failed to issue access token", logger.Err(err))
		return nil, ErrTokenServerError
	}

	log.Info("client_credentials token issued",
		logger.TenantID(tenantSlug),
		logger.String("client_id", req.ClientID),
	)

	return &TokenResponse{
		AccessToken: access,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(exp).Seconds()),
		Scope:       scopeOut,
	}, nil
}

// --- Helper methods ---

func (s *tokenService) lookupClient(ctx context.Context, tenantSlug, clientID string) (*repository.Client, string, error) {
	if s.cp == nil {
		return nil, "", fmt.Errorf("control plane not initialized")
	}

	// Try the provided tenant slug first
	if tenantSlug != "" {
		c, err := s.cp.GetClient(ctx, tenantSlug, clientID)
		if err == nil && c != nil {
			return c, tenantSlug, nil
		}
	}

	// Search across all tenants
	tenants, err := s.cp.ListTenants(ctx)
	if err != nil {
		return nil, "", err
	}
	for _, t := range tenants {
		if t.Slug == tenantSlug {
			continue
		}
		c, err := s.cp.GetClient(ctx, t.Slug, clientID)
		if err == nil && c != nil {
			return c, t.Slug, nil
		}
	}
	return nil, "", fmt.Errorf("client not found")
}

func (s *tokenService) resolveEffectiveIssuer(ctx context.Context, tenantSlug string) string {
	if s.cp == nil || s.issuer == nil {
		if s.issuer != nil {
			return s.issuer.Iss
		}
		return ""
	}
	ten, err := s.cp.GetTenant(ctx, tenantSlug)
	if err != nil || ten == nil {
		return s.issuer.Iss
	}
	return jwtx.ResolveIssuer(s.issuer.Iss, string(ten.Settings.IssuerMode), ten.Slug, ten.Settings.IssuerOverride)
}

func (s *tokenService) validateClientSecret(ctx context.Context, tenantSlug string, client *repository.Client, providedSecret string) error {
	if client.Type != repository.ClientTypeConfidential {
		return nil // only confidential clients have secrets
	}
	dec, err := s.cp.DecryptClientSecret(ctx, tenantSlug, client.ClientID)
	if err != nil {
		return err
	}
	if dec == "" || !subtleEq(dec, providedSecret) {
		return fmt.Errorf("invalid secret")
	}
	return nil
}

func (s *tokenService) createRefreshToken(ctx context.Context, tenantSlug, clientID, userID string) (string, error) {
	// Generate opaque token
	rawRT, err := tokens.GenerateOpaqueToken(32)
	if err != nil {
		return "", err
	}

	// Get tenant data access
	tenantData, err := s.dal.ForTenant(ctx, tenantSlug)
	if err != nil {
		return "", fmt.Errorf("tenant data access: %w", err)
	}

	// Store hashed refresh token
	tokenHash := tokens.SHA256Base64URL(rawRT)
	ttlSeconds := int(s.refreshTTL.Seconds())

	// Create refresh token in repo
	_, err = tenantData.Tokens().Create(ctx, repository.CreateRefreshTokenInput{
		TenantID:   tenantSlug,
		ClientID:   clientID,
		UserID:     userID,
		TokenHash:  tokenHash,
		TTLSeconds: ttlSeconds,
	})
	if err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}

	return rawRT, nil
}

// atHash computes at_hash = base64url(left-most 128 bits of SHA-256(access_token))
func atHash(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:len(sum)/2])
}

// subtleEq performs constant-time string comparison
func subtleEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
