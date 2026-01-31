# ✅ CLAY DESIGN CHECKLIST - QA Verification

**Propósito**: Checklist de verificación para asegurar calidad profesional del Clay Design System.

**Uso**: Ejecutar al finalizar cada migración de página.

**Fecha**: 2026-01-31

---

## 🎨 VISUAL QA

### Colors

- [ ] **NO hardcoded hex colors**
  - Verificar: `rg "#[0-9a-fA-F]{3,6}" ui/app/(admin)/admin/{page}/` debe dar 0 resultados
  - Todas las referencias de color usan tokens semánticos

- [ ] **Semantic tokens usados**
  - Background: `bg-background`, `bg-card`, `bg-muted`
  - Foreground: `text-foreground`, `text-muted-foreground`
  - Accent: `bg-accent`, `text-accent-foreground`, `border-accent`
  - Destructive: `bg-destructive`, `text-destructive`

- [ ] **Dark mode support**
  - Abrir DevTools → Toggle dark mode
  - Todos los colores se adaptan correctamente
  - No hay "hard white" o "hard black" que no cambian

### Shadows

- [ ] **4-layer shadow stacking**
  - Buttons: `shadow-clay-button`
  - Cards: `shadow-clay-card`
  - Float (hover): `shadow-clay-float`
  - Modals: `shadow-clay-modal`

- [ ] **NO single-layer shadows**
  - Evitar: `shadow-sm`, `shadow-md`, `shadow-lg`
  - Usar: clay shadows definidos en sistema

- [ ] **Depth perception**
  - Elementos claramente "lifted" en hover
  - Elementos "pressed" en active
  - Modals claramente sobre fondo

### Typography

- [ ] **Font families correctas**
  - Body text: DM Sans (`font-sans`)
  - Headings: Nunito (`font-display` o `font-heading`)
  - Mono: Geist Mono (`font-mono`)

- [ ] **Font weights apropiados**
  - Body: 400, 500
  - Headings: 600, 700, 800
  - Emphasis: 600

- [ ] **Line heights correctos**
  - Text legible (no "squished")
  - Spacing vertical consistente

- [ ] **No font size hardcoding**
  - Usar: `text-xs`, `text-sm`, `text-base`, etc.
  - Evitar: `text-[14px]`, custom sizes

### Spacing

- [ ] **Spacing scale consistente**
  - Base 4px usado consistentemente
  - Gaps: `gap-1` (4px), `gap-2` (8px), `gap-3` (12px), `gap-4` (16px)
  - Padding: `p-4` (16px), `p-6` (24px), `p-8` (32px)
  - Margin: similar pattern

- [ ] **NO spacing arbitrario**
  - Evitar: `p-[13px]`, custom spacing
  - Usar: valores de scale

### Border Radius

- [ ] **Radius consistente**
  - Small: `rounded-sm` (4px)
  - Medium: `rounded-md` (6px), `rounded-lg` (8px)
  - Large: `rounded-xl` (12px), `rounded-2xl` (16px)
  - Pills: `rounded-full`

- [ ] **NO radius hardcoded**
  - Evitar: `rounded-[10px]`

---

## ⚡ INTERACTION QA

### Micro-interactions

- [ ] **Hover lift en buttons**
  - Detectar: `-translate-y-0.5` o `-translate-y-1`
  - Shadow aumenta: `hover:shadow-clay-card` o `hover:shadow-clay-float`
  - Transition smooth: `transition-all duration-200`

- [ ] **Active press en buttons**
  - Detectar: `active:translate-y-0`
  - Shadow reduce: `active:shadow-clay-button`
  - Feedback táctil

- [ ] **Interactive cards**
  - Hover: lift + shadow increase
  - Active: press feedback
  - Cursor: `cursor-pointer`

### Focus States

- [ ] **Focus rings visibles**
  - Todos los elementos interactivos tienen focus ring
  - Ring color: `focus-visible:ring-accent` o `focus-visible:ring-2`
  - Ring offset: `focus-visible:ring-offset-2`

- [ ] **Keyboard navigation funcional**
  - Tab order lógico
  - Focus visible en cada step
  - Enter/Space activan botones

### Transitions

- [ ] **Smooth transitions**
  - Color changes: `transition-colors duration-150`
  - Transforms: `transition-all duration-200`
  - Layout shifts: `transition-all duration-300`

- [ ] **NO jarring jumps**
  - Elementos no "saltan" sin transition
  - Hover states suaves

### Animations

- [ ] **Performance 60fps**
  - Abrir DevTools → Performance tab
  - Animations smooth, no frame drops

- [ ] **Animations purposeful**
  - No distraen de contenido
  - Enhance UX, no molestan

- [ ] **Blob animations (si aplicable)**
  - `animate-blob-float` funciona
  - Timing apropiado (20s+)
  - `animation-delay-*` escalonado

---

## 🔧 FUNCTIONALITY QA

### Forms

- [ ] **Validation funciona**
  - Required fields detectados
  - Error messages visibles
  - Error states visuales (border rojo)

- [ ] **Disabled states**
  - Buttons disabled cuando apropiado
  - Inputs disabled visualmente claros
  - Cursor: `cursor-not-allowed`

- [ ] **Submit behavior**
  - Loading states durante submit
  - Toast notifications on success/error
  - Form reset después de success (si aplicable)

### Lists & Tables

- [ ] **Pagination funciona**
  - Prev/Next buttons
  - Page numbers clickeables
  - Page size selector
  - Total count display

- [ ] **Search funciona**
  - Debounce aplicado (300ms típico)
  - Clear button visible
  - Query params sync (si aplicable)

- [ ] **Sorting funciona (si aplicable)**
  - Column headers clickeables
  - Sort direction indicator
  - Data re-ordena correctamente

### Dialogs & Modals

- [ ] **Open/close suavemente**
  - Fade in/out
  - Scale/zoom animation
  - Backdrop blur

- [ ] **Escape key cierra**
  - ESC key detectado
  - Modal cierra sin error

- [ ] **Click outside cierra (si aplicable)**
  - Backdrop clickeable
  - Modal cierra apropiadamente

### Data Fetching

- [ ] **Loading states**
  - Skeleton loaders preservan layout
  - Spinners donde apropiado
  - Disabled buttons durante fetch

- [ ] **Error states**
  - Error messages claros
  - Retry buttons (si aplicable)
  - EmptyStates informativos

- [ ] **Success states**
  - Toast notifications
  - Data refresh visible
  - Optimistic updates (si aplicable)

---

## ♿ ACCESSIBILITY QA

### Keyboard

- [ ] **Tab navigation**
  - Orden lógico
  - Todos los elementos interactivos alcanzables
  - No focus traps

- [ ] **Shortcuts work (si aplicable)**
  - Enter submits forms
  - Space toggles checkboxes
  - Arrow keys en select/radio

### Screen Readers

- [ ] **ARIA labels presentes**
  - Iconos tienen `aria-label`
  - Buttons con solo icono tienen label
  - Form inputs asociados a labels

- [ ] **Semantic HTML**
  - `<button>` para buttons (no `<div onClick>`)
  - `<label>` asociado a inputs
  - Headings jerárquicos (`h1`, `h2`, etc.)

### Color Contrast

- [ ] **WCAG AA compliance**
  - Text contrast ratio >= 4.5:1
  - Large text >= 3:1
  - Interactive elements discernibles

- [ ] **No reliance on color alone**
  - Status también indicado por icon/text
  - Links underlined o bold (no solo color)

---

## 🚀 PERFORMANCE QA

### Build

- [ ] **Build succeeds**
  - `npm run build` sin errores
  - `npm run typecheck` sin errores
  - No warnings críticos

- [ ] **Build time acceptable**
  - < 2 minutos para full build
  - < 30 segundos para incremental

### Page Load

- [ ] **Initial load < 3s**
  - Chrome DevTools → Network tab
  - Throttle to "Fast 3G"
  - Page usable en < 3s

- [ ] **No console errors**
  - Chrome DevTools → Console
  - 0 errors on load
  - 0 errors on interaction

### Runtime

- [ ] **No memory leaks**
  - DevTools → Memory tab
  - Heap size stable después de interactions
  - No crecimiento infinito

- [ ] **Smooth scrolling**
  - No janky scroll
  - Virtualization si lista larga (100+ items)

---

## 📱 RESPONSIVE QA

### Mobile (375px - 640px)

- [ ] **Layout stacks correctamente**
  - Cards apilan verticalmente
  - Forms single column
  - Touch targets >= 44px

- [ ] **Text readable**
  - No text overflow
  - No horizontal scroll necesario
  - Font sizes apropiados

### Tablet (640px - 1024px)

- [ ] **Layout adapta**
  - Grid 2 columns donde apropiado
  - Sidebar colapsa (si aplicable)

### Desktop (1024px+)

- [ ] **Layout usa espacio**
  - Grid 3-4 columns
  - Max width containers (no text full-width)
  - Sidebar visible (si aplicable)

---

## 🎯 CLAY-SPECIFIC CHECKS

### Design System Usage

- [ ] **Components from DS**
  - Button, Card, Input, Badge, etc. importados de `/components/ds/`
  - NO custom button/card re-implementations
  - Consistent API usage

- [ ] **Variants usados correctamente**
  - Button: `default`, `outline`, `ghost`
  - Badge: `default`, `secondary`, `destructive`, `success`
  - Card: `interactive` cuando clickeable

### Gradients

- [ ] **Multi-stop gradients usados**
  - Buttons: `bg-gradient-to-b from-accent-2 to-accent-3`
  - NO single-color backgrounds en primary buttons

- [ ] **Subtle gradients**
  - No harsh transitions
  - Clay aesthetic maintained

### Backdrop Effects

- [ ] **Backdrop blur usado apropiadamente**
  - Overlays: `backdrop-blur-sm`
  - Modals: `backdrop-blur-md`
  - Cards: `backdrop-blur-sm` si background semi-transparent

### Surface Depth

- [ ] **Visual hierarchy clara**
  - Background → Cards → Buttons → Modals
  - Depth perceptible visualmente
  - Z-index apropiados

---

## 📋 FINAL CHECKLIST

Antes de marcar como DONE:

- [ ] **Visual QA**: 100% passed
- [ ] **Interaction QA**: 100% passed
- [ ] **Functionality QA**: 100% passed
- [ ] **Accessibility QA**: 100% passed
- [ ] **Performance QA**: 100% passed
- [ ] **Responsive QA**: 100% passed
- [ ] **Clay-specific QA**: 100% passed

- [ ] **Build succeeds**: `npm run build`
- [ ] **Typecheck passes**: `npm run typecheck`
- [ ] **NO hex hardcoded**: `rg "#[0-9a-fA-F]{3,6}"` en página = 0
- [ ] **Screenshot comparativo**: Before/After guardados

- [ ] **Documentation updated**:
  - `pages/{page}.md` → status ✅ DONE
  - `PROGRESS.md` → status ✅ DONE
  - QA results documentados

---

## 🐛 COMMON ISSUES

### Issue: Hardcoded colors found

**Síntoma**: `rg "#[0-9a-fA-F]{3,6}"` devuelve matches

**Fix**:
1. Identificar líneas con hex
2. Reemplazar con tokens semánticos
3. Ejemplo: `#A78BFA` → `bg-accent-2` o `hsl(var(--accent-2))`

### Issue: Dark mode broken

**Síntoma**: Colores no cambian en dark mode

**Fix**:
1. Verificar uso de tokens semánticos (NO hardcoded)
2. Verificar CSS variables definidas en `:root` y `.dark`
3. Test toggle dark mode en DevTools

### Issue: Focus ring invisible

**Síntoma**: No se ve focus al tabular

**Fix**:
1. Agregar `focus-visible:ring-2 focus-visible:ring-accent`
2. Agregar `focus-visible:ring-offset-2`
3. Test keyboard navigation

### Issue: Hover no funciona en mobile

**Síntoma**: Hover states "stuck" en touch

**Fix**:
1. Usar `@media (hover: hover)` para hover-only styles
2. O usar `active:` states para touch feedback

### Issue: Build fails

**Síntoma**: `npm run build` error

**Common causes**:
1. Imports incorrectos
2. Typescript errors
3. Missing exports en barrel files

**Fix**:
1. Check `npm run typecheck`
2. Verificar imports path
3. Verificar exports en `index.ts`

---

**VERSION**: 1.0
**FECHA**: 2026-01-31
**STATUS**: Active Checklist
