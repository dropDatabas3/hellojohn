# UI Unification Workplan

**Current Phase:** 3 (Page Migrations) — `/admin/tenants` dark iteration complete
**Today's Task:** page(tenants) dark iteration

## Next Steps
1. Light iteration for `/admin/tenants` (verify contrast, shadows, readability)
2. Cierre: DoD verification + commit
3. Start next Priority 1 page (/admin/users or /admin/clients)

## Blockers
- None

## Design Decisions Made
- **DataTable pattern**: Used list-style layout with dividers instead of traditional table (better responsive, cleaner DS styling)
- **SearchInput pattern**: Used Input with manual icon positioning (no separate component needed yet)

## Completed Pages
- ✅ `/admin` (Dashboard) — Full DS migration with InlineAlert + EmptyState

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
