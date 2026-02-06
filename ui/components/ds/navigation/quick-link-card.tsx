"use client"

import * as React from "react"
import Link from "next/link"
import { ArrowRight, type LucideIcon } from "lucide-react"
import { cn } from "../utils/cn"

/**
 * QuickLinkCard — Design System
 *
 * Interactive navigation card with icon, title, description and arrow indicator.
 * Used for quick access grids in dashboard-style pages.
 *
 * Supports an `attention` prop that triggers a subtle looping pulse animation
 * with an amber glow, useful for onboarding cues (e.g., "configure storage next").
 *
 * @example
 * ```tsx
 * <QuickLinkCard
 *   href="/admin/users"
 *   icon={Users}
 *   title="Users"
 *   description="Manage user accounts"
 * />
 *
 * <QuickLinkCard
 *   href="/admin/storage"
 *   icon={Database}
 *   title="Storage"
 *   description="Configure database"
 *   attention
 * />
 * ```
 */

/** Base visual classes shared by all cards (no transform / transition — those are mode-specific). */
const baseClasses = [
    "group relative overflow-hidden",
    "rounded-xl border p-4",
    "bg-gradient-to-br",
    "from-white/80 via-slate-50/60 to-slate-100/50",
    "dark:from-slate-800/80 dark:via-slate-800/60 dark:to-slate-900/50",
    "border-white/70",
    "dark:border-white/15",
    "backdrop-blur-md",
    // 3D depth shadow
    "shadow-[0_1px_2px_rgba(0,0,0,0.04),0_2px_4px_rgba(0,0,0,0.04),0_4px_8px_rgba(0,0,0,0.06),0_8px_16px_rgba(0,0,0,0.06),inset_0_1px_0_rgba(255,255,255,0.7)]",
    "dark:shadow-[0_1px_2px_rgba(0,0,0,0.1),0_2px_4px_rgba(0,0,0,0.15),0_4px_8px_rgba(0,0,0,0.2),0_8px_16px_rgba(0,0,0,0.15),inset_0_1px_0_rgba(255,255,255,0.08)]",
    // Violet glow overlay on hover (::after)
    "after:absolute after:inset-0 after:rounded-xl after:pointer-events-none",
    "after:bg-gradient-to-br after:from-violet-500/0 after:via-purple-500/0 after:to-fuchsia-500/0",
    "after:opacity-0 after:transition-all after:duration-500 after:ease-[cubic-bezier(0.4,0,0.2,1)]",
    "hover:after:from-violet-500/15 hover:after:via-purple-500/10 hover:after:to-fuchsia-500/5",
    "hover:after:opacity-100",
    "dark:hover:after:from-violet-500/20 dark:hover:after:via-purple-500/15 dark:hover:after:to-fuchsia-500/10",
    // Subtle inner highlight (::before)
    "before:absolute before:inset-0 before:rounded-xl before:opacity-0 before:transition-all before:duration-500",
    "before:bg-gradient-to-br before:from-white/30 before:via-transparent before:to-transparent",
    "hover:before:opacity-100",
    // Hover shadow
    "hover:shadow-[0_4px_6px_rgba(139,92,246,0.06),0_6px_12px_rgba(139,92,246,0.04),0_12px_24px_rgba(0,0,0,0.05),inset_0_1px_0_rgba(255,255,255,0.8)]",
    "dark:hover:shadow-[0_4px_6px_rgba(139,92,246,0.08),0_6px_12px_rgba(139,92,246,0.06),0_12px_24px_rgba(0,0,0,0.15),inset_0_1px_0_rgba(255,255,255,0.12)]",
    // Hover border glow
    "hover:border-violet-300/30 dark:hover:border-violet-500/20",
    // Focus
    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-500 focus-visible:ring-offset-2 focus-visible:ring-offset-background",
].join(" ")

/**
 * Normal mode extras: transition-all + Tailwind hover transform + active reset.
 * These use Tailwind's transform utilities which would conflict with the keyframe,
 * so they are only applied to non-attention cards.
 */
const normalModeClasses = [
    "transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)]",
    "hover:-translate-y-2 hover:scale-[1.03]",
    "active:translate-y-0 active:scale-[1.01]",
].join(" ")

/**
 * Attention mode extras: the CSS class `.animate-onboarding-nudge` in globals.css
 * drives the looping translateY+scale via @property custom props.
 * Transition excludes transform so it doesn't fight the keyframe.
 */
const attentionModeClasses = [
    "transition-[color,background-color,border-color,text-decoration-color,fill,stroke,opacity,box-shadow,filter,backdrop-filter]",
    "duration-500 ease-[cubic-bezier(0.4,0,0.2,1)]",
    "animate-onboarding-nudge",
].join(" ")

const iconColorMap: Record<string, string> = {
    default: "text-muted-foreground",
    accent: "text-accent",
    info: "text-info",
    success: "text-success",
    warning: "text-warning",
    danger: "text-danger",
}

const baseIconClasses = [
    "p-2.5 rounded-xl",
    "bg-white/80 dark:bg-white/15",
    "shadow-[0_2px_4px_rgba(0,0,0,0.08),0_4px_8px_rgba(0,0,0,0.04),inset_0_1px_0_rgba(255,255,255,0.9)]",
    "dark:shadow-[0_2px_4px_rgba(0,0,0,0.25),0_4px_8px_rgba(0,0,0,0.15),inset_0_1px_0_rgba(255,255,255,0.15)]",
    "transition-all duration-300",
    "group-hover:scale-115 group-hover:-rotate-3",
    "group-hover:shadow-[0_4px_8px_rgba(0,0,0,0.12),0_8px_16px_rgba(0,0,0,0.08),inset_0_1px_0_rgba(255,255,255,1)]",
    "dark:group-hover:shadow-[0_4px_8px_rgba(0,0,0,0.35),0_8px_16px_rgba(0,0,0,0.25),inset_0_1px_0_rgba(255,255,255,0.2)]",
].join(" ")

export type QuickLinkCardIconColor = "default" | "accent" | "info" | "success" | "warning" | "danger"

export interface QuickLinkCardProps {
    /** Navigation destination */
    href: string
    /** Lucide icon component */
    icon: LucideIcon
    /** Card title */
    title: string
    /** Card description (1-2 lines) */
    description: string
    /** Icon color variant */
    iconColor?: QuickLinkCardIconColor
    /** Enable attention-seeking pulse animation (onboarding cue) */
    attention?: boolean
    /** Additional CSS classes */
    className?: string
}

export function QuickLinkCard({
    href,
    icon: Icon,
    title,
    description,
    iconColor = "default",
    attention = false,
    className,
}: QuickLinkCardProps) {
    return (
        <Link
            href={href}
            className={cn(
                baseClasses,
                attention ? attentionModeClasses : normalModeClasses,
                className,
            )}
        >
            {/* Attention overlay — teal/cyan wash synced to nudge movement via --nudge-glow */}
            {attention && (
                <span
                    aria-hidden
                    className="absolute inset-0 rounded-xl pointer-events-none z-[1] bg-gradient-to-br from-blue-500/30 via-cyan-500/22 to-sky-500/14 dark:from-blue-400/35 dark:via-cyan-400/25 dark:to-sky-400/16"
                    style={{ opacity: "var(--nudge-glow)" } as React.CSSProperties}
                />
            )}
            <div className="relative z-10 flex items-start gap-3">
                <div className={cn(baseIconClasses, iconColorMap[iconColor] ?? iconColorMap.default)}>
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
