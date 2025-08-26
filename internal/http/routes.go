package http

import (
	stdhttp "net/http"
)

func NewMux(jwksHandler stdhttp.Handler, authLoginHandler stdhttp.Handler, authRegisterHandler stdhttp.Handler) *stdhttp.ServeMux {
	mux := stdhttp.NewServeMux()

	mux.HandleFunc("/healthz", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle("/.well-known/jwks.json", jwksHandler)
	mux.Handle("/v1/auth/login", authLoginHandler)
	mux.Handle("/v1/auth/register", authRegisterHandler)

	return mux
}
