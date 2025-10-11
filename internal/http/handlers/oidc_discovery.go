package handlers

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
)

type oidcMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserinfoEndpoint                  string   `json:"userinfo_endpoint"`
	JWKSURI                           string   `json:"jwks_uri"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	ClaimsSupported                   []string `json:"claims_supported,omitempty"`
}

// NewOIDCDiscoveryHandler publica el documento de configuración OIDC.
// Usa el issuer configurado y arma las URLs absolutas para los endpoints.
func NewOIDCDiscoveryHandler(c *app.Container) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/HEAD", 1001)
			return
		}

		iss := strings.TrimRight(c.Issuer.Iss, "/")
		meta := oidcMetadata{
			Issuer:                iss,
			AuthorizationEndpoint: iss + "/oauth2/authorize",
			TokenEndpoint:         iss + "/oauth2/token",
			UserinfoEndpoint:      iss + "/userinfo",
			JWKSURI:               iss + "/.well-known/jwks.json",

			// Soportamos Authorization Code (con PKCE)
			ResponseTypesSupported: []string{"code"},
			GrantTypesSupported:    []string{"authorization_code", "refresh_token"},
			SubjectTypesSupported:  []string{"public"},

			// Firmamos ID Tokens con EdDSA (Ed25519)
			IDTokenSigningAlgValuesSupported: []string{"EdDSA"},

			// No exigimos auth de cliente (client_secret) para SPA: public client
			TokenEndpointAuthMethodsSupported: []string{"none"},

			// PKCE S256
			CodeChallengeMethodsSupported: []string{"S256"},

			// Scopes y claims típicos que exponemos hoy
			ScopesSupported: []string{"openid", "email", "profile", "offline_access"},
			ClaimsSupported: []string{
				"iss", "sub", "aud", "exp", "iat", "nbf",
				"nonce", "auth_time", "acr", "amr",
				"at_hash", "tid",
				"email", "email_verified",
			},
		}

		// Cache razonable (los clientes suelen cachear discovery por un rato)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=600, must-revalidate")
		w.Header().Set("Expires", time.Now().Add(10*time.Minute).UTC().Format(http.TimeFormat))

		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, meta)
	})
}

// NewTenantOIDCDiscoveryHandler publica el documento OIDC por tenant en:
//
//	GET/HEAD /t/{slug}/.well-known/openid-configuration
//
// Mantiene endpoints globales (authorize/token/userinfo) para compatibilidad;
// el issuer y jwks_uri son por tenant.
func NewTenantOIDCDiscoveryHandler(c *app.Container) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/HEAD", 1001)
			return
		}

		// Path esperado: /t/{slug}/.well-known/openid-configuration
		const prefix = "/t/"
		const suffix = "/.well-known/openid-configuration"
		path := r.URL.Path
		if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
			http.NotFound(w, r)
			return
		}
		slug := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
		// Validar slug [a-z0-9-]{1,64}
		if !regexp.MustCompile(`^[a-z0-9\-]{1,64}$`).MatchString(slug) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid tenant slug", 2100)
			return
		}

		if cpctx.Provider == nil {
			httpx.WriteError(w, http.StatusInternalServerError, "server_error", "control-plane no configurado", 2105)
			return
		}
		tdef, err := cpctx.Provider.GetTenantBySlug(r.Context(), slug)
		if err != nil || tdef == nil {
			httpx.WriteError(w, http.StatusNotFound, "invalid_request", "tenant not found", 2106)
			return
		}

		base := strings.TrimRight(c.Issuer.Iss, "/")
		iss := jwtx.ResolveIssuer(base, tdef.Settings.IssuerMode, slug, tdef.Settings.IssuerOverride)

		// Endpoints globales (sin /t/{slug}/) para no romper rutas existentes
		meta := oidcMetadata{
			Issuer:                            iss,
			AuthorizationEndpoint:             base + "/oauth2/authorize",
			TokenEndpoint:                     base + "/oauth2/token",
			UserinfoEndpoint:                  base + "/userinfo",
			JWKSURI:                           base + "/.well-known/jwks/" + slug + ".json",
			ResponseTypesSupported:            []string{"code"},
			GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
			SubjectTypesSupported:             []string{"public"},
			IDTokenSigningAlgValuesSupported:  []string{"EdDSA"},
			TokenEndpointAuthMethodsSupported: []string{"none"},
			CodeChallengeMethodsSupported:     []string{"S256"},
			ScopesSupported:                   []string{"openid", "email", "profile", "offline_access"},
			ClaimsSupported: []string{
				"iss", "sub", "aud", "exp", "iat", "nbf",
				"nonce", "auth_time", "acr", "amr",
				"at_hash", "tid",
				"email", "email_verified",
			},
		}

		// Per-tenant discovery: preferimos no cachear agresivamente (cambios de issuer/jwks por rotación)
		setNoStore(w)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, meta)
	})
}
