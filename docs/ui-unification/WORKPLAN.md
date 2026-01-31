# UI Unification Workplan

**Current Phase:** 3 (Page Migrations) — `/admin/users` 🚧 DARK_IN_PROGRESS
**Today's Task:** `/admin/users` dark iteration completed ✅ (1,785 lines, all DS components)

## Next Steps
1. ✅ **DONE**: Implemented Ola 4 (Forms) components - Commit: a722b79
2. ✅ **DONE**: `/admin/users` dark iteration - Full migration to DS (1,785 lines, -19% reduction)
3. **NEXT**: `/admin/users` light iteration (verify contrast, shadows, readability in light mode)
4. After `/admin/users` complete: Move to Priority 2 pages (/admin/keys, /admin/cluster, /admin/settings, /admin/scopes)

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
- 🚧 `/admin/users` (DARK COMPLETE) — Migrated to 1,785 lines (-19%), all DS components, ready for light iteration

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
