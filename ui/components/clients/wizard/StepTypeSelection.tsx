"use client"

import {
    Globe,
    Smartphone,
    Server,
    Cpu,
    Check,
} from "lucide-react"
import { Badge, cn } from "@/components/ds"
import { APP_SUB_TYPES } from "./constants"
import type { AppSubType, ClientFormState } from "./types"

const SUB_TYPE_ICONS: Record<string, React.ElementType> = {
    Globe,
    Smartphone,
    Server,
    Cpu,
}

interface StepTypeSelectionProps {
    form: ClientFormState
    onSelect: (subType: AppSubType) => void
}

export function StepTypeSelection({ form, onSelect }: StepTypeSelectionProps) {
    const entries = Object.entries(APP_SUB_TYPES) as [AppSubType, (typeof APP_SUB_TYPES)[AppSubType]][]

    return (
        <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3">
                {entries.map(([key, config]) => {
                    const Icon = SUB_TYPE_ICONS[config.icon] || Globe
                    const selected = form.subType === key
                    const isPublic = config.type === "public"

                    return (
                        <button
                            key={key}
                            type="button"
                            onClick={() => onSelect(key)}
                            className={cn(
                                "relative flex flex-col items-start p-4 sm:p-5 rounded-xl border-2 text-left transition-all duration-300 group",
                                selected
                                    ? isPublic
                                        ? "border-green-500 shadow-clay-card ring-2 ring-green-500/30 scale-[1.01]"
                                        : "border-accent bg-gradient-to-br from-accent/12 to-accent/5 shadow-clay-card ring-2 ring-accent/20 scale-[1.01]"
                                    : isPublic
                                        ? "border-border/60 hover:border-green-500/40 hover:bg-green-500/5 hover:shadow-clay-card hover:scale-[1.005]"
                                        : "border-border/60 hover:border-primary/40 hover:bg-muted/40 hover:shadow-clay-card hover:scale-[1.005]"
                            )}
                            style={selected && isPublic ? {
                                background: "linear-gradient(to bottom right, rgba(16, 185, 129, 0.15), rgba(16, 185, 129, 0.05))"
                            } : undefined}
                        >
                            {/* Tag label - con check integrado si está seleccionado */}
                            {config.tagLabel && (
                                <Badge
                                    variant="secondary"
                                    className={cn(
                                        "absolute top-3 right-3 text-[10px] flex items-center gap-1",
                                        selected && isPublic && "bg-green-500/20 text-green-500 border-green-500/30"
                                    )}
                                >
                                    {selected && (
                                        <Check className="h-3 w-3 stroke-[3]" />
                                    )}
                                    {config.tagLabel}
                                </Badge>
                            )}

                            {/* Selected check - solo para tarjetas SIN tagLabel */}
                            {selected && !config.tagLabel && (
                                <div className={cn(
                                    "absolute top-3 right-3 rounded-full p-1 flex items-center justify-center",
                                    isPublic
                                        ? "bg-green-500 text-white"
                                        : "bg-accent text-white",
                                )}>
                                    <Check className="h-3.5 w-3.5 stroke-[3]" />
                                </div>
                            )}

                            {/* Icon */}
                            <div
                                className={cn(
                                    "p-3 rounded-xl mb-3 transition-all duration-300",
                                    selected
                                        ? isPublic
                                            ? "text-emerald-500 shadow-sm"
                                            : "bg-accent/15 text-accent shadow-sm shadow-accent/10"
                                        : isPublic
                                            ? "text-emerald-500/60 group-hover:text-emerald-500"
                                            : "bg-accent/8 text-accent/60 group-hover:bg-accent/15 group-hover:text-accent"
                                )}
                                style={selected && isPublic ? { backgroundColor: "rgba(16, 185, 129, 0.2)" } :
                                    isPublic ? { backgroundColor: "rgba(16, 185, 129, 0.1)" } : undefined}
                            >
                                <Icon className="h-6 w-6" />
                            </div>

                            {/* Title */}
                            <h3 className={cn(
                                "font-bold mb-0.5 transition-colors",
                                selected
                                    ? isPublic ? "text-emerald-500 text-base" : "text-accent text-base"
                                    : "text-foreground text-sm"
                            )}>
                                {config.label}
                            </h3>

                            {/* Description */}
                            <p className="text-xs text-muted-foreground mb-3">
                                {config.description}
                            </p>

                            {/* Features */}
                            <ul className="text-xs space-y-1 mt-auto">
                                {config.features.map((f, i) => (
                                    <li key={i} className="flex items-center gap-1.5 text-muted-foreground">
                                        <Check className={cn(
                                            "h-3 w-3 stroke-[2.5]",
                                            selected
                                                ? isPublic ? "text-emerald-500" : "text-accent"
                                                : "text-muted-foreground/40"
                                        )} />
                                        {f}
                                    </li>
                                ))}
                            </ul>

                            {/* Type indicator */}
                            <div className="mt-3 pt-3 border-t border-border/50 w-full">
                                <Badge
                                    variant="outline"
                                    className={cn(
                                        "text-[10px]",
                                        selected && isPublic && "border-emerald-500/50 text-emerald-500 bg-emerald-500/10",
                                        selected && !isPublic && "border-accent/40 text-accent bg-accent/5",
                                    )}
                                >
                                    {isPublic ? "Public (PKCE)" : "Confidential (Secret)"}
                                </Badge>
                            </div>
                        </button>
                    )
                })}
            </div>
        </div>
    )
}
