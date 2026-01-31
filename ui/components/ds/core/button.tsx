import * as React from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../utils/cn'

/**
 * Button variants using CVA.
 * Provides consistent clay interactions with semantic tokens.
 */
const buttonVariants = cva(
  // Base styles
  'inline-flex items-center justify-center gap-2 rounded-button font-medium transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        default:
          'bg-gradient-to-b from-accent-2-clay to-accent-3-clay text-white shadow-clay-button hover:shadow-clay-card hover:-translate-y-0.5 active:translate-y-0 active:shadow-clay-button transition-all duration-200',
        primary:
          'bg-accent text-accent-foreground shadow-button hover:bg-accent-hover hover:-translate-y-0.5 active:bg-accent-active active:translate-y-0 active:scale-[0.98]',
        secondary:
          'bg-secondary text-secondary-foreground shadow-card hover:shadow-float hover:-translate-y-0.5 active:translate-y-0 active:scale-[0.98]',
        ghost:
          'bg-transparent hover:bg-accent/10 transition-colors duration-150',
        outline:
          'border-2 border-border bg-background/80 backdrop-blur-sm hover:bg-accent/5 hover:border-accent hover:shadow-clay-button transition-all duration-200',
        danger:
          'bg-danger text-white shadow-button hover:opacity-90 hover:-translate-y-0.5 active:translate-y-0 active:scale-[0.98]',
      },
      size: {
        sm: 'h-9 px-3 text-sm',
        md: 'h-11 px-4 text-base',
        lg: 'h-14 px-6 text-lg',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'md',
    },
  }
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  /**
   * If true, renders as a child component (using Radix Slot).
   * Useful for rendering as Link, etc.
   */
  asChild?: boolean
  /**
   * If true, shows loading spinner and disables button.
   */
  loading?: boolean
  /**
   * Optional icon to render before children.
   */
  leftIcon?: React.ReactNode
  /**
   * Optional icon to render after children.
   */
  rightIcon?: React.ReactNode
}

/**
 * Button component with clay interactions and semantic styling.
 *
 * @example
 * ```tsx
 * <Button variant="primary" size="md">
 *   Click me
 * </Button>
 *
 * <Button variant="danger" loading>
 *   Deleting...
 * </Button>
 *
 * <Button variant="outline" leftIcon={<Icon />}>
 *   With icon
 * </Button>
 * ```
 */
const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      variant,
      size,
      asChild = false,
      loading = false,
      leftIcon,
      rightIcon,
      children,
      disabled,
      ...props
    },
    ref
  ) => {
    const Comp = asChild ? Slot : 'button'

    // When asChild is true, Slot expects exactly ONE child element.
    // We can't wrap with extra elements, so icons/spinner are not supported with asChild.
    // The child element (e.g., Link) should contain its own content.
    if (asChild) {
      return (
        <Comp
          className={cn(buttonVariants({ variant, size, className }))}
          ref={ref}
          {...props}
        >
          {children}
        </Comp>
      )
    }

    // Normal button with full features (icons, loading state)
    return (
      <Comp
        className={cn(buttonVariants({ variant, size, className }))}
        ref={ref}
        disabled={disabled || loading}
        {...props}
      >
        {loading && (
          <svg
            className="h-4 w-4 animate-spin"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
            aria-hidden="true"
          >
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            />
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            />
          </svg>
        )}
        {!loading && leftIcon}
        {children}
        {!loading && rightIcon}
      </Comp>
    )
  }
)

Button.displayName = 'Button'

export { Button, buttonVariants }
