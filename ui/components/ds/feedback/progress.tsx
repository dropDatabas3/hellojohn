"use client"

import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "../utils/cn"

/**
 * Progress — Design System
 * 
 * A premium, animated progress bar with clay styling.
 * Features smooth transitions, gradient fills, and optional pulse animation.
 * 
 * @example
 * ```tsx
 * // Basic
 * <Progress value={50} />
 * 
 * // With variants
 * <Progress value={75} variant="success" size="lg" />
 * 
 * // Animated during loading
 * <Progress value={30} animated />
 * 
 * // Indeterminate loading
 * <Progress indeterminate />
 * ```
 */

const progressVariants = cva(
    "relative w-full overflow-hidden rounded-full bg-muted/50 shadow-clay-inset",
    {
        variants: {
            size: {
                sm: "h-1.5",
                md: "h-2.5",
                lg: "h-4",
            },
        },
        defaultVariants: {
            size: "md",
        },
    }
)

const progressBarVariants = cva(
    "h-full rounded-full transition-all duration-500 ease-out",
    {
        variants: {
            variant: {
                default: "bg-gradient-to-r from-accent via-accent to-accent/80",
                success: "bg-gradient-to-r from-success via-success to-success/80",
                warning: "bg-gradient-to-r from-warning via-warning/90 to-warning/80",
                danger: "bg-gradient-to-r from-danger via-danger to-danger/80",
                info: "bg-gradient-to-r from-info via-info to-info/80",
                premium: "bg-gradient-to-r from-accent via-[#8B5CF6] to-[#EC4899]",
            },
        },
        defaultVariants: {
            variant: "default",
        },
    }
)

export interface ProgressProps
    extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof progressVariants>,
    VariantProps<typeof progressBarVariants> {
    /**
     * The current progress value (0-100)
     */
    value?: number
    /**
     * Show a shimmer animation on the progress bar
     */
    animated?: boolean
    /**
     * Show an indeterminate loading animation (ignores value)
     */
    indeterminate?: boolean
    /**
     * Show the percentage label
     */
    showLabel?: boolean
    /**
     * Show a glow effect under the progress bar
     */
    glow?: boolean
}

const Progress = React.forwardRef<HTMLDivElement, ProgressProps>(
    (
        {
            className,
            value = 0,
            variant,
            size,
            animated = false,
            indeterminate = false,
            showLabel = false,
            glow = false,
            ...props
        },
        ref
    ) => {
        // Clamp value between 0 and 100
        const clampedValue = Math.min(100, Math.max(0, value))

        return (
            <div className="relative w-full">
                {/* Glow effect */}
                {glow && clampedValue > 0 && (
                    <div
                        className={cn(
                            "absolute inset-0 rounded-full blur-md opacity-40 transition-all duration-500",
                            variant === "success" && "bg-success",
                            variant === "warning" && "bg-warning",
                            variant === "danger" && "bg-danger",
                            variant === "info" && "bg-info",
                            variant === "premium" && "bg-gradient-to-r from-accent via-[#8B5CF6] to-[#EC4899]",
                            (!variant || variant === "default") && "bg-accent"
                        )}
                        style={{ width: `${clampedValue}%` }}
                    />
                )}

                {/* Track */}
                <div
                    ref={ref}
                    role="progressbar"
                    aria-valuenow={indeterminate ? undefined : clampedValue}
                    aria-valuemin={0}
                    aria-valuemax={100}
                    className={cn(progressVariants({ size }), className)}
                    {...props}
                >
                    {/* Fill bar */}
                    <div
                        className={cn(
                            progressBarVariants({ variant }),
                            "relative overflow-hidden",
                            // Indeterminate animation
                            indeterminate && "animate-indeterminate"
                        )}
                        style={{
                            width: indeterminate ? "40%" : `${clampedValue}%`,
                        }}
                    >
                        {/* Shimmer overlay animation */}
                        {animated && !indeterminate && (
                            <div className="absolute inset-0 -translate-x-full animate-shimmer bg-gradient-to-r from-transparent via-white/30 to-transparent" />
                        )}

                        {/* Inner highlight for 3D effect */}
                        <div
                            className={cn(
                                "absolute inset-x-0 top-0 h-1/2 rounded-t-full",
                                "bg-gradient-to-b from-white/25 to-transparent"
                            )}
                        />
                    </div>
                </div>

                {/* Label */}
                {showLabel && !indeterminate && (
                    <div className="mt-1.5 flex justify-between text-xs text-muted-foreground">
                        <span>{clampedValue}%</span>
                        {clampedValue === 100 && (
                            <span className="text-success font-medium animate-in fade-in">
                                ✓ Completado
                            </span>
                        )}
                    </div>
                )}
            </div>
        )
    }
)
Progress.displayName = "Progress"

export { Progress, progressVariants }
