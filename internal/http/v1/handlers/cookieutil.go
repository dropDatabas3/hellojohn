/*
cookieutil.go — Helpers para cookies de sesión (SameSite/Secure/Domain/TTL) [NO es handler HTTP]

Qué es este archivo
-------------------
Este archivo NO implementa endpoints HTTP. Es una caja de herramientas para construir cookies
de sesión de manera consistente y con flags de seguridad razonables, especialmente para el flujo
de “cookie session” que se usa alrededor de `/oauth2/authorize` (login/logout de sesión) y
cualquier otro handler que necesite setear/borrar una cookie.

En concreto expone:
- parseSameSite(s string) http.SameSite
- BuildSessionCookie(name, value, domain, sameSite string, secure bool, ttl time.Duration) *http.Cookie
- BuildDeletionCookie(name, domain, sameSite string, secure bool) *http.Cookie

================================================================================
Qué hace (objetivo funcional)
================================================================================
1) Normaliza/configura SameSite
- Convierte strings de config (`"", "lax", "strict", "none"`, case-insensitive) a `http.SameSite`.
- Default: Lax.
- Si recibe un valor desconocido:
  - Loguea warning
  - Vuelve a Lax (fail-safe “más permisivo” que Strict, pero usualmente correcto para compat).

2) Construye cookies de sesión con defaults seguros
- `HttpOnly=true` (protege contra lectura desde JS ante XSS).
- `Path="/"` (aplica a todo el sitio/API).
- `Secure` y `SameSite` se setean según config.
- `Domain` se setea sólo si viene no vacío (evita setear Domain accidentalmente).
- TTL:
  - setea `Expires = now + ttl` (en UTC)
  - setea `MaxAge = int(ttl.Seconds())`

3) Construye cookies de “borrado” (logout)
- Devuelve una cookie con:
  - `Expires` en el pasado
  - `MaxAge = -1`
  - Mismos `Name/Domain/SameSite/Secure/HttpOnly/Path` que la de sesión
- Objetivo: que el user-agent la sobreescriba correctamente (clave para que “borrar” funcione).

================================================================================
Cómo se usa (en el resto del package)
================================================================================
- Este helper se usa típicamente por handlers de sesión (por ejemplo `session_login.go` y `session_logout.go`)
  para:
  - Setear cookie al autenticar una sesión (value = sessionID/token de sesión)
  - Borrarla en logout

Importante: este archivo NO decide el nombre de la cookie ni el dominio; eso viene de config
y/o del wiring en el handler que la usa.

================================================================================
Flujo paso a paso (por función)
================================================================================
parseSameSite(s)
1) Trim + strings.ToLower
2) Mapea:
   - "" | "lax"    => Lax
   - "strict"      => Strict
   - "none"        => None
3) Si es "none" y `secure=false`:
   - NO lo corrige (no fuerza Secure)
   - deja un warning contextual (para no romper localhost sin HTTPS)
4) Valor desconocido:
   - log warning
   - retorna Lax

BuildSessionCookie(...)
1) Obtiene SameSite con parseSameSite
2) Si SameSite=None y secure=false:
   - log warning (“algunos navegadores pueden rechazarla”)
3) Calcula timestamps:
   - now := time.Now().UTC()
   - exp := now.Add(ttl)
4) Construye `http.Cookie` con:
   - Name/Value, Path="/"
   - Domain="" (y se asigna si `domain != ""`)
   - Expires=exp, MaxAge=int(ttl.Seconds())
   - Secure=secure, HttpOnly=true, SameSite=ss
5) Retorna la cookie lista para `http.SetCookie(w, cookie)`

BuildDeletionCookie(...)
1) Obtiene SameSite con parseSameSite
2) Construye cookie con:
   - Value=""
   - Expires=time.Unix(0,0).UTC()
   - MaxAge=-1
   - Secure/HttpOnly/SameSite/Domain/Path alineados
3) Retorna cookie lista para `http.SetCookie`

================================================================================
Dependencias reales
================================================================================
- stdlib:
  - `net/http` (tipo http.Cookie y http.SameSite)
  - `time` (TTL y expiración)
  - `strings` (normalización)
  - `log` (warnings)
- No usa `app.Container`, `Store`, `TenantSQLManager`, `cpctx.Provider`, `Issuer`, `Cache`, etc.

================================================================================
Seguridad / invariantes importantes
================================================================================
- `HttpOnly=true`:
  - Buen default para cookies de sesión (mitiga exfiltración vía JS ante XSS).
- `SameSite=None`:
  - En navegadores modernos, suele requerir `Secure=true`.
  - Este helper NO fuerza Secure (para no romper ambientes sin HTTPS), sólo loguea warning.
  - Riesgo: en prod, si alguien configura `SameSite=None` y `secure=false`, la cookie puede ser ignorada
    por el browser y el login por cookie “parece” fallar.
- `Domain`:
  - Se setea sólo si viene explícito. Esto evita errores comunes (cookies que no matchean host).
- TTL:
  - `MaxAge=int(ttl.Seconds())`: si `ttl < 1s`, MaxAge puede quedar 0 (comportamiento inesperado).
  - `Expires` se calcula siempre y se setea (UTC), lo cual ayuda a compat.

================================================================================
Patrones detectados (GoF / arquitectura)
================================================================================
- Factory / Builder (GoF-ish):
  - `BuildSessionCookie` y `BuildDeletionCookie` son “constructores” que encapsulan defaults y reglas.
- Guard + Observability:
  - Warnings al detectar combinaciones peligrosas (SameSite=None sin Secure).

No hay concurrencia ni estados globales (funciones puras salvo logging).

================================================================================
Cosas no usadas / legacy / riesgos
================================================================================
- No hay imports ni variables “muertas” en este archivo.
- Riesgo de “config inválida silenciosa”:
  - Valores SameSite desconocidos caen a Lax (con warning). Si el logging no se observa, queda oculto.
- Riesgo de compat (SameSite=None sin Secure):
  - Se advierte pero no se bloquea; en prod convendría tratarlo como error de config.

================================================================================
Ideas para V2 (sin decidir nada)
================================================================================
1) Convertir a “CookiePolicy / CookieService”
- En vez de helpers sueltos, centralizar en un componente con opciones:
  - `CookieOptions{ Name, Domain, SameSite, Secure, TTL, Path }`
  - `BuildSessionCookie(opts, value)` y `BuildDeletionCookie(opts)`
- Beneficio: una única fuente de verdad para cookies (session, csrf si aplica, mfa_trust, etc.).

2) Validación de config (fail-fast)
- En bootstrap v2, validar:
  - `SameSite=None` => exigir `Secure=true` salvo modo dev/local explícito.
  - TTL mínimo razonable.

3) Consistencia cross-handlers
- Asegurar que login/logout usen exactamente la misma política (name/domain/samesite/secure),
  porque la cookie de borrado debe matchear para que el browser realmente la elimine.

4) Observabilidad más clara
- En v2, preferible log estructurado (nivel warn + request_id/contexto de entorno) o validación
  de config con error explícito antes de levantar el server.

================================================================================
Resumen
================================================================================
- cookieutil.go es un helper (no handler HTTP) que construye cookies de sesión y de borrado con flags
  de seguridad razonables, y normaliza SameSite desde strings de config.
- Es un buen candidato a convertirse en una “CookiePolicy” central en V2, con validación fail-fast
  para evitar combinaciones que rompen en browsers (SameSite=None sin Secure).
*/

package handlers

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// parseSameSite convierte el string de config a http.SameSite.
// Acepta: "", "lax", "strict", "none" (case-insensitive).
// Default: Lax.
func parseSameSite(s string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		// Nota: para navegadores modernos, SameSite=None requiere Secure=true.
		// No forzamos Secure acá para no romper ambientes http://localhost,
		// pero dejamos un warning en BuildSessionCookie si viene inseguro.
		return http.SameSiteNoneMode
	default:
		log.Printf("cookie: SameSite desconocido=%q, usando Lax", s)
		return http.SameSiteLaxMode
	}
}

// BuildSessionCookie construye la cookie de sesión con flags de seguridad.
// - name/value: nombre de cookie y valor (ej. ID de sesión o token de sesión)
// - domain: Domain opcional (si vacío, no se setea)
// - sameSite: "", "lax", "strict", "none" (case-insensitive). Default Lax.
// - secure: si true, marca Secure (recomendado en prod con https)
// - ttl: duración de la sesión; se setea Expires y Max-Age acorde
func BuildSessionCookie(name, value, domain, sameSite string, secure bool, ttl time.Duration) *http.Cookie {
	ss := parseSameSite(sameSite)
	if ss == http.SameSiteNoneMode && !secure {
		// Advertimos: algunos navegadores rechazan SameSite=None sin Secure.
		log.Printf("cookie: SameSite=None sin Secure; algunos navegadores pueden rechazar la cookie (domain=%q)", domain)
	}
	now := time.Now().UTC()
	exp := now.Add(ttl)

	c := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   "",
		Expires:  exp,
		MaxAge:   int(ttl.Seconds()),
		Secure:   secure,
		HttpOnly: true,
		SameSite: ss,
	}
	if domain != "" {
		c.Domain = domain
	}
	return c
}

// BuildDeletionCookie devuelve una cookie que "borra" la sesión del browser.
// Usa mismo nombre/domain/samesite/secure para que el user-agent la sobreescriba.
func BuildDeletionCookie(name, domain, sameSite string, secure bool) *http.Cookie {
	ss := parseSameSite(sameSite)
	c := &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Domain:   "",
		Expires:  time.Unix(0, 0).UTC(), // pasado
		MaxAge:   -1,                    // eliminar
		Secure:   secure,
		HttpOnly: true,
		SameSite: ss,
	}
	if domain != "" {
		c.Domain = domain
	}
	return c
}
