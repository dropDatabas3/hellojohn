package router

import (
	"net/http"
	"strings"

	ctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/admin"
	httperrors "github.com/dropDatabas3/hellojohn/internal/http/v2/errors"
	mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
)

// RegisterTenantAdminRoutes registra rutas de administración de tenants.
// Control Plane: Create/List/Get/Update/Delete Tenants.
// Ops: Migrate, Keys, TestConnection.
func RegisterTenantAdminRoutes(mux *http.ServeMux, deps AdminRouterDeps) {
	// Reusamos deps.Controllers pero necesitamos el Tenants specifically
	if deps.Controllers.Tenants == nil {
		return
	}

	c := deps.Controllers.Tenants
	issuer := deps.Issuer
	limiter := deps.RateLimiter

	// Handler con switch path
	handler := adminTenantsHandler(c)

	// Middleware chain: SysAdmin forced (or Admin w/o tenant)
	chain := sysAdminBaseChain(issuer, limiter)

	// Rutas exactas y prefijo
	mux.Handle("/v2/admin/tenants", mw.Chain(handler, chain...))
	mux.Handle("/v2/admin/tenants/", mw.Chain(handler, chain...))

	// Ops global
	mux.Handle("/v2/admin/tenants/test-connection", mw.Chain(http.HandlerFunc(c.TestConnection), chain...))
}

func adminTenantsHandler(c *ctrl.TenantsController) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Collection
		if path == "/v2/admin/tenants" || path == "/v2/admin/tenants/" {
			switch r.Method {
			case http.MethodGet:
				c.ListTenants(w, r)
			case http.MethodPost:
				c.CreateTenant(w, r)
			default:
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
			}
			return
		}

		// Todo lo demás tiene que colgar de /v2/admin/tenants/
		if !strings.HasPrefix(path, "/v2/admin/tenants/") {
			httperrors.WriteError(w, httperrors.ErrRouteNotFound)
			return
		}

		rest := strings.TrimPrefix(path, "/v2/admin/tenants/")
		// rest: "{id}" o "{id}/settings" etc

		switch {
		case rest == "": // /v2/admin/tenants/
			httperrors.WriteError(w, httperrors.ErrRouteNotFound)
			return

		case strings.HasSuffix(rest, "/keys/rotate"):
			if r.Method != http.MethodPost {
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
				return
			}
			c.RotateKeys(w, r)
			return

		case strings.HasSuffix(rest, "/settings"):
			switch r.Method {
			case http.MethodGet:
				c.GetSettings(w, r)
			case http.MethodPut:
				c.UpdateSettings(w, r)
			default:
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
			}
			return

		case strings.HasSuffix(rest, "/migrate"):
			if r.Method != http.MethodPost {
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
				return
			}
			c.MigrateTenant(w, r)
			return

		case strings.HasSuffix(rest, "/schema/apply"):
			if r.Method != http.MethodPost {
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
				return
			}
			c.ApplySchema(w, r)
			return

		case strings.HasSuffix(rest, "/infra-stats"):
			if r.Method != http.MethodGet {
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
				return
			}
			c.InfraStats(w, r)
			return

		case strings.HasSuffix(rest, "/cache/test-connection"):
			if r.Method != http.MethodPost {
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
				return
			}
			c.TestCache(w, r)
			return

		case strings.HasSuffix(rest, "/mailing/test"):
			if r.Method != http.MethodPost {
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
				return
			}
			c.TestMailing(w, r)
			return

		case strings.HasSuffix(rest, "/user-store/test-connection"):
			if r.Method != http.MethodPost {
				httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
				return
			}
			c.TestTenantDBConnection(w, r)
			return
		}

		// Si no matcheó nada, solo permitimos "{idOrSlug}" (1 segmento)
		if strings.Contains(strings.TrimSuffix(rest, "/"), "/") {
			httperrors.WriteError(w, httperrors.ErrRouteNotFound)
			return
		}

		// Item
		switch r.Method {
		case http.MethodGet:
			c.GetTenant(w, r)
		case http.MethodPut:
			c.UpdateTenant(w, r)
		case http.MethodDelete:
			c.DeleteTenant(w, r)
		default:
			httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
		}
	})
}

// sysAdminBaseChain es similar a adminBaseChain pero SIN TenantResolution,
// y forzando SysAdmin (todavía no existe en mw, así que usamos RequireAdmin normal pero sin tenant).
func sysAdminBaseChain(issuer *jwtx.Issuer, limiter mw.RateLimiter) []mw.Middleware {
	chain := []mw.Middleware{
		mw.WithRecover(),
		mw.WithRequestID(),
	}

	if issuer != nil {
		// Logica de SysAdmin: en teoría debería requerir un rol global o un token maestro.
		// Por ahora, usamos RequireAdmin con config genérica, pero OJO:
		// RequireAdmin suele chequear roles dentro del tenant context.
		// Si no hay tenant context, RequireAdmin podría fallar o comportarse raro dependiendo de la implementación.
		// El prompt dice: "Si no existe RequireSysAdmin, dejá RequireAdmin pero SIN RequireTenant."

		chain = append(chain,
			mw.RequireAuth(issuer),
			mw.RequireSysAdmin(issuer, mw.AdminConfigFromEnv()),
		)
	}

	if limiter != nil {
		chain = append(chain, mw.WithRateLimit(mw.RateLimitConfig{
			Limiter: limiter,
			KeyFunc: mw.IPOnlyRateKey,
		}))
	}

	chain = append(chain, mw.WithLogging())

	return chain
}
