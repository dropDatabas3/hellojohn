/*
oauth_token.go â€” Token Endpoint OAuth2/OIDC â€œtodo-en-unoâ€: auth_code(PKCE)+refresh(rotaciÃ³n)+client_credentials(M2M)

QuÃ© es este archivo (la posta)
------------------------------
Este archivo implementa el endpoint `/token` (OAuth2 Token Endpoint) en un solo handler gigantesco:
- Multiplexa por `grant_type`:
  1) `authorization_code` (con PKCE S256) -> emite access + refresh + id_token
  2) `refresh_token` (rotaciÃ³n)            -> emite access + refresh nuevo (y revoca el viejo)
  3) `client_credentials` (M2M)           -> emite access (sin refresh)
- Resuelve â€œstore activoâ€ con precedencia `tenantDB > globalDB` porque:
  - los refresh tokens estÃ¡n en DB (y en multi-tenant cada tenant tiene su propio schema/DB)
  - ademÃ¡s lee user metadata + RBAC desde el store para armar claims â€œSYS namespaceâ€
- Resuelve issuer efectivo por tenant (issuerMode: global/path/domain + override) para:
  - firmar con issuer correcto
  - poner system claims en namespace correcto (claimsNS.SystemNamespace)

Este archivo ES el core del login OIDC: si acÃ¡ falla, se cae todo.

Entradas / Formatos admitidos
-----------------------------
- Solo POST.
- Content-Type esperado: `application/x-www-form-urlencoded` (OAuth2 estÃ¡ndar).
- Lee `grant_type` y el resto de campos desde `r.PostForm`.
- Limita body a 64KB con `http.MaxBytesReader`.
- Timeout hard: 3s para todo el handler (context.WithTimeout).

âš ï¸ Nota: con 3s, cualquier lookup lento (DB, provider, cache) te puede pegar timeout y devolver errores 500/timeout.

Pieza clave: â€œActive Storeâ€ y por quÃ© importa tanto
---------------------------------------------------
El handler elige `activeStore` asÃ­:
1) Si hay `TenantSQLManager`, intenta `GetPG(ctx, cpctx.ResolveTenant(r))`.
2) Si no, cae a `c.Store` global.

Pero OJO: en `authorization_code` y `refresh_token`, **luego vuelve a re-seleccionar store**
basado en el `tenantSlug` real obtenido al resolver el client (`helpers.LookupClient`).
Eso es CRÃTICO porque:
- el cÃ³digo/consent pueden estar en cache â€œglobalâ€, pero el refresh token va a DB del tenant
- si usÃ¡s el store equivocado, te explota por FK / â€œtoken not foundâ€ / escribir en el tenant incorrecto.

Caminos principales (por grant_type)
====================================

A) grant_type = authorization_code (OIDC code flow + PKCE)
---------------------------------------------------------
Entrada esperada (form):
- grant_type=authorization_code
- code=...
- redirect_uri=...
- client_id=...
- code_verifier=...  (PKCE S256)

Pasos:
1) ValidaciÃ³n de campos obligatorios.
2) Resolve client + tenantSlug:
   - `client, tenantSlug := helpers.LookupClient(ctx, r, clientID)`
   - Valida existencia del client. (AcÃ¡ NO valida secret todavÃ­a; estÃ¡ comentado.)
3) Re-selecciÃ³n del store por tenantSlug:
   - `TenantSQLManager.GetPG(ctx, tenantSlug)` si existe.
4) â€œCompat layerâ€ a core.Client legacy:
   - cl.ID = client.ClientID
   - cl.TenantID = tenantSlug
   - cl.RedirectURIs = client.RedirectURIs
   - cl.Scopes = client.Scopes
5) Consume authorization code desde cache (one-shot):
   - key := "code:" + code   (match con authorize handler)
   - si no existe => invalid_grant
   - delete inmediato (one-shot)
   - unmarshal -> authCode payload
6) Validaciones del authCode:
   - no expirado
   - coincide `client_id` y `redirect_uri`
   - PKCE: compara challenge S256:
       verifierHash := tokens.SHA256Base64URL(code_verifier)
       ac.CodeChallenge debe == verifierHash
7) ConstrucciÃ³n de claims para Access Token:
   - scopes: strings.Fields(ac.Scope)
   - amr: ac.AMR
   - acr: loa1 o loa2 si amr incluye "mfa"
   - std claims incluye:
       tid, amr, acr, scope, scp
   - custom claims inicial vacÃ­o
   - hook: `applyAccessClaimsHook(...)` (puede modificar std/custom)
8) Resolver issuer efectivo del tenant:
   - effIss = jwtx.ResolveIssuer(baseIss, issuerMode, slug, override)
9) SYS namespace claims (roles/perms/metadata):
   - activeStore.GetUserByID(ac.UserID)
   - si store soporta rbacReader => roles/perms
   - helpers.PutSystemClaimsV2(custom, effIss, metadata, roles, perms)
10) Emitir access token:
   - c.Issuer.IssueAccessForTenant(tenantSlug, effIss, userID, clientID, std, custom)
11) Emitir refresh token (rotaciÃ³n):
   - requiere hasStore=true (DB disponible)
   - si store soporta `CreateRefreshTokenTC(tenantID, clientID, userID, ttl)`:
       - intenta resolver realTenantID (UUID) via Provider.GetTenantBySlug
       - crea refresh â€œTCâ€ (token crudo lo genera el store)
   - else legacy:
       - genera rawRT (opaque)
       - guarda hash en DB usando:
         a) CreateRefreshTokenTC(...) legacy raro (hash hex) o
         b) CreateRefreshToken(...) clÃ¡sico (hash base64url)
       - acÃ¡ hay mezcla de hashes/formats: es una fuente de bugs
12) Emitir ID Token (OIDC):
   - idStd: tid, at_hash(access), azp, acr, amr
   - idExtra: nonce si existe
   - hook: applyIDClaimsHook(...)
   - c.Issuer.IssueIDTokenForTenant(...)
13) Respuesta JSON no-store:
   - access_token, refresh_token, id_token, expires_in, scope

Puntos sensibles / donde se rompe:
- Si authorize guardÃ³ code en otro prefijo => invalid_grant.
- Si PKCE hash no coincide (ojo, acÃ¡ usa SHA256Base64URL del verifier, no la fÃ³rmula exacta base64url(SHA256(verifier)) â€œsin hexâ€; asumimos que SHA256Base64URL hace eso).
- Si tenantSlug y tenantID real se confunden: refresh token TC pide tenant UUID; code trae tid como slug.

B) grant_type = refresh_token (rotaciÃ³n)
----------------------------------------
Entrada esperada:
- grant_type=refresh_token
- client_id=...
- refresh_token=...

Pasos:
1) ValidaciÃ³n: requiere DB (hasStore).
2) Resolve client + tenantSlug con LookupClient.
3) Re-selecciÃ³n del store por tenantSlug (CRÃTICO).
4) â€œCompat layerâ€ core.Client legacy (igual que arriba).
5) Lookup refresh token:
   - Si store soporta tcRefresh:
       - hash := tokens.SHA256Base64URL(refresh_token)
       - GetRefreshTokenByHashTC(ctx, tenantSlug, client.ClientID, hash)
       - OJO: acÃ¡ tenantSlug se pasa como tenantID: si la implementaciÃ³n espera UUID, cagaste.
   - else legacy:
       - GetRefreshTokenByHash(ctx, hash)
6) Validaciones del refresh:
   - no revocado
   - no expirado
   - rt.ClientIDText == client.ClientID (mismatch => invalid_grant)
7) ConstrucciÃ³n claims access:
   - amr=["refresh"], acr loa1, tid=tenantSlug, scp vacÃ­o
   - hook applyAccessClaimsHook
   - SYS claims igual (GetUserByID + roles/perms) con issuer efectivo
8) Emitir access:
   - IssueAccessForTenant(tenantSlug, effIss, rt.UserID, clientID, std, custom)
9) RotaciÃ³n refresh:
   - Si tcRefresh:
       - RevokeRefreshTokensByUserClientTC(tenantSlug, client.ClientID, rt.UserID)
       - CreateRefreshTokenTC(tenantSlug, client.ClientID, rt.UserID, ttl)
       - (revoca â€œtodosâ€ del user+client; agresivo pero simple)
   - else legacy:
       - genera newRT y guarda CreateRefreshToken(... parentID=&rt.ID)
       - revoca el viejo rt
10) Respuesta:
   - access_token + refresh_token nuevo + expires_in

Puntos sensibles:
- Inconsistencia de â€œtenant identifierâ€: a veces slug, a veces UUID.
- Hashing: TC vs legacy usan funciones distintas (base64url vs hex) en el mismo archivo.
- RotaciÃ³n por â€œrevoke all user+clientâ€ te puede romper multi-device (depende del producto).

C) grant_type = client_credentials (M2M)
----------------------------------------
Entrada:
- grant_type=client_credentials
- client_id=...
- client_secret=...
- scope=... (opcional)

Pasos:
1) LookupClient -> (client, tenantSlug)
2) Validaciones:
   - client.Type debe ser "confidential"
   - ValidateClientSecret(...) debe pasar
3) Validar scopes:
   - requested scopes debe ser subset de client scopes (DefaultIsScopeAllowed)
4) ConstrucciÃ³n claims:
   - amr=["client"], acr loa1
   - tid=tenantSlug
   - scopeOut: si viene scope en req => ese; sino default client.Scopes
   - std["scope"/"scp"] set
   - hook applyAccessClaimsHook
5) Resolver issuer efectivo por tenant
6) Emitir access:
   - sub = clientID (emite â€œon behalf of clientâ€)
7) Respuesta JSON:
   - access_token + scope + expires_in (sin refresh)

Puntos sensibles:
- Si ValidateClientSecret depende de provider/secret storage lento -> timeout 3s.
- â€œtidâ€ en M2M: hoy es slug; si mÃ¡s adelante querÃ©s UUID, esto cambia claim contract.

Problemas gordos detectables (de diseÃ±o, no de estilo)
------------------------------------------------------
1) Mezcla de responsabilidades a lo pavote:
   - parsing + validaciÃ³n oauth
   - lookup client / tenant
   - cache (code)
   - store selection + persist refresh
   - issuer resolution + firma
   - RBAC/metadata -> custom claims
   - hooks
   Todo en una funciÃ³n.

2) Identidad de tenant inconsistente (slug vs UUID):
   - authCode.TenantID a veces es slug (segÃºn authorize)
   - CreateRefreshTokenTC a veces exige UUID (por FK)
   - refresh_token flow usa tenantSlug en calls TC
   => esto es bug waiting to happen.

3) Hash formats inconsistentes:
   - SHA256Base64URL vs SHA256Hex (aparece en legacy TC path)
   - y encima el introspect/revoke usan SHA256Base64URL
   => si guardÃ¡s con hex y buscÃ¡s con base64url, no lo encontrÃ¡s nunca.

4) Cache key inconsistente entre handlers:
   - acÃ¡ usa "code:"+code
   - consent handler usa "oidc:code:"+SHA256Base64URL(code)
   => hay dos â€œfamiliasâ€ de codes. Si mezclÃ¡s flows, invalid_grant.

5) Timeout fijo 3s para TODO:
   - en prod con DB medio lenta o provider remoto, te va a cortar piernas.

CÃ³mo separarlo bien (V2) â€” por capas y por caminos (bien concreto)
==================================================================

Objetivo
--------
Que el handler quede como â€œcontrollerâ€ finito:
- parsea request
- llama a un service por grant_type
- traduce errores a OAuth JSON
y que lo heavy viva en servicios/repos con interfaces claras.

Carpeta sugerida
----------------
/internal/oauth/
  token_controller.go          // HTTP handler, parse + routing por grant
  token_dtos.go                // request/response structs + validation
  token_errors.go              // mapping a {error, error_description}
  services/
    token_service.go           // interface + orchestraciÃ³n general
    auth_code_service.go       // GrantAuthorizationCode
    refresh_service.go         // GrantRefreshToken
    client_credentials_service.go // GrantClientCredentials
  ports/
    client_registry.go         // Lookup client, validate redirect/secret, allowed scopes
    code_store.go              // guardar/consumir auth codes (cache)
    consent_store.go           // opcional
    refresh_tokens.go          // crear/buscar/revocar refresh (UNIFICADO)
    user_repo.go               // GetUserByID + RBAC
    issuer.go                  // Resolve issuer + Issue access/id tokens
    hooks.go                   // access/id hooks
  adapters/
    controlplane_client_registry.go
    cache_code_store.go
    pg_refresh_tokens_repo.go
    pg_user_repo.go
    issuer_adapter.go

1) Controller (HTTP)
--------------------
Responsabilidad:
- Enforce POST
- Parse form (64KB)
- Crear â€œrequest contextâ€ (timeout configurable)
- Construir DTO segÃºn grant_type
- Llamar `TokenService.Exchange(ctx, dto)`
- Responder JSON no-store

DTOs (ejemplo mental):
- AuthCodeTokenRequest { code, redirectURI, clientID, codeVerifier }
- RefreshTokenRequest { clientID, refreshToken }
- ClientCredentialsRequest { clientID, clientSecret, scope }

Errores:
- Mapear a OAuth estÃ¡ndar:
  - invalid_request
  - invalid_client
  - invalid_grant
  - unsupported_grant_type
  - invalid_scope
  - server_error
Y siempre setear no-store.

2) Service por grant (negocio/orquestaciÃ³n)
-------------------------------------------
Cada grant en su servicio con un flujo claro y testeable.

A) AuthCodeService.Exchange(...)
- Steps:
  1) client := ClientRegistry.Lookup(clientID) -> devuelve (client, tenantRef)
  2) ClientRegistry.ValidateRedirect(client, redirectURI)
  3) codePayload := CodeStore.Consume(code) (one-shot)  // UN SOLO FORMATO de key
  4) Validate codePayload: exp, client match, redirect match, pkce
  5) tenant := TenantResolver.Resolve(tenantRef) -> {tenantSlug, tenantUUID, issuerEffective}
  6) user := UserRepo.GetUser(codePayload.userID)
  7) claims := ClaimsBuilder.BuildAccessClaims(tenant, user, reqScopes, amr, hooks)
  8) access := Issuer.IssueAccess(tenantSlug, issuerEffective, userID, clientID, claims)
  9) refresh := RefreshTokens.RotateOrCreate(tenantUUID, clientID, userID, ttl)
  10) idToken := Issuer.IssueIDToken(... at_hash, nonce, acr/amr)
  11) return TokenResponse{access, refresh, id_token, exp, scope}

B) RefreshService.Exchange(...)
- Steps:
  1) client := ClientRegistry.Lookup(clientID) -> tenantRef
  2) tenant := TenantResolver.Resolve(tenantRef)
  3) rt := RefreshTokens.ValidateAndGet(tenantUUID, clientID, refreshToken)
  4) claims builder (amr=["refresh"])
  5) access := issue
  6) newRefresh := RefreshTokens.Rotate(tenantUUID, clientID, userID, refreshToken, ttl)
  7) return response

C) ClientCredentialsService.Exchange(...)
- Steps:
  1) client := ClientRegistry.Lookup(clientID)
  2) Validate confidential + ValidateSecret
  3) Validate scope subset
  4) tenant resolve => issuerEffective
  5) issue access sub=clientID
  6) return response

3) Ports (interfaces) â€” donde se corta lo feo
---------------------------------------------
A) TenantResolver / ClientRegistry
- ClientRegistry:
  - LookupClient(ctx, clientID) -> (client, tenantSlug)
  - ValidateClientSecret(ctx, tenantSlug, client, secret)
  - IsScopeAllowed(client, scope) bool
- TenantResolver:
  - ResolveBySlug(ctx, slug) -> {TenantSlug, TenantUUID, IssuerEffective}
  - (cacheable)

B) CodeStore (cache)
- Consume(code string) (*AuthCodePayload, error)
- Store(payload) (si el authorize lo hace acÃ¡ tambiÃ©n)
Regla: UN SOLO esquema de key:
- "oidc:code:"+code (raw) o hash, pero **uno**.
Yo prefiero hash:
- key := "oidc:code:"+SHA256Base64URL(code)
porque te evita keys enormes y logs con token crudo.

C) RefreshTokensRepository (UNIFICAR)
Este es EL punto mÃ¡s importante.
DefinÃ­ una interfaz Ãºnica:
- Create(ctx, tenantUUID, clientIDText, userID, ttl) (rawToken string, err)
- GetByRaw(ctx, tenantUUID, clientIDText, rawToken) (*RefreshToken, err)
- Revoke(ctx, tenantUUID, tokenID)
- Rotate(ctx, tenantUUID, clientIDText, rawToken, ttl) (newRaw string, err)

Y adentro decidÃ­s:
- siempre persistir hash con el MISMO algoritmo (ej SHA256Base64URL)
- nunca mezclar hex/base64url

D) UserRepo / RBAC
- GetUserByID(ctx, userID) -> user(metadata)
- GetRoles/GetPerms opcional
Esto evita type assertions en runtime.

E) IssuerPort
- ResolveIssuer(ctx, tenantSlug) -> string (efectivo)
- IssueAccess(ctx, tenantSlug, effIss, sub, aud, std, custom) -> (jwt, exp)
- IssueIDToken(ctx, ...) -> (jwt, exp)

F) HooksPort
- ApplyAccessClaimsHook(...)
- ApplyIDClaimsHook(...)
Como un decorator que modifica maps.

4) QuÃ© queda en cada capa (resumen ultra claro)
-----------------------------------------------
HTTP Controller:
- parse + validate â€œshapeâ€ de request
- mapping errores oauth
- no-store headers

Services:
- orquestaciÃ³n del grant
- decisiones (rotaciÃ³n, scopes, acr/amr)
- llama a ports

Adapters:
- Cache (c.Cache)
- Control plane provider (cpctx.Provider)
- TenantSQLManager (GetPG)
- DB repos (refresh/user/rbac)
- Issuer real (c.Issuer)

Contrato interno recomendado (para no repetir bugs)
===================================================
- TenantRef interno SIEMPRE incluye:
  - tenantSlug (para issuer keys y rutas)
  - tenantUUID (para DB FK)
- Hash de refresh token SIEMPRE:
  - `tokens.SHA256Base64URL(raw)` (y listo)
- Key de auth code SIEMPRE:
  - prefijo Ãºnico + hash (no raw) o raw consistente en TODO el sistema
  - y â€œconsumeâ€ siempre borra

Chequeos extra que yo meterÃ­a (sin cambiar tu producto)
--------------------------------------------------------
- Client auth en authorization_code (confidential):
  - hoy estÃ¡ TODO commented: cualquiera con code+verifier puede canjear si roba code.
  - mÃ­nimo: si client.Type == confidential => exigir secret o private_key_jwt.
- Rate limit por IP/client_id en /token (especial refresh).
- Observabilidad:
  - loggear request_id + tenantSlug + clientID + grant_type (sin tokens)
- Timeout por dependencia:
  - 3s total es medio agresivo; mejor:
    - 3s para DB
    - 1s para provider
    - 50ms para cache
  con context sub-timeouts adentro del service.

*/

package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/http/v1/helpers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/v1/core"
)

// compute at_hash = base64url( left-most 128 bits of SHA-256(access_token) )
func atHash(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:len(sum)/2]) // 16 bytes
}

func NewOAuthTokenHandler(c *app.Container, refreshTTL time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Timeout de 3s para endpoint crÃ­tico
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		r = r.WithContext(ctx)

		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}

		// Resolver store con precedencia: tenantDB > globalDB
		var activeStore core.Repository
		var hasStore bool

		if c.TenantSQLManager != nil {
			// Intentar obtener store del tenant actual
			tenantSlug := cpctx.ResolveTenant(r)
			if tenantStore, err := c.TenantSQLManager.GetPG(r.Context(), tenantSlug); err == nil && tenantStore != nil {
				activeStore = tenantStore
				hasStore = true
			}
		}
		// Fallback a global store si no hay tenant store
		if !hasStore && c.Store != nil {
			activeStore = c.Store
			hasStore = true
		}
		// OAuth2: application/x-www-form-urlencoded
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10) // 64KB
		if err := r.ParseForm(); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "form invÃ¡lido", 2201)
			return
		}
		grantType := strings.TrimSpace(r.PostForm.Get("grant_type"))

		switch grantType {

		// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ authorization_code + PKCE â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
		case "authorization_code":
			code := strings.TrimSpace(r.PostForm.Get("code"))
			redirectURI := strings.TrimSpace(r.PostForm.Get("redirect_uri"))
			clientID := strings.TrimSpace(r.PostForm.Get("client_id"))
			codeVerifier := strings.TrimSpace(r.PostForm.Get("code_verifier"))

			if code == "" || redirectURI == "" || clientID == "" || codeVerifier == "" {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "faltan parÃ¡metros", 2203)
				return
			}

			ctx := r.Context()

			client, tenantSlug, err := helpers.LookupClient(ctx, r, clientID)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client not found", 2204)
				return
			}

			// Re-select activeStore based on resolved tenant (CRITICAL for Refresh Token FK)
			if c.TenantSQLManager != nil {
				if tStore, errS := c.TenantSQLManager.GetPG(ctx, tenantSlug); errS == nil && tStore != nil {
					activeStore = tStore
				}
			}

			// TODO: Implementar ValidateClientSecret cuando se agregue auth del cliente
			// if err := helpers.ValidateClientSecret(ctx, r, tenantSlug, client, clientSecret); err != nil {
			//     httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "bad credentials", 2205)
			//     return
			// }

			// Mapear client FS a estructura legacy para compatibilidad
			cl := &core.Client{
				ID:           client.ClientID,
				TenantID:     tenantSlug,
				RedirectURIs: client.RedirectURIs,
				Scopes:       client.Scopes,
			}
			_ = tenantSlug

			// Cargar y consumir el code (1 uso)
			// key := "oidc:code:" + tokens.SHA256Base64URL(code) // OLD mismatch
			key := "code:" + code // MATCH authorize handler
			log.Printf("DEBUG: oauth_token attempting to retrieve key: %s", key)
			data, ok := c.Cache.Get(key)
			if !ok {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_grant", "authorization code invÃ¡lido", 2205)
				return
			}
			c.Cache.Delete(key)

			var ac authCode
			if err := json.Unmarshal(data, &ac); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_grant", "authorization code corrupto", 2206)
				return
			}
			// Expirado
			if time.Now().After(ac.ExpiresAt) {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_grant", "authorization code expirado", 2207)
				return
			}
			// Validar contra el UUID interno del client (ac.ClientID contiene cl.ID desde authorize)
			// Aceptamos que en el form venga el client_id "pÃºblico": lo resolvemos y comparamos con cl.ID
			if ac.ClientID != cl.ID || ac.RedirectURI != redirectURI {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_grant", "client/redirect_uri no coinciden", 2208)
				return
			}
			// PKCE S256
			verifierHash := tokens.SHA256Base64URL(codeVerifier)
			if !strings.EqualFold(ac.ChallengeMethod, "S256") || !strings.EqualFold(ac.CodeChallenge, verifierHash) {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_grant", "PKCE invÃ¡lido", 2209)
				return
			}

			// Access Token (std/custom + hook)
			reqScopes := strings.Fields(ac.Scope)
			accessAMR := ac.AMR
			acrVal := "urn:hellojohn:loa:1"
			for _, v := range accessAMR {
				if v == "mfa" {
					acrVal = "urn:hellojohn:loa:2"
					break
				}
			}
			std := map[string]any{
				"tid":   ac.TenantID,
				"amr":   accessAMR,
				"acr":   acrVal,
				"scope": strings.Join(reqScopes, " "),
				"scp":   reqScopes, // compatibilidad con SDKs que esperan lista
			}
			custom := map[string]any{}

			std, custom = applyAccessClaimsHook(ctx, c, ac.TenantID, clientID, ac.UserID, reqScopes, ac.AMR, std, custom)

			// Resolver issuer efectivo por tenant (para SYS namespace y firma)
			effIss := c.Issuer.Iss
			if cpctx.Provider != nil {
				if ten, errTen := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug); errTen == nil && ten != nil {
					effIss = jwtx.ResolveIssuer(c.Issuer.Iss, string(ten.Settings.IssuerMode), ten.Slug, ten.Settings.IssuerOverride)
				}
			}

			// SYS namespace a partir de metadata + RBAC (Fase 2)
			if u, err := activeStore.GetUserByID(ctx, ac.UserID); err == nil && u != nil {
				type rbacReader interface {
					GetUserRoles(ctx context.Context, userID string) ([]string, error)
					GetUserPermissions(ctx context.Context, userID string) ([]string, error)
				}
				var roles, perms []string
				if rr, ok := activeStore.(rbacReader); ok {
					roles, _ = rr.GetUserRoles(ctx, ac.UserID)
					perms, _ = rr.GetUserPermissions(ctx, ac.UserID)
				}
				custom = helpers.PutSystemClaimsV2(custom, effIss, u.Metadata, roles, perms)
			}

			access, exp, err := c.Issuer.IssueAccessForTenant(tenantSlug, effIss, ac.UserID, clientID, std, custom)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 2210)
				return
			}

			// Refresh (rotaciÃ³n igual que en /v1/auth/*)
			var rawRT string
			if !hasStore {
				httpx.WriteError(w, http.StatusServiceUnavailable, "db_not_configured", "no hay base de datos configurada para emitir refresh tokens", 2212)
				return
			}

			type tc interface {
				CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error)
			}
			if tcs, ok := activeStore.(tc); ok {
				// Resolver Tenant UUID real (ac.TenantID es slug)
				var realTenantID string
				if cpctx.Provider != nil {
					if tObj, errT := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug); errT == nil && tObj != nil {
						realTenantID = tObj.ID
					}
				}
				if realTenantID == "" {
					// Fallback: si no pudimos resolver (raro), asumimos que el store aceptarÃ¡ el slug o fallarÃ¡
					realTenantID = ac.TenantID
				}

				// preferir TC con client_id_text (sin FK)
				tok, err := tcs.CreateRefreshTokenTC(ctx, realTenantID, cl.ClientID, ac.UserID, refreshTTL)
				if err != nil {
					log.Printf("oauth token: create refresh TC err: %v", err)
					httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 2212)
					return
				}
				rawRT = tok // ya viene raw (tu TC genera el token)
			} else {
				// legacy
				var err error
				rawRT, err = tokens.GenerateOpaqueToken(32)
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 2211)
					return
				}

				// Usar CreateRefreshTokenTC para oauth token endpoint
				if tcStore, ok := activeStore.(interface {
					CreateRefreshTokenTC(context.Context, string, string, string, time.Time, *string) (string, error)
				}); ok {
					hash := tokens.SHA256Hex(rawRT)
					if _, err := tcStore.CreateRefreshTokenTC(ctx, ac.TenantID, cl.ClientID, hash, time.Now().Add(refreshTTL), nil); err != nil {
						log.Printf("oauth token: create refresh TC err: %v", err)
						httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh TC", 2212)
						return
					}
				} else {
					// Fallback legacy
					hash := tokens.SHA256Base64URL(rawRT)
					if _, err := activeStore.CreateRefreshToken(ctx, ac.UserID, cl.ID, hash, time.Now().Add(refreshTTL), nil); err != nil {
						log.Printf("oauth token: create refresh err: %v", err)
						httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 2212)
						return
					}
				}
			}

			// ID Token (sin SYS_NS)
			idStd := map[string]any{
				"tid":     ac.TenantID,
				"at_hash": atHash(access),
				"azp":     clientID,
				"acr":     acrVal,
				"amr":     accessAMR, // aÃ±adir AMR al ID Token para interoperabilidad
			}
			idExtra := map[string]any{}
			if ac.Nonce != "" {
				idExtra["nonce"] = ac.Nonce
			}
			idStd, idExtra = applyIDClaimsHook(ctx, c, ac.TenantID, clientID, ac.UserID, reqScopes, ac.AMR, idStd, idExtra)

			idToken, _, err := c.Issuer.IssueIDTokenForTenant(tenantSlug, effIss, ac.UserID, clientID, idStd, idExtra)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el id_token", 2213)
				return
			}

			// Evitar cache
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")

			resp := map[string]any{
				"token_type":    "Bearer",
				"expires_in":    int64(time.Until(exp).Seconds()),
				"access_token":  access,
				"refresh_token": rawRT,
				"id_token":      idToken,
				"scope":         ac.Scope,
			}
			httpx.WriteJSON(w, http.StatusOK, resp)

		// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ refresh_token (rotaciÃ³n) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
		case "refresh_token":
			clientID := strings.TrimSpace(r.PostForm.Get("client_id"))
			refreshToken := strings.TrimSpace(r.PostForm.Get("refresh_token"))
			if clientID == "" || refreshToken == "" {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "client_id y refresh_token son obligatorios", 2220)
				return
			}

			ctx := r.Context()

			if !hasStore {
				httpx.WriteError(w, http.StatusServiceUnavailable, "db_not_configured", "no hay base de datos configurada para refrescar tokens", 2222)
				return
			}

			client, tenantSlug, err := helpers.LookupClient(ctx, r, clientID)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client not found", 2221)
				return
			}
			// Re-select activeStore based on resolved tenant
			if c.TenantSQLManager != nil {
				if tStore, errS := c.TenantSQLManager.GetPG(ctx, tenantSlug); errS == nil && tStore != nil {
					activeStore = tStore
				}
			}

			// Mapear client FS a estructura legacy para compatibilidad
			cl := &core.Client{
				ID:           client.ClientID,
				TenantID:     tenantSlug,
				RedirectURIs: client.RedirectURIs,
				Scopes:       client.Scopes,
			}
			_ = tenantSlug

			type tcRefresh interface {
				CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error)
				GetRefreshTokenByHashTC(ctx context.Context, tenantID, clientIDText, tokenHash string) (*core.RefreshToken, error)
				RevokeRefreshTokensByUserClientTC(ctx context.Context, tenantID, clientID, userID string) (int64, error)
			}
			type legacyGet interface {
				GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*core.RefreshToken, error)
			}

			var rt *core.RefreshToken
			if tcr, ok := activeStore.(tcRefresh); ok {
				// Usar mÃ©todo TC con tenant+client
				hash := tokens.SHA256Base64URL(refreshToken)
				rt, err = tcr.GetRefreshTokenByHashTC(ctx, tenantSlug, client.ClientID, hash)
				if err != nil {
					status := http.StatusInternalServerError
					if err == core.ErrNotFound {
						status = http.StatusBadRequest
					}
					httpx.WriteError(w, status, "invalid_grant", "refresh invÃ¡lido", 2222)
					return
				}
			} else if lg, ok := activeStore.(legacyGet); ok {
				// legacy tal como hoy...
				hash := tokens.SHA256Base64URL(refreshToken)
				rt, err = lg.GetRefreshTokenByHash(ctx, hash)
				if err != nil {
					status := http.StatusInternalServerError
					if err == core.ErrNotFound {
						status = http.StatusBadRequest
					}
					httpx.WriteError(w, status, "invalid_grant", "refresh invÃ¡lido", 2222)
					return
				}
			} else {
				httpx.WriteError(w, http.StatusServiceUnavailable, "store_not_supported", "store no soporta refresh tokens", 2222)
				return
			}
			now := time.Now()
			if rt.RevokedAt != nil || !now.Before(rt.ExpiresAt) || rt.ClientIDText != client.ClientID {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_grant", "refresh revocado/expirado o mismatched client", 2223)
				return
			}

			std := map[string]any{
				"tid": cl.TenantID,
				"amr": []string{"refresh"},
				"acr": "urn:hellojohn:loa:1",
				"scp": []string{}, // refresh flow: sin scopes explÃ­citos aquÃ­
			}
			custom := map[string]any{}

			std, custom = applyAccessClaimsHook(ctx, c, cl.TenantID, clientID, rt.UserID, []string{}, []string{"refresh"}, std, custom)
			if u, err := activeStore.GetUserByID(ctx, rt.UserID); err == nil && u != nil {
				type rbacReader interface {
					GetUserRoles(ctx context.Context, userID string) ([]string, error)
					GetUserPermissions(ctx context.Context, userID string) ([]string, error)
				}
				var roles, perms []string
				if rr, ok := activeStore.(rbacReader); ok {
					roles, _ = rr.GetUserRoles(ctx, rt.UserID)
					perms, _ = rr.GetUserPermissions(ctx, rt.UserID)
				}
				// Resolver issuer efectivo
				effIss := c.Issuer.Iss
				if cpctx.Provider != nil {
					if ten, errTen := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug); errTen == nil && ten != nil {
						effIss = jwtx.ResolveIssuer(c.Issuer.Iss, string(ten.Settings.IssuerMode), ten.Slug, ten.Settings.IssuerOverride)
					}
				}
				custom = helpers.PutSystemClaimsV2(custom, effIss, u.Metadata, roles, perms)
			}

			// Emitir usando clave del tenant y issuer efectivo
			effIss := c.Issuer.Iss
			if cpctx.Provider != nil {
				if ten, errTen := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug); errTen == nil && ten != nil {
					effIss = jwtx.ResolveIssuer(c.Issuer.Iss, string(ten.Settings.IssuerMode), ten.Slug, ten.Settings.IssuerOverride)
				}
			}
			access, exp, err := c.Issuer.IssueAccessForTenant(tenantSlug, effIss, rt.UserID, clientID, std, custom)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir access", 2224)
				return
			}

			var newRT string
			if tcs, ok := activeStore.(tcRefresh); ok {
				// TC: revocar tokens del usuario+cliente y crear uno nuevo
				_, _ = tcs.RevokeRefreshTokensByUserClientTC(ctx, tenantSlug, client.ClientID, rt.UserID)
				newRT, err = tcs.CreateRefreshTokenTC(ctx, tenantSlug, client.ClientID, rt.UserID, refreshTTL)
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh TC", 2226)
					return
				}
			} else {
				// legacy
				newRT, err = tokens.GenerateOpaqueToken(32)
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 2225)
					return
				}
				newHash := tokens.SHA256Base64URL(newRT)
				if _, err := activeStore.CreateRefreshToken(ctx, rt.UserID, cl.ID, newHash, now.Add(refreshTTL), &rt.ID); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 2226)
					return
				}
				_ = activeStore.RevokeRefreshToken(ctx, rt.ID)
			}

			// Evitar cache
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")

			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"token_type":    "Bearer",
				"expires_in":    int64(time.Until(exp).Seconds()),
				"access_token":  access,
				"refresh_token": newRT,
			})

		// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ client_credentials (M2M) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
		case "client_credentials":
			clientID := strings.TrimSpace(r.PostForm.Get("client_id"))
			clientSecret := strings.TrimSpace(r.PostForm.Get("client_secret"))
			scope := strings.TrimSpace(r.PostForm.Get("scope"))

			if clientID == "" {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "client_id requerido", 2230)
				return
			}

			ctx := r.Context()
			client, tenantSlug, err := helpers.LookupClient(ctx, r, clientID)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client not found", 2231)
				return
			}

			// Must be confidential and secret must match
			if client.Type != "confidential" {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized_client", "client_credentials solo para clientes confidenciales", 2232)
				return
			}
			if err := helpers.ValidateClientSecret(ctx, r, tenantSlug, client, clientSecret); err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "credenciales invÃ¡lidas", 2233)
				return
			}

			// Validate requested scopes subset of client scopes
			reqScopes := []string{}
			if scope != "" {
				for _, s := range strings.Fields(scope) {
					reqScopes = append(reqScopes, s)
				}
			}
			for _, s := range reqScopes {
				if !controlplane.DefaultIsScopeAllowed(client, s) {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_scope", "scope no permitido", 2234)
					return
				}
			}

			// Standard M2M claims
			amr := []string{"client"}
			acr := "urn:hellojohn:loa:1"
			std := map[string]any{
				"tid": tenantSlug, // FS mode uses slug; hook may adjust or add further details
				"amr": amr,
				"acr": acr,
			}
			var scopeOut string
			if len(reqScopes) > 0 {
				scopeOut = strings.Join(reqScopes, " ")
				std["scp"] = scopeOut
				std["scope"] = scopeOut
			} else {
				// default to client's configured scopes
				scopeOut = strings.Join(client.Scopes, " ")
				std["scp"] = scopeOut
				std["scope"] = scopeOut
			}
			custom := map[string]any{}

			// Hook
			std, custom = applyAccessClaimsHook(ctx, c, tenantSlug, clientID, "", reqScopes, amr, std, custom)

			// Resolve effective issuer for tenant
			effIss := c.Issuer.Iss
			if cpctx.Provider != nil {
				if ten, errTen := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug); errTen == nil && ten != nil {
					effIss = jwtx.ResolveIssuer(c.Issuer.Iss, string(ten.Settings.IssuerMode), ten.Slug, ten.Settings.IssuerOverride)
				}
			}

			// Issue access token on behalf of the client (sub = client_id)
			// Issue token via issuer helper (per-tenant key if needed)
			access, exp, err := c.Issuer.IssueAccessForTenant(tenantSlug, effIss, clientID, clientID, std, custom)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir access", 2236)
				return
			}

			// No refresh token for client_credentials
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"token_type":   "Bearer",
				"expires_in":   int64(time.Until(exp).Seconds()),
				"access_token": access,
				"scope":        scopeOut,
			})

		default:
			httpx.WriteError(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type no soportado", 2202)
		}
	}
}
