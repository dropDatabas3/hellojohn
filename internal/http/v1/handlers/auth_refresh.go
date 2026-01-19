/*
auth_refresh.go â€” Refresh + Logout (rotaciÃ³n de refresh tokens) + refresh â€œadmin statelessâ€ por JWT

Este archivo en realidad contiene DOS handlers:
  1) NewAuthRefreshHandler  -> POST /v1/auth/refresh (renueva access + rota refresh)
  2) NewAuthLogoutHandler   -> POST /v1/auth/logout  (revoca un refresh puntual; idempotente)

AdemÃ¡s, soporta un caso especial:
  - â€œRefresh token JWTâ€ (stateless) para admins FS/globales: si el refresh parece JWT, lo valida y re-emite access+refresh JWT.

================================================================================
1) POST /v1/auth/refresh â€” NewAuthRefreshHandler
================================================================================

QuÃ© hace (objetivo funcional)
-----------------------------
- Recibe (client_id, refresh_token) y un â€œtenant contextâ€ (tenant_id opcional o derivado del request).
- Valida el refresh token contra el storage (DB per-tenant) usando hash SHA-256 (hex).
- Si estÃ¡ OK:
    - Emite un NUEVO access token (JWT EdDSA)
    - Emite un NUEVO refresh token (rotaciÃ³n)
    - Revoca el refresh viejo
- En paralelo, arma claims:
    - tid, amr=["refresh"], scp=scopes del client
    - custom SYS claims (roles/perms/is_admin/etc) si el repo lo soporta
    - issuer efectivo segÃºn tenant (IssuerMode) y firma con key global o per-tenant (si IssuerModePath)

Entrada / salida
----------------
Request JSON:
  {
    "tenant_id": "<opcional>",   // ACEPTADO pero (en teorÃ­a) no deberÃ­a ser fuente de verdad.
    "client_id": "<obligatorio>",
    "refresh_token": "<obligatorio>"
  }

Response JSON 200:
  {
    "access_token": "...",
    "token_type": "Bearer",
    "expires_in": <segundos>,
    "refresh_token": "..."
  }

Errores tÃ­picos:
- 400 missing_fields si falta client_id o refresh_token
- 401 invalid_grant si el refresh no existe / revocado / expirado
- 401 invalid_client si el client_id no matchea con el del refresh
- 500 si no hay TenantSQLManager / no se pueden obtener keys / etc
- â€œtenant db missing/errorâ€ con helpers/httpx wrappers

Flujo paso a paso (detallado)
-----------------------------
A) Validaciones HTTP bÃ¡sicas
   - Solo POST.
   - Content-Type set a JSON.
   - ReadJSON en RefreshRequest.
   - Trim: refresh_token y client_id.
   - Requiere ambos.

B) Resolver tenant (para ubicar el repo inicial)
   - tenantSlug se obtiene en orden:
       1) body.tenant_id
       2) helpers.ResolveTenantSlug(r)
   - Si sigue vacÃ­o => 400.
   - Llama a helpers.ResolveTenantSlugAndID(ctx, tenantSlug) pero ignora el resultado (solo â€œwarmupâ€).
     âš ï¸ Nota: ese resultado no se usa, y ademÃ¡s el comentario dice â€œsource of truth remains RTâ€.

C) Caso especial: refresh token con formato JWT (admin stateless)
   - HeurÃ­stica: strings.Count(token, ".") == 2
   - jwt.Parse con keyfunc custom:
       - extrae kid del header
       - busca public key por KID en c.Issuer.Keys.PublicKeyByKID
   - Verifica claim token_use == "refresh"
   - Si es vÃ¡lido:
       - emite nuevo ACCESS JWT (aud="admin", tid="global", amr=["pwd","refresh"], scopes hardcode)
       - emite nuevo REFRESH JWT (token_use="refresh", aud="admin") con TTL refreshTTL
       - responde 200 con ambos
   - Si falla parse/valid => log y cae al flujo â€œDB refreshâ€.
   ðŸ“Œ PatrÃ³n: â€œDual-mode token strategyâ€ (stateless vs stateful). EstÃ¡ mezclado en el handler.

D) Flujo principal (refresh stateful via DB) â€” â€œRT como fuente de verdadâ€
   1) Hashear refresh_token:
        sha256(refresh_token) -> hex string
      Comentario: â€œalineado con store PGâ€.

   2) Abrir repo per-tenant (por tenantSlug resuelto â€œdel contextoâ€):
        repo := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug)
      Maneja:
        - tenant invÃ¡lido -> 401 invalid_client
        - sin DB -> httpx.WriteTenantDBMissing
        - otros -> httpx.WriteTenantDBError

   3) Buscar refresh token por hash:
        rt := repo.GetRefreshTokenByHash(ctx, hashHex)
      Si no existe => 401 invalid_grant (â€œrefresh invÃ¡lidoâ€).

   4) Validar estado del refresh:
        - si rt.RevokedAt != nil o now >= rt.ExpiresAt => 401 invalid_grant

   5) Validar client:
        - clientID := rt.ClientIDText
        - si request.ClientID no coincide => 401 invalid_client

   6) â€œRT define el tenantâ€ (re-abrir repo si corresponde)
      - Si rt.TenantID (texto/uuid) no coincide con tenantSlug actual:
          slug2 := helpers.ResolveTenantSlugAndID(ctx, rt.TenantID)
          repo2 := OpenTenantRepo(ctx, slug2)
        y se pasa a usar repo2.
      âš ï¸ Esto mezcla â€œslugâ€ vs â€œuuidâ€ de forma peligrosa:
         rt.TenantID se comenta como UUID, pero tenantSlug es slug. Compararlos con EqualFold puede fallar siempre.
         Igual la idea es correcta: â€œsi el token pertenece a otro tenant, usar ese tenantâ€.

   7) Rechazar refresh si el usuario estÃ¡ deshabilitado
      - repo.GetUserByID(rt.UserID)
      - si DisabledAt != nil => 401 user_disabled
      âš ï¸ Solo mira DisabledAt, no DisabledUntil (en login sÃ­ se mira DisabledUntil).

   8) Scopes
      - Intenta obtener scopes desde FS:
          helpers.ResolveClientFSBySlug(ctx, rt.TenantID, clientID)  (ojo: le pasa rt.TenantID)
        si falla => scopes=["openid"]
      - std claims: tid=rt.TenantID, amr=["refresh"], scp="..."

   9) Hook de claims + SYS namespace
      - applyAccessClaimsHook(...) modifica std/custom (hook tipo â€œpolicy engineâ€)
      - Luego calcula issuer efectivo:
          effIss = jwtx.ResolveIssuer(base, issuerMode, slug, override)
      - Agrega system claims (roles/perms) si el repo implementa RBAC:
          GetUserRoles/GetUserPermissions
        y luego:
          helpers.PutSystemClaimsV2(custom, effIss, u.Metadata, roles, perms)

   10) SelecciÃ³n de key para firmar (global vs per-tenant)
      - Si issuerMode del tenant == Path => usa ActiveForTenant(slugForKeys)
      - Si no => Active() global
      - Emite access token JWT (aud=clientID, sub=userID, iss=effIss)

   11) RotaciÃ³n de refresh token (nuevo refresh + revocar viejo)
      - Camino â€œnuevoâ€ preferido:
          CreateRefreshTokenTC(ctx, tenantID, clientID, userID, ttl) (devuelve raw token)
      - Si no existe:
          - genera raw opaque token
          - hashea a hex
          - intenta un mÃ©todo TC alternativo con firma rara:
              CreateRefreshTokenTC(ctx, tenantID, clientID, newHash, expiresAt, &oldID)
            âš ï¸ Esto parece OTRA interfaz con mismo nombre pero distinta firma: peligro de confusiÃ³n.
          - si no, usa legacy repo.CreateRefreshToken(...)
      - Finalmente revoca el viejo:
          repo.RevokeRefreshToken(ctx, rt.ID) (si falla, log y sigue)

   12) Respuesta
      - Cache-Control no-store + Pragma no-cache
      - 200 con access_token + refresh_token nuevo

QuÃ© NO se usa / cosas raras (marcadas, sin decidir todavÃ­a)
-----------------------------------------------------------
- RefreshRequest.TenantID: comentario dice â€œaceptado por contrato; no usado para lÃ³gicaâ€.
  En realidad SÃ se usa como primer intento para tenantSlug. Lo que â€œno se usaâ€ es como fuente de verdad final:
  el refresh token encontrado define el tenant real.
- Se llama helpers.ResolveTenantSlugAndID(ctx, tenantSlug) y se descarta => es â€œdead-ishâ€ (side effects?).
- ComparaciÃ³n rt.TenantID vs tenantSlug es dudosa (UUID vs slug). Riesgo de reabrir repo mal.
- Doble sistema de refresh â€œTCâ€ con interfaces distintas y mismo nombre => deuda tÃ©cnica fuerte.
- Inconsistencia de bloqueo de usuario (solo DisabledAt, no DisabledUntil).
- Mezcla de â€œadmin refresh JWTâ€ y â€œuser refresh DBâ€ en el mismo handler => alto acoplamiento.

Patrones / refactor propuesto (con ganas, para V2)
--------------------------------------------------
A) Separar responsabilidades (Single Responsibility + GoF Strategy)
   - Strategy: RefreshModeStrategy
       1) JWTStatelessRefreshStrategy (admin/global)
       2) DBRefreshStrategy (tenant/user)
     El handler solo decide cuÃ¡l aplica y delega.

B) Service Layer (Application Service / Use Case)
   - RefreshService.Refresh(ctx, RefreshCommand) -> RefreshResult
   - LogoutService.Logout(ctx, LogoutCommand) -> error
   Esto te permite testear sin HTTP y reutilizar lÃ³gica desde otros flows (ej: device sessions).

C) Repository Port + Adapter
   - Definir una interfaz clara:
       RefreshTokenRepo {
         FindByHash(ctx, tenantSlug, hash) (*RefreshToken, error)
         Rotate(ctx, tokenID, tenantID, clientID, userID, ttl) (newRaw string, error)
         Revoke(ctx, tokenID) error
       }
     Luego adapters:
       - PostgresTenantRepoAdapter
       - (Opcional) LegacyAdapter
     EvitÃ¡s los type assertions repetidos y las firmas â€œTCâ€ duplicadas.

D) Factory para â€œissuer + signing keyâ€ (Factory Method / Abstract Factory)
   - IssuerResolver.Resolve(tenantSlug) -> effIss, mode
   - KeySelector.Select(mode, tenantSlug) -> (kid, priv)
   SacÃ¡s el if/else repetido.

E) Template Method para construir claims
   - buildBaseClaims(...)
   - enrichWithHook(...)
   - enrichWithRBACIfSupported(...)
   AsÃ­ el flujo refresh/login comparten construcciÃ³n de claims.

F) Seguridad / consistencia
   - Normalizar â€œtenant identityâ€:
       TenantRef {Slug, UUID}
     Y dejar UNA sola comparaciÃ³n (no slug vs uuid).
   - Asegurar que scope lookup usa slug correcto (no rt.TenantID si es UUID).
   - Hacer revocaciÃ³n/rotaciÃ³n transaccional si el store lo banca (ideal):
       rotate => create new + revoke old en una transacciÃ³n.

G) Concurrencia (si aplica, sin inventar)
   - AcÃ¡ no hace falta worker pool: es request/response puro.
   - Lo Ãºnico concurrente Ãºtil serÃ­a:
       - paralelizar (con errgroup) lookup de tenant config + rbac roles/perms,
         pero solo si esos accesos son independientes y no agregan carga innecesaria.
     Ojo: primero claridad, despuÃ©s micro-optimizaciÃ³n.

================================================================================
2) POST /v1/auth/logout â€” NewAuthLogoutHandler
================================================================================

QuÃ© hace
--------
- Recibe refresh_token + client_id (+ tenant context)
- Busca el refresh por hash en el repo del tenant â€œcontextualâ€
- Si no existe: devuelve 204 (idempotente, no filtra existencia)
- Si existe:
    - valida que client_id matchee
    - si el token pertenece a otro tenant, reabre repo para ese tenant
    - intenta revocar por hash con mÃ©todo TC (si existe)
    - devuelve 204

Notas importantes
-----------------
- Logout es idempotente: si el refresh no existe, igual 204.
- AcÃ¡ NO se usa repo.RevokeRefreshToken(tokenID) de forma directa; usa un mÃ©todo TC opcional
  (Revoker por hash) y si no existe, no hace nada mÃ¡s (igual responde 204).
  Eso puede dejar tokens sin revocar si el store no implementa revoker TC.

Patrones/refactor para logout
-----------------------------
- Compartir el mismo â€œRefreshTokenResolverâ€ del refresh:
    resolveByRawToken -> (repo, rt)
- Command + Service:
    LogoutService.RevokeRefresh(ctx, tenantRef, clientID, rawRefresh) -> error
- Strategy:
    - Revocar por ID (si ya encontraste rt.ID)
    - Revocar por hash (si el store lo prefiere)
  ElegÃ­s la estrategia por capacidades del repo.

Resumen corto
-------------
- auth_refresh.go mezcla: refresh stateless (admin JWT) + refresh stateful (DB) + logout puntual.
- La idea central es correcta (RT como fuente de verdad, rotaciÃ³n + revocaciÃ³n),
  pero estÃ¡ todo muy pegado con type assertions, comparaciones slug/uuid confusas y dos APIs â€œTCâ€ distintas.
- En V2 lo mÃ¡s rentable es separar en servicios + strategies + factories para issuer/keys,
  y estandarizar TenantRef para no volver a sufrir el quilombo slug/uuid.
*/

package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/controlplane/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/http/v1/helpers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/v1/core"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type RefreshRequest struct {
	TenantID     string `json:"tenant_id,omitempty"` // aceptado por contrato; no usado para lÃ³gica
	ClientID     string `json:"client_id,omitempty"`
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func NewAuthRefreshHandler(c *app.Container, refreshTTL time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req RefreshRequest
		if !httpx.ReadJSON(w, r, &req) {
			return
		}
		req.RefreshToken = strings.TrimSpace(req.RefreshToken)
		req.ClientID = strings.TrimSpace(req.ClientID)
		if req.RefreshToken == "" || req.ClientID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "client_id y refresh_token son obligatorios", 1002)
			return
		}

		ctx := r.Context()

		// 0) Resolver tenant slug en este orden: body.tenant_id -> cpctx.ResolveTenant(r) -> helpers.ResolveTenantSlug(r)
		tenantSlug := strings.TrimSpace(req.TenantID)
		if tenantSlug == "" {
			tenantSlug = helpers.ResolveTenantSlug(r)
		}
		if strings.TrimSpace(tenantSlug) == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id o contexto de tenant requerido", 1002)
			return
		}
		// Resolve UUID too if needed later; source of truth remains RT
		_, _ = helpers.ResolveTenantSlugAndID(ctx, tenantSlug)

		// Check if it's a JWT refresh token (stateless admin)
		if strings.Count(req.RefreshToken, ".") == 2 {
			// Parse and verify JWT
			token, err := jwtv5.Parse(req.RefreshToken, func(token *jwtv5.Token) (interface{}, error) {
				// Extract kid from header
				kid, ok := token.Header["kid"].(string)
				if !ok {
					return nil, jwtv5.ErrTokenUnverifiable
				}
				// Use specific key by KID
				return c.Issuer.Keys.PublicKeyByKID(kid)
			})

			if err == nil && token.Valid {
				if claims, ok := token.Claims.(jwtv5.MapClaims); ok {
					if use, ok := claims["token_use"].(string); ok && use == "refresh" {
						// It's a valid admin refresh token
						userID, _ := claims.GetSubject()

						// Issue new access token
						now := time.Now().UTC()
						exp := now.Add(c.Issuer.AccessTTL)
						kid, priv, _, kerr := c.Issuer.Keys.Active()
						if kerr != nil {
							httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma", 1405)
							return
						}

						// Admin claims
						amrSlice := []string{"pwd", "refresh"}
						grantedScopes := []string{"openid", "profile", "email"}
						std := map[string]any{
							"tid": "global",
							"amr": amrSlice,
							"acr": "urn:hellojohn:loa:1",
							"scp": strings.Join(grantedScopes, " "),
						}
						// Minimal system claims for admin
						custom := helpers.PutSystemClaimsV2(map[string]any{}, c.Issuer.Iss, map[string]any{"is_admin": true}, []string{"sys:admin"}, nil)

						atClaims := jwtv5.MapClaims{
							"iss": c.Issuer.Iss,
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
						atString, err := atToken.SignedString(priv)
						if err != nil {
							httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1405)
							return
						}

						// Issue new refresh token (rotation)
						rtClaims := jwtv5.MapClaims{
							"iss":       c.Issuer.Iss,
							"sub":       userID,
							"aud":       "admin",
							"iat":       now.Unix(),
							"nbf":       now.Unix(),
							"exp":       now.Add(refreshTTL).Unix(),
							"token_use": "refresh",
						}
						rtToken := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, rtClaims)
						rtToken.Header["kid"] = kid
						rtToken.Header["typ"] = "JWT"
						rtString, err := rtToken.SignedString(priv)
						if err != nil {
							httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el refresh token", 1204)
							return
						}

						w.Header().Set("Cache-Control", "no-store")
						w.Header().Set("Pragma", "no-cache")
						httpx.WriteJSON(w, http.StatusOK, RefreshResponse{
							AccessToken:  atString,
							TokenType:    "Bearer",
							ExpiresIn:    int64(time.Until(exp).Seconds()),
							RefreshToken: rtString,
						})
						return
					}
				}
			}
			// If JWT parsing failed or invalid, fall through to DB check (could be a coincidence or invalid token)
			// But since it looked like a JWT, we might want to log it.
			log.Printf("refresh: invalid JWT refresh token: %v", err)
		}

		// 1) Cargar RT como fuente de verdad (por hash). No usar c.Store en runtime.
		var (
			rt  *core.RefreshToken
			err error
		)
		// hash en HEX (alineado con store PG)
		sum := sha256.Sum256([]byte(req.RefreshToken))
		hashHex := hex.EncodeToString(sum[:])

		// Intentar en el repo del tenant resuelto
		if c.TenantSQLManager == nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "tenant manager not initialized", 1003)
			return
		}
		repo, err := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug)
		if err != nil {
			if helpers.IsTenantNotFound(err) {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "tenant invÃ¡lido", 2100)
				return
			}
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		}
		if rtx, e2 := repo.GetRefreshTokenByHash(ctx, hashHex); e2 == nil {
			rt = rtx
		}
		if rt == nil {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_grant", "refresh invÃ¡lido", 1401)
			return
		}

		now := time.Now()
		if rt.RevokedAt != nil || !now.Before(rt.ExpiresAt) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_grant", "refresh revocado o expirado", 1402)
			return
		}

		// 2) ClientID desde RT si no vino en request; si vino y no coincide, invalid_client
		clientID := rt.ClientIDText
		if req.ClientID != "" && !strings.EqualFold(req.ClientID, clientID) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "cliente invÃ¡lido", 1403)
			return
		}

		// 3) Si el RT pertenece a otro tenant, reabrir repo por rt.TenantID (RT define el tenant)
		if !strings.EqualFold(rt.TenantID, tenantSlug) {
			// rt.TenantID is UUID; open repo by its slug equivalent
			slug2, _ := helpers.ResolveTenantSlugAndID(ctx, rt.TenantID)
			repo2, e2 := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, slug2)
			if e2 != nil {
				if helpers.IsNoDBForTenant(e2) {
					httpx.WriteTenantDBMissing(w)
					return
				}
				httpx.WriteTenantDBError(w, e2.Error())
				return
			}
			repo = repo2
		}

		// Rechazar refresh si el usuario estÃ¡ deshabilitado
		if u, err := repo.GetUserByID(ctx, rt.UserID); err == nil && u != nil {
			if u.DisabledAt != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "user_disabled", "usuario deshabilitado", 1410)
				return
			}
		}

		// Scopes: intentar desde FS (hint). Si no, fallback a openid.
		var grantedScopes []string
		if fsc2, err2 := helpers.ResolveClientFSBySlug(ctx, rt.TenantID, clientID); err2 == nil {
			grantedScopes = append([]string{}, fsc2.Scopes...)
		} else {
			grantedScopes = []string{"openid"}
		}
		std := map[string]any{
			"tid": rt.TenantID,
			"amr": []string{"refresh"},
			"scp": strings.Join(grantedScopes, " "),
		}
		// Hook + SYS namespace
		custom := map[string]any{}
		std, custom = applyAccessClaimsHook(r.Context(), c, rt.TenantID, clientID, rt.UserID, grantedScopes, []string{"refresh"}, std, custom)
		// Resolver issuer efectivo del tenant (para SYS y firma)
		effIss := c.Issuer.Iss
		if cpctx.Provider != nil {
			// Resolve slug from RT tenant UUID to compute issuer
			slug2, _ := helpers.ResolveTenantSlugAndID(ctx, rt.TenantID)
			if ten, errTen := cpctx.Provider.GetTenantBySlug(ctx, slug2); errTen == nil && ten != nil {
				effIss = jwtx.ResolveIssuer(c.Issuer.Iss, string(ten.Settings.IssuerMode), ten.Slug, ten.Settings.IssuerOverride)
			}
		}
		// derivar is_admin + RBAC (Fase 2)
		if u, err := repo.GetUserByID(r.Context(), rt.UserID); err == nil && u != nil {
			type rbacReader interface {
				GetUserRoles(ctx context.Context, userID string) ([]string, error)
				GetUserPermissions(ctx context.Context, userID string) ([]string, error)
			}
			var roles, perms []string
			if rr, ok := any(repo).(rbacReader); ok {
				roles, _ = rr.GetUserRoles(r.Context(), rt.UserID)
				perms, _ = rr.GetUserPermissions(r.Context(), rt.UserID)
			}
			custom = helpers.PutSystemClaimsV2(custom, effIss, u.Metadata, roles, perms)
		}

		// Per-tenant signing key (tenant comes from RT)
		now2 := time.Now().UTC()
		exp := now2.Add(c.Issuer.AccessTTL)
		// Keys: por-tenant sÃ³lo si el issuer del tenant estÃ¡ en modo Path; caso contrario, global
		slugForKeys, _ := helpers.ResolveTenantSlugAndID(ctx, rt.TenantID)
		var (
			kid  string
			priv any
			kerr error
		)
		if cpctx.Provider != nil {
			if ten, errTen := cpctx.Provider.GetTenantBySlug(ctx, slugForKeys); errTen == nil && ten != nil && ten.Settings.IssuerMode == controlplane.IssuerModePath {
				kid, priv, _, kerr = c.Issuer.Keys.ActiveForTenant(slugForKeys)
			} else {
				kid, priv, _, kerr = c.Issuer.Keys.Active()
			}
		} else {
			kid, priv, _, kerr = c.Issuer.Keys.Active()
		}
		if kerr != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma", 1405)
			return
		}
		claims := jwtv5.MapClaims{
			"iss": effIss,
			"sub": rt.UserID,
			"aud": clientID,
			"iat": now2.Unix(),
			"nbf": now2.Unix(),
			"exp": exp.Unix(),
		}
		for k, v := range std {
			claims[k] = v
		}
		if custom != nil {
			claims["custom"] = custom
		}
		tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
		tk.Header["kid"] = kid
		tk.Header["typ"] = "JWT"
		token, err := tk.SignedString(priv)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1405)
			return
		}

		// Crear nuevo refresh token usando mÃ©todo TC si estÃ¡ disponible
		var rawRT string
		tcCreateStore, tcOk := any(repo).(interface {
			CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error)
		})
		if tcOk {
			// Usar mÃ©todo TC para crear el nuevo sobre el mismo tenant del RT (UUID)
			rawRT, err = tcCreateStore.CreateRefreshTokenTC(ctx, rt.TenantID, clientID, rt.UserID, refreshTTL)
			if err != nil {
				log.Printf("refresh: create new rt TC err: %v", err)
				httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir nuevo refresh", 1407)
				return
			}
		} else {
			// Fallback al mÃ©todo viejo
			rawRT, err = tokens.GenerateOpaqueToken(32)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 1406)
				return
			}
			newHash := tokens.SHA256Hex(rawRT)
			expiresAt := now.Add(refreshTTL)

			// Usar CreateRefreshTokenTC para rotaciÃ³n
			if tcStore, ok := any(repo).(interface {
				CreateRefreshTokenTC(context.Context, string, string, string, time.Time, *string) (string, error)
			}); ok {
				if _, err := tcStore.CreateRefreshTokenTC(ctx, rt.TenantID, clientID, newHash, expiresAt, &rt.ID); err != nil {
					log.Printf("refresh: create new rt TC err: %v", err)
					httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir nuevo refresh", 1407)
					return
				}
			} else {
				// Fallback legacy
				if _, err := repo.CreateRefreshToken(ctx, rt.UserID, clientID, newHash, expiresAt, &rt.ID); err != nil {
					log.Printf("refresh: create new rt err: %v", err)
					httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir nuevo refresh", 1407)
					return
				}
			}
		}

		// Revocar el token viejo
		if err := repo.RevokeRefreshToken(ctx, rt.ID); err != nil {
			log.Printf("refresh: revoke old rt err: %v", err)
		}

		// evitar cache en respuestas que incluyen tokens
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")

		httpx.WriteJSON(w, http.StatusOK, RefreshResponse{
			AccessToken:  token,
			TokenType:    "Bearer",
			ExpiresIn:    int64(time.Until(exp).Seconds()),
			RefreshToken: rawRT,
		})
	}
}

// ------- LOGOUT -------

type LogoutRequest struct {
	TenantID     string `json:"tenant_id,omitempty"`
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
}

func NewAuthLogoutHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		var req LogoutRequest
		if !httpx.ReadJSON(w, r, &req) {
			return
		}
		req.RefreshToken = strings.TrimSpace(req.RefreshToken)
		req.ClientID = strings.TrimSpace(req.ClientID)
		req.TenantID = strings.TrimSpace(req.TenantID)
		if req.RefreshToken == "" || req.ClientID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "client_id y refresh_token son obligatorios", 1002)
			return
		}

		ctx := r.Context()
		// Usar el mismo hashing que los mÃ©todos TC (hex en lugar de base64)
		sum := sha256.Sum256([]byte(req.RefreshToken))
		hash := hex.EncodeToString(sum[:])

		// Resolver tenant: body -> contexto -> fallback
		tenantSlug := req.TenantID
		if tenantSlug == "" {
			tenantSlug = helpers.ResolveTenantSlug(r)
		}
		if tenantSlug == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id o contexto de tenant requerido", 1002)
			return
		}

		// Resolver repo y buscar RT para validar client y potencial cruce de tenant
		if c.TenantSQLManager == nil {
			httpx.WriteTenantDBError(w, "tenant manager not initialized")
			return
		}
		repo, err := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug)
		if err != nil {
			if helpers.IsTenantNotFound(err) {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "tenant invÃ¡lido", 2100)
				return
			}
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		}

		// Intentar obtener RT por hash
		rt, _ := repo.GetRefreshTokenByHash(ctx, hash)
		if rt == nil {
			// Idempotente: no filtrar existencia
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Validar client id coincida
		if !strings.EqualFold(req.ClientID, rt.ClientIDText) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client_id no coincide", 2101)
			return
		}
		// Reabrir repo si el RT pertenece a otro tenant
		if !strings.EqualFold(rt.TenantID, tenantSlug) {
			repo2, e2 := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, rt.TenantID)
			if e2 != nil {
				if helpers.IsNoDBForTenant(e2) {
					httpx.WriteTenantDBMissing(w)
					return
				}
				httpx.WriteTenantDBError(w, e2.Error())
				return
			}
			repo = repo2
		}
		type revoker interface {
			RevokeRefreshByHashTC(ctx context.Context, tenantID, clientIDText, tokenHash string) (int64, error)
		}
		if rv, ok := any(repo).(revoker); ok {
			_, _ = rv.RevokeRefreshByHashTC(ctx, rt.TenantID, rt.ClientIDText, hash)
		}

		w.WriteHeader(http.StatusNoContent) // 204
	}
}
