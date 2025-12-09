package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
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
