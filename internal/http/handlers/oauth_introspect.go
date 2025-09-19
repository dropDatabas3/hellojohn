package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
)

type clientBasicAuth interface {
	ValidateClientAuth(r *http.Request) (tenantID string, clientID string, ok bool)
}

func NewOAuthIntrospectHandler(c *app.Container, auth clientBasicAuth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 2600)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")

		if _, _, ok := auth.ValidateClientAuth(r); !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "auth requerida", 2601)
			return
		}

		if err := r.ParseForm(); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "form inválido", 2602)
			return
		}
		tok := strings.TrimSpace(r.PostForm.Get("token"))
		if tok == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "falta token", 2603)
			return
		}

		// Caso 1: refresh opaco (nuestro formato)
		if len(tok) >= 40 && !strings.Contains(tok, ".") {
			hash := tokens.SHA256Base64URL(tok)
			rt, err := c.Store.GetRefreshTokenByHash(r.Context(), hash)
			if err != nil || rt == nil {
				httpx.WriteJSON(w, http.StatusOK, map[string]any{"active": false})
				return
			}
			active := rt.RevokedAt == nil && rt.ExpiresAt.After(time.Now().UTC())
			resp := map[string]any{
				"active":     active,
				"token_type": "refresh_token",
				"sub":        rt.UserID, // string
				"client_id":  rt.ClientID,
				"exp":        rt.ExpiresAt.Unix(),
				"iat":        rt.IssuedAt.Unix(), // IssuedAt existe; no CreatedAt
			}
			httpx.WriteJSON(w, http.StatusOK, resp)
			return
		}

		// Caso 2: access JWT firmado (EdDSA). Validar firma/issuer/exp.
		claims, err := jwtx.ParseEdDSA(tok, c.Issuer.Keys, c.Issuer.Iss)
		if err != nil {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"active": false})
			return
		}
		expF, _ := claims["exp"].(float64)
		iatF, _ := claims["iat"].(float64)
		amr, _ := claims["amr"].([]any)
		sub, _ := claims["sub"].(string)
		clientID, _ := claims["aud"].(string)
		scopeRaw, _ := claims["scope"].(string)
		tid, _ := claims["tid"].(string)
		acr, _ := claims["acr"].(string)
		var scope []string
		if scopeRaw != "" {
			scope = strings.Fields(scopeRaw)
		}
		active := time.Unix(int64(expF), 0).After(time.Now())
		// Normalizar AMR
		var amrVals []string
		for _, v := range amr {
			if s, ok := v.(string); ok {
				amrVals = append(amrVals, s)
			}
		}

		resp := map[string]any{
			"active":     active,
			"token_type": "access_token",
			"sub":        sub,
			"client_id":  clientID,
			"scope":      strings.Join(scope, " "),
			"exp":        int64(expF),
			"iat":        int64(iatF),
			"amr":        amrVals,
			"tid":        tid,
		}
		if acr != "" {
			resp["acr"] = acr
		}
		// Opcional: introspection puede incluir jti, iss, etc., si existen.
		if jti, ok := claims["jti"].(string); ok {
			resp["jti"] = jti
		}
		if iss, ok := claims["iss"].(string); ok {
			resp["iss"] = iss
		}
		// Validación ligera de formato UUID en sub si parece UUID.
		if _, err := uuid.Parse(sub); err != nil { /* ignore */
		}
		httpx.WriteJSON(w, http.StatusOK, resp)
	}
}
