"use client"

import {
    LayoutGrid,
    FileText,
    Link2,
    CheckCircle2,
    Check,
} from "lucide-react"
import { cn } from "@/components/ds"
import type { WizardStep } from "./types"

const STEP_ICONS: Record<string, React.ElementType> = {
    LayoutGrid,
    FileText,
    Link2,
    CheckCircle2,
}

interface WizardStepperProps {
    currentStep: number
    onStepClick: (step: number) => void
    /** Highest step the user has visited (enables clicking back) */
    maxVisitedStep: number
    /** Dynamic steps based on client subType */
    steps: WizardStep[]
}

export function WizardStepper({ currentStep, onStepClick, maxVisitedStep, steps }: WizardStepperProps) {
    return (
        <nav className="flex items-center justify-between px-2 py-4" aria-label="Wizard progress">
            {steps.map((step, idx) => {
                const Icon = STEP_ICONS[step.icon] || LayoutGrid
                const isCompleted = currentStep > step.id
                const isActive = currentStep === step.id
                const isClickable = step.id <= maxVisitedStep
                const isLast = idx === steps.length - 1

                return (
                    <div key={step.id} className="flex items-center flex-1 last:flex-none">
                        {/* Step circle + label */}
                        <button
                            type="button"
                            onClick={() => isClickable && onStepClick(step.id)}
                            disabled={!isClickable}
                            className={cn(
                                "flex items-center gap-2.5 group transition-all duration-200",
                                isClickable && !isActive && "cursor-pointer",
                                !isClickable && "cursor-default"
                            )}
                            aria-current={isActive ? "step" : undefined}
                        >
                            {/* Circle */}
                            <div
                                className={cn(
                                    "flex items-center justify-center h-9 w-9 rounded-full border-2 transition-all duration-300 shrink-0",
                                    isCompleted && "bg-primary border-primary text-white",
                                    isActive && "bg-primary/10 border-primary text-primary shadow-md shadow-primary/20",
                                    !isCompleted && !isActive && "bg-muted/50 border-border text-muted-foreground",
                                    isClickable && !isActive && !isCompleted && "group-hover:border-primary/40 group-hover:text-primary/70",
                                )}
                            >
                                {isCompleted ? (
                                    <Check className="h-4 w-4" />
                                ) : (
                                    <Icon className="h-4 w-4" />
                                )}
                            </div>

                            {/* Label */}
                            <div className="hidden sm:block">
                                <p
                                    className={cn(
                                        "text-sm font-medium leading-tight transition-colors duration-200",
                                        isActive && "text-primary",
                                        isCompleted && "text-foreground",
                                        !isActive && !isCompleted && "text-muted-foreground",
                                    )}
                                >
                                    {step.label}
                                </p>
                                <p className="text-[11px] text-muted-foreground leading-tight">
                                    {step.description}
                                </p>
                            </div>
                        </button>

                        {/* Connector line */}
                        {!isLast && (
                            <div className="flex-1 mx-3">
                                <div
                                    className={cn(
                                        "h-0.5 rounded-full transition-all duration-500",
                                        isCompleted ? "bg-primary" : "bg-border",
                                    )}
                                />
                            </div>
                        )}
                    </div>
                )
            })}
        </nav>
    )
}
