package social

import (
	"context"
	"fmt"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/oauth/github"
	"github.com/dropDatabas3/hellojohn/internal/oauth/google"
	sec "github.com/dropDatabas3/hellojohn/internal/security/secretbox"
)

// OIDCClient provides OAuth/OIDC operations for a provider.
type OIDCClient interface {
	// AuthURL returns the authorization URL for redirecting the user.
	AuthURL(ctx context.Context, state, nonce string) (string, error)
	// ExchangeCode exchanges an authorization code for tokens.
	ExchangeCode(ctx context.Context, code string) (*OIDCTokens, error)
	// VerifyIDToken verifies an ID token and returns claims.
	VerifyIDToken(ctx context.Context, idToken, nonce string) (*OIDCClaims, error)
}

// OIDCTokens contains tokens from code exchange.
type OIDCTokens struct {
	AccessToken  string
	IDToken      string
	RefreshToken string
	ExpiresIn    int
}

// OIDCClaims contains claims from ID token.
type OIDCClaims struct {
	Sub           string
	Email         string
	EmailVerified bool
	Name          string
	GivenName     string
	FamilyName    string
	Picture       string
	Locale        string
	Nonce         string
}

// OIDCFactory creates OIDCClient instances for providers.
type OIDCFactory interface {
	// Google returns a Google OIDC client for the tenant.
	Google(ctx context.Context, tenantSlug, baseURL string) (OIDCClient, error)
	// GitHub returns a GitHub OAuth client for the tenant.
	GitHub(ctx context.Context, tenantSlug, baseURL string) (OIDCClient, error)
}

// DefaultOIDCFactory implements OIDCFactory using TenantProvider.
type DefaultOIDCFactory struct {
	tenantProvider TenantProvider
}

// NewOIDCFactory creates a new OIDCFactory.
func NewOIDCFactory(tp TenantProvider) OIDCFactory {
	return &DefaultOIDCFactory{tenantProvider: tp}
}

// Google creates a Google OIDC client for the tenant.
func (f *DefaultOIDCFactory) Google(ctx context.Context, tenantSlug, baseURL string) (OIDCClient, error) {
	// Get tenant from TenantProvider
	if f.tenantProvider == nil {
		return nil, fmt.Errorf("tenant provider not configured")
	}

	tenant, err := f.tenantProvider.GetTenant(ctx, tenantSlug)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	settings := &tenant.Settings
	if settings.SocialProviders == nil {
		return nil, fmt.Errorf("social providers not configured")
	}

	// Check Google enabled
	if !settings.SocialLoginEnabled && !settings.SocialProviders.GoogleEnabled {
		return nil, fmt.Errorf("google not enabled for tenant")
	}

	clientID := settings.SocialProviders.GoogleClient
	secretEnc := settings.SocialProviders.GoogleSecret

	if clientID == "" {
		return nil, fmt.Errorf("google client_id not configured")
	}

	// Decrypt secret
	var clientSecret string
	if secretEnc != "" {
		clientSecret, err = sec.Decrypt(secretEnc)
		if err != nil {
			// Fallback: if it doesn't look encrypted (no pipe separator), use as plain text (dev mode)
			if !strings.Contains(secretEnc, "|") {
				clientSecret = secretEnc
			} else {
				return nil, fmt.Errorf("failed to decrypt google secret: %w", err)
			}
		}
	}

	if clientSecret == "" {
		return nil, fmt.Errorf("google client_secret not configured")
	}

	// Build callback URL
	redirectURL := fmt.Sprintf("%s/v2/auth/social/google/callback", strings.TrimRight(baseURL, "/"))

	// Create Google OIDC client
	oidc := google.New(clientID, clientSecret, redirectURL, []string{"openid", "profile", "email"})

	return &googleOIDCAdapter{oidc: oidc}, nil
}

// googleOIDCAdapter adapts google.OIDC to OIDCClient interface.
type googleOIDCAdapter struct {
	oidc *google.OIDC
}

func (a *googleOIDCAdapter) AuthURL(ctx context.Context, state, nonce string) (string, error) {
	return a.oidc.AuthURL(ctx, state, nonce)
}

func (a *googleOIDCAdapter) ExchangeCode(ctx context.Context, code string) (*OIDCTokens, error) {
	resp, err := a.oidc.ExchangeCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return &OIDCTokens{
		AccessToken:  resp.AccessToken,
		IDToken:      resp.IDToken,
		RefreshToken: resp.RefreshTok,
		ExpiresIn:    resp.ExpiresIn,
	}, nil
}

func (a *googleOIDCAdapter) VerifyIDToken(ctx context.Context, idToken, nonce string) (*OIDCClaims, error) {
	claims, err := a.oidc.VerifyIDToken(ctx, idToken, nonce)
	if err != nil {
		return nil, err
	}
	return &OIDCClaims{
		Sub:           claims.Sub,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		Picture:       claims.Picture,
		Locale:        claims.Locale,
		Nonce:         claims.Nonce,
	}, nil
}

// ─────────────────────────────────────────────────────────────
// GitHub OAuth Client
// ─────────────────────────────────────────────────────────────

// GitHub creates a GitHub OAuth client for the tenant.
func (f *DefaultOIDCFactory) GitHub(ctx context.Context, tenantSlug, baseURL string) (OIDCClient, error) {
	if f.tenantProvider == nil {
		return nil, fmt.Errorf("tenant provider not configured")
	}

	tenant, err := f.tenantProvider.GetTenant(ctx, tenantSlug)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	settings := &tenant.Settings
	if settings.SocialProviders == nil {
		return nil, fmt.Errorf("social providers not configured")
	}

	// Check GitHub enabled
	if !settings.SocialLoginEnabled && !settings.SocialProviders.GitHubEnabled {
		return nil, fmt.Errorf("github not enabled for tenant")
	}

	clientID := settings.SocialProviders.GitHubClient
	secretEnc := settings.SocialProviders.GitHubSecret

	if clientID == "" {
		return nil, fmt.Errorf("github client_id not configured")
	}

	// Decrypt secret
	var clientSecret string
	if secretEnc != "" {
		clientSecret, err = sec.Decrypt(secretEnc)
		if err != nil {
			// Fallback: if it doesn't look encrypted (no pipe separator), use as plain text (dev mode)
			if !strings.Contains(secretEnc, "|") {
				clientSecret = secretEnc
			} else {
				return nil, fmt.Errorf("failed to decrypt github secret: %w", err)
			}
		}
	}

	if clientSecret == "" {
		return nil, fmt.Errorf("github client_secret not configured")
	}

	// Build callback URL
	redirectURL := fmt.Sprintf("%s/v2/auth/social/github/callback", strings.TrimRight(baseURL, "/"))

	// Create GitHub OAuth client
	oauth := github.New(clientID, clientSecret, redirectURL, []string{"user:email", "read:user"})

	return &githubOAuthAdapter{oauth: oauth}, nil
}

// githubOAuthAdapter adapts github.OAuth to OIDCClient interface.
// Note: GitHub uses OAuth 2.0, not OIDC, so VerifyIDToken requires API call.
type githubOAuthAdapter struct {
	oauth       *github.OAuth
	accessToken string // cached for VerifyIDToken
}

func (a *githubOAuthAdapter) AuthURL(ctx context.Context, state, nonce string) (string, error) {
	return a.oauth.AuthURL(ctx, state, nonce)
}

func (a *githubOAuthAdapter) ExchangeCode(ctx context.Context, code string) (*OIDCTokens, error) {
	resp, err := a.oauth.ExchangeCode(ctx, code)
	if err != nil {
		return nil, err
	}
	// Cache access token for VerifyIDToken (GitHub doesn't have ID tokens)
	a.accessToken = resp.AccessToken
	return &OIDCTokens{
		AccessToken: resp.AccessToken,
		IDToken:     "", // GitHub doesn't provide ID tokens
	}, nil
}

// VerifyIDToken for GitHub fetches user info from API since there's no ID token.
// The nonce parameter is ignored for GitHub (it's in the state token).
func (a *githubOAuthAdapter) VerifyIDToken(ctx context.Context, idToken, nonce string) (*OIDCClaims, error) {
	// For GitHub, we use the access token to fetch user info
	accessToken := a.accessToken
	if accessToken == "" && idToken != "" {
		// Fallback: use idToken as accessToken if passed
		accessToken = idToken
	}
	if accessToken == "" {
		return nil, fmt.Errorf("no access token available")
	}

	// Fetch user info with email
	userInfo, err := a.oauth.GetUserWithEmail(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get github user info: %w", err)
	}

	// Map GitHub user info to OIDC claims
	return &OIDCClaims{
		Sub:           fmt.Sprintf("%d", userInfo.ID), // GitHub user ID as string
		Email:         userInfo.Email,
		EmailVerified: true, // GitHub emails are verified by API
		Name:          userInfo.Name,
		GivenName:     "", // GitHub doesn't provide first/last name split
		FamilyName:    "",
		Picture:       userInfo.AvatarURL,
		Locale:        "",
		Nonce:         nonce, // Pass through for consistency
	}, nil
}
