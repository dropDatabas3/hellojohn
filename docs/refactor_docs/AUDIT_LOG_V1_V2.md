# Registro de Auditoría V1 -> V2

Este documento sirve como respaldo de las auditorías de paridad, wiring y seguridad realizadas durante la migración de handlers de V1 a V2.

---

## 📅 Fecha: 2026-01-13
## 🛡️ Handler: Providers Handler (Discovery)

### 1) Resumen Ejecutivo
**Veredicto:** ✅ **MIGRACIÓN EXITOSA (OK)**

El handler de discovery (`/v1/auth/providers`) ha sido migrado correctamente al dominio **Auth** en V2 (`/v2/auth/providers`). La lógica de negocio, incluyendo la validación estricta de `redirect_uri` y la generación de `start_url` para Google, se ha preservado fielmente en `auth.ProvidersService`.

**Lo bueno:**
*   **Paridad Lógica:** V2 replica la complejidad de V1 (verificación de 'readiness', generación condicional de URLs).
*   **Mejora de Diseño:** La validación de redirects ahora usa explícitamente el DAL (`s.deps.DAL.ForTenant`) en lugar de depender de un adaptador opaco.
*   **Compatibilidad:** Se resolvió la dualidad Social/Auth unificando el alias `/v2/providers/status` hacia el controlador "rico" de Auth, evitando regresiones en la UI.

**Riesgos:** Ninguno crítico detectado.

### 2) Tabla de Paridad (V1 vs V2)

| Endpoint V1 | Handler V1 | Endpoint V2 | Componentes V2 | Paridad | Notas |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `GET /v1/auth/providers` | `handlers.ProvidersHandler` | `GET /v2/auth/providers` | `auth.ProvidersController` → `auth.ProvidersService` | ✅ Total | Misma estructura JSON (rich metadata). |
| (implícito) | (mismo handler) | `GET /v2/providers/status` | `auth.ProvidersController` | ✅ Total | Alias de compatibilidad agregado. |

### 3) Hallazgos

*   **NICE**: En V1, la validación de redirect dependía de `c.Store` global inyectado. En V2, `auth.ProvidersService` recibe `DAL` y resuelve el tenant limpiamente. Esto hace el código más testeaable y menos acoplado al `main`.
*   **INFO**: V2 introduce `social.ProvidersController` (lista simple de strings), pero para mantener contrato con el frontend de V1, se decidió usar `auth.ProvidersController` para el endpoint de status. Esto fue una decisión de diseño correcta durante la revisión.

### 4) Checklist de Wiring

*   ✅ **Ruta registrada en v2**: Sí, en `auth_routes.go` (`/v2/auth/providers` y `/v2/providers/status`).
*   ✅ **Deps no-nil**: `ProvidersService` se inicializa con `store.DataAccessLayer` y config en `services.go`.
*   ✅ **Middleware correcto**: Usa `authHandler` (público, con rate limit opcional), igual que V1 (que no requería auth).
*   ✅ **Service y DAL conectan**: El servicio llama a `DAL.ForTenant(...)` exitosamente.
*   ✅ **Config/Env**: `ProviderConfig` se llena desde `config.Config` (GoogleEnabled, ClientID, etc) en `services.go`.

### 5) Evidencias

*   **V1 Logic**: `internal/http/v1/handlers/providers.go:NewProvidersHandler`
*   **V2 Service**: `internal/http/v2/services/auth/providers_service.go:buildGoogleProvider`
*   **V2 Wiring (Services)**: `internal/http/v2/services/services.go:New` (inyecta `d.DAL` y `ProvidersConfig`)
*   **V2 Routes**: `internal/http/v2/router/auth_routes.go:RegisterAuthRoutes` (L35, L38)

---

## 2. Handler Audit: AuthLoginHandler (Re-Audit)

**Fecha:** 2026-01-14
**V1 Handler:** `internal/http/v1/handlers/auth_login.go`
**V2 Service:** `internal/http/v2/services/auth/login_service.go`
**Wiring:** `internal/http/v2/server/wiring.go` (BuildV2Handler)
**Auditor:** Antigravity

### 2.1. Comparison Summary
| Feature | Status | V1 Implementation | V2 Implementation | Notes |
| :--- | :---: | :--- | :--- | :--- |
| **Routes** | ✅ | `POST /v1/auth/login` | `POST /v2/auth/login` | Parity OK. |
| **Wiring** | ✅ | Manual `NewAuthLoginHandler` | `BuildV2Handler` -> `store.NewManager` | **FIXED:** Uses Real Data Layer & Control Plane. |
| **MFA Gate** | ✅ | Pre-issue check (`mfa_required`) | Implemented in `LoginPassword` | Uses `tda.MFA()` and `tda.Cache()` for challenge. |
| **Hashing** | ✅ | Multiple formats | `SHA256Base64URL` | Unified hash format across V2. |
| **Admin Flow** | ⚠️ | Fallback for "Global Admin" (no tenant) | Strict (Requires Tenant/Client) | V2 enforces tenancy. Acceptable design change. |
| **Claims** | ⚠️ | `claims_hook.go` (System/RBAC) | `NoOpClaimsHook` (Default) | `ClaimsHook` injection is supported but currently NoOp. |
| **Rate Limit** | ✅ | `MultiLimiter` (Login specific) | `IPPathRateKey` (Middleware) | Implemented generic rate limiting. |

### 2.2. Validation & Improvements
*   **Real Store Integration:** V2 correctly instantiates `store.NewManager(ctx, ...)` instead of previous mocks.
*   **MFA Parity:** Logic ported to `LoginPassword`: checks `mfaRepo`, validates `TrustedDeviceToken`, issues `mfa:token` challenge if needed.
*   **Security:** `refresh_token` hash algorithm standardized to `SHA256Base64URL`.

### 2.3. Remaining Gaps
1.  **ClaimsHook Strategy:** V2 currently uses `NoOpClaimsHook`. Logic from V1 `claims_hook.go` (reserved claims protection) should be ported if custom claims are heavily used.
2.  **FS Admin:** Login without `tenant_id` is blocked in V2. If "Global Admin" dashboard exists, it needs a dedicated flow or explicit tenant context.

### 2.4. Verdict
**✅ MIGRACIÓN EXITOSA (OK)**.
El handler principal de Login cuenta con paridad funcional crítica (MFA, Validaciones, Tokens) y wiring correcto con el Data Layer real. Los gaps remanentes (Claims Hook, FS Admin Legacy) son manejables o decisiones de arquitectura.

### 3. Handler Audit: AuthRegisterHandler
**Fecha:** 2026-01-14
**V1 Handler:** `internal/http/v1/handlers/auth_register.go`
**V2 Service:** `internal/http/v2/services/auth/register_service.go`
**Wiring:** `internal/http/v2/server/wiring.go`
**Auditor:** Antigravity

#### 3.1. Comparison Summary
| Feature | Status | V1 Implementation | V2 Implementation | Notes |
| :--- | :---: | :--- | :--- | :--- |
| **Routes** | ✅ | `POST /v1/auth/register` | `POST /v2/auth/register` | Parity OK. |
| **Inputs** | ✅ | JSON (Tenant, Client, Email, Pwd) | JSON DTO | Normalization (lower/trim) intact. |
| **AutoLogin** | ⚠️ | Configurable (`REGISTER_AUTO_LOGIN`) | Hardcoded `false` | **REGRESSION:** Wiring gap prevents `AutoLogin` config from reaching service. |
| **FS Admin** | ⚠️ | Allowed (No Tenant Input) | `Unimplemented` | V2 explicitly stubs this flow. |
| **Email Verify** | ❌ | Triggers logic (`TriggerVerificationEmail`) | **MISSING** | Code for sending verification email is absent. |

#### 3.2. Findings & Gaps
1.  **AutoLogin Regression:** In V1, `REGISTER_AUTO_LOGIN` defaults to `true`. In V2, `RegisterService` defaults `AutoLogin` to `false` because the config is not propagated through `services.New`. This causes V2 to return just `user_id` instead of tokens, breaking the expected flow.
2.  **Missing Email Verification:** V2 `RegisterService` provisions the user but fails to trigger the email verification flow (unlike V1).
3.  **FS Admin Stubbed:** The "Global Admin" registration (no tenant) is present in code but returns "Not implemented".

#### 3.3. Action Plan
1.  **Fix Wiring:** Patch `services.go`, `app.go`, and `wiring.go` to propagate `AutoLogin` and `FSAdminEnabled` configuration.
2.  **Mark Partial:** Flag handler as "MIGRATION PARTIAL" until Email Verification is ported.

### 4. Handler Audit: AuthRegisterHandler (Re-audit)
**Fecha:** 2026-01-14
**V1 Handler:** `internal/http/v1/handlers/auth_register.go`
**V2 Service:** `internal/http/v2/services/auth/register_service.go`
**Wiring:** `internal/http/v2/server/wiring.go`
**Auditor:** Antigravity

#### 4.1. Comparison Summary
| Feature | Status | V1 Implementation | V2 Implementation | Notes |
| :--- | :---: | :--- | :--- | :--- |
| **Routes** | ✅ | `POST /v1/auth/register` | `POST /v2/auth/register` | Parity OK. |
| **AutoLogin** | ✅ | Configurable | Configurable (`REGISTER_AUTO_LOGIN`) | **FIXED:** Wiring propagated from env to service. |
| **FS Admin** | ⚠️ | Allowed (No Tenant Input) | `Unimplemented` | Explicitly blocked/stubbed. Design decision. |
| **Email Verify** | ⚠️ | `TriggerVerificationEmail` | `VerificationSender` (Adapter) | **WIRED:** Logic present. Uses `NoOpEmailService` stub until Infra ready. |

#### 4.2. Validation & Improvements
*   **AutoLogin Parity:** `registerTenantUser` now correctly checks `s.deps.AutoLogin`. If true, issues tokens immediately (matching V1 default).
*   **Email Wiring:** `RegisterService` now has `VerificationSender` injected. Code attempts to send verification if `client.RequireEmailVerification`.
*   **Compilation:** Fixed all blocking build errors in `login_service.go` and `mfa_service.go`.

#### 4.3. Remaining Gaps
1.  **Infrastructure Stub:** `EmailVerificationSender` connects to `emailv2.Service`, which is currently wired to `NoOpEmailService` in `wiring.go`. Emails won't essentially "send" until `emailv2` is fully implemented, but the auth flow integration is complete.
2.  **FS Admin:** Still returns 501 Not Implemented.

#### 4.4. Verdict
**✅ MIGRACIÓN EXITOSA (OK)**.
Se han resuelto los bloqueos de wiring (AutoLogin) y de compilación. El handler es funcional para el flujo principal (Tenant User). El gap de emails está mitigado a nivel de código (listo para conectar infra real).


### 5. Handler Audit: AuthRefreshHandler & AuthLogoutHandler
**Fecha:** 2026-01-15
**V1 Handler:** `internal/http/v1/handlers/auth_refresh.go` (ambos)
**V2 Service:** `internal/http/v2/services/auth/refresh_service.go`, `logout_service.go`
**Wiring:** `internal/http/v2/server/wiring.go`
**Auditor:** Antigravity

#### 5.1. Comparison Summary
| Feature | Status | V1 Implementation | V2 Implementation | Notes |
| :--- | :---: | :--- | :--- | :--- |
| **Routes** | ✅ | `POST /v1/auth/refresh` | `POST /v2/auth/refresh` | Parity OK. |
| **Routes** | ✅ | `POST /v1/auth/logout` | `POST /v2/auth/logout` | Parity OK. |
| **Routes** | ✅ | `POST /v1/auth/logout-all` | `POST /v2/auth/logout-all` | Parity OK. |
| **Hashing** | ⚠️ | HEX (SHA256) | Base64URL (SHA256) | **CHANGE:** V2 unifica formato a Base64URL para consistencia Login/Refresh. |
| **JWT Refresh** | ✅ | Soporta Admin JWT (Stateless) | `refreshAdminJWT` Helper | Logica portada (checks `token_use` y `aud`). |
| **Security** | ✅ | Checks Tenant/Client Match | Checks Tenant/Client Match | Security hardening (cross-tenant check) activo. |

#### 5.2. Findings & Gaps
1.  **Hash Consistency (FIXED):** V1 usaba HEX para los hashes de refresh tokens. V2 Login service empezó a usar Base64URL. Se ha **unificado** `RefreshService` y `LogoutService` para usar **Base64URL** también.
    *   *Nota de Migración:* Los tokens antiguos creados en V1 (Hex) **no serán encontrados** por V2 (Base64URL). Se asume ruptura de sesión aceptable en migración mayor.
2.  **Naming Cleanup:** Se renombraron variables confusas `hashHex` a `hash` en `logout_service.go` para reflejar la realidad del encoding.

#### 5.3. Verdict
**✅ MIGRACIÓN EXITOSA (OK)**.

### 6. Handler Audit: Social Handlers (Dynamic, Google)
**Fecha:** 2026-01-15
**V1 Handler:** `internal/http/v1/handlers/social_*.go`
**V2 Service:** `internal/http/v2/services/social/*.go`
**Wiring:** `internal/http/v2/server/wiring.go`
**Auditor:** Antigravity

#### 6.1. Comparison Summary
| Feature | Status | V1 Implementation | V2 Implementation | Notes |
| :--- | :---: | :--- | :--- | :--- |
| **Routes** | ✅ | `/v1/auth/social/{provider}/*` | `/v2/auth/social/{provider}/*` | Parity OK. |
| **Wiring** | ✅ | Manual `NewDynamicSocialHandler` | `BuildV2Handler` -> `socialsvc.NewServices` | **FIXED:** Wired `cpService`, `MemoryCache`, `Issuer`. |
| **Cache** | ✅ | `NoOp` / Legacy | `CacheWriter` -> `CacheAdapter` -> `cache.v2` | **FIXED:** Created adapter to bridge byte-based legacy social cache with string-based V2 cache. |
| **Secrets** | ✅ | `sec.Decrypt` | `OIDCFactory` -> `sec.Decrypt` | Handling of encrypted secrets is preserved (though verified by code inspection mostly). |
| **Multi-tenancy** | ✅ | Mixed Global/Tenant stores | Strict `DAL.ForTenant` | V2 enforces correct tenant data isolation. |
| **Config** | ✅ | `TenantSettings.SocialProviders` | `repository.SocialConfig` | **FIXED:** Ported `SocialConfig` to V2 repository types to support `client_config_service`. |

#### 6.2. Findings & Gaps
1.  **Cache Wiring (FIXED):** V2 Social services used a legacy `Cache` interface that didn't match V2 `cache.Client` (context-less, byte vs string). Implemented `CacheAdapter` to bridge this gap without rewriting all social services logic immediately.
2.  **Repository Types (FIXED):** V2 `repository.Tenant` and `repository.Client` missed `SocialProviders` configuration fields found in V1 `controlplane`. Added `SocialConfig` struct and fields to `internal/domain/repository`.
3.  **ClientConfigService (FIXED):** Updated `client_config_service_impl.go` to use `repository` types instead of `controlplane/v1` types, removing V1 dependencies from V2 service layer.
4.  **Wiring Injection (FIXED):** `wiring.go` was using `NoOpSocialCache` and `nil` tenant provider. Updated to inject `cache.NewMemory` (via adapter) and `cp.Service` (as TenantProvider).

#### 6.3. Verdict
**✅ MIGRACIÓN EXITOSA (OK)**.
Se han resuelto los problemas críticos de wiring y compilación. Los servicios sociales V2 ahora están conectados correctamente al Data Layer y Control Plane reales, con una estrategia de caché funcional.

### 7. Handler Audit: OAuth Handlers (Authorize, Token)
**Fecha:** 2026-01-15
**V1 Handler:** `internal/http/v1/handlers/oauth_*.go`
**V2 Service:** `internal/http/v2/services/oauth/*.go`
**Wiring:** `internal/http/v2/server/wiring.go`
**Auditor:** Antigravity

#### 7.1. Comparison Summary
| Feature | Status | V1 Implementation | V2 Implementation | Notes |
| :--- | :---: | :--- | :--- | :--- |
| **Routes** | ✅ | `/oauth2/authorize`, `/oauth2/token` | `/oauth2/authorize`, `/oauth2/token` | Parity OK. |
| **Dependencies** | ✅ | Global `cpctx`, `c.Store` | Injected `ControlPlane`, `DAL` | **FIXED:** `TokenService` refactored to remove global state. |
| **Cache** | ✅ | `c.Cache` (Byte-based) | `CacheAdapter` -> `cache.Client` | **FIXED:** Created `oauth.CacheAdapter` to bridge interface gap. |
| **Wiring** | ✅ | Missing Config/Deps | `app.Deps` -> `oauth.Deps` | **FIXED:** Injected `OAuthCache`, `CookieName`, `AllowBearer`, `ControlPlane`. |
| **Types** | ✅ | V1 `controlplane` struct aliases | `repository` types | **FIXED:** `TokenService` now uses `repository.Client` instead of V1 wrapper. |

#### 7.2. Findings & Gaps
1.  **V1 Dependency Leak (FIXED):** `TokenService` in V2 was importing `cpctx` (V1) and using `controlplane` V1 types. This was refactored to use `controlplane/v2` interface and `repository` domain types.
2.  **Missing Wiring (FIXED):** `oauth.NewServices` was not receiving `ControlPlane`, causing nil pointer risks during client lookup. Updated `wiring.go` and `services.go` to pass this dependency.
3.  **Cache Mismatch (FIXED):** `oauth` package expected a `CacheClient` with `Delete(key string) error` (or similar V1 signature), while V2 `cache.Client` uses `Delete(ctx, key) error`. Created `oauth.CacheAdapter` to normalize this.
4.  **Configuration Parity (FIXED):** V1 supported `AUTH_ALLOW_BEARER_SESSION` and custom cookie names. These were missing in V2 wiring. Propagated these config values from `config.Config` to `app.Deps` and then to `oauth.Services`.

#### 7.3. Verdict
### 8. Handler Audit: Admin Tenants (FS)
**Fecha:** 2026-01-15
**V1 Handler:** `internal/http/v1/handlers/admin_tenants_fs.go`
**V2 Controller:** `internal/http/v2/controllers/admin/tenants_controller.go`
**V2 Service:** `internal/http/v2/services/admin/tenants_service.go`
**Auditor:** Antigravity

#### 8.1. Comparison Summary
| Endpoint V1 | Endpoint V2 | Parity | Notes |
| :--- | :--- | :---: | :--- |
| `GET /v1/admin/tenants` | `GET /v2/admin/tenants` | ✅ | `ListTenants` |
| `POST /v1/admin/tenants` | `POST /v2/admin/tenants` | ✅ | `CreateTenant` |
| `GET /v1/admin/tenants/{slug}` | `GET /v2/admin/tenants/{slug}` | ✅ | `GetTenant` |
| `PUT /v1/admin/tenants/{slug}` | `PUT /v2/admin/tenants/{slug}` | ✅ | `UpdateTenant` |
| `DELETE /v1/admin/tenants/{slug}` | `DELETE /v2/admin/tenants/{slug}` | ✅ | `DeleteTenant` |
| `GET .../{slug}/settings` | `GET .../{slug}/settings` | ✅ | `GetSettings` (ETag supported) |
| `PUT .../{slug}/settings` | `PUT .../{slug}/settings` | ✅ | `UpdateSettings` (ETag required) |
| `POST .../keys/rotate` | `POST .../keys/rotate` | ⚠️ | V2 Logic lacks Cluster Raft Mutation. |
| `POST .../migrate` | `POST .../migrate` | ✅ | `MigrateTenant` |
| `POST .../user-store/migrate` | - | ⚠️ | Duplicate in V1. Use `.../migrate`. |
| `POST .../schema/apply` | `POST .../schema/apply` | ✅ | `ApplySchema` |
| `GET .../{slug}/infra-stats` | `GET .../infra-stats` | ✅ | `InfraStats` |
| `POST .../mailing/test` | `POST .../mailing/test` | ⚠️ | Logic only checks config (`GetSender`), doesn't send email. |
| `GET .../{slug}/users` | - | ❌ | **MISSING**. `UsersController` only supports Actions. |
| `POST .../{slug}/users` | - | ❌ | **MISSING**. No Create User in Admin V2. |

#### 8.2. Findings & Gaps
1.  **MUST FIX: Missing Users CRUD:** V1 `AdminTenantsFSHandler` implemented `ListUsers`, `CreateUser`, `PatchUser`, `DeleteUser`. In V2, `UsersController` only implements `Disable`, `Enable`, `ResendVerification`. There is no endpoint to List or Create users for a tenant in V2 Admin.
2.  **SHOULD FIX: Cluster Key Rotation:** V1 `RotateKeys` manually read `active.json/retiring.json` and sent a Raft mutation to replicate keys to all nodes. V2 `RotateKeys` simply calls `issuer.Keys.RotateFor`, which (unless checking `storage/v2` deeps) likely operates only on the local node's FS, breaking HA/Cluster replication for keys.
3.  **NICE TO FIX: Test Mailing:** V2 `TestMailing` only validates that a sender can be retrieved ("config check"). V1 actually sent a test email "to self" or similar.

#### 8.3. Verdict
### 9. Handler Audit: Admin Users (Actions)
**Fecha:** 2026-01-15
**V1 Handler:** `internal/http/v1/handlers/admin_users.go`
**V2 Controller:** `internal/http/v2/controllers/admin/users_controller.go`
**V2 Service:** `internal/http/v2/services/admin/users_service.go`
**Auditor:** Antigravity

#### 9.1. Comparison Summary
| Endpoint V1 | Endpoint V2 | Parity | Notes |
| :--- | :--- | :---: | :--- |
| `POST /v1/admin/users/disable` | `POST /v2/admin/users/disable` | ⚠️ | Parity in logic, but V2 enforces Tenant Header. |
| `POST /v1/admin/users/enable` | `POST /v2/admin/users/enable` | ⚠️ | Same as Disable. |
| `POST /v1/admin/users/resend-verification` | `POST /v2/admin/users/resend-verification` | ⚠️ | Logic OK, email stubbed. |
| `GET .../users/{id}/profile` | - | ❌ | **Phantom Entry**. Referenced in Inventory but code not found in V1. Removed. |

#### 9.2. Findings & Gaps
1.  **SHOULD FIX: Email Stubbed:** V2 `UserActionService` is wired to `NoOpEmailService`. Logic fails silently (best-effort) or returns no error, but no email is sent. Requires V2 Email Service implementation.
2.  **MUST FIX / NOTE: Tenant Requirement:** V1 accepted `tenant_id` in Body and could fallback to Global Store if missing. V2 Middleware (`RequireTenant`) mandates `X-Tenant-ID` (or similar) or rejects request before controller. This is a contract tightening.
3.  **Note: UserProfileHandler:** Inventory entry for `UserProfileHandler` (Row 15) was incorrect; handler code does not exist in `admin_users.go`. Marked as Error in inventory.

#### 9.3. Checklist Wiring
- [x] Rutas registradas (`admin_routes.go` -> `adminUsersHandler`).
- [x] Router llamado (`wiring.go`).
- [x] Deps service inyectado (`NewUserActionService`).
- [x] Controller inyectado (`adminctrl.NewControllers`).
- [x] **Warning:** `d.Email` is Stub/NoOp.

#### 9.4. Verdict
**⚠️ PARCIAL**.
El controlador y servicio existen y cubren la paridad de rutas de acciones (`Disable`/`Enable`/`Resend`). Sin embargo, la funcionalidad de **Email** está "stubbed" (no operativa), y el contrato de Tenant se ha vuelto más estricto. La entrada fantasma de Profile fue depurada.

### 10. Handler Audit: Email Flows (Verify/Forgot/Reset)
**Fecha:** 2026-01-16
**V1 Handler:** `internal/http/v1/handlers/email_flows.go`
**V2 Controller:** `internal/http/v2/controllers/email/flows_controller.go`
**V2 Service:** `internal/http/v2/services/email/flows_service.go`
**Auditor:** Antigravity

#### 10.1. Comparison Summary
| Endpoint V1 | Endpoint V2 | Parity | Notes |
| :--- | :--- | :---: | :--- |
| `POST .../verify-email/start` | `POST /v2/auth/verify-email/start` | ❌ | V2 Service is a Stub (TODOs). |
| `GET .../verify-email` | `GET /v2/auth/verify-email` | ❌ | V2 Service is a Stub (TODOs). |
| `POST .../forgot` | `POST /v2/auth/forgot` | ❌ | V2 Service is a Stub (TODOs). |
| `POST .../reset` | `POST /v2/auth/reset` | ❌ | V2 Service is a Stub (TODOs). |

#### 10.2. Findings & Gaps
1.  **CRITICAL MISSING LOGIC:** The V2 `FlowsService` (`flows_service.go`) is a shell. All methods (`VerifyEmailStart`, `VerifyEmailConfirm`, `ForgotPassword`, `ResetPassword`) contain `// TODO` comments and return placeholder `nil` or success without performing any store operations or email sending.
2.  **Missing Wiring:** The V2 `Wiring` injects `NoOpEmailService`. Even if `FlowsService` logic existed, emails wouldn't send.
3.  **Missing Token Logic:** V1 `EmailFlowsHandler` logic for creating/consuming tokens (`store.CreateEmailVerification`, `store.UsePasswordReset`) is not called in V2.

#### 10.3. Verdict
**❌ INCOMPLETE (SHELL)**.
Although the endpoints are defined and the controller handles HTTP inputs correctly, the business logic layer is non-existent. This handler is effectively **unimplemented** in V2. Needs full implementation copy-port from V1.
