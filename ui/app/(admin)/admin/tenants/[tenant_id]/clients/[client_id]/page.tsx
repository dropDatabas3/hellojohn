"use client"

import { useState, useEffect } from "react"
import { useQuery } from "@tanstack/react-query"
import { useParams, useRouter, useSearchParams } from "next/navigation"
import {
    ArrowLeft,
    Copy,
    Check,
    Eye,
    EyeOff,
    AlertTriangle,
    Terminal,
    CheckCircle2,
    Rocket,
    Lock,
    Globe,
    Server,
    Cpu,
    Smartphone,
    Settings2,
    Shield,
    KeyRound,
    Key,
    Link2,
    RefreshCw,
    Zap,
    FileCode2,
    RotateCcw,
    Loader2,
    CheckCircle,
    XCircle,
} from "lucide-react"
import Link from "next/link"
import { api } from "@/lib/api"
import { useToast } from "@/hooks/use-toast"
import type { Tenant } from "@/lib/types"

import {
    Button,
    Badge,
    Card, CardContent, CardDescription, CardHeader, CardTitle,
    Label,
    Tabs, TabsContent, TabsList, TabsTrigger,
    Checkbox,
    InlineAlert,
    cn,
} from "@/components/ds"

import type { ClientRow, AppSubType } from "@/components/clients/wizard"
import {
    GRANT_TYPES,
    AVAILABLE_PROVIDERS,
    formatTTL,
} from "@/components/clients/wizard"

import { InfoTooltip } from "@/components/clients/shared"
import { CodeSnippet } from "@/components/clients/quickstart/CodeSnippet"
import {
    SUB_TYPE_DEFAULT_SDK,
    getSnippet,
    getNextSteps,
    getFilteredSdkTabs,
} from "@/components/clients/quickstart/snippets"
import type { SnippetConfig } from "@/components/clients/quickstart/snippets"

// ============================================================================
// TYPE ICONS
// ============================================================================

const TYPE_ICONS: Record<AppSubType, React.ElementType> = {
    spa: Globe,
    mobile: Smartphone,
    api_server: Server,
    m2m: Cpu,
}

const TYPE_LABELS: Record<AppSubType, string> = {
    spa: "Single Page App",
    mobile: "Mobile App",
    api_server: "API Server",
    m2m: "Machine-to-Machine",
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

export default function ClientDetailPage() {
    const params = useParams()
    const router = useRouter()
    const searchParams = useSearchParams()
    const { toast } = useToast()

    const tenantId = params.tenant_id as string
    const clientId = params.client_id as string

    // Check if we're in "just created" mode
    const isJustCreated = searchParams.get("created") === "true"
    const subTypeFromUrl = searchParams.get("subType") as AppSubType | null

    // Read secret from sessionStorage (more secure than URL)
    const [secretFromStorage, setSecretFromStorage] = useState<string | null>(null)

    useEffect(() => {
        if (globalThis.window !== undefined && isJustCreated) {
            const storedSecret = sessionStorage.getItem(`client_secret_${clientId}`)
            if (storedSecret) {
                setSecretFromStorage(storedSecret)
                // Auto-cleanup: remove from sessionStorage after reading
                sessionStorage.removeItem(`client_secret_${clientId}`)
            }
        }
    }, [clientId, isJustCreated])

    // UI State
    const [secretConfirmed, setSecretConfirmed] = useState(false)
    const [showSecret, setShowSecret] = useState(false)
    const { copied, copy } = useCopyFeedback()

    // SDK tab: pre-select based on sub-type
    const defaultSdk = subTypeFromUrl ? (SUB_TYPE_DEFAULT_SDK[subTypeFromUrl] || "node") : "node"
    const [selectedSdk, setSelectedSdk] = useState(defaultSdk)

    // ========================================================================
    // QUERIES
    // ========================================================================

    const { data: tenant } = useQuery({
        queryKey: ["tenant", tenantId],
        enabled: !!tenantId,
        queryFn: () => api.get<Tenant>(`/v2/admin/tenants/${tenantId}`),
    })

    const { data: client, isLoading } = useQuery({
        queryKey: ["client", tenantId, clientId],
        enabled: !!tenantId && !!clientId,
        queryFn: () => api.get<ClientRow>(`/v2/admin/tenants/${tenantId}/clients/${clientId}`),
    })

    // ========================================================================
    // DERIVED STATE
    // ========================================================================

    // For post-creation, we can derive type from subType
    const isConfidential = subTypeFromUrl
        ? (subTypeFromUrl === "api_server" || subTypeFromUrl === "m2m")
        : client?.type === "confidential"

    const isM2M = subTypeFromUrl === "m2m"
    const TypeIcon = subTypeFromUrl ? TYPE_ICONS[subTypeFromUrl] : (isConfidential ? Server : Globe)

    // Build domain from current window
    const domain = typeof window !== "undefined"
        ? window.location.origin
        : "https://auth.example.com"

    // Get filtered SDK tabs based on client type
    const availableSdkTabs = getFilteredSdkTabs(subTypeFromUrl || undefined)

    // For post-creation view, use URL params directly
    const snippetConfig: SnippetConfig = {
        clientId: clientId, // Use URL param directly
        tenantSlug: tenant?.slug || "",
        domain,
        type: isConfidential ? "confidential" : "public",
        secret: secretFromStorage || undefined,
        subType: subTypeFromUrl || undefined,
    }

    const nextSteps = getNextSteps(selectedSdk, subTypeFromUrl || undefined)

    // Can leave if secret is confirmed (for confidential clients with secret)
    const canLeave = !isJustCreated || !isConfidential || !secretFromStorage || secretConfirmed

    // Type label
    const getTypeLabel = () => {
        if (subTypeFromUrl) return TYPE_LABELS[subTypeFromUrl]
        if (isM2M) return "Machine-to-Machine"
        if (isConfidential) return "Confidential"
        return "Public"
    }

    // ========================================================================
    // HANDLERS
    // ========================================================================

    const handleBack = () => {
        if (!canLeave) {
            toast({
                title: "Confirma el secret",
                description: "Debes confirmar que guardaste el secret antes de salir.",
                variant: "destructive",
            })
            return
        }
        router.push(`/admin/tenants/${tenantId}/clients`)
    }

    const handleDone = () => {
        if (!canLeave) return
        router.push(`/admin/tenants/${tenantId}/clients`)
    }

    // ========================================================================
    // POST-CREATION VIEW (Quick Start Style)
    // ========================================================================

    if (isJustCreated) {
        return (
            <div className="space-y-6 animate-in fade-in duration-500">
                {/* Header con botón fijo */}
                <div className="flex items-center justify-between min-h-[52px]">
                    <div className="flex items-center gap-3">
                        <div className="flex items-center justify-center w-12 h-12 rounded-xl bg-success/10 border border-success/20">
                            <CheckCircle2 className="h-6 w-6 text-success" />
                        </div>
                        <div>
                            <h2 className="text-xl font-bold">¡Cliente creado!</h2>
                            <div className="flex items-center gap-2 text-sm text-muted-foreground">
                                <TypeIcon className="h-3.5 w-3.5" />
                                <Badge
                                    variant={isM2M ? "warning" : isConfidential ? "default" : "success"}
                                    className="text-xs"
                                >
                                    {getTypeLabel()}
                                </Badge>
                            </div>
                        </div>
                    </div>

                    {/* Botón "Listo" - posición fija con advertencia relativa */}
                    <div className="relative">
                        {isConfidential && secretFromStorage && !secretConfirmed && (
                            <p className="absolute -top-6 right-0 text-xs text-amber-500 flex items-center gap-1.5 whitespace-nowrap">
                                <AlertTriangle className="h-3.5 w-3.5" />
                                Confirma que guardaste el secret
                            </p>
                        )}
                        <Button
                            onClick={handleDone}
                            disabled={!canLeave}
                            className="min-w-[100px]"
                        >
                            Listo
                        </Button>
                    </div>
                </div>

                {/* Grid principal responsive */}
                <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-6 items-start">
                    {/* ================================================================
                        LEFT - Credentials
                        ================================================================ */}
                    <Card className="border-success/30 bg-gradient-to-br from-success/5 via-background to-background">
                        <CardHeader>
                            <CardTitle className="text-lg">Credenciales del cliente</CardTitle>
                            <CardDescription>Guarda estas credenciales de forma segura</CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            {/* Client ID */}
                            <div className="space-y-2">
                                <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                                    Client ID
                                </Label>
                                <div className="flex items-center gap-2">
                                    <code className="flex-1 rounded-lg bg-muted/50 border px-4 py-3 font-mono text-sm overflow-hidden text-ellipsis">
                                        {clientId}
                                    </code>
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        onClick={() => copy(clientId, "client_id")}
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
                            {isConfidential && secretFromStorage && (
                                <div className="space-y-3">
                                    <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide flex items-center gap-1.5">
                                        Client Secret
                                        <Lock className="h-3 w-3" />
                                    </Label>
                                    <div className="flex items-center gap-2">
                                        <code className="flex-1 rounded-lg bg-amber-500/5 border border-amber-500/20 px-4 py-3 font-mono text-sm overflow-hidden break-all max-w-full">
                                            {showSecret ? secretFromStorage : "•".repeat(32)}
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
                                            onClick={() => copy(secretFromStorage, "secret")}
                                            className="shrink-0 h-10 w-10 p-0"
                                        >
                                            {copied === "secret" ? (
                                                <Check className="h-4 w-4 text-success" />
                                            ) : (
                                                <Copy className="h-4 w-4" />
                                            )}
                                        </Button>
                                    </div>

                                    {/* Warning + Confirmation */}
                                    <div className="space-y-3">
                                        <div className="rounded-lg bg-amber-500/10 border border-amber-500/20 p-3 flex items-start gap-2.5">
                                            <AlertTriangle className="h-4 w-4 text-amber-500 shrink-0 mt-0.5" />
                                            <p className="text-xs text-amber-600 dark:text-amber-400 leading-relaxed">
                                                <strong>Guarda este secret ahora.</strong> No podrás verlo de nuevo.
                                            </p>
                                        </div>

                                        <label className="flex items-center gap-3 p-3 rounded-lg border bg-muted/30 cursor-pointer hover:bg-muted/50 transition-colors">
                                            <Checkbox
                                                checked={secretConfirmed}
                                                onCheckedChange={(checked) => setSecretConfirmed(!!checked)}
                                            />
                                            <span className="text-sm leading-relaxed">
                                                Confirmo que guardé el secret de forma segura
                                            </span>
                                        </label>
                                    </div>
                                </div>
                            )}
                        </CardContent>
                    </Card>

                    {/* ================================================================
                        RIGHT - Next Steps
                        ================================================================ */}
                    <Card className="lg:sticky lg:top-6">
                        <CardHeader>
                            <CardTitle className="text-base flex items-center gap-2">
                                <Rocket className="h-4 w-4 text-primary" />
                                Próximos pasos
                            </CardTitle>
                            <CardDescription>Sigue estos pasos para integrar</CardDescription>
                        </CardHeader>
                        <CardContent>
                            <div className="space-y-3">
                                {nextSteps.map((step, i) => (
                                    <div key={i} className="flex items-start gap-3">
                                        <span className="flex items-center justify-center h-6 w-6 rounded-full text-xs font-bold bg-primary/10 text-primary shrink-0 mt-0.5">
                                            {i + 1}
                                        </span>
                                        <p className="text-sm text-muted-foreground leading-relaxed">
                                            {step}
                                        </p>
                                    </div>
                                ))}
                            </div>
                        </CardContent>
                    </Card>
                </div>

                {/* ================================================================
                    BOTTOM - SDK Integration (Full Width)
                    ================================================================ */}
                <Card>
                    <CardHeader>
                        <div className="flex items-center gap-3">
                            <div className="p-2 rounded-lg bg-primary/10">
                                <Terminal className="h-5 w-5 text-primary" />
                            </div>
                            <div>
                                <CardTitle>Integración rápida</CardTitle>
                                <CardDescription>Copia el código para comenzar</CardDescription>
                            </div>
                        </div>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <Tabs defaultValue={selectedSdk} onValueChange={setSelectedSdk} className="w-full">
                            <TabsList className="w-full max-w-md h-11 p-1 grid mb-4" style={{ gridTemplateColumns: `repeat(${availableSdkTabs.length}, 1fr)` }}>
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
                                    <div className="flex items-center gap-3 p-3 rounded-lg bg-muted/50 border max-w-2xl overflow-hidden">
                                        <Terminal className="h-4 w-4 text-muted-foreground shrink-0" />
                                        <code className="text-sm font-mono flex-1 overflow-hidden text-ellipsis">
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
                    </CardContent>
                </Card>
            </div>
        )
    }

    // ========================================================================
    // NORMAL DETAIL VIEW (When accessed from table row)
    // ========================================================================

    if (isLoading) {
        return (
            <div className="flex items-center justify-center min-h-[400px]">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
        )
    }

    if (!client) {
        return (
            <div className="flex flex-col items-center justify-center min-h-[400px] gap-4">
                <AlertTriangle className="h-12 w-12 text-warning" />
                <h2 className="text-xl font-semibold">Cliente no encontrado</h2>
                <Button variant="outline" onClick={() => router.push(`/admin/tenants/${tenantId}/clients`)}>
                    <ArrowLeft className="h-4 w-4 mr-2" />
                    Volver a Clients
                </Button>
            </div>
        )
    }

    return (
        <div className="space-y-6 animate-in fade-in duration-500">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                    <Button variant="ghost" size="sm" asChild>
                        <Link href={`/admin/tenants/${tenantId}/clients`}>
                            <ArrowLeft className="h-4 w-4" />
                        </Link>
                    </Button>
                    <div className="flex items-center gap-3">
                        <div className={cn(
                            "p-3 rounded-xl",
                            client.type === "confidential"
                                ? "bg-amber-500/10 text-amber-500"
                                : "bg-success/10 text-success"
                        )}>
                            {client.type === "confidential" ? <Server className="h-6 w-6" /> : <Globe className="h-6 w-6" />}
                        </div>
                        <div>
                            <h1 className="text-2xl font-bold tracking-tight">{client.name}</h1>
                            <div className="flex items-center gap-2 text-sm text-muted-foreground">
                                <code className="text-xs bg-muted px-2 py-0.5 rounded">{client.client_id}</code>
                                <Badge variant={client.type === "confidential" ? "default" : "success"}>
                                    {client.type === "confidential" ? "Confidential" : "Public"}
                                </Badge>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            {/* Main Content Tabs */}
            <Tabs defaultValue="general">
                <TabsList className="grid w-full max-w-md grid-cols-4">
                    <TabsTrigger value="general" className="flex items-center gap-2">
                        <Settings2 className="h-4 w-4" />
                        <span className="hidden sm:inline">General</span>
                    </TabsTrigger>
                    <TabsTrigger value="security" className="flex items-center gap-2">
                        <Shield className="h-4 w-4" />
                        <span className="hidden sm:inline">Seguridad</span>
                    </TabsTrigger>
                    <TabsTrigger value="tokens" className="flex items-center gap-2">
                        <KeyRound className="h-4 w-4" />
                        <span className="hidden sm:inline">Tokens</span>
                    </TabsTrigger>
                    <TabsTrigger value="integration" className="flex items-center gap-2">
                        <Terminal className="h-4 w-4" />
                        <span className="hidden sm:inline">Código</span>
                    </TabsTrigger>
                </TabsList>

                {/* ============================================================
                    TAB: GENERAL
                    ============================================================ */}
                <TabsContent value="general" className="mt-6 space-y-6">
                    {/* Client ID */}
                    <Card>
                        <CardHeader>
                            <CardTitle className="flex items-center gap-2 text-base">
                                <Key className="h-4 w-4" />
                                Client ID
                            </CardTitle>
                        </CardHeader>
                        <CardContent>
                            <div className="flex items-center gap-2">
                                <code className="flex-1 rounded bg-muted px-4 py-2.5 text-sm font-mono">{client.client_id}</code>
                                <Button variant="outline" size="sm" onClick={() => copy(client.client_id, "client_id")}>
                                    {copied === "client_id" ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                                </Button>
                            </div>
                        </CardContent>
                    </Card>

                    {/* Redirect URIs */}
                    <Card>
                        <CardHeader>
                            <CardTitle className="flex items-center gap-2 text-base">
                                <Link2 className="h-4 w-4" />
                                URIs de redirección
                                <InfoTooltip content="URLs permitidas para redirección después del login." />
                            </CardTitle>
                        </CardHeader>
                        <CardContent>
                            {client.redirect_uris && client.redirect_uris.length > 0 ? (
                                <div className="space-y-2">
                                    {client.redirect_uris.map((uri) => (
                                        <div key={uri} className="flex items-center gap-2 rounded bg-muted p-2">
                                            <Link2 className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
                                            <code className="text-sm flex-1 truncate">{uri}</code>
                                        </div>
                                    ))}
                                </div>
                            ) : (
                                <p className="text-sm text-muted-foreground">Sin URIs configuradas</p>
                            )}
                        </CardContent>
                    </Card>

                    {/* Allowed Origins (public only) */}
                    {client.type === "public" && (
                        <Card>
                            <CardHeader>
                                <CardTitle className="flex items-center gap-2 text-base">
                                    <Globe className="h-4 w-4" />
                                    Orígenes permitidos (CORS)
                                    <InfoTooltip content="Dominios desde los que se permiten requests." />
                                </CardTitle>
                            </CardHeader>
                            <CardContent>
                                {client.allowed_origins && client.allowed_origins.length > 0 ? (
                                    <div className="space-y-2">
                                        {client.allowed_origins.map((origin) => (
                                            <div key={origin} className="flex items-center gap-2 rounded bg-muted p-2">
                                                <Globe className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
                                                <code className="text-sm">{origin}</code>
                                            </div>
                                        ))}
                                    </div>
                                ) : (
                                    <p className="text-sm text-muted-foreground">Sin orígenes configurados</p>
                                )}
                            </CardContent>
                        </Card>
                    )}

                    {/* Scopes */}
                    <Card>
                        <CardHeader>
                            <CardTitle className="text-base">Scopes permitidos</CardTitle>
                        </CardHeader>
                        <CardContent>
                            <div className="flex flex-wrap gap-2">
                                {(client.scopes || ["openid", "profile", "email"]).map((scope) => (
                                    <Badge key={scope} variant="outline">{scope}</Badge>
                                ))}
                            </div>
                        </CardContent>
                    </Card>

                    {/* Providers (public only) */}
                    {client.type === "public" && client.providers && client.providers.length > 0 && (
                        <Card>
                            <CardHeader>
                                <CardTitle className="text-base">Proveedores de autenticación</CardTitle>
                            </CardHeader>
                            <CardContent>
                                <div className="flex flex-wrap gap-2">
                                    {client.providers.map((p) => {
                                        const provider = AVAILABLE_PROVIDERS.find(pr => pr.id === p)
                                        return (
                                            <Badge key={p} variant="secondary">
                                                {provider?.icon} {provider?.label || p}
                                            </Badge>
                                        )
                                    })}
                                </div>
                            </CardContent>
                        </Card>
                    )}
                </TabsContent>

                {/* ============================================================
                    TAB: SECURITY
                    ============================================================ */}
                <TabsContent value="security" className="mt-6 space-y-6">
                    {/* Client Secret (confidential only) */}
                    {client.type === "confidential" && (
                        <Card className="border-warning/30 bg-warning/5">
                            <CardHeader className="pb-3">
                                <CardTitle className="text-base flex items-center gap-2">
                                    <Key className="h-4 w-4 text-warning" />
                                    Client Secret
                                </CardTitle>
                                <CardDescription>
                                    El secret se muestra solo al crear o rotar. Guárdalo de forma segura.
                                </CardDescription>
                            </CardHeader>
                            <CardContent>
                                <div className="flex items-center gap-2">
                                    <code className="flex-1 rounded bg-muted px-4 py-2.5 text-sm font-mono text-muted-foreground">
                                        •••••••••••••••••••••••••••••••••••••••
                                    </code>
                                    <Button variant="outline" asChild>
                                        <Link href={`/admin/tenants/${tenantId}/clients`}>
                                            <RotateCcw className="h-4 w-4 mr-2" />
                                            Rotar Secret
                                        </Link>
                                    </Button>
                                </div>
                            </CardContent>
                        </Card>
                    )}

                    {/* Grant Types */}
                    <Card>
                        <CardHeader>
                            <CardTitle className="text-base">Grant Types habilitados</CardTitle>
                        </CardHeader>
                        <CardContent>
                            <div className="grid gap-2">
                                {GRANT_TYPES.map((gt) => {
                                    const enabled = client.grant_types?.includes(gt.id) ||
                                        (gt.id === "authorization_code" && !client.grant_types)
                                    return (
                                        <div
                                            key={gt.id}
                                            className={cn(
                                                "flex items-center justify-between p-3 rounded-lg border transition-colors",
                                                enabled ? "bg-success/5 border-success/20" : "bg-muted/30"
                                            )}
                                        >
                                            <div className="flex items-center gap-3">
                                                {enabled ? (
                                                    <CheckCircle className="h-4 w-4 text-success" />
                                                ) : (
                                                    <XCircle className="h-4 w-4 text-muted-foreground" />
                                                )}
                                                <div>
                                                    <p className="font-medium text-sm">{gt.label}</p>
                                                    <p className="text-xs text-muted-foreground">{gt.description}</p>
                                                </div>
                                            </div>
                                            {gt.deprecated && <Badge variant="destructive" className="text-[10px]">Deprecado</Badge>}
                                            {gt.recommended && enabled && <Badge variant="secondary" className="text-[10px]">Recomendado</Badge>}
                                        </div>
                                    )
                                })}
                            </div>
                        </CardContent>
                    </Card>

                    {/* PKCE info for public clients */}
                    {client.type === "public" && (
                        <InlineAlert
                            variant="info"
                            title="PKCE habilitado"
                            description="Los clients públicos usan PKCE (Proof Key for Code Exchange) automáticamente para proteger el flujo de autorización sin necesidad de un client_secret."
                        />
                    )}
                </TabsContent>

                {/* ============================================================
                    TAB: TOKENS
                    ============================================================ */}
                <TabsContent value="tokens" className="mt-6 space-y-6">
                    <div className="grid md:grid-cols-3 gap-4">
                        <Card>
                            <CardHeader className="pb-2">
                                <CardTitle className="text-sm flex items-center gap-2">
                                    <Zap className="h-4 w-4 text-warning" />
                                    Access Token
                                </CardTitle>
                            </CardHeader>
                            <CardContent>
                                <p className="text-2xl font-bold">{formatTTL(client.access_token_ttl || 15)}</p>
                                <p className="text-xs text-muted-foreground">Tiempo de vida</p>
                            </CardContent>
                        </Card>
                        <Card>
                            <CardHeader className="pb-2">
                                <CardTitle className="text-sm flex items-center gap-2">
                                    <RefreshCw className="h-4 w-4 text-success" />
                                    Refresh Token
                                </CardTitle>
                            </CardHeader>
                            <CardContent>
                                <p className="text-2xl font-bold">{formatTTL(client.refresh_token_ttl || 43200)}</p>
                                <p className="text-xs text-muted-foreground">Tiempo de vida</p>
                            </CardContent>
                        </Card>
                        <Card>
                            <CardHeader className="pb-2">
                                <CardTitle className="text-sm flex items-center gap-2">
                                    <FileCode2 className="h-4 w-4 text-info" />
                                    ID Token
                                </CardTitle>
                            </CardHeader>
                            <CardContent>
                                <p className="text-2xl font-bold">{formatTTL(client.id_token_ttl || 60)}</p>
                                <p className="text-xs text-muted-foreground">Tiempo de vida</p>
                            </CardContent>
                        </Card>
                    </div>

                    <InlineAlert
                        variant="default"
                        description="Los tiempos de vida de tokens se pueden modificar editando el cliente. Valores más cortos son más seguros pero requieren renovación más frecuente."
                    />
                </TabsContent>

                {/* ============================================================
                    TAB: INTEGRATION
                    ============================================================ */}
                <TabsContent value="integration" className="mt-6 space-y-6">
                    <Card>
                        <CardHeader>
                            <div className="flex items-center gap-3">
                                <div className="p-2 rounded-lg bg-primary/10">
                                    <Terminal className="h-5 w-5 text-primary" />
                                </div>
                                <div>
                                    <CardTitle>Código de integración</CardTitle>
                                    <CardDescription>Copia el código para integrar HelloJohn en tu aplicación</CardDescription>
                                </div>
                            </div>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            <Tabs defaultValue={availableSdkTabs[0]?.id || "node"} className="w-full">
                                <TabsList className="w-full max-w-md h-11 p-1 grid mb-4" style={{ gridTemplateColumns: `repeat(${availableSdkTabs.length}, 1fr)` }}>
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
                                        <div className="flex items-center gap-3 p-3 rounded-lg bg-muted/50 border max-w-2xl">
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
                                            code={getSnippet(tab.id, {
                                                clientId: client.client_id,
                                                tenantSlug: tenant?.slug || "",
                                                domain,
                                                type: client.type,
                                                secret: undefined, // No secret in normal view
                                                subType: undefined,
                                            })}
                                            language={tab.language}
                                            filename={tab.filename}
                                        />
                                    </TabsContent>
                                ))}
                            </Tabs>

                            <InlineAlert
                                variant="info"
                                description="El código de ejemplo usa valores de configuración de este cliente. Asegúrate de que las redirect URIs coincidan con tu configuración."
                            />
                        </CardContent>
                    </Card>
                </TabsContent>
            </Tabs>
        </div>
    )
}
