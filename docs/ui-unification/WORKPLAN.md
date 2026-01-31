# UI Unification Workplan

**Current Phase:** 3 (Page Migrations) — `/admin/users` 🔍 AUDIT
**Today's Task:** Ola 4 components implemented ✅, ready for `/admin/users` dark iteration

## Next Steps
1. ✅ **DONE**: Implemented Ola 4 (Forms) components:
   - Label, Select, Switch, Checkbox, Textarea (core form components)
   - Tabs (navigation)
   - Commit: a722b79
2. **DECISION NEEDED**: `/admin/users` dark iteration is COMPLEX (2,205 lines, 5+ dialogs, estimated 3-4 days)
   - Option A: Proceed with `/admin/users` (multi-day effort, break into subtasks)
   - Option B: Audit simpler Priority 2 pages first (/admin/keys, /admin/cluster, /admin/settings, /admin/scopes)
   - Note: `/admin/clients` marked as Priority 1 doesn't exist yet in codebase
3. Move to Priority 2 pages after completing available Priority 1 pages

## Blockers
- **Complexity**: `/admin/users` requires breaking into multiple subtasks (3-4 days estimated effort)

## Design Decisions Made
- **DataTable pattern**: Used list-style layout with dividers instead of traditional table (better responsive, cleaner DS styling)
- **SearchInput pattern**: Used Input with manual icon positioning (no separate component needed yet)

## Completed Pages
- ✅ `/admin` (Dashboard) — Full DS migration with InlineAlert + EmptyState
- ✅ `/admin/tenants` — List pattern with dividers, Ola 3 Dialog + Dropdown

## Completed DS Components (Olas)
- ✅ **Ola 1** (Core): Button, Card, Input, Badge, PageShell, PageHeader, Section, Skeleton
- ✅ **Ola 2** (Feedback): InlineAlert, EmptyState, Toast, Toaster
- ✅ **Ola 3** (Overlays): Dialog, DropdownMenu
- ✅ **Ola 4** (Forms & Navigation): Label, Checkbox, Switch, Textarea, Select, Tabs

## Pages in Progress
- 🔍 `/admin/users` (AUDIT) — 2,205 lines, COMPLEX (estimated 3-4 days), all required DS components now available

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
