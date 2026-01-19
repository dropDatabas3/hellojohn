/*
csrf.go — Emisión de CSRF token (double-submit) vía cookie + JSON

Qué hace este handler
---------------------
Este archivo implementa un único endpoint para emitir un CSRF token efímero que se usa como
“double-submit token” para proteger endpoints basados en cookies (especialmente el login de sesión).

Endpoint que maneja
-------------------
- GET /v1/csrf
	- Setea una cookie con el CSRF token
	- Devuelve el mismo token en JSON: {"csrf_token":"..."}
	- Headers anti-cache: Cache-Control: no-store, Pragma: no-cache

Este token se valida (cuando está habilitado) con el middleware `RequireCSRF` (en
`internal/http/v1/middleware.go`), que compara:
- Header (default `X-CSRF-Token`, configurable)
- Cookie (default `csrf_token`, configurable)
Ambos deben existir y matchear exactamente.

Cómo se usa (en el wiring actual)
--------------------------------
- En `cmd/service/v1/main.go` se construye el handler:
		`handlers.NewCSRFGetHandler(getenv("CSRF_COOKIE_NAME", "csrf_token"), 30*time.Minute)`
	y se registra en el mux como:
		`GET /v1/csrf`

- El enforcement de CSRF se aplica (opcionalmente) sólo a `POST /v1/session/login`:
	- Si `CSRF_COOKIE_ENFORCED=1`, `sessionLoginHandler` se envuelve con
		`httpserver.RequireCSRF(csrfHeader, csrfCookie)`.
	- Si hay Bearer auth, el middleware saltea CSRF (porque no es un flujo cookie).

Flujo paso a paso (GET /v1/csrf)
--------------------------------
1) Validación HTTP
	 - Sólo acepta método GET.
	 - Si no es GET: 405 Method Not Allowed con `httpx.WriteError`.

2) Generación del token
	 - Genera 32 bytes aleatorios (`crypto/rand.Read`) y los serializa a hex.
	 - El resultado es un string de 64 chars (hex) típicamente.
	 - Nota: el error de `rand.Read` se ignora (best-effort).

3) Seteo de cookie
	 - Cookie name: configurable (default `csrf_token`).
	 - TTL: configurable (default 30m).
	 - Atributos actuales:
		 - SameSite=Lax
		 - HttpOnly=false  (intencional: el frontend lo lee para mandarlo en el header)
		 - Secure=false    (hardcode)
		 - Path=/
		 - Expires=now+ttl

4) Response
	 - `Cache-Control: no-store` + `Pragma: no-cache`
	 - 200 OK con JSON: {"csrf_token": tok}

Dependencias reales
-------------------
- stdlib:
	- `crypto/rand`, `encoding/hex`, `net/http`, `time`
- internas:
	- `internal/http/v1` como `httpx` para `WriteError` y `WriteJSON`

No usa `app.Container`, `Store`, `TenantSQLManager`, `cpctx.Provider`, `Issuer`, `Cache`, etc.

Seguridad / invariantes
-----------------------
- Patrón implementado: “Double-submit cookie”
	- El CSRF token vive en cookie (para el browser) y también se devuelve al JS (para header).
	- El server valida igualdad cookie/header en requests sensibles *basados en cookies*.

- SameSite=Lax
	- Reduce CSRF en muchos casos por default, pero NO reemplaza la validación double-submit.

- HttpOnly=false (por diseño)
	- Necesario para que el frontend pueda leerlo y reenviarlo.
	- Implica que ante XSS el token es legible; CSRF no protege XSS, así que es esperado.

- Secure=false (hardcode)
	- En producción con HTTPS esto debería ser Secure=true.
	- Tal cual está, depende de que el deployment acepte cookie sin Secure (y/o que haya otros controles).

- Token no firmado
	- Es un random bearer token; la “validación” es compararlo contra lo que el server mismo setea en cookie.
	- No está atado a usuario/tenant; el scope es “sesión del navegador” (cookie jar).

Patrones detectados (GoF / arquitectura)
----------------------------------------
- Factory / Constructor de handler:
	- `NewCSRFGetHandler(cookieName, ttl)` devuelve un `http.HandlerFunc` configurado.
- Security pattern:
	- Double-submit CSRF token.

No hay concurrencia (goroutines/locks) ni estado compartido; es stateless salvo el seteo de cookie.

Cosas no usadas / legacy / riesgos
---------------------------------
- `rand.Read` ignora el error:
	- Si fallara (muy raro), el token podría ser predecible/empty dependiendo del contenido del buffer.
	- Sería mejor fail-closed con 500 si no se pudo generar entropía.

- `Secure=false` hardcode:
	- Riesgo de seguridad en prod (cookie viaja por HTTP si está permitido).
	- También puede causar inconsistencias si el sitio corre sólo en HTTPS y el browser exige Secure
		bajo ciertas políticas.

- No setea `Max-Age`:
	- Sólo usa `Expires`; suele ser suficiente, pero algunos clientes se comportan distinto.

Ideas para V2 (sin decidir nada)
--------------------------------
1) CSRFService / CookiePolicy compartida
	 - Unificar criterios de cookie (Secure, SameSite, Domain, TTL) con lo que ya existe en `cookieutil.go`.
	 - Evitar hardcodes (especialmente Secure).

2) Detección HTTPS/proxy
	 - Calcular `Secure` en base a `r.TLS` / `X-Forwarded-Proto` (similar a `isHTTPS()` del middleware)
		 o validar por config de entorno.

3) Endpoints y enforcement consistentes
	 - Centralizar qué endpoints requieren CSRF en el router/middleware, no en `main.go`.
	 - Mantener el “skip si Bearer” (cookie-flow vs token-flow) como regla explícita.

4) Mejoras de robustez
	 - Manejar error de `rand.Read` (fail-closed con 500).
	 - Definir envelope de error consistente (ya se usa `httpx.WriteError`).

Guía de “desarme” en capas (para comprensión y mantenibilidad)
--------------------------------------------------------------
- DTO/transport:
	- Response: `{csrf_token: string}` (simple; podría formalizarse como struct en V2).
- Controller:
	- Validación de método + orquestación de emisión.
- Service:
	- Generación del token + política de expiración + armado de cookie.
- Infra/util:
	- Fuente de entropía (rand) + helpers de cookies (policy).

Resumen
-------
- `csrf.go` emite un CSRF token para el patrón double-submit y lo entrega por cookie + JSON.
- Se usa para proteger el flujo de login de sesión basado en cookies cuando `CSRF_COOKIE_ENFORCED=1`.
- Hay oportunidades claras para V2: unificar policy de cookies, evitar `Secure=false` hardcode y
	manejar fail-closed si la entropía falla.
*/

package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
)

// NewCSRFGetHandler returns a GET handler that issues a CSRF token via cookie and JSON body.
// Cookie attributes: SameSite=Lax, HttpOnly=false, Path=/, short TTL (e.g., 30m).
// Response: {"csrf_token":"..."} with Cache-Control: no-store.
func NewCSRFGetHandler(cookieName string, ttl time.Duration) http.HandlerFunc {
	if cookieName == "" {
		cookieName = "csrf_token"
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET", 1000)
			return
		}
		// generate random token
		var b [32]byte
		_, _ = rand.Read(b[:])
		tok := hex.EncodeToString(b[:])
		exp := time.Now().Add(ttl).UTC()

		// non-HttpOnly by design so frontend can read it (double-submit); SameSite Lax
		http.SetCookie(w, &http.Cookie{
			Name:     cookieName,
			Value:    tok,
			Path:     "/",
			HttpOnly: false,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
			Expires:  exp,
		})

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"csrf_token": tok})
	}
}
