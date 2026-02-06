"use client"

import { useState } from "react"
import {
    Copy,
    Check,
    Eye,
    EyeOff,
    AlertTriangle,
    ExternalLink,
    Terminal,
    Key,
    CheckCircle2,
    Rocket,
    Lock,
    Globe,
    Server,
    Cpu,
    Smartphone,
    X,
} from "lucide-react"
import {
    Button,
    Badge,
    Dialog,
    DialogContent,
    Checkbox,
    Label,
    Tabs,
    TabsContent,
    TabsList,
    TabsTrigger,
    cn,
} from "@/components/ds"
import { CodeSnippet } from "./CodeSnippet"
import { SUB_TYPE_DEFAULT_SDK, getSnippet, getNextSteps, getFilteredSdkTabs } from "./snippets"
import type { SnippetConfig } from "./snippets"
import type { ClientRow, AppSubType } from "@/components/clients/wizard"

// ============================================================================
// TYPE ICONS
// ============================================================================

const TYPE_ICONS: Record<AppSubType, React.ElementType> = {
    spa: Globe,
    mobile: Smartphone,
    api_server: Server,
    m2m: Cpu,
}

// ============================================================================
// PROPS
// ============================================================================

interface ClientQuickStartProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    client: ClientRow
    secret?: string | null
    tenantSlug: string
    subType?: AppSubType
    onViewDetails: () => void
}

// ============================================================================
// COPY HELPER
// ============================================================================

function useCopyFeedback() {
    const [copied, setCopied] = useState<string | null>(null)

    const copy = (value: string, key: string) => {
        navigator.clipboard.writeText(value)
        setCopied(key)
        setTimeout(() => setCopied(null), 2000)
    }

    return { copied, copy }
}

// ============================================================================
// MAIN COMPONENT
// ============================================================================

export function ClientQuickStart({
    open,
    onOpenChange,
    client,
    secret,
    tenantSlug,
    subType,
    onViewDetails,
}: ClientQuickStartProps) {
    const isConfidential = client.type === "confidential"
    const [secretConfirmed, setSecretConfirmed] = useState(false)
    const [showSecret, setShowSecret] = useState(false)
    const { copied, copy } = useCopyFeedback()

    // SDK tab: pre-select based on sub-type
    const defaultSdk = subType ? (SUB_TYPE_DEFAULT_SDK[subType] || "javascript") : "javascript"
    const [selectedSdk, setSelectedSdk] = useState(defaultSdk)

    // Build domain from current window
    const domain = typeof window !== "undefined"
        ? window.location.origin
        : "https://auth.example.com"

    const isM2M = subType === "m2m"
    const TypeIcon = subType ? TYPE_ICONS[subType] : Globe

    // Get filtered SDK tabs based on client type
    const availableSdkTabs = getFilteredSdkTabs(subType)

    const snippetConfig: SnippetConfig = {
        clientId: client.client_id,
        tenantSlug,
        domain,
        type: client.type,
        secret: secret || undefined,
        subType,
    }

    const canExit = !isConfidential || !secret || secretConfirmed
    const nextSteps = getNextSteps(selectedSdk, subType)

    const handleClose = () => {
        if (!canExit) return
        onOpenChange(false)
    }

    const handleViewDetails = () => {
        if (!canExit) return
        onOpenChange(false)
        onViewDetails()
    }

    // Type label
    const getTypeLabel = () => {
        if (isM2M) return "Machine-to-Machine"
        if (isConfidential) return "Confidential"
        return "Public"
    }

    return (
        <Dialog open={open} onOpenChange={(v) => { if (!v && canExit) onOpenChange(false) }}>
            <DialogContent
                showCloseButton={false}
                className="!max-w-5xl !w-[95vw] max-h-[90vh] p-0 gap-0 overflow-hidden border-0 shadow-2xl"
            >
                {/* Close button */}
                {canExit && (
                    <button
                        onClick={handleClose}
                        className="absolute right-6 top-6 z-10 rounded-full p-2 hover:bg-muted/80 transition-colors"
                    >
                        <X className="h-5 w-5 text-muted-foreground" />
                    </button>
                )}

                <div className="flex flex-col lg:flex-row min-h-[500px] max-h-[85vh]">
                    {/* ================================================================
                        LEFT PANEL - Success & Credentials
                        ================================================================ */}
                    <div className="w-full lg:w-[400px] lg:min-w-[400px] shrink-0 bg-gradient-to-br from-success/5 via-background to-background p-6 lg:p-8 flex flex-col border-b lg:border-b-0 lg:border-r overflow-y-auto">
                        {/* Success header */}
                        <div className="text-center lg:text-left mb-8">
                            <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-success/10 border border-success/20 mb-5">
                                <CheckCircle2 className="h-8 w-8 text-success" />
                            </div>
                            <h1 className="text-2xl font-bold mb-2">¡Cliente creado!</h1>
                            <div className="flex items-center justify-center lg:justify-start gap-2 text-muted-foreground">
                                <TypeIcon className="h-4 w-4" />
                                <span className="font-medium text-foreground">{client.name}</span>
                                <Badge
                                    variant={isM2M ? "warning" : isConfidential ? "default" : "success"}
                                    className="text-xs"
                                >
                                    {getTypeLabel()}
                                </Badge>
                            </div>
                        </div>

                        {/* Credentials */}
                        <div className="space-y-4 flex-1">
                            {/* Client ID */}
                            <div className="space-y-2">
                                <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                                    Client ID
                                </Label>
                                <div className="flex items-center gap-2">
                                    <code className="flex-1 rounded-lg bg-muted/50 border px-4 py-3 font-mono text-sm truncate">
                                        {client.client_id}
                                    </code>
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        onClick={() => copy(client.client_id, "client_id")}
                                        className="shrink-0 h-10 w-10 p-0"
                                    >
                                        {copied === "client_id" ? (
                                            <Check className="h-4 w-4 text-success" />
                                        ) : (
                                            <Copy className="h-4 w-4" />
                                        )}
                                    </Button>
                                </div>
                            </div>

                            {/* Client Secret */}
                            {isConfidential && secret && (
                                <div className="space-y-3">
                                    <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide flex items-center gap-1.5">
                                        Client Secret
                                        <Lock className="h-3 w-3" />
                                    </Label>
                                    <div className="flex items-center gap-2">
                                        <code className="flex-1 rounded-lg bg-amber-500/5 border border-amber-500/20 px-4 py-3 font-mono text-sm truncate">
                                            {showSecret ? secret : "•".repeat(32)}
                                        </code>
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            onClick={() => setShowSecret(!showSecret)}
                                            className="shrink-0 h-10 w-10 p-0"
                                        >
                                            {showSecret ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                                        </Button>
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            onClick={() => copy(secret, "secret")}
                                            className="shrink-0 h-10 w-10 p-0"
                                        >
                                            {copied === "secret" ? (
                                                <Check className="h-4 w-4 text-success" />
                                            ) : (
                                                <Copy className="h-4 w-4" />
                                            )}
                                        </Button>
                                    </div>

                                    {/* Warning */}
                                    <div className="rounded-lg bg-amber-500/10 border border-amber-500/20 p-3 flex items-start gap-2.5">
                                        <AlertTriangle className="h-4 w-4 text-amber-500 shrink-0 mt-0.5" />
                                        <p className="text-xs text-amber-600 dark:text-amber-400 leading-relaxed">
                                            <strong>Guarda este secret ahora.</strong> No podrás verlo de nuevo.
                                        </p>
                                    </div>

                                    {/* Confirmation */}
                                    <label className="flex items-start gap-3 p-3 rounded-lg border bg-muted/30 cursor-pointer hover:bg-muted/50 transition-colors">
                                        <Checkbox
                                            checked={secretConfirmed}
                                            onCheckedChange={(checked) => setSecretConfirmed(!!checked)}
                                            className="mt-0.5"
                                        />
                                        <span className="text-xs leading-relaxed">
                                            Confirmo que guardé el secret de forma segura
                                        </span>
                                    </label>
                                </div>
                            )}
                        </div>

                        {/* Actions */}
                        <div className="flex flex-col gap-2 mt-6 pt-6 border-t">
                            {isConfidential && secret && !secretConfirmed && (
                                <p className="text-xs text-amber-500 flex items-center gap-1.5 mb-2">
                                    <AlertTriangle className="h-3.5 w-3.5" />
                                    Confirma que guardaste el secret
                                </p>
                            )}
                            <div className="flex gap-2">
                                <Button
                                    variant="outline"
                                    onClick={handleViewDetails}
                                    disabled={!canExit}
                                    className="flex-1"
                                >
                                    <ExternalLink className="h-4 w-4 mr-2" />
                                    Ver detalles
                                </Button>
                                <Button
                                    onClick={handleClose}
                                    disabled={!canExit}
                                    className="flex-1"
                                >
                                    Listo
                                </Button>
                            </div>
                        </div>
                    </div>

                    {/* ================================================================
                        RIGHT PANEL - SDK Integration
                        ================================================================ */}
                    <div className="flex-1 min-w-0 overflow-y-auto p-6 lg:p-8 bg-background">
                        {/* SDK Integration */}
                        <div className="space-y-6">
                            <div className="flex items-center gap-3">
                                <div className="p-2 rounded-lg bg-primary/10">
                                    <Terminal className="h-5 w-5 text-primary" />
                                </div>
                                <div>
                                    <h2 className="text-lg font-semibold">Integración rápida</h2>
                                    <p className="text-sm text-muted-foreground">Copia el código para comenzar</p>
                                </div>
                            </div>

                            <Tabs defaultValue={selectedSdk} onValueChange={setSelectedSdk} className="w-full">
                                <TabsList className="w-full h-11 p-1 grid mb-4" style={{ gridTemplateColumns: `repeat(${availableSdkTabs.length}, 1fr)` }}>
                                    {availableSdkTabs.map((tab) => (
                                        <TabsTrigger
                                            key={tab.id}
                                            value={tab.id}
                                            className="text-sm font-medium"
                                        >
                                            {tab.label}
                                        </TabsTrigger>
                                    ))}
                                </TabsList>

                                {availableSdkTabs.map((tab) => (
                                    <TabsContent key={tab.id} value={tab.id} className="space-y-4 mt-0">
                                        {/* Install command */}
                                        <div className="flex items-center gap-3 p-3 rounded-lg bg-muted/50 border">
                                            <Terminal className="h-4 w-4 text-muted-foreground shrink-0" />
                                            <code className="text-sm font-mono flex-1 truncate">
                                                {tab.installCmd}
                                            </code>
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                onClick={() => copy(tab.installCmd, `install-${tab.id}`)}
                                                className="h-8 w-8 p-0 shrink-0"
                                            >
                                                {copied === `install-${tab.id}` ? (
                                                    <Check className="h-4 w-4 text-success" />
                                                ) : (
                                                    <Copy className="h-4 w-4" />
                                                )}
                                            </Button>
                                        </div>

                                        {/* Code snippet */}
                                        <CodeSnippet
                                            code={getSnippet(tab.id, snippetConfig)}
                                            language={tab.language}
                                            filename={tab.filename}
                                        />
                                    </TabsContent>
                                ))}
                            </Tabs>

                            {/* Next steps */}
                            <div className="space-y-4 pt-4 border-t">
                                <div className="flex items-center gap-2">
                                    <Rocket className="h-4 w-4 text-muted-foreground" />
                                    <h3 className="text-sm font-semibold">Próximos pasos</h3>
                                </div>
                                <div className="space-y-2">
                                    {nextSteps.map((step, i) => (
                                        <div key={i} className="flex items-start gap-3">
                                            <span className="flex items-center justify-center h-5 w-5 rounded-full text-xs font-bold bg-primary/10 text-primary shrink-0 mt-0.5">
                                                {i + 1}
                                            </span>
                                            <p className="text-sm text-muted-foreground leading-relaxed">
                                                {step}
                                            </p>
                                        </div>
                                    ))}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </DialogContent>
        </Dialog>
    )
}
