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
        "group relative overflow-hidden",
        "rounded-xl border p-4",
        "bg-gradient-to-br",
        "backdrop-blur-md",
        "transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)]",
        // Enhanced 3D depth effect
        "shadow-[0_1px_2px_rgba(0,0,0,0.04),0_2px_4px_rgba(0,0,0,0.04),0_4px_8px_rgba(0,0,0,0.06),0_8px_16px_rgba(0,0,0,0.06),inset_0_1px_0_rgba(255,255,255,0.7)]",
        "dark:shadow-[0_1px_2px_rgba(0,0,0,0.1),0_2px_4px_rgba(0,0,0,0.15),0_4px_8px_rgba(0,0,0,0.2),0_8px_16px_rgba(0,0,0,0.15),inset_0_1px_0_rgba(255,255,255,0.08)]",
        // Violet glow overlay on hover (pseudo-element)
        "after:absolute after:inset-0 after:rounded-xl after:pointer-events-none",
        "after:bg-gradient-to-br after:from-violet-500/0 after:via-purple-500/0 after:to-fuchsia-500/0",
        "after:opacity-0 after:transition-all after:duration-500 after:ease-[cubic-bezier(0.4,0,0.2,1)]",
        "hover:after:from-violet-500/15 hover:after:via-purple-500/10 hover:after:to-fuchsia-500/5",
        "hover:after:opacity-100",
        "dark:hover:after:from-violet-500/20 dark:hover:after:via-purple-500/15 dark:hover:after:to-fuchsia-500/10",
        // Subtle inner highlight
        "before:absolute before:inset-0 before:rounded-xl before:opacity-0 before:transition-all before:duration-500",
        "before:bg-gradient-to-br before:from-white/30 before:via-transparent before:to-transparent",
        "hover:before:opacity-100",
        // Hover state with enhanced 3D lift
        "hover:-translate-y-2 hover:scale-[1.03]",
        "hover:shadow-[0_4px_6px_rgba(139,92,246,0.06),0_6px_12px_rgba(139,92,246,0.04),0_12px_24px_rgba(0,0,0,0.05),inset_0_1px_0_rgba(255,255,255,0.8)]",
        "dark:hover:shadow-[0_4px_6px_rgba(139,92,246,0.08),0_6px_12px_rgba(139,92,246,0.06),0_12px_24px_rgba(0,0,0,0.15),inset_0_1px_0_rgba(255,255,255,0.12)]",
        // Violet border glow on hover
        "hover:border-violet-300/30 dark:hover:border-violet-500/20",
        // Active state
        "active:translate-y-0 active:scale-[1.01]",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-500 focus-visible:ring-offset-2 focus-visible:ring-offset-background",
    ].join(" "),
    {
        variants: {
            variant: {
                default: [
                    "from-white/80 via-slate-50/60 to-slate-100/50",
                    "dark:from-slate-800/80 dark:via-slate-800/60 dark:to-slate-900/50",
                    "border-white/70",
                    "dark:border-white/15",
                ].join(" "),
                accent: [
                    "from-violet-100/40 via-purple-50/30 to-fuchsia-50/20",
                    "dark:from-violet-950/40 dark:via-purple-950/30 dark:to-fuchsia-950/20",
                    "border-violet-200/50",
                    "dark:border-violet-500/25",
                ].join(" "),
                info: [
                    "from-white/80 via-slate-50/60 to-slate-100/50",
                    "dark:from-slate-800/80 dark:via-slate-800/60 dark:to-slate-900/50",
                    "border-white/70",
                    "dark:border-white/15",
                ].join(" "),
                success: [
                    "from-white/80 via-slate-50/60 to-slate-100/50",
                    "dark:from-slate-800/80 dark:via-slate-800/60 dark:to-slate-900/50",
                    "border-white/70",
                    "dark:border-white/15",
                ].join(" "),
                warning: [
                    "from-white/80 via-slate-50/60 to-slate-100/50",
                    "dark:from-slate-800/80 dark:via-slate-800/60 dark:to-slate-900/50",
                    "border-white/70",
                    "dark:border-white/15",
                ].join(" "),
                danger: [
                    "from-white/80 via-slate-50/60 to-slate-100/50",
                    "dark:from-slate-800/80 dark:via-slate-800/60 dark:to-slate-900/50",
                    "border-white/70",
                    "dark:border-white/15",
                ].join(" "),
            },
        },
        defaultVariants: {
            variant: "default",
        },
    }
)

const iconVariants = cva([
    "p-2.5 rounded-xl",
    "bg-white/80 dark:bg-white/15",
    "shadow-[0_2px_4px_rgba(0,0,0,0.08),0_4px_8px_rgba(0,0,0,0.04),inset_0_1px_0_rgba(255,255,255,0.9)]",
    "dark:shadow-[0_2px_4px_rgba(0,0,0,0.25),0_4px_8px_rgba(0,0,0,0.15),inset_0_1px_0_rgba(255,255,255,0.15)]",
    "transition-all duration-300",
    "group-hover:scale-115 group-hover:-rotate-3",
    "group-hover:shadow-[0_4px_8px_rgba(0,0,0,0.12),0_8px_16px_rgba(0,0,0,0.08),inset_0_1px_0_rgba(255,255,255,1)]",
    "dark:group-hover:shadow-[0_4px_8px_rgba(0,0,0,0.35),0_8px_16px_rgba(0,0,0,0.25),inset_0_1px_0_rgba(255,255,255,0.2)]",
].join(" "), {
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
