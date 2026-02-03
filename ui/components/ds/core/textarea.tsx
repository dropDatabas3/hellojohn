/**
 * Textarea Component — Design System (Ola 4 - Forms)
 *
 * Multi-line text input with auto-resize option.
 * Uses DS semantic tokens for consistent styling.
 */

import * as React from "react"
import { cn } from "../utils/cn"

export interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  /**
   * Display error state styling
   */
  error?: boolean
}

/**
 * Textarea component for multi-line text input.
 *
 * @example
 * ```tsx
 * <Textarea
 *   placeholder="Enter your message..."
 *   value={message}
 *   onChange={(e) => setMessage(e.target.value)}
 *   rows={4}
 * />
 * ```
 */
const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, error, ...props }, ref) => {
    return (
      <textarea
        ref={ref}
        className={cn(
          "flex min-h-[80px] w-full rounded-md border bg-background px-3 py-2 text-sm text-foreground",
          "placeholder:text-muted-foreground",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background",
          "disabled:cursor-not-allowed disabled:opacity-50",
          "transition-all duration-200",
          error
            ? "border-danger focus-visible:ring-danger"
            : "border-input",
          className
        )}
        {...props}
      />
    )
  }
)
Textarea.displayName = "Textarea"

export { Textarea }
