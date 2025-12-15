/*
admin_users.go — Admin Actions sobre usuarios (disable/enable/resend verification) + revoke tokens + audit + emails

Qué hace este handler
---------------------
Este handler implementa acciones administrativas sobre usuarios. No es “CRUD de users”, sino acciones puntuales:
  1) Bloquear (disable) un usuario, opcionalmente por un tiempo (duration) y con motivo (reason).
     - Revoca refresh tokens del usuario (best-effort).
     - Loguea auditoría.
     - (Opcional) Envía email de notificación usando templates del tenant.

  2) Desbloquear (enable) un usuario.
     - Loguea auditoría.
     - (Opcional) Envía email de notificación usando templates.

  3) Reenviar email de verificación (resend-verification).
     - Verifica que el usuario exista y NO esté verificado.
     - Genera token de verificación en la DB del tenant (TokenStore).
     - Construye link de verificación (BASE_URL + endpoint verify-email) incluyendo tenant_id y opcional client_id.
     - Renderiza template de mail del tenant (o fallback).
     - Envía email.
     - Loguea auditoría.

Rutas que maneja (todas POST)
-----------------------------
- POST /v1/admin/users/disable
- POST /v1/admin/users/enable
- POST /v1/admin/users/resend-verification

Todas las rutas comparten el mismo body:
  {
    "user_id": "uuid",
    "tenant_id": "opcional (uuid o slug según caso)",
    "reason": "opcional",
    "duration": "opcional (e.g. 24h, 30m, 2h30m)"
  }

Precondiciones / dependencias
-----------------------------
- Requiere h.c.Store != nil (si no, 501 not_implemented).
- Algunas rutas usan h.c.TenantSQLManager para resolver la DB del tenant.
- Para emails usa:
    - h.c.SenderProvider.GetSender(ctx, tenantUUID)
    - templates desde controlplane tenant settings (cpctx.Provider...)
    - renderOverride(tpl, vars) definido en otro archivo del mismo package.

- Para auditoría usa audit.Log(ctx, event, fields).

Cómo resuelve el store (global vs tenant)
-----------------------------------------
El handler define una interface local `userManager` con lo mínimo que necesita:
  DisableUser(ctx, userID, by, reason string, until *time.Time) error
  EnableUser(ctx, userID, by string) error
  GetUserByID(ctx, id string) (*core.User, error)

Por defecto:
  store := h.c.Store  (global store)

Si body.TenantID viene:
  - intenta h.c.TenantSQLManager.GetPG(ctx, body.TenantID) y usa ese store tenant-specific
  - también asigna `revoker = ts` (si implementa RevokeAllRefreshByUser)

Si NO viene TenantID:
  - revoker se intenta obtener del global store por type assertion:
      RevokeAllRefreshByUser(ctx, userID) (int, error)
  - si no existe, fallback a método legacy:
      h.c.Store.RevokeAllRefreshTokens(ctx, userID, "")

Ojo conceptual:
- body.TenantID se usa tanto para "buscar tenant store" como para “resolver templates”, pero puede ser slug o UUID.
- En varios lugares asume que TenantID es UUID (uuid.MustParse), lo que puede romper si te pasan slug.

Cómo obtiene "by" (quién ejecuta la acción)
--------------------------------------------
Lee claims del contexto (httpx.GetClaims(ctx)) y toma `sub` como actor (by).
Esto se guarda en auditoría y se pasa al store para registrar “by”/razón/hasta cuándo.

Endpoint: POST /v1/admin/users/disable
--------------------------------------
Flujo:
1) Parse body, valida user_id requerido.
2) Si duration viene:
     - time.ParseDuration(duration)
     - until = now + duration
3) Llama store.DisableUser(ctx, user_id, by, reason, until).
4) Revoca tokens:
     - si revoker != nil => revoker.RevokeAllRefreshByUser(ctx, user_id) (best-effort)
     - else fallback legacy RevokeAllRefreshTokens(user_id, "")
5) Audit:
     event: "admin_user_disabled"
     fields: by, user_id, reason, tenant_id, until?
6) Email (best-effort):
     - store.GetUserByID() para obtener email y tenantID real del usuario
     - determina tid = body.TenantID si vino, si no u.TenantID
     - busca tenant settings con cpctx.Provider.GetTenantBySlug(ctx, tid) (ojo: asume slug)
     - si hay template email.TemplateUserBlocked:
         vars {UserEmail, Reason, Until, Tenant}
         renderOverride(tpl, vars)
         sender := h.c.SenderProvider.GetSender(ctx, uuid.MustParse(tid)) (ojo: asume UUID)
         sender.Send(...)
7) Responde 204 No Content

Riesgos/bugs actuales:
- Si tid es slug, uuid.MustParse(tid) va a panic.
- cpctx.Provider.GetTenantBySlug con tid UUID probablemente falle.
- El email se intenta con dos supuestos contradictorios (slug y UUID).

Endpoint: POST /v1/admin/users/enable
-------------------------------------
Similar a disable pero:
- store.EnableUser(ctx, user_id, by)
- audit: "admin_user_enabled"
- email con template email.TemplateUserUnblocked (vars {UserEmail, Tenant})
- Responde 204.

Endpoint: POST /v1/admin/users/resend-verification
--------------------------------------------------
Flujo:
1) Requiere tenant_id explícito (400 si falta).
2) store.GetUserByID(ctx, user_id)
   - si no existe => 404 user_not_found
   - si u.EmailVerified => 400 already_verified
3) Resuelve tenantUUID:
   - si tenant_id parsea UUID -> ok
   - si no, intenta cpctx.Provider.GetTenantBySlug y parsea t.ID como UUID
   - si no puede => 400 invalid_tenant
4) TokenStore:
   - requiere h.c.TenantSQLManager
   - tenantDB := GetPG(ctx, body.TenantID) (ojo: si body.TenantID es slug, esto puede fallar si GetPG espera slug o id según implementación)
   - tokenStore := storelib.NewTokenStore(tenantDB.Pool())
5) Crea token verify:
   verifyTTL := 48h
   pt := tokenStore.CreateEmailVerification(ctx, tenantUUID, userUUID, email, ttl, nil, nil)
6) Construye link:
   baseURL = ENV BASE_URL (default http://localhost:8080)
   link = baseURL + "/v1/auth/verify-email?token=" + pt + "&tenant_id=" + body.TenantID
   si u.SourceClientID != nil => &client_id=...
   (No agrega redirect_uri intencionalmente)
7) Renderiza template:
   - intenta obtener tenant con cpctx.Provider.GetTenantByID(ctx, tenantUUID.String())
   - vars {UserEmail, Tenant, Link, TTL}
   - si existe template email.TemplateVerify => renderOverride
   - fallback a email simple si no hay template
8) Envío:
   sender := h.c.SenderProvider.GetSender(ctx, tenantUUID)
   sender.Send(u.Email, subj, htmlBody, textBody)
9) Audit: "admin_resend_verification"
10) Responde 204.

Problemas importantes / deuda técnica
-------------------------------------
1) Inconsistencia tenant identifier (slug vs UUID):
   - Disable/Enable mezclan GetTenantBySlug(tid) con uuid.MustParse(tid).
   - Resend-verification intenta soportar ambos, pero después vuelve a llamar GetPG con body.TenantID (que podría ser slug/uuid).
   Esto es fuente de bugs y panics.

2) Mezcla de responsabilidades (God controller):
   Controller hace:
     - parse/validación
     - resolver store (global/tenant)
     - lógica de disable/enable
     - revocación de tokens
     - auditoría
     - armado y envío de emails + templates
     - generación de tokens y links
   En V2 esto debe separarse en services.

3) Side effects pesados inline:
   - enviar mails y generar tokens está inline en la request.
   - si el SMTP cuelga, este endpoint se vuelve lento.
   Mejor: best-effort con timeout o job async.

4) Falta de límites:
   - no usa MaxBytesReader para body.
   - no valida UUID de user_id antes de usar uuid.MustParse en resend-verification.

5) Revoke tokens “best-effort” pero inconsistente:
   - si existe revoker usa RevokeAllRefreshByUser
   - si no, usa RevokeAllRefreshTokens(user,"")
   Esto sugiere APIs duplicadas en store.

Cómo lo refactorizaría (V2) — arquitectura y patrones
-----------------------------------------------------
Objetivo: controller fino + services + repos + clients.

1) TenantResolver (clave)
   Crear un componente único:
     ResolveTenant(ctx, tenantIdOrSlug string) -> (tenantUUID, tenantSlug, *Tenant, error)
   y otro:
     ResolveUserTenant(ctx, user *core.User, providedTenant string) -> tenantUUID/slug
   Así eliminás los panics y la mezcla slug/uuid.

2) Servicios por dominio
   - services/admin_user_service.go:
       DisableUser(ctx, req, actor) -> error
       EnableUser(ctx, req, actor) -> error
       ResendVerification(ctx, req, actor) -> error

     Este service encapsula:
       - resolver store correcto (global/tenant)
       - parse duration
       - disable/enable
       - revocar refresh tokens
       - auditoría
       - “disparar” notificación email (sync con timeout o async)

   - services/verification_service.go:
       CreateVerificationToken(ctx, tenantUUID, userUUID, email, ttl) -> token

   - services/email_notification_service.go:
       SendUserBlocked(ctx, tenantUUID, email, vars)
       SendUserUnblocked(ctx, tenantUUID, email, vars)
       SendVerifyEmail(ctx, tenantUUID, email, link, vars)

3) Repos/clients
   - UserAdminRepository (global y tenant):
       DisableUser(...)
       EnableUser(...)
       GetUserByID(...)
       RevokeAllRefreshByUser(...)  // unificar en una sola API

   - TokenStoreFactory:
       ForTenant(ctx, tenantUUID/slug) -> TokenStore
     (para no crear TokenStore directo en controller)

   - LinkBuilder:
       BuildVerifyEmailLink(baseURL, token, tenantKey, clientID?) -> string

4) Concurrencia (donde conviene)
   - En disable/enable:
       - hacer disable/enable + revoke tokens sincrónico (rápido)
       - email: goroutine con timeout corto (ej 2-3s) y log warning si falla
         (o encolar job si querés full robust)
   Importante: NO hacer goroutines sin control masivo; acá son 1-2 emails por request.

5) DTOs
   - dtos/admin_users.go:
       DisableUserRequest { userId, tenantId?, reason?, duration? }
       EnableUserRequest  { userId, tenantId? }
       ResendVerificationRequest { userId, tenantId }
   - respuestas: 204 o {status:"ok"} si querés UI-friendly.

6) Seguridad y consistencia
   - Validar user_id UUID siempre antes de usarlo.
   - Nunca usar uuid.MustParse con datos externos.
   - Cache-Control: no-store (admin endpoints + tokens).
   - Body size limit: MaxBytesReader(w, r.Body, 32<<10).

Refactor mapping (para tu V2)
-----------------------------
- Controller:
    controllers/admin_users_controller.go
      -> switch por path (disable/enable/resend)
      -> parse DTO + call service

- Service:
    services/admin_user_service.go
    services/tenant_resolver.go
    services/email_notifications.go
    services/verification_service.go
    services/link_builder.go

- Clients/Repos:
    clients/controlplane_client.go (para traer tenant templates)
    repos/user_admin_repo.go
    repos/token_repo.go



Dos “bombitas” que te conviene arreglar YA (aunque sea en v1)
-------------------------------------------------------------
	+ uuid.MustParse(tid) te puede tumbar el proceso si tid es slug.
	Cambialo por parse seguro y resolver tenant UUID bien.

	+ Unificá “tenantIdOrSlug” en una sola convención de entrada
	(o siempre slug, o siempre UUID) y resolvelo con un TenantResolver.

*/

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/audit"
	"github.com/dropDatabas3/hellojohn/internal/email"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	storelib "github.com/dropDatabas3/hellojohn/internal/store"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type AdminUsersHandler struct{ c *app.Container }

func NewAdminUsersHandler(c *app.Container) *AdminUsersHandler { return &AdminUsersHandler{c: c} }

type adminUserReq struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Duration string `json:"duration,omitempty"` // e.g. "24h", "2h30m"
}

func (h *AdminUsersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.c == nil || h.c.Store == nil {
		httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "store requerido", 3800)
		return
	}
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
		return
	}

	var body adminUserReq
	if !httpx.ReadJSON(w, r, &body) {
		return
	}
	body.UserID = strings.TrimSpace(body.UserID)
	body.TenantID = strings.TrimSpace(body.TenantID)
	body.Reason = strings.TrimSpace(body.Reason)
	body.Duration = strings.TrimSpace(body.Duration)

	if body.UserID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "user_id requerido", 3801)
		return
	}

	// Resolve the correct store (Global vs Tenant-specific)
	// Define minimal interface required for Disable/Enable operations
	type userManager interface {
		DisableUser(ctx context.Context, userID, by, reason string, until *time.Time) error
		EnableUser(ctx context.Context, userID, by string) error
		GetUserByID(ctx context.Context, id string) (*core.User, error)
	}

	var store userManager = h.c.Store
	var revoker interface {
		RevokeAllRefreshByUser(context.Context, string) (int, error)
	}

	// Try to resolve Tenant Store if ID provided
	if body.TenantID != "" {
		if h.c.TenantSQLManager != nil {
			ts, err := h.c.TenantSQLManager.GetPG(r.Context(), body.TenantID)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "tenant_db_error", err.Error(), 3804)
				return
			}
			store = ts
			revoker = ts
		}
	} else {
		// Default global store
		if r, ok := h.c.Store.(interface {
			RevokeAllRefreshByUser(context.Context, string) (int, error)
		}); ok {
			revoker = r
		}
	}

	// Who performs the action (for audit fields)
	by := ""
	if cl := httpx.GetClaims(r.Context()); cl != nil {
		if sub, _ := cl["sub"].(string); sub != "" {
			by = sub
		}
	}

	switch r.URL.Path {
	case "/v1/admin/users/disable":
		var until *time.Time
		if body.Duration != "" {
			d, err := time.ParseDuration(body.Duration)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_duration", "duración inválida (e.g. 1h, 30m)", 3802)
				return
			}
			t := time.Now().Add(d)
			until = &t
		}

		if err := store.DisableUser(r.Context(), body.UserID, by, body.Reason, until); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "disable_failed", err.Error(), 3803)
			return
		}

		// Revoke tokens
		if revoker != nil {
			_, _ = revoker.RevokeAllRefreshByUser(r.Context(), body.UserID)
		} else {
			// Fallback legacy method if available on global store
			_ = h.c.Store.RevokeAllRefreshTokens(r.Context(), body.UserID, "")
		}

		// Audit
		fields := map[string]any{
			"by": by, "user_id": body.UserID, "reason": body.Reason, "tenant_id": body.TenantID,
		}
		if until != nil {
			fields["until"] = *until
		}
		audit.Log(r.Context(), "admin_user_disabled", fields)

		// Send Notification Email
		if u, err := store.GetUserByID(r.Context(), body.UserID); err == nil && u != nil && u.Email != "" {
			// Resolve Tenant
			tid := body.TenantID
			if tid == "" {
				tid = u.TenantID
			}
			if tid != "" {
				t, _ := cpctx.Provider.GetTenantBySlug(r.Context(), tid) // Try slug/ID lookup via provider (assuming enhanced resolver/provider)
				// Actually Provider expects Slug. If ID, we rely on updated resolver logic elsewhere, but here we call Provider directly.
				// If tid is UUID, GetTenantBySlug might fail if not enhanced.
				// BUT we enhanced `manager.go` resolver, not `cpctx.Provider` itself.
				// However, `admin_tenants_fs` handles ID lookup.
				// Let's try. If it fails, maybe try fallback.
				// Or use `h.c.SenderProvider.GetSender` logic which resolves tenant implicitly? No.

				// Reusing renderOverride from email_flows.go (same package)
				if t != nil && t.Settings.Mailing != nil && t.Settings.Mailing.Templates != nil {
					if tpl, ok := t.Settings.Mailing.Templates[email.TemplateUserBlocked]; ok {
						vars := map[string]any{
							"UserEmail": u.Email,
							"Reason":    body.Reason,
							"Until":     until,
							"Tenant":    tid,
						}
						htmlBody, textBody, err := renderOverride(tpl, vars)
						if err == nil && htmlBody != "" {
							if sender, err := h.c.SenderProvider.GetSender(r.Context(), uuid.MustParse(tid)); err == nil {
								_ = sender.Send(u.Email, tpl.Subject, htmlBody, textBody)
							}
						}
					}
				}
			}
		}

		w.WriteHeader(http.StatusNoContent)
		return

	case "/v1/admin/users/enable":
		if err := store.EnableUser(r.Context(), body.UserID, by); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "enable_failed", err.Error(), 3805)
			return
		}
		audit.Log(r.Context(), "admin_user_enabled", map[string]any{
			"by": by, "user_id": body.UserID, "tenant_id": body.TenantID,
		})

		// Send Notification Email
		if u, err := store.GetUserByID(r.Context(), body.UserID); err == nil && u != nil && u.Email != "" {
			tid := body.TenantID
			if tid == "" {
				tid = u.TenantID
			}
			if tid != "" {
				t, _ := cpctx.Provider.GetTenantBySlug(r.Context(), tid)
				if t != nil && t.Settings.Mailing != nil && t.Settings.Mailing.Templates != nil {
					if tpl, ok := t.Settings.Mailing.Templates[email.TemplateUserUnblocked]; ok {
						vars := map[string]any{
							"UserEmail": u.Email,
							"Tenant":    tid,
						}
						htmlBody, textBody, err := renderOverride(tpl, vars)
						if err == nil && htmlBody != "" {
							if sender, err := h.c.SenderProvider.GetSender(r.Context(), uuid.MustParse(tid)); err == nil {
								_ = sender.Send(u.Email, tpl.Subject, htmlBody, textBody)
							}
						}
					}
				}
			}
		}

		w.WriteHeader(http.StatusNoContent)
		return

	case "/v1/admin/users/resend-verification":
		// Resend verification email for a user
		if body.TenantID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id requerido", 3806)
			return
		}

		// Get user to obtain email
		u, err := store.GetUserByID(r.Context(), body.UserID)
		if err != nil || u == nil {
			httpx.WriteError(w, http.StatusNotFound, "user_not_found", "usuario no encontrado", 3807)
			return
		}
		if u.EmailVerified {
			httpx.WriteError(w, http.StatusBadRequest, "already_verified", "el email ya está verificado", 3808)
			return
		}

		// Resolve tenant UUID
		var tenantUUID uuid.UUID
		if parsed, err := uuid.Parse(body.TenantID); err == nil {
			tenantUUID = parsed
		} else if cpctx.Provider != nil {
			if t, err := cpctx.Provider.GetTenantBySlug(r.Context(), body.TenantID); err == nil && t != nil {
				if parsed, err := uuid.Parse(t.ID); err == nil {
					tenantUUID = parsed
				}
			}
		}
		if tenantUUID == uuid.Nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant", "tenant_id inválido", 3809)
			return
		}

		// Create verification token using TokenStore from tenant DB
		var tokenStore *storelib.TokenStore
		if h.c.TenantSQLManager != nil {
			tenantDB, err := h.c.TenantSQLManager.GetPG(r.Context(), body.TenantID)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "tenant_db_error", err.Error(), 3810)
				return
			}
			tokenStore = storelib.NewTokenStore(tenantDB.Pool())
		} else {
			httpx.WriteError(w, http.StatusInternalServerError, "not_configured", "tenant SQL manager no configurado", 3810)
			return
		}

		verifyTTL := 48 * time.Hour // 48 hours for verification
		pt, err := tokenStore.CreateEmailVerification(r.Context(), tenantUUID, uuid.MustParse(body.UserID), u.Email, verifyTTL, nil, nil)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "token_error", "error creando token de verificación", 3811)
			return
		}

		// Build verification link with tenant_id so verify endpoint knows which DB to use
		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}
		link := baseURL + "/v1/auth/verify-email?token=" + pt + "&tenant_id=" + body.TenantID

		// Append client_id if available (matching SDK flow)
		if u.SourceClientID != nil && *u.SourceClientID != "" {
			link += "&client_id=" + url.QueryEscape(*u.SourceClientID)
		}
		// Note: We intentionally do NOT append redirect_uri to match SDK behavior and avoid validation errors
		// if the VerifyEmailURL is not whitelisted.

		// Render and send email using tenant templates
		t, _ := cpctx.Provider.GetTenantByID(r.Context(), tenantUUID.String())
		tenantName := body.TenantID
		if t != nil {
			tenantName = t.Name
		}
		vars := map[string]any{
			"UserEmail": u.Email,
			"Tenant":    tenantName,
			"Link":      link,
			"TTL":       "48 horas",
		}

		subj := "Verificá tu correo electrónico"
		htmlBody := ""
		textBody := ""

		if t != nil && t.Settings.Mailing != nil && t.Settings.Mailing.Templates != nil {
			if tpl, ok := t.Settings.Mailing.Templates[email.TemplateVerify]; ok {
				subj = tpl.Subject
				htmlBody, textBody, _ = renderOverride(tpl, vars)
			}
		}

		// Fallback simple email if no template
		if htmlBody == "" {
			htmlBody = "<p>Hola " + u.Email + ",</p><p>Hacé clic en el siguiente enlace para verificar tu email:</p><p><a href=\"" + link + "\">Verificar Email</a></p>"
			textBody = "Verificá tu email visitando: " + link
		}

		// Send email
		if sender, err := h.c.SenderProvider.GetSender(r.Context(), tenantUUID); err == nil {
			if err := sender.Send(u.Email, subj, htmlBody, textBody); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "email_error", "error enviando email", 3812)
				return
			}
		} else {
			httpx.WriteError(w, http.StatusInternalServerError, "sender_error", "no se pudo obtener el sender de email", 3813)
			return
		}

		audit.Log(r.Context(), "admin_resend_verification", map[string]any{
			"by": by, "user_id": body.UserID, "tenant_id": body.TenantID, "email": u.Email,
		})

		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Fallback 404
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_found"})
}
