// Package router define las rutas HTTP V2 del servicio.
package router

import (
	"net/http"
	"strings"

	ctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/admin"
	httperrors "github.com/dropDatabas3/hellojohn/internal/http/v2/errors"
	mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// AdminRouterDeps contiene las dependencias para el router admin.
type AdminRouterDeps struct {
	DAL         store.DataAccessLayer
	Issuer      *jwtx.Issuer
	Controllers *ctrl.Controllers
	RateLimiter mw.RateLimiter // Opcional: rate limiter por IP
}

// RegisterAdminRoutes registra todas las rutas administrativas en un mux.
// Esto se llama desde el server/wiring principal.
func RegisterAdminRoutes(mux *http.ServeMux, deps AdminRouterDeps) {
	dal := deps.DAL
	c := deps.Controllers
	issuer := deps.Issuer
	limiter := deps.RateLimiter

	// Admin Clients (Control Plane - no requiere DB)
	mux.Handle("/v2/admin/clients", adminClientsHandler(dal, issuer, limiter, c.Clients, false))
	mux.Handle("/v2/admin/clients/", adminClientsHandler(dal, issuer, limiter, c.Clients, false))

	// Admin Consents (Data Plane - requiere DB)
	mux.Handle("/v2/admin/consents", adminConsentsHandler(dal, issuer, limiter, c.Consents, true))
	mux.Handle("/v2/admin/consents/", adminConsentsHandler(dal, issuer, limiter, c.Consents, true))

	// Admin Users (Data Plane - requiere DB)
	mux.Handle("/v2/admin/users/", adminUsersHandler(dal, issuer, limiter, c.Users, true))

	// Admin Scopes (Control Plane - no requiere DB)
	mux.Handle("/v2/admin/scopes", adminScopesHandler(dal, issuer, limiter, c.Scopes, false))
	mux.Handle("/v2/admin/scopes/", adminScopesHandler(dal, issuer, limiter, c.Scopes, false))

	// Admin RBAC (Data Plane - requiere DB)
	mux.Handle("/v2/admin/rbac/", adminRBACHandler(dal, issuer, limiter, c.RBAC, true))

	// Admin Tenants (Control Plane - System Admin)
	RegisterTenantAdminRoutes(mux, deps)

	// User CRUD (Data Plane - requiere DB)
	// Note: No trailing slash to avoid conflict with tenant routes
	userHandler := adminUserCRUDHandler(dal, issuer, limiter, c.UsersCRUD, true)
	mux.Handle("POST /v2/admin/tenants/{id}/users", userHandler)
	mux.Handle("GET /v2/admin/tenants/{id}/users", userHandler)
	mux.Handle("GET /v2/admin/tenants/{id}/users/{userId}", userHandler)
	mux.Handle("PUT /v2/admin/tenants/{id}/users/{userId}", userHandler)
	mux.Handle("DELETE /v2/admin/tenants/{id}/users/{userId}", userHandler)
}

// ─── Helpers para crear handlers con middleware chain ───

func adminBaseChain(dal store.DataAccessLayer, issuer *jwtx.Issuer, limiter mw.RateLimiter, requireDB bool) []mw.Middleware {
	chain := []mw.Middleware{
		mw.WithRecover(),
		mw.WithRequestID(),
		// Tenant resolution antes de auth para que estén en context
		mw.WithTenantResolution(dal, false),
		mw.RequireTenant(),
	}

	if requireDB {
		chain = append(chain, mw.RequireTenantDB())
	}

	// Auth obligatorio para admin
	if issuer != nil {
		chain = append(chain,
			mw.RequireAuth(issuer),
			mw.RequireAdmin(mw.AdminConfigFromEnv()),
		)
	}

	// Rate limiting por IP (sin client_id ya que admin panel no lo tiene)
	if limiter != nil {
		chain = append(chain, mw.WithRateLimit(mw.RateLimitConfig{
			Limiter: limiter,
			KeyFunc: mw.IPOnlyRateKey,
		}))
	}

	// Logging al final para que tenant/user ya estén en context
	chain = append(chain, mw.WithLogging())

	return chain
}

// ─── Admin Clients ───

func adminClientsHandler(dal store.DataAccessLayer, issuer *jwtx.Issuer, limiter mw.RateLimiter, c *ctrl.ClientsController, requireDB bool) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		case path == "/v2/admin/clients" || path == "/v2/admin/clients/":
			switch r.Method {
			case http.MethodGet:
				c.ListClients(w, r)
			case http.MethodPost:
				c.CreateClient(w, r)
			default:
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
			}

		case strings.HasSuffix(path, "/revoke"):
			// Handle /v2/admin/clients/{clientId}/revoke
			if r.Method == http.MethodPost {
				c.RevokeSecret(w, r)
			} else {
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
			}

		case strings.HasPrefix(path, "/v2/admin/clients/"):
			switch r.Method {
			case http.MethodPut, http.MethodPatch:
				c.UpdateClient(w, r)
			case http.MethodDelete:
				c.DeleteClient(w, r)
			default:
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
			}

		default:
			httperrors.WriteError(w, httperrors.ErrNotFound)
		}
	})

	return mw.Chain(handler, adminBaseChain(dal, issuer, limiter, requireDB)...)
}

// ─── Admin Consents ───

func adminConsentsHandler(dal store.DataAccessLayer, issuer *jwtx.Issuer, limiter mw.RateLimiter, c *ctrl.ConsentsController, requireDB bool) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		case r.Method == http.MethodPost && path == "/v2/admin/consents/upsert":
			c.Upsert(w, r)

		case r.Method == http.MethodPost && path == "/v2/admin/consents/revoke":
			c.Revoke(w, r)

		case r.Method == http.MethodGet && strings.HasPrefix(path, "/v2/admin/consents/by-user/"):
			c.ListByUser(w, r)

		case r.Method == http.MethodGet && (path == "/v2/admin/consents" || path == "/v2/admin/consents/"):
			c.List(w, r)

		case r.Method == http.MethodDelete && strings.HasPrefix(path, "/v2/admin/consents/"):
			c.Delete(w, r)

		default:
			httperrors.WriteError(w, httperrors.ErrNotFound)
		}
	})

	return mw.Chain(handler, adminBaseChain(dal, issuer, limiter, requireDB)...)
}

// ─── Admin Users ───

func adminUsersHandler(dal store.DataAccessLayer, issuer *jwtx.Issuer, limiter mw.RateLimiter, c *ctrl.UsersController, requireDB bool) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
			return
		}

		switch r.URL.Path {
		case "/v2/admin/users/disable":
			c.Disable(w, r)
		case "/v2/admin/users/enable":
			c.Enable(w, r)
		case "/v2/admin/users/resend-verification":
			c.ResendVerification(w, r)
		default:
			httperrors.WriteError(w, httperrors.ErrNotFound)
		}
	})

	return mw.Chain(handler, adminBaseChain(dal, issuer, limiter, requireDB)...)
}

// ─── Admin Scopes ───

func adminScopesHandler(dal store.DataAccessLayer, issuer *jwtx.Issuer, limiter mw.RateLimiter, c *ctrl.ScopesController, requireDB bool) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		case path == "/v2/admin/scopes" || path == "/v2/admin/scopes/":
			switch r.Method {
			case http.MethodGet:
				c.ListScopes(w, r)
			case http.MethodPost, http.MethodPut:
				c.UpsertScope(w, r)
			default:
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
			}

		case strings.HasPrefix(path, "/v2/admin/scopes/"):
			switch r.Method {
			case http.MethodDelete:
				c.DeleteScope(w, r)
			default:
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
			}

		default:
			httperrors.WriteError(w, httperrors.ErrNotFound)
		}
	})

	return mw.Chain(handler, adminBaseChain(dal, issuer, limiter, requireDB)...)
}

// ─── Admin RBAC ───

func adminRBACHandler(dal store.DataAccessLayer, issuer *jwtx.Issuer, limiter mw.RateLimiter, c *ctrl.RBACController, requireDB bool) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		// /v2/admin/rbac/users/{userID}/roles
		case strings.Contains(path, "/users/") && strings.HasSuffix(path, "/roles"):
			switch r.Method {
			case http.MethodGet:
				c.GetUserRoles(w, r)
			case http.MethodPost:
				c.UpdateUserRoles(w, r)
			default:
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
			}

		// /v2/admin/rbac/roles/{role}/perms
		case strings.Contains(path, "/roles/") && strings.HasSuffix(path, "/perms"):
			switch r.Method {
			case http.MethodGet:
				c.GetRolePerms(w, r)
			case http.MethodPost:
				c.UpdateRolePerms(w, r)
			default:
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
			}

		default:
			httperrors.WriteError(w, httperrors.ErrNotFound)
		}
	})

	return mw.Chain(handler, adminBaseChain(dal, issuer, limiter, requireDB)...)
}


// ─── Admin User CRUD ───

func adminUserCRUDHandler(dal store.DataAccessLayer, issuer *jwtx.Issuer, limiter mw.RateLimiter, c *ctrl.UsersCRUDController, requireDB bool) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		// POST/GET /v2/admin/tenants/{id}/users - Create user or List users
		case strings.Contains(path, "/tenants/") && strings.HasSuffix(path, "/users"):
			if r.Method == http.MethodPost {
				c.CreateUser(w, r)
			} else if r.Method == http.MethodGet {
				c.ListUsers(w, r)
			} else {
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
			}

		// GET/PUT/DELETE /v2/admin/tenants/{id}/users/{userId}
		case strings.Contains(path, "/tenants/") && strings.Contains(path, "/users/"):
			switch r.Method {
			case http.MethodGet:
				c.GetUser(w, r)
			case http.MethodPut:
				c.UpdateUser(w, r)
			case http.MethodDelete:
				c.DeleteUser(w, r)
			default:
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
			}

		default:
			// No match, skip to avoid conflict with other handlers
			http.NotFound(w, r)
		}
	})

	return mw.Chain(handler, adminBaseChain(dal, issuer, limiter, requireDB)...)
}
