# UI Unification Workplan

**Current Phase:** 3 (Page Migrations) — `/admin/users` 🔍 AUDIT
**Today's Task:** `/admin/users` audit complete, needs Ola 4 components before dark iteration

## Next Steps
1. **BLOCKER**: Implement Ola 4 (Forms) components before `/admin/users` dark iteration:
   - Label, Select, Switch, Checkbox, Textarea (core form components)
   - Tabs (navigation), Tooltip (overlays), Pagination (data)
2. After Ola 4 ready: Start `/admin/users` dark iteration (COMPLEX: 2,205 lines, 5+ dialogs)
3. Continue with remaining Priority 1 page: `/admin/clients`
4. Move to Priority 2 pages after completing Priority 1

## Blockers
- None

## Design Decisions Made
- **DataTable pattern**: Used list-style layout with dividers instead of traditional table (better responsive, cleaner DS styling)
- **SearchInput pattern**: Used Input with manual icon positioning (no separate component needed yet)

## Completed Pages
- ✅ `/admin` (Dashboard) — Full DS migration with InlineAlert + EmptyState
- ✅ `/admin/tenants` — List pattern with dividers, Ola 3 Dialog + Dropdown

## Pages in Progress
- 🔍 `/admin/users` (AUDIT) — 2,205 lines, requires Ola 4 (Forms) components: Label, Select, Switch, Checkbox, Textarea, Tabs, Tooltip, Pagination

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
