# Master List de Rutas HTTP V1 (Inventario Completo)

Este documento consolida TODAS las rutas activas encontradas en `main.go`, `routes.go` y handlers con sub-routing interno (switch/case) para el servicio `hellojohn`.

## 1. System / Infra
| Método | Ruta | Handler | Fuente | Descripción |
| :--- | :--- | :--- | :--- | :--- |
| **GET** | `/readyz` | `handlers.Readyz` | readyz.go | Health Check (Control Plane, DB, Redis) |
| **GET** | `/livez` | `handlers.Livez` | main.go | Liveness Probe |
| **GET** | `/version` | `handlers.Version` | main.go | Build info |
| **GET** | `/metrics` | `promhttp.Handler` | main.go | Prometheus key metrics |
| **GET** | `/v1/public/tenants/{slug}/forms/{type}` | `handlers.PublicForms` | public_forms.go | Config de forms Login/Register (CP) |
| **GET** | `/__dev/shutdown` | (func) | routes.go | Graceful shutdown (solo dev, ENV `ALLOW_DEV_SHUTDOWN=1`) |
| **GET** | `/v1/assets/*` | `http.FileServer` | routes.go | Servidor de assets estáticos (solo imágenes) |

## 2. OIDC & Discovery
| Método | Ruta | Handler | Fuente | Descripción |
| :--- | :--- | :--- | :--- | :--- |
| **GET** | `/.well-known/jwks.json` | `handlers.JWKS.GetGlobal` | jwks.go | JWKS global (soporta HEAD) |
| **GET** | `/.well-known/jwks/{slug}.json` | `handlers.JWKS.GetByTenant` | jwks.go | JWKS por tenant (soporta HEAD) |
| **GET, HEAD** | `/.well-known/openid-configuration` | `handlers.OIDCDiscovery` | oidc_discovery.go | Discovery global |
| **GET, HEAD** | `/t/{slug}/.well-known/openid-configuration` | `handlers.TenantOIDCDiscovery` | oidc_discovery.go | Discovery por tenant |

## 3. Auth & Session (Public)
| Método | Ruta | Handler | Fuente | Descripción |
| :--- | :--- | :--- | :--- | :--- |
| **GET** | `/oauth2/authorize` | `handlers.OAuthAuthorize` | oauth_authorize.go | OIDC Authorize endpoint |
| **POST** | `/oauth2/token` | `handlers.OAuthToken` | oauth_token.go | OIDC Token exchange (Code, Refresh, ClientCreds) |
| **POST** | `/oauth2/revoke` | `handlers.OAuthRevoke` | oauth_revoke.go | Token revocation (RFC 7009) |
| **POST** | `/oauth2/introspect` | `handlers.OAuthIntrospect` | oauth_introspect.go | Token introspection (RFC 7662) |
| **GET, POST** | `/userinfo` | `handlers.UserInfo` | userinfo.go | OIDC UserInfo |
| **POST** | `/v1/auth/login` | `handlers.AuthLogin` | auth_login.go | Login API (JWT) |
| **POST** | `/v1/auth/register` | `handlers.AuthRegister` | auth_register.go | Registro de usuario |
| **POST** | `/v1/auth/refresh` | `handlers.AuthRefresh` | auth_refresh.go | Refresh token rotation |
| **POST** | `/v1/auth/logout` | `handlers.AuthLogout` | auth_refresh.go | Logout simple |
| **POST** | `/v1/auth/logout-all` | `handlers.AuthLogoutAll` | auth_logout_all.go | Revocar todas las sesiones |
| **GET** | `/v1/me` | `handlers.Me` | me.go | Perfil del usuario actual (Claims) |
| **POST** | `/v1/session/login` | `handlers.SessionLogin` | session_login.go | Cookie-based login (para OIDC flow) |
| **POST** | `/v1/session/logout` | `handlers.SessionLogout` | session_logout.go | Cookie logout (with return_to) |
| **POST** | `/v1/auth/consent/accept` | `handlers.ConsentAccept` | oauth_consent.go | Aceptar consent (SPA) |

## 4. Email Flows & Password
| Método | Ruta | Handler | Fuente | Descripción |
| :--- | :--- | :--- | :--- | :--- |
| **POST** | `/v1/auth/verify-email/start` | `handlers.VerifyEmailStart` | email_flows.go | Iniciar verificación email |
| **GET** | `/v1/auth/verify-email` | `handlers.VerifyEmailConfirm` | email_flows.go | Confirmar email (link click) |
| **POST** | `/v1/auth/forgot` | `handlers.Forgot` | email_flows.go | Solicitar password reset |
| **POST** | `/v1/auth/reset` | `handlers.Reset` | email_flows.go | Ejecutar password reset |

## 5. MFA (TOTP)
| Método | Ruta | Handler | Fuente | Descripción |
| :--- | :--- | :--- | :--- | :--- |
| **POST** | `/v1/mfa/totp/enroll` | `handlers.MFAEnroll` | mfa_totp.go | Iniciar enrolamiento TOTP |
| **POST** | `/v1/mfa/totp/verify` | `handlers.MFAVerify` | mfa_totp.go | Verificar y activar TOTP |
| **POST** | `/v1/mfa/totp/challenge` | `handlers.MFAChallenge` | mfa_totp.go | Validar TOTP durante login |
| **POST** | `/v1/mfa/totp/disable` | `handlers.MFADisable` | mfa_totp.go | Desactivar MFA |
| **POST** | `/v1/mfa/recovery/rotate` | `handlers.MFARecoveryRotate` | mfa_totp.go | Rotar recovery codes |

## 6. Social Login
| Método | Ruta | Handler | Fuente | Descripción |
| :--- | :--- | :--- | :--- | :--- |
| **POST** | `/v1/auth/social/exchange` | `handlers.SocialExchange` | routes.go | Exchange manual (mobile/SPA) |
| **GET** | `/v1/auth/social/{provider}/start` | `handlers.DynamicSocial` | routes.go | Iniciar flujo social (redirect) |
| **GET** | `/v1/auth/social/{provider}/callback` | `handlers.DynamicSocial` | routes.go | Callback social provider |
| **GET** | `/v1/auth/social/result` | `handlers.SocialResult` | main.go | UI resultado final login (si Google legacy on) |
| **POST** | `/v1/auth/complete-profile` | `handlers.AuthCompleteProfile` | main.go | Completar perfil post-social |

## 7. Admin Core
| Método | Ruta | Handler | Fuente | Descripción |
| :--- | :--- | :--- | :--- | :--- |
| **GET** | `/v1/admin/scopes` | `handlers.AdminScopesFS` | admin_scopes_fs.go | Listar scopes (usa Header/Query tenant) |
| **POST** | `/v1/admin/scopes` | `handlers.AdminScopesFS` | admin_scopes_fs.go | Upsert scope (Create/Update por nombre) |
| **PUT** | `/v1/admin/scopes` | `handlers.AdminScopesFS` | admin_scopes_fs.go | Upsert scope (Alias de POST) |
| **DELETE** | `/v1/admin/scopes/{name}` | `handlers.AdminScopesFS` | admin_scopes_fs.go | Borrar scope por nombre |
| **GET** | `/v1/admin/consents` | `handlers.AdminConsents` | admin_consents.go | Listar consents (filters: user_id, client_id) |
| **GET** | `/v1/admin/consents/by-user/{uid}` | `handlers.AdminConsents` | admin_consents.go | Listar consents por usuario |
| **POST** | `/v1/admin/consents/upsert` | `handlers.AdminConsents` | admin_consents.go | Upsert consent (Force scopes) |
| **POST** | `/v1/admin/consents/revoke` | `handlers.AdminConsents` | admin_consents.go | Revoke consent (POST) + best-effort token revoke |
| **DELETE** | `/v1/admin/consents/{uid}/{cid}` | `handlers.AdminConsents` | admin_consents.go | Revoke consent (Delete) |

*Nota: Existe `admin_scopes.go` (DB implementation) y `admin_clients.go` (DB implementation) pero NO se usan en `main.go`. V1 usa las versiones FS/ControlPlane.*
*Nota: `admin_keys.go` está vacío (deprecated). `admin_mailing.go` es helper de `admin_tenants_fs.go`.*

## 8. Admin Tenants & Users (FS Control Plane - "Monohandler")
**Source:** `internal/http/v1/handlers/admin_tenants_fs.go` (Handler: `AdminTenantsFSHandler`)
**Status:** God Handler mezclando Routing, Lógica y Persistencia.

| Método | Ruta | Descripción | Notas |
| :--- | :--- | :--- | :--- |
| **GET** | `/v1/admin/tenants` | Listar Tenants | Lee de FS (Provider) |
| **POST** | `/v1/admin/tenants` | Crear Tenant | Escribe en FS + Cluster |
| **GET** | `/v1/admin/tenants/{slug}` | Obtener Tenant | Retorna config + ETag |
| **DELETE** | `/v1/admin/tenants/{slug}` | Eliminar Tenant | Borra de FS + Cluster |
| **GET** | `/v1/admin/tenants/{slug}/settings` | Leer Settings | Retorna settings + ETag |
| **PUT** | `/v1/admin/tenants/{slug}/settings` | Actualizar Settings | Requiere `If-Match`. Encripta secretos. |
| **POST** | `/v1/admin/tenants/{slug}/keys/rotate` | Rotar Claves (JWKS) | Genera nuevas keys, invalida cache JWKS. |
| **POST** | `/v1/admin/tenants/{slug}/user-store/migrate` | Migrar DB Tenant | Alias de legacy `/migrate`. |
| **POST** | `/v1/admin/tenants/{slug}/migrate` | Migrar DB Tenant | Alias de `user-store/migrate`. |
| **POST** | `/v1/admin/tenants/{slug}/user-store/test-connection` | Test DB Connection | Ping a la DB del tenant. |
| **GET** | `/v1/admin/tenants/{slug}/infra-stats` | Infra Stats | Devuelve stats DB y Cache. |
| **POST** | `/v1/admin/tenants/{slug}/schema/apply` | Apply User Schema | Aplica índices para campos custom. |
| **POST** | `/v1/admin/tenants/{slug}/mailing/test` | Test Mailing | Envía email de prueba (SMTP tenant). |
| **POST** | `/v1/admin/tenants/{slug}/cache/test-connection` | Test Cache | Ping a Redis/Cache del tenant. |
| **GET** | `/v1/admin/tenants/{slug}/users` | List Users | Lista usuarios (usa Store interface casting). |
| **POST** | `/v1/admin/tenants/{slug}/users` | Create User | Crea usuario e identidad. |
| **PATCH** | `/v1/admin/tenants/{slug}/users/{id}` | Update User | Actualiza campos (ej. `source_client_id`). |
| **DELETE** | `/v1/admin/tenants/{slug}/users/{id}` | Delete User | Elimina usuario. |
| **PUT/POST**| `/v1/admin/tenants/{slug}/clients/{clientID}` | Upsert Client (Legacy) | Para compatibilidad cluster (ruta anidada). |
| **PUT/POST**| `/v1/admin/tenants/{slug}/scopes` | Bulk Upsert Scopes | Sincronización masiva de scopes. |

## 9. Admin Clients (FS Based)
**Source:** `internal/http/v1/handlers/admin_clients_fs.go` (Handler: `AdminClientsFSHandler`)
**Nota:** V1 utiliza esta versión basada en FS/ControlPlane. Existe `admin_clients.go` (DB implementation) pero es **código muerto** en `main.go`.

| Método | Ruta | Descripción | Notas |
| :--- | :--- | :--- | :--- |
| **GET** | `/v1/admin/clients` | `handlers.AdminClientsFS` | admin_clients_fs.go | List Clients (Tenant en header/query) |
| **POST** | `/v1/admin/clients` | `handlers.AdminClientsFS` | admin_clients_fs.go | Create Client (Upsert FS/Cluster) |
| **PUT, PATCH** | `/v1/admin/clients/{clientId}` | `handlers.AdminClientsFS` | admin_clients_fs.go | Update Client (Upsert FS/Cluster) |
| **DELETE** | `/v1/admin/clients/{clientId}` | `handlers.AdminClientsFS` | admin_clients_fs.go | Delete Client |

## 10. Admin RBAC
**Source:** `internal/http/v1/handlers/admin_rbac.go` (Handlers: `AdminRBACUsersRolesHandler`, `AdminRBACRolePermsHandler`)

| Método | Ruta | Descripción | Notas |
| :--- | :--- | :--- | :--- |
| **GET** | `/v1/admin/rbac/users/{userID}/roles` | `handlers.AdminRBACUsersRoles` | admin_rbac.go | List User Roles |
| **POST** | `/v1/admin/rbac/users/{userID}/roles` | `handlers.AdminRBACUsersRoles` | admin_rbac.go | Modificar roles. Body: `{ "add": [], "remove": [] }` |
| **GET** | `/v1/admin/rbac/roles/{role}/perms` | `handlers.AdminRBACRolePerms` | admin_rbac.go | List Role Perms (Tenant en Token) |
| **POST** | `/v1/admin/rbac/roles/{role}/perms` | `handlers.AdminRBACRolePerms` | admin_rbac.go | Modificar perms. Body: `{ "add": [], "remove": [] }` |

## 11. Admin Users Actions
**Source:** `internal/http/v1/handlers/admin_users.go` (Handler: `AdminUsersHandler`)

| Método | Ruta | Descripción | Notas |
| :--- | :--- | :--- | :--- |
| **POST** | `/v1/admin/users/disable` | Disable User | Desactiva usuario. |
| **POST** | `/v1/admin/users/enable` | Enable User | Activa usuario. |
| **POST** | `/v1/admin/users/resend-verification` | Resend Verify | Reenvía email de verificación. |

## 12. Utils, Config & Helpers
**Source:** `internal/http/v1/handlers/` y `routes.go` closures.

| Método | Ruta | Handler | Fuente | Descripción |
| :--- | :--- | :--- | :--- | :--- |
| **GET** | `/v1/csrf` | `handlers.CSRFGet` | csrf.go | Obtener CSRF token |
| **GET** | `/v1/auth/config` | `handlers.AuthConfig` | routes.go | Config pública/branding |
| **GET** | `/v1/profile` | `handlers.Profile` | profile.go | Recurso protegido demo (WhoAmI) |
| **GET** | `/v1/auth/social/{provider}/{action}` | `handlers.DynamicSocial` | social_dynamic.go | Social Start/Callback (Google) |
| **POST** | `/v1/auth/social/exchange` | `handlers.SocialExchange` | social_exchange.go | Canjear login code por tokens |
| **GET** | `/v1/auth/social/result` | `handlers.SocialResult` | social_result.go | UI resultado social/debug |
| **GET** | `/v1/auth/social/debug/code` | (func) | routes.go | Debug social code (dev only) |
| **GET** | `/v1/auth/providers` | `handlers.Providers` | providers.go | Listar providers habilitados (bootstrap UI) |
| **GET** | `/v1/providers/status` | `handlers.Providers` | providers.go | Alias de providers list |



# Notas: 
**Utils:** claims_hook.go, cookieutil.go, json.go, session_logout_util.go son helpers. mfa_types.go son definiciones de tipos.
**Legacy/Internal:** email_flows_wiring.go y email_flows_wrappers.go son infraestructura del handler principal `email_flows.go`.
**Impl/Logic:** social_google.go implementa la lógica de Google invocada por `social_dynamic.go`.
**Empty/TODO:** oauth_callback.go, oauth_start.go, registry_clients.go están vacíos.