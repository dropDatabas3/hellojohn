"use client"

import { useState, useEffect, useMemo } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
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
    Save,
    Plus,
    Trash2,
    ExternalLink,
    Info,
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
    Input,
    Tabs, TabsContent, TabsList, TabsTrigger,
    Checkbox,
    Switch,
    Separator,
    InlineAlert,
    cn,
    Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
    Slider,
} from "@/components/ds"

import type { ClientRow, AppSubType } from "@/components/clients/wizard"
import {
    GRANT_TYPES,
    AVAILABLE_PROVIDERS,
    formatTTL,
    formatRelativeTime,
    TOKEN_TTL_OPTIONS,
    PREDEFINED_SCOPES,
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
// EDITABLE URI LIST
// ============================================================================

function EditableUriList({
    label,
    icon: Icon,
    tooltip,
    values,
    onChange,
    placeholder = "https://example.com/callback",
}: {
    label: string
    icon: React.ElementType
    tooltip: string
    values: string[]
    onChange: (values: string[]) => void
    placeholder?: string
}) {
    const [newUri, setNewUri] = useState("")

    const addUri = () => {
        const trimmed = newUri.trim()
        if (trimmed && !values.includes(trimmed)) {
            onChange([...values, trimmed])
            setNewUri("")
        }
    }

    const removeUri = (uri: string) => {
        onChange(values.filter((v) => v !== uri))
    }

    return (
        <div className="space-y-3">
            <Label className="flex items-center gap-2 text-sm font-medium">
                <Icon className="h-4 w-4 text-muted-foreground" />
                {label}
                <InfoTooltip content={tooltip} />
            </Label>
            {values.length > 0 && (
                <div className="space-y-2">
                    {values.map((uri) => (
                        <div key={uri} className="flex items-center gap-2 rounded-lg bg-muted/50 border p-2.5">
                            <Icon className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
                            <code className="text-sm flex-1 truncate">{uri}</code>
                            <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 w-7 p-0 text-muted-foreground hover:text-danger"
                                onClick={() => removeUri(uri)}
                            >
                                <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                        </div>
                    ))}
                </div>
            )}
            <div className="flex items-center gap-2">
                <Input
                    placeholder={placeholder}
                    value={newUri}
                    onChange={(e) => setNewUri(e.target.value)}
                    onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addUri() } }}
                    className="flex-1 text-sm"
                />
                <Button variant="outline" size="sm" onClick={addUri} disabled={!newUri.trim()}>
                    <Plus className="h-4 w-4" />
                </Button>
            </div>
        </div>
    )
}


// ============================================================================
// MAIN COMPONENT
// ============================================================================

export default function ClientDetailPage() {
    const params = useParams()
    const router = useRouter()
    const searchParams = useSearchParams()
    const { toast } = useToast()
    const queryClient = useQueryClient()

    const tenantId = params.tenant_id as string
    const clientId = params.client_id as string

    // Check if we're in "just created" mode
    const isJustCreated = searchParams.get("created") === "true"
    const subTypeFromUrl = searchParams.get("subType") as AppSubType | null

    // Read secret from sessionStorage (more secure than URL)
    const [secretFromStorage, setSecretFromStorage] = useState<string | null>(null)

    useEffect(() => {
        if (typeof window !== "undefined" && isJustCreated) {
            const storedSecret = sessionStorage.getItem(`client_secret_${clientId}`)
            if (storedSecret) {
                setSecretFromStorage(storedSecret)
                sessionStorage.removeItem(`client_secret_${clientId}`)
            }
        }
    }, [clientId, isJustCreated])

    // UI State
    const [secretConfirmed, setSecretConfirmed] = useState(false)
    const [showSecret, setShowSecret] = useState(false)
    const { copied, copy } = useCopyFeedback()

    // Edit state
    const [isEditing, setIsEditing] = useState(false)
    const [editForm, setEditForm] = useState<Partial<ClientRow>>({})

    // Dialogs
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [rotateSecretDialogOpen, setRotateSecretDialogOpen] = useState(false)

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

    const { data: client, isLoading, error: clientError } = useQuery({
        queryKey: ["client", tenantId, clientId],
        enabled: !!tenantId && !!clientId,
        queryFn: async () => {
            const response = await api.get<ClientRow[]>(`/v2/admin/tenants/${tenantId}/clients/${clientId}`)
            return response[0] || null
        },
    })

    // Sync edit form when client loads
    useEffect(() => {
        if (client && !isEditing) {
            setEditForm({
                name: client.name,
                redirect_uris: client.redirect_uris || [],
                allowed_origins: client.allowed_origins || [],
                post_logout_uris: client.post_logout_uris || [],
                providers: client.providers || [],
                scopes: client.scopes || [],
                grant_types: client.grant_types || [],
                access_token_ttl: client.access_token_ttl || 15,
                refresh_token_ttl: client.refresh_token_ttl || 43200,
                id_token_ttl: client.id_token_ttl || 60,
                require_email_verification: client.require_email_verification || false,
                reset_password_url: client.reset_password_url || "",
                verify_email_url: client.verify_email_url || "",
                description: client.description || "",
            })
        }
    }, [client, isEditing])

    // ========================================================================
    // MUTATIONS
    // ========================================================================

    const updateMutation = useMutation({
        mutationFn: (data: Partial<ClientRow>) =>
            api.put<ClientRow>(`/v2/admin/tenants/${tenantId}/clients/${clientId}`, {
                name: data.name,
                redirect_uris: data.redirect_uris || [],
                allowed_origins: data.allowed_origins || [],
                post_logout_uris: data.post_logout_uris || [],
                providers: data.providers || [],
                scopes: data.scopes || [],
                grant_types: data.grant_types || [],
                access_token_ttl: data.access_token_ttl,
                refresh_token_ttl: data.refresh_token_ttl,
                id_token_ttl: data.id_token_ttl,
                require_email_verification: data.require_email_verification || false,
                reset_password_url: data.reset_password_url || "",
                verify_email_url: data.verify_email_url || "",
                description: data.description || "",
            }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["client", tenantId, clientId] })
            queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
            setIsEditing(false)
            toast({ title: "Cliente actualizado", description: "Los cambios han sido guardados." })
        },
        onError: (error: any) => {
            toast({ title: "Error", description: error.message || "No se pudo actualizar el cliente", variant: "destructive" })
        },
    })

    const deleteMutation = useMutation({
        mutationFn: () => api.delete(`/v2/admin/tenants/${tenantId}/clients/${clientId}`),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
            toast({ title: "Cliente eliminado", description: "El cliente ha sido eliminado permanentemente." })
            router.push(`/admin/tenants/${tenantId}/clients`)
        },
        onError: (error: any) => {
            toast({ title: "Error", description: error.message || "No se pudo eliminar el cliente", variant: "destructive" })
        },
    })

    const rotateSecretMutation = useMutation({
        mutationFn: () => api.post<{ client_id: string; new_secret: string }>(`/v2/admin/tenants/${tenantId}/clients/${clientId}/revoke`, {}),
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["client", tenantId, clientId] })
            queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
            setRotateSecretDialogOpen(false)

            if (data.new_secret) {
                sessionStorage.setItem(`client_secret_${data.client_id}`, data.new_secret)
            }
            const qs = new URLSearchParams({ created: "true" })
            router.push(`/admin/tenants/${tenantId}/clients/${data.client_id}?${qs.toString()}`)

            toast({ title: "Secret rotado", description: "El client_secret ha sido rotado. Guarda el nuevo secret ahora." })
        },
        onError: (error: any) => {
            toast({ title: "Error", description: error.message || "No se pudo rotar el secret", variant: "destructive" })
        },
    })

    // ========================================================================
    // DERIVED STATE
    // ========================================================================

    const isConfidential = subTypeFromUrl
        ? (subTypeFromUrl === "api_server" || subTypeFromUrl === "m2m")
        : client?.type === "confidential"

    const isM2M = subTypeFromUrl === "m2m"
    const TypeIcon = subTypeFromUrl ? TYPE_ICONS[subTypeFromUrl] : (isConfidential ? Server : Globe)

    const domain = typeof window !== "undefined"
        ? window.location.origin
        : "https://auth.example.com"

    const availableSdkTabs = getFilteredSdkTabs(subTypeFromUrl || undefined)

    const snippetConfig: SnippetConfig = {
        clientId,
        tenantSlug: tenant?.slug || "",
        domain,
        type: isConfidential ? "confidential" : "public",
        secret: secretFromStorage || undefined,
        subType: subTypeFromUrl || undefined,
    }

    const nextSteps = getNextSteps(selectedSdk, subTypeFromUrl || undefined)
    const canLeave = !isJustCreated || !isConfidential || !secretFromStorage || secretConfirmed

    const getTypeLabel = () => {
        if (subTypeFromUrl) return TYPE_LABELS[subTypeFromUrl]
        if (isM2M) return "Machine-to-Machine"
        if (isConfidential) return "Confidential"
        return "Public"
    }

    // Check if form has changes
    const hasChanges = useMemo(() => {
        if (!client) return false
        return (
            editForm.name !== client.name ||
            JSON.stringify(editForm.redirect_uris) !== JSON.stringify(client.redirect_uris || []) ||
            JSON.stringify(editForm.allowed_origins) !== JSON.stringify(client.allowed_origins || []) ||
            JSON.stringify(editForm.post_logout_uris) !== JSON.stringify(client.post_logout_uris || []) ||
            JSON.stringify(editForm.providers) !== JSON.stringify(client.providers || []) ||
            JSON.stringify(editForm.scopes) !== JSON.stringify(client.scopes || []) ||
            JSON.stringify(editForm.grant_types) !== JSON.stringify(client.grant_types || []) ||
            editForm.access_token_ttl !== (client.access_token_ttl || 15) ||
            editForm.refresh_token_ttl !== (client.refresh_token_ttl || 43200) ||
            editForm.id_token_ttl !== (client.id_token_ttl || 60) ||
            editForm.require_email_verification !== (client.require_email_verification || false) ||
            editForm.reset_password_url !== (client.reset_password_url || "") ||
            editForm.verify_email_url !== (client.verify_email_url || "") ||
            editForm.description !== (client.description || "")
        )
    }, [client, editForm])

    // ========================================================================
    // HANDLERS
    // ========================================================================

    const handleBack = () => {
        if (!canLeave) {
            toast({ title: "Confirma el secret", description: "Debes confirmar que guardaste el secret antes de salir.", variant: "destructive" })
            return
        }
        router.push(`/admin/tenants/${tenantId}/clients`)
    }

    const handleDone = () => {
        if (!canLeave) return
        router.push(`/admin/tenants/${tenantId}/clients`)
    }

    const handleSave = () => {
        updateMutation.mutate(editForm)
    }

    const handleCancelEdit = () => {
        setIsEditing(false)
        if (client) {
            setEditForm({
                name: client.name,
                redirect_uris: client.redirect_uris || [],
                allowed_origins: client.allowed_origins || [],
                post_logout_uris: client.post_logout_uris || [],
                providers: client.providers || [],
                scopes: client.scopes || [],
                grant_types: client.grant_types || [],
                access_token_ttl: client.access_token_ttl || 15,
                refresh_token_ttl: client.refresh_token_ttl || 43200,
                id_token_ttl: client.id_token_ttl || 60,
                require_email_verification: client.require_email_verification || false,
                reset_password_url: client.reset_password_url || "",
                verify_email_url: client.verify_email_url || "",
                description: client.description || "",
            })
        }
    }

    const toggleGrantType = (gtId: string) => {
        const current = editForm.grant_types || []
        if (current.includes(gtId)) {
            setEditForm({ ...editForm, grant_types: current.filter((g) => g !== gtId) })
        } else {
            setEditForm({ ...editForm, grant_types: [...current, gtId] })
        }
    }

    const toggleProvider = (pid: string) => {
        const current = editForm.providers || []
        if (current.includes(pid)) {
            setEditForm({ ...editForm, providers: current.filter((p) => p !== pid) })
        } else {
            setEditForm({ ...editForm, providers: [...current, pid] })
        }
    }

    const toggleScope = (scope: string) => {
        const current = editForm.scopes || []
        if (current.includes(scope)) {
            setEditForm({ ...editForm, scopes: current.filter((s) => s !== scope) })
        } else {
            setEditForm({ ...editForm, scopes: [...current, scope] })
        }
    }

    // ========================================================================
    // POST-CREATION VIEW (Quick Start Style)
    // ========================================================================

    if (isJustCreated) {
        return (
            <div className="space-y-6 animate-in fade-in duration-500">
                {/* Header */}
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

                    <div className="relative">
                        {isConfidential && secretFromStorage && !secretConfirmed && (
                            <p className="absolute -top-6 right-0 text-xs text-amber-500 flex items-center gap-1.5 whitespace-nowrap">
                                <AlertTriangle className="h-3.5 w-3.5" />
                                Confirma que guardaste el secret
                            </p>
                        )}
                        <Button onClick={handleDone} disabled={!canLeave} className="min-w-[100px]">
                            Listo
                        </Button>
                    </div>
                </div>

                {/* Grid principal */}
                <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-6 items-start">
                    {/* LEFT - Credentials */}
                    <Card className="border-success/30 bg-gradient-to-br from-success/5 via-background to-background">
                        <CardHeader>
                            <CardTitle className="text-lg">Credenciales del cliente</CardTitle>
                            <CardDescription>Guarda estas credenciales de forma segura</CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            {/* Client ID */}
                            <div className="space-y-2">
                                <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Client ID</Label>
                                <div className="flex items-center gap-2">
                                    <code className="flex-1 rounded-lg bg-muted/50 border px-4 py-3 font-mono text-sm overflow-hidden text-ellipsis">
                                        {clientId}
                                    </code>
                                    <Button variant="ghost" size="sm" onClick={() => copy(clientId, "client_id")} className="shrink-0 h-10 w-10 p-0">
                                        {copied === "client_id" ? <Check className="h-4 w-4 text-success" /> : <Copy className="h-4 w-4" />}
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
                                        <Button variant="ghost" size="sm" onClick={() => setShowSecret(!showSecret)} className="shrink-0 h-10 w-10 p-0">
                                            {showSecret ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                                        </Button>
                                        <Button variant="ghost" size="sm" onClick={() => copy(secretFromStorage, "secret")} className="shrink-0 h-10 w-10 p-0">
                                            {copied === "secret" ? <Check className="h-4 w-4 text-success" /> : <Copy className="h-4 w-4" />}
                                        </Button>
                                    </div>

                                    <div className="space-y-3">
                                        <div className="rounded-lg bg-amber-500/10 border border-amber-500/20 p-3 flex items-start gap-2.5">
                                            <AlertTriangle className="h-4 w-4 text-amber-500 shrink-0 mt-0.5" />
                                            <p className="text-xs text-amber-600 dark:text-amber-400 leading-relaxed">
                                                <strong>Guarda este secret ahora.</strong> No podrás verlo de nuevo.
                                            </p>
                                        </div>
                                        <label className="flex items-center gap-3 p-3 rounded-lg border bg-muted/30 cursor-pointer hover:bg-muted/50 transition-colors">
                                            <Checkbox checked={secretConfirmed} onCheckedChange={(checked) => setSecretConfirmed(!!checked)} />
                                            <span className="text-sm leading-relaxed">
                                                Confirmo que guardé el secret de forma segura
                                            </span>
                                        </label>
                                    </div>
                                </div>
                            )}
                        </CardContent>
                    </Card>

                    {/* RIGHT - Next Steps */}
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
                                        <p className="text-sm text-muted-foreground leading-relaxed">{step}</p>
                                    </div>
                                ))}
                            </div>
                        </CardContent>
                    </Card>
                </div>

                {/* BOTTOM - SDK Integration */}
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
                                    <TabsTrigger key={tab.id} value={tab.id} className="text-sm font-medium">
                                        {tab.label}
                                    </TabsTrigger>
                                ))}
                            </TabsList>

                            {availableSdkTabs.map((tab) => (
                                <TabsContent key={tab.id} value={tab.id} className="space-y-4 mt-0">
                                    <div className="flex items-center gap-3 p-3 rounded-lg bg-muted/50 border max-w-2xl overflow-hidden">
                                        <Terminal className="h-4 w-4 text-muted-foreground shrink-0" />
                                        <code className="text-sm font-mono flex-1 overflow-hidden text-ellipsis">{tab.installCmd}</code>
                                        <Button variant="ghost" size="sm" onClick={() => copy(tab.installCmd, `install-${tab.id}`)} className="h-8 w-8 p-0 shrink-0">
                                            {copied === `install-${tab.id}` ? <Check className="h-4 w-4 text-success" /> : <Copy className="h-4 w-4" />}
                                        </Button>
                                    </div>
                                    <CodeSnippet code={getSnippet(tab.id, snippetConfig)} language={tab.language} filename={tab.filename} />
                                </TabsContent>
                            ))}
                        </Tabs>
                    </CardContent>
                </Card>
            </div>
        )
    }

    // ========================================================================
    // LOADING STATE
    // ========================================================================

    if (isLoading) {
        return (
            <div className="flex items-center justify-center min-h-[400px]">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
        )
    }

    // ========================================================================
    // ERROR / NOT FOUND
    // ========================================================================

    if (clientError || !client) {
        return (
            <div className="flex flex-col items-center justify-center min-h-[400px] gap-4">
                <div className="p-4 rounded-xl bg-warning/10 border border-warning/20">
                    <AlertTriangle className="h-10 w-10 text-warning" />
                </div>
                <h2 className="text-xl font-semibold">Cliente no encontrado</h2>
                <p className="text-sm text-muted-foreground max-w-sm text-center">
                    El cliente <code className="bg-muted px-1.5 py-0.5 rounded text-xs">{clientId}</code> no existe o no tienes acceso.
                </p>
                <Button variant="outline" onClick={() => router.push(`/admin/tenants/${tenantId}/clients`)}>
                    <ArrowLeft className="h-4 w-4 mr-2" />
                    Volver a Clients
                </Button>
            </div>
        )
    }

    // ========================================================================
    // NORMAL DETAIL VIEW
    // ========================================================================

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
                            <div className="flex items-center gap-3">
                                <h1 className="text-2xl font-bold tracking-tight">{client.name}</h1>
                                <Badge variant={client.type === "confidential" ? "warning" : "success"} className="text-xs">
                                    {client.type === "confidential" ? "🔒 Confidential" : "🌐 Public"}
                                </Badge>
                                {client.type === "public" && (
                                    <Badge variant="info" className="text-[10px]">PKCE automático</Badge>
                                )}
                            </div>
                            <div className="flex items-center gap-2 mt-1 text-sm text-muted-foreground">
                                <code className="text-xs bg-muted px-2 py-0.5 rounded font-mono">{client.client_id}</code>
                                {client.description && (
                                    <span className="text-xs text-muted-foreground/70">— {client.description}</span>
                                )}
                                <Separator orientation="vertical" className="h-3" />
                                <span className="text-[11px] text-muted-foreground/60">
                                    {client.created_at ? `Creado ${formatRelativeTime(client.created_at)}` : ""}
                                    {client.updated_at ? ` · Editado ${formatRelativeTime(client.updated_at)}` : ""}
                                </span>
                            </div>
                        </div>
                    </div>
                </div>

                {/* Actions */}
                <div className="flex items-center gap-2">
                    {isEditing ? (
                        <>
                            <Button variant="outline" size="sm" onClick={handleCancelEdit}>
                                Cancelar
                            </Button>
                            <Button size="sm" onClick={handleSave} disabled={!hasChanges || updateMutation.isPending}>
                                {updateMutation.isPending ? (
                                    <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Guardando...</>
                                ) : (
                                    <><Save className="mr-2 h-4 w-4" /> Guardar cambios</>
                                )}
                            </Button>
                        </>
                    ) : (
                        <Button variant="outline" size="sm" onClick={() => setIsEditing(true)}>
                            <Settings2 className="mr-2 h-4 w-4" />
                            Editar
                        </Button>
                    )}
                </div>
            </div>

            {/* Main Content Tabs */}
            <Tabs defaultValue="overview">
                <TabsList className="grid w-full max-w-2xl grid-cols-5">
                    <TabsTrigger value="overview" className="flex items-center gap-2">
                        <Info className="h-4 w-4" />
                        <span className="hidden sm:inline">Resumen</span>
                    </TabsTrigger>
                    <TabsTrigger value="config" className="flex items-center gap-2">
                        <Settings2 className="h-4 w-4" />
                        <span className="hidden sm:inline">Config</span>
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
                    TAB: OVERVIEW (Resumen)
                    ============================================================ */}
                <TabsContent value="overview" className="mt-6 space-y-6">
                    {/* Client ID (read-only) */}
                    <Card>
                        <CardHeader>
                            <CardTitle className="flex items-center gap-2 text-base">
                                <Key className="h-4 w-4" />
                                Client ID
                            </CardTitle>
                        </CardHeader>
                        <CardContent>
                            <div className="flex items-center gap-2">
                                <code className="flex-1 rounded-lg bg-muted px-4 py-2.5 text-sm font-mono">{client.client_id}</code>
                                <Button variant="outline" size="sm" onClick={() => copy(client.client_id, "client_id")}>
                                    {copied === "client_id" ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                                </Button>
                            </div>
                        </CardContent>
                    </Card>

                    {/* Name + Description */}
                    <Card>
                        <CardHeader>
                            <CardTitle className="flex items-center gap-2 text-base">
                                Nombre y descripción
                                <InfoTooltip content="Nombre descriptivo del cliente para identificarlo en el dashboard." />
                            </CardTitle>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            {isEditing ? (
                                <>
                                    <div className="space-y-1.5">
                                        <Label className="text-xs text-muted-foreground">Nombre</Label>
                                        <Input
                                            value={editForm.name || ""}
                                            onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
                                            placeholder="Mi Aplicación"
                                        />
                                    </div>
                                    <div className="space-y-1.5">
                                        <Label className="text-xs text-muted-foreground">Descripción (opcional)</Label>
                                        <Input
                                            value={editForm.description || ""}
                                            onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
                                            placeholder="Breve descripción del propósito de este cliente..."
                                        />
                                    </div>
                                </>
                            ) : (
                                <>
                                    <p className="text-sm font-medium">{client.name}</p>
                                    <p className="text-sm text-muted-foreground">{client.description || "Sin descripción"}</p>
                                </>
                            )}
                        </CardContent>
                    </Card>

                    {/* Type-Specific Overview */}
                    <Card className={cn(
                        "border-l-4",
                        client.type === "confidential" ? "border-l-amber-500" : "border-l-green-500"
                    )}>
                        <CardHeader className="pb-3">
                            <CardTitle className="text-base flex items-center gap-2">
                                {client.type === "confidential" ? <Lock className="h-4 w-4 text-amber-500" /> : <Globe className="h-4 w-4 text-success" />}
                                {client.type === "confidential" ? "Cliente Confidential" : "Cliente Público"}
                            </CardTitle>
                        </CardHeader>
                        <CardContent>
                            {client.type === "confidential" ? (
                                <div className="space-y-3 text-sm text-muted-foreground">
                                    <p>Este cliente puede guardar secretos de forma segura (backend, servidor).</p>
                                    <div className="flex flex-wrap gap-2">
                                        <Badge variant="warning" className="text-xs">🔑 Tiene client_secret</Badge>
                                        <Badge variant="secondary" className="text-xs">Auth Code + PKCE</Badge>
                                        {(client.grant_types || []).includes("client_credentials") && (
                                            <Badge variant="info" className="text-xs">Client Credentials</Badge>
                                        )}
                                    </div>
                                </div>
                            ) : (
                                <div className="space-y-3 text-sm text-muted-foreground">
                                    <p>Este cliente corre en un entorno no confiable (navegador, app móvil). Usa PKCE automáticamente.</p>
                                    <div className="flex flex-wrap gap-2">
                                        <Badge variant="success" className="text-xs">🛡️ PKCE automático</Badge>
                                        <Badge variant="secondary" className="text-xs">Sin client_secret</Badge>
                                        <Badge variant="secondary" className="text-xs">Auth Code Flow</Badge>
                                    </div>
                                </div>
                            )}
                        </CardContent>
                    </Card>

                    {/* Quick Stats */}
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                        <div className="p-4 rounded-xl border bg-muted/30 space-y-1">
                            <p className="text-xs text-muted-foreground">Redirect URIs</p>
                            <p className="text-xl font-bold">{(client.redirect_uris || []).length}</p>
                        </div>
                        <div className="p-4 rounded-xl border bg-muted/30 space-y-1">
                            <p className="text-xs text-muted-foreground">Scopes</p>
                            <p className="text-xl font-bold">{(client.scopes || []).length}</p>
                        </div>
                        <div className="p-4 rounded-xl border bg-muted/30 space-y-1">
                            <p className="text-xs text-muted-foreground">Providers</p>
                            <p className="text-xl font-bold">{(client.providers || []).length}</p>
                        </div>
                        <div className="p-4 rounded-xl border bg-muted/30 space-y-1">
                            <p className="text-xs text-muted-foreground">Grant Types</p>
                            <p className="text-xl font-bold">{(client.grant_types || []).length}</p>
                        </div>
                    </div>

                    {/* Redirect URIs */}
                    <Card>
                        <CardHeader>
                            <CardTitle className="flex items-center gap-2 text-base">
                                <Link2 className="h-4 w-4" />
                                URIs de redirección
                                <InfoTooltip content="URLs permitidas para redirección después del login. Deben coincidir exactamente con las configuradas en tu aplicación." />
                            </CardTitle>
                        </CardHeader>
                        <CardContent>
                            {isEditing ? (
                                <EditableUriList
                                    label=""
                                    icon={Link2}
                                    tooltip=""
                                    values={editForm.redirect_uris || []}
                                    onChange={(uris) => setEditForm({ ...editForm, redirect_uris: uris })}
                                    placeholder="https://app.example.com/callback"
                                />
                            ) : (
                                (client.redirect_uris && client.redirect_uris.length > 0) ? (
                                    <div className="space-y-2">
                                        {client.redirect_uris.map((uri) => (
                                            <div key={uri} className="flex items-center gap-2 rounded-lg bg-muted/50 border p-2.5">
                                                <Link2 className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
                                                <code className="text-sm flex-1 truncate">{uri}</code>
                                                <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={() => copy(uri, `uri-${uri}`)}>
                                                    {copied === `uri-${uri}` ? <Check className="h-3 w-3 text-success" /> : <Copy className="h-3 w-3" />}
                                                </Button>
                                            </div>
                                        ))}
                                    </div>
                                ) : (
                                    <p className="text-sm text-muted-foreground italic">Sin URIs configuradas</p>
                                )
                            )}
                        </CardContent>
                    </Card>

                    {/* Allowed Origins (public only) */}
                    {(client.type === "public" || isEditing) && (
                        <Card>
                            <CardHeader>
                                <CardTitle className="flex items-center gap-2 text-base">
                                    <Globe className="h-4 w-4" />
                                    Orígenes permitidos (CORS)
                                    <InfoTooltip content="Dominios desde los que se permiten requests al servidor de autenticación." />
                                </CardTitle>
                            </CardHeader>
                            <CardContent>
                                {isEditing ? (
                                    <EditableUriList
                                        label=""
                                        icon={Globe}
                                        tooltip=""
                                        values={editForm.allowed_origins || []}
                                        onChange={(origins) => setEditForm({ ...editForm, allowed_origins: origins })}
                                        placeholder="https://app.example.com"
                                    />
                                ) : (
                                    (client.allowed_origins && client.allowed_origins.length > 0) ? (
                                        <div className="space-y-2">
                                            {client.allowed_origins.map((origin) => (
                                                <div key={origin} className="flex items-center gap-2 rounded-lg bg-muted/50 border p-2.5">
                                                    <Globe className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
                                                    <code className="text-sm">{origin}</code>
                                                </div>
                                            ))}
                                        </div>
                                    ) : (
                                        <p className="text-sm text-muted-foreground italic">Sin orígenes configurados</p>
                                    )
                                )}
                            </CardContent>
                        </Card>
                    )}

                    {/* Post Logout URIs */}
                    {(isEditing || (client.post_logout_uris && client.post_logout_uris.length > 0)) && (
                        <Card>
                            <CardHeader>
                                <CardTitle className="flex items-center gap-2 text-base">
                                    <ExternalLink className="h-4 w-4" />
                                    Post Logout URIs
                                    <InfoTooltip content="URLs permitidas para redirección después del logout." />
                                </CardTitle>
                            </CardHeader>
                            <CardContent>
                                {isEditing ? (
                                    <EditableUriList
                                        label=""
                                        icon={ExternalLink}
                                        tooltip=""
                                        values={editForm.post_logout_uris || []}
                                        onChange={(uris) => setEditForm({ ...editForm, post_logout_uris: uris })}
                                        placeholder="https://app.example.com"
                                    />
                                ) : (
                                    <div className="space-y-2">
                                        {(client.post_logout_uris || []).map((uri) => (
                                            <div key={uri} className="flex items-center gap-2 rounded-lg bg-muted/50 border p-2.5">
                                                <ExternalLink className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
                                                <code className="text-sm">{uri}</code>
                                            </div>
                                        ))}
                                    </div>
                                )}
                            </CardContent>
                        </Card>
                    )}

                    {/* Scopes */}
                    <Card>
                        <CardHeader>
                            <CardTitle className="flex items-center gap-2 text-base">
                                Scopes permitidos
                                <InfoTooltip content="Scopes que este cliente puede solicitar en los tokens." />
                            </CardTitle>
                        </CardHeader>
                        <CardContent>
                            {isEditing ? (
                                <div className="flex flex-wrap gap-2">
                                    {PREDEFINED_SCOPES.map((scope) => {
                                        const active = (editForm.scopes || []).includes(scope.id)
                                        return (
                                            <Badge
                                                key={scope.id}
                                                variant={active ? "default" : "outline"}
                                                className={cn("cursor-pointer transition-all hover:scale-105", active && "bg-primary text-primary-foreground")}
                                                onClick={() => toggleScope(scope.id)}
                                            >
                                                {active ? "✓ " : ""}{scope.id}
                                            </Badge>
                                        )
                                    })}
                                </div>
                            ) : (
                                <div className="flex flex-wrap gap-2">
                                    {(client.scopes && client.scopes.length > 0 ? client.scopes : ["openid", "profile", "email"]).map((scope) => (
                                        <Badge key={scope} variant="outline">{scope}</Badge>
                                    ))}
                                </div>
                            )}
                        </CardContent>
                    </Card>

                    {/* Providers */}
                    <Card>
                        <CardHeader>
                            <CardTitle className="text-base">
                                Proveedores de autenticación
                                <InfoTooltip content="Métodos de autenticación disponibles para los usuarios de este cliente." />
                            </CardTitle>
                        </CardHeader>
                        <CardContent>
                            {isEditing ? (
                                <div className="grid gap-2">
                                    {AVAILABLE_PROVIDERS.map((p) => {
                                        const active = (editForm.providers || []).includes(p.id)
                                        return (
                                            <div
                                                key={p.id}
                                                className={cn(
                                                    "flex items-center justify-between p-3 rounded-lg border transition-all",
                                                    p.enabled ? (active ? "bg-success/5 border-success/20 shadow-sm" : "bg-muted/30 hover:bg-muted/50") : "opacity-50 cursor-not-allowed"
                                                )}
                                                onClick={() => p.enabled && toggleProvider(p.id)}
                                            >
                                                <div className="flex items-center gap-3 cursor-pointer">
                                                    {active ? <CheckCircle className="h-4 w-4 text-success" /> : <XCircle className="h-4 w-4 text-muted-foreground" />}
                                                    <span className="text-sm">{p.icon} {p.label}</span>
                                                </div>
                                                {p.comingSoon && <Badge variant="secondary" className="text-[10px]">Próximamente</Badge>}
                                            </div>
                                        )
                                    })}
                                </div>
                            ) : (
                                (client.providers && client.providers.length > 0) ? (
                                    <div className="flex flex-wrap gap-2">
                                        {client.providers.map((p) => {
                                            const provider = AVAILABLE_PROVIDERS.find((pr) => pr.id === p)
                                            return (
                                                <Badge key={p} variant="secondary">
                                                    {provider?.icon} {provider?.label || p}
                                                </Badge>
                                            )
                                        })}
                                    </div>
                                ) : (
                                    <p className="text-sm text-muted-foreground italic">Sin proveedores configurados</p>
                                )
                            )}
                        </CardContent>
                    </Card>
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
                                    <code className="flex-1 rounded-lg bg-muted px-4 py-2.5 text-sm font-mono text-muted-foreground">
                                        •••••••••••••••••••••••••••••••••••••••
                                    </code>
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        onClick={() => setRotateSecretDialogOpen(true)}
                                    >
                                        <RotateCcw className="h-4 w-4 mr-2" />
                                        Rotar Secret
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
                                    const enabled = isEditing
                                        ? (editForm.grant_types || []).includes(gt.id)
                                        : (client.grant_types?.includes(gt.id) || (gt.id === "authorization_code" && !client.grant_types?.length))

                                    const isClickable = isEditing && (!gt.confidentialOnly || client.type === "confidential")

                                    return (
                                        <div
                                            key={gt.id}
                                            className={cn(
                                                "flex items-center justify-between p-3 rounded-lg border transition-all",
                                                enabled ? "bg-success/5 border-success/20 shadow-sm" : "bg-muted/30",
                                                isClickable && "cursor-pointer hover:bg-muted/50"
                                            )}
                                            onClick={() => isClickable && toggleGrantType(gt.id)}
                                        >
                                            <div className="flex items-center gap-3">
                                                {enabled ? (
                                                    <CheckCircle className="h-4 w-4 text-success" />
                                                ) : (
                                                    <XCircle className="h-4 w-4 text-muted-foreground" />
                                                )}
                                                <div>
                                                    <div className="flex items-center gap-2">
                                                        <p className="font-medium text-sm">{gt.label}</p>
                                                        {gt.confidentialOnly && <Badge variant="outline" className="text-[10px] h-4">Backend</Badge>}
                                                    </div>
                                                    <p className="text-xs text-muted-foreground">{gt.description}</p>
                                                </div>
                                            </div>
                                            <div className="flex items-center gap-2">
                                                {gt.deprecated && <Badge variant="destructive" className="text-[10px]">Deprecado</Badge>}
                                                {gt.recommended && enabled && <Badge variant="success" className="text-[10px]">Recomendado</Badge>}
                                            </div>
                                        </div>
                                    )
                                })}
                            </div>
                        </CardContent>
                    </Card>

                    {/* Email Verification */}
                    <Card>
                        <CardHeader>
                            <CardTitle className="text-base flex items-center gap-2">
                                Verificación de email
                                <InfoTooltip content="Requiere que los usuarios verifiquen su email antes de poder autenticarse." />
                            </CardTitle>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            {isEditing ? (
                                <>
                                    <div className="flex items-center gap-4 p-3 rounded-lg border bg-muted/20">
                                        <Switch
                                            checked={editForm.require_email_verification || false}
                                            onCheckedChange={(checked) => setEditForm({ ...editForm, require_email_verification: !!checked })}
                                        />
                                        <Label className="text-sm font-medium cursor-pointer" onClick={() => setEditForm({ ...editForm, require_email_verification: !editForm.require_email_verification })}>
                                            Requerir verificación de email
                                        </Label>
                                    </div>

                                    {editForm.require_email_verification && (
                                        <div className="space-y-3 pl-2 animate-in slide-in-from-top-2 duration-200">
                                            <div className="space-y-1.5">
                                                <Label className="text-xs">URL de verificación de email</Label>
                                                <Input
                                                    value={editForm.verify_email_url || ""}
                                                    onChange={(e) => setEditForm({ ...editForm, verify_email_url: e.target.value })}
                                                    placeholder="https://app.example.com/verify-email"
                                                />
                                            </div>
                                            <div className="space-y-1.5">
                                                <Label className="text-xs">URL de reset de password</Label>
                                                <Input
                                                    value={editForm.reset_password_url || ""}
                                                    onChange={(e) => setEditForm({ ...editForm, reset_password_url: e.target.value })}
                                                    placeholder="https://app.example.com/reset-password"
                                                />
                                            </div>
                                        </div>
                                    )}
                                </>
                            ) : (
                                <div className="flex items-center gap-3">
                                    {client.require_email_verification ? (
                                        <CheckCircle className="h-4 w-4 text-success" />
                                    ) : (
                                        <XCircle className="h-4 w-4 text-muted-foreground" />
                                    )}
                                    <span className="text-sm">
                                        {client.require_email_verification ? "Verificación de email requerida" : "Verificación de email no requerida"}
                                    </span>
                                </div>
                            )}
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

                    {/* Danger Zone */}
                    <Card className="border-danger/30">
                        <CardHeader>
                            <CardTitle className="text-base text-danger flex items-center gap-2">
                                <AlertTriangle className="h-4 w-4" />
                                Zona de peligro
                            </CardTitle>
                        </CardHeader>
                        <CardContent>
                            <div className="flex items-center justify-between">
                                <div>
                                    <p className="text-sm font-medium">Eliminar cliente</p>
                                    <p className="text-xs text-muted-foreground">
                                        Esta acción es irreversible y revocará todos los tokens activos.
                                    </p>
                                </div>
                                <Button variant="danger" size="sm" onClick={() => setDeleteDialogOpen(true)}>
                                    <Trash2 className="h-4 w-4 mr-2" />
                                    Eliminar
                                </Button>
                            </div>
                        </CardContent>
                    </Card>
                </TabsContent>

                {/* ============================================================
                    TAB: TOKENS
                    ============================================================ */}
                <TabsContent value="tokens" className="mt-6 space-y-6">
                    <div className="grid gap-6">
                        {[
                            {
                                label: "Access Token TTL",
                                description: "Tiempo de vida del token de acceso (corto plazo).",
                                icon: Zap,
                                color: "text-warning",
                                value: client.access_token_ttl || 15,
                                field: "access_token_ttl" as const,
                                options: TOKEN_TTL_OPTIONS.access,
                                min: 5,
                                max: 1440
                            },
                            {
                                label: "Refresh Token TTL",
                                description: "Tiempo de vida del token de refresco (largo plazo).",
                                icon: RefreshCw,
                                color: "text-success",
                                value: client.refresh_token_ttl || 43200,
                                field: "refresh_token_ttl" as const,
                                options: TOKEN_TTL_OPTIONS.refresh,
                                min: 60,
                                max: 525600 // 1 year
                            },
                            {
                                label: "ID Token TTL",
                                description: "Tiempo de vida del token de identidad (OIDC).",
                                icon: FileCode2,
                                color: "text-info",
                                value: client.id_token_ttl || 60,
                                field: "id_token_ttl" as const,
                                options: TOKEN_TTL_OPTIONS.id,
                                min: 5,
                                max: 1440
                            },
                        ].map((token) => (
                            <Card key={token.field}>
                                <CardHeader className="pb-4">
                                    <div className="flex items-center justify-between">
                                        <CardTitle className="text-base flex items-center gap-2">
                                            <div className={cn("p-2 rounded-lg bg-muted/50", token.color)}>
                                                <token.icon className="h-4 w-4" />
                                            </div>
                                            {token.label}
                                        </CardTitle>
                                        <Badge variant="outline" className="text-sm font-mono">
                                            {formatTTL(isEditing ? (editForm[token.field] || token.value) : token.value)}
                                        </Badge>
                                    </div>
                                    <CardDescription>{token.description}</CardDescription>
                                </CardHeader>
                                <CardContent>
                                    {isEditing ? (
                                        <div className="space-y-6">
                                            {/* Presets */}
                                            <div className="flex flex-wrap gap-2">
                                                {token.options.map((opt) => (
                                                    <Badge
                                                        key={opt.value}
                                                        variant={(editForm[token.field] || token.value) === opt.value ? "default" : "outline"}
                                                        className={cn("cursor-pointer transition-all hover:scale-105", (editForm[token.field] || token.value) === opt.value && "bg-primary text-primary-foreground")}
                                                        onClick={() => setEditForm({ ...editForm, [token.field]: opt.value })}
                                                    >
                                                        {opt.label}
                                                    </Badge>
                                                ))}
                                            </div>

                                            {/* Custom Slider */}
                                            <div className="space-y-4 pt-2">
                                                <div className="flex justify-between text-xs text-muted-foreground">
                                                    <span>Min: {formatTTL(token.min)}</span>
                                                    <span>Max: {formatTTL(token.max)}</span>
                                                </div>
                                                <Slider
                                                    min={token.min}
                                                    max={token.max}
                                                    step={5}
                                                    value={[editForm[token.field] || token.value]}
                                                    onValueChange={(val: number[]) => setEditForm({ ...editForm, [token.field]: val[0] })}
                                                />
                                            </div>
                                        </div>
                                    ) : (
                                        <div className="relative pt-1">
                                            <div className="overflow-hidden h-2 mb-4 text-xs flex rounded bg-muted/50">
                                                <div
                                                    style={{ width: `${Math.min(100, Math.max(5, ((token.value - token.min) / (token.max - token.min)) * 100))}%` }}
                                                    className={cn("shadow-none flex flex-col text-center whitespace-nowrap text-white justify-center transition-all duration-500",
                                                        token.field === "access_token_ttl" ? "bg-warning" :
                                                            token.field === "refresh_token_ttl" ? "bg-success" : "bg-info"
                                                    )}
                                                ></div>
                                            </div>
                                            <div className="flex justify-between text-xs text-muted-foreground">
                                                <span>{formatTTL(token.min)}</span>
                                                <span className="font-medium text-foreground">{formatTTL(token.value)}</span>
                                                <span>{formatTTL(token.max)}</span>
                                            </div>
                                        </div>
                                    )}
                                </CardContent>
                            </Card>
                        ))}
                    </div>

                    <InlineAlert
                        variant="default"
                        description="Los tiempos de vida de tokens se pueden modificar haciendo click en Editar. Valores más cortos son más seguros pero requieren renovación más frecuente."
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
                                        <TabsTrigger key={tab.id} value={tab.id} className="text-sm font-medium">
                                            {tab.label}
                                        </TabsTrigger>
                                    ))}
                                </TabsList>

                                {availableSdkTabs.map((tab) => (
                                    <TabsContent key={tab.id} value={tab.id} className="space-y-4 mt-0">
                                        <div className="flex items-center gap-3 p-3 rounded-lg bg-muted/50 border max-w-2xl">
                                            <Terminal className="h-4 w-4 text-muted-foreground shrink-0" />
                                            <code className="text-sm font-mono flex-1 truncate">{tab.installCmd}</code>
                                            <Button variant="ghost" size="sm" onClick={() => copy(tab.installCmd, `install-${tab.id}`)} className="h-8 w-8 p-0 shrink-0">
                                                {copied === `install-${tab.id}` ? <Check className="h-4 w-4 text-success" /> : <Copy className="h-4 w-4" />}
                                            </Button>
                                        </div>
                                        <CodeSnippet
                                            code={getSnippet(tab.id, {
                                                clientId: client.client_id,
                                                tenantSlug: tenant?.slug || "",
                                                domain,
                                                type: client.type as "public" | "confidential",
                                                secret: undefined,
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

            {/* ============================================================
                DELETE CONFIRMATION DIALOG
                ============================================================ */}
            <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2 text-danger">
                            <AlertTriangle className="h-5 w-5" />
                            Eliminar Client
                        </DialogTitle>
                        <DialogDescription>
                            ¿Estás seguro de que deseas eliminar <strong>{client.name}</strong>?
                            Esta acción es irreversible y revocará todos los tokens activos.
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>Cancelar</Button>
                        <Button variant="danger" onClick={() => deleteMutation.mutate()} disabled={deleteMutation.isPending}>
                            {deleteMutation.isPending ? (
                                <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Eliminando...</>
                            ) : (
                                "Eliminar permanentemente"
                            )}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* ============================================================
                ROTATE SECRET CONFIRMATION DIALOG
                ============================================================ */}
            <Dialog open={rotateSecretDialogOpen} onOpenChange={setRotateSecretDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2 text-warning">
                            <RotateCcw className="h-5 w-5" />
                            Rotar Client Secret
                        </DialogTitle>
                        <DialogDescription>
                            ¿Estás seguro de que deseas rotar el secret de <strong>{client.name}</strong>?
                            El secret actual dejará de funcionar inmediatamente.
                        </DialogDescription>
                    </DialogHeader>
                    <InlineAlert
                        variant="danger"
                        className="my-4"
                        description="Todas las aplicaciones que usen el secret actual dejarán de funcionar hasta que actualices la configuración con el nuevo secret."
                    />
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setRotateSecretDialogOpen(false)}>Cancelar</Button>
                        <Button variant="danger" onClick={() => rotateSecretMutation.mutate()} disabled={rotateSecretMutation.isPending}>
                            {rotateSecretMutation.isPending ? (
                                <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Rotando...</>
                            ) : (
                                "Rotar Secret"
                            )}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
