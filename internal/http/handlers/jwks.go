package handlers

import (
	"net/http"
)

func NewJWKSHandler(jwksJSON []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jwksJSON)
	}
}
