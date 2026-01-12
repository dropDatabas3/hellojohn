package router

import (
	"net/http"

	ctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/auth"
	mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
)

// AuthRouterDeps contiene las dependencias para el router de auth.
type AuthRouterDeps struct {
	Controllers *ctrl.Controllers
	RateLimiter mw.RateLimiter // Opcional: rate limiter por IP
	Issuer      *jwtx.Issuer   // Para endpoints que requieren auth
}

// RegisterAuthRoutes registra rutas de autenticación V2.
func RegisterAuthRoutes(mux *http.ServeMux, deps AuthRouterDeps) {
	c := deps.Controllers

	// POST /v2/auth/login
	mux.Handle("/v2/auth/login", authHandler(deps.RateLimiter, http.HandlerFunc(c.Login.Login)))

	// POST /v2/auth/register
	mux.Handle("/v2/auth/register", authHandler(deps.RateLimiter, http.HandlerFunc(c.Register.Register)))

	// POST /v2/auth/refresh
	mux.Handle("/v2/auth/refresh", authHandler(deps.RateLimiter, http.HandlerFunc(c.Refresh.Refresh)))

	// GET /v2/auth/config
	mux.Handle("/v2/auth/config", authHandler(deps.RateLimiter, http.HandlerFunc(c.Config.GetConfig)))

	// GET /v2/auth/providers
	mux.Handle("/v2/auth/providers", authHandler(deps.RateLimiter, http.HandlerFunc(c.Providers.GetProviders)))

	// POST /v2/auth/complete-profile (requires auth)
	mux.Handle("/v2/auth/complete-profile", authedHandler(deps.RateLimiter, deps.Issuer, http.HandlerFunc(c.CompleteProfile.CompleteProfile)))

	// GET /v2/me (requires auth)
	mux.Handle("/v2/me", authedHandler(deps.RateLimiter, deps.Issuer, http.HandlerFunc(c.Me.Me)))

	// GET /v2/profile (requires auth + scope profile:read)
	mux.Handle("/v2/profile", scopedHandler(deps.RateLimiter, deps.Issuer, "profile:read", http.HandlerFunc(c.Profile.GetProfile)))

	// POST /v2/auth/logout
	mux.Handle("/v2/auth/logout", authHandler(deps.RateLimiter, http.HandlerFunc(c.Logout.Logout)))

	// POST /v2/auth/logout-all
	mux.Handle("/v2/auth/logout-all", authHandler(deps.RateLimiter, http.HandlerFunc(c.Logout.LogoutAll)))
}

// authHandler crea el middleware chain para endpoints de auth públicos.
// Estos endpoints son especiales: tenant viene en body, no en path/header.
func authHandler(limiter mw.RateLimiter, handler http.Handler) http.Handler {
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

// authedHandler crea el middleware chain para endpoints que requieren autenticación.
func authedHandler(limiter mw.RateLimiter, issuer *jwtx.Issuer, handler http.Handler) http.Handler {
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

	// Auth required
	chain = append(chain, mw.RequireAuth(issuer))

	// Logging al final
	chain = append(chain, mw.WithLogging())

	return mw.Chain(handler, chain...)
}

// scopedHandler crea el middleware chain para endpoints que requieren auth + scope específico.
func scopedHandler(limiter mw.RateLimiter, issuer *jwtx.Issuer, scope string, handler http.Handler) http.Handler {
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

	// Auth required
	chain = append(chain, mw.RequireAuth(issuer))

	// Scope required
	chain = append(chain, mw.RequireScope(scope))

	// Logging al final
	chain = append(chain, mw.WithLogging())

	return mw.Chain(handler, chain...)
}
