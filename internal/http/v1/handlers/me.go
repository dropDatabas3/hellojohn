/*
me.go — Endpoint de introspección “ligera” del Access Token (parse JWT y devuelve claims seleccionadas)

Qué hace este handler
---------------------
Este handler implementa un endpoint simple para “ver quién soy” a partir del access token.
En vez de depender del middleware `RequireAuth` (que parsea JWT y mete claims en el contexto),
este handler hace el parse/validación del JWT por su cuenta usando `github.com/golang-jwt/jwt/v5`.

Ruta que maneja
--------------
- GET /v1/me

Entrada / salida
---------------
- Requiere header:
	- `Authorization: Bearer <access_token>`

- Respuesta 200 (JSON):
	{
		"sub": <claim sub>,
		"tid": <claim tid>,
		"aud": <claim aud>,
		"amr": <claim amr>,
		"custom": <claim custom>,
		"exp": <claim exp>
	}

Errores relevantes:
- 405 method_not_allowed si no es GET
- 401 missing_bearer si falta Authorization: Bearer
- 401 invalid_token si el token no valida firma/issuer o claims inválidas

================================================================================
Flujo paso a paso (GET /v1/me)
================================================================================
1) Validación de método
	 - Sólo GET.
	 - Si no: `httpx.WriteError(405, "method_not_allowed", ...)`.

2) Extracción de bearer token
	 - Lee `Authorization`.
	 - Verifica prefijo `bearer ` (case-insensitive).
	 - Si falta: 401 `missing_bearer`.

3) Parse y validación de JWT
	 - Usa `jwtv5.Parse(raw, c.Issuer.Keyfunc(), ...)`.
	 - Restricciones aplicadas:
		 - `WithValidMethods([]string{"EdDSA"})`
		 - `WithIssuer(c.Issuer.Iss)`
	 - Si falla o `!tk.Valid`: 401 `invalid_token`.

4) Extracción de claims
	 - Espera `jwtv5.MapClaims`.
	 - Si el tipo no coincide: 401 `invalid_token`.

5) Respuesta
	 - `Content-Type: application/json; charset=utf-8`
	 - Encode de un objeto con claims seleccionadas (sin transformar tipos).

================================================================================
Dependencias reales
================================================================================
- `app.Container`:
	- Usa `c.Issuer` (issuer string y Keyfunc para validar firma).

- Internas:
	- `internal/http/v1` como `httpx` para `WriteError`.

- Externas:
	- `github.com/golang-jwt/jwt/v5` para parse/validación.

No usa `Store`, `TenantSQLManager`, `cpctx.Provider`, `Cache`, `JWKSCache`.

================================================================================
Seguridad / invariantes
================================================================================
- Valida:
	- método de firma (EdDSA)
	- issuer (iss) exacto según `c.Issuer.Iss`

- NO valida explícitamente:
	- audience (aud)
	- scopes
	- expiración más allá de lo que haga el parser/claims estándar (depende del contenido y del uso de MapClaims)

- Filtración controlada:
	- Devuelve `custom` completo tal cual viene en el token.
	- Si el token contiene datos sensibles en `custom`, este endpoint los refleja.

================================================================================
Patrones detectados (GoF / arquitectura)
================================================================================
- “Inline auth” / duplicación de middleware:
	- Este handler reimplementa en pequeño lo que `RequireAuth` ya hace (parsear JWT y extraer claims).

- Controller-only:
	- No hay “service” ni “repo”; es puro parseo y response.

No hay concurrencia.

================================================================================
Cosas no usadas / legacy / riesgos
================================================================================
- Duplicación con `internal/http/v1/middleware.RequireAuth`:
	- En el codebase ya existe un mecanismo estándar para validar tokens y obtener claims del contexto.
	- Este endpoint podría ser un “legacy convenience” o un debug endpoint mantenido por compat.

- Tipos de claims:
	- `aud/amr/custom` se devuelven como `any` sin normalización; clientes deben tolerar múltiples shapes.

- Issuer único:
	- Valida issuer contra `c.Issuer.Iss` global. Si el sistema usa issuer efectivo por tenant
		(IssuerModePath / override), este endpoint podría comportarse distinto a otros flujos.
		(Depende de cómo se emiten los tokens en login/refresh y qué iss llevan.)

================================================================================
Ideas para V2 (sin decidir nada)
================================================================================
1) Unificar auth parsing
	 - Convertir `/v2/me` en un handler que dependa de middleware auth estándar:
		 - `RequireAuth` (o equivalente v2) + `GetClaims(ctx)`.
	 - Evita duplicación y asegura mismas reglas de validación en todo el stack.

2) Definir contrato de salida
	 - DTO claro (y estable) para “me”:
		 - sub, tid, aud, amr, exp
		 - y opcionalmente un subset de `custom` (o “sys namespace” controlado)
	 - Evitar exponer `custom` completo si contiene flags internos.

3) Validaciones adicionales
	 - Considerar validación de `aud` (si el endpoint se usa por clients específicos).
	 - Considerar enforcement de scopes (si /me debe requerir alguno).

Guía de “desarme” en capas
--------------------------
- Controller:
	- Método + extracción bearer + response.
- Auth component:
	- Validación de JWT + claims parsing (idealmente middleware compartido).
- DTO:
	- Definir shape estable del JSON de salida.

Resumen
-------
- `me.go` implementa `GET /v1/me` y devuelve un subset de claims del access token.
- Valida EdDSA + issuer, pero no usa el middleware estándar de auth y puede duplicar lógica.
- Es un buen candidato a normalizar en V2 alrededor de `RequireAuth` + `GetClaims` y un DTO estable.
*/

package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
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
