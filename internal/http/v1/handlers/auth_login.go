/*
auth_login.go — Login “password” + emisión de tokens (access + refresh) + gating por client + rate limit + MFA + RBAC (opcional)

Qué hace este handler
---------------------
Implementa (en la práctica):
  POST /v1/auth/login

Recibe credenciales (email + password) y, si valida:
- Verifica que el client exista y permita provider "password"
- Busca usuario por email en el user-store del tenant
- Chequea password hash
- Bloquea si el usuario está deshabilitado
- Opcional: exige email verificado si el client lo pide (FS controlplane)
- Opcional: bifurca a MFA (TOTP) si está habilitada y el device no es trusted
- Emite Access Token (JWT EdDSA) con issuer efectivo por tenant
- Persiste/crea Refresh Token via CreateRefreshTokenTC (store method)
- Devuelve JSON con access_token + refresh_token + expiración

Además:
- Soporta login “FS Admin” (sin tenant/client) si FSAdminEnabled()
- Soporta body JSON o form-urlencoded
- Aplica rate limiting específico de login (si MultiLimiter configurado)

Entrada / salida
----------------
Request (JSON o form):
  {
    tenant_id, client_id, email, password
  }
Notas:
- tenant_id y client_id son opcionales SOLO para el modo “FS admin”
- email y password siempre obligatorios

Response (OK):
  { access_token, token_type="Bearer", expires_in, refresh_token }

Errores relevantes:
- 400 missing_fields (si falta email/pass o tenant/client cuando no hay FS admin)
- 401 invalid_credentials (usuario o password)
- 401 invalid_client (tenant/client inválidos o password no permitido)
- 423 user_disabled
- 403 email_not_verified (si client exige verificación)
- 429 rate limit (lo escribe EnforceLoginLimit)
- 5xx varios (tenant manager, store, emisión tokens, etc.)

Flujo paso a paso (normal, NO FS-admin)
---------------------------------------
1) Validación HTTP + parse body
   - Solo POST.
   - Content-Type:
       a) application/json:
          - lee body (MaxBytes 1MB)
          - json.Unmarshal a AuthLoginRequest (snake_case)
          - fallback extra: intenta PascalCase (TenantID/ClientID/Email/Password)
             * Esto existe por compat de tests/clients viejos.
       b) application/x-www-form-urlencoded:
          - ParseForm() y lee tenant_id/client_id/email/password
       c) otro CT => 400

   Normaliza email => trim + strings.ToLower

2) Validación mínima
   - email y password obligatorios
   - si falta tenant_id o client_id:
       - si helpers.FSAdminEnabled(): intenta FSAdminVerify(email, password)
           - si OK => emite token “admin” (JWT access + refresh JWT stateless)
           - si FAIL => 401 invalid_credentials
       - si FSAdmin NO habilitado => 400 (tenant_id y client_id obligatorios)

3) Resolve tenant slug + tenant UUID
   - helpers.ResolveTenantSlugAndID(ctx, req.TenantID)
   - Devuelve tenantSlug y tenantUUID (string con UUID? O slug? según helper)

4) Resolver client (prefer FS) ANTES de abrir DB
   - helpers.ResolveClientFSBySlug(ctx, tenantSlug, req.ClientID)
   - si existe:
       - guarda scopes/providers desde FSClient
       - haveFSClient = true
   - si no:
       - se deja para fallback DB más adelante (cuando el repo ya esté abierto)

   Objetivo: “si client es inválido, no abras DB al pedo” (aunque hoy igual la abre antes del fallback DB; el comentario dice una cosa y el código hace otra en algunos caminos).

5) Abrir repo del tenant (TenantSQLManager)
   - helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug)
   - Si error:
       - si tenant inválido => 401 invalid_client (tenant inválido)
       - si FSAdminEnabled() => fallback FS admin (pero ahora con aud=req.ClientID)
           * Provider gating: si el client tenía providers y no incluye "password" => bloquea
           * Emite access token (sin refresh)
       - si no DB configurada => httpx.WriteTenantDBMissing
       - otros => httpx.WriteTenantDBError
   - Si OK: guarda repoCore y lo mete en context (helpers.WithTenantRepo)

6) Rate limiting (si c.MultiLimiter != nil)
   - Lee cfg.Rate.Login.Window (parse duration)
   - EnforceLoginLimit(w, r, limiter, loginCfg, req.TenantID, req.Email)
   - Si rate limited => ya respondió y corta

7) Si no había FS client, lookup en DB ahora que repo está abierto
   - interface clientGetter { GetClientByClientID(...) }
   - si el repo lo implementa, trae scopes/providers
   - si no existe client => 401 invalid_client
   - si repo no implementa => 401 invalid_client

8) Provider gating (clientProviders)
   - si providers no vacío => debe contener "password"
   - si no => 401 invalid_client (“password login deshabilitado para este client”)

9) Buscar usuario + identidad por email
   - repo.GetUserByEmail(ctx, tenantUUID, email)
   - si ErrNotFound => 401 invalid_credentials
   - otros => 500 invalid_credentials (hoy usa InternalServerError para err != NotFound)

10) Bloqueo de usuario
   - Si DisabledUntil != nil y now < DisabledUntil => locked
   - Si DisabledAt != nil => locked
   - Responde 423 Locked (bien)

11) Verificar password
   - Si identity/passwordHash nil o vacío => 401
   - repo.CheckPassword(hash, req.Password) => 401 si no coincide

12) Email verification (opcional por client FS)
   - cpctx.Provider.GetClient(ctx, tenantSlug, req.ClientID)
   - si RequireEmailVerification && !u.EmailVerified => 403 email_not_verified

13) MFA pre-issue (opcional)
   - Si repo implementa:
       - GetMFATOTP(userID) -> si ConfirmedAt != nil => MFA configurada
       - Si cookie "mfa_trust" existe y repo implementa IsTrustedDevice:
           - calcula hash tokens.SHA256Base64URL(cookie.Value)
           - si trusted => trustedByCookie=true
       - Si NO trusted => bifurca:
           - crea mfaChallenge (struct no visible acá)
           - genera opaque token mid
           - guarda en c.Cache (TTL 5m) bajo key mfa:token:<mid>
           - responde 200 con {mfa_required:true, mfa_token:mid, amr:["pwd"]}

14) Claims base + hooks + RBAC opcional
   - amr/acr:
       - default amr=["pwd"], acr=loa:1
       - si trustedByCookie => amr=["pwd","mfa"], acr=loa:2
   - scopes: grantedScopes = clientScopes
   - std claims: tid, amr, acr, scp
   - applyAccessClaimsHook(...) (hook opcional, puede mutar std/custom)
   - RBAC opcional:
       - si repo implementa GetUserRoles/GetUserPermissions => los agrega al “custom system claims”

15) Resolver issuer efectivo por tenant (y key para firmar)
   - effIss := c.Issuer.Iss
   - cpctx.Provider.GetTenantBySlug(ctx, tenantSlug)
   - effIss = jwtx.ResolveIssuer(globalIss, issuerMode, slug, override)
   - PutSystemClaimsV2(custom, effIss, u.Metadata, roles, perms)
   - Selección de key:
       - si IssuerModePath => c.Issuer.Keys.ActiveForTenant(tenantSlug)
       - else => c.Issuer.Keys.Active()

16) Emitir access token (JWT EdDSA)
   - claims: iss, sub, aud=client_id, iat/nbf/exp + std + custom
   - SignedString(priv)

17) Crear refresh token persistente (TC)
   - repo debe implementar CreateRefreshTokenTC(ctx, tenantID, clientID, userID, ttl)
   - si no => 500 store_not_supported
   - si error => 500 persist_failed

18) Responder
   - Cache-Control no-store, Pragma no-cache
   - JSON: AuthLoginResponse (access + refresh + expires)

Cosas que están “raras” o para marcar (sin decidir aún)
-------------------------------------------------------
1) Debug logs a lo pavote
   - log.Printf("DEBUG: ...") por todos lados.
   - Esto en prod te llena logs y te puede afectar performance.
   - No está usando logger estructurado, ni levels reales.

2) Import potencialmente no usado
   - En ESTE archivo, revisá: `github.com/dropDatabas3/hellojohn/internal/controlplane`
     Solo se usa para comparar `ten.Settings.IssuerMode == controlplane.IssuerModePath`.
     O sea: sí se usa.
   - `tokens` y `jwtx` y `util` también se usan.
   - `cpctx` se usa.
   (Igual, el compilador te lo canta si alguno sobra.)

3) Doble modo “FS admin”
   - Hay dos caminos:
       a) cuando faltan tenant/client (stateless refresh JWT para admin)
       b) cuando falla abrir repo tenant y FSAdminEnabled() (sin refresh)
   - Inconsistente: en un caso emite refresh JWT, en otro no.
   - Aud distinto: "admin" vs req.ClientID.
   - Tid fijo "global" (ok) pero mezcla claims/flows.

4) Mezcla de fuentes para client gating
   - Primero intenta FS, después DB.
   - Después vuelve a consultar cpctx.Provider.GetClient() solo para email verification.
   - Es decir: el “client config” se obtiene 2-3 veces por distintos caminos.

5) Cache usage para MFA
   - c.Cache.Set(...) asume que Cache existe y está inicializada.
   - No hay nil-check (si c.Cache puede ser nil, esto explota).
   - El tipo `mfaChallenge` no está definido acá: dependencia implícita del paquete.

6) Comentarios vs comportamiento
   - “Primero resolver client desde FS… y no abrir DB”,
     pero después abre repo igual, y si no había FS client recién ahí intenta DB.
     El objetivo se cumple parcialmente, pero no siempre.

7) Token issuance duplicado
   - La lógica de construir JWT (claims + headers + SignedString) está repetida
     en varios handlers (y dentro de este mismo para FS admin vs normal).
   - Refactor claro hacia un “TokenIssuer” (Builder / Factory) cuando hagamos visión global.

Patrones que encajan para la futura refactor (sin implementarla todavía)
-----------------------------------------------------------------------
- GoF: Facade / Service Layer
  AuthService.LoginPassword(...) que devuelva un “resultado” (tokens o mfa_required).

- GoF: Strategy + Chain of Responsibility
  ClientResolver (FS -> DB), TenantResolver (ID/slug), AdminAuthStrategy.

- GoF: Builder
  Para armar JWT claims + headers de forma consistente (evitar duplicación).

- GoF: Template Method
  “emitAccessToken()” con pasos fijos (issuer -> key -> claims -> sign),
  y variaciones por tipo de sesión (user/admin).

- Concurrencia (solo donde aporte)
  No hace falta en login; el bottleneck es DB + hashing.
  Si se agrega, sería para “resolver client+tenant config” en paralelo con cache,
  pero cuidado: no sumar latencia por goroutines al pedo.

Ideas de eficiencia/reutilización para el repaso global
-------------------------------------------------------
- Unificar parse de request (JSON/form + fallback PascalCase) en helper común.
- Unificar resolución de client/tenant (y cachearlo).
- Unificar issuer/key selection en un “IssuerResolver/KeySelector”.
- Unificar emisión de tokens en un componente reutilizable.
- Limpiar FS admin flows (definir 1 sola política coherente).
- Mover rate limiting a middleware semántico (o helper menos invasivo).
- Evitar múltiples hits a cpctx.Provider (traer client config 1 vez y reusar).

En resumen
----------
Este handler es el “centro neurálgico” del login password: parsea, rate-limitea, valida client, busca user, chequea password,
aplica políticas (email verification, user disabled), opcionalmente MFA, arma claims (scopes + RBAC + hook),
resuelve issuer/key por tenant y emite tokens (access + refresh persistido).
Está funcional, pero hoy tiene duplicación fuerte (token issuance), caminos FS admin inconsistentes,
y mucha lógica de orquestación que pide a gritos separarse en servicios/resolvers reutilizables.
*/

package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/config"
	"github.com/dropDatabas3/hellojohn/internal/controlplane"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/http/v1/helpers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	"github.com/dropDatabas3/hellojohn/internal/util"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type AuthLoginRequest struct {
	TenantID string `json:"tenant_id"`
	ClientID string `json:"client_id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthLoginResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"` // "Bearer"
	ExpiresIn    int64  `json:"expires_in"` // segundos
	RefreshToken string `json:"refresh_token"`
}

func NewAuthLoginHandler(c *app.Container, cfg *config.Config, refreshTTL time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Debug: log entry
		log.Printf("DEBUG: auth_login handler entry")

		// Usar context del request directamente por ahora
		ctx := r.Context()

		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req AuthLoginRequest
		ct := strings.ToLower(r.Header.Get("Content-Type"))
		switch {
		case strings.Contains(ct, "application/json"):
			// Leemos el body con límite (igual que ReadJSON) y soportamos claves alternativas
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			defer r.Body.Close()
			body, err := io.ReadAll(r.Body)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "json inválido", 1102)
				return
			}

			// Intento 1: snake_case estándar
			_ = json.Unmarshal(body, &req)

			// Fallback: PascalCase (compat con tests que no ponen tags)
			if req.TenantID == "" || req.ClientID == "" || req.Email == "" || req.Password == "" {
				var alt struct {
					TenantID string `json:"TenantID"`
					ClientID string `json:"ClientID"`
					Email    string `json:"Email"`
					Password string `json:"Password"`
				}
				if err := json.Unmarshal(body, &alt); err == nil {
					if req.TenantID == "" {
						req.TenantID = strings.TrimSpace(alt.TenantID)
					}
					if req.ClientID == "" {
						req.ClientID = strings.TrimSpace(alt.ClientID)
					}
					if req.Email == "" {
						req.Email = strings.TrimSpace(alt.Email)
					}
					if req.Password == "" {
						req.Password = alt.Password
					}
				}
			}

		case strings.Contains(ct, "application/x-www-form-urlencoded"):
			if err := r.ParseForm(); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_form", "form inválido", 1001)
				return
			}
			req.TenantID = strings.TrimSpace(r.FormValue("tenant_id"))
			req.ClientID = strings.TrimSpace(r.FormValue("client_id"))
			req.Email = strings.TrimSpace(strings.ToLower(r.FormValue("email")))
			req.Password = r.FormValue("password")

		default:
			httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Content-Type debe ser application/json", 1102)
			return
		}

		// normalización consistente
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		log.Printf("DEBUG: after email normalization, validating fields")

		// Require email and password. Tenant and client are optional to support
		// global FS-admins that do not belong to any tenant or client.
		if req.Email == "" || req.Password == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "email y password son obligatorios", 1002)
			return
		}

		// If tenant or client is missing, attempt FS-admin login when enabled.
		if req.TenantID == "" || req.ClientID == "" {
			if helpers.FSAdminEnabled() {
				// Try to verify FS admin directly. If valid, issue an admin token and return.
				if ufs, ok := helpers.FSAdminVerify(req.Email, req.Password); ok {
					// Provider gating: if clientProviders declared and password not allowed, block.
					// Since no client provided, we skip provider gating here for global admins.
					amrSlice := []string{"pwd"}
					grantedScopes := []string{"openid", "profile", "email"}
					std := map[string]any{
						"tid": "global",
						"amr": amrSlice,
						"acr": "urn:hellojohn:loa:1",
						"scp": strings.Join(grantedScopes, " "),
					}
					custom := helpers.PutSystemClaimsV2(map[string]any{}, c.Issuer.Iss, ufs.Metadata, []string{"sys:admin"}, nil)

					now := time.Now().UTC()
					exp := now.Add(c.Issuer.AccessTTL)
					kid, priv, _, kerr := c.Issuer.Keys.Active()
					if kerr != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma 1", 1204)
						return
					}
					claims := jwtv5.MapClaims{
						"iss": c.Issuer.Iss,
						"sub": ufs.ID,
						"aud": "admin",
						"iat": now.Unix(),
						"nbf": now.Unix(),
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
						httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1204)
						return
					}
					// Issue refresh token as JWT for stateless admin session
					rtClaims := jwtv5.MapClaims{
						"iss":       c.Issuer.Iss,
						"sub":       ufs.ID,
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
					httpx.WriteJSON(w, http.StatusOK, AuthLoginResponse{
						AccessToken:  token,
						TokenType:    "Bearer",
						ExpiresIn:    int64(time.Until(exp).Seconds()),
						RefreshToken: rtString,
					})
					return
				}
				// If FS admin verification failed, return invalid credentials.
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "usuario o password inválidos", 1202)
				return
			}

			// If FS admin not enabled, require tenant and client.
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id y client_id son obligatorios", 1002)
			return
		}

		// Debug: check container
		log.Printf("DEBUG: container=%v, tenantSQLMgr=%v", c != nil, c != nil && c.TenantSQLManager != nil)

		// Resolver slug + UUID del tenant
		tenantSlug, tenantUUID := helpers.ResolveTenantSlugAndID(ctx, req.TenantID)

		// Primero: resolver client desde FS. Si falla, devolver 401 invalid_client y no abrir DB.
		// Resolve client. Prefer FS control-plane; if unavailable, we'll try DB client catalog later only if repo opens.
		var (
			fsClient        helpers.FSClient
			clientScopes    []string
			clientProviders []string
			haveFSClient    bool
		)
		if fsc, err := helpers.ResolveClientFSBySlug(ctx, tenantSlug, req.ClientID); err == nil {
			fsClient = fsc
			clientScopes = append([]string{}, fsClient.Scopes...)
			clientProviders = append([]string{}, fsClient.Providers...)
			haveFSClient = true
		}

		// Abrir repo por tenant solo después de tener un client válido (o si no hay en FS, intentaremos fallback DB más abajo)
		// Compatibility: if per-tenant DB is not configured, fall back to global store (if available).
		var repoCore core.Repository
		if c == nil || c.TenantSQLManager == nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "tenant manager not initialized", 1004)
			return
		}
		if rc, err := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug); err != nil {
			// Phase 4: gate by tenant DB. No fallback to global store in FS-only mode.
			if helpers.IsTenantNotFound(err) {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "tenant inválido", 2100)
				return
			}
			if helpers.FSAdminEnabled() {
				// Optional FS-admin fallback: allow admin login when FS_ADMIN_ENABLE=1
				// Triggered on any tenant repo open error when explicitly enabled.
				ufs, ok := helpers.FSAdminVerify(req.Email, req.Password)
				if !ok {
					httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "usuario o password inválidos", 1202)
					return
				}
				// Provider gating remains: if FS had clientProviders and did not include password, block
				if len(clientProviders) > 0 {
					allowed := false
					for _, p := range clientProviders {
						if strings.EqualFold(p, "password") {
							allowed = true
							break
						}
					}
					if !allowed {
						httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "password login deshabilitado para este client", 1207)
						return
					}
				}
				// Issue admin token (no refresh persistence in FS mode)
				amrSlice := []string{"pwd"}
				grantedScopes := append([]string{}, clientScopes...)
				if len(grantedScopes) == 0 {
					grantedScopes = []string{"openid", "profile", "email"}
				}
				std := map[string]any{
					"tid": "global",
					"amr": amrSlice,
					"acr": "urn:hellojohn:loa:1",
					"scp": strings.Join(grantedScopes, " "),
				}
				custom := map[string]any{}
				effIss := c.Issuer.Iss
				custom = helpers.PutSystemClaimsV2(custom, effIss, ufs.Metadata, []string{"sys:admin"}, nil)

				now := time.Now().UTC()
				exp := now.Add(c.Issuer.AccessTTL)
				kid, priv, _, kerr := c.Issuer.Keys.Active()
				if kerr != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma 2", 1204)
					return
				}
				claims := jwtv5.MapClaims{
					"iss": effIss,
					"sub": ufs.ID,
					"aud": req.ClientID,
					"iat": now.Unix(),
					"nbf": now.Unix(),
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
					httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1204)
					return
				}
				// avoid cache
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Pragma", "no-cache")
				httpx.WriteJSON(w, http.StatusOK, AuthLoginResponse{
					AccessToken: token,
					TokenType:   "Bearer",
					ExpiresIn:   int64(time.Until(exp).Seconds()),
					// No refresh in FS admin mode
				})
				return
			}
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		} else {
			repoCore = rc
		}
		// Optional: cache repo in request context for downstream calls
		ctx = helpers.WithTenantRepo(ctx, repoCore)

		log.Printf("DEBUG: passed guards, parsing request")

		// Rate limiting específico para login (endpoint semántico)
		log.Printf("DEBUG: checking rate limiting, MultiLimiter=%v", c.MultiLimiter != nil)
		if c.MultiLimiter != nil {
			// Parseamos la configuración específica para login
			loginWindow, err := time.ParseDuration(cfg.Rate.Login.Window)
			if err != nil {
				log.Printf("DEBUG: rate limit window parse error: %v, using fallback", err)
				loginWindow = time.Minute // fallback
			}

			loginCfg := helpers.LoginRateConfig{
				Limit:  cfg.Rate.Login.Limit,
				Window: loginWindow,
			}

			log.Printf("DEBUG: calling EnforceLoginLimit with limit=%d, window=%s", loginCfg.Limit, loginCfg.Window)

			// Rate limiting
			rateLimited := !helpers.EnforceLoginLimit(w, r, c.MultiLimiter, loginCfg, req.TenantID, req.Email)

			if rateLimited {
				// Rate limited - la función ya escribió la respuesta 429
				return
			}
		}

		log.Printf("DEBUG: passed rate limiting")

		// Si no teníamos client en FS, intentar lookup en DB solo ahora que repo está abierto
		if !haveFSClient {
			// Fallback: try DB client lookup (works with global repo)
			type clientGetter interface {
				GetClientByClientID(ctx context.Context, clientID string) (*core.Client, *core.ClientVersion, error)
			}
			if cg, ok := any(repoCore).(clientGetter); ok {
				if cdb, _, e2 := cg.GetClientByClientID(ctx, req.ClientID); e2 == nil && cdb != nil {
					clientScopes = append([]string{}, cdb.Scopes...)
					clientProviders = append([]string{}, cdb.Providers...)
					// synthesize minimal fsClient for downstream fields if needed
					fsClient = helpers.FSClient{TenantSlug: tenantSlug, ClientID: cdb.ClientID}
				} else {
					httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1203)
					return
				}
			} else {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1203)
				return
			}
		}

		// Provider gating: ensure password login is allowed for this client.
		// If providers are defined (FS or DB), require "password".
		if len(clientProviders) > 0 {
			allowed := false
			for _, p := range clientProviders {
				if strings.EqualFold(p, "password") {
					allowed = true
					break
				}
			}
			if !allowed {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "password login deshabilitado para este client", 1207)
				return
			}
		}

		// Debug: before repo call
		log.Printf("DEBUG: calling GetUserByEmail with tenant_id=%s, email=%s", tenantUUID, util.MaskEmail(req.Email))

		// ctx ya está definido con timeout arriba
		repo, _ := helpers.GetTenantRepo(ctx)
		u, id, err := repo.GetUserByEmail(ctx, tenantUUID, req.Email)
		if err != nil {
			status := http.StatusInternalServerError
			if err == core.ErrNotFound {
				status = http.StatusUnauthorized
			}
			log.Printf("auth login: user not found or err: %v (tenant=%s email=%s)", err, req.TenantID, util.MaskEmail(req.Email))
			httpx.WriteError(w, status, "invalid_credentials", "usuario o password inválidos", 1201)
			return
		}

		// Bloqueo por usuario deshabilitado
		isBlocked := false
		if u.DisabledUntil != nil {
			if time.Now().Before(*u.DisabledUntil) {
				isBlocked = true
			}
		} else if u.DisabledAt != nil {
			isBlocked = true
		}

		if isBlocked {
			// prefer 423 Locked for login when disabled
			httpx.WriteError(w, http.StatusLocked, "user_disabled", "usuario deshabilitado", 1210)
			return
		}
		if id == nil || id.PasswordHash == nil || *id.PasswordHash == "" || !repo.CheckPassword(id.PasswordHash, req.Password) {
			log.Printf("auth login: verify=false (tenant=%s email=%s)", req.TenantID, util.MaskEmail(req.Email))
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "usuario o password inválidos", 1202)
			return
		}
		// Client already validated by FS; nothing else to load from DB here

		// Email verification check: if client requires verification and user isn't verified, block login
		if cpctx.Provider != nil {
			if clientCfg, err := cpctx.Provider.GetClient(ctx, tenantSlug, req.ClientID); err == nil && clientCfg != nil {
				if clientCfg.RequireEmailVerification && !u.EmailVerified {
					log.Printf(`{"level":"info","msg":"login_email_not_verified","tenant":"%s","email":"%s"}`, req.TenantID, util.MaskEmail(req.Email))
					httpx.WriteError(w, http.StatusForbidden, "email_not_verified", "Debes verificar tu email para iniciar sesión", 1211)
					return
				}
			}
		}

		// MFA (pre-issue) hook: si el usuario tiene MFA TOTP confirmada y no se detecta trusted device => bifurca flujo.
		// Requiere métodos stub en Store: GetMFATOTP, IsTrustedDevice. Si no existen aún, este bloque no compilará hasta implementarlos.
		type mfaGetter interface {
			GetMFATOTP(ctx context.Context, userID string) (*core.MFATOTP, error)
		}
		type trustedChecker interface {
			IsTrustedDevice(ctx context.Context, userID, deviceHash string, now time.Time) (bool, error)
		}
		trustedByCookie := false
		if mg, ok := any(repo).(mfaGetter); ok {
			if m, _ := mg.GetMFATOTP(ctx, u.ID); m != nil && m.ConfirmedAt != nil { // usuario tiene MFA configurada
				if devCookie, err := r.Cookie("mfa_trust"); err == nil && devCookie != nil {
					if tc, ok2 := any(repo).(trustedChecker); ok2 {
						dh := tokens.SHA256Base64URL(devCookie.Value)
						if ok3, _ := tc.IsTrustedDevice(ctx, u.ID, dh, time.Now()); ok3 {
							trustedByCookie = true
						}
					}
				}
				if !trustedByCookie { // pedir MFA interactiva
					ch := mfaChallenge{
						UserID:   u.ID,
						TenantID: req.TenantID,
						ClientID: req.ClientID,
						AMRBase:  []string{"pwd"},
						Scope:    []string{},
					}
					mid, _ := tokens.GenerateOpaqueToken(24)
					key := "mfa:token:" + mid
					buf, _ := json.Marshal(ch)
					c.Cache.Set(key, buf, 5*time.Minute) // TTL 5m

					w.Header().Set("Cache-Control", "no-store")
					w.Header().Set("Pragma", "no-cache")
					httpx.WriteJSON(w, http.StatusOK, map[string]any{
						"mfa_required": true,
						"mfa_token":    mid,
						"amr":          []string{"pwd"},
					})
					return
				}
			}
		}

		// Base claims (normal path)
		amrSlice := []string{"pwd"}
		acrVal := "urn:hellojohn:loa:1"
		if trustedByCookie { // Dispositivo previamente validado por MFA
			amrSlice = []string{"pwd", "mfa"}
			acrVal = "urn:hellojohn:loa:2"
		}
		// Scopes placeholder: grant client default scopes for now (Phase 4 minimal)
		grantedScopes := append([]string{}, clientScopes...)
		std := map[string]any{
			"tid": tenantUUID,
			"amr": amrSlice,
			"acr": acrVal,
			"scp": strings.Join(grantedScopes, " "),
		}
		custom := map[string]any{}

		// Hook opcional (CEL/webhook/etc.)
		std, custom = applyAccessClaimsHook(ctx, c, tenantUUID, req.ClientID, u.ID, grantedScopes, amrSlice, std, custom)

		// ── RBAC (Fase 2): roles/perms opcionales si el repo per-tenant los implementa
		type rbacReader interface {
			GetUserRoles(ctx context.Context, userID string) ([]string, error)
			GetUserPermissions(ctx context.Context, userID string) ([]string, error)
		}
		var roles, perms []string
		if repoRR, ok := any(repo).(rbacReader); ok {
			roles, _ = repoRR.GetUserRoles(ctx, u.ID)
			perms, _ = repoRR.GetUserPermissions(ctx, u.ID)
		}
		// issuer se ajusta más abajo según tenant

		// Resolver issuer efectivo por tenant y firmar con clave del tenant si existe
		effIss := c.Issuer.Iss
		if cpctx.Provider != nil {
			if ten, errTen := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug); errTen == nil && ten != nil {
				effIss = jwtx.ResolveIssuer(c.Issuer.Iss, ten.Settings.IssuerMode, ten.Slug, ten.Settings.IssuerOverride)
			}
		}
		// Actualizar system claims con el issuer efectivo
		custom = helpers.PutSystemClaimsV2(custom, effIss, u.Metadata, roles, perms)

		now := time.Now().UTC()
		exp := now.Add(c.Issuer.AccessTTL)
		var (
			kid  string
			priv any
			kerr error
		)
		// Elegir clave según modo del issuer: Path => por tenant; Global/Default => global
		if cpctx.Provider != nil {
			if ten, errTen := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug); errTen == nil && ten != nil && ten.Settings.IssuerMode == controlplane.IssuerModePath {
				kid, priv, _, kerr = c.Issuer.Keys.ActiveForTenant(tenantSlug)
			} else {
				kid, priv, _, kerr = c.Issuer.Keys.Active()
			}
		} else {
			kid, priv, _, kerr = c.Issuer.Keys.Active()
		}
		if kerr != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma 3", 1204)
			return
		}
		claims := jwtv5.MapClaims{
			"iss": effIss,
			"sub": u.ID,
			"aud": req.ClientID,
			"iat": now.Unix(),
			"nbf": now.Unix(),
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
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1204)
			return
		}

		// Crear refresh token usando método TC
		tcStore, ok := any(repo).(interface {
			CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error)
		})
		if !ok {
			httpx.WriteError(w, http.StatusInternalServerError, "store_not_supported", "store no soporta métodos TC", 1205)
			return
		}

		rawRT, err := tcStore.CreateRefreshTokenTC(ctx, tenantUUID, req.ClientID, u.ID, refreshTTL)
		if err != nil {
			log.Printf("login: create refresh err: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 1206)
			return
		}

		// evitar cache en respuestas que incluyen tokens
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")

		httpx.WriteJSON(w, http.StatusOK, AuthLoginResponse{
			AccessToken:  token,
			TokenType:    "Bearer",
			ExpiresIn:    int64(time.Until(exp).Seconds()),
			RefreshToken: rawRT,
		})
	}
}
