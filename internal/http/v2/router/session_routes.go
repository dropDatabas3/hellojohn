package router

import (
	"net/http"

	ctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/session"
	mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// SessionRouterDeps contiene las dependencias para el router session.
type SessionRouterDeps struct {
	Controllers *ctrl.Controllers
	RateLimiter mw.RateLimiter        // Opcional: rate limiter por IP
	DAL         store.DataAccessLayer // DAL para tenant resolution
}

// RegisterSessionRoutes registra rutas de session V2.
func RegisterSessionRoutes(mux *http.ServeMux, deps SessionRouterDeps) {
	c := deps.Controllers

	// POST /v2/session/logout - Session cookie logout
	mux.Handle("/v2/session/logout", sessionHandler(deps.RateLimiter, http.HandlerFunc(c.Logout.Logout)))

	// POST /v2/session/login - Session cookie login (requires tenant resolution)
	mux.Handle("/v2/session/login", sessionLoginHandler(deps, http.HandlerFunc(c.Login.Login)))
}

// sessionHandler crea el middleware chain para endpoints de session.
func sessionHandler(limiter mw.RateLimiter, handler http.Handler) http.Handler {
	chain := []mw.Middleware{
		mw.WithRecover(),
		mw.WithRequestID(),
		mw.WithSecurityHeaders(),
		mw.WithNoStore(),
	}

	// Rate limiting por IP si está configurado
	if limiter != nil {
		chain = append(chain, mw.WithRateLimit(mw.RateLimitConfig{
			Limiter: limiter,
			KeyFunc: mw.IPOnlyRateKey,
		}))
	}

	// Logging al final
	chain = append(chain, mw.WithLogging())

	return mw.Chain(handler, chain...)
}

// sessionLoginHandler crea el middleware chain para login que necesita tenant resolution.
func sessionLoginHandler(deps SessionRouterDeps, handler http.Handler) http.Handler {
	chain := []mw.Middleware{
		mw.WithRecover(),
		mw.WithRequestID(),
		mw.WithSecurityHeaders(),
		mw.WithNoStore(),
	}

	// Rate limiting por IP si está configurado
	if deps.RateLimiter != nil {
		chain = append(chain, mw.WithRateLimit(mw.RateLimitConfig{
			Limiter: deps.RateLimiter,
			KeyFunc: mw.IPOnlyRateKey,
		}))
	}

	// Tenant resolution (optional - from request body tenant_id/client_id)
	if deps.DAL != nil {
		chain = append(chain, mw.WithTenantResolution(deps.DAL, true))
	}

	// Logging al final
	chain = append(chain, mw.WithLogging())

	return mw.Chain(handler, chain...)
}
