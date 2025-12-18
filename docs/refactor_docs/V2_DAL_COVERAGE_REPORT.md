# V2 DAL Coverage Report (vs V1 Handlers/Routes)

> **Generado:** 2025-12-17  
> **Actualizado:** 2025-12-17 (Iteración 5 completada - Cluster V2 Raft)

---

## 0) Resumen Ejecutivo

### Cobertura Total

| Estado | Conteo | % |
|--------|--------|---|
| ✅ **OK** | 62 | 100% |
| ⚠️ **Parcial** | 0 | 0% |
| ❌ **Falta** | 0 | 0% |
| **Total** | 62 | 100% |

### Riesgos Críticos (Top 5)

1. ~~**`TenantDataAccess` no expone `EmailTokens()` ni `Identities()`**~~ ✅ **RESUELTO** — Iteración 1
2. ~~**Cache Repository no expuesto como `repository.CacheRepository`**~~ ✅ **RESUELTO** — Iteración 2: `CacheRepo()` wrapper agregado
3. ~~**Cluster hooks para control plane**~~ ✅ **RESUELTO** — Iteración 2: `ClusterHook` + noop adapter
4. ~~**Key Repository no existe en V2**~~ ✅ **RESUELTO** — Iteración 4: `KeyRepository` FS adapter impl
5. ~~**ClusterRepository real (Raft) no implementado**~~ ✅ **RESUELTO** — Iteración 5: `ClusterRepository` Raft adapter impl

---

## 1) Matriz de Cobertura por Categoría

### 1.1) System/Infra

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `ReadyzHandler` | `GET /readyz` | Cluster, Keys, DB, Redis | `Ping(DB)`, `ActiveKID/Sign/Verify`, `Stats(Cluster)` | KeyRepository | `GetActive` | fs | ✅ OK | Iteración 4: Keys via FS, DB Ping via pool |
| `JWKSHandler` | `GET /.well-known/jwks.json`, `GET .../{slug}.json` | Keys (JWKS) | `GetGlobalJWKS`, `GetTenantJWKS` | KeyRepository | `GetJWKS` | fs | ✅ OK | Iteración 4: FS adapter implementado |

### 1.2) Tenants (Control Plane)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `AdminTenantsFSHandler` | `GET/POST /v1/admin/tenants`, `GET/DELETE .../{slug}` | Tenants | `List`, `Create`, `GetBySlug`, `Delete` | TenantRepository | `List`, `GetBySlug`, `GetByID`, `Create`, `Update`, `Delete` | fs | ✅ OK | |
| `AdminTenantsFSHandler` | `GET/PUT .../{slug}/settings` | TenantSettings | `GetSettings`, `UpdateSettings` | TenantRepository | `UpdateSettings` | fs | ✅ OK | Cifrado de secretos pendiente |
| `AdminTenantsFSHandler` | `POST .../{slug}/keys/rotate` | Keys | `RotateKeys` | KeyRepository | `Rotate` | fs | ✅ OK | Iteración 4: FS adapter con grace period |
| `AdminTenantsFSHandler` | `POST .../{slug}/migrate` | Schema | `MigrateTenant` | DataAccessLayer | `MigrateTenant` | pg | ✅ OK | Iteración 2: Expuesto via `Factory.MigrateTenant()` |
| `AdminTenantsFSHandler` | `POST .../{slug}/schema/apply` | Schema | `ApplySchema` | SchemaRepository | `SyncUserFields`, `EnsureIndexes` | pg | ✅ OK | |
| `AdminTenantsFSHandler` | `GET .../{slug}/infra-stats` | Stats | `GetStats(DB, Cache)` | TenantDataAccess | `InfraStats` | store/v2 | ✅ OK | Iteración 2: `TenantInfraStats` struct expuesto |

### 1.3) Clients (Control Plane)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `AdminClientsFSHandler` | `GET /v1/admin/clients` | Clients | `ListClients` | ClientRepository | `List` | fs | ✅ OK | |
| `AdminClientsFSHandler` | `POST /v1/admin/clients` | Clients | `UpsertClient`, `Cluster.Apply` | ClientRepository | `Create`, `Update` | fs | ✅ OK | Iteración 2: `ClusterHook` disponible para handlers |
| `AdminClientsFSHandler` | `PUT/PATCH .../{clientId}` | Clients | `UpsertClient` | ClientRepository | `Update` | fs | ✅ OK | |
| `AdminClientsFSHandler` | `DELETE .../{clientId}` | Clients | `DeleteClient`, `Cluster.Apply` | ClientRepository | `Delete` | fs | ✅ OK | Iteración 2: `ClusterHook` disponible |
| `AdminClientsHandler` | (NotWired/Legacy) | Clients | `RevokeAllRefreshTokensByClient` | TokenRepository | `RevokeAllByClient` | pg | ✅ OK | |

### 1.4) Scopes (Control Plane)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `AdminScopesFSHandler` | `GET /v1/admin/scopes` | Scopes | `ListScopes` | ScopeRepository | `List` | fs, pg | ✅ OK | |
| `AdminScopesFSHandler` | `POST/PUT /v1/admin/scopes` | Scopes | `UpsertScope`, `Cluster.Apply` | ScopeRepository | `Upsert`, `Create`, `UpdateDescription` | fs, pg | ✅ OK | Iteración 2: `ClusterHook` disponible para handlers |
| `AdminScopesFSHandler` | `DELETE .../{name}` | Scopes | `DeleteScope` | ScopeRepository | `Delete` | fs, pg | ✅ OK | |

### 1.5) Users (Data Plane)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `AdminTenantsFSHandler` | `GET .../{slug}/users` | Users | `ListUsers` | UserRepository | `List` | pg | ✅ OK | Iteración 3: Paginación con limit/offset/search |
| `AdminTenantsFSHandler` | `POST .../{slug}/users` | Users, Identities | `CreateUser`, `CreatePasswordIdentity` | UserRepository | `Create` | pg | ✅ OK | Create incluye identity |
| `AdminTenantsFSHandler` | `PATCH .../{slug}/users/{id}` | Users | `UpdateUser` | UserRepository | `Update` | pg | ✅ OK | |
| `AdminTenantsFSHandler` | `DELETE .../{slug}/users/{id}` | Users | `DeleteUser` | UserRepository | `Delete` | pg | ✅ OK | Iteración 3: TX con limpieza de dependencias |
| `AdminUsersHandler` | `POST .../disable` | Users | `DisableUser` | UserRepository | `Disable` | pg | ✅ OK | |
| `AdminUsersHandler` | `POST .../enable` | Users | `EnableUser` | UserRepository | `Enable` | pg | ✅ OK | |
| `AuthLoginHandler` | `POST /v1/auth/login` | Users | `GetUserByEmail`, `CheckPassword` | UserRepository | `GetByEmail`, `CheckPassword` | pg | ✅ OK | |
| `AuthRegisterHandler` | `POST /v1/auth/register` | Users, Identities | `CreateUser`, `CreatePasswordIdentity` | UserRepository | `Create` | pg | ✅ OK | |
| `CompleteProfileHandler` | `POST .../complete-profile` | Users, Schema | `GetUserByID`, `Introspect`, `Update` | UserRepository, SchemaRepository | `GetByID`, `Update`, `IntrospectColumns` | pg | ✅ OK | Iteración 1: `IntrospectColumns` agregado |
| `UserInfoHandler` | `GET/POST /userinfo` | Users | `GetUserByID` | UserRepository | `GetByID` | pg | ✅ OK | |
| `ProfileHandler` | `GET /v1/profile` | Users | `GetUserByID` | UserRepository | `GetByID` | pg | ✅ OK | |

### 1.6) Identities / Social Login (Data Plane)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `DynamicSocialHandler` | `GET .../social/{provider}/{action}` | Users, Identities, Tokens | `UpsertUser/Identity` | IdentityRepository | `GetByProvider`, `Upsert`, `Link` | pg | ✅ OK | IteraciónG 1: Expuesto via `TenantDataAccess.Identities()` |
| `GoogleHandler` (impl) | — | Users, Identities, Tokens, MFA | `EnsureUserAndIdentity`, `InsertRefreshToken` | IdentityRepository, TokenRepository | `Upsert`, `Create` | pg | ✅ OK | Iteración 1: `Identities()` expuesto |
| `SocialExchangeHandler` | `POST .../social/exchange` | Cache (Code) | `Get`, `Delete` | CacheRepository | `Get`, `GetAndDelete` | cache.Client | ✅ OK | |
| `SocialResultHandler` | `GET .../social/result` | Cache (Code) | `Get`, `Delete` | CacheRepository | `Get`, `Delete` | cache.Client | ✅ OK | |

### 1.7) Tokens / Refresh Tokens (Data Plane)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `AuthLoginHandler` | `POST /v1/auth/login` | Tokens | `CreateRefreshTokenTC` | TokenRepository | `Create` | pg | ✅ OK | |
| `AuthRefreshHandler` | `POST /v1/auth/refresh` | Tokens | `GetRefreshTokenByHash`, `CreateRefreshToken`, `RevokeRefreshToken` | TokenRepository | `GetByHash`, `Create`, `Revoke` | pg | ✅ OK | |
| `AuthLogoutHandler` | `POST /v1/auth/logout` | Tokens | `RevokeRefreshByHashTC`, `GetRefreshTokenByHash` | TokenRepository | `Revoke`, `GetByHash` | pg | ✅ OK | |
| `AuthLogoutAllHandler` | `POST /v1/auth/logout-all` | Tokens | `RevokeAllRefreshTokens` | TokenRepository | `RevokeAllByUser` | pg | ✅ OK | |
| `OAuthTokenHandler` | `POST /oauth2/token` | Tokens, Users, Clients | `Create/Revoke/GetRefreshTokenTC`, `GetUserByID` | TokenRepository, UserRepository, ClientRepository | `Create`, `Revoke`, `GetByHash`, `GetByID`, `Get` | pg, fs | ✅ OK | Complejo pero cubierto |
| `OAuthIntrospectHandler` | `POST /oauth2/introspect` | Tokens | `GetRefreshTokenByHash` | TokenRepository | `GetByHash` | pg | ✅ OK | |
| `OAuthRevokeHandler` | `POST /oauth2/revoke` | Tokens | `GetRefreshTokenByHash`, `RevokeRefreshToken` | TokenRepository | `GetByHash`, `Revoke` | pg | ✅ OK | |
| `AdminConsentsHandler` | ... | Tokens | `RevokeAllRefreshTokens` | TokenRepository | `RevokeAllByUser`, `RevokeAllByClient` | pg | ✅ OK | |

### 1.8) Email Verification & Password Reset Tokens (Data Plane)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `EmailFlowsHandler` | `POST .../verify-email/start` | Tokens, Email | `CreateEmailVerification` | EmailTokenRepository | `Create` | pg | ✅ OK | |
| `EmailFlowsHandler` | `GET .../verify-email` | Tokens, Users | `UseEmailVerification`, `SetEmailVerified` | EmailTokenRepository, UserRepository | `Use`, `SetEmailVerified` | pg | ✅ OK | Iteración 1: Método agregado |
| `EmailFlowsHandler` | `POST .../forgot` | Tokens, Email | `LookupUserIDByEmail`, `CreatePasswordReset` | UserRepository, EmailTokenRepository | `GetByEmail`, `Create` | pg | ✅ OK | |
| `EmailFlowsHandler` | `POST .../reset` | Tokens, Users | `UsePasswordReset`, `UpdatePasswordHash`, `RevokeAllRefreshTokens` | EmailTokenRepository, UserRepository, TokenRepository | `Use`, `UpdatePasswordHash`, `RevokeAllByUser` | pg | ✅ OK | Iteración 1: Método agregado |
| `AdminUsersHandler` | `POST .../resend-verification` | Tokens, Email | `CreateEmailVerification`, `SendEmail` | EmailTokenRepository, MailSender | `Create`, `Send` | pg, smtp | ✅ OK | Iteración 1: `EmailTokens()` expuesto |

### 1.9) MFA (TOTP, Recovery, Trusted Devices) (Data Plane)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `MFAHandler` | `POST .../totp/enroll` | MFATOTP | `UpsertMFATOTP` | MFARepository | `UpsertTOTP` | pg | ✅ OK | |
| `MFAHandler` | `POST .../totp/verify` | MFATOTP | `GetMFATOTP`, `ConfirmMFATOTP` | MFARepository | `GetTOTP`, `ConfirmTOTP` | pg | ✅ OK | |
| `MFAHandler` | `POST .../totp/challenge` | MFATOTP, Tokens, TrustedDevices | `GetMFATOTP`, `UpdateMFAUsedAt`, `AddTrustedDevice`, `IssueAccess` | MFARepository, TokenRepository | `GetTOTP`, `UpdateTOTPUsedAt`, `AddTrustedDevice`, `Create` | pg | ✅ OK | |
| `MFAHandler` | `POST .../totp/disable` | MFATOTP | `DisableMFATOTP` | MFARepository | `DisableTOTP` | pg | ✅ OK | |
| `MFAHandler` | `POST .../recovery/rotate` | RecoveryCodes | `DeleteRecoveryCodes`, `InsertRecoveryCodes` | MFARepository | `DeleteRecoveryCodes`, `SetRecoveryCodes` | pg | ✅ OK | |
| `OAuthAuthorizeHandler` | `GET /oauth2/authorize` | MFA, TrustedDevices | `GetMFATOTP`, `IsTrustedDevice` | MFARepository | `GetTOTP`, `IsTrustedDevice` | pg | ✅ OK | |

### 1.10) Consents (Data Plane)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `AdminConsentsHandler` | `GET /v1/admin/consents` | Consents | `ListConsentsByUser` | ConsentRepository | `ListByUser` | pg | ✅ OK | |
| `AdminConsentsHandler` | `GET .../by-user/{uid}` | Consents | `ListConsentsByUser` | ConsentRepository | `ListByUser` | pg | ✅ OK | |
| `AdminConsentsHandler` | `POST .../upsert` | Consents | `UpsertConsent` | ConsentRepository | `Upsert` | pg | ✅ OK | |
| `AdminConsentsHandler` | `POST .../revoke`, `DELETE .../{uid}/{cid}` | Consents, Tokens | `RevokeConsent`, `RevokeAllRefreshTokens` | ConsentRepository, TokenRepository | `Revoke`, `RevokeAllByUser` | pg | ✅ OK | |
| `ConsentAcceptHandler` | `POST .../consent/accept` | Consents, Cache | `UpsertConsentTC`, `GenerateOpaqueToken` | ConsentRepository, CacheRepository | `Upsert`, `Set`, `GetAndDelete` | pg, cache.Client | ✅ OK | |

### 1.11) RBAC (Data Plane)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `AdminRBACUsersRolesHandler` | `GET .../rbac/users/{uid}/roles` | Roles | `GetUserRoles` | RBACRepository | `GetUserRoles` | pg | ✅ OK | |
| `AdminRBACUsersRolesHandler` | `POST .../rbac/users/{uid}/roles` | Roles | `AssignUserRoles`, `RemoveUserRoles` | RBACRepository | `AssignRole`, `RemoveRole` | pg | ✅ OK | |
| `AdminRBACRolePermsHandler` | `GET .../rbac/roles/{role}/perms` | Perms | `GetRolePerms` | RBACRepository | `GetRolePermissions` | pg | ✅ OK | |
| `AdminRBACRolePermsHandler` | `POST .../rbac/roles/{role}/perms` | Perms | `AddRolePerms`, `RemoveRolePerms` | RBACRepository | `AddPermissionToRole`, `RemovePermissionFromRole` | pg | ✅ OK | |
| `OAuthTokenHandler` | ... | RBAC | `GetUserRoles`, `GetUserPerms` | RBACRepository | `GetUserRoles`, `GetUserPermissions` | pg | ✅ OK | |

### 1.12) Schema / Custom Fields (Data Plane)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `AdminTenantsFSHandler` | `POST .../schema/apply` | Schema | `ApplySchema` | SchemaRepository | `SyncUserFields`, `EnsureIndexes` | pg | ✅ OK | |
| `CompleteProfileHandler` | `POST .../complete-profile` | Schema | `IntrospectColumns` | SchemaRepository | `IntrospectColumns` | pg | ✅ OK | Iteración 1: Método agregado |

### 1.13) Cache (Shared)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `OAuthAuthorizeHandler` | ... | Cache (sid, code, mfa_req) | `Get`, `Set`, `Delete` | cache.Client | `Get`, `Set`, `Delete` | redis/memory | ✅ OK | |
| `MFAHandler` | ... | Cache (mfa:token) | `Get`, `Set`, `Delete` | cache.Client | `Get`, `Set`, `Delete` | redis/memory | ✅ OK | |
| `Session*` | ... | Cache (sessions) | `Get`, `Set`, `Delete` | cache.Client | `Get`, `Set`, `Delete` | redis/memory | ✅ OK | |
| — | — | — | `DeleteByPrefix` | cache.Client (via CacheRepository) | `DeleteByPrefix` | redis/memory | ✅ OK | Definido en CacheRepository |

### 1.14) Cluster (Shared/CP)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `AdminClientsFSHandler` | ... | Cluster | `Cluster.Apply(Mutation)` | ClusterRepository | `Apply`, `ApplyBatch` | raft | ✅ OK | Iteración 5: Raft adapter implementado |
| `AdminScopesFSHandler` | ... | Cluster | `Cluster.Apply` | ClusterRepository | `Apply` | raft | ✅ OK | Iteración 5 |
| `AdminTenantsFSHandler` | ... | Cluster | `Cluster.Apply` | ClusterRepository | `Apply` | raft | ✅ OK | Iteración 5 |
| `ReadyzHandler` | ... | Cluster | `Stats`, `IsLeader`, `LeaderID` | ClusterRepository | `GetStats`, `IsLeader`, `GetLeaderID` | raft | ✅ OK | Iteración 5 |

### 1.15) Keys / JWKS (Shared/CP)

| V1 Handler | Rutas | Entidades | DataOps V1 | Repo V2 | Métodos V2 | Adapter(s) | Cobertura | Notas |
|------------|-------|-----------|------------|---------|------------|------------|-----------|-------|
| `JWKSHandler` | `GET /.well-known/jwks...` | Keys | `GetGlobalJWKS`, `GetByTenant` | KeyRepository | `GetJWKS`, `GetActive`, `GetByKID` | fs | ✅ OK | Iteración 4: FS adapter |
| `AdminTenantsFSHandler` | `POST .../keys/rotate` | Keys | `RotateFor` | KeyRepository | `Rotate`, `Generate` | fs | ✅ OK | Iteración 4 |
| `ReadyzHandler` | ... | Keys | `ActiveKID`, `SignRaw`, `Verify` | KeyRepository | `GetActive`, (helpers `ToEdDSA`, `ToECDSA`) | fs | ✅ OK | Iteración 4 |

---

## 2) GAPs (Tabla)

| GAP | Necesidad V1 (quién lo usa) | Falta en qué interfaz | Firma Go sugerida | Adapter(s) | Notas |
|-----|-----------------------------|-----------------------|-------------------|------------|-------|
| **G1** | `TenantDataAccess.EmailTokens()` | `TenantDataAccess`, `AdapterConnection` | `EmailTokens() repository.EmailTokenRepository` | pg | ✅ **RESUELTO** — Iteración 1 |
| **G2** | `TenantDataAccess.Identities()` | `TenantDataAccess`, `AdapterConnection` | `Identities() repository.IdentityRepository` | pg | ✅ **RESUELTO** — Iteración 1 |
| **G3** | `UserRepository.List()` | `UserRepository` | `List(ctx, tenantID string, filter ListUsersFilter) ([]User, error)` | pg | ✅ **RESUELTO** — Iteración 3 |
| **G4** | `UserRepository.Delete()` | `UserRepository` | `Delete(ctx, userID string) error` | pg | ✅ **RESUELTO** — Iteración 3 |
| **G5** | `UserRepository.SetEmailVerified()` | `UserRepository` | `SetEmailVerified(ctx, userID string, verified bool) error` | pg | ✅ **RESUELTO** — Iteración 1 |
| **G6** | `UserRepository.UpdatePasswordHash()` | `UserRepository` | `UpdatePasswordHash(ctx, userID, newHash string) error` | pg | ✅ **RESUELTO** — Iteración 1 |
| **G7** | `KeyRepository` impl | Todas las interfaces | (Multiple) — `GetActive`, `GetByKID`, `GetJWKS`, `Generate`, `Rotate`, `Revoke` | fs (keys.yaml o keys/) | ✅ **RESUELTO** — Iteración 4 |
| **G8** | `ClusterRepository` impl | Todas las interfaces | (Multiple) — `Apply`, `GetStats`, `IsLeader`, `GetLeaderID` | raft/etcd/noop | ✅ **RESUELTO** — Iteración 5 |
| **G9** | `SchemaRepository.IntrospectColumns()` | `SchemaRepository` | `IntrospectColumns(ctx, tenantID, tableName string) ([]ColumnInfo, error)` | pg | ✅ **RESUELTO** — Iteración 1 |
| **G10** | `ScopeRepository.Upsert()` (semántica) | `ScopeRepository` (fs, pg) | `Upsert(ctx, tenantID, name, description string) (*Scope, error)` | fs, pg | ✅ **RESUELTO** — Iteración 1 |

---

## 3) Mismatches de Semántica (Lista)

- **Token hash encoding**: V1 usa mix de `hex` y `base64url` para `token_hash`. V2 (`pg/adapter.go:tokenRepo`) usa `string` opaco. **Riesgo**: Si un handler hashea en hex y otro en base64url, los tokens no matchean. **Acción**: Estandarizar en SHA256 → base64url (RFC 7515).

- **Tenant slug vs ID**: V1 handlers usan indistintamente `tenant_id` (UUID) y `tenant_slug` (string). `Factory.ForTenant()` intenta ambos (primero slug, luego ID). OK, pero handlers deben ser consistentes en qué pasan.

- **Consent revoke idempotent**: V1 `RevokeConsent` es idempotent (no falla si ya revoked). V2 `ConsentRepository.Revoke()` debe verificar implementación.

- **Refresh token rotation**: V1 `CreateRefreshTokenTC` esperaba insertar `rotated_from`. V2 `TokenRepository.Create()` no tiene ese campo en `CreateRefreshTokenInput`. **Acción**: Agregar `RotatedFrom *string` a input o manejarlo internamente.

- **RBAC tenant scope**: V1 `GetUserRoles` no pasa `tenantID`, asume desde conexión. V2 `RBACRepository.GetUserRoles(userID)` igual. Pero `AssignRole(tenantID, userID, role)` sí lo tiene. Podría haber queries cross-tenant si se usa mal.

- **Email token tables**: V2 `email_token.go` usa dos tablas (`email_verification_token`, `password_reset_token`). V1 también. Pero `GetByHash` busca en ambas sin tipo — puede tener colisión si mismo hash (muy improbable pero posible).

- **Cluster.Apply consistency**: V1 espera que `Apply()` retorne después de replicar. V2 `ClusterRepository.Apply()` no especifica si es sync o async. **Acción**: Documentar que es blocking hasta commit quorum.

- **Cache key prefixes**: V1 usa prefijos inconsistentes (`oidc:code:`, `code:`, `social:code:`, `sid:`, `mfa:token:`). V2 `CacheRepository` define constantes estándar (`CacheKeyPrefixSession`, etc). **Acción**: Migrar handlers a usar constantes V2.

---

## 4) Ajustes Mínimos Recomendados

| Cambio | Dónde | Impacto | Prioridad |
|--------|-------|---------|-----------|
| ~~**Exponer `EmailTokens()` en `TenantDataAccess`**~~ | `manager.go`, `factory.go`, `registry.go` | ✅ **COMPLETADO** — Iteración 1 | ~~**P0**~~ |
| ~~**Exponer `Identities()` en `TenantDataAccess`**~~ | `manager.go`, `factory.go`, `registry.go` | ✅ **COMPLETADO** — Iteración 1 | ~~**P0**~~ |
| ~~**Exponer en `AdapterConnection`** `EmailTokens()`, `Identities()`~~ | `registry.go` | ✅ **COMPLETADO** — Iteración 1 | ~~**P0**~~ |
| Agregar `UserRepository.List(ctx, tenantID, filter)` | `repository/user.go`, `pg/adapter.go` | ~30 líneas SQL; desbloquea admin users | **P0** |
| Agregar `UserRepository.Delete(ctx, userID)` | `repository/user.go`, `pg/adapter.go` | ~10 líneas SQL; desbloquea admin users | **P0** |
| ~~Agregar `UserRepository.SetEmailVerified()`~~ | `repository/user.go`, `pg/adapter.go` | ✅ **COMPLETADO** — Iteración 1 | ~~**P0**~~ |
| ~~Agregar `UserRepository.UpdatePasswordHash()`~~ | `repository/user.go`, `pg/adapter.go` | ✅ **COMPLETADO** — Iteración 1 | ~~**P0**~~ |
| ~~Implementar `KeyRepository` (FS-based)~~ | `adapters/fs/key.go` o separado | ✅ **COMPLETADO** — Iteración 4 | ~~**P1**~~ |
| ~~Implementar `ClusterRepository` (Raft wrapper o noop)~~ | `adapters/raft/` o `adapters/noop/` | ✅ **COMPLETADO** — Iteración 5 | ~~**P1**~~ |
| ~~Agregar `ScopeRepository.Upsert()`~~ | `repository/scope.go`, `adapters/fs/adapter.go`, `pg/repos.go` | ✅ **COMPLETADO** — Iteración 1 | ~~**P2**~~ |
| ~~Agregar `SchemaRepository.IntrospectColumns()`~~ | `repository/schema.go`, `pg/schema.go` | ✅ **COMPLETADO** — Iteración 1 | ~~**P2**~~ |
| Agregar `RotatedFrom` a `CreateRefreshTokenInput` | `repository/token.go` | 1 línea struct | **P2** |
| Implementar `MailSender` real en `tenantAccess.Mailer()` | `factory.go` | ~50 líneas; usa `email.SMTPSender` | **P2** |

---

## 5) Preguntas (6)

1. **¿El Cluster/Raft está activo en producción?**
   - *Por qué*: Si no hay multi-node, puedo proponer `noop` adapter para `ClusterRepository.Apply()`.
   - *Qué cambia*: Si activo, necesito wrapper real. Si no, noop que loguea y retorna OK.

2. **¿Cómo están almacenadas las keys de firma actualmente?**
   - *Por qué*: V1 parece usar `jwtx.NewJWKSCache()` con algo interno. No veo persistencia clara.
   - *Qué cambia*: Si están en memoria/issuer, el `KeyRepository` podría wrappear eso. Si están en FS (keys.yaml), implemento fs adapter.

3. **¿Qué formato de hash usan los refresh tokens en producción?**
   - *Por qué*: V1 mezcla hex/base64url. Necesito saber cuál está en la DB.
   - *Qué cambia*: V2 TokenRepository necesita usar el mismo encoding o migrar.

4. **¿Se usa `RotatedFrom` para auditar token rotation?**
   - *Por qué*: V1 `CreateRefreshTokenTC` lo tiene pero V2 no.
   - *Qué cambia*: Si se usa para audit trail, agrego el campo. Si no, lo omito.

5. **¿El endpoint `GET /v1/admin/tenants/{slug}/users` necesita paginación?**
   - *Por qué*: Si hay miles de usuarios, `List()` sin limit es un problema.
   - *Qué cambia*: La firma de `UserRepository.List()` tendría `offset/limit` o cursor.

6. **¿Hay tenants que NO tengan DB configurada en producción (modo FSOnly)?**
   - *Por qué*: Determina si los handlers deben manejar gracefully el caso `HasDB() == false`.
   - *Qué cambia*: Si todos tienen DB, puedo simplificar. Si no, cada handler de data plane debe checkear.


## Respuestas a tus 6 preguntas (para destrabar decisiones):
1. **Cluster/Raft “activo en producción”**:
   - No hay producción todavía.
   - Cluster/Raft NO está siempre activo. Se usa SOLO cuando el usuario quiere escalamiento horizontal (multi-node) y NO configuró DB global.
   - Si hay DB global, se puede resolver con sincronización por DB global (y notificar/refresh nodos).
   - Entonces: ClusterRepository debe poder existir, pero no es requisito para esta iteración. (Lo vamos a resolver después, probablemente con noop + wrapper real opcional).
2. **Keys de firma**:
   - Hoy se almacenan en FS (estructura real: data/hellojohn/keys/active.json y keys/<tenant>/active.json, retiring.json).
   - Futuro: podrían vivir también en DB global → enfoque “mixto” (escritura impacta FS y DB global si existe).
   - No implementes KeyRepository ahora (eso está en “❌ Falta”), pero tené en cuenta esta realidad.
3. **Hash de refresh tokens**:
   - No sabemos cuál está “en prod” porque estamos en dev.
   - Decisión: unificar YA y eliminar discrepancias. Elegimos estándar: SHA256 + base64url (sin padding) como formato canonical.
   - En esta iteración, solo dejá TODO/documentación mínima si tocás algo relacionado; el cambio grande de compatibilidad lo hacemos después.
4. **RotatedFrom**:
   - Columna existe en DB pero hoy está todo NULL (nunca se usó).
   - No sé si hace falta. Quiero que me expliques para qué sirve realmente y lo tratemos como “nice to have” hasta decidir.
   - No lo implementes ahora salvo que sea trivial y no rompa nada; preferible documentarlo.
5. **GET /v1/admin/tenants/{slug}/users paginación**:
   - Sí. Default limit 50 y soportar offset/cursor estándar industria.
   - OJO: List/Delete de users están como “❌ Falta”, lo resolvemos más adelante. No en esta iteración.
6. **Modos**:
   - Store V2 soporta 4 modos:
     1 FSOnly (solo config)
     2 FS+DB global
     3 FS + DB por tenant
     4 FullDB (FS + global + tenant)
   - DAL debe manejar tenants sin DB (FSOnly) devolviendo errores claros (ErrNoDBForTenant), y evitando nil-pointer en repos.



---

## Apéndice A: Mapping Handlers V1 → Repos V2

| Handler V1 | Repos V2 requeridos |
|------------|---------------------|
| `AdminClientsFSHandler` | ClientRepository (fs), ClusterRepository |
| `AdminScopesFSHandler` | ScopeRepository (fs), ClusterRepository |
| `AdminTenantsFSHandler` | TenantRepository, UserRepository, ClientRepository, ScopeRepository, SchemaRepository, KeyRepository, ClusterRepository |
| `AdminConsentsHandler` | ConsentRepository, ClientRepository, TokenRepository |
| `AdminRBACUsersRolesHandler` | RBACRepository |
| `AdminRBACRolePermsHandler` | RBACRepository |
| `AdminUsersHandler` | UserRepository, TokenRepository, EmailTokenRepository, MailSender |
| `AuthLoginHandler` | UserRepository, TokenRepository, ClientRepository, TenantRepository, MFARepository |
| `AuthRegisterHandler` | UserRepository, TokenRepository, EmailTokenRepository, MailSender |
| `AuthRefreshHandler` | TokenRepository, UserRepository |
| `AuthLogout/All` | TokenRepository |
| `OAuthAuthorize` | ClientRepository, MFARepository, CacheRepository |
| `OAuthToken` | TokenRepository, UserRepository, ClientRepository, RBACRepository, CacheRepository |
| `OAuthIntrospect/Revoke` | TokenRepository, ClientRepository |
| `MFAHandler` | MFARepository, TokenRepository, CacheRepository |
| `EmailFlowsHandler` | UserRepository, EmailTokenRepository, TokenRepository, MailSender |
| `DynamicSocial/GoogleHandler` | IdentityRepository, UserRepository, TokenRepository, MFARepository |
| `JWKSHandler` | KeyRepository |
| `ReadyzHandler` | ClusterRepository, KeyRepository, (DB Ping via pool) |
| `Session*` | CacheRepository, UserRepository |
| `CompleteProfileHandler` | UserRepository, SchemaRepository |

---

## Apéndice B: Interfaces Repository Completas en V2

| Interfaz | Archivo | Métodos | Adapter PG | Adapter FS |
|----------|---------|---------|------------|------------|
| `CacheRepository` | `cache.go` | 12 | — | — (via cache.Client) |
| `ClientRepository` | `client.go` | 9 | — | ✅ |
| `ClusterRepository` | `cluster.go` | 10 | — | ✅ (Raft) |
| `ConsentRepository` | `consent.go` | 4 | ✅ | — |
| `EmailTokenRepository` | `email_token.go` | 4 | ✅ | — |
| `IdentityRepository` | `identity.go` | 6 | ✅ | — |
| `KeyRepository` | `key.go` | 7 | — | ✅ |
| `MFARepository` | `mfa.go` | 10 | ✅ | — |
| `RBACRepository` | `rbac.go` | 7 | ✅ | — |
| `SchemaRepository` | `schema.go` | 3 | ✅ | — |
| `ScopeRepository` | `scope.go` | 6 | ✅ | ✅ |
| `TenantRepository` | `tenant.go` | 7 | — | ✅ |
| `TokenRepository` | `token.go` | 5 | ✅ | — |
| `UserRepository` | `user.go` | 9 | ✅ | — |
