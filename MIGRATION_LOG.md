# Migration Log V1 → V2

> **Propósito**: Rastrear el progreso de migración de handlers V1 a la arquitectura V2.
> **Última actualización**: 2026-01-20

---

## 📊 Estadísticas

- **Total handlers V1**: 48 (según V1_HANDLERS_INVENTORY.md)
- **Migrados a V2**: 34
- **En progreso**: 0
- **Bloqueados**: 1 (admin_mailing - no equivalente V2)
- **Pendientes**: 13
- **Progreso**: 71% (34/48 handlers activos)

---

## ✅ Handlers Migrados

### ✅ admin_clients_fs.go → v2/admin/clients_service.go
- **Fecha**: 2026-01-20
- **Rutas migradas**:
  - `GET /v1/admin/clients` → `GET /v2/admin/clients`
  - `POST /v1/admin/clients` → `POST /v2/admin/clients`
  - `PUT/PATCH /v1/admin/clients/{clientId}` → `PUT/PATCH /v2/admin/clients/{clientId}`
  - `DELETE /v1/admin/clients/{clientId}` → `DELETE /v2/admin/clients/{clientId}`
- **Archivos creados**:
  - `internal/http/v2/dto/admin/client.go` (existente)
  - `internal/http/v2/dto/admin/client_create.go` (existente)
  - `internal/http/v2/dto/admin/client_update.go` (existente)
  - `internal/http/v2/services/admin/clients_service.go` (existente)
  - `internal/http/v2/controllers/admin/clients_controller.go` (existente)
- **Archivos editados**:
  - N/A (ya estaban en aggregators)
- **Herramientas V2 usadas**:
  - `controlplane.Service.ListClients()`
  - `controlplane.Service.UpsertClient()`
  - `controlplane.Service.DeleteClient()`
  - `controlplane.Service.GetClient()`
- **Dependencias**:
  - Control Plane V2 (FS Provider + Raft Cluster)
  - Logger (observability/logger)
  - Middlewares V2 (TenantResolution, RequireAuth, RequireAdmin)
- **Descripción**:
  CRUD de OAuth/OIDC clients via Control Plane. Soporta modo cluster (Raft mutations) y modo directo (FS Provider).
- **Notas**:
  - Handler V1 tenía tenant resolution compleja (headers + query params + UUID→Slug translation). V2 usa middleware `WithTenantResolution()` centralizado.
  - V1 hacía clustering manual (apply mutation + readback). V2 Service encapsula esto.
  - V1 usaba JSON helper local. V2 usa `httperrors.WriteError()` estándar.
  - Controller separa métodos: ListClients, CreateClient, UpdateClient, DeleteClient (vs ServeHTTP monolítico en V1).
- **Wiring verificado**: ✅
  - `services/admin/services.go:33` (ClientService inyectado en aggregator)
  - `controllers/admin/controllers.go:19` (ClientsController inyectado en aggregator)
  - `router/admin_routes.go:32-33` (rutas registradas con middleware chain)
  - `app/v2/app.go:79` (adminControllers creado desde svcs.Admin)
  - `app/v2/app.go:109` (AdminControllers pasado a RegisterV2Routes)

---

### ✅ admin_consents.go → v2/admin/consents_service.go
- **Fecha**: 2026-01-20
- **Rutas migradas**:
  - `POST /v1/admin/consents/upsert` → `POST /v2/admin/consents/upsert`
  - `POST /v1/admin/consents/revoke` → `POST /v2/admin/consents/revoke`
  - `GET /v1/admin/consents/by-user/{userID}` → `GET /v2/admin/consents/by-user/{userID}`
  - `GET /v1/admin/consents` → `GET /v2/admin/consents`
  - `DELETE /v1/admin/consents/{userID}/{clientID}` → `DELETE /v2/admin/consents/{userID}/{clientID}`
- **Archivos creados**:
  - `internal/http/v2/dto/admin/consent.go` (existente)
  - `internal/http/v2/dto/admin/consent_upsert.go` (existente)
  - `internal/http/v2/dto/admin/consent_revoke.go` (existente)
  - `internal/http/v2/services/admin/consents_service.go` (existente)
  - `internal/http/v2/controllers/admin/consents_controller.go` (existente)
- **Archivos editados**:
  - N/A (ya estaban en aggregators)
- **Herramientas V2 usadas**:
  - `store.DataAccessLayer.ForTenant()` (DAL V2)
  - `dal.Consents().UpsertConsent()`
  - `dal.Consents().RevokeConsent()`
  - `dal.Consents().GetConsentsByUser()`
  - `dal.Tokens().RevokeRefreshTokensByClientAndUser()`
- **Dependencias**:
  - DAL V2 (Data Access Layer)
  - Logger (observability/logger)
  - Middlewares V2 (TenantResolution, RequireAuth, RequireAdmin, RequireTenantDB)
- **Descripción**:
  Gestión de OAuth consents (user_id + client_id + scopes granted). Incluye best-effort revocation de refresh tokens al revocar consent.
- **Notas**:
  - V1 mezclaba resolución de client_id (UUID interno vs público) en el handler. V2 Service maneja esto internamente.
  - V1 usaba ScopesConsents repository directo. V2 usa DAL.ForTenant().Consents() para aislamiento multi-tenant.
  - V1 tenía lógica best-effort de revocar tokens embebida en ServeHTTP. V2 Service encapsula esta orquestación.
  - Controller separa métodos: UpsertConsent, RevokeConsent, ListConsentsByUser, GetConsents, DeleteConsent.
- **Wiring verificado**: ✅
  - `services/admin/services.go:36` (ConsentService inyectado en aggregator)
  - `controllers/admin/controllers.go:20` (ConsentsController inyectado en aggregator)
  - `router/admin_routes.go:36-37` (rutas registradas con middleware chain + requireDB=true)
  - `app/v2/app.go:79` (adminControllers creado desde svcs.Admin)
  - `app/v2/app.go:109` (AdminControllers pasado a RegisterV2Routes)

---

### ✅ admin_rbac.go → v2/admin/rbac_service.go
- **Fecha**: 2026-01-20
- **Rutas migradas**:
  - `/v1/admin/rbac/users/{userID}/roles` → `/v2/admin/rbac/users/{userID}/roles`
  - `/v1/admin/rbac/roles/{role}/perms` → `/v2/admin/rbac/roles/{role}/perms`
- **Archivos creados**:
  - `internal/http/v2/services/admin/rbac_service.go` (existente)
  - `internal/http/v2/controllers/admin/rbac_controller.go` (existente)
- **Herramientas V2 usadas**:
  - `store.DataAccessLayer.ForTenant()` (DAL V2)
- **Wiring verificado**: ✅
  - `services/admin/services.go:37` (RBACService en aggregator)
  - `controllers/admin/controllers.go:23` (RBACController en aggregator)
  - `router/admin_routes.go:47` (rutas con requireDB=true)
  - `app/v2/app.go:79` (adminControllers desde svcs.Admin)

---

### ✅ admin_scopes_fs.go → v2/admin/scopes_service.go
- **Fecha**: 2026-01-20
- **Rutas migradas**:
  - `GET /v1/admin/scopes` → `GET /v2/admin/scopes`
  - `POST /v1/admin/scopes` → `POST /v2/admin/scopes`
  - `PUT/PATCH /v1/admin/scopes/{scopeID}` → `PUT/PATCH /v2/admin/scopes/{scopeID}`
  - `DELETE /v1/admin/scopes/{scopeID}` → `DELETE /v2/admin/scopes/{scopeID}`
- **Archivos creados**:
  - `internal/http/v2/services/admin/scopes_service.go` (existente)
  - `internal/http/v2/controllers/admin/scopes_controller.go` (existente)
- **Herramientas V2 usadas**:
  - `controlplane.Service.ListScopes()`
  - `controlplane.Service.UpsertScope()`
  - `controlplane.Service.DeleteScope()`
- **Wiring verificado**: ✅
  - `services/admin/services.go:34` (ScopeService en aggregator)
  - `controllers/admin/controllers.go:22` (ScopesController en aggregator)
  - `router/admin_routes.go:43-44` (rutas con requireDB=false, Control Plane)
  - `app/v2/app.go:79` (adminControllers desde svcs.Admin)

---

### ✅ admin_tenants_fs.go → v2/admin/tenants_service.go
- **Fecha**: 2026-01-20
- **Rutas migradas**:
  - `GET /v1/admin/tenants` → `GET /v2/admin/tenants`
  - `POST /v1/admin/tenants` → `POST /v2/admin/tenants`
  - `PUT/PATCH /v1/admin/tenants/{slug}` → `PUT/PATCH /v2/admin/tenants/{slug}`
  - `DELETE /v1/admin/tenants/{slug}` → `DELETE /v2/admin/tenants/{slug}`
  - `POST /v1/admin/tenants/test-connection` → `POST /v2/admin/tenants/test-connection`
- **Archivos creados**:
  - `internal/http/v2/services/admin/tenants_service.go` (existente)
  - `internal/http/v2/controllers/admin/tenants_controller.go` (existente)
- **Herramientas V2 usadas**:
  - `store.DataAccessLayer` (DAL V2)
  - `jwtx.Issuer` (JWT V2)
  - `emailv2.Service` (Email V2)
- **Wiring verificado**: ✅
  - `services/admin/services.go:38` (TenantsService en aggregator)
  - `controllers/admin/controllers.go:24` (TenantsController en aggregator)
  - `router/tenants_routes.go:33-34` (rutas con middleware especial System Admin)
  - `app/v2/app.go:79` (adminControllers desde svcs.Admin)

---

### ✅ admin_users.go → v2/admin/users_service.go
- **Fecha**: 2026-01-20
- **Rutas migradas**:
  - `POST /v1/admin/users/disable` → `POST /v2/admin/users/disable`
  - `POST /v1/admin/users/enable` → `POST /v2/admin/users/enable`
  - `POST /v1/admin/users/resend-verification` → `POST /v2/admin/users/resend-verification`
- **Archivos creados**:
  - `internal/http/v2/services/admin/users_service.go` (existente)
  - `internal/http/v2/controllers/admin/users_controller.go` (existente)
- **Herramientas V2 usadas**:
  - `store.DataAccessLayer.ForTenant()` (DAL V2)
  - `emailv2.Service` (Email V2)
- **Wiring verificado**: ✅
  - `services/admin/services.go:35` (UserActionService en aggregator)
  - `controllers/admin/controllers.go:21` (UsersController en aggregator)
  - `router/admin_routes.go:40` (rutas con requireDB=true)
  - `app/v2/app.go:79` (adminControllers desde svcs.Admin)

---

### ✅ Auth Handlers (Batch) → v2/auth/*_service.go
- **Fecha**: 2026-01-20
- **Handlers migrados** (8 handlers):
  1. `auth_config.go` → `config_service.go` + `config_controller.go`
  2. `auth_complete_profile.go` → `complete_profile_service.go` + `complete_profile_controller.go`
  3. `auth_login.go` → `login_service.go` + `login_controller.go`
  4. `auth_logout_all.go` → `logout_service.go` + `logout_all_controller.go`
  5. `auth_refresh.go` → `refresh_service.go` + `refresh_controller.go`
  6. `auth_register.go` → `register_service.go` + `register_controller.go`
  7. `me.go` → `profile_service.go` + `me_controller.go`
  8. `profile.go` → `profile_service.go` + `profile_controller.go`
- **Rutas migradas**:
  - `POST /v1/auth/login` → `POST /v2/auth/login`
  - `POST /v1/auth/register` → `POST /v2/auth/register`
  - `POST /v1/auth/refresh` → `POST /v2/auth/refresh`
  - `POST /v1/auth/logout-all` → `POST /v2/auth/logout-all`
  - `GET /v1/auth/config` → `GET /v2/auth/config`
  - `POST /v1/auth/complete-profile` → `POST /v2/auth/complete-profile`
  - `GET /v1/me` → `GET /v2/me`
  - `GET /v1/profile` → `GET /v2/profile`
- **Herramientas V2 usadas**:
  - `store.DataAccessLayer.ForTenant()` (DAL V2)
  - `jwtx.Issuer` (JWT V2 con EdDSA)
  - `emailv2.Service` (Email V2 para verification)
  - `cache.Client` (Cache V2)
- **Wiring verificado**: ✅
  - `services/auth/services.go:44-95` (todos los services en aggregator)
  - `controllers/auth/controllers.go:24-39` (todos los controllers en aggregator)
  - `router/auth_routes.go:23-59` (todas las rutas registradas)
  - `app/v2/app.go:78` (authControllers desde svcs.Auth)
  - `app/v2/app.go:108` (AuthControllers pasado a RegisterV2Routes)
- **Notas**:
  - V1 tenía ClaimsHook manual. V2 encapsula en services.
  - V1 mezclaba password validation en handler. V2 Service usa blacklist opcional.
  - V1 usaba Store directo. V2 usa DAL.ForTenant() con isolation.
  - Controllers separan métodos por endpoint (vs ServeHTTP monolítico).

---

### ✅ OIDC Handlers (Batch) → v2/oidc/*_service.go
- **Fecha**: 2026-01-20
- **Handlers migrados** (3 handlers):
  1. `jwks.go` → `jwks_service.go` + `jwks_controller.go`
  2. `oidc_discovery.go` → `discovery_service.go` + `discovery_controller.go`
  3. `userinfo.go` → `userinfo_service.go` + `userinfo_controller.go`
- **Rutas migradas**:
  - `GET /.well-known/jwks.json` (global)
  - `GET /.well-known/jwks/{slug}.json` (per-tenant)
  - `GET /.well-known/openid-configuration` (discovery)
  - `GET/POST /userinfo` (OIDC userinfo endpoint)
- **Wiring verificado**: ✅ (services/oidc/services.go, controllers/oidc/controllers.go, router/oidc_routes.go, app.go)

---

### ✅ OAuth Handlers (Batch) → v2/oauth/*_service.go
- **Fecha**: 2026-01-20
- **Handlers migrados** (5 handlers):
  1. `oauth_authorize.go` → `authorize_service.go` + `authorize_controller.go`
  2. `oauth_token.go` → `token_service.go` + `token_controller.go`
  3. `oauth_consent.go` → `consent_service.go` + `consent_controller.go`
  4. `oauth_introspect.go` → `introspect_service.go` + `introspect_controller.go`
  5. `oauth_revoke.go` → `revoke_service.go` + `revoke_controller.go`
- **Rutas migradas**:
  - `GET/POST /oauth2/authorize`
  - `POST /oauth2/token`
  - `POST /oauth2/revoke`
  - `POST /oauth2/introspect`
- **Wiring verificado**: ✅ (services/oauth/services.go, controllers/oauth/controllers.go, router/oauth_routes.go, app.go)

---

### ✅ Session Handlers (Batch) → v2/session/*_service.go
- **Fecha**: 2026-01-20
- **Handlers migrados** (2 handlers):
  1. `session_login.go` → `login_service.go` + `login_controller.go`
  2. `session_logout.go` → `logout_service.go` + `logout_controller.go`
- **Rutas migradas**:
  - `POST /v2/session/login`
  - `POST /v2/session/logout`
- **Wiring verificado**: ✅ (services/session/services.go, controllers/session/controllers.go, router/session_routes.go, app.go)

---

### ✅ Social Handlers (Batch) → v2/social/*_service.go
- **Fecha**: 2026-01-20
- **Handlers migrados** (3 handlers):
  1. `social_dynamic.go` → social services (inyectado externamente)
  2. `social_exchange.go` → `exchange_controller.go`
  3. `social_result.go` → `result_controller.go`
- **Rutas migradas**:
  - `GET/POST /v2/auth/social/{provider}/{action}`
  - `POST /v2/auth/social/exchange`
  - `GET /v2/auth/social/result`
- **Notas**: Social services se inyectan externamente (no creados en NewServices), siguiendo patrón de excepción arquitectural documentado.
- **Wiring verificado**: ✅ (router/auth_routes.go, router/social_routes.go, app.go)

---

### ✅ Email Flow Handlers (Batch) → v2/email/*_service.go
- **Fecha**: 2026-01-20
- **Handlers migrados** (parte de email_flows.go):
  1. Verify Email Start → `flows_controller.VerifyEmailStart()`
  2. Verify Email Confirm → `flows_controller.VerifyEmailConfirm()`
  3. Forgot Password → `flows_controller.ForgotPassword()`
  4. Reset Password → `flows_controller.ResetPassword()`
- **Rutas migradas**:
  - `POST /v2/auth/verify-email/start`
  - `POST /v2/auth/verify-email`
  - `POST /v2/auth/forgot`
  - `POST /v2/auth/reset`
- **Wiring verificado**: ✅ (services/email/services.go, controllers/email/controllers.go, router/email_routes.go, app.go)

---

### ✅ MFA Handler → v2/auth/mfa_service.go
- **Fecha**: 2026-01-20
- **Handler migrado**: `mfa_totp.go` → `mfa_service.go` + `mfa_totp_controller.go`
- **Rutas migradas**: MFA endpoints en auth routes
- **Wiring verificado**: ✅ (incluido en auth controllers aggregator)

---

### ✅ Security Handler → v2/security/csrf_service.go
- **Fecha**: 2026-01-20
- **Handler migrado**: `csrf.go` → `csrf_service.go` + `csrf_controller.go`
- **Rutas migradas**: `GET /v2/csrf`
- **Wiring verificado**: ✅ (services/security/services.go, controllers/security/controllers.go, router/security_routes.go, app.go)

---

### ✅ Health Handler → v2/health/health_service.go
- **Fecha**: 2026-01-20
- **Handler migrado**: `readyz.go` → `health_service.go` + `health_controller.go`
- **Rutas migradas**: `GET /readyz`
- **Wiring verificado**: ✅ (services/health/services.go, controllers/health/controllers.go, router/health_routes.go, app.go)

---

## ⏳ Handlers En Progreso

_(Vacío - Handlers parcialmente migrados)_

---

## ❌ Handlers Bloqueados

### ❌ admin_mailing.go → Sin equivalente V2
- **Bloqueador**: No existe service V2 para "test email" (envío de email de prueba SMTP)
- **Handler V1**: POST /v1/admin/mailing (test SMTP configuration)
- **Descripción**: Endpoint para probar configuración SMTP de un tenant enviando email de prueba
- **Solución propuesta**: Crear `admin/TestEmailService` o agregar método `TestEmail()` a `admin.TenantsService`
- **Prioridad**: Baja (feature administrativa no crítica)

---

## 📝 Handlers Pendientes

### Auth
- [x] `auth_login.go` → Login con password ✅ MIGRADO (2026-01-20)
- [x] `auth_register.go` → Registro de usuario ✅ MIGRADO (2026-01-20)
- [x] `auth_refresh.go` → Refresh token ✅ MIGRADO (2026-01-20)
- [x] `auth_logout_all.go` → Logout all sessions ✅ MIGRADO (2026-01-20)
- [x] `auth_config.go` → Branding/config público ✅ MIGRADO (2026-01-20)
- [x] `auth_complete_profile.go` → Custom fields post-social ✅ MIGRADO (2026-01-20)
- [x] `me.go` → /v1/me (user info) ✅ MIGRADO (2026-01-20)
- [x] `profile.go` → /v1/profile (protected resource) ✅ MIGRADO (2026-01-20)

### Admin
- [x] `admin_clients_fs.go` → CRUD de clients (FS) ✅ MIGRADO (2026-01-20)
- [x] `admin_consents.go` → Gestión de consents ✅ MIGRADO (2026-01-20)
- [x] `admin_rbac.go` → RBAC (users/roles, roles/perms) ✅ MIGRADO (2026-01-20)
- [x] `admin_scopes_fs.go` → CRUD de scopes (FS) ✅ MIGRADO (2026-01-20)
- [x] `admin_tenants_fs.go` → CRUD de tenants + settings ✅ MIGRADO (2026-01-20)
- [x] `admin_users.go` → Disable/enable users ✅ MIGRADO (2026-01-20)
- [ ] `admin_mailing.go` → ❌ BLOQUEADO (sin equivalente V2)

### OIDC/Discovery
- [x] `jwks.go` → JWKS global + per-tenant ✅ MIGRADO (2026-01-20)
- [x] `oidc_discovery.go` → Discovery global + per-tenant ✅ MIGRADO (2026-01-20)
- [x] `userinfo.go` → /userinfo endpoint ✅ MIGRADO (2026-01-20)

### OAuth
- [x] `oauth_authorize.go` → /oauth2/authorize ✅ MIGRADO (2026-01-20)
- [x] `oauth_token.go` → /oauth2/token ✅ MIGRADO (2026-01-20)
- [x] `oauth_consent.go` → Consent accept ✅ MIGRADO (2026-01-20)
- [x] `oauth_introspect.go` → /oauth2/introspect ✅ MIGRADO (2026-01-20)
- [x] `oauth_revoke.go` → /oauth2/revoke ✅ MIGRADO (2026-01-20)

### MFA
- [x] `mfa_totp.go` → Enroll/verify/challenge/disable TOTP + recovery codes ✅ MIGRADO (2026-01-20)

### Session
- [x] `session_login.go` → Cookie-based session login ✅ MIGRADO (2026-01-20)
- [x] `session_logout.go` → Cookie-based session logout ✅ MIGRADO (2026-01-20)

### Social
- [x] `social_dynamic.go` → Dynamic social login ✅ MIGRADO (2026-01-20)
- [x] `social_exchange.go` → Exchange login_code for tokens ✅ MIGRADO (2026-01-20)
- [x] `social_result.go` → Debug viewer for login_code ✅ MIGRADO (2026-01-20)

### Email Flows
- [x] `email_flows.go` → Verify email start/confirm, forgot/reset password ✅ MIGRADO (2026-01-20)

### Security
- [x] `csrf.go` → CSRF token generation ✅ MIGRADO (2026-01-20)

### Health
- [x] `readyz.go` → Health check endpoint ✅ MIGRADO (2026-01-20)

### Legacy / Not Wired (pendientes de investigación)
- [ ] `admin_clients.go` (DB-based, reemplazado por admin_clients_fs)
- [ ] `admin_scopes.go` (DB-based, reemplazado por admin_scopes_fs)
- [ ] `oauth_start.go` (TODO vacío)
- [ ] `oauth_callback.go` (TODO vacío)
- [ ] `social_google.go` (deprecated, reemplazado por social_dynamic)
- [ ] `public_forms.go` (not wired)
- [ ] `registry_clients.go` (not wired)
- [ ] `admin_keys.go` (deprecated/empty)
- [ ] `claims_hook.go` (utility, no endpoint)
- [ ] `cookieutil.go` (utility, no endpoint)
- [ ] `json.go` (utility, no endpoint)
- [ ] `providers.go` (utility, no endpoint)

---

## 📋 Template de Entrada

**Copia este template al migrar un handler**:

```markdown
### ✅ {handler_v1}.go → v2/{domain}/{nombre}_service.go
- **Fecha**: YYYY-MM-DD
- **Rutas migradas**:
  - `METHOD /v1/path` → `METHOD /v2/path`
- **Archivos creados**:
  - `internal/http/v2/dto/{domain}/{nombre}.go`
  - `internal/http/v2/services/{domain}/{nombre}_service.go`
  - `internal/http/v2/controllers/{domain}/{nombre}_controller.go`
- **Archivos editados**:
  - `internal/http/v2/services/{domain}/services.go`
  - `internal/http/v2/controllers/{domain}/controllers.go`
  - `internal/http/v2/router/{domain}_routes.go`
- **Herramientas V2 usadas**:
  - `store.DataAccessLayer.ForTenant()`
  - `{método específico del DAL}`
- **Dependencias**:
  - DAL (store.Manager)
  - Issuer (jwtx.Issuer)
  - {otras deps}
- **Descripción**:
  {Breve descripción del handler (1-2 líneas)}
- **Notas**:
  - {Edge cases, mejoras vs V1, decisiones de diseño}
- **Wiring verificado**: ✅
  - `app/v2/app.go:{línea}` ({qué se inyectó})
  - `router/router.go:{línea}` ({qué se registró})

---
```

---

## 🔍 Criterios de "Migrado Completo"

Un handler se considera **✅ Migrado** cuando:

1. ✅ **DTOs creados** en `dto/{domain}/`
2. ✅ **Service interface** definida en `services/{domain}/contracts.go`
3. ✅ **Service implementado** en `services/{domain}/{nombre}_service.go`
4. ✅ **Service agregado** a `services/{domain}/services.go`
5. ✅ **Controller creado** en `controllers/{domain}/{nombre}_controller.go`
6. ✅ **Controller agregado** a `controllers/{domain}/controllers.go`
7. ✅ **Rutas registradas** en `router/{domain}_routes.go`
8. ✅ **Wiring verificado** en `app/v2/app.go`
9. ✅ **Herramientas V2** usadas (DAL V2, JWT V2, Email V2, etc)
10. ✅ **Testing manual** con cURL/Postman (al menos 1 caso exitoso)
11. ✅ **Errores mapeados** a HTTP via `httperrors`
12. ✅ **Logging agregado** con `logger.From(ctx)`

---

## 📌 Notas Generales

### Priorización de Handlers

**Alta prioridad** (core auth flows):
1. `auth_login.go`
2. `auth_register.go`
3. `auth_refresh.go`
4. `oauth_token.go`
5. `oauth_authorize.go`

**Media prioridad** (admin + discovery):
1. `admin_clients_fs.go`
2. `admin_tenants_fs.go`
3. `jwks.go`
4. `oidc_discovery.go`

**Baja prioridad** (features avanzadas):
1. `mfa_totp.go`
2. `social_dynamic.go`
3. `email_flows.go`

### Handlers Legacy (Skipear)

Estos handlers NO se migrarán (deprecated o no wired):
- `admin_clients.go` (DB-based, reemplazado por `admin_clients_fs.go`)
- `admin_scopes.go` (DB-based, reemplazado por `admin_scopes_fs.go`)
- `oauth_start.go` (TODO vacío)
- `oauth_callback.go` (TODO vacío)
- `social_google.go` (deprecated, reemplazado por `social_dynamic.go`)
- `public_forms.go` (not wired)
- `registry_clients.go` (not wired)
- `admin_keys.go` (deprecated/empty)

---

## 🚀 Comandos Útiles

```bash
# Contar handlers pendientes
grep -c "\[ \]" MIGRATION_LOG.md

# Contar handlers migrados
grep -c "✅" MIGRATION_LOG.md

# Ver progreso
echo "scale=2; $(grep -c "✅" MIGRATION_LOG.md) / 48 * 100" | bc

# Listar handlers bloqueados
grep "❌" MIGRATION_LOG.md -A 10
```

---

**FIN DEL LOG**
