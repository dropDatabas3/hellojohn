package router

import (
	"net/http"

	ctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/auth"
	mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"
)

// AuthRouterDeps contiene las dependencias para el router de auth.
type AuthRouterDeps struct {
	Controllers *ctrl.Controllers
	RateLimiter mw.RateLimiter // Opcional: rate limiter por IP
}

// RegisterAuthRoutes registra rutas de autenticación V2.
func RegisterAuthRoutes(mux *http.ServeMux, deps AuthRouterDeps) {
	c := deps.Controllers

	// POST /v2/auth/login
	mux.Handle("/v2/auth/login", authLoginHandler(deps.RateLimiter, http.HandlerFunc(c.Login.Login)))
}

// authLoginHandler crea el middleware chain para login.
// Login es especial: tenant viene en body, no en path/header.
func authLoginHandler(limiter mw.RateLimiter, handler http.Handler) http.Handler {
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
