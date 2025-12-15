/*
Email Flows Wiring (email_flows_wiring.go)
───────────────────────────────────────────────────────────────────────────────
Qué es esto (y qué NO es)
- NO es un handler HTTP “de endpoint” directo.
- Es el “wiring / composition root” de los flows de email (verify email / forgot password / reset password):
  arma dependencias, adapta interfaces y devuelve handlers listos (`http.Handler`) para registrar en el router.

Qué construye / expone
- Función pública: `BuildEmailFlowHandlers(...)` devuelve:
  - `verifyStart`   -> handler para iniciar verificación de email (genera token + envía mail).
  - `verifyConfirm` -> handler para confirmar verificación (consume token).
  - `forgot`        -> handler para “olvidé mi contraseña” (genera token + envía mail).
  - `reset`         -> handler para resetear password (consume token; opcionalmente auto-login).
  - `efHandler`     -> instancia de `EmailFlowsHandler` ya configurada (para tests/uso interno).
  - `cleanup`       -> función de cierre (solo útil si se abrió conexión manual).
  - `err`           -> error de wiring (si faltan templates/DSN/etc.).

Cómo se usa (enrutamiento típico)
- En el bootstrap de HTTP (main/router):
  - `verifyStart, verifyConfirm, forgot, reset, _, cleanup, err := BuildEmailFlowHandlers(...)`
  - Registrar endpoints: `router.Handle("/v1/auth/verify-email/start", verifyStart)` etc. (según routes reales).
  - `defer cleanup()` (si corresponde).
- Los handlers concretos viven en `EmailFlowsHandler` (ver `email_flows.go`), acá solo se arma todo.

Flujo del wiring (paso a paso)
1) Templates:
   - `email.LoadTemplates(cfg.Email.TemplatesDir)`
   - Si falla, aborta el wiring (no tiene sentido exponer flows sin templates).
2) Política de password:
   - Construye `password.Policy` desde `cfg.Security.PasswordPolicy`.
   - Se pasa al handler para validar passwords en reset/registro (según flujo).
3) Rate limiting opcional (Redis):
   - Si `cfg.Rate.Enabled` y `cfg.Cache.Kind == "redis"`:
     - Crea `redis.Client`.
     - Crea `rate.NewRedisLimiter(...)`.
     - Lo envuelve con `flowsLimiterAdapter` para cumplir `RateLimiter`.
   - Si no, queda `flowsLimiter` nil (=> el adapter permite todo).
4) DB ops / pool reutilizable:
   - Intenta reusar el pool si `c.Store` expone `Pool() *pgxpool.Pool`.
     - Objetivo: evitar “too many connections”.
   - Fallback (legacy/riesgo): abre `pgx.Connect(ctx, cfg.Storage.DSN)` si el store no es pool-compatible.
     - En este caso `cleanup()` cierra esa conexión.
5) Stores internos de flows:
   - `TokenStore`: `store.NewTokenStore(dbOps)` (tokens de verificación/reset).
   - `UserStore`: `&store.UserStore{DB: dbOps}` (no usa constructor).
6) Adaptadores (puente entre flows y el resto del sistema):
   - `redirectValidatorAdapter`:
     - Valida `redirect_uri` contra client catalog (SQL) o fallback FS (control-plane).
   - `tokenIssuerAdapter`:
     - Emite access/refresh post-reset (auto-login) usando issuer + persistencia de refresh (preferencia TC).
   - `currentUserProviderAdapter`:
     - Extrae usuario/tenant/email desde Bearer JWT (para flows que lo necesiten).
7) Construye `EmailFlowsHandler` con todo lo anterior:
   - Inyecta SenderProvider + templates + policy + limiter + base URLs + TTLs + debug flags
   - Inyecta TenantMgr + Provider para resolver tenant/client (FS/DB).
8) Exporta handlers concretos:
   - `verifyStart = http.HandlerFunc(ef.verifyEmailStart)` etc.

Dependencias reales (qué usa y por qué)
- `config.Config`: fuente de truth de templates, cache/rate, política de password, TTLs, base URL.
- `app.Container`:
  - `c.Store`: repo/store global (catálogo clientes, users, roles/perms en algunos casos).
  - `c.TenantSQLManager`: abrir store per-tenant para persistir refresh tokens (modo TC).
  - `c.SenderProvider`: envío real de emails (abstrae SMTP/Sendgrid/etc).
  - `c.Issuer`: emite JWT (y resuelve keys por `kid`).
  - `c.ClaimsHook`: opcional, inyecta claims en access tokens.
- `cpctx.Provider`: control-plane FS (tenants/clients) para fallback y resoluciones.
- `store.TokenStore`, `store.UserStore`: stores “de flows” sobre `DBOps`.

Seguridad / invariantes que se están cuidando
- Redirect URI validation:
  - `redirectValidatorAdapter.ValidateRedirectURI(...)` busca `redirectURIs` del client:
    - Preferencia SQL (`repo.GetClientByClientID`), fallback FS (`cpctx.Provider`).
  - Verifica tenant match (`clientTenantID` vs `tenantID`) y exact match del redirect.
  - Si falla, loggea warnings (evita open redirect).
- Emisión de tokens en reset:
  - `tokenIssuerAdapter.IssueTokens(...)` valida que el client pertenezca al tenant.
  - Usa `applyAccessClaimsHook` + `helpers.PutSystemClaimsV2` para “SYS namespace” controlado.
  - Emite access con `c.Issuer.IssueAccess(...)`.
  - Emite refresh preferentemente vía `CreateRefreshTokenTC` (hash SHA256+hex interno).
  - Setea `Cache-Control: no-store` / `Pragma: no-cache`.
- Auth provider:
  - `currentUserProviderAdapter` parsea Bearer JWT:
    - valida método EdDSA y `issuer` (estricto) con `jwtv5.WithIssuer(a.issuer.Iss)`.
    - keyfunc resuelve por `kid` (JWKS/keystore).
  - Devuelve UUIDs desde claims `sub` y `tid`.

Patrones detectados (GoF + arquitectura)
- Adapter (GoF):
  - `flowsLimiterAdapter`, `redirectValidatorAdapter`, `tokenIssuerAdapter`, `currentUserProviderAdapter`
  - Todos “adaptan” contratos esperados por `EmailFlowsHandler` a implementaciones reales (rate limiter, repo, issuer, jwt parser).
- Facade / Composition Root:
  - `BuildEmailFlowHandlers` funciona como fachada de inicialización: arma y retorna handlers listos.
- Strategy (ligera):
  - “Estrategia” de storage: reusar pool vs abrir conexión manual.
  - “Estrategia” de validación client catalog: SQL primero, FS fallback.
  - “Estrategia” de refresh persistence: TC preferido, legacy fallback.
- Ports & Adapters (arquitectura):
  - `EmailFlowsHandler` consume puertos (`Redirect`, `Issuer`, `Auth`, `Limiter`) y acá se enchufan adaptadores concretos.

Cosas no usadas / legacy / riesgos (marcar sin decidir)
- `rid := w.Header().Get("X-Request-ID")`:
  - OJO: si el request-id lo setea middleware, perfecto. Si no, puede venir vacío (no rompe, pero log pierde trazabilidad).
- `goto proceed`:
  - Funciona, pero es un smell para legibilidad (podría ser un early-return + función helper).
- `redirectValidatorAdapter` depende de `cpctx.Provider` pero no checkea nil antes de usarlo en fallback:
  - Hoy se asume inicializado; si `cpctx.Provider` es nil y el SQL lookup falla, podría panickear.
- `tokenIssuerAdapter.IssueTokens` usa `ti.c.Store.GetUserByID` para metadata/RBAC:
  - En multi-tenant, “user por tenant DB” vs “global store” puede quedar inconsistente (depende cómo está modelado `Store`).
  - Hay mezcla: refresh se intenta persistir en tenant store, pero user/roles se leen del store global.
- Imports:
  - En este archivo se usan todos los imports listados (no veo “(No se usa)” obvio), pero ojo con `jwtx`:
    - Se usa solo para `currentUserProviderAdapter` (`issuer *jwtx.Issuer`), ok.
- Rate limiter:
  - `flowsLimiterAdapter.Allow` usa `context.Background()` (no request context):
    - Para rate limiting está bien (evita cancelaciones), pero si querés trazabilidad/timeout, podría pasar ctx request.

Ideas para V2 (sin decidir nada, solo guía de desarme en capas)
1) “DTO / Contracts”
   - Definir interfaces explícitas en un paquete `emailflows/ports`:
     - `RateLimiter`, `RedirectValidator`, `TokenIssuer`, `CurrentUserProvider`
   - Evitar que el handler conozca detalles de Redis/PGX/JWT.
2) “Service”
   - Crear `EmailFlowsService` (dominio) que contenga reglas:
     - generar tokens, validar TTL, aplicar policy, disparar mails, etc.
   - `EmailFlowsHandler` queda como controller HTTP liviano.
3) “Controller (HTTP)”
   - `email_flows.go` debería tener solo parseo/validación HTTP + llamada al service + response mapping.
   - Centralizar seteo de headers (`no-store`, content-type) en helpers.
4) “Infra / Clients”
   - `RedisLimiterClient` en infra/cache.
   - `ClientCatalog` (SQL/FS) como Strategy: `SQLClientCatalog` + `FSClientCatalog` + `CompositeCatalog`.
   - `RefreshTokenRepository` (TC/legacy) encapsulado.
5) “Builder / Wiring”
   - Mantener un único composition root:
     - `BuildEmailFlowHandlers` debería:
       - NO abrir conexiones si ya hay pool (idealmente siempre inyectar pool desde afuera).
       - Resolver `cpctx.Provider` nil-safe.
6) Patrones recomendables para V2
   - Composite + Strategy:
     - Para catálogo de clients (SQL + FS) y resolución de tenant/slug/uuid.
   - Template Method:
     - Para “armado de token response” repetido (access + refresh + headers).
   - Chain of Responsibility:
     - Para validaciones del flow (redirect ok -> token ok -> policy ok -> send ok), evitando ifs enormes en handlers.
   - Circuit Breaker / Bulkhead (si el mail provider es externo):
     - No está hoy, pero si el sender falla, conviene aislar y degradar sin tumbar todo.
   - Concurrencia:
     - No se usa explícitamente acá.
     - (Opcional) En envío de emails: si en algún flow se dispara “async send”, usar worker pool / queue,
       pero solo si el sistema lo necesita (hoy parece sync y controlado).

Resumen
- Este archivo arma el “pack” de handlers de email flows y sus dependencias.
- Implementa varios Adapters para desacoplar EmailFlowsHandler de Redis/DB/JWT/Issuer/ClientCatalog.
- Hay un enfoque claro de fallback (SQL -> FS, TC -> legacy) y de seguridad (redirect validation, no-store, issuer/keys).
- Para V2: separar puertos/servicios/infra y centralizar estrategias (catalog, refresh persistence) para reducir mezcla y duplicación.
*/

package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	rdb "github.com/redis/go-redis/v9"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/config"
	"github.com/dropDatabas3/hellojohn/internal/email"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/http/v1/helpers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/rate"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// ───────────────────── Adaptadores para Email Flows ─────────────────────

type flowsLimiterAdapter struct{ inner rate.Limiter }

func (a flowsLimiterAdapter) Allow(key string) (bool, time.Duration) {
	if a.inner == nil {
		return true, 0
	}
	res, err := a.inner.Allow(context.Background(), "mailflows:"+key)
	if err != nil {
		log.Printf(`{"level":"warn","msg":"flows_rl_error","key":"%s","err":"%v"}`, key, err)
		return true, 0
	}
	return res.Allowed, res.RetryAfter
}

type redirectValidatorAdapter struct{ repo core.Repository }

func (v redirectValidatorAdapter) ValidateRedirectURI(tenantID uuid.UUID, clientID, redirectURI string) bool {
	if clientID == "" || redirectURI == "" {
		return false
	}

	var redirectURIs []string
	var clientTenantID string

	// 1. Try SQL Repo
	cl, _, err := v.repo.GetClientByClientID(context.Background(), clientID)
	if err == nil && cl != nil {
		redirectURIs = cl.RedirectURIs
		clientTenantID = cl.TenantID
	} else {
		// 2. Fallback FS Provider
		// Resolve tenant to get slug
		if t, tErr := cpctx.Provider.GetTenantByID(context.Background(), tenantID.String()); tErr == nil && t != nil {
			if fsCl, fsErr := cpctx.Provider.GetClient(context.Background(), t.Slug, clientID); fsErr == nil && fsCl != nil {
				redirectURIs = fsCl.RedirectURIs
				// Since we found it via the tenant looked up by ID, the tenant matches.
				clientTenantID = tenantID.String()
			}
		}
	}

	if len(redirectURIs) == 0 {
		log.Printf(`{"level":"warn","msg":"redirect_validate_no_client","client_id":"%s","err":"not found in db or fs"}`, clientID)
		return false
	}

	if !strings.EqualFold(clientTenantID, tenantID.String()) {
		log.Printf(`{"level":"warn","msg":"redirect_validate_bad_tenant","client_id":"%s","expected_tid":"%s","client_tid":"%s"}`, clientID, tenantID, clientTenantID)
		return false
	}

	for _, ru := range redirectURIs {
		// Validar exact match o reglas más laxas si es necesario (ej: trailing slash)
		// Por ahora exact match como estaba.
		if ru == redirectURI {
			return true
		}
	}
	log.Printf(`{"level":"warn","msg":"redirect_validate_not_allowed","client_id":"%s","redirect":"%s"}`, clientID, redirectURI)
	return false
}

type tokenIssuerAdapter struct {
	c          *app.Container
	refreshTTL time.Duration
}

func (ti tokenIssuerAdapter) IssueTokens(w http.ResponseWriter, r *http.Request, tenantID uuid.UUID, clientID string, userID uuid.UUID) error {
	rid := w.Header().Get("X-Request-ID")

	// Try SQL store first
	cl, _, err := ti.c.Store.GetClientByClientID(r.Context(), clientID)
	if err != nil || cl == nil {
		// Fallback to FS provider: first get tenant by UUID, then lookup client
		if tenant, tErr := cpctx.Provider.GetTenantByID(r.Context(), tenantID.String()); tErr == nil && tenant != nil {
			if fsCl, fsErr := cpctx.Provider.GetClient(r.Context(), tenant.Slug, clientID); fsErr == nil && fsCl != nil {
				// Client found in FS, proceed with token issuance
				log.Printf(`{"level":"debug","msg":"issuer_client_from_fs","request_id":"%s","client_id":"%s","tenant":"%s"}`, rid, clientID, tenant.Slug)
				goto proceed
			}
		}
		log.Printf(`{"level":"warn","msg":"issuer_invalid_client","request_id":"%s","client_id":"%s","tenant_id":"%s","err":"%v"}`, rid, clientID, tenantID, err)
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1203)
		return nil
	}
	if !strings.EqualFold(cl.TenantID, tenantID.String()) {
		log.Printf(`{"level":"warn","msg":"issuer_invalid_client","request_id":"%s","client_id":"%s","tenant_id":"%s","err":"tenant mismatch"}`, rid, clientID, tenantID)
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1203)
		return nil
	}

proceed:
	std := map[string]any{"tid": tenantID.String(), "amr": []string{"reset"}}
	custom := map[string]any{}
	// Hook + SYS namespace (Fase 2)
	std, custom = applyAccessClaimsHook(r.Context(), ti.c, tenantID.String(), clientID, userID.String(), []string{}, []string{"reset"}, std, custom)
	if u, err := ti.c.Store.GetUserByID(r.Context(), userID.String()); err == nil && u != nil {
		type rbacReader interface {
			GetUserRoles(ctx context.Context, userID string) ([]string, error)
			GetUserPermissions(ctx context.Context, userID string) ([]string, error)
		}
		var roles, perms []string
		if rr, ok := ti.c.Store.(rbacReader); ok {
			roles, _ = rr.GetUserRoles(r.Context(), userID.String())
			perms, _ = rr.GetUserPermissions(r.Context(), userID.String())
		}
		custom = helpers.PutSystemClaimsV2(custom, ti.c.Issuer.Iss, u.Metadata, roles, perms)
	}

	access, exp, err := ti.c.Issuer.IssueAccess(userID.String(), clientID, std, custom)
	if err != nil {
		log.Printf(`{"level":"error","msg":"issuer_issue_access_err","request_id":"%s","err":"%v"}`, rid, err)
		httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1204)
		return nil
	}
	log.Printf(`{"level":"info","msg":"issuer_issue_access_ok","request_id":"%s","expires_in_sec":%d}`, rid, int(time.Until(exp).Seconds()))

	var rawRT string
	// IMPORTANTE: usamos CreateRefreshTokenTC (tenant + client_id_text) que hashea con SHA256+hex.
	// No volver a CreateRefreshToken (legacy, Base64URL) salvo en fallback cuando el store no implemente TC.

	// Try to get per-tenant store first
	var tenantStore interface {
		CreateRefreshTokenTC(ctx context.Context, tenantID, clientIDText, userID string, ttl time.Duration) (string, error)
	}

	if ti.c.TenantSQLManager != nil {
		// Get tenant by UUID to get slug, then get store
		if tenant, tErr := cpctx.Provider.GetTenantByID(r.Context(), tenantID.String()); tErr == nil && tenant != nil {
			if pgStore, sErr := ti.c.TenantSQLManager.GetPG(r.Context(), tenant.Slug); sErr == nil {
				tenantStore = pgStore
				log.Printf(`{"level":"debug","msg":"issuer_using_tenant_store","request_id":"%s","tenant":"%s"}`, rid, tenant.Slug)
			}
		}
	}

	// Fallback to global store
	if tenantStore == nil {
		if tcs, ok := ti.c.Store.(interface {
			CreateRefreshTokenTC(ctx context.Context, tenantID, clientIDText, userID string, ttl time.Duration) (string, error)
		}); ok {
			tenantStore = tcs
		}
	}

	if tenantStore != nil {
		// Preferir TC: (tenant + client_id_text) y hash SHA256+hex interno
		rawRT, err = tenantStore.CreateRefreshTokenTC(r.Context(), tenantID.String(), clientID, userID.String(), ti.refreshTTL)
		if err != nil {
			log.Printf(`{"level":"error","msg":"issuer_persist_refresh_tc_err","request_id":"%s","err":"%v"}`, rid, err)
			httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 1206)
			return nil
		}
	} else {
		// Fallback legacy (solo si el store no soporta TC)
		rawRT, err = tokens.GenerateOpaqueToken(32)
		if err != nil {
			log.Printf(`{"level":"error","msg":"issuer_gen_refresh_err","request_id":"%s","err":"%v"}`, rid, err)
			httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 1205)
			return nil
		}
		hash := tokens.SHA256Base64URL(rawRT)
		if _, err := ti.c.Store.CreateRefreshToken(r.Context(), userID.String(), cl.ID, hash, time.Now().Add(ti.refreshTTL), nil); err != nil {
			log.Printf(`{"level":"error","msg":"issuer_persist_refresh_legacy_err","request_id":"%s","err":"%v"}`, rid, err)
			httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 1206)
			return nil
		}
	}
	log.Printf(`{"level":"info","msg":"issuer_refresh_ok","request_id":"%s"}`, rid)

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(AuthLoginResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(exp).Seconds()),
		RefreshToken: rawRT,
	})
}

type currentUserProviderAdapter struct{ issuer *jwtx.Issuer }

func (a currentUserProviderAdapter) parse(r *http.Request) (jwtv5.MapClaims, error) {
	ah := strings.TrimSpace(r.Header.Get("Authorization"))
	if ah == "" || !strings.HasPrefix(strings.ToLower(ah), "bearer ") {
		return nil, http.ErrNoCookie
	}
	raw := strings.TrimSpace(ah[len("Bearer "):])
	tk, err := jwtv5.Parse(raw,
		a.issuer.Keyfunc(), // ← usa keystore/JWKS (resuelve según 'kid')
		jwtv5.WithValidMethods([]string{"EdDSA"}),
		jwtv5.WithIssuer(a.issuer.Iss),
	)
	if err != nil || !tk.Valid {
		return nil, http.ErrNoCookie
	}
	if c, ok := tk.Claims.(jwtv5.MapClaims); ok {
		return c, nil
	}
	return nil, http.ErrNoCookie
}
func (a currentUserProviderAdapter) CurrentUserID(r *http.Request) (uuid.UUID, error) {
	c, err := a.parse(r)
	if err != nil {
		return uuid.Nil, err
	}
	if s, _ := c["sub"].(string); s != "" {
		return uuid.Parse(s)
	}
	return uuid.Nil, http.ErrNoCookie
}
func (a currentUserProviderAdapter) CurrentTenantID(r *http.Request) (uuid.UUID, error) {
	c, err := a.parse(r)
	if err != nil {
		return uuid.Nil, err
	}
	if s, _ := c["tid"].(string); s != "" {
		return uuid.Parse(s)
	}
	return uuid.Nil, http.ErrNoCookie
}
func (a currentUserProviderAdapter) CurrentUserEmail(r *http.Request) (string, error) {
	c, err := a.parse(r)
	if err != nil {
		return "", err
	}
	if s, _ := c["email"].(string); s != "" {
		return s, nil
	}
	if m, ok := c["custom"].(map[string]any); ok {
		if s, _ := m["email"].(string); s != "" {
			return s, nil
		}
	}
	return "", nil
}

// ───────────────────── Builder público ─────────────────────

func BuildEmailFlowHandlers(
	ctx context.Context,
	cfg *config.Config,
	c *app.Container,
	refreshTTL time.Duration,
) (verifyStart http.Handler, verifyConfirm http.Handler, forgot http.Handler, reset http.Handler, efHandler *EmailFlowsHandler, cleanup func(), err error) {

	// Mailer initialization removed in favor of SenderProvider
	// mailer := email.NewSMTPSender(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.From, cfg.SMTP.Username, cfg.SMTP.Password)
	// mailer.TLSMode = cfg.SMTP.TLS
	// mailer.InsecureSkipVerify = cfg.SMTP.InsecureSkipVerify
	// log.Printf(`{"level":"info","msg":"email_wiring_mailer","host":"%s","port":%d,"from":"%s","tls_mode":"%s"}`, cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.From, cfg.SMTP.TLS)

	// Templates
	tmpls, err := email.LoadTemplates(cfg.Email.TemplatesDir)
	if err != nil {
		log.Printf(`{"level":"error","msg":"email_wiring_templates_err","dir":"%s","err":"%v"}`, cfg.Email.TemplatesDir, err)
		return nil, nil, nil, nil, nil, func() {}, err
	}
	log.Printf(`{"level":"info","msg":"email_wiring_templates_ok","dir":"%s"}`, cfg.Email.TemplatesDir)

	// Password policy
	pol := password.Policy{
		MinLength:     cfg.Security.PasswordPolicy.MinLength,
		RequireUpper:  cfg.Security.PasswordPolicy.RequireUpper,
		RequireLower:  cfg.Security.PasswordPolicy.RequireLower,
		RequireDigit:  cfg.Security.PasswordPolicy.RequireDigit,
		RequireSymbol: cfg.Security.PasswordPolicy.RequireSymbol,
	}
	log.Printf(`{"level":"info","msg":"email_wiring_policy","min_len":%d,"upper":%t,"lower":%t,"digit":%t,"symbol":%t}`,
		pol.MinLength, pol.RequireUpper, pol.RequireLower, pol.RequireDigit, pol.RequireSymbol)

	// Limiter (opcional, Redis)
	var flowsLimiter RateLimiter
	if cfg.Rate.Enabled && strings.EqualFold(cfg.Cache.Kind, "redis") {
		rc := rdb.NewClient(&rdb.Options{
			Addr: cfg.Cache.Redis.Addr,
			DB:   cfg.Cache.Redis.DB,
		})
		win, _ := time.ParseDuration(cfg.Rate.Window)
		if win == 0 {
			win = time.Minute
		}
		rl := rate.NewRedisLimiter(rc, cfg.Cache.Redis.Prefix+"rl:", cfg.Rate.MaxRequests, win)
		flowsLimiter = flowsLimiterAdapter{inner: rl}
		log.Printf(`{"level":"info","msg":"email_wiring_rate","enabled":true,"window":"%s","max":%d}`, win, cfg.Rate.MaxRequests)
	} else {
		log.Printf(`{"level":"info","msg":"email_wiring_rate","enabled":false}`)
	}

	// Stores: reusar pool existente si es posible para evitar "too many connections"
	var dbOps store.DBOps
	if pgStore, ok := c.Store.(interface{ Pool() *pgxpool.Pool }); ok {
		dbOps = pgStore.Pool()
		log.Printf(`{"level":"info","msg":"email_wiring_reuse_pool"}`)
		cleanup = func() {} // Pool managed by main store
	} else {
		// Fallback: connect manually (should not happen if hasGlobalDB is true and driver is PG)
		log.Printf(`{"level":"warn","msg":"email_wiring_new_conn","reason":"store_not_pool_compatible"}`)
		pgxConn, err := pgx.Connect(ctx, cfg.Storage.DSN)
		if err != nil {
			log.Printf(`{"level":"error","msg":"email_wiring_pgx_connect_err","err":"%v"}`, err)
			return nil, nil, nil, nil, nil, func() {}, err
		}
		dbOps = pgxConn
		cleanup = func() { _ = pgxConn.Close(ctx) }
	}
	log.Printf(`{"level":"info","msg":"email_wiring_pgx_ok"}`)

	ts := store.NewTokenStore(dbOps)
	us := &store.UserStore{DB: dbOps} // sin constructor

	// Adapters
	rvalidator := redirectValidatorAdapter{repo: c.Store}
	tissuer := tokenIssuerAdapter{c: c, refreshTTL: refreshTTL}
	authProv := currentUserProviderAdapter{issuer: c.Issuer}
	ef := &EmailFlowsHandler{
		Tokens:         ts,
		Users:          us,
		SenderProvider: c.SenderProvider,
		Tmpl:           tmpls,
		Policy:         pol,
		Redirect:       rvalidator,
		Issuer:         tissuer,
		Auth:           authProv,
		Limiter:        flowsLimiter,
		BaseURL:        cfg.Email.BaseURL,
		VerifyTTL:      cfg.Auth.Verify.TTL,
		ResetTTL:       cfg.Auth.Reset.TTL,
		AutoLoginReset: cfg.Auth.Reset.AutoLogin,
		DebugEchoLinks: cfg.Email.DebugEchoLinks,
		BlacklistPath:  cfg.Security.PasswordBlacklistPath,
		TenantMgr:      c.TenantSQLManager,
		Provider:       cpctx.Provider,
	}
	if strings.TrimSpace(cfg.Security.PasswordBlacklistPath) != "" {
		log.Printf(`{"level":"info","msg":"email_wiring_blacklist","path":"%s"}`, cfg.Security.PasswordBlacklistPath)
	} else {
		log.Printf(`{"level":"info","msg":"email_wiring_blacklist","path":"(empty)"}`)
	}
	log.Printf(`{"level":"info","msg":"email_wiring_ready","base_url":"%s","verify_ttl":"%s","reset_ttl":"%s","autologin":%t,"debug_links":%t}`,
		ef.BaseURL, ef.VerifyTTL, ef.ResetTTL, ef.AutoLoginReset, ef.DebugEchoLinks)

	verifyStart = http.HandlerFunc(ef.verifyEmailStart)
	verifyConfirm = http.HandlerFunc(ef.verifyEmailConfirm)
	forgot = http.HandlerFunc(ef.forgot)
	reset = http.HandlerFunc(ef.reset)

	return verifyStart, verifyConfirm, forgot, reset, ef, cleanup, nil
}
