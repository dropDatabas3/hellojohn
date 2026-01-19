# GUIDE V2 — Bootstrap & Wiring (from v1 facts)

Este documento describe el *orden real* de inicialización en v1 (y cómo replicarlo limpiamente en v2), basado en `cmd/service/v1/main.go` y `internal/http/v1/*`.

## 1) Objetivo de v2

- Separar **bootstrap (cmd)**, **infra** (control-plane, stores, cache, cluster), **app container**, y **HTTP router/middlewares**.
- Eliminar el wiring frágil por parámetros posicionales (`internal/http/v1/routes.go:NewMux(...)`).
- Hacer explícito el modo de storage (FS / Global DB / Tenant DB) y sus consecuencias.

## 2) Orden de inicialización en v1 (fuente de verdad)

En `cmd/service/v1/main.go` el flujo efectivo es:

1. Flags y dotenv
   - `-config`, `-env`, `-env-file`, `-print-config`.
   - Si no `DISABLE_DOTENV=1`, intenta cargar `.env`.

2. Carga de config
   - `-env` → `loadConfigFromEnv()` + `cfg.Validate()`.
   - caso contrario → `config.Load(cfgPath)`.

3. Hard requirements (FS control-plane)
   - `SECRETBOX_MASTER_KEY` requerido (base64 de **32 bytes**).
   - `CONTROL_PLANE_FS_ROOT` requerido.
   - `SIGNING_MASTER_KEY` requerido y con longitud `>= 32`.

4. Control-plane FS
   - `cpctx.Provider = cpfs.New(cfg.ControlPlane.FSRoot)`.
   - `cpctx.ResolveTenant = func(r) string { X-Tenant-Slug → ?tenant → "local" }`.

5. Cluster (opcional)
   - Si `cfg.Cluster.Mode == "embedded"`:
     - crea `FSRoot/raft`.
     - inicia `cluster.NewNode(...)` con `BootstrapPreferred = CLUSTER_BOOTSTRAP`.
     - guarda `clusterNode` para inyectarlo al container.

6. Tenant SQL Manager (siempre)
   - `tenantsql.New(...)` con `migrations/postgres/tenant`.
   - Se cierra con `defer tenantSQLManager.Close()`.

7. Global DB (opcional)
   - `hasGlobalDB := cfg.Storage.Driver != "" && cfg.Storage.DSN != ""`.
   - Si `hasGlobalDB`:
     - `store.OpenStores(...)` → `stores` y `repo = stores.Repository`.
     - (opcional) migraciones globales (`cfg.Flags.Migrate`).

8. JWT / keystore persistente
   - `jwtx.PersistentKeystore` siempre.
   - Si `hasGlobalDB`:
     - usa `jwtx.NewHybridSigningKeyStore(pgRepo, fileStore)` donde `fileStore` vive en `FSRoot/keys`.
   - Si FS-only:
     - usa `jwtx.NewFileSigningKeyStore(FSRoot/keys)`.
   - `ks.EnsureBootstrap()` (log WARN si falla).

9. Cache genérica
   - `cachefactory.Open(...)` (memory/redis).

10. Construcción del container
   - `app.Container{ Store: repo (puede ser nil), Issuer, Cache, TenantSQLManager, Stores, SenderProvider }`.
   - `SenderProvider = email.NewTenantSenderProvider(cpctx.Provider, cfg.Security.SecretBoxMasterKey)`.
   - Inyecta: `ClusterNode`, `LeaderRedirects`, `RedirectHostAllowlist`.
   - `container.ScopesConsents = stores.ScopesConsents` si existe DB.

11. JWKS cache por tenant
   - `container.JWKSCache = jwtx.NewJWKSCache(15s, loader)`.
   - Reglas:
     - tenant vacío o `global` → JWKS global.
     - tenant != global → `issuer.Keys.JWKSJSONForTenant(tenant)` (sin bootstrap automático).
   - `cpctx.InvalidateJWKS` y `cpctx.ClearFSDegraded` se conectan al container.

12. Email flows (solo si hay DB global)
   - Si `hasGlobalDB`: `handlers.BuildEmailFlowHandlers(...)`.
   - Si FS-only: handlers dummy que responden `501 Not Implemented` con “Feature requires database connection”.

13. Handlers HTTP + middlewares
   - Construye handlers concretos.
   - Admin endpoints se envuelven con:
     - `RequireAuth`, `RequireSysAdmin`.
     - y para mutaciones de FS: `RequireLeader(&container)`.

14. Rate limiting
   - Si `cfg.Rate.Enabled && cfg.Cache.Kind == "redis"`:
     - inicializa Redis y `multiLimiter = rate.NewLimiterPoolAdapter(...)`.
     - (back-compat) `limiter` global.
   - si no: `multiLimiter = rate.NoopMultiLimiter{}`.
   - `container.MultiLimiter = multiLimiter`.

15. Router
   - `mux := httpserver.NewMux(...)` (posicional; frágil).
   - Rutas extra registradas en main:
     - `/metrics`.
     - `/t/` (OIDC discovery per-tenant).
     - `/v1/auth/providers` y `/v1/providers/status`.
     - `/v1/auth/complete-profile`.
     - `/v1/auth/social/result` (condicional: `cfg.Providers.Google.Enabled`).

16. (Opcional) Admin UI server
   - Si `ADMIN_UI_DIR` y `UI_SERVER_ADDR` están set:
     - levanta un server adicional (SPA handler) y agrega origen a CORS.

## 3) Contrato mínimo de v2 (recomendado)

En v2, evitar `NewMux(handler1, handler2, ...)`.

- Crear `internal/http/v2/router` con un builder tipo:
  - `router.Register(mux, deps)` por módulo.
  - o `router.New(deps).RegisterRoutes(mux)`.
- Encapsular el “modo de storage” en un struct:
  - `type RuntimeCapabilities struct { HasGlobalDB bool; HasTenantDB bool; ... }`
  - que los handlers puedan consultar (en vez de `repo == nil` implícito).

## 4) Matriz de modos de storage (lo que existe vs lo deseado)

v1 hoy sólo *gatea* explícitamente por `hasGlobalDB` (email flows, migraciones globales, parte del keystore). El manager de tenant DB existe siempre, pero el éxito depende de que el tenant tenga settings + DB accesible.

Definición práctica (para documentación/refactor):

- **FS-only**: control-plane FS + keystore en FS. Global DB ausente.
  - Limitación v1: email flows 501; otras rutas pueden fallar si requieren `Store` global.
- **FS + Global DB**: control-plane FS + store global disponible.
  - Habilita email flows, introspect, migraciones globales, y keystore híbrido.
- **FS + Tenant DB**: control-plane FS + DB por-tenant configurada en `TenantSettings.UserDB`.
  - Idealmente: auth de usuarios y custom fields viven acá.
- **FS + Global DB + Tenant DB**: modo completo (producción/HA).

## 5) Checklist de “transcripción” v1 → v2

- Extraer funciones pequeñas en `cmd/service/v2`:
  - `loadConfig()`, `initControlPlane()`, `initCluster()`, `initTenantSQLManager()`, `initGlobalStores()`, `initIssuerAndKeystore()`, `initCaches()`, `initContainer()`.
- Hacer que el router v2 sea declarativo y por módulos (Auth/OAuth/Admin/etc).
- Asegurar que el “effective routes set” sea: `routes.go + main.go` (y en v2 unificarlo).
