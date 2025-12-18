/*
oidc_discovery.go â€” OIDC Discovery (global + per-tenant) + issuer/jwks por tenant

QuÃ© es este archivo (la posta)
------------------------------
Este archivo publica documentos â€œdiscoveryâ€ para clientes OIDC:
	- Discovery global: un solo issuer (c.Issuer.Iss) + endpoints globales
	- Discovery por tenant: issuer resuelto por settings del tenant + jwks_uri por tenant

En la prÃ¡ctica, estos endpoints son los que consumen:
	- SPAs/Frontends (para saber authorize/token/userinfo/jwks)
	- SDKs/CLI (para bootstrap)

Ojo: acÃ¡ NO se registran rutas ni se usa chi params; el handler por-tenant parsea el path â€œa manoâ€.

Dependencias reales
-------------------
- c.Issuer.Iss: base issuer global (string)
- cpctx.Provider (solo per-tenant): lookup de tenant por slug
- jwtx.ResolveIssuer(base, issuerMode, slug, issuerOverride): define issuer efectivo por tenant
- httpx.WriteJSON / httpx.WriteError: contrato de JSON error/response
- setNoStore(w): helper compartido (definido en jwks.go) para evitar cache en respuestas sensibles

Rutas soportadas (contrato efectivo)
------------------------------------
A) Discovery global
-------------------
- GET/HEAD /.well-known/openid-configuration
		(ruta exacta depende del wiring del router, pero este handler asume que se monta ahÃ­)

		Response:
			- issuer = strings.TrimRight(c.Issuer.Iss, "/")
			- authorization_endpoint = {issuer}/oauth2/authorize
			- token_endpoint         = {issuer}/oauth2/token
			- userinfo_endpoint      = {issuer}/userinfo
			- jwks_uri               = {issuer}/.well-known/jwks.json

		Headers:
			- Cache-Control: public, max-age=600, must-revalidate
			- Expires: now+10m

B) Discovery por tenant
-----------------------
- GET/HEAD /t/{slug}/.well-known/openid-configuration
		Parsing:
			- Valida que el path tenga prefix "/t/" y suffix "/.well-known/openid-configuration"
			- Extrae slug y valida regex ^[a-z0-9\-]{1,64}$

		Fuente de verdad:
			- Requiere cpctx.Provider
			- cpctx.Provider.GetTenantBySlug(ctx, slug)
			- iss = jwtx.ResolveIssuer(base, issuerMode, slug, issuerOverride)

		Nota de compat:
			- Mantiene endpoints globales (authorize/token/userinfo) para no romper rutas existentes
			- Pero jwks_uri sÃ­ es por tenant: {base}/.well-known/jwks/{slug}.json

		Headers:
			- setNoStore(w) (no-cache/no-store) para evitar cache agresivo cuando rota issuer/jwks

Campos OIDC â€œde factoâ€
----------------------
Este discovery declara:
	- response_types_supported: ["code"]
	- grant_types_supported: ["authorization_code", "refresh_token"]
	- id_token_signing_alg_values_supported: ["EdDSA"]
	- token_endpoint_auth_methods_supported: ["none"] (public client)
	- code_challenge_methods_supported: ["S256"] (PKCE)
	- scopes_supported: ["openid","email","profile","offline_access"]

Importante: esto debe ser consistente con lo que realmente acepta /oauth2/token.

Problemas principales (cuellos de botella + bugs probables)
-----------------------------------------------------------
1) regexp.MustCompile por request (per-tenant)
	 En NewTenantOIDCDiscoveryHandler se compila el regex en cada request.
	 V2: declarar un var regexp global y reusar.

2) Base issuer vs â€œhost externoâ€
	 Usa c.Issuer.Iss tal cual (config). DetrÃ¡s de proxies puede diferir del host/scheme pÃºblico.
	 SoluciÃ³n tÃ­pica: fijar issuer explÃ­citamente (recomendado) o derivarlo de headers confiables.

3) Contrato de endpoints parcialmente â€œtenant-awareâ€
	 El discovery por tenant cambia issuer y jwks_uri, pero deja authorize/token/userinfo globales.
	 Esto es compat-friendly, pero en V2 convendrÃ­a que todo el surface sea coherente:
		 /t/{slug}/oauth2/authorize, /t/{slug}/oauth2/token, /t/{slug}/userinfo, etc.

CÃ³mo lo refactorizarÃ­a a V2 (plan concreto)
-------------------------------------------
FASE 1 â€” ValidaciÃ³n/parseo consistente
	- Usar router con params (chi) o un parser central para rutas /t/{slug}/...
	- Regex precompilada.

FASE 2 â€” Discovery coherente por tenant
	- Publicar endpoints tenant-scoped (si el router v2 lo soporta) o documentar formalmente
		que solo issuer/jwks varÃ­an y el resto es global.

FASE 3 â€” Metadata driven
	- Generar scopes/claims supported desde configuraciÃ³n/registries reales (evitar drift).

*/

package handlers

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
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

// NewOIDCDiscoveryHandler publica el documento de configuraciÃ³n OIDC.
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

			// Scopes y claims tÃ­picos que exponemos hoy
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
		iss := jwtx.ResolveIssuer(base, string(tdef.Settings.IssuerMode), slug, tdef.Settings.IssuerOverride)

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

		// Per-tenant discovery: preferimos no cachear agresivamente (cambios de issuer/jwks por rotaciÃ³n)
		setNoStore(w)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, meta)
	})
}
