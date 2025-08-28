package handlers

import (
	"net/http"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
)

// NewJWKSHandler expone el JWKS público a partir de las llaves del Issuer en el container.
// Sirve application/json y permite cachearlo (public, max-age=600).
func NewJWKSHandler(c *app.Container) http.Handler {
	// No cacheamos en memoria aquí porque el JWKS ya lo devuelve listo en JSON desde las llaves.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		// Permite algo de caching intermedio (rotación de claves es rara; cuando ocurra se invalida por cambio de KID).
		w.Header().Set("Cache-Control", "public, max-age=600, must-revalidate")
		w.Header().Set("Expires", time.Now().Add(10*time.Minute).UTC().Format(http.TimeFormat))

		// Emitimos el JWKS actual desde el emisor
		jwks := c.Issuer.Keys.JWKSJSON()
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jwks)
	})
}
