import * as React from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../utils/cn'

/**
 * Button variants using CVA.
 * Provides consistent clay interactions with semantic tokens.
 */
const buttonVariants = cva(
  // Base styles - more rounded, softer feel
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-full font-medium transition-all duration-200 ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0',
  {
    variants: {
      variant: {
        default:
          'bg-gradient-to-b from-accent to-accent/90 text-white shadow-[0_1px_2px_rgba(0,0,0,0.1),0_2px_4px_rgba(0,0,0,0.08),inset_0_1px_0_rgba(255,255,255,0.15)] hover:shadow-[0_4px_8px_rgba(0,0,0,0.12),0_2px_4px_rgba(0,0,0,0.08),inset_0_1px_0_rgba(255,255,255,0.2)] hover:-translate-y-0.5 hover:brightness-110 hover:[text-shadow:0_0_12px_rgba(255,255,255,0.5)] active:translate-y-0 active:shadow-[0_1px_2px_rgba(0,0,0,0.1),inset_0_1px_2px_rgba(0,0,0,0.1)] active:brightness-95',
        primary:
          'bg-gradient-to-b from-primary to-primary/90 text-primary-foreground shadow-[0_1px_2px_rgba(0,0,0,0.1),0_2px_4px_rgba(0,0,0,0.08),inset_0_1px_0_rgba(255,255,255,0.1)] hover:shadow-[0_4px_8px_rgba(0,0,0,0.15),0_2px_4px_rgba(0,0,0,0.1)] hover:-translate-y-0.5 hover:brightness-105 hover:[text-shadow:0_0_12px_rgba(255,255,255,0.5)] active:translate-y-0 active:brightness-95',
        secondary:
          'bg-gradient-to-b from-secondary to-secondary/95 text-secondary-foreground border border-border/50 shadow-[0_1px_2px_rgba(0,0,0,0.04),inset_0_1px_0_rgba(255,255,255,0.5)] hover:shadow-[0_4px_8px_rgba(0,0,0,0.08),0_2px_4px_rgba(0,0,0,0.04)] hover:-translate-y-0.5 hover:border-border hover:[text-shadow:0_0_8px_rgba(0,0,0,0.1)] dark:hover:[text-shadow:0_0_8px_rgba(255,255,255,0.2)] active:translate-y-0 active:shadow-[inset_0_1px_2px_rgba(0,0,0,0.05)]',
        ghost:
          'bg-transparent text-foreground hover:bg-accent/10 hover:[text-shadow:0_0_8px_rgba(0,0,0,0.1)] dark:hover:[text-shadow:0_0_8px_rgba(255,255,255,0.2)] active:bg-accent/15',
        outline:
          'border-2 border-border/60 bg-background/50 backdrop-blur-sm text-foreground hover:bg-accent/5 hover:border-accent/50 hover:shadow-[0_2px_8px_rgba(var(--accent-h),var(--accent-s),var(--accent-l),0.15)] hover:[text-shadow:0_0_8px_rgba(0,0,0,0.1)] dark:hover:[text-shadow:0_0_8px_rgba(255,255,255,0.2)] active:bg-accent/10',
        danger:
          'bg-gradient-to-b from-destructive to-destructive/90 text-destructive-foreground shadow-[0_1px_2px_rgba(0,0,0,0.1),0_2px_4px_rgba(220,38,38,0.2),inset_0_1px_0_rgba(255,255,255,0.1)] hover:shadow-[0_4px_8px_rgba(220,38,38,0.3),0_2px_4px_rgba(0,0,0,0.1)] hover:-translate-y-0.5 hover:brightness-110 hover:[text-shadow:0_0_12px_rgba(255,255,255,0.5)] active:translate-y-0 active:brightness-95',
        success:
          'bg-gradient-to-b from-success to-success/90 text-white shadow-[0_1px_2px_rgba(0,0,0,0.1),0_2px_4px_rgba(34,197,94,0.2),inset_0_1px_0_rgba(255,255,255,0.1)] hover:shadow-[0_4px_8px_rgba(34,197,94,0.3),0_2px_4px_rgba(0,0,0,0.1)] hover:-translate-y-0.5 hover:brightness-110 hover:[text-shadow:0_0_12px_rgba(255,255,255,0.5)] active:translate-y-0 active:brightness-95',
        link:
          'text-accent underline-offset-4 hover:underline hover:brightness-110',
      },
      size: {
        xs: 'h-7 px-2.5 text-xs',
        sm: 'h-9 px-3.5 text-sm',
        md: 'h-10 px-5 text-sm',
        lg: 'h-12 px-6 text-base',
        xl: 'h-14 px-8 text-lg',
        icon: 'h-10 w-10',
        'icon-sm': 'h-8 w-8',
        'icon-lg': 'h-12 w-12',
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
