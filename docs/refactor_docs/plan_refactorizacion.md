## Roadmap definitivo de refactor Backend (HelloJohn)

> Propósito: esta guía es el “manual de operaciones” para refactorizar HelloJohn **sin reescribir todo**, frenando la expansión de features para recuperar orden, aislar legacy y reducir riesgos cross-tenant. El enfoque es **incremental (strangler fig)** y por **vertical slices**.

---

### 0) Resumen ejecutivo (veredicto)

- El problema no es “falta de features”, es **falta de fronteras claras** entre:
  - *HTTP/controller vs negocio* (handlers enormes)
  - *global vs tenant-scoped* (fallbacks a global store)
  - *wiring/DI vs runtime* (router frágil por orden)
- La estrategia correcta es:
  1) **Poner guardrails** (tenant scoping + políticas centralizadas)
  2) **Eliminar fragilidad del router y del main** (registración modular)
  3) **Crear un carril de refactor** (TenantResources + services) para migrar sin romper
  4) Migrar **por slices verticales** (Auth/OAuth, Email, Admin Tenants, etc.)
  5) Borrar legacy cuando cada slice queda “cerrado”

> Resultado esperado: arquitectura legible, dependencias explícitas, aislamiento por tenant real, facilidad de sumar drivers/proveedores sin “esparcir if/else y fallbacks”.

---

## 1) Estado actual (hechos observables en el repo)

### 1.1 Router frágil por orden
- Hoy se construye un mux con `httpserver.NewMux(...)` pasando una lista posicional de handlers (ej. en `cmd/service/main.go`).
- La definición de rutas está en `internal/http/routes.go` y su constructor recibe muchos handlers en orden.
- Problema: **si el orden cambia** (o se añade uno en el medio) el router puede quedar mal cableado o “romperse” de forma no obvia.

**Impacto**
- Refactorizar o agregar endpoints cuesta demasiado.
- Es fácil introducir bugs por wiring.

### 1.2 Multi-tenant real “a medias” (mezcla de paradigmas)
- Existen managers por tenant:
  - DB: `internal/infra/tenantsql/manager.go` (pools on-demand + migrations)
  - Cache: `internal/infra/tenantcache/manager.go`
- Pero hay handlers que **resuelven store por su cuenta** y a veces hacen `tenantDB -> globalDB fallback`.

**Impacto**
- Riesgo de cross-tenant
- Comportamiento inconsistente por endpoint

### 1.3 Email: avances + cruces legacy
- Existe `SenderProvider` por tenant (`internal/email/provider.go`).
- En email flows hay soporte de templates por tenant via `tenant.Settings.Mailing.Templates`.
- Aun aparecen fallbacks/hardcodes en algunos lugares (p.ej. “fallback simple email”) y lógica de email repartida.

### 1.4 Config “global” perdió centralidad
- El sistema soporta env-only y FS control-plane (`CONTROL_PLANE_FS_ROOT`).
- `config.yaml` existe, pero ya no representa el “estado real” multi-tenant.

---

## 2) Principios y reglas duras (P0)

Estas reglas se aplican antes de refactorizar “bonito”. Son guardrails.

1. **Tenant-first**: Todo endpoint que toque usuarios/identidades/sesiones/tokens debe operar con tenant explícito.
2. **Global DB NO es source of truth** para usuarios/identidades de tenants.
3. **Policy centralizada**: la elección de store/cache/sender se hace en *una sola capa*, no en cada handler.
4. **Email tenant-scoped**: sender + templates siempre salen del tenant.
5. **Compatibilidad controlada**: si hay fallback legacy, debe estar:
   - (a) centralizado,
   - (b) detrás de una política/flag,
   - (c) con plan de retiro.

---

## 3) Arquitectura target (a dónde queremos llegar)

### 3.1 Capas

1) **HTTP Controllers** (`internal/http/controllers/...`)
- Parseo, validación superficial, authz/authn, mapping a HTTP.
- No DB, no negocio.

2) **Services** (`internal/domain/<feature>/service/...`)
- Reglas de negocio: login, emisión de tokens, email flows, admin ops, etc.

3) **TenantResources (Factory/Provider)** (`internal/infra/tenantresources/...`)
- Construye y cachea recursos tenant-scoped: DB, cache, mail sender, keystore.
- Aplica políticas: tenant-only, global-only, fallback permitido, etc.

4) **Repos por capacidad** (`internal/domain/<feature>/repo`) 
- Interfaces pequeñas por caso de uso (ej. `UserRepo`, `TokenRepo`, `ClientRepo`).

5) **Router modular**
- Cada módulo expone `RegisterRoutes(mux, deps)` sin orden posicional.

### 3.2 Contratos clave

- `TenantContext`:
  - Canoniza cómo se resuelve tenant (header/query/path), qué es válido en prod, y cómo se expone en `context.Context`.

- `TenantResources`:
  - `GetDB(ctx, tenant)`
  - `GetCache(ctx, tenant)`
  - `GetMailer(ctx, tenantID)`
  - `GetKeystore(ctx, tenant)`
  - `Stats()/Close()`

---

## 4) Roadmap por fases (P0 → P2)

### P0 — Frenar el caos (guardrails + router + policy)

**Meta**: reducir riesgo y crear un carril para refactor.

#### P0.1 Feature Freeze + checklist de invariantes
- Qué: pausar features nuevos; cada PR debe declarar impacto en scoping tenant.
- Por qué: sin freeze, el refactor se vuelve “correr detrás”.

**Done**
- Checklist acordado y aplicado.

#### P0.2 TenantContext único y obligatorio
- Dónde impacta: middleware HTTP + `cpctx.ResolveTenant`.
- Qué:
  - Unificar resolución de tenant.
  - En prod: prohibir fallback silencioso a `local`.
  - En dev: permitirlo con log explícito.

**Riesgos**
- UI/SDK puede depender de `?tenant=...` hoy.

**Done**
- Todos los endpoints críticos usan el tenant del contexto.

#### P0.3 Router modular (eliminar `NewMux` posicional)
- Dónde impacta:
  - `cmd/service/main.go`
  - `internal/http/routes.go`
- Qué:
  - Introducir `RegisterRoutes` por módulo.
  - Reemplazar wiring posicional por registro explícito.

**Estrategia strangler**
- Mantener `NewMux` temporalmente.
- Migrar módulos de a uno y reducir parámetros hasta eliminar.

**Done**
- Agregar rutas no requiere tocar la firma de `NewMux`.

#### P0.4 Centralizar store/cache/sender selection
- Qué:
  - Crear `TenantResources` con política única.
  - Mover lógica tenantDB/globalDB a esa capa.

**Done**
- Los handlers no deciden store a mano.

---

### P1 — Separar responsabilidades (controllers/services) por slices

**Meta**: reducir complejidad por archivo y hacer el negocio testeable.

#### P1.1 Slice piloto: Auth + Tokens
- Entrada típica:
  - `internal/http/handlers/auth_login.go`
  - `internal/http/handlers/oauth_token.go`
  - `internal/http/handlers/session_login.go`
- Qué:
  - Extraer servicios: `AuthService`, `TokenService`.
  - Repos por capacidad.
  - Controllers finos.

**Done**
- Sin fallback global oculto.
- Tests unitarios del service.

#### P1.2 Slice: Email flows
- Entrada:
  - `internal/http/handlers/email_flows.go`
  - `internal/http/handlers/email_flows_wiring.go`
  - `internal/email/provider.go`
- Qué:
  - Unificar render + envío.
  - Templates: tenant override primero, fallback a defaults.
  - Eliminar hardcodes dispersos.

**Done**
- Todos los envíos pasan por un servicio.

#### P1.3 Slice: Admin Tenants
- Entrada:
  - `internal/http/handlers/admin_tenants_fs.go`
  - `internal/infra/tenantsql/schema_manager.go`
- Qué:
  - Separar CRUD/settings/schema ops.

---

### P2 — Limpieza final + multi-driver real (si aplica)

**Meta**: sacar legacy y habilitar crecimiento ordenado.

#### P2.1 Multi-driver per-tenant
- Nota: hoy `tenantsql.Manager` es PG-only.
- Plan:
  - Primero la interfaz.
  - Luego drivers uno por uno.

#### P2.2 Quitar singleton `cpctx` (opcional)
- Migrar a DI explícita.

#### P2.3 Router con path params (opcional)
- Solo después de modularizar.

---

## 5) Procedimientos repetibles (cómo hacerlo “a pie de letra”)

### Procedimiento A: Migrar un handler a Controller + Service
1. Definir DTOs request/response.
2. Extraer reglas de negocio a service.
3. Definir interfaces mínimas (repos por capacidad).
4. Controller sólo orquesta: parse → service → response.
5. Tests unitarios para el service.

### Procedimiento B: Eliminar fallback global (sin romper)
1. Identificar dónde cae a global store.
2. Mover política a `TenantResources`.
3. Convertir endpoint a tenant-only.
4. Si hay compat temporal: feature flag y fecha de retiro.
5. Borrar legacy al cerrar slice.

### Procedimiento C: Introducir interfaces sin reescribir todo
1. Interface pequeña.
2. Adapter desde implementación actual.
3. Cambiar service a depender de interface.
4. Wiring en un solo lugar.

---

## 6) Definition of Done (por endpoint)

Un endpoint está “migrado” cuando:
1. Tenant requerido y validado.
2. DB/cache/mail scoping por tenant.
3. Sin `global fallback` implícito.
4. Lógica de negocio fuera de HTTP.
5. Errores consistentes y con codes.
6. Tests de service (mínimos).

---

## 7) Checklist de ejecución del refactor (operativo)

- [ ] Freeze features
- [ ] Acordar invariantes tenant-first
- [ ] Router modular
- [ ] TenantResources
- [ ] Slice Auth+Tokens
- [ ] Slice Email
- [ ] Slice Admin Tenants
- [ ] Borrado de legacy por slice
- [ ] Docs actualizadas

---

## 8) Notas finales

- Este roadmap está hecho para que el sistema siga funcionando mientras se refactoriza.
- La prioridad es **aislar y centralizar** decisiones de tenant scoping.
- La refactor “bonita” viene después de estabilizar.
