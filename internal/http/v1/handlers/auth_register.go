/*
auth_register.go â€” Registro de usuario (tenant/client) + opciÃ³n FS-admin + (opcional) auto-login + (opcional) email de verificaciÃ³n

Este handler implementa el endpoint de registro â€œpassword-basedâ€ y, dependiendo de flags/config, tambiÃ©n:
  - Permite registrar â€œFS adminsâ€ globales (sin tenant/client) cuando FS_ADMIN_ENABLE=1
  - Puede hacer auto-login (emitir access + refresh) tras registrar (autoLogin=true)
  - Puede disparar email de verificaciÃ³n si el client lo requiere y hay emailHandler

================================================================================
QuÃ© hace (objetivo funcional)
================================================================================
1) Valida request y normaliza email/ids.
2) Determina si es un registro normal (tenant/client) o un registro FS-admin global.
3) En registro normal:
   - Resuelve tenantSlug + tenantUUID
   - Resuelve client (prefer FS control-plane; fallback a DB si no estÃ¡ en FS)
   - Aplica â€œprovider gatingâ€: si el client declara providers y NO incluye "password", bloquea.
   - Aplica polÃ­tica de password blacklist (opcional).
   - Hashea password.
   - Crea usuario en el repo del tenant + crea identidad password (username/password).
   - Si autoLogin:
        - Emite access JWT (issuer efectivo segÃºn tenant)
        - Crea refresh token (prefer mÃ©todo TC si existe; fallback legacy)
        - (opcional) envÃ­a email de verificaciÃ³n si el client lo exige
        - Responde con tokens + user_id
     Si NO autoLogin:
        - Responde solo user_id

================================================================================
Entrada / salida
================================================================================
Request JSON:
  {
    "tenant_id": "<requerido salvo modo FS-admin>",
    "client_id": "<requerido salvo modo FS-admin>",
    "email": "<requerido>",
    "password": "<requerido>",
    "custom_fields": { ... }  // opcional
  }

Response JSON (segÃºn modo):
- FS-admin register: { user_id, access_token, token_type, expires_in }   (sin refresh)
- Normal sin autoLogin: { user_id }
- Normal con autoLogin: { user_id, access_token, token_type, expires_in, refresh_token }

Errores tÃ­picos:
- 400 missing_fields si falta email/password; o falta tenant/client cuando no estÃ¡ FS-admin enabled
- 401 invalid_client si tenant/client invÃ¡lido, o password provider deshabilitado para ese client
- 409 email_taken si CreatePasswordIdentity devuelve core.ErrConflict
- 400 policy_violation si password estÃ¡ en blacklist
- 500/4xx tenant db missing/error via httpx helpers

================================================================================
Flujo paso a paso (detallado)
================================================================================

A) Validaciones HTTP + parseo
- Solo POST.
- ReadJSON sobre AuthRegisterRequest.
- Normaliza:
    email => trim + lower
    tenant_id/client_id => trim
- Requiere email + password siempre.

B) Rama â€œsin tenant_id/client_idâ€: modo FS-admin (si estÃ¡ habilitado)
- Si tenant_id=="" || client_id=="":
    - Si helpers.FSAdminEnabled():
        - helpers.FSAdminRegister(email, password)
        - Emite ACCESS token JWT (aud="admin", tid="global", amr=["pwd"], scopes fijos openid/profile/email)
        - Responde 200 con user_id + access_token (sin refresh)
    - Si NO FSAdminEnabled => 400 (tenant_id y client_id obligatorios)
ðŸ“Œ PatrÃ³n: Strategy/Branching por â€œtipo de sujetoâ€ (tenant user vs global admin)
âš ï¸ ObservaciÃ³n: esta rama repite bastante lÃ³gica de emisiÃ³n JWT que aparece en login/refresh.

C) Registro normal: resolver tenant + abrir repo
- ctx := r.Context()
- tenantSlug, tenantUUID := helpers.ResolveTenantSlugAndID(ctx, req.TenantID)

- Resolver client:
    - Primero intenta FS: helpers.ResolveClientFSBySlug(ctx, tenantSlug, req.ClientID)
      Si OK => clientProviders/clientScopes vienen de FS.
    - DespuÃ©s abre repo del tenant (gating por DSN):
        helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug)
      Maneja:
        - tenant invÃ¡lido => 401 invalid_client
        - (cualquier otro error) si FS admin enabled => fallback registra FS-admin (!!)
        - si IsNoDBForTenant => WriteTenantDBMissing
        - else WriteTenantDBError
  âš ï¸ Ojo importante: el â€œfallback FS-adminâ€ se activa ante CUALQUIER error de apertura de repo
     (excepto tenant inexistente). Eso incluye caÃ­das temporales de DB: podÃ©s terminar creando admins
     por un problema infra. No digo que estÃ© mal ahora, pero es una decisiÃ³n heavy para revisar despuÃ©s.

- Si no habÃ­a client en FS, intenta lookup en DB:
    repo.(clientGetter).GetClientByClientID(...)
  y toma providers/scopes desde ahÃ­.
  Si el repo no implementa esa interfaz => invalid_client.
ðŸ“Œ PatrÃ³n: Adapter/Capability-based programming via type assertion (ok para compat, pero ensucia handler).

D) Provider gating (password login habilitado)
- Si clientProviders no estÃ¡ vacÃ­o:
    requiere que incluya "password"
  Si no => 401 invalid_client (â€œpassword login deshabilitado para este clientâ€)
ðŸ“Œ PatrÃ³n: Policy Gate / Guard Clause.

E) Password policy (blacklist opcional)
- Obtiene path:
    - param blacklistPath
    - o env SECURITY_PASSWORD_BLACKLIST_PATH
- Si hay path:
    - password.GetCachedBlacklist(path)
    - si Contains(password) => 400 policy_violation
ðŸ“Œ PatrÃ³n: Cache (memoization) vÃ­a GetCachedBlacklist.
âš ï¸ Esto mete polÃ­tica en el handler; en V2 convendrÃ­a mover a un PasswordPolicyService.

F) Crear usuario + identidad password
- phc := password.Hash(password.Default, req.Password)
- Construye core.User:
    TenantID=tenantUUID
    Email, EmailVerified=false
    Metadata={}
    CustomFields=req.CustomFields
    SourceClientID=&req.ClientID
- repo.CreateUser(ctx, u)
- repo.CreatePasswordIdentity(ctx, u.ID, email, verified=false, phc)
    - Si ErrConflict => 409 email_taken
ðŸ“Œ PatrÃ³n: Transaction Script (todo en un handler). En V2 ideal â€œRegisterUserUseCaseâ€.

G) Si autoLogin == false
- Responde 200 con {user_id} y listo.

H) Si autoLogin == true: emitir tokens
- grantedScopes := clientScopes (copiado)
- std claims:
    tid=tenantUUID
    amr=["pwd"]
    acr="urn:hellojohn:loa:1"
    scp="..."
- custom := {}
- applyAccessClaimsHook(...) puede mutar std/custom

- Resolver issuer efectivo por tenant:
    effIss = jwtx.ResolveIssuer(base, issuerMode, slug, override)
- SelecciÃ³n de key:
    - si issuerMode == Path => Keys.ActiveForTenant(tenantSlug)
    - else => Keys.Active()
- Emite access token EdDSA.

- Refresh token:
    - genera rawRT (pero si existe CreateRefreshTokenTC, lo reemplaza con el retornado por TC)
    - Si repo implementa CreateRefreshTokenTC(ctx, tenantID, clientID, userID, ttl) => usa eso
    - else fallback legacy:
        hash := tokens.SHA256Base64URL(rawRT)   (nota: en refresh.go se usa hex; inconsistencia)
        repo.CreateRefreshToken(...)

- Headers no-store/no-cache.

I) Email verification (opcional, â€œsoftâ€)
- Determina si el client requiere verificaciÃ³n:
    - cpctx.Provider.GetTenantBySlug(...)
    - cpctx.Provider.GetClient(...).RequireEmailVerification
- Si verificationRequired && emailHandler != nil:
    - construye tidUUID, uidUUID (Parse)
    - emailHandler.SendVerificationEmail(ctx, rid, tidUUID, uidUUID, email, redirect="", clientID)
  âš ï¸ EstÃ¡ â€œsoft failâ€: ignora error (por diseÃ±o).
  âš ï¸ El rid se saca de w.Header().Get("X-Request-ID") (probablemente vacÃ­o si nadie lo setea antes).

J) Responde 200 con AuthRegisterResponse completo.

================================================================================
QuÃ© NO se usa / cosas raras (marcadas)
================================================================================
- Func contains(ss, v) estÃ¡ DECLARADA pero NO se usa en este archivo.
- Import "context" se usa (para clientGetter interface, etc) OK.
- Import "jwtx" se usa.
- Variable fsClient se guarda pero prÃ¡cticamente no se usa luego (mÃ¡s que para â€œhaveFSClientâ€ y scopes/providers).
- tokens.GenerateOpaqueToken(32) se llama aunque luego se pisa con CreateRefreshTokenTC si existe.
  (micro-ineficiente; no rompe nada).
- Inconsistencia de hashing de refresh en fallback:
    - acÃ¡: SHA256Base64URL(rawRT)
    - en refresh.go/logout: SHA256 hex
  Esto es re importante a revisar globalmente despuÃ©s (vos ya venÃ­s viendo esa mezcla TC/legacy).
- Fallback FS-admin cuando falla abrir repo (por cualquier error) puede ser riesgoso.

================================================================================
Patrones (GoF / arquitectura) y cÃ³mo lo refactorizarÃ­a en V2
================================================================================

1) Strategy (GoF) para â€œmodo de registroâ€
- RegisterStrategy:
    - FSAdminRegisterStrategy
    - TenantUserRegisterStrategy
  El handler decide y delega. Te limpia el â€œif tenant/client missingâ€ + el fallback raro.

2) Application Service / Use Case (Service Layer)
- RegisterUserService.Register(ctx, cmd) -> result
  cmd incluye: tenantRef, clientID, email, password, customFields, autoLogin flag.
  Eso te separa:
    - ValidaciÃ³n/polÃ­ticas
    - Persistencia
    - EmisiÃ³n de tokens
    - Side-effects (email)

3) Template Method para â€œemitir access tokenâ€
- Hoy la emisiÃ³n JWT se repite en login/refresh/register (y admin variants).
- Crear:
    TokenIssuer.IssueAccess(ctx, params) -> token string, exp time.Time
  internamente:
    - resolve issuer
    - select key
    - build claims std/custom
    - sign
  AsÃ­ dejÃ¡s un solo camino.

4) Policy objects (GoF-ish) / Chain of Responsibility para validaciones
- PasswordPolicyChain:
    - BlacklistPolicy
    - MinLengthPolicy (si existe)
    - StrengthPolicy (si existe)
- ClientAuthPolicy:
    - ProviderGatingPolicy (password enabled)
    - EmailVerificationPolicy (si aplica al login/register)
  Cada policy retorna error tipado => el handler traduce a HTTP.

5) Ports & Adapters (Clean-ish)
- Definir interfaces â€œclarasâ€:
    ClientCatalog (FS/DB)
    UserRepository
    IdentityRepository
    RefreshTokenRepository
  En vez de type assertions â€œsi implementa Xâ€.
  En V1 podÃ©s mantener adapters que envuelvan repo y expongan las capacidades.

6) Observer / Domain Events (GoF: Observer)
- En vez de que el handler â€œmande emailâ€:
    - EmitÃ­s evento UserRegistered{tenant, user, client, verificationRequired}
    - Subscriber: EmailVerificationSender
  AsÃ­, si maÃ±ana querÃ©s colgar auditorÃ­a, mÃ©tricas, welcome email, etc, no tocÃ¡s el handler.

7) Consistencia de tenantRef (slug + uuid)
- Formalizar un TenantRef:
    type TenantRef struct { Slug string; ID uuid.UUID }
  y listo. Deja de haber comparaciones raras y parseos sueltos.

================================================================================
Resumen
================================================================================
- auth_register.go hace: register tenant-user con password (+ blacklist + provider gating), crea identidad,
  y opcionalmente auto-login + refresh + email verification; y tambiÃ©n permite registrar FS-admin global.
- EstÃ¡ cargado de responsabilidades mezcladas (token issuance, policies, repo opening, email side-effect),
  con repeticiÃ³n respecto a login/refresh y varias â€œcompat branchesâ€ (FS vs DB).
- Para V2, lo mÃ¡s rentable es separar por Strategy (FS-admin vs tenant user), extraer TokenIssuer compartido,
  y mover polÃ­ticas + side effects (email) a services/events para limpiar el handler.
*/

package handlers

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/controlplane/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/http/v1/helpers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/v1/core"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type AuthRegisterRequest struct {
	TenantID     string         `json:"tenant_id"`
	ClientID     string         `json:"client_id"`
	Email        string         `json:"email"`
	Password     string         `json:"password"`
	CustomFields map[string]any `json:"custom_fields,omitempty"`
}

type AuthRegisterResponse struct {
	UserID       string `json:"user_id,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}

func NewAuthRegisterHandler(c *app.Container, emailHandler *EmailFlowsHandler, autoLogin bool, refreshTTL time.Duration, blacklistPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req AuthRegisterRequest
		if !httpx.ReadJSON(w, r, &req) {
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		req.TenantID = strings.TrimSpace(req.TenantID)
		req.ClientID = strings.TrimSpace(req.ClientID)
		// Require email and password. Tenant and client optional to allow global FS-admin register.
		if req.Email == "" || req.Password == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "email y password son obligatorios", 1002)
			return
		}
		if req.TenantID == "" || req.ClientID == "" {
			if helpers.FSAdminEnabled() {
				// Register as FS admin directly
				ufs, ferr := helpers.FSAdminRegister(req.Email, req.Password)
				if ferr != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "register_failed", ferr.Error(), 1204)
					return
				}
				grantedScopes := []string{"openid", "profile", "email"}
				std := map[string]any{
					"tid": "global",
					"amr": []string{"pwd"},
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
					httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma", 1201)
					return
				}
				claims := jwtv5.MapClaims{
					"iss": effIss,
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
					httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1201)
					return
				}
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Pragma", "no-cache")
				httpx.WriteJSON(w, http.StatusOK, AuthRegisterResponse{
					UserID:      ufs.ID,
					AccessToken: token,
					TokenType:   "Bearer",
					ExpiresIn:   int64(time.Until(exp).Seconds()),
				})
				return
			}
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id y client_id son obligatorios", 1002)
			return
		}

		// Contexto
		ctx := r.Context()

		// Resolver slug + UUID del tenant
		tenantSlug, tenantUUID := helpers.ResolveTenantSlugAndID(ctx, req.TenantID)

		// Resolver client desde FS si existe
		var (
			fsClient        helpers.FSClient
			haveFSClient    bool
			clientProviders []string
			clientScopes    []string
		)
		if fsc, err := helpers.ResolveClientFSBySlug(ctx, tenantSlug, req.ClientID); err == nil {
			fsClient = fsc
			haveFSClient = true
			clientProviders = append([]string{}, fsClient.Providers...)
			clientScopes = append([]string{}, fsClient.Scopes...)
		}

		// Abrir repo por tenant (gating por DSN)
		if c == nil || c.TenantSQLManager == nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "tenant manager not initialized", 1003)
			return
		}
		var repo core.Repository
		rc, err := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug)
		if err != nil {
			if helpers.IsTenantNotFound(err) {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "tenant invÃ¡lido", 2100)
				return
			}
			// Fallback FS-admin para cualquier error de apertura (excepto tenant inexistente)
			if helpers.FSAdminEnabled() {
				ufs, ferr := helpers.FSAdminRegister(req.Email, req.Password)
				if ferr != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "register_failed", ferr.Error(), 1204)
					return
				}
				grantedScopes := []string{"openid", "profile", "email"}
				std := map[string]any{
					"tid": "global",
					"amr": []string{"pwd"},
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
					httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma", 1201)
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
					httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1201)
					return
				}
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Pragma", "no-cache")
				httpx.WriteJSON(w, http.StatusOK, AuthRegisterResponse{
					UserID:      ufs.ID,
					AccessToken: token,
					TokenType:   "Bearer",
					ExpiresIn:   int64(time.Until(exp).Seconds()),
				})
				return
			}
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		}
		repo = rc

		// Si no hay client en FS, intentar obtenerlo desde DB (modo compat)
		if !haveFSClient {
			type clientGetter interface {
				GetClientByClientID(ctx context.Context, clientID string) (*core.Client, *core.ClientVersion, error)
			}
			if cg, ok := any(repo).(clientGetter); ok {
				if cdb, _, e2 := cg.GetClientByClientID(ctx, req.ClientID); e2 == nil && cdb != nil {
					clientProviders = append([]string{}, cdb.Providers...)
					clientScopes = append([]string{}, cdb.Scopes...)
					fsClient = helpers.FSClient{TenantSlug: tenantSlug, ClientID: cdb.ClientID}
				} else {
					httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client invÃ¡lido", 1102)
					return
				}
			} else {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client invÃ¡lido", 1102)
				return
			}
		}

		// Provider gating: si existen providers y no incluye password => bloquear
		if len(clientProviders) > 0 {
			allowed := false
			for _, p := range clientProviders {
				if strings.EqualFold(p, "password") {
					allowed = true
					break
				}
			}
			if !allowed {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "password login deshabilitado para este client", 1104)
				return
			}
		}

		// Blacklist opcional
		p := strings.TrimSpace(blacklistPath)
		if p == "" {
			p = strings.TrimSpace(os.Getenv("SECURITY_PASSWORD_BLACKLIST_PATH"))
		}
		if p != "" {
			if bl, err := password.GetCachedBlacklist(p); err == nil && bl.Contains(req.Password) {
				httpx.WriteError(w, http.StatusBadRequest, "policy_violation", "password no permitido por polÃ­tica", 2401)
				return
			}
		}

		phc, err := password.Hash(password.Default, req.Password)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "hash_failed", "no se pudo hashear el password", 1200)
			return
		}

		u := &core.User{
			TenantID:       tenantUUID,
			Email:          req.Email,
			EmailVerified:  false,
			Metadata:       map[string]any{},
			CustomFields:   req.CustomFields,
			SourceClientID: &req.ClientID,
		}
		if err := repo.CreateUser(ctx, u); err != nil {
			log.Printf("register: create user err: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "register_failed", "no se pudo crear el usuario", 1204)
			return
		}

		if err := repo.CreatePasswordIdentity(ctx, u.ID, req.Email, false, phc); err != nil {
			if err == core.ErrConflict {
				httpx.WriteError(w, http.StatusConflict, "email_taken", "ya existe un usuario con ese email", 1409)
				return
			}
			log.Printf("register: create identity err: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "register_failed", "no se pudo crear la identidad", 1204)
			return
		}

		// Si no hay auto-login, devolver sÃ³lo user_id
		if !autoLogin {
			httpx.WriteJSON(w, http.StatusOK, AuthRegisterResponse{UserID: u.ID})
			return
		}

		// Auto-login + refresh inicial
		grantedScopes := append([]string{}, clientScopes...)
		std := map[string]any{
			"tid": tenantUUID,
			"amr": []string{"pwd"},
			"acr": "urn:hellojohn:loa:1",
			"scp": strings.Join(grantedScopes, " "),
		}
		custom := map[string]any{}

		// Hook opcional
		std, custom = applyAccessClaimsHook(ctx, c, tenantUUID, req.ClientID, u.ID, grantedScopes, []string{"pwd"}, std, custom)

		// Per-tenant issuer + signing key
		effIss := c.Issuer.Iss
		if cpctx.Provider != nil {
			if ten, errTen := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug); errTen == nil && ten != nil {
				effIss = jwtx.ResolveIssuer(c.Issuer.Iss, string(ten.Settings.IssuerMode), ten.Slug, ten.Settings.IssuerOverride)
			}
		}
		now := time.Now().UTC()
		exp := now.Add(c.Issuer.AccessTTL)
		var (
			kid  string
			priv any
			kerr error
		)
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
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma", 1201)
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
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1201)
			return
		}

		rawRT, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh token", 1202)
			return
		}
		if tcs, ok := any(repo).(interface {
			CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error)
		}); ok {
			rawRT, err = tcs.CreateRefreshTokenTC(ctx, tenantUUID, req.ClientID, u.ID, refreshTTL)
			if err != nil {
				log.Printf("register: create refresh TC err: %v", err)
				httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh token", 1203)
				return
			}
		} else {
			hash := tokens.SHA256Base64URL(rawRT)
			if _, err := repo.CreateRefreshToken(ctx, u.ID, req.ClientID, hash, time.Now().Add(refreshTTL), nil); err != nil {
				log.Printf("register: create refresh err: %v", err)
				httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh token", 1203)
				return
			}
		}

		// evitar cache en respuestas que incluyen tokens
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")

		// Check for email verification requirement (FS Client)
		var verificationRequired bool
		if cpctx.Provider != nil {
			if ten, tErr := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug); tErr == nil && ten != nil {
				if fsc, cErr := cpctx.Provider.GetClient(ctx, tenantSlug, req.ClientID); cErr == nil && fsc != nil {
					verificationRequired = fsc.RequireEmailVerification
				}
			}
		}

		// Trigger verification email if required
		if verificationRequired && emailHandler != nil {
			// We use a background context or the request context? Request context is fine, but we should not fail register if mail fails (soft fail).
			rid := w.Header().Get("X-Request-ID")
			// Create uuid from string for tenantID/userID
			tidUUID, _ := uuid.Parse(tenantUUID) // should be valid
			uidUUID, _ := uuid.Parse(u.ID)

			// We pass empty redirect (will use default) or we can try to guess/construct it.
			// The email link will point to /v1/auth/verify-email?token=... which redirects to Valid Redirect URI.
			// Since we don't have a specific redirect URI in Register Request, we pass empty string.
			_ = emailHandler.SendVerificationEmail(ctx, rid, tidUUID, uidUUID, req.Email, "", req.ClientID)
		}

		httpx.WriteJSON(w, http.StatusOK, AuthRegisterResponse{
			UserID:       u.ID,
			AccessToken:  token,
			TokenType:    "Bearer",
			ExpiresIn:    int64(time.Until(exp).Seconds()),
			RefreshToken: rawRT,
		})
	}
}
