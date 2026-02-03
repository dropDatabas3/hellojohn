import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { AlertCircle, CheckCircle2, Info, AlertTriangle } from "lucide-react"
import { cn } from "../utils/cn"

const inlineAlertVariants = cva(
  [
    "relative w-full rounded-xl border p-4 transition-all duration-300",
    // Premium violet gradient overlay
    "bg-gradient-to-r from-violet-500/10 via-purple-500/5 to-transparent",
    "dark:from-violet-500/15 dark:via-purple-500/8 dark:to-transparent",
    // 3D shadow and depth
    "shadow-[0_2px_4px_rgba(139,92,246,0.06),0_4px_8px_rgba(139,92,246,0.04),0_8px_16px_rgba(0,0,0,0.03)]",
    "dark:shadow-[0_2px_4px_rgba(139,92,246,0.1),0_4px_8px_rgba(139,92,246,0.08),0_8px_16px_rgba(0,0,0,0.15)]",
    // Inner highlight
    "ring-1 ring-inset ring-white/40 dark:ring-white/10",
  ],
  {
    variants: {
      variant: {
        default: "border-violet-200/60 dark:border-violet-500/30",
        info: "border-info/40 dark:border-info/50",
        success: "border-success/40 dark:border-success/50",
        warning: "border-warning/40 dark:border-warning/50",
        danger: "border-danger/40 dark:border-danger/50",
        destructive: "border-danger/40 dark:border-danger/50",
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
  danger: AlertCircle,
  destructive: AlertCircle,
}

const iconColorMap = {
  default: "text-muted",
  info: "text-info",
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
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
              <h5 className="font-medium leading-none tracking-tight text-foreground">{title}</h5>
            )}
            {(description || children) && (
              <div className="text-sm text-foreground/80 leading-relaxed dark:text-foreground/85">
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
