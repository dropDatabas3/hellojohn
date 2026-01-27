"use client"

import { useState, useCallback, useMemo } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import {
    Check,
    ChevronRight,
    ChevronDown,
    Building2,
    Database,
    Mail,
    Globe,
    Zap,
    Sparkles,
    AlertCircle
} from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
    Dialog,
    DialogContent,
    DialogTitle,
} from "@/components/ui/dialog"
import { useToast } from "@/hooks/use-toast"
import { Switch } from "@/components/ui/switch"
import { cn } from "@/lib/utils"
import { LogoDropzone } from "@/components/tenant/LogoDropzone"
import type { Tenant } from "@/lib/types"

interface CreateTenantWizardProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    onSuccess?: (tenant: Tenant) => void
}

// Step configuration
const STEPS = [
    { id: 0, title: "Nueva Organización", description: "Configura los detalles de tu nueva organización" },
    { id: 1, title: "Infraestructura", description: "Configura las opciones de almacenamiento y servicios" },
    { id: 2, title: "Revisa tu configuración", description: "Confirma los detalles antes de crear la organización" },
]

type SectionId = "db" | "cache" | "smtp" | "social" | null

// Expandable Section Component
function ExpandableSection({
    id,
    title,
    description,
    icon: Icon,
    enabled,
    onToggle,
    children,
    isExpanded,
    onExpand,
    status
}: {
    id: SectionId
    title: string
    description: string
    icon: React.ElementType
    enabled: boolean
    onToggle: (enabled: boolean) => void
    children: React.ReactNode
    isExpanded: boolean
    onExpand: (id: SectionId) => void
    status: "idle" | "complete" | "incomplete"
}) {
    const handleToggle = (checked: boolean) => {
        onToggle(checked)
        if (checked) onExpand(id)
    }

    const handleClick = () => {
        if (enabled) {
            onExpand(isExpanded ? null : id)
        }
    }

    return (
        <div className={cn(
            "rounded-2xl border-2 transition-all duration-300",
            enabled
                ? status === "complete"
                    ? "border-emerald-500/50 bg-emerald-500/[0.03]"
                    : status === "incomplete"
                        ? "border-amber-500/50 bg-amber-500/[0.03]"
                        : "border-foreground/20 bg-foreground/[0.02]"
                : "border-border/50"
        )}>
            <div
                className="flex items-center justify-between p-5 cursor-pointer select-none"
                onClick={handleClick}
            >
                <div className="flex items-center gap-4">
                    <div className={cn(
                        "w-10 h-10 rounded-xl flex items-center justify-center transition-colors duration-300",
                        enabled
                            ? status === "complete"
                                ? "bg-emerald-500 text-white"
                                : status === "incomplete"
                                    ? "bg-amber-500 text-white"
                                    : "bg-foreground text-background"
                            : "bg-muted text-muted-foreground"
                    )}>
                        {enabled && status === "complete" ? (
                            <Check className="h-5 w-5" />
                        ) : enabled && status === "incomplete" ? (
                            <AlertCircle className="h-5 w-5" />
                        ) : (
                            <Icon className="h-5 w-5" />
                        )}
                    </div>
                    <div>
                        <div className="flex items-center gap-2">
                            <h4 className="font-medium text-[15px]">{title}</h4>
                            {enabled && !isExpanded && (
                                <span className={cn(
                                    "text-xs px-2 py-0.5 rounded-full",
                                    status === "complete"
                                        ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                                        : status === "incomplete"
                                            ? "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
                                            : ""
                                )}>
                                    {status === "complete" ? "Configurado" : status === "incomplete" ? "Incompleto" : ""}
                                </span>
                            )}
                        </div>
                        <p className="text-sm text-muted-foreground">{description}</p>
                    </div>
                </div>
                <div className="flex items-center gap-3">
                    {enabled && (
                        <ChevronDown className={cn(
                            "h-4 w-4 text-muted-foreground transition-transform duration-200",
                            isExpanded && "rotate-180"
                        )} />
                    )}
                    <Switch
                        checked={enabled}
                        onCheckedChange={handleToggle}
                        onClick={(e) => e.stopPropagation()}
                    />
                </div>
            </div>

            {enabled && isExpanded && (
                <div className="px-5 pb-5 pt-0 animate-in fade-in slide-in-from-top-2 duration-200">
                    <div className="pt-4 border-t border-border/50">
                        {children}
                    </div>
                </div>
            )}
        </div>
    )
}

export function CreateTenantWizard({ open, onOpenChange, onSuccess }: CreateTenantWizardProps) {
    const { t } = useI18n()
    const { toast } = useToast()
    const queryClient = useQueryClient()

    // Wizard state
    const [currentStep, setCurrentStep] = useState(0)
    const [expandedSection, setExpandedSection] = useState<SectionId>(null)

    // Form state
    const [formData, setFormData] = useState({
        name: "",
        slug: "",
        displayName: "",
        logoUrl: "",
    })

    // Infrastructure toggles
    const [enableUserDB, setEnableUserDB] = useState(false)
    const [enableCache, setEnableCache] = useState(false)
    const [enableSMTP, setEnableSMTP] = useState(false)
    const [enableSocial, setEnableSocial] = useState(false)

    // DB Configuration
    const [dbConfig, setDbConfig] = useState({ dsn: "" })
    const [connectionStatus, setConnectionStatus] = useState<"idle" | "testing" | "success" | "error">("idle")

    // Cache Configuration
    const [cacheConfig, setCacheConfig] = useState({
        driver: "memory" as "memory" | "redis",
        host: "",
        port: 6379,
        password: "",
    })

    // SMTP Configuration
    const [smtpConfig, setSmtpConfig] = useState({
        host: "",
        port: 587,
        username: "",
        password: "",
        fromEmail: "",
    })

    // Social Configuration
    const [socialConfig, setSocialConfig] = useState({
        googleClientId: "",
        googleClientSecret: "",
    })

    // Validation functions
    const getDbStatus = useCallback((): "idle" | "complete" | "incomplete" => {
        if (!enableUserDB) return "idle"
        if (dbConfig.dsn.length > 10) return "complete"
        return "incomplete"
    }, [enableUserDB, dbConfig.dsn])

    const getCacheStatus = useCallback((): "idle" | "complete" | "incomplete" => {
        if (!enableCache) return "idle"
        if (cacheConfig.driver === "memory") return "complete"
        if (cacheConfig.driver === "redis" && cacheConfig.host) return "complete"
        return "incomplete"
    }, [enableCache, cacheConfig])

    const getSmtpStatus = useCallback((): "idle" | "complete" | "incomplete" => {
        if (!enableSMTP) return "idle"
        const { host, port, fromEmail } = smtpConfig
        if (host && port && fromEmail) return "complete"
        if (host || fromEmail) return "incomplete"
        return "incomplete"
    }, [enableSMTP, smtpConfig])

    const getSocialStatus = useCallback((): "idle" | "complete" | "incomplete" => {
        if (!enableSocial) return "idle"
        const { googleClientId, googleClientSecret } = socialConfig
        if (googleClientId && googleClientSecret) return "complete"
        if (googleClientId || googleClientSecret) return "incomplete"
        return "incomplete"
    }, [enableSocial, socialConfig])

    // Check if Step 1 (Infrastructure) has any incomplete sections
    const hasIncompleteInfrastructure = useMemo(() => {
        const statuses = [getDbStatus(), getCacheStatus(), getSmtpStatus(), getSocialStatus()]
        return statuses.some(s => s === "incomplete")
    }, [getDbStatus, getCacheStatus, getSmtpStatus, getSocialStatus])

    const getIncompleteMessage = () => {
        const incomplete: string[] = []
        if (getDbStatus() === "incomplete") incomplete.push("Base de Datos")
        if (getCacheStatus() === "incomplete") incomplete.push("Cache")
        if (getSmtpStatus() === "incomplete") incomplete.push("SMTP")
        if (getSocialStatus() === "incomplete") incomplete.push("Login Social")
        return incomplete
    }

    // Reset wizard
    const resetWizard = useCallback(() => {
        setCurrentStep(0)
        setExpandedSection(null)
        setFormData({ name: "", slug: "", displayName: "", logoUrl: "" })
        setEnableUserDB(false)
        setEnableCache(false)
        setEnableSMTP(false)
        setEnableSocial(false)
        setDbConfig({ dsn: "" })
        setCacheConfig({ driver: "memory", host: "", port: 6379, password: "" })
        setSmtpConfig({ host: "", port: 587, username: "", password: "", fromEmail: "" })
        setSocialConfig({ googleClientId: "", googleClientSecret: "" })
        setConnectionStatus("idle")
    }, [])

    const handleOpenChange = (isOpen: boolean) => {
        onOpenChange(isOpen)
        if (!isOpen) setTimeout(resetWizard, 300)
    }

    // Mutations
    const createMutation = useMutation({
        mutationFn: (data: any) => api.post<Tenant & { bootstrap_error?: string }>("/v2/admin/tenants", data),
        onSuccess: (response) => {
            queryClient.invalidateQueries({ queryKey: ["tenants"] })
            handleOpenChange(false)

            if (response.bootstrap_error) {
                toast({
                    title: "Organización creada con advertencia",
                    description: response.bootstrap_error,
                    variant: "destructive"
                })
            } else {
                toast({ title: "Organización creada", description: "La configuración se guardó correctamente." })
            }

            onSuccess?.(response)
        },
        onError: (error: any) => {
            toast({ title: "Error", description: error.message, variant: "destructive" })
        },
    })

    // Connection error message state
    const [connectionError, setConnectionError] = useState("")

    const testConnectionMutation = useMutation({
        mutationFn: (dsn: string) => api.post("/v2/admin/tenants/test-connection", { dsn }),
        onSuccess: () => {
            setConnectionStatus("success")
            setConnectionError("")
        },
        onError: (error: any) => {
            setConnectionStatus("error")
            // Try to get the detail from the API response
            const detail = error?.detail || error?.message || "No se pudo conectar al servidor"
            setConnectionError(detail)
        },
    })

    // Validation
    const isStep0Valid = formData.name.length >= 2 && /^[a-z0-9-]+$/.test(formData.slug)

    // Handle next step with validation
    const handleNext = () => {
        if (currentStep === 1 && hasIncompleteInfrastructure) {
            const incomplete = getIncompleteMessage()
            toast({
                title: "Configuración incompleta",
                description: `Completa o desactiva: ${incomplete.join(", ")}`,
                variant: "destructive"
            })
            return
        }
        setCurrentStep(prev => prev + 1)
        setExpandedSection(null)
    }

    const canProceed = currentStep === 0 ? isStep0Valid : true

    // Handle Create
    const handleCreate = () => {
        const payload = {
            name: formData.name,
            slug: formData.slug,
            display_name: formData.displayName,
            settings: {
                logoUrl: formData.logoUrl || undefined,
                socialLoginEnabled: enableSocial,
                userDb: enableUserDB ? { dsn: dbConfig.dsn, driver: "postgres" } : undefined,
                cache: enableCache ? {
                    driver: cacheConfig.driver,
                    enabled: true,
                    ...(cacheConfig.driver === "redis" && {
                        host: cacheConfig.host,
                        port: cacheConfig.port,
                        password: cacheConfig.password,
                    })
                } : undefined,
                smtp: enableSMTP ? smtpConfig : undefined,
                socialProviders: enableSocial ? {
                    googleEnabled: !!socialConfig.googleClientId,
                    googleClient: socialConfig.googleClientId,
                    googleSecretEnc: socialConfig.googleClientSecret,
                } : undefined,
            }
        }
        createMutation.mutate(payload)
    }

    // Auto-generate slug from name
    const handleNameChange = (name: string) => {
        const newSlug = formData.slug || name.toLowerCase().replaceAll(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
        setFormData(prev => ({ ...prev, name, slug: prev.slug ? prev.slug : newSlug }))
    }

    // Current step info
    const currentStepInfo = STEPS[currentStep]

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent className="max-w-2xl p-0 gap-0 overflow-hidden bg-background border-border/50 shadow-2xl">
                {/* Accessible Title (visually hidden) */}
                <DialogTitle className="sr-only">Crear Organización</DialogTitle>

                {/* Header with Stepper */}
                <div className="px-8 pt-8 pb-6">
                    {/* Dynamic Title */}
                    <h2 className="text-2xl font-semibold tracking-tight mb-1">{currentStepInfo.title}</h2>
                    <p className="text-muted-foreground text-sm">{currentStepInfo.description}</p>

                    {/* Minimal Stepper */}
                    <div className="flex items-center gap-3 mt-6">
                        {STEPS.map((step, index) => (
                            <div key={step.id} className="flex items-center gap-3">
                                <button
                                    onClick={() => index < currentStep && setCurrentStep(index)}
                                    disabled={index > currentStep}
                                    className={cn(
                                        "flex items-center gap-2 px-3 py-1.5 rounded-full text-sm font-medium transition-all duration-300",
                                        currentStep === index
                                            ? "bg-foreground text-background"
                                            : currentStep > index
                                                ? "bg-foreground/10 text-foreground cursor-pointer hover:bg-foreground/15"
                                                : "text-muted-foreground/50"
                                    )}
                                >
                                    {currentStep > index ? (
                                        <Check className="h-3.5 w-3.5" />
                                    ) : (
                                        <span className="w-5 h-5 rounded-full border-2 border-current flex items-center justify-center text-xs">
                                            {index + 1}
                                        </span>
                                    )}
                                    <span className="hidden sm:inline">{index === 0 ? "Identidad" : index === 1 ? "Infra" : "Revisar"}</span>
                                </button>
                                {index < STEPS.length - 1 && (
                                    <ChevronRight className="h-4 w-4 text-muted-foreground/30" />
                                )}
                            </div>
                        ))}
                    </div>
                </div>

                {/* Content Area with custom scrollbar */}
                <div className="px-8 pb-6 min-h-[380px] max-h-[50vh] overflow-y-auto scrollbar-thin scrollbar-track-transparent scrollbar-thumb-muted-foreground/20 hover:scrollbar-thumb-muted-foreground/30"
                    style={{
                        scrollbarWidth: 'thin',
                        scrollbarColor: 'hsl(var(--muted-foreground) / 0.2) transparent'
                    }}
                >

                    {/* Step 0: Identity */}
                    {currentStep === 0 && (
                        <div className="space-y-6 animate-in fade-in slide-in-from-right-4 duration-300">
                            <div className="grid grid-cols-2 gap-5">
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">
                                        Nombre <span className="text-red-500">*</span>
                                    </label>
                                    <Input
                                        value={formData.name}
                                        onChange={(e) => handleNameChange(e.target.value)}
                                        placeholder="Mi Organización"
                                        className="h-12 text-base bg-muted/30 border-0 focus-visible:ring-2 focus-visible:ring-foreground/20"
                                        autoFocus
                                    />
                                </div>
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">
                                        Identificador <span className="text-red-500">*</span>
                                    </label>
                                    <div className="relative">
                                        <Input
                                            value={formData.slug}
                                            onChange={(e) => setFormData(prev => ({ ...prev, slug: e.target.value.toLowerCase() }))}
                                            placeholder="mi-organizacion"
                                            className={cn(
                                                "h-12 text-base font-mono bg-muted/30 border-0 focus-visible:ring-2 focus-visible:ring-foreground/20 pr-10",
                                                formData.slug && (/^[a-z0-9-]+$/.test(formData.slug) ? "" : "ring-2 ring-red-500/50")
                                            )}
                                        />
                                        {formData.slug && (
                                            <div className="absolute right-3 top-1/2 -translate-y-1/2">
                                                {/^[a-z0-9-]+$/.test(formData.slug) ? (
                                                    <Check className="h-4 w-4 text-emerald-500" />
                                                ) : (
                                                    <span className="text-xs text-red-500">inválido</span>
                                                )}
                                            </div>
                                        )}
                                    </div>
                                    <p className="text-xs text-muted-foreground">Solo minúsculas, números y guiones</p>
                                </div>
                            </div>

                            <div className="space-y-2">
                                <label className="text-sm font-medium">Nombre para mostrar</label>
                                <Input
                                    value={formData.displayName}
                                    onChange={(e) => setFormData(prev => ({ ...prev, displayName: e.target.value }))}
                                    placeholder="Mi Organización S.A. (opcional)"
                                    className="h-12 text-base bg-muted/30 border-0 focus-visible:ring-2 focus-visible:ring-foreground/20"
                                />
                            </div>

                            <div className="space-y-2">
                                <label className="text-sm font-medium">Logo</label>
                                <LogoDropzone
                                    value={formData.logoUrl}
                                    onChange={(url) => setFormData(prev => ({ ...prev, logoUrl: url || "" }))}
                                />
                            </div>
                        </div>
                    )}

                    {/* Step 1: Infrastructure */}
                    {currentStep === 1 && (
                        <div className="space-y-3 animate-in fade-in slide-in-from-right-4 duration-300">

                            {/* Database */}
                            <ExpandableSection
                                id="db"
                                title="Base de Datos"
                                description="Almacenamiento de usuarios dedicado"
                                icon={Database}
                                enabled={enableUserDB}
                                onToggle={setEnableUserDB}
                                isExpanded={expandedSection === "db"}
                                onExpand={setExpandedSection}
                                status={getDbStatus()}
                            >
                                <div className="space-y-4">
                                    <div className="space-y-2">
                                        <label className="text-sm font-medium">PostgreSQL DSN</label>
                                        <Input
                                            value={dbConfig.dsn}
                                            onChange={(e) => {
                                                setDbConfig({ dsn: e.target.value })
                                                setConnectionStatus("idle")
                                            }}
                                            type="password"
                                            placeholder="postgres://user:pass@host:5432/database"
                                            className="h-11 font-mono text-sm bg-muted/30 border-0"
                                        />
                                    </div>
                                    <div className="flex items-center gap-3">
                                        <Button
                                            size="sm"
                                            variant="outline"
                                            onClick={() => {
                                                setConnectionStatus("testing")
                                                testConnectionMutation.mutate(dbConfig.dsn)
                                            }}
                                            disabled={!dbConfig.dsn || testConnectionMutation.isPending}
                                            className="h-9"
                                        >
                                            {testConnectionMutation.isPending ? (
                                                <div className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent mr-2" />
                                            ) : (
                                                <Database className="h-4 w-4 mr-2" />
                                            )}
                                            Probar conexión
                                        </Button>
                                        {connectionStatus === "success" && (
                                            <span className="text-sm text-emerald-600 flex items-center gap-1">
                                                <Check className="h-4 w-4" /> Conectado
                                            </span>
                                        )}
                                        {connectionStatus === "error" && (
                                            <span className="text-sm text-red-500 max-w-xs">
                                                {connectionError || "Error de conexión"}
                                            </span>
                                        )}
                                    </div>
                                </div>
                            </ExpandableSection>

                            {/* Cache */}
                            <ExpandableSection
                                id="cache"
                                title="Cache"
                                description="Mejora el rendimiento de sesiones"
                                icon={Zap}
                                enabled={enableCache}
                                onToggle={setEnableCache}
                                isExpanded={expandedSection === "cache"}
                                onExpand={setExpandedSection}
                                status={getCacheStatus()}
                            >
                                <div className="space-y-4">
                                    <div className="flex gap-2">
                                        <Button
                                            size="sm"
                                            variant={cacheConfig.driver === "memory" ? "default" : "outline"}
                                            onClick={() => setCacheConfig(prev => ({ ...prev, driver: "memory" }))}
                                            className="h-9"
                                        >
                                            In-Memory
                                        </Button>
                                        <Button
                                            size="sm"
                                            variant={cacheConfig.driver === "redis" ? "default" : "outline"}
                                            onClick={() => setCacheConfig(prev => ({ ...prev, driver: "redis" }))}
                                            className="h-9"
                                        >
                                            Redis
                                        </Button>
                                    </div>

                                    {cacheConfig.driver === "memory" && (
                                        <p className="text-sm text-amber-600 bg-amber-50 dark:bg-amber-950/30 p-3 rounded-lg">
                                            ⚠️ Los datos se perderán al reiniciar el servidor
                                        </p>
                                    )}

                                    {cacheConfig.driver === "redis" && (
                                        <div className="grid grid-cols-2 gap-3">
                                            <Input
                                                value={cacheConfig.host}
                                                onChange={(e) => setCacheConfig(prev => ({ ...prev, host: e.target.value }))}
                                                placeholder="Host *"
                                                className="h-10 bg-muted/30 border-0"
                                            />
                                            <Input
                                                type="number"
                                                value={cacheConfig.port}
                                                onChange={(e) => setCacheConfig(prev => ({ ...prev, port: Number.parseInt(e.target.value) }))}
                                                placeholder="6379"
                                                className="h-10 bg-muted/30 border-0"
                                            />
                                            <Input
                                                type="password"
                                                value={cacheConfig.password}
                                                onChange={(e) => setCacheConfig(prev => ({ ...prev, password: e.target.value }))}
                                                placeholder="Contraseña (opcional)"
                                                className="h-10 bg-muted/30 border-0 col-span-2"
                                            />
                                        </div>
                                    )}
                                </div>
                            </ExpandableSection>

                            {/* SMTP */}
                            <ExpandableSection
                                id="smtp"
                                title="Email (SMTP)"
                                description="Configura tu servidor de correo"
                                icon={Mail}
                                enabled={enableSMTP}
                                onToggle={setEnableSMTP}
                                isExpanded={expandedSection === "smtp"}
                                onExpand={setExpandedSection}
                                status={getSmtpStatus()}
                            >
                                <div className="grid grid-cols-2 gap-3">
                                    <Input
                                        value={smtpConfig.host}
                                        onChange={(e) => setSmtpConfig(prev => ({ ...prev, host: e.target.value }))}
                                        placeholder="smtp.ejemplo.com *"
                                        className="h-10 bg-muted/30 border-0"
                                    />
                                    <Input
                                        type="number"
                                        value={smtpConfig.port}
                                        onChange={(e) => setSmtpConfig(prev => ({ ...prev, port: Number.parseInt(e.target.value) }))}
                                        placeholder="587"
                                        className="h-10 bg-muted/30 border-0"
                                    />
                                    <Input
                                        value={smtpConfig.username}
                                        onChange={(e) => setSmtpConfig(prev => ({ ...prev, username: e.target.value }))}
                                        placeholder="Usuario"
                                        className="h-10 bg-muted/30 border-0"
                                    />
                                    <Input
                                        type="password"
                                        value={smtpConfig.password}
                                        onChange={(e) => setSmtpConfig(prev => ({ ...prev, password: e.target.value }))}
                                        placeholder="Contraseña"
                                        className="h-10 bg-muted/30 border-0"
                                    />
                                    <Input
                                        value={smtpConfig.fromEmail}
                                        onChange={(e) => setSmtpConfig(prev => ({ ...prev, fromEmail: e.target.value }))}
                                        placeholder="noreply@ejemplo.com *"
                                        className="h-10 bg-muted/30 border-0 col-span-2"
                                    />
                                </div>
                            </ExpandableSection>

                            {/* Social Login */}
                            <ExpandableSection
                                id="social"
                                title="Login Social"
                                description="Autenticación con Google"
                                icon={Globe}
                                enabled={enableSocial}
                                onToggle={setEnableSocial}
                                isExpanded={expandedSection === "social"}
                                onExpand={setExpandedSection}
                                status={getSocialStatus()}
                            >
                                <div className="space-y-3">
                                    <Input
                                        value={socialConfig.googleClientId}
                                        onChange={(e) => setSocialConfig(prev => ({ ...prev, googleClientId: e.target.value }))}
                                        placeholder="Google Client ID *"
                                        className="h-10 bg-muted/30 border-0"
                                    />
                                    <Input
                                        type="password"
                                        value={socialConfig.googleClientSecret}
                                        onChange={(e) => setSocialConfig(prev => ({ ...prev, googleClientSecret: e.target.value }))}
                                        placeholder="Google Client Secret *"
                                        className="h-10 bg-muted/30 border-0"
                                    />
                                </div>
                            </ExpandableSection>
                        </div>
                    )}

                    {/* Step 2: Review */}
                    {currentStep === 2 && (
                        <div className="animate-in fade-in slide-in-from-right-4 duration-300">
                            <div className="space-y-3 max-w-lg mx-auto">
                                <div className="flex justify-between py-3 border-b border-border/50">
                                    <span className="text-muted-foreground">Nombre</span>
                                    <span className="font-medium">{formData.name}</span>
                                </div>
                                <div className="flex justify-between py-3 border-b border-border/50">
                                    <span className="text-muted-foreground">Identificador</span>
                                    <code className="font-mono text-sm bg-muted px-2 py-0.5 rounded">{formData.slug}</code>
                                </div>

                                <div className="flex justify-between py-3 border-b border-border/50">
                                    <span className="text-muted-foreground">Base de Datos</span>
                                    <span className={cn(
                                        enableUserDB ? "text-emerald-600" : "text-muted-foreground"
                                    )}>
                                        {enableUserDB ? "✓ Configurada" : "—"}
                                    </span>
                                </div>
                                <div className="flex justify-between py-3 border-b border-border/50">
                                    <span className="text-muted-foreground">Cache</span>
                                    <span className={cn(
                                        enableCache ? "text-emerald-600" : "text-muted-foreground"
                                    )}>
                                        {enableCache ? `✓ ${cacheConfig.driver === "redis" ? "Redis" : "Memory"}` : "—"}
                                    </span>
                                </div>
                                <div className="flex justify-between py-3 border-b border-border/50">
                                    <span className="text-muted-foreground">SMTP</span>
                                    <span className={cn(
                                        enableSMTP ? "text-emerald-600" : "text-muted-foreground"
                                    )}>
                                        {enableSMTP ? `✓ ${smtpConfig.host}` : "—"}
                                    </span>
                                </div>
                                <div className="flex justify-between py-3">
                                    <span className="text-muted-foreground">Login Social</span>
                                    <span className={cn(
                                        enableSocial ? "text-emerald-600" : "text-muted-foreground"
                                    )}>
                                        {enableSocial ? "✓ Google" : "—"}
                                    </span>
                                </div>
                            </div>
                        </div>
                    )}
                </div>

                {/* Footer */}
                <div className="px-8 py-5 border-t border-border/50 bg-muted/30 flex justify-between items-center">
                    <Button
                        variant="ghost"
                        onClick={() => currentStep === 0 ? handleOpenChange(false) : setCurrentStep(prev => prev - 1)}
                        className="text-muted-foreground hover:text-foreground"
                    >
                        {currentStep === 0 ? "Cancelar" : "Atrás"}
                    </Button>

                    {currentStep < 2 ? (
                        <Button
                            onClick={handleNext}
                            disabled={!canProceed}
                            className="h-11 px-8 gap-2 bg-foreground text-background hover:bg-foreground/90"
                        >
                            Continuar
                            <ChevronRight className="h-4 w-4" />
                        </Button>
                    ) : (
                        <Button
                            onClick={handleCreate}
                            disabled={createMutation.isPending}
                            className="h-11 px-8 gap-2 bg-foreground text-background hover:bg-foreground/90"
                        >
                            {createMutation.isPending ? (
                                <>
                                    <div className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                                    Creando...
                                </>
                            ) : (
                                <>
                                    Crear Organización
                                    <Check className="h-4 w-4" />
                                </>
                            )}
                        </Button>
                    )}
                </div>
            </DialogContent>
        </Dialog>
    )
}
