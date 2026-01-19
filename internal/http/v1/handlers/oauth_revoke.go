/*
oauth_revoke.go — OAuth2 Token Revocation (RFC 7009-ish) para refresh opaco (hash+DB) con inputs flexibles

Qué es este archivo (la posta)
------------------------------
Este archivo define `NewOAuthRevokeHandler(c)` que implementa un endpoint de “revocación” estilo RFC 7009:
- Acepta un `token` a revocar (principalmente **refresh token opaco** de HelloJohn).
- Hace la operación **idempotente** y “no filtrante”: si no existe, igual responde OK.
- Solo revoca refresh tokens persistidos en DB (lookup por hash SHA256 Base64URL).
- Es deliberadamente permisivo con el formato de entrada (form / bearer / JSON) para compat con distintos clientes.

Rutas soportadas / contrato
---------------------------
- POST (la ruta la define el router externo; el handler no fija path)
  Entrada (cualquiera de estas):
  1) x-www-form-urlencoded: `token=<...>`  (+ opcional `token_type_hint`, se ignora)
  2) Header: `Authorization: Bearer <token>` (fallback si no vino en form)
  3) JSON `{ "token": "..." }` (fallback si Content-Type incluye application/json)

Respuestas:
- 405 JSON si no es POST.
- 400 JSON si no hay token (input mal formado).
- 200 OK siempre que el input sea “bien formado”, haya o no token en DB (RFC7009 behavior).

Flujo paso a paso
-----------------
1) Validación de método:
   - Solo POST, si no => `httpx.WriteError(..., 405, ..., 1000)`.
2) Defensa de tamaño:
   - `r.Body = http.MaxBytesReader(..., 32<<10)` (32KB) antes de parsear.
3) Parseo primario:
   - `r.ParseForm()` y lee `token` de `r.PostForm.Get("token")`.
4) Fallbacks para token:
   - Fallback #1: `Authorization: Bearer ...` si no había token en el form.
   - Fallback #2: si `Content-Type` contiene `application/json`, intenta decode de `{token}` con `io.LimitReader` (32KB).
5) Validación:
   - Si token sigue vacío => 400 `invalid_request`.
6) Revocación (best effort, no filtrante):
   - Calcula `hash := tokens.SHA256Base64URL(token)`.
   - `c.Store.GetRefreshTokenByHash(ctx, hash)`:
     - Si existe => `_ = c.Store.RevokeRefreshToken(ctx, rt.ID)` (ignora error).
     - Si error != nil y != core.ErrNotFound => se ignora (para no filtrar info).
7) Respuesta final:
   - Setea `Cache-Control: no-store` y `Pragma: no-cache`.
   - `w.WriteHeader(200)` siempre (si el input fue válido).

Dependencias (reales)
---------------------
- `c.Store`:
  - `GetRefreshTokenByHash(ctx, hash)` para buscar refresh token persistido.
  - `RevokeRefreshToken(ctx, rt.ID)` para marcar revocado.
- `tokens.SHA256Base64URL(token)`:
  - Hash del token opaco (formato consistente con cómo se persisten refresh en DB).
- `httpx.WriteError(...)`:
  - Wrapper de errores JSON con códigos internos.
- `core.ErrNotFound`:
  - Permite distinguir “no existe” vs error real, pero igual no se expone nada.

Seguridad / invariantes
-----------------------
- No filtración:
  - Nunca revela si el token existía o no (200 OK igual).
  - Ignora errores internos (salvo “input inválido”), evitando side-channels.
- Limitación de payload:
  - 32KB evita DoS por bodies gigantes (bien).
- No-store:
  - Correcto para proxies/cache.

OJO / Riesgos (cosas a marcar)
------------------------------
- No autentica al cliente que revoca:
  - RFC 7009 normalmente requiere auth del cliente (o algún criterio).
  - Acá cualquiera que pegue al endpoint con un refresh token válido podría revocarlo.
  - Capaz es intencional (logout “self-service”), pero aumenta superficie si se filtra un RT.
- Solo revoca refresh opaco persistido:
  - Si te pasan un access JWT o un refresh de otro formato, no hace nada pero responde 200.
  - Está ok por idempotencia, pero conviene dejarlo explícito a nivel contrato.
- `token_type_hint` ignorado:
  - Bien para idempotencia y simpleza, pero si querés compat RFC completa, podrías usarlo como hint para evitar lookup inútil.

Patrones detectados
-------------------
- Tolerant Reader / Robust Input Handling:
  - Soporta múltiples formatos de entrada (form/header/json) como “Adapter” de request.
- Idempotent Command:
  - Revocar es “best effort” y responde OK aunque ya esté revocado/no exista.

Ideas para V2 (sin decidir nada) + guía de desarme en capas
-----------------------------------------------------------
1) Controller / DTO
- Controller minimal:
  - `ExtractToken(r) -> (token, ok, errCode)` (unificar los 3 caminos).
  - Devuelve 400 si no hay token, 200 si hay token (sin decir nada más).

2) Service (negocio)
- `RevocationService.Revoke(ctx, token, clientContext)`
  - Clasifica token por formato (refresh opaque vs jwt).
  - Si refresh opaque: lookup + revoke.
  - Si JWT: (opcional) revocar vía denylist/jti store si existe.
  - Siempre retorna éxito lógico (para idempotencia), y loggea internamente errores inesperados.

3) Ports/Repos
- `RefreshTokenRepository`:
  - `GetByHash(ctx, hash)`
  - `RevokeByID(ctx, id)`
  - (opcional) `RevokeByHash(ctx, hash)` para evitar 2 llamadas.
- `ClientAuthenticator` (si decidís exigir auth):
  - Similar a `clientBasicAuth` que ya usan otros oauth_* handlers.
  - Permite exigir client auth, o permitir modo “public” solo para ciertos flows.

4) Seguridad extra opcional
- Si querés hacerlo más estricto:
  - Requerir client auth por default.
  - O permitir sin auth solo si el token viene del mismo “session context” (cookie/session) y el request está autenticado.
- Agregar rate-limit por IP / token hash prefix (para evitar brute).

Concurrencia
------------
No suma meter goroutines: es 1 lookup + 1 update. Lo importante es robustez y auth, no paralelismo.

Resumen
-------
- Handler chico, claro y práctico: parsea token de 3 formas, hashea, busca refresh en DB y revoca si existe.
- Es idempotente y no filtra información (bien), pero hoy no valida quién está autorizado a revocar (ojo si el endpoint queda público).
*/

package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/v1/core"
)

func NewOAuthRevokeHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10) // 32KB alcanza
		if err := r.ParseForm(); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "form inválido", 2301)
			return
		}
		token := strings.TrimSpace(r.PostForm.Get("token"))
		// Fallback 1: Authorization: Bearer <token>
		if token == "" {
			if h := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(h), "bearer ") {
				token = strings.TrimSpace(h[len("Bearer "):])
			}
		}
		// Fallback 2: JSON {"token":"..."}
		if token == "" && strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
			var body struct {
				Token string `json:"token"`
			}
			if err := json.NewDecoder(io.LimitReader(r.Body, 32<<10)).Decode(&body); err == nil {
				token = strings.TrimSpace(body.Token)
			}
		}
		if token == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "token es obligatorio", 2302)
			return
		}
		// token_type_hint puede venir o no; ignoramos para idempotencia
		// Semántica: si existe y es refresh, lo revocamos; si no, igual 200.
		hash := tokens.SHA256Base64URL(token)
		if rt, err := c.Store.GetRefreshTokenByHash(r.Context(), hash); err == nil && rt != nil {
			_ = c.Store.RevokeRefreshToken(r.Context(), rt.ID)
		} else if err != nil && err != core.ErrNotFound {
			// errores inesperados no deben filtrar información
		}
		// RFC 7009: 200 OK siempre que el input sea bien formado
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		w.WriteHeader(http.StatusOK)
	}
}
