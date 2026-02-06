"use client"

import { useState, useCallback, useEffect, useMemo } from "react"
import {
    Sparkles,
    ChevronRight,
    ChevronLeft,
    Loader2,
} from "lucide-react"
import {
    Button,
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
    cn,
} from "@/components/ds"
import { WizardStepper } from "./WizardStepper"
import { StepTypeSelection } from "./StepTypeSelection"
import { StepBasicInfo } from "./StepBasicInfo"
import { StepUris } from "./StepUris"
import { StepReview } from "./StepReview"
import { APP_SUB_TYPES, DEFAULT_FORM, getStepsForSubType } from "./constants"
import { generateClientId } from "./helpers"
import type { AppSubType, ClientFormState } from "./types"

interface ClientWizardProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    tenantSlug: string
    onSubmit: (form: ClientFormState) => void
    isPending: boolean
}

export function ClientWizard({
    open,
    onOpenChange,
    tenantSlug,
    onSubmit,
    isPending,
}: ClientWizardProps) {
    const [step, setStep] = useState(1)
    const [maxVisitedStep, setMaxVisitedStep] = useState(1)
    const [form, setForm] = useState<ClientFormState>({ ...DEFAULT_FORM })
    const [direction, setDirection] = useState<"forward" | "backward">("forward")

    // Calculate dynamic steps based on subType
    const wizardSteps = useMemo(() => getStepsForSubType(form.subType), [form.subType])

    // Get the step IDs that are available for this subType
    const availableStepIds = useMemo(() => wizardSteps.map(s => s.id), [wizardSteps])

    // Find current step index in the dynamic steps array
    const currentStepIndex = useMemo(() =>
        availableStepIds.indexOf(step),
        [availableStepIds, step]
    )

    // Check if we're on the last step
    const isLastStep = currentStepIndex === wizardSteps.length - 1

    // Reset on close
    useEffect(() => {
        if (!open) {
            // Small delay so animation plays out
            const timer = setTimeout(() => {
                setStep(1)
                setMaxVisitedStep(1)
                setForm({ ...DEFAULT_FORM })
            }, 200)
            return () => clearTimeout(timer)
        }
    }, [open])

    // Auto-generate clientId when name or type changes
    useEffect(() => {
        if (form.name && tenantSlug && open) {
            setForm(prev => ({
                ...prev,
                clientId: generateClientId(tenantSlug, prev.name, prev.type)
            }))
        }
    }, [form.name, form.type, tenantSlug, open])

    const handleFormChange = useCallback((patch: Partial<ClientFormState>) => {
        setForm(prev => ({ ...prev, ...patch }))
    }, [])

    const handleSubTypeSelect = useCallback((subType: AppSubType) => {
        const config = APP_SUB_TYPES[subType]
        setForm(prev => ({
            ...prev,
            subType,
            type: config.type,
            grantTypes: config.defaultGrantTypes,
            providers: config.defaultProviders,
        }))
    }, [])

    const handleRegenerateId = useCallback(() => {
        setForm(prev => ({
            ...prev,
            clientId: generateClientId(tenantSlug, prev.name, prev.type)
        }))
    }, [tenantSlug])

    const goToStep = useCallback((targetStep: number) => {
        setDirection(targetStep > step ? "forward" : "backward")
        setStep(targetStep)
        setMaxVisitedStep(prev => Math.max(prev, targetStep))
    }, [step])

    // Navigate to the next available step in the dynamic sequence
    const handleNext = useCallback(() => {
        const nextIndex = currentStepIndex + 1
        if (nextIndex < wizardSteps.length) {
            const nextStepId = wizardSteps[nextIndex].id
            goToStep(nextStepId)
        }
    }, [currentStepIndex, wizardSteps, goToStep])

    // Navigate to the previous available step in the dynamic sequence
    const handleBack = useCallback(() => {
        const prevIndex = currentStepIndex - 1
        if (prevIndex >= 0) {
            const prevStepId = wizardSteps[prevIndex].id
            setDirection("backward")
            setStep(prevStepId)
        }
    }, [currentStepIndex, wizardSteps])

    const handleSubmit = useCallback(() => {
        onSubmit(form)
    }, [form, onSubmit])

    // Validation per step — returns reason if blocked
    const validation = useMemo(() => {
        switch (step) {
            case 1: return { valid: true, reason: "" }
            case 2:
                if (!form.name.trim()) return { valid: false, reason: "El nombre es obligatorio" }
                return { valid: true, reason: "" }
            case 3: {
                // This step only appears if requiresRedirectUris is true
                const config = APP_SUB_TYPES[form.subType]
                const isPublic = config.type === "public"
                if (isPublic && form.redirectUris.length === 0)
                    return { valid: false, reason: "Agrega al menos una redirect URI" }
                return { valid: true, reason: "" }
            }
            case 4: return { valid: true, reason: "" }
            default: return { valid: false, reason: "" }
        }
    }, [step, form.name, form.subType, form.redirectUris.length])

    const canProceed = validation.valid

    // Dynamic step descriptions
    const getStepDescription = useCallback((stepId: number) => {
        const descriptions: Record<number, string> = {
            1: "Selecciona el tipo de aplicacion",
            2: "Informacion basica del cliente",
            3: "Configura las URLs de redireccion",
            4: "Revisa la configuracion y crea el cliente",
        }
        return descriptions[stepId] || ""
    }, [])

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-3xl max-h-[90vh] overflow-y-auto gap-3 p-5 sm:p-6 rounded-2xl">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2">
                        <Sparkles className="h-5 w-5 text-primary" />
                        Crear nuevo client OAuth2
                    </DialogTitle>
                    <DialogDescription>
                        Paso {currentStepIndex + 1} de {wizardSteps.length}: {getStepDescription(step)}
                    </DialogDescription>
                </DialogHeader>

                {/* Stepper - now receives dynamic steps */}
                <WizardStepper
                    currentStep={step}
                    onStepClick={goToStep}
                    maxVisitedStep={maxVisitedStep}
                    steps={wizardSteps}
                />

                {/* Step content with transition */}
                <div className="min-h-[200px] py-1 px-1 overflow-hidden">
                    <div
                        key={`step-${step}`}
                        className={cn(
                            "animate-in fade-in duration-300",
                            direction === "forward"
                                ? "slide-in-from-right-4"
                                : "slide-in-from-left-4"
                        )}
                    >
                        {step === 1 && (
                            <StepTypeSelection
                                form={form}
                                onSelect={handleSubTypeSelect}
                            />
                        )}

                        {step === 2 && (
                            <StepBasicInfo
                                form={form}
                                tenantSlug={tenantSlug}
                                onChange={handleFormChange}
                                onRegenerateId={handleRegenerateId}
                            />
                        )}

                        {step === 3 && (
                            <StepUris
                                form={form}
                                onChange={handleFormChange}
                            />
                        )}

                        {step === 4 && (
                            <StepReview
                                form={form}
                                onChange={handleFormChange}
                            />
                        )}
                    </div>
                </div>

                {/* Navigation */}
                <TooltipProvider delayDuration={200}>
                    <DialogFooter className="flex justify-between mt-4">
                        <div>
                            {currentStepIndex > 0 && (
                                <Button variant="ghost" onClick={handleBack}>
                                    <ChevronLeft className="mr-2 h-4 w-4" />
                                    Anterior
                                </Button>
                            )}
                        </div>
                        <div className="flex gap-2">
                            <Button
                                variant="outline"
                                onClick={() => onOpenChange(false)}
                            >
                                Cancelar
                            </Button>
                            {!isLastStep ? (
                                <Tooltip>
                                    <TooltipTrigger asChild>
                                        <span tabIndex={!canProceed ? 0 : undefined}>
                                            <Button onClick={handleNext} disabled={!canProceed}>
                                                Siguiente
                                                <ChevronRight className="ml-2 h-4 w-4" />
                                            </Button>
                                        </span>
                                    </TooltipTrigger>
                                    {!canProceed && validation.reason && (
                                        <TooltipContent side="top">
                                            <p className="text-xs">{validation.reason}</p>
                                        </TooltipContent>
                                    )}
                                </Tooltip>
                            ) : (
                                <Button
                                    onClick={handleSubmit}
                                    disabled={isPending || !canProceed}
                                >
                                    {isPending ? (
                                        <>
                                            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                            Creando...
                                        </>
                                    ) : (
                                        "Crear Client"
                                    )}
                                </Button>
                            )}
                        </div>
                    </DialogFooter>
                </TooltipProvider>
            </DialogContent>
        </Dialog>
    )
}
