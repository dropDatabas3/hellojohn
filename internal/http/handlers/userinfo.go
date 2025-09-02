package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

func NewUserInfoHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/POST", 1000)
			return
		}
		ah := strings.TrimSpace(r.Header.Get("Authorization"))
		if ah == "" || !strings.HasPrefix(strings.ToLower(ah), "bearer ") {
			httpx.WriteError(w, http.StatusUnauthorized, "missing_bearer", "falta Authorization: Bearer <token>", 2301)
			return
		}
		raw := strings.TrimSpace(ah[len("Bearer "):])
		tk, err := jwtv5.Parse(raw, c.Issuer.Keyfunc(),
			jwtv5.WithValidMethods([]string{"EdDSA"}), jwtv5.WithIssuer(c.Issuer.Iss))
		if err != nil || !tk.Valid {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "token inválido o expirado", 2302)
			return
		}
		claims, ok := tk.Claims.(jwtv5.MapClaims)
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "claims inválidos", 2303)
			return
		}

		sub, _ := claims["sub"].(string)
		resp := map[string]any{"sub": sub}

		var scopes []string
		if v, ok := claims["scp"].([]any); ok {
			for _, i := range v {
				if s, ok := i.(string); ok {
					scopes = append(scopes, s)
				}
			}
		} else if s, ok := claims["scope"].(string); ok {
			scopes = strings.Fields(s)
		}
		hasScope := func(want string) bool {
			for _, s := range scopes {
				if strings.EqualFold(s, want) {
					return true
				}
			}
			return false
		}

		if hasScope("email") {
			u, err := c.Store.GetUserByID(r.Context(), sub)
			if err == nil && u != nil {
				resp["email"] = u.Email
				resp["email_verified"] = u.EmailVerified
			} else if err == core.ErrNotFound {
			} else if err != nil {
			}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
