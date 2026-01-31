/**
 * Checkbox Component — Design System (Ola 4 - Forms)
 *
 * Checkbox for boolean selections with indeterminate state support.
 * Based on Radix UI Checkbox with DS semantic tokens.
 */

import * as React from "react"
import * as CheckboxPrimitive from "@radix-ui/react-checkbox"
import { Check, Minus } from "lucide-react"
import { cn } from "../utils/cn"

export interface CheckboxProps extends React.ComponentPropsWithoutRef<typeof CheckboxPrimitive.Root> {
  /**
   * Indeterminate state (for select-all scenarios)
   */
  indeterminate?: boolean
}

/**
 * Checkbox component for form selections and bulk actions.
 *
 * @example
 * ```tsx
 * <Checkbox
 *   checked={isChecked}
 *   onCheckedChange={setIsChecked}
 *   aria-label="Select item"
 * />
 * ```
 */
const Checkbox = React.forwardRef<
  React.ElementRef<typeof CheckboxPrimitive.Root>,
  CheckboxProps
>(({ className, indeterminate, ...props }, ref) => (
  <CheckboxPrimitive.Root
    ref={ref}
    className={cn(
      "peer h-4 w-4 shrink-0 rounded-sm border border-input bg-background",
      "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background",
      "disabled:cursor-not-allowed disabled:opacity-50",
      "data-[state=checked]:bg-accent data-[state=checked]:text-accent-foreground data-[state=checked]:border-accent",
      "data-[state=indeterminate]:bg-accent data-[state=indeterminate]:text-accent-foreground data-[state=indeterminate]:border-accent",
      "transition-all duration-200",
      className
    )}
    {...props}
    checked={indeterminate ? "indeterminate" : props.checked}
  >
    <CheckboxPrimitive.Indicator
      className={cn("flex items-center justify-center text-current")}
    >
      {indeterminate ? (
        <Minus className="h-3 w-3" aria-hidden="true" />
      ) : (
        <Check className="h-3 w-3" aria-hidden="true" />
      )}
    </CheckboxPrimitive.Indicator>
  </CheckboxPrimitive.Root>
))
Checkbox.displayName = "Checkbox"

export { Checkbox }
