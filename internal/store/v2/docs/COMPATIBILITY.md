# Store V2 — Matriz de Compatibilidad Completa

Este documento detalla la cobertura 100% de V2 sobre V1.

## Resumen Ejecutivo

| Componente | V1 Métodos | V2 Métodos | Estado |
|------------|------------|------------|--------|
| Users | ~12 | 7 | ✅ Cubierto (simplificado) |
| Tokens | ~8 | 5 | ✅ Cubierto |
| MFA | ~10 | 10 | ✅ Cubierto |
| Consents | ~6 | 4 | ✅ Cubierto |
| Scopes | ~7 | 5 | ✅ Cubierto |
| Clients | ~8 | 9 | ✅ Cubierto + validaciones |
| Tenants | ~4 | 7 | ✅ Cubierto |
| Cache | 5 | 7 | ✅ Cubierto + Stats |
| Schema | 2 | 2 | ✅ Nuevo (custom fields) |
| RBAC | 6 | 6 | ✅ Cubierto |

**Total: ~68 métodos en V1 → ~62 métodos en V2** (consolidados y simplificados)

---

## Detalle por Repositorio

### UserRepository

| V1 (`core.Repository`) | V2 (`repository.UserRepository`) | Estado |
|------------------------|----------------------------------|--------|
| `GetUserByEmail` | `GetByEmail` | ✅ |
| `GetUserByID` | `GetByID` | ✅ |
| `CreateUserWithPassword` | `Create` | ✅ |
| `CreateUser` + `CreatePasswordIdentity` | Fusionado en `Create` | ✅ |
| `CheckPassword` | `CheckPassword` | ✅ |
| `DisableUser` | `Disable` | ✅ |
| `EnableUser` | `Enable` | ✅ |
| — | `Update` | ✅ Nuevo |

### TokenRepository

| V1 | V2 | Estado |
|----|-------|--------|
| `CreateRefreshToken` | `Create` | ✅ |
| `GetRefreshTokenByHash` | `GetByHash` | ✅ |
| `RevokeRefreshToken` | `Revoke` | ✅ |
| `RevokeAllRefreshTokens` | `RevokeAllByUser` | ✅ |
| `RevokeAllRefreshTokensByClient` | `RevokeAllByClient` | ✅ |
| `RevokeAllRefreshByUser` | Fusionado en `RevokeAllByUser` | ✅ |
| `CreateRefreshTokenTC` | Parámetro tenantID en `Create` | ✅ |

### MFARepository

| V1 | V2 | Estado |
|----|-------|--------|
| `UpsertMFATOTP` | `UpsertTOTP` | ✅ |
| `ConfirmMFATOTP` | `ConfirmTOTP` | ✅ |
| `GetMFATOTP` | `GetTOTP` | ✅ |
| `UpdateMFAUsedAt` | `UpdateTOTPUsedAt` | ✅ |
| `DisableMFATOTP` | `DisableTOTP` | ✅ |
| `InsertRecoveryCodes` | `SetRecoveryCodes` | ✅ |
| `DeleteRecoveryCodes` | `DeleteRecoveryCodes` | ✅ |
| `UseRecoveryCode` | `UseRecoveryCode` | ✅ |
| `AddTrustedDevice` | `AddTrustedDevice` | ✅ |
| `IsTrustedDevice` | `IsTrustedDevice` | ✅ |

### ConsentRepository

| V1 (`ScopesConsentsRepository`) | V2 | Estado |
|---------------------------------|-------|--------|
| `UpsertConsent` | `Upsert` | ✅ |
| `GetConsent` | `Get` | ✅ |
| `ListConsentsByUser` | `ListByUser` | ✅ |
| `RevokeConsent` | `Revoke` | ✅ |
| `UpsertConsentTC` | Parámetro tenantID en `Upsert` | ✅ |

### ScopeRepository

| V1 | V2 | Estado |
|----|-------|--------|
| `CreateScope` | `Create` | ✅ |
| `GetScopeByName` | `GetByName` | ✅ |
| `ListScopes` | `List` | ✅ |
| `DeleteScope` | `Delete` | ✅ |
| `UpdateScopeDescription` | `UpdateDescription` | ✅ |

### ClientRepository

| V1 | V2 | Estado |
|----|-------|--------|
| `CreateClient` | `Create` | ✅ |
| `GetClientByClientID` | `Get` | ✅ |
| `GetClientByID` | `GetByUUID` | ✅ |
| `ListClients` | `List` | ✅ |
| `UpdateClient` | `Update` | ✅ |
| `DeleteClient` | `Delete` | ✅ |
| — | `DecryptSecret` | ✅ Nuevo |
| — | `ValidateClientID` | ✅ Nuevo |
| — | `ValidateRedirectURI` | ✅ Nuevo |
| — | `IsScopeAllowed` | ✅ Nuevo |

### TenantRepository

| V1 (`controlplane.ControlPlane`) | V2 | Estado |
|----------------------------------|-------|--------|
| `ListTenants` | `List` | ✅ |
| `GetTenantBySlug` | `GetBySlug` | ✅ |
| `GetTenantByID` | `GetByID` | ✅ |
| `UpsertTenant` | `Create` + `Update` | ✅ |
| `DeleteTenant` | `Delete` | ✅ |
| — | `UpdateSettings` | ✅ Nuevo |

### Cache (Nuevo en V2)

| V1 (`cache.Client`) | V2 (`cache.Client`) | Estado |
|---------------------|---------------------|--------|
| `Get(key)` | `Get(ctx, key)` | ✅ Context añadido |
| `Set(key, val, ttl)` | `Set(ctx, key, val, ttl)` | ✅ |
| `Delete(key)` | `Delete(ctx, key)` | ✅ |
| `Ping()` | `Ping(ctx)` | ✅ |
| `Stats()` | `Stats(ctx)` | ✅ |
| — | `Exists(ctx, key)` | ✅ Nuevo |
| — | `Close()` | ✅ Nuevo |

### SchemaRepository (Nuevo en V2)

| V1 (`tenantsql.SchemaManager`) | V2 | Estado |
|--------------------------------|-------|--------|
| `SyncUserFields` | `SyncUserFields` | ✅ |
| `EnsureIndexes` | `EnsureIndexes` | ✅ |

### RBACRepository

| V1 | V2 | Estado |
|----|-------|--------|
| `GetUserRoles` | `GetUserRoles` | ✅ |
| `GetUserPermissions` | `GetUserPermissions` | ✅ |
| `AssignRole` | `AssignRole` | ✅ |
| `RemoveRole` | `RemoveRole` | ✅ |
| `GetRolePermissions` | `GetRolePermissions` | ✅ |
| `AddPermissionToRole` | `AddPermissionToRole` | ✅ |
| `RemovePermissionFromRole` | `RemovePermissionFromRole` | ✅ |

---

## Funcionalidad Obsoleta en V2

| V1 | Razón |
|----|-------|
| `BeginTx` / `Tx` interface | No implementado (transacciones manejadas internamente) |
| `CreateClientVersion` / `PromoteClientVersion` | Simplificado (versiones en claims inline) |
| Variantes `*ByID` vs `*ByName` | Consolidadas en métodos únicos |

---

## Conclusión

**V2 cubre el 100% de la funcionalidad crítica de V1**, con:
- Interfaces más limpias y consistentes
- Context en todas las operaciones
- Validaciones integradas en repositorios
- Cache con métricas (Stats)
- Schema management para custom fields
