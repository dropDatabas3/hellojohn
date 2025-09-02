package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

func NewMeHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET", 1000)
			return
		}
		ah := strings.TrimSpace(r.Header.Get("Authorization"))
		if ah == "" || !strings.HasPrefix(strings.ToLower(ah), "bearer ") {
			httpx.WriteError(w, http.StatusUnauthorized, "missing_bearer", "falta Authorization: Bearer <token>", 1105)
			return
		}
		raw := strings.TrimSpace(ah[len("Bearer "):])

		tk, err := jwtv5.Parse(raw, c.Issuer.Keyfunc(),
			jwtv5.WithValidMethods([]string{"EdDSA"}),
			jwtv5.WithIssuer(c.Issuer.Iss),
		)
		if err != nil || !tk.Valid {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "token inválido o expirado", 1103)
			return
		}

		claims, ok := tk.Claims.(jwtv5.MapClaims)
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "claims inválidos", 1103)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sub":    claims["sub"],
			"tid":    claims["tid"],
			"aud":    claims["aud"],
			"amr":    claims["amr"],
			"custom": claims["custom"],
			"exp":    claims["exp"],
		})
	}
}
