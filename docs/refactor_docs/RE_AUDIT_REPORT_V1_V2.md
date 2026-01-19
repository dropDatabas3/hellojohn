# RE-AUDIT REPORT — V1 ↔ V2

Fecha: 2026-01-16

---

## 1) Estado general

- **V2 runtime:** ✅ **Vivo**
  - Entry point: `cmd/service_v2/main.go` llama a `BuildV2Handler()` y levanta server HTTP separado en `:8082`.
  - Wiring: `internal/http/v2/server/wiring.go` → `BuildV2Handler()` instancia: `store.NewManager`, `cp.NewService`, `emailv2.NewService`, `jwtx.NewIssuer`.
  - Router: `internal/http/v2/router/router.go` → `RegisterV2Routes()` llama a 10 sub-registradores (Auth, Admin, OAuth, OIDC, Social, Session, Email, Security, Health, MFA).
  
- **% Paridad estimada (solo de los auditados):** ~85%
  - 7/10 handlers ✅ CLOSED
  - 2/10 handlers ⚠️ PARTIAL
  - 1/10 handler marcado INCOMPLETE → ahora **FIXED** (Email Flows)

- **Top 5 Riesgos:**
  1. **Email Service Stub:** `wiring.go` puede crear `emailv2.NewService` pero el SMTP sender real depende de config por tenant. Si falta config, usa internamente `NoOpSender`.
  2. **Admin Users CRUD Missing:** V2 solo tiene Actions (`Disable`, `Enable`, `Resend`). Falta `ListUsers`, `CreateUser`, `UpdateUser`, `DeleteUser` en rutas Admin.
  3. **Cluster Key Rotation:** V2 `RotateKeys` no replica keys vía Raft. Problema en HA.
  4. **ClaimsHook NoOp:** V2 usa `NoOpClaimsHook`. Si V1 tenía custom claims, no se portan.
  5. **Hash Migration:** V2 usa Base64URL para hashes de refresh tokens. Tokens antiguos (Hex) serán invalidados.

---

## 2) Tabla resumen (solo lo auditado)

| Área | Handler/Feature | V1 Endpoints | V2 Endpoints | Estado | Gaps abiertos | Prioridad |
| :--- | :--- | :--- | :--- | :---: | :--- | :---: |
| Discovery | ProvidersHandler | `/v1/auth/providers` | `/v2/auth/providers`, `/v2/providers/status` | ✅ | Ninguno | - |
| Auth | LoginHandler | `POST /v1/auth/login` | `POST /v2/auth/login` | ✅ | ClaimsHook NoOp | P2 |
| Auth | RegisterHandler | `POST /v1/auth/register` | `POST /v2/auth/register` | ✅ | FS Admin stubbed | P2 |
| Auth | Refresh/Logout | `/v1/auth/refresh`, `/logout`, `/logout-all` | `/v2/auth/refresh`, `/logout`, `/logout-all` | ✅ | Hash migration | P2 |
| Social | DynamicSocial, Google, Exchange, Result | `/v1/auth/social/*` | `/v2/auth/social/*` | ✅ | Ninguno | - |
| OAuth | Authorize, Token, Consent | `/oauth2/authorize`, `/oauth2/token`, `/v1/auth/consent/accept` | `/oauth2/authorize`, `/oauth2/token`, `/v2/auth/consent/accept` | ✅ | Ninguno | - |
| Admin | Tenants FS | `/v1/admin/tenants/*` | `/v2/admin/tenants/*` | ⚠️ | Users CRUD, Cluster Key Rotation | P1 |
| Admin | Users Actions | `/v1/admin/users/disable`, `/enable`, `/resend-verification` | `/v2/admin/users/disable`, `/enable`, `/resend-verification` | ⚠️ | Email stub | P1 |
| Email | Flows | `/v1/auth/verify-email/*`, `/forgot`, `/reset` | `/v2/auth/verify-email/*`, `/forgot`, `/reset` | ✅ | Infraestructura email (tenant SMTP) | P1 |
| MFA | TOTP | `/v1/mfa/totp/*` | `/v2/mfa/totp/*` | ✅ | Ninguno | - |

---

## 3) Detalle por ítem

### 3.1 Providers/Discovery
- **Veredicto actual:** ✅ CLOSED
- **Evidencia:**
  - V2 Service: `internal/http/v2/services/auth/providers_service.go`
  - V2 Route: `internal/http/v2/router/auth_routes.go` (L35, L38)
- **Paridad:** Total (same JSON structure, redirect_uri validation).
- **Wiring:** ✅ Registrado, deps inyectadas (DAL, ProviderConfig).
- **Seguridad:** Rate limit via middleware chain. No secrets exposed.
- **Gaps:** Ninguno.

### 3.2 Auth Login
- **Veredicto actual:** ✅ CLOSED
- **Evidencia:**
  - V2 Service: `internal/http/v2/services/auth/login_service.go`
  - V2 Route: `internal/http/v2/router/auth_routes.go`
- **Paridad:** MFA gate, password check, token issuance, tenant/client validation.
- **Wiring:** ✅ Real DAL, Real Issuer, Real ControlPlane.
- **Seguridad:** Rate limit, body limit (32KB), No-Store headers.
- **Gaps:** ClaimsHook es NoOp. FS Admin login blocked (design decision).

### 3.3 Auth Register
- **Veredicto actual:** ✅ CLOSED (was PARTIAL, now FIXED)
- **Evidencia:**
  - V2 Service: `internal/http/v2/services/auth/register_service.go`
- **Paridad:** AutoLogin configurable, email verification trigger.
- **Wiring:** ✅ AutoLogin env propagated, VerificationSender wired.
- **Seguridad:** Body limit, cross-tenant check.
- **Gaps:** FS Admin registration stubbed.

### 3.4 Auth Refresh/Logout
- **Veredicto actual:** ✅ CLOSED
- **Evidencia:**
  - V2 Services: `refresh_service.go`, `logout_service.go`
- **Paridad:** Rotation logic, Admin JWT refresh, logout-all.
- **Wiring:** ✅ TokenRepo, Issuer.
- **Seguridad:** Hash consistency (Base64URL), No-Store.
- **Gaps:** Migration breaks old HEX tokens.

### 3.5 Social Handlers
- **Veredicto actual:** ✅ CLOSED
- **Evidencia:**
  - V2 Services: `internal/http/v2/services/social/*`
  - V2 Routes: `internal/http/v2/router/social_routes.go`
- **Paridad:** Start/Callback, Exchange, Result endpoints.
- **Wiring:** ✅ CacheAdapter, TenantProvider (cpService), encrypted secrets.
- **Seguridad:** Redirect validation, state JWT.
- **Gaps:** Ninguno.

### 3.6 OAuth (Authorize, Token, Consent)
- **Veredicto actual:** ✅ CLOSED
- **Evidencia:**
  - V2 Services: `internal/http/v2/services/oauth/*`
  - V2 Routes: `internal/http/v2/router/oauth_routes.go`
- **Paridad:** PKCE, AuthCode, Refresh, ClientCreds.
- **Wiring:** ✅ OAuthCache, ControlPlane, DAL.
- **Seguridad:** Rate limit, body limit.
- **Gaps:** Ninguno.

### 3.7 Admin Tenants FS
- **Veredicto actual:** ⚠️ PARTIAL
- **Evidencia:**
  - V2 Controller: `internal/http/v2/controllers/admin/tenants_controller.go`
  - V2 Service: `internal/http/v2/services/admin/tenants_service.go`
  - V2 Routes: `internal/http/v2/router/admin_routes.go`, `tenants_routes.go`
- **Paridad:**
  - ✅ List, Create, Get, Update, Delete Tenant
  - ✅ Get/Update Settings (ETag)
  - ✅ Migrate, ApplySchema, InfraStats
  - ❌ Users CRUD (List, Create, Update, Delete) — MISSING
  - ⚠️ Keys Rotate (no Cluster replication)
  - ⚠️ Test Mailing (config check only, no actual send)
- **Wiring:** ✅ Para lo existente. Falta UsersController CRUD.
- **Seguridad:** ETag for optimistic locking.
- **Gaps:**
  1. `GET/POST /v2/admin/tenants/{slug}/users` — No existe
  2. `PATCH/DELETE /v2/admin/tenants/{slug}/users/{id}` — No existe
  3. `POST .../keys/rotate` — No replica a cluster
  4. `POST .../mailing/test` — Solo valida config, no envía email

### 3.8 Admin Users Actions
- **Veredicto actual:** ⚠️ PARTIAL
- **Evidencia:**
  - V2 Controller: `internal/http/v2/controllers/admin/users_controller.go`
  - V2 Service: `internal/http/v2/services/admin/users_service.go`
  - V2 Routes: `internal/http/v2/router/admin_routes.go`
- **Paridad:** Disable, Enable, Resend-Verification.
- **Wiring:** ✅
- **Seguridad:** Requires Tenant, AuthMiddleware.
- **Gaps:**
  1. Email Service es stub (`NoOpEmailService` si no hay SMTP config). `ResendVerification` loguea warn y no envía.
  2. Tenant Header estrictamente requerido (V1 aceptaba body).

### 3.9 Email Flows (Verify/Forgot/Reset)
- **Veredicto actual:** ✅ CLOSED (was INCOMPLETE/SHELL)
- **Evidencia:**
  - V2 Service: `internal/http/v2/services/email/flows_service.go` (467 lines, FULLY IMPLEMENTED)
  - V2 Controller: `internal/http/v2/controllers/email/flows_controller.go`
  - V2 Routes: `internal/http/v2/router/email_routes.go`
- **Cambios vs audit anterior:** **FIXED**. El service ya NO es un stub. Implementa:
  - Token generation (raw + SHA256 hash)
  - Token persistence via `repository.EmailTokenRepository`
  - Email sending via `emailv2.Service`
  - Redirect validation via ControlPlane
  - Anti-enumeration (soft-fail)
  - Tenant consistency (`tda` as source of truth)
  - Open redirect prevention (no blind redirect)
- **Wiring:** ✅ `emailv2.NewService` con SMTP config loading.
- **Seguridad:**
  - Token type enforcement (`EmailTokenVerification` vs `EmailTokenPasswordReset`)
  - Body limit (32KB)
  - Rate limit
  - Redirect URI validation
  - Tenant mismatch returns 400
- **Gaps:**
  1. Infraestructura: Si el tenant no tiene SMTP configurado, email send falla silenciosamente (soft-fail).

### 3.10 MFA TOTP
- **Veredicto actual:** ✅ CLOSED
- **Evidencia:**
  - V2 Controller: `internal/http/v2/controllers/auth/mfa_controller.go`
  - V2 Service: `internal/http/v2/services/auth/mfa_service.go`
  - V2 Routes: `internal/http/v2/router/mfa_routes.go`
- **Paridad:** 5/5 endpoints (Enroll, Verify, Challenge, Disable, Recovery Rotate).
- **Wiring:** ✅
- **Seguridad:** MFA token via Cache, trusted device, master key encryption.
- **Gaps:** Ninguno.

---

## 4) Rutas V2: Implementadas vs Registradas

### 4.1 Archivos de rutas existentes
| Archivo | Llamado por `router.go` | Estado |
| :--- | :---: | :--- |
| `admin_routes.go` | ✅ | Registra Admin routes |
| `auth_routes.go` | ✅ | Registra Auth routes |
| `email_routes.go` | ✅ | Registra Email flows routes |
| `health_routes.go` | ✅ | Registra `/readyz`, `/livez` |
| `mfa_routes.go` | ✅ | Registra MFA routes |
| `oauth_routes.go` | ✅ | Registra OAuth routes |
| `oidc_routes.go` | ✅ | Registra OIDC routes (Discovery, JWKS, UserInfo) |
| `security_routes.go` | ✅ | Registra CSRF |
| `session_routes.go` | ✅ | Registra Session routes |
| `social_routes.go` | ✅ | Registra Social routes |
| `tenants_routes.go` | ❌ | **NOT CALLED** by `router.go`. May be dead/legacy. |
| `assets_routes.go` | ❌ | Empty (16 bytes) |
| `csrf_routes.go` | ❌ | Not called directly (security_routes handles CSRF) |
| `dev_routes.go` | ❌ | Empty (55 bytes) |
| `public_routes.go` | ❌ | Empty (16 bytes) |
| `user_routes.go` | ❌ | Empty (108 bytes) |
| `users_routes.go` | ❌ | Empty (16 bytes) |

### 4.2 Endpoints implementados pero no expuestos
- `tenants_routes.go` contiene rutas pero **no es llamado** desde `router.go`. Posible dead code o rutas duplicadas con `admin_routes.go`.

### 4.3 Endpoints expuestos con lógica stub/NoOp
- **Email Sending:** Si no hay SMTP config, `emailv2.NewService` usa sender interno que puede fallar. Logging OK, pero no email real.
- **Test Mailing:** `POST /v2/admin/tenants/{slug}/mailing/test` solo valida GetSender(), no envía email.

---

## 5) Plan de cierre (acciones)

### P0 — Bloqueantes
- [x] ~~Email Flows service implementado~~ **(DONE)** — Verificado que `flows_service.go` tiene lógica completa.
- [ ] **Verificar `tenants_routes.go`**: Determinar si está duplicado con `admin_routes.go` o si debe integrarse. Si está duplicado, eliminar.

### P1 — Importantes
- [ ] **Implementar Users CRUD en Admin:**
  - Archivos: `internal/http/v2/controllers/admin/users_controller.go`, `internal/http/v2/services/admin/users_service.go`
  - Endpoints: `GET/POST /v2/admin/tenants/{slug}/users`, `PATCH/DELETE .../users/{id}`
- [ ] **Cluster Key Rotation:**
  - Archivo: `internal/http/v2/services/admin/tenants_service.go` → `RotateKeys`
  - Acción: Agregar lógica para propagar keys via Raft/Cluster.
- [ ] **Test Mailing Real Send:**
  - Archivo: `internal/http/v2/services/admin/tenants_service.go` → `TestMailing`
  - Acción: Implementar envío de email de prueba real (no solo config check).
- [ ] **Email Infra Fallback:**
  - Documentar que si tenant no tiene SMTP, emails no se envían.
  - Considerar fallback a SMTP global si existe.

### P2 — Nice to have
- [ ] **Port ClaimsHook from V1:**
  - Archivo: `internal/http/v2/services/auth/login_service.go`
  - Acción: Reemplazar `NoOpClaimsHook` con lógica real de `claims_hook.go`.
- [ ] **FS Admin Registration:**
  - Archivo: `internal/http/v2/services/auth/register_service.go`
  - Acción: Implementar `registerFSAdmin` si se necesita dashboard global.
- [ ] **Hash Migration Documentation:**
  - Documentar que tokens V1 (HEX) serán invalidados. Usuarios deberán re-login.
- [ ] **Limpiar archivos vacíos de router:**
  - Eliminar: `assets_routes.go`, `dev_routes.go`, `public_routes.go`, `user_routes.go`, `users_routes.go`, `csrf_routes.go` (si no usados).

---

## Cierre

### Lista de checks rápidos para el próximo sprint
1. [ ] Confirmar todos los endpoints V2 auditados responden correctamente (smoke test).
2. [ ] Verificar que `tenants_routes.go` no introduce rutas duplicadas.
3. [ ] Probar flujo de email con tenant con SMTP configurado.
4. [ ] Validar MFA challenge flow end-to-end.
5. [ ] Revisar logs de producción para errores de wiring (`tenant_load_error`, `email tokens repo not wired`).

### Qué 10 handlers conviene auditar/migrar después (orden sugerido)
1. `OIDCDiscoveryHandler` / `TenantOIDCDiscoveryHandler` (si no cubierto por oidc_routes)
2. `JWKSHandler` (verificar cache y tenant resolution)
3. `UserInfoHandler` (OIDC UserInfo)
4. `OAuthIntrospectHandler` / `OAuthRevokeHandler`
5. `CompleteProfileHandler`
6. `MeHandler` / `ProfileHandler`
7. `AdminClientsHandler` (FS based)
8. `AdminConsentsHandler`
9. `AdminRBACUsersRolesHandler` / `AdminRBACRolePermsHandler`
10. `SessionLoginHandler` / `SessionLogoutHandler`

---

*Generado automáticamente por Antigravity RE-AUDIT. Revisar y validar manualmente.*
