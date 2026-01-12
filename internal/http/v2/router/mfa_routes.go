package router

import (
	"net/http"

	authctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/auth"
	mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"
	storev2 "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// MFARouterDeps contiene las dependencias para el router MFA.
type MFARouterDeps struct {
	MFATOTPController *authctrl.MFATOTPController
	DAL               storev2.DataAccessLayer // Required for tenant resolution
	RateLimiter       mw.RateLimiter          // Rate limiter opcional
	AuthMiddleware    mw.Middleware           // RequireAuth middleware (valida JWT)
}

// RegisterMFARoutes registra rutas MFA V2.
// Estas rutas requieren usuario autenticado (via JWT).
func RegisterMFARoutes(mux *http.ServeMux, deps MFARouterDeps) {
	c := deps.MFATOTPController
	if c == nil {
		return // MFA not configured
	}

	// POST /v2/mfa/totp/enroll - Start TOTP enrollment
	mux.Handle("/v2/mfa/totp/enroll", mfaHandler(deps, http.HandlerFunc(c.Enroll), true))

	// POST /v2/mfa/totp/verify - Confirm TOTP enrollment
	mux.Handle("/v2/mfa/totp/verify", mfaHandler(deps, http.HandlerFunc(c.Verify), true))

	// POST /v2/mfa/totp/challenge - Complete MFA challenge (no JWT auth, mfa_token driven)
	mux.Handle("/v2/mfa/totp/challenge", mfaHandler(deps, http.HandlerFunc(c.Challenge), false))

	// POST /v2/mfa/totp/disable - Disable TOTP (requires password + 2FA)
	mux.Handle("/v2/mfa/totp/disable", mfaHandler(deps, http.HandlerFunc(c.Disable), true))

	// POST /v2/mfa/recovery/rotate - Rotate recovery codes (requires password + 2FA)
	mux.Handle("/v2/mfa/recovery/rotate", mfaHandler(deps, http.HandlerFunc(c.RotateRecovery), true))
}

// mfaHandler crea el middleware chain para endpoints MFA.
// Orden: Recover → RequestID → TenantResolution → RequireTenant → [Auth] → SecurityHeaders → NoStore → RateLimit → Logging
func mfaHandler(deps MFARouterDeps, handler http.Handler, requireAuth bool) http.Handler {
	chain := []mw.Middleware{
		mw.WithRecover(),
		mw.WithRequestID(),
	}

	// Tenant resolution (required for MFA)
	if deps.DAL != nil {
		chain = append(chain, mw.WithTenantResolution(deps.DAL, false)) // required, not optional
		chain = append(chain, mw.RequireTenant())
	}

	// Auth middleware (validates JWT, sets claims in context)
	if requireAuth && deps.AuthMiddleware != nil {
		chain = append(chain, deps.AuthMiddleware)
	}

	chain = append(chain,
		mw.WithSecurityHeaders(),
		mw.WithNoStore(), // MFA responses contain sensitive data
	)

	// Rate limiting por IP si está configurado
	if deps.RateLimiter != nil {
		chain = append(chain, mw.WithRateLimit(mw.RateLimitConfig{
			Limiter: deps.RateLimiter,
			KeyFunc: mw.IPOnlyRateKey,
		}))
	}

	// Logging al final
	chain = append(chain, mw.WithLogging())

	return mw.Chain(handler, chain...)
}
