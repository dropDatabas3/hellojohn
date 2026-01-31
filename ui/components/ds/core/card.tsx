import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../utils/cn'

/**
 * Card variants using CVA.
 */
const cardVariants = cva(
  'rounded-xl border border-border/50 shadow-clay-card transition-all duration-200',
  {
    variants: {
      variant: {
        default: 'bg-card text-card-foreground',
        glass: 'bg-card/80 text-card-foreground backdrop-blur-sm',
        gradient: 'bg-gradient-to-br from-card to-secondary text-card-foreground',
      },
      interactive: {
        true: 'cursor-pointer hover:shadow-clay-float hover:-translate-y-1 active:translate-y-0 active:shadow-clay-card',
        false: '',
      },
      hoverable: {
        true: 'hover:shadow-float cursor-pointer',
        false: '',
      },
    },
    defaultVariants: {
      variant: 'default',
      interactive: false,
      hoverable: false,
    },
  }
)

export interface CardProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof cardVariants> {}

/**
 * Card component with clay styling.
 *
 * @example
 * ```tsx
 * <Card>
 *   <CardHeader>
 *     <CardTitle>Title</CardTitle>
 *     <CardDescription>Description</CardDescription>
 *   </CardHeader>
 *   <CardContent>
 *     Content goes here
 *   </CardContent>
 *   <CardFooter>
 *     <Button>Action</Button>
 *   </CardFooter>
 * </Card>
 * ```
 */
const Card = React.forwardRef<HTMLDivElement, CardProps>(
  ({ className, variant, interactive, hoverable, ...props }, ref) => (
    <div
      ref={ref}
      className={cn(cardVariants({ variant, interactive, hoverable, className }))}
      {...props}
    />
  )
)
Card.displayName = 'Card'

/**
 * Card header section.
 */
const CardHeader = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn('flex flex-col space-y-1.5 p-6', className)}
    {...props}
  />
))
CardHeader.displayName = 'CardHeader'

/**
 * Card title (typically h3).
 */
const CardTitle = React.forwardRef<
  HTMLHeadingElement,
  React.HTMLAttributes<HTMLHeadingElement>
>(({ className, ...props }, ref) => (
  <h3
    ref={ref}
    className={cn('text-2xl font-semibold leading-none tracking-tight', className)}
    {...props}
  />
))
CardTitle.displayName = 'CardTitle'

/**
 * Card description (muted text).
 */
const CardDescription = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLParagraphElement>
>(({ className, ...props }, ref) => (
  <p
    ref={ref}
    className={cn('text-sm text-muted-foreground', className)}
    {...props}
  />
))
CardDescription.displayName = 'CardDescription'

/**
 * Card content section.
 */
const CardContent = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div ref={ref} className={cn('p-6 pt-0', className)} {...props} />
))
CardContent.displayName = 'CardContent'

/**
 * Card footer section.
 */
const CardFooter = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn('flex items-center p-6 pt-0', className)}
    {...props}
  />
))
CardFooter.displayName = 'CardFooter'

export { Card, CardHeader, CardFooter, CardTitle, CardDescription, CardContent }
