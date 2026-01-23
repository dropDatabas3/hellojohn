# UI Routes Migration V1 → V2

## Endpoints Identificados en la UI

### ✅ Public/Auth Endpoints
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/readyz` | GET | Dashboard health check | `/readyz` | ✅ Same | health_routes.go |
| `/v1/auth/login` | POST | Login page | `/v2/auth/login` | ✅ Available | auth_routes.go:23 |
| `/v1/auth/register` | POST | Register page | `/v2/auth/register` | ✅ Available | auth_routes.go:26 |
| `/v1/auth/refresh` | POST | Token refresh | `/v2/auth/refresh` | ✅ Available | auth_routes.go:29 |
| `/v1/auth/logout` | POST | Logout | `/v2/auth/logout` | ✅ Available | auth_routes.go:50 |
| `/v1/auth/logout-all` | POST | Logout all sessions | `/v2/auth/logout-all` | ✅ Available | auth_routes.go:53 |
| `/v1/auth/providers` | GET | Get OAuth providers | `/v2/auth/providers` | ✅ Available | auth_routes.go:35 |
| `/v1/auth/config` | GET | Get auth config/branding | `/v2/auth/config` | ✅ Available | auth_routes.go:32 |
| `/v1/me` | GET | Get current user | `/v2/me` | ✅ Available | auth_routes.go:44 |
| `/v1/profile` | GET | Get user profile | `/v2/profile` | ✅ Available | auth_routes.go:47 |
| `/v1/session/login` | POST | Cookie session login | `/v2/session/login` | ✅ Available | session_routes.go:26 |
| `/v1/session/logout` | POST | Cookie session logout | `/v2/session/logout` | ✅ Available | session_routes.go:23 |

### 🔧 Admin - Tenants
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/admin/tenants` | GET | List all tenants | `/v2/admin/tenants` | ✅ Available | tenants_routes.go:48 |
| `/v1/admin/tenants` | POST | Create tenant | `/v2/admin/tenants` | ✅ Available | tenants_routes.go:50 |
| `/v1/admin/tenants/{id}` | GET | Get tenant details | `/v2/admin/tenants/{id}` | ✅ Available | tenants_routes.go:148 |
| `/v1/admin/tenants/{id}` | PUT | Update tenant | `/v2/admin/tenants/{id}` | ✅ Available | tenants_routes.go:150 |
| `/v1/admin/tenants/{id}` | DELETE | Delete tenant | `/v2/admin/tenants/{id}` | ✅ Available | tenants_routes.go:152 |
| `/v1/admin/tenants/{id}/settings` | GET | Get tenant settings | `/v2/admin/tenants/{id}/settings` | ✅ Available | tenants_routes.go:82 |
| `/v1/admin/tenants/{id}/settings` | PUT | Update tenant settings | `/v2/admin/tenants/{id}/settings` | ✅ Available | tenants_routes.go:84 |
| `/v1/admin/tenants/{id}/users` | GET | List users in tenant | `/v2/admin/tenants/{id}/users` | ✅ Available | admin_routes.go:56 |
| `/v1/admin/tenants/{id}/users` | POST | Create user | `/v2/admin/tenants/{id}/users` | ✅ Available | admin_routes.go:55 |
| `/v1/admin/tenants/{id}/users/{userId}` | GET | Get user | `/v2/admin/tenants/{id}/users/{userId}` | ✅ Available | admin_routes.go:57 |
| `/v1/admin/tenants/{id}/users/{userId}` | PUT | Update user | `/v2/admin/tenants/{id}/users/{userId}` | ✅ Available | admin_routes.go:58 |
| `/v1/admin/tenants/{id}/users/{userId}` | DELETE | Delete user | `/v2/admin/tenants/{id}/users/{userId}` | ✅ Available | admin_routes.go:59 |
| `/v1/admin/tenants/{id}/migrate` | POST | Run migrations | `/v2/admin/tenants/{id}/migrate` | ✅ Available | tenants_routes.go:95 |
| `/v1/admin/tenants/{id}/schema/apply` | POST | Apply schema | `/v2/admin/tenants/{id}/schema/apply` | ✅ Available | tenants_routes.go:102 |
| `/v1/admin/tenants/{id}/keys/rotate` | POST | Rotate tenant keys | `/v2/admin/tenants/{id}/keys/rotate` | ✅ Available | tenants_routes.go:76 |
| `/v1/admin/tenants/test-connection` | POST | Test DB connection | `/v2/admin/tenants/test-connection` | ✅ Available | tenants_routes.go:37 |

### 🔧 Admin - Clients (OAuth Apps)
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/admin/clients` | GET | List clients | `/v2/admin/clients` | ✅ Available | admin_routes.go:109 |
| `/v1/admin/clients` | POST | Create client | `/v2/admin/clients` | ✅ Available | admin_routes.go:111 |
| `/v1/admin/clients/{clientId}` | PUT/PATCH | Update client | `/v2/admin/clients/{clientId}` | ✅ Available | admin_routes.go:127 |
| `/v1/admin/clients/{clientId}` | DELETE | Delete client | `/v2/admin/clients/{clientId}` | ✅ Available | admin_routes.go:129 |
| `/v1/admin/clients/{clientId}/revoke` | POST | Revoke secret | `/v2/admin/clients/{clientId}/revoke` | ✅ Available | admin_routes.go:119 |

### 🔧 Admin - Scopes
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/admin/scopes` | GET | List scopes | `/v2/admin/scopes` | ✅ Available | admin_routes.go:206 |
| `/v1/admin/scopes` | POST/PUT | Upsert scope | `/v2/admin/scopes` | ✅ Available | admin_routes.go:208 |
| `/v1/admin/scopes/{name}` | DELETE | Delete scope | `/v2/admin/scopes/{name}` | ✅ Available | admin_routes.go:216 |

### 🔧 Admin - Consents
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/admin/consents` | GET | List consents | `/v2/admin/consents` | ✅ Available | admin_routes.go:159 |
| `/v1/admin/consents/upsert` | POST | Upsert consent | `/v2/admin/consents/upsert` | ✅ Available | admin_routes.go:150 |
| `/v1/admin/consents/revoke` | POST | Revoke consent | `/v2/admin/consents/revoke` | ✅ Available | admin_routes.go:153 |
| `/v1/admin/consents/{id}` | DELETE | Delete consent | `/v2/admin/consents/{id}` | ✅ Available | admin_routes.go:162 |
| `/v1/admin/consents/by-user/{userId}` | GET | Get user consents | `/v2/admin/consents/by-user/{userId}` | ✅ Available | admin_routes.go:156 |

### 🔧 Admin - RBAC
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/admin/rbac/users/{userId}/roles` | GET | Get user roles | `/v2/admin/rbac/users/{userId}/roles` | ✅ Available | admin_routes.go:240 |
| `/v1/admin/rbac/users/{userId}/roles` | POST | Update user roles | `/v2/admin/rbac/users/{userId}/roles` | ✅ Available | admin_routes.go:242 |
| `/v1/admin/rbac/roles/{role}/perms` | GET | Get role permissions | `/v2/admin/rbac/roles/{role}/perms` | ✅ Available | admin_routes.go:250 |
| `/v1/admin/rbac/roles/{role}/perms` | POST | Update role perms | `/v2/admin/rbac/roles/{role}/perms` | ✅ Available | admin_routes.go:252 |

### 🔧 Admin - Users
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/admin/users/disable` | POST | Disable user | `/v2/admin/users/disable` | ✅ Available | admin_routes.go:183 |
| `/v1/admin/users/enable` | POST | Enable user | `/v2/admin/users/enable` | ✅ Available | admin_routes.go:185 |
| `/v1/admin/users/resend-verification` | POST | Resend verification | `/v2/admin/users/resend-verification` | ✅ Available | admin_routes.go:187 |

### 🔧 Admin - Keys
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/keys` | GET | List keys | `/v2/keys` | ⚠️ TODO | - |
| `/v1/keys/rotate` | POST | Rotate signing key | `/v2/keys/rotate` | ⚠️ TODO | - |

### 🔧 Admin - Stats
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/admin/stats` | GET | Get system stats | `/v2/admin/stats` | ⚠️ TODO | - |

### 🔧 Admin - Providers (Social)
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/providers/status` | GET | Get providers status | `/v2/providers/status` | ✅ Available | auth_routes.go:38 |

### 🔧 Admin - Config
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/admin/config` | GET | Get admin config | `/v2/admin/config` | ⚠️ TODO | - |
| `/v1/admin/config` | PUT | Update admin config | `/v2/admin/config` | ⚠️ TODO | - |

### 🌐 OIDC/OAuth2
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/.well-known/openid-configuration` | GET | OIDC discovery | `/.well-known/openid-configuration` | ✅ Same | oidc_routes.go:30 |
| `/.well-known/jwks.json` | GET | Get JWKS | `/.well-known/jwks.json` | ✅ Same | oidc_routes.go:24 |
| `/t/{slug}/.well-known/openid-configuration` | GET | OIDC discovery tenant | `/t/{slug}/.well-known/openid-configuration` | ✅ Same | oidc_routes.go:33 |
| `/oauth2/authorize` | GET | OAuth authorization | `/oauth2/authorize` | ✅ Same | oauth_routes.go:21 |
| `/oauth2/token` | POST | Token exchange | `/oauth2/token` | ✅ Same | oauth_routes.go:24 |
| `/oauth2/revoke` | POST | Revoke token | `/oauth2/revoke` | ✅ Same | oauth_routes.go:27 |
| `/oauth2/introspect` | POST | Token introspection | `/oauth2/introspect` | ✅ Same | oauth_routes.go:30 |
| `/userinfo` | GET/POST | Get user info | `/userinfo` | ✅ Same | oidc_routes.go:36 |
| `/v2/auth/consent/accept` | POST | Accept consent | `/v2/auth/consent/accept` | ✅ Available | oauth_routes.go:33 |

### 🔐 MFA
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/mfa/totp/enroll` | POST | Enroll TOTP | `/v2/mfa/totp/enroll` | ✅ Available | mfa_routes.go:28 |
| `/v1/mfa/totp/verify` | POST | Verify TOTP | `/v2/mfa/totp/verify` | ✅ Available | mfa_routes.go:31 |
| `/v1/mfa/totp/challenge` | POST | Challenge TOTP | `/v2/mfa/totp/challenge` | ✅ Available | mfa_routes.go:34 |
| `/v1/mfa/totp/disable` | POST | Disable TOTP | `/v2/mfa/totp/disable` | ✅ Available | mfa_routes.go:37 |
| `/v1/mfa/recovery/rotate` | POST | Rotate recovery codes | `/v2/mfa/recovery/rotate` | ✅ Available | mfa_routes.go:40 |

### 📧 Email Flows
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/auth/verify-email/start` | POST | Start email verification | `/v2/auth/verify-email/start` | ✅ Available | email_routes.go:23 |
| `/v1/auth/verify-email` | GET | Verify email token | `/v2/auth/verify-email` | ✅ Available | email_routes.go:26 |
| `/v1/auth/forgot` | POST | Request password reset | `/v2/auth/forgot` | ✅ Available | email_routes.go:29 |
| `/v1/auth/reset` | POST | Reset password | `/v2/auth/reset` | ✅ Available | email_routes.go:32 |

### 🔗 Social Auth
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/auth/social/exchange` | POST | Exchange social code | `/v2/auth/social/exchange` | ✅ Available | social_routes.go:21 |
| `/v1/auth/social/result` | GET | View social result | `/v2/auth/social/result` | ✅ Available | social_routes.go:24 |
| `/v1/auth/social/{provider}/start` | GET | Start social flow | `/v2/auth/social/{provider}/start` | ✅ Available | social_routes.go:27 |
| `/v1/auth/social/{provider}/callback` | GET | OAuth callback | `/v2/auth/social/{provider}/callback` | ✅ Available | social_routes.go:30 |

### 🛡️ Security
| V1 Endpoint | Método | Uso en UI | V2 Equivalent | Status | Archivo Router |
|-------------|--------|-----------|---------------|--------|----------------|
| `/v1/csrf` | GET | Get CSRF token | `/v2/csrf` | ⚠️ TODO | - |
| `/v1/auth/consent/accept` | POST | Accept consent | `/v2/auth/consent/accept` | ✅ Available | oauth_routes.go:33 |

---

## Notas de Migración

### ✅ Rutas que NO cambian (Standard OAuth2/OIDC)
Estas rutas son estándares y se mantienen igual:
- `/.well-known/*` (OIDC Discovery + JWKS)
- `/oauth2/*` (authorize, token, revoke, introspect)
- `/userinfo` (OIDC UserInfo)
- `/readyz` (Health check)

### 🔄 Cambio Global
**Todas las rutas `/v1/*` se convierten en `/v2/*`**

Ejemplos:
- `/v1/auth/login` → `/v2/auth/login`
- `/v1/admin/tenants` → `/v2/admin/tenants`
- `/v1/mfa/totp/enroll` → `/v2/mfa/totp/enroll`

### ⚠️ Endpoints Pendientes (TODO V2)
Los siguientes endpoints **NO están implementados en V2** y necesitan ser creados:

1. **Admin Keys** (`/v2/keys`, `/v2/keys/rotate`)
   - Lista y rotación de signing keys global
   - Requiere: Controller, Service, Router

2. **Admin Stats** (`/v2/admin/stats`)
   - Estadísticas del sistema (users, tenants, tokens, etc.)
   - Requiere: Controller, Service, Router

3. **Admin Config** (`/v2/admin/config`)
   - Configuración global del sistema
   - Requiere: Controller, Service, Router

4. **CSRF** (`/v2/csrf`)
   - Token CSRF para formularios
   - Requiere: Controller, Service, Router

### ✅ Rutas Verificadas en V2
**Total: 55+ endpoints migrados**

- ✅ Auth (login, register, refresh, logout, providers, config, me, profile)
- ✅ Session (login, logout)
- ✅ Admin Tenants (CRUD completo + settings, migrate, schema, keys rotate, test-connection)
- ✅ Admin Clients (CRUD completo + revoke)
- ✅ Admin Scopes (list, upsert, delete)
- ✅ Admin Consents (list, upsert, revoke, delete, by-user)
- ✅ Admin RBAC (user roles, role perms)
- ✅ Admin Users (disable, enable, resend-verification)
- ✅ Admin User CRUD (create, list, get, update, delete)
- ✅ OAuth2/OIDC (authorize, token, revoke, introspect, consent)
- ✅ MFA (enroll, verify, challenge, disable, recovery rotate)
- ✅ Email Flows (verify-email start/confirm, forgot, reset)
- ✅ Social Auth (exchange, result, start, callback)

---

## Plan de Acción

1. ✅ Identificar todas las rutas V1 usadas por UI
2. ✅ Verificar disponibilidad en V2 (revisar router/\*.go)
3. ⏳ Crear utilidad de mapeo de rutas en `ui/lib/routes.ts`
4. ⏳ Actualizar API client (`ui/lib/api.ts`) con rutas V2
5. ⬜ Mapear DTOs V1 ↔ V2 (verificar compatibilidad)
6. ⬜ Implementar endpoints faltantes (keys, stats, config, csrf)
7. ⬜ Testing por módulo (auth → admin → oauth → mfa)
8. ⬜ Testing end-to-end (flujos completos UI)
