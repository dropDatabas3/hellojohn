"use client"

import { useState, useMemo, useEffect } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useParams } from "next/navigation"
import {
    Plus,
    Search,
    Trash2,
    Eye,
    Copy,
    Check,
    ArrowLeft,
    Globe,
    Server,
    ChevronRight,
    ChevronLeft,
    Sparkles,
    Shield,
    ShieldCheck,
    Key,
    KeyRound,
    RefreshCw,
    Settings2,
    AlertTriangle,
    CheckCircle2,
    XCircle,
    MoreHorizontal,
    Link2,
    LogOut,
    FileCode2,
    Zap,
    EyeOff,
    RotateCcw,
    Loader2,
    Monitor,
    Info,
} from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import Link from "next/link"
import { useToast } from "@/hooks/use-toast"
import type { Client, ClientInput, Tenant } from "@/lib/types"

// DS Components (UI Unification)
import {
    Button,
    Input,
    Card, CardContent, CardDescription, CardHeader, CardTitle, CardFooter,
    Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
    Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
    Label,
    Badge,
    Switch,
    Checkbox,
    Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
    Tabs, TabsContent, TabsList, TabsTrigger,
    Textarea,
    InlineAlert,
    DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger,
    Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
    Skeleton,
    cn,
} from "@/components/ds"

// ============================================================================
// TYPES
// ============================================================================

type ClientType = "public" | "confidential"

interface ClientFormState {
    name: string
    clientId: string
    type: ClientType
    description: string
    redirectUris: string[]
    allowedOrigins: string[]
    postLogoutUris: string[]
    scopes: string[]
    providers: string[]
    grantTypes: string[]
    accessTokenTTL: number
    refreshTokenTTL: number
    idTokenTTL: number
    requireEmailVerification: boolean
    resetPasswordUrl: string
    verifyEmailUrl: string
    frontChannelLogoutUrl: string
    backChannelLogoutUrl: string
}

interface ClientRow {
    id: string
    client_id: string
    name: string
    type: "public" | "confidential"
    redirect_uris: string[]
    allowed_origins?: string[]
    post_logout_uris?: string[]
    providers?: string[]
    scopes?: string[]
    grant_types?: string[]
    secret?: string
    secret_hash?: string
    access_token_ttl?: number
    refresh_token_ttl?: number
    id_token_ttl?: number
    require_email_verification?: boolean
    reset_password_url?: string
    verify_email_url?: string
    front_channel_logout_url?: string
    back_channel_logout_url?: string
    created_at?: string
    updated_at?: string
}

// ============================================================================
// CONSTANTS
// ============================================================================

const GRANT_TYPES = [
    { id: "authorization_code", label: "Authorization Code", description: "Flujo OAuth2 estándar con PKCE", recommended: true },
    { id: "refresh_token", label: "Refresh Token", description: "Permitir renovación de tokens", recommended: true },
    { id: "client_credentials", label: "Client Credentials", description: "Autenticación M2M (solo backend)", confidentialOnly: true },
    { id: "implicit", label: "Implicit", description: "Flujo legacy - NO RECOMENDADO", deprecated: true },
]

const DEFAULT_SCOPES = ["openid", "profile", "email", "offline_access"]

const AVAILABLE_PROVIDERS = [
    { id: "password", label: "Email + Password", icon: "🔑", enabled: true },
    { id: "google", label: "Google", icon: "G", enabled: true },
    { id: "github", label: "GitHub", icon: "🐙", enabled: false, comingSoon: true },
    { id: "apple", label: "Apple", icon: "🍎", enabled: false, comingSoon: true },
]

const DEFAULT_FORM: ClientFormState = {
    name: "",
    clientId: "",
    type: "public",
    description: "",
    redirectUris: [],
    allowedOrigins: [],
    postLogoutUris: [],
    scopes: ["openid", "profile", "email"],
    providers: ["password"],
    grantTypes: ["authorization_code", "refresh_token"],
    accessTokenTTL: 15,
    refreshTokenTTL: 43200,
    idTokenTTL: 60,
    requireEmailVerification: false,
    resetPasswordUrl: "",
    verifyEmailUrl: "",
    frontChannelLogoutUrl: "",
    backChannelLogoutUrl: "",
}

// ============================================================================
// HELPERS
// ============================================================================

function slugify(text: string): string {
    return text
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "_")
        .replace(/^_|_$/g, "")
        .slice(0, 20)
}

function generateClientId(tenantSlug: string, name: string, type: ClientType): string {
    const nameSlug = slugify(name)
    const typeShort = type === "public" ? "web" : "srv"
    const rand = Math.random().toString(36).substring(2, 6)
    return `${tenantSlug}_${nameSlug}_${typeShort}_${rand}`
}

function formatTTL(minutes: number): string {
    if (minutes < 60) return `${minutes} min`
    if (minutes < 1440) return `${Math.round(minutes / 60)}h`
    return `${Math.round(minutes / 1440)}d`
}

function formatRelativeTime(date: string): string {
    if (!date) return "—"
    const now = Date.now()
    const ts = new Date(date).getTime()
    const diff = now - ts
    if (diff < 60 * 1000) return "hace menos de 1 min"
    if (diff < 60 * 60 * 1000) return `hace ${Math.floor(diff / 60000)} min`
    if (diff < 24 * 60 * 60 * 1000) return `hace ${Math.floor(diff / 3600000)}h`
    return `hace ${Math.floor(diff / 86400000)} días`
}

// ============================================================================
// HELPER COMPONENTS
// ============================================================================

function InfoTooltip({ content }: { content: string }) {
    return (
        <TooltipProvider>
            <Tooltip>
                <TooltipTrigger asChild>
                    <Info className="h-3.5 w-3.5 text-muted-foreground cursor-help ml-1 inline" />
                </TooltipTrigger>
                <TooltipContent className="max-w-xs">
                    <p className="text-xs">{content}</p>
                </TooltipContent>
            </Tooltip>
        </TooltipProvider>
    )
}

function ClientTypeCard({
    type,
    title,
    description,
    features,
    icon: Icon,
    selected,
    onClick,
}: {
    type: ClientType
    title: string
    description: string
    features: string[]
    icon: React.ElementType
    selected: boolean
    onClick: () => void
}) {
    return (
        <button
            type="button"
            onClick={onClick}
            className={cn(
                "flex flex-col items-start p-6 rounded-xl border transition-all duration-300 text-left w-full group",
                selected
                    ? type === "public"
                        ? "border-success bg-gradient-to-br from-success to-success/80 text-white shadow-clay-card"
                        : "border-accent bg-gradient-to-br from-accent to-accent/80 text-white shadow-clay-card"
                    : "border-border hover:border-primary/40 hover:bg-muted/30 hover:shadow-clay-button"
            )}
        >
            <div className={cn(
                "p-3 rounded-xl mb-4 transition-all duration-300",
                selected 
                    ? "bg-white/20 backdrop-blur-sm" 
                    : type === "public" 
                        ? "bg-success/10 group-hover:bg-success/20" 
                        : "bg-accent/10 group-hover:bg-accent/20"
            )}>
                <Icon className={cn(
                    "h-6 w-6 transition-colors",
                    selected 
                        ? "text-white" 
                        : type === "public" 
                            ? "text-success" 
                            : "text-accent"
                )} />
            </div>
            <h3 className="text-lg font-semibold mb-1">{title}</h3>
            <p className={cn("text-sm mb-4", selected ? "text-white/85" : "text-muted-foreground")}>{description}</p>
            <ul className="text-sm space-y-1.5">
                {features.map((f, i) => (
                    <li key={i} className="flex items-center gap-2">
                        <span className={cn(
                            "text-xs font-bold",
                            selected 
                                ? "text-white/90" 
                                : type === "public" 
                                    ? "text-success" 
                                    : "text-accent"
                        )}>✓</span> {f}
                    </li>
                ))}
            </ul>
        </button>
    )
}

function StatCard({ icon: Icon, label, value, variant = "default", isLoading = false }: {
    icon: any
    label: string
    value: string | number
    variant?: "default" | "success" | "warning" | "danger"
    isLoading?: boolean
}) {
    const colorClasses = {
        default: "bg-info/10 text-info",
        success: "bg-success/10 text-success",
        warning: "bg-warning/10 text-warning",
        danger: "bg-danger/10 text-danger",
    }
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

// ============================================================================
// MAIN COMPONENT
// ============================================================================

export default function ClientsClientPage() {
    const params = useParams()
    const { t } = useI18n()
    const { toast } = useToast()
    const queryClient = useQueryClient()
    const tenantId = params.tenant_id as string

    // UI State
    const [search, setSearch] = useState("")
    const [createDialogOpen, setCreateDialogOpen] = useState(false)
    const [detailDialogOpen, setDetailDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [rotateSecretDialogOpen, setRotateSecretDialogOpen] = useState(false)
    const [selectedClient, setSelectedClient] = useState<ClientRow | null>(null)
    const [copiedField, setCopiedField] = useState<string | null>(null)
    const [showSecret, setShowSecret] = useState(false)
    const [newSecret, setNewSecret] = useState<string | null>(null)

    // Wizard state
    const [step, setStep] = useState(1)
    const [form, setForm] = useState<ClientFormState>({ ...DEFAULT_FORM })
    const [redirectUriInput, setRedirectUriInput] = useState("")
    const [originInput, setOriginInput] = useState("")
    const [postLogoutUriInput, setPostLogoutUriInput] = useState("")

    // Detail dialog tab
    const [detailTab, setDetailTab] = useState("general")

    // ========================================================================
    // QUERIES
    // ========================================================================

    const { data: tenant } = useQuery({
        queryKey: ["tenant", tenantId],
        enabled: !!tenantId,
        queryFn: () => api.get<Tenant>(`/v2/admin/tenants/${tenantId}`),
    })

    const { data: clientsRaw, isLoading, refetch } = useQuery({
        queryKey: ["clients", tenantId],
        enabled: !!tenantId,
        queryFn: () => api.get<ClientRow[]>(`/v2/admin/clients`, {
            headers: { "X-Tenant-ID": tenantId }
        }),
    })

    const clients = clientsRaw || []

    // Auto-generate clientId when name or type changes (in create mode)
    useEffect(() => {
        if (form.name && tenant?.slug && createDialogOpen) {
            setForm(prev => ({
                ...prev,
                clientId: generateClientId(tenant.slug, form.name, form.type)
            }))
        }
    }, [form.name, form.type, tenant?.slug, createDialogOpen])

    // ========================================================================
    // MUTATIONS
    // ========================================================================

    const createMutation = useMutation({
        mutationFn: (data: ClientFormState) =>
            api.post<ClientRow>(`/v2/admin/clients`, {
                client_id: data.clientId,
                name: data.name,
                type: data.type,
                redirect_uris: data.redirectUris,
                allowed_origins: data.allowedOrigins || [],
                post_logout_uris: data.postLogoutUris || [],
                providers: data.providers || [],
                scopes: data.scopes || [],
                grant_types: data.grantTypes || [],
                access_token_ttl: data.accessTokenTTL,
                refresh_token_ttl: data.refreshTokenTTL,
                id_token_ttl: data.idTokenTTL,
                require_email_verification: data.requireEmailVerification || false,
                reset_password_url: data.resetPasswordUrl || "",
                verify_email_url: data.verifyEmailUrl || "",
                front_channel_logout_url: data.frontChannelLogoutUrl || "",
                back_channel_logout_url: data.backChannelLogoutUrl || "",
            }, {
                headers: { "X-Tenant-ID": tenantId }
            }),
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
            setSelectedClient(data)
            setNewSecret(data.secret || null)
            resetForm()
            setCreateDialogOpen(false)
            setDetailDialogOpen(true)
            setDetailTab("security")
            toast({
                title: "Cliente creado",
                description: "El cliente OAuth2 ha sido creado exitosamente.",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo crear el cliente",
                variant: "destructive",
            })
        },
    })

    const updateMutation = useMutation({
        mutationFn: ({ clientId, data }: { clientId: string; data: Partial<ClientFormState> }) =>
            api.put<ClientRow>(`/v2/admin/clients/${clientId}`, {
                name: data.name,
                redirect_uris: data.redirectUris,
                allowed_origins: data.allowedOrigins || [],
                post_logout_uris: data.postLogoutUris || [],
                providers: data.providers || [],
                scopes: data.scopes || [],
                grant_types: data.grantTypes || [],
                access_token_ttl: data.accessTokenTTL,
                refresh_token_ttl: data.refreshTokenTTL,
                id_token_ttl: data.idTokenTTL,
                require_email_verification: data.requireEmailVerification || false,
                reset_password_url: data.resetPasswordUrl || "",
                verify_email_url: data.verifyEmailUrl || "",
                front_channel_logout_url: data.frontChannelLogoutUrl || "",
                back_channel_logout_url: data.backChannelLogoutUrl || "",
            }, tenantId),
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
            setSelectedClient(data)
            toast({
                title: "Cliente actualizado",
                description: "Los cambios han sido guardados.",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo actualizar el cliente",
                variant: "destructive",
            })
        },
    })

    const deleteMutation = useMutation({
        mutationFn: (clientId: string) => api.delete(`/v2/admin/clients/${clientId}`, {
            headers: { "X-Tenant-ID": tenantId }
        }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
            setDeleteDialogOpen(false)
            setDetailDialogOpen(false)
            setSelectedClient(null)
            toast({
                title: "Cliente eliminado",
                description: "El cliente ha sido eliminado permanentemente.",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo eliminar el cliente",
                variant: "destructive",
            })
        },
    })

    const rotateSecretMutation = useMutation({
        mutationFn: (clientId: string) => api.post<{ client_id: string; new_secret: string }>(`/v2/admin/clients/${clientId}/revoke`, {}, {
            headers: { "X-Tenant-ID": tenantId }
        }),
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
            setNewSecret(data.new_secret)
            setRotateSecretDialogOpen(false)
            toast({
                title: "Secret rotado",
                description: "El client_secret ha sido rotado. Guarda el nuevo secret ahora.",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo rotar el secret",
                variant: "destructive",
            })
        },
    })

    // ========================================================================
    // HANDLERS
    // ========================================================================

    const filteredClients = useMemo(() => {
        return clients.filter(
            (client) =>
                client.name.toLowerCase().includes(search.toLowerCase()) ||
                client.client_id?.toLowerCase().includes(search.toLowerCase())
        )
    }, [clients, search])

    const resetForm = () => {
        setForm({ ...DEFAULT_FORM })
        setRedirectUriInput("")
        setOriginInput("")
        setPostLogoutUriInput("")
        setStep(1)
    }

    const handleCreate = () => {
        if (!form.name) {
            toast({ title: "Error", description: "El nombre es obligatorio", variant: "destructive" })
            return
        }
        if (form.type === "public" && form.redirectUris.length === 0) {
            toast({ title: "Error", description: "Las apps frontend requieren al menos una URI de redirección", variant: "destructive" })
            return
        }
        createMutation.mutate(form)
    }

    const handleDelete = () => {
        if (selectedClient) {
            deleteMutation.mutate(selectedClient.client_id)
        }
    }

    const handleRotateSecret = () => {
        if (selectedClient) {
            rotateSecretMutation.mutate(selectedClient.client_id)
        }
    }

    const openDetailDialog = (client: ClientRow) => {
        setSelectedClient(client)
        setNewSecret(null)
        setShowSecret(false)
        setDetailTab("general")
        setDetailDialogOpen(true)
    }

    const copyToClipboard = (text: string, field: string) => {
        navigator.clipboard.writeText(text)
        setCopiedField(field)
        setTimeout(() => setCopiedField(null), 2000)
        toast({ title: "Copiado", description: `${field} copiado al portapapeles.` })
    }

    // URI handlers
    const addRedirectUri = () => {
        if (redirectUriInput && !form.redirectUris.includes(redirectUriInput)) {
            setForm({ ...form, redirectUris: [...form.redirectUris, redirectUriInput] })
            setRedirectUriInput("")
        }
    }
    const removeRedirectUri = (uri: string) => {
        setForm({ ...form, redirectUris: form.redirectUris.filter((u) => u !== uri) })
    }
    const addOrigin = () => {
        if (originInput && !form.allowedOrigins.includes(originInput)) {
            setForm({ ...form, allowedOrigins: [...form.allowedOrigins, originInput] })
            setOriginInput("")
        }
    }
    const removeOrigin = (origin: string) => {
        setForm({ ...form, allowedOrigins: form.allowedOrigins.filter((o) => o !== origin) })
    }
    const addPostLogoutUri = () => {
        if (postLogoutUriInput && !form.postLogoutUris.includes(postLogoutUriInput)) {
            setForm({ ...form, postLogoutUris: [...form.postLogoutUris, postLogoutUriInput] })
            setPostLogoutUriInput("")
        }
    }
    const removePostLogoutUri = (uri: string) => {
        setForm({ ...form, postLogoutUris: form.postLogoutUris.filter((u) => u !== uri) })
    }

    // Stats
    const stats = useMemo(() => ({
        total: clients.length,
        public: clients.filter(c => c.type === "public").length,
        confidential: clients.filter(c => c.type === "confidential").length,
    }), [clients])

    // ========================================================================
    // RENDER
    // ========================================================================

    return (
        <div className="space-y-6 animate-in fade-in duration-500">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                    <Button variant="ghost" size="sm" asChild>
                        <Link href={`/admin/tenants/detail?id=${tenantId}`}>
                            <ArrowLeft className="h-4 w-4" />
                        </Link>
                    </Button>
                    <div className="flex items-center gap-3">
                        <div>
                            <h1 className="text-2xl font-bold tracking-tight">Clients OAuth2</h1>
                            <p className="text-sm text-muted-foreground">
                                {tenant?.name} — Gestiona las aplicaciones que pueden autenticar usuarios
                            </p>
                        </div>
                    </div>
                </div>
                <Button onClick={() => { resetForm(); setCreateDialogOpen(true) }} className="shadow-clay-button hover:shadow-clay-card transition-shadow">
                    <Plus className="mr-2 h-4 w-4" />
                    Nuevo Client
                </Button>
            </div>

            {/* Info Banner - Premium gradient */}
            <InlineAlert variant="success">
                    Un <strong>Client</strong> representa una aplicación que puede autenticar usuarios mediante HelloJohn.
                    Los clients <strong>públicos</strong> (SPAs, apps móviles) usan PKCE sin secreto.
                    Los clients <strong>confidenciales</strong> (backends, APIs) tienen un client_secret seguro.
            </InlineAlert>

            {/* Stats */}
            <div className="grid grid-cols-3 gap-4">
                <StatCard icon={Globe} label="Total Clients" value={stats.total} variant="default" isLoading={isLoading} />
                <StatCard icon={Monitor} label="Frontend (Públicos)" value={stats.public} variant="success" isLoading={isLoading} />
                <StatCard icon={Server} label="Backend (Confidenciales)" value={stats.confidential} variant="warning" isLoading={isLoading} />
            </div>

            {/* Table Card */}
            <Card>
                <CardHeader className="pb-3">
                    <div className="flex items-center justify-between">
                        <div className="relative flex-1 max-w-sm">
                            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                            <Input
                                placeholder="Buscar por nombre o client_id..."
                                value={search}
                                onChange={(e) => setSearch(e.target.value)}
                                className="pl-9"
                            />
                        </div>
                        <Button variant="outline" size="sm" onClick={() => refetch()}>
                            <RefreshCw className="h-4 w-4 mr-2" />
                            Actualizar
                        </Button>
                    </div>
                </CardHeader>
                <CardContent>
                    {isLoading ? (
                        <div className="flex items-center justify-center py-12">
                            <Loader2 className="h-8 w-8 animate-spin text-accent" />
                        </div>
                    ) : filteredClients.length === 0 ? (
                        <div className="flex flex-col items-center justify-center py-16">
                            <div className="relative mb-6 group">
                                <div className="absolute inset-0 bg-gradient-to-br from-success/30 to-accent/20 rounded-full blur-2xl scale-150 group-hover:scale-175 transition-transform duration-700" />
                                <div className="relative rounded-2xl bg-gradient-to-br from-success/10 to-accent/5 p-8 border border-success/20 shadow-clay-card">
                                    <Globe className="h-12 w-12 text-success" />
                                </div>
                            </div>
                            <h3 className="text-xl font-bold mb-2">No hay clients</h3>
                            <p className="text-muted-foreground text-center max-w-sm text-sm mb-6">
                                {search
                                    ? "No se encontraron clients con ese criterio de búsqueda."
                                    : "Crea tu primer client OAuth2 para permitir que aplicaciones autentiquen usuarios."}
                            </p>
                            {!search && (
                                <Button onClick={() => { resetForm(); setCreateDialogOpen(true) }} size="lg" className="shadow-clay-button">
                                    <Plus className="mr-2 h-4 w-4" />
                                    Crear primer client
                                </Button>
                            )}
                        </div>
                    ) : (
                        <Table>
                            <TableHeader>
                                <TableRow className="bg-muted/30">
                                    <TableHead>Nombre</TableHead>
                                    <TableHead>Client ID</TableHead>
                                    <TableHead>Tipo</TableHead>
                                    <TableHead>Grant Types</TableHead>
                                    <TableHead>URIs</TableHead>
                                    <TableHead className="text-right">Acciones</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {filteredClients.map((client) => (
                                    <TableRow key={client.client_id} className="cursor-pointer hover:bg-muted/50 transition-colors" onClick={() => openDetailDialog(client)}>
                                        <TableCell>
                                            <div className="flex items-center gap-3">
                                                <div className={cn(
                                                    "p-2 rounded-lg transition-all duration-200",
                                                    client.type === "public"
                                                        ? "bg-success/10 text-success"
                                                        : "bg-accent/10 text-accent"
                                                )}>
                                                    {client.type === "public" ? <Globe className="h-4 w-4" /> : <Server className="h-4 w-4" />}
                                                </div>
                                                <div>
                                                    <p className="font-medium">{client.name}</p>
                                                    <p className="text-xs text-muted-foreground">
                                                        {client.created_at ? `Creado ${formatRelativeTime(client.created_at)}` : ""}
                                                    </p>
                                                </div>
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            <div className="flex items-center gap-2">
                                                <code className="text-xs bg-muted px-2 py-1 rounded max-w-[150px] truncate" title={client.client_id}>
                                                    {client.client_id}
                                                </code>
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="h-6 w-6"
                                                    onClick={(e) => { e.stopPropagation(); copyToClipboard(client.client_id, "Client ID") }}
                                                >
                                                    {copiedField === "Client ID" ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
                                                </Button>
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            <Badge variant={client.type === "confidential" ? "default" : "secondary"}>
                                                {client.type === "confidential" ? "Backend" : "Frontend"}
                                            </Badge>
                                        </TableCell>
                                        <TableCell>
                                            <div className="flex flex-wrap gap-1">
                                                {(client.grant_types || ["authorization_code"]).slice(0, 2).map(gt => (
                                                    <Badge key={gt} variant="outline" className="text-[10px]">
                                                        {gt.replace("_", " ")}
                                                    </Badge>
                                                ))}
                                                {(client.grant_types || []).length > 2 && (
                                                    <Badge variant="outline" className="text-[10px]">+{(client.grant_types || []).length - 2}</Badge>
                                                )}
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            <span className="text-sm text-muted-foreground">
                                                {client.redirect_uris?.length || 0} redirect
                                            </span>
                                        </TableCell>
                                        <TableCell className="text-right">
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                                                    <Button variant="ghost" size="sm" className="h-8 w-8">
                                                        <MoreHorizontal className="h-4 w-4" />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    <DropdownMenuItem onClick={(e) => { e.stopPropagation(); openDetailDialog(client) }}>
                                                        <Eye className="mr-2 h-4 w-4" /> Ver detalles
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem onClick={(e) => { e.stopPropagation(); copyToClipboard(client.client_id, "Client ID") }}>
                                                        <Copy className="mr-2 h-4 w-4" /> Copiar Client ID
                                                    </DropdownMenuItem>
                                                    {client.type === "confidential" && (
                                                        <DropdownMenuItem onClick={(e) => { e.stopPropagation(); setSelectedClient(client); setRotateSecretDialogOpen(true) }}>
                                                            <RotateCcw className="mr-2 h-4 w-4" /> Rotar Secret
                                                        </DropdownMenuItem>
                                                    )}
                                                    <DropdownMenuSeparator />
                                                    <DropdownMenuItem
                                                        onClick={(e) => { e.stopPropagation(); setSelectedClient(client); setDeleteDialogOpen(true) }}
                                                        className="text-danger hover:text-danger hover:bg-danger/10"
                                                    >
                                                        <Trash2 className="mr-2 h-4 w-4" /> Eliminar
                                                    </DropdownMenuItem>
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    )}
                </CardContent>
            </Card>

            {/* ============================================================
                CREATE DIALOG - WIZARD
                ============================================================ */}
            <Dialog open={createDialogOpen} onOpenChange={(open) => { if (!open) resetForm(); setCreateDialogOpen(open) }}>
                <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2">
                            <Sparkles className="h-5 w-5 text-primary" />
                            Crear nuevo client OAuth2
                        </DialogTitle>
                        <DialogDescription>
                            {step === 1 && "Paso 1 de 4: Selecciona el tipo de aplicación"}
                            {step === 2 && "Paso 2 de 4: Información básica"}
                            {step === 3 && "Paso 3 de 4: URIs y orígenes permitidos"}
                            {step === 4 && "Paso 4 de 4: Configuración avanzada"}
                        </DialogDescription>
                    </DialogHeader>

                    {/* Step Indicator */}
                    <div className="flex items-center justify-center gap-2 py-4">
                        {[1, 2, 3, 4].map((s) => (
                            <div
                                key={s}
                                className={cn("h-2 w-12 rounded-full transition-colors", s <= step ? "bg-primary" : "bg-muted")}
                            />
                        ))}
                    </div>

                    {/* Step 1: Client Type */}
                    {step === 1 && (
                        <div className="grid md:grid-cols-2 gap-4">
                            <ClientTypeCard
                                type="public"
                                title="Aplicación Frontend"
                                description="Para aplicaciones que corren en el navegador"
                                features={[
                                    "SPAs (React, Vue, Angular)",
                                    "Apps móviles nativas",
                                    "Sin secreto (usa PKCE)",
                                ]}
                                icon={Globe}
                                selected={form.type === "public"}
                                onClick={() => setForm({ ...form, type: "public", grantTypes: ["authorization_code", "refresh_token"] })}
                            />
                            <ClientTypeCard
                                type="confidential"
                                title="Servidor / Backend"
                                description="Para servicios que corren en un servidor seguro"
                                features={[
                                    "APIs y microservicios",
                                    "Machine-to-Machine (M2M)",
                                    "Con client_secret seguro",
                                ]}
                                icon={Server}
                                selected={form.type === "confidential"}
                                onClick={() => setForm({ ...form, type: "confidential", grantTypes: ["authorization_code", "refresh_token", "client_credentials"] })}
                            />
                        </div>
                    )}

                    {/* Step 2: Basic Info */}
                    {step === 2 && (
                        <div className="space-y-6">
                            <div className="space-y-2">
                                <Label className="flex items-center">
                                    Nombre del cliente *
                                    <InfoTooltip content="Un nombre descriptivo para identificar esta aplicación." />
                                </Label>
                                <Input
                                    value={form.name}
                                    onChange={(e) => setForm({ ...form, name: e.target.value })}
                                    placeholder="Mi Aplicación Web"
                                    className="text-lg"
                                />
                            </div>

                            <div className="space-y-2">
                                <Label className="flex items-center">
                                    Client ID
                                    <InfoTooltip content="Identificador único generado automáticamente. Es público." />
                                </Label>
                                <div className="flex items-center gap-2">
                                    <code className="flex-1 rounded-md bg-muted px-4 py-3 text-sm font-mono">
                                        {form.clientId || "Se generará automáticamente..."}
                                    </code>
                                    {form.clientId && (
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            onClick={() => setForm({ ...form, clientId: generateClientId(tenant?.slug || "", form.name, form.type) })}
                                            title="Regenerar"
                                        >
                                            <Sparkles className="h-4 w-4" />
                                        </Button>
                                    )}
                                </div>
                            </div>

                            <div className="p-4 rounded-lg bg-muted/50 border">
                                <div className="flex items-center gap-2 mb-2">
                                    <Badge variant={form.type === "public" ? "secondary" : "default"}>
                                        {form.type === "public" ? "Frontend" : "Backend"}
                                    </Badge>
                                    <span className="text-sm">seleccionado</span>
                                </div>
                                <p className="text-sm text-muted-foreground">
                                    {form.type === "public"
                                        ? "Este cliente NO tendrá un secreto. Usará PKCE para autenticación segura."
                                        : "Este cliente recibirá un client_secret que deberás guardar de forma segura."}
                                </p>
                            </div>
                        </div>
                    )}

                    {/* Step 3: URIs */}
                    {step === 3 && (
                        <div className="space-y-6">
                            {/* Redirect URIs */}
                            <div className="space-y-2">
                                <Label className="flex items-center">
                                    URIs de redirección {form.type === "public" && <span className="text-danger ml-1">*</span>}
                                    <InfoTooltip content="URLs a las que HelloJohn redirigirá después del login." />
                                </Label>
                                <div className="flex gap-2">
                                    <Input
                                        value={redirectUriInput}
                                        onChange={(e) => setRedirectUriInput(e.target.value)}
                                        placeholder="https://miapp.com/callback"
                                        onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addRedirectUri() } }}
                                    />
                                    <Button type="button" variant="outline" onClick={addRedirectUri}>Agregar</Button>
                                </div>
                                {form.type === "public" && form.redirectUris.length === 0 && (
                                    <p className="text-sm text-warning flex items-center gap-1">
                                        <AlertTriangle className="h-3.5 w-3.5" />
                                        Las apps frontend requieren al menos una URI
                                    </p>
                                )}
                                {form.redirectUris.length > 0 && (
                                    <div className="space-y-1 mt-2">
                                        {form.redirectUris.map((uri) => (
                                            <div key={uri} className="flex items-center justify-between rounded bg-muted p-2">
                                                <code className="text-sm truncate flex-1">{uri}</code>
                                                <Button variant="ghost" size="sm" className="h-6 w-6" onClick={() => removeRedirectUri(uri)}>
                                                    <Trash2 className="h-3.5 w-3.5" />
                                                </Button>
                                            </div>
                                        ))}
                                    </div>
                                )}
                            </div>

                            {/* Allowed Origins (public only) */}
                            {form.type === "public" && (
                                <div className="space-y-2">
                                    <Label className="flex items-center">
                                        Orígenes permitidos (CORS)
                                        <InfoTooltip content="Dominios desde los que se permiten requests JavaScript." />
                                    </Label>
                                    <div className="flex gap-2">
                                        <Input
                                            value={originInput}
                                            onChange={(e) => setOriginInput(e.target.value)}
                                            placeholder="http://localhost:3000"
                                            onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addOrigin() } }}
                                        />
                                        <Button type="button" variant="outline" onClick={addOrigin}>Agregar</Button>
                                    </div>
                                    {form.allowedOrigins.length > 0 && (
                                        <div className="space-y-1 mt-2">
                                            {form.allowedOrigins.map((origin) => (
                                                <div key={origin} className="flex items-center justify-between rounded bg-muted p-2">
                                                    <code className="text-sm">{origin}</code>
                                                    <Button variant="ghost" size="sm" className="h-6 w-6" onClick={() => removeOrigin(origin)}>
                                                        <Trash2 className="h-3.5 w-3.5" />
                                                    </Button>
                                                </div>
                                            ))}
                                        </div>
                                    )}
                                </div>
                            )}

                            {/* Post-Logout URIs */}
                            <div className="space-y-2">
                                <Label className="flex items-center">
                                    URIs post-logout
                                    <InfoTooltip content="URLs a las que redirigir después del logout." />
                                </Label>
                                <div className="flex gap-2">
                                    <Input
                                        value={postLogoutUriInput}
                                        onChange={(e) => setPostLogoutUriInput(e.target.value)}
                                        placeholder="https://miapp.com/logged-out"
                                        onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addPostLogoutUri() } }}
                                    />
                                    <Button type="button" variant="outline" onClick={addPostLogoutUri}>Agregar</Button>
                                </div>
                                {form.postLogoutUris.length > 0 && (
                                    <div className="space-y-1 mt-2">
                                        {form.postLogoutUris.map((uri) => (
                                            <div key={uri} className="flex items-center justify-between rounded bg-muted p-2">
                                                <code className="text-sm">{uri}</code>
                                                <Button variant="ghost" size="sm" className="h-6 w-6" onClick={() => removePostLogoutUri(uri)}>
                                                    <Trash2 className="h-3.5 w-3.5" />
                                                </Button>
                                            </div>
                                        ))}
                                    </div>
                                )}
                            </div>
                        </div>
                    )}

                    {/* Step 4: Advanced Config */}
                    {step === 4 && (
                        <div className="space-y-6">
                            {/* Grant Types */}
                            <div className="space-y-3">
                                <Label className="flex items-center">
                                    Grant Types permitidos
                                    <InfoTooltip content="Flujos OAuth2 que este cliente puede usar." />
                                </Label>
                                <div className="space-y-2">
                                    {GRANT_TYPES.map((gt) => {
                                        const disabled = gt.confidentialOnly && form.type === "public"
                                        const checked = form.grantTypes.includes(gt.id)
                                        return (
                                            <label
                                                key={gt.id}
                                                className={cn(
                                                    "flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors",
                                                    disabled && "opacity-50 cursor-not-allowed",
                                                    checked && !disabled && "border-primary bg-primary/5",
                                                    !checked && !disabled && "hover:bg-muted/50"
                                                )}
                                            >
                                                <Checkbox
                                                    checked={checked}
                                                    disabled={disabled}
                                                    onCheckedChange={(c) => {
                                                        if (c) {
                                                            setForm({ ...form, grantTypes: [...form.grantTypes, gt.id] })
                                                        } else {
                                                            setForm({ ...form, grantTypes: form.grantTypes.filter(g => g !== gt.id) })
                                                        }
                                                    }}
                                                    className="mt-0.5"
                                                />
                                                <div className="flex-1">
                                                    <div className="flex items-center gap-2">
                                                        <span className="font-medium text-sm">{gt.label}</span>
                                                        {gt.recommended && <Badge variant="secondary" className="text-[10px]">Recomendado</Badge>}
                                                        {gt.deprecated && <Badge variant="destructive" className="text-[10px]">Deprecado</Badge>}
                                                        {gt.confidentialOnly && <Badge variant="outline" className="text-[10px]">Solo Backend</Badge>}
                                                    </div>
                                                    <p className="text-xs text-muted-foreground mt-0.5">{gt.description}</p>
                                                </div>
                                            </label>
                                        )
                                    })}
                                </div>
                            </div>

                            {/* Token TTLs */}
                            <div className="space-y-3">
                                <Label>Duración de Tokens</Label>
                                <div className="grid grid-cols-3 gap-4">
                                    <div className="space-y-2">
                                        <Label className="text-xs text-muted-foreground">Access Token</Label>
                                        <Select
                                            value={String(form.accessTokenTTL)}
                                            onValueChange={(v) => setForm({ ...form, accessTokenTTL: Number(v) })}
                                        >
                                            <SelectTrigger><SelectValue /></SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="5">5 min</SelectItem>
                                                <SelectItem value="15">15 min</SelectItem>
                                                <SelectItem value="30">30 min</SelectItem>
                                                <SelectItem value="60">1 hora</SelectItem>
                                            </SelectContent>
                                        </Select>
                                    </div>
                                    <div className="space-y-2">
                                        <Label className="text-xs text-muted-foreground">Refresh Token</Label>
                                        <Select
                                            value={String(form.refreshTokenTTL)}
                                            onValueChange={(v) => setForm({ ...form, refreshTokenTTL: Number(v) })}
                                        >
                                            <SelectTrigger><SelectValue /></SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="10080">7 días</SelectItem>
                                                <SelectItem value="20160">14 días</SelectItem>
                                                <SelectItem value="43200">30 días</SelectItem>
                                                <SelectItem value="129600">90 días</SelectItem>
                                            </SelectContent>
                                        </Select>
                                    </div>
                                    <div className="space-y-2">
                                        <Label className="text-xs text-muted-foreground">ID Token</Label>
                                        <Select
                                            value={String(form.idTokenTTL)}
                                            onValueChange={(v) => setForm({ ...form, idTokenTTL: Number(v) })}
                                        >
                                            <SelectTrigger><SelectValue /></SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="15">15 min</SelectItem>
                                                <SelectItem value="60">1 hora</SelectItem>
                                                <SelectItem value="480">8 horas</SelectItem>
                                                <SelectItem value="1440">24 horas</SelectItem>
                                            </SelectContent>
                                        </Select>
                                    </div>
                                </div>
                            </div>

                            {/* Scopes */}
                            <div className="space-y-2">
                                <Label className="flex items-center">
                                    Scopes permitidos
                                    <InfoTooltip content="Permisos que este cliente puede solicitar." />
                                </Label>
                                <Input
                                    placeholder="openid profile email offline_access"
                                    value={form.scopes.join(" ")}
                                    onChange={(e) => setForm({ ...form, scopes: e.target.value.split(/\s+/).filter(Boolean) })}
                                />
                                <p className="text-xs text-muted-foreground">Separados por espacio</p>
                            </div>

                            {/* Providers (public only) */}
                            {form.type === "public" && (
                                <div className="space-y-2">
                                    <Label>Proveedores de autenticación</Label>
                                    <div className="rounded-lg border divide-y">
                                        {AVAILABLE_PROVIDERS.map((p) => (
                                            <div key={p.id} className={cn("flex items-center justify-between px-4 py-3", !p.enabled && "opacity-50")}>
                                                <div className="flex items-center gap-3">
                                                    <span className="text-lg">{p.icon}</span>
                                                    <span className="text-sm font-medium">{p.label}</span>
                                                    {p.comingSoon && <Badge variant="outline" className="text-[10px]">Próximamente</Badge>}
                                                </div>
                                                <Switch
                                                    disabled={!p.enabled}
                                                    checked={form.providers.includes(p.id)}
                                                    onCheckedChange={(c) => {
                                                        if (c) {
                                                            setForm({ ...form, providers: [...form.providers, p.id] })
                                                        } else {
                                                            setForm({ ...form, providers: form.providers.filter(pr => pr !== p.id) })
                                                        }
                                                    }}
                                                />
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            )}
                        </div>
                    )}

                    <DialogFooter className="flex justify-between mt-6">
                        <div>
                            {step > 1 && (
                                <Button variant="ghost" onClick={() => setStep(step - 1)}>
                                    <ChevronLeft className="mr-2 h-4 w-4" />
                                    Anterior
                                </Button>
                            )}
                        </div>
                        <div className="flex gap-2">
                            <Button variant="outline" onClick={() => { resetForm(); setCreateDialogOpen(false) }}>
                                Cancelar
                            </Button>
                            {step < 4 ? (
                                <Button onClick={() => setStep(step + 1)} disabled={step === 2 && !form.name}>
                                    Siguiente
                                    <ChevronRight className="ml-2 h-4 w-4" />
                                </Button>
                            ) : (
                                <Button onClick={handleCreate} disabled={createMutation.isPending}>
                                    {createMutation.isPending ? (
                                        <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Creando...</>
                                    ) : (
                                        "Crear Client"
                                    )}
                                </Button>
                            )}
                        </div>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* ============================================================
                DETAIL DIALOG - TABS VIEW
                ============================================================ */}
            <Dialog open={detailDialogOpen} onOpenChange={setDetailDialogOpen}>
                <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
                    <DialogHeader>
                        <div className="flex items-center justify-between">
                            <div className="flex items-center gap-3">
                                <div className={cn(
                                    "p-2.5 rounded-xl transition-colors",
                                    selectedClient?.type === "public"
                                        ? "bg-success/10 text-success"
                                        : "bg-accent/10 text-accent"
                                )}>
                                    {selectedClient?.type === "public" ? <Globe className="h-5 w-5" /> : <Server className="h-5 w-5" />}
                                </div>
                                <div>
                                    <DialogTitle>{selectedClient?.name}</DialogTitle>
                                    <DialogDescription asChild>
                                        <div className="flex items-center gap-2 mt-1 text-sm text-muted-foreground">
                                            <code className="text-xs bg-muted px-2 py-0.5 rounded">{selectedClient?.client_id}</code>
                                            <Badge variant={selectedClient?.type === "confidential" ? "default" : "secondary"}>
                                                {selectedClient?.type === "confidential" ? "Backend" : "Frontend"}
                                            </Badge>
                                        </div>
                                    </DialogDescription>
                                </div>
                            </div>
                            <div className="flex items-center gap-2">
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={() => { setSelectedClient(selectedClient); setDeleteDialogOpen(true) }}
                                >
                                    <Trash2 className="h-4 w-4 mr-2 text-danger" />
                                    Eliminar
                                </Button>
                            </div>
                        </div>
                    </DialogHeader>

                    {selectedClient && (
                        <Tabs value={detailTab} onValueChange={setDetailTab} className="mt-4">
                            <TabsList className="grid w-full grid-cols-4">
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
                                <TabsTrigger value="logout" className="flex items-center gap-2">
                                    <LogOut className="h-4 w-4" />
                                    <span className="hidden sm:inline">Logout</span>
                                </TabsTrigger>
                            </TabsList>

                            {/* Tab: General */}
                            <TabsContent value="general" className="space-y-6 mt-6">
                                {/* Client ID */}
                                <div className="space-y-2">
                                    <Label className="flex items-center gap-1">
                                        Client ID
                                        <InfoTooltip content="Identificador público del cliente. Se usa en las solicitudes OAuth." />
                                    </Label>
                                    <div className="flex items-center gap-2">
                                        <code className="flex-1 rounded bg-muted px-4 py-2.5 text-sm font-mono">{selectedClient.client_id}</code>
                                        <Button variant="outline" size="sm" onClick={() => copyToClipboard(selectedClient.client_id, "Client ID")}>
                                            {copiedField === "Client ID" ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                                        </Button>
                                    </div>
                                </div>

                                {/* Redirect URIs */}
                                <div className="space-y-2">
                                    <Label className="flex items-center gap-1">
                                        URIs de redirección
                                        <InfoTooltip content="URLs permitidas para redirección después del login." />
                                    </Label>
                                    {selectedClient.redirect_uris && selectedClient.redirect_uris.length > 0 ? (
                                        <div className="space-y-1">
                                            {selectedClient.redirect_uris.map((uri) => (
                                                <div key={uri} className="flex items-center gap-2 rounded bg-muted p-2">
                                                    <Link2 className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
                                                    <code className="text-sm flex-1 truncate">{uri}</code>
                                                </div>
                                            ))}
                                        </div>
                                    ) : (
                                        <p className="text-sm text-muted-foreground">Sin URIs configuradas</p>
                                    )}
                                </div>

                                {/* Allowed Origins */}
                                {selectedClient.type === "public" && (
                                    <div className="space-y-2">
                                        <Label className="flex items-center gap-1">
                                            Orígenes permitidos (CORS)
                                            <InfoTooltip content="Dominios desde los que se permiten requests." />
                                        </Label>
                                        {selectedClient.allowed_origins && selectedClient.allowed_origins.length > 0 ? (
                                            <div className="space-y-1">
                                                {selectedClient.allowed_origins.map((origin) => (
                                                    <div key={origin} className="flex items-center gap-2 rounded bg-muted p-2">
                                                        <Globe className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
                                                        <code className="text-sm">{origin}</code>
                                                    </div>
                                                ))}
                                            </div>
                                        ) : (
                                            <p className="text-sm text-muted-foreground">Sin orígenes configurados</p>
                                        )}
                                    </div>
                                )}

                                {/* Scopes */}
                                <div className="space-y-2">
                                    <Label>Scopes permitidos</Label>
                                    <div className="flex flex-wrap gap-2">
                                        {(selectedClient.scopes || ["openid", "profile", "email"]).map((scope) => (
                                            <Badge key={scope} variant="outline">{scope}</Badge>
                                        ))}
                                    </div>
                                </div>

                                {/* Providers */}
                                {selectedClient.type === "public" && selectedClient.providers && (
                                    <div className="space-y-2">
                                        <Label>Proveedores de autenticación</Label>
                                        <div className="flex flex-wrap gap-2">
                                            {selectedClient.providers.map((p) => {
                                                const provider = AVAILABLE_PROVIDERS.find(pr => pr.id === p)
                                                return (
                                                    <Badge key={p} variant="secondary">
                                                        {provider?.icon} {provider?.label || p}
                                                    </Badge>
                                                )
                                            })}
                                        </div>
                                    </div>
                                )}
                            </TabsContent>

                            {/* Tab: Security */}
                            <TabsContent value="security" className="space-y-6 mt-6">
                                {/* Client Secret (confidential only) */}
                                {selectedClient.type === "confidential" && (
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
                                        <CardContent className="space-y-4">
                                            {newSecret ? (
                                                <div className="space-y-3">
                                                    <InlineAlert 
                                                        variant="destructive" 
                                                        title="¡Guarda este secret ahora!"
                                                        description="No podrás verlo de nuevo. Si lo pierdes, tendrás que rotarlo."
                                                    />
                                                    <div className="flex items-center gap-2">
                                                        <code className={cn(
                                                            "flex-1 rounded bg-muted px-4 py-2.5 text-sm font-mono",
                                                            !showSecret && "select-none"
                                                        )}>
                                                            {showSecret ? newSecret : "•".repeat(40)}
                                                        </code>
                                                        <Button variant="outline" size="sm" onClick={() => setShowSecret(!showSecret)}>
                                                            {showSecret ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                                                        </Button>
                                                        <Button variant="outline" size="sm" onClick={() => copyToClipboard(newSecret, "Client Secret")}>
                                                            {copiedField === "Client Secret" ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                                                        </Button>
                                                    </div>
                                                </div>
                                            ) : (
                                                <div className="flex items-center gap-2">
                                                    <code className="flex-1 rounded bg-muted px-4 py-2.5 text-sm font-mono text-muted-foreground">
                                                        •••••••••••••••••••••••••••••••••••••••
                                                    </code>
                                                    <Button
                                                        variant="outline"
                                                        onClick={() => setRotateSecretDialogOpen(true)}
                                                    >
                                                        <RotateCcw className="h-4 w-4 mr-2" />
                                                        Rotar Secret
                                                    </Button>
                                                </div>
                                            )}
                                        </CardContent>
                                    </Card>
                                )}

                                {/* Grant Types */}
                                <div className="space-y-3">
                                    <Label>Grant Types habilitados</Label>
                                    <div className="grid gap-2">
                                        {GRANT_TYPES.map((gt) => {
                                            const enabled = selectedClient.grant_types?.includes(gt.id) ||
                                                (gt.id === "authorization_code" && !selectedClient.grant_types)
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
                                                            <CheckCircle2 className="h-4 w-4 text-success" />
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
                                </div>

                                {/* PKCE info for public clients */}
                                {selectedClient.type === "public" && (
                                    <InlineAlert 
                                        variant="info" 
                                        title="PKCE habilitado"
                                        description="Los clients públicos usan PKCE (Proof Key for Code Exchange) automáticamente para proteger el flujo de autorización sin necesidad de un client_secret."
                                    />
                                )}
                            </TabsContent>

                            {/* Tab: Tokens */}
                            <TabsContent value="tokens" className="space-y-6 mt-6">
                                <div className="grid grid-cols-3 gap-4">
                                    <Card>
                                        <CardHeader className="pb-2">
                                            <CardTitle className="text-sm flex items-center gap-2">
                                                <Zap className="h-4 w-4 text-warning" />
                                                Access Token
                                            </CardTitle>
                                        </CardHeader>
                                        <CardContent>
                                            <p className="text-2xl font-bold">{formatTTL(selectedClient.access_token_ttl || 15)}</p>
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
                                            <p className="text-2xl font-bold">{formatTTL(selectedClient.refresh_token_ttl || 43200)}</p>
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
                                            <p className="text-2xl font-bold">{formatTTL(selectedClient.id_token_ttl || 60)}</p>
                                            <p className="text-xs text-muted-foreground">Tiempo de vida</p>
                                        </CardContent>
                                    </Card>
                                </div>

                                <InlineAlert 
                                    variant="default" 
                                    description="Los tiempos de vida de tokens se pueden modificar editando el cliente. Valores más cortos son más seguros pero requieren renovación más frecuente."
                                />
                            </TabsContent>

                            {/* Tab: Logout */}
                            <TabsContent value="logout" className="space-y-6 mt-6">
                                {/* Post-Logout URIs */}
                                <div className="space-y-2">
                                    <Label className="flex items-center gap-1">
                                        URIs post-logout
                                        <InfoTooltip content="URLs a las que redirigir después del logout." />
                                    </Label>
                                    {selectedClient.post_logout_uris && selectedClient.post_logout_uris.length > 0 ? (
                                        <div className="space-y-1">
                                            {selectedClient.post_logout_uris.map((uri) => (
                                                <div key={uri} className="flex items-center gap-2 rounded bg-muted p-2">
                                                    <LogOut className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
                                                    <code className="text-sm">{uri}</code>
                                                </div>
                                            ))}
                                        </div>
                                    ) : (
                                        <p className="text-sm text-muted-foreground">Sin URIs post-logout configuradas</p>
                                    )}
                                </div>

                                {/* Front/Back Channel Logout */}
                                <div className="grid md:grid-cols-2 gap-4">
                                    <div className="space-y-2">
                                        <Label className="flex items-center gap-1">
                                            Front-Channel Logout URL
                                            <InfoTooltip content="URL para logout iniciado desde el navegador." />
                                        </Label>
                                        {selectedClient.front_channel_logout_url ? (
                                            <code className="block rounded bg-muted p-2 text-sm">{selectedClient.front_channel_logout_url}</code>
                                        ) : (
                                            <p className="text-sm text-muted-foreground">No configurado</p>
                                        )}
                                    </div>
                                    <div className="space-y-2">
                                        <Label className="flex items-center gap-1">
                                            Back-Channel Logout URL
                                            <InfoTooltip content="URL para logout servidor-a-servidor." />
                                        </Label>
                                        {selectedClient.back_channel_logout_url ? (
                                            <code className="block rounded bg-muted p-2 text-sm">{selectedClient.back_channel_logout_url}</code>
                                        ) : (
                                            <p className="text-sm text-muted-foreground">No configurado</p>
                                        )}
                                    </div>
                                </div>

                                <InlineAlert 
                                    variant="default" 
                                    description="El logout federado permite cerrar sesión en múltiples aplicaciones simultáneamente. Configura las URLs de logout para habilitar esta funcionalidad."
                                />
                            </TabsContent>
                        </Tabs>
                    )}
                </DialogContent>
            </Dialog>

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
                            ¿Estás seguro de que deseas eliminar <strong>{selectedClient?.name}</strong>?
                            Esta acción es irreversible y revocará todos los tokens activos.
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>Cancelar</Button>
                        <Button variant="danger" onClick={handleDelete} disabled={deleteMutation.isPending}>
                            {deleteMutation.isPending ? (
                                <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Eliminando...</>
                            ) : (
                                "Eliminar"
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
                            ¿Estás seguro de que deseas rotar el secret de <strong>{selectedClient?.name}</strong>?
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
                        <Button variant="danger" onClick={handleRotateSecret} disabled={rotateSecretMutation.isPending}>
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
