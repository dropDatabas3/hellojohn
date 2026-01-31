# 🔄 FUTURE MIGRATION WORKFLOW

**Propósito**: Workflow estandarizado para migrar páginas restantes con Clay Design System.

**Aplicable a**: Todas las páginas después de `/admin/users` (clients, keys, cluster, etc.)

**Fecha**: 2026-01-31

---

## 📋 WORKFLOW OVERVIEW

### Fase 1: Audit (1-2 horas)
- Analizar página existente
- Documentar funcionalidades
- Identificar componentes custom
- Crear checklist de features

### Fase 2: Design (30-45 mins)
- Aplicar clay patterns
- Identificar componentes DS a usar
- Plan de micro-interactions
- Sketch layout changes

### Fase 3: Implementation (3-6 horas)
- Migrar a clay components
- Aplicar semantic tokens
- Implementar micro-interactions
- Test funcionalidad

### Fase 4: QA (30-45 mins)
- Ejecutar CLAY_DESIGN_CHECKLIST.md
- Visual QA
- Functionality QA
- Performance QA

### Fase 5: Documentation (15-20 mins)
- Actualizar pages/{page}.md
- Actualizar PROGRESS.md
- Screenshot before/after
- Commit

---

## 🎯 FASE 1: AUDIT

### Paso 1.1: Crear Page Document

**Ubicación**: `docs/ui-unification/pages/{page}.md`

**Template**:
```markdown
# {Page Name} - Migration Audit

**Route**: `/admin/{page}`
**Priority**: {1-4}
**Complexity**: {SIMPLE|MEDIUM|COMPLEX}
**Status**: 🔍 AUDIT

---

## 1. Current State

### Route Parameters
- [ ] List params (search, filter, sort, pagination)
- [ ] Detail params (id)

### Data Sources
- Endpoints:
  - [ ] GET /v2/admin/{resource}
  - [ ] POST /v2/admin/{resource}
  - [ ] PATCH /v2/admin/{resource}/:id
  - [ ] DELETE /v2/admin/{resource}/:id

### TanStack Query Keys
- [ ] List: `['{resource}', 'list', params]`
- [ ] Detail: `['{resource}', 'detail', id]`

---

## 2. Features Inventory

### Core Features
- [ ] Feature 1: Description
- [ ] Feature 2: Description
- [ ] ...

### Actions
- [ ] Create {resource}
- [ ] Edit {resource}
- [ ] Delete {resource}
- [ ] Bulk operations (if applicable)

### Filters & Search
- [ ] Search by: ...
- [ ] Filter by: ...
- [ ] Sort by: ...

---

## 3. Components Breakdown

### Existing Components
- [ ] Component 1 (from where)
- [ ] Component 2 (from where)

### Missing Components (to create)
- [ ] Component 1 (describe)
- [ ] Component 2 (describe)

### DS Components to Use
- [ ] Button
- [ ] Card
- [ ] Input
- [ ] Badge
- [ ] ...

---

## 4. State Management

### Local State
- [ ] State 1: useState/useReducer
- [ ] State 2: ...

### Server State
- [ ] Query 1: useQuery
- [ ] Mutation 1: useMutation

### URL State
- [ ] Param 1: useSearchParams
- [ ] Param 2: ...

---

## 5. Validation Rules

### Form Validation
- Field 1: rules
- Field 2: rules

---

## 6. Empty States

- [ ] No results (search)
- [ ] No data yet (first time)
- [ ] Error state
- [ ] Loading state

---

## 7. Permissions & Access

- [ ] Required scopes
- [ ] Role restrictions (if RBAC)

---

## 8. Migration Notes

### Risks
- Risk 1: description
- Risk 2: description

### Dependencies
- Depends on: component X, API Y

---

**Audit Date**: YYYY-MM-DD
**Audited By**: Claude/Human
```

### Paso 1.2: Fill Audit Document

**Acción**: Leer código actual de la página y completar template

**Comandos útiles**:
```bash
# Leer página actual
cat ui/app/\(admin\)/admin/{page}/page.tsx

# Buscar endpoints usados
rg "\/v2\/admin\/{resource}" ui/app/\(admin\)/admin/{page}/

# Buscar query keys
rg "useQuery|useMutation" ui/app/\(admin\)/admin/{page}/
```

### Paso 1.3: Screenshot Before

**Ubicación**: `docs/ui-unification/screenshots/{page}/before-migration.png`

**Acción**: Navegar a página y tomar screenshot completo (scroll si necesario)

---

## 🎨 FASE 2: DESIGN

### Paso 2.1: Identify Clay Patterns

**Checklist**:
- [ ] **Background**: Usar BackgroundBlobs?
- [ ] **Layout**: PageShell + PageHeader
- [ ] **Cards**: Interactive variant donde aplicable
- [ ] **Buttons**: Primary actions con gradient, secondary con outline
- [ ] **Badges**: Status indicators con semantic colors
- [ ] **Forms**: Clay input style con recessed effect
- [ ] **Tables/Lists**: Hover lift en rows
- [ ] **Modals**: Clay modal shadows
- [ ] **Empty States**: Clay aesthetic

### Paso 2.2: Map Components

**Table**:

| Current Component | Clay DS Component | Notes |
|-------------------|-------------------|-------|
| Custom button | `<Button>` | Use variant="default" |
| Card div | `<Card interactive>` | Add hover lift |
| Input | `<Input>` | Clay style applied |
| ... | ... | ... |

### Paso 2.3: Plan Micro-interactions

**Per component type**:

**Buttons**:
- Hover: `-translate-y-0.5 shadow-clay-card`
- Active: `translate-y-0 shadow-clay-button`

**Cards**:
- Hover: `-translate-y-1 shadow-clay-float`
- Active: `translate-y-0 shadow-clay-card`

**Inputs**:
- Focus: `ring-2 ring-accent shadow-clay-button`

---

## 🔧 FASE 3: IMPLEMENTATION

### Paso 3.1: Setup Imports

**Template**:
```typescript
// Design System Components
import {
  Button,
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  Input,
  Badge,
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
  // ... otros
} from "@/components/ds"

// Icons (Lucide)
import { IconName, ... } from "lucide-react"

// Utils
import { cn } from "@/components/ds/utils/cn"

// API
import { useQuery, useMutation } from "@tanstack/react-query"
```

### Paso 3.2: Apply PageShell

**Template**:
```typescript
export default function PageName() {
  return (
    <PageShell>
      <BackgroundBlobs />

      <PageHeader
        title="Page Title"
        description="Page description"
      >
        <Button variant="default" onClick={handleAction}>
          <PlusIcon className="h-4 w-4 mr-2" />
          Primary Action
        </Button>
      </PageHeader>

      {/* Main content */}
      <div className="space-y-6">
        {/* Content aquí */}
      </div>
    </PageShell>
  )
}
```

### Paso 3.3: Migrate Components

**Reglas**:
1. **NO hardcodear colors**: Usar tokens semánticos
2. **Aplicar clay shadows**: `shadow-clay-*`
3. **Micro-interactions**: hover lift, active press
4. **Transitions**: `transition-all duration-200`
5. **Focus states**: `focus-visible:ring-2`

**Example - Stats Card**:
```typescript
<Card interactive className="p-6">
  <div className="flex items-center justify-between">
    <div>
      <p className="text-sm text-muted-foreground">Metric Name</p>
      <h3 className="text-3xl font-display font-bold text-foreground">
        {value}
      </h3>
    </div>
    <div className="rounded-full bg-accent/10 p-3">
      <MetricIcon className="h-6 w-6 text-accent" />
    </div>
  </div>
</Card>
```

**Example - List Item**:
```typescript
<Card
  interactive
  className="p-4 cursor-pointer hover:-translate-y-1 hover:shadow-clay-float transition-all duration-200"
  onClick={() => handleClick(item.id)}
>
  <div className="flex items-center justify-between">
    <div className="flex items-center gap-3">
      <div className="h-10 w-10 rounded-full bg-accent/10 flex items-center justify-center">
        <ItemIcon className="h-5 w-5 text-accent" />
      </div>
      <div>
        <h4 className="font-semibold text-foreground">{item.name}</h4>
        <p className="text-sm text-muted-foreground">{item.description}</p>
      </div>
    </div>

    <Badge variant={getVariant(item.status)}>
      {item.status}
    </Badge>
  </div>
</Card>
```

### Paso 3.4: Implement Forms

**Template**:
```typescript
<form onSubmit={handleSubmit} className="space-y-4">
  <div className="space-y-2">
    <Label htmlFor="field">Field Label</Label>
    <Input
      id="field"
      type="text"
      value={value}
      onChange={(e) => setValue(e.target.value)}
      placeholder="Placeholder"
      className={cn(error && "border-destructive")}
    />
    {error && (
      <p className="text-xs text-destructive">{error}</p>
    )}
  </div>

  <div className="flex gap-2 justify-end">
    <Button type="button" variant="outline" onClick={onCancel}>
      Cancel
    </Button>
    <Button type="submit" variant="default" disabled={loading}>
      {loading ? "Saving..." : "Save"}
    </Button>
  </div>
</form>
```

### Paso 3.5: Implement Empty States

**Template**:
```typescript
{items.length === 0 && (
  <Card className="p-12">
    <div className="flex flex-col items-center text-center space-y-4">
      <div className="rounded-full bg-muted p-4">
        <EmptyIcon className="h-8 w-8 text-muted-foreground" />
      </div>
      <div className="space-y-2">
        <h3 className="text-lg font-semibold text-foreground">
          No {resources} found
        </h3>
        <p className="text-sm text-muted-foreground max-w-sm">
          {description}
        </p>
      </div>
      <Button variant="default" onClick={handleCreate}>
        <PlusIcon className="h-4 w-4 mr-2" />
        Create {Resource}
      </Button>
    </div>
  </Card>
)}
```

### Paso 3.6: Implement Loading States

**Template - Skeleton**:
```typescript
{isLoading && (
  <div className="space-y-4">
    {[...Array(5)].map((_, i) => (
      <Card key={i} className="p-4">
        <div className="flex items-center gap-3">
          <Skeleton className="h-10 w-10 rounded-full" />
          <div className="space-y-2 flex-1">
            <Skeleton className="h-4 w-1/3" />
            <Skeleton className="h-3 w-1/2" />
          </div>
        </div>
      </Card>
    ))}
  </div>
)}
```

---

## ✅ FASE 4: QA

### Paso 4.1: Execute Checklist

**Referencia**: `CLAY_DESIGN_CHECKLIST.md`

**Completar todas las secciones**:
- [ ] Visual QA
- [ ] Interaction QA
- [ ] Functionality QA
- [ ] Accessibility QA
- [ ] Performance QA
- [ ] Responsive QA
- [ ] Clay-specific QA

### Paso 4.2: Verify No Hardcoded Colors

```bash
rg "#[0-9a-fA-F]{3,6}" ui/app/\(admin\)/admin/{page}/
# Expected: 0 results (or only in comments)
```

### Paso 4.3: Verify Build

```bash
cd ui
npm run build
npm run typecheck
```

**Expected**: Both succeed with 0 errors

### Paso 4.4: Screenshot After

**Ubicación**: `docs/ui-unification/screenshots/{page}/after-migration.png`

**Comparar con**: `before-migration.png`

---

## 📝 FASE 5: DOCUMENTATION

### Paso 5.1: Update Page Document

**Archivo**: `docs/ui-unification/pages/{page}.md`

**Agregar sección al final**:
```markdown
## X. Clay Migration Implementation

**Fecha**: YYYY-MM-DD

### Components Applied
- BackgroundBlobs
- Clay Button (gradient, lift hover)
- Clay Card (interactive shadows)
- Clay Input (recessed style)
- Clay Badge (semantic variants)
- ... otros

### Functionality Preserved
✅ All original features working:
- Feature 1
- Feature 2
- ...

### Design System
Tokens used:
- Colors: accent-*, gray-*, semantic tokens
- Shadows: shadow-clay-*
- Animations: animate-*

### QA Results
- Visual QA: ✅ Passed
- Interaction QA: ✅ Passed
- Functionality QA: ✅ Passed
- Accessibility QA: ✅ Passed
- Performance QA: ✅ Passed

See: CLAY_DESIGN_CHECKLIST.md
```

### Paso 5.2: Update PROGRESS.md

**Archivo**: `docs/ui-unification/PROGRESS.md`

**Actualizar fila**:
```markdown
| **/admin/{page}** | ✅ | ✅ | ✅ DONE | {commit-hash} | YYYY-MM-DD | Priority X, Clay migration complete |
```

### Paso 5.3: Commit

**Template**:
```bash
git add ui/app/\(admin\)/admin/{page}/
git add docs/ui-unification/pages/{page}.md
git add docs/ui-unification/PROGRESS.md
git add docs/ui-unification/screenshots/{page}/
git commit -m "feat({page}): migrate to clay design system

Complete migration with high-fidelity claymorphism:
- Apply BackgroundBlobs for ambient depth
- Use clay components (Button, Card, Input, Badge)
- Implement micro-interactions (hover lift, active press)
- Apply semantic tokens (zero hardcoded colors)

All functionality preserved:
- {feature 1}
- {feature 2}
- ...

QA: All checks passed
See: docs/ui-unification/pages/{page}.md"
```

---

## 🔁 ITERATIVE PROCESS

### For Each New Page

1. **User says**: "proceed with /admin/{page} migration"

2. **Claude**:
   - FASE 1: Audit (create page doc, screenshot before)
   - FASE 2: Design (identify patterns, plan components)
   - FASE 3: Implementation (migrate to clay)
   - FASE 4: QA (execute checklist)
   - FASE 5: Documentation (update docs, commit)
   - Notify: "✅ /admin/{page} migration complete"

3. **User reviews** and approves

4. **Repeat** for next page

---

## 📊 MIGRATION ORDER

### Priority 1 (Core Features)
- [ ] /admin/users ← DONE (ejemplo de referencia)
- [ ] /admin/clients
- [ ] /admin/tenants

### Priority 2 (Configuration)
- [ ] /admin/keys
- [ ] /admin/cluster
- [ ] /admin/settings
- [ ] /admin/scopes

### Priority 3 (Advanced)
- [ ] /admin/sessions
- [ ] /admin/tokens
- [ ] /admin/rbac
- [ ] /admin/consents

### Priority 4 (Tools)
- [ ] /admin/playground
- [ ] /admin/logs
- [ ] /admin/metrics
- [ ] /admin/database
- [ ] /admin/mailing

---

## 🎯 SUCCESS CRITERIA

Para marcar página como DONE:

- [ ] **Funcionalidad**: 100% features preserved
- [ ] **Visual**: Clay aesthetic applied consistently
- [ ] **Code Quality**: No hardcoded colors, semantic tokens used
- [ ] **QA**: All checklist items passed
- [ ] **Performance**: Build < 2min, page load < 3s
- [ ] **Accessibility**: WCAG AA compliance
- [ ] **Documentation**: Page doc updated, PROGRESS.md updated
- [ ] **Screenshots**: Before/After saved
- [ ] **Commit**: Descriptive message with all changes

---

## 🚀 QUICK REFERENCE

### Common Commands

```bash
# Read current page
cat ui/app/\(admin\)/admin/{page}/page.tsx

# Find hardcoded colors
rg "#[0-9a-fA-F]{3,6}" ui/app/\(admin\)/admin/{page}/

# Build & typecheck
cd ui && npm run build && npm run typecheck

# Run dev server
npm run dev

# Git workflow
git add .
git commit -m "feat({page}): ..."
git push
```

### Component Import Template

```typescript
import {
  Button,
  Card, CardHeader, CardTitle, CardContent,
  Input,
  Label,
  Badge,
  Select, SelectTrigger, SelectValue, SelectContent, SelectItem,
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
  Tabs, TabsList, TabsTrigger, TabsContent,
  BackgroundBlobs,
  PageShell,
  PageHeader,
} from "@/components/ds"
```

### Clay Pattern Quick Reference

```typescript
// Button with lift
<Button
  variant="default"
  className="hover:-translate-y-0.5 hover:shadow-clay-card active:translate-y-0"
>
  Action
</Button>

// Interactive card
<Card
  interactive
  className="hover:-translate-y-1 hover:shadow-clay-float"
>
  Content
</Card>

// Clay input
<Input
  className="shadow-inner focus-visible:ring-2 focus-visible:ring-accent focus-visible:shadow-clay-button"
/>

// Badge with semantic variant
<Badge variant={status === 'active' ? 'default' : 'secondary'}>
  {status}
</Badge>
```

---

**VERSION**: 1.0
**FECHA**: 2026-01-31
**STATUS**: Active Workflow
