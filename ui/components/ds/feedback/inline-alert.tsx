import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { AlertCircle, CheckCircle2, Info, AlertTriangle } from "lucide-react"
import { cn } from "../utils/cn"

const inlineAlertVariants = cva(
  "relative w-full rounded-card border p-4 transition-colors duration-200",
  {
    variants: {
      variant: {
        default: "bg-card border-border text-text",
        info: "bg-info/10 border-info/30 text-text",
        success: "bg-success/10 border-success/30 text-text",
        warning: "bg-warning/10 border-warning/30 text-text",
        destructive: "bg-danger/10 border-danger/30 text-text",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

const iconMap = {
  default: Info,
  info: Info,
  success: CheckCircle2,
  warning: AlertTriangle,
  destructive: AlertCircle,
}

const iconColorMap = {
  default: "text-muted",
  info: "text-info",
  success: "text-success",
  warning: "text-warning",
  destructive: "text-danger",
}

export interface InlineAlertProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof inlineAlertVariants> {
  /**
   * Optional title for the alert
   */
  title?: string
  /**
   * Main description/message
   */
  description?: React.ReactNode
  /**
   * Optional action element (e.g., Button for retry)
   */
  action?: React.ReactNode
  /**
   * Whether to show the icon (default: true)
   */
  showIcon?: boolean
}

/**
 * InlineAlert - Display contextual alerts with variants
 *
 * @example
 * ```tsx
 * <InlineAlert variant="warning" title="Warning">
 *   This action cannot be undone.
 * </InlineAlert>
 *
 * <InlineAlert
 *   variant="destructive"
 *   title="Error"
 *   action={<Button size="sm" onClick={retry}>Retry</Button>}
 * >
 *   Failed to load data.
 * </InlineAlert>
 * ```
 */
const InlineAlert = React.forwardRef<HTMLDivElement, InlineAlertProps>(
  ({ className, variant, title, description, action, showIcon = true, children, ...props }, ref) => {
    const Icon = iconMap[variant || "default"]
    const iconColor = iconColorMap[variant || "default"]

    return (
      <div
        ref={ref}
        role="alert"
        className={cn(inlineAlertVariants({ variant }), className)}
        {...props}
      >
        <div className="flex gap-3">
          {showIcon && (
            <Icon className={cn("h-5 w-5 flex-shrink-0", iconColor)} aria-hidden="true" />
          )}
          <div className="flex-1 space-y-1">
            {title && (
              <h5 className="font-medium leading-none tracking-tight">{title}</h5>
            )}
            {(description || children) && (
              <div className="text-sm text-muted leading-relaxed">
                {description || children}
              </div>
            )}
          </div>
          {action && (
            <div className="flex-shrink-0">
              {action}
            </div>
          )}
        </div>
      </div>
    )
  }
)
InlineAlert.displayName = "InlineAlert"

export { InlineAlert, inlineAlertVariants }
