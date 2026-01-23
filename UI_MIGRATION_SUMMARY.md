# UI Routes V1→V2 Migration Summary

## ✅ Completado

### 1. Análisis y Documentación
- ✅ Identificadas todas las rutas V1 usadas por el UI
- ✅ Verificadas rutas V2 disponibles en los routers
- ✅ Creado documento completo de migración (`UI_ROUTES_MIGRATION.md`)
- ✅ Mapeadas **55+ endpoints** de V1 a V2

### 2. Infraestructura de Migración
- ✅ Creada utilidad de mapeo de rutas (`ui/lib/routes.ts`)
- ✅ Actualizado API client con mapeo automático V1→V2
- ✅ Definidas constantes tipadas para todas las rutas V2

### 3. Sistema de Mapeo Automático

El API client ahora mapea automáticamente las rutas V1 a V2:

```typescript
// Antes (V1)
api.get("/v1/auth/login")

// Ahora (auto-mapeado a V2)
api.get("/v1/auth/login")  // Se convierte en /v2/auth/login automáticamente

// O usando constantes tipadas (recomendado)
import { API_ROUTES } from "@/lib/routes"
api.get(API_ROUTES.AUTH_LOGIN)  // /v2/auth/login
```

### 4. Control de Versión

Agregado control de versión via variable de entorno:

```bash
# .env.local
NEXT_PUBLIC_API_VERSION=v2  # "v1" o "v2" (default: v2)
```

---

## 📊 Estadísticas de Migración

### Endpoints Verificados en V2

| Categoría | Endpoints V2 | Estado |
|-----------|--------------|--------|
| **Auth** | 12 | ✅ Disponibles |
| **Session** | 2 | ✅ Disponibles |
| **Admin - Tenants** | 12 | ✅ Disponibles |
| **Admin - Clients** | 5 | ✅ Disponibles |
| **Admin - Scopes** | 3 | ✅ Disponibles |
| **Admin - Consents** | 5 | ✅ Disponibles |
| **Admin - RBAC** | 4 | ✅ Disponibles |
| **Admin - Users** | 6 | ✅ Disponibles |
| **OAuth2/OIDC** | 9 | ✅ Disponibles |
| **MFA** | 5 | ✅ Disponibles |
| **Email Flows** | 4 | ✅ Disponibles |
| **Social Auth** | 4 | ✅ Disponibles |
| **TOTAL** | **55+** | **✅ 100%** |

### Endpoints Pendientes en V2

Los siguientes endpoints **NO están implementados en V2** aún:

1. `/v2/keys` (GET) - Listar signing keys
2. `/v2/keys/rotate` (POST) - Rotar signing key global
3. `/v2/admin/stats` (GET) - Estadísticas del sistema
4. `/v2/admin/config` (GET/PUT) - Configuración global
5. `/v2/csrf` (GET) - Token CSRF

**Recomendación**: Estos endpoints necesitan ser implementados en V2 antes de deprecar completamente V1.

---

## 🔧 Archivos Modificados

### Creados
- `ui/lib/routes.ts` - Utilidad de mapeo de rutas + constantes tipadas
- `UI_ROUTES_MIGRATION.md` - Documentación completa de migración
- `UI_MIGRATION_SUMMARY.md` - Este resumen

### Modificados
- `ui/lib/api.ts` - Agregado mapeo automático V1→V2

---

## 📝 Próximos Pasos

### 1. Actualizar Componentes UI (En Progreso)

Reemplazar strings hardcodeados con constantes tipadas:

**Antes:**
```typescript
const { data } = await api.get("/v1/admin/tenants")
```

**Después:**
```typescript
import { API_ROUTES } from "@/lib/routes"
const { data } = await api.get(API_ROUTES.ADMIN_TENANTS)
```

**Archivos a actualizar:**
- `ui/app/(admin)/admin/page.tsx` ← Dashboard
- `ui/app/(admin)/admin/tenants/**/*.tsx` ← Gestión de tenants
- `ui/app/(admin)/admin/database/page.tsx` ← Base de datos
- `ui/app/(admin)/admin/tenants/consents/page.tsx` ← Consents
- `ui/app/(admin)/admin/rbac/page.tsx` ← RBAC
- ... (todos los componentes que usan `api.get/post/put/delete`)

### 2. Verificar DTOs V1 vs V2

Comparar estructuras de request/response entre V1 y V2:

- Login Request/Response
- Register Request/Response
- Tenant CRUD
- Client CRUD
- ... (todos los DTOs críticos)

### 3. Implementar Endpoints Faltantes

Crear controllers/services/routes V2 para:
- Admin Keys (`/v2/keys`, `/v2/keys/rotate`)
- Admin Stats (`/v2/admin/stats`)
- Admin Config (`/v2/admin/config`)
- CSRF (`/v2/csrf`)

### 4. Testing

1. **Unit Tests**: Verificar mapeo de rutas
2. **Integration Tests**: Probar endpoints V2
3. **E2E Tests**: Flujos completos del UI
4. **Manual Testing**:
   - Login/Logout
   - Crear/Editar Tenant
   - Crear/Editar Client
   - RBAC (asignar roles/permisos)
   - Consents
   - MFA flows

---

## 🚀 Migración Gradual

### Fase 1: Preparación (✅ Completada)
- ✅ Documentación completa
- ✅ Utilidad de mapeo
- ✅ API client actualizado

### Fase 2: Transición (🟡 En Progreso)
- ⏳ Actualizar componentes UI
- ⏳ Verificar DTOs
- ⏳ Testing inicial

### Fase 3: Consolidación (⬜ Pendiente)
- ⬜ Implementar endpoints faltantes
- ⬜ Testing completo
- ⬜ Documentar cambios breaking

### Fase 4: Deprecación V1 (⬜ Futuro)
- ⬜ Marcar V1 como deprecated
- ⬜ Establecer fecha de EOL
- ⬜ Remover código V1

---

## ⚙️ Configuración Recomendada

### .env.local
```bash
# API Configuration
NEXT_PUBLIC_API_BASE=http://localhost:8082  # V2 server (default: 8080)
NEXT_PUBLIC_API_VERSION=v2                  # v1 o v2 (default: v2)
```

### Desarrollo
```bash
# Terminal 1: Backend V2
cd hellojohn
FS_ROOT=./data/hellojohn \
SIGNING_MASTER_KEY=your-key \
V2_SERVER_ADDR=:8082 \
./hellojohn

# Terminal 2: Frontend
cd hellojohn/ui
npm run dev
```

---

## 📚 Referencias

- `UI_ROUTES_MIGRATION.md` - Mapeo completo de rutas V1→V2
- `CLAUDE.md` - Arquitectura V2 (Cascada)
- `internal/http/v2/router/*.go` - Routers V2
- `ui/lib/routes.ts` - Constantes y mapeo

---

## ❓ Preguntas Frecuentes

### ¿Puedo seguir usando V1 durante la migración?
**Sí**. El sistema de mapeo automático permite:
1. Usar rutas V1 en el código (se mapean a V2 automáticamente)
2. Cambiar `NEXT_PUBLIC_API_VERSION=v1` para volver a V1 temporalmente

### ¿Qué pasa si un endpoint no existe en V2?
El mapeo intentará usar V2. Si el endpoint no existe en V2:
- Recibirás un error 404
- Debes implementar el endpoint en V2
- O temporalmente usar `NEXT_PUBLIC_API_VERSION=v1`

### ¿Cómo sé si un endpoint está en V2?
Consulta `UI_ROUTES_MIGRATION.md` o busca en:
```bash
grep -r "mux.Handle" internal/http/v2/router/
```

### ¿Los DTOs son compatibles entre V1 y V2?
**Mayormente sí**, pero necesita verificación. La arquitectura V2 usa DTOs explícitos en `internal/http/v2/dto/`, mientras que V1 usaba structs anónimos o implícitos.

---

## 🎯 Objetivo Final

**Sistema 100% en V2:**
- ✅ Todos los endpoints migrados
- ✅ UI usando constantes tipadas
- ✅ DTOs verificados
- ✅ Testing completo
- ✅ V1 deprecado y removido

**Beneficios:**
- Código más mantenible (arquitectura en capas)
- Type safety (constantes tipadas)
- Mejor separación de responsabilidades
- Facilita testing y debugging
