package http

import (
	stdhttp "net/http"
)

func NewMux(
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

	// NUEVOS
	oauthRevoke stdhttp.Handler,
	sessionLogin stdhttp.Handler,
	sessionLogout stdhttp.Handler,
) *stdhttp.ServeMux {
	mux := stdhttp.NewServeMux()

	mux.HandleFunc("/healthz", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/readyz", readyz)
	mux.Handle("/.well-known/jwks.json", jwksHandler)

	// OIDC Discovery
	mux.Handle("/.well-known/openid-configuration", oidcDiscovery)

	// OAuth2/OIDC
	mux.Handle("/oauth2/authorize", oauthAuthorize)
	mux.Handle("/oauth2/token", oauthToken)
	mux.Handle("/oauth2/revoke", oauthRevoke)
	mux.Handle("/userinfo", userInfo)

	// Autenticación clásica
	mux.Handle("/v1/auth/login", authLoginHandler)
	mux.Handle("/v1/auth/register", authRegisterHandler)
	mux.Handle("/v1/auth/refresh", authRefreshHandler)
	mux.Handle("/v1/auth/logout", authLogoutHandler)
	mux.Handle("/v1/me", meHandler)

	// Sesión por cookie (para /oauth2/authorize)
	mux.Handle("/v1/session/login", sessionLogin)
	mux.Handle("/v1/session/logout", sessionLogout)

	return mux
}
