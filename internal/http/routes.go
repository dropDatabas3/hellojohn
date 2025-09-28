package http

import (
	stdhttp "net/http"
	"os"
)

func NewMux(
	// Core / health
	jwksHandler stdhttp.Handler,
	authLoginHandler stdhttp.Handler,
	authRegisterHandler stdhttp.Handler,
	authRefreshHandler stdhttp.Handler,
	authLogoutHandler stdhttp.Handler,
	meHandler stdhttp.Handler,
	readyz stdhttp.Handler,

	// OIDC
	oidcDiscovery stdhttp.Handler,
	oauthAuthorize stdhttp.Handler,
	oauthToken stdhttp.Handler,
	userInfo stdhttp.Handler,

	// Adicionales
	oauthRevoke stdhttp.Handler,
	sessionLogin stdhttp.Handler,
	sessionLogout stdhttp.Handler,
	consentAccept stdhttp.Handler, // POST /v1/auth/consent/accept

	// Email Flows
	verifyEmailStartHandler stdhttp.Handler, // POST /v1/auth/verify-email/start
	verifyEmailConfirmHandler stdhttp.Handler, // GET  /v1/auth/verify-email
	forgotHandler stdhttp.Handler, // POST /v1/auth/forgot
	resetHandler stdhttp.Handler, // POST /v1/auth/reset

	// OAuth2 introspection
	oauthIntrospect stdhttp.Handler, // POST /oauth2/introspect
	authLogoutAll stdhttp.Handler, // POST /v1/auth/logout-all

	// MFA TOTP
	mfaEnroll stdhttp.Handler, // POST /v1/mfa/totp/enroll
	mfaVerify stdhttp.Handler, // POST /v1/mfa/totp/verify
	mfaChallenge stdhttp.Handler, // POST /v1/mfa/totp/challenge
	mfaDisable stdhttp.Handler, // POST /v1/mfa/totp/disable
	mfaRecoveryRotate stdhttp.Handler, // POST /v1/mfa/recovery/rotate

	// Social exchange
	socialExchange stdhttp.Handler, // POST /v1/auth/social/exchange

	// ─── Admin clásicos ───
	adminScopes stdhttp.Handler,
	adminConsents stdhttp.Handler,
	adminClients stdhttp.Handler,

	// ─── Admin RBAC (NEW) ───
	adminRBACUsers stdhttp.Handler, // /v1/admin/rbac/users/{userID}/roles (GET/POST)
	adminRBACRoles stdhttp.Handler, // /v1/admin/rbac/roles/{role}/perms (GET/POST)

	// ─── Admin Users (disable/enable) ───
	adminUsers stdhttp.Handler, // POST /v1/admin/users/disable | /v1/admin/users/enable
) *stdhttp.ServeMux {
	mux := stdhttp.NewServeMux()

	// Health
	mux.HandleFunc("/healthz", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/readyz", readyz)

	// JWKS
	mux.Handle("/.well-known/jwks.json", jwksHandler)

	// OIDC Discovery
	mux.Handle("/.well-known/openid-configuration", oidcDiscovery)

	// OAuth2/OIDC
	mux.Handle("/oauth2/authorize", oauthAuthorize)
	mux.Handle("/oauth2/token", oauthToken)
	mux.Handle("/oauth2/revoke", oauthRevoke)
	mux.Handle("/oauth2/introspect", oauthIntrospect)
	mux.Handle("/userinfo", userInfo)

	// Auth “clásica”
	mux.Handle("/v1/auth/login", authLoginHandler)
	mux.Handle("/v1/auth/register", authRegisterHandler)
	mux.Handle("/v1/auth/refresh", authRefreshHandler)
	mux.Handle("/v1/auth/logout", authLogoutHandler)
	mux.Handle("/v1/me", meHandler)

	// Logout all
	mux.Handle("/v1/auth/logout-all", authLogoutAll)

	// Cookie session para /oauth2/authorize
	mux.Handle("/v1/session/login", sessionLogin)
	mux.Handle("/v1/session/logout", sessionLogout)

	// Consent (SPA/automatizable; sin rol admin)
	mux.Handle("/v1/auth/consent/accept", consentAccept)

	// Email flows
	mux.Handle("/v1/auth/verify-email/start", verifyEmailStartHandler) // POST
	mux.Handle("/v1/auth/verify-email", verifyEmailConfirmHandler)     // GET
	mux.Handle("/v1/auth/forgot", forgotHandler)                       // POST
	mux.Handle("/v1/auth/reset", resetHandler)                         // POST

	// MFA TOTP
	mux.Handle("/v1/mfa/totp/enroll", mfaEnroll)
	mux.Handle("/v1/mfa/totp/verify", mfaVerify)
	mux.Handle("/v1/mfa/totp/challenge", mfaChallenge)
	mux.Handle("/v1/mfa/totp/disable", mfaDisable)
	mux.Handle("/v1/mfa/recovery/rotate", mfaRecoveryRotate)

	// Social exchange
	mux.Handle("/v1/auth/social/exchange", socialExchange)

	// ─── Admin Scopes ───
	mux.Handle("/v1/admin/scopes", adminScopes)
	mux.Handle("/v1/admin/scopes/", adminScopes) // para DELETE con path param

	// ─── Admin Consents ───
	mux.Handle("/v1/admin/consents", adminConsents)
	mux.Handle("/v1/admin/consents/", adminConsents)
	mux.Handle("/v1/admin/consents/by-user", adminConsents)
	mux.Handle("/v1/admin/consents/by-user/", adminConsents)

	// ─── Admin Clients ───
	mux.Handle("/v1/admin/clients", adminClients)
	mux.Handle("/v1/admin/clients/", adminClients)

	// ─── Admin RBAC (NEW) ───
	// GET/POST /v1/admin/rbac/users/{userID}/roles
	mux.Handle("/v1/admin/rbac/users/", adminRBACUsers)
	// GET/POST /v1/admin/rbac/roles/{role}/perms
	mux.Handle("/v1/admin/rbac/roles/", adminRBACRoles)

	// ─── Admin Users (disable/enable) ───
	mux.Handle("/v1/admin/users/disable", adminUsers) // POST
	mux.Handle("/v1/admin/users/enable", adminUsers)  // POST

	// Debug social (sólo si SOCIAL_DEBUG_LOG=1)
	mux.HandleFunc("/v1/auth/social/debug/code", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if os.Getenv("SOCIAL_DEBUG_LOG") != "1" {
			w.WriteHeader(404)
			return
		}
		if r.Method != stdhttp.MethodGet {
			w.WriteHeader(405)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			w.WriteHeader(400)
			_, _ = w.Write([]byte("missing code"))
			return
		}
		_, _ = w.Write([]byte("cache introspection no implementada en este scope"))
	})

	return mux
}
