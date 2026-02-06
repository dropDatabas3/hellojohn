"use client"

import { Skeleton, cn } from "@/components/ds"

interface StatCardProps {
    icon: React.ElementType
    label: string
    value: string | number
    variant?: "default" | "success" | "warning" | "danger"
    isLoading?: boolean
}

const colorClasses = {
    default: "bg-info/10 text-info",
    success: "bg-success/10 text-success",
    warning: "bg-warning/10 text-warning",
    danger: "bg-danger/10 text-danger",
}

export function StatCard({ icon: Icon, label, value, variant = "default", isLoading = false }: StatCardProps) {
    return (
        <div className="flex items-center gap-4 p-4 rounded-xl bg-gradient-to-br from-card to-muted/30 border shadow-clay-card hover:shadow-clay-float transition-all duration-300 hover:-translate-y-0.5">
            <div className={cn("p-3 rounded-xl shadow-inner", isLoading ? "bg-muted/30" : colorClasses[variant])}>
                {isLoading ? (
                    <Skeleton className="h-5 w-5 rounded-full" />
                ) : (
                    <Icon className="h-5 w-5" />
                )}
            </div>
            <div className="space-y-1">
                {isLoading ? (
                    <>
                        <Skeleton className="h-3 w-20" />
                        <Skeleton className="h-7 w-10" />
                    </>
                ) : (
                    <>
                        <p className="text-xs text-muted-foreground font-medium">{label}</p>
                        <p className="text-2xl font-bold">{value}</p>
                    </>
                )}
            </div>
        </div>
    )
}
