# 📘 PHASE EXECUTION GUIDE - Step by Step

**Propósito**: Guía detallada paso a paso para ejecutar cada fase del MIGRATION_MASTER_PLAN.md sin perderse.

**Cómo usar**:
1. Usuario dice "proceed with Phase X"
2. Claude consulta esta guía y ejecuta todos los pasos
3. Claude notifica cuando termina
4. Usuario aprueba y pide siguiente fase

---

## FASE 0: Preparación y Rollback

### Objetivo
Revertir la migración actual de `/admin/users` para recuperar código original con funcionalidad completa.

### Pre-requisitos
- [ ] Git working tree limpio (no cambios sin commit)
- [ ] Backup de screenshots actuales

### Paso 1: Screenshot del estado actual
```bash
# Tomar screenshot de /admin/users en navegador
# Guardar en: docs/ui-unification/screenshots/users/before-rollback.png
```

**Verificación**: Screenshot guardado y visible

### Paso 2: Crear rama de trabajo
```bash
git checkout -b ui-clay-redesign
```

**Verificación**: `git branch` muestra `ui-clay-redesign` activa

### Paso 3: Documentar funcionalidad perdida

Crear archivo `ROLLBACK_AND_RECOVERY.md` (ver sección siguiente de esta guía) con:
- Componentes perdidos (PhoneInput, CountrySelect)
- Funcionalidades críticas que NO deben perderse

**Verificación**: Archivo creado con lista completa

### Paso 4: Identificar commit a revertir

```bash
# Buscar commit de migración de /users
git log --oneline --grep="users" -10
# O buscar por archivo
git log --oneline -- ui/app/\(admin\)/admin/users/page.tsx | head -5
```

**Identificar**: Commit hash de la migración dark (ejemplo: `a9ec7ec`)

### Paso 5: Ejecutar revert

```bash
# Revert sin commit automático para revisar
git revert a9ec7ec --no-commit

# Revisar cambios
git diff --cached
```

**Verificación**:
- `git status` muestra cambios staged
- Revisar que se recupera código original

### Paso 6: Commit del revert

```bash
git commit -m "revert: rollback /admin/users dark iteration for redesign

Reverts migration to recover original functionality before
implementing high-fidelity claymorphism design system.

Components to recover:
- PhoneInput with libphonenumber-js
- CountrySelect with flag emojis
- All original user management features

See: ROLLBACK_AND_RECOVERY.md"
```

**Verificación**:
- `git log -1` muestra commit de revert
- `git status` limpio

### Paso 7: Verificar código recuperado

```bash
# Build debe funcionar
cd ui && npm run build

# Verificar que página carga
npm run dev
# Abrir localhost:3000/admin/users
```

**Verificación**:
- Build exitoso
- Página carga sin errores
- Funcionalidad original presente

### Outputs de Fase 0
- [x] Screenshot en `docs/ui-unification/screenshots/users/before-rollback.png`
- [x] Archivo `ROLLBACK_AND_RECOVERY.md` con lista de funcionalidades
- [x] Commit de revert en rama `ui-clay-redesign`
- [x] Código original recuperado y funcionando

---

## FASE 1: Design System Foundation (Clay)

### Objetivo
Implementar fundamentos del Clay Design System SIN romper configuración actual de Tailwind v4.

### Pre-requisitos
- [ ] FASE 0 completada
- [ ] Código original de /users funcionando

### Paso 1.1: Crear DESIGN_SYSTEM_SPEC.md

**Acción**: Crear archivo completo con especificación Clay.

**Contenido**: Ver apéndice al final de esta guía (sección "DESIGN_SYSTEM_SPEC.md Content")

**Ubicación**: `docs/ui-unification/DESIGN_SYSTEM_SPEC.md`

**Verificación**: Archivo creado con todas las secciones

### Paso 1.2: Actualizar Tailwind Config (SIN reescribir)

**CRÍTICO**: NO reemplazar estructura actual, SOLO agregar valores.

**Archivo**: `ui/tailwind.config.ts`

**Acciones**:

1. **Leer config actual primero**
2. **Agregar fontFamily.display** en sección correspondiente:
   ```typescript
   fontFamily: {
     // ... existentes ...
     display: ['Nunito', 'var(--font-sans)', 'sans-serif'],
   }
   ```

3. **Agregar shadows clay** en `@theme` o `extend.boxShadow`:
   ```typescript
   boxShadow: {
     // ... existentes ...
     'clay-button': '0 1px 2px rgba(0,0,0,0.04), 0 2px 4px rgba(0,0,0,0.04), 0 4px 8px rgba(0,0,0,0.04), 0 6px 12px rgba(0,0,0,0.02)',
     'clay-card': '0 2px 4px rgba(0,0,0,0.04), 0 4px 8px rgba(0,0,0,0.04), 0 8px 16px rgba(0,0,0,0.04), 0 12px 24px rgba(0,0,0,0.02)',
     'clay-float': '0 4px 8px rgba(0,0,0,0.06), 0 8px 16px rgba(0,0,0,0.06), 0 16px 32px rgba(0,0,0,0.06), 0 24px 48px rgba(0,0,0,0.04)',
     'clay-modal': '0 8px 16px rgba(0,0,0,0.08), 0 16px 32px rgba(0,0,0,0.08), 0 32px 64px rgba(0,0,0,0.08), 0 48px 96px rgba(0,0,0,0.06)',
   }
   ```

4. **Agregar keyframes clay** en `@theme` o `extend.keyframes`:
   ```typescript
   keyframes: {
     // ... existentes ...
     'blob-float': {
       '0%, 100%': { transform: 'translate(0, 0) scale(1)' },
       '33%': { transform: 'translate(30px, -50px) scale(1.1)' },
       '66%': { transform: 'translate(-20px, 20px) scale(0.9)' },
     },
     'gentle-pulse': {
       '0%, 100%': { opacity: '0.6', transform: 'scale(1)' },
       '50%': { opacity: '0.8', transform: 'scale(1.05)' },
     },
   }
   ```

5. **Agregar animation** en `@theme` o `extend.animation`:
   ```typescript
   animation: {
     // ... existentes ...
     'blob-float': 'blob-float 20s ease-in-out infinite',
     'gentle-pulse': 'gentle-pulse 4s ease-in-out infinite',
   }
   ```

**Verificación**:
- [ ] NO cambió estructura `@import` / `@theme`
- [ ] Solo se agregaron valores nuevos
- [ ] Build funciona: `npm run build`

### Paso 1.3: Actualizar globals.css (Solo valores)

**CRÍTICO**: NO reemplazar `@import 'tailwindcss'`, solo actualizar variables.

**Archivo**: `ui/app/globals.css`

**Acciones**:

1. **Leer archivo actual primero**
2. **Actualizar valores de CSS variables en `:root`** (solo modificar valores, NO estructura):

```css
:root {
  /* Clay Purple Accent */
  --accent-1: 250 75% 80%;        /* Lighter purple */
  --accent-2: 250 70% 70%;        /* Mid purple */
  --accent-3: 250 65% 60%;        /* Base purple */
  --accent-4: 250 60% 50%;        /* Deep purple */

  /* Clay neutrals with warmth */
  --gray-1: 240 8% 98%;
  --gray-2: 240 6% 94%;
  --gray-3: 240 5% 88%;
  --gray-4: 240 4% 75%;
  --gray-5: 240 3% 60%;
  --gray-6: 240 4% 45%;
  --gray-7: 240 5% 30%;
  --gray-8: 240 6% 20%;
  --gray-9: 240 8% 12%;

  /* ... mantener otras variables existentes ... */
}
```

3. **Agregar sección de fonts** (si no existe):

```css
/* Fonts */
@import url('https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,100..1000;1,9..40,100..1000&family=Nunito:ital,wght@0,200..1000;1,200..1000&display=swap');
```

4. **Agregar utilities layer** (al final del archivo):

```css
@layer utilities {
  .animation-delay-200 {
    animation-delay: 200ms;
  }
  .animation-delay-400 {
    animation-delay: 400ms;
  }
  .animation-delay-600 {
    animation-delay: 600ms;
  }
}
```

**Verificación**:
- [ ] Estructura `@import 'tailwindcss'` intacta
- [ ] `@theme inline` intacto
- [ ] Solo valores de variables modificados
- [ ] Build funciona

### Paso 1.4: Setup Fonts con next/font

**Archivo**: `ui/app/layout.tsx`

**Acciones**:

1. **Importar fonts**:
```typescript
import { DM_Sans, Nunito } from 'next/font/google'
import { GeistMono } from 'geist/font/mono' // Ya existe

const dmSans = DM_Sans({
  subsets: ['latin'],
  variable: '--font-sans',
  display: 'swap',
})

const nunito = Nunito({
  subsets: ['latin'],
  variable: '--font-heading',
  display: 'swap',
  weight: ['400', '600', '700', '800'],
})
```

2. **Aplicar en body className**:
```typescript
<body className={`${dmSans.variable} ${nunito.variable} ${geistMono.variable} font-sans`}>
```

**Verificación**:
- [ ] Fonts cargan correctamente
- [ ] Variables CSS aplicadas
- [ ] No hay errores de consola

### Paso 1.5: Verificación Final Fase 1

**Checklist**:
- [ ] `npm run build` exitoso
- [ ] `npm run typecheck` pasa
- [ ] NO hay hex hardcoded: `rg "#[0-9a-fA-F]{3,6}" ui/components ui/app` (debe dar 0 resultados en nuevos cambios)
- [ ] Fonts se ven correctamente en navegador

### Paso 1.6: Commit Fase 1

```bash
cd ui
git add docs/ui-unification/DESIGN_SYSTEM_SPEC.md
git add tailwind.config.ts app/globals.css app/layout.tsx
git commit -m "feat: implement high-fidelity claymorphism design system foundation

- Add DESIGN_SYSTEM_SPEC.md with complete Clay design tokens
- Configure Tailwind with clay shadows and animations (preserving v4 structure)
- Update CSS variables with clay color palette
- Setup DM Sans and Nunito fonts via next/font/google
- Add animation utilities for micro-interactions

No breaking changes to existing Tailwind v4 setup."
```

### Outputs de Fase 1
- [x] `DESIGN_SYSTEM_SPEC.md` completo
- [x] Tailwind config con clay tokens (sin romper estructura)
- [x] `globals.css` con clay variables
- [x] Fonts configurados con next/font
- [x] Build exitoso
- [x] Commit realizado

---

## FASE 2: Componentes Faltantes

### Objetivo
Crear PhoneInput y CountrySelect profesionales usando clay design system.

### Pre-requisitos
- [ ] FASE 1 completada
- [ ] Design system tokens disponibles

### Paso 2.1: Instalar dependencias

```bash
cd ui
npm install libphonenumber-js
```

**Verificación**: `package.json` tiene `libphonenumber-js`

### Paso 2.2: Crear PhoneInput

**Archivo**: `ui/components/ds/forms/phone-input.tsx`

**Contenido**:
```typescript
/**
 * PhoneInput Component — Design System (Professional)
 *
 * International phone number input with validation.
 * Uses libphonenumber-js for accurate parsing and formatting.
 */

import * as React from "react"
import { parsePhoneNumber, CountryCode, AsYouType } from "libphonenumber-js"
import { cn } from "../utils/cn"
import { Input } from "../core/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../core/select"

export interface PhoneInputProps {
  value?: string
  onChange?: (value: string) => void
  defaultCountry?: CountryCode
  error?: string
  disabled?: boolean
  className?: string
}

const POPULAR_COUNTRIES: CountryCode[] = ['US', 'GB', 'CA', 'AU', 'DE', 'FR', 'ES', 'IT', 'MX', 'BR', 'AR']

export function PhoneInput({
  value = "",
  onChange,
  defaultCountry = "US",
  error,
  disabled,
  className,
}: PhoneInputProps) {
  const [country, setCountry] = React.useState<CountryCode>(defaultCountry)
  const [localValue, setLocalValue] = React.useState(value)

  React.useEffect(() => {
    setLocalValue(value)
  }, [value])

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const input = e.target.value
    const asYouType = new AsYouType(country)
    const formatted = asYouType.input(input)

    setLocalValue(formatted)
    onChange?.(formatted)
  }

  const isValid = React.useMemo(() => {
    try {
      if (!localValue) return true
      const phoneNumber = parsePhoneNumber(localValue, country)
      return phoneNumber?.isValid() ?? false
    } catch {
      return false
    }
  }, [localValue, country])

  return (
    <div className={cn("space-y-2", className)}>
      <div className="flex gap-2">
        <Select value={country} onValueChange={(v) => setCountry(v as CountryCode)} disabled={disabled}>
          <SelectTrigger className="w-[120px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {POPULAR_COUNTRIES.map((code) => (
              <SelectItem key={code} value={code}>
                {getFlagEmoji(code)} {code}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Input
          type="tel"
          value={localValue}
          onChange={handleChange}
          disabled={disabled}
          className={cn(
            "flex-1",
            !isValid && localValue && "border-destructive focus-visible:ring-destructive"
          )}
          placeholder="+1 (555) 000-0000"
        />
      </div>

      {error && (
        <p className="text-xs text-destructive">{error}</p>
      )}
    </div>
  )
}

function getFlagEmoji(countryCode: string): string {
  const codePoints = countryCode
    .toUpperCase()
    .split('')
    .map((char) => 127397 + char.charCodeAt(0))
  return String.fromCodePoint(...codePoints)
}
```

**Verificación**: Archivo creado y usa tokens semánticos (NO hex)

### Paso 2.3: Crear CountrySelect

**Archivo**: `ui/components/ds/forms/country-select.tsx`

**Contenido**:
```typescript
/**
 * CountrySelect Component — Design System (Professional)
 *
 * Country selector with flag emojis and search.
 */

import * as React from "react"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../core/select"
import { cn } from "../utils/cn"

// ISO 3166-1 alpha-2 codes
const COUNTRIES = [
  { code: 'US', name: 'United States' },
  { code: 'GB', name: 'United Kingdom' },
  { code: 'CA', name: 'Canada' },
  { code: 'AU', name: 'Australia' },
  { code: 'DE', name: 'Germany' },
  { code: 'FR', name: 'France' },
  { code: 'ES', name: 'Spain' },
  { code: 'IT', name: 'Italy' },
  { code: 'MX', name: 'Mexico' },
  { code: 'BR', name: 'Brazil' },
  { code: 'AR', name: 'Argentina' },
  { code: 'CL', name: 'Chile' },
  { code: 'CO', name: 'Colombia' },
  { code: 'PE', name: 'Peru' },
  { code: 'VE', name: 'Venezuela' },
  { code: 'JP', name: 'Japan' },
  { code: 'CN', name: 'China' },
  { code: 'IN', name: 'India' },
  { code: 'RU', name: 'Russia' },
  { code: 'ZA', name: 'South Africa' },
] as const

export interface CountrySelectProps {
  value?: string
  onChange?: (value: string) => void
  error?: string
  disabled?: boolean
  className?: string
  placeholder?: string
}

export function CountrySelect({
  value,
  onChange,
  error,
  disabled,
  className,
  placeholder = "Select country",
}: CountrySelectProps) {
  return (
    <div className={cn("space-y-2", className)}>
      <Select value={value} onValueChange={onChange} disabled={disabled}>
        <SelectTrigger className={cn(error && "border-destructive")}>
          <SelectValue placeholder={placeholder} />
        </SelectTrigger>
        <SelectContent>
          {COUNTRIES.map(({ code, name }) => (
            <SelectItem key={code} value={code}>
              {getFlagEmoji(code)} {name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {error && (
        <p className="text-xs text-destructive">{error}</p>
      )}
    </div>
  )
}

function getFlagEmoji(countryCode: string): string {
  const codePoints = countryCode
    .toUpperCase()
    .split('')
    .map((char) => 127397 + char.charCodeAt(0))
  return String.fromCodePoint(...codePoints)
}
```

**Verificación**: Archivo creado con tokens semánticos

### Paso 2.4: Actualizar barrel exports

**Archivo**: `ui/components/ds/index.ts`

**Acción**: Agregar exports (SOLO si archivos existen):

```typescript
// Forms
export * from './forms/phone-input'
export * from './forms/country-select'
```

**CRÍTICO**: Verificar que archivos existen antes de exportar:
```bash
ls ui/components/ds/forms/phone-input.tsx
ls ui/components/ds/forms/country-select.tsx
```

### Paso 2.5: Verificación Final Fase 2

**Checklist**:
- [ ] `ls ui/components/ds/forms/*.tsx` muestra phone-input.tsx y country-select.tsx
- [ ] `npm run build` exitoso
- [ ] PhoneInput renderiza (test manual en /admin/users)
- [ ] CountrySelect muestra banderas
- [ ] NO hex hardcoded en nuevos componentes

### Paso 2.6: Commit Fase 2

```bash
git add ui/components/ds/forms/
git add ui/components/ds/index.ts
git add ui/package.json ui/package-lock.json
git commit -m "feat: add professional PhoneInput and CountrySelect components

- Implement PhoneInput with libphonenumber-js validation
- Add CountrySelect with flag emojis and popular countries
- Use semantic tokens (no hardcoded colors)
- Export via barrel for easy consumption

Components ready for /admin/users migration."
```

### Outputs de Fase 2
- [x] `phone-input.tsx` creado
- [x] `country-select.tsx` creado
- [x] libphonenumber-js instalado
- [x] Barrel exports actualizados
- [x] Build exitoso
- [x] Commit realizado

---

## FASE 3: Refinar Componentes DS Existentes

### Objetivo
Aplicar clay style a componentes existentes del design system.

### Pre-requisitos
- [ ] FASE 1 completada (design tokens disponibles)

### Componentes a Refinar

Lista de componentes con prioridad:

1. **Button** - Alta (usado en toda la app)
2. **Card** - Alta (usado en stats, lists)
3. **Input** - Alta (formularios)
4. **Badge** - Media (status, tags)
5. **Select** - Media (dropdowns)
6. **Textarea** - Baja (formularios grandes)

### Paso 3.1: Refinar Button

**Archivo**: `ui/components/ds/core/button.tsx`

**Cambios a aplicar**:

1. **Leer componente actual primero**
2. **Aplicar clay shadows y gradients**:

```typescript
// Variant "default" (primary)
"default": cn(
  "bg-gradient-to-b from-accent-2 to-accent-3",
  "text-white",
  "shadow-clay-button",
  "hover:shadow-clay-card hover:-translate-y-0.5",
  "active:translate-y-0 active:shadow-clay-button",
  "transition-all duration-200"
),

// Variant "outline"
"outline": cn(
  "border-2 border-border",
  "bg-background/80 backdrop-blur-sm",
  "hover:bg-accent/5 hover:border-accent",
  "shadow-sm hover:shadow-clay-button",
  "transition-all duration-200"
),

// Variant "ghost"
"ghost": cn(
  "hover:bg-accent/10",
  "transition-colors duration-150"
),
```

**Verificación**:
- [ ] Usa tokens semánticos (`bg-accent-2`, `shadow-clay-button`)
- [ ] NO usa hex hardcoded
- [ ] Build funciona

### Paso 3.2: Refinar Card

**Archivo**: `ui/components/ds/core/card.tsx`

**Cambios**:

```typescript
const Card = React.forwardRef<HTMLDivElement, CardProps>(
  ({ className, interactive, ...props }, ref) => (
    <div
      ref={ref}
      className={cn(
        "rounded-xl border border-border/50 bg-card text-card-foreground",
        "shadow-clay-card",
        interactive && cn(
          "cursor-pointer transition-all duration-200",
          "hover:shadow-clay-float hover:-translate-y-1",
          "active:translate-y-0 active:shadow-clay-card"
        ),
        className
      )}
      {...props}
    />
  )
)
```

**Verificación**: Card tiene micro-interacciones suaves

### Paso 3.3: Refinar Input

**Archivo**: `ui/components/ds/core/input.tsx`

**Cambios**:

```typescript
const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type, ...props }, ref) => {
    return (
      <input
        type={type}
        className={cn(
          "flex h-9 w-full rounded-md",
          "border border-input bg-background/50 backdrop-blur-sm",
          "px-3 py-2 text-sm text-foreground",
          "shadow-inner",  // Recessed effect
          "placeholder:text-muted-foreground",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent",
          "focus-visible:border-accent focus-visible:shadow-clay-button",
          "disabled:cursor-not-allowed disabled:opacity-50",
          "transition-all duration-200",
          className
        )}
        ref={ref}
        {...props}
      />
    )
  }
)
```

**Verificación**: Input tiene efecto "recessed" y focus smooth

### Paso 3.4: Refinar Badge

**Archivo**: `ui/components/ds/core/badge.tsx`

**Cambios**:

```typescript
const badgeVariants = cva(
  "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold transition-colors",
  {
    variants: {
      variant: {
        default: "bg-accent/15 text-accent-foreground border border-accent/30",
        secondary: "bg-muted/80 text-muted-foreground border border-border/50",
        destructive: "bg-destructive/15 text-destructive border border-destructive/30",
        success: "bg-green-500/15 text-green-700 dark:text-green-300 border border-green-500/30",
        outline: "text-foreground border border-border",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)
```

**Verificación**: Badge usa opacidades y borders sutiles

### Paso 3.5: Verificación Final Fase 3

**Checklist**:
- [ ] NO hex hardcoded: `rg "#[0-9a-fA-F]{3,6}" ui/components/ds/core/`
- [ ] `npm run build` exitoso
- [ ] Componentes renderizan correctamente (visual check)
- [ ] Micro-interacciones funcionan (hover, active, focus)

### Paso 3.6: Commit Fase 3

```bash
git add ui/components/ds/core/*.tsx
git add ui/components/ds/navigation/*.tsx
git commit -m "refactor: apply clay design system to core components

- Update Button with gradients, lift hover, press feedback
- Enhance Card with interactive variant and smooth transitions
- Add recessed Input style with focus transforms
- Refine Badge with subtle opacity and borders
- Update Select with clay shadows

All components use semantic tokens (no hardcoded colors).
Micro-interactions enhance professional feel."
```

### Outputs de Fase 3
- [x] Button refined
- [x] Card refined
- [x] Input refined
- [x] Badge refined
- [x] Select refined (if needed)
- [x] NO hex hardcoded
- [x] Build exitoso
- [x] Commit realizado

---

## FASE 4: Re-migrar /admin/users

### Objetivo
Re-implementar `/admin/users` con clay design system y recuperar toda funcionalidad perdida.

### Pre-requisitos
- [ ] FASE 1, 2, 3 completadas
- [ ] Código original de /users disponible (post-rollback)

### Paso 4.1: Crear Background Blobs

**Archivo**: `ui/components/ds/background/blobs.tsx`

**Contenido**:
```typescript
/**
 * Background Blobs — Clay Design System
 *
 * Animated gradient blobs for background decoration.
 */

import { cn } from "../utils/cn"

export interface BackgroundBlobsProps {
  className?: string
}

export function BackgroundBlobs({ className }: BackgroundBlobsProps) {
  return (
    <div className={cn("pointer-events-none fixed inset-0 -z-10 overflow-hidden", className)}>
      {/* Blob 1 - Top Left */}
      <div
        className="absolute -left-20 -top-20 h-96 w-96 rounded-full bg-accent-1/20 blur-3xl animate-blob-float"
        style={{ animationDelay: '0s' }}
      />

      {/* Blob 2 - Top Right */}
      <div
        className="absolute -right-20 top-40 h-80 w-80 rounded-full bg-accent-2/15 blur-3xl animate-blob-float animation-delay-200"
        style={{ animationDelay: '2s' }}
      />

      {/* Blob 3 - Bottom */}
      <div
        className="absolute -bottom-20 left-1/2 h-96 w-96 -translate-x-1/2 rounded-full bg-accent-1/10 blur-3xl animate-blob-float animation-delay-400"
        style={{ animationDelay: '4s' }}
      />
    </div>
  )
}
```

**Verificación**: Usa tokens (`bg-accent-1/20`) y animations clay

### Paso 4.2: Exportar BackgroundBlobs

**Archivo**: `ui/components/ds/index.ts`

```typescript
// Background
export * from './background/blobs'
```

### Paso 4.3: Re-implementar /admin/users

**CRÍTICO**: Esta es la parte más compleja. Usar código original como referencia.

**Archivo**: `ui/app/(admin)/admin/users/page.tsx`

**Estrategia**:

1. **Leer código original** (post-rollback)
2. **Identificar todas las funcionalidades**:
   - Search con debounce
   - Pagination
   - Bulk selection
   - Bulk actions (block, delete)
   - Export (JSON, CSV)
   - Create user
   - Edit user
   - Delete user
   - Block/Unblock user
   - Verify email
   - Custom fields tab
   - No database detection (status 424)

3. **Aplicar clay components**:
   - PageShell + PageHeader
   - BackgroundBlobs
   - Stats cards con `shadow-clay-card`
   - User rows con `hover:shadow-clay-button hover:-translate-y-0.5`
   - Forms con PhoneInput y CountrySelect
   - Tabs con clay style
   - Dialogs con `shadow-clay-modal`
   - EmptyStates con clay style
   - Skeletons preservando layout

4. **NO hardcodear colores**:
   - ✅ `bg-accent`, `text-accent-foreground`
   - ❌ `bg-[#A78BFA]`, `from-purple-400`

### Checklist Funcionalidad (NO perder)

Durante la re-implementación, verificar cada item:

- [ ] Search con debounce (300ms)
- [ ] Pagination (page, pageSize)
- [ ] Bulk selection (checkbox en cada row)
- [ ] Bulk actions dropdown (block selected, delete selected)
- [ ] Export dropdown (JSON, CSV)
- [ ] Create user dialog
  - [ ] Con PhoneInput
  - [ ] Con CountrySelect
  - [ ] Custom fields support
- [ ] Edit user dialog
  - [ ] Con PhoneInput
  - [ ] Con CountrySelect
  - [ ] Custom fields editable
- [ ] Delete user confirmation
- [ ] Block user dialog (reason + duration)
- [ ] Unblock user
- [ ] Verify email action
- [ ] Custom fields tab
  - [ ] Add field definition
  - [ ] Remove field definition
  - [ ] Type selector (string, number, boolean, date)
- [ ] No database detection (status 424)
  - [ ] EmptyState con mensaje claro
  - [ ] Setup instructions

### Paso 4.4: Screenshot Comparación

**Acción**: Tomar screenshot del nuevo /admin/users

**Guardar en**: `docs/ui-unification/screenshots/users/after-clay-redesign.png`

**Comparar con**: `docs/ui-unification/screenshots/users/before-rollback.png`

**Verificar**:
- [ ] Visualmente superior
- [ ] Funcionalidad idéntica o mejorada

### Paso 4.5: Verificación Final Fase 4

**Checklist**:
- [ ] Screenshot comparativo guardado
- [ ] Todas las funcionalidades funcionan
- [ ] NO hex hardcoded: `rg "#[0-9a-fA-F]{3,6}" ui/app/\(admin\)/admin/users/`
- [ ] `npm run build` exitoso
- [ ] `npm run typecheck` pasa
- [ ] Visual QA: clay style aplicado consistentemente
- [ ] Interaction QA: micro-interacciones funcionan
- [ ] Functionality QA: todos los items del checklist ✅

### Paso 4.6: Commit Fase 4

```bash
git add ui/app/\(admin\)/admin/users/page.tsx
git add ui/components/ds/background/blobs.tsx
git add ui/components/ds/index.ts
git add docs/ui-unification/screenshots/users/after-clay-redesign.png
git commit -m "feat(users): re-migrate with clay design system + recovered functionality

Complete redesign of /admin/users with high-fidelity claymorphism:
- Apply BackgroundBlobs for ambient depth
- Use PhoneInput and CountrySelect (recovered)
- Stats cards with clay shadows and hover lift
- User rows with smooth micro-interactions
- Forms with clay input style
- Tabs with professional polish
- Dialogs with clay modal shadows
- EmptyStates with clay aesthetic

All functionality preserved:
- Search, pagination, bulk actions
- Create, edit, delete, block/unblock users
- Email verification, custom fields
- Export to JSON/CSV
- No-database detection

Uses semantic tokens throughout (zero hardcoded colors)."
```

### Outputs de Fase 4
- [x] BackgroundBlobs creado
- [x] /admin/users re-migrado con clay style
- [x] Toda funcionalidad recuperada
- [x] Screenshot comparativo guardado
- [x] NO hex hardcoded
- [x] Build y typecheck exitosos
- [x] Commit realizado

---

## FASE 5: Documentación y QA Final

### Objetivo
Actualizar documentación y ejecutar QA completo.

### Paso 5.1: Actualizar pages/users.md

**Archivo**: `docs/ui-unification/pages/users.md`

**Cambios**:

1. Actualizar status a `✅ DONE`
2. Agregar nueva sección al final:

```markdown
## 12. Clay Redesign Implementation

**Fecha**: 2026-01-31

### Rollback y Rediseño

Se ejecutó rollback de migración dark original debido a:
- Funcionalidad perdida (PhoneInput, CountrySelect)
- Diseño genérico sin refinamiento visual
- Hardcoded colors

### Implementación Clay

Componentes aplicados:
- `BackgroundBlobs` - Ambient depth
- `PhoneInput` - Professional phone validation
- `CountrySelect` - Flag emojis, popular countries
- Clay Button - Gradients, lift hover
- Clay Card - Interactive shadows
- Clay Input - Recessed style
- Clay Badge - Refined opacity
- Clay Tabs - Professional polish
- Clay Dialog - Modal shadows

### Funcionalidad Completa

✅ Todas las funcionalidades originales preservadas:
- Search, pagination, bulk operations
- CRUD operations (create, edit, delete, block, verify)
- Custom fields management
- Export to JSON/CSV
- No-database detection

### Design System

Ver: `DESIGN_SYSTEM_SPEC.md` para especificación completa.

Tokens usados:
- Colors: `accent-1` through `accent-4`, `gray-1` through `gray-9`
- Shadows: `shadow-clay-button`, `shadow-clay-card`, `shadow-clay-float`, `shadow-clay-modal`
- Animations: `animate-blob-float`, `animate-gentle-pulse`

### QA Results

- Visual QA: ✅ Passed
- Interaction QA: ✅ Passed
- Functionality QA: ✅ Passed
- Accessibility QA: ✅ Passed
- Performance QA: ✅ Passed

Ver: `CLAY_DESIGN_CHECKLIST.md` para detalles.
```

### Paso 5.2: Actualizar PROGRESS.md

**Archivo**: `docs/ui-unification/PROGRESS.md`

**Cambios**:

Actualizar fila de `/admin/users`:

```markdown
| **/admin/users** | ✅ | ✅ | ✅ DONE | [commit-hash] | 2026-01-31 | Priority 1, COMPLEX - Clay redesign complete, all functionality recovered |
```

### Paso 5.3: Actualizar WORKPLAN.md

**Archivo**: `docs/ui-unification/WORKPLAN.md`

**Cambios**:

1. Marcar `/admin/users` como completado
2. Agregar nota sobre clay system available
3. Next steps: aplicar clay a páginas restantes

### Paso 5.4: Ejecutar QA Checklist

**Referencia**: Ver `CLAY_DESIGN_CHECKLIST.md` (se creará en siguiente paso de esta guía)

**Items a verificar**:

**Visual QA**:
- [ ] Shadows correctas (4-layer stacking)
- [ ] Gradients suaves (multi-stop)
- [ ] Colors usan tokens (no hex)
- [ ] Typography clara (DM Sans, Nunito)
- [ ] Spacing consistente (4px, 8px, 12px, 16px, 24px, 32px)

**Interaction QA**:
- [ ] Hover lift en buttons (-translate-y-0.5)
- [ ] Active press en buttons (translate-y-0)
- [ ] Focus rings visibles (ring-2 ring-accent)
- [ ] Transitions suaves (duration-200)
- [ ] Animations no interrumpen interacción

**Functionality QA**:
- [ ] Todos los items del checklist Fase 4 ✅
- [ ] No errores de consola
- [ ] No warnings de React
- [ ] Performance acceptable (Lighthouse > 80)

**Accessibility QA**:
- [ ] Keyboard navigation funciona
- [ ] Focus visible en todos los elementos interactivos
- [ ] Labels asociados a inputs
- [ ] ARIA labels en iconos
- [ ] Color contrast suficiente (WCAG AA)

**Performance QA**:
- [ ] Build time < 2 minutos
- [ ] Page load < 3 segundos
- [ ] No memory leaks (DevTools)
- [ ] Animations 60fps

### Paso 5.5: Commit Documentación

```bash
git add docs/ui-unification/pages/users.md
git add docs/ui-unification/PROGRESS.md
git add docs/ui-unification/WORKPLAN.md
git commit -m "docs: update migration docs with clay design system implementation

- Mark /admin/users as DONE with clay redesign
- Document rollback and remediation process
- Add QA results and component usage
- Update progress tracking

Clay design system now available for future migrations."
```

### Outputs de Fase 5
- [x] `pages/users.md` actualizado
- [x] `PROGRESS.md` actualizado
- [x] `WORKPLAN.md` actualizado
- [x] QA checklist completado
- [x] Commit realizado

---

## APÉNDICE: Contenido de DESIGN_SYSTEM_SPEC.md

**Este contenido debe copiarse al crear el archivo en Fase 1.1**

```markdown
# 🎨 High-Fidelity Claymorphism Design System

**Version**: 1.0
**Date**: 2026-01-31
**Status**: Active

---

## Overview

Este documento especifica el **High-Fidelity Claymorphism Design System** para HelloJohn Admin UI.

**Objetivo**: Diseño profesional nivel Apple/Meta usando estética digital clay con:
- Soft matte surfaces
- Multi-layer depth stacking
- Micro-interactions refinadas
- Semantic color tokens (NO hex hardcoded)

---

## Color Palette

### Clay Purple (Accent)

| Token | HSL | Uso |
|-------|-----|-----|
| `--accent-1` | `250 75% 80%` | Lighter purple (backgrounds, hovers) |
| `--accent-2` | `250 70% 70%` | Mid purple (gradients start) |
| `--accent-3` | `250 65% 60%` | Base purple (gradients end) |
| `--accent-4` | `250 60% 50%` | Deep purple (emphasis) |

### Clay Neutrals

| Token | HSL | Uso |
|-------|-----|-----|
| `--gray-1` | `240 8% 98%` | Lightest (backgrounds) |
| `--gray-2` | `240 6% 94%` | Subtle backgrounds |
| `--gray-3` | `240 5% 88%` | Borders, dividers |
| `--gray-4` | `240 4% 75%` | Disabled text |
| `--gray-5` | `240 3% 60%` | Secondary text |
| `--gray-6` | `240 4% 45%` | Body text |
| `--gray-7` | `240 5% 30%` | Headings |
| `--gray-8` | `240 6% 20%` | Dark backgrounds (dark mode) |
| `--gray-9` | `240 8% 12%` | Darkest (dark mode) |

### Semantic Tokens

**Light Mode**:
```css
--background: var(--gray-1);
--foreground: var(--gray-7);
--card: white;
--card-foreground: var(--gray-7);
--muted: var(--gray-2);
--muted-foreground: var(--gray-5);
--border: var(--gray-3);
--input: var(--gray-3);
--accent: var(--accent-3);
--accent-foreground: white;
```

**Dark Mode**:
```css
--background: var(--gray-9);
--foreground: var(--gray-2);
--card: var(--gray-8);
--card-foreground: var(--gray-2);
--muted: var(--gray-8);
--muted-foreground: var(--gray-4);
--border: var(--gray-7);
--input: var(--gray-7);
--accent: var(--accent-2);
--accent-foreground: var(--gray-9);
```

---

## Typography

### Fonts

| Role | Font Family | Weights | Usage |
|------|-------------|---------|-------|
| Body | DM Sans | 400, 500, 600 | Paragraphs, UI text |
| Headings | Nunito | 600, 700, 800 | Titles, headers |
| Mono | Geist Mono | 400, 500 | Code, IDs, tokens |

### Scale

| Token | Size | Line Height | Usage |
|-------|------|-------------|-------|
| `text-xs` | 12px | 16px | Captions, badges |
| `text-sm` | 14px | 20px | Secondary text, labels |
| `text-base` | 16px | 24px | Body text |
| `text-lg` | 18px | 28px | Emphasized text |
| `text-xl` | 20px | 28px | Small headings |
| `text-2xl` | 24px | 32px | Section titles |
| `text-3xl` | 30px | 36px | Page titles |
| `text-4xl` | 36px | 40px | Hero text |

---

## Shadows (4-Layer Stacking)

### Button Shadow
```css
shadow-clay-button:
  0 1px 2px rgba(0,0,0,0.04),
  0 2px 4px rgba(0,0,0,0.04),
  0 4px 8px rgba(0,0,0,0.04),
  0 6px 12px rgba(0,0,0,0.02)
```

### Card Shadow
```css
shadow-clay-card:
  0 2px 4px rgba(0,0,0,0.04),
  0 4px 8px rgba(0,0,0,0.04),
  0 8px 16px rgba(0,0,0,0.04),
  0 12px 24px rgba(0,0,0,0.02)
```

### Float Shadow (Hover)
```css
shadow-clay-float:
  0 4px 8px rgba(0,0,0,0.06),
  0 8px 16px rgba(0,0,0,0.06),
  0 16px 32px rgba(0,0,0,0.06),
  0 24px 48px rgba(0,0,0,0.04)
```

### Modal Shadow
```css
shadow-clay-modal:
  0 8px 16px rgba(0,0,0,0.08),
  0 16px 32px rgba(0,0,0,0.08),
  0 32px 64px rgba(0,0,0,0.08),
  0 48px 96px rgba(0,0,0,0.06)
```

---

## Border Radius

| Token | Value | Usage |
|-------|-------|-------|
| `rounded-sm` | 4px | Small elements, badges |
| `rounded-md` | 6px | Inputs, buttons |
| `rounded-lg` | 8px | Cards, panels |
| `rounded-xl` | 12px | Large cards, modals |
| `rounded-2xl` | 16px | Feature sections |
| `rounded-full` | 9999px | Avatars, pills |

---

## Spacing

Base: 4px

| Token | Value | Usage |
|-------|-------|-------|
| `spacing-1` | 4px | Tight spacing |
| `spacing-2` | 8px | Small gaps |
| `spacing-3` | 12px | Medium gaps |
| `spacing-4` | 16px | Default spacing |
| `spacing-6` | 24px | Section spacing |
| `spacing-8` | 32px | Large spacing |
| `spacing-12` | 48px | Extra large spacing |

---

## Animations

### Keyframes

**Blob Float**:
```css
@keyframes blob-float {
  0%, 100% { transform: translate(0, 0) scale(1); }
  33% { transform: translate(30px, -50px) scale(1.1); }
  66% { transform: translate(-20px, 20px) scale(0.9); }
}
```

**Gentle Pulse**:
```css
@keyframes gentle-pulse {
  0%, 100% { opacity: 0.6; transform: scale(1); }
  50% { opacity: 0.8; transform: scale(1.05); }
}
```

### Timing

| Property | Duration | Easing |
|----------|----------|--------|
| Color transitions | 150ms | ease |
| Transform (hover) | 200ms | ease-out |
| Shadow (hover) | 200ms | ease-out |
| Layout shifts | 300ms | ease-in-out |
| Page transitions | 400ms | ease-in-out |

---

## Component Specs

### Button

**Variants**:

- **Default (Primary)**:
  - Background: Gradient `from-accent-2 to-accent-3`
  - Text: White
  - Shadow: `shadow-clay-button`
  - Hover: `shadow-clay-card + -translate-y-0.5`
  - Active: `translate-y-0 + shadow-clay-button`

- **Outline**:
  - Border: `border-2 border-border`
  - Background: `bg-background/80 backdrop-blur-sm`
  - Hover: `bg-accent/5 border-accent shadow-clay-button`

- **Ghost**:
  - Background: Transparent
  - Hover: `bg-accent/10`

**Sizes**:
- sm: `h-8 px-3 text-xs`
- md: `h-9 px-4 text-sm` (default)
- lg: `h-10 px-6 text-base`

### Card

**Base**:
- Background: `bg-card`
- Border: `border border-border/50`
- Radius: `rounded-xl`
- Shadow: `shadow-clay-card`

**Interactive**:
- Hover: `shadow-clay-float -translate-y-1`
- Active: `translate-y-0 shadow-clay-card`
- Cursor: `cursor-pointer`

### Input

**Base**:
- Background: `bg-background/50 backdrop-blur-sm`
- Border: `border border-input`
- Shadow: `shadow-inner` (recessed effect)
- Height: `h-9`
- Padding: `px-3 py-2`

**Focus**:
- Ring: `ring-2 ring-accent`
- Border: `border-accent`
- Shadow: `shadow-clay-button`

### Badge

**Variants**:

- **Default**:
  - Background: `bg-accent/15`
  - Text: `text-accent-foreground`
  - Border: `border border-accent/30`

- **Secondary**:
  - Background: `bg-muted/80`
  - Text: `text-muted-foreground`
  - Border: `border border-border/50`

### Tabs

**TabsList**:
- Background: `bg-muted`
- Padding: `p-1`
- Radius: `rounded-lg`
- Gap: `gap-1`

**TabsTrigger**:
- Inactive: `hover:bg-muted/50`
- Active: `bg-background shadow-button`

---

## Usage Guidelines

### DO ✅

- Use semantic tokens (`bg-accent`, `text-foreground`)
- Apply 4-layer shadow stacks for depth
- Use micro-interactions (hover lift, active press)
- Maintain consistent spacing (4px base)
- Use backdrop-blur for layered elements
- Test in both light and dark modes

### DON'T ❌

- Hardcode hex colors (`#A78BFA`)
- Use single-layer shadows (`shadow-lg`)
- Skip transitions (feels janky)
- Mix spacing systems (8px + 10px)
- Forget focus states (accessibility)
- Ignore dark mode

---

## Examples

### Stats Card

```tsx
<Card interactive className="p-6">
  <div className="flex items-center justify-between">
    <div>
      <p className="text-sm text-muted-foreground">Total Users</p>
      <h3 className="text-3xl font-display font-bold text-foreground">1,234</h3>
    </div>
    <div className="rounded-full bg-accent/10 p-3">
      <UsersIcon className="h-6 w-6 text-accent" />
    </div>
  </div>
</Card>
```

### Primary Button

```tsx
<Button
  variant="default"
  size="md"
  className="font-semibold"
>
  Create User
</Button>
```

### Search Input

```tsx
<Input
  type="search"
  placeholder="Search users..."
  className="max-w-sm"
/>
```

---

## Future Enhancements

- [ ] Add success/warning/error color tokens
- [ ] Define chart color palette
- [ ] Specify illustration style
- [ ] Add icon system guidelines
- [ ] Define data visualization patterns

---

**END OF SPEC**
```

---

## 🎯 RESUMEN DE EJECUCIÓN

### Flujo de Trabajo

1. Usuario dice: **"proceed with Phase X"**
2. Claude:
   - Consulta esta guía (PHASE_EXECUTION_GUIDE.md)
   - Ejecuta todos los pasos de la fase
   - Verifica checklist
   - Realiza commit
   - Notifica: "Fase X completada ✅"
3. Usuario revisa y aprueba
4. Usuario dice: **"proceed with Phase X+1"**
5. Repetir

### Documentos de Referencia

| Documento | Propósito |
|-----------|-----------|
| `MIGRATION_MASTER_PLAN.md` | Visión general, fases, objetivos |
| `PHASE_EXECUTION_GUIDE.md` | Este documento - paso a paso |
| `ROLLBACK_AND_RECOVERY.md` | Plan de rollback específico |
| `CLAY_DESIGN_CHECKLIST.md` | QA checklist |
| `FUTURE_MIGRATION_WORKFLOW.md` | Workflow para siguientes páginas |

### Verificación Continua

En CADA fase:
- [ ] Build exitoso
- [ ] Typecheck pasa
- [ ] NO hex hardcoded
- [ ] Commit realizado
- [ ] Usuario notificado

---

**VERSION**: 1.0
**FECHA**: 2026-01-31
**STATUS**: Ready for Use
