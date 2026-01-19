# Middlewares V2 - Guía de Referencia Rápida

Biblioteca de middlewares para `internal/http/v2`. Diseñada para implementar rutas de forma rápida y consistente.

---

## Tabla de Contenidos

1. [Instalación Rápida](#instalación-rápida)
2. [Middlewares Disponibles](#middlewares-disponibles)
3. [Referencia por Categoría](#referencia-por-categoría)
4. [Patrones de Uso](#patrones-de-uso)
5. [Helpers de Contexto](#helpers-de-contexto)

---

## Instalación Rápida

```go
import mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"
```

### Composición con Chain

```go
handler := mw.Chain(myHandler,
    mw.WithRecover(),
    mw.WithRequestID(),
    mw.WithLogging(),
    // ... más middlewares
)
```

> **Orden**: El primer middleware en la lista es el más externo (primero en interceptar, último en ver respuesta).

---

## Middlewares Disponibles

| Middleware | Archivo | Propósito |
|------------|---------|-----------|
| `Chain` | chain.go | Componer middlewares |
| `WithCORS` | cors.go | Manejo de CORS |
| `WithSecurityHeaders` | security_headers.go | Headers de seguridad |
| `WithRequestID` | request_id.go | Request ID tracking |
| `WithRecover` | recover.go | Captura panics |
| `WithLogging` | logging.go | Logs JSON estructurados |
| `WithRateLimit` | rate.go | Rate limiting |
| `WithNoStore` | no_store.go | Cache-Control: no-store |
| `WithCacheControl` | no_store.go | Cache-Control configurable |
| `WithCSRF` | csrf.go | Protección CSRF |
| `WithTenantResolution` | tenant.go | Resolución de tenant |
| `RequireTenant` | tenant.go | Verificar tenant presente |
| `RequireTenantDB` | tenant.go | Verificar tenant tiene DB |
| `RequireAuth` | auth.go | Autenticación JWT requerida |
| `OptionalAuth` | auth.go | Autenticación JWT opcional |
| `RequireUser` | auth.go | Verificar usuario autenticado |
| `RequireAdmin` | admin.go | Admin de tenant requerido |
| `RequireSysAdmin` | admin.go | Admin de sistema requerido |
| `RequireRole` | rbac.go | Rol requerido |
| `RequireAllRoles` | rbac.go | Todos los roles requeridos |
| `RequirePerm` | rbac.go | Permiso requerido |
| `RequireScope` | scopes.go | Scope OAuth requerido |
| `RequireAnyScope` | scopes.go | Algún scope requerido |
| `RequireAllScopes` | scopes.go | Todos los scopes requeridos |
| `RequireLeader` | cluster.go | Solo nodo líder (HA) |

---

## Referencia por Categoría

### 1. Infraestructura Base

#### `WithRecover()`
Captura panics y devuelve 500 en lugar de crashear.

```go
mw.WithRecover()
```

**Cuándo usar**: Siempre. Primera línea de defensa.

---

#### `WithRequestID()`
Genera o propaga X-Request-ID para tracing.

```go
mw.WithRequestID()
```

**Cuándo usar**: Siempre. Esencial para debugging.

**Acceso en handler**:
```go
rid := mw.GetRequestID(r.Context())
```

---

#### `WithLogging()`
Log JSON estructurado de cada request.

```go
mw.WithLogging()
```

**Output**:
```json
{"level":"info","msg":"http","request_id":"abc123","method":"POST","path":"/v1/auth/login","status":200,"bytes":256,"duration_ms":45}
```

---

#### `WithCORS(allowed []string)`
Manejo de CORS con orígenes configurables.

```go
mw.WithCORS([]string{"https://app.example.com", "https://admin.example.com"})
mw.WithCORS([]string{"*"}) // Permitir todos (solo desarrollo)
```

---

#### `WithSecurityHeaders()`
Inyecta headers de seguridad estándar.

```go
mw.WithSecurityHeaders()
```

**Headers incluidos**:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Content-Security-Policy: default-src 'none'`
- `Strict-Transport-Security` (si HTTPS)
- Y más...

---

#### `WithNoStore()` / `WithCacheControl(directive)`
Control de cache.

```go
mw.WithNoStore()                           // Cache-Control: no-store
mw.WithCacheControl("public, max-age=600") // Custom
```

**Cuándo usar**: JWKS, discovery, datos sensibles.

---

### 2. Rate Limiting

#### `WithRateLimit(cfg RateLimitConfig)`
Rate limiting configurable.

```go
mw.WithRateLimit(mw.RateLimitConfig{
    Limiter:   myLimiter,                           // Implementa RateLimiter interface
    KeyFunc:   mw.DefaultRateKey,                   // Opcional, default: IP+path+client_id
    Whitelist: []string{"/healthz", "/readyz"},     // Excluir paths
})
```

**Interface RateLimiter**:
```go
type RateLimiter interface {
    Allow(ctx context.Context, key string) (RateLimitResult, error)
}
```

---

### 3. Seguridad

#### `WithCSRF(cfg CSRFConfig)`
Protección CSRF con double-submit cookie.

```go
mw.WithCSRF(mw.CSRFConfig{
    HeaderName: "X-CSRF-Token",  // Default
    CookieName: "csrf_token",    // Default
})
```

**Comportamiento**:
- Skip si hay `Authorization: Bearer` (no es flujo cookie)
- Solo para POST, PUT, PATCH, DELETE
- Compara header vs cookie en tiempo constante

---

### 4. Tenant Resolution

#### `WithTenantResolution(dal, optional)`
Resuelve tenant e inyecta `TenantDataAccess` en contexto.

```go
mw.WithTenantResolution(dal, false) // Requerido - falla si no hay tenant
mw.WithTenantResolution(dal, true)  // Opcional - continúa sin tenant
```

**Orden de detección** (default):
1. Header `X-Tenant-ID`
2. Header `X-Tenant-Slug`
3. Query param `tenant`
4. Query param `tenant_id`
5. Subdominio (ej: `acme.example.com` → `acme`)

**Acceso en handler**:
```go
tda := mw.GetTenant(r.Context())     // Puede ser nil
tda := mw.MustGetTenant(r.Context()) // Panic si nil
```

---

#### `RequireTenant()`
Verifica que haya tenant en contexto. Usar después de `WithTenantResolution`.

```go
mw.RequireTenant()
```

---

#### `RequireTenantDB()`
Verifica que el tenant tenga base de datos configurada.

```go
mw.RequireTenantDB()
```

**Cuándo usar**: Rutas que acceden a Users, Tokens, MFA, etc.

---

### 5. Autenticación

#### `RequireAuth(issuer)`
Valida JWT Bearer token y guarda claims en contexto.

```go
mw.RequireAuth(issuer)
```

**Comportamiento**:
- Requiere header `Authorization: Bearer <token>`
- Valida firma EdDSA con issuer
- Inyecta claims en contexto
- Responde 401 si falla

**Acceso en handler**:
```go
claims := mw.GetClaims(r.Context())
userID := mw.GetUserID(r.Context()) // Extraído de claim "sub"
```

---

#### `OptionalAuth(issuer)`
Igual que RequireAuth pero NO falla si no hay token.

```go
mw.OptionalAuth(issuer)
```

**Cuándo usar**: Rutas públicas con comportamiento diferente para usuarios autenticados.

---

#### `RequireUser()`
Verifica que `GetUserID(ctx) != ""`. Usar después de `RequireAuth`.

```go
mw.RequireUser()
```

---

### 6. Autorización - Admin

#### `RequireAdmin(cfg)`
Verifica que el usuario sea admin del tenant.

```go
mw.RequireAdmin(mw.AdminConfigFromEnv()) // Carga desde ADMIN_ENFORCE y ADMIN_SUBS
mw.RequireAdmin(mw.AdminConfig{
    EnforceAdmin: true,
    AdminSubs:    []string{"user-id-1", "user-id-2"},
})
```

**Reglas de verificación** (en orden):
1. Si `ADMIN_ENFORCE != "1"` → permitir (modo dev)
2. Si `custom.is_admin == true` → permitir
3. Si `custom.roles` incluye `"admin"` → permitir
4. Si `sub` está en `ADMIN_SUBS` → permitir
5. Si no → 403

---

#### `RequireSysAdmin(issuer, cfg)`
Admin del sistema (usa namespace del issuer).

```go
mw.RequireSysAdmin(issuer, mw.AdminConfigFromEnv())
```

**Diferencia con RequireAdmin**: Busca en `custom[sysNS].is_admin` y `custom[sysNS].roles` con rol `sys:admin`.

---

### 7. Autorización - RBAC

#### `RequireRole(issuer, roles...)`
Verifica que el usuario tenga AL MENOS UNO de los roles.

```go
mw.RequireRole(issuer, "admin", "moderator")
```

**Dónde busca**: `custom[sysNS].roles`

---

#### `RequireAllRoles(issuer, roles...)`
Verifica que el usuario tenga TODOS los roles.

```go
mw.RequireAllRoles(issuer, "admin", "billing")
```

---

#### `RequirePerm(issuer, perms...)`
Verifica que el usuario tenga AL MENOS UNO de los permisos.

```go
mw.RequirePerm(issuer, "users:read", "users:write")
```

**Dónde busca**: `custom[sysNS].perms`

---

### 8. Autorización - OAuth Scopes

#### `RequireScope(scope)`
Verifica que el token tenga el scope requerido.

```go
mw.RequireScope("profile:read")
```

**Dónde busca**:
- `scp: ["scope1", "scope2"]` (array, preferido)
- `scope: "scope1 scope2"` (string separado por espacios)

---

#### `RequireAnyScope(scopes...)`
Verifica que el token tenga AL MENOS UNO de los scopes.

```go
mw.RequireAnyScope("read:users", "admin")
```

---

#### `RequireAllScopes(scopes...)`
Verifica que el token tenga TODOS los scopes.

```go
mw.RequireAllScopes("openid", "profile", "email")
```

---

### 9. Cluster / HA

#### `RequireLeader(clusterRepo, leaderRedirects)`
Solo permite escrituras en el nodo líder.

```go
mw.RequireLeader(clusterRepo, map[string]string{
    "node-1": "https://node1.example.com",
    "node-2": "https://node2.example.com",
})
```

**Comportamiento**:
- Solo aplica a POST, PUT, PATCH, DELETE
- Si es líder o no hay cluster → pasa
- Si es follower → 409 con header `X-Leader`
- Si cliente pide redirect (`X-Leader-Redirect: 1`) → 307

---

## Patrones de Uso

### Ruta Pública (Health Check)

```go
mux.Handle("/healthz", mw.Chain(healthHandler,
    mw.WithRecover(),
    mw.WithRequestID(),
))
```

### Ruta Pública con Tenant

```go
mux.Handle("/v1/auth/login", mw.Chain(loginHandler,
    mw.WithRecover(),
    mw.WithRequestID(),
    mw.WithLogging(),
    mw.WithCORS(allowedOrigins),
    mw.WithTenantResolution(dal, false),
    mw.RequireTenantDB(),
))
```

### Ruta Protegida

```go
mux.Handle("/v1/profile", mw.Chain(profileHandler,
    mw.WithRecover(),
    mw.WithRequestID(),
    mw.WithLogging(),
    mw.WithCORS(allowedOrigins),
    mw.WithTenantResolution(dal, false),
    mw.RequireAuth(issuer),
    mw.RequireScope("profile:read"),
))
```

### Ruta Admin

```go
mux.Handle("/v1/admin/users", mw.Chain(adminUsersHandler,
    mw.WithRecover(),
    mw.WithRequestID(),
    mw.WithLogging(),
    mw.WithCORS(allowedOrigins),
    mw.WithTenantResolution(dal, false),
    mw.RequireTenantDB(),
    mw.RequireAuth(issuer),
    mw.RequireAdmin(mw.AdminConfigFromEnv()),
))
```

### Ruta Cluster-Aware

```go
mux.Handle("/v1/admin/tenants", mw.Chain(tenantsHandler,
    mw.WithRecover(),
    mw.WithRequestID(),
    mw.WithLogging(),
    mw.RequireAuth(issuer),
    mw.RequireSysAdmin(issuer, mw.AdminConfigFromEnv()),
    mw.RequireLeader(clusterRepo, leaderRedirects),
))
```

---

## Helpers de Contexto

### Getters

| Helper | Retorna | Descripción |
|--------|---------|-------------|
| `GetClaims(ctx)` | `map[string]any` | Claims JWT (nil si no auth) |
| `GetTenant(ctx)` | `TenantDataAccess` | Tenant (nil si no resuelto) |
| `MustGetTenant(ctx)` | `TenantDataAccess` | Tenant (panic si nil) |
| `GetUserID(ctx)` | `string` | User ID del claim "sub" |
| `GetRequestID(ctx)` | `string` | Request ID |

### Claim Helpers

| Helper | Descripción |
|--------|-------------|
| `ClaimString(claims, key)` | Extrae string |
| `ClaimBool(claims, key)` | Extrae bool |
| `ClaimStringSlice(claims, key)` | Extrae []string |
| `ClaimMap(claims, key)` | Extrae map[string]any |

**Ejemplo**:
```go
claims := mw.GetClaims(r.Context())
email := mw.ClaimString(claims, "email")
roles := mw.ClaimStringSlice(mw.ClaimMap(claims, "custom"), "roles")
```

---

## Variables de Entorno

| Variable | Usado por | Descripción |
|----------|-----------|-------------|
| `ADMIN_ENFORCE` | RequireAdmin, RequireSysAdmin | Si es "1", enforce admin checks |
| `ADMIN_SUBS` | RequireAdmin, RequireSysAdmin | Lista CSV de user IDs admin |

---

## Errores Estándar

Los middlewares usan el paquete `internal/http/v2/errors`:

| Código | HTTP | Cuándo |
|--------|------|--------|
| `TOKEN_MISSING` | 401 | No hay Authorization header |
| `TOKEN_INVALID` | 401 | Token JWT inválido |
| `FORBIDDEN` | 403 | Sin permisos |
| `INSUFFICIENT_SCOPES` | 403 | Scope faltante |
| `TENANT_NOT_FOUND` | 404 | Tenant no existe |
| `RATE_LIMIT_EXCEEDED` | 429 | Rate limit superado |
| `CONFLICT` | 409 | Operación en follower (cluster) |
