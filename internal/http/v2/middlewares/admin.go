package middlewares

import (
	"net/http"
	"os"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/claims"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/errors"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
)

// =================================================================================
// ADMIN MIDDLEWARES
// =================================================================================

// AdminConfig configura el comportamiento de los middlewares de admin.
type AdminConfig struct {
	// EnforceAdmin si es true, requiere que el usuario sea admin.
	// Si es false (modo desarrollo), siempre permite.
	EnforceAdmin bool
	// AdminSubs lista de user IDs que son admin por defecto (fallback de emergencia)
	AdminSubs []string
}

// AdminConfigFromEnv carga la configuración desde variables de entorno.
func AdminConfigFromEnv() AdminConfig {
	cfg := AdminConfig{
		EnforceAdmin: strings.TrimSpace(os.Getenv("ADMIN_ENFORCE")) == "1",
	}
	if csv := strings.TrimSpace(os.Getenv("ADMIN_SUBS")); csv != "" {
		for _, p := range strings.Split(csv, ",") {
			if s := strings.TrimSpace(p); s != "" {
				cfg.AdminSubs = append(cfg.AdminSubs, s)
			}
		}
	}
	return cfg
}

// RequireAdmin valida que el usuario tenga permisos de admin.
// Reglas (en este orden):
//  1. Si ADMIN_ENFORCE != "1": permitir (modo compatible por defecto).
//  2. Si custom.is_admin == true => permitir.
//  3. Si custom.roles incluye "admin" => permitir.
//  4. Si el sub (user id) está en ADMIN_SUBS (lista CSV) => permitir.
//     Si no, 403.
func RequireAdmin(cfg AdminConfig) Middleware {
	adminSubs := make(map[string]struct{})
	for _, s := range cfg.AdminSubs {
		adminSubs[s] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.EnforceAdmin {
				next.ServeHTTP(w, r)
				return
			}

			cl := GetClaims(r.Context())
			if cl == nil {
				errors.WriteError(w, errors.ErrUnauthorized.WithDetail("no claims in context"))
				return
			}

			// custom.is_admin
			if cust := ClaimMap(cl, "custom"); cust != nil {
				if v, ok := cust["is_admin"].(bool); ok && v {
					next.ServeHTTP(w, r)
					return
				}
				// custom.roles: ["admin", ...]
				if arr := ClaimStringSlice(cust, "roles"); len(arr) > 0 {
					for _, role := range arr {
						if strings.EqualFold(role, "admin") {
							next.ServeHTTP(w, r)
							return
						}
					}
				}
			}

			// Admin por SUB (fallback por env)
			if sub := ClaimString(cl, "sub"); sub != "" {
				if _, ok := adminSubs[sub]; ok {
					next.ServeHTTP(w, r)
					return
				}
			}

			errors.WriteError(w, errors.ErrForbidden.WithDetail("admin required"))
		})
	}
}

// RequireSysAdmin valida admin del SISTEMA usando el namespace anclado al issuer.
// Reglas:
//  1. Si ADMIN_ENFORCE != "1": permitir (modo dev/compat).
//  2. Leer custom[SYS_NS].is_admin == true => permitir.
//  3. Leer custom[SYS_NS].roles incluye "sys:admin" => permitir.
//  4. Fallback de emergencia: sub ∈ ADMIN_SUBS => permitir.
func RequireSysAdmin(issuer *jwtx.Issuer, cfg AdminConfig) Middleware {
	adminSubs := make(map[string]struct{})
	for _, s := range cfg.AdminSubs {
		adminSubs[s] = struct{}{}
	}
	sysNS := claims.SystemNamespace(issuer.Iss)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.EnforceAdmin {
				next.ServeHTTP(w, r)
				return
			}

			cl := GetClaims(r.Context())
			if cl == nil {
				errors.WriteError(w, errors.ErrUnauthorized.WithDetail("no claims in context"))
				return
			}

			if cust := ClaimMap(cl, "custom"); cust != nil {
				if sysMap := ClaimMap(cust, sysNS); sysMap != nil {
					if v, ok := sysMap["is_admin"].(bool); ok && v {
						next.ServeHTTP(w, r)
						return
					}
					if rs := ClaimStringSlice(sysMap, "roles"); len(rs) > 0 {
						for _, role := range rs {
							if strings.EqualFold(role, "sys:admin") {
								next.ServeHTTP(w, r)
								return
							}
						}
					}
				}
			}

			// Fallback ADMIN_SUBS
			if sub := ClaimString(cl, "sub"); sub != "" {
				if _, ok := adminSubs[sub]; ok {
					next.ServeHTTP(w, r)
					return
				}
			}

			errors.WriteError(w, errors.ErrForbidden.WithDetail("sys admin required"))
		})
	}
}
