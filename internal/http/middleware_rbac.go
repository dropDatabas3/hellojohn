package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/claims"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

func hasAny(haystack []string, needles []string) bool {
	if len(haystack) == 0 || len(needles) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(haystack))
	for _, v := range haystack {
		set[strings.ToLower(strings.TrimSpace(v))] = struct{}{}
	}
	for _, n := range needles {
		if _, ok := set[strings.ToLower(strings.TrimSpace(n))]; ok {
			return true
		}
	}
	return false
}

func extractSysNS(c *app.Container, r *http.Request) (map[string]any, jwtv5.MapClaims, bool) {
	ah := strings.TrimSpace(r.Header.Get("Authorization"))
	if ah == "" {
		return nil, nil, false
	}
	// tolerant “Bearer xxx” (case-insensitive)
	var raw string
	if i := strings.IndexByte(ah, ' '); i > 0 && strings.EqualFold(ah[:i], "Bearer") {
		raw = strings.TrimSpace(ah[i+1:])
	}
	if raw == "" {
		return nil, nil, false
	}
	tk, err := jwtv5.Parse(raw, c.Issuer.Keyfunc(),
		jwtv5.WithValidMethods([]string{"EdDSA"}),
		jwtv5.WithIssuer(c.Issuer.Iss),
		jwtv5.WithLeeway(30*time.Second),
	)
	if err != nil || !tk.Valid {
		return nil, nil, false
	}
	claimsMap, ok := tk.Claims.(jwtv5.MapClaims)
	if !ok {
		return nil, nil, false
	}
	custom, _ := claimsMap["custom"].(map[string]any)
	if custom == nil {
		return nil, claimsMap, false
	}
	ns := claims.SystemNamespace(c.Issuer.Iss)
	sysNS, _ := custom[ns].(map[string]any)
	return sysNS, claimsMap, sysNS != nil
}

func RequireRole(c *app.Container, roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sysNS, _, ok := extractSysNS(c, r)
			if !ok {
				// 401 para token inválido/ausente
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
				WriteError(w, http.StatusUnauthorized, "unauthorized", "token inválido o ausente", 1900)
				return
			}
			var have []string
			if v, _ := sysNS["roles"].([]any); len(v) > 0 {
				for _, i := range v {
					if s, ok := i.(string); ok {
						have = append(have, s)
					}
				}
			} else if v2, _ := sysNS["roles"].([]string); len(v2) > 0 {
				have = v2
			}
			if !hasAny(have, roles) {
				WriteError(w, http.StatusForbidden, "forbidden", "rol insuficiente", 1902)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequirePerm(c *app.Container, perms ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sysNS, _, ok := extractSysNS(c, r)
			if !ok {
				// 401 para token inválido/ausente
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
				WriteError(w, http.StatusUnauthorized, "unauthorized", "token inválido o ausente", 1900)
				return
			}
			var have []string
			if v, _ := sysNS["perms"].([]any); len(v) > 0 {
				for _, i := range v {
					if s, ok := i.(string); ok {
						have = append(have, s)
					}
				}
			} else if v2, _ := sysNS["perms"].([]string); len(v2) > 0 {
				have = v2
			}
			if !hasAny(have, perms) {
				WriteError(w, http.StatusForbidden, "forbidden", "permiso insuficiente", 1912)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
