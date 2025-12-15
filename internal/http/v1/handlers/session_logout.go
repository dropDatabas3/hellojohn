/*
session_logout.go — Session Cookie Logout (borra sid en cache + cookie) + redirect return_to allowlist

Qué es este archivo (la posta)
------------------------------
Este archivo define NewSessionLogoutHandler(...) que implementa el “logout” del modo cookie/session:
	- Si existe cookie de sesión (sid):
			* borra la sesión del cache server-side
			* setea una cookie de borrado (expirada) para limpiar el browser
	- Opcionalmente redirige a return_to si el host está en allowlist

No revoca refresh/access tokens (eso es otro subsistema); acá solo limpia la sesión tipo sid.

Dependencias reales
-------------------
- c.Cache: Delete("sid:<hash>")
- cookieutil.go: BuildDeletionCookie(...) para expirar cookie
- c.RedirectHostAllowlist: mapa host->bool para permitir redirects
- session_logout_util.go: tokensSHA256 para hashear el raw cookie value

Ruta soportada (contrato efectivo)
----------------------------------
- POST /v1/session/logout
		Cookies:
			- lee cookieName (configurable)

		Query:
			- return_to (opcional): si es URL absoluta y su host está en allowlist, 303 redirect

		Response:
			- 204 No Content (por defecto)
			- o 303 See Other a return_to (si validación OK)

Flujo interno
-------------
1) Validación de método
	 - solo POST

2) Si existe cookie de sesión
	 - r.Cookie(cookieName)
	 - si tiene valor no vacío:
			 a) server-side: key = "sid:" + tokensSHA256(cookieValue)
					c.Cache.Delete(key)
			 b) client-side: setea cookie de borrado (BuildDeletionCookie)
	 Nota: si NO hay cookie, no setea deletion cookie (logout no es 100% idempotente a nivel browser).

3) Redirect opcional return_to
	 - Requiere que return_to sea URL absoluta (scheme y host no vacíos)
	 - El host se baja a lowercase y se compara con c.RedirectHostAllowlist
	 - Si pasa: http.Redirect(..., StatusSeeOther)

4) Si no hay redirect: 204

Seguridad / invariantes
-----------------------
- Borrado server-side:
	Depende de que el hashing usado aquí coincida con el usado en session_login.go.
	Login usa tokens.SHA256Base64URL; logout usa tokensSHA256 (sha256 + RawURLEncoding).
	Hoy parecen equivalentes, pero es deuda técnica.

- Redirect allowlist:
	Bien: valida que sea absoluta y aplica allowlist.
	Pero usa u.Host tal cual (puede incluir ":port").
	Si el allowlist guarda hosts sin puerto, el match puede fallar.
	V2: usar u.Hostname() + puerto opcional, o normalizar ambos.

- CSRF:
	Logout también es endpoint cookie-based; idealmente debería estar cubierto por la misma política CSRF
	que el resto de endpoints sensibles (depende de middleware global).

Problemas principales (cuellos de botella + bugs probables)
-----------------------------------------------------------
1) Duplicación de hashing
	 tokensSHA256 duplicado vs tokens.SHA256Base64URL.

2) Logout no siempre limpia cookie
	 Si el cliente no manda cookie (o el nombre cambió), no se setea deletion cookie.
	 Puede ser preferible siempre setear cookie expirada para hacer logout idempotente.

3) Normalización de return_to
	 Comparar con u.Host puede incluir puerto; conviene normalizar.

Cómo lo refactorizaría a V2 (plan concreto)
-------------------------------------------
FASE 1 — Unificar contrato sid
	- session.CacheKey(sessionID) en un package único
	- Eliminar session_logout_util.go y usar tokens.SHA256Base64URL

FASE 2 — Logout idempotente
	- Siempre setear deletion cookie (aunque no exista cookie entrante)
	- Borrar server-side si cookie presente

FASE 3 — Redirect seguro y predecible
	- Normalizar host con u.Hostname()
	- Considerar allowlist por (scheme, host, port)

*/

package handlers

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
)

// Usamos BuildDeletionCookie(name, domain, sameSite, secure) definido en cookieutil.go
func NewSessionLogoutHandler(c *app.Container, cookieName, cookieDomain, sameSite string, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		ck, err := r.Cookie(cookieName)
		if err == nil && ck != nil && strings.TrimSpace(ck.Value) != "" {
			// Borrar server-side
			key := "sid:" + tokensSHA256(ck.Value)
			c.Cache.Delete(key)
			// Borrar client-side
			del := BuildDeletionCookie(cookieName, cookieDomain, sameSite, secure)
			http.SetCookie(w, del)
		}

		// Handle optional return_to redirect check
		returnTo := r.URL.Query().Get("return_to")
		if returnTo != "" {
			if u, err := url.Parse(returnTo); err == nil && u.Scheme != "" && u.Host != "" {
				host := strings.ToLower(u.Host)
				if c.RedirectHostAllowlist != nil && c.RedirectHostAllowlist[host] {
					http.Redirect(w, r, returnTo, http.StatusSeeOther)
					return
				}
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
