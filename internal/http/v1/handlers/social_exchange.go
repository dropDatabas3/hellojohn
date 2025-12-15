/*
social_exchange.go — comentario/diagnóstico

Qué hace este endpoint
----------------------
Este handler implementa la “segunda pata” del flow social cuando usás **login_code**:
- En el callback social (ej: google), vos emitís tokens (access/refresh) pero en vez de devolvérselos directo
  al frontend, generás un `code` corto y lo guardás en cache con key:
    "social:code:<code>"
  y redirigís al `redirect_uri` del cliente con `?code=...`.

- Después el frontend (o el backend del cliente) pega a:
    POST /v1/auth/social/exchange   (asumo esta ruta)
  con JSON:
    { "code": "...", "client_id": "...", "tenant_id": "..."? }
  y este handler devuelve el JSON final `AuthLoginResponse` (tokens).

O sea: es un “token exchanger” one-shot basado en cache.

Flujo exacto del código
-----------------------
1) Solo POST.
2) Lee JSON a `SocialExchangeRequest`.
3) Valida: code y client_id obligatorios.
4) Busca en cache `social:code:<code>`.
   - Si no existe => 404 code_not_found.
5) Unmarshal payload en:
   { client_id, tenant_id, response(AuthLoginResponse) }.
6) Valida que `req.client_id` == `stored.client_id` (case-insensitive, trim).
7) Si vino `req.tenant_id`, valida que coincida con `stored.tenant_id`.
8) Borra del cache (one-shot) **recién después de validar**.
9) Devuelve 200 con `stored.Response`.

Cosas que están bien
--------------------
- One-shot correcto: borrás el code solo si pasa validación.
- No-store/no-cache headers (clave: no querés tokens en caches).
- `client_id` binding: evita que alguien robe un code y lo canjee desde otro cliente.
- Logs debug opcionales para E2E (bien, y están controlados por env).

Puntos flojos / riesgos reales
------------------------------

1) Falta límite de tamaño del body
   -------------------------------
   Acá no ponés `http.MaxBytesReader`. En otros endpoints sí.
   Solución: antes de `ReadJSON`, meté:
     r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
   con 32KB alcanza.

2) El error 404 “code_not_found” filtra existencia de codes (leve)
   ---------------------------------------------------------------
   RFC-style a veces prefiere responder 400 genérico para no dar señales,
   pero acá es relativamente inocuo porque el code es high entropy (randB64(32) en googleHandler).
   Igual, si querés “menos oracle”, devolvé 400 “invalid_grant” siempre.

3) No validás expiración acá (depende 100% del cache TTL)
   ------------------------------------------------------
   En tu diseño, el TTL del cache define expiración, perfecto.
   Pero si el cache backend tiene bugs o TTL distinto, no hay segunda validación.

   Opcional: guardar en payload un `exp` y chequearlo acá también.

4) TenantID opcional => puede permitir canje cross-tenant si se reusa code (raro pero…)
   -----------------------------------------------------------------------------------
   Vos validás tenant_id *solo si lo mandan*.
   Si el cliente no lo manda, entonces solo validás client_id.
   Si un mismo client_id existiera en más de un tenant (depende tu modelo),
   podrías terminar aceptando canje “cruzado”.

   Si en tu sistema `client_id` es global-unique, entonces no hay drama.
   Si no lo es (por tenant), entonces: tenant_id debería ser obligatorio.
   Alternativa más limpia: en vez de pedir tenant_id al cliente, sacalo del payload y listo:
   - devolvés siempre lo que está en cache, pero igual *log* si req.TenantID no coincide.
   (o directamente hacé requerido `tenant_id`).

5) Re-jugar code si el request se cae en el medio
   ----------------------------------------------
   Hoy borrás del cache antes de escribir JSON (ok) pero si el cliente se desconecta justo después,
   perdió la chance de reintentar.
   Esto es un tradeoff:
   - Seguridad > UX: como está, más seguro.
   - UX > seguridad: podrías “consumir” con un flag `used=true` y un TTL cortito post-consumo (5-10s)
     para permitir reintentos idempotentes desde el mismo client_id.
   Yo lo dejaría como está salvo que te esté jodiendo en producción.

6) Comparación case-insensitive de client_id
   -----------------------------------------
   `EqualFold` para client_id… depende.
   Si `client_id` se trata como identifier case-sensitive (muchos sistemas lo tratan así),
   podrías estar aflojando una regla.
   Igual no es una vulnerabilidad seria, pero por prolijidad:
   - o normalizás client_id a lower-case siempre en todo el sistema
   - o comparás exacto (strings.TrimSpace y listo)

7) No autenticás al cliente
   -------------------------
   Este endpoint no exige client secret ni mTLS ni nada.
   El “secreto” acá es el `code`.
   De nuevo: si el code es fuerte y de vida corta, está ok.
   Si querés subir la vara:
   - permitir Basic Auth del cliente (confidential) en exchange
   - o exigir `PKCE-like` extra (un verifier) almacenado en payload

Qué separaría / cómo lo ubicaría en capas
-----------------------------------------
Este archivo hoy está bien “handler puro”, pero si lo querés más limpio:

- handlers/social_exchange.go (HTTP):
  - parse JSON
  - valida input
  - llama a SocialCodeService.ExchangeCode(...)

- internal/auth/social/service.go:
  type SocialCodeService interface {
      ExchangeCode(ctx, code, clientID, tenantID string) (AuthLoginResponse, error)
  }
  - encapsula cache get/unmarshal/validaciones/borrado

- internal/auth/social/store_cache.go:
  - wrapper sobre c.Cache para:
    GetSocialCodePayload(code) / DeleteSocialCode(code)

Eso te da:
- unit tests fáciles del service sin HTTP
- el handler queda chiquito y consistente con tus otros handlers.

Quick wins (cambios mínimos que te recomiendo YA)
-------------------------------------------------
1) Limitar body:
   r.Body = http.MaxBytesReader(w, r.Body, 32<<10)

2) Si tu `client_id` NO es global-unique, hacé `tenant_id` requerido y listo:
   if req.TenantID == "" => 400.

3) (Opcional) Cambiar 404 por 400 "invalid_grant" si querés menos señalización.

En resumen
----------
`social_exchange.go` está correcto para el patrón “login_code one-shot”.
Lo más urgente es meter MaxBytesReader y decidir la política de `tenant_id` (opcional vs requerido)
según si tu `client_id` es global o por-tenant. Si es por-tenant, hoy estás medio jugado.
*/

package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
)

type SocialExchangeRequest struct {
	Code     string `json:"code"`
	ClientID string `json:"client_id"`
	TenantID string `json:"tenant_id,omitempty"`
}

func NewSocialExchangeHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1655)
			return
		}
		var req SocialExchangeRequest
		if !httpx.ReadJSON(w, r, &req) {
			return
		}
		req.Code = strings.TrimSpace(req.Code)
		if req.Code == "" || strings.TrimSpace(req.ClientID) == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "faltan parámetros", 1656)
			return
		}
		key := "social:code:" + req.Code
		payload, ok := c.Cache.Get(key)
		if !ok || len(payload) == 0 {
			httpx.WriteError(w, http.StatusNotFound, "code_not_found", "código inválido o expirado", 1657)
			return
		}

		// payload guardado: {client_id, tenant_id, response}
		var stored struct {
			ClientID string            `json:"client_id"`
			TenantID string            `json:"tenant_id"`
			Response AuthLoginResponse `json:"response"`
		}
		if err := json.Unmarshal(payload, &stored); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "payload_invalid", "payload inválido", 1658)
			return
		}
		if os.Getenv("SOCIAL_DEBUG_LOG") == "1" {
			log.Printf(`{"level":"debug","msg":"social_exchange_payload","req_client":"%s","stored_client":"%s","stored_tenant":"%s","code":"%s"}`,
				strings.TrimSpace(req.ClientID), strings.TrimSpace(stored.ClientID), stored.TenantID, req.Code)
		}
		// Validar client_id (requerido) y tenant_id (si provisto)
		if !strings.EqualFold(strings.TrimSpace(stored.ClientID), strings.TrimSpace(req.ClientID)) {
			// Debug opcional para investigar por qué el client_id no coincide en E2E (controlado por SOCIAL_DEBUG_LOG=1)
			if os.Getenv("SOCIAL_DEBUG_LOG") == "1" {
				log.Printf(`{"level":"debug","msg":"social_exchange_client_mismatch","req_client":"%s","stored_client":"%s","code":"%s","time":"%s"}`,
					strings.TrimSpace(req.ClientID), strings.TrimSpace(stored.ClientID), req.Code, time.Now().Format(time.RFC3339))
			}
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "client_id no coincide para este code", 1659)
			return
		}
		if strings.TrimSpace(req.TenantID) != "" && !strings.EqualFold(strings.TrimSpace(req.TenantID), strings.TrimSpace(stored.TenantID)) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "tenant_id no coincide para este code", 1660)
			return
		}

		// Antes de emitir la respuesta final, si el access venía con AMR base (p.e. solo google) y el usuario tiene MFA configurada
		// deberíamos bifurcar igual que en login/password: devolver mfa_required.
		// Detectamos userID a partir del refresh? No lo tenemos aquí; en el flujo social el response ya contiene tokens.
		// Simplificación: pedimos un token introspect? Evitamos overhead; en su lugar ampliamos el payload almacenado para incluir user_id (futuro). Por ahora asumimos que Google callback ya manejó MFA.
		// Para mantener consistencia con password login cuando se usa login_code se replicará lógica mínima si añadimos user_id en payload futuro.

		// 1 uso: eliminar solo tras validación exitosa
		c.Cache.Delete(key)

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		httpx.WriteJSON(w, http.StatusOK, stored.Response)
	}
}
