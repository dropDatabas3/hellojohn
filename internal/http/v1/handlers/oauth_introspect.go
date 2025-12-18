/*
oauth_introspect.go â€” OAuth2 Token Introspection (POST) con auth de cliente + soporte refresh opaco y access JWT (EdDSA)

QuÃ© es este archivo (la posta)
------------------------------
Este archivo define `NewOAuthIntrospectHandler(c, auth)` que expone un endpoint de introspecciÃ³n estilo RFC 7662:
- Exige **autenticaciÃ³n del cliente** (via `clientBasicAuth` inyectado).
- Recibe `token` por **x-www-form-urlencoded** (`r.ParseForm()`).
- Devuelve siempre **200 OK** con `{ "active": true|false, ... }` para tokens invÃ¡lidos (comportamiento tÃ­pico de introspection).
- Soporta 2 tipos de token:
  1) **Refresh token opaco** (nuestro formato) => lookup en DB por hash.
  2) **Access token JWT** (EdDSA) => valida firma usando keystore/JWKS y revisa expiraciÃ³n + issuer esperado.

Rutas soportadas / contrato
---------------------------
- POST (path depende del router; el handler no fija ruta)
  Content-Type: application/x-www-form-urlencoded
  Form:
    token=<string>
  Query opcional:
    include_sys=1|true  (solo si active=true: expone roles/perms del namespace â€œsystemâ€ dentro de claims custom)

Respuestas:
- 405 JSON si no es POST.
- 401 JSON si falla auth del cliente.
- 400 JSON si el form es invÃ¡lido o falta token.
- 200 JSON:
  - `{ "active": false }` si token invÃ¡lido / no encontrado / firma invÃ¡lida / expired / issuer mismatch.
  - `{ "active": true, ... }` con campos segÃºn tipo de token.

Flujo paso a paso (secuencia real)
----------------------------------
1) ValidaciÃ³n HTTP bÃ¡sica:
   - MÃ©todo: POST.
   - Headers: setea `Cache-Control: no-store` y `Pragma: no-cache` siempre.
2) AuthN del cliente:
   - `auth.ValidateClientAuth(r)` debe dar ok.
   - Nota: devuelve (tenantID, clientID) pero acÃ¡ se ignora (solo se usa el ok).
3) Parseo del request:
   - `r.ParseForm()`
   - Lee `token` de `r.PostForm.Get("token")`.
4) Ruta A: refresh token opaco (heurÃ­stica por formato)
   - CondiciÃ³n: `len(tok) >= 40` y **no contiene "."**.
   - Hash: `tokens.SHA256Base64URL(tok)`.
   - Busca en DB global: `c.Store.GetRefreshTokenByHash(ctx, hash)`.
   - Si no existe / error => `{active:false}`.
   - Si existe:
     - `active := rt.RevokedAt == nil && rt.ExpiresAt.After(time.Now().UTC())`
     - Responde:
       - `token_type=refresh_token`
       - `sub` = rt.UserID
       - `client_id` = rt.ClientIDText
       - `exp`/`iat` desde `ExpiresAt`/`IssuedAt`
5) Ruta B: access token JWT (EdDSA)
   - `jwtv5.Parse(tok, c.Issuer.KeyfuncFromTokenClaims(), WithValidMethods(["EdDSA"]))`
     - Keyfunc â€œderivaâ€ la key desde claims (por tenant/issuer/kid) vÃ­a keystore/JWKS.
   - Si parse/firma invÃ¡lida => `{active:false}`.
   - Extrae claims relevantes:
     - `exp`, `iat` (float64), `sub`, `aud`(client_id), `scope` o `scp`, `tid`, `acr`, `amr[]`.
   - `active := exp > now`.
   - ValidaciÃ³n extra de issuer (multi-tenant / issuer mode):
     - Si hay `iss` y existe `cpctx.Provider`, intenta derivar `slug` desde `iss` (modo path / heurÃ­stica de `/t/<slug>/...`).
     - Carga tenant por slug y calcula issuer esperado:
       `jwtx.ResolveIssuer(c.Issuer.Iss, string(ten.Settings.IssuerMode), ten.Slug, ten.Settings.IssuerOverride)`
     - Si `expected != iss` => `active=false`.
   - Construye respuesta:
     - `token_type=access_token`
     - `sub`, `client_id`, `scope` (string), `exp`, `iat`, `amr` normalizado, `tid`
     - opcional: `acr`, `jti`, `iss`
6) Extras opcionales: `include_sys`
   - Si `active=true` y query `include_sys=1|true`:
     - Busca `claims["custom"]` y dentro:
       1) Namespace recomendado: `claimsNS.SystemNamespace(c.Issuer.Iss)` (map con `roles` y `perms`)
       2) Fallback compat: clave `c.Issuer.Iss` directo (legacy)
     - Normaliza roles/perms aceptando `[]any` o `[]string`.
7) Fin:
   - Hace un `uuid.Parse(sub)` â€œbest effortâ€ pero **no falla** si no es UUID (solo ignora).
   - Responde JSON 200.

Dependencias (reales) y cÃ³mo se usan
------------------------------------
- `clientBasicAuth` (inyectado):
  - Port para validar auth del cliente (probablemente Basic Auth o similar).
  - AcÃ¡ se usa solo como â€œgateâ€ (no se cruza contra el token).
- `c.Store`:
  - Requerido para refresh token introspection: `GetRefreshTokenByHash`.
  - OJO: es global store (no per-tenant).
- `c.Issuer`:
  - `KeyfuncFromTokenClaims()` para validar JWT con keystore/JWKS (segÃºn claims/kid).
  - `c.Issuer.Iss` para namespace system y para resolver issuer esperado.
- `cpctx.Provider`:
  - Fuente de verdad para buscar tenant por slug y traer settings (IssuerMode / Override).
- `jwtx.ResolveIssuer(...)`:
  - Recalcula el issuer esperado segÃºn modo (global/path/domain) y override.
- `claimsNS.SystemNamespace(...)`:
  - ConvenciÃ³n de nombres para ubicar claims del â€œsystem namespaceâ€ (roles/perms).
- `tokens.SHA256Base64URL`:
  - Hash del refresh opaco (y criterio de lookup).

Seguridad / invariantes (lo crÃ­tico)
------------------------------------
- Introspection protegido por auth de cliente:
  - Si no hay auth => 401 invalid_client.
  - (Pero) no valida que el `client_id` autenticado coincida con el `aud`/`client_id` del token JWT ni con `rt.ClientIDText`.
  - En introspection â€œformalâ€ eso suele ser esperado: el cliente solo puede introspectar tokens propios.
- Refresh token lookup:
  - Usa hash SHA256 (no almacena el token crudo) => ok.
  - Marca active=false si revocado o expirado.
- JWT access token:
  - Valida firma EdDSA con keyfunc basada en claims (multi-tenant ready).
  - Chequea exp.
  - Chequeo extra de issuer esperado por tenant (reduce riesgo de aceptar tokens con issuer â€œparecidoâ€).
- No-store/no-cache:
  - Bien puesto para que proxies no cacheen introspection.

Patrones detectados
-------------------
- Strategy / Port-Adapter:
  - `clientBasicAuth` como puerto para auth; permite cambiar implementaciÃ³n sin tocar handler.
- Dual-path parsing (heurÃ­stica por formato):
  - â€œopaque refresh vs JWTâ€ por presencia de '.' y longitud.
- Policy gate por issuer-mode:
  - Recalcula issuer esperado usando settings del tenant (control-plane) y lo compara.

Cosas no usadas / legacy / riesgos
----------------------------------
- Riesgo #1 (alto): introspection no ata el token al cliente autenticado.
  - JWT: responde `client_id` desde `aud` pero no verifica contra el cliente autenticado.
  - Refresh: devuelve datos del refresh sin verificar ownership.
  â‡’ Cualquier cliente con credenciales vÃ¡lidas del introspection endpoint podrÃ­a consultar tokens ajenos.
- HeurÃ­stica refresh opaco:
  - `len >= 40` + â€œsin puntosâ€ puede matchear otros tokens opacos (o algÃºn JWT raro sin '.').
  - Si maÃ±ana cambiÃ¡s formato de refresh, esto se rompe.
- DerivaciÃ³n de slug desde `iss`:
  - Es heurÃ­stica por split de path; si cambian rutas de issuer, puede dejar de validar bien.
  - Si no puede derivar slug, directamente no aplica la comparaciÃ³n expected vs iss (queda â€œbest effortâ€).
- `tenantID, clientID` de `ValidateClientAuth` se ignoran:
  - Parece que estaban pensados para aplicar validaciones extra (ownership) pero quedÃ³ a medio camino.

Ideas para V2 (sin decidir nada) + guÃ­a de desarme en capas
-----------------------------------------------------------
Objetivo: que introspection sea consistente, multi-tenant real, y con autorizaciÃ³n correcta.

1) DTO / Controller
- Controller minimal:
  - parse method + form (`token`, opcionales flags)
  - llama a service y devuelve JSON

2) Services (lÃ³gica)
- `TokenClassifier`:
  - `Detect(token) -> TokenKind{RefreshOpaque, AccessJWT, Unknown}` (sin heurÃ­stica mÃ¡gica hardcodeada)
- `RefreshIntrospectionService`:
  - `IntrospectRefresh(ctx, rawToken) -> IntrospectionResult`
  - verifica revocaciÃ³n/expiraciÃ³n
  - (importante) valida ownership vs client autenticado
- `JWTIntrospectionService`:
  - `ParseAndValidate(ctx, jwtRaw) -> claims + active`
  - issuer check: sacar â€œparse slugâ€ a una funciÃ³n `TenantFromIssuer(iss)` o usar claim `tid`/`tenant` si existe.
  - ownership: valida `aud` vs client autenticado (y si es array aud, soportarlo).
- `SystemClaimsProjector`:
  - `ExtractRolesPerms(claims) -> roles, perms` (con compat legacy).

3) Repos / Ports
- `ClientAuth` (ya existe) pero devolver identidad completa:
  - tenantID, clientID, authMethod, ok
- `TenantResolver`:
  - resolver tenant desde `tid` claim primero (mÃ¡s confiable) y fallback a `iss`.
- `RefreshTokenRepository` con mÃ©todo tenant/client aware:
  - `GetByHash(ctx, tenantID, clientID, hash)` o por lo menos â€œcheck ownershipâ€.

4) Patrones GoF aplicables
- Strategy:
  - `IntrospectionStrategy` por tipo de token (refresh vs jwt).
- Chain of Responsibility:
  - pipeline de validaciones (parse -> signature -> issuer -> expiry -> ownership -> projection).
- Adapter:
  - adaptar los distintos formatos de claims (`aud` string vs array) y `scope` vs `scp`.

Concurrencia
------------
No hay ganancia real usando goroutines: es 1 lookup DB o 1 parse JWT + 1 lookup control-plane.
Si querÃ©s optimizar:
- PodÃ©s hacer issuer expected + parse claims en paralelo, pero la complejidad no lo vale.
Mejor: caching corto del tenant resolve (slug->settings) si esto pega muy seguido.

Resumen
-------
- Introspect endpoint sÃ³lido en â€œactive false on invalidâ€, con soporte refresh opaco + JWT EdDSA y chequeo de issuer esperado.
- Punto rojo: falta autorizaciÃ³n de ownership (atar token al cliente autenticado) y hay heurÃ­sticas/compat legacy que conviene encapsular.
*/

package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	claimsNS "github.com/dropDatabas3/hellojohn/internal/claims"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
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
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "form invÃ¡lido", 2602)
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
					expected := jwtx.ResolveIssuer(c.Issuer.Iss, string(ten.Settings.IssuerMode), ten.Slug, ten.Settings.IssuerOverride)
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

		// Si ?include_sys=1, exponemos roles/perms del namespace de sistema cuando el token estÃ¡ activo.
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

		// ValidaciÃ³n ligera de formato UUID en sub si parece UUID (no aborta en error)
		if _, err := uuid.Parse(sub); err != nil { /* ignore */
		}

		httpx.WriteJSON(w, http.StatusOK, resp)
	}
}
