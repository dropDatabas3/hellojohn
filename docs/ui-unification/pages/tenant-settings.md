# Page Audit — /admin/tenants/settings

**Status**: 🔍 AUDIT
**Priority**: 2 (Core pages)
**Complexity**: MEGA-COMPLEX (~2130 lines, 5 tabs, 6+ subcomponents)
**Audit Date**: 2026-01-31

---

## 1. Purpose

Página de configuración completa del tenant:
- **General**: Nombre, slug, idioma
- **Branding**: Logo, colores de marca, favicon
- **Security**: Duración de sesión, tokens, MFA, social login, políticas de contraseña
- **Issuer**: Modo de issuer (path/subdomain/global)
- **Export/Import**: Exportar/importar configuración completa

---

## 2. Primary Actions

- [x] Edit tenant name, slug, display_name
- [x] Upload/change logo
- [x] Select brand color (presets + custom)
- [x] Configure session duration (presets + custom)
- [x] Configure refresh token duration
- [x] Enable/disable MFA
- [x] Enable/disable social login
- [x] Configure password policies
- [x] Select issuer mode
- [x] Export full configuration to JSON
- [x] Import configuration from JSON
- [x] Validate import before applying

**Destructive actions**: Import overwrites existing config (confirmation implemented)

---

## 3. Current UI Inventory

| Element | Component Used | Notes |
|---------|----------------|-------|
| Button | `@/components/ui/button` | ❌ Multiple uses |
| Input | `@/components/ui/input` | ❌ Multiple uses |
| Label | `@/components/ui/label` | ❌ Multiple uses |
| Switch | `@/components/ui/switch` | ❌ Multiple uses |
| Badge | `@/components/ui/badge` | ❌ Multiple uses |
| Card | `@/components/ui/card` | ❌ Multiple uses |
| Tabs | `@/components/ui/tabs` | ❌ Multiple uses |
| Dialog | `@/components/ui/dialog` | ❌ Multiple uses |
| Select | `@/components/ui/select` | ❌ Multiple uses |
| Tooltip | `@/components/ui/tooltip` | ❌ Multiple uses |
| Alert | `@/components/ui/alert` | ⚠️ No existe DS Alert |
| Progress | `@/components/ui/progress` | ⚠️ No existe DS Progress |
| Separator | `@/components/ui/separator` | ⚠️ No existe DS Separator |

---

## 4. Subcomponentes Locales

| Component | Lines | Description |
|-----------|-------|-------------|
| ColorPicker | 197-277 | Selector de color con presets |
| LogoUploader | 280-456 | Drag & drop logo con preview |
| DurationSelector | 459-538 | Selector de duración con presets |
| BrandingPreview | 541-604 | Vista previa de página de login |
| ExportDialog | 608-799 | Dialog de exportación con opciones |
| ImportDialog | 802-1093 | Dialog de importación con validación |

---

## 5. Colores Hardcodeados Detectados

### En LogoUploader:
- `bg-zinc-900` — preview dark background
- `bg-white` — preview light background

### En ExportDialog:
- `bg-green-500/10 text-green-500` — success state
- `bg-red-500/10 text-red-500` — error state

### En ImportDialog:
- `border-amber-500/30 bg-amber-500/10 text-amber-500` — warning Alert
- `text-amber-200/80` — warning text
- `border-green-500/30 bg-green-500/10 text-green-500` — success validation
- `text-amber-400` — warning list

### En Header:
- `from-zinc-500/20 to-slate-500/20 border-zinc-500/20` — icon gradient
- `text-zinc-400` — icon color
- `text-amber-500 border-amber-500/30` — unsaved changes badge

### En Info Banner:
- `border-indigo-500/30 from-indigo-500/10 via-purple-500/5` — gradient
- `text-indigo-400` — info icon

### En General Tab:
- `text-green-500` — valid slug checkmark
- `text-red-500` — invalid slug X

---

## 6. Data & States

| State | Currently Handled? | Notes |
|-------|-------------------|-------|
| Loading | ⚠️ Spinner only | Loader2 spinner, no skeleton |
| Empty | ✅ Yes | No tenant selected state |
| Error | ✅ Toast | Toast on mutation errors |
| Success | ✅ Toast | Toast on save |
| Unsaved | ✅ Badge | "Cambios sin guardar" badge |

---

## 7. Needed DS Components

### Ya Existen ✅
- PageShell, Card, Badge, Button, Input, Label, Switch
- Select, Dialog, Tabs, Tooltip (DS versions)
- InlineAlert, Skeleton, EmptyState

### Necesitan Crearse o Reutilizarse ⚠️
- **Progress** — para export progress bar
- **Separator** — para divisores (puede usar `<hr>` con estilos)
- **Alert** → usar **InlineAlert** del DS

---

## 8. Risks

- **MEGA alta complejidad** — 2130 líneas, 6 subcomponentes
- **Muchas interacciones** — file uploads, color pickers, presets
- **APIs de backend** — export/import con validación
- **Progress component** — no existe en DS
- **Separator component** — no existe en DS

---

## 9. Migration Strategy

Dado que es una página MEGA-COMPLEX (2130 líneas), se recomienda:

### Approach: Migración Completa (estimado 6-8+ horas)

1. **Imports**: Cambiar todos los ui/ a ds/
2. **Header + Tabs**: Usar PageShell y DS Tabs
3. **Cards**: Migrar a DS Card
4. **Dialogs**: Migrar ExportDialog e ImportDialog
5. **Alerts**: Cambiar Alert a InlineAlert
6. **Colores**: Reemplazar todos los hardcoded con tokens semánticos
7. **Loading**: Agregar Skeleton
8. **Progress**: Crear DS Progress o usar solución temporal

### Tokens a Aplicar:
- `success` → estados válidos, exportación exitosa
- `warning` → alertas de precaución, cambios sin guardar
- `danger` → errores, estados inválidos
- `info` → banners informativos
- `accent` → iconos destacados

---

## 10. Components to Create

| Component | Priority | Description |
|-----------|----------|-------------|
| Progress | Medium | Barra de progreso para exportación |
| Separator | Low | Divisor simple (puede ser `<hr>`) |

---

## 11. Additional Notes

- Mantener todos los subcomponentes (ColorPicker, etc.), solo actualizar estilos
- ETag handling para settings update debe mantenerse intacto
- Export/Import APIs (ISS-11-02, ISS-11-03) deben seguir funcionando
- Animaciones existentes (animate-in) pueden mantenerse

---

**Next Steps**:
1. Confirmar approach con usuario
2. Crear DS Progress component (si se necesita)
3. Iniciar migración por tabs
