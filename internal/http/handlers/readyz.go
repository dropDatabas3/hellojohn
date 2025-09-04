package handlers

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

func NewReadyzHandler(c *app.Container, checkRedis func(ctx context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if v := os.Getenv("SERVICE_VERSION"); v != "" {
			w.Header().Set("X-Service-Version", v)
		}
		if git := os.Getenv("SERVICE_COMMIT"); git != "" {
			w.Header().Set("X-Service-Commit", git)
		}
		if c != nil && c.Issuer != nil && c.Issuer.Keys != nil {
			if kid, err := c.Issuer.ActiveKID(); err == nil && kid != "" {
				w.Header().Set("X-JWKS-KID", kid)
			}
		}

		// 1) DB
		if err := c.Store.Ping(r.Context()); err != nil {
			// Log interno con detalle
			// (import "log")
			log.Printf(`{"level":"error","msg":"db_unavailable","err":"%v"}`, err)
			httpx.WriteError(w, http.StatusServiceUnavailable, "db_unavailable", "database unavailable", 2001)
			return
		}

		// 2) Self-check EdDSA: firmar y verificar un JWT efímero en memoria
		now := time.Now().UTC()
		claims := jwtv5.MapClaims{
			"iss": c.Issuer.Iss,
			"sub": "selfcheck",
			"aud": "health",
			"iat": now.Unix(),
			"nbf": now.Unix(),
			"exp": now.Add(60 * time.Second).Unix(),
		}
		signed, _, err := c.Issuer.SignRaw(claims)
		if err != nil {
			httpx.WriteError(w, http.StatusServiceUnavailable, "sign_failed", "no se pudo firmar self-check", 2004)
			return
		}

		parsed, err := jwtv5.Parse(signed, c.Issuer.Keyfunc(),
			jwtv5.WithValidMethods([]string{"EdDSA"}),
			jwtv5.WithIssuer(c.Issuer.Iss),
		)
		if err != nil || !parsed.Valid {
			httpx.WriteError(w, http.StatusServiceUnavailable, "verify_failed", "self-check: verificación falló", 2005)
			return
		}
		if cl, ok := parsed.Claims.(jwtv5.MapClaims); ok {
			if s, _ := cl["sub"].(string); s != "selfcheck" {
				httpx.WriteError(w, http.StatusServiceUnavailable, "verify_failed", "self-check: sub inesperado", 2006)
				return
			}
			switch a := cl["aud"].(type) {
			case string:
				if a != "health" {
					httpx.WriteError(w, http.StatusServiceUnavailable, "verify_failed", "self-check: aud inesperado", 2007)
					return
				}
			case []any:
				okAud := false
				for _, it := range a {
					if s, _ := it.(string); s == "health" {
						okAud = true
						break
					}
				}
				if !okAud {
					httpx.WriteError(w, http.StatusServiceUnavailable, "verify_failed", "self-check: aud inesperado", 2007)
					return
				}
			}
		} else {
			httpx.WriteError(w, http.StatusServiceUnavailable, "verify_failed", "self-check: claims inválidos", 2008)
			return
		}

		// 3) Redis (opcional)
		if checkRedis != nil {
			if err := checkRedis(r.Context()); err != nil {
				log.Printf(`{"level":"error","msg":"redis_unavailable","err":"%v"}`, err)
				httpx.WriteError(w, http.StatusServiceUnavailable, "redis_unavailable", "redis unavailable", 2003)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}
}
