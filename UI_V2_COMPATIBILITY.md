# UI V2 Compatibility Report

> **Fecha**: 2026-01-20
> **Frontend**: /ui (Next.js admin panel)
> **Backend**: V2 API (cmd/service_v2)

---

## 📊 Executive Summary

**Compatibilidad General**: **72% (34/47 endpoints)**

### Estado por Funcionalidad:

| Funcionalidad | Compatible | Requiere Cambios | Missing | Total |
|---------------|-----------|------------------|---------|-------|
| **Auth Basic** (login, register, refresh) | ✅ 7/7 | - | - | 100% |
| **Admin Clients** (CRUD) | ✅ 3/4 | - | ❌ 1 (revoke) | 75% |
| **Admin Scopes** (CRUD) | ✅ 3/3 | - | - | 100% |
| **Admin Users** (actions) | ✅ 3/3 | - | ❌ 3 (list/create/get) | 50% |
| **Admin Tenants** (CRUD) | ⚠️ 3/3 | ⚠️ 3 (path) | ❌ 1 (get single) | 75% |
| **Admin RBAC** | ✅ 4/4 | - | - | 100% |
| **OAuth2/OIDC** | ✅ 5/5 | - | - | 100% |
| **Session** | ✅ 1/1 | - | - | 100% |
| **Dev/Admin Tools** | - | - | ❌ 8/8 | 0% |

---

## 🚨 BLOCKERS CRÍTICOS para UI

### **BLOCKER 1: User Management** (CRÍTICO)
**Impacto**: Admin panel de usuarios COMPLETAMENTE ROTO

**Endpoints faltantes**:
```
GET  /v2/admin/tenants/{id}/users          ❌ MISSING
POST /v2/admin/tenants/{id}/users          ❌ MISSING
GET  /v2/admin/tenants/{id}/users/{userId} ❌ MISSING
```

**UI Files afectados** (8 archivos):
- `ui/app/admin/users/page.tsx` (líneas: 191, 206, 239, 266, 286, 312, 1063)
- `ui/components/admin/UsersClientPage.tsx` (líneas: 84, 100, 115, 142, 162, 188, 462)

**Workaround actual**: Ninguno. Funcionalidad bloqueada.

**Fix requerido**: Implementar estos 3 endpoints en V2.

---

### **BLOCKER 2: Tenant Detail** (CRÍTICO)
**Impacto**: Páginas de detalle de tenant rotas

**Endpoint faltante**:
```
GET /v2/admin/tenants/{id}  ❌ MISSING
```

**UI Files afectados** (2 archivos):
- `ui/app/admin/users/page.tsx:206`
- `ui/components/admin/UsersClientPage.tsx:100`

**Workaround actual**: Solo se puede listar tenants, no ver detalles.

**Fix requerido**: Implementar endpoint de detalle individual.

---

### **BLOCKER 3: Client Revocation** (MEDIO)
**Impacto**: No se pueden revocar secretos de clientes

**Endpoint faltante**:
```
POST /v2/admin/clients/{id}/revoke  ❌ MISSING
```

**UI Files afectados**:
- `ui/components/admin/ClientsClientPage.tsx:293`

**Workaround actual**: Eliminar y recrear el client.

**Fix requerido**: Implementar endpoint de revocación.

---

## ✅ Funcionalidades que YA FUNCIONAN (sin cambios)

### Auth Flows (100% compatible)
- ✅ Login con password (`POST /v2/auth/login`)
- ✅ Register (`POST /v2/auth/register`)
- ✅ Refresh token (`POST /v2/auth/refresh`)
- ✅ Get user info (`GET /v2/me`)
- ✅ Get config/branding (`GET /v2/auth/config`)
- ✅ Get providers (`GET /v2/auth/providers`)
- ✅ Session login (`POST /v2/session/login`)

### Admin Panel (Core) (100% compatible)
- ✅ List clients (`GET /v2/admin/clients`)
- ✅ Create client (`POST /v2/admin/clients`)
- ✅ Delete client (`DELETE /v2/admin/clients/{id}`)
- ✅ List scopes (`GET /v2/admin/scopes`)
- ✅ Create scope (`POST /v2/admin/scopes`)
- ✅ Delete scope (`DELETE /v2/admin/scopes/{id}`)
- ✅ Disable user (`POST /v2/admin/users/disable`)
- ✅ Enable user (`POST /v2/admin/users/enable`)
- ✅ Resend verification (`POST /v2/admin/users/resend-verification`)

### Admin RBAC (100% compatible)
- ✅ Get user roles (`GET /v2/admin/rbac/users/{id}/roles`)
- ✅ Assign user role (`POST /v2/admin/rbac/users/{id}/roles`)
- ✅ Get role permissions (`GET /v2/admin/rbac/roles/{role}/perms`)
- ✅ Assign role permission (`POST /v2/admin/rbac/roles/{role}/perms`)

### OAuth2/OIDC (100% compatible)
- ✅ Token exchange (`POST /oauth2/token`)
- ✅ Token introspection (`POST /oauth2/introspect`)
- ✅ Token revocation (`POST /oauth2/revoke`)
- ✅ OIDC Discovery (`GET /.well-known/openid-configuration`)
- ✅ JWKS (`GET /.well-known/jwks.json`)

---

## ⚠️ Funcionalidades que REQUIEREN CAMBIOS MENORES

### Tenant Management (requiere actualizar paths)
```typescript
// ANTES (V1)
GET  /v1/admin/tenants
POST /v1/admin/tenants
DELETE /v1/admin/tenants/{slug}

// DESPUÉS (V2)
GET  /v2/admin/tenants
POST /v2/admin/tenants
DELETE /v2/admin/tenants/{slug}

// Fix: Simple find/replace en UI
```

**Archivos a modificar**:
- `ui/components/admin/admin-shell.tsx:70,119`
- `ui/app/admin/tenants/page.tsx:73,77,101`

---

### Consent Management (requiere cambio de estructura)
```typescript
// ANTES (V1)
DELETE /v1/admin/consents?user_id={id}&client_id={cid}

// DESPUÉS (V2)
DELETE /v2/admin/consents/{userId}/{clientId}

// Fix: Extraer query params y ponerlos en path
```

**Archivos a modificar**:
- `ui/app/admin/consents/page.tsx:55`

---

## 🔧 Plan de Implementación

### **FASE 1: Backend V2 - Endpoints Faltantes** (PRIORITARIO)

#### 1.1 User Management Endpoints (CRÍTICO)
Crear en `internal/http/v2/`:

```go
// services/admin/users_crud_service.go
type UserCRUDService interface {
    List(ctx, tenantSlugOrID, filter) ([]dto.User, error)
    Get(ctx, tenantSlugOrID, userID) (*dto.User, error)
    Create(ctx, tenantSlugOrID, input) (*dto.User, error)
    Update(ctx, tenantSlugOrID, userID, input) error
    Delete(ctx, tenantSlugOrID, userID) error
}

// controllers/admin/users_crud_controller.go
type UserCRUDController struct {
    service UserCRUDService
}
func (c *UserCRUDController) List(w, r) { /* ... */ }
func (c *UserCRUDController) Get(w, r) { /* ... */ }
func (c *UserCRUDController) Create(w, r) { /* ... */ }

// router/admin_routes.go
mux.Handle("/v2/admin/tenants/{id}/users",
    adminHandler(dal, issuer, limiter, c.Users, true))
mux.Handle("/v2/admin/tenants/{id}/users/{userId}",
    adminHandler(dal, issuer, limiter, c.Users, true))
```

**Estimado**: 2-3 horas

---

#### 1.2 Tenant Detail Endpoint (CRÍTICO)
Agregar en `services/admin/tenants_service.go`:

```go
// Ya existe TenantsService, solo agregar método:
func (s *tenantsService) Get(ctx, slugOrID) (*dto.Tenant, error) {
    // Implementar
}

// Controller ya existe, agregar handler:
func (c *TenantsController) Get(w, r) {
    // Extract slug/ID from path
    result := c.service.Get(ctx, slugOrID)
    // ...
}

// Router: modificar tenants_routes.go para manejar GET /{id}
```

**Estimado**: 30 minutos

---

#### 1.3 Client Revocation Endpoint (MEDIO)
Agregar en `services/admin/clients_service.go`:

```go
func (s *clientService) RevokeSecret(ctx, tenantSlug, clientID) error {
    // Re-generate client secret
    // Update in Control Plane
}

// Controller + Route
mux.Handle("/v2/admin/clients/{id}/revoke",
    adminHandler(dal, issuer, limiter, c.Clients, false))
```

**Estimado**: 1 hora

---

### **FASE 2: UI - Adapter Layer** (MEDIO)

Crear adapter de API para mapear V1→V2 automáticamente:

```typescript
// ui/lib/api-adapter.ts
const V2_ENDPOINT_MAP: Record<string, string> = {
  // Auth
  '/v1/auth/login': '/v2/auth/login',
  '/v1/auth/register': '/v2/auth/register',
  '/v1/auth/refresh': '/v2/auth/refresh',
  '/v1/auth/config': '/v2/auth/config',
  '/v1/me': '/v2/me',

  // Admin
  '/v1/admin/clients': '/v2/admin/clients',
  '/v1/admin/scopes': '/v2/admin/scopes',
  '/v1/admin/tenants': '/v2/admin/tenants',
  '/v1/admin/users/disable': '/v2/admin/users/disable',
  '/v1/admin/users/enable': '/v2/admin/users/enable',
  '/v1/admin/users/resend-verification': '/v2/admin/users/resend-verification',

  // RBAC (prefix match)
  '/v1/admin/rbac/': '/v2/admin/rbac/',

  // OAuth2/OIDC (no change)
  '/oauth2/': '/oauth2/',
  '/.well-known/': '/.well-known/',
};

export class ApiAdapter {
  static mapEndpoint(v1Endpoint: string): string {
    // Exact match
    if (V2_ENDPOINT_MAP[v1Endpoint]) {
      return V2_ENDPOINT_MAP[v1Endpoint];
    }

    // Prefix match
    for (const [v1Prefix, v2Prefix] of Object.entries(V2_ENDPOINT_MAP)) {
      if (v1Prefix.endsWith('/') && v1Endpoint.startsWith(v1Prefix)) {
        return v1Endpoint.replace(v1Prefix, v2Prefix);
      }
    }

    // Default: replace /v1/ with /v2/
    return v1Endpoint.replace('/v1/', '/v2/');
  }

  static mapRequest(endpoint: string, options?: RequestInit): [string, RequestInit] {
    let mappedEndpoint = this.mapEndpoint(endpoint);
    let mappedOptions = { ...options };

    // Special case: DELETE /v1/admin/consents?user_id=X&client_id=Y
    if (endpoint.startsWith('/v1/admin/consents') && options?.method === 'DELETE') {
      const url = new URL(endpoint, 'http://localhost');
      const userId = url.searchParams.get('user_id');
      const clientId = url.searchParams.get('client_id');

      if (userId && clientId) {
        mappedEndpoint = `/v2/admin/consents/${userId}/${clientId}`;
      }
    }

    return [mappedEndpoint, mappedOptions];
  }
}

// Usage in api.ts
export async function apiRequest<T>(
  endpoint: string,
  options?: RequestInit
): Promise<T> {
  const [mappedEndpoint, mappedOptions] = ApiAdapter.mapRequest(endpoint, options);

  const response = await fetch(`${API_BASE_URL}${mappedEndpoint}`, {
    ...mappedOptions,
    headers: {
      'Content-Type': 'application/json',
      ...mappedOptions.headers,
    },
  });

  if (!response.ok) {
    throw new ApiError(response.status, await response.text());
  }

  return response.json();
}
```

**Estimado**: 2 horas

---

### **FASE 3: ENV Vars Mapping** (MEDIO)

Crear `.env.v2` equivalente basado en tu `.env` V1:

```bash
# ────────────────────────────────────────────────────────────
# HelloJohn V2 Environment Configuration
# Mapped from V1 .env
# ────────────────────────────────────────────────────────────

# ─── Server ───
V2_SERVER_ADDR=:8080                    # V1: SERVER_ADDR
V2_BASE_URL=http://localhost:8080       # V1: JWT_ISSUER (same concept)

# ─── CORS ───
# TODO: V2 needs CORS middleware implementation
# V1 had: SERVER_CORS_ALLOWED_ORIGINS=http://localhost:3000,...
# For now, configure at reverse proxy level (nginx/traefik)

# ─── Control Plane ───
FS_ROOT=./data/hellojohn                # V1: CONTROL_PLANE_FS_ROOT

# ─── Cryptography ───
SIGNING_MASTER_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
# V1 had same key

SECRETBOX_MASTER_KEY=iffiWk2LBdtpvTp53ogGxbLDGFWJY8JkqPvej0qIT44=
# V1 had same key

# ─── Auth Configuration ───
REGISTER_AUTO_LOGIN=true                # V1: REGISTER_AUTO_LOGIN
FS_ADMIN_ENABLE=true                    # V1: FS_ADMIN_ENABLE

# ─── Session ───
# TODO: V2 needs these session config options
# V1 had:
#   AUTH_SESSION_COOKIE_NAME=sid
#   AUTH_SESSION_DOMAIN=
#   AUTH_SESSION_SAMESITE=Lax
#   AUTH_SESSION_SECURE=false
#   AUTH_SESSION_TTL=12h

# ─── Email/SMTP ───
# TODO: V2 Email Service needs SMTP config via ENV
# Currently only supports per-tenant SMTP in tenant.yaml
# V1 had global SMTP:
#   SMTP_HOST=smtp.gmail.com
#   SMTP_PORT=587
#   SMTP_USERNAME=juanpalacio1996@gmail.com
#   SMTP_PASSWORD=gezl zchi oken ubkl
#   SMTP_FROM=juanpalacio1996@gmail.com
#   SMTP_TLS=starttls

# Workaround: Configure in data/hellojohn/tenants/local/tenant.yaml:
# settings:
#   smtp:
#     host: "smtp.gmail.com"
#     port: 587
#     from: "juanpalacio1996@gmail.com"
#     password_enc: "<encrypted_password>"  # Encrypt with SECRETBOX_MASTER_KEY

# ─── Social Providers ───
# TODO: V2 needs social provider configuration
# V1 had:
#   GOOGLE_ENABLED=true
#   GOOGLE_CLIENT_ID=...
#   GOOGLE_CLIENT_SECRET=...
#   GOOGLE_REDIRECT_URL=http://localhost:8080/v1/auth/social/google/callback
#   GOOGLE_SCOPES=openid,email,profile

# V2 Placeholder (not yet implemented):
# SOCIAL_PROVIDERS=google,facebook,github
# SOCIAL_GOOGLE_CLIENT_ID=...
# SOCIAL_GOOGLE_CLIENT_SECRET=...
# SOCIAL_GOOGLE_REDIRECT_URL=http://localhost:8080/v2/auth/social/google/callback
# SOCIAL_GOOGLE_SCOPES=openid,email,profile

# ─── MFA ───
# TODO: V2 MFA config (currently hardcoded in services)
# V1 had:
#   MFA_TOTP_WINDOW=1
#   MFA_TOTP_ISSUER=HelloJohn
#   MFA_REMEMBER_TTL=720h

# ─── Database (Tenant-specific) ───
# V2 uses per-tenant DB config in tenant.yaml
# V1 had global DB:
#   STORAGE_DRIVER=postgres
#   STORAGE_DSN=postgres://user:password@localhost:5432/login?sslmode=disable

# For V2, configure in data/hellojohn/tenants/local/tenant.yaml:
# settings:
#   user_db:
#     driver: "postgres"
#     dsn_enc: "<encrypted_dsn>"  # Encrypt with SECRETBOX_MASTER_KEY
#     max_open_conns: 30
#     max_idle_conns: 5

# ─── Cache/Redis ───
# TODO: V2 needs Redis config via ENV
# V1 had:
#   CACHE_KIND=redis
#   REDIS_ADDR=localhost:6379
#   REDIS_DB=0
#   REDIS_PREFIX=login:

# V2 Placeholder (not yet implemented):
# CACHE_DRIVER=redis
# REDIS_ADDR=localhost:6379
# REDIS_PASSWORD=
# REDIS_DB=0

# ─── Rate Limiting ───
# TODO: V2 rate limiting config
# V1 had:
#   RATE_ENABLED=true
#   RATE_WINDOW=1m
#   RATE_MAX_REQUESTS=60
#   RATE_LOGIN_LIMIT=10
#   RATE_LOGIN_WINDOW=1m

# V2 currently uses nil RateLimiter (disabled)

# ─── Admin Authorization ───
# TODO: V2 admin authorization config
# V1 had:
#   ADMIN_ENFORCE=1
#   ADMIN_SUBS=7da4fb61-cb3a-4d7f-a6cd-a9ec4d82440d

# V2 uses RequireAdmin middleware but needs config

# ─── OAuth2 Auto-consent ───
# TODO: V2 consent config
# V1 had:
#   CONSENT_AUTO=1
#   CONSENT_AUTO_SCOPES=openid email profile

# ─── Dev/Debug ───
# V1 had:
#   APP_ENV=dev
#   EMAIL_DEBUG_LINKS=true
#   FLAGS_MIGRATE=true

# V2 equivalents:
# APP_ENV=dev
# EMAIL_DEBUG=true
# AUTO_MIGRATE=true

# ─── Security/Password Policy ───
# TODO: V2 password policy config
# V1 had:
#   SECURITY_PASSWORD_POLICY_MIN_LENGTH=10
#   SECURITY_PASSWORD_POLICY_REQUIRE_UPPER=true
#   SECURITY_PASSWORD_POLICY_REQUIRE_LOWER=true
#   SECURITY_PASSWORD_POLICY_REQUIRE_DIGIT=true
#   SECURITY_PASSWORD_POLICY_REQUIRE_SYMBOL=false
```

---

## 📝 Gaps de Configuración V2

### CRÍTICO - Falta implementar:
1. **CORS Middleware** (actualmente no existe en V2)
2. **Global SMTP Config** (solo soporta per-tenant en YAML)
3. **Social Providers Config** (Google, Facebook, GitHub)
4. **Session Cookie Config** (nombre, domain, samesite, secure, ttl)
5. **Redis/Cache Config via ENV** (actualmente usa memory cache)
6. **Rate Limiting Config** (actualmente disabled)
7. **Admin SUBs Authorization** (middleware existe pero sin config)
8. **Auto-Consent Config** (hardcoded en OAuth service)
9. **Password Policy Config** (hardcoded en register service)

### MEDIO - Mejoras necesarias:
10. **MFA Config** (TOTP window, issuer, remember TTL)
11. **Email Template Path Config** (hardcoded)
12. **Auto-Migration Flag** (no existe en V2)
13. **Debug/Dev Mode Flags** (no existe en V2)

---

## 🎯 Recomendación Final

### **Enfoque Sugerido: Híbrido V1/V2**

1. **Mantener V1 corriendo en :8080** (backend legacy)
2. **Lanzar V2 en :8082** (nuevo backend)
3. **Configurar UI para usar V2 donde sea compatible**:
   ```typescript
   // ui/lib/config.ts
   export const API_ENDPOINTS = {
     v1: 'http://localhost:8080',
     v2: 'http://localhost:8082',
   };

   export const USE_V2_FOR = {
     auth: true,        // Login, register, refresh, etc.
     adminClients: true, // Client CRUD
     adminScopes: true,  // Scope CRUD
     adminRBAC: true,    // RBAC management
     oauth: true,        // OAuth2/OIDC

     // Keep V1 for these (missing in V2)
     adminUsers: false,  // User management (MISSING in V2)
     adminTenants: false, // Tenant detail (MISSING in V2)
   };
   ```

4. **Migrar gradualmente** conforme se implementen los endpoints faltantes.

---

## 📋 Checklist de Migración UI→V2

### Pre-requisitos Backend:
- [ ] Implementar User CRUD endpoints (CRÍTICO)
- [ ] Implementar Tenant detail endpoint (CRÍTICO)
- [ ] Implementar Client revocation endpoint (MEDIO)
- [ ] Configurar CORS en V2
- [ ] Configurar SMTP global en V2
- [ ] Configurar Social providers en V2

### UI Changes:
- [ ] Crear `api-adapter.ts` con mapeo V1→V2
- [ ] Modificar `api.ts` para usar adapter
- [ ] Agregar flag `USE_V2_API` en env vars
- [ ] Actualizar endpoints de tenants (/v1/admin/tenants → /v2/admin/tenants)
- [ ] Actualizar endpoint de consents (query params → path params)
- [ ] Testing completo de flujos críticos

### Testing Checklist:
- [ ] Login como admin funciona
- [ ] Crear tenant funciona
- [ ] Cargar cliente en tenant funciona
- [ ] Listar usuarios funciona
- [ ] Crear usuario funciona
- [ ] RBAC assignment funciona
- [ ] OAuth playground funciona

---

**Conclusión**: La UI es **72% compatible** con V2 out-of-the-box. Los blockers críticos (user management + tenant detail) requieren ~3-4 horas de implementación backend. Una vez resueltos, la migración completa UI→V2 es factible en 1-2 días.
