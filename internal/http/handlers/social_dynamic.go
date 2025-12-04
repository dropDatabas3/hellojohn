package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/config"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/oauth/google"
	sec "github.com/dropDatabas3/hellojohn/internal/security/secretbox"
)

// DynamicSocialHandler handles social login requests by loading tenant configuration dynamically.
type DynamicSocialHandler struct {
	cfg        *config.Config
	c          *app.Container
	refreshTTL time.Duration
}

func NewDynamicSocialHandler(cfg *config.Config, c *app.Container, refreshTTL time.Duration) *DynamicSocialHandler {
	return &DynamicSocialHandler{
		cfg:        cfg,
		c:          c,
		refreshTTL: refreshTTL,
	}
}

// ServeHTTP dispatches to the appropriate provider handler based on path and tenant config.
// Path expected: /v1/auth/social/{provider}/{action}
// e.g. /v1/auth/social/google/start?tenant=acme
func (h *DynamicSocialHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/auth/social/"), "/")
	if len(parts) < 2 {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "ruta incompleta", 1690)
		return
	}
	provider := parts[0]
	action := parts[1]

	// Extract Tenant Slug from query param (for start) or state (for callback)
	var tenantSlug string
	var err error

	if action == "start" {
		tenantSlug = r.URL.Query().Get("tenant")
		// Fallback to tenant_id if tenant slug is missing, but prefer slug.
		if tenantSlug == "" {
			tenantSlug = r.URL.Query().Get("tenant_id")
		}
		if tenantSlug == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_tenant", "tenant slug requerido", 1691)
			return
		}
	} else if action == "callback" {
		state := r.URL.Query().Get("state")
		if state == "" {
			// Try to fallback to debug headers if enabled, otherwise error
			if r.Header.Get("X-Debug-Google-Email") != "" {
				// Debug mode might not have state, but let's assume standard flow first
			}
			httpx.WriteError(w, http.StatusBadRequest, "missing_state", "state requerido", 1693)
			return
		}
		// We need to parse state to get Tenant Slug.
		claims, err := h.parseStateGeneric(state)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_state", "state inválido: "+err.Error(), 1694)
			return
		}
		// We stored slug in "tenant_slug" or we can look up by "tid" (UUID) if we have a reverse lookup.
		// But better to store slug in state if we want to look it up by slug.
		// However, googleHandler expects "tid" (UUID).
		// So we should have stored both or look up by UUID.
		// Since we need to load tenant config to get credentials to verify token, we need to know which tenant!
		// If we stored "tenant_slug" in state, we can use it.
		tenantSlug, _ = claims["tenant_slug"].(string)
		if tenantSlug == "" {
			// Fallback: try to find tenant by TID UUID if possible?
			// cpctx.Provider (FS) might not support GetTenantByID efficiently or at all if ID is inside YAML.
			// But we can iterate.
			// For now, let's assume we stored tenant_slug.
			httpx.WriteError(w, http.StatusBadRequest, "invalid_state_tenant", "tenant_slug en state inválido", 1695)
			return
		}
	} else {
		httpx.WriteError(w, http.StatusNotFound, "unknown_action", "acción desconocida", 1696)
		return
	}

	// Load Tenant Config from Control Plane (FS)
	if cpctx.Provider == nil {
		httpx.WriteError(w, http.StatusInternalServerError, "cp_missing", "control plane no inicializado", 1697)
		return
	}

	tenant, err := cpctx.Provider.GetTenantBySlug(r.Context(), tenantSlug)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 1698)
		return
	}

	// Check Provider Config in Tenant Settings
	settings := tenant.Settings
	if settings.SocialProviders == nil {
		httpx.WriteError(w, http.StatusForbidden, "provider_disabled", "providers no configurados", 1699)
		return
	}

	var clientID, clientSecret string
	var enabled bool

	switch provider {
	case "google":
		enabled = settings.SocialLoginEnabled || settings.SocialProviders.GoogleEnabled
		clientID = settings.SocialProviders.GoogleClient
		secretEnc := settings.SocialProviders.GoogleSecret
		if secretEnc != "" {
			clientSecret, err = sec.Decrypt(secretEnc)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "decrypt_error", "error descifrando secreto", 1705)
				return
			}
		}
	default:
		httpx.WriteError(w, http.StatusBadRequest, "unknown_provider", "proveedor no soportado: "+provider, 1703)
		return
	}

	if !enabled {
		httpx.WriteError(w, http.StatusForbidden, "provider_disabled", provider+" deshabilitado", 1701)
		return
	}

	if clientID == "" || clientSecret == "" {
		httpx.WriteError(w, http.StatusInternalServerError, "provider_misconfigured", "credenciales faltantes", 1702)
		return
	}

	// Instantiate Provider Handler
	switch provider {
	case "google":
		// Redirect URI construction
		redirectURL := fmt.Sprintf("%s://%s/v1/auth/social/google/callback", r.URL.Scheme, r.Host)
		if r.URL.Scheme == "" {
			scheme := "https"
			if strings.HasPrefix(r.Host, "localhost") || strings.HasPrefix(r.Host, "127.0.0.1") {
				scheme = "http"
			}
			redirectURL = fmt.Sprintf("%s://%s/v1/auth/social/google/callback", scheme, r.Host)
		}

		oidc := google.New(clientID, clientSecret, redirectURL, []string{"openid", "profile", "email"})

		// Attempt to get pool from Store
		type poolGetter interface {
			Pool() *pgxpool.Pool
		}
		var pool *pgxpool.Pool
		if pg, ok := h.c.Store.(poolGetter); ok {
			pool = pg.Pool()
		} else {
			httpx.WriteError(w, http.StatusInternalServerError, "store_incompatible", "store no es compatible con social login (no pool)", 1704)
			return
		}

		gh := &googleHandler{
			cfg:       h.cfg,
			c:         h.c,
			pool:      pool,
			oidc:      oidc,
			issuer:    h.c.Issuer,
			validator: redirectValidatorAdapter{repo: h.c.Store},
			issuerTok: tokenIssuerAdapter{c: h.c, refreshTTL: h.refreshTTL},
		}

		if action == "start" {
			// Extract params
			cid := r.URL.Query().Get("client_id")
			redir := r.URL.Query().Get("redirect_uri")
			if cid == "" {
				httpx.WriteError(w, http.StatusBadRequest, "missing_client_id", "client_id requerido", 1602)
				return
			}

			// Parse Tenant UUID
			tid, err := uuid.Parse(tenant.ID)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "invalid_tenant_id", "tenant ID inválido en configuración", 1707)
				return
			}

			// Generate state compatible with googleHandler
			nonce := randB64(16)
			// We use h.generateState to include tenant_slug AND tid/cid/redir/nonce
			state, err := h.generateState(tid, tenantSlug, cid, redir, nonce)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "state_error", "error generando state", 1706)
				return
			}

			// Redirect to Google
			authURL, err := oidc.AuthURL(r.Context(), state, nonce)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "auth_url_error", "error generando auth url", 1606)
				return
			}
			http.Redirect(w, r, authURL, http.StatusFound)

		} else {
			// Callback
			// Delegate to googleHandler.callback
			// It will parse state, verify signature, extract tid/cid, exchange code, etc.
			// Since we initialized gh with the correct OIDC (tenant creds), it should work.
			gh.callback(w, r)
		}

	default:
		httpx.WriteError(w, http.StatusBadRequest, "unknown_provider", "proveedor no soportado: "+provider, 1703)
	}
}

func (h *DynamicSocialHandler) generateState(tid uuid.UUID, tenantSlug, cid, redir, nonce string) (string, error) {
	now := time.Now().UTC()
	claims := jwtv5.MapClaims{
		"iss":         h.c.Issuer.Iss,
		"aud":         "google-state", // Must match what callback expects
		"exp":         now.Add(h.cfg.Providers.LoginCodeTTL).Unix(),
		"iat":         now.Unix(),
		"nbf":         now.Unix(),
		"jti":         fmt.Sprintf("%d", time.Now().UnixNano()),
		"tid":         tid.String(),
		"tenant_slug": tenantSlug, // Extra claim for us to resolve tenant in callback
		"cid":         cid,
		"redir":       redir,
		"nonce":       nonce,
	}
	signed, _, err := h.c.Issuer.SignRaw(claims)
	return signed, err
}

func (h *DynamicSocialHandler) parseStateGeneric(s string) (map[string]any, error) {
	tk, err := jwtv5.Parse(s, h.c.Issuer.Keyfunc(), jwtv5.WithValidMethods([]string{"EdDSA"}))
	if err != nil || !tk.Valid {
		return nil, errors.New("invalid_state_token")
	}
	claims, ok := tk.Claims.(jwtv5.MapClaims)
	if !ok {
		return nil, errors.New("bad_state_claims")
	}
	if iss, _ := claims["iss"].(string); iss != h.c.Issuer.Iss {
		return nil, errors.New("state_iss_mismatch")
	}
	if aud, _ := claims["aud"].(string); aud != "google-state" {
		return nil, errors.New("state_aud_mismatch")
	}
	if expf, ok := claims["exp"].(float64); ok {
		if time.Unix(int64(expf), 0).Before(time.Now().Add(-30 * time.Second)) {
			return nil, errors.New("state_expired")
		}
	}
	return map[string]any(claims), nil
}
