/*
session_login.go — Session Cookie Login (sid) + tenant resolution (SQL/FS) + cache de sesión

Qué es este archivo (la posta)
------------------------------
Este archivo define NewSessionLoginHandler(...) que devuelve un http.HandlerFunc para:
	- Autenticar email+password ("password grant" pero en modo cookie/session)
	- Resolver el tenant de manera flexible (por tenant_id o por client_id)
	- Emitir una sesión server-side en cache (key "sid:<hash>")
	- Setear una cookie de sesión (sid) usando helpers en cookieutil.go

No emite JWT directamente: su responsabilidad es establecer una cookie para que
otros endpoints (principalmente /oauth2/* en modo browser) puedan continuar el flujo.

Dependencias reales (lo que toca)
---------------------------------
- c.Store (obligatorio):
		- GetClientByClientID (cuando viene client_id)
		- GetUserByEmail + CheckPassword (ya sea en store global o tenant repo)
- c.TenantSQLManager (opcional): abre un repo por tenant con helpers.OpenTenantRepo
- cpctx.Provider (opcional): lookup en control-plane FS para resolver "tenant slug" por client_id
- c.Cache (requerido en práctica): persiste payload de sesión bajo "sid:<sha256(rawSID)>"
- cookieutil.go: BuildSessionCookie(...) para construir cookie (SameSite/Secure/Domain/TTL)

⚠️ Nota: el handler valida c.Store != nil, pero NO valida c.Cache != nil.
Si el container se inicializa sin cache, esto puede panic.

Ruta soportada (contrato efectivo)
----------------------------------
- POST /v1/session/login
		Request JSON:
			{
				"tenant_id": "..."  (slug o UUID; opcional si viene client_id)
				"client_id": "..."  (opcional si viene tenant_id)
				"email": "...",
				"password": "..."
			}
		Response:
			- 204 No Content
			- Set-Cookie: <cookieName>=<rawSID>; ...
			- Cache-Control: no-store

Flujo interno (por etapas)
--------------------------
1) Validación básica
	 - solo POST
	 - tenant_id o client_id requerido
	 - email se normaliza (trim + lower)

2) Resolución de tenant (lo más “frágil” del archivo)
	 A) Si viene client_id:
			- Intenta SQL: c.Store.GetClientByClientID
			- Si lo encuentra, intenta “forzar slug FS”:
					* cpctx.Provider.ListTenants
					* loop por tenants + Provider.GetClient(tenantSlug, clientID)
					* si matchea: tenantSlug = t.Slug y tenantID = cl.TenantID (UUID)
			- Si no matchea en FS: helpers.ResolveTenantSlugAndID(ctx, cl.TenantID)
			- Si SQL falla, fallback FS-only:
					* itera tenants en FS buscando el client
					* tenantSlug=t.Slug y tenantID=t.ID (o fallback = slug)

	 B) Si viene tenant_id:
			- helpers.ResolveTenantSlugAndID(ctx, req.TenantID)

	 Resultado: tenantSlug se usa como "tenant final" de la sesión para ser compatible
	 con oauth_authorize (que hace fallback FS y espera slug en algunos paths).

3) Selección de store (global vs tenant DB)
	 - Define una interfaz mínima userAuthStore (GetUserByEmail + CheckPassword)
	 - Intenta abrir tenant repo via TenantSQLManager + helpers.OpenTenantRepo(tenantSlug)
	 - Si no se puede, usa store global si satisface la interfaz
	 - Esto deja un comportamiento “best effort”: puede autenticarse contra DB global
		 si la tenant DB no está disponible.

4) Auth con retries por key mismatch
	 - lookupID inicialmente tenantID (UUID) o tenantSlug
	 - llama GetUserByEmail(lookupID)
	 - si falla y tenantSlug != lookupID, reintenta con tenantSlug

5) Crear sesión en cache + cookie
	 - rawSID: token opaco 32 bytes
	 - payload JSON (SessionPayload): {user_id, tenant_id, expires}
	 - cache key: "sid:"+SHA256Base64URL(rawSID)
	 - cookie: BuildSessionCookie(cookieName, rawSID, domain, sameSite, secure, ttl)
	 - responde 204

Seguridad / invariantes
-----------------------
- CSRF:
	Este endpoint está pensado para browser cookies. El enforcement de CSRF se aplica
	(opcionalmente) desde middleware a POST /v1/session/login (ver csrf.go y tests e2e).

- Cache-Control:
	Bien: fuerza no-store en la respuesta.

- Logging:
	Hace log DEBUG con email/tenant y decisiones de routing. Esto es útil, pero puede
	filtrar metadata sensible (emails) si se habilita en producción.

- Tenant resolution por loop:
	El lookup FS de client_id hace ListTenants + GetClient por tenant: O(#tenants).
	V2 debería tener un índice (client_id -> tenant) o una API directa.

Problemas principales (cuellos de botella + bugs probables)
-----------------------------------------------------------
1) Resolver tenant mezclando SQL y FS
	 El flujo es complejo y tiene “override” FS. Esto existe para compatibilidad con oauth_authorize,
	 pero aumenta riesgo de inconsistencias (UUID vs slug vs fallback).

2) Fallback “best effort” a store global
	 Si la tenant DB no abre, puede autenticar contra el store global si implementa la interfaz.
	 Eso puede ser deseable (degraded mode) o peligroso (cross-tenant) dependiendo del store.

3) Respuesta con Content-Type JSON pero 204
	 Setea "application/json" aunque no devuelve body. No rompe, pero es inconsistente.

4) Cache requerido pero no validado
	 Si c.Cache es nil, c.Cache.Set paniquea.

Cómo lo refactorizaría a V2 (plan concreto)
-------------------------------------------
Objetivo: hacer que /session/login sea una capa fina y determinística:

FASE 1 — Resolver tenant de forma única
	- Introducir TenantResolver único (client_id -> tenantSlug+tenantUUID) sin loops.
	- Hacer que oauth_authorize deje de depender de slug “mágico” en sesión.

FASE 2 — Service de sesión
	- services/session_service.go:
			LoginWithPassword(ctx, tenant, email, password) -> sessionID + expires
			StoreSession(ctx, sessionID, payload, ttl)
			BuildCookie(sessionID, opts)

FASE 3 — Contratos y seguridad
	- Enforzar CSRF en este endpoint siempre en modo cookie
	- Reducir logs o moverlos a trazas con request_id y sin PII
	- Validar (o requerir) c.Cache

*/

package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/http/v1/helpers"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/v1/core"
)

type SessionLoginRequest struct {
	TenantID string `json:"tenant_id"`
	ClientID string `json:"client_id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SessionPayload struct {
	UserID   string    `json:"user_id"`
	TenantID string    `json:"tenant_id"`
	Expires  time.Time `json:"expires"`
}

func NewSessionLoginHandler(c *app.Container, cookieName, cookieDomain, sameSite string, secure bool, ttl time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req SessionLoginRequest
		if !httpx.ReadJSON(w, r, &req) {
			return
		}
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		if req.TenantID == "" && req.ClientID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id o client_id son requeridos", 1002)
			return
		}

		// Guard: verificar que el store esté inicializado
		if c.Store == nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "store not initialized", 1003)
			return
		}

		ctx := r.Context()

		// 1. Resolve Tenant Slug/ID
		var tenantSlug string
		var tenantID string // UUID

		// Helper to resolve client -> tenant
		if req.ClientID != "" {
			// Try SQL first
			cl, _, err := c.Store.GetClientByClientID(ctx, req.ClientID)
			if err == nil && cl != nil {
				// SQL Found. We effectively trust this, but we need the correct "Slug" for the session
				// to match what oauth_authorize expects (which uses FS lookup).
				// Attempt to resolve Slug via FS Client Lookup.
				foundFS := false
				if cpctx.Provider != nil {
					tenants, _ := cpctx.Provider.ListTenants(ctx)
					for _, t := range tenants {
						// Note: GetClient checks this tenant's clients
						if cFS, errGet := cpctx.Provider.GetClient(ctx, t.Slug, req.ClientID); errGet == nil && cFS != nil {
							tenantSlug = t.Slug
							tenantID = cl.TenantID // Prefer UUID from SQL
							foundFS = true
							log.Printf("DEBUG: session_login matched client %s to tenant slug %s (FS override)", req.ClientID, tenantSlug)
							break
						}
					}
				}

				if !foundFS {
					// Traditional resolve if not found in FS
					s, i := helpers.ResolveTenantSlugAndID(ctx, cl.TenantID)
					tenantSlug, tenantID = s, i
					log.Printf("DEBUG: session_login resolved tenant %s/%s from client %s (SQL-only)", tenantSlug, tenantID, req.ClientID)
				}
			} else {
				// Fallback FS (Client not in SQL)
				if cpctx.Provider != nil {
					tenants, _ := cpctx.Provider.ListTenants(ctx)
					for _, t := range tenants {
						if cFS, errGet := cpctx.Provider.GetClient(ctx, t.Slug, req.ClientID); errGet == nil && cFS != nil {
							tenantSlug = t.Slug
							tenantID = t.ID
							if tenantID == "" {
								tenantID = tenantSlug
							} // fallback
							log.Printf("DEBUG: session_login resolved tenant %s/%s from client %s (FS only)", tenantSlug, tenantID, req.ClientID)
							break
						}
					}
				}
			}
		} else if req.TenantID != "" {
			// Direct tenant resolution
			tenantSlug, tenantID = helpers.ResolveTenantSlugAndID(ctx, req.TenantID)
		}

		if tenantSlug == "" {
			log.Printf("DEBUG: session_login could not resolve tenant for client=%s tenant=%s", req.ClientID, req.TenantID)
			httpx.WriteError(w, http.StatusBadRequest, "missing_tenant", "tenant could not be resolved", 1002)
			return
		}

		// 2. Open Tenant Store
		// Interface for what we need to avoid type mismatches
		type userAuthStore interface {
			GetUserByEmail(ctx context.Context, tenantID, email string) (*core.User, *core.Identity, error)
			CheckPassword(hash *string, password string) bool
		}

		// Default to global if something fails or is global
		var storeToUse userAuthStore
		if s, ok := c.Store.(userAuthStore); ok {
			storeToUse = s
		}

		if c.TenantSQLManager != nil {
			repo, err := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug)
			if err == nil && repo != nil {
				if s, ok := any(repo).(userAuthStore); ok {
					storeToUse = s
					log.Printf("DEBUG: session_login switched to Tenant DB for %s", tenantSlug)
				}
			} else {
				if !helpers.IsNoDBForTenant(err) {
					log.Printf("DEBUG: session_login failed to open tenant repo: %v", err)
				}
			}
		}

		if storeToUse == nil {
			log.Printf("DEBUG: session_login warning: store does not satisfy userAuthStore interface?")
			httpx.WriteError(w, http.StatusInternalServerError, "store_error", "store interface mismatch", 1003)
			return
		}

		// 3. Authenticate
		// Use UUID ID if available, otherwise Slug
		lookupID := tenantID
		if lookupID == "" {
			lookupID = tenantSlug
		}

		log.Printf("DEBUG: session_login executing GetUserByEmail for %s in tenant %s", req.Email, lookupID)
		u, id, err := storeToUse.GetUserByEmail(ctx, lookupID, req.Email)

		// Retry logic for key mismatch (only if we failed and keys differ)
		if (err != nil || u == nil) && tenantSlug != "" && tenantSlug != lookupID {
			log.Printf("DEBUG: session_login retry with slug %s", tenantSlug)
			u, id, err = storeToUse.GetUserByEmail(ctx, tenantSlug, req.Email)
		}

		if err != nil {
			log.Printf("DEBUG: session_login user lookup failed: %v", err)
		} else if id == nil || id.PasswordHash == nil {
			log.Printf("DEBUG: session_login user has no password or identity")
		} else if !storeToUse.CheckPassword(id.PasswordHash, req.Password) {
			log.Printf("DEBUG: session_login password mismatch")
		}

		if err != nil || id == nil || id.PasswordHash == nil || !storeToUse.CheckPassword(id.PasswordHash, req.Password) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "usuario o password inválidos", 1202)
			return
		}

		rawSID, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar sid", 1301)
			return
		}

		// Use tenantSlug for Session TenantID because oauth_authorize expects it for FS fallback
		finalTenantID := tenantSlug
		if finalTenantID == "" && u.TenantID != "" {
			// fallback attempt, but if tenantSlug is found, use it.
			finalTenantID = u.TenantID
		}
		// RE-FORCE SLUG if we found it via FS above. It's safer for authorize endpoint.
		if tenantSlug != "" {
			finalTenantID = tenantSlug
		}

		exp := time.Now().Add(ttl)
		payload := SessionPayload{UserID: u.ID, TenantID: finalTenantID, Expires: exp}
		b, _ := json.Marshal(payload)
		c.Cache.Set("sid:"+tokens.SHA256Base64URL(rawSID), b, ttl)

		// Seteamos cookie usando el helper centralizado
		cookie := BuildSessionCookie(cookieName, rawSID, cookieDomain, sameSite, secure, ttl)
		log.Printf("DEBUG: session_login setting cookie: Name=%s Domain=%s Path=%s Secure=%v SameSite=%v",
			cookie.Name, cookie.Domain, cookie.Path, cookie.Secure, cookie.SameSite)
		http.SetCookie(w, cookie)

		// Evitar cacheo de respuestas que tocan sesión
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")

		w.WriteHeader(http.StatusNoContent) // 204
	}
}
