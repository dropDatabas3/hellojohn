import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../utils/cn'

/**
 * Badge variants using CVA.
 */
const badgeVariants = cva(
  'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold transition-colors',
  {
    variants: {
      variant: {
        default: 'bg-accent/15 text-accent-foreground border border-accent/30',
        secondary: 'bg-muted/80 text-muted-foreground border border-border/50',
        success: 'bg-success/15 text-green-700 dark:text-green-300 border border-success/30',
        warning: 'bg-warning/15 text-yellow-700 dark:text-yellow-300 border border-warning/30',
        destructive: 'bg-destructive/15 text-destructive border border-destructive/30',
        info: 'bg-info/15 text-blue-700 dark:text-blue-300 border border-info/30',
        outline: 'text-foreground border border-border',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

/**
 * Badge component for status indicators, labels, etc.
 *
 * @example
 * ```tsx
 * <Badge>Default</Badge>
 * <Badge variant="success">Active</Badge>
 * <Badge variant="warning">Pending</Badge>
 * <Badge variant="danger">Error</Badge>
 * <Badge variant="info">Info</Badge>
 * ```
 */
function Badge({ className, variant, ...props }: BadgeProps) {
  return (
    <div className={cn(badgeVariants({ variant }), className)} {...props} />
  )
}

export { Badge, badgeVariants }
