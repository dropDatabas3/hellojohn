# Plan de Implementación: Estandarización Multi-Tenant Admin

> **Proyecto:** HelloJohn
> **Fecha de Creación:** 2026-02-03
> **Responsable:** [ASIGNAR]
> **Versión:** 1.0
> **Estado:** PENDING

---

## 📋 TABLA DE CONTENIDOS

1. [Resumen Ejecutivo](#resumen-ejecutivo)
2. [Objetivos y Alcance](#objetivos-y-alcance)
3. [Análisis de Impacto](#análisis-de-impacto)
4. [Fases de Implementación](#fases-de-implementación)
5. [Plan de Testing](#plan-de-testing)
6. [Plan de Rollout](#plan-de-rollout)
7. [Criterios de Aceptación](#criterios-de-aceptación)
8. [Evidencias de Auditoría](#evidencias-de-auditoría)
9. [Checklist de Ejecución](#checklist-de-ejecución)
10. [Rollback Plan](#rollback-plan)

---

## 📊 RESUMEN EJECUTIVO

### Problema Identificado

El sistema actual tiene **múltiples métodos no estandarizados** para resolver el tenant en requests:
- Path parameters (`{id}`, `{tenant}`, `{tenantId}`)
- Headers (`X-Tenant-ID`, `X-Tenant-Slug`)
- Query params (`tenant`, `tenant_id`)
- Subdomain resolution

Además, existe una **vulnerabilidad de seguridad crítica**: no se valida que el admin tenga acceso al tenant solicitado (tenant elevation attack).

### Solución Propuesta

1. **Estandarización:** Un único método para resolver tenant: **Path parameter `/tenants/{tenant_id}/...`**
2. **Seguridad:** Middleware de validación de acceso multi-tenant admin
3. **Arquitectura:** Admin Global (acceso total) + Admin Tenant (acceso limitado)

### Beneficios

| Beneficio | Descripción | Impacto |
|-----------|-------------|---------|
| **Mantenibilidad** | 1 solo lugar para cambiar resolución de tenant | 🟢 ALTO |
| **Seguridad** | Previene tenant elevation attacks | 🔴 CRÍTICO |
| **Claridad** | Formato único y predecible | 🟢 ALTO |
| **Debugging** | Tenant siempre visible en URL | 🟢 MEDIO |

### Métricas de Éxito

- [ ] 100% de endpoints admin usan path parameter estándar
- [ ] 0 vulnerabilidades de tenant elevation
- [ ] 0 errores de resolución de tenant en logs
- [ ] 100% de tests de seguridad pasando

---

## 🎯 OBJETIVOS Y ALCANCE

### Objetivos Principales

1. **OBJ-001:** Estandarizar resolución de tenant en un único método (path parameter)
2. **OBJ-002:** Implementar validación de acceso multi-tenant admin
3. **OBJ-003:** Migrar frontend a formato estandarizado
4. **OBJ-004:** Crear suite de tests de seguridad
5. **OBJ-005:** Documentar arquitectura y decisiones

### Alcance

#### ✅ Incluido

- Backend: Middlewares, router, controllers
- Frontend: Rutas, API client, páginas admin
- Tests: Unitarios, integración, seguridad
- Documentación: Arquitectura, decisiones, evidencias

#### ❌ Excluido

- Endpoints no-admin (auth, oauth, oidc)
- SDKs públicos (JS, React, Node)
- Migraciones de base de datos
- Cambios en Control Plane FileSystem

### Dependencias

- Sistema de admin ya implementado (`internal/domain/repository/admin.go`)
- JWT con claims de admin (`internal/jwt/admin_claims.go`)
- Middleware de autenticación admin (`RequireAdminAuth`)

### Supuestos

1. El sistema ya soporta `AdminTypeGlobal` y `AdminTypeTenant`
2. El JWT de admin incluye `admin_type` y `tenants[]`
3. No hay operaciones admin en producción durante el rollout

---

## 📈 ANÁLISIS DE IMPACTO

### Impacto por Componente

| Componente | Archivos Afectados | Líneas Estimadas | Complejidad | Riesgo | Tiempo Estimado |
|------------|-------------------|------------------|-------------|--------|-----------------|
| **Middlewares** | 1 archivo | ~80 líneas | 🟢 Baja | 🟢 Bajo | 2h |
| **Router** | 1 archivo | ~30 líneas | 🟢 Baja | 🟢 Bajo | 1h |
| **Controllers** | ~10 archivos | ~20 líneas | 🟡 Media | 🟢 Bajo | 3h |
| **Frontend API** | 1 archivo | ~100 líneas | 🟡 Media | 🟡 Medio | 4h |
| **Frontend Pages** | ~15 archivos | ~200 líneas | 🟡 Media | 🟡 Medio | 6h |
| **Tests** | Nuevos | ~300 líneas | 🟡 Media | 🟢 Bajo | 4h |
| **Documentación** | Nuevos | ~500 líneas | 🟢 Baja | 🟢 Bajo | 2h |
| **TOTAL** | ~28 archivos | ~730 líneas | - | - | **22h (~3 días)** |

### Matriz de Riesgo

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| **R-001:** Breaking changes en UI | 🟡 Media | 🔴 Alto | Tests E2E, rollout por fases |
| **R-002:** Regresión en seguridad | 🟢 Baja | 🔴 Crítico | Tests de seguridad exhaustivos |
| **R-003:** Admin global pierde acceso | 🟢 Baja | 🔴 Alto | Validar bypass en middleware |
| **R-004:** Frontend incompatible | 🟡 Media | 🔴 Alto | Desplegar backend primero |
| **R-005:** Rollback complejo | 🟡 Media | 🟡 Medio | Plan de rollback documentado |

---

## 🏗️ FASES DE IMPLEMENTACIÓN

---

## **FASE 1: PREPARACIÓN Y ANÁLISIS**

**Objetivo:** Crear evidencias del estado actual y planificar cambios

**Duración Estimada:** 2 horas

---

### **PASO 1.1: Auditoría del Estado Actual**

**Objetivo:** Documentar todos los lugares donde se resuelve tenant actualmente

**Archivos a Analizar:**
- `internal/http/middlewares/tenant.go`
- `internal/http/router/admin_routes.go`
- `internal/http/controllers/admin/*.go`
- `ui/lib/api.ts`
- `ui/app/(admin)/admin/**/*.tsx`

**Tareas:**

- [ ] **T-1.1.1:** Ejecutar grep para encontrar todos los `PathValue("id")`, `PathValue("tenant")`
  ```bash
  grep -r 'PathValue("id")' internal/http/controllers/ > docs/audit/path_value_id.txt
  grep -r 'PathValue("tenant")' internal/http/controllers/ > docs/audit/path_value_tenant.txt
  ```

- [ ] **T-1.1.2:** Listar todos los resolvers en `tenant.go`
  ```bash
  grep -A 10 'ChainResolvers' internal/http/middlewares/tenant.go > docs/audit/current_resolvers.txt
  ```

- [ ] **T-1.1.3:** Documentar rutas admin actuales
  ```bash
  grep 'mux.Handle.*admin/tenants' internal/http/router/admin_routes.go > docs/audit/current_routes.txt
  ```

- [ ] **T-1.1.4:** Auditar métodos de API en frontend
  ```bash
  grep -r 'tenants/' ui/lib/ > docs/audit/frontend_api_calls.txt
  grep -r 'searchParams.get' ui/app/(admin)/ > docs/audit/frontend_query_params.txt
  ```

**Criterios de Salida (DoD):**

- [x] Archivo `docs/audit/path_value_id.txt` creado
- [x] Archivo `docs/audit/path_value_tenant.txt` creado
- [x] Archivo `docs/audit/current_resolvers.txt` creado
- [x] Archivo `docs/audit/current_routes.txt` creado
- [x] Archivo `docs/audit/frontend_api_calls.txt` creado
- [x] Archivo `docs/audit/frontend_query_params.txt` creado

**Evidencias:**
- [ ] Screenshot de directorios `docs/audit/` con archivos generados
- [ ] Commit en Git: `docs: audit current tenant resolution implementation`

---

### **PASO 1.2: Crear Rama de Desarrollo**

**Objetivo:** Aislar cambios en rama dedicada

**Tareas:**

- [ ] **T-1.2.1:** Crear rama desde `main`
  ```bash
  git checkout main
  git pull origin main
  git checkout -b feature/admin-multi-tenant-standardization
  ```

- [ ] **T-1.2.2:** Pushear rama vacía
  ```bash
  git push -u origin feature/admin-multi-tenant-standardization
  ```

**Criterios de Salida (DoD):**

- [x] Rama `feature/admin-multi-tenant-standardization` creada
- [x] Rama pusheada a origin

**Evidencias:**
- [ ] Screenshot de `git branch -a` mostrando la rama
- [ ] URL de la rama en GitHub/GitLab

---

### **PASO 1.3: Configurar Entorno de Testing**

**Objetivo:** Preparar entorno para ejecutar tests

**Tareas:**

- [ ] **T-1.3.1:** Verificar que el proyecto compila
  ```bash
  go build ./cmd/service
  ```

- [ ] **T-1.3.2:** Ejecutar tests actuales (baseline)
  ```bash
  go test ./... -v > docs/test-results/baseline-go-tests.txt
  ```

- [ ] **T-1.3.3:** Verificar que el frontend compila
  ```bash
  cd ui && npm run build
  ```

- [ ] **T-1.3.4:** Ejecutar tests frontend (baseline)
  ```bash
  cd ui && npm test > ../docs/test-results/baseline-ui-tests.txt
  ```

**Criterios de Salida (DoD):**

- [x] Backend compila sin errores
- [x] Frontend compila sin errores
- [x] Tests baseline ejecutados y documentados

**Evidencias:**
- [ ] Archivo `docs/test-results/baseline-go-tests.txt`
- [ ] Archivo `docs/test-results/baseline-ui-tests.txt`
- [ ] Screenshot de compilación exitosa

---

## **FASE 2: BACKEND - ESTANDARIZACIÓN DE TENANT RESOLUTION**

**Objetivo:** Simplificar middleware de tenant a un único resolver

**Duración Estimada:** 3 horas

---

### **PASO 2.1: Simplificar Middleware de Tenant**

**Archivo:** `internal/http/middlewares/tenant.go`

**Objetivo:** Remover todos los resolvers excepto `PathValueTenantResolver`

**Cambios a Realizar:**

```go
// ANTES (líneas ~100-110)
resolver = ChainResolvers(
    PathValueTenantResolver("id"),
    HeaderTenantResolver("X-Tenant-ID"),
    HeaderTenantResolver("X-Tenant-Slug"),
    QueryTenantResolver("tenant"),
    QueryTenantResolver("tenant_id"),
    SubdomainTenantResolver(),
)

// DESPUÉS
resolver = PathValueTenantResolver("tenant_id")
```

**Tareas:**

- [ ] **T-2.1.1:** Crear backup del archivo original
  ```bash
  cp internal/http/middlewares/tenant.go internal/http/middlewares/tenant.go.backup
  ```

- [ ] **T-2.1.2:** Editar `NewTenantMiddleware()` función (línea ~100)
  - Remover llamada a `ChainResolvers()`
  - Usar solo `PathValueTenantResolver("tenant_id")`

- [ ] **T-2.1.3:** Agregar comentario de documentación
  ```go
  // NewTenantMiddleware crea un middleware de resolución de tenant.
  // ESTÁNDAR: Solo resuelve desde path parameter "tenant_id" en rutas /tenants/{tenant_id}/...
  // Rutas esperadas: /v2/admin/tenants/{tenant_id}/users, /v2/admin/tenants/{tenant_id}/sessions, etc.
  ```

- [ ] **T-2.1.4:** Verificar que compila
  ```bash
  go build ./cmd/service
  ```

- [ ] **T-2.1.5:** Ejecutar tests de middlewares
  ```bash
  go test ./internal/http/middlewares/... -v
  ```

**Criterios de Salida (DoD):**

- [x] Archivo `tenant.go.backup` creado
- [x] Solo un resolver activo: `PathValueTenantResolver("tenant_id")`
- [x] Comentarios de documentación agregados
- [x] Código compila sin errores
- [x] Tests de middlewares pasan

**Evidencias:**
- [ ] Diff del archivo: `git diff internal/http/middlewares/tenant.go > docs/changes/step-2.1-tenant-middleware.diff`
- [ ] Output de compilación exitosa
- [ ] Output de tests: `docs/test-results/step-2.1-middleware-tests.txt`
- [ ] Commit: `refactor(middleware): standardize tenant resolution to path parameter only`

---

### **PASO 2.2: Estandarizar Rutas en Router**

**Archivo:** `internal/http/router/admin_routes.go`

**Objetivo:** Cambiar todos los path parameters a `{tenant_id}`

**Cambios a Realizar:**

```go
// ANTES
mux.Handle("GET /v2/admin/tenants/{id}/users", ...)
mux.Handle("GET /v2/admin/tenants/{tenant}/sessions", ...)

// DESPUÉS
mux.Handle("GET /v2/admin/tenants/{tenant_id}/users", ...)
mux.Handle("GET /v2/admin/tenants/{tenant_id}/sessions", ...)
```

**Tareas:**

- [ ] **T-2.2.1:** Crear backup del archivo
  ```bash
  cp internal/http/router/admin_routes.go internal/http/router/admin_routes.go.backup
  ```

- [ ] **T-2.2.2:** Buscar todas las rutas con `{id}` o `{tenant}`
  ```bash
  grep -n '{id}' internal/http/router/admin_routes.go
  grep -n '{tenant}' internal/http/router/admin_routes.go
  ```

- [ ] **T-2.2.3:** Reemplazar globalmente en el archivo
  - Todas las ocurrencias de `/tenants/{id}/` → `/tenants/{tenant_id}/`
  - Todas las ocurrencias de `/tenants/{tenant}/` → `/tenants/{tenant_id}/`

- [ ] **T-2.2.4:** Listar rutas modificadas
  ```bash
  grep 'tenants/{tenant_id}' internal/http/router/admin_routes.go > docs/changes/step-2.2-routes-list.txt
  ```

- [ ] **T-2.2.5:** Verificar que compila
  ```bash
  go build ./cmd/service
  ```

**Criterios de Salida (DoD):**

- [x] Archivo backup creado
- [x] Todas las rutas usan `{tenant_id}` consistentemente
- [x] Lista de rutas documentada
- [x] Código compila sin errores

**Evidencias:**
- [ ] Diff: `git diff internal/http/router/admin_routes.go > docs/changes/step-2.2-router.diff`
- [ ] Lista de rutas: `docs/changes/step-2.2-routes-list.txt`
- [ ] Commit: `refactor(router): standardize all admin routes to use {tenant_id} parameter`

---

### **PASO 2.3: Actualizar Controllers**

**Archivos Afectados:**
- `internal/http/controllers/admin/users_controller.go`
- `internal/http/controllers/admin/sessions_controller.go`
- `internal/http/controllers/admin/tokens_controller.go`
- `internal/http/controllers/admin/rbac_controller.go`
- `internal/http/controllers/admin/consents_controller.go`
- (Y otros controllers admin)

**Objetivo:** Cambiar `r.PathValue("id")` y `r.PathValue("tenant")` a `r.PathValue("tenant_id")`

**Tareas:**

- [ ] **T-2.3.1:** Buscar todos los controllers que usan PathValue
  ```bash
  grep -rn 'PathValue("id")' internal/http/controllers/admin/ > docs/changes/step-2.3-pathvalue-before.txt
  grep -rn 'PathValue("tenant")' internal/http/controllers/admin/ >> docs/changes/step-2.3-pathvalue-before.txt
  ```

- [ ] **T-2.3.2:** Crear script de reemplazo masivo
  ```bash
  # Crear archivo: scripts/update-controllers.sh
  #!/bin/bash
  find internal/http/controllers/admin/ -name "*.go" -exec sed -i 's/PathValue("id")/PathValue("tenant_id")/g' {} \;
  find internal/http/controllers/admin/ -name "*.go" -exec sed -i 's/PathValue("tenant")/PathValue("tenant_id")/g' {} \;
  ```

- [ ] **T-2.3.3:** Ejecutar script
  ```bash
  chmod +x scripts/update-controllers.sh
  ./scripts/update-controllers.sh
  ```

- [ ] **T-2.3.4:** Verificar cambios
  ```bash
  grep -rn 'PathValue("tenant_id")' internal/http/controllers/admin/ > docs/changes/step-2.3-pathvalue-after.txt
  ```

- [ ] **T-2.3.5:** Revisar manualmente cada archivo modificado
  - [ ] `users_controller.go`
  - [ ] `sessions_controller.go`
  - [ ] `tokens_controller.go`
  - [ ] `rbac_controller.go`
  - [ ] `consents_controller.go`
  - [ ] `scopes_controller.go`
  - [ ] `clients_controller.go`
  - [ ] `claims_controller.go`
  - [ ] `keys_controller.go`
  - [ ] `tenants_controller.go`

- [ ] **T-2.3.6:** Compilar y verificar
  ```bash
  go build ./cmd/service
  ```

- [ ] **T-2.3.7:** Ejecutar tests de controllers
  ```bash
  go test ./internal/http/controllers/admin/... -v > docs/test-results/step-2.3-controller-tests.txt
  ```

**Criterios de Salida (DoD):**

- [x] Todos los controllers usan `PathValue("tenant_id")`
- [x] No quedan referencias a `PathValue("id")` o `PathValue("tenant")` en controllers admin
- [x] Código compila sin errores
- [x] Tests de controllers pasan

**Evidencias:**
- [ ] Archivos: `docs/changes/step-2.3-pathvalue-before.txt` y `step-2.3-pathvalue-after.txt`
- [ ] Diff consolidado: `git diff internal/http/controllers/admin/ > docs/changes/step-2.3-controllers.diff`
- [ ] Output de tests: `docs/test-results/step-2.3-controller-tests.txt`
- [ ] Commit: `refactor(controllers): update all admin controllers to use tenant_id path parameter`

---

### **PASO 2.4: Verificación Integral Backend**

**Objetivo:** Asegurar que todos los cambios funcionan correctamente

**Tareas:**

- [ ] **T-2.4.1:** Compilar proyecto completo
  ```bash
  go build -o hellojohn ./cmd/service
  ```

- [ ] **T-2.4.2:** Ejecutar suite completa de tests
  ```bash
  go test ./... -v -race -coverprofile=docs/test-results/step-2.4-coverage.out
  go tool cover -html=docs/test-results/step-2.4-coverage.out -o docs/test-results/step-2.4-coverage.html
  ```

- [ ] **T-2.4.3:** Ejecutar linter
  ```bash
  golangci-lint run ./... > docs/test-results/step-2.4-lint.txt
  ```

- [ ] **T-2.4.4:** Ejecutar servidor localmente y probar endpoint
  ```bash
  # Terminal 1: Iniciar servidor
  ./hellojohn

  # Terminal 2: Probar endpoint (debe fallar correctamente sin tenant_id)
  curl -X GET http://localhost:8080/v2/admin/tenants/test-tenant/users \
    -H "Authorization: Bearer TOKEN"
  ```

- [ ] **T-2.4.5:** Documentar resultados de pruebas manuales
  - Crear archivo: `docs/test-results/step-2.4-manual-tests.md`
  - Documentar requests y responses

**Criterios de Salida (DoD):**

- [x] Proyecto compila sin warnings
- [x] Tests pasan con >80% coverage
- [x] Linter sin errores críticos
- [x] Servidor inicia correctamente
- [x] Endpoint responde correctamente (con error esperado si no hay auth)

**Evidencias:**
- [ ] Binario compilado: `hellojohn` o `hellojohn.exe`
- [ ] Reporte de coverage: `docs/test-results/step-2.4-coverage.html`
- [ ] Output de linter: `docs/test-results/step-2.4-lint.txt`
- [ ] Documento de pruebas manuales: `docs/test-results/step-2.4-manual-tests.md`
- [ ] Screenshot de servidor corriendo
- [ ] Commit: `test(backend): verify tenant resolution standardization`

---

## **FASE 3: BACKEND - SEGURIDAD MULTI-TENANT ADMIN**

**Objetivo:** Implementar validación de acceso multi-tenant

**Duración Estimada:** 4 horas

---

### **PASO 3.1: Implementar Middleware de Validación de Acceso**

**Archivo:** `internal/http/middlewares/tenant.go`

**Objetivo:** Crear `RequireAdminTenantAccess()` middleware

**Código a Agregar:**

```go
// RequireAdminTenantAccess valida que el admin tenga acceso al tenant del request.
// Debe usarse DESPUÉS de WithTenantResolution y RequireAdminAuth.
//
// Reglas de autorización:
// - Admin Global (admin_type="global"): Acceso a TODOS los tenants
// - Admin Tenant (admin_type="tenant"): Acceso solo a tenants en claims["tenants"][]
//
// Responde:
// - 401 Unauthorized: Si no hay admin claims en contexto
// - 403 Forbidden: Si el admin no tiene acceso al tenant solicitado
// - 200 OK: Si tiene acceso, continúa al siguiente handler
//
// Logging:
// - Registra intentos de acceso denegados para auditoría
func RequireAdminTenantAccess() Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 1. Obtener tenant del contexto (resuelto por WithTenantResolution)
            tda := GetTenant(r.Context())
            if tda == nil {
                // Sin tenant en el request, continuar (ej: /admin/tenants lista)
                next.ServeHTTP(w, r)
                return
            }

            // 2. Obtener admin claims del contexto (seteado por RequireAdminAuth)
            adminClaims := GetAdminClaims(r.Context())
            if adminClaims == nil {
                log.Printf(`{"level":"warn","event":"admin_tenant_access_denied","reason":"no_admin_claims","path":"%s","method":"%s","remote_addr":"%s"}`,
                    r.URL.Path, r.Method, r.RemoteAddr)
                errors.WriteError(w, errors.ErrUnauthorized.WithDetail("admin authentication required"))
                return
            }

            // 3. Admin Global: acceso total (bypass)
            if adminClaims.AdminType == "global" {
                log.Printf(`{"level":"debug","event":"admin_tenant_access_granted","admin_id":"%s","admin_type":"global","tenant":"%s","path":"%s"}`,
                    adminClaims.AdminID, tda.ID(), r.URL.Path)
                next.ServeHTTP(w, r)
                return
            }

            // 4. Admin Tenant: verificar acceso a tenant específico
            tenantID := tda.ID()
            tenantSlug := tda.Slug()

            hasAccess := false
            for _, assignedTenant := range adminClaims.Tenants {
                if assignedTenant == tenantID || assignedTenant == tenantSlug {
                    hasAccess = true
                    break
                }
            }

            if !hasAccess {
                // AUDITORÍA CRÍTICA: Intento de acceso no autorizado
                log.Printf(`{"level":"warn","event":"admin_tenant_access_denied","reason":"tenant_not_assigned","admin_id":"%s","admin_type":"tenant","admin_email":"%s","requested_tenant_id":"%s","requested_tenant_slug":"%s","assigned_tenants":%q,"path":"%s","method":"%s","remote_addr":"%s"}`,
                    adminClaims.AdminID, adminClaims.Email, tenantID, tenantSlug, adminClaims.Tenants, r.URL.Path, r.Method, r.RemoteAddr)
                errors.WriteError(w, errors.ErrForbidden.WithDetail("admin does not have access to this tenant"))
                return
            }

            // 5. Acceso concedido
            log.Printf(`{"level":"debug","event":"admin_tenant_access_granted","admin_id":"%s","admin_type":"tenant","tenant":"%s","path":"%s"}`,
                adminClaims.AdminID, tenantID, r.URL.Path)
            next.ServeHTTP(w, r)
        })
    }
}
```

**Tareas:**

- [ ] **T-3.1.1:** Agregar función al final de `tenant.go` (después de `RequireTenantDB`)

- [ ] **T-3.1.2:** Verificar que `GetAdminClaims()` existe en `context.go`
  ```bash
  grep -n "GetAdminClaims" internal/http/middlewares/context.go
  ```

- [ ] **T-3.1.3:** Si no existe, agregarlo a `context.go`:
  ```go
  // GetAdminClaims obtiene las admin claims del contexto.
  func GetAdminClaims(ctx context.Context) *jwtx.AdminAccessClaims {
      if v := ctx.Value(ctxAdminClaimsKey); v != nil {
          if c, ok := v.(*jwtx.AdminAccessClaims); ok {
              return c
          }
      }
      return nil
  }
  ```

- [ ] **T-3.1.4:** Compilar
  ```bash
  go build ./cmd/service
  ```

- [ ] **T-3.1.5:** Ejecutar tests
  ```bash
  go test ./internal/http/middlewares/... -v
  ```

**Criterios de Salida (DoD):**

- [x] Función `RequireAdminTenantAccess()` implementada
- [x] Logging de auditoría incluido
- [x] Documentación inline completa
- [x] Código compila sin errores
- [x] Tests pasan

**Evidencias:**
- [ ] Diff: `git diff internal/http/middlewares/tenant.go > docs/changes/step-3.1-admin-tenant-access.diff`
- [ ] Commit: `feat(middleware): implement admin tenant access validation`

---

### **PASO 3.2: Integrar Middleware en Cadena Admin**

**Archivo:** `internal/http/router/admin_routes.go`

**Objetivo:** Agregar `RequireAdminTenantAccess()` a la cadena de middlewares

**Cambios a Realizar:**

```go
// Función: adminBaseChain()
// ANTES
func adminBaseChain(dal store.DataAccessLayer, issuer *jwtx.Issuer, limiter mw.RateLimiter, requireDB bool) []mw.Middleware {
    chain := []mw.Middleware{
        mw.WithRecover(),
        mw.WithRequestID(),
        mw.WithSecurityHeaders(),
        mw.WithNoStore(),
        mw.WithTenantResolution(dal, false),
        mw.RequireAdminAuth(issuer),
        mw.WithRateLimit(limiter),
    }
    if requireDB {
        chain = append(chain, mw.RequireTenantDB())
    }
    return chain
}

// DESPUÉS
func adminBaseChain(dal store.DataAccessLayer, issuer *jwtx.Issuer, limiter mw.RateLimiter, requireDB bool) []mw.Middleware {
    chain := []mw.Middleware{
        mw.WithRecover(),                    // 1. Panic recovery
        mw.WithRequestID(),                  // 2. Request tracing
        mw.WithSecurityHeaders(),            // 3. Security headers (CORS, CSP)
        mw.WithNoStore(),                    // 4. Cache control

        // TENANT RESOLUTION
        mw.WithTenantResolution(dal, false), // 5. Extrae tenant del path {tenant_id}

        // AUTHENTICATION
        mw.RequireAdminAuth(issuer),         // 6. Valida JWT y setea AdminClaims

        // AUTHORIZATION
        mw.RequireAdminTenantAccess(),       // 7. ← NUEVO: Valida acceso al tenant

        // RATE LIMITING
        mw.WithRateLimit(limiter),           // 8. Rate limiting por IP
    }

    // Opcionalmente verificar que el tenant tenga DB
    if requireDB {
        chain = append(chain, mw.RequireTenantDB())
    }

    return chain
}
```

**Tareas:**

- [ ] **T-3.2.1:** Editar función `adminBaseChain()` en `admin_routes.go`

- [ ] **T-3.2.2:** Agregar `mw.RequireAdminTenantAccess()` después de `RequireAdminAuth()`

- [ ] **T-3.2.3:** Agregar comentarios explicativos

- [ ] **T-3.2.4:** Compilar
  ```bash
  go build ./cmd/service
  ```

- [ ] **T-3.2.5:** Verificar orden de middlewares con test manual
  ```bash
  # Iniciar servidor
  ./hellojohn

  # Probar sin auth (debe dar 401)
  curl -X GET http://localhost:8080/v2/admin/tenants/test-tenant/users

  # Probar con JWT de tenant incorrecto (debe dar 403)
  curl -X GET http://localhost:8080/v2/admin/tenants/other-tenant/users \
    -H "Authorization: Bearer TOKEN_TENANT_A"
  ```

**Criterios de Salida (DoD):**

- [x] Middleware agregado en posición correcta (después de auth, antes de rate limit)
- [x] Comentarios de documentación agregados
- [x] Código compila sin errores
- [x] Orden de ejecución validado

**Evidencias:**
- [ ] Diff: `git diff internal/http/router/admin_routes.go > docs/changes/step-3.2-admin-chain.diff`
- [ ] Documento de pruebas manuales: `docs/test-results/step-3.2-middleware-order.md`
- [ ] Commit: `feat(router): integrate admin tenant access validation in middleware chain`

---

### **PASO 3.3: Verificar Emisión de AdminClaims en JWT**

**Archivo:** `internal/http/services/admin/auth_service.go`

**Objetivo:** Asegurar que el JWT de admin incluye `admin_type` y `tenants[]`

**Tareas:**

- [ ] **T-3.3.1:** Revisar función de login de admin
  ```bash
  grep -A 50 "func.*Login" internal/http/services/admin/auth_service.go
  ```

- [ ] **T-3.3.2:** Verificar que se incluyen los claims correctos al emitir JWT
  - Buscar donde se llama a `issuer.IssueToken()` o similar
  - Verificar que se incluyen:
    - `sub`: Admin ID
    - `email`: Email del admin
    - `admin_type`: "global" o "tenant"
    - `tenants`: Array de tenant IDs (solo si admin_type="tenant")
    - `aud`: "hellojohn:admin"

- [ ] **T-3.3.3:** Si los claims no están completos, actualizar código
  ```go
  // Ejemplo de claims correctos
  std := map[string]any{
      "email":      admin.Email,
      "admin_type": string(admin.Type),
      "aud":        "hellojohn:admin",
  }
  if admin.Type == repository.AdminTypeTenant {
      std["tenants"] = admin.AssignedTenants
  }

  token, exp, err := s.deps.Issuer.IssueToken(admin.ID, std, nil)
  ```

- [ ] **T-3.3.4:** Compilar y probar login
  ```bash
  go build ./cmd/service
  ./hellojohn

  # Hacer login y decodificar JWT
  curl -X POST http://localhost:8080/v2/admin/login \
    -H "Content-Type: application/json" \
    -d '{"email":"admin@example.com","password":"password"}' \
    | jq -r '.access_token' \
    | cut -d. -f2 \
    | base64 -d \
    | jq .
  ```

- [ ] **T-3.3.5:** Verificar estructura del JWT decodificado
  - Debe contener: `admin_type`, `tenants[]` (si tenant admin), `aud`

**Criterios de Salida (DoD):**

- [x] JWT de admin incluye todos los claims requeridos
- [x] Admin Global NO tiene campo `tenants[]` o está vacío
- [x] Admin Tenant tiene campo `tenants[]` con lista de IDs
- [x] Campo `aud` es "hellojohn:admin"

**Evidencias:**
- [ ] JWT decodificado de admin global: `docs/test-results/step-3.3-jwt-global.json`
- [ ] JWT decodificado de admin tenant: `docs/test-results/step-3.3-jwt-tenant.json`
- [ ] Diff (si hubo cambios): `git diff internal/http/services/admin/ > docs/changes/step-3.3-admin-jwt.diff`
- [ ] Commit (si hubo cambios): `fix(admin): ensure JWT includes admin_type and tenants claims`

---

### **PASO 3.4: Crear Tests de Seguridad**

**Archivo Nuevo:** `internal/http/middlewares/tenant_security_test.go`

**Objetivo:** Tests exhaustivos de validación de acceso multi-tenant

**Código a Implementar:**

```go
package middlewares_test

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/dropDatabas3/hellojohn/internal/http/middlewares"
    jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
    "github.com/dropDatabas3/hellojohn/internal/store"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// TestRequireAdminTenantAccess_GlobalAdmin verifica que admin global tiene acceso a todos los tenants
func TestRequireAdminTenantAccess_GlobalAdmin(t *testing.T) {
    // Setup
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })

    middleware := middlewares.RequireAdminTenantAccess()
    wrappedHandler := middleware(handler)

    // Mock TenantDataAccess
    mockTDA := &store.MockTenantDataAccess{
        IDFunc:   func() string { return "tenant-a" },
        SlugFunc: func() string { return "tenant-a" },
    }

    // Mock AdminClaims (GLOBAL)
    adminClaims := &jwtx.AdminAccessClaims{
        AdminID:   "admin-global-1",
        Email:     "global@example.com",
        AdminType: "global",
        Tenants:   nil, // Global admin no tiene lista de tenants
    }

    // Test
    req := httptest.NewRequest("GET", "/v2/admin/tenants/tenant-a/users", nil)
    ctx := req.Context()
    ctx = middlewares.WithTenant(ctx, mockTDA)
    ctx = middlewares.SetAdminClaims(ctx, adminClaims)
    req = req.WithContext(ctx)

    rec := httptest.NewRecorder()
    wrappedHandler.ServeHTTP(rec, req)

    // Assert
    assert.Equal(t, http.StatusOK, rec.Code, "Global admin should have access")
}

// TestRequireAdminTenantAccess_TenantAdmin_Allowed verifica que tenant admin puede acceder a sus tenants
func TestRequireAdminTenantAccess_TenantAdmin_Allowed(t *testing.T) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })

    middleware := middlewares.RequireAdminTenantAccess()
    wrappedHandler := middleware(handler)

    mockTDA := &store.MockTenantDataAccess{
        IDFunc:   func() string { return "tenant-a" },
        SlugFunc: func() string { return "tenant-a" },
    }

    // Admin con acceso a tenant-a y tenant-b
    adminClaims := &jwtx.AdminAccessClaims{
        AdminID:   "admin-tenant-1",
        Email:     "tenant-admin@example.com",
        AdminType: "tenant",
        Tenants:   []string{"tenant-a", "tenant-b"},
    }

    req := httptest.NewRequest("GET", "/v2/admin/tenants/tenant-a/users", nil)
    ctx := req.Context()
    ctx = middlewares.WithTenant(ctx, mockTDA)
    ctx = middlewares.SetAdminClaims(ctx, adminClaims)
    req = req.WithContext(ctx)

    rec := httptest.NewRecorder()
    wrappedHandler.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code, "Tenant admin should have access to assigned tenant")
}

// TestRequireAdminTenantAccess_TenantAdmin_Forbidden verifica que tenant admin NO puede acceder a otros tenants
func TestRequireAdminTenantAccess_TenantAdmin_Forbidden(t *testing.T) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })

    middleware := middlewares.RequireAdminTenantAccess()
    wrappedHandler := middleware(handler)

    // Request a tenant-c (NO asignado)
    mockTDA := &store.MockTenantDataAccess{
        IDFunc:   func() string { return "tenant-c" },
        SlugFunc: func() string { return "tenant-c" },
    }

    // Admin con acceso solo a tenant-a y tenant-b
    adminClaims := &jwtx.AdminAccessClaims{
        AdminID:   "admin-tenant-1",
        Email:     "tenant-admin@example.com",
        AdminType: "tenant",
        Tenants:   []string{"tenant-a", "tenant-b"},
    }

    req := httptest.NewRequest("GET", "/v2/admin/tenants/tenant-c/users", nil)
    ctx := req.Context()
    ctx = middlewares.WithTenant(ctx, mockTDA)
    ctx = middlewares.SetAdminClaims(ctx, adminClaims)
    req = req.WithContext(ctx)

    rec := httptest.NewRecorder()
    wrappedHandler.ServeHTTP(rec, req)

    // Assert: Debe devolver 403 Forbidden
    assert.Equal(t, http.StatusForbidden, rec.Code, "Tenant admin should NOT have access to unassigned tenant")
}

// TestRequireAdminTenantAccess_NoAdminClaims verifica que sin admin claims devuelve 401
func TestRequireAdminTenantAccess_NoAdminClaims(t *testing.T) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    middleware := middlewares.RequireAdminTenantAccess()
    wrappedHandler := middleware(handler)

    mockTDA := &store.MockTenantDataAccess{
        IDFunc:   func() string { return "tenant-a" },
        SlugFunc: func() string { return "tenant-a" },
    }

    req := httptest.NewRequest("GET", "/v2/admin/tenants/tenant-a/users", nil)
    ctx := middlewares.WithTenant(req.Context(), mockTDA)
    // NO se setean AdminClaims
    req = req.WithContext(ctx)

    rec := httptest.NewRecorder()
    wrappedHandler.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusUnauthorized, rec.Code, "Should return 401 without admin claims")
}

// TestRequireAdminTenantAccess_NoTenant verifica que sin tenant en contexto continúa (bypass)
func TestRequireAdminTenantAccess_NoTenant(t *testing.T) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    middleware := middlewares.RequireAdminTenantAccess()
    wrappedHandler := middleware(handler)

    // Sin tenant en contexto (ej: /v2/admin/tenants lista)
    req := httptest.NewRequest("GET", "/v2/admin/tenants", nil)
    rec := httptest.NewRecorder()
    wrappedHandler.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code, "Should bypass validation when no tenant in context")
}
```

**Tareas:**

- [ ] **T-3.4.1:** Crear archivo `tenant_security_test.go`

- [ ] **T-3.4.2:** Implementar los 5 test cases principales:
  - [ ] `TestRequireAdminTenantAccess_GlobalAdmin`
  - [ ] `TestRequireAdminTenantAccess_TenantAdmin_Allowed`
  - [ ] `TestRequireAdminTenantAccess_TenantAdmin_Forbidden`
  - [ ] `TestRequireAdminTenantAccess_NoAdminClaims`
  - [ ] `TestRequireAdminTenantAccess_NoTenant`

- [ ] **T-3.4.3:** Crear mocks si no existen
  - Verificar si existe `store.MockTenantDataAccess`
  - Si no, crear en `internal/store/mocks.go`

- [ ] **T-3.4.4:** Ejecutar tests
  ```bash
  go test ./internal/http/middlewares/ -v -run TestRequireAdminTenantAccess
  ```

- [ ] **T-3.4.5:** Verificar coverage
  ```bash
  go test ./internal/http/middlewares/ -coverprofile=docs/test-results/step-3.4-coverage.out
  go tool cover -html=docs/test-results/step-3.4-coverage.out -o docs/test-results/step-3.4-coverage.html
  ```

**Criterios de Salida (DoD):**

- [x] 5 test cases implementados
- [x] Todos los tests pasan
- [x] Coverage >90% de `RequireAdminTenantAccess()`
- [x] Tests documentados con comentarios claros

**Evidencias:**
- [ ] Archivo de tests: `internal/http/middlewares/tenant_security_test.go`
- [ ] Output de tests: `docs/test-results/step-3.4-security-tests.txt`
- [ ] Coverage report: `docs/test-results/step-3.4-coverage.html`
- [ ] Commit: `test(middleware): add comprehensive security tests for admin tenant access`

---

### **PASO 3.5: Tests de Integración End-to-End**

**Archivo Nuevo:** `test/integration/admin_multi_tenant_test.go`

**Objetivo:** Tests de integración completos simulando requests reales

**Tareas:**

- [ ] **T-3.5.1:** Crear directorio de tests de integración
  ```bash
  mkdir -p test/integration
  ```

- [ ] **T-3.5.2:** Crear archivo `admin_multi_tenant_test.go`

- [ ] **T-3.5.3:** Implementar test de admin global
  ```go
  func TestIntegration_GlobalAdmin_AccessAllTenants(t *testing.T) {
      // 1. Setup: Crear admin global en FS
      // 2. Login: Obtener JWT
      // 3. Test: Hacer requests a múltiples tenants
      // 4. Assert: Todos devuelven 200 OK
  }
  ```

- [ ] **T-3.5.4:** Implementar test de admin tenant (acceso permitido)
  ```go
  func TestIntegration_TenantAdmin_AccessAssignedTenants(t *testing.T) {
      // 1. Setup: Crear admin tenant con tenants=[A, B]
      // 2. Login: Obtener JWT
      // 3. Test: Hacer requests a tenant-A y tenant-B
      // 4. Assert: Ambos devuelven 200 OK
  }
  ```

- [ ] **T-3.5.5:** Implementar test de admin tenant (acceso denegado)
  ```go
  func TestIntegration_TenantAdmin_DeniedUnassignedTenant(t *testing.T) {
      // 1. Setup: Crear admin tenant con tenants=[A, B]
      // 2. Login: Obtener JWT
      // 3. Test: Hacer request a tenant-C (no asignado)
      // 4. Assert: Devuelve 403 Forbidden
  }
  ```

- [ ] **T-3.5.6:** Ejecutar tests de integración
  ```bash
  go test ./test/integration/ -v -tags=integration > docs/test-results/step-3.5-integration.txt
  ```

**Criterios de Salida (DoD):**

- [x] 3 tests de integración implementados
- [x] Tests usan servidor HTTP real (httptest.Server)
- [x] Tests crean admins en FileSystem temporal
- [x] Todos los tests pasan

**Evidencias:**
- [ ] Archivo: `test/integration/admin_multi_tenant_test.go`
- [ ] Output: `docs/test-results/step-3.5-integration.txt`
- [ ] Commit: `test(integration): add end-to-end tests for multi-tenant admin access`

---

## **FASE 4: FRONTEND - ESTANDARIZACIÓN Y MIGRACIÓN**

**Objetivo:** Migrar frontend a path parameters estandarizados

**Duración Estimada:** 8 horas

---

### **PASO 4.1: Reestructurar Rutas en Next.js**

**Objetivo:** Mover páginas a estructura `/tenants/[tenant_id]/...`

**Estructura Actual:**
```
ui/app/(admin)/admin/tenants/
├── users/page.tsx          → Usa searchParams.get("id")
├── sessions/page.tsx       → Usa searchParams.get("id")
├── tokens/page.tsx         → Usa searchParams.get("id")
└── ...
```

**Estructura Objetivo:**
```
ui/app/(admin)/admin/tenants/
├── [tenant_id]/            ← Dynamic route segment
│   ├── users/
│   │   └── page.tsx
│   ├── sessions/
│   │   └── page.tsx
│   ├── tokens/
│   │   └── page.tsx
│   ├── rbac/
│   │   └── page.tsx
│   ├── settings/
│   │   └── page.tsx
│   └── layout.tsx          ← Optional: shared layout
└── page.tsx                ← Lista de tenants
```

**Tareas:**

- [ ] **T-4.1.1:** Crear backup del directorio actual
  ```bash
  cp -r ui/app/\(admin\)/admin/tenants ui/app/\(admin\)/admin/tenants.backup
  ```

- [ ] **T-4.1.2:** Crear directorio `[tenant_id]`
  ```bash
  mkdir -p ui/app/\(admin\)/admin/tenants/\[tenant_id\]
  ```

- [ ] **T-4.1.3:** Mover páginas a nuevo directorio
  ```bash
  mv ui/app/\(admin\)/admin/tenants/users ui/app/\(admin\)/admin/tenants/\[tenant_id\]/
  mv ui/app/\(admin\)/admin/tenants/sessions ui/app/\(admin\)/admin/tenants/\[tenant_id\]/
  mv ui/app/\(admin\)/admin/tenants/tokens ui/app/\(admin\)/admin/tenants/\[tenant_id\]/
  mv ui/app/\(admin\)/admin/tenants/rbac ui/app/\(admin\)/admin/tenants/\[tenant_id\]/
  mv ui/app/\(admin\)/admin/tenants/settings ui/app/\(admin\)/admin/tenants/\[tenant_id\]/
  # ... mover otras páginas
  ```

- [ ] **T-4.1.4:** Listar páginas movidas
  ```bash
  ls -la ui/app/\(admin\)/admin/tenants/\[tenant_id\]/ > docs/changes/step-4.1-moved-pages.txt
  ```

**Criterios de Salida (DoD):**

- [x] Directorio `[tenant_id]` creado
- [x] Todas las páginas tenant-scoped movidas
- [x] Backup del directorio original creado
- [x] Lista de páginas movidas documentada

**Evidencias:**
- [ ] Lista de páginas: `docs/changes/step-4.1-moved-pages.txt`
- [ ] Screenshot de estructura de directorios
- [ ] Commit: `refactor(ui): restructure tenant pages to use dynamic route segment`

---

### **PASO 4.2: Actualizar Páginas para Usar `useParams()`**

**Objetivo:** Cambiar de `searchParams.get("id")` a `params.tenant_id`

**Archivo Ejemplo:** `ui/app/(admin)/admin/tenants/[tenant_id]/users/page.tsx`

**Cambios:**

```tsx
// ANTES
import { useSearchParams } from 'next/navigation'

export default function UsersPage() {
  const searchParams = useSearchParams()
  const tenantId = searchParams.get("id")

  // ...
}

// DESPUÉS
import { useParams } from 'next/navigation'

export default function UsersPage() {
  const params = useParams()
  const tenantId = params.tenant_id as string

  // ...
}
```

**Tareas:**

- [ ] **T-4.2.1:** Listar todos los archivos que usan `searchParams`
  ```bash
  grep -r "searchParams.get" ui/app/\(admin\)/admin/tenants/\[tenant_id\]/ > docs/changes/step-4.2-searchparams-usage.txt
  ```

- [ ] **T-4.2.2:** Actualizar cada página:
  - [ ] `users/page.tsx`
  - [ ] `sessions/page.tsx`
  - [ ] `tokens/page.tsx`
  - [ ] `rbac/page.tsx`
  - [ ] `settings/page.tsx`
  - [ ] `consents/page.tsx`
  - [ ] `scopes/page.tsx`
  - [ ] `clients/page.tsx`
  - [ ] `claims/page.tsx`
  - [ ] `mailing/page.tsx`

  Para cada archivo:
  1. Cambiar import: `useSearchParams` → `useParams`
  2. Cambiar obtención: `searchParams.get("id")` → `params.tenant_id`
  3. Agregar type assertion: `as string`

- [ ] **T-4.2.3:** Verificar que compila
  ```bash
  cd ui && npm run build
  ```

- [ ] **T-4.2.4:** Ejecutar linter
  ```bash
  cd ui && npm run lint > ../docs/test-results/step-4.2-lint.txt
  ```

**Criterios de Salida (DoD):**

- [x] Todas las páginas usan `useParams()`
- [x] No quedan referencias a `searchParams.get("id")`
- [x] Código compila sin errores
- [x] Linter sin errores

**Evidencias:**
- [ ] Lista de cambios: `docs/changes/step-4.2-searchparams-usage.txt`
- [ ] Diff consolidado: `git diff ui/app/\(admin\)/admin/tenants/ > docs/changes/step-4.2-pages-diff.txt`
- [ ] Output de lint: `docs/test-results/step-4.2-lint.txt`
- [ ] Commit: `refactor(ui): update tenant pages to use useParams instead of searchParams`

---

### **PASO 4.3: Centralizar API Client**

**Archivo:** `ui/lib/api.ts` (o `ui/lib/admin-api.ts` si se crea nuevo)

**Objetivo:** Crear cliente HTTP centralizado para requests admin

**Código a Implementar:**

```typescript
// ui/lib/admin-api.ts

interface AdminAPIConfig {
  baseURL?: string
  getToken?: () => string | null
}

class AdminAPIClient {
  private baseURL: string
  private getToken: () => string | null

  constructor(config: AdminAPIConfig = {}) {
    this.baseURL = config.baseURL || process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
    this.getToken = config.getToken || (() => {
      if (typeof window !== 'undefined') {
        return localStorage.getItem('admin_access_token')
      }
      return null
    })
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const token = this.getToken()
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    }

    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    const response = await fetch(`${this.baseURL}${path}`, {
      ...options,
      headers,
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: response.statusText }))
      throw new Error(error.message || `HTTP ${response.status}`)
    }

    return response.json()
  }

  // Helper para construir rutas tenant-scoped
  private tenantPath(tenantId: string, path: string): string {
    return `/v2/admin/tenants/${tenantId}${path}`
  }

  // ==================== TENANT-SCOPED METHODS ====================

  // Users
  async getTenantUsers(tenantId: string, filters?: UserFilters) {
    const query = filters ? `?${new URLSearchParams(filters as any).toString()}` : ''
    return this.request(this.tenantPath(tenantId, `/users${query}`))
  }

  async createTenantUser(tenantId: string, data: CreateUserRequest) {
    return this.request(this.tenantPath(tenantId, '/users'), {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }

  // Sessions
  async getTenantSessions(tenantId: string) {
    return this.request(this.tenantPath(tenantId, '/sessions'))
  }

  async getTenantSessionStats(tenantId: string) {
    return this.request(this.tenantPath(tenantId, '/sessions/stats'))
  }

  async revokeSession(tenantId: string, sessionId: string) {
    return this.request(this.tenantPath(tenantId, `/sessions/${sessionId}/revoke`), {
      method: 'POST',
    })
  }

  // Tokens
  async getTenantTokens(tenantId: string, filters?: TokenFilters) {
    const query = filters ? `?${new URLSearchParams(filters as any).toString()}` : ''
    return this.request(this.tenantPath(tenantId, `/tokens${query}`))
  }

  async getTenantTokenStats(tenantId: string) {
    return this.request(this.tenantPath(tenantId, '/tokens/stats'))
  }

  async revokeToken(tenantId: string, tokenId: string) {
    return this.request(this.tenantPath(tenantId, `/tokens/${tokenId}`), {
      method: 'DELETE',
    })
  }

  // ... más métodos para otros recursos

  // ==================== NON-TENANT METHODS ====================

  // Admin Auth
  async login(email: string, password: string) {
    return this.request<AdminLoginResult>('/v2/admin/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    })
  }

  async refresh(refreshToken: string) {
    return this.request<AdminLoginResult>('/v2/admin/refresh', {
      method: 'POST',
      body: JSON.stringify({ refresh_token: refreshToken }),
    })
  }

  // Tenants list (no tenant-scoped)
  async listTenants() {
    return this.request('/v2/admin/tenants')
  }
}

// Export singleton instance
export const adminAPI = new AdminAPIClient()

// Export class for custom instances
export { AdminAPIClient }
```

**Tareas:**

- [ ] **T-4.3.1:** Crear archivo `ui/lib/admin-api.ts`

- [ ] **T-4.3.2:** Implementar clase `AdminAPIClient` con métodos principales:
  - [ ] Constructor con config
  - [ ] Método privado `request()`
  - [ ] Helper `tenantPath()`
  - [ ] Métodos de users (get, create)
  - [ ] Métodos de sessions (get, stats, revoke)
  - [ ] Métodos de tokens (get, stats, revoke)
  - [ ] Métodos de RBAC
  - [ ] Métodos de consents
  - [ ] Métodos de auth (login, refresh)
  - [ ] Métodos de tenants (list)

- [ ] **T-4.3.3:** Exportar instancia singleton `adminAPI`

- [ ] **T-4.3.4:** Crear archivo de tipos: `ui/lib/admin-api-types.ts`
  ```typescript
  export interface UserFilters {
    page?: number
    limit?: number
    search?: string
  }

  export interface CreateUserRequest {
    email: string
    password: string
    name?: string
  }

  // ... más tipos
  ```

- [ ] **T-4.3.5:** Verificar que compila
  ```bash
  cd ui && npm run build
  ```

**Criterios de Salida (DoD):**

- [x] Archivo `admin-api.ts` creado
- [x] Clase `AdminAPIClient` implementada
- [x] Al menos 10 métodos principales implementados
- [x] Tipos definidos en archivo separado
- [x] Singleton exportado
- [x] Código compila sin errores

**Evidencias:**
- [ ] Archivo: `ui/lib/admin-api.ts`
- [ ] Archivo: `ui/lib/admin-api-types.ts`
- [ ] Commit: `feat(ui): create centralized admin API client with tenant-scoped methods`

---

### **PASO 4.4: Migrar Páginas a Usar API Centralizado**

**Objetivo:** Reemplazar fetches directos por métodos del `adminAPI`

**Ejemplo de Migración:**

```tsx
// ANTES
const UsersPage = () => {
  const params = useParams()
  const tenantId = params.tenant_id as string

  const { data: users } = useQuery({
    queryKey: ['users', tenantId],
    queryFn: async () => {
      const res = await fetch(`http://localhost:8080/v2/admin/tenants/${tenantId}/users`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      })
      return res.json()
    },
  })

  // ...
}

// DESPUÉS
import { adminAPI } from '@/lib/admin-api'

const UsersPage = () => {
  const params = useParams()
  const tenantId = params.tenant_id as string

  const { data: users } = useQuery({
    queryKey: ['users', tenantId],
    queryFn: () => adminAPI.getTenantUsers(tenantId),
  })

  // ...
}
```

**Tareas:**

- [ ] **T-4.4.1:** Listar todos los fetches directos
  ```bash
  grep -r "fetch.*tenants" ui/app/\(admin\)/admin/tenants/ > docs/changes/step-4.4-direct-fetches.txt
  ```

- [ ] **T-4.4.2:** Migrar página por página:
  - [ ] `users/page.tsx`
    - GET users → `adminAPI.getTenantUsers()`
    - POST user → `adminAPI.createTenantUser()`

  - [ ] `sessions/page.tsx`
    - GET sessions → `adminAPI.getTenantSessions()`
    - GET stats → `adminAPI.getTenantSessionStats()`
    - POST revoke → `adminAPI.revokeSession()`

  - [ ] `tokens/page.tsx`
    - GET tokens → `adminAPI.getTenantTokens()`
    - GET stats → `adminAPI.getTenantTokenStats()`
    - DELETE token → `adminAPI.revokeToken()`

  - [ ] `rbac/page.tsx`
  - [ ] `settings/page.tsx`
  - [ ] `consents/page.tsx`
  - [ ] (Otras páginas)

- [ ] **T-4.4.3:** Actualizar imports en cada página
  ```typescript
  import { adminAPI } from '@/lib/admin-api'
  ```

- [ ] **T-4.4.4:** Verificar que no quedan fetches directos
  ```bash
  grep -r "fetch.*v2/admin" ui/app/\(admin\)/admin/tenants/ | grep -v "admin-api.ts"
  ```

- [ ] **T-4.4.5:** Compilar y verificar
  ```bash
  cd ui && npm run build
  ```

**Criterios de Salida (DoD):**

- [x] Todas las páginas usan `adminAPI`
- [x] No quedan fetches directos en páginas (solo en `admin-api.ts`)
- [x] Código compila sin errores
- [x] TypeScript sin errores de tipos

**Evidencias:**
- [ ] Lista de fetches: `docs/changes/step-4.4-direct-fetches.txt`
- [ ] Diff: `git diff ui/app/\(admin\)/admin/tenants/ > docs/changes/step-4.4-pages-api-migration.diff`
- [ ] Commit: `refactor(ui): migrate tenant pages to use centralized adminAPI client`

---

### **PASO 4.5: Actualizar Navegación y Links**

**Objetivo:** Cambiar todos los links internos a formato `/tenants/{tenant_id}/...`

**Archivos Afectados:**
- Sidebar / Navigation components
- Breadcrumbs
- Links en páginas
- Redirects

**Ejemplo de Cambio:**

```tsx
// ANTES
<Link href={`/admin/tenants/users?id=${tenantId}`}>
  Users
</Link>

// DESPUÉS
<Link href={`/admin/tenants/${tenantId}/users`}>
  Users
</Link>
```

**Tareas:**

- [ ] **T-4.5.1:** Buscar todos los links con query params
  ```bash
  grep -r "href.*tenants.*\\?id=" ui/components/ > docs/changes/step-4.5-links-before.txt
  grep -r "href.*tenants.*\\?id=" ui/app/\(admin\)/ >> docs/changes/step-4.5-links-before.txt
  ```

- [ ] **T-4.5.2:** Actualizar componente de navegación (si existe)
  - Ejemplo: `ui/components/admin/TenantNav.tsx`
  - Cambiar formato de URLs

- [ ] **T-4.5.3:** Actualizar breadcrumbs (si existe)
  - Ejemplo: `ui/components/admin/Breadcrumbs.tsx`

- [ ] **T-4.5.4:** Buscar y reemplazar en todos los componentes
  ```bash
  # Crear script de reemplazo
  find ui/components/ -name "*.tsx" -exec sed -i 's|/tenants/\([^/]*\)?id=\${tenantId}|/tenants/${tenantId}/\1|g' {} \;
  find ui/app/\(admin\)/ -name "*.tsx" -exec sed -i 's|/tenants/\([^/]*\)?id=\${tenantId}|/tenants/${tenantId}/\1|g' {} \;
  ```

- [ ] **T-4.5.5:** Revisar manualmente links importantes:
  - [ ] Sidebar navigation
  - [ ] Tenant selector/switcher
  - [ ] Tenant detail page
  - [ ] Quick actions/shortcuts

- [ ] **T-4.5.6:** Verificar que compila
  ```bash
  cd ui && npm run build
  ```

**Criterios de Salida (DoD):**

- [x] Todos los links usan path parameters
- [x] No quedan links con `?id=` en tenant routes
- [x] Navegación funciona correctamente
- [x] Código compila sin errores

**Evidencias:**
- [ ] Lista de links antes: `docs/changes/step-4.5-links-before.txt`
- [ ] Lista de links después: `docs/changes/step-4.5-links-after.txt`
- [ ] Diff: `git diff ui/ > docs/changes/step-4.5-navigation.diff`
- [ ] Commit: `refactor(ui): update all navigation links to use path parameters`

---

### **PASO 4.6: Testing Frontend**

**Objetivo:** Verificar que todas las páginas cargan y funcionan correctamente

**Tareas:**

- [ ] **T-4.6.1:** Iniciar servidor de desarrollo
  ```bash
  cd ui && npm run dev
  ```

- [ ] **T-4.6.2:** Iniciar backend
  ```bash
  ./hellojohn
  ```

- [ ] **T-4.6.3:** Login como admin global
  - Navegar a `/admin/login`
  - Hacer login
  - Verificar que redirecciona a dashboard

- [ ] **T-4.6.4:** Probar navegación a cada página tenant:
  - [ ] `/admin/tenants/{tenant_id}/users`
  - [ ] `/admin/tenants/{tenant_id}/sessions`
  - [ ] `/admin/tenants/{tenant_id}/tokens`
  - [ ] `/admin/tenants/{tenant_id}/rbac`
  - [ ] `/admin/tenants/{tenant_id}/settings`
  - [ ] `/admin/tenants/{tenant_id}/consents`
  - [ ] `/admin/tenants/{tenant_id}/scopes`
  - [ ] `/admin/tenants/{tenant_id}/clients`

- [ ] **T-4.6.5:** Verificar que datos cargan correctamente
  - Abrir Network tab en DevTools
  - Verificar que requests van a `/v2/admin/tenants/{tenant_id}/...`
  - Verificar que responses tienen datos

- [ ] **T-4.6.6:** Probar acciones CRUD
  - Crear un user
  - Editar un user
  - Eliminar un user
  - Revocar una sesión
  - Revocar un token

- [ ] **T-4.6.7:** Documentar resultados en archivo
  ```markdown
  # Manual Testing Results - Frontend

  ## Test Environment
  - Browser: Chrome 120
  - Backend: localhost:8080
  - Frontend: localhost:3000

  ## Test Cases

  ### TC-4.6.1: Navigation to Users Page
  - URL: /admin/tenants/test-tenant/users
  - Result: ✅ Page loads
  - Data: ✅ Users list displayed
  - API Call: ✅ GET /v2/admin/tenants/test-tenant/users

  ### TC-4.6.2: Create User
  - Action: Click "Create User" button
  - Result: ✅ Modal opens
  - Action: Fill form and submit
  - Result: ✅ User created
  - API Call: ✅ POST /v2/admin/tenants/test-tenant/users

  # ... más test cases
  ```

- [ ] **T-4.6.8:** Crear archivo: `docs/test-results/step-4.6-frontend-manual-tests.md`

**Criterios de Salida (DoD):**

- [x] Todas las páginas cargan sin errores
- [x] Datos se muestran correctamente
- [x] Navegación funciona entre páginas
- [x] Acciones CRUD funcionan
- [x] No hay errores en console
- [x] Todas las requests usan formato correcto

**Evidencias:**
- [ ] Documento: `docs/test-results/step-4.6-frontend-manual-tests.md`
- [ ] Screenshots de cada página funcionando
- [ ] Video de navegación (opcional)
- [ ] Commit: `test(ui): verify frontend migration with manual testing`

---

## **FASE 5: TESTING INTEGRAL**

**Objetivo:** Suite completa de tests de seguridad y funcionalidad

**Duración Estimada:** 3 horas

---

### **PASO 5.1: Tests de Seguridad - Tenant Elevation Attack**

**Objetivo:** Verificar que NO es posible acceder a tenants no autorizados

**Archivo Nuevo:** `test/security/tenant_elevation_test.go`

**Tareas:**

- [ ] **T-5.1.1:** Crear directorio
  ```bash
  mkdir -p test/security
  ```

- [ ] **T-5.1.2:** Crear archivo de test con escenarios de ataque

```go
package security_test

import (
    "testing"
    "net/http/httptest"
    "github.com/stretchr/testify/assert"
)

// TestTenantElevationAttack_PathParameter verifica que no se puede cambiar tenant en path
func TestTenantElevationAttack_PathParameter(t *testing.T) {
    // Setup: Admin de tenant-A intenta acceder a tenant-B

    // 1. Crear admin tenant con acceso solo a tenant-A
    adminToken := createTenantAdminJWT(t, []string{"tenant-a"})

    // 2. Intentar acceder a tenant-B (ATAQUE)
    req := httptest.NewRequest("GET", "/v2/admin/tenants/tenant-b/users", nil)
    req.Header.Set("Authorization", "Bearer "+adminToken)

    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)

    // 3. Assert: Debe devolver 403 Forbidden
    assert.Equal(t, 403, rec.Code, "Should deny access to unassigned tenant")

    // 4. Verificar log de auditoría
    assertAuditLog(t, "admin_tenant_access_denied", "tenant_not_assigned")
}

// TestTenantElevationAttack_ModifyJWT verifica que no se puede modificar JWT
func TestTenantElevationAttack_ModifyJWT(t *testing.T) {
    // Setup: Intentar modificar claim "tenants" en JWT

    // 1. Obtener JWT válido
    validToken := createTenantAdminJWT(t, []string{"tenant-a"})

    // 2. Decodificar JWT
    parts := strings.Split(validToken, ".")
    payload := base64Decode(parts[1])

    // 3. Modificar payload (agregar tenant-b)
    modifiedPayload := strings.Replace(payload, `["tenant-a"]`, `["tenant-a","tenant-b"]`, 1)

    // 4. Reconstruir JWT (sin firma válida)
    maliciousToken := parts[0] + "." + base64Encode(modifiedPayload) + "." + parts[2]

    // 5. Intentar usar JWT modificado
    req := httptest.NewRequest("GET", "/v2/admin/tenants/tenant-b/users", nil)
    req.Header.Set("Authorization", "Bearer "+maliciousToken)

    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)

    // 6. Assert: Debe rechazar JWT (firma inválida)
    assert.Equal(t, 401, rec.Code, "Should reject JWT with invalid signature")
}

// TestTenantElevationAttack_ReplayToken verifica protección contra replay attacks
func TestTenantElevationAttack_ReplayToken(t *testing.T) {
    // Setup: Admin de tenant-A obtiene token, luego es desasignado

    // 1. Crear admin con tenant-A
    admin := createTenantAdmin(t, []string{"tenant-a"})
    token := loginAdmin(t, admin.Email, admin.Password)

    // 2. Admin puede acceder inicialmente
    req1 := httptest.NewRequest("GET", "/v2/admin/tenants/tenant-a/users", nil)
    req1.Header.Set("Authorization", "Bearer "+token)
    rec1 := httptest.NewRecorder()
    handler.ServeHTTP(rec1, req1)
    assert.Equal(t, 200, rec1.Code, "Should have initial access")

    // 3. Desasignar tenant-A del admin (simular cambio de permisos)
    updateAdmin(t, admin.ID, []string{}) // Sin tenants asignados

    // 4. Intentar reusar token antiguo (REPLAY ATTACK)
    req2 := httptest.NewRequest("GET", "/v2/admin/tenants/tenant-a/users", nil)
    req2.Header.Set("Authorization", "Bearer "+token)
    rec2 := httptest.NewRecorder()
    handler.ServeHTTP(rec2, req2)

    // 5. Assert: Debe denegar acceso (token todavía válido pero permisos cambiaron)
    // NOTA: Esto requiere que el middleware verifique permisos ACTUALES, no solo JWT
    assert.Equal(t, 403, rec2.Code, "Should deny access after permissions changed")
}
```

- [ ] **T-5.1.3:** Ejecutar tests de seguridad
  ```bash
  go test ./test/security/ -v -tags=security > docs/test-results/step-5.1-security-tests.txt
  ```

- [ ] **T-5.1.4:** Documentar resultados

**Criterios de Salida (DoD):**

- [x] 3 tests de seguridad implementados
- [x] Todos los tests pasan
- [x] Intentos de ataque son bloqueados correctamente
- [x] Logs de auditoría se generan

**Evidencias:**
- [ ] Archivo: `test/security/tenant_elevation_test.go`
- [ ] Output: `docs/test-results/step-5.1-security-tests.txt`
- [ ] Commit: `test(security): add tenant elevation attack prevention tests`

---

### **PASO 5.2: Tests End-to-End - Flujos Completos**

**Objetivo:** Probar flujos completos de usuario admin

**Tareas:**

- [ ] **T-5.2.1:** Instalar herramienta de E2E (si no existe)
  ```bash
  # Opción 1: Playwright
  cd ui && npm install -D @playwright/test

  # Opción 2: Cypress
  cd ui && npm install -D cypress
  ```

- [ ] **T-5.2.2:** Crear test E2E: Admin Global
  ```typescript
  // ui/e2e/admin-global-access.spec.ts

  test('Admin Global can access all tenants', async ({ page }) => {
    // 1. Login como admin global
    await page.goto('/admin/login')
    await page.fill('input[name="email"]', 'global@example.com')
    await page.fill('input[name="password"]', 'password')
    await page.click('button[type="submit"]')

    // 2. Navegar a tenant-A
    await page.goto('/admin/tenants/tenant-a/users')
    await expect(page.locator('h1')).toContainText('Users')

    // 3. Navegar a tenant-B
    await page.goto('/admin/tenants/tenant-b/users')
    await expect(page.locator('h1')).toContainText('Users')

    // 4. Navegar a tenant-C
    await page.goto('/admin/tenants/tenant-c/users')
    await expect(page.locator('h1')).toContainText('Users')
  })
  ```

- [ ] **T-5.2.3:** Crear test E2E: Admin Tenant (Acceso Permitido)
  ```typescript
  test('Admin Tenant can access assigned tenants only', async ({ page }) => {
    // 1. Login como admin tenant (asignado a tenant-A y tenant-B)
    await page.goto('/admin/login')
    await page.fill('input[name="email"]', 'tenant-admin@example.com')
    await page.fill('input[name="password"]', 'password')
    await page.click('button[type="submit"]')

    // 2. Puede acceder a tenant-A
    await page.goto('/admin/tenants/tenant-a/users')
    await expect(page.locator('h1')).toContainText('Users')

    // 3. Puede acceder a tenant-B
    await page.goto('/admin/tenants/tenant-b/users')
    await expect(page.locator('h1')).toContainText('Users')
  })
  ```

- [ ] **T-5.2.4:** Crear test E2E: Admin Tenant (Acceso Denegado)
  ```typescript
  test('Admin Tenant cannot access unassigned tenants', async ({ page }) => {
    // 1. Login como admin tenant
    await page.goto('/admin/login')
    await page.fill('input[name="email"]', 'tenant-admin@example.com')
    await page.fill('input[name="password"]', 'password')
    await page.click('button[type="submit"]')

    // 2. Intentar acceder a tenant-C (no asignado)
    await page.goto('/admin/tenants/tenant-c/users')

    // 3. Debe mostrar error 403
    await expect(page.locator('.error-message')).toContainText('403')
    // O redirigir a página de error
  })
  ```

- [ ] **T-5.2.5:** Ejecutar tests E2E
  ```bash
  cd ui && npx playwright test
  # o
  cd ui && npx cypress run
  ```

- [ ] **T-5.2.6:** Documentar resultados
  ```bash
  # Playwright genera reporte automático
  cd ui && npx playwright show-report

  # Copiar reporte
  cp -r ui/playwright-report docs/test-results/step-5.2-e2e-report/
  ```

**Criterios de Salida (DoD):**

- [x] 3 tests E2E implementados
- [x] Tests pasan en entorno local
- [x] Reporte de tests generado
- [x] Screenshots/videos de tests (Playwright/Cypress auto-genera)

**Evidencias:**
- [ ] Archivos de tests: `ui/e2e/*.spec.ts`
- [ ] Reporte: `docs/test-results/step-5.2-e2e-report/`
- [ ] Commit: `test(e2e): add end-to-end tests for multi-tenant admin access`

---

### **PASO 5.3: Performance Testing - Carga y Stress**

**Objetivo:** Verificar que el middleware no introduce overhead significativo

**Tareas:**

- [ ] **T-5.3.1:** Instalar herramienta de benchmark
  ```bash
  # Opción 1: Apache Bench (ya instalado en muchos sistemas)
  which ab

  # Opción 2: wrk
  # macOS: brew install wrk
  # Linux: apt-get install wrk
  ```

- [ ] **T-5.3.2:** Crear script de benchmark
  ```bash
  #!/bin/bash
  # scripts/benchmark-admin-endpoints.sh

  API_URL="http://localhost:8080"
  ADMIN_TOKEN="your-admin-token-here"

  echo "Benchmarking Admin Endpoints with Tenant Resolution..."

  # Test 1: GET /v2/admin/tenants/{tenant_id}/users
  echo "Test 1: List Users"
  ab -n 1000 -c 10 -H "Authorization: Bearer $ADMIN_TOKEN" \
    "$API_URL/v2/admin/tenants/test-tenant/users" \
    > docs/test-results/step-5.3-benchmark-users.txt

  # Test 2: GET /v2/admin/tenants/{tenant_id}/sessions
  echo "Test 2: List Sessions"
  ab -n 1000 -c 10 -H "Authorization: Bearer $ADMIN_TOKEN" \
    "$API_URL/v2/admin/tenants/test-tenant/sessions" \
    > docs/test-results/step-5.3-benchmark-sessions.txt

  # Test 3: GET /v2/admin/tenants/{tenant_id}/tokens
  echo "Test 3: List Tokens"
  ab -n 1000 -c 10 -H "Authorization: Bearer $ADMIN_TOKEN" \
    "$API_URL/v2/admin/tenants/test-tenant/tokens" \
    > docs/test-results/step-5.3-benchmark-tokens.txt

  echo "Benchmark completed. Results in docs/test-results/"
  ```

- [ ] **T-5.3.3:** Ejecutar benchmark
  ```bash
  chmod +x scripts/benchmark-admin-endpoints.sh
  ./scripts/benchmark-admin-endpoints.sh
  ```

- [ ] **T-5.3.4:** Analizar resultados
  - Requests per second (RPS) debe ser >100
  - Latencia p95 debe ser <100ms
  - Sin errores bajo carga

- [ ] **T-5.3.5:** Documentar métricas
  ```markdown
  # Performance Test Results

  ## Test Configuration
  - Concurrency: 10 concurrent requests
  - Total Requests: 1000 per endpoint
  - Backend: Go 1.21, single instance

  ## Results

  ### Endpoint: GET /v2/admin/tenants/{tenant_id}/users
  - RPS: 450
  - Latency (avg): 22ms
  - Latency (p95): 45ms
  - Errors: 0

  ### Endpoint: GET /v2/admin/tenants/{tenant_id}/sessions
  - RPS: 480
  - Latency (avg): 20ms
  - Latency (p95): 42ms
  - Errors: 0

  ## Conclusion
  ✅ All endpoints meet performance requirements
  ```

**Criterios de Salida (DoD):**

- [x] Script de benchmark creado
- [x] Tests ejecutados con 1000+ requests
- [x] Métricas documentadas
- [x] Performance aceptable (>100 RPS, <100ms p95)

**Evidencias:**
- [ ] Script: `scripts/benchmark-admin-endpoints.sh`
- [ ] Resultados: `docs/test-results/step-5.3-benchmark-*.txt`
- [ ] Análisis: `docs/test-results/step-5.3-performance-analysis.md`
- [ ] Commit: `test(perf): add performance benchmarks for admin endpoints`

---

## **FASE 6: DOCUMENTACIÓN Y ROLLOUT**

**Objetivo:** Documentar cambios y preparar deployment

**Duración Estimada:** 2 horas

---

### **PASO 6.1: Documentación Técnica**

**Objetivo:** Documentar arquitectura y decisiones

**Tareas:**

- [ ] **T-6.1.1:** Crear documento de arquitectura
  ```markdown
  # Arquitectura Multi-Tenant Admin

  ## Resumen
  Este documento describe la arquitectura de autenticación y autorización
  para admins multi-tenant en HelloJohn.

  ## Tipos de Admins

  ### Admin Global
  - Acceso ilimitado a todos los tenants
  - Claim JWT: `admin_type: "global"`
  - No tiene lista de tenants asignados

  ### Admin Tenant
  - Acceso limitado a tenants específicos
  - Claim JWT: `admin_type: "tenant"`, `tenants: ["id1", "id2"]`
  - Validación en cada request

  ## Flujo de Autenticación

  [Diagrama de secuencia]

  ## Flujo de Autorización

  [Diagrama de decisión]

  ## Resolución de Tenant

  ### Método Estándar: Path Parameter
  Formato: `/v2/admin/tenants/{tenant_id}/resource`

  Razones:
  - RESTful
  - Visible en logs
  - Cacheable
  - No requiere headers custom

  ## Seguridad

  ### Prevención de Tenant Elevation
  - Middleware `RequireAdminTenantAccess()` valida acceso
  - Orden crítico: TenantResolution → Auth → Authorization
  - Logging de intentos de acceso denegados

  ## APIs

  [Documentación de endpoints]
  ```

- [ ] **T-6.1.2:** Crear archivo: `docs/architecture/MULTI_TENANT_ADMIN.md`

- [ ] **T-6.1.3:** Crear documento de decisiones (ADR)
  ```markdown
  # ADR-001: Path Parameter para Tenant Resolution

  ## Contexto
  El sistema tenía múltiples formas de enviar tenant (headers, query params, path).

  ## Decisión
  Estandarizar en path parameter único: `/tenants/{tenant_id}/...`

  ## Consecuencias

  ### Positivas
  - Un solo lugar para mantener
  - URLs claras y RESTful
  - Debugging simplificado

  ### Negativas
  - Requiere migración de frontend
  - Breaking change en URLs
  ```

- [ ] **T-6.1.4:** Crear archivo: `docs/architecture/ADR-001-PATH-PARAMETER.md`

- [ ] **T-6.1.5:** Actualizar README principal
  ```markdown
  # HelloJohn

  ## Admin Multi-Tenant

  HelloJohn soporta dos tipos de administradores:

  - **Global Admin**: Acceso a todos los tenants
  - **Tenant Admin**: Acceso limitado a tenants asignados

  Ver [Documentación de Arquitectura](docs/architecture/MULTI_TENANT_ADMIN.md)
  ```

**Criterios de Salida (DoD):**

- [x] Documento de arquitectura creado
- [x] ADR documentado
- [x] README actualizado
- [x] Diagramas incluidos (opcional)

**Evidencias:**
- [ ] Archivo: `docs/architecture/MULTI_TENANT_ADMIN.md`
- [ ] Archivo: `docs/architecture/ADR-001-PATH-PARAMETER.md`
- [ ] Commit: `docs(architecture): document multi-tenant admin architecture and decisions`

---

### **PASO 6.2: Crear CHANGELOG**

**Objetivo:** Documentar cambios para usuarios/desarrolladores

**Tareas:**

- [ ] **T-6.2.1:** Crear/actualizar CHANGELOG.md
  ```markdown
  # Changelog

  ## [Unreleased]

  ### Added
  - Multi-tenant admin support with granular access control
  - Middleware `RequireAdminTenantAccess()` for tenant-level authorization
  - Centralized API client in frontend (`adminAPI`)
  - Comprehensive security tests for tenant elevation prevention

  ### Changed
  - **BREAKING**: Admin routes now use path parameter `/tenants/{tenant_id}/...`
  - **BREAKING**: Frontend pages migrated to dynamic route segments
  - Simplified tenant resolution to single method (path parameter only)
  - Admin middleware chain order: TenantResolution → Auth → TenantAccess

  ### Removed
  - Multiple tenant resolvers (headers, query params) - now only path parameter
  - Query parameter `?id=` in admin routes

  ### Fixed
  - Tenant elevation security vulnerability (admin could access unauthorized tenants)

  ### Security
  - Added validation that admin JWT claims match requested tenant
  - Added audit logging for denied tenant access attempts
  ```

- [ ] **T-6.2.2:** Commit CHANGELOG
  ```bash
  git add CHANGELOG.md
  git commit -m "docs: add changelog for multi-tenant admin implementation"
  ```

**Criterios de Salida (DoD):**

- [x] CHANGELOG.md actualizado
- [x] Breaking changes claramente marcados
- [x] Security fixes documentados

**Evidencias:**
- [ ] Archivo: `CHANGELOG.md`
- [ ] Commit de changelog

---

### **PASO 6.3: Crear Migration Guide**

**Objetivo:** Guía para usuarios actuales migrando a nueva versión

**Tareas:**

- [ ] **T-6.3.1:** Crear guía de migración
  ```markdown
  # Migration Guide: Multi-Tenant Admin Standardization

  ## Para Desarrolladores (Backend)

  ### Cambios en Controllers

  **ANTES:**
  ```go
  tenantID := r.PathValue("id")
  ```

  **DESPUÉS:**
  ```go
  tenantID := r.PathValue("tenant_id")
  ```

  ### Cambios en Middlewares

  Si tenías middlewares custom que resolvían tenant:

  **ANTES:**
  ```go
  tenant := r.Header.Get("X-Tenant-ID")
  ```

  **DESPUÉS:**
  ```go
  tda := middlewares.GetTenant(r.Context())
  tenantID := tda.ID()
  ```

  ## Para Desarrolladores (Frontend)

  ### Cambios en URLs

  **ANTES:**
  ```typescript
  /admin/tenants/users?id=tenant-uuid
  ```

  **DESPUÉS:**
  ```typescript
  /admin/tenants/tenant-uuid/users
  ```

  ### Cambios en Código

  **ANTES:**
  ```typescript
  const searchParams = useSearchParams()
  const tenantId = searchParams.get("id")
  ```

  **DESPUÉS:**
  ```typescript
  const params = useParams()
  const tenantId = params.tenant_id
  ```

  ### API Calls

  **ANTES:**
  ```typescript
  fetch(`/v2/admin/tenants/${tenantId}/users`, {
    headers: { 'X-Tenant-ID': tenantId }
  })
  ```

  **DESPUÉS:**
  ```typescript
  import { adminAPI } from '@/lib/admin-api'
  adminAPI.getTenantUsers(tenantId)
  ```

  ## Para Administradores de Sistema

  ### Verificar Permisos de Admins

  1. Listar todos los admins:
     ```bash
     cat data/hellojohn/admins/admins.yaml
     ```

  2. Verificar que cada admin tiene `type` y `assigned_tenants` correctos

  3. Si un admin debe ser global:
     ```yaml
     type: "global"
     assigned_tenants: []
     ```

  4. Si un admin es de tenant específico:
     ```yaml
     type: "tenant"
     assigned_tenants:
       - "tenant-uuid-1"
       - "tenant-uuid-2"
     ```

  ### Testing de Migración

  1. Hacer login como admin tenant
  2. Intentar acceder a tenant asignado (debe funcionar)
  3. Intentar acceder a tenant NO asignado (debe dar 403)
  4. Revisar logs: buscar `admin_tenant_access_denied`
  ```

- [ ] **T-6.3.2:** Crear archivo: `docs/migration/MULTI_TENANT_ADMIN.md`

**Criterios de Salida (DoD):**

- [x] Guía de migración completa
- [x] Ejemplos de código antes/después
- [x] Pasos para verificar migración exitosa

**Evidencias:**
- [ ] Archivo: `docs/migration/MULTI_TENANT_ADMIN.md`
- [ ] Commit: `docs(migration): add migration guide for multi-tenant admin changes`

---

### **PASO 6.4: Preparar Pull Request**

**Objetivo:** Consolidar todos los cambios en PR para revisión

**Tareas:**

- [ ] **T-6.4.1:** Revisar todos los commits
  ```bash
  git log --oneline feature/admin-multi-tenant-standardization
  ```

- [ ] **T-6.4.2:** Squash commits si es necesario (opcional)
  ```bash
  # Agrupar commits relacionados
  git rebase -i main
  ```

- [ ] **T-6.4.3:** Push final
  ```bash
  git push origin feature/admin-multi-tenant-standardization
  ```

- [ ] **T-6.4.4:** Crear PR en GitHub/GitLab
  - Título: `feat: implement multi-tenant admin with standardized tenant resolution`
  - Descripción:
    ```markdown
    ## Summary
    Implements multi-tenant admin access control and standardizes tenant resolution to single method (path parameter).

    ## Changes

    ### Security
    - ✅ Fixes tenant elevation vulnerability (admin could access unauthorized tenants)
    - ✅ Adds middleware `RequireAdminTenantAccess()` for validation
    - ✅ Adds comprehensive security tests

    ### Backend
    - ✅ Simplifies tenant resolution to path parameter only
    - ✅ Updates all admin routes to `/tenants/{tenant_id}/...`
    - ✅ Updates all controllers to use `PathValue("tenant_id")`

    ### Frontend
    - ✅ Migrates pages to dynamic route segments
    - ✅ Creates centralized API client (`adminAPI`)
    - ✅ Updates all navigation links

    ### Testing
    - ✅ Unit tests for middleware (5 test cases)
    - ✅ Integration tests (3 scenarios)
    - ✅ Security tests (3 attack scenarios)
    - ✅ E2E tests (3 user flows)
    - ✅ Performance benchmarks

    ### Documentation
    - ✅ Architecture documentation
    - ✅ ADR for path parameter decision
    - ✅ Migration guide
    - ✅ Updated CHANGELOG

    ## Breaking Changes

    ⚠️ **URLs Changed:**
    - Old: `/admin/tenants/users?id=tenant-uuid`
    - New: `/admin/tenants/tenant-uuid/users`

    ⚠️ **Headers Removed:**
    - `X-Tenant-ID` and `X-Tenant-Slug` no longer supported

    See [Migration Guide](docs/migration/MULTI_TENANT_ADMIN.md)

    ## Testing

    - [ ] All unit tests pass (see `docs/test-results/`)
    - [ ] All integration tests pass
    - [ ] Security tests pass (no tenant elevation)
    - [ ] E2E tests pass
    - [ ] Performance benchmarks acceptable (>100 RPS)
    - [ ] Manual testing completed (see `docs/test-results/step-4.6-frontend-manual-tests.md`)

    ## Checklist

    - [ ] Code reviewed
    - [ ] Tests added/updated
    - [ ] Documentation updated
    - [ ] CHANGELOG updated
    - [ ] Migration guide created
    - [ ] Breaking changes documented

    ## Evidence & Audit Trail

    All implementation evidence documented in `docs/` directory:
    - `docs/audit/` - Current state audit
    - `docs/changes/` - Diffs for each step
    - `docs/test-results/` - All test outputs
    - `docs/architecture/` - Architecture docs
    - `docs/migration/` - Migration guide

    Closes #[ISSUE_NUMBER]
    ```

- [ ] **T-6.4.5:** Asignar reviewers

- [ ] **T-6.4.6:** Etiquetar PR
  - Labels: `security`, `breaking-change`, `enhancement`

**Criterios de Salida (DoD):**

- [x] PR creado con descripción completa
- [x] Breaking changes claramente marcados
- [x] Reviewers asignados
- [x] Labels agregados

**Evidencias:**
- [ ] URL del PR
- [ ] Screenshot del PR

---

### **PASO 6.5: Preparar Deployment**

**Objetivo:** Preparar deployment a staging/producción

**Tareas:**

- [ ] **T-6.5.1:** Crear checklist de deployment
  ```markdown
  # Deployment Checklist

  ## Pre-Deployment

  - [ ] PR aprobado por al menos 2 reviewers
  - [ ] Todos los tests CI/CD pasan
  - [ ] CHANGELOG actualizado
  - [ ] Migration guide disponible
  - [ ] Backups de datos realizados

  ## Deployment Staging

  - [ ] Merge PR a branch `develop`
  - [ ] Deploy a staging
  - [ ] Ejecutar smoke tests
  - [ ] Verificar admin global puede acceder
  - [ ] Verificar admin tenant tiene acceso limitado
  - [ ] Verificar logs de auditoría
  - [ ] Verificar performance (RPS, latencia)

  ## Deployment Production

  - [ ] Crear tag de versión: `git tag -a v1.X.0`
  - [ ] Merge `develop` a `main`
  - [ ] Deploy a producción
  - [ ] Ejecutar smoke tests en producción
  - [ ] Monitorear logs por 1 hora
  - [ ] Verificar métricas (errores, latencia)

  ## Post-Deployment

  - [ ] Notificar a equipo de cambios
  - [ ] Actualizar documentación pública
  - [ ] Archivar evidencias de auditoría
  ```

- [ ] **T-6.5.2:** Crear archivo: `docs/deployment/DEPLOYMENT_CHECKLIST.md`

- [ ] **T-6.5.3:** Crear smoke tests script
  ```bash
  #!/bin/bash
  # scripts/smoke-tests.sh

  API_URL="${1:-https://api.hellojohn.com}"

  echo "Running smoke tests on $API_URL"

  # Test 1: Health check
  echo "Test 1: Health Check"
  curl -f "$API_URL/health" || exit 1

  # Test 2: Admin login
  echo "Test 2: Admin Login"
  TOKEN=$(curl -s -X POST "$API_URL/v2/admin/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"admin@example.com","password":"password"}' \
    | jq -r '.access_token')

  [ -z "$TOKEN" ] && echo "Login failed" && exit 1

  # Test 3: Access tenant (should work for global admin)
  echo "Test 3: Global Admin Access"
  curl -f -H "Authorization: Bearer $TOKEN" \
    "$API_URL/v2/admin/tenants/test-tenant/users" || exit 1

  echo "✅ All smoke tests passed"
  ```

- [ ] **T-6.5.4:** Crear archivo: `scripts/smoke-tests.sh`

**Criterios de Salida (DoD):**

- [x] Checklist de deployment creado
- [x] Smoke tests script creado
- [x] Plan de rollback documentado (ver Fase 7)

**Evidencias:**
- [ ] Archivo: `docs/deployment/DEPLOYMENT_CHECKLIST.md`
- [ ] Archivo: `scripts/smoke-tests.sh`
- [ ] Commit: `chore(deploy): add deployment checklist and smoke tests`

---

## **FASE 7: ROLLBACK PLAN**

**Objetivo:** Plan de contingencia en caso de problemas

---

### **PASO 7.1: Documentar Rollback Plan**

**Tareas:**

- [ ] **T-7.1.1:** Crear documento de rollback
  ```markdown
  # Rollback Plan: Multi-Tenant Admin Implementation

  ## Cuándo Hacer Rollback

  Ejecutar rollback si:
  - Errores críticos en producción (>5% error rate)
  - Admin global no puede acceder a tenants
  - Tenant elevation successful (security breach)
  - Performance degradation (>50% slower)

  ## Procedimiento de Rollback

  ### Opción 1: Git Revert (Recomendado)

  ```bash
  # 1. Revertir merge commit
  git revert -m 1 <merge-commit-hash>

  # 2. Push a main
  git push origin main

  # 3. Deploy versión anterior
  ./deploy.sh

  # 4. Verificar que todo funciona
  ./scripts/smoke-tests.sh
  ```

  ### Opción 2: Tag Anterior

  ```bash
  # 1. Checkout tag anterior
  git checkout v1.X-1.0

  # 2. Deploy
  ./deploy.sh

  # 3. Verificar
  ./scripts/smoke-tests.sh
  ```

  ## Post-Rollback

  1. Notificar a equipo
  2. Investigar causa raíz
  3. Crear issue con análisis
  4. Planificar re-implementación

  ## Datos Afectados

  Esta implementación NO modifica datos:
  - ✅ No hay migraciones de base de datos
  - ✅ No se modifican estructuras de admin en FileSystem
  - ✅ Rollback es seguro

  ## Tiempo Estimado de Rollback

  - Revert + deploy: ~5 minutos
  - Verificación: ~10 minutos
  - **Total: ~15 minutos**
  ```

- [ ] **T-7.1.2:** Crear archivo: `docs/deployment/ROLLBACK_PLAN.md`

**Criterios de Salida (DoD):**

- [x] Rollback plan documentado
- [x] Procedimientos claros paso a paso
- [x] Tiempo estimado definido

**Evidencias:**
- [ ] Archivo: `docs/deployment/ROLLBACK_PLAN.md`
- [ ] Commit: `docs(deploy): add rollback plan for multi-tenant admin`

---

## 📊 CRITERIOS DE ACEPTACIÓN (GLOBAL)

### Funcionales

- [ ] **FA-001:** Admin global puede acceder a todos los tenants
- [ ] **FA-002:** Admin tenant puede acceder solo a tenants asignados
- [ ] **FA-003:** Admin tenant recibe 403 al intentar acceder a tenant no asignado
- [ ] **FA-004:** Todas las rutas admin usan path parameter `/tenants/{tenant_id}/...`
- [ ] **FA-005:** Frontend usa `useParams()` en lugar de `searchParams`
- [ ] **FA-006:** API centralizada en `adminAPI` funciona correctamente

### No Funcionales

- [ ] **NF-001:** Performance: >100 RPS en endpoints admin
- [ ] **NF-002:** Latencia: p95 <100ms
- [ ] **NF-003:** Coverage: >80% en nuevos middlewares
- [ ] **NF-004:** Logs de auditoría para accesos denegados
- [ ] **NF-005:** Zero downtime durante deployment

### Seguridad

- [ ] **SEC-001:** Tenant elevation attack prevenido (tests pasan)
- [ ] **SEC-002:** JWT validation correcta (firma, expiration)
- [ ] **SEC-003:** No hay bypass del middleware de autorización
- [ ] **SEC-004:** Logs de auditoría completos y estructurados

### Documentación

- [ ] **DOC-001:** Arquitectura documentada
- [ ] **DOC-002:** ADR creado para decisión de path parameter
- [ ] **DOC-003:** Migration guide disponible
- [ ] **DOC-004:** CHANGELOG actualizado con breaking changes
- [ ] **DOC-005:** Tests documentados con resultados

---

## 📁 EVIDENCIAS DE AUDITORÍA

### Estructura de Evidencias

```
docs/
├── audit/                                # Estado inicial
│   ├── path_value_id.txt
│   ├── path_value_tenant.txt
│   ├── current_resolvers.txt
│   ├── current_routes.txt
│   ├── frontend_api_calls.txt
│   └── frontend_query_params.txt
│
├── changes/                              # Diffs de cada paso
│   ├── step-2.1-tenant-middleware.diff
│   ├── step-2.2-router.diff
│   ├── step-2.3-controllers.diff
│   ├── step-3.1-admin-tenant-access.diff
│   ├── step-4.2-pages-diff.txt
│   └── ... (más diffs)
│
├── test-results/                         # Outputs de tests
│   ├── baseline-go-tests.txt
│   ├── baseline-ui-tests.txt
│   ├── step-2.3-controller-tests.txt
│   ├── step-3.4-security-tests.txt
│   ├── step-5.1-security-tests.txt
│   ├── step-5.2-e2e-report/
│   └── step-5.3-benchmark-*.txt
│
├── architecture/                         # Docs técnicos
│   ├── MULTI_TENANT_ADMIN.md
│   └── ADR-001-PATH-PARAMETER.md
│
├── migration/                            # Guías
│   └── MULTI_TENANT_ADMIN.md
│
└── deployment/                           # Deployment
    ├── DEPLOYMENT_CHECKLIST.md
    └── ROLLBACK_PLAN.md
```

### Evidencias Requeridas por Paso

| Fase | Paso | Evidencias Requeridas |
|------|------|-----------------------|
| 1 | 1.1 | Archivos de auditoría en `docs/audit/` |
| 1 | 1.2 | Screenshot de rama creada |
| 1 | 1.3 | Baseline tests outputs |
| 2 | 2.1 | Diff del middleware + output de tests |
| 2 | 2.2 | Diff del router + lista de rutas |
| 2 | 2.3 | Diffs de controllers + tests |
| 2 | 2.4 | Coverage report + manual tests |
| 3 | 3.1 | Diff del middleware con validación |
| 3 | 3.2 | Diff de admin chain |
| 3 | 3.3 | JWTs decodificados (global + tenant) |
| 3 | 3.4 | Tests de seguridad + coverage |
| 3 | 3.5 | Tests de integración E2E |
| 4 | 4.1 | Lista de páginas movidas |
| 4 | 4.2 | Diffs de páginas actualizadas |
| 4 | 4.3 | Archivos `admin-api.ts` + types |
| 4 | 4.4 | Diffs de migración a API |
| 4 | 4.5 | Diffs de navegación |
| 4 | 4.6 | Documento de tests manuales + screenshots |
| 5 | 5.1 | Tests de seguridad + outputs |
| 5 | 5.2 | Tests E2E + reporte |
| 5 | 5.3 | Benchmarks + análisis |
| 6 | 6.1 | Docs de arquitectura + ADR |
| 6 | 6.2 | CHANGELOG actualizado |
| 6 | 6.3 | Migration guide |
| 6 | 6.4 | URL del PR |
| 6 | 6.5 | Deployment checklist + smoke tests |
| 7 | 7.1 | Rollback plan |

---

## ✅ CHECKLIST DE EJECUCIÓN

### Pre-Implementación

- [ ] Plan revisado y aprobado
- [ ] Equipo asignado
- [ ] Entorno de desarrollo listo
- [ ] Backups realizados

### FASE 1: Preparación

- [ ] ✅ Paso 1.1: Auditoría del estado actual
- [ ] ✅ Paso 1.2: Crear rama de desarrollo
- [ ] ✅ Paso 1.3: Configurar entorno de testing

### FASE 2: Backend - Tenant Resolution

- [ ] ✅ Paso 2.1: Simplificar middleware de tenant
- [ ] ✅ Paso 2.2: Estandarizar rutas en router
- [ ] ✅ Paso 2.3: Actualizar controllers
- [ ] ✅ Paso 2.4: Verificación integral backend

### FASE 3: Backend - Seguridad Multi-Tenant

- [ ] ✅ Paso 3.1: Implementar middleware de validación
- [ ] ✅ Paso 3.2: Integrar middleware en cadena
- [ ] ✅ Paso 3.3: Verificar emisión de AdminClaims
- [ ] ✅ Paso 3.4: Crear tests de seguridad
- [ ] ✅ Paso 3.5: Tests de integración E2E

### FASE 4: Frontend - Migración

- [ ] ✅ Paso 4.1: Reestructurar rutas en Next.js
- [ ] ✅ Paso 4.2: Actualizar páginas para usar useParams
- [ ] ✅ Paso 4.3: Centralizar API client
- [ ] ✅ Paso 4.4: Migrar páginas a API centralizado
- [ ] ✅ Paso 4.5: Actualizar navegación y links
- [ ] ✅ Paso 4.6: Testing frontend manual

### FASE 5: Testing Integral

- [ ] ✅ Paso 5.1: Tests de seguridad (tenant elevation)
- [ ] ✅ Paso 5.2: Tests E2E - Flujos completos
- [ ] ✅ Paso 5.3: Performance testing

### FASE 6: Documentación y Rollout

- [ ] ✅ Paso 6.1: Documentación técnica
- [ ] ✅ Paso 6.2: Crear CHANGELOG
- [ ] ✅ Paso 6.3: Crear migration guide
- [ ] ✅ Paso 6.4: Preparar Pull Request
- [ ] ✅ Paso 6.5: Preparar deployment

### FASE 7: Rollback Plan

- [ ] ✅ Paso 7.1: Documentar rollback plan

### Post-Implementación

- [ ] PR aprobado y merged
- [ ] Deployed a staging
- [ ] Smoke tests en staging
- [ ] Deployed a producción
- [ ] Smoke tests en producción
- [ ] Monitoreo activo (24h)
- [ ] Evidencias archivadas
- [ ] Retrospectiva realizada

---

## 📞 CONTACTOS Y RESPONSABLES

| Rol | Nombre | Contacto |
|-----|--------|----------|
| **Project Lead** | [ASIGNAR] | [EMAIL] |
| **Backend Lead** | [ASIGNAR] | [EMAIL] |
| **Frontend Lead** | [ASIGNAR] | [EMAIL] |
| **QA Lead** | [ASIGNAR] | [EMAIL] |
| **DevOps** | [ASIGNAR] | [EMAIL] |
| **Security** | [ASIGNAR] | [EMAIL] |

---

## 📅 CRONOGRAMA ESTIMADO

| Fase | Duración | Fechas Estimadas |
|------|----------|------------------|
| FASE 1: Preparación | 2h | Día 1 mañana |
| FASE 2: Backend Tenant Resolution | 3h | Día 1 tarde |
| FASE 3: Backend Seguridad | 4h | Día 2 mañana |
| FASE 4: Frontend Migración | 8h | Día 2 tarde + Día 3 |
| FASE 5: Testing Integral | 3h | Día 3 tarde |
| FASE 6: Documentación | 2h | Día 4 mañana |
| **TOTAL** | **22h (~3 días)** | **4 días con buffer** |

---

## 🔄 CONTROL DE CAMBIOS

| Versión | Fecha | Autor | Cambios |
|---------|-------|-------|---------|
| 1.0 | 2026-02-03 | [AUTOR] | Creación inicial del plan |
|  |  |  |  |

---

## ✍️ FIRMAS Y APROBACIONES

| Rol | Nombre | Firma | Fecha |
|-----|--------|-------|-------|
| **Autor del Plan** | [NOMBRE] | __________ | ____/____/____ |
| **Reviewer Técnico** | [NOMBRE] | __________ | ____/____/____ |
| **Aprobador (Tech Lead)** | [NOMBRE] | __________ | ____/____/____ |
| **Aprobador (Product)** | [NOMBRE] | __________ | ____/____/____ |

---

**FIN DEL PLAN DE IMPLEMENTACIÓN**
