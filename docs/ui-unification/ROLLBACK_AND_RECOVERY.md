# 🔄 ROLLBACK AND RECOVERY - /admin/users

**Propósito**: Documentar funcionalidades críticas de `/admin/users` que NO deben perderse durante la re-migración.

**Fecha**: 2026-01-31
**Status**: Active Reference

---

## 📋 FUNCIONALIDADES CRÍTICAS

### 1. Search & Filtering

**Componente**: Search input con debounce

**Comportamiento**:
- Debounce: 300ms
- Búsqueda por: email, nombre, ID
- Query param: `?search=...`
- Clear button visible cuando hay texto

**Estado**:
```typescript
const [searchTerm, setSearchTerm] = useState("")
const debouncedSearch = useDebounce(searchTerm, 300)
```

**NO perder**:
- Debounce timing
- Query param sync
- Clear functionality

---

### 2. Pagination

**Componente**: Pagination controls (prev, next, page numbers)

**Comportamiento**:
- Page size options: 10, 25, 50, 100
- Query params: `?page=1&pageSize=25`
- Total count display
- Prev/Next buttons con disabled states

**Estado**:
```typescript
const [page, setPage] = useState(1)
const [pageSize, setPageSize] = useState(25)
```

**NO perder**:
- Page size selector
- Query param sync
- Total count display
- Disabled states correctos

---

### 3. Bulk Selection

**Componente**: Checkbox en header + checkbox por row

**Comportamiento**:
- Select all (header checkbox)
- Select individual (row checkbox)
- Indeterminate state cuando algunos seleccionados
- Count display: "3 selected"

**Estado**:
```typescript
const [selectedUsers, setSelectedUsers] = useState<Set<string>>(new Set())
const allSelected = selectedUsers.size === users.length && users.length > 0
const someSelected = selectedUsers.size > 0 && !allSelected
```

**NO perder**:
- Indeterminate state visual
- Select all functionality
- Count display

---

### 4. Bulk Actions

**Componente**: Dropdown con acciones bulk

**Opciones**:
- Block selected users
- Delete selected users
- Export selected (JSON, CSV)

**Comportamiento**:
- Solo visible cuando `selectedUsers.size > 0`
- Confirmación antes de destructive actions
- Clear selection después de acción

**NO perder**:
- Visibility condicional
- Confirmación dialogs
- Clear selection post-action

---

### 5. Export Functionality

**Componente**: Export dropdown

**Opciones**:
- Export all to JSON
- Export all to CSV
- Export selected to JSON
- Export selected to CSV

**Comportamiento**:
- Download archivo con timestamp
- Filename: `users-export-2026-01-31.json`
- Incluye todos los campos

**NO perder**:
- Export format options
- Timestamp en filename
- Todos los campos exportados

---

### 6. Create User

**Componente**: Dialog con form

**Campos**:
- Email (required, validation)
- Password (required, strength indicator)
- Name (optional)
- **Phone** (PhoneInput component) ⚠️
- **Country** (CountrySelect component) ⚠️
- Custom fields (dynamic based on tenant config)

**Validación**:
- Email format
- Password strength (min 8 chars, uppercase, lowercase, number)
- Phone number validation (libphonenumber-js)

**Comportamiento**:
- Auto-login después de crear (si `REGISTER_AUTO_LOGIN=true`)
- Toast notification on success
- Clear form on success
- Close dialog on success

**NO perder**:
- PhoneInput component ⚠️ CRÍTICO
- CountrySelect component ⚠️ CRÍTICO
- Custom fields support
- Validation rules
- Auto-login behavior

---

### 7. Edit User

**Componente**: Dialog con form pre-filled

**Campos**:
- Email (editable)
- Name (editable)
- **Phone** (PhoneInput component) ⚠️
- **Country** (CountrySelect component) ⚠️
- Custom fields (editable)
- Role (si RBAC enabled)
- Status (active, disabled)

**Comportamiento**:
- Pre-fill con datos actuales
- Validación en submit
- Toast notification on success
- Refetch users list on success

**NO perder**:
- PhoneInput component ⚠️ CRÍTICO
- CountrySelect component ⚠️ CRÍTICO
- Custom fields editable
- Pre-fill functionality

---

### 8. Delete User

**Componente**: Confirmation dialog

**Comportamiento**:
- Confirmación con user email display
- Warning message: "This action cannot be undone"
- Input para confirmar email (double-check)
- Toast notification on success
- Refetch users list on success

**NO perder**:
- Double-check con email input
- Warning message
- Refetch post-delete

---

### 9. Block User

**Componente**: Dialog con form

**Campos**:
- Reason (required, textarea)
- Duration (select: 1 day, 7 days, 30 days, permanent)
- Notify user (checkbox)

**Comportamiento**:
- User status → "blocked"
- Block expiry timestamp calculado
- Email notification si checkbox checked
- Toast notification on success
- Refetch users list on success

**NO perder**:
- Reason field
- Duration options
- Notify checkbox
- Expiry calculation

---

### 10. Unblock User

**Componente**: Confirmation dialog

**Comportamiento**:
- Confirmación simple
- User status → "active"
- Clear block reason y expiry
- Toast notification on success
- Refetch users list on success

**NO perder**:
- Status update
- Clear block metadata

---

### 11. Verify Email

**Componente**: Action button/menu item

**Comportamiento**:
- Mark email as verified
- `email_verified` → true
- `email_verified_at` → now
- Toast notification on success
- Visual indicator update (badge)

**NO perder**:
- Timestamp update
- Visual indicator change

---

### 12. Custom Fields Tab

**Componente**: Tabs (Overview, Custom Fields)

**Custom Fields Tab Contenido**:
- List of field definitions
- Add field definition button
- Remove field definition button
- Field properties:
  - Name (string)
  - Type (select: string, number, boolean, date, email, url)
  - Required (checkbox)
  - Default value (input)

**Comportamiento**:
- CRUD operations on field definitions
- Field definitions stored in tenant config
- Used in create/edit user forms
- Toast notifications on success

**NO perder**:
- Full CRUD functionality
- Type selector options
- Required checkbox
- Default value support

---

### 13. No Database Detection

**Componente**: EmptyState

**Trigger**: API returns status 424 (Failed Dependency)

**Display**:
- Icon: DatabaseOff
- Title: "No database configured"
- Message: "This tenant doesn't have a database configured. Configure a database in tenant settings to manage users."
- Action button: "Go to Tenant Settings"

**Comportamiento**:
- Mostrar en lugar de user list
- Ocultar actions (create, bulk, etc.)
- Link a tenant settings

**NO perder**:
- Status 424 detection
- EmptyState display
- Settings link

---

## 🚨 COMPONENTES PERDIDOS (RECUPERAR)

### PhoneInput

**Ubicación original**: `ui/components/ds/forms/phone-input.tsx` (NO EXISTE)

**Features requeridas**:
- Country code selector (dropdown con banderas)
- Phone number input
- Validation con libphonenumber-js
- Format as-you-type
- Error display
- Disabled state support

**Props interface**:
```typescript
interface PhoneInputProps {
  value?: string
  onChange?: (value: string) => void
  defaultCountry?: CountryCode
  error?: string
  disabled?: boolean
  className?: string
}
```

**Dependencia**: `libphonenumber-js`

---

### CountrySelect

**Ubicación original**: `ui/components/ds/forms/country-select.tsx` (NO EXISTE)

**Features requeridas**:
- Country list (ISO 3166-1 alpha-2)
- Flag emojis
- Popular countries at top
- Search/filter support (opcional pero nice-to-have)
- Error display
- Disabled state support

**Props interface**:
```typescript
interface CountrySelectProps {
  value?: string
  onChange?: (value: string) => void
  error?: string
  disabled?: boolean
  className?: string
  placeholder?: string
}
```

---

## 📊 ESTADÍSTICAS (Stats Cards)

**Componentes**: 4 cards en grid

**Métricas**:
1. **Total Users**
   - Count: total users
   - Icon: Users
   - Variant: default

2. **Active Users**
   - Count: users con status=active
   - Icon: UserCheck
   - Variant: success

3. **Blocked Users**
   - Count: users con status=blocked
   - Icon: UserX
   - Variant: destructive

4. **Unverified Emails**
   - Count: users con email_verified=false
   - Icon: Mail
   - Variant: warning

**Layout**: Grid responsive (1 col mobile, 2 cols tablet, 4 cols desktop)

**NO perder**:
- Todas las 4 métricas
- Iconos correctos
- Variants correctos

---

## 🎨 VISUAL ELEMENTS

### User Row Layout

**Estructura**:
```
[Checkbox] [Avatar] [Name + Email] [Phone] [Country] [Status Badge] [Actions Menu]
```

**Avatar**:
- Initials si no hay foto
- Background: accent color
- Size: 40x40px

**Status Badge**:
- Active: green
- Blocked: red
- Pending: yellow

**Actions Menu**:
- Edit
- Delete
- Block/Unblock
- Verify Email
- Separator
- View Details

**NO perder**:
- Avatar con initials
- Status badge variants
- All action options

---

### EmptyState (No Results)

**Trigger**: Search returns 0 results

**Display**:
- Icon: SearchX
- Title: "No users found"
- Message: "Try adjusting your search terms"
- Action button: "Clear Search"

**NO perder**:
- Clear search functionality

---

### Skeleton Loading

**Trigger**: Initial load, refetch

**Layout**: Preserve exact layout of user rows

**NO perder**:
- Layout preservation
- Correct number of skeleton rows

---

## 🔍 API ENDPOINTS USADOS

| Endpoint | Method | Uso |
|----------|--------|-----|
| `/v2/admin/users` | GET | List users (with search, pagination) |
| `/v2/admin/users` | POST | Create user |
| `/v2/admin/users/:id` | GET | Get user details |
| `/v2/admin/users/:id` | PATCH | Update user |
| `/v2/admin/users/:id` | DELETE | Delete user |
| `/v2/admin/users/:id/block` | POST | Block user |
| `/v2/admin/users/:id/unblock` | POST | Unblock user |
| `/v2/admin/users/:id/verify-email` | POST | Verify email |
| `/v2/admin/tenants/:id/custom-fields` | GET | Get field definitions |
| `/v2/admin/tenants/:id/custom-fields` | POST | Create field definition |
| `/v2/admin/tenants/:id/custom-fields/:fieldId` | DELETE | Delete field definition |

**NO perder**:
- Todos los endpoints integrados
- Manejo de errores por endpoint

---

## ✅ CHECKLIST DE RECUPERACIÓN

Al re-implementar, verificar:

**Componentes**:
- [ ] PhoneInput creado y funcionando
- [ ] CountrySelect creado y funcionando
- [ ] Stats cards (4 métricas)
- [ ] User rows layout completo
- [ ] EmptyState (no results)
- [ ] EmptyState (no database)
- [ ] Skeleton loading

**Funcionalidades**:
- [ ] Search con debounce (300ms)
- [ ] Pagination completa
- [ ] Bulk selection (select all, individual)
- [ ] Bulk actions (block, delete)
- [ ] Export (JSON, CSV, all, selected)
- [ ] Create user (con phone, country, custom fields)
- [ ] Edit user (con phone, country, custom fields)
- [ ] Delete user (con confirmación)
- [ ] Block user (reason, duration)
- [ ] Unblock user
- [ ] Verify email
- [ ] Custom fields tab (full CRUD)
- [ ] No database detection (status 424)

**Estado**:
- [ ] Search state + query params
- [ ] Pagination state + query params
- [ ] Selection state (Set<string>)
- [ ] TanStack Query para fetching
- [ ] Optimistic updates donde aplique

**Validaciones**:
- [ ] Email format
- [ ] Password strength
- [ ] Phone number (libphonenumber-js)
- [ ] Required fields

**UX**:
- [ ] Toast notifications (success, error)
- [ ] Loading states
- [ ] Disabled states
- [ ] Error messages
- [ ] Confirmation dialogs

---

## 🎯 PLAN DE ROLLBACK

### Commit a Revertir

**Identificar**: Commit de migración dark de /users

```bash
git log --oneline -- ui/app/\(admin\)/admin/users/page.tsx | head -5
```

**Ejemplo**: `a9ec7ec feat(users): migrate to dark iteration`

### Comando Revert

```bash
git revert a9ec7ec --no-commit
git diff --cached  # Review changes
git commit -m "revert: rollback /admin/users dark iteration for redesign"
```

### Verificación Post-Rollback

```bash
npm run build  # Must succeed
npm run dev    # Check /admin/users
```

**Verificar visualmente**:
- [ ] PhoneInput presente
- [ ] CountrySelect presente
- [ ] Todas las funcionalidades funcionan

---

## 📸 SCREENSHOTS

**Before Rollback**: `docs/ui-unification/screenshots/users/before-rollback.png`
**After Clay Redesign**: `docs/ui-unification/screenshots/users/after-clay-redesign.png`

**Comparación**:
- Funcionalidad idéntica ✅
- Visual superior ✅

---

**VERSION**: 1.0
**FECHA**: 2026-01-31
**STATUS**: Active Reference
