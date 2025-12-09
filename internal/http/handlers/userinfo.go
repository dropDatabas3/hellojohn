package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
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
			w.Header().Set("WWW-Authenticate", `Bearer realm="userinfo", error="invalid_token", error_description="missing bearer token"`)
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "falta Authorization: Bearer <token>", 2301)
			return
		}
		raw := strings.TrimSpace(ah[len("Bearer "):])
		// Validar firma usando Keyfunc que busca por KID en active/retiring keys
		tk, err := jwtv5.Parse(raw, c.Issuer.Keyfunc(), jwtv5.WithValidMethods([]string{"EdDSA"}))
		if err != nil || !tk.Valid {
			// DEBUG: Loguear razón del fallo
			rawPrefix := raw
			if len(rawPrefix) > 20 {
				rawPrefix = rawPrefix[:20]
			}
			log.Printf("userinfo_invalid_token_debug: err=%v raw_prefix=%s", err, rawPrefix)

			w.Header().Set("WWW-Authenticate", `Bearer realm="userinfo", error="invalid_token", error_description="token inválido o expirado"`)
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "token inválido o expirado", 2302)
			return
		}
		claims, ok := tk.Claims.(jwtv5.MapClaims)
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="userinfo", error="invalid_token", error_description="claims inválidos"`)
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "claims inválidos", 2303)
			return
		}

		// Resolver issuer esperado del tenant y compararlo con iss del token
		issStr, _ := claims["iss"].(string)
		if issStr != "" && cpctx.Provider != nil {
			// Derivar slug desde iss path: .../t/{slug}
			slug := ""
			if u, err := url.Parse(issStr); err == nil {
				parts := strings.Split(strings.Trim(u.Path, "/"), "/")
				for i := 0; i < len(parts)-1; i++ {
					if parts[i] == "t" && i+1 < len(parts) {
						slug = parts[i+1]
					}
				}
				if slug == "" && len(parts) > 0 {
					slug = parts[len(parts)-1]
				}
			}
			if slug != "" {
				if ten, err := cpctx.Provider.GetTenantBySlug(r.Context(), slug); err == nil && ten != nil {
					expected := jwtx.ResolveIssuer(c.Issuer.Iss, ten.Settings.IssuerMode, ten.Slug, ten.Settings.IssuerOverride)
					if expected != issStr {
						w.Header().Set("WWW-Authenticate", `Bearer realm="userinfo", error="invalid_token", error_description="issuer mismatch"`)
						httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "issuer mismatch", 2304)
						return
					}
				}
			}
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

		// Always fetch user to get custom_fields for CompleteProfile flow
		// Email fields are gated by scope, but custom_fields are always returned

		// Resolver store correcto (Global vs Tenant) basado en 'tid'
		userStore := c.Store // Default Global
		tid, _ := claims["tid"].(string)
		if tid != "" && c.TenantSQLManager != nil {
			// tid podría ser UUID o slug. Intentamos resolver a slug.
			tenantSlug := tid
			if cpctx.Provider != nil {
				// Si es UUID, buscar el slug correspondiente
				if tenants, err := cpctx.Provider.ListTenants(r.Context()); err == nil {
					for _, t := range tenants {
						if t.ID == tid {
							tenantSlug = t.Slug
							break
						}
					}
				}
			}
			if tStore, errS := c.TenantSQLManager.GetPG(r.Context(), tenantSlug); errS == nil && tStore != nil {
				userStore = tStore
			}
		}

		u, err := userStore.GetUserByID(r.Context(), sub)
		if err == nil && u != nil {
			// Standard OIDC Claims
			if u.Name != "" {
				resp["name"] = u.Name
			}
			if u.GivenName != "" {
				resp["given_name"] = u.GivenName
			}
			if u.FamilyName != "" {
				resp["family_name"] = u.FamilyName
			}
			if u.Picture != "" {
				resp["picture"] = u.Picture
			}
			if u.Locale != "" {
				resp["locale"] = u.Locale
			}

			// Email fields only if scope allows
			if hasScope("email") {
				resp["email"] = u.Email
				resp["email_verified"] = u.EmailVerified
			}
			// Always include custom_fields for CompleteProfile flow
			// Merge Metadata["custom_fields"] and u.CustomFields
			finalCF := make(map[string]any)

			// 1. From Metadata (Legacy or non-column fields)
			if u.Metadata != nil {
				if cf, ok := u.Metadata["custom_fields"].(map[string]any); ok {
					for k, v := range cf {
						finalCF[k] = v
					}
				}
			}

			// 2. From Dynamic Columns (u.CustomFields) - These take precedence or add to the map
			if u.CustomFields != nil {
				for k, v := range u.CustomFields {
					finalCF[k] = v
				}
			}

			resp["custom_fields"] = finalCF
		} else if err == core.ErrNotFound {
			// User not found - return empty custom_fields
			resp["custom_fields"] = map[string]any{}
		} else if err != nil {
			log.Printf("userinfo: GetUserByID error: %v", err)
			resp["custom_fields"] = map[string]any{}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Add("Vary", "Authorization")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
