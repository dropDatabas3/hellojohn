# GUIDE V2 — Data Plane vs Control Plane (FS / Global DB / Tenant DB)

Este documento explica cómo se relacionan los componentes principales en v1 para diseñar interfaces limpias en v2.

## 1) Definiciones

- **Control Plane**: configuración y “source of truth” de tenants/clients/scopes/settings. En v1, el provider principal es FS.
- **Data Plane**: persistencia operacional (tokens/sesiones/consents, usuarios, MFA, etc). Puede ser global DB, tenant DB, o ambos.

## 2) Control Plane en v1 (FS provider)

Hechos (v1):

- Se inicializa con:
  - `cpctx.Provider = cpfs.New(cfg.ControlPlane.FSRoot)` en `cmd/service/v1/main.go`.
- Resolución de tenant:
  - `X-Tenant-Slug` → query `?tenant` → fallback `"local"`.
- Seguridad:
  - `SECRETBOX_MASTER_KEY` (base64 32 bytes) es requerida para desencriptar secretos del control-plane.

El contrato está en:
- `internal/controlplane/provider.go`
- `internal/controlplane/types.go`

Puntos relevantes del schema:
- `TenantSettings` incluye SMTP, MailingTemplates, `UserDBSettings` (driver + DSN **encriptado**), cache settings, issuer mode, etc.

## 3) Data Plane en v1

### 3.1 Global DB (opcional)

En v1, la DB global se activa si:
- `cfg.Storage.Driver` y `cfg.Storage.DSN` están seteados.

Entonces se hace:
- `store.OpenStores(...)` → `stores` y `repo = stores.Repository`.
- Migraciones globales (si `cfg.Flags.Migrate`).

Consecuencias actuales:
- Email flows **solo** se habilitan si hay DB global.
- El keystore de signing keys se vuelve **híbrido**:
  - lee/escribe en PG (si está) pero también persiste en FS (`FSRoot/keys`).

### 3.2 Tenant DB (per-tenant)

Siempre se inicializa:
- `tenantSQLManager := tenantsql.New(...)` con migraciones `migrations/postgres/tenant`.

La idea del sistema (y la dirección para v2):
- Las operaciones de identidad por tenant (usuarios, custom fields, etc) deben vivir en DB por tenant (aislamiento real).
- El DSN por tenant se define en el control-plane (`TenantSettings.UserDB`) y se desencripta con SecretBox.

## 4) JWKS / Signing keys

v1 implementa un patrón importante para HA:

- Global JWKS: puede bootstrapearse.
- Per-tenant JWKS:
  - el cache loader devuelve `JWKSJSONForTenant(tenant)`.
  - **no** hace auto-bootstrap para evitar que en HA cada nodo genere keys distintas.

v2 debería mantener esta regla y moverla a un módulo claro (keys/issuer).

## 5) Email y templates

v1 crea:
- `container.SenderProvider = email.NewTenantSenderProvider(cpctx.Provider, secretboxMasterKey)`

Esto sugiere el modelo v2:
- El sender (SMTP settings) proviene del control-plane (tenant settings).
- Los flows de verify/reset pueden ser data-plane (tokens/persistencia) y necesitan store.

En v1, si no hay DB global, los endpoints de email responden 501.

## 6) Cluster / control-plane replication (embedded Raft)

v1 soporta modo:
- `cfg.Cluster.Mode == "embedded"`.

Inicialización:
- carpeta `FSRoot/raft`
- `cluster.NewNode(...)` con peers/tls/bootstrapping.
- `container.ClusterNode` se inyecta.

Se expone además un patrón para coherencia:
- `cpctx.InvalidateJWKS(tenant)` para invalidar caches tras restore/applies.
- `cpctx.ClearFSDegraded()` para que el provider marque degraded/healthy en readyz.

## 7) Diseño recomendado para v2 (interfaces)

Objetivo: que ningún handler decida “fallback” por su cuenta.

### 7.1 Resolver explícito de repositorios

Crear un componente único:

- `TenantContextResolver`:
  - Resuelve tenant slug
  - Carga tenant settings (control-plane)
  - Abre/obtiene repo de tenant (tenant SQL manager)
  - Expone `Capabilities` (tiene global? tiene tenant DB? read-only? leader?)

Los handlers v2 operan con:
- `ctx := resolver.Resolve(r)` → `ctx.Tenant`, `ctx.TenantRepo`, `ctx.GlobalRepo`, `ctx.Settings`.

### 7.2 Eliminar “fallback silencioso”

Regla sugerida:
- Si una ruta es tenant-scoped y falta tenant DB/config → responder error explícito (`409`/`501`/`503` según caso), nunca usar global DB como “accidental fallback”.

### 7.3 Mutaciones del control-plane

v1 ya muestra la intención:
- endpoints admin que mutan FS llevan `RequireLeader(&container)`.

En v2:
- separar read vs write:
  - reads: pueden servir en followers
  - writes: requieren leader o redirección controlada (allowlist)

## 8) Dónde mirar en el repo

- Bootstrap: `cmd/service/v1/main.go`
- Router: `internal/http/v1/routes.go`
- Container: `internal/app/v1/app.go`
- Control plane: `internal/controlplane/provider.go`, `internal/controlplane/types.go`
- Tenant SQL manager: `internal/infra/tenantsql/*`
- Middleware (CORS/headers/recover/log/rate): `internal/http/v1/middleware.go`
