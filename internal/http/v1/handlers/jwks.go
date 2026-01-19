/*
jwks.go — JWKS endpoints (global + por-tenant) con cache de corto TTL

Qué hace este handler
---------------------
Este archivo implementa el handler que expone las llaves públicas (JWKS) para que:
- clientes OIDC/OAuth
- RPs
- resource servers
puedan validar JWTs emitidos por HelloJohn.

Publica DOS variantes:
1) JWKS global
2) JWKS por tenant (slug)

Esto está alineado con el modelo v1 donde el issuer puede ser:
- global
- o “path-based” por tenant (IssuerModePath), con llaves por tenant.

Rutas que maneja (registro real en router)
-----------------------------------------
Se registra desde `internal/http/v1/routes.go` de esta forma:
- `GET|HEAD /.well-known/jwks.json` -> `JWKSHandler.GetGlobal`
- `GET|HEAD /.well-known/jwks/{slug}.json` -> `JWKSHandler.GetByTenant`
	Nota: por limitaciones del stdlib `ServeMux`, en realidad se registra como prefix:
		`/.well-known/jwks/` y el handler parsea el sufijo `/{slug}.json`.

Formato de responses
--------------------
- Content-Type: `application/json; charset=utf-8`
- Cache-Control: `no-store` + `Pragma: no-cache`
- 200 OK:
	- GET: body contiene JWKS JSON (raw)
	- HEAD: devuelve sólo headers (sin body)

Errores típicos:
- 405 method_not_allowed si no es GET/HEAD (incluye `Allow: GET, HEAD`)
- 404 Not Found si el path no matchea `/.well-known/jwks/{slug}.json`
- 400 invalid_request si el slug es inválido
- 500 server_error si falla el loader del JWKSCache

================================================================================
Flujo paso a paso
================================================================================

1) GetGlobal — `GET|HEAD /.well-known/jwks.json`
------------------------------------------------
1. Valida método: sólo GET/HEAD.
2. Setea `no-store`.
3. Si HEAD: responde 200 y corta.
4. Pide el JWKS al cache:
		 `h.Cache.Get("global")`
5. Devuelve 200 con el JSON crudo.

2) GetByTenant — `GET|HEAD /.well-known/jwks/{slug}.json`
---------------------------------------------------------
1. Valida método: sólo GET/HEAD.
2. Parseo manual del path (ServeMux):
	 - Requiere prefijo `/.well-known/jwks/`
	 - Requiere sufijo `.json`
	 - Extrae slug (lo que queda en el medio)
3. Valida slug por regex: `^[a-z0-9\-]{1,64}$`
	 - Si no matchea: 400 invalid_request.
4. Setea `no-store`.
5. Si HEAD: responde 200 y corta.
6. Pide JWKS al cache:
		 `h.Cache.Get(slug)`
7. Devuelve 200 con el JSON crudo.

================================================================================
Dependencias reales
================================================================================
- stdlib:
	- `net/http` (handler, status, headers)
	- `strings` (parseo de path)
	- `regexp` (validación de slug)

- internas:
	- `internal/http/v1` como `httpx` (WriteError)
	- `internal/jwt` como `jwtx` (JWKSCache)

El handler NO usa `app.Container` directamente. La dependencia relevante se inyecta como:
- `JWKSHandler.Cache *jwtx.JWKSCache`

Cómo se construye el cache (contexto v1)
----------------------------------------
En `cmd/service/v1/main.go` se crea `container.JWKSCache = jwtx.NewJWKSCache(15s, loader)` donde:
- tenant vacío o `global` -> llama a `issuer.Keys.JWKSJSON()`.
- tenant != global -> llama a `issuer.Keys.JWKSJSONForTenant(tenant)`.

Punto importante: el loader per-tenant NO hace auto-bootstrap en v1 (por diseño HA):
- si faltan keys del tenant, la idea es fallar y que la rotación/restore cree las llaves.

Además, el cache soporta invalidación:
- `JWKSCache.Invalidate(tenant)`
	- usado por admin/cluster restore (ej: `cpctx.InvalidateJWKS` y handlers admin de tenants).

================================================================================
Seguridad / invariantes
================================================================================
- JWKS es público (no requiere authN/authZ).
- Se envían headers `no-store`.
	- Esto es conservador; evita caches intermedias.
	- El costo se amortiza con `JWKSCache` (TTL corto) del lado del server.

- Validación de slug:
	- Restringe a `[a-z0-9-]` y longitud <= 64.
	- Reduce superficie de path tricks y evita slugs raros.

- Método HEAD soportado:
	- Útil para health checks / discovery sin descargar el payload.

================================================================================
Patrones detectados (GoF / arquitectura)
================================================================================
- Cache / Memoization:
	- `jwtx.JWKSCache` implementa un cache in-memory con TTL, protegido con RWMutex.

- Dependency Injection (manual):
	- El handler depende de una abstracción “loader” encapsulada dentro de `JWKSCache`.
	- Se inyecta desde `main.go` (composition root).

- Adapter (leve):
	- `JWKSHandler` adapta el `JWKSCache` (que retorna `json.RawMessage`) al contrato HTTP.

No hay concurrencia en el handler; la concurrencia está encapsulada en el cache.

================================================================================
Cosas no usadas / legacy / riesgos
================================================================================
- Regex recompilada por request:
	- `regexp.MustCompile(...)` está dentro de `GetByTenant`, por lo que se compila en cada request.
	- No rompe funcionalidad, pero es innecesario y puede impactar performance bajo carga.
	- En V2 conviene moverlo a un `var slugRe = regexp.MustCompile(...)` a nivel package.

- Errores 500 para missing tenant keys:
	- Si el loader devuelve error por “no keys for tenant”, hoy se expone como 500 server_error.
	- Puede estar bien (depende del contrato), pero en V2 convendría tipar el error:
		- 404/400/503 según el caso (tenant inexistente vs keys no generadas vs backend caído).

================================================================================
Ideas para V2 (sin decidir nada)
================================================================================
1) Controller + Service explícitos
	 - Controller: parseo de método/path/slug + headers.
	 - Service: `GetJWKS(ctx, tenant)` que conoce loader/keystore y errores tipados.

2) Router declarativo
	 - Evitar parseo manual del path; usar router con params (chi o similar) o patrones.

3) Error taxonomy
	 - Estandarizar errores de JWKS:
		 - tenant inválido
		 - keys inexistentes
		 - backend caído
	 - Mantener envelope `httpx.WriteError` consistente.

4) Caché y cache-control
	 - Evaluar `Cache-Control: public, max-age=...` + ETag en edge/CDN si se busca performance.
	 - Mantener invalidación por tenant ante rotación/restore.

Guía de “desarme” en capas
--------------------------
- Transport/controller:
	- Método + parseo slug + response writer (headers/status).
- Service:
	- Resolver JWKS (global vs tenant) con contrato de errores.
- Infra:
	- `jwtx.JWKSCache` (TTL + locking)
	- `PersistentKeystore` / store de signing keys.

Resumen
-------
- `jwks.go` expone JWKS global y por tenant con soporte GET/HEAD.
- Usa `jwtx.JWKSCache` para amortiguar costos y permite invalidación por tenant.
- Tiene oportunidades claras para V2: router declarativo, regex precompilada y errores tipados.
*/

package handlers

import (
	"net/http"
	"regexp"
	"strings"

	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
)

// JWKSHandler expone endpoints JWKS globales y por tenant, usando cache por tenant.
type JWKSHandler struct {
	Cache *jwtx.JWKSCache
}

func NewJWKSHandler(cache *jwtx.JWKSCache) *JWKSHandler { return &JWKSHandler{Cache: cache} }

// GetGlobal maneja GET/HEAD /.well-known/jwks.json
func (h *JWKSHandler) GetGlobal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/HEAD", 1001)
		return
	}
	setNoStore(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	data, err := h.Cache.Get("global")
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 1501)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// GetByTenant maneja GET/HEAD /.well-known/jwks/{slug}.json
func (h *JWKSHandler) GetByTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/HEAD", 1001)
		return
	}
	// Extraer slug del path (stdlib ServeMux)
	const prefix = "/.well-known/jwks/"
	path := r.URL.Path
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, ".json") {
		http.NotFound(w, r)
		return
	}
	slug := strings.TrimSuffix(strings.TrimPrefix(path, prefix), ".json")
	if !regexp.MustCompile(`^[a-z0-9\-]{1,64}$`).MatchString(slug) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid slug", 1502)
		return
	}
	setNoStore(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	data, err := h.Cache.Get(slug)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 1501)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func setNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}
