import * as React from 'react'
import { cn } from '../utils/cn'

export interface PageShellProps extends React.HTMLAttributes<HTMLDivElement> {
  /**
   * If true, applies max-width container constraint.
   * Default: true
   */
  contained?: boolean
}

/**
 * PageShell provides consistent page layout with semantic spacing.
 *
 * @example
 * ```tsx
 * <PageShell>
 *   <PageHeader title="Dashboard" />
 *   <Section>
 *     <Card>Content</Card>
 *   </Section>
 * </PageShell>
 * ```
 */
const PageShell = React.forwardRef<HTMLDivElement, PageShellProps>(
  ({ className, contained = true, ...props }, ref) => {
    return (
      <div
        ref={ref}
        className={cn(
          'w-full px-2 py-4 md:px-4 md:py-5',
          contained && 'mx-auto max-w-7xl',
          className
        )}
        {...props}
      />
    )
  }
)
PageShell.displayName = 'PageShell'

export { PageShell }
