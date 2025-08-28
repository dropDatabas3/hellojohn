package handlers

import (
	"context"
	"net/http"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
)

func NewReadyzHandler(c *app.Container, checkRedis func(ctx context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := c.Store.Ping(r.Context()); err != nil {
			httpx.WriteError(w, http.StatusServiceUnavailable, "db_unavailable", err.Error(), 2001)
			return
		}
		if c.Issuer == nil || c.Issuer.Keys == nil || c.Issuer.Keys.Priv == nil {
			httpx.WriteError(w, http.StatusServiceUnavailable, "keys_unavailable", "no hay claves cargadas", 2002)
			return
		}
		if checkRedis != nil {
			if err := checkRedis(r.Context()); err != nil {
				httpx.WriteError(w, http.StatusServiceUnavailable, "redis_unavailable", err.Error(), 2003)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}
}
