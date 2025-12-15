/*
Email Flows Handler (email_flows.go)
───────────────────────────────────────────────────────────────────────────────
Qué es esto
- Este archivo SÍ implementa handlers HTTP reales para flujos de email “account recovery”:
  - Verificación de email (start + confirm)
  - Forgot password (generar token + mandar mail)
  - Reset password (consumir token + setear nueva password + revocar refresh)
- También define interfaces “puerto” (RedirectValidator, TokenIssuer, CurrentUserProvider, RateLimiter)
  para desacoplar el handler del resto del sistema (DB/issuer/ratelimit/auth).

Endpoints registrados (chi)
- POST  /v1/auth/verify-email/start   -> `verifyEmailStart`
- GET   /v1/auth/verify-email         -> `verifyEmailConfirm`
- POST  /v1/auth/forgot               -> `forgot`
- POST  /v1/auth/reset                -> `reset`
(Registrados en `Register(r chi.Router)`)

Flujo general (Template Method mental)
En todos los endpoints el patrón se repite:
1) Decode JSON / query params
2) Resolve tenant (UUID o slug) + validaciones mínimas
3) (Opcional) “gating” por tenant DB (si existe TenantMgr)
4) Rate limit (si hay Limiter)
5) Abrir/usar stores correctos (tenant store preferido, fallback global)
6) Token flow (crear o consumir token)
7) Side-effect: enviar email / setear verified / update password / revocar refresh
8) Respuesta HTTP (204/200/302 o error JSON)

Dependencias y roles (qué usa y para qué)
- `store.TokenStore` (h.Tokens): crea/consume tokens de verificación y reset.
- `store.UserStore` (h.Users): lookup de usuario por email, set verified, update password hash, revocar refresh.
- `email.Templates` (h.Tmpl): templates base (VerifyHTML/TXT, ResetHTML/TXT).
- `email.SenderProvider` (h.SenderProvider): resuelve sender por tenant (multi-tenant SMTP/provider).
- `password.Policy` (h.Policy): valida fuerza de password en reset.
- `RedirectValidator` (h.Redirect): valida redirect_uri permitido por tenant/client.
- `TokenIssuer` (h.Issuer): emite access/refresh post-reset si AutoLoginReset está habilitado.
- `CurrentUserProvider` (h.Auth): intenta extraer usuario actual (Bearer) para verify-start autenticado.
- `RateLimiter` (h.Limiter): rate limit por key (IP/email/client) y agrega Retry-After.
- `controlplane.ControlPlane` (h.Provider): lookup tenant por slug/ID, y override templates / URLs por client.
- `tenantsql.Manager` (h.TenantMgr): Phase 4.1, “gating” y selección explícita de DB por tenant (evita mezclar tenants).
- `httpx` + `helpers`: errores de tenant DB (WriteTenantDBMissing/WriteTenantDBError) y OpenTenantRepo (gating).

OJO: mezcla de estilos de error
- Hay un `writeErr(...)` local que escribe OAuth-like (`error`, `error_description`).
- En algunos casos se usa `httpx.WriteTenantDBMissing/Error` (otro formato).
  ⇒ esto está bueno marcarlo para V2: respuesta consistente.

───────────────────────────────────────────────────────────────────────────────
Verify Email: POST /v1/auth/verify-email/start (verifyEmailStart)
Qué hace
- Inicia flujo de verificación (o reenvío):
  - Si viene autenticado (Bearer): usa el usuario actual y su email.
  - Si NO viene autenticado: requiere `email` en body y hace lookup (sin enumeración: si no existe, 204).

Input JSON (DTO local)
- `verifyStartIn { tenant_id, client_id, email?, redirect_uri? }`
- tenant_id puede ser UUID o slug (se resuelve de ambas formas).

Paso a paso
1) Decode JSON.
2) Resolver tenant UUID:
   - si `tenant_id` parsea como UUID => OK
   - si no, usa `Provider.GetTenantBySlug` y parsea `tenant.ID`.
3) Validar: tenantID != nil y client_id != "".
4) Gating por tenant DB (Phase 4.1):
   - si hay TenantMgr: `helpers.OpenTenantRepo(ctx, TenantMgr, tenantID.String())`
   - si falta DB => 501 (WriteTenantDBMissing) o 500 (WriteTenantDBError)
5) Rate limiting:
   - `verify_start:<ip>` (si hay Limiter)
6) Determinar “modo”:
   - Autenticado: `Auth.CurrentUserID`, luego `Auth.CurrentUserEmail` (fallback: Users.GetEmailByID)
   - No autenticado: requiere `email`, rate limit `verify_resend:<ip>`, lookup por email en UserStore tenant-aware.
     - Si no existe: 204 (anti-enumeración)
7) Validar redirect_uri si viene:
   - `Redirect.ValidateRedirectURI(tenantID, clientID, redirect)`
8) Delegar envío real:
   - `SendVerificationEmail(ctx, rid, tenantID, userID, email, redirect, clientID)`
9) Respuesta: 204 No Content.

Notas / invariantes
- Anti-enumeración: el “resend” devuelve 204 cuando no encuentra usuario.
- redirect_uri siempre validada por tenant+client para evitar open-redirect.

Cosas a marcar
- En modo autenticado: si `Auth.CurrentUserEmail` falla, hace fallback a `h.Users.GetEmailByID` (pero OJO: ahí usa el store global,
  no el userStore tenant-aware resuelto arriba). Esto puede ser bug multi-tenant (mezcla de DB) o deuda técnica.
- “Gating” abre repo sólo para chequear DB, pero no reutiliza repo/store resultante (se vuelve a pedir pool más abajo).

───────────────────────────────────────────────────────────────────────────────
SendVerificationEmail (método reusable)
Qué hace
- Crea token de verificación (per-tenant si posible), arma link, renderiza template (con overrides por tenant),
  resuelve sender por tenant y manda email.
- Está expuesto porque lo usa Register handler (para mandar mail sin duplicar lógica).

Paso a paso
1) Resolver TokenStore:
   - default `h.Tokens`
   - si `TenantMgr` permite: `tenantDB.Pool()` => `store.NewTokenStore(pool)`
2) `CreateEmailVerification(tenantID, userID, email, VerifyTTL, ipPtr, uaPtr)`
   - IP/UA están en nil (porque no hay request acá), deuda: se podrían pasar desde verifyEmailStart.
3) Armar link con `buildLink("/v1/auth/verify-email", token, redirect, clientID, tenantID)`
4) Buscar tenant por ID para:
   - nombre y overrides de templates (`tenant.Settings.Mailing.Templates`)
5) Render:
   - `renderVerify` (usa override TemplateVerify si existe, sino templates base)
6) Sender:
   - `SenderProvider.GetSender(ctx, tenantID)`
7) Send:
   - `sender.Send(to, "Verificá tu email", html, text)`
   - Manejo “soft”: si falla el send, log + `return nil` (NO rompe el flujo).
     Esto es intencional para no romper register/verify start aunque SMTP esté caído.

Riesgo importante
- “Soft fail” en envío: deja usuario sin mail. Está bien para register, pero para “verify start” quizá sería mejor retornar error
  y avisar al cliente. (No decidir ahora, sólo marcarlo).

───────────────────────────────────────────────────────────────────────────────
Verify Email: GET /v1/auth/verify-email (verifyEmailConfirm)
Qué hace
- Consume token de verificación y marca `EmailVerified=true`, luego redirige a redirect_uri con `status=verified`
  o devuelve JSON `{status:"verified"}`.

Input (query params)
- token (obligatorio)
- redirect_uri (opcional)
- client_id (opcional)
- tenant_id (opcional pero clave para seleccionar DB correcta)

Paso a paso
1) Leer query params y validar `token`.
2) Selección de stores por tenant_id:
   - default: `h.Tokens` + `h.Users`
   - si viene tenant_id y hay TenantMgr: usa pool tenant => TokenStore + UserStore tenant-specific.
3) `UseEmailVerification(token)` => retorna (tenantID?, userID).
4) Resolver redirect default si redirect vacío:
   - con Provider: GetTenantByID(tenantID) => slug
   - GetClient(slug, clientID) y elige:
     - RedirectURIs[0] si existe, sino VerifyEmailURL
5) Validar redirect si existe: `Redirect.ValidateRedirectURI(tenantID, clientID, redirect)`
6) `userStore.SetEmailVerified(userID)`
7) Respuesta:
   - si redirect != "" -> 302 Found a `redirect?status=verified`
   - si no -> JSON.

Cosas a marcar
- `tenantID` se parsea sólo si el query param viene como UUID. Si viene slug, no se resuelve (a diferencia de verifyStart/forgot/reset).
  En V2 conviene unificar “resolver tenant id/slug” para todos.
- Mezcla de formatos de error: `writeErr` vs `httpx.WriteTenantDB*`.

───────────────────────────────────────────────────────────────────────────────
Forgot Password: POST /v1/auth/forgot (forgot)
Qué hace
- Flujo “no enumerar usuarios”: siempre responde `{status:"ok"}` aunque el email no exista.
- Si existe: genera token reset + manda mail con link de reset (custom per client si está configurado).

Input JSON (forgotIn)
- tenant_id (UUID o slug)
- client_id
- email
- redirect_uri (opcional)

Paso a paso
1) Decode JSON.
2) Resolver tenant UUID (UUID o Provider.GetTenantBySlug).
3) (Opcional) gating DB: OpenTenantRepo(tenantID.String()) sólo para validar existencia de DB.
4) Construir stores tenant-aware:
   - si TenantMgr: GetPG(ctx, in.TenantID) (OJO usa in.TenantID, puede ser slug) => UserStore + TokenStore del tenant.
   - else: usa stores globales del handler.
5) Rate limit: key `forgot:<tenantUUID>:<emailLower>`
6) Lookup user por email (en tenant):
   - si no existe: devolver OK igual
7) Validar redirect_uri (si no valida, se limpia y se sigue).
8) Crear token: `CreatePasswordReset(tenantID, uid, email, ResetTTL, ipPtr, uaPtr)`
   - IP/UA se capturan del request.
9) Armar link:
   - si el client tiene `ResetPasswordURL`, lo usa y le agrega `token=...`
   - si no: backend `/v1/auth/reset` con query params (token, redirect, client_id, tenant_id)
10) Render template reset (override TemplateReset si existe)
11) SenderProvider.GetSender(tenantID) y enviar mail.
12) Respuesta final: JSON `{status:"ok"}`.

Riesgos / detalles
- El “custom reset URL” usa solo token y no agrega tenant/client; está perfecto si esa URL ya sabe el tenant por dominio/ruta.
  Si no, podría perder contexto.
- `tenant, _ := h.Provider.GetTenantBySlug(ctx, in.TenantID)` asume que `in.TenantID` es slug, pero podría ser UUID.
  (En ese caso, templates/tenantDisplayName podrían fallar o quedar pobres). Esto es deuda clara para V2:
  resolver slug de forma robusta (uuid -> tenant -> slug).

───────────────────────────────────────────────────────────────────────────────
Reset Password: POST /v1/auth/reset (reset)
Qué hace
- Consume token de reset, valida policy/blacklist, hashea password, actualiza password hash,
  revoca refresh tokens y opcionalmente hace auto-login devolviendo tokens.

Input JSON (resetIn)
- tenant_id (UUID o slug)
- client_id
- token
- new_password

Paso a paso
1) Decode JSON.
2) Resolver tenant UUID (UUID o Provider.GetTenantBySlug).
3) Crear stores tenant-aware (UserStore + TokenStore) si TenantMgr existe.
4) Rate limit: `reset:<ip>`
5) Validar password con Policy.Validate -> si falla, 400 weak_password.
6) Consumir token: `UsePasswordReset(token)` => userID
7) Blacklist opcional: `password.GetCachedBlacklist(BlacklistPath).Contains(new_password)`
8) Hash: `password.Hash(password.Default, new_password)`
9) Update password: `userStore.UpdatePasswordHash(userID, hash)`
10) Revocar sesiones: `userStore.RevokeAllRefreshTokens(userID)` (invalidate all devices)
11) Si AutoLoginReset:
   - `Issuer.IssueTokens(w,r, tenantID, clientID, userID)` (escribe JSON de tokens tipo login)
   - Headers no-store/no-cache
12) Si no: 204 No Content.

Invariantes de seguridad
- Tokens de reset/verify son de un solo uso (UsePasswordReset/UseEmailVerification).
- Rate limit para evitar abuso (enumeración, bombardeo de mails, brute force de reset).
- Validación de password + blacklist (política).
- Revocación de refresh tokens post-reset (correcto: corta sesiones viejas).
- Redirect URI validada siempre con tenant+client.

───────────────────────────────────────────────────────────────────────────────
Helpers internos (no HTTP)
- `writeErr`: emite JSON estilo OAuth (`error`, `error_description`).
- `rlOr429`: aplica rate limit y setea Retry-After; devuelve bool “se cortó” (early return).
- `renderVerify`, `renderReset`, `renderOverride`: render templates base + overrides por tenant (Template pattern).
- `sanitizeUA`, `clientIPOrEmpty`, `strPtrOrNil`: utilidades locales.

Patrones detectados (GoF / arquitectura)
- Adapter / Ports & Adapters:
  - Las interfaces RedirectValidator / TokenIssuer / CurrentUserProvider / RateLimiter son “ports”;
    el wiring (email_flows_wiring.go) inyecta adapters concretos.
- Strategy:
  - Validación de redirect (RedirectValidator) y rate limit (RateLimiter) cambian sin tocar el handler.
- Template Method:
  - Flujo repetido: parse -> validate -> resolve -> store -> action -> respond.
- Facade:
  - EmailFlowsHandler junta varias dependencias y expone endpoints simples.
- (Concurrencia) No hay goroutines acá. Todo es sync.

Cosas no usadas / legacy / riesgos (marcar)
- `context`, `tenantsql.ErrNoDBForTenant` importado en este archivo? (Acá se importa `tenantsql` pero no se usa ErrNoDBForTenant; solo `tenantsql.Manager`).
- `httpx` se usa sólo para errores de tenant DB; el resto usa writeErr. Inconsistencia intencional/accidental.
- Mezcla de “tenant_id slug vs UUID” en distintas llamadas al Provider:
  - a veces se asume slug (GetTenantBySlug) y a veces ID (GetTenantByID).
  - en forgot() se usa GetTenantBySlug(in.TenantID) aunque puede ser UUID.
- En verifyEmailStart() el fallback `h.Users.GetEmailByID` puede estar yendo al store global aunque el tenant DB sea otro.

Ideas para V2 (sin decidir nada, solo guía de “desarme” en capas)
1) DTOs (entrada/salida)
   - verifyStartIn, forgotIn, resetIn como DTOs en `dto/` (y validar campos ahí).
   - Respuestas: unificar formato (usar httpx.WriteError o un ResponseWriter común).

2) Controller (HTTP)
   - Controller por endpoint que solo haga:
     - parse + validate
     - armar “command” para service
     - mapear errores a HTTP
   - Sacar loggers y writeErr disperso a helpers comunes.

3) Services (casos de uso)
   - `EmailVerificationService`:
     - Start(tenant, client, authContext, email, redirect) -> side effect send email
     - Confirm(token, tenantHint, client, redirect) -> mark verified + redirect resolver
   - `PasswordResetService`:
     - Forgot(tenant, client, email, redirect) -> create token + send mail
     - Reset(tenant, client, token, newPassword) -> consume token + update password + revoke + optional issue tokens

4) Clients / Integraciones
   - `ControlPlaneClient` (Provider) para:
     - ResolveTenant( slug|uuid ) -> {uuid, slug, displayName}
     - ResolveClient(tenantSlug, clientID) -> {redirectURIs, resetURL, verifyURL, templates...}
   - `Mailer/SenderClient` wrapper sobre SenderProvider (con retry/diagnostics si querés).

5) Repo / Store
   - Encapsular “tenant store selection”:
     - `TenantStoreResolver` que devuelva `UserStore` + `TokenStore` correctos por tenant (uuid/slug)
     - Evitar duplicar GetPG + NewTokenStore en cada endpoint.

6) Patrones a aplicar en el refactor
   - Strategy para “LinkBuilder” (backend link vs client custom URL) y para “ErrorResponder”.
   - Facade/Service para “TenantContextResolver” (te devuelve tenantUUID, tenantSlug y te valida DB).
   - Chain of Responsibility (middlewares) para:
     - rate limit
     - tenant gating
     - parse de tenant/client
     - logging de request_id
   - Decorator para Sender (diagnóstico + retry soft para temporales).

Resumen
- Este archivo es EL núcleo de “email flows”: verify + forgot/reset, con multi-tenant y overrides por control-plane.
- Tiene buenas ideas (ports/adapters, rate limit, anti-enumeración, revocación post-reset),
  pero mezcla responsabilidades (HTTP + store selection + templating + mail + redirect validation).
- Para V2: separar “resolver tenant/client + stores” y unificar formato de error/redirect/slug-vs-uuid para que no se te escape un bug multi-tenant.
*/

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	texttpl "text/template"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/controlplane"
	"github.com/dropDatabas3/hellojohn/internal/email"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/http/v1/helpers"
	"github.com/dropDatabas3/hellojohn/internal/infra/tenantsql"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	"github.com/dropDatabas3/hellojohn/internal/store"
)

// --- Interfaces de integración con tu core ---

type RedirectValidator interface {
	ValidateRedirectURI(tenantID uuid.UUID, clientID string, redirectURI string) bool
}

type TokenIssuer interface {
	IssueTokens(w http.ResponseWriter, r *http.Request, tenantID uuid.UUID, clientID string, userID uuid.UUID) error
}

type CurrentUserProvider interface {
	CurrentUserID(r *http.Request) (uuid.UUID, error)
	CurrentTenantID(r *http.Request) (uuid.UUID, error)
	CurrentUserEmail(r *http.Request) (string, error)
}

type RateLimiter interface {
	Allow(key string) (allowed bool, retryAfter time.Duration)
}

// --- Handler ---

type EmailFlowsHandler struct {
	Tokens *store.TokenStore
	Users  *store.UserStore
	// Mailer        email.Sender // Removed in favor of provider
	SenderProvider email.SenderProvider
	Tmpl           *email.Templates
	Policy         password.Policy
	Redirect       RedirectValidator
	Issuer         TokenIssuer
	Auth           CurrentUserProvider
	Limiter        RateLimiter
	BlacklistPath  string
	Provider       controlplane.ControlPlane

	// Phase 4.1: per-tenant DB gating for email flows
	TenantMgr *tenantsql.Manager

	BaseURL        string
	VerifyTTL      time.Duration
	ResetTTL       time.Duration
	AutoLoginReset bool

	DebugEchoLinks bool // expone links en headers para DX/testing
}

func (h *EmailFlowsHandler) Register(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Post("/v1/auth/verify-email/start", h.verifyEmailStart)
		r.Get("/v1/auth/verify-email", h.verifyEmailConfirm)

		r.Post("/v1/auth/forgot", h.forgot)
		r.Post("/v1/auth/reset", h.reset)
	})
}

func writeErr(w http.ResponseWriter, code, desc string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": code, "error_description": desc,
	})
}

func (h *EmailFlowsHandler) rlOr429(w http.ResponseWriter, key string) bool {
	if h.Limiter == nil {
		return false
	}
	allowed, retry := h.Limiter.Allow(key)
	if allowed {
		return false
	}
	if retry > 0 {
		secs := int(math.Ceil(retry.Seconds()))
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
	}
	writeErr(w, "rate_limited", "Too many requests", http.StatusTooManyRequests)
	return true
}

// --- Verify Email ---

type verifyStartIn struct {
	TenantID string `json:"tenant_id"`
	ClientID string `json:"client_id"`
	Email    string `json:"email"`
	Redirect string `json:"redirect_uri,omitempty"`
}

func (h *EmailFlowsHandler) verifyEmailStart(w http.ResponseWriter, r *http.Request) {
	rid := w.Header().Get("X-Request-ID")

	var in verifyStartIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		log.Printf(`{"level":"warn","msg":"verify_start_decode","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_json", "Malformed body", http.StatusBadRequest)
		return
	}

	// Resolve tenant ID by UUID or slug
	var tenantID uuid.UUID
	if parsed, err := uuid.Parse(in.TenantID); err == nil {
		tenantID = parsed
	} else if h.Provider != nil {
		if tenant, err := h.Provider.GetTenantBySlug(r.Context(), in.TenantID); err == nil && tenant != nil {
			if parsed, err := uuid.Parse(tenant.ID); err == nil {
				tenantID = parsed
			}
		}
	}

	log.Printf(`{"level":"info","msg":"verify_start_begin","request_id":"%s","tenant_id":"%s","client_id":"%s","redirect":"%s"}`, rid, tenantID, in.ClientID, in.Redirect)

	if tenantID == uuid.Nil || in.ClientID == "" {
		log.Printf(`{"level":"warn","msg":"verify_start_missing_fields","request_id":"%s"}`, rid)
		writeErr(w, "missing_fields", "tenant_id and client_id required", http.StatusBadRequest)
		return
	}

	// Gate by tenant DB availability (Phase 4.1). If TenantMgr is present, try opening repo.
	if h.TenantMgr != nil {
		if _, err := helpers.OpenTenantRepo(r.Context(), h.TenantMgr, tenantID.String()); err != nil {
			// Mirror 501/500 semantics
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		}
	}

	if h.rlOr429(w, "verify_start:"+clientIPOrEmpty(r)) {
		log.Printf(`{"level":"warn","msg":"verify_start_rate_limited","request_id":"%s","remote":"%s"}`, rid, clientIPOrEmpty(r))
		return
	}

	userID, err := h.Auth.CurrentUserID(r)
	var emailStr string

	// Resolve TokenStore (prefer Tenant DB) explicitly for this request
	var userStore = h.Users // fallback default

	if h.TenantMgr != nil {
		if tenantDB, err := h.TenantMgr.GetPG(r.Context(), tenantID.String()); err == nil {
			userStore = &store.UserStore{DB: tenantDB.Pool()}
		} else {
			log.Printf(`{"level":"warn","msg":"verify_start_tenant_db_err","err":"%v"}`, err)
		}
	}

	if err == nil {
		// Authenticated request
		emailStr, _ = h.Auth.CurrentUserEmail(r)
		if emailStr == "" {
			if e, err := h.Users.GetEmailByID(r.Context(), userID); err == nil && e != "" {
				emailStr = e
			}
		}
	} else {
		// Unauthenticated request (resend flow)
		if in.Email == "" {
			writeErr(w, "login_required", "Bearer or email required", http.StatusUnauthorized)
			return
		}

		// Rate limit by IP for public endpoint
		if h.rlOr429(w, "verify_resend:"+clientIPOrEmpty(r)) {
			return
		}

		uid, ok, err := userStore.LookupUserIDByEmail(r.Context(), tenantID, in.Email)
		if err != nil {
			log.Printf(`{"level":"error","msg":"verify_lookup_error","request_id":"%s","err":"%v"}`, rid, err)
			writeErr(w, "server_error", "lookup failed", http.StatusInternalServerError)
			return
		}
		if !ok {
			// User not found. Return 204 to avoid enumeration.
			log.Printf(`{"level":"info","msg":"verify_user_not_found","request_id":"%s","email":"%s"}`, rid, in.Email)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		userID = uid
		emailStr = in.Email
	}

	if emailStr == "" {
		writeErr(w, "server_error", "user email not found", http.StatusInternalServerError)
		return
	}

	if in.Redirect != "" && !h.Redirect.ValidateRedirectURI(tenantID, in.ClientID, in.Redirect) {
		log.Printf(`{"level":"warn","msg":"verify_start_invalid_redirect","request_id":"%s","redirect":"%s"}`, rid, in.Redirect)
		writeErr(w, "invalid_redirect_uri", "redirect_uri not allowed", http.StatusBadRequest)
		return
	}

	// Delegate to shared method
	if err := h.SendVerificationEmail(r.Context(), rid, tenantID, userID, emailStr, in.Redirect, in.ClientID); err != nil {
		// SendVerificationEmail logs errors. We just map to HTTP status if needed or return 500.
		// Since validation errors (like redirect) happen inside, we might want to move validation OUT.
		// For now, assume global error is 500.
		writeErr(w, "server_error", "could not send verification email", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SendVerificationEmail sends the validation email. Exposed for Register handler.
func (h *EmailFlowsHandler) SendVerificationEmail(ctx context.Context, rid string, tenantID, userID uuid.UUID, emailStr, redirect, clientID string) error {
	// IP/UA handling
	var ipPtr, uaPtr *string
	// extracting from context is hard without request, so we pass defaults or nil if internal

	// Resolve TokenStore (prefer Tenant DB) explicitly
	var tokenStore = h.Tokens // fallback default
	if h.TenantMgr != nil {
		if tenantDB, err := h.TenantMgr.GetPG(ctx, tenantID.String()); err == nil {
			tokenStore = store.NewTokenStore(tenantDB.Pool())
		} else {
			log.Printf(`{"level":"warn","msg":"verify_send_tenant_db_err","err":"%v"}`, err)
		}
	}

	log.Printf(`{"level":"info","msg":"verify_send_create_token_try","request_id":"%s","tenant_id":"%s","user_id":"%s","email":"%s","ttl_sec":%d}`,
		rid, tenantID, userID, emailStr, int(h.VerifyTTL.Seconds()))

	// Use local tokenStore
	pt, err := tokenStore.CreateEmailVerification(ctx, tenantID, userID, emailStr, h.VerifyTTL, ipPtr, uaPtr)
	if err != nil {
		log.Printf(`{"level":"error","msg":"verify_send_create_token_err","request_id":"%s","err":"%v"}`, rid, err)
		return err
	}
	log.Printf(`{"level":"info","msg":"verify_send_token_ok","request_id":"%s"}`, rid)

	link := h.buildLink("/v1/auth/verify-email", pt, redirect, clientID, tenantID.String())
	if h.DebugEchoLinks {
		log.Printf(`{"level":"debug","msg":"verify_send_link","request_id":"%s","link":"%s"}`, rid, link)
	}

	// Fetch tenant settings for templates
	tenant, err := h.Provider.GetTenantByID(ctx, tenantID.String())
	if err != nil {
		log.Printf(`{"level":"warn","msg":"verify_send_tenant_fetch_err","err":"%v"}`, err)
	}

	tenantName := tenantID.String()
	if tenant != nil {
		tenantName = tenant.Name
	}

	htmlBody, textBody, err := renderVerify(h.Tmpl, tenant, email.VerifyVars{
		UserEmail: emailStr, Tenant: tenantName, Link: link, TTL: h.VerifyTTL.String(),
	})
	if err != nil {
		log.Printf(`{"level":"error","msg":"verify_send_template_err","request_id":"%s","err":"%v"}`, rid, err)
		return err
	}

	// Resolve sender
	sender, err := h.SenderProvider.GetSender(ctx, tenantID)
	if err != nil {
		log.Printf(`{"level":"error","msg":"verify_send_sender_err","request_id":"%s","err":"%v"}`, rid, err)
		return err
	}

	if err := sender.Send(emailStr, "Verificá tu email", htmlBody, textBody); err != nil {
		diag := email.DiagnoseSMTP(err)
		lvl := "error"
		if diag.Temporary {
			lvl = "warn"
		}
		log.Printf(`{"level":"%s","msg":"verify_send_mail_err","request_id":"%s","to":"%s","code":"%s","temporary":%t,"retry_after_sec":%d,"err":"%v"}`,
			lvl, rid, emailStr, diag.Code, diag.Temporary, int(diag.RetryAfter.Seconds()), err)
		// Return error? VerifyStart consumed it without error.
		// Register might want to know? Unlikely to rollback register.
		// Just log and return nil (soft failure) or error?
		// Register: "registro exitoso... debe ejecutar flujo". If mail fails, user created but not verified.
		// We return nil to avoid breaking flow?
		return nil
	}
	log.Printf(`{"level":"info","msg":"verify_send_mail_ok","request_id":"%s","to":"%s"}`, rid, emailStr)
	return nil
}

func (h *EmailFlowsHandler) verifyEmailConfirm(w http.ResponseWriter, r *http.Request) {
	rid := w.Header().Get("X-Request-ID")

	q := r.URL.Query()
	token := q.Get("token")
	redirect := q.Get("redirect_uri")
	clientID := q.Get("client_id")
	tenantIDParam := q.Get("tenant_id")
	log.Printf(`{"level":"info","msg":"verify_confirm_begin","request_id":"%s","redirect":"%s","client_id":"%s","tenant_id":"%s"}`, rid, redirect, clientID, tenantIDParam)

	if token == "" {
		log.Printf(`{"level":"warn","msg":"verify_confirm_missing_token","request_id":"%s"}`, rid)
		writeErr(w, "invalid_token", "token required", http.StatusBadRequest)
		return
	}

	// Determine which TokenStore and UserStore to use based on tenant_id
	var tokenStore *store.TokenStore = h.Tokens
	var userStore *store.UserStore = h.Users
	var tenantID uuid.UUID
	if tenantIDParam != "" {
		if parsed, err := uuid.Parse(tenantIDParam); err == nil {
			tenantID = parsed
		}
	}

	if tenantIDParam != "" && h.TenantMgr != nil {
		// Use tenant-specific stores
		tenantDB, err := h.TenantMgr.GetPG(r.Context(), tenantIDParam)
		if err == nil {
			tokenStore = store.NewTokenStore(tenantDB.Pool())
			userStore = &store.UserStore{DB: tenantDB.Pool()}
			log.Printf(`{"level":"debug","msg":"verify_using_tenant_db","request_id":"%s","tenant_id":"%s"}`, rid, tenantIDParam)
		} else {
			log.Printf(`{"level":"warn","msg":"verify_tenant_db_err","request_id":"%s","err":"%v"}`, rid, err)
		}
	}

	_, userID, err := tokenStore.UseEmailVerification(r.Context(), token)
	if err != nil {
		log.Printf(`{"level":"warn","msg":"verify_confirm_token_bad","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_token", "token invalid/expired/used", http.StatusBadRequest)
		return
	}
	log.Printf(`{"level":"info","msg":"verify_confirm_token_ok","request_id":"%s","tenant_id":"%s","user_id":"%s"}`, rid, tenantID, userID)

	// If redirect is empty, try to resolve a default from client config
	if redirect == "" && clientID != "" && h.Provider != nil {
		// We need tenant slug for GetClient. We only have tenantID (UUID).
		// Try to resolve slug from ID or assume we can look it up.
		// Since we already might have fetched tenant for store, reuse or fetch.
		var tenantSlug string
		if t, err := h.Provider.GetTenantByID(r.Context(), tenantID.String()); err == nil && t != nil {
			tenantSlug = t.Slug
		}

		if tenantSlug != "" {
			if c, err := h.Provider.GetClient(r.Context(), tenantSlug, clientID); err == nil && c != nil {
				// Priority: User defined RedirectURIs > VerifyEmailURL
				if len(c.RedirectURIs) > 0 {
					redirect = c.RedirectURIs[0]
				} else if c.VerifyEmailURL != "" {
					redirect = c.VerifyEmailURL
				}
				log.Printf(`{"level":"info","msg":"verify_confirm_resolved_default_redirect","request_id":"%s","redirect":"%s"}`, rid, redirect)
			}
		}
	}

	if redirect != "" && !h.Redirect.ValidateRedirectURI(tenantID, clientID, redirect) {
		log.Printf(`{"level":"warn","msg":"verify_confirm_invalid_redirect","request_id":"%s","redirect":"%s"}`, rid, redirect)
		writeErr(w, "invalid_redirect_uri", "redirect_uri not allowed", http.StatusBadRequest)
		return
	}
	if err := userStore.SetEmailVerified(r.Context(), userID); err != nil {
		log.Printf(`{"level":"error","msg":"verify_confirm_mark_verified_err","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "server_error", "could not mark verified", http.StatusInternalServerError)
		return
	}
	log.Printf(`{"level":"info","msg":"verify_confirm_mark_verified_ok","request_id":"%s"}`, rid)

	if redirect != "" {
		u, _ := url.Parse(redirect)
		qs := u.Query()
		qs.Set("status", "verified")
		// Also add token/email hints if needed? No, just status usually enough.
		u.RawQuery = qs.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "verified"})
}

func (h *EmailFlowsHandler) buildLink(path, token, redirect, clientID, tenantID string) string {
	u, _ := url.Parse(h.BaseURL)
	u.Path = path
	q := u.Query()
	q.Set("token", token)
	if redirect != "" {
		q.Set("redirect_uri", redirect)
	}
	if clientID != "" {
		q.Set("client_id", clientID)
	}
	if tenantID != "" {
		q.Set("tenant_id", tenantID)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// --- Forgot / Reset ---

type forgotIn struct {
	TenantID string `json:"tenant_id"` // Can be UUID or slug
	ClientID string `json:"client_id"`
	Email    string `json:"email"`
	Redirect string `json:"redirect_uri,omitempty"`
}

func (h *EmailFlowsHandler) forgot(w http.ResponseWriter, r *http.Request) {
	rid := w.Header().Get("X-Request-ID")

	var in forgotIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		log.Printf(`{"level":"warn","msg":"forgot_decode","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_json", "Malformed body", http.StatusBadRequest)
		return
	}

	// Resolve tenant ID (can be UUID or slug)
	var tenantID uuid.UUID
	if parsed, err := uuid.Parse(in.TenantID); err == nil {
		tenantID = parsed
	} else if h.Provider != nil {
		// Try to lookup by slug
		if tenant, err := h.Provider.GetTenantBySlug(r.Context(), in.TenantID); err == nil && tenant != nil {
			if parsed, err := uuid.Parse(tenant.ID); err == nil {
				tenantID = parsed
			}
		}
	}

	if h.TenantMgr != nil && tenantID != uuid.Nil {
		if _, err := helpers.OpenTenantRepo(r.Context(), h.TenantMgr, tenantID.String()); err != nil {
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		}
	}
	log.Printf(`{"level":"info","msg":"forgot_begin","request_id":"%s","tenant_id":"%s","client_id":"%s","email":"%s","redirect":"%s"}`, rid, tenantID, in.ClientID, in.Email, in.Redirect)

	if tenantID == uuid.Nil || in.ClientID == "" || in.Email == "" {
		log.Printf(`{"level":"warn","msg":"forgot_missing_fields","request_id":"%s"}`, rid)
		writeErr(w, "missing_fields", "tenant_id, client_id, email required", http.StatusBadRequest)
		return
	}

	// Get tenant DB for user lookup AND token creation
	var userStore *store.UserStore
	var tokenStore = h.Tokens // fallback
	if h.TenantMgr != nil {
		tenantDB, err := h.TenantMgr.GetPG(r.Context(), in.TenantID) // Use slug for lookup
		if err != nil {
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		}
		userStore = &store.UserStore{DB: tenantDB.Pool()}
		tokenStore = store.NewTokenStore(tenantDB.Pool())
	} else {
		userStore = h.Users
	}

	if h.rlOr429(w, "forgot:"+tenantID.String()+":"+strings.ToLower(in.Email)) {
		log.Printf(`{"level":"warn","msg":"forgot_rate_limited","request_id":"%s"}`, rid)
		return
	}

	if uid, ok, err := userStore.LookupUserIDByEmail(r.Context(), tenantID, in.Email); err == nil && ok {
		if in.Redirect != "" && !h.Redirect.ValidateRedirectURI(tenantID, in.ClientID, in.Redirect) {
			log.Printf(`{"level":"warn","msg":"forgot_invalid_redirect","request_id":"%s","redirect":"%s"}`, rid, in.Redirect)
			in.Redirect = ""
		}

		ipStr := clientIPOrEmpty(r)
		uaStr := r.UserAgent()
		ipPtr := strPtrOrNil(ipStr)
		uaPtr := strPtrOrNil(uaStr)

		log.Printf(`{"level":"info","msg":"forgot_create_token_try","request_id":"%s","tenant_id":"%s","user_id":"%s","email":"%s","ttl_sec":%d,"ip":"%s","ua":"%s"}`,
			rid, tenantID, uid, in.Email, int(h.ResetTTL.Seconds()), ipStr, sanitizeUA(uaStr))

		if pt, err := tokenStore.CreatePasswordReset(r.Context(), tenantID, uid, in.Email, h.ResetTTL, ipPtr, uaPtr); err == nil {
			// Fetch tenant for templates and client config
			tenant, _ := h.Provider.GetTenantBySlug(r.Context(), in.TenantID)

			// Lookup client to check for custom reset URL
			var link string
			if h.Provider != nil {
				if client, err := h.Provider.GetClient(r.Context(), tenant.Slug, in.ClientID); err == nil && client != nil && client.ResetPasswordURL != "" {
					// Use custom reset URL from client config
					u, _ := url.Parse(client.ResetPasswordURL)
					q := u.Query()
					q.Set("token", pt)
					u.RawQuery = q.Encode()
					link = u.String()
					log.Printf(`{"level":"info","msg":"forgot_using_custom_reset_url","request_id":"%s","url":"%s"}`, rid, client.ResetPasswordURL)
				}
			}
			if link == "" {
				// Fallback to backend reset endpoint
				link = h.buildLink("/v1/auth/reset", pt, in.Redirect, in.ClientID, tenantID.String())
			}

			if h.DebugEchoLinks {
				log.Printf(`{"level":"debug","msg":"forgot_link","request_id":"%s","link":"%s"}`, rid, link)
			}

			// Get tenant display name for email template
			tenantDisplayName := in.TenantID
			if tenant != nil && tenant.DisplayName != "" {
				tenantDisplayName = tenant.DisplayName
			} else if tenant != nil && tenant.Name != "" {
				tenantDisplayName = tenant.Name
			}

			htmlBody, textBody, _ := renderReset(h.Tmpl, tenant, email.ResetVars{
				UserEmail: in.Email, Tenant: tenantDisplayName, Link: link, TTL: h.ResetTTL.String(),
			})
			// Resolve sender
			sender, err := h.SenderProvider.GetSender(r.Context(), tenantID)
			if err != nil {
				log.Printf(`{"level":"error","msg":"forgot_sender_err","request_id":"%s","err":"%v"}`, rid, err)
				writeErr(w, "email_config_error", "Error de configuración de email", http.StatusInternalServerError)
				return
			}

			if err := sender.Send(in.Email, "Restablecé tu contraseña", htmlBody, textBody); err != nil {
				diag := email.DiagnoseSMTP(err)
				lvl := "error"
				if diag.Temporary {
					lvl = "warn"
				}
				log.Printf(`{"level":"%s","msg":"forgot_mail_send_err","request_id":"%s","to":"%s","code":"%s","temporary":%t,"retry_after_sec":%d,"err":"%v"}`,
					lvl, rid, in.Email, diag.Code, diag.Temporary, int(diag.RetryAfter.Seconds()), err)
				writeErr(w, "email_send_failed", "Error al enviar el email de recuperación", http.StatusServiceUnavailable)
				return
			}
			log.Printf(`{"level":"info","msg":"forgot_mail_send_ok","request_id":"%s","to":"%s"}`, rid, in.Email)

			if h.DebugEchoLinks {
				w.Header().Set("X-Debug-Reset-Link", link)
			}
		} else {
			log.Printf(`{"level":"error","msg":"forgot_create_token_err","request_id":"%s","err":"%v"}`, rid, err)
			writeErr(w, "token_error", "Error interno al procesar la solicitud", http.StatusInternalServerError)
			return
		}
	} else if err != nil {
		log.Printf(`{"level":"error","msg":"forgot_lookup_user_err","request_id":"%s","err":"%v"}`, rid, err)
		// For security, still return OK to not reveal if email exists
	}
	// Note: We return OK even if user not found (security: don't reveal if email exists)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type resetIn struct {
	TenantID    string `json:"tenant_id"` // Can be UUID or slug
	ClientID    string `json:"client_id"`
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (h *EmailFlowsHandler) reset(w http.ResponseWriter, r *http.Request) {
	rid := w.Header().Get("X-Request-ID")

	var in resetIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		log.Printf(`{"level":"warn","msg":"reset_decode","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_json", "Malformed body", http.StatusBadRequest)
		return
	}

	// Resolve tenant ID (can be UUID or slug)
	var tenantID uuid.UUID
	if parsed, err := uuid.Parse(in.TenantID); err == nil {
		tenantID = parsed
	} else if h.Provider != nil {
		// Try to lookup by slug
		if tenant, err := h.Provider.GetTenantBySlug(r.Context(), in.TenantID); err == nil && tenant != nil {
			if parsed, err := uuid.Parse(tenant.ID); err == nil {
				tenantID = parsed
			}
		}
	}

	log.Printf(`{"level":"info","msg":"reset_begin","request_id":"%s","tenant_id":"%s","client_id":"%s"}`, rid, tenantID, in.ClientID)

	if tenantID == uuid.Nil || in.ClientID == "" || in.Token == "" || in.NewPassword == "" {
		log.Printf(`{"level":"warn","msg":"reset_missing_fields","request_id":"%s"}`, rid)
		writeErr(w, "missing_fields", "tenant_id, client_id, token, new_password required", http.StatusBadRequest)
		return
	}

	// Get tenant DB and create per-tenant UserStore AND TokenStore
	var userStore *store.UserStore
	var tokenStore = h.Tokens // fallback
	if h.TenantMgr != nil {
		tenantDB, err := h.TenantMgr.GetPG(r.Context(), in.TenantID)
		if err != nil {
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		}
		userStore = &store.UserStore{DB: tenantDB.Pool()}
		tokenStore = store.NewTokenStore(tenantDB.Pool())
	} else {
		userStore = h.Users
	}

	if h.rlOr429(w, "reset:"+clientIPOrEmpty(r)) {
		log.Printf(`{"level":"warn","msg":"reset_rate_limited","request_id":"%s"}`, rid)
		return
	}

	ok, reasons := h.Policy.Validate(in.NewPassword)
	if !ok {
		log.Printf(`{"level":"warn","msg":"reset_weak_password","request_id":"%s","reasons":"%s"}`, rid, strings.Join(reasons, ","))
		writeErr(w, "weak_password", strings.Join(reasons, ","), http.StatusBadRequest)
		return
	}

	// Use tenant-specific token store. tenantID returned is nil/ignored as check is implicit by using Tenant DB
	_, userID, err := tokenStore.UsePasswordReset(r.Context(), in.Token)
	if err != nil {
		log.Printf(`{"level":"warn","msg":"reset_token_bad","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_token", "token invalid/expired/used", http.StatusBadRequest)
		return
	}
	log.Printf(`{"level":"info","msg":"reset_token_ok","request_id":"%s","tenant_id":"%s","user_id":"%s"}`, rid, tenantID, userID)

	// Blacklist opcional (config/ENV)
	if p := strings.TrimSpace(h.BlacklistPath); p != "" {
		if bl, err := password.GetCachedBlacklist(p); err == nil && bl.Contains(in.NewPassword) {
			writeErr(w, "policy_violation", "password no permitido por política", http.StatusBadRequest)
			return
		}
	}

	phash, err := password.Hash(password.Default, in.NewPassword)
	if err != nil {
		log.Printf(`{"level":"error","msg":"reset_hash_err","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "server_error", "hash error", http.StatusInternalServerError)
		return
	}
	if err := userStore.UpdatePasswordHash(r.Context(), userID, phash); err != nil {
		log.Printf(`{"level":"error","msg":"reset_update_pwd_err","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "server_error", "update password error", http.StatusInternalServerError)
		return
	}
	_ = userStore.RevokeAllRefreshTokens(r.Context(), userID)
	log.Printf(`{"level":"info","msg":"reset_password_updated","request_id":"%s"}`, rid)

	if h.AutoLoginReset && h.Issuer != nil {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		if err := h.Issuer.IssueTokens(w, r, tenantID, in.ClientID, userID); err != nil {
			log.Printf(`{"level":"error","msg":"reset_issue_tokens_err","request_id":"%s","err":"%v"}`, rid, err)
			writeErr(w, "server_error", "issue tokens error", http.StatusInternalServerError)
		} else {
			log.Printf(`{"level":"info","msg":"reset_issue_tokens_ok","request_id":"%s"}`, rid)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// templating shortcuts

func renderVerify(t *email.Templates, tenant *controlplane.Tenant, v email.VerifyVars) (html, text string, err error) {
	// Check overrides
	if tenant != nil && tenant.Settings.Mailing != nil && tenant.Settings.Mailing.Templates != nil {
		if tpl, ok := tenant.Settings.Mailing.Templates[email.TemplateVerify]; ok {
			return renderOverride(tpl, v)
		}
	}

	var hb, tb strings.Builder
	if err = t.VerifyHTML.Execute(&hb, v); err != nil {
		return
	}
	if err = t.VerifyTXT.Execute(&tb, v); err != nil {
		return
	}
	return hb.String(), tb.String(), nil
}

func renderReset(t *email.Templates, tenant *controlplane.Tenant, v email.ResetVars) (html, text string, err error) {
	// Check overrides
	if tenant != nil && tenant.Settings.Mailing != nil && tenant.Settings.Mailing.Templates != nil {
		if tpl, ok := tenant.Settings.Mailing.Templates[email.TemplateReset]; ok {
			return renderOverride(tpl, v)
		}
	}

	var hb, tb strings.Builder
	if err = t.ResetHTML.Execute(&hb, v); err != nil {
		return
	}
	if err = t.ResetTXT.Execute(&tb, v); err != nil {
		return
	}
	return hb.String(), tb.String(), nil
}

func renderOverride(tpl controlplane.EmailTemplate, v any) (html, text string, err error) {
	if tpl.Body == "" {
		return "", "", nil
	}
	t, err := texttpl.New("override").Parse(tpl.Body)
	if err != nil {
		return "", "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, v); err != nil {
		return "", "", err
	}
	// For now return same content for HTML and Text
	return buf.String(), buf.String(), nil
}

// sanitiza user-agent para no romper logs
func sanitizeUA(ua string) string {
	return strings.ReplaceAll(ua, `"`, `'`)
}

// IP desde XFF o RemoteAddr; si falla, string vacío
func clientIPOrEmpty(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return ""
}

// string -> *string (nil si vacío)
func strPtrOrNil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v := s
	return &v
}
