package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	claimsNS "github.com/dropDatabas3/hellojohn/internal/claims"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
				"client_id":  rt.ClientIDText,
				"exp":        rt.ExpiresAt.Unix(),
				"iat":        rt.IssuedAt.Unix(), // IssuedAt existe; no CreatedAt
			}
			httpx.WriteJSON(w, http.StatusOK, resp)
			return
		}

		// Caso 2: access JWT firmado (EdDSA).
		// Validar firma per-tenant (derivar tenant desde claims/iss) y luego comparar issuer esperado.
		parsed, err := jwtv5.Parse(tok, c.Issuer.KeyfuncFromTokenClaims(), jwtv5.WithValidMethods([]string{"EdDSA"}))
		if err != nil || !parsed.Valid {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"active": false})
			return
		}
		claims, ok := parsed.Claims.(jwtv5.MapClaims)
		if !ok {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"active": false})
			return
		}

		expF, _ := claims["exp"].(float64)
		iatF, _ := claims["iat"].(float64)
		amr, _ := claims["amr"].([]any)
		sub, _ := claims["sub"].(string)
		clientID, _ := claims["aud"].(string)
		// Aceptar tanto "scope" como "scp" (espacio-separado)
		scopeRaw, _ := claims["scope"].(string)
		if scopeRaw == "" {
			if scp, ok := claims["scp"].(string); ok {
				scopeRaw = scp
			}
		}
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

		// Verificar que el issuer del token coincida con el esperado para el slug derivado de iss (modo path)
		if iss, ok := claims["iss"].(string); ok && iss != "" && cpctx.Provider != nil {
			parts := strings.Split(strings.Trim(iss, "/"), "/")
			slug := ""
			for i := 0; i < len(parts)-1; i++ {
				if parts[i] == "t" && i+1 < len(parts) {
					slug = parts[i+1]
				}
			}
			if slug == "" && len(parts) > 0 {
				slug = parts[len(parts)-1]
			}
			if slug != "" {
				if ten, err := cpctx.Provider.GetTenantBySlug(r.Context(), slug); err == nil && ten != nil {
					expected := jwtx.ResolveIssuer(c.Issuer.Iss, ten.Settings.IssuerMode, ten.Slug, ten.Settings.IssuerOverride)
					if expected != iss {
						active = false
					}
				}
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

		// Si ?include_sys=1, exponemos roles/perms del namespace de sistema cuando el token está activo.
		if active {
			if v := r.URL.Query().Get("include_sys"); v == "1" || strings.EqualFold(v, "true") {
				var roles, perms []string
				if m, ok := claims["custom"].(map[string]any); ok {
					// 1) clave recomendada (namespace de sistema)
					if sys, ok := m[claimsNS.SystemNamespace(c.Issuer.Iss)].(map[string]any); ok {
						if rr, ok := sys["roles"].([]any); ok {
							for _, it := range rr {
								if s, ok := it.(string); ok && s != "" {
									roles = append(roles, s)
								}
							}
						} else if rr2, ok := sys["roles"].([]string); ok {
							roles = append(roles, rr2...)
						}
						if pp, ok := sys["perms"].([]any); ok {
							for _, it := range pp {
								if s, ok := it.(string); ok && s != "" {
									perms = append(perms, s)
								}
							}
						} else if pp2, ok := sys["perms"].([]string); ok {
							perms = append(perms, pp2...)
						}
					} else if sys2, ok := m[c.Issuer.Iss].(map[string]any); ok {
						// 2) compat: algunos flows guardaron bajo issuer plano
						if rr, ok := sys2["roles"].([]any); ok {
							for _, it := range rr {
								if s, ok := it.(string); ok && s != "" {
									roles = append(roles, s)
								}
							}
						} else if rr2, ok := sys2["roles"].([]string); ok {
							roles = append(roles, rr2...)
						}
						if pp, ok := sys2["perms"].([]any); ok {
							for _, it := range pp {
								if s, ok := it.(string); ok && s != "" {
									perms = append(perms, s)
								}
							}
						} else if pp2, ok := sys2["perms"].([]string); ok {
							perms = append(perms, pp2...)
						}
					}
				}
				resp["roles"] = roles
				resp["perms"] = perms
			}
		}

		// Validación ligera de formato UUID en sub si parece UUID (no aborta en error)
		if _, err := uuid.Parse(sub); err != nil { /* ignore */
		}

		httpx.WriteJSON(w, http.StatusOK, resp)
	}
}
