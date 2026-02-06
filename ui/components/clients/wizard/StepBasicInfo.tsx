"use client"

import { useState, useMemo } from "react"
import {
    Pencil,
    RotateCcw,
    Check,
    X,
    Info,
    Plus,
    AlertCircle,
} from "lucide-react"
import {
    Badge,
    Button,
    Input,
    Label,
    Textarea,
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
    cn,
} from "@/components/ds"
import { InfoTooltip } from "@/components/clients/shared"
import {
    APP_SUB_TYPES,
    M2M_SCOPE_PLACEHOLDER,
    getScopesForSubType,
    getInfoMessage,
} from "./constants"
import { validateClientId } from "./helpers"
import type { ClientFormState } from "./types"

interface StepBasicInfoProps {
    form: ClientFormState
    tenantSlug: string
    onChange: (patch: Partial<ClientFormState>) => void
    onRegenerateId: () => void
}

export function StepBasicInfo({ form, tenantSlug, onChange, onRegenerateId }: StepBasicInfoProps) {
    const [editingClientId, setEditingClientId] = useState(false)
    const [manualClientId, setManualClientId] = useState("")
    const [clientIdError, setClientIdError] = useState<string | null>(null)

    const subTypeConfig = APP_SUB_TYPES[form.subType]

    // Determine if this is M2M (API scopes) or user-interactive (OIDC scopes)
    const isM2M = form.subType === "m2m"
    const availableScopes = useMemo(() => getScopesForSubType(form.subType), [form.subType])
    const infoMessage = useMemo(() => getInfoMessage(form.subType), [form.subType])

    const handleStartEditClientId = () => {
        setManualClientId(form.clientId)
        setClientIdError(null)
        setEditingClientId(true)
    }

    const handleConfirmClientId = () => {
        const result = validateClientId(manualClientId)
        if (!result.valid) {
            setClientIdError(result.error || "Invalid")
            return
        }
        onChange({ clientId: manualClientId })
        setEditingClientId(false)
        setClientIdError(null)
    }

    const handleCancelEditClientId = () => {
        setEditingClientId(false)
        setClientIdError(null)
    }

    const [customScopeInput, setCustomScopeInput] = useState("")

    const toggleScope = (scopeId: string) => {
        const current = form.scopes
        if (current.includes(scopeId)) {
            onChange({ scopes: current.filter(s => s !== scopeId) })
        } else {
            onChange({ scopes: [...current, scopeId] })
        }
    }

    // For user scopes, use PREDEFINED_SCOPES; for API scopes, use API_SCOPES_EXAMPLES
    const predefinedIds = availableScopes.map(s => s.id)
    const customScopes = form.scopes.filter(s => !predefinedIds.includes(s))

    const handleAddCustomScope = () => {
        const trimmed = customScopeInput.trim().toLowerCase().replace(/\s+/g, "_")
        if (!trimmed) return
        if (form.scopes.includes(trimmed)) {
            setCustomScopeInput("")
            return
        }
        onChange({ scopes: [...form.scopes, trimmed] })
        setCustomScopeInput("")
    }

    const handleRemoveCustomScope = (scopeId: string) => {
        onChange({ scopes: form.scopes.filter(s => s !== scopeId) })
    }

    // Get scope label/description helper text based on type
    const getScopeHelperText = () => {
        if (isM2M) {
            return "Define los permisos que este servicio tendra sobre tus APIs. Ejemplo: read:users, write:data"
        }
        return "Permisos que este cliente puede solicitar. openid: identidad basica. profile: nombre y avatar. email: direccion de email."
    }

    const getScopeSectionTitle = () => {
        if (isM2M) {
            return "Scopes de API"
        }
        return "Scopes"
    }

    return (
        <div className="space-y-6">
            {/* Contextual info message */}
            <div className={cn(
                "p-3 rounded-lg border flex items-start gap-3",
                isM2M
                    ? "bg-amber-500/5 border-amber-500/20"
                    : "bg-primary/5 border-primary/20"
            )}>
                <Info className={cn(
                    "h-4 w-4 mt-0.5 shrink-0",
                    isM2M ? "text-amber-500" : "text-primary"
                )} />
                <div className="space-y-1">
                    <p className={cn(
                        "text-sm font-medium",
                        isM2M ? "text-amber-600 dark:text-amber-400" : "text-primary"
                    )}>
                        {subTypeConfig.label}
                    </p>
                    <p className="text-xs text-muted-foreground">
                        {infoMessage}
                    </p>
                </div>
            </div>

            {/* Name */}
            <div className="space-y-2">
                <Label className="flex items-center gap-1">
                    Nombre del cliente
                    <span className="text-danger">*</span>
                    <InfoTooltip
                        title="Nombre del cliente"
                        content="Un nombre descriptivo para identificar esta aplicacion en el dashboard."
                        example="Mi App Web, Portal Admin"
                    />
                </Label>
                <Input
                    value={form.name}
                    onChange={(e) => onChange({ name: e.target.value })}
                    placeholder={isM2M ? "Mi Servicio Backend" : "Mi Aplicacion Web"}
                    className="text-base"
                    autoFocus
                />
                {!form.name && (
                    <p className="text-xs text-muted-foreground">
                        El nombre generara automaticamente el Client ID.
                    </p>
                )}
            </div>

            {/* Description (optional) */}
            <div className="space-y-2">
                <Label className="flex items-center gap-1">
                    Descripcion
                    <span className="text-xs text-muted-foreground font-normal">(opcional)</span>
                </Label>
                <Textarea
                    value={form.description}
                    onChange={(e) => onChange({ description: e.target.value })}
                    placeholder={isM2M
                        ? "Servicio que sincroniza datos con el CRM..."
                        : "Breve descripcion del proposito de este cliente..."
                    }
                    rows={2}
                    className="resize-none"
                />
            </div>

            {/* Client ID */}
            <div className="space-y-2">
                <Label className="flex items-center gap-1">
                    Client ID
                    <InfoTooltip
                        title="Client ID"
                        content="Identificador unico publico. Se genera automaticamente pero puedes editarlo."
                        example="acme_mi-app-web_a3f1"
                    />
                </Label>

                {editingClientId ? (
                    <div className="space-y-1.5">
                        <div className="flex items-center gap-2">
                            <Input
                                value={manualClientId}
                                onChange={(e) => {
                                    setManualClientId(e.target.value.toLowerCase())
                                    setClientIdError(null)
                                }}
                                placeholder="mi-app-web"
                                className="font-mono text-sm"
                                autoFocus
                                onKeyDown={(e) => {
                                    if (e.key === "Enter") { e.preventDefault(); handleConfirmClientId() }
                                    if (e.key === "Escape") handleCancelEditClientId()
                                }}
                            />
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={handleConfirmClientId}
                                className="shrink-0"
                            >
                                <Check className="h-4 w-4" />
                            </Button>
                            <Button
                                variant="ghost"
                                size="sm"
                                onClick={handleCancelEditClientId}
                                className="shrink-0"
                            >
                                <X className="h-4 w-4" />
                            </Button>
                        </div>
                        {clientIdError && (
                            <p className="text-xs text-danger">{clientIdError}</p>
                        )}
                        <p className="text-xs text-muted-foreground">
                            Solo letras minusculas, numeros, guiones y underscores (3-64 caracteres).
                        </p>
                    </div>
                ) : (
                    <div className="flex items-center gap-2">
                        <div className="flex-1 rounded-lg bg-muted/60 border px-4 py-2.5 font-mono text-sm text-foreground/80 truncate">
                            {form.clientId || (
                                <span className="text-muted-foreground italic">
                                    Escribe un nombre para generar...
                                </span>
                            )}
                        </div>
                        {form.clientId && (
                            <>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={handleStartEditClientId}
                                    title="Editar manualmente"
                                    className="shrink-0"
                                >
                                    <Pencil className="h-3.5 w-3.5" />
                                </Button>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={onRegenerateId}
                                    title="Regenerar"
                                    className="shrink-0"
                                >
                                    <RotateCcw className="h-3.5 w-3.5" />
                                </Button>
                            </>
                        )}
                    </div>
                )}
            </div>

            {/* Scopes as chips - dynamic based on type */}
            <div className="space-y-3">
                <Label className="flex items-center gap-1">
                    {getScopeSectionTitle()}
                    <InfoTooltip
                        title={isM2M ? "Scopes de API" : "Scopes (Permisos)"}
                        content={getScopeHelperText()}
                    />
                </Label>

                {/* M2M info banner */}
                {isM2M && (
                    <div className="p-2.5 rounded-md bg-muted/50 border border-border/50 flex items-start gap-2">
                        <AlertCircle className="h-4 w-4 text-muted-foreground mt-0.5 shrink-0" />
                        <p className="text-xs text-muted-foreground">
                            Los scopes de M2M definen que recursos puede acceder este servicio.
                            Puedes usar los ejemplos o agregar tus propios scopes personalizados.
                        </p>
                    </div>
                )}

                <TooltipProvider delayDuration={300}>
                    <div className="flex flex-wrap gap-2">
                        {availableScopes.map((scope) => {
                            const selected = form.scopes.includes(scope.id)
                            return (
                                <Tooltip key={scope.id}>
                                    <TooltipTrigger asChild>
                                        <button
                                            type="button"
                                            onClick={() => toggleScope(scope.id)}
                                            className={cn(
                                                "inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border text-sm transition-all duration-200",
                                                selected
                                                    ? isM2M
                                                        ? "bg-amber-500/10 border-amber-500/30 text-amber-600 dark:text-amber-400 font-medium"
                                                        : "bg-primary/10 border-primary/30 text-primary font-medium"
                                                    : "bg-muted/40 border-border text-muted-foreground hover:border-primary/20 hover:text-foreground"
                                            )}
                                        >
                                            {selected && <Check className="h-3 w-3" />}
                                            <code className="text-xs">{scope.label}</code>
                                        </button>
                                    </TooltipTrigger>
                                    <TooltipContent side="bottom" className="text-xs max-w-[200px]">
                                        {scope.description}
                                    </TooltipContent>
                                </Tooltip>
                            )
                        })}
                        {/* Custom scopes */}
                        {customScopes.map((scopeId) => (
                            <button
                                key={scopeId}
                                type="button"
                                onClick={() => handleRemoveCustomScope(scopeId)}
                                className={cn(
                                    "inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border text-sm transition-all duration-200 font-medium",
                                    isM2M
                                        ? "bg-amber-500/10 border-amber-500/30 text-amber-600 dark:text-amber-400"
                                        : "bg-accent/10 border-accent/30 text-accent"
                                )}
                            >
                                <code className="text-xs">{scopeId}</code>
                                <X className="h-3 w-3" />
                            </button>
                        ))}
                    </div>
                </TooltipProvider>

                {/* Custom scope input */}
                <div className="flex items-center gap-2">
                    <Input
                        value={customScopeInput}
                        onChange={(e) => setCustomScopeInput(e.target.value)}
                        placeholder={isM2M ? M2M_SCOPE_PLACEHOLDER : "Agregar scope personalizado..."}
                        className="text-xs h-8 max-w-[280px]"
                        onKeyDown={(e) => {
                            if (e.key === "Enter") { e.preventDefault(); handleAddCustomScope() }
                        }}
                    />
                    <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={handleAddCustomScope}
                        disabled={!customScopeInput.trim()}
                        className="h-8 text-xs"
                    >
                        <Plus className="h-3 w-3 mr-1" />
                        Agregar
                    </Button>
                </div>

                <p className="text-xs text-muted-foreground flex items-center gap-1">
                    <Info className="h-3 w-3" />
                    {form.scopes.length} scope{form.scopes.length !== 1 ? "s" : ""} seleccionado{form.scopes.length !== 1 ? "s" : ""}
                    {customScopes.length > 0 && ` (${customScopes.length} personalizado${customScopes.length !== 1 ? "s" : ""})`}
                </p>
            </div>

            {/* Type indicator - enhanced with more context */}
            <div className={cn(
                "p-3 rounded-lg border flex items-center gap-3",
                isM2M
                    ? "bg-amber-500/5 border-amber-500/20"
                    : subTypeConfig.type === "public"
                        ? "bg-emerald-500/5 border-emerald-500/20"
                        : "bg-muted/40 border-border/50"
            )}>
                <Badge
                    variant={subTypeConfig.type === "public" ? "success" : "default"}
                    className="shrink-0"
                >
                    {subTypeConfig.type === "public" ? "Publico" : "Confidencial"}
                </Badge>
                <span className="text-xs text-muted-foreground">
                    {isM2M
                        ? "Recibira un client_secret. El secret tiene acceso total a los scopes - protegelo bien."
                        : subTypeConfig.type === "public"
                            ? "Sin secreto. Usa PKCE para autenticacion segura."
                            : "Recibira un client_secret que deberas guardar de forma segura."}
                </span>
            </div>
        </div>
    )
}
