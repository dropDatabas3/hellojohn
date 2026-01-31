"use client"

import * as React from "react"
import Link from "next/link"
import { ArrowRight, type LucideIcon } from "lucide-react"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "../utils/cn"

/**
 * QuickLinkCard — Design System
 * 
 * Interactive navigation card with icon, title, description and arrow indicator.
 * Used for quick access grids in dashboard-style pages.
 * 
 * @example
 * ```tsx
 * <QuickLinkCard
 *   href="/admin/users"
 *   icon={Users}
 *   title="Users"
 *   description="Manage user accounts"
 *   variant="info"
 * />
 * ```
 */

const quickLinkCardVariants = cva(
    [
        "group relative",
        "rounded-xl border p-4",
        "bg-gradient-to-br",
        "transition-all duration-200",
        "hover:-translate-y-1 hover:shadow-float",
        "active:translate-y-0 active:shadow-card",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background",
    ].join(" "),
    {
        variants: {
            variant: {
                default: [
                    "from-muted/50 to-muted/20",
                    "border-border/50 hover:border-border",
                ].join(" "),
                accent: [
                    "from-accent/15 to-accent/5",
                    "border-accent/30 hover:border-accent/50",
                ].join(" "),
                info: [
                    "from-info/15 to-info/5",
                    "border-info/30 hover:border-info/50",
                ].join(" "),
                success: [
                    "from-success/15 to-success/5",
                    "border-success/30 hover:border-success/50",
                ].join(" "),
                warning: [
                    "from-warning/15 to-warning/5",
                    "border-warning/30 hover:border-warning/50",
                ].join(" "),
                danger: [
                    "from-danger/15 to-danger/5",
                    "border-danger/30 hover:border-danger/50",
                ].join(" "),
            },
        },
        defaultVariants: {
            variant: "default",
        },
    }
)

const iconVariants = cva("p-2 rounded-lg bg-background/50", {
    variants: {
        variant: {
            default: "text-muted-foreground",
            accent: "text-accent",
            info: "text-info",
            success: "text-success",
            warning: "text-warning",
            danger: "text-danger",
        },
    },
    defaultVariants: {
        variant: "default",
    },
})

export interface QuickLinkCardProps
    extends VariantProps<typeof quickLinkCardVariants> {
    /** Navigation destination */
    href: string
    /** Lucide icon component */
    icon: LucideIcon
    /** Card title */
    title: string
    /** Card description (1-2 lines) */
    description: string
    /** Additional CSS classes */
    className?: string
}

export function QuickLinkCard({
    href,
    icon: Icon,
    title,
    description,
    variant = "default",
    className,
}: QuickLinkCardProps) {
    return (
        <Link
            href={href}
            className={cn(quickLinkCardVariants({ variant }), className)}
        >
            <div className="flex items-start gap-3">
                <div className={iconVariants({ variant })}>
                    <Icon className="h-5 w-5" />
                </div>
                <div className="flex-1 min-w-0">
                    <h3 className="font-medium text-sm text-foreground flex items-center gap-2">
                        {title}
                        <ArrowRight className="h-3 w-3 opacity-0 -translate-x-2 group-hover:opacity-100 group-hover:translate-x-0 transition-all duration-200" />
                    </h3>
                    <p className="text-xs text-muted-foreground mt-0.5 line-clamp-2">
                        {description}
                    </p>
                </div>
            </div>
        </Link>
    )
}

QuickLinkCard.displayName = "QuickLinkCard"
