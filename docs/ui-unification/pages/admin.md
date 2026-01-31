# Page Audit — /admin (Dashboard)

**Status**: ✅ DONE

---

## 1. Purpose

The `/admin` page is the main dashboard that provides a system-wide overview for administrators. It displays critical system health metrics (status, version, cluster role, active key), detailed cluster information, component health status, tenant overview with quick access, and quick action shortcuts to other admin sections.

## 2. Primary Actions

- [ ] View system health and status in real-time (auto-refresh every 10s)
- [ ] Navigate to tenant details from the tenant list
- [ ] Create new tenant via "Crear Organización" button (when no tenants exist)
- [ ] Access quick actions: Manage Tenants, View Cluster, Metrics, OAuth Tools

## 3. Current UI Inventory

| Element | Component Used | Notes |
|---------|----------------|-------|
| Page wrapper | Custom `<div className="space-y-6">` | Missing consistent page shell pattern |
| Header | Custom `<h1>` + `<p>` | No PageHeader component, inline styling |
| Status cards (4x) | `Card` from `@/components/ui/card` | Old UI kit, not DS |
| Alert banner | `Alert` from `@/components/ui/alert` | Old UI kit, not DS |
| Badge indicators | `Badge` from `@/components/ui/badge` | Old UI kit, not DS |
| Action buttons | `Button` from `@/components/ui/button` | Old UI kit, not DS |
| Tenant list items | Custom `<Link>` with inline styles | No DS component for list items |
| Quick action cards | `Button asChild` with `variant="outline"` | Old UI kit, inconsistent pattern |
| Loading states | Inline text `{t.common.loading}` | No Skeleton component usage |

## 4. Data & States

| State | Currently Handled? | Notes |
|-------|-------------------|-------|
| Loading | ⚠️ Partial | Shows text "Loading..." instead of Skeleton |
| Empty (no tenants) | ✅ Yes | Shows message + "Crear Organización" CTA |
| Empty (no health data) | ⚠️ Partial | Shows "No hay datos disponibles" text |
| Error | ❌ No | No explicit error handling for failed API calls |
| Success | ✅ Yes | Data displays correctly when loaded |
| Degraded status | ✅ Yes | Alert banner shows degraded/unavailable status |

## 5. UX Issues Detected

1. **Inconsistent header pattern**: Uses custom `<div>` + `<h1>` instead of `PageHeader` DS component
2. **No loading skeletons**: Loading states show plain text instead of skeleton placeholders that maintain layout
3. **No error states**: API failures (e.g., `/readyz` or `/admin/tenants` fail) are not visually handled with retry actions
4. **Hardcoded colors**: Uses `text-green-500`, `text-yellow-500`, `text-red-500`, `bg-transparent` directly instead of semantic tokens
5. **Mixed UI kits**: Uses old `@/components/ui/*` components instead of new DS components
6. **Spacing inconsistency**: Page wrapper uses `space-y-6` which is not aligned with DS spacing tokens
7. **No page shell**: Missing `PageShell` wrapper for consistent padding/container behavior
8. **Status color logic in component**: `getStatusColor()` and `getStatusIcon()` functions hardcode colors, should use Badge variants
9. **Tenant list lacks empty state component**: When tenants exist but list is empty, should use `EmptyState` DS component
10. **Quick actions cards are buttons**: Using `Button` with `asChild` for navigation cards is semantically incorrect, should be custom DS pattern

## 6. Needed DS Components

### Already Available (Ola 1):
- [x] `PageShell` — exists
- [x] `PageHeader` — exists
- [x] `Section` — exists
- [x] `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter` — exists
- [x] `Button` — exists
- [x] `Badge` — exists
- [x] `Skeleton` — exists
- [x] `Toast` — exists (for error feedback)

### Missing (Need Ola 2):
- [ ] `InlineAlert` — For degraded/unavailable status banner (current `Alert` is from old UI kit)
- [ ] `EmptyState` — For "no data available" scenarios with icon + message + action

### Missing (Need Ola 3):
- [ ] `Link` wrapper or `NavCard` — For quick action cards (currently using `Button asChild` which is semantically awkward)

### Missing (Custom Pattern):
- [ ] `StatCard` or reuse `Card` — For the 4 stat cards (Status, Version, Role, Active Key). Can be done with existing `Card` + custom layout pattern, no new component needed.
- [ ] `KeyValueRow` — For cluster details rows (e.g., "Modo:", "Rol:", "Leader ID:"). Can be done with inline divs for now, but if pattern repeats in other pages, should become DS component.

## 7. Risks

- **API contract dependency**: Page relies on `/readyz` and `/v2/admin/tenants` endpoints. Changes to response shape will break rendering.
- **Auto-refresh logic**: `refetchInterval: 10000` on health query. Ensure this doesn't cause performance issues or excessive backend load.
- **Translations dependency**: Uses `useUIStore` and `getTranslations(locale)` for i18n. Ensure migration doesn't break translation keys.
- **CreateTenantWizard integration**: Uses `CreateTenantWizard` component that may also need DS migration. Don't migrate it in this page's scope, but verify it still works after page migration.
- **Link behavior**: Many `<Link>` components point to other admin pages. Ensure migration doesn't break navigation or styles.
- **Theme switching**: Page must work in both dark and light modes. Current hardcoded colors will need semantic token replacement.

## 8. Screenshots

**NOTE: As per project rules, screenshots are NOT required and should NOT be added to this document.**

---

## 9. Dark Iteration Implementation Notes

**Completed Changes:**
- ✅ Replaced custom page wrapper with `PageShell` + `PageHeader`
- ✅ Added refresh action button in PageHeader for refetching data
- ✅ Replaced old UI kit components with DS equivalents:
  - `Card`, `Badge`, `Button` from `@/components/ds`
  - Removed all imports from `@/components/ui/*`
- ✅ Implemented `InlineAlert` for degraded/unavailable status and error states with retry actions
- ✅ Implemented `EmptyState` for no-data scenarios (cluster, components, tenants)
- ✅ Replaced loading text with `Skeleton` components that preserve layout
- ✅ Added error handling with `isError` state and retry buttons
- ✅ Removed hardcoded colors:
  - `getStatusColor()` function removed (was using `bg-green-500`, etc)
  - Replaced with semantic Badge variants (`success`, `warning`, `danger`)
  - Icons use `text-muted` or `text-accent` semantic tokens
- ✅ Added `aria-hidden="true"` to all decorative icons
- ✅ Quick action cards use semantic tokens (`text-accent`, `border-border`, `bg-surface`)
- ✅ Improved focus states with `focus-visible:ring-accent` and `ring-offset-background`
- ✅ Tenant list items have hover lift effect and proper focus rings

**DS Patterns Used:**
- **PageShell + PageHeader + Section**: Consistent page layout with proper spacing
- **Card**: All content sections wrapped in Card with clay shadows
- **Skeleton**: Loading states preserve layout (stat cards, cluster details, components grid, tenant list)
- **InlineAlert**: System status alerts with variants and retry actions
- **EmptyState**: No-data scenarios with icons, messages, and CTAs
- **Badge**: Status indicators with semantic variants (success/warning/danger/outline)
- **Button**: Actions with variants (primary/secondary), sizes, and leftIcon support

**States Implemented:**
- **Loading**: Skeleton placeholders for all data sections
- **Empty**: EmptyState components for cluster, components, and tenants
- **Error**: InlineAlert with retry actions for health and tenants API failures
- **Success**: Data displays correctly with proper semantic styling
- **Degraded/Unavailable**: InlineAlert banner at top of page

**Accessibility Improvements:**
- All decorative icons have `aria-hidden="true"`
- Focus rings use `focus-visible:ring-2 ring-accent ring-offset-2 ring-offset-background`
- Proper semantic HTML structure maintained
- Tab order is logical (PageHeader actions → alerts → content sections)

**Performance:**
- No heavy animations on grid items (subtle hover lift only)
- Shadows applied to Card containers, not individual rows
- Skeleton animations use base shimmer (prefers-reduced-motion respected)

---

## 9. DS Gap Analysis

### Blockers (Ola 2 components needed BEFORE migration):
1. **InlineAlert** — Required for status banner (degraded/unavailable system status)
2. **EmptyState** — Required for "no data available" states

### Nice-to-have (can defer to later ola):
3. **NavCard** or **ActionCard** — For quick action buttons (can use existing `Card` + `Button` pattern as workaround)

### No Blockers (can use existing Ola 1):
- `PageShell`, `PageHeader`, `Section`, `Card`, `Button`, `Badge`, `Skeleton` are sufficient for most of the page.

---

## 10. Next Steps

1. **Implement Ola 2 DS components** (before dark iteration):
   - `InlineAlert` (feedback component, critical)
   - `EmptyState` (feedback component, high priority)
2. **Dark Iteration** (after Ola 2 ready):
   - Replace page layout with `PageShell` + `PageHeader`
   - Replace old UI components with DS equivalents
   - Implement Skeleton loading states
   - Add error handling with Toast/InlineAlert
   - Remove hardcoded colors, use semantic tokens
   - Verify theme switching works
3. **Light Iteration**:
   - Verify contrast and readability in light mode
   - Adjust tokens if needed
   - Screenshot and verify DoD
4. **Cierre**:
   - Update PROGRESS.md and WORKPLAN.md
   - Commit checkpoint: `page(admin): done + docs`

---

---

## 10. Light Iteration Verification

**Light Mode Quality Check:**
- ✅ **Contrast verified**: All text elements meet WCAG AA minimum
  - `text-foreground` (222 47% 11%) on `bg-background` (0 0% 100%) — excellent contrast
  - `text-muted-foreground` (220 9% 46%) readable on light backgrounds
  - Badge variants (success/warning/danger) use appropriate lightness values for readability
- ✅ **Shadows clay**: Light mode shadows have high-fidelity depth
  - `shadow-card`: Multi-layer with inset highlights (no flat appearance)
  - `shadow-float`: Enhanced depth on hover states
  - Cards maintain visual separation from background
- ✅ **Borders visible**: `border` (220 13% 91%) provides subtle but clear separation
- ✅ **Surface hierarchy**:
  - `background` (0 0% 100%) vs `card` (0 0% 100%) — same base with shadow separation
  - `surface` (220 14% 96%) provides hover state differentiation
- ✅ **Interactive states**:
  - Hover `bg-surface` (220 14% 96%) perceptible but subtle
  - Focus rings `ring-primary` (254 75% 64%) highly visible
  - `ring-offset-background` ensures proper separation
- ✅ **Empty states & alerts**:
  - `InlineAlert` backgrounds (`bg-warning/10`, `bg-danger/10`) visible and appropriate
  - `EmptyState` icons with `text-muted opacity-50` balanced visibility
- ✅ **Semantic tokens verified**:
  - Accent colors have appropriate lightness for light backgrounds
  - All HSL opacity modifiers work correctly (verified in hardening)

**No Token Changes Required**: Light mode tokens are well-balanced and provide premium appearance without being "washed out".

---

## 11. Final Status

**Migration Complete:** ✅ DONE

**What Changed:**
- Migrated from old UI kit (`@/components/ui/*`) to Design System (`@/components/ds`)
- Implemented `PageShell` + `PageHeader` + `Section` layout pattern
- Added comprehensive state handling: Loading (Skeleton), Empty (EmptyState), Error (InlineAlert with retry)
- Removed all hardcoded colors, using only semantic tokens
- Improved accessibility with ARIA labels and focus rings
- Added refresh functionality to PageHeader actions

**DS Components Used:**
- Ola 1: PageShell, PageHeader, Section, Card, Button, Badge, Skeleton
- Ola 2 (new): InlineAlert, EmptyState

**Known Issues:** None

**Theme Support:** ✅ Dark & Light both verified and working
