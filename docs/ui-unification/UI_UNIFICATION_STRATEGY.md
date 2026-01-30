# UI Unification Strategy — HelloJohn Admin Panel

> **This is the source of truth. Execution state lives in [WORKPLAN](WORKPLAN.md) / [PROGRESS](PROGRESS.md) / [DECISIONS](DECISIONS.md).**


**Objetivo:** Unificar UX/UI del Admin Panel bajo un **Design System Claymorphism "High-Fidelity"** adaptado a contexto enterprise, con **paletas separadas** para **Dark** y **Light**, manteniendo una identidad consistente y código mantenible.

**Versión:** 2.1 — Single-Dev, Low Ceremony  
**Fecha:** 2026-01-30  
**Modo de trabajo:** **Single-dev (sin PRs/branches por defecto)**. **1 página a la vez**, **2 iteraciones por página** (**Dark primero, Light después**). Control y trazabilidad vía **documentación local + commits por checkpoints**.

---

## ✅ Decisiones Cerradas

- ✅ **Paletas separadas** (Opción B) — máxima calidad visual
- ✅ **Dark primero, Light después** — Dark es la "verdad" del sistema
- ✅ **Animaciones: punto medio** (profesional + impacto visible)
- ✅ **Reemplazo completo de UI kit** — Se permiten primitivas headless (Radix) solo por accesibilidad
- ✅ **Cambios visuales notorios: OK** — Prioridad a consistencia sobre backward compatibility visual
- ✅ **Single-dev, low ceremony** — el orden lo pone la **doc local** y los **commits checkpoint**, no PRs/branches

---

## 0. Principios No Negociables

### 0.1 Consistencia > Creatividad Suelta
- **Un solo lenguaje visual** (headers, cards, spacing, inputs, tablas, dialogs, toasts)
- **Nada de "esta page se ve distinta porque sí"**
- Si hay excepción, se documenta y se vuelve patrón oficial

### 0.2 Performance & "Single Binary Mindset"
Este panel vive dentro del repo y puede terminar embebido:
- **No meter dependencias pesadas** sin justificación real
- Evitar animaciones caras en listas/tablas largas
- Evitar CSS gigante y duplicado: **tokens + componentes**
- Priorizar bundle size y lighthouse scores

### 0.3 Accesibilidad "First-Class"
- Focus visible y navegación por teclado real
- Contrastes WCAG AA mínimo (AAA donde sea posible)
- `prefers-reduced-motion` respetado
- ARIA labels y semantic HTML

### 0.4 Reusabilidad Obligatoria
- Si una page necesita un patrón nuevo (ej: "KeyValueList con copy"), **se crea como componente DS reutilizable ANTES** de usar en la page
- **Regla de 2 usos**: Si un patrón aparece 2+ veces, se convierte en componente DS oficial
- Cero estilos ad-hoc repetidos

---

## 1. Modelo de Trabajo — Single-Dev, Low Ceremony

### 1.1 Regla Base
- Se trabaja directo sobre `main` (o el branch principal del repo).
- Se evita el overhead de PRs y ramas salvo casos puntuales (ver 1.5).
- La trazabilidad se garantiza con:
  1) **Documentación local** (fuente de verdad operativa)
  2) **Commits checkpoint** (historial claro y reversible)

### 1.2 "Un carril" con dos tipos de trabajo
En vez de "dos agentes", hay dos **modos** que alternan:

**Modo A — Design System (DS) / Fundación**
- Tokens, theme switching, sombras clay, background, motion base
- Componentes base reutilizables
- Documentación de patrones

**Modo B — Migración de Páginas**
- Auditoría de página → Dark → Light → cierre/DoD

**Regla:** si una página requiere un patrón DS nuevo, se vuelve a **Modo A**, se crea el componente DS, y luego se retoma la página.

### 1.3 Cómo evitamos el caos sin PRs/branches
- **Una sola tarea grande a la vez**: o estás en Fase DS, o estás en una página.
- **Checkpoints obligatorios**: antes y después de cada hito importante, se hace commit con formato estándar.
- **Doc manda**: el estado real se escribe en `docs/ui-unification/WORKPLAN.md` y `docs/ui-unification/PROGRESS.md`.

### 1.4 Convención de commits (Checkpoints)
Formato recomendado (simple y legible):
- `phase0: setup workplan + progress + templates`
- `phase1: tokens + globals + theme switching`
- `ds: add Button/Card/Input (ola1)`
- `page(tenants): audit`
- `page(tenants): dark iteration`
- `page(tenants): light iteration`
- `page(tenants): done + docs + screenshots`
- `phase4: hardening perf + a11y + cleanup`

**Regla:** si un commit mezcla demasiadas cosas, lo partís. Un checkpoint = un hito claro.

### 1.5 Cuándo sí usar branch (excepción, no regla)
Solo si:
- Estás por tocar algo muy riesgoso (refactor masivo que puede romper todo)
- Querés experimentar sin ensuciar `main`
- Estás migrando una página gigante y querés aislarla unos días

En esos casos:
- Creás UNA rama temporal tipo `wip/ui-{topic}` y la borrás al terminar.
- Igual seguís usando docs + commits.

---

## 2. Definition of Done (DoD) por Página

Una página está "✅ DONE" cuando cumple **TODOS** estos criterios:

### ✅ Visual
- [ ] Se ve consistente con DS (cards, headers, spacing, inputs, tablas)
- [ ] Dark ✅ (iteración 1) + Light ✅ (iteración 2)
- [ ] Estados completos: Loading / Empty / Error / Success / Disabled
- [ ] Microcopy claro (mensajes y labels entendibles)
- [ ] Animaciones sutiles (hover lift, press, focus)

### ✅ UX
- [ ] Jerarquía clara: título → descripción → acciones primarias/secundarias
- [ ] Acciones peligrosas con confirmación (dialog con advertencia)
- [ ] Feedback inmediato: toast / inline message / loader/skeleton
- [ ] Search debounced (si aplica)
- [ ] Bulk actions con confirmación (si aplica)

### ✅ Accesibilidad
- [ ] Focus visible y correcto (tab order lógico)
- [ ] Labels y `aria-*` donde corresponda
- [ ] Contraste OK en ambos temas (verificado con axe DevTools)
- [ ] Keyboard shortcuts documentados (si aplica)

### ✅ Performance / Code
- [ ] No hay estilos ad-hoc repetidos (si se repite → componente DS)
- [ ] No hay "magic numbers" visuales sin token
- [ ] No se introducen re-renders innecesarios (memo donde aplique)
- [ ] No hay dependencias nuevas pesadas (bundle size verificado)
- [ ] Linter/Typecheck OK

### ✅ QA
- [ ] Screenshots before/after (dark y light)
- [ ] Visual regression actualizado (si aplica)
- [ ] Testing manual: flujos principales verificados
- [ ] Registro local actualizado (PROGRESS + audit + workplan)

---

## 3. Fases del Proyecto (Orden Real de Ejecución)

> **Importante:** No arrancamos migración page-by-page fuerte hasta tener fundación + kit mínimo.

---

### FASE 0 — Setup Operativo (Single-Dev Control Plane)

**Duración Estimada:** 30-90 min  
**Meta:** Tener "control de misión" local para que el proceso sea ordenado sin PRs/branches.

#### Outputs mínimos
1) **Workplan diario y estado**
   - `docs/ui-unification/WORKPLAN.md`

2) **Progreso por páginas**
   - `docs/ui-unification/PROGRESS.md`

3) **Audits por página**
   - `docs/ui-unification/pages/{pageSlug}.md`
   - `docs/ui-unification/pages/_template.md`

4) **Decisiones / cambios de criterio**
   - `docs/ui-unification/DECISIONS.md`

#### Estructura sugerida

```
docs/ui-unification/
├── UI_UNIFICATION_STRATEGY.md
├── WORKPLAN.md
├── PROGRESS.md
├── DECISIONS.md
└── pages/
    ├── _template.md
    ├── tenants.md
    ├── users.md
    └── clients.md
```

#### Checklist de Completitud (FASE 0)
- [ ] Crear `WORKPLAN.md` con estado actual + próximos pasos concretos
- [ ] Crear/actualizar `PROGRESS.md` con tabla de páginas y estados
- [ ] Crear `pages/_template.md` para auditorías
- [ ] Crear `DECISIONS.md` para registrar excepciones/cambios
- [ ] Definir convención de commits checkpoint (sección 1.4)
- [ ] Commit checkpoint: `phase0: setup workplan + progress + templates`

---

### FASE 1 — Fundación (Theming + Tokens + Global UX)

**Duración Estimada:** 8-12 horas  
**Blocker:** Nada puede avanzar sin esto

**Output mínimo para empezar a migrar pages sin sufrir:**
1. Tokens semánticos (no "colors sueltos")
2. Theme switching (dark/light) consistente
3. Shadows clay (dark/light) definidas con HSL para fácil manipulación
4. Global layout/background/motion base
5. Tipografía cargada y aplicada

**Deliverables:**

| Archivo | Descripción |
|---------|-------------|
| `ui/lib/design/tokens.ts` | Tokens fuente (TypeScript) |
| `ui/app/globals.css` | CSS vars + base styles + reduced motion |
| `ui/tailwind.config.ts` | Mapeo tailwind → CSS vars |
| `ui/components/ds/theme-provider.tsx` | Theme switching (usar `next-themes`) |
| `docs/ui-unification/DESIGN_TOKENS.md` | Documentación de tokens |

**Checklist de Completitud:**
- [ ] Tokens semánticos definidos (bg, surface, card, text, muted, border, accent, etc.)
- [ ] HSL mapping para fácil generación de variantes
- [ ] Shadows clay 4-layer system (card, float, press, button) en dark/light
- [ ] Tailwind config mapeado a CSS vars
- [ ] Theme switching funcional con persistencia
- [ ] Fonts cargadas (Nunito, DM Sans, Fira Code)
- [ ] Motion base (ease-out, duraciones) definido
- [ ] `prefers-reduced-motion` respetado
- [ ] Documentación actualizada
- [ ] Commit checkpoint: `phase1: tokens + globals + theme switching`

---

### FASE 2 — Design System Kit (Componentes DS)

**Duración Estimada:** 20-30 horas  
**Blocker:** Requiere FASE 1 completa

**Meta:** Crear un set chico pero poderoso que cubra el 80% del panel.

#### 2.1 Componentes — Prioridad por Olas

**Ola 1 — Fundación (ANTES de migrar páginas):**

Estos componentes son **CRÍTICOS** y deben estar 100% completos antes de migrar cualquier página.

| Componente | Variantes | Estados | Prioridad |
|------------|-----------|---------|-----------|
| `Button` | primary, secondary, ghost, danger, outline | default, hover, active, disabled, loading | 🔴 CRÍTICO |
| `Card` | default, glass, gradient | default, hover | 🔴 CRÍTICO |
| `Input` | default, error | default, focus, disabled, error | 🔴 CRÍTICO |
| `Textarea` | - | default, focus, disabled | 🟡 ALTO |
| `Badge` | default, success, warning, danger, info | - | 🟡 ALTO |
| `PageShell` | - | - | 🔴 CRÍTICO |
| `PageHeader` | with/without actions | - | 🔴 CRÍTICO |
| `Section` | - | - | 🔴 CRÍTICO |
| `Skeleton` | - | shimmer animation | 🔴 CRÍTICO |
| `Loader` | spinner, dots | - | 🔴 CRÍTICO |
| `Toast` | success, error, warning, info | - | 🔴 CRÍTICO |

**Ola 2 — Data Display (para pages con tablas):**

| Componente | Descripción | Prioridad |
|------------|-------------|-----------|
| `DataTable` | Headless base + clay styling, sortable | 🟡 ALTO |
| `Pagination` | Server-side ready | 🟢 MEDIO |
| `EmptyState` | Mensaje + acción CTA | 🟡 ALTO |
| `InlineAlert` | Info, warning, error, success | 🟡 ALTO |

**Ola 3 — Overlays & Advanced:**

| Componente | Base | Prioridad |
|------------|------|-----------|
| `Dialog` | Radix UI Dialog | 🟡 ALTO |
| `Dropdown` | Radix UI DropdownMenu | 🟢 MEDIO |
| `Tooltip` | Radix UI Tooltip | 🟢 MEDIO |
| `Select` | Radix UI Select | 🟡 ALTO |

**Ola 4 — Utilities (según necesidad):**

| Componente | Descripción | Prioridad |
|------------|-------------|-----------|
| `CopyButton` | Copy to clipboard con feedback | 🟢 MEDIO |
| `CodeBlock` | Syntax highlight con copy | 🟢 MEDIO |
| `KeyValueRow` | Key-value pair display | 🟢 MEDIO |
| `Separator` | Divider horizontal/vertical | 🟢 BAJO |
| `Toolbar` | Search + filters container | 🟢 MEDIO |

**Regla de Oro:** **No arrancar migración page-by-page hasta tener Ola 1 completa.**

#### 2.2 Estructura de Archivos

```
ui/components/ds/
├── core/
│   ├── button.tsx
│   ├── card.tsx
│   ├── input.tsx
│   ├── textarea.tsx
│   ├── select.tsx
│   ├── badge.tsx
│   └── separator.tsx
├── layout/
│   ├── page-shell.tsx
│   ├── page-header.tsx
│   ├── toolbar.tsx
│   └── section.tsx
├── feedback/
│   ├── toast.tsx
│   ├── inline-alert.tsx
│   ├── empty-state.tsx
│   ├── loader.tsx
│   └── skeleton.tsx
├── overlays/
│   ├── dialog.tsx
│   ├── dropdown.tsx
│   └── tooltip.tsx
├── data/
│   ├── data-table.tsx
│   └── pagination.tsx
├── utils/
│   ├── cn.ts              # classnames utility
│   ├── copy-button.tsx
│   ├── code-block.tsx
│   └── key-value-row.tsx
├── theme-provider.tsx
└── index.ts               # Barrel export
```

#### 2.3 Reglas de Implementación DS

**Variantes con CVA:**
```typescript
import { cva, type VariantProps } from "class-variance-authority"

const buttonVariants = cva(
  "inline-flex items-center justify-center rounded-button font-medium transition-all duration-200",
  {
    variants: {
      variant: {
        primary: "bg-accent text-white shadow-button hover:shadow-float active:scale-[0.98]",
        secondary: "bg-surface text-text shadow-card hover:shadow-float",
        ghost: "hover:bg-surface",
        danger: "bg-danger text-white shadow-button hover:shadow-float",
      },
      size: {
        sm: "h-9 px-3 text-sm",
        md: "h-11 px-4 text-base",
        lg: "h-14 px-6 text-lg",
      },
    },
    defaultVariants: {
      variant: "primary",
      size: "md",
    },
  }
)
```

**Reglas Obligatorias:**
- ✅ Todo componente tiene `className` prop mergeable con `cn()`
- ✅ Estados: `disabled`, `loading` (cuando aplica)
- ✅ Focus visible y consistente con ring accent
- ✅ Radix permitido como base headless, pero **el look lo define DS**
- ✅ JSDoc comments obligatorios
- ✅ Export en `index.ts`

**Checkpoints recomendados (sin PRs):**
- Un commit por componente o por "paquete chico" de componentes.
- Ejemplo: `ds: add Button/Card/Input (ola1)`

---

### FASE 3 — Migración Page-by-Page (Ciclo Estricto)

**Duración Estimada:** 40-60 horas (distribuidas en iteraciones)  
**Blocker:** Requiere FASE 2 (Ola 1) completa

Cada página sigue **EXACTAMENTE** este ciclo:

#### CICLO POR PÁGINA (2 iteraciones)

##### A) Auditoría Rápida (antes de tocar UI)

**Duración:** 30-60 min  
**Output:** `docs/ui-unification/pages/{pageSlug}.md`

**Template Copy/Paste:**

```markdown
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

---

**Next Steps**:
1. Crear `DebouncedInput` en DS antes de migrar
2. Verificar `DataTable` soporta bulk actions
3. Arrancar iteración Dark
```

**Checklist de Auditoría:**
- [ ] Qué hace la página (1 párrafo)
- [ ] Flujos principales (crear/editar/borrar/rotar/etc.)
- [ ] Componentes que usa hoy (tabla inventario)
- [ ] Patrones UX (tabs, wizard, table, filters)
- [ ] Estados existentes (loading/empty/error)
- [ ] Inconsistencias detectadas (lista)
- [ ] "Riesgos" (cosas sensibles de romper)
- [ ] Screenshot BEFORE (dark y light)

**Regla:** Si detectás que faltan componentes DS, **se crean antes** de rediseñar la página.

**Checkpoint recomendado:**
- Commit: `page({pageSlug}): audit`

---

##### B) Iteración 1 — Dark (la "verdad" del sistema)

**Duración:** 3-6 horas  
**Objetivo:** La página queda final-form en dark.

**Pasos:**
1. **Layout**: Reemplazar por `PageShell` + `PageHeader` + `Section`
2. **UI Components**: Reemplazar por componentes DS (card/button/input/table/dialog)
3. **Jerarquía**: Definir título + descripción + acciones primarias/secundarias
4. **Estados**: Implementar skeleton/empty/error/toast
5. **Microinteracciones**: Hover lift + press + focus
6. **Screenshot AFTER** (dark)

**DoD Iteración Dark:**
- [ ] La página se siente "HelloJohn DS" en dark
- [ ] Acciones claves tienen feedback (toast/inline)
- [ ] Tab order y focus OK
- [ ] No quedan estilos inline raros
- [ ] Loading states con skeleton (no spinner solo)
- [ ] Empty states con mensaje + acción
- [ ] Error states con retry action
- [ ] Confirmación dialogs para acciones peligrosas

**Checkpoint recomendado:**
- Commit: `page({pageSlug}): dark iteration`

---

##### C) Iteración 2 — Light (paridad visual)

**Duración:** 1-3 horas  
**Objetivo:** Misma calidad que dark, sin "lavarse" ni perder contraste.

**Pasos:**
1. **Ajustar tokens** si aparecen problemas (pero mínimo)
2. **Revisar contraste** con axe DevTools
3. **Revisar sombras**, border, superficies
4. **Confirmar estados** (loading/empty/error) se ven bien
5. **Screenshot AFTER** (light)

**DoD Iteración Light:**
- [ ] La página se ve igual de premium que dark
- [ ] Contrastes WCAG AA mínimo (AAA donde posible)
- [ ] No hay "mismo componente pero distinto look" respecto al resto
- [ ] Shadows clay se ven correctas (no "planas")
- [ ] Accents legibles y con suficiente contraste

**Checkpoint recomendado:**
- Commit: `page({pageSlug}): light iteration`

---

##### D) Cierre de Página

**Checklist Final:**
- [ ] DoD completo (Visual + UX + A11y + Performance + QA)
- [ ] Screenshots before/after (dark/light) guardados o linkeados en doc
- [ ] Actualizar `docs/ui-unification/PROGRESS.md`
- [ ] Actualizar `docs/ui-unification/WORKPLAN.md` (próximo paso)
- [ ] **Smoke run local** (obligatorio antes de commit final):
  - [ ] `pnpm lint` — sin errores
  - [ ] `pnpm typecheck` — sin errores
  - [ ] Abrir la página en **dark** y verificar 2 flujos principales
  - [ ] Abrir la página en **light** y verificar 2 flujos principales
- [ ] Commit final: `page({pageSlug}): done + docs + screenshots`

**Nota:** No existe "merge a main" como paso porque ya se trabaja sobre main.

---

### FASE 4 — Hardening & Optimización Global

**Duración Estimada:** 8-12 horas  
**Timing:** Cuando varias páginas ya están migradas (o al final)

**Bundle/CSS:**
- [ ] Evitar duplicación de estilos
- [ ] Auditar clases repetidas → subir a componentes DS
- [ ] PurgeCSS configurado correctamente
- [ ] Bundle size < 200KB (gzip)

**Performance:**
- [ ] Tablas grandes: virtualización solo si duele (200+ rows)
- [ ] Evitar blur/sombras heavy en listas con 100+ items
- [ ] Lighthouse score 90+ en todas las páginas core
- [ ] First Contentful Paint < 1.5s

**A11y Audit:**
- [ ] Axe DevTools sobre páginas core (dashboard/tenants/users/clients)
- [ ] Keyboard navigation completa sin mouse
- [ ] Screen reader testing (NVDA/VoiceOver) en 3 páginas clave
- [ ] Contrast ratio verificado en ambos temas

**Consistencia:**
- [ ] Revisar headers, spacing, toasts, dialogs en todo el panel
- [ ] Verificar que patrones son consistentes (no 3 formas de hacer lo mismo)
- [ ] Eliminar componentes/patrones deprecados

**Checkpoint recomendado:**
- Commit: `phase4: hardening perf + a11y + cleanup`

---

## 4. Theming & Tokens (Paletas Separadas, Mismo Lenguaje)

### 4.1 Regla: Tokens Semánticos, No "Colores Sueltos"

**❌ MAL:**
```tsx
<div className="bg-purple-500 text-white">...</div>
```

**✅ BIEN:**
```tsx
<div className="bg-accent text-white">...</div>
```

**Tokens Obligatorios:**
- **Base**: `--bg`, `--bg-2`, `--surface`, `--surface-hover`, `--card`
- **Text**: `--text`, `--muted`, `--subtle`
- **Borders**: `--border`
- **Semantic**: `--accent`, `--accent-2`, `--info`, `--success`, `--warning`, `--danger`
- **Shadows**: `--shadow-card`, `--shadow-float`, `--shadow-press`, `--shadow-button`

### 4.2 CSS Variables (HSL para Fácil Manipulación)

```css
/* ui/app/globals.css */

:root {
  /* Typography */
  --font-body: "DM Sans", ui-sans-serif, system-ui;
  --font-heading: "Nunito", ui-sans-serif, system-ui;
  --font-mono: ui-monospace, SFMono-Regular, Menlo, monospace;

  /* Motion */
  --ease-out: cubic-bezier(0.16, 1, 0.3, 1);
  --dur-1: 120ms;
  --dur-2: 200ms;
  --dur-3: 320ms;

  /* Radii */
  --r-lg: 28px;
  --r-card: 24px;
  --r-md: 18px;
  --r-sm: 14px;
  --r-button: 20px;

  /* Spacing scale (semantic) */
  --page-px: 24px;
  --page-py: 24px;
  --section-gap: 16px;
}

/* ============================================
   DARK MODE — "Midnight Clay"
   Premium, cinematic, depth
   ============================================ */
.dark {
  /* Base colors */
  --bg: #050506;
  --bg-2: #0a0a0c;

  --surface: rgba(255,255,255,0.06);
  --surface-hover: rgba(255,255,255,0.09);
  --card: rgba(255,255,255,0.07);

  --text: #EDEDEF;
  --muted: #A1A1AA;
  --subtle: rgba(255,255,255,0.65);

  --border: rgba(255,255,255,0.09);

  /* Accent colors en HSL (fácil generar variantes) */
  --accent-h: 258;
  --accent-s: 77%;
  --accent-l: 57%;
  --accent: hsl(var(--accent-h) var(--accent-s) var(--accent-l));
  --accent-hover: hsl(var(--accent-h) var(--accent-s) calc(var(--accent-l) + 5%));
  --accent-active: hsl(var(--accent-h) var(--accent-s) calc(var(--accent-l) - 5%));

  /* Accent opacity variants */
  --accent-10: hsl(var(--accent-h) var(--accent-s) var(--accent-l) / 0.1);
  --accent-20: hsl(var(--accent-h) var(--accent-s) var(--accent-l) / 0.2);
  --accent-30: hsl(var(--accent-h) var(--accent-s) var(--accent-l) / 0.3);

  --accent-2: #DB2777;
  --info: #38BDF8;
  --success: #34D399;
  --warning: #FBBF24;
  --danger: #FB7185;

  /* Clay shadows (dark) — menos "plastic", más depth */
  --shadow-card:
    0 0 0 1px rgba(255,255,255,0.07),
    0 14px 40px rgba(0,0,0,0.55),
    0 0 80px rgba(124,58,237,0.08);

  --shadow-float:
    0 0 0 1px rgba(255,255,255,0.09),
    0 22px 70px rgba(0,0,0,0.6),
    0 0 120px rgba(124,58,237,0.12);

  --shadow-press:
    inset 10px 10px 22px rgba(0,0,0,0.65),
    inset -10px -10px 22px rgba(255,255,255,0.03);

  --shadow-button:
    0 0 0 1px rgba(124,58,237,0.40),
    0 10px 26px rgba(124,58,237,0.22),
    inset 0 1px 0 rgba(255,255,255,0.14);
}

/* ============================================
   LIGHT MODE — "Candy Clay" Refinado
   Enterprise-friendly, high fidelity
   ============================================ */
.light {
  /* Base colors */
  --bg: #F4F1FA;
  --bg-2: #FFFFFF;

  --surface: rgba(255,255,255,0.66);
  --surface-hover: rgba(255,255,255,0.82);
  --card: rgba(255,255,255,0.74);

  --text: #332F3A;
  --muted: #635F69;
  --subtle: rgba(51,47,58,0.72);

  --border: rgba(51,47,58,0.10);

  /* Accent colors en HSL */
  --accent-h: 258;
  --accent-s: 77%;
  --accent-l: 57%;
  --accent: hsl(var(--accent-h) var(--accent-s) var(--accent-l));
  --accent-hover: hsl(var(--accent-h) var(--accent-s) calc(var(--accent-l) + 5%));
  --accent-active: hsl(var(--accent-h) var(--accent-s) calc(var(--accent-l) - 5%));

  /* Accent opacity variants */
  --accent-10: hsl(var(--accent-h) var(--accent-s) var(--accent-l) / 0.1);
  --accent-20: hsl(var(--accent-h) var(--accent-s) var(--accent-l) / 0.2);
  --accent-30: hsl(var(--accent-h) var(--accent-s) var(--accent-l) / 0.3);

  --accent-2: #DB2777;
  --info: #0EA5E9;
  --success: #10B981;
  --warning: #F59E0B;
  --danger: #FB7185;

  /* Clay shadows (light) — high fidelity */
  --shadow-card:
    16px 16px 32px rgba(160,150,180,0.22),
    -10px -10px 24px rgba(255,255,255,0.92),
    inset 6px 6px 12px rgba(124,58,237,0.04),
    inset -6px -6px 12px rgba(255,255,255,1);

  --shadow-float:
    18px 18px 44px rgba(160,150,180,0.26),
    -12px -12px 28px rgba(255,255,255,0.96),
    inset 6px 6px 12px rgba(124,58,237,0.05),
    inset -6px -6px 12px rgba(255,255,255,1);

  --shadow-press:
    inset 10px 10px 20px #d9d4e3,
    inset -10px -10px 20px #ffffff;

  --shadow-button:
    12px 12px 24px rgba(124,58,237,0.28),
    -8px -8px 16px rgba(255,255,255,0.42),
    inset 4px 4px 8px rgba(255,255,255,0.40),
    inset -4px -4px 8px rgba(0,0,0,0.10);
}

/* Reduced Motion */
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
```

---

## 5. Tailwind (Mapeo a Tokens)

```typescript
// ui/tailwind.config.ts
import type { Config } from "tailwindcss"

const config: Config = {
  darkMode: ["class"],
  content: [
    "./pages/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./app/**/*.{ts,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        bg: "var(--bg)",
        "bg-2": "var(--bg-2)",
        surface: "var(--surface)",
        "surface-hover": "var(--surface-hover)",
        card: "var(--card)",
        text: "var(--text)",
        muted: "var(--muted)",
        subtle: "var(--subtle)",
        border: "var(--border)",
        accent: "var(--accent)",
        "accent-hover": "var(--accent-hover)",
        "accent-active": "var(--accent-active)",
        "accent-2": "var(--accent-2)",
        info: "var(--info)",
        success: "var(--success)",
        warning: "var(--warning)",
        danger: "var(--danger)",
      },
      boxShadow: {
        card: "var(--shadow-card)",
        float: "var(--shadow-float)",
        press: "var(--shadow-press)",
        button: "var(--shadow-button)",
      },
      borderRadius: {
        lg: "var(--r-lg)",
        card: "var(--r-card)",
        md: "var(--r-md)",
        sm: "var(--r-sm)",
        button: "var(--r-button)",
      },
      fontFamily: {
        body: "var(--font-body)",
        heading: "var(--font-heading)",
        mono: "var(--font-mono)",
      },
      transitionTimingFunction: {
        out: "var(--ease-out)",
      },
      transitionDuration: {
        120: "var(--dur-1)",
        200: "var(--dur-2)",
        320: "var(--dur-3)",
      },
    },
  },
  plugins: [require("tailwindcss-animate")],
}

export default config
```

---

## 6. Patrones UX Obligatorios (para TODO el Panel)

### 6.1 Page Layout Estándar

**Toda página de admin debe seguir este patrón:**

```tsx
<PageShell>
  <PageHeader
    title="Cluster Management"
    description="Manage Raft cluster nodes and configuration"
    actions={
      <>
        <Button variant="secondary">
          <RefreshCw className="w-4 h-4 mr-2" />
          Refresh
        </Button>
        <Button variant="primary">
          <Plus className="w-4 h-4 mr-2" />
          Add Node
        </Button>
      </>
    }
  />

  {/* Toolbar (si hay tabla/filtros) */}
  <Toolbar>
    <Input placeholder="Search..." />
    <Select>...</Select>
  </Toolbar>

  {/* Content */}
  <Section>
    <Card>
      {/* Contenido principal */}
    </Card>
  </Section>
</PageShell>
```

**Componentes:**
- `PageHeader`: Título + descripción + acciones primarias (derecha)
- `Toolbar` (si hay tabla): Search + filters + bulk actions
- `Content`: Cards/sections consistentes con spacing correcto

### 6.2 Estados Estándar

| Estado | Implementación | Obligatorio |
|--------|---------------|-------------|
| **Loading** | `<Skeleton>` para layout final (no spinner solo) | ✅ Sí |
| **Empty** | `<EmptyState>` con mensaje + acción CTA | ✅ Sí |
| **Error** | `<InlineAlert variant="error">` + retry action | ✅ Sí |
| **Success** | `<Toast variant="success">` + detalles opcionales | ✅ Sí |
| **Confirmación** | `<Dialog>` para delete/rotate/reset | ✅ Sí (acciones peligrosas) |

### 6.3 Microinteracciones (Punto Medio)

**Animaciones sutiles pero perceptibles:**
- **Hover lift**: `2-6px` (no más) — `hover:-translate-y-1`
- **Press**: `scale(0.98)` o "pressed shadow" — `active:scale-[0.98]`
- **Duraciones**: `120-320ms` con `--ease-out`
- **Focus ring**: `ring-2 ring-accent/30 ring-offset-2`

**Nada de rebotes tipo "juguete" en admin; sí "tacto clay" premium.**

---

## 7. Performance Rules (Para Que Vuele)

### 7.1 Listas y Tablas Grandes

**Regla:** Evitar sombras ultra pesadas en cada row.

**Patrón correcto:**
- Las rows de tabla usan borde/hover suave
- La "clay depth" queda en contenedores (Card/Table wrapper)

**Si hay 200+ rows reales (no paginadas):**

#### Opción A — Virtualización con `@tanstack/react-virtual` (11kb)

```tsx
import { useVirtualizer } from '@tanstack/react-virtual'
import { useRef } from 'react'

export function VirtualTable({ rows }: { rows: Row[] }) {
  const parentRef = useRef<HTMLDivElement>(null)

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 48, // row height
    overscan: 5,
  })

  return (
    <div ref={parentRef} className="h-[600px] overflow-auto">
      <div style={{ height: `${virtualizer.getTotalSize()}px`, position: 'relative' }}>
        {virtualizer.getVirtualItems().map((virtualRow) => (
          <div
            key={virtualRow.index}
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              height: `${virtualRow.size}px`,
              transform: `translateY(${virtualRow.start}px)`,
            }}
          >
            <TableRow data={rows[virtualRow.index]} />
          </div>
        ))}
      </div>
    </div>
  )
}
```

#### Opción B — Paginación Server-Side (siempre mejor si es posible)

- Limitar a 50-100 rows por página
- Backend maneja el paging
- Usar `<Pagination>` component

**Regla:** Si no duele (< 200 rows), **no virtualizar**.

### 7.2 Animaciones: Dónde Sí / Dónde No

**✅ Sí:**
- Botones, cards, dialogs, toasts
- Skeleton shimmer suave
- Hover effects en elementos individuales

**❌ No:**
- Animaciones complejas en tablas enormes
- Blur gigante por todos lados
- Animaciones heavy en cada row de lista larga

### 7.3 Componentes Caros

- Dialog, Dropdown, Tooltip deben ser **livianos y reusables**
- Evitar recomputar columns/rows en cada render (usar `useMemo`)
- Lazy load componentes pesados: `const Dialog = lazy(() => import('./dialog'))`

---

## 8. Testing & QA (Mínimo Viable pero Serio)

### 8.1 Visual Regression (Recomendado)

**Playwright screenshots por página:**

```typescript
// tests/visual/cluster.spec.ts
test.describe('Cluster Page Visual Regression', () => {
  test('dark mode screenshot', async ({ page }) => {
    await page.goto('/admin/cluster')
    await page.evaluate(() => document.documentElement.classList.add('dark'))
    await expect(page).toHaveScreenshot('cluster-dark.png')
  })

  test('light mode screenshot', async ({ page }) => {
    await page.goto('/admin/cluster')
    await page.evaluate(() => document.documentElement.classList.remove('dark'))
    await expect(page).toHaveScreenshot('cluster-light.png')
  })
})
```

**Proceso:**
1. Tomar baseline (primera vez)
2. Comparar en cada cambio grande
3. Si hay diff intencional, actualizar baseline

### 8.2 A11y Smoke Test

**Axe DevTools sobre páginas core:**
- Dashboard
- Tenants
- Users
- Clients

**Checklist mínimo:**
- [ ] Contrast ratio OK
- [ ] Focus order lógico
- [ ] Labels en inputs
- [ ] Headings jerárquicos (h1 → h2 → h3)
- [ ] ARIA labels donde corresponda

---

## 9. Documentación Operativa (Fuente de Verdad de Proceso)

### 9.1 Workplan (control diario)

**Archivo:** `docs/ui-unification/WORKPLAN.md`

**Debe contener:**
- Fase actual
- Tarea actual
- Próximos 3 pasos concretos
- Bloqueos
- Notas de decisiones recientes (o link a DECISIONS)

**Template sugerido:**

```markdown
# UI Unification — WORKPLAN

## Current Phase
- Phase: 0 | 1 | 2 | 3 | 4

## Today
- Task: ...

## Next Steps (max 3)
1. ...
2. ...
3. ...

## Blockers
- None | ...

## Notes
- ...
```

### 9.2 Progress Tracking

**Archivo:** `docs/ui-unification/PROGRESS.md`

**Tabla:**

| Page | Dark | Light | Status | Last Commit | Updated | Notes |
|------|------|-------|--------|-------------|---------|-------|
| `/admin` | ✅ | ✅ | ✅ DONE | `a1b2c3d` | 2026-01-30 | Dashboard migrado |
| `/admin/tenants` | ✅ | 🚧 | 🎨 LIGHT_IN_PROGRESS | `e4f5g6h` | 2026-01-30 | Ajustando contraste |
| `/admin/cluster` | ⏳ | ⏳ | ⏳ PENDING | - | - | - |

**Columnas:**
- **Last Commit**: Hash corto o mensaje del último checkpoint (permite rastrear sin abrir Git history)
- **Updated**: Fecha del último cambio en esa página

**Estados:**
- `⏳ PENDING`
- `🔍 AUDIT`
- `🚧 DARK_IN_PROGRESS`
- `🎨 LIGHT_IN_PROGRESS`
- `✅ DONE`
- `🚫 BLOCKED`

### 9.3 Decisions Log

**Archivo:** `docs/ui-unification/DECISIONS.md`

**Objetivo:** registrar cambios grandes de criterio (para no olvidarte por qué se decidió algo).

**Template sugerido:**

```markdown
# UI Unification — Decisions Log

## 2026-01-30 — Single-dev, low ceremony
- Decision: No PRs/branches by default
- Why: Solo dev, proyecto en desarrollo, fricción innecesaria
- Impact: Checkpoints via commits + docs

## YYYY-MM-DD — Title
- Decision:
- Why:
- Impact:
- Alternatives considered:
```

### 9.4 Screenshots

Guardar before/after por página (dark/light).

Puede ser en carpeta local (ej: `docs/ui-unification/screenshots/{pageSlug}/...`) o link.

Lo importante: que el audit doc apunte a dónde están.

---

## 10. Orden Sugerido de Migración (Prioridad por Valor)

**Prioridad 1: Core Pages (High Traffic)**
1. `/admin` — Dashboard/Home
2. `/admin/tenants` — Tenant management
3. `/admin/users` — User management
4. `/admin/clients` — OAuth clients

**Prioridad 2: Configuración**
5. `/admin/keys` — Keys/rotation
6. `/admin/cluster` — Cluster management
7. `/admin/settings` — Tenant settings
8. `/admin/scopes` — Scopes management

**Prioridad 3: Features Específicos**
9. `/admin/sessions` — Session management
10. `/admin/tokens` — Token management
11. `/admin/rbac` — RBAC management
12. `/admin/consents` — Consents management

**Prioridad 4: Utilities & Tools**
13. `/admin/playground` — OAuth playground
14. `/admin/logs` — Logs viewer
15. `/admin/metrics` — Metrics dashboard
16. `/admin/database` — Database management
17. `/admin/mailing` — Mailing configuration

**Nota:** El orden final puede ajustarse, pero **siempre se respeta el ciclo Dark→Light y DoD.**

---

## 11. Protocolos de Creación de Componentes DS

**Cuando una page pide algo nuevo:**

1. **Identificar patrón** (ej: "IconStatCard", "KeyValueList", "DangerZone")
2. **Verificar si ya existe** en DS con otro nombre
3. **Crear en `ui/components/ds/...`** con estructura correcta
4. **Documentar** en JSDoc + ejemplos simples
5. **Usar en la page** (0 estilos ad-hoc duplicados)
6. **Si aparece un segundo uso**: Se convierte en patrón oficial y se documenta

**Regla de Promoción:**
- 1 uso = Componente DS válido
- 2+ usos = Patrón oficial (documentar en `DESIGN_SYSTEM.md`)

---

## 12. Anti-Patterns (Prohibidos)

**❌ NUNCA hacer esto:**

### 1. Colores hardcodeados en páginas
```tsx
// ❌ MAL
<div className="bg-purple-500 text-white">...</div>

// ✅ BIEN
<div className="bg-accent text-white">...</div>
```

### 2. Corners genéricos (rounded-md)
```tsx
// ❌ MAL
<Card className="rounded-md">...</Card>

// ✅ BIEN
<Card className="rounded-card">...</Card>
```

### 3. 10 variantes de headers (cada una distinta)
- Usar `<PageHeader>` en TODAS las páginas

### 4. Botones sin estados (loading/disabled)
```tsx
// ❌ MAL
<button onClick={handleClick}>Submit</button>

// ✅ BIEN
<Button onClick={handleClick} loading={isLoading} disabled={!isValid}>
  Submit
</Button>
```

### 5. Mensajes de error crípticos sin acción
```tsx
// ❌ MAL
<InlineAlert variant="error">Error</InlineAlert>

// ✅ BIEN
<InlineAlert variant="error" action={<Button onClick={retry}>Retry</Button>}>
  Failed to load data. Please try again.
</InlineAlert>
```

### 6. Animaciones pesadas en tablas grandes
- No aplicar `shadow-float` + `hover:scale-105` en cada row de 200+ items

### 7. "Arreglos" visuales con padding/margin random sin tokens
```tsx
// ❌ MAL
<div className="mb-[17px] ml-[23px]">...</div>

// ✅ BIEN
<div className="mb-4 ml-6">...</div>
```

---

## 13. Próximos Pasos (Ejecución Inmediata)

### ✅ Paso 0: FASE 0 (Setup Operativo)
1. Crear `WORKPLAN.md`, `PROGRESS.md`, `DECISIONS.md`
2. Crear `pages/_template.md`
3. Commit: `phase0: setup workplan + progress + templates`

### ✅ Paso 1: FASE 1 (Fundación)
1. Implementar tokens (`ui/lib/design/tokens.ts`)
2. Crear `globals.css` con CSS vars
3. Configurar Tailwind mapping
4. Implementar theme switching
5. Cargar fonts (Nunito, DM Sans, Fira Code)

### ✅ Paso 2: FASE 2 Ola 1 (DS Kit Mínimo)
Armar componentes críticos:
- `Button`, `Card`, `Input`, `Badge`
- `PageShell`, `PageHeader`, `Section`
- `Skeleton`, `Toast`

### ✅ Paso 3: Migración (Ciclo: Audit → Dark → Light → Done)
1. Elegir página de Prioridad 1
2. Auditar (`docs/ui-unification/pages/{pageSlug}.md`)
3. Iteración Dark
4. Iteración Light
5. Cierre + actualizar PROGRESS/WORKPLAN + commit DONE

### ✅ Paso 4: FASE 4 (Hardening)
1. Bundle optimization
2. A11y audit completo
3. Performance testing
4. Consistencia global

---

## Appendix A — Migration Checklist (Copy/Paste)

```markdown
# Migration Checklist — /admin/{pageSlug}

## Audit
- [ ] Purpose + primary actions
- [ ] UI inventory (components/patterns)
- [ ] States (loading/empty/error)
- [ ] Risks
- [ ] Screenshot BEFORE (dark + light)
- [ ] Commit: page({pageSlug}): audit

## Dark Iteration
- [ ] PageShell + PageHeader + Sections
- [ ] Replace UI with DS components
- [ ] Loading / Empty / Error / Success
- [ ] Dialog confirmations
- [ ] Focus & keyboard
- [ ] Screenshot AFTER (dark)
- [ ] Commit: page({pageSlug}): dark iteration

## Light Iteration
- [ ] Palette parity (no washed-out)
- [ ] Contrast check (axe DevTools)
- [ ] Shadows/surfaces consistent
- [ ] Screenshot AFTER (light)
- [ ] Commit: page({pageSlug}): light iteration

## Done
- [ ] DoD complete (Visual + UX + A11y + Perf + QA)
- [ ] Update PROGRESS.md + WORKPLAN.md
- [ ] Save/link screenshots
- [ ] Commit: page({pageSlug}): done + docs + screenshots
```

---

## Appendix B — Visual Examples (Do/Don't)

### ✅ DO — Page Header Consistente

```tsx
<PageShell>
  <PageHeader
    title="Cluster Management"
    description="Manage Raft cluster nodes and configuration"
    actions={
      <>
        <Button variant="secondary">
          <RefreshCw className="w-4 h-4 mr-2" />
          Refresh
        </Button>
        <Button variant="primary">
          <Plus className="w-4 h-4 mr-2" />
          Add Node
        </Button>
      </>
    }
  />
  <Section>
    {/* content */}
  </Section>
</PageShell>
```

### ❌ DON'T — Header Custom Ad-Hoc

```tsx
<div className="mb-8">
  <h1 className="text-3xl font-bold mb-2">Cluster Management</h1>
  <p className="text-gray-500 mb-4">Manage nodes...</p>
  <div className="flex gap-2">
    <button className="px-4 py-2 bg-white rounded">Refresh</button>
    <button className="px-4 py-2 bg-purple-500 text-white rounded">Add</button>
  </div>
</div>
```

**Por qué:** Headers custom rompen consistencia visual y son código duplicado.

---

### ✅ DO — Loading State con Skeleton

```tsx
{isLoading ? (
  <Card>
    <Skeleton className="h-12 w-full mb-4" />
    <Skeleton className="h-8 w-3/4 mb-2" />
    <Skeleton className="h-8 w-1/2" />
  </Card>
) : (
  <Card>{data}</Card>
)}
```

### ❌ DON'T — Spinner Solo

```tsx
{isLoading ? <Spinner /> : <Card>{data}</Card>}
```

**Por qué:** Skeleton mantiene layout (no hay "jump") y se siente más rápido.

---

### ✅ DO — Empty State con Acción

```tsx
<EmptyState
  icon={<Inbox className="w-12 h-12" />}
  title="No tenants found"
  description="Get started by creating your first tenant"
  action={
    <Button onClick={handleCreate}>
      <Plus className="w-4 h-4 mr-2" />
      Create Tenant
    </Button>
  }
/>
```

### ❌ DON'T — Mensaje Genérico Sin Acción

```tsx
<div className="text-center py-12">
  <p className="text-gray-500">No data</p>
</div>
```

**Por qué:** Empty states deben guiar al usuario hacia la acción correcta.

---

### ✅ DO — Confirmación de Acción Peligrosa

```tsx
<Dialog>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>Delete Tenant</DialogTitle>
      <DialogDescription>
        This will permanently delete "{tenantName}" and all associated data.
        This action cannot be undone.
      </DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="ghost" onClick={onCancel}>Cancel</Button>
      <Button variant="danger" onClick={onConfirm} loading={isDeleting}>
        Delete Tenant
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

### ❌ DON'T — Delete Sin Confirmación

```tsx
<Button onClick={() => deleteTenant(id)}>Delete</Button>
```

**Por qué:** Acciones destructivas requieren confirmación explícita.

---

### ✅ DO — Error con Retry Action

```tsx
<InlineAlert
  variant="error"
  action={<Button size="sm" onClick={retry}>Retry</Button>}
>
  Failed to load cluster nodes. Please try again.
</InlineAlert>
```

### ❌ DON'T — Error Críptico

```tsx
<div className="text-red-500">Error</div>
```

**Por qué:** Errores deben ser claros y ofrecer solución.

---

## Appendix C — Component API Reference (Quick)

### Button

```tsx
<Button
  variant="primary" | "secondary" | "ghost" | "danger" | "outline"
  size="sm" | "md" | "lg"
  loading={boolean}
  disabled={boolean}
  onClick={handler}
>
  Children
</Button>
```

### Card

```tsx
<Card
  variant="default" | "glass" | "gradient"
  className={string}
>
  Children
</Card>
```

### PageHeader

```tsx
<PageHeader
  title={string}
  description={string}
  actions={ReactNode}
/>
```

### Skeleton

```tsx
<Skeleton className="h-12 w-full" />
```

### Toast

```tsx
toast({
  title: string,
  description: string,
  variant: "success" | "error" | "warning" | "info",
  duration: number, // ms
})
```

### Dialog

```tsx
<Dialog open={isOpen} onOpenChange={setIsOpen}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>{title}</DialogTitle>
      <DialogDescription>{description}</DialogDescription>
    </DialogHeader>
    {children}
    <DialogFooter>
      <Button variant="ghost" onClick={onCancel}>Cancel</Button>
      <Button variant="primary" onClick={onConfirm}>Confirm</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

---

## 🚀 Ready to Start

**Este documento es la fuente de verdad para la unificación UI.**

**Próximo paso recomendado:** Ejecutar FASE 0 (Setup Operativo) y dejar listo el control local.

---

**FIN DEL DOCUMENTO**
