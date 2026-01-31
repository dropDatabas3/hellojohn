# UI Unification Workplan

**Current Phase:** 3 (Page Migrations) — Clay Design System Active ✅
**Today's Task:** `/admin/users` Clay redesign ✅ COMPLETE

## Next Steps
1. ✅ **DONE**: Clay Design System Foundation
   - DESIGN_SYSTEM_SPEC.md created
   - Tailwind config with clay shadows & animations
   - Clay color tokens (accent-1 through accent-4, gray-1 through gray-9)
   - DM Sans & Nunito fonts via next/font
   - Commits: 6ce2214, d4c48bd

2. ✅ **DONE**: Professional Form Components
   - PhoneInput with libphonenumber-js validation
   - CountrySelect with flag emojis
   - Commit: 4fd4345

3. ✅ **DONE**: Core Components Clay Refinement
   - Button (clay gradient, backdrop-blur outline)
   - Card (interactive variant with lift hover)
   - Input (recessed style, backdrop-blur)
   - Badge (semantic variants, 15% opacity)
   - Commit: 758ad92

4. ✅ **DONE**: `/admin/users` Clay Migration
   - BackgroundBlobs component
   - Full re-migration with clay design system
   - 100% functionality preserved (13 critical features)
   - Zero hardcoded colors
   - Commit: 94343e4

5. **NEXT**: Apply Clay to Priority 1 pages
   - `/admin/clients` (Priority 1)
   - `/admin/tenants` (upgrade to clay - currently uses old tokens)

## Blockers
- None — Clay Design System ready for future migrations

## Design Decisions Made
- **DataTable pattern**: Used list-style layout with dividers instead of traditional table (better responsive, cleaner DS styling)
- **SearchInput pattern**: Used Input with manual icon positioning (no separate component needed yet)

## Completed Pages
- ✅ `/admin` (Dashboard) — Full DS migration with InlineAlert + EmptyState
- ✅ `/admin/tenants` — List pattern with dividers, Ola 3 Dialog + Dropdown
- ✅ `/admin/users` (Clay) — High-fidelity claymorphism, PhoneInput + CountrySelect recovered, 13 critical features preserved

## Completed DS Components (Olas)
- ✅ **Ola 1** (Core): Button, Card, Input, Badge, PageShell, PageHeader, Section, Skeleton
- ✅ **Ola 2** (Feedback): InlineAlert, EmptyState, Toast, Toaster
- ✅ **Ola 3** (Overlays): Dialog, DropdownMenu
- ✅ **Ola 4** (Forms & Navigation): Label, Checkbox, Switch, Textarea, Select, Tabs

## Pages Ready for Clay Migration
- ⏳ `/admin/clients` (Priority 1)
- ⏳ `/admin/keys` (Priority 2)
- ⏳ `/admin/cluster` (Priority 2)
- ⏳ `/admin/settings` (Priority 2)
- ⏳ `/admin/scopes` (Priority 2)

## Phase 1 Completado
- ✅ ThemeProvider canónico (`ui/components/providers/theme-provider.tsx`)
- ✅ Tokens semánticos en CSS vars (`ui/app/globals.css`)
- ✅ Tailwind config mapeado (`ui/tailwind.config.ts`)
- ✅ TypeScript contract (`ui/lib/design/tokens.ts`)
- ✅ Reduced motion support
- ✅ Dark/Light palettes separadas
- ✅ Build verde (Next.js 16)
- ✅ Documentación actualizada (DESIGN_TOKENS.md)

## Phase 2 Ola 1 Completado & Hardened
- ✅ `cn()` utility (`ui/components/ds/utils/cn.ts`)
- ✅ **Core:** Button, Card, Input, Badge (semantic tokens, CVA variants, a11y)
- ✅ **Layout:** PageShell, PageHeader, Section (consistent spacing)
- ✅ **Feedback:** Skeleton, Toast, Toaster (DS-styled, prefers-reduced-motion)
- ✅ Barrel exports (`ui/components/ds/index.ts`)
- ✅ Build verde (28 páginas compiladas)
- ✅ TypeScript: DS components 0 errors (`npm run typecheck`)
- ✅ Hook `use-toast` desacoplado de UI components
- ✅ Layout usa DS Toaster (no UI viejo)
- ✅ Semantic colors en HSL (opacity modifiers funcionan)
- ✅ Focus rings con `ring-offset-background` (consistencia dark/light)
- ✅ Zero hardcoded colors verificado

**Hardening Doc:** `docs/ui-unification/PHASE2_HARDENING.md`

## DECISIONS & PROGRESS
- See [DECISIONS.md](DECISIONS.md)
- See [PROGRESS.md](PROGRESS.md)

---

## Daily Log

### 2026-01-30
- [ ] Finish Phase 0 setup
- [ ] Verify scaffolding
- [ ] Commit

**Done Today:**
- Created control plane docs.

**Notes:**
- Initial setup.
