0) "✅ Migración Completada: AdminClientsFSHandler

Handler Seleccionado
HandlerID: AdminClientsFSHandler
Archivo V1: admin_clients_fs.go
Rutas
Método Ruta V1 Ruta V2
GET /v1/admin/clients /v2/admin/clients
POST /v1/admin/clients /v2/admin/clients
PUT/PATCH /v1/admin/clients/{clientId} /v2/admin/clients/{clientId}
DELETE /v1/admin/clients/{clientId} /v2/admin/clients/{clientId}
Resumen V1
Resuelve tenant via header/query (X-Tenant-Slug, X-Tenant-ID, tenant, tenant_id)
Parsea JSON cp.ClientInput
Si hay cluster: aplica mutation Raft + read-back
Si no hay cluster: write directo vía cpctx.Provider
Default tenant = "local"
Diseño V2 (Archivos creados/modificados)
Capa Archivo Descripción
DTO dto/admin/client.go Request/Response para clients
Service services/admin/clients_service.go Orquesta controlplane.Service
Controller controllers/admin/clients_controller.go HTTP parsing + validación + mapeo errores
Router router/admin_routes.go Middlewares + dispatch
Middleware chain aplicada
Recover → RequestID → Logging → TenantResolution → RequireTenant
Notas de compatibilidad
V2 usa controlplane.Service que internamente maneja cluster (no explícito en controller)
V2 no usa fallback a "local" - requiere tenant explícito (mejor práctica)
Contratos de request/response compatibles con V1
Cómo probar
bash
List clients
curl -X GET http://localhost:8080/v2/admin/clients -H "X-Tenant-Slug: bananacorp"

Create
curl -X POST http://localhost:8080/v2/admin/clients
-H "X-Tenant-Slug: bananacorp"
-H "Content-Type: application/json"
-d '{"client_id":"my-app","name":"My App","type":"public"}'

Update
curl -X PUT http://localhost:8080/v2/admin/clients/my-app
-H "X-Tenant-Slug: bananacorp"
-H "Content-Type: application/json"
-d '{"name":"My Updated App","type":"public"}'

Delete
curl -X DELETE http://localhost:8080/v2/admin/clients/my-app
-H "X-Tenant-Slug: bananacorp"

Build test
go build ./internal/http/v2/...
8) Doc Update
Inventario actualizado: AdminClientsFSHandler marcado con ✅ y rutas V2.

⚠️ Pendiente para integración completa:

Registrar rutas V2 en main.go cuando se active el server V2
Los lints de SonarQube sobre strings duplicados son conocidos y pueden limpiarse en una iteración de cleanup"



1)
✅ Migración Completada: AdminConsentsHandler
1) Qué se migró / qué hiciste
HandlerID: AdminConsentsHandler
V1: internal/http/v1/handlers/admin_consents.go
V2:
Moví parseo/validación a controller
Saqué lógica de negocio a service (resolución clientID + revoke tokens)
Reusé ConsentRepository + TokenRepository.RevokeAllByUser + ClientRepository.Get
Agregué middleware RequireTenantDB() (Data Plane)
2) Rutas (routes) creadas o modificadas
GET /v2/admin/consents → ConsentsController.List (router: admin_routes.go)
GET /v2/admin/consents/by-user/{userID} → ConsentsController.ListByUser
POST /v2/admin/consents/upsert → ConsentsController.Upsert
POST /v2/admin/consents/revoke → ConsentsController.Revoke
DELETE /v2/admin/consents/{userID}/{clientID} → ConsentsController.Delete
3) Controllers creados o modificados
internal/http/v2/controllers/admin/consents_controller.go
ConsentsController.Upsert
ConsentsController.ListByUser
ConsentsController.List
ConsentsController.Revoke
ConsentsController.Delete
4) Services creados o modificados
internal/http/v2/services/admin/consents_service.go
ConsentService.Upsert
ConsentService.ListByUser
ConsentService.Get
ConsentService.Revoke
ConsentService.ResolveClientUUID
5) Tests unitarios recomendados (LISTA)
Controller:

Parse/validate OK
Body inválido / JSON inválido
Validaciones: user_id vacío, user_id no UUID, client_id vacío, scopes vacíos
Mapeo de errores: 400/404/503/500
Response shape: array de consents
Service:

Caso feliz con mocks de repos
RequireDB → error si tenant sin DB (503)
Not-found client → 404
Revoke + best-effort token revocation (mock Tokens repo)
ResolveClientUUID: UUID directo vs lookup público
6) Tests de integración recomendados (LISTA)
Endpoint E2E con httptest
Sin tenant → 400
Tenant sin DB → 503
Upsert + Get → consent visible
Revoke + Get activeOnly=true → lista vacía
Delete via path → 204
Client_id público vs UUID → ambos funcionan
Compat V1 vs V2: mismo input/output shapes







2)✅ Migración Completada: AdminUsersHandler
1) Qué se migró / qué hiciste
HandlerID: AdminUsersHandler
V1: internal/http/v1/handlers/admin_users.go
V2:
Moví parseo/validación a controller
Saqué lógica de negocio a service (disable/enable/token creation/emails)
Emails de notificación ahora son async (goroutine best-effort)
Reusé UserRepository.Disable/Enable, TokenRepository, EmailTokenRepository, emailv2.Service
2) Rutas (routes) creadas o modificadas
POST /v2/admin/users/disable → UsersController.Disable
POST /v2/admin/users/enable → UsersController.Enable
POST /v2/admin/users/resend-verification → UsersController.ResendVerification
3) Controllers creados o modificados
internal/http/v2/controllers/admin/users_controller.go
UsersController.Disable
UsersController.Enable
UsersController.ResendVerification
4) Services creados o modificados
internal/http/v2/services/admin/users_service.go
UserActionService.Disable
UserActionService.Enable
UserActionService.ResendVerification
sendBlockNotification (async)
sendUnblockNotification (async)
5) Tests unitarios recomendados (LISTA)
Controller:

Parse/validate OK
Body inválido / JSON inválido
Validaciones: user_id vacío, user_id no UUID, duration inválida
Mapeo de errores: 400/404/503/500
Response: 204 No Content
Service:

Caso feliz con mocks de repos
RequireDB → error si tenant sin DB (503)
User not found → 404
Email already verified → 400
Token creation falla → 500
Email send falla → 500 (solo resend-verification)
Token revocation best-effort (no bloquea)
6) Tests de integración recomendados (LISTA)
Endpoint E2E con httptest
Sin tenant → 400
Tenant sin DB → 503
Disable + verificar user status
Enable + verificar user status
Resend-verification: user no existe → 404
Resend-verification: email verified → 400
Flujo completo: resend → token creado → email enviado
Nota: Los lints de SonarQube sobre strings duplicados son conocidos y se limpiarán en cleanup general.



3)✅ Migración Completada: AdminScopesFSHandler
1) Qué se migró / qué hiciste
HandlerID: AdminScopesFSHandler
V1: internal/http/v1/handlers/admin_scopes_fs.go
V2:
Moví parseo/validación a controller
Saqué lógica de negocio a service
Reusé controlplane.Service (ListScopes/CreateScope/DeleteScope)
Control Plane, no requiere DB
2) Rutas (routes) creadas o modificadas
GET /v2/admin/scopes → ScopesController.ListScopes
POST /v2/admin/scopes → ScopesController.UpsertScope
PUT /v2/admin/scopes → ScopesController.UpsertScope
DELETE /v2/admin/scopes/{name} → ScopesController.DeleteScope
3) Controllers creados o modificados
internal/http/v2/controllers/admin/scopes_controller.go
ScopesController.ListScopes
ScopesController.UpsertScope
ScopesController.DeleteScope
4) Services creados o modificados
internal/http/v2/services/admin/scopes_service.go
ScopeService.List
ScopeService.Upsert
ScopeService.Delete
5) Tests unitarios recomendados (LISTA)
Controller:

Parse/validate OK
Body inválido / JSON inválido
Validación: name vacío
Mapeo de errores: 400/404/409/500
Response shape: lista de scopes / status ok
Service:

Caso feliz con mock de controlplane.Service
Name vacío → error
Scope in use → 409
6) Tests de integración recomendados (LISTA)
Endpoint E2E con httptest
Sin tenant → 400
List scopes vacío → []
Upsert + List → scope visible
Delete scope → 200 {status:ok}
Delete scope inexistente → comportamiento esperado (404 o sin error)
Compat V1 vs V2: mismo input/output shapes
Resumen hasta ahora (4 handlers migrados):

✅ AdminClientsFSHandler
✅ AdminConsentsHandler
✅ AdminUsersHandler
✅ AdminScopesFSHandler




4) ✅ Migración Completada: AdminRBACUsersRolesHandler + AdminRBACRolePermsHandler
1) Qué se migró / qué hiciste
HandlerID: AdminRBACUsersRolesHandler + AdminRBACRolePermsHandler
V1: internal/http/v1/handlers/admin_rbac.go
V2:
Moví parseo/validación a controller
Saqué lógica de negocio a service
Service usa type assertions sobre TDA para detectar soporte RBAC
Si store no soporta RBAC → 501 Not Implemented
2) Rutas (routes) creadas o modificadas
GET /v2/admin/rbac/users/{userID}/roles → RBACController.GetUserRoles
POST /v2/admin/rbac/users/{userID}/roles → RBACController.UpdateUserRoles
GET /v2/admin/rbac/roles/{role}/perms → RBACController.GetRolePerms
POST /v2/admin/rbac/roles/{role}/perms → RBACController.UpdateRolePerms
3) Controllers creados o modificados
internal/http/v2/controllers/admin/rbac_controller.go
RBACController.GetUserRoles
RBACController.UpdateUserRoles
RBACController.GetRolePerms
RBACController.UpdateRolePerms
4) Services creados o modificados
internal/http/v2/services/admin/rbac_service.go
RBACService.GetUserRoles
RBACService.UpdateUserRoles
RBACService.GetRolePerms
RBACService.UpdateRolePerms
Interfaces RBACReader, RBACWriter para type assertions
5) Tests unitarios recomendados (LISTA)
Controller:

Parse/validate OK
Body inválido / JSON inválido
user_id no UUID → 400
role vacío → 400
Mapeo de errores: 400/501/503/500
Service:

Store soporta RBAC → caso feliz
Store NO soporta RBAC → 501
RequireDB falla → 503
Dedup/trim de add/remove




5) ✅ Migración Completada: JWKSHandler
1) Qué se migró / qué hiciste
HandlerID: JWKSHandler
V1: internal/http/v1/handlers/jwks.go
V2:
Moví parseo de path/método a controller
Saqué lógica de obtener JWKS a service
Regex precompilada para validación de slugs
Endpoint público (no auth requerido)
2) Rutas (routes) creadas
GET/HEAD /.well-known/jwks.json → JWKSController.GetGlobal
GET/HEAD /.well-known/jwks/{slug}.json → JWKSController.GetByTenant
Router: internal/http/v2/router/oidc_routes.go
3) Controllers creados
internal/http/v2/controllers/oidc/jwks_controller.go
JWKSController.GetGlobal
JWKSController.GetByTenant
4) Services creados
internal/http/v2/services/oidc/jwks_service.go
JWKSService.GetGlobalJWKS
JWKSService.GetTenantJWKS
Regex precompilada: slugRe
Error tipado: ErrInvalidSlug
5) Tests unitarios recomendados
Controller:

GET/HEAD válidos → 200
Otro método → 405 + Allow header
Slug inválido → 400
No-Store headers presentes
Service:

GetGlobalJWKS → cache.Get("global")
GetTenantJWKS con slug válido
GetTenantJWKS con slug inválido → ErrInvalidSlug
6) Tests de integración recomendados
E2E con httptest
HEAD devuelve solo headers (no body)
Slug válido devuelve JWKS JSON
Slug inválido → 400
Cache miss → 500
Nota: UserProfileHandler mencionado en inventario no existe en código actual (marcado como pendiente de investigar).



6) ✅ Migración Completada: OIDCDiscoveryHandler + TenantOIDCDiscoveryHandler
1) Qué se migró / qué hiciste
HandlerID: OIDCDiscoveryHandler, TenantOIDCDiscoveryHandler
V1: internal/http/v1/handlers/oidc_discovery.go
V2:
Moví parseo de path/método a controller
Saqué lógica de OIDC metadata a service
Regex precompilada para validación de slugs
Reusé controlplane.Service.GetTenant() para lookup de tenant
Endpoints públicos (no auth requerido)
2) Rutas (routes) creadas
GET/HEAD /.well-known/openid-configuration → DiscoveryController.GetGlobal
GET/HEAD /t/{slug}/.well-known/openid-configuration → DiscoveryController.GetByTenant
Router: internal/http/v2/router/oidc_routes.go
3) Controllers creados
internal/http/v2/controllers/oidc/discovery_controller.go
DiscoveryController.GetGlobal
DiscoveryController.GetByTenant
4) Services creados
internal/http/v2/services/oidc/discovery_service.go
DiscoveryService.GetGlobalDiscovery
DiscoveryService.GetTenantDiscovery
Regex precompilada: tenantSlugRe
Errores: ErrInvalidTenantSlug, ErrTenantNotFound
5) DTOs creados
internal/http/v2/dto/oidc/discovery.go
OIDCMetadata struct
6) Tests unitarios recomendados
Controller:

GET/HEAD válidos → 200
Otro método → 405 + Allow header
Slug inválido → 400
Tenant no encontrado → 404
Cache-Control headers correctos
Service:

GetGlobalDiscovery → metadata con baseIssuer
GetTenantDiscovery con slug válido
GetTenantDiscovery con slug inválido → ErrInvalidTenantSlug
Tenant no existe → ErrTenantNotFound
7) Tests de integración recomendados
E2E con httptest
HEAD devuelve solo headers
Global: Cache-Control: public, max-age=600
Per-tenant: Cache-Control: no-store
Issuer resuelto correctamente por tenant settings
Resumen de handlers migrados hasta ahora (9 handlers):

✅ AdminClientsFSHandler
✅ AdminConsentsHandler
✅ AdminUsersHandler
✅ AdminScopesFSHandler
✅ AdminRBACUsersRolesHandler
✅ AdminRBACRolePermsHandler
✅ JWKSHandler
✅ OIDCDiscoveryHandler
✅ TenantOIDCDiscoveryHandler






7) ✅ Migración Completada: ReadyzHandler
1) Qué se migró / qué hiciste
HandlerID: ReadyzHandler
V1: internal/http/v1/handlers/readyz.go
V2:
Moví parseo de método a controller
Saqué lógica de probes a service
Dependencias inyectables via functions (DBCheck, RedisCheck, ClusterChecker, TenantPools)
No depende de Container V1
2) Rutas (routes) creadas
GET /readyz → HealthController.Readyz
Router: internal/http/v2/router/health_routes.go
3) Controllers creados
internal/http/v2/controllers/health/health_controller.go
HealthController.Readyz
4) Services creados
internal/http/v2/services/health/health_service.go
HealthService.Check
Probes: control_plane, keystore, db_global, redis, tenant_pools, cluster
Status: ready/degraded/unavailable
5) DTOs creados
internal/http/v2/dto/health/health.go
HealthStatus
HealthResponse
6) Tests unitarios recomendados
Controller:

GET válido → 200/503 según status
Otro método → 405
Headers X-Service-Version/Commit/KID presentes
Service:

Todos los probes OK → status="ready"
ControlPlane/Keystore error → status="unavailable"
DB/Redis error → status="degraded"
Cluster info correctamente construido
7) Tests de integración recomendados
E2E con httptest
Sin dependencias → degraded/unavailable
Con mocks de probes → ready
Resumen de handlers migrados (10 handlers):

✅ AdminClientsFSHandler
✅ AdminConsentsHandler
✅ AdminUsersHandler
✅ AdminScopesFSHandler
✅ AdminRBACUsersRolesHandler
✅ AdminRBACRolePermsHandler
✅ JWKSHandler
✅ OIDCDiscoveryHandler
✅ TenantOIDCDiscoveryHandler
✅ ReadyzHandler



8) ✅ Migración Completada: UserInfoHandler
1) Qué se migró / qué hiciste
HandlerID: UserInfoHandler
V1: internal/http/v1/handlers/userinfo.go
V2:
Moví validación JWT a service
Saqué scope gating a service
Tenant resolution usando ControlPlane.GetTenant y ControlPlane.GetTenantByID
Soporte robusto de scopes (scp []any, scp string, scope string)
WWW-Authenticate headers en errores
2) Rutas (routes) creadas
GET/POST /userinfo → UserInfoController.GetUserInfo
Router: internal/http/v2/router/oidc_routes.go
3) Controllers creados
internal/http/v2/controllers/oidc/userinfo_controller.go
UserInfoController.GetUserInfo
4) Services creados
internal/http/v2/services/oidc/userinfo_service.go
UserInfoService.GetUserInfo
Errores: ErrMissingToken, ErrInvalidToken, ErrIssuerMismatch, ErrMissingSub
5) DTOs creados
internal/http/v2/dto/oidc/userinfo.go
UserInfoResponse
6) Tests unitarios recomendados
Controller:

GET/POST válidos → 200
Otro método → 405
Sin token → 401 + WWW-Authenticate
Token inválido → 401
Service:

Token válido → UserInfoResponse con claims
Token expirado/inválido → ErrInvalidToken
Issuer mismatch → ErrIssuerMismatch
Sin sub → ErrMissingSub
Scope "email" presente → email incluido
Scope "email" ausente → email omitido
7) Tests de integración recomendados
E2E caso feliz con token válido
Sin Authorization header → 401
Token con issuer incorrecto → 401
Token de usuario inexistente → 200 con custom_fields vacío
Resumen de handlers migrados (11 handlers): ✅ AdminClientsFSHandler, AdminConsentsHandler, AdminUsersHandler, AdminScopesFSHandler, AdminRBACUsersRolesHandler, AdminRBACRolePermsHandler, JWKSHandler, OIDCDiscoveryHandler, TenantOIDCDiscoveryHandler, ReadyzHandler, UserInfoHandler

ℹ️ Nota SonarQube: El método GetUserInfo tiene complejidad cognitiva 21 (límite 15). Considerar refactor futuro.



9) ✅ Migración Completada: AuthLoginHandler (Core Iteration 1)
1) Qué se migró / qué hiciste
HandlerID: AuthLoginHandler
V1: internal/http/v1/handlers/auth_login.go (858 líneas)
V2: ~400 líneas en arquitectura modular por capas
Moví parseo JSON/form a controller
Saqué lógica de negocio a service
Usé DAL V2: Users().GetByEmail(), Tokens().Create()
Token issuance con jwtx.ResolveIssuer() + key selection
2) Rutas (routes) creadas
POST /v2/auth/login → LoginController.Login
Router: internal/http/v2/router/auth_routes.go
3) Controllers creados
internal/http/v2/controllers/auth/login_controller.go
LoginController.Login
Parse JSON/form, MaxBytesReader 64KB, mapeo errores
4) Services creados
internal/http/v2/services/auth/login_service.go
LoginService.LoginPassword
Errores: ErrMissingFields, ErrInvalidClient, ErrPasswordNotAllowed, ErrInvalidCredentials, ErrUserDisabled, ErrEmailNotVerified, ErrNoDatabase, ErrTokenIssueFailed
internal/http/v2/services/auth/contracts.go
LoginService interface + ClaimsHook extensible
5) DTOs creados
internal/http/v2/dto/auth/login.go
LoginRequest, LoginResponse, MFARequiredResponse, LoginResult
6) Features incluidas (Iteración 1)
✅ Tenant resolution via DAL
✅ Client resolution desde FS
✅ Provider gating ("password" permitido)
✅ Email/password verification
✅ User disabled check
✅ Email verification gating (si client lo requiere)
✅ Claims base (tid, amr, acr, scp)
✅ ClaimsHook extensible (NoOp por defecto)
✅ Issuer resolution por tenant + key selection
✅ Access + Refresh token issuance
7) Features pendientes (iteraciones futuras)
⬜ MFA gate + challenge cache (Iteración 3)
⬜ RBAC roles/perms en claims (Iteración 2)
⬜ FS Admin login separado (endpoint dedicado)
⬜ Rate limit específico de login en service
8) Tests recomendados
Controller:

POST válido → 200 + tokens
Otro método → 405
JSON inválido → 400
Missing fields → 400
Errores service → HTTP correcto
Service:

Tenant inválido → ErrInvalidClient
Client sin "password" → ErrPasswordNotAllowed
Usuario no existe → ErrInvalidCredentials
Password incorrecto → ErrInvalidCredentials
Usuario deshabilitado → ErrUserDisabled
Email not verified → ErrEmailNotVerified
Caso feliz → tokens válidos
Resumen de handlers migrados (12 handlers): ✅ AdminClientsFSHandler, AdminConsentsHandler, AdminUsersHandler, AdminScopesFSHandler, AdminRBACUsersRolesHandler, AdminRBACRolePermsHandler, JWKSHandler, OIDCDiscoveryHandler, TenantOIDCDiscoveryHandler, ReadyzHandler, UserInfoHandler, AuthLoginHandler (Core)