import * as React from "react"
import { cn } from "../utils/cn"

export interface EmptyStateProps extends React.HTMLAttributes<HTMLDivElement> {
  /**
   * Icon to display (Lucide icon or custom SVG)
   */
  icon?: React.ReactNode
  /**
   * Title text
   */
  title: string
  /**
   * Description text
   */
  description?: string
  /**
   * Optional CTA action (usually a Button)
   */
  action?: React.ReactNode
}

/**
 * EmptyState - Display empty states with icon, message, and optional CTA
 *
 * @example
 * ```tsx
 * <EmptyState
 *   icon={<Inbox className="w-12 h-12" />}
 *   title="No tenants found"
 *   description="Get started by creating your first tenant"
 *   action={
 *     <Button onClick={handleCreate}>
 *       <Plus className="w-4 h-4 mr-2" />
 *       Create Tenant
 *     </Button>
 *   }
 * />
 * ```
 */
const EmptyState = React.forwardRef<HTMLDivElement, EmptyStateProps>(
  ({ className, icon, title, description, action, ...props }, ref) => {
    return (
      <div
        ref={ref}
        className={cn(
          "flex flex-col items-center justify-center text-center py-12 px-4",
          className
        )}
        {...props}
      >
        {icon && (
          <div className="mb-4 text-muted opacity-50" aria-hidden="true">
            {icon}
          </div>
        )}
        <h3 className="text-lg font-semibold text-text mb-2">{title}</h3>
        {description && (
          <p className="text-sm text-muted max-w-md mb-6">{description}</p>
        )}
        {action && <div>{action}</div>}
      </div>
    )
  }
)
EmptyState.displayName = "EmptyState"

export { EmptyState }
