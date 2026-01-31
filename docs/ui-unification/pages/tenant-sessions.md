# Page Audit — /admin/tenants/sessions

**Status**: ✅ DONE
**Priority**: 3 (Advanced features)
**Complexity**: COMPLEX (~1098 lines → ~900 lines migrated)
**Audit Date**: 2026-01-31
**Migration Completed**: 2026-01-31

---

## 1. Purpose

Página de gestión de sesiones de un tenant. Permite:
- Visualizar todas las sesiones activas de usuarios
- Filtrar por dispositivo, estado, búsqueda
- Revocar sesiones individuales, por usuario, o masivamente
- Configurar políticas de sesión (duración, timeout, concurrencia, seguridad)

---

## 2. Primary Actions

- [x] View sessions table with filters
- [x] Revoke single session
- [x] Revoke all user sessions
- [x] Revoke all tenant sessions
- [x] View session detail modal
- [x] Edit session policies (duration, timeout, max concurrent)
- [x] Toggle security settings (notify new device, require 2FA)

**Destructive actions**: Revoke sessions (confirmation dialog implemented)

---

## 3. Current UI Inventory

| Element | Component Used | Notes |
|---------|----------------|-------|
| Button | `@/components/ui/button` | ❌ Debe usar DS |
| Input | `@/components/ui/input` | ❌ Debe usar DS |
| Label | `@/components/ui/label` | ❌ Debe usar DS |
| Switch | `@/components/ui/switch` | ❌ Debe usar DS |
| Badge | `@/components/ui/badge` | ❌ Debe usar DS |
| Tabs | `@/components/ui/tabs` | ❌ Debe usar DS |
| Dialog | `@/components/ui/dialog` | ❌ Debe usar DS |
| DropdownMenu | `@/components/ui/dropdown-menu` | ❌ Debe usar DS |
| Select | `@/components/ui/select` | ❌ Debe usar DS |
| Table | `@/components/ui/table` | ⚠️ No hay DS Table aún |
| Tooltip | `@/components/ui/tooltip` | ⚠️ No hay DS Tooltip aún |
| StatCard | Local component | ⚠️ Colores hardcoded |
| EmptyState | Local component | ❌ Debe usar DS EmptyState |
| SessionDetailModal | Local component | ⚠️ Colores hardcoded |
| ConfirmDialog | Local component | ⚠️ Colores hardcoded |

---

## 4. Colores Hardcodeados Detectados

### En getStatusColor():
- `bg-emerald-500/10 text-emerald-600 border-emerald-500/20` (active)
- `bg-amber-500/10 text-amber-600 border-amber-500/20` (idle)
- `bg-zinc-500/10 text-zinc-500 border-zinc-500/20` (expired)

### En StatCard:
- `zinc`, `emerald`, `blue`, `violet` — hardcoded color maps

### En SessionDetailModal:
- `from-violet-500 to-purple-600` — gradient hardcoded
- `from-blue-500 to-cyan-500` — user avatar gradient
- `bg-zinc-50 dark:bg-zinc-800/50` — panel backgrounds

### En ConfirmDialog:
- `text-amber-500` — warning icon color

### En Policies Tab:
- `bg-violet-100 dark:bg-violet-900/30 text-violet-600` — info sidebar
- `bg-amber-50 dark:bg-amber-950/20 text-amber-600` — warning banner
- `text-violet-500` — chevron icons

### En Info Banner:
- `bg-blue-50 dark:bg-blue-950/20 border-blue-200 text-blue-600` — info banner

### En Actions:
- `text-red-600`, `text-amber-600` — dropdown item colors

---

## 5. Data & States

| State | Currently Handled? | Notes |
|-------|-------------------|-------|
| Loading | ⚠️ Spinner only | No skeleton, just RefreshCw spinner |
| Empty | ✅ Yes | Local EmptyState component |
| Error | ⚠️ Toast only | No dedicated error UI |
| Success | ✅ Toast | Toast notifications on mutations |

---

## 6. Needed DS Components

### Ya Existen ✅
- PageShell, PageHeader
- Card, CardContent
- Badge, Button, Input
- Label, Switch, Select
- Dialog, DropdownMenu
- Tabs, TabsList, TabsTrigger, TabsContent
- EmptyState, Skeleton

### Necesitan Crearse ⚠️
- **Table** (considerar si migrar o usar componente existente)
- **Tooltip** (considerar si migrar o usar componente existente)
- **InlineAlert** — para banners info/warning (puede que ya exista)

---

## 7. Risks

- **Alta complejidad** — 1098 líneas, múltiples secciones
- **Table component** — No existe en DS, evaluar si crear o reutilizar ui/table
- **Mutations activas** — revokeSession, revokeUserSessions, revokeAllSessions, updatePolicy
- **Mock data presente** — generateMockSessions() puede causar confusión
- **Integración con sessionsAdminAPI** — mantener intacta

---

## 8. Migration Strategy

Dado que es una página COMPLEX, se recomienda migrar en sub-tareas:

### Sub-task 1: Imports + Layout + Header
- Cambiar imports de `@/components/ui/` a `@/components/ds`
- Wrap en PageShell
- Migrar header a PageHeader

### Sub-task 2: Stats Cards + Helper Functions
- Migrar StatCard a usar tokens semánticos
- Actualizar getStatusColor() para usar tokens

### Sub-task 3: Filters + Table
- Migrar filtros a DS components
- Decidir estrategia para Table (keep/migrate/create)

### Sub-task 4: Tabs + Policies Form
- Migrar Tabs a DS
- Migrar form inputs a DS

### Sub-task 5: Modals
- Migrar SessionDetailModal
- Migrar ConfirmDialog

### Sub-task 6: Loading States + EmptyState
- Skeleton loading
- DS EmptyState

---

## 9. Recommended Approach

**Opción A: Migración Completa** (4-6 horas)
- Migrar toda la página en una sesión
- Pros: Consistencia total
- Cons: Alto riesgo, difícil rollback

**Opción B: Migración Incremental** (recomendada)
- Migrar por sub-tasks con commits checkpoint
- Pros: Rollback fácil, testeable por partes
- Cons: Más tiempo total

---

## 10. Additional Notes

- La página incluye **mock data** (MOCK_SESSIONS) — evaluar si eliminar
- Textos en español hardcodeados — mantener
- Suspense boundary ya implementado correctamente
- Funciones helper útiles: formatTimeAgo, formatDuration, getDeviceIcon

---

**Next Steps**:
1. Decidir approach (completa vs incremental)
2. Verificar si Table/Tooltip necesitan DS component
3. Arrancar Dark iteration
