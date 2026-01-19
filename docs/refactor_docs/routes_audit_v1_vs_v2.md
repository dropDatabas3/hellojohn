# Auditoría de Rutas HTTP: V1 (Legacy) vs V2 (Target)

**Fecha:** 13 de Enero de 2026
**Alcance:** Análisis estático y de runtime (wiring) de endpoints HTTP.

## 1. Resumen Ejecutivo

| Métrica | Estado | Detalles |
| :--- | :--- | :--- |
| **Arquitectura V1** | **ACTIVA (100%)** | Monolítica, centralizada en `internal/http/v1/routes.go`. Maneja todo el tráfico actual. |
| **Arquitectura V2** | **INACTIVA (0%)** | Modular, pero **NO ESTÁ CONECTADA**. `RegisterV2Routes` no es llamada por `app.go` ni `server.go`. |
| **Paridad de Código** | ~70% | Gran parte de controllers/services existen en V2, pero no están "cableados" al router principal. |
| **Riesgo Principal** | **Código Muerto** | Todo el código V2 es actualmente "dead code" en runtime. No se puede probar end-to-end sin wiring. |

> [!WARNING]
> **HALLAZGO CRÍTICO**: El router principal V2 (`internal/http/v2/router/router.go`) define `RegisterV2Routes`, pero **esta función NO tiene referencias en el código base**. Ningún servidor V2 arranca realmente.

## 2. Inventario V2: Rutas "Latentes" (Código existente no activo)

Aunque V2 no está activo, el router agregador (`v2/router/router.go`) tiene lógica parcial para registrar rutas. Si se activara hoy, solo expondría:

### Rutas "Wired" en el Agregador V2 (pero el agregador está desconectado)
| Dominio | Status en `router.go` | Endpoints Latentes |
| :--- | :--- | :--- |
| **Admin** | ✅ Registrado | `/v2/admin/tenants/*` (CRUD, Utils), `/v2/admin/clients`, `/v2/admin/consents`, `/v2/admin/scopes`, `/v2/admin/users/*` |
| **MFA** | ✅ Registrado | `/v2/mfa/totp/enroll`, `/monitor/verify`, `/challenge`, etc. |

### Rutas "Implemented but Unregistered" (Existen pero `router.go` las ignora)
Estos dominios tienen archivos de router (`*_routes.go`) y controllers, pero están comentados o ausentes en `RegisterV2Routes`:

| Dominio | Router File | Estado en Agregador | Notas |
| :--- | :--- | :--- | :--- |
| **Auth** | `auht_routes.go` | ❌ Ausente | `RegisterAuthRoutes` está comentado. Login/Register no andarían en V2. |
| **OAuth2** | `oauth_routes.go` | ❌ Ausente | `RegisterOAuthRoutes` comentado. Authorize/Token inactivos. |
| **OIDC** | `oidc_routes.go` | ❌ Ausente | `RegisterOIDCRoutes` ausente. Discovery/UserInfo inactivos. |
| **Session** | `session_routes.go` | ❌ Ausente | `RegisterSessionRoutes` comentado. |
| **Social** | `social_routes.go` | ❌ Ausente | Ausente. |
| **Email** | `email_routes.go` | ❌ Ausente | Ausente. Flows de reset password inactivos. |
| **Security** | `security_routes.go`| ❌ Ausente | Ausente. CSRF inactivo. |
| **Health** | `health_routes.go` | ❌ Ausente | Ausente. `/readyz` inactivo. |

### Rutas Placeholder / Stub V2
| Dominio | Estado |
| :--- | :--- |
| **Assets** | ❌ Archivo vacío (`assets_routes.go`). Sin implementación. |
| **Dev** | ❌ Archivo vacío (`dev_routes.go`). |
| **Public** | ❌ Archivo vacío (`public_routes.go`). Config pública/branding sin ruta. |

## 3. Inventario V1 (Ground Truth - Rutas Activas)
El archivo `internal/http/v1/routes.go` registra todo lo que funciona hoy:

| Dominio | Método | Path V1 | Handler / Notas |
| :--- | :--- | :--- | :--- |
| **Health** | GET | `/healthz`, `/readyz` | Inline func / Handler |
| **OIDC** | GET | `/.well-known/openid-configuration` | `oidcDiscovery` |
| **OIDC** | GET | `/.well-known/jwks.json` | `jwksHandler` |
| **OAuth2** | GET | `/oauth2/authorize` | `oauthAuthorize` |
| **OAuth2** | POST | `/oauth2/token` | `oauthToken` |
| **OAuth2** | POST | `/oauth2/revoke` | `oauthRevoke` |
| **OAuth2** | POST | `/oauth2/introspect` | `oauthIntrospect` |
| **OAuth2** | GET | `/userinfo` | `userInfo` |
| **Auth** | POST | `/v1/auth/login` | `authLoginHandler` |
| **Auth** | POST | `/v1/auth/register` | `authRegisterHandler` |
| **Auth** | POST | `/v1/auth/refresh` | `authRefreshHandler` |
| **Auth** | POST | `/v1/auth/logout` | `authLogoutHandler` |
| **Auth** | POST | `/v1/auth/logout-all` | `authLogoutAll` |
| **Session** | POST | `/v1/session/login` | `sessionLogin` |
| **Session** | POST | `/v1/session/logout` | `sessionLogout` |
| **Email** | POST | `/v1/auth/verify-email/start` | `verifyEmailStartHandler` |
| **Email** | GET | `/v1/auth/verify-email` | `verifyEmailConfirmHandler` |
| **Email** | POST | `/v1/auth/forgot` | `forgotHandler` |
| **Email** | POST | `/v1/auth/reset` | `resetHandler` |
| **MFA** | POST | `/v1/mfa/totp/*` | `mfaEnroll`, `mfaVerify`... |
| **Admin** | GET/POST | `/v1/admin/tenants` | `adminTenants` (Monohandler) |
| **Admin** | GET/PUT | `/v1/admin/tenants/*` | `adminTenants` (Settings, Migrate, UserStore) |
| **Admin** | POST | `/v1/admin/users/*` | `adminUsers` |
| **Admin** | ALL | `/v1/admin/clients` | `adminClients` |
| **Admin** | ALL | `/v1/admin/scopes` | `adminScopes` |
| **Admin** | ALL | `/v1/admin/consents` | `adminConsents` |
| **Assets** | GET | `/v1/assets/*` | `http.FileServer` (con filtro de extensión) |

## 4. Matriz de Migración & Gaps

| Endpoint V1 | Equivalente V2 | Estado V2 Código | Estado V2 Wiring | Notas |
| :--- | :--- | :--- | :--- | :--- |
| `/v1/admin/tenants` | `/v2/admin/tenants` | ✅ Impl | ✅ Wired | Listo para integración. |
| `/v1/auth/login` | `/v2/auth/login` | ✅ Impl | ❌ Unwired | Código existe, router desconectado. |
| `/oauth2/token` | `/oauth2/token` | ✅ Impl | ❌ Unwired | Código existe, router desconectado. |
| `/v1/assets/*` | ? | ❌ Stub | ❌ Missing | Falta implementar `assets_routes.go`. |
| `/__dev/shutdown` | ? | ❌ Stub | ❌ Missing | Falta `dev_routes.go`. |
| `/v1/auth/logout-all` | `/v2/auth/logout-all` | 🟡 Incompleto | ❌ Unwired | Controller devuelve `ErrNotImplemented`. |

## 5. Próximos Pasos (Plan de Acción)

### P0: Wiring de Aplicación (CRÍTICO)
No tiene sentido seguir migrando endpoints si V2 no arranca.
1.  Implementar `internal/app/v2/app.go`: Crear `NewApp(...)` o similar.
2.  Implementar `internal/http/v2/server/wiring.go`: Construir dependencias y llamar a `RegisterV2Routes`.
3.  Exponer V2 en un puerto (ej: 8081) o prefijo (`/v2`) del servidor principal.

### P1: Completar Agregador V2
El archivo `router.go` debe llamar a los registros de rutas que ya existen pero están comentados:
-   `RegisterAuthRoutes`
-   `RegisterOAuthRoutes`
-   `RegisterOIDCRoutes`
-   `RegisterSocialRoutes`
-   `RegisterSessionRoutes`

### P2: Rellenar Gaps
1.  Implementar `AssetsController` (FileServer seguro).
2.  Implementar `DevController` (Shutdown, Debug).
3.  Terminar `LogoutAll` y `RBAC`.

## Conclusión
La migración a V2 ha avanzado mucho en **código de negocio** (Controllers/Services/Stores), pero la capa de **infraestructura/wiring** es inexistente. El sistema V2 es actualmente un conjunto de componentes desconectados.
