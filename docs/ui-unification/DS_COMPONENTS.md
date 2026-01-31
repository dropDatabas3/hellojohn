# Design System Components — Quick Reference

> **Phase 2 Ola 1** — Core, Layout, and Feedback components
> **Location:** `ui/components/ds/`
> **Import:** `import { Button, Card, ... } from '@/components/ds'`

---

## 🎯 Principles

1. **Semantic tokens only** — No hardcoded colors (`bg-purple-500` ❌)
2. **Accessible by default** — Focus visible, keyboard nav, ARIA labels
3. **Reusable & composable** — Generic, not page-specific
4. **Clay interactions** — Subtle lift on hover, press feedback
5. **Theme-aware** — Works in both dark and light modes

---

## 📦 Component Inventory

### Core (`core/`)

| Component | Variants | Key Props | File |
|-----------|----------|-----------|------|
| **Button** | primary, secondary, ghost, danger, outline | `loading`, `asChild`, `leftIcon`, `rightIcon` | `button.tsx` |
| **Card** | default, glass, gradient | `hoverable` | `card.tsx` |
| **Input** | - | `error`, `disabled` | `input.tsx` |
| **Badge** | default, success, warning, danger, info, outline | - | `badge.tsx` |

### Layout (`layout/`)

| Component | Props | Purpose | File |
|-----------|-------|---------|------|
| **PageShell** | `contained` | Page wrapper with spacing | `page-shell.tsx` |
| **PageHeader** | `title`, `description`, `actions` | Consistent page headers | `page-header.tsx` |
| **Section** | `title`, `description` | Vertical rhythm | `section.tsx` |

### Feedback (`feedback/`)

| Component | Variants | Purpose | File |
|-----------|----------|---------|------|
| **Skeleton** | - | Loading placeholders | `skeleton.tsx` |
| **Toast** | default, destructive, success, info, warning | Notifications | `toast.tsx` |
| **Toaster** | - | Toast container | `toaster.tsx` |
| **InlineAlert** | default, info, success, warning, destructive | Contextual alerts | `inline-alert.tsx` |
| **EmptyState** | - | Empty state with CTA | `empty-state.tsx` |

### Overlays (`overlays/`) — Ola 3

| Component | Subcomponents | Purpose | File |
|-----------|---------------|---------|------|
| **Dialog** | Dialog, DialogTrigger, DialogContent, DialogHeader, DialogFooter, DialogTitle, DialogDescription, DialogClose | Modal dialogs | `dialog.tsx` |
| **DropdownMenu** | DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuCheckboxItem, DropdownMenuRadioItem | Context menus | `dropdown-menu.tsx` |

### Utils (`utils/`)

| Utility | Purpose | File |
|---------|---------|------|
| **cn()** | Merge Tailwind classes | `cn.ts` |

---

## 🚀 Usage Examples

### Button

```tsx
import { Button } from '@/components/ds'

// Primary action
<Button variant="primary">Save Changes</Button>

// Loading state
<Button loading disabled>Saving...</Button>

// With icons
<Button leftIcon={<PlusIcon />}>Add User</Button>

// Dangerous action
<Button variant="danger">Delete Account</Button>

// As link (using Radix Slot)
<Button asChild>
  <Link href="/dashboard">Go to Dashboard</Link>
</Button>
```

### Card

```tsx
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '@/components/ds'

<Card hoverable>
  <CardHeader>
    <CardTitle>User Settings</CardTitle>
    <CardDescription>Manage your account preferences</CardDescription>
  </CardHeader>
  <CardContent>
    {/* Form or content */}
  </CardContent>
  <CardFooter>
    <Button>Save</Button>
  </CardFooter>
</Card>
```

### Input

```tsx
import { Input } from '@/components/ds'

// Standard
<Input placeholder="Enter email" />

// With error state
<Input error placeholder="Invalid email" aria-describedby="email-error" />

// Disabled
<Input disabled placeholder="Read-only" />
```

### Badge

```tsx
import { Badge } from '@/components/ds'

<Badge variant="success">Active</Badge>
<Badge variant="warning">Pending</Badge>
<Badge variant="danger">Error</Badge>
<Badge variant="info">Beta</Badge>
```

### Page Layout

```tsx
import { PageShell, PageHeader, Section, Card, Button } from '@/components/ds'

export default function DashboardPage() {
  return (
    <PageShell>
      <PageHeader
        title="Dashboard"
        description="Overview of your system"
        actions={
          <>
            <Button variant="secondary">Refresh</Button>
            <Button>Add New</Button>
          </>
        }
      />

      <Section title="Recent Activity">
        <Card>
          {/* Activity content */}
        </Card>
      </Section>

      <Section title="Statistics">
        <div className="grid gap-4 md:grid-cols-3">
          <Card>Stat 1</Card>
          <Card>Stat 2</Card>
          <Card>Stat 3</Card>
        </div>
      </Section>
    </PageShell>
  )
}
```

### Loading States

```tsx
import { Skeleton } from '@/components/ds'

// While loading
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

### Dialog (Ola 3)

```tsx
import { 
  Dialog, DialogContent, DialogHeader, DialogFooter, 
  DialogTitle, DialogDescription, Button 
} from '@/components/ds'

function DeleteConfirmation({ open, onOpenChange, onConfirm }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete Item</DialogTitle>
          <DialogDescription>
            Are you sure you want to delete this item? This action cannot be undone.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="danger" onClick={onConfirm}>
            Delete
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

### DropdownMenu (Ola 3)

```tsx
import { 
  DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, 
  DropdownMenuItem, DropdownMenuSeparator, Button 
} from '@/components/ds'
import { MoreHorizontal, Settings, Trash2 } from 'lucide-react'

function ActionsMenu({ onEdit, onDelete }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm">
          <MoreHorizontal className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={onEdit}>
          <Settings className="mr-2 h-4 w-4" />
          Edit
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="danger" onClick={onDelete}>
          <Trash2 className="mr-2 h-4 w-4" />
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
```

### Toast Notifications

```tsx
import { useToast } from '@/hooks/use-toast'

function MyComponent() {
  const { toast } = useToast()

  const handleSave = async () => {
    try {
      await saveData()
      toast({
        title: "Success",
        description: "Your changes have been saved",
        variant: "success"
      })
    } catch (error) {
      toast({
        title: "Error",
        description: "Failed to save changes",
        variant: "destructive"
      })
    }
  }

  return <Button onClick={handleSave}>Save</Button>
}
```

---

## 🎨 Styling Patterns

### Using cn() for className merging

```tsx
import { cn } from '@/components/ds/utils/cn'

<Button
  className={cn(
    'w-full',
    isActive && 'bg-accent',
    className  // Allow external override
  )}
>
  Click me
</Button>
```

### Custom variants with CVA

```tsx
import { cva } from 'class-variance-authority'
import { cn } from '@/components/ds/utils/cn'

const myVariants = cva(
  'base-classes',
  {
    variants: {
      size: {
        sm: 'text-sm',
        lg: 'text-lg'
      }
    }
  }
)

<div className={cn(myVariants({ size: 'lg' }), className)} />
```

---

## ⚠️ Important Notes

### Don't Mix Old and New

```tsx
// ❌ BAD - Using old components/ui
import { Button } from '@/components/ui/button'

// ✅ GOOD - Using new DS components
import { Button } from '@/components/ds'
```

### Semantic Tokens Only

```tsx
// ❌ BAD - Hardcoded colors
<div className="bg-purple-500 text-white">

// ✅ GOOD - Semantic tokens
<div className="bg-accent text-accent-foreground">
```

### Accessibility First

```tsx
// ❌ BAD - Missing labels
<Input placeholder="Email" />

// ✅ GOOD - Proper labels
<Label htmlFor="email">Email</Label>
<Input id="email" placeholder="Enter your email" />
```

### Focus Visible

All DS components have focus-visible rings by default:

```tsx
// Already included in Button, Input, etc.
focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2
```

---

## 🔜 Next Olas (Future Components)

### ✅ Ola 2 — Feedback (Complete)
- InlineAlert ✅
- EmptyState ✅

### ✅ Ola 3 — Overlays (Complete)
- Dialog ✅
- DropdownMenu ✅

### Ola 3.5 — Overlays (Pending)
- Tooltip
- Select

### Ola 4 — Utilities
- CopyButton
- CodeBlock
- KeyValueRow
- Separator
- Toolbar

### Ola 5 — Data Display (As Needed)
- DataTable (sortable, server-side ready)
- Pagination

---

## 📚 References

- **Strategy:** `docs/ui-unification/UI_UNIFICATION_STRATEGY.md`
- **Tokens:** `docs/ui-unification/DESIGN_TOKENS.md`
- **Source:** `ui/components/ds/`
- **Imports:** `import { ... } from '@/components/ds'`

---

**Status:** ✅ Phase 2 Ola 3 Complete (Core, Layout, Feedback, Overlays)
**Next:** Page Migrations (Phase 3) — `/admin/tenants` unblocked
