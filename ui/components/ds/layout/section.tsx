import * as React from 'react'
import { cn } from '../utils/cn'

export interface SectionProps extends React.HTMLAttributes<HTMLElement> {
  /**
   * Optional section title (renders as h2).
   */
  title?: string
  /**
   * Optional section description.
   */
  description?: string
}

/**
 * Section component for consistent vertical rhythm and spacing.
 *
 * @example
 * ```tsx
 * <Section title="Settings" description="Manage your account settings">
 *   <Card>Content</Card>
 * </Section>
 *
 * <Section>
 *   <Card>Content without title</Card>
 * </Section>
 * ```
 */
const Section = React.forwardRef<HTMLElement, SectionProps>(
  ({ className, title, description, children, ...props }, ref) => {
    return (
      <section
        ref={ref}
        className={cn('space-y-6', className)}
        {...props}
      >
        {(title || description) && (
          <div className="space-y-1">
            {title && (
              <h2 className="text-2xl font-semibold tracking-tight text-foreground">
                {title}
              </h2>
            )}
            {description && (
              <p className="text-sm text-muted-foreground">{description}</p>
            )}
          </div>
        )}
        {children}
      </section>
    )
  }
)
Section.displayName = 'Section'

export { Section }
