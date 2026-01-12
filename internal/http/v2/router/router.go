// Package router contains the V2 route aggregator.
package router

import (
	"net/http"

	authctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/auth"
	mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"
	storev2 "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// V2RouterDeps contains all dependencies for the V2 router.
type V2RouterDeps struct {
	Mux *http.ServeMux

	// Data access
	DAL storev2.DataAccessLayer

	// Controllers
	AuthControllers *authctrl.Controllers

	// Middlewares
	AuthMiddleware mw.Middleware  // JWT validation middleware
	RateLimiter    mw.RateLimiter // Optional rate limiter
}

// RegisterV2Routes registers all V2 routes.
// This is the main entry point for V2 routing.
// Call this from app.go or equivalent main wiring file.
func RegisterV2Routes(deps V2RouterDeps) {
	mux := deps.Mux
	if mux == nil {
		return
	}

	// ===========================================================================
	// MFA Routes
	// ===========================================================================
	if deps.AuthControllers != nil && deps.AuthControllers.MFATOTP != nil {
		RegisterMFARoutes(mux, MFARouterDeps{
			MFATOTPController: deps.AuthControllers.MFATOTP,
			DAL:               deps.DAL,
			RateLimiter:       deps.RateLimiter,
			AuthMiddleware:    deps.AuthMiddleware,
		})
	}

	// ===========================================================================
	// Other route registrations go here...
	// ===========================================================================
	// Example:
	// RegisterAuthRoutes(mux, AuthRouterDeps{...})
	// RegisterOAuthRoutes(mux, OAuthRouterDeps{...})
	// RegisterSessionRoutes(mux, SessionRouterDeps{...})
}
