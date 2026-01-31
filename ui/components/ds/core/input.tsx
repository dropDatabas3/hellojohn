import * as React from 'react'
import { cn } from '../utils/cn'

export interface InputProps
  extends React.InputHTMLAttributes<HTMLInputElement> {
  /**
   * If true, applies error styling (red border + ring).
   */
  error?: boolean
}

/**
 * Input component with proper focus and error states.
 *
 * @example
 * ```tsx
 * <Input placeholder="Enter email" />
 * <Input error placeholder="Invalid email" />
 * <Input disabled placeholder="Disabled" />
 * ```
 */
const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type, error, ...props }, ref) => {
    return (
      <input
        type={type}
        className={cn(
          'flex h-9 w-full rounded-md',
          'border border-input bg-background/50 backdrop-blur-sm',
          'px-3 py-2 text-sm text-foreground',
          'shadow-inner',
          'placeholder:text-muted-foreground',
          'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent',
          'focus-visible:border-accent focus-visible:shadow-clay-button',
          'disabled:cursor-not-allowed disabled:opacity-50',
          'transition-all duration-200',
          error &&
            'border-destructive focus-visible:ring-destructive',
          className
        )}
        ref={ref}
        aria-invalid={error ? 'true' : undefined}
        {...props}
      />
    )
  }
)
Input.displayName = 'Input'

export { Input }
