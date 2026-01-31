# Design Tokens — UI Unification Phase 1

> **Status:** ✅ Implemented (Phase 1 Complete)
> **Source:** `ui/app/globals.css` (CSS Variables) + `ui/lib/design/tokens.ts` (TypeScript Contract)
> **Last Updated:** 2026-01-30

---

## 1. Semantic Colors

### Base Colors
- `--background` / `--foreground` — Main background and text
- `--card` / `--card-foreground` — Card surfaces
- `--popover` / `--popover-foreground` — Popup/overlay surfaces

### Surfaces & Backgrounds
- `--secondary` / `--secondary-foreground` — Secondary surfaces
- `--muted` / `--muted-foreground` — Muted text/surfaces
- `--border` — Border color
- `--input` — Input border color

### Accent Colors (HSL-based for manipulation)
- `--accent-h`, `--accent-s`, `--accent-l` — Accent base (Purple: 258, 77%, 57%)
- `--accent` — Computed accent color
- `--accent-hover` — Computed hover state (+5% lightness)
- `--accent-active` — Computed active state (-5% lightness)
- `--accent-foreground` — Text on accent background

**Opacity Variants:**
- `--accent-10`, `--accent-20`, `--accent-30` — Transparent accent variants

### Semantic Colors
- `--accent-2` — Secondary accent (#DB2777)
- `--info` — Information (#0EA5E9 light / #38BDF8 dark)
- `--success` — Success (#10B981 light / #34D399 dark)
- `--warning` — Warning (#F59E0B light / #FBBF24 dark)
- `--danger` — Danger/Error (#FB7185)

### Destructive
- `--destructive` / `--destructive-foreground` — Destructive actions

### Sidebar
- `--sidebar`, `--sidebar-foreground`
- `--sidebar-primary`, `--sidebar-primary-foreground`
- `--sidebar-accent`, `--sidebar-accent-foreground`
- `--sidebar-border`, `--sidebar-ring`

---

## 2. Typography

### Font Families
- `--font-body` — DM Sans (body text)
- `--font-heading` — Nunito (headings)
- `--font-mono` — Fira Code (code/monospace)
- `--font-sans` — Inter (system default, fallback)

**Note:** Fonts are loaded via Next.js font system (see `ui/app/layout.tsx`).

---

## 3. Border Radii

### Scale
- `--r-lg` — 28px (large components)
- `--r-card` — 24px (cards)
- `--r-md` — 18px (medium components)
- `--r-sm` — 14px (small components)
- `--r-button` — 20px (buttons)

### Tailwind Mapping
- `rounded-lg`, `rounded-card`, `rounded-md`, `rounded-sm`, `rounded-button`

---

## 4. Clay Shadows

### 4-Layer Shadow System

#### Card Shadow (`--shadow-card`)
Default shadow for cards and surfaces.

**Light Mode:**
```css
16px 16px 32px rgba(160, 150, 180, 0.22),
-10px -10px 24px rgba(255, 255, 255, 0.92),
inset 6px 6px 12px rgba(124, 58, 237, 0.04),
inset -6px -6px 12px rgba(255, 255, 255, 1)
```

**Dark Mode:**
```css
0 0 0 1px rgba(255, 255, 255, 0.07),
0 14px 40px rgba(0, 0, 0, 0.55),
0 0 80px rgba(124, 58, 237, 0.08)
```

#### Float Shadow (`--shadow-float`)
Elevated shadow for hover states.

**Light Mode:**
```css
18px 18px 44px rgba(160, 150, 180, 0.26),
-12px -12px 28px rgba(255, 255, 255, 0.96),
inset 6px 6px 12px rgba(124, 58, 237, 0.05),
inset -6px -6px 12px rgba(255, 255, 255, 1)
```

**Dark Mode:**
```css
0 0 0 1px rgba(255, 255, 255, 0.09),
0 22px 70px rgba(0, 0, 0, 0.6),
0 0 120px rgba(124, 58, 237, 0.12)
```

#### Press Shadow (`--shadow-press`)
Inset shadow for pressed/active states.

**Light Mode:**
```css
inset 10px 10px 20px #d9d4e3,
inset -10px -10px 20px #ffffff
```

**Dark Mode:**
```css
inset 10px 10px 22px rgba(0, 0, 0, 0.65),
inset -10px -10px 22px rgba(255, 255, 255, 0.03)
```

#### Button Shadow (`--shadow-button`)
Special shadow for primary buttons.

**Light Mode:**
```css
12px 12px 24px rgba(124, 58, 237, 0.28),
-8px -8px 16px rgba(255, 255, 255, 0.42),
inset 4px 4px 8px rgba(255, 255, 255, 0.40),
inset -4px -4px 8px rgba(0, 0, 0, 0.10)
```

**Dark Mode:**
```css
0 0 0 1px rgba(124, 58, 237, 0.40),
0 10px 26px rgba(124, 58, 237, 0.22),
inset 0 1px 0 rgba(255, 255, 255, 0.14)
```

### Tailwind Mapping
- `shadow-card`, `shadow-float`, `shadow-press`, `shadow-button`

---

## 5. Motion & Timing

### Durations
- `--dur-1` — 120ms (fast interactions)
- `--dur-2` — 200ms (default interactions)
- `--dur-3` — 320ms (slow/complex interactions)

### Easing
- `--ease-out` — `cubic-bezier(0.16, 1, 0.3, 1)` (smooth, natural)

### Transitions
- `--transition-base` — 200ms cubic-bezier(0.4, 0, 0.2, 1)
- `--transition-fast` — 150ms cubic-bezier(0.4, 0, 0.2, 1)
- `--transition-slow` — 300ms cubic-bezier(0.4, 0, 0.2, 1)

### Tailwind Mapping
- `transition-base`, `transition-fast`, `transition-slow`
- `duration-120`, `duration-200`, `duration-320`
- `ease-out`

---

## 6. Spacing

### Semantic Spacing
- `--page-px` — 24px (horizontal page padding)
- `--page-py` — 24px (vertical page padding)
- `--section-gap` — 16px (gap between sections)

### Tailwind Mapping
- `px-page-px`, `py-page-py`, `gap-section-gap`

---

## 7. Accessibility

### Reduced Motion
All animations respect `prefers-reduced-motion: reduce`:
```css
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

## 8. Theme Switching

### Implementation
- **Provider:** `next-themes` (`ui/components/providers/theme-provider.tsx`)
- **Attribute:** `class` (adds `.dark` to `<html>`)
- **Default Theme:** `dark`
- **System Detection:** Disabled (`enableSystem: false`)

### Palettes
- **Dark Mode:** "Midnight Clay" (deep, cinematic, purple accent glow)
- **Light Mode:** "Candy Clay" (high-fidelity, enterprise-friendly, soft purple)

---

## 9. Usage Examples

### Colors
```tsx
// ✅ GOOD
<div className="bg-background text-foreground">
<Card className="shadow-card">
<Button className="bg-accent hover:bg-accent-hover">

// ❌ BAD
<div className="bg-purple-500 text-white">
```

### Shadows
```tsx
// ✅ GOOD
<Card className="shadow-card hover:shadow-float">

// ❌ BAD
<Card style={{ boxShadow: '0 4px 6px rgba(0,0,0,0.1)' }}>
```

### Borders
```tsx
// ✅ GOOD
<div className="rounded-card border border-border">

// ❌ BAD
<div className="rounded-md border border-gray-200">
```

---

## 10. How to Update Tokens

1. **Modify CSS Variables:** Edit `ui/app/globals.css` (`:root` for light, `.dark` for dark)
2. **Update TypeScript Contract:** If adding new tokens, update `ui/lib/design/tokens.ts`
3. **Update Tailwind Config:** Add new mappings to `ui/tailwind.config.ts` if needed
4. **Update This Doc:** Document new tokens here
5. **Test Both Themes:** Verify light and dark modes look correct

---

## 11. Files Reference

- **CSS Variables:** `ui/app/globals.css`
- **TypeScript Contract:** `ui/lib/design/tokens.ts`
- **Tailwind Config:** `ui/tailwind.config.ts`
- **Theme Provider:** `ui/components/providers/theme-provider.tsx`
- **Strategy Doc:** `docs/ui-unification/UI_UNIFICATION_STRATEGY.md`

---

**Phase 1 Complete:** ✅ Foundation (Theming + Tokens + Global UX)
**Phase 2 Ola 1 Complete:** ✅ DS Components (Button/Card/Input/Badge/PageShell/PageHeader/Section/Skeleton/Toast)

---

## 12. Design System Components (Phase 2 — Ola 1)

All DS components live in `ui/components/ds/` and use semantic tokens exclusively.

### Import

```tsx
import { Button, Card, Input, PageShell, PageHeader, Section } from '@/components/ds'
```

### Core Components

#### Button
**Location:** `ui/components/ds/core/button.tsx`

**Variants:** `primary`, `secondary`, `ghost`, `danger`, `outline`
**Sizes:** `sm`, `md`, `lg`
**Props:** `loading`, `asChild`, `leftIcon`, `rightIcon`

```tsx
<Button variant="primary" size="md">Click me</Button>
<Button variant="danger" loading>Deleting...</Button>
<Button variant="outline" leftIcon={<Icon />}>With icon</Button>
```

#### Card
**Location:** `ui/components/ds/core/card.tsx`

**Variants:** `default`, `glass`, `gradient`
**Subcomponents:** `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter`

```tsx
<Card hoverable>
  <CardHeader>
    <CardTitle>Title</CardTitle>
    <CardDescription>Description</CardDescription>
  </CardHeader>
  <CardContent>Content</CardContent>
  <CardFooter><Button>Action</Button></CardFooter>
</Card>
```

#### Input
**Location:** `ui/components/ds/core/input.tsx`

**Props:** `error`, `disabled`, standard HTML input props

```tsx
<Input placeholder="Enter email" />
<Input error placeholder="Invalid email" />
```

#### Badge
**Location:** `ui/components/ds/core/badge.tsx`

**Variants:** `default`, `success`, `warning`, `danger`, `info`, `outline`

```tsx
<Badge variant="success">Active</Badge>
<Badge variant="warning">Pending</Badge>
```

### Layout Components

#### PageShell
**Location:** `ui/components/ds/layout/page-shell.tsx`

Standard page wrapper with consistent padding.

```tsx
<PageShell>
  <PageHeader title="Dashboard" />
  <Section><Card>Content</Card></Section>
</PageShell>
```

#### PageHeader
**Location:** `ui/components/ds/layout/page-header.tsx`

**Props:** `title`, `description`, `actions`

```tsx
<PageHeader
  title="Cluster Management"
  description="Manage nodes and configuration"
  actions={<><Button variant="secondary">Refresh</Button><Button>Add</Button></>}
/>
```

#### Section
**Location:** `ui/components/ds/layout/section.tsx`

**Props:** `title`, `description`

```tsx
<Section title="Settings" description="Configure your account">
  <Card>Content</Card>
</Section>
```

### Feedback Components

#### Skeleton
**Location:** `ui/components/ds/feedback/skeleton.tsx`

```tsx
<Skeleton className="h-12 w-full" />
<Skeleton className="h-8 w-3/4" />
```

#### Toast
**Location:** `ui/components/ds/feedback/toast.tsx` + `toaster.tsx`

**Variants:** `default`, `destructive`, `success`, `info`, `warning`

```tsx
// In layout.tsx
import { Toaster } from '@/components/ds'
<Toaster />

// Usage
import { useToast } from '@/hooks/use-toast'
const { toast } = useToast()

toast({
  title: "Success",
  description: "Changes saved",
  variant: "success"
})
```

### Utilities

#### cn()
**Location:** `ui/components/ds/utils/cn.ts`

Merge Tailwind classes with proper precedence.

```tsx
import { cn } from '@/components/ds/utils/cn'

<div className={cn('base-class', isActive && 'active-class', className)} />
```

---

## 13. Next Phase: Page Migrations

With DS Ola 1 complete, we can now start migrating admin pages following the strict cycle:

1. **Audit** → Document current state
2. **Dark Iteration** → Implement with DS components
3. **Light Iteration** → Verify parity
4. **Done** → Update PROGRESS.md + screenshots

See `UI_UNIFICATION_STRATEGY.md` for detailed migration process.
