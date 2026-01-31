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
