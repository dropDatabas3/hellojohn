import * as React from 'react'
import { cn } from '../utils/cn'

export interface PageHeaderProps extends React.HTMLAttributes<HTMLDivElement> {
  /**
   * Page title (renders as h1).
   */
  title: string
  /**
   * Optional page description.
   */
  description?: string
  /**
   * Optional actions to render on the right side.
   * Typically buttons or other CTAs.
   */
  actions?: React.ReactNode
}

/**
 * PageHeader provides consistent page title, description, and actions layout.
 *
 * @example
 * ```tsx
 * <PageHeader
 *   title="Cluster Management"
 *   description="Manage Raft cluster nodes and configuration"
 *   actions={
 *     <>
 *       <Button variant="secondary">Refresh</Button>
 *       <Button variant="primary">Add Node</Button>
 *     </>
 *   }
 * />
 * ```
 */
const PageHeader = React.forwardRef<HTMLDivElement, PageHeaderProps>(
  ({ className, title, description, actions, ...props }, ref) => {
    return (
      <div
        ref={ref}
        className={cn(
          'flex flex-col gap-4 pb-8 md:flex-row md:items-start md:justify-between',
          className
        )}
        {...props}
      >
        <div className="flex-1 space-y-2">
          <h1 className="text-3xl font-bold tracking-tight text-foreground md:text-4xl">
            {title}
          </h1>
          {description && (
            <p className="text-base text-muted-foreground md:text-lg">
              {description}
            </p>
          )}
        </div>
        {actions && (
          <div className="flex flex-wrap items-center gap-2">{actions}</div>
        )}
      </div>
    )
  }
)
PageHeader.displayName = 'PageHeader'

export { PageHeader }
