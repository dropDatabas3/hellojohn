/**
 * Switch Component — Design System (Ola 4 - Forms)
 *
 * Toggle switch for boolean values.
 * Based on Radix UI Switch with DS semantic tokens.
 */

import * as React from "react"
import * as SwitchPrimitive from "@radix-ui/react-switch"
import { cn } from "../utils/cn"

export interface SwitchProps extends React.ComponentPropsWithoutRef<typeof SwitchPrimitive.Root> {}

/**
 * Switch component for boolean toggles.
 *
 * @example
 * ```tsx
 * <div className="flex items-center gap-2">
 *   <Switch
 *     checked={isEnabled}
 *     onCheckedChange={setIsEnabled}
 *     id="email-verified"
 *   />
 *   <Label htmlFor="email-verified">Email verified</Label>
 * </div>
 * ```
 */
const Switch = React.forwardRef<
  React.ElementRef<typeof SwitchPrimitive.Root>,
  SwitchProps
>(({ className, ...props }, ref) => (
  <SwitchPrimitive.Root
    ref={ref}
    className={cn(
      "peer inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent",
      "transition-colors duration-200",
      "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background",
      "disabled:cursor-not-allowed disabled:opacity-50",
      "data-[state=checked]:bg-accent data-[state=unchecked]:bg-input",
      className
    )}
    {...props}
  >
    <SwitchPrimitive.Thumb
      className={cn(
        "pointer-events-none block h-4 w-4 rounded-full bg-background shadow-button",
        "transition-transform duration-200",
        "data-[state=checked]:translate-x-4 data-[state=unchecked]:translate-x-0"
      )}
    />
  </SwitchPrimitive.Root>
))
Switch.displayName = "Switch"

export { Switch }
