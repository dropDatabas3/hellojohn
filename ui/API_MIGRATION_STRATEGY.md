# Estrategia de Migración de API - UI

## Problema Resuelto

**Problema Original**: Los componentes estaban usando `fetch()` directamente con rutas relativas (ej: `/v2/admin/tenants/123/users`), lo que causaba que las llamadas se dirigieran a `localhost:3000` (servidor Next.js) en lugar de `localhost:8080` (API backend).

**Ejemplo del problema**:
```typescript
// ❌ INCORRECTO - Llama a localhost:3000
const res = await fetch(`/v2/admin/tenants/${tenantId}/users`, {
  headers: { "Authorization": `Bearer ${token}` }
})
```

## Solución Implementada

### 1. Función Helper: `apiFetch()`

Se agregó una función wrapper en [`/ui/lib/routes.ts`](ui/lib/routes.ts) que automáticamente:
- Agrega el `API_BASE_URL` (localhost:8080 o el configurado)
- Aplica el mapeo de rutas (v1 → v2)
- Incluye credenciales para cookies cross-site

```typescript
// ✅ CORRECTO - Llama a localhost:8080
import { apiFetch } from "@/lib/routes"

const res = await apiFetch(`/v2/admin/tenants/${tenantId}/users`, {
  headers: { "Authorization": `Bearer ${token}` }
})
```

### 2. Cliente API Existente: `useApi()`

Para casos más complejos, sigue usando el hook `useApi()`:

```typescript
import { useApi } from "@/lib/hooks/use-api"

const api = useApi()
const users = await api.get(`/v2/admin/tenants/${tenantId}/users`)
```

## Archivos Migrados

✅ **Completamente migrados a V2**:
- [`ui/app/(admin)/admin/tenants/[id]/users/UsersClientPage.tsx`](ui/app/(admin)/admin/tenants/[id]/users/UsersClientPage.tsx)
- [`ui/app/(admin)/admin/users/page.tsx`](ui/app/(admin)/admin/users/page.tsx)
- [`ui/app/(admin)/admin/playground/page.tsx`](ui/app/(admin)/admin/playground/page.tsx)
- [`ui/lib/auth-refresh.ts`](ui/lib/auth-refresh.ts)
- [`ui/app/(auth)/login/page.tsx`](ui/app/(auth)/login/page.tsx)
- [`ui/app/(auth)/register/page.tsx`](ui/app/(auth)/register/page.tsx)
- [`ui/app/(admin)/admin/tenants/[id]/TenantDetailClientPage.tsx`](ui/app/(admin)/admin/tenants/[id]/TenantDetailClientPage.tsx)

## Cambios Realizados

### 1. Eliminación de Referencias V1 → V2

Todas las rutas `/v1/*` fueron migradas a `/v2/*`:

| Ruta V1 (Antigua) | Ruta V2 (Nueva) |
|-------------------|-----------------|
| `/v1/auth/login` | `/v2/auth/login` |
| `/v1/auth/register` | `/v2/auth/register` |
| `/v1/auth/refresh` | `/v2/auth/refresh` |
| `/v1/auth/config` | `/v2/auth/config` |
| `/v1/me` | `/v2/me` |
| `/v1/session/login` | `/v2/session/login` |
| `/v1/admin/users/disable` | `/v2/admin/users/disable` |
| `/v1/admin/users/enable` | `/v2/admin/users/enable` |
| `/v1/admin/users/resend-verification` | `/v2/admin/users/resend-verification` |
| `/v1/tenants` | `/v2/admin/tenants` |
| `/v1/tenants/${id}/clients` | `/v2/admin/clients?tenant_id=${id}` |
| `/v1/admin/clients` | `/v2/admin/clients` |
| `/v1/admin/scopes` | `/v2/admin/scopes` |
| `/v1/tenants/${id}/authorize` | `/oauth2/authorize` (estándar OAuth2) |
| `/v1/tenants/${id}/token` | `/v2/tenants/${id}/token` |

### 2. Migración de fetch() Directos

Todos los `fetch()` directos fueron reemplazados por `apiFetch()`:

**Antes**:
```typescript
const res = await fetch(`/v1/admin/users/disable`, {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    "Authorization": `Bearer ${token}`
  },
  body: JSON.stringify({ user_id, tenant_id })
})
```

**Después**:
```typescript
const res = await apiFetch(`/v2/admin/users/disable`, {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    "Authorization": `Bearer ${token}`
  },
  body: JSON.stringify({ user_id, tenant_id })
})
```

## Reglas para Nuevos Componentes

### ✅ DO (Hacer)

1. **Usar `useApi()` para llamadas con manejo de errores completo**:
   ```typescript
   import { useApi } from "@/lib/hooks/use-api"
   
   const api = useApi()
   const data = await api.get('/v2/admin/tenants')
   ```

2. **Usar `apiFetchWithTenant()` para endpoints que requieren tenant context**:
   ```typescript
   import { apiFetchWithTenant } from "@/lib/routes"
   
   const res = await apiFetchWithTenant('/v2/admin/users', tenantId, {
     headers: { Authorization: `Bearer ${token}` }
   })
   // Automáticamente agrega X-Tenant-ID header
   ```

3. **Usar `apiFetch()` para fetch directo sin tenant**:
   ```typescript
   import { apiFetch } from "@/lib/routes"
   
   const res = await apiFetch('/v2/admin/tenants', {
     headers: { Authorization: `Bearer ${token}` }
   })
   ```

4. **Usar constantes de rutas de `API_ROUTES`**:
   ```typescript
   import { API_ROUTES } from "@/lib/routes"
   
   const data = await api.get(API_ROUTES.ADMIN_TENANTS)
   const tenant = await api.get(API_ROUTES.ADMIN_TENANT(tenantId))
   ```

### ❌ DON'T (No hacer)

1. **NO usar `fetch()` con rutas relativas**:
   ```typescript
   // ❌ INCORRECTO
   fetch('/v2/admin/tenants')
   ```

2. **NO usar rutas V1**:
   ```typescript
   // ❌ INCORRECTO
   api.get('/v1/auth/login')
   ```

3. **NO hardcodear URLs base**:
   ```typescript
   // ❌ INCORRECTO
   fetch('http://localhost:8080/v2/admin/tenants')
   ```

4. **NO poner tenant en la URL si se envía por header**:
   ```typescript
   // ❌ INCORRECTO - Duplicación innecesaria
   apiFetchWithTenant('/v2/admin/tenants/123/users', '123', ...)
   
   // ✅ CORRECTO - Tenant solo en header
   apiFetchWithTenant('/v2/admin/users', tenantId, ...)
   ```

## Sistema de Adaptación V1 → V2

El sistema actual mantiene la función `mapRoute()` en [`routes.ts`](ui/lib/routes.ts#L40-L64) para compatibilidad, pero **ya no hay rutas V1 en uso en la aplicación**.

La función `mapRoute()` se mantiene por:
- Transición gradual futura si es necesaria
- Compatibilidad con componentes legacy externos
- Documentación de la migración realizada

## Variables de Entorno

```bash
# .env.local (UI)
NEXT_PUBLIC_API_BASE=http://localhost:8080
NEXT_PUBLIC_API_VERSION=v2  # Default, puede omitirse
```

## Verificación

Para verificar que no hay llamadas incorrectas:

```powershell
# Buscar fetch() con rutas relativas (posibles problemas)
grep -r "fetch\s*\(\s*['\`\"]/" ui/app --include="*.tsx" --include="*.ts"

# Buscar referencias a /v1
grep -r "/v1/" ui/app --include="*.tsx" --include="*.ts"
```

## Próximos Pasos

- ✅ Todos los componentes migrados a V2
- ✅ Todas las rutas V1 eliminadas
- ✅ Sistema de `apiFetch()` implementado
- ⚠️ Monitorear si aparecen nuevos componentes con `fetch()` directo
- 📝 Considerar agregar un ESLint rule para prevenir uso de `fetch()` directo

## Resumen

**Estado**: ✅ **Migración Completada**

- **Problema**: fetch() relativo llamaba a localhost:3000
- **Solución**: `apiFetch()` usa `API_BASE_URL` (localhost:8080)
- **Limpieza**: Todas las rutas V1 migradas a V2
- **Herramientas**: `useApi()`, `apiFetch()`, `API_ROUTES`
- **Archivos migrados**: 7 archivos principales

---

**Última actualización**: 2026-01-27
