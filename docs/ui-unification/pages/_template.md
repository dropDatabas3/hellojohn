# Page Audit — /admin/{pageSlug}

**Status**: 🔍 AUDIT | 🚧 DARK_IN_PROGRESS | 🎨 LIGHT_IN_PROGRESS | ✅ DONE

---

## 1. Purpose
> Qué hace esta página en 2-3 oraciones

## 2. Primary Actions
- [ ] Action 1 (e.g., "Create tenant")
- [ ] Action 2 (e.g., "Delete tenant")
- [ ] Action 3 (e.g., "Export JSON")

## 3. Current UI Inventory

| Element | Component Used | Notes |
|---------|----------------|-------|
| Header | Custom div | Inconsistent with other pages |
| Table | shadcn Table | 200+ rows, needs virtualization? |
| Create button | Custom Button | Different radius than DS |

## 4. Data & States

| State | Currently Handled? | Notes |
|-------|-------------------|-------|
| Loading | ✅ Skeleton | Good |
| Empty | ❌ No | Muestra tabla vacía |
| Error | ⚠️ Partial | Toast genérico |
| Success | ✅ Toast | OK |

## 5. UX Issues Detected
1. Header style diferente a `/admin/cluster`
2. No hay confirmación en "Delete all"
3. Search no es debounced (golpea backend en cada keystroke)

## 6. Needed DS Components
- [ ] `PageShell` (ya existe)
- [ ] `DataTable` (necesita implementarse)
- [ ] `ConfirmDialog` (ya existe)
- [ ] `DebouncedInput` (crear nuevo patrón DS)

## 7. Risks
- Lógica de bulk delete compleja, no romper
- Feature flags en tenant settings, verificar permisos
- Integración con backend `/v2/admin/tenants` (no cambiar contrato)

## 8. Screenshots
- [Before Dark](../screenshots/{pageSlug}/before-dark.png)
- [Before Light](../screenshots/{pageSlug}/before-light.png)
- [After Dark](../screenshots/{pageSlug}/after-dark.png)
- [After Light](../screenshots/{pageSlug}/after-light.png)

---

**Next Steps**:
1. Crear `DebouncedInput` en DS antes de migrar
2. Verificar `DataTable` soporta bulk actions
3. Arrancar iteración Dark
