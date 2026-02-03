/**
 * Label Component — Design System (Ola 4 - Forms)
 *
 * Accessible form label component with proper htmlFor association.
 * Based on Radix UI Label with DS semantic tokens.
 */

import * as React from "react"
import * as LabelPrimitive from "@radix-ui/react-label"
import { cn } from "../utils/cn"

export interface LabelProps extends React.ComponentPropsWithoutRef<typeof LabelPrimitive.Root> {
  /**
   * Indicates if the associated field is required
   */
  required?: boolean
}

/**
 * Label component for form fields with accessibility support.
 *
 * @example
 * ```tsx
 * <Label htmlFor="email" required>Email</Label>
 * <Input id="email" type="email" />
 * ```
 */
const Label = React.forwardRef<
  React.ElementRef<typeof LabelPrimitive.Root>,
  LabelProps
>(({ className, required, children, ...props }, ref) => (
  <LabelPrimitive.Root
    ref={ref}
    className={cn(
      "text-sm font-medium leading-none text-foreground",
      "peer-disabled:cursor-not-allowed peer-disabled:opacity-50",
      className
    )}
    {...props}
  >
    {children}
    {required && (
      <span className="text-danger ml-1" aria-label="required">
        *
      </span>
    )}
  </LabelPrimitive.Root>
))
Label.displayName = "Label"

export { Label }
