# Page Audit — /admin/users

**Status**: 🔍 AUDIT

---

## 1. Purpose

The `/admin/users` page is the comprehensive user management interface for a tenant. It displays a paginated, searchable table of all users with advanced filtering, bulk actions, and detailed management capabilities. Users can create, edit, block, delete users, manage email verification, change passwords, view detailed user information, and configure custom user fields. The page has two main tabs: "Lista de Usuarios" (user list with CRUD operations) and "Campos Personalizados" (custom field configuration for user registration).

## 2. Primary Actions

- [ ] Search/filter users by email or ID (client-side debounced search)
- [ ] Create new user via "Crear Usuario" dialog (opens CreateUserForm)
- [ ] View user details via dropdown menu → "Ver detalles" (opens UserDetails dialog)
- [ ] Edit user via dropdown menu → "Editar" (opens EditUserForm dialog)
- [ ] Block/suspend user via dropdown menu → "Bloquear" (opens BlockUserDialog with reason + duration)
- [ ] Unblock user via dropdown menu → "Desbloquear" (immediate action with confirmation)
- [ ] Delete user via dropdown menu → "Eliminar" (confirmation dialog)
- [ ] Verify email manually via dropdown menu → "Verificar email" (toggle action)
- [ ] Change password via dropdown menu → "Cambiar contraseña" (inline prompt)
- [ ] Bulk select users (checkbox column)
- [ ] Bulk block selected users (bulk action bar)
- [ ] Bulk delete selected users (bulk action bar)
- [ ] Export users to JSON or CSV via dropdown menu
- [ ] Navigate to database configuration if tenant has no DB (CTA button)
- [ ] Refresh user list (refresh button in toolbar)
- [ ] Paginate through users (pagination controls at bottom)
- [ ] Configure custom user fields in "Campos Personalizados" tab (add/edit/delete field definitions)

## 3. Current UI Inventory

| Element | Component Used | Notes |
|---------|----------------|-------|
| Page wrapper | Custom `<div className="space-y-6 animate-in fade-in duration-500">` | Missing PageShell pattern |
| Header | Custom flex layout with gradient background decorations | No PageHeader component, uses hardcoded gradient decorations with `bg-purple-400/20`, `from-purple-500/10` |
| Info banner | `Alert` from `@/components/ui/alert` | Old UI kit, uses hardcoded `from-purple-50 to-indigo-50`, `border-purple-200` |
| Tabs | `Tabs` from `@/components/ui/tabs` | Old UI kit, needs DS Tabs component |
| Stats cards (4) | Custom `StatCard` component | Uses hardcoded colors: `bg-blue-500/10 text-blue-600`, `bg-green-500/10`, `bg-amber-500/10`, `bg-red-500/10` |
| Search bar | `Input` from `@/components/ui/input` + `Search` icon | Old UI kit, inside toolbar |
| Refresh button | `Button` from `@/components/ui/button` | Old UI kit, not DS |
| Export dropdown | `DropdownMenu` from `@/components/ui/dropdown-menu` | Old UI kit (Radix headless OK, but styling needs DS) |
| Create button | `Button` from `@/components/ui/button` | Old UI kit, not DS |
| Bulk action bar | Custom flex layout with `Checkbox` + `Button` | Appears on selection, uses old UI buttons |
| Table | `Table` from `@/components/ui/table` | Old UI kit, needs DS DataTable or list pattern |
| User row avatar | Custom div with hardcoded gradients | Uses `from-purple-100 to-indigo-100 dark:from-purple-900/30` |
| User status badge | `Badge` from `@/components/ui/badge` | Old UI kit, uses hardcoded `bg-green-600 hover:bg-green-700`, `text-red-500` |
| Verification status | Custom div with icons | Uses hardcoded `text-green-600`, `text-amber-600` |
| Row actions dropdown | `DropdownMenu` from `@/components/ui/dropdown-menu` | Old UI kit |
| Create dialog | `Dialog` from `@/components/ui/dialog` | Old UI kit, needs DS Dialog |
| Edit dialog | `Dialog` from `@/components/ui/dialog` | Old UI kit, needs DS Dialog |
| Details dialog | `Dialog` from `@/components/ui/dialog` | Old UI kit, needs DS Dialog, shows tabs for user info/activity/raw JSON |
| Block dialog | Custom `BlockUserDialog` component | Uses old UI Dialog |
| Bulk action dialog | `Dialog` confirmation | Old UI kit |
| Form inputs | `Input`, `Label`, `Select`, `Switch`, `Checkbox`, `Textarea` from `@/components/ui/*` | All old UI kit |
| Phone input | `PhoneInput` from `@/components/ui/phone-input` | Old UI kit custom component |
| Country select | `CountrySelect` from `@/components/ui/country-select` | Old UI kit custom component |
| Tooltip | `Tooltip` from `@/components/ui/tooltip` | Old UI kit (Radix headless OK, but styling needs DS) |
| Loading spinner | `Loader2` icon inside TableCell | No Skeleton component preserving layout |
| Empty state | TableCell with custom div + icon + text | No EmptyState component |
| No database CTA | Custom flex layout with gradient decorations | Uses hardcoded `from-amber-400/20`, `from-amber-50 to-orange-50`, needs EmptyState pattern |
| Pagination controls | Custom pagination component | Not using DS Pagination (if exists) |
| Field settings table | Custom table in "Campos Personalizados" tab | Uses old UI Table |

## 4. Data & States

| State | Currently Handled? | Notes |
|-------|-------------------|-------|
| Loading | ⚠️ Partial | Shows `Loader2` spinner in TableCell instead of Skeleton that preserves layout |
| Empty (no users) | ⚠️ Partial | Shows custom empty message in TableCell, not proper EmptyState component |
| Empty (no search results) | ⚠️ Partial | Same as above, shows hint to try another search term |
| Empty (no database) | ✅ Yes | Shows custom CTA to configure database (but uses hardcoded colors, needs EmptyState) |
| Error (fetch failed) | ❌ No | No error handling for failed user fetch (no InlineAlert) |
| Error (delete failed) | ✅ Yes | Toast with error message |
| Error (create failed) | ✅ Yes | Toast with error message |
| Error (update failed) | ✅ Yes | Toast with error message |
| Error (block failed) | ✅ Yes | Toast with error message |
| Success (create) | ✅ Yes | Toast with success message |
| Success (update) | ✅ Yes | Toast with success message |
| Success (delete) | ✅ Yes | Toast with success message |
| Success (block) | ✅ Yes | Toast with success message |
| Success (enable) | ✅ Yes | Toast with success message |
| Success (email verified) | ✅ Yes | Toast with success message |
| Success (password changed) | ✅ Yes | Toast with success message |
| Success (bulk block) | ✅ Yes | Toast with success message |
| Success (bulk delete) | ✅ Yes | Toast with success message |
| Pending (create) | ✅ Yes | Dialog form shows pending state on submit button |
| Pending (update) | ✅ Yes | Dialog form shows pending state on submit button |
| Pending (delete) | ✅ Yes | Button shows loading state |
| Pending (block) | ✅ Yes | Dialog shows pending state on submit button |
| Pending (bulk) | ✅ Yes | Buttons show loading state |
| Bulk selection | ✅ Yes | Bulk action bar appears with selected count, toggle all checkbox works |
| Pagination | ✅ Yes | Pagination controls work with page/pageSize state |

## 5. UX Issues Detected

1. **No PageShell/PageHeader**: Uses custom header layout with hardcoded gradient decorations inconsistent with other migrated pages
2. **Hardcoded colors everywhere**: Extensive use of `purple-500/10`, `indigo-500/10`, `blue-500/10`, `green-600`, `red-500`, `amber-600` instead of semantic tokens
3. **Search is debounced**: Good pattern (300ms delay), but not using DS component
4. **Loading shows spinner**: Should use Skeleton that preserves table layout (no jump when data loads)
5. **Empty state is text-only**: No EmptyState component with icon + CTA for no users scenario
6. **No error handling for fetch**: If `/v2/admin/tenants/${tenantId}/users` fails, page shows nothing or crashes (no InlineAlert)
7. **Stats cards use hardcoded colors**: `StatCard` component has hardcoded variant colors instead of semantic tokens
8. **Table uses old UI kit**: Needs migration to DS DataTable (or list pattern like /admin/tenants)
9. **All dialogs use old UI kit**: Create, Edit, Details, Block dialogs all need DS Dialog component
10. **All form components use old UI kit**: Input, Label, Select, Switch, Checkbox, Textarea all need DS versions
11. **Dropdown menus use old UI kit**: Export dropdown and row actions dropdown need DS styling
12. **Tooltip uses old UI kit**: InfoTooltip component needs DS Tooltip
13. **Tabs use old UI kit**: TabsList, TabsTrigger, TabsContent need DS Tabs component
14. **Badge uses hardcoded colors**: Status badges use `bg-green-600`, `text-red-500` instead of semantic variants
15. **Avatar uses hardcoded gradients**: User row avatar uses `from-purple-100 to-indigo-100` instead of semantic tokens
16. **No database CTA uses hardcoded colors**: Orange/amber gradients and shadows instead of semantic tokens
17. **Verification status uses hardcoded colors**: `text-green-600`, `text-amber-600` instead of semantic tokens
18. **Pagination component not DS**: Custom pagination needs DS Pagination component (if exists) or inline with DS styling
19. **PhoneInput and CountrySelect**: Custom components need DS versions or removal
20. **Very large page (2,205 lines)**: May need splitting into smaller components for maintainability
21. **Bulk action bar animation**: Uses `animate-in slide-in-from-top-2` (Tailwind animate), needs prefers-reduced-motion support check
22. **Alert banner uses hardcoded colors**: Info banner uses `from-purple-50 to-indigo-50`, `border-purple-200` instead of semantic tokens
23. **Export functionality**: Exports users to JSON/CSV but no visual feedback during export (should show loading state)
24. **Field settings tab**: "Campos Personalizados" tab uses separate table with old UI components, needs migration too

## 6. Needed DS Components

### Already Available (Ola 1):
- [x] `PageShell` — exists
- [x] `PageHeader` — exists
- [x] `Section` — exists
- [x] `Card` — exists (but current Card is old UI kit)
- [x] `Button` — exists
- [x] `Badge` — exists (but needs semantic variants, not hardcoded colors)
- [x] `Input` — exists
- [x] `Skeleton` — exists
- [x] `Toast` — exists (already used for success/error messages)

### Already Available (Ola 2):
- [x] `InlineAlert` — exists (for error states if needed)
- [x] `EmptyState` — exists (for no users / no search results / no database)

### Already Available (Ola 3):
- [x] `Dialog` — exists (for create, edit, details, block, bulk confirmations)
- [x] `DropdownMenu` — exists (for export menu and row actions)

### Missing (Need Ola 3/4 - Forms & Data):
- [ ] **`Tabs`** — For switching between "Lista de Usuarios" and "Campos Personalizados"
  - **Carpeta destino**: `ui/components/ds/navigation/tabs.tsx`
  - **Responsabilidad**: Tabbed navigation with keyboard support
  - **Props mínimas**: `value`, `onValueChange`, `children` (TabsList, TabsTrigger, TabsContent)
  - **Base**: Radix UI Tabs (headless) with DS styling
  - **Uso en página**: Main tabs at line 256-275
  - **Regla de 2 usos**: ✅ Sí (UserDetails dialog also has tabs at line ~1500+, likely appears in other admin pages)

- [ ] **`Label`** — For form field labels
  - **Carpeta destino**: `ui/components/ds/core/label.tsx`
  - **Responsabilidad**: Accessible form labels with proper htmlFor association
  - **Props mínimas**: `htmlFor`, `children`, `required?`, `className`
  - **Base**: Radix UI Label (headless) with DS styling
  - **Uso en página**: CreateUserForm (line 1130+), EditUserForm (line 1276+), field settings form
  - **Regla de 2 usos**: ✅ Sí (appears in all forms across admin panel)

- [ ] **`Select`** — For dropdown selections
  - **Carpeta destino**: `ui/components/ds/core/select.tsx`
  - **Responsabilidad**: Dropdown select with keyboard navigation and search
  - **Props mínimas**: `value`, `onValueChange`, `items[]`, `placeholder`, `className`
  - **Base**: Radix UI Select (headless) with DS styling
  - **Uso en página**: CreateUserForm (client selection, custom fields), pagination pageSize selector
  - **Regla de 2 usos**: ✅ Sí (appears in forms and filters across admin panel)

- [ ] **`Switch`** — For boolean toggles
  - **Carpeta destino**: `ui/components/ds/core/switch.tsx`
  - **Responsabilidad**: Toggle switch for boolean values
  - **Props mínimas**: `checked`, `onCheckedChange`, `disabled?`, `className`
  - **Base**: Radix UI Switch (headless) with DS styling
  - **Uso en página**: CreateUserForm (email_verified, disabled fields), field settings (required, unique, indexed)
  - **Regla de 2 usos**: ✅ Sí (appears in forms across admin panel)

- [ ] **`Checkbox`** — For boolean selections and bulk actions
  - **Carpeta destino**: `ui/components/ds/core/checkbox.tsx`
  - **Responsabilidad**: Checkbox for individual selection and bulk select-all
  - **Props mínimas**: `checked`, `onCheckedChange`, `disabled?`, `className`, `indeterminate?`
  - **Base**: Radix UI Checkbox (headless) with DS styling
  - **Uso en página**: Table header select-all (line 743), table row selection (line 1030), bulk action bar (line 694)
  - **Regla de 2 usos**: ✅ Sí (appears in tables and forms across admin panel)

- [ ] **`Textarea`** — For multi-line text input
  - **Carpeta destino**: `ui/components/ds/core/textarea.tsx`
  - **Responsabilidad**: Multi-line text input with auto-resize option
  - **Props mínimas**: `value`, `onChange`, `placeholder`, `rows?`, `disabled?`, `className`
  - **Uso en página**: BlockUserDialog (block reason line ~1850+), possibly custom fields with type "text"
  - **Regla de 2 usos**: ✅ Sí (appears in forms across admin panel)

- [ ] **`Tooltip`** — For help text and info icons
  - **Carpeta destino**: `ui/components/ds/overlays/tooltip.tsx`
  - **Responsabilidad**: Contextual help tooltips
  - **Props mínimas**: `content`, `children`, `side?`, `className`
  - **Base**: Radix UI Tooltip (headless) with DS styling
  - **Uso en página**: InfoTooltip component (line 166-179), used for field descriptions
  - **Regla de 2 usos**: ✅ Sí (appears across admin panel for contextual help)

- [ ] **`Pagination`** — For table pagination controls
  - **Carpeta destino**: `ui/components/ds/data/pagination.tsx`
  - **Responsabilidad**: Page navigation with prev/next/first/last buttons and page size selector
  - **Props mínimas**: `page`, `totalPages`, `pageSize`, `onPageChange`, `onPageSizeChange`, `totalCount`, `className`
  - **Uso en página**: Bottom of user list (likely after line 800+)
  - **Regla de 2 usos**: ✅ Sí (appears in all paginated tables across admin panel)

### Missing (Custom components to evaluate):
- [ ] **`PhoneInput`** — Custom phone number input with country code
  - **Decision needed**: ¿Migrar a DS o usar Input con pattern validation?
  - **Uso en página**: CreateUserForm, EditUserForm (for custom fields with type "phone")
  - **Recommendation**: Start with standard Input + regex validation, promote to DS if pattern repeats 3+ times

- [ ] **`CountrySelect`** — Custom country dropdown
  - **Decision needed**: ¿Migrar a DS Select con country data o usar Select genérico?
  - **Uso en página**: CreateUserForm, EditUserForm (for custom fields with type "country")
  - **Recommendation**: Use DS Select with country options array, no need for separate component

- [ ] **`DataTable`** or Table pattern decision
  - **Decision needed**: ¿Crear componente DataTable genérico o migrar Table inline con DS styling?
  - **Recommendation**: Given complexity (pagination, bulk actions, sorting, filtering), consider creating DS DataTable component OR use list pattern like /admin/tenants (dividers instead of table)
  - **Not a blocker**: Can use Card + custom table markup with DS tokens as interim

### No Blockers (For later phases if needed):
- **DebouncedInput**: Current implementation uses `useEffect` for debounce (lines 308-314), no separate component needed
- **StatCard**: Can be inline with semantic tokens, no need for separate DS component (unless pattern repeats 3+ times)

## 7. Risks

- **API contract dependencies**: Page relies on multiple endpoints:
  - `GET /v2/admin/tenants/${tenantId}/users` (with pagination params)
  - `POST /v2/admin/tenants/${tenantId}/users` (create)
  - `PUT /v2/admin/tenants/${tenantId}/users/${userId}` (update)
  - `DELETE /v2/admin/tenants/${tenantId}/users/${userId}` (delete)
  - `POST /v2/admin/tenants/${tenantId}/users/${userId}/disable` (block)
  - `POST /v2/admin/tenants/${tenantId}/users/${userId}/enable` (unblock)
  - `POST /v2/admin/tenants/${tenantId}/users/${userId}/set-email-verified` (verify email)
  - `POST /v2/admin/tenants/${tenantId}/users/${userId}/set-password` (change password)
  - `GET /v2/admin/tenants/${tenantId}` (fetch tenant settings for custom fields)
  - `GET /v2/admin/clients` (fetch clients for user creation)
  - Changes to any response shape will break rendering
- **Very large file (2,205 lines)**: Migration will be complex and time-consuming, high risk of breaking functionality
- **Complex state management**: Multiple dialogs, bulk selection, pagination, tabs, debounced search - ensure no regressions
- **Custom fields logic**: Dynamic form fields based on `tenant.settings.userFields[]` - must preserve flexibility
- **Bulk actions with Promise.allSettled**: Ensure error handling for partial failures remains intact
- **Export functionality**: JSON/CSV export logic must remain functional
- **Pagination with server-side data**: Ensure page/pageSize state sync with backend doesn't break
- **User blocking with duration**: Block dialog supports temporary suspension with `disabled_until` field - preserve logic
- **Email verification fallback**: `setEmailVerifiedMutation` has fallback toast if endpoint doesn't exist (line 459-461) - keep fallback
- **No database detection**: Special handling for `TENANT_NO_DATABASE` error (status 424) - must preserve this flow
- **Field definitions in tenant settings**: UserFieldsSettings component modifies `tenant.settings.userFields` - ensure PUT /tenants/:id works
- **User row actions navigation**: Multiple dropdown actions must work correctly (details/edit/block/delete/verify/password)
- **Bulk selection state**: `Set<string>` for selected IDs - ensure toggle logic remains correct
- **Debounce logic**: Search debounce with `useEffect` cleanup - preserve to avoid performance issues
- **Stats computation**: Memoized stats calculation from user list (active/blocked/verified counts) - ensure useMemo dependencies correct
- **Table sorting**: Not currently implemented, but may be expected - clarify requirements
- **Responsive table**: Hidden columns on mobile (lg:table-cell, md:table-cell) - preserve responsive behavior
- **Avatar initials logic**: Uses `email.slice(0, 2).toUpperCase()` - preserve or improve
- **Date formatting**: Uses Intl.DateTimeFormat with try/catch - preserve locale handling
- **UserDetails dialog tabs**: Shows "Información", "Actividad", "JSON Raw" tabs - complex dialog needs careful migration
- **Hardcoded i18n strings**: Many strings are hardcoded in Spanish ("Usuarios", "Crear Usuario", etc.) instead of using `t()` - low priority fix
- **PhoneInput and CountrySelect**: Custom components may not have DS equivalents - decide migration strategy
- **Tenant ID from URL params**: Uses `useSearchParams` and `useParams` to get tenantId - ensure routing works after migration
- **Query key structure**: TanStack Query keys like `["users", tenantId, page, pageSize, debouncedSearch]` - preserve for caching
- **Optimistic updates**: Uses `queryClient.invalidateQueries` for cache invalidation - ensure refetch logic works
- **Toast variant usage**: Uses `variant: "success"` and `variant: "destructive"` - verify DS Toast supports these variants

---

## 8. Screenshots

**NOTE: As per project rules, screenshots are NOT required and should NOT be added to this document.**

---

## 9. DS Gap Analysis

### Critical Blockers (Must implement BEFORE dark iteration):

1. **`Tabs`** — Required for main page navigation between "Lista de Usuarios" and "Campos Personalizados"
   - Base: Radix UI Tabs (headless)
   - Styling: DS semantic tokens, proper keyboard navigation
   - API: `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent`
   - **BLOCKER**: Cannot migrate main page layout without this

2. **`Label`** — Required for all form fields
   - Base: Radix UI Label (headless)
   - Styling: DS semantic tokens, required indicator
   - API: `Label` with `htmlFor`, `required`, `className`
   - **BLOCKER**: Cannot migrate CreateUserForm/EditUserForm without this

3. **`Select`** — Required for dropdowns in forms and pagination
   - Base: Radix UI Select (headless)
   - Styling: DS semantic tokens, proper z-index, shadows
   - API: `Select`, `SelectTrigger`, `SelectValue`, `SelectContent`, `SelectItem`
   - **BLOCKER**: Cannot migrate forms and pagination without this

4. **`Switch`** — Required for boolean toggles in forms
   - Base: Radix UI Switch (headless)
   - Styling: DS semantic tokens, focus rings
   - API: `Switch` with `checked`, `onCheckedChange`, `disabled`
   - **BLOCKER**: Cannot migrate CreateUserForm/EditUserForm without this

5. **`Checkbox`** — Required for bulk selection and form fields
   - Base: Radix UI Checkbox (headless)
   - Styling: DS semantic tokens, indeterminate state
   - API: `Checkbox` with `checked`, `onCheckedChange`, `indeterminate`
   - **BLOCKER**: Cannot migrate table bulk actions without this

6. **`Textarea`** — Required for multi-line inputs
   - Styling: DS semantic tokens, auto-resize option
   - API: `Textarea` with standard input props
   - **BLOCKER**: Cannot migrate BlockUserDialog reason field without this

### High Priority (Recommended for quality migration):

7. **`Tooltip`** — Recommended for InfoTooltip component
   - Base: Radix UI Tooltip (headless)
   - Styling: DS semantic tokens, proper z-index
   - API: `Tooltip`, `TooltipTrigger`, `TooltipContent`, `TooltipProvider`
   - **Not a blocker**: Can temporarily remove tooltips or use old UI version

8. **`Pagination`** — Recommended for table navigation
   - Styling: DS semantic tokens, responsive layout
   - API: `Pagination` with page/pageSize/totalPages props
   - **Not a blocker**: Can use inline buttons with DS styling as interim

9. **`DataTable`** or Table pattern decision
   - **Decision needed**: ¿Crear componente DataTable genérico con pagination/bulk actions/sorting o usar list pattern?
   - **Recommendation**: Given /admin/tenants precedent (list pattern), consider migrating to list-style layout with dividers
   - **Alternative**: Create DS DataTable if table structure is preferred (more work, more reusable)
   - **Not a blocker**: Can use Card + custom markup with DS tokens as interim

### No Blockers (Already exists):
- `PageShell`, `PageHeader`, `Section`, `Card`, `Button`, `Input`, `Badge`, `Skeleton`, `EmptyState`, `InlineAlert`, `Toast`, `Dialog`, `DropdownMenu`

---

## 10. Next Steps

1. **Implement missing DS form components (Ola 4 - Forms)** BEFORE dark iteration:
   - `Label` (core/label.tsx) — Form labels with accessibility
   - `Select` (core/select.tsx) — Dropdown selects
   - `Switch` (core/switch.tsx) — Toggle switches
   - `Checkbox` (core/checkbox.tsx) — Checkboxes with indeterminate state
   - `Textarea` (core/textarea.tsx) — Multi-line text input

2. **Implement missing DS navigation/overlay components** BEFORE dark iteration:
   - `Tabs` (navigation/tabs.tsx) — Tabbed navigation
   - `Tooltip` (overlays/tooltip.tsx) — Contextual help tooltips

3. **Consider implementing DS data components**:
   - `Pagination` (data/pagination.tsx) — Table pagination controls
   - `DataTable` (data/data-table.tsx) — OR decide on list pattern like /admin/tenants

4. **Design decisions**:
   - Table vs List: ¿Keep table structure or migrate to list pattern with dividers (like /admin/tenants)?
     - **Recommendation**: List pattern is more responsive and cleaner with DS, but table provides better data density
     - **Suggest**: Try list pattern first given precedent, can revert to table if UX suffers
   - PhoneInput/CountrySelect: ¿Migrate to DS or simplify to standard Input/Select?
     - **Recommendation**: Use standard DS Input with pattern validation for phone, DS Select with country array for country
   - StatCard: ¿Extract to DS component or keep inline?
     - **Recommendation**: Keep inline with semantic tokens unless pattern repeats 3+ times

5. **Dark iteration** (after Ola 4 ready):
   - Replace page layout with PageShell + PageHeader
   - Remove all hardcoded colors (purple, indigo, blue, green, red, amber gradients)
   - Migrate forms to DS components (Label, Input, Select, Switch, Checkbox, Textarea)
   - Migrate dialogs to DS Dialog (Create, Edit, Details, Block, Bulk confirmations)
   - Migrate table to DS pattern (DataTable or list with dividers)
   - Implement loading Skeleton preserving layout, EmptyState, error InlineAlert
   - Replace Tabs, Tooltip, Pagination with DS versions
   - Remove hardcoded badge colors, use semantic variants
   - Remove hardcoded avatar gradients, use semantic tokens
   - Remove hardcoded alert banner gradients, use semantic InlineAlert
   - Ensure all animations respect prefers-reduced-motion
   - Preserve all functionality: debounced search, pagination, bulk actions, custom fields, export

6. **Light iteration**: Verify contrast, shadows, and readability across all components and states

7. **Cierre**: DoD verification + commit

---

**Complexity Assessment**: ⚠️ **HIGH** — This page is 2,205 lines with extensive functionality (2 tabs, 5+ dialogs, bulk actions, pagination, custom fields, export). Migration will require significant time and care. Recommend breaking into smaller tasks:
- Task 1: Implement missing DS components (Ola 4)
- Task 2: Migrate main page layout + header
- Task 3: Migrate user list table/layout
- Task 4: Migrate Create/Edit forms
- Task 5: Migrate Details/Block dialogs
- Task 6: Migrate Field Settings tab
- Task 7: Final polish + testing

**Estimated Effort**: 3-4 days (vs 1 day for /admin/tenants)
