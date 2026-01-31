# Page Audit — /admin/tenants

**Status**: 🚧 DARK_IN_PROGRESS

---

## 1. Purpose

The `/admin/tenants` page is the main tenant management interface. It displays a searchable table of all configured tenants in the system with their name, slug, and logo/avatar. Users can create new tenants via a wizard dialog, navigate to tenant detail/settings pages, and delete existing tenants with confirmation.

## 2. Primary Actions

- [ ] Search/filter tenants by name or slug (client-side filtering)
- [ ] Create new tenant via "Create Tenant" button (opens CreateTenantWizard dialog)
- [ ] Navigate to tenant detail page (click on table row)
- [ ] Edit tenant settings (via dropdown menu → Settings)
- [ ] Delete tenant with confirmation dialog (via dropdown menu → Delete)

## 3. Current UI Inventory

| Element | Component Used | Notes |
|---------|----------------|-------|
| Page wrapper | Custom `<div className="space-y-6">` | Missing PageShell pattern |
| Header | Custom `<h1>` + `<p>` + `<Button>` in flex layout | No PageHeader component |
| Search bar | `Input` from `@/components/ui/input` + `Search` icon | Old UI kit, inside Card header |
| Table | `Table` from `@/components/ui/table` | Old UI kit, needs DS DataTable |
| Create button | `Button` from `@/components/ui/button` | Old UI kit, not DS |
| Badge (slug) | `Badge` from `@/components/ui/badge` | Old UI kit, not DS |
| Dropdown menu | `DropdownMenu` from `@/components/ui/dropdown-menu` | Old UI kit (Radix headless OK, but styling needs DS) |
| Delete dialog | `Dialog` from `@/components/ui/dialog` | Old UI kit, needs DS Dialog |
| Card container | `Card` from `@/components/ui/card` | Old UI kit, not DS |
| Loading spinner | Custom animated div (border-2 border-primary) | No Skeleton component usage |
| Empty state | TableCell with text | No EmptyState component |
| Logo/Avatar | Custom `<img>` or `<div>` with hardcoded styles | Uses `bg-slate-100`, `text-slate-700` (hardcoded colors) |

## 4. Data & States

| State | Currently Handled? | Notes |
|-------|-------------------|-------|
| Loading | ⚠️ Partial | Shows spinner instead of Skeleton that preserves layout |
| Empty (no tenants) | ⚠️ Partial | Shows text in TableCell, not proper EmptyState component |
| Empty (search no results) | ⚠️ Partial | Same as above, same message for both cases |
| Error (fetch failed) | ❌ No | No error handling for failed tenant fetch |
| Error (delete failed) | ✅ Yes | Toast with error message |
| Success (delete) | ✅ Yes | Toast with success message |
| Deleting | ✅ Yes | Button shows "Deleting..." and is disabled |

## 5. UX Issues Detected

1. **No PageShell/PageHeader**: Uses custom header layout inconsistent with `/admin` (dashboard)
2. **Search not debounced**: Updates on every keystroke (currently client-side so not critical, but bad pattern)
3. **Loading shows spinner**: Should use Skeleton that preserves table layout (no jump when data loads)
4. **Empty state is text-only**: No EmptyState component with icon + CTA
5. **No error handling for fetch**: If `/v2/admin/tenants` fails, page shows nothing or crashes
6. **Hardcoded colors in logo/avatar**: Uses `bg-slate-100`, `text-slate-700` instead of semantic tokens
7. **Table uses old UI kit**: Needs migration to DS DataTable (or custom table with DS styling)
8. **Dropdown menu styling**: Uses old UI kit, needs DS Dropdown (Radix OK as headless, but styling must be DS)
9. **Dialog uses old UI kit**: Delete confirmation needs DS Dialog component
10. **Search bar inside Card header**: Non-standard pattern, should be in Toolbar or separate Section
11. **Row click vs dropdown click**: UX could be confusing (whole row navigates, but dropdown stops propagation)

## 6. Needed DS Components

### Already Available (Ola 1):
- [x] `PageShell` — exists
- [x] `PageHeader` — exists
- [x] `Section` — exists
- [x] `Card` — exists (but current Card is old UI kit)
- [x] `Button` — exists
- [x] `Badge` — exists
- [x] `Skeleton` — exists
- [x] `Toast` — exists (already used for delete success/error)

### Already Available (Ola 2):
- [x] `InlineAlert` — exists (for error states if needed)
- [x] `EmptyState` — exists (for no tenants / no search results)

### Missing (Need Ola 3 - Overlays):
- [ ] **`Dialog`** — For delete confirmation
  - **Carpeta destino**: `ui/components/ds/overlays/dialog.tsx`
  - **Responsabilidad**: Modal dialogs with header, content, footer, and overlay backdrop
  - **Props mínimas**: `open`, `onOpenChange`, `title`, `description`, `children`, `className`
  - **Base**: Radix UI Dialog (headless) with DS styling
  - **Uso en página**: Delete confirmation dialog (líneas 211-226)
  - **Regla de 2 usos**: ✅ Sí (aparece en muchas páginas del admin panel)

- [ ] **`Dropdown`** (or `DropdownMenu`)
  - **Carpeta destino**: `ui/components/ds/overlays/dropdown.tsx`
  - **Responsabilidad**: Context menus for actions (Edit/Delete/etc)
  - **Props mínimas**: `trigger`, `items`, `align`, `className`
  - **Base**: Radix UI DropdownMenu (headless) with DS styling
  - **Uso en página**: Actions menu per tenant row (líneas 158-194)
  - **Regla de 2 usos**: ✅ Sí (aparece en tablas de todo el admin panel)

### Missing (Need Custom or Ola 4):
- [ ] **`DataTable`** or custom Table pattern
  - **Carpeta destino**: `ui/components/ds/data/data-table.tsx` OR use Card + custom markup
  - **Responsabilidad**: Display tabular data with sorting, clickable rows, etc.
  - **Props mínimas**: `columns`, `data`, `onRowClick`, `loading`, `empty`, `className`
  - **Uso en página**: Main tenant list table (líneas 112-201)
  - **Decision needed**: ¿Crear DataTable genérico o solo migrar Table a DS styling?
  - **Regla de 2 usos**: ✅ Sí (tablas en Users, Clients, Sessions, Tokens, etc.)

- [ ] **`SearchInput`** or use `Input` with icon pattern
  - **Carpeta destino**: `ui/components/ds/core/input.tsx` (extend existing) OR `ui/components/ds/utils/search-input.tsx`
  - **Responsabilidad**: Input with search icon and optional debounce
  - **Props mínimas**: `placeholder`, `value`, `onChange`, `debounce?`, `className`
  - **Uso en página**: Search bar (líneas 96-104)
  - **Decision needed**: ¿Extender Input existente o crear componente separado?
  - **Regla de 2 usos**: ✅ Sí (search en Users, Clients, Logs, etc.)

## 7. Risks

- **API contract dependency**: Page relies on `/v2/admin/tenants` (GET) and `/v2/admin/tenants/:slug` (DELETE). Changes to response shape will break rendering.
- **CreateTenantWizard integration**: External component (`@/components/tenant/CreateTenantWizard`) that may also need DS migration. Don't migrate it in this page's scope, but verify it still works.
- **Delete mutation**: Uses optimistic UI update via `queryClient.invalidateQueries`. Ensure this doesn't break with DS migration.
- **Tenant logo URLs**: Logic for logo display includes base URL concatenation. Ensure this doesn't break with component changes.
- **Hardcoded avatar styles**: `bg-slate-100`, `text-slate-700` must be replaced with semantic tokens (e.g., `bg-muted`, `text-muted-foreground`).
- **Navigation patterns**: Row click navigates to detail, dropdown has Edit/Delete. Ensure stopPropagation logic remains intact.
- **Dropdown menu alignment**: Uses `align="end"`. Verify DS Dropdown supports this prop.
- **Client-side search**: Currently filters in-memory. If tenant count grows large (100+), consider server-side search.
- **No pagination**: If tenant count exceeds ~50, consider adding Pagination component (Ola 2).

---

## 8. Screenshots

**NOTE: As per project rules, screenshots are NOT required and should NOT be added to this document.**

---

## 9. DS Gap Analysis

### Critical Blockers (Ola 3 - Must implement BEFORE dark iteration):

1. **`Dialog`** — Required for delete confirmation
   - Base: Radix UI Dialog (headless)
   - Styling: DS semantic tokens, clay shadows, accent colors
   - API: `DialogContent`, `DialogHeader`, `DialogTitle`, `DialogDescription`, `DialogFooter`
   - **BLOCKER**: Cannot replace old Dialog without this

2. **`Dropdown`** (or `DropdownMenu`)
   - Base: Radix UI DropdownMenu (headless)
   - Styling: DS semantic tokens, proper z-index, shadows
   - API: `DropdownMenu`, `DropdownMenuTrigger`, `DropdownMenuContent`, `DropdownMenuItem`, `DropdownMenuSeparator`
   - **BLOCKER**: Cannot replace actions menu without this

### High Priority (Ola 3/4 - Recommended for quality migration):

3. **`DataTable`** or Table pattern decision
   - **Decision needed**: ¿Crear componente DataTable genérico o migrar Table inline con DS styling?
   - **Recommendation**: Start with inline DS-styled table, promote to DataTable if pattern repeats 2+ times
   - **Not a blocker**: Can use Card + custom table markup with DS tokens as interim

4. **`SearchInput`** pattern
   - **Decision needed**: ¿Extender Input con `leftIcon` prop o crear componente separado?
   - **Recommendation**: Add `leftIcon`/`rightIcon` props to existing DS Input component
   - **Not a blocker**: Can use Input + manual icon positioning as interim

### No Blockers (Already exists):
- `PageShell`, `PageHeader`, `Section`, `Card`, `Button`, `Badge`, `Skeleton`, `EmptyState`, `InlineAlert`, `Toast`

---

## 10. Next Steps

1. **Implement Ola 3 DS components BEFORE dark iteration**:
   - `Dialog` (overlays/dialog.tsx) — Delete confirmation and other modals
   - `Dropdown` (overlays/dropdown.tsx) — Context menus for actions
2. **Design decisions**:
   - DataTable: ¿Genérico o inline? → Recommend inline first, extract if repeats
   - SearchInput: ¿Extend Input o separado? → Recommend extend Input with icon props
3. **Dark iteration** (after Ola 3 ready):
   - Replace page layout with PageShell + PageHeader
   - Migrate table to DS styling (or DataTable if implemented)
   - Implement loading Skeleton, EmptyState, error handling
   - Replace Dialog and Dropdown with DS versions
   - Remove hardcoded colors from avatar/logo
4. **Light iteration**: Verify contrast, shadows, and readability
5. **Cierre**: DoD verification + commit

---

---

## 10. Dark Iteration Implementation Notes

**Completed Changes:**

### Layout & Structure:
- ✅ Replaced custom page wrapper with `PageShell` + `PageHeader` + `Section`
- ✅ Added create button in PageHeader actions with `leftIcon` prop
- ✅ Removed old UI kit components completely (no imports from `@/components/ui/*`)

### Components Migrated to DS:
- ✅ `Button` — All buttons (create, actions, dialog) using DS with proper variants
- ✅ `Card` — Main content container with DS styling
- ✅ `Input` — Search input with semantic tokens
- ✅ `Badge` — Tenant slug display with `variant="outline"`
- ✅ `Dialog` — Delete confirmation using Ola 3 Dialog component
- ✅ `DropdownMenu` — Actions menu using Ola 3 Dropdown component
- ✅ `Skeleton` — Loading placeholders preserving layout (5 rows with avatar + text)
- ✅ `EmptyState` — No tenants / no search results with icon + CTA
- ✅ `InlineAlert` — Error state with retry button

### States Implemented:
- ✅ **Loading**: Skeleton rows with avatar placeholder + text skeletons (preserves exact layout)
- ✅ **Empty (no tenants)**: EmptyState with Building2 icon + "Create Tenant" CTA
- ✅ **Empty (no search results)**: EmptyState with search message, no CTA
- ✅ **Error (fetch failed)**: InlineAlert with error message + Retry button
- ✅ **Success (delete)**: Toast with success message
- ✅ **Error (delete failed)**: Toast with error message
- ✅ **Deleting**: Button shows loading state with `loading` prop

### Hardcoded Colors Removed:
- ✅ Avatar background: `bg-slate-100` → `bg-muted`
- ✅ Avatar text: `text-slate-700` → `text-foreground`
- ✅ Logo border: `border` → `border-border`
- ✅ Search icon: `text-muted-foreground` → `text-muted`
- ✅ Dropdown delete item: `text-destructive` → `text-danger`

### Accessibility Improvements:
- ✅ Search input has `aria-label="Search tenants"`
- ✅ Dropdown trigger has `aria-label` with tenant name
- ✅ Icons have `aria-hidden="true"`
- ✅ Tenant rows are keyboard navigable with `tabIndex={0}` and `onKeyDown` handler
- ✅ Proper `role="button"` on clickable tenant rows
- ✅ Focus rings with `focus-visible:ring-accent` + `ring-offset-background`

### UX Improvements:
- ✅ Hover state on tenant rows: `hover:bg-surface`
- ✅ Smooth transitions with `transition-all duration-200`
- ✅ Actions dropdown appears on hover/open with opacity transition
- ✅ Delete button uses `loading` prop instead of text change
- ✅ Empty state differentiates between "no tenants" and "no search results"
- ✅ Error state shows retry action instead of just message

### Design Decisions Made:
1. **Table pattern**: Used list-style layout with dividers instead of traditional table
   - Reason: Better responsive behavior, cleaner DS styling, no need for DataTable component
   - Pattern: `divide-y divide-border` with clickable rows
2. **Search pattern**: Used Input with manual icon positioning (`pl-9`)
   - Reason: No need for separate SearchInput component yet (not 2+ uses)
   - Can extract if pattern repeats in other pages

### Performance Notes:
- No heavy animations on list items (only subtle hover lift)
- Shadows applied to Card container, not individual rows
- Skeleton uses base shimmer animation (prefers-reduced-motion respected)

---

**Conclusion**: Page migrated to DS successfully. All Ola 3 components (Dialog, Dropdown) were already available. Dark iteration complete, ready for light verification.
