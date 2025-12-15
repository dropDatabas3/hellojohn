/*
userinfo.go — review bien “a lo grande” (paths, capas, responsabilidades, qué separaría, y fixes concretos)

Qué es este handler
-------------------
Implementa el endpoint OIDC UserInfo:
  GET|POST /userinfo   (o el path que lo monte tu router)
que devuelve claims del usuario a partir de un Access Token (Bearer JWT).

En tu caso:
- exige Authorization: Bearer <jwt>
- valida firma EdDSA con `c.Issuer.Keyfunc()`
- (extra) verifica “issuer esperado” por tenant comparando `iss` vs issuer resuelto del tenant
- resuelve store correcto por `tid` (tenant DB vs global)
- busca user por `sub` y arma respuesta con:
   - claims estándar (name, given_name, family_name, picture, locale)
   - email/email_verified solo si scope incluye "email"
   - custom_fields siempre (merge de metadata + columnas dinámicas)

Rutas y formas
--------------
Métodos:
- GET /userinfo
- POST /userinfo
Ambos requieren:
- Header `Authorization: Bearer <token>`
Respuestas:
- 401 + WWW-Authenticate en casos invalid_token
- 200 JSON con claims

Capas (cómo debería estar separado)
-----------------------------------
Ahora todo está en un solo handler. Funciona, pero está mezclando responsabilidades.

Yo lo separaría así (sin cambiar funcionalidad):

1) transport/http (handlers)
   - parsear método + auth header
   - llamar a un servicio `UserInfoService`
   - setear headers (cache-control, content-type, vary, www-auth si falla)
   - serializar JSON

2) domain/service (lógica de negocio)
   - ValidateAccessToken(rawToken) -> Claims (o un struct tipado)
   - ResolveTenantFromClaims(iss, tid) -> tenantSlug + expectedIssuer
   - ValidateIssuerMatch(expected, tokenIss)
   - ResolveUserStore(tenantSlug / tid) -> repo
   - BuildUserInfoResponse(user, scopes) -> map[string]any o struct

3) infra/adapters
   - tenant resolver: `cpctx.Provider` (FS control plane)
   - store resolver: `TenantSQLManager.GetPG`
   - issuer resolver: `jwtx.ResolveIssuer(...)`

Esto te deja:
- test unitarios al service sin HTTP
- el handler se vuelve “finito”, no un monstruo

Puntos fuertes del código actual
--------------------------------
✅ Aceptar GET y POST: ok (OIDC lo permite; muchas libs usan GET).
✅ `Vary: Authorization` + `no-store/no-cache`: perfecto para tokens.
✅ Scope gating de `email` está bien pensado.
✅ custom_fields siempre: consistente con tu flujo de CompleteProfile.
✅ Verificación de issuer per-tenant: está buena para evitar tokens “firmados ok” pero con iss incorrecto.

Los “che, esto lo arreglaría YA”
--------------------------------

1) LOG de token inválido (leak / ruido)
   -----------------------------------
Esto:
  log.Printf("userinfo_invalid_token_debug: err=%v raw_prefix=%s", err, rawPrefix)

- aunque cortás a 20 chars, sigue siendo “material sensible” (y encima en logs).
- además el error puede incluir cosas del parsing.

Fix:
- logueá solo `err` y un request-id, o un hash del token:
    tokHash := tokens.SHA256Base64URL(raw)[:10]
- y hacerlo solo bajo flag de debug.

2) Keyfunc “global” vs Keyfunc “per-tenant”
   -----------------------------------------
Estás usando `c.Issuer.Keyfunc()` (lookup por KID en active/retiring keys). Bien.
Pero ojo con multi-tenant + rotación:
- si tu Keyfunc ya resuelve por KID global y eso incluye keys de varios tenants,
  igual después chequeás issuer, así que ok.
- si querés más duro: primero derivar tenant por `iss` sin confiar en firma...
  (pero sin firma tampoco confías en `iss`). Entonces este orden es aceptable:
  - validar firma -> claims -> validar issuer per-tenant.

3) Resolución de slug desde iss: fallback dudoso
   --------------------------------------------
Estás asumiendo que el slug está en el path o en el último segmento.
Si el issuer override es algo tipo `https://id.acme.com/oidc`, tu fallback “último segmento”
te va a inventar slug = "oidc" y podrías intentar buscar un tenant que no existe.
Hoy eso no falla duro (solo si encuentra tenant y mismatch), pero es raro.

Mejor:
- solo derivar slug si encontrás el patrón explícito `/t/{slug}`.
- si no está, no intentes inferir slug. (O usar `tid` en claims para resolver tenant).

4) Resolver tenant por tid hace ListTenants() (O(N))
   -------------------------------------------------
Este bloque:
  if tenants, err := cpctx.Provider.ListTenants(...); err == nil { for ... }

Para userinfo puede ser muy hot-path. Si tenés 1k tenants, es un bajón.

Fix:
- agregar en provider un método `GetTenantByID(ctx, id)` (ideal)
- o cachear un map id->slug (en memoria con TTL)
- o guardar `tslug` directo en el token (tipo claim `tslug`) y listo.

5) Scopes parsing: `scp` puede ser string en tu sistema
   ----------------------------------------------------
En otros handlers vos manejás:
- scope string
- scp string
- scp []string
Acá solo hacés:
- scp []any
- scope string
Si tu access token emite `scp` como string (en oauth_token hiciste a veces string y a veces []),
userinfo puede “no ver” scopes -> no devuelve email aunque debería.

Fix (robusto):
- aceptar:
  - scp []any
  - scp []string
  - scp string (space-separated)
  - scope string

6) Validar exp/nbf explícito (opcional, pero recomendado)
   -----------------------------------------------------
jwt.Parse con MapClaims por default suele validar exp/nbf si usás `jwt.WithLeeway` / options…
pero depende de cómo lo uses. En v5, `Parse` valida “registered claims” si son tipo Claims correcto,
con MapClaims a veces no es tan estricto como querés.

Para userinfo yo lo dejaría explícito:
- check exp
- check nbf si existe
- y check aud (si querés) o al menos que token sea access token (por ejemplo `token_use=access`).

7) `sub` vacío / no string
   ------------------------
Si sub no viene, respondés {"sub":""} y después intentás GetUserByID("").
Mejor:
- si sub == "" -> 401 invalid_token.

8) GET|POST: soportás POST pero no leés body
   -----------------------------------------
OIDC userinfo POST suele ser igual que GET (bearer token en header), así que no pasa nada.
Pero si alguien manda token en form body (some implementations), vos no lo aceptás.
Está ok si vos definís tu contrato así.

Cómo lo “partiría” (archivos y funciones)
-----------------------------------------
Ejemplo de estructura:

internal/oidc/userinfo/service.go
- type Service struct { Issuer, Provider, Store, TenantSQLManager... }
- func (s *Service) Handle(ctx, rawBearer string) (UserInfoResponse, *OIDCError)

internal/oidc/userinfo/token.go
- func ParseAndValidateAccessToken(raw string) (claims Claims, err error)
- type Claims struct { Sub, Tid, Iss string; Scopes []string; ... }

internal/oidc/userinfo/tenant.go
- func ResolveTenant(ctx, claims Claims) (tenantSlug string, expectedIssuer string, err error)

internal/oidc/userinfo/response.go
- func BuildResponse(u *core.User, scopes []string) map[string]any

internal/http/v1/handlers/userinfo.go
- parse header, call service, write headers, write JSON

Bonus: contratos/errores
------------------------
En vez de repetir WriteError + WWW-Authenticate a mano, hacé helper:
- httpx.WriteOIDCAuthError(w, realm, code, desc, status)

Así te queda consistente con token/introspect/etc.

Mini lista de “cambios” que metería en este mismo archivo sin refactor
----------------------------------------------------------------------
- No loguear raw_prefix.
- Validar `sub != ""` -> 401.
- Scope parsing robusto (scp string/list).
- Derivar slug solo si path tiene `/t/{slug}` (sin fallback al último segmento).
- Evitar ListTenants O(N): cache o método GetTenantByID.

Si querés, te lo reescribo “clean” (mismo comportamiento) ya con:
- parse scopes robusto
- check sub
- safer issuer slug derivation
- debug logging seguro
y lo dejamos listo para después extraer a service.

*/

package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
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
