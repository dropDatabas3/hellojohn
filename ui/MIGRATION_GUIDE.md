# Guía de Migración UI V1→V2

## ✅ Trabajo Completado

### Infraestructura
- ✅ Sistema de mapeo automático V1→V2 (`ui/lib/routes.ts`)
- ✅ API client actualizado con mapeo automático
- ✅ Constantes tipadas para todas las rutas V2
- ✅ Documentación completa de endpoints

### Componentes Actualizados
- ✅ `app/(admin)/admin/page.tsx` (Dashboard)
- ✅ `app/(admin)/admin/tenants/consents/page.tsx` (Consents)
- ✅ `app/(admin)/admin/rbac/page.tsx` (RBAC)

---

## 📝 Tareas Pendientes

### 1. Actualizar Componentes Restantes

Los siguientes componentes aún usan rutas hardcodeadas. Deben ser actualizados para usar `API_ROUTES`:

**Patrón a seguir:**

```typescript
// ❌ ANTES (V1 hardcodeado)
import { api } from "@/lib/api"

const data = await api.get("/v1/admin/tenants")

// ✅ DESPUÉS (V2 con constantes)
import { api } from "@/lib/api"
import { API_ROUTES } from "@/lib/routes"

const data = await api.get(API_ROUTES.ADMIN_TENANTS)
```

**Archivos a actualizar:**

```bash
# Buscar todos los archivos que usan rutas hardcodeadas
cd ui
grep -r "api\.get.*\"/v1" app/
grep -r "api\.post.*\"/v1" app/
grep -r "api\.put.*\"/v1" app/
grep -r "api\.delete.*\"/v1" app/
```

**Componentes identificados:**

1. **Auth Components**
   - `app/(auth)/login/page.tsx`
   - `app/(auth)/register/page.tsx`
   - Actualizar: `/v1/auth/login` → `API_ROUTES.AUTH_LOGIN`
   - Actualizar: `/v1/auth/register` → `API_ROUTES.AUTH_REGISTER`

2. **Admin - Tenants**
   - `app/(admin)/admin/tenants/**/*.tsx`
   - Actualizar rutas de CRUD de tenants
   - Usar: `API_ROUTES.ADMIN_TENANT(id)`, `API_ROUTES.ADMIN_TENANT_SETTINGS(id)`, etc.

3. **Admin - Clients**
   - `app/(admin)/admin/clients/**/*.tsx`
   - Actualizar: `/v1/admin/clients` → `API_ROUTES.ADMIN_CLIENTS`
   - Actualizar: `/v1/admin/clients/${id}` → `API_ROUTES.ADMIN_CLIENT(id)`

4. **Admin - Database**
   - `app/(admin)/admin/database/page.tsx`
   - Verificar qué endpoints usa
   - Actualizar según corresponda

5. **Otros componentes admin**
   - Buscar con grep todos los `api.get/post/put/delete` en `app/(admin)`
   - Actualizar uno por uno

### 2. Verificar DTOs

Comparar las estructuras de request/response entre V1 y V2:

**Archivo de referencia:** `internal/http/v2/dto/`

**DTOs Críticos a verificar:**

```typescript
// Auth
LoginRequest: { tenant_id, client_id, email, password }
LoginResult: { access_token, refresh_token, expires_in, token_type }
RegisterRequest: { tenant_id, client_id, email, password, ... }

// Tenants
CreateTenantInput: { slug, name, language, settings }
TenantSettings: { issuer_mode, user_db, smtp, cache, branding }

// Clients
CreateClientInput: { client_id, name, type, redirect_uris, ... }
UpdateClientInput: { name?, type?, redirect_uris?, ... }

// Users
CreateUserInput: { email, password, ... }
UpdateUserInput: { email?, ... }
```

**Proceso:**

1. Leer DTOs de V2 en `internal/http/v2/dto/`
2. Comparar con tipos TypeScript en `ui/lib/types.ts`
3. Actualizar tipos si es necesario
4. Crear tests para verificar compatibilidad

### 3. Implementar Endpoints Faltantes

Los siguientes endpoints **NO están en V2** y necesitan ser implementados:

#### 3.1. Admin Keys

**Endpoints:**
- `GET /v2/keys` - Listar signing keys
- `POST /v2/keys/rotate` - Rotar signing key global

**Archivos a crear:**

```
internal/http/v2/
├── dto/admin/keys.go              (KeyDTO, RotateRequest)
├── services/admin/keys_service.go  (KeysService interface + impl)
├── controllers/admin/keys_controller.go
└── router/keys_routes.go
```

**Agregado en:**
- `services/admin/services.go` (agregar Keys)
- `controllers/admin/controllers.go` (agregar Keys)
- `router/router.go` (RegisterKeysRoutes)

#### 3.2. Admin Stats

**Endpoint:**
- `GET /v2/admin/stats` - Estadísticas del sistema

**DTO de respuesta:**
```go
type StatsResponse struct {
    TotalTenants  int `json:"total_tenants"`
    TotalUsers    int `json:"total_users"`
    TotalClients  int `json:"total_clients"`
    TotalTokens   int `json:"total_tokens"`
    ActiveUsers24h int `json:"active_users_24h"`
    // ... más stats
}
```

**Archivos a crear:**
```
internal/http/v2/
├── dto/admin/stats.go
├── services/admin/stats_service.go
├── controllers/admin/stats_controller.go
└── router/stats_routes.go (o agregar a admin_routes.go)
```

#### 3.3. Admin Config

**Endpoints:**
- `GET /v2/admin/config` - Obtener config global
- `PUT /v2/admin/config` - Actualizar config global

**Archivos a crear:**
```
internal/http/v2/
├── dto/admin/config.go
├── services/admin/config_service.go
├── controllers/admin/config_controller.go
└── router/config_routes.go
```

#### 3.4. CSRF

**Endpoint:**
- `GET /v2/csrf` - Obtener token CSRF

**Archivos a crear:**
```
internal/http/v2/
├── dto/security/csrf.go
├── services/security/csrf_service.go
├── controllers/security/csrf_controller.go
└── router/security_routes.go
```

### 4. Testing

#### 4.1. Unit Tests

```bash
# Crear tests para mapeo de rutas
ui/__tests__/lib/routes.test.ts

# Casos a testear:
- mapRoute("/v1/auth/login") → "/v2/auth/login"
- mapRoute("/oauth2/token") → "/oauth2/token" (sin cambios)
- mapRoute("/readyz") → "/readyz" (sin cambios)
- API_VERSION=v1 mantiene V1
- API_VERSION=v2 usa V2
```

#### 4.2. Integration Tests

```bash
# Backend tests
cd hellojohn
go test ./internal/http/v2/... -v

# Endpoints críticos:
- POST /v2/auth/login
- POST /v2/auth/register
- POST /v2/auth/refresh
- GET /v2/admin/tenants
- POST /v2/admin/tenants
- GET /v2/admin/clients
- POST /v2/admin/clients
```

#### 4.3. E2E Tests

**Flujos a testear:**

1. **Auth Flow**
   - Registro de usuario
   - Login
   - Refresh token
   - Logout

2. **Admin - Tenants**
   - Crear tenant
   - Listar tenants
   - Editar tenant settings
   - Migrar tenant
   - Test connection

3. **Admin - Clients**
   - Crear client
   - Editar client
   - Revoke secret
   - Eliminar client

4. **Admin - RBAC**
   - Asignar rol a usuario
   - Asignar permisos a rol
   - Verificar permisos

5. **Admin - Consents**
   - Listar consents
   - Revocar consent

#### 4.4. Manual Testing Checklist

```
Dashboard
[ ] Health check carga correctamente
[ ] Lista de tenants se muestra
[ ] Cluster info se muestra
[ ] Links funcionan

Tenants
[ ] Crear tenant funciona
[ ] Editar settings funciona
[ ] Migrar funciona
[ ] Test connection funciona
[ ] Eliminar tenant funciona

Clients
[ ] Crear client funciona
[ ] Editar client funciona
[ ] Revoke secret funciona
[ ] Eliminar client funciona

RBAC
[ ] Buscar usuario funciona
[ ] Asignar rol funciona
[ ] Remover rol funciona
[ ] Buscar rol funciona
[ ] Asignar permiso funciona
[ ] Remover permiso funciona

Consents
[ ] Listar consents funciona
[ ] Revocar consent funciona
[ ] Filtrar por usuario funciona

Auth
[ ] Login funciona
[ ] Register funciona
[ ] Logout funciona
[ ] Refresh funciona
```

---

## 🚀 Flujo de Trabajo Recomendado

### Paso 1: Actualizar Componentes (1-2 días)

```bash
# 1. Buscar componentes con rutas hardcodeadas
grep -r "\"\/v1\/" ui/app/

# 2. Para cada archivo encontrado:
#    a. Agregar import { API_ROUTES } from "@/lib/routes"
#    b. Reemplazar strings con constantes
#    c. Testear manualmente

# 3. Commit por módulo
git add ui/app/(auth)
git commit -m "feat(ui): migrate auth routes to V2"

git add ui/app/(admin)/admin/tenants
git commit -m "feat(ui): migrate tenants routes to V2"

# ... etc
```

### Paso 2: Verificar DTOs (0.5-1 día)

```bash
# 1. Comparar DTOs
diff <(cat internal/http/v2/dto/auth/login.go) ui/lib/types.ts

# 2. Crear tipos faltantes en ui/lib/types.ts

# 3. Commit
git add ui/lib/types.ts
git commit -m "feat(ui): update DTOs to match V2"
```

### Paso 3: Implementar Endpoints Faltantes (2-3 días)

```bash
# Por cada endpoint faltante:
# 1. Crear DTO
# 2. Crear Service
# 3. Crear Controller
# 4. Registrar Route
# 5. Testear

# Orden sugerido:
# 1. Admin Stats (más simple)
# 2. Admin Keys (medio)
# 3. Admin Config (medio)
# 4. CSRF (más complejo si requiere validación)
```

### Paso 4: Testing (1-2 días)

```bash
# 1. Unit tests
cd ui
npm test

# 2. Integration tests
cd ..
go test ./internal/http/v2/...

# 3. E2E tests (manual)
# Seguir checklist de arriba

# 4. Performance testing
# ab -n 1000 -c 10 http://localhost:8082/v2/auth/login
```

---

## 📚 Referencias

- `UI_ROUTES_MIGRATION.md` - Mapeo completo V1→V2
- `UI_MIGRATION_SUMMARY.md` - Resumen ejecutivo
- `CLAUDE.md` - Arquitectura V2
- `ui/lib/routes.ts` - Constantes y mapeo
- `internal/http/v2/dto/` - DTOs V2
- `internal/http/v2/router/` - Routers V2

---

## ❓ Preguntas Frecuentes

**Q: ¿Por qué algunos endpoints no cambian?**
A: Los endpoints estándar de OAuth2/OIDC (`/oauth2/*`, `/.well-known/*`, `/userinfo`) no cambian porque son especificaciones estándar.

**Q: ¿Puedo usar V1 y V2 al mismo tiempo?**
A: Sí, el mapeo automático permite que código V1 funcione con backend V2. Pero lo ideal es migrar todo a constantes V2.

**Q: ¿Qué pasa si un endpoint no existe en V2?**
A: El backend retornará 404. Necesitas implementar el endpoint en V2 o temporalmente usar `NEXT_PUBLIC_API_VERSION=v1`.

**Q: ¿Cómo sé si un componente está actualizado?**
A: Busca `import { API_ROUTES }` en el archivo. Si no lo tiene, necesita actualización.

**Q: ¿Cuánto tiempo toma la migración completa?**
A: Estimado: 5-7 días (2 días componentes + 3 días endpoints faltantes + 2 días testing).

---

## 🎯 Criterios de Éxito

### Migración Completa cuando:

- [ ] **Cero** referencias a rutas hardcodeadas `/v1/` en `ui/app/`
- [ ] **Todos** los componentes usan constantes `API_ROUTES`
- [ ] **Todos** los DTOs V2 documentados y verificados
- [ ] **Todos** los endpoints faltantes implementados
- [ ] **100%** de tests pasando (unit + integration)
- [ ] **Zero** errores 404 en testing manual
- [ ] Documentación actualizada
- [ ] V1 puede ser deprecado

---

## 🔐 Seguridad

### Checklist de Seguridad Post-Migración

- [ ] CSRF protection funcionando en todos los forms
- [ ] Rate limiting activo en endpoints públicos
- [ ] JWT validation correcta en endpoints autenticados
- [ ] Tenant isolation verificado (no cross-tenant leaks)
- [ ] Admin middleware funcionando correctamente
- [ ] RBAC enforcement verificado
- [ ] Secrets encriptados en Control Plane
- [ ] No hardcoded credentials en código

---

## 📞 Soporte

Si encuentras problemas durante la migración:

1. **Revisar documentación**: `UI_ROUTES_MIGRATION.md`, `CLAUDE.md`
2. **Revisar código V2**: `internal/http/v2/`
3. **Comparar con V1**: `internal/http/v1/handlers/`
4. **Abrir issue** en GitHub con detalles
