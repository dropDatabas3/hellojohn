# GUIDE V2 — HTTP Router & Handlers (effective routes)

Este documento lista las rutas HTTP *efectivas* del service v1 y cómo mapearlas a v2.

Fuente de verdad:
- `internal/http/v1/routes.go` (la mayoría de rutas)
- `cmd/service/v1/main.go` (rutas extra registradas en main)

## 1) Problema actual (v1)

- Router frágil: `internal/http/v1/routes.go` expone `NewMux(...)` con muchos handlers **posicionales**.
- Rutas incompletas si sólo mirás `routes.go`: hay rutas registradas en `main.go`.

## 2) Rutas en `internal/http/v1/routes.go`

### Health
- `GET /healthz`
- `GET /readyz`

### JWKS
- `GET /.well-known/jwks.json` (global)
- `GET /.well-known/jwks/{slug}.json` (por-tenant; implementado como `/.well-known/jwks/` y el handler espera el sufijo)

### OIDC discovery
- `GET /.well-known/openid-configuration` (global)

### OAuth2 / OIDC
- `GET /oauth2/authorize`
- `POST /oauth2/token`
- `POST /oauth2/revoke`
- `POST /oauth2/introspect`
- `GET /userinfo`

### Auth API
- `POST /v1/auth/login`
- `POST /v1/auth/register`
- `POST /v1/auth/refresh`
- `POST /v1/auth/logout`
- `GET  /v1/me`
- `POST /v1/auth/logout-all`

### Cookie session (para /oauth2/authorize)
- `POST /v1/session/login`
- `POST /v1/session/logout`

### Consent
- `POST /v1/auth/consent/accept`

### Email flows
- `POST /v1/auth/verify-email/start`
- `GET  /v1/auth/verify-email`
- `POST /v1/auth/forgot`
- `POST /v1/auth/reset`

### Branding/Public config
- `GET /v1/auth/config`

### CSRF
- `GET /v1/csrf`

### MFA (TOTP)
- `POST /v1/mfa/totp/enroll`
- `POST /v1/mfa/totp/verify`
- `POST /v1/mfa/totp/challenge`
- `POST /v1/mfa/totp/disable`
- `POST /v1/mfa/recovery/rotate`

### Social
- `POST /v1/auth/social/exchange`
- `ANY  /v1/auth/social/` (subrutas manejadas por el handler dinámico)

### Admin
- Scopes
  - `GET/POST/DELETE /v1/admin/scopes` + `.../scopes/{id}`
- Consents
  - `GET/DELETE /v1/admin/consents` + variantes `.../consents/by-user`
- Clients
  - `GET/POST/PUT/DELETE /v1/admin/clients` + `.../clients/{id}`
- RBAC
  - `GET/POST /v1/admin/rbac/users/{userID}/roles`
  - `GET/POST /v1/admin/rbac/roles/{role}/perms`
- Users
  - `POST /v1/admin/users/disable`
  - `POST /v1/admin/users/enable`
  - `POST /v1/admin/users/resend-verification`
- Tenants
  - `GET/POST /v1/admin/tenants`
  - `GET/PUT /v1/admin/tenants/{slug}/settings`

### Demo protected resource
- `GET /v1/profile` (requiere `RequireAuth` + scope `profile:read`)

### Debug / Dev
- `GET /v1/auth/social/debug/code` (sólo si `SOCIAL_DEBUG_LOG=1`)
- `GET|POST /__dev/shutdown` (sólo si `ALLOW_DEV_SHUTDOWN=1`)

### Assets
- `GET /v1/assets/*` (FileServer bajo `./data/hellojohn`, con allowlist de extensiones de imagen)

## 3) Rutas registradas en `cmd/service/v1/main.go` (fuera de routes.go)

- `GET /metrics` (Prometheus)
- `GET /t/{tenant}/.well-known/openid-configuration` (per-tenant discovery)
  - En v1 se registra como `mux.Handle("/t/", tenantOIDCDiscoveryHandler)` y el handler valida sufijos.
- `GET /v1/auth/providers` (estado/URLs de providers)
- `GET /v1/providers/status` (back-compat)
- `POST /v1/auth/complete-profile` (custom fields post-social)
- `GET /v1/auth/social/result` (condicional: `cfg.Providers.Google.Enabled`)

## 4) Mapeo handler → archivo (v1)

Rutas wired (seguro) por estar en main/routes:

- JWKS: `internal/http/v1/handlers/jwks.go`
- OIDC discovery (global + per-tenant): `internal/http/v1/handlers/oidc_discovery.go`
- OAuth authorize/token/revoke/introspect: `internal/http/v1/handlers/oauth_authorize.go`, `oauth_token.go`, `oauth_revoke.go`, `oauth_introspect.go`
- UserInfo: `internal/http/v1/handlers/userinfo.go`
- Auth login/register/refresh/logout/logout-all: `internal/http/v1/handlers/auth_login.go`, `auth_register.go`, `auth_refresh.go`, `auth_logout_all.go` (y logout en su archivo)
- Me: `internal/http/v1/handlers/me.go`
- Session login/logout: `internal/http/v1/handlers/session_login.go`, `session_logout.go`
- Consent accept: `internal/http/v1/handlers/oauth_consent.go` (el accept handler vive acá en v1)
- Email flows: `internal/http/v1/handlers/email_flows*.go`
- Auth config: `internal/http/v1/handlers/auth_config.go`
- CSRF: `internal/http/v1/handlers/csrf.go`
- MFA: `internal/http/v1/handlers/mfa_totp.go`
- Social exchange/dynamic/result: `internal/http/v1/handlers/social_exchange.go`, `social_dynamic.go`, `social_result.go`
- Providers + complete profile: `internal/http/v1/handlers/providers.go`, `auth_complete_profile.go`
- Admin: `internal/http/v1/handlers/admin_scopes_fs.go`, `admin_consents.go`, `admin_clients_fs.go`, `admin_rbac.go`, `admin_users.go`, `admin_tenants_fs.go`
- Profile demo resource: `internal/http/v1/handlers/profile.go`
- Readyz: `internal/http/v1/handlers/readyz.go`

Handlers presentes pero **no aparecen registrados** en el wiring actual (candidatos a legacy/no usados o rutas futuras):
- `oauth_start.go`, `oauth_callback.go`
- `social_google.go` (comentado como DEPRECATED en main)
- `admin_keys.go`, `admin_mailing.go`, `admin_clients.go`, `admin_scopes.go`
- `public_forms.go`, `registry_clients.go`

Nota: para confirmar “no wired” en cada caso, buscá sus `New*Handler` en `cmd/service/v1/main.go` y `internal/http/v1/routes.go`.

## 6) Inventario completo de `internal/http/v1/handlers/*`

Leyenda:
- **Wired**: se registra en `internal/http/v1/routes.go` y/o `cmd/service/v1/main.go`.
- **Helper**: utilidades/types usados por otros handlers.
- **No wired/legacy**: existe el archivo pero no aparece en el wiring actual (puede ser legacy, futuro, o invocado indirectamente).

- Wired: `admin_clients_fs.go` — Admin CRUD de clients contra control-plane FS.
- No wired/legacy: `admin_clients.go` — variante no-FS (revisar si quedó legacy).
- Wired: `admin_consents.go` — Admin consents (list/delete, by-user).
- No wired/legacy: `admin_keys.go` — Admin de keys (no registrado en router v1).
- No wired/legacy: `admin_mailing.go` — Admin mailing/templates (no registrado en router v1).
- Wired: `admin_rbac.go` — RBAC roles/perms + asignación de roles a users (subrutas bajo `/v1/admin/rbac/...`).
- Wired: `admin_scopes_fs.go` — Admin scopes contra control-plane FS (con `RequireLeader`).
- No wired/legacy: `admin_scopes.go` — variante no-FS (revisar si quedó legacy).
- Wired: `admin_tenants_fs.go` — Admin tenants + settings contra control-plane FS (con `RequireLeader`).
- Wired: `admin_users.go` — Admin enable/disable/resend-verification.

- Wired: `auth_complete_profile.go` — POST complete-profile (custom fields post-social).
- Wired: `auth_config.go` — GET auth branding/config pública.
- Wired: `auth_login.go` — login (tokens/refresh).
- Wired: `auth_logout_all.go` — logout-all.
- Wired: `auth_refresh.go` — refresh.
- Wired: `auth_register.go` — register (+ opcional auto-login).

- Helper/optional: `claims_hook.go` — hook/extensión de claims (se inyecta vía container si se usa).
- Helper: `cookieutil.go` — helpers de cookies/session.
- Wired: `csrf.go` — endpoint emisor de CSRF token + helpers.

- Wired (si `hasGlobalDB`): `email_flows.go` — verify/reset/forgot flows.
- Helper: `email_flows_wiring.go` — wiring/builders de email flows.
- Helper: `email_flows_wrappers.go` — wrappers/utilidades para email flows.

- Helper: `json.go` — helpers JSON/errores.
- Wired: `jwks.go` — JWKS global + per-tenant.
- Wired: `me.go` — /v1/me.

- Wired: `mfa_totp.go` — endpoints MFA TOTP + recovery rotate.
- Helper: `mfa_types.go` — types/modelos MFA.

- Wired: `oauth_authorize.go` — /oauth2/authorize.
- No wired/legacy: `oauth_callback.go` — callback legacy (no registrado en router v1).
- Wired: `oauth_consent.go` — consent accept.
- Wired: `oauth_introspect.go` — /oauth2/introspect.
- Wired: `oauth_revoke.go` — /oauth2/revoke.
- No wired/legacy: `oauth_start.go` — start legacy (no registrado en router v1).
- Wired: `oauth_token.go` — /oauth2/token.

- Wired: `oidc_discovery.go` — discovery global + per-tenant bajo `/t/` (el registro per-tenant se hace en main).

- Wired: `profile.go` — demo resource protegido por scope.
- Wired: `providers.go` — providers status (`/v1/auth/providers`, `/v1/providers/status`).
- No wired/legacy: `public_forms.go` — forms públicos (no registrado en router v1).
- Wired: `readyz.go` — /readyz.
- No wired/legacy: `registry_clients.go` — registry de clients (no registrado en router v1).

- Wired: `session_login.go` — cookie session login.
- Wired: `session_logout.go` — cookie session logout.
- Helper: `session_logout_util.go` — utilidades logout.

- Wired: `social_dynamic.go` — handler dinámico bajo `/v1/auth/social/`.
- Wired: `social_exchange.go` — exchange endpoint.
- No wired/legacy: `social_google.go` — deprecated (en main aparece comentado como reemplazado por dynamic).
- Wired (condicional): `social_result.go` — `/v1/auth/social/result` (solo si Google enabled).

- Wired: `userinfo.go` — /userinfo.

## 5) Recomendación v2: router por módulos

En vez de `NewMux(h1,h2,...)`:

- `internal/http/v2/router`
  - `RegisterHealth(mux, deps)`
  - `RegisterOIDC(mux, deps)`
  - `RegisterOAuth(mux, deps)`
  - `RegisterAuth(mux, deps)`
  - `RegisterMFA(mux, deps)`
  - `RegisterAdmin(mux, deps)`
  - `RegisterAssets(mux, deps)`

Cada módulo recibe un `Deps` (container + config + capabilities) y registra rutas explícitas.
