"use client"

import { useState, useEffect, useMemo } from "react"
import { useParams } from "next/navigation"
import Link from "next/link"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
    CardFooter,
    Button,
    Input,
    Label,
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
    Switch,
    Checkbox,
} from "@/components/ds"
import { useToast } from "@/hooks/use-toast"
import { api, tokensAdminAPI } from "@/lib/api"
import { API_ROUTES } from "@/lib/routes"
import { useAuthStore } from "@/lib/auth-store"
import { useI18n } from "@/lib/i18n"
import type { TokenResponse, TokenStats as GlobalTokenStats, ListTokensResponse, TokenFilters } from "@/lib/types"
import {
    Key,
    KeyRound,
    Search,
    Loader2,
    MoreHorizontal,
    Copy,
    ChevronRight,
    AlertCircle,
    Info,
    CheckCircle2,
    XCircle,
    Clock,
    RefreshCw,
    Trash2,
    Eye,
    EyeOff,
    Shield,
    ShieldCheck,
    ShieldAlert,
    Smartphone,
    Monitor,
    Tablet,
    Globe,
    Activity,
    BarChart3,
    History,
    Ban,
    Play,
    FileJson,
    Lock,
    Unlock,
    Zap,
    Timer,
    CalendarClock,
    Users,
    Database,
    ChevronDown,
    ChevronUp,
    ExternalLink,
    ArrowRight,
    AlertTriangle,
    ArrowLeft,
} from "lucide-react"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
    Textarea,
    InlineAlert,
    Badge,
    Tabs,
    TabsContent,
    TabsList,
    TabsTrigger,
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
    cn,
    NoDatabaseConfigured,
    isNoDatabaseError,
} from "@/components/ds"
import { Tenant } from "@/lib/types"

// ----- Types -----
interface IntrospectionResult {
    active: boolean
    sub?: string
    client_id?: string
    username?: string
    token_type?: string
    exp?: number
    iat?: number
    nbf?: number
    aud?: string | string[]
    iss?: string
    jti?: string
    scope?: string
    // Custom claims
    roles?: string[]
    permissions?: string[]
    [key: string]: any
}

interface DecodedJWT {
    header: Record<string, any>
    payload: Record<string, any>
    signature: string
}

interface ActiveToken {
    id: string
    user_id: string
    user_email: string
    client_id: string
    client_name: string
    device?: string
    browser?: string
    os?: string
    ip?: string
    location?: string
    issued_at: string
    expires_at: string
    last_used?: string
    is_current?: boolean
}

interface TokenStats {
    total_active: number
    by_client: { client_id: string; client_name: string; count: number }[]
    by_device: { device: string; count: number }[]
    issued_today: number
    revoked_today: number
    avg_lifetime_hours: number
}

interface TokenHistoryEntry {
    id: string
    action: "issued" | "revoked" | "refreshed" | "expired"
    user_email?: string
    client_id: string
    client_name?: string
    ip?: string
    device?: string
    timestamp: string
    reason?: string
}

// ----- Mock Data -----
const MOCK_ACTIVE_TOKENS: ActiveToken[] = [
    {
        id: "tok_1",
        user_id: "user_abc123",
        user_email: "john.doe@example.com",
        client_id: "web-app",
        client_name: "Web Application",
        device: "Desktop",
        browser: "Chrome 120",
        os: "Windows 11",
        ip: "192.168.1.100",
        location: "New York, US",
        issued_at: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
        expires_at: new Date(Date.now() + 22 * 60 * 60 * 1000).toISOString(),
        last_used: new Date(Date.now() - 5 * 60 * 1000).toISOString(),
        is_current: true,
    },
    {
        id: "tok_2",
        user_id: "user_abc123",
        user_email: "john.doe@example.com",
        client_id: "mobile-app",
        client_name: "Mobile App",
        device: "Mobile",
        browser: "Safari",
        os: "iOS 17",
        ip: "10.0.0.50",
        location: "San Francisco, US",
        issued_at: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(),
        expires_at: new Date(Date.now() + 6 * 24 * 60 * 60 * 1000).toISOString(),
        last_used: new Date(Date.now() - 30 * 60 * 1000).toISOString(),
    },
    {
        id: "tok_3",
        user_id: "user_def456",
        user_email: "jane.smith@example.com",
        client_id: "web-app",
        client_name: "Web Application",
        device: "Desktop",
        browser: "Firefox 121",
        os: "macOS Sonoma",
        ip: "172.16.0.25",
        location: "London, UK",
        issued_at: new Date(Date.now() - 1 * 60 * 60 * 1000).toISOString(),
        expires_at: new Date(Date.now() + 23 * 60 * 60 * 1000).toISOString(),
        last_used: new Date(Date.now() - 2 * 60 * 1000).toISOString(),
    },
    {
        id: "tok_4",
        user_id: "user_ghi789",
        user_email: "bob.wilson@example.com",
        client_id: "api-service",
        client_name: "API Service",
        device: "Server",
        ip: "10.10.10.10",
        location: "AWS us-east-1",
        issued_at: new Date(Date.now() - 48 * 60 * 60 * 1000).toISOString(),
        expires_at: new Date(Date.now() + 5 * 24 * 60 * 60 * 1000).toISOString(),
        last_used: new Date(Date.now() - 1 * 60 * 1000).toISOString(),
    },
]

const MOCK_STATS: TokenStats = {
    total_active: 1247,
    by_client: [
        { client_id: "web-app", client_name: "Web Application", count: 723 },
        { client_id: "mobile-app", client_name: "Mobile App", count: 412 },
        { client_id: "api-service", client_name: "API Service", count: 89 },
        { client_id: "admin-portal", client_name: "Admin Portal", count: 23 },
    ],
    by_device: [
        { device: "Desktop", count: 634 },
        { device: "Mobile", count: 498 },
        { device: "Tablet", count: 78 },
        { device: "Server", count: 37 },
    ],
    issued_today: 234,
    revoked_today: 45,
    avg_lifetime_hours: 18.5,
}

const MOCK_HISTORY: TokenHistoryEntry[] = [
    { id: "h1", action: "issued", user_email: "john.doe@example.com", client_id: "web-app", client_name: "Web App", ip: "192.168.1.100", device: "Chrome/Windows", timestamp: new Date(Date.now() - 5 * 60 * 1000).toISOString() },
    { id: "h2", action: "refreshed", user_email: "jane.smith@example.com", client_id: "mobile-app", client_name: "Mobile App", ip: "10.0.0.50", device: "Safari/iOS", timestamp: new Date(Date.now() - 15 * 60 * 1000).toISOString() },
    { id: "h3", action: "revoked", user_email: "bob.wilson@example.com", client_id: "web-app", client_name: "Web App", reason: "User logout", timestamp: new Date(Date.now() - 30 * 60 * 1000).toISOString() },
    { id: "h4", action: "expired", user_email: "alice.jones@example.com", client_id: "api-service", client_name: "API Service", timestamp: new Date(Date.now() - 1 * 60 * 60 * 1000).toISOString() },
    { id: "h5", action: "issued", user_email: "charlie.brown@example.com", client_id: "admin-portal", client_name: "Admin Portal", ip: "172.16.0.25", device: "Firefox/macOS", timestamp: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString() },
    { id: "h6", action: "revoked", user_email: "john.doe@example.com", client_id: "mobile-app", client_name: "Mobile App", reason: "Session limit reached", timestamp: new Date(Date.now() - 3 * 60 * 60 * 1000).toISOString() },
]

// ----- Helper Functions -----
function decodeJWT(token: string): DecodedJWT | null {
    try {
        const parts = token.split(".")
        if (parts.length !== 3) return null

        const header = JSON.parse(atob(parts[0].replace(/-/g, "+").replace(/_/g, "/")))
        const payload = JSON.parse(atob(parts[1].replace(/-/g, "+").replace(/_/g, "/")))

        return { header, payload, signature: parts[2] }
    } catch {
        return null
    }
}

function formatTimestamp(ts: number): string {
    return new Date(ts * 1000).toLocaleString()
}

function formatDuration(ms: number): string {
    const seconds = Math.floor(ms / 1000)
    const minutes = Math.floor(seconds / 60)
    const hours = Math.floor(minutes / 60)
    const days = Math.floor(hours / 24)

    if (days > 0) return `${days}d ${hours % 24}h`
    if (hours > 0) return `${hours}h ${minutes % 60}m`
    if (minutes > 0) return `${minutes}m`
    return `${seconds}s`
}

function formatRelativeTime(date: string): string {
    const now = Date.now()
    const ts = new Date(date).getTime()
    const diff = now - ts

    if (diff < 0) {
        // Future
        const absDiff = Math.abs(diff)
        if (absDiff < 60 * 1000) return "en menos de 1 minuto"
        if (absDiff < 60 * 60 * 1000) return `en ${Math.floor(absDiff / 60000)} min`
        if (absDiff < 24 * 60 * 60 * 1000) return `en ${Math.floor(absDiff / 3600000)}h`
        return `en ${Math.floor(absDiff / 86400000)} días`
    }

    if (diff < 60 * 1000) return "hace menos de 1 min"
    if (diff < 60 * 60 * 1000) return `hace ${Math.floor(diff / 60000)} min`
    if (diff < 24 * 60 * 60 * 1000) return `hace ${Math.floor(diff / 3600000)}h`
    return `hace ${Math.floor(diff / 86400000)} días`
}

function getExpirationStatus(expiresAt: string): { label: string; variant: "default" | "success" | "warning" | "danger" } {
    const now = Date.now()
    const exp = new Date(expiresAt).getTime()
    const diff = exp - now

    if (diff < 0) return { label: "Expirado", variant: "danger" }
    if (diff < 60 * 60 * 1000) return { label: "< 1h", variant: "warning" }
    if (diff < 24 * 60 * 60 * 1000) return { label: formatDuration(diff), variant: "default" }
    return { label: formatDuration(diff), variant: "success" }
}

function getDeviceIcon(device?: string) {
    if (!device) return Globe
    const d = device.toLowerCase()
    if (d.includes("mobile") || d.includes("phone")) return Smartphone
    if (d.includes("tablet") || d.includes("ipad")) return Tablet
    if (d.includes("server") || d.includes("api")) return Database
    return Monitor
}

// ----- Helper Components -----
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

function StatCard({ icon: Icon, label, value, subvalue, variant = "default" }: {
    icon: any
    label: string
    value: string | number
    subvalue?: string
    variant?: "default" | "success" | "warning" | "danger"
}) {
    const colorClasses = {
        default: "bg-info/10 text-info",
        success: "bg-success/10 text-success",
        warning: "bg-warning/10 text-warning",
        danger: "bg-danger/10 text-danger",
    }
    return (
        <div className="flex items-center gap-3 p-3 rounded-lg bg-muted/30 border">
            <div className={cn("p-2 rounded-lg", colorClasses[variant])}>
                <Icon className="h-4 w-4" />
            </div>
            <div>
                <p className="text-xs text-muted-foreground">{label}</p>
                <p className="text-lg font-semibold">{value}</p>
                {subvalue && <p className="text-xs text-muted-foreground">{subvalue}</p>}
            </div>
        </div>
    )
}

function DistributionBar({ items, total }: { items: { label: string; count: number; color: string }[]; total: number }) {
    return (
        <div className="space-y-2">
            <div className="flex h-3 rounded-full overflow-hidden bg-muted">
                {items.map((item, i) => (
                    <div
                        key={i}
                        className={cn("h-full", item.color)}
                        style={{ width: `${(item.count / total) * 100}%` }}
                        title={`${item.label}: ${item.count}`}
                    />
                ))}
            </div>
            <div className="flex flex-wrap gap-x-4 gap-y-1">
                {items.map((item, i) => (
                    <div key={i} className="flex items-center gap-1.5 text-xs">
                        <div className={cn("w-2.5 h-2.5 rounded-full", item.color)} />
                        <span className="text-muted-foreground">{item.label}</span>
                        <span className="font-medium">{item.count}</span>
                        <span className="text-muted-foreground">({Math.round((item.count / total) * 100)}%)</span>
                    </div>
                ))}
            </div>
        </div>
    )
}

// ----- Main Component -----
export default function TokensClientPage() {
    const params = useParams()
    const tenantIdParam = params.tenant_id as string
    const { t } = useI18n()
    const { token: authToken } = useAuthStore()
    const { toast } = useToast()

    const [selectedTenantId, setSelectedTenantId] = useState<string>(tenantIdParam)
    const [activeTab, setActiveTab] = useState("inspector")

    // Fetch tenants for selector
    const { data: tenants } = useQuery<Tenant[]>({
        queryKey: ["tenants-list"],
        queryFn: () => api.get<Tenant[]>("/v2/admin/tenants"),
        enabled: !!authToken,
    })

    // Auto-select tenant from URL
    useEffect(() => {
        if (tenantIdParam) {
            setSelectedTenantId(tenantIdParam)
        } else if (!selectedTenantId && tenants && tenants.length > 0) {
            setSelectedTenantId(tenants[0].id)
        }
    }, [tenants, tenantIdParam, selectedTenantId])

    return (
        <div className="space-y-6 animate-in fade-in duration-500">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                    <Button variant="ghost" size="sm" asChild>
                        <Link href={`/admin/tenants/${selectedTenantId}/detail`}>
                            <ArrowLeft className="h-4 w-4" />
                        </Link>
                    </Button>
                    <div className="flex items-center gap-3">
                        <div>
                            <h1 className="text-2xl font-bold tracking-tight">Tokens</h1>
                            <p className="text-sm text-muted-foreground">
                                Inspecciona, revoca y monitorea tokens OAuth2.
                            </p>
                        </div>
                    </div>
                </div>
            </div>

            {/* Info Banner */}
            <InlineAlert variant="info">
                <Key className="h-4 w-4" />
                <div>
                    <p className="font-semibold">Gestión de Tokens OAuth2</p>
                    <p className="text-sm opacity-90">
                        Los <strong>Access Tokens</strong> permiten a las aplicaciones acceder a recursos protegidos.
                        Los <strong>Refresh Tokens</strong> permiten obtener nuevos access tokens sin re-autenticación.
                        Aquí puedes inspeccionar, revocar y monitorear todos los tokens activos.
                    </p>
                </div>
            </InlineAlert>

            {!selectedTenantId ? (
                <Card className="p-12 shadow-clay-card">
                    <div className="flex flex-col items-center text-center space-y-4">
                        <div className="relative">
                            <div className="absolute inset-0 bg-gradient-to-br from-warning/20 to-danger/10 rounded-full blur-2xl" />
                            <div className="relative rounded-2xl bg-gradient-to-br from-warning/10 to-danger/5 p-5">
                                <Database className="h-8 w-8 text-warning" />
                            </div>
                        </div>
                        <div className="space-y-2">
                            <h3 className="text-xl font-semibold text-foreground">Selecciona un Tenant</h3>
                            <p className="text-muted-foreground max-w-sm text-sm">
                                Para gestionar tokens, primero selecciona un tenant del menú superior.
                            </p>
                        </div>
                    </div>
                </Card>
            ) : (
                <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-4">
                    <TabsList className="grid w-full max-w-2xl grid-cols-4">
                        <TabsTrigger value="inspector" className="flex items-center gap-2">
                            <Search className="h-4 w-4" />
                            <span className="hidden sm:inline">Inspector</span>
                        </TabsTrigger>
                        <TabsTrigger value="active" className="flex items-center gap-2">
                            <Key className="h-4 w-4" />
                            <span className="hidden sm:inline">Activos</span>
                        </TabsTrigger>
                        <TabsTrigger value="stats" className="flex items-center gap-2">
                            <BarChart3 className="h-4 w-4" />
                            <span className="hidden sm:inline">Estadísticas</span>
                        </TabsTrigger>
                        <TabsTrigger value="history" className="flex items-center gap-2">
                            <History className="h-4 w-4" />
                            <span className="hidden sm:inline">Historial</span>
                        </TabsTrigger>
                    </TabsList>

                    <TabsContent value="inspector" className="space-y-4">
                        <InspectorTab tenantId={selectedTenantId} />
                    </TabsContent>

                    <TabsContent value="active" className="space-y-4">
                        <ActiveTokensTab tenantId={selectedTenantId} />
                    </TabsContent>

                    <TabsContent value="stats" className="space-y-4">
                        <StatsTab tenantId={selectedTenantId} />
                    </TabsContent>

                    <TabsContent value="history" className="space-y-4">
                        <HistoryTab tenantId={selectedTenantId} />
                    </TabsContent>
                </Tabs>
            )}
        </div>
    )
}

// ----------------------------------------------------------------------
// TAB: Inspector
// ----------------------------------------------------------------------
function InspectorTab({ tenantId }: { tenantId: string }) {
    const { toast } = useToast()
    const { t } = useI18n()

    const [clientId, setClientId] = useState("")
    const [clientSecret, setClientSecret] = useState("")
    const [tokenInput, setTokenInput] = useState("")
    const [includeSys, setIncludeSys] = useState(true)
    const [showSecret, setShowSecret] = useState(false)
    const [result, setResult] = useState<IntrospectionResult | null>(null)
    const [decodedJWT, setDecodedJWT] = useState<DecodedJWT | null>(null)
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [activeView, setActiveView] = useState<"introspect" | "decoded">("introspect")
    const [showFullJson, setShowFullJson] = useState(false)

    const basicHeader = () => {
        const creds = btoa(`${clientId}:${clientSecret}`)
        return { Authorization: `Basic ${creds}` }
    }

    const introspect = async () => {
        setLoading(true)
        setError(null)
        try {
            const form = new URLSearchParams()
            form.set("token", tokenInput)
            if (includeSys) form.set("include_sys", "1")
            const res = await api.postForm<IntrospectionResult>(`/oauth2/introspect`, form, basicHeader())
            setResult(res)
            setActiveView("introspect")

            // Also decode JWT
            const decoded = decodeJWT(tokenInput)
            setDecodedJWT(decoded)
        } catch (e: any) {
            setError(e.message || "Introspect failed")
            toast({ title: "Error", description: e.message || "Introspect failed", variant: "destructive" })
        } finally {
            setLoading(false)
        }
    }

    const revoke = async () => {
        setLoading(true)
        setError(null)
        try {
            const form = new URLSearchParams()
            form.set("token", tokenInput)
            await api.postForm(`/oauth2/revoke`, form, basicHeader())
            toast({ title: "Token revocado", description: "El token ha sido revocado exitosamente." })
            setResult(null)
            setDecodedJWT(null)
        } catch (e: any) {
            setError(e.message || "Revoke failed")
            toast({ title: "Error", description: e.message || "Revoke failed", variant: "destructive" })
        } finally {
            setLoading(false)
        }
    }

    const copyToClipboard = (text: string, label: string) => {
        navigator.clipboard.writeText(text)
        toast({ title: "Copiado", description: `${label} copiado al portapapeles.` })
    }

    const clearAll = () => {
        setTokenInput("")
        setResult(null)
        setDecodedJWT(null)
        setError(null)
    }

    return (
        <div className="grid gap-6 lg:grid-cols-2">
            {/* Input Panel */}
            <Card>
                <CardHeader>
                    <div className="flex items-center gap-2">
                        <div className="p-2 bg-info/10 rounded-lg">
                            <Search className="h-5 w-5 text-info" />
                        </div>
                        <div>
                            <CardTitle className="text-lg">Token Inspector</CardTitle>
                            <CardDescription>
                                Inspecciona o revoca un token OAuth2.
                            </CardDescription>
                        </div>
                    </div>
                </CardHeader>
                <CardContent className="space-y-4">
                    {/* Credentials */}
                    <div className="p-4 bg-muted/30 rounded-lg border border-dashed space-y-3">
                        <Label className="text-xs font-semibold text-muted-foreground uppercase flex items-center gap-1">
                            <Lock className="h-3 w-3" />
                            Credenciales del Client
                            <InfoTooltip content="Las credenciales del client que emitió el token. Necesarias para validar la introspección." />
                        </Label>
                        <div className="grid gap-3">
                            <Input
                                placeholder="client_id"
                                value={clientId}
                                onChange={(e) => setClientId(e.target.value)}
                            />
                            <div className="relative">
                                <Input
                                    placeholder="client_secret"
                                    type={showSecret ? "text" : "password"}
                                    value={clientSecret}
                                    onChange={(e) => setClientSecret(e.target.value)}
                                />
                                <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    className="absolute right-1 top-1/2 -translate-y-1/2 h-7 w-7 p-0"
                                    onClick={() => setShowSecret(!showSecret)}
                                >
                                    {showSecret ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                                </Button>
                            </div>
                        </div>
                    </div>

                    {/* Token Input */}
                    <div className="space-y-2">
                        <Label className="flex items-center gap-1">
                            Token
                            <InfoTooltip content="Pega aquí el access_token o refresh_token que quieres inspeccionar." />
                        </Label>
                        <Textarea
                            placeholder="eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9..."
                            value={tokenInput}
                            onChange={(e) => setTokenInput(e.target.value)}
                            className="font-mono text-xs min-h-[100px] resize-y"
                        />
                    </div>

                    {/* Options */}
                    <div className="flex items-center gap-4">
                        <label className="flex items-center gap-2 text-sm cursor-pointer">
                            <Checkbox
                                checked={includeSys}
                                onCheckedChange={(v) => setIncludeSys(!!v)}
                            />
                            <span className="text-muted-foreground">
                                Incluir roles y permisos
                                <InfoTooltip content="Si está habilitado, la respuesta incluirá los roles y permisos del usuario (si existen)." />
                            </span>
                        </label>
                    </div>

                    {/* Actions */}
                    <div className="flex gap-2 pt-2">
                        <Button
                            onClick={introspect}
                            disabled={!clientId || !clientSecret || !tokenInput || loading}
                            className="flex-1"
                        >
                            {loading ? (
                                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                            ) : (
                                <Search className="mr-2 h-4 w-4" />
                            )}
                            Introspect
                        </Button>
                        <Button
                            variant="danger"
                            onClick={revoke}
                            disabled={!clientId || !clientSecret || !tokenInput || loading}
                        >
                            {loading ? (
                                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                            ) : (
                                <Ban className="mr-2 h-4 w-4" />
                            )}
                            Revoke
                        </Button>
                        {(result || decodedJWT) && (
                            <Button variant="outline" onClick={clearAll}>
                                <RefreshCw className="h-4 w-4" />
                            </Button>
                        )}
                    </div>

                    {error && (
                        <InlineAlert variant="danger">
                            <AlertCircle className="h-4 w-4" />
                            {error}
                        </InlineAlert>
                    )}
                </CardContent>
            </Card>

            {/* Results Panel */}
            <Card>
                <CardHeader>
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                            <div className="p-2 bg-accent/10 rounded-lg">
                                <FileJson className="h-5 w-5 text-accent" />
                            </div>
                            <div>
                                <CardTitle className="text-lg">Resultado</CardTitle>
                                <CardDescription>
                                    {result
                                        ? result.active
                                            ? "Token válido y activo"
                                            : "Token inválido o expirado"
                                        : "Ingresa un token para inspeccionar"
                                    }
                                </CardDescription>
                            </div>
                        </div>
                        {result && (
                            <Badge variant={result.active ? "default" : "destructive"} className="text-xs">
                                {result.active ? (
                                    <><CheckCircle2 className="h-3 w-3 mr-1" /> Activo</>
                                ) : (
                                    <><XCircle className="h-3 w-3 mr-1" /> Inactivo</>
                                )}
                            </Badge>
                        )}
                    </div>
                </CardHeader>
                <CardContent>
                    {!result && !decodedJWT ? (
                        <div className="flex flex-col items-center justify-center py-12 text-center">
                            <Key className="h-12 w-12 text-muted-foreground/30 mb-4" />
                            <p className="text-muted-foreground text-sm">
                                Los resultados aparecerán aquí
                            </p>
                        </div>
                    ) : (
                        <div className="space-y-4">
                            {/* View Toggle */}
                            {decodedJWT && (
                                <div className="flex gap-2 p-1 bg-muted rounded-lg">
                                    <Button
                                        variant={activeView === "introspect" ? "default" : "ghost"}
                                        size="sm"
                                        className="flex-1"
                                        onClick={() => setActiveView("introspect")}
                                    >
                                        Introspection
                                    </Button>
                                    <Button
                                        variant={activeView === "decoded" ? "default" : "ghost"}
                                        size="sm"
                                        className="flex-1"
                                        onClick={() => setActiveView("decoded")}
                                    >
                                        JWT Decoded
                                    </Button>
                                </div>
                            )}

                            {activeView === "introspect" && result && (
                                <div className="space-y-4">
                                    {/* Key claims */}
                                    <div className="grid grid-cols-2 gap-3">
                                        {result.sub && (
                                            <div className="p-3 rounded-lg bg-muted/50 border">
                                                <p className="text-xs text-muted-foreground">Subject (User ID)</p>
                                                <p className="font-mono text-sm truncate">{result.sub}</p>
                                            </div>
                                        )}
                                        {result.client_id && (
                                            <div className="p-3 rounded-lg bg-muted/50 border">
                                                <p className="text-xs text-muted-foreground">Client ID</p>
                                                <p className="font-mono text-sm truncate">{result.client_id}</p>
                                            </div>
                                        )}
                                        {result.exp && (
                                            <div className="p-3 rounded-lg bg-muted/50 border">
                                                <p className="text-xs text-muted-foreground">Expires</p>
                                                <p className="text-sm">
                                                    {formatTimestamp(result.exp)}
                                                    {result.exp * 1000 < Date.now() && (
                                                        <Badge variant="destructive" className="ml-2 text-[10px]">Expirado</Badge>
                                                    )}
                                                </p>
                                            </div>
                                        )}
                                        {result.scope && (
                                            <div className="p-3 rounded-lg bg-muted/50 border">
                                                <p className="text-xs text-muted-foreground">Scopes</p>
                                                <div className="flex flex-wrap gap-1 mt-1">
                                                    {result.scope.split(" ").map((s) => (
                                                        <Badge key={s} variant="secondary" className="text-[10px]">{s}</Badge>
                                                    ))}
                                                </div>
                                            </div>
                                        )}
                                    </div>

                                    {/* Roles & Permissions */}
                                    {(result.roles || result.permissions) && (
                                        <div className="space-y-2">
                                            {result.roles && result.roles.length > 0 && (
                                                <div className="p-3 rounded-lg bg-success/5 border border-success/20">
                                                    <p className="text-xs text-muted-foreground flex items-center gap-1">
                                                        <Shield className="h-3 w-3" /> Roles
                                                    </p>
                                                    <div className="flex flex-wrap gap-1 mt-1">
                                                        {result.roles.map((r) => (
                                                            <Badge key={r} variant="outline" className="text-[10px] bg-success/10">{r}</Badge>
                                                        ))}
                                                    </div>
                                                </div>
                                            )}
                                            {result.permissions && result.permissions.length > 0 && (
                                                <div className="p-3 rounded-lg bg-info/5 border border-info/20">
                                                    <p className="text-xs text-muted-foreground flex items-center gap-1">
                                                        <Key className="h-3 w-3" /> Permissions
                                                    </p>
                                                    <div className="flex flex-wrap gap-1 mt-1 max-h-[80px] overflow-y-auto">
                                                        {result.permissions.map((p) => (
                                                            <Badge key={p} variant="outline" className="text-[10px] font-mono bg-info/10">{p}</Badge>
                                                        ))}
                                                    </div>
                                                </div>
                                            )}
                                        </div>
                                    )}

                                    {/* Full JSON */}
                                    <div className="space-y-2">
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            className="w-full justify-between"
                                            onClick={() => setShowFullJson(!showFullJson)}
                                        >
                                            <span className="text-xs text-muted-foreground">Ver JSON completo</span>
                                            {showFullJson ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                                        </Button>
                                        {showFullJson && (
                                            <div className="relative mt-2 animate-in slide-in-from-top-2 duration-200">
                                                <pre className="p-3 rounded-lg bg-muted/50 border text-xs font-mono overflow-auto max-h-[200px]">
                                                    {JSON.stringify(result, null, 2)}
                                                </pre>
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="absolute top-2 right-2 h-7 w-7 p-0"
                                                    onClick={() => copyToClipboard(JSON.stringify(result, null, 2), "JSON")}
                                                >
                                                    <Copy className="h-3.5 w-3.5" />
                                                </Button>
                                            </div>
                                        )}
                                    </div>
                                </div>
                            )}

                            {activeView === "decoded" && decodedJWT && (
                                <div className="space-y-3">
                                    {/* Header */}
                                    <div className="p-3 rounded-lg bg-warning/5 border border-warning/20">
                                        <div className="flex items-center justify-between mb-2">
                                            <p className="text-xs font-semibold text-warning">HEADER</p>
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                className="h-6 w-6 p-0"
                                                onClick={() => copyToClipboard(JSON.stringify(decodedJWT.header, null, 2), "Header")}
                                            >
                                                <Copy className="h-3 w-3" />
                                            </Button>
                                        </div>
                                        <pre className="text-xs font-mono text-warning overflow-auto">
                                            {JSON.stringify(decodedJWT.header, null, 2)}
                                        </pre>
                                    </div>

                                    {/* Payload */}
                                    <div className="p-3 rounded-lg bg-accent/5 border border-accent/20">
                                        <div className="flex items-center justify-between mb-2">
                                            <p className="text-xs font-semibold text-accent">PAYLOAD</p>
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                className="h-6 w-6 p-0"
                                                onClick={() => copyToClipboard(JSON.stringify(decodedJWT.payload, null, 2), "Payload")}
                                            >
                                                <Copy className="h-3 w-3" />
                                            </Button>
                                        </div>
                                        <pre className="text-xs font-mono text-accent overflow-auto max-h-[200px]">
                                            {JSON.stringify(decodedJWT.payload, null, 2)}
                                        </pre>
                                    </div>

                                    {/* Signature */}
                                    <div className="p-3 rounded-lg bg-info/5 border border-info/20">
                                        <div className="flex items-center justify-between mb-2">
                                            <p className="text-xs font-semibold text-info">SIGNATURE</p>
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                className="h-6 w-6 p-0"
                                                onClick={() => copyToClipboard(decodedJWT.signature, "Signature")}
                                            >
                                                <Copy className="h-3 w-3" />
                                            </Button>
                                        </div>
                                        <p className="text-xs font-mono text-info break-all">
                                            {decodedJWT.signature}
                                        </p>
                                    </div>
                                </div>
                            )}
                        </div>
                    )}
                </CardContent>
            </Card>
        </div>
    )
}

// ----------------------------------------------------------------------
// TAB: Active Tokens
// ----------------------------------------------------------------------
function ActiveTokensTab({ tenantId }: { tenantId: string }) {
    const { toast } = useToast()
    const queryClient = useQueryClient()
    const [search, setSearch] = useState("")
    const [filterClient, setFilterClient] = useState<string>("_all")
    const [filterStatus, setFilterStatus] = useState<string>("active")
    const [page, setPage] = useState(1)
    const [selectedTokens, setSelectedTokens] = useState<Set<string>>(new Set())

    // Fetch tokens from API
    const { data: tokensData, isLoading, refetch, error } = useQuery({
        queryKey: ["admin-tokens", tenantId, page, filterClient, filterStatus, search],
        queryFn: () => tokensAdminAPI.list(tenantId, {
            page,
            page_size: 50,
            client_id: filterClient !== "_all" ? filterClient : undefined,
            status: filterStatus !== "_all" ? filterStatus as "active" | "expired" | "revoked" : undefined,
            search: search || undefined,
        }),
        enabled: !!tenantId,
        retry: (failureCount, error) => {
            if (isNoDatabaseError(error)) return false
            return failureCount < 3
        },
    })



    const tokens = tokensData?.tokens || []
    const totalCount = tokensData?.total_count || 0

    // Revoke single token mutation
    const revokeMutation = useMutation({
        mutationFn: (tokenId: string) => tokensAdminAPI.revoke(tenantId, tokenId),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["admin-tokens", tenantId] })
            queryClient.invalidateQueries({ queryKey: ["admin-token-stats", tenantId] })
            toast({ title: "Token revocado", description: "El token ha sido revocado exitosamente." })
        },
        onError: (err: any) => {
            toast({ title: "Error", description: err.message || "No se pudo revocar el token", variant: "destructive" })
        },
    })

    // Revoke by user mutation
    const revokeByUserMutation = useMutation({
        mutationFn: (userId: string) => tokensAdminAPI.revokeByUser(tenantId, userId),
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["admin-tokens", tenantId] })
            queryClient.invalidateQueries({ queryKey: ["admin-token-stats", tenantId] })
            toast({ title: "Tokens revocados", description: data.message || `${data.revoked_count} tokens revocados` })
        },
        onError: (err: any) => {
            toast({ title: "Error", description: err.message || "No se pudieron revocar los tokens", variant: "destructive" })
        },
    })

    // Get unique clients from tokens
    const clients = useMemo(() => {
        const map = new Map<string, string>()
        tokens.forEach((t) => map.set(t.client_id, t.client_id))
        return Array.from(map.entries())
    }, [tokens])

    const toggleSelect = (id: string) => {
        const newSet = new Set(selectedTokens)
        if (newSet.has(id)) {
            newSet.delete(id)
        } else {
            newSet.add(id)
        }
        setSelectedTokens(newSet)
    }

    const toggleSelectAll = () => {
        if (selectedTokens.size === tokens.length) {
            setSelectedTokens(new Set())
        } else {
            setSelectedTokens(new Set(tokens.map((t) => t.id)))
        }
    }

    const revokeSelected = async () => {
        for (const tokenId of selectedTokens) {
            await revokeMutation.mutateAsync(tokenId)
        }
        setSelectedTokens(new Set())
    }

    const revokeToken = (id: string) => {
        revokeMutation.mutate(id)
    }

    // Debounce search
    useEffect(() => {
        const timer = setTimeout(() => {
            setPage(1)
        }, 300)
        return () => clearTimeout(timer)
    }, [search])

    if (isNoDatabaseError(error)) {
        return (
            <NoDatabaseConfigured
                tenantId={tenantId}
                message="Conecta una base de datos para ver los tokens activos."
            />
        )
    }

    return (
        <div className="space-y-4">
            {/* Filters */}
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
                <div className="flex items-center gap-2 flex-wrap">
                    <div className="relative">
                        <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                        <Input
                            placeholder="Buscar por email..."
                            value={search}
                            onChange={(e) => setSearch(e.target.value)}
                            className="pl-9 h-9 w-[200px]"
                        />
                    </div>
                    <Select value={filterClient} onValueChange={(v) => { setFilterClient(v); setPage(1); }}>
                        <SelectTrigger className="w-[150px] h-9">
                            <SelectValue placeholder="Client" />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="_all">Todos los clients</SelectItem>
                            {clients.map(([id]) => (
                                <SelectItem key={id} value={id}>{id}</SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    <Select value={filterStatus} onValueChange={(v) => { setFilterStatus(v); setPage(1); }}>
                        <SelectTrigger className="w-[130px] h-9">
                            <SelectValue placeholder="Estado" />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="_all">Todos</SelectItem>
                            <SelectItem value="active">Activos</SelectItem>
                            <SelectItem value="expired">Expirados</SelectItem>
                            <SelectItem value="revoked">Revocados</SelectItem>
                        </SelectContent>
                    </Select>
                </div>
                <div className="flex items-center gap-2">
                    <Button variant="outline" size="sm" onClick={() => refetch()} disabled={isLoading}>
                        <RefreshCw className={cn("h-4 w-4 mr-2", isLoading && "animate-spin")} />
                        Refresh
                    </Button>
                </div>
            </div>

            {/* Bulk Actions */}
            {selectedTokens.size > 0 && (
                <div className="flex items-center justify-between p-3 bg-destructive/10 border border-destructive/30 rounded-lg animate-in slide-in-from-top-2">
                    <p className="text-sm">
                        <strong>{selectedTokens.size}</strong> token(s) seleccionado(s)
                    </p>
                    <div className="flex items-center gap-2">
                        <Button variant="ghost" size="sm" onClick={() => setSelectedTokens(new Set())}>
                            Cancelar
                        </Button>
                        <Button variant="danger" size="sm" onClick={revokeSelected} disabled={revokeMutation.isPending}>
                            {revokeMutation.isPending ? (
                                <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                            ) : (
                                <Ban className="h-4 w-4 mr-2" />
                            )}
                            Revocar Todos
                        </Button>
                    </div>
                </div>
            )}

            {/* Table */}
            <div className="border rounded-lg overflow-hidden">
                <Table>
                    <TableHeader>
                        <TableRow className="bg-muted/30">
                            <TableHead className="w-[40px]">
                                <Checkbox
                                    checked={selectedTokens.size === tokens.length && tokens.length > 0}
                                    onCheckedChange={toggleSelectAll}
                                />
                            </TableHead>
                            <TableHead>Usuario</TableHead>
                            <TableHead>Client</TableHead>
                            <TableHead>Emitido</TableHead>
                            <TableHead>Expira</TableHead>
                            <TableHead>Estado</TableHead>
                            <TableHead className="w-[60px]"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {isLoading ? (
                            <TableRow>
                                <TableCell colSpan={7} className="text-center py-8">
                                    <Loader2 className="h-6 w-6 animate-spin mx-auto text-muted-foreground" />
                                </TableCell>
                            </TableRow>
                        ) : tokens.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                                    No se encontraron tokens.
                                </TableCell>
                            </TableRow>
                        ) : (
                            tokens.map((token) => {
                                const expStatus = getExpirationStatus(token.expires_at)
                                return (
                                    <TableRow key={token.id}>
                                        <TableCell>
                                            <Checkbox
                                                checked={selectedTokens.has(token.id)}
                                                onCheckedChange={() => toggleSelect(token.id)}
                                            />
                                        </TableCell>
                                        <TableCell>
                                            <div className="flex flex-col">
                                                <span className="font-medium text-sm">{token.user_email || "—"}</span>
                                                <span className="text-xs text-muted-foreground font-mono">{token.user_id}</span>
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            <span className="text-sm font-mono">{token.client_id}</span>
                                        </TableCell>
                                        <TableCell>
                                            <span className="text-sm text-muted-foreground">
                                                {new Date(token.issued_at).toLocaleString()}
                                            </span>
                                        </TableCell>
                                        <TableCell>
                                            <span className="text-sm text-muted-foreground">
                                                {new Date(token.expires_at).toLocaleString()}
                                            </span>
                                        </TableCell>
                                        <TableCell>
                                            <Badge
                                                variant={
                                                    token.status === "active" ? "default" :
                                                        token.status === "revoked" ? "destructive" : "outline"
                                                }
                                                className="text-xs"
                                            >
                                                {token.status === "active" ? "Activo" :
                                                    token.status === "revoked" ? "Revocado" : "Expirado"}
                                            </Badge>
                                        </TableCell>
                                        <TableCell>
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild>
                                                    <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
                                                        <MoreHorizontal className="h-4 w-4" />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    <DropdownMenuItem onClick={() => navigator.clipboard.writeText(token.id)}>
                                                        <Copy className="mr-2 h-4 w-4" /> Copiar Token ID
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem onClick={() => navigator.clipboard.writeText(token.user_id)}>
                                                        <Copy className="mr-2 h-4 w-4" /> Copiar User ID
                                                    </DropdownMenuItem>
                                                    <DropdownMenuSeparator />
                                                    <DropdownMenuItem
                                                        onClick={() => revokeByUserMutation.mutate(token.user_id)}
                                                        disabled={token.status !== "active"}
                                                    >
                                                        <Users className="mr-2 h-4 w-4" /> Revocar del Usuario
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem
                                                        onClick={() => revokeToken(token.id)}
                                                        className="text-danger"
                                                        disabled={token.status !== "active"}
                                                    >
                                                        <Ban className="mr-2 h-4 w-4" /> Revocar
                                                    </DropdownMenuItem>
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </TableCell>
                                    </TableRow>
                                )
                            })
                        )}
                    </TableBody>
                </Table>
            </div>

            {/* Pagination */}
            {totalCount > 50 && (
                <div className="flex items-center justify-between">
                    <p className="text-sm text-muted-foreground">
                        Mostrando {tokens.length} de {totalCount} tokens
                    </p>
                    <div className="flex items-center gap-2">
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setPage(p => Math.max(1, p - 1))}
                            disabled={page === 1}
                        >
                            Anterior
                        </Button>
                        <span className="text-sm">Página {page}</span>
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setPage(p => p + 1)}
                            disabled={tokens.length < 50}
                        >
                            Siguiente
                        </Button>
                    </div>
                </div>
            )}
        </div>
    )
}

// ----------------------------------------------------------------------
// TAB: Statistics
// ----------------------------------------------------------------------
function StatsTab({ tenantId }: { tenantId: string }) {
    const { toast } = useToast()
    const queryClient = useQueryClient()

    // Fetch stats from API
    const { data: stats, isLoading, refetch, error } = useQuery({
        queryKey: ["admin-token-stats", tenantId],
        queryFn: () => tokensAdminAPI.getStats(tenantId),
        enabled: !!tenantId,
        retry: (failureCount, error) => {
            if (isNoDatabaseError(error)) return false
            return failureCount < 3
        },
    })



    // Revoke all mutation
    const revokeAllMutation = useMutation({
        mutationFn: () => tokensAdminAPI.revokeAll(tenantId),
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["admin-tokens", tenantId] })
            queryClient.invalidateQueries({ queryKey: ["admin-token-stats", tenantId] })
            toast({ title: "Tokens revocados", description: data.message || `${data.revoked_count} tokens revocados` })
        },
        onError: (err: any) => {
            toast({ title: "Error", description: err.message || "No se pudieron revocar los tokens", variant: "destructive" })
        },
    })

    // Revoke by client mutation
    const revokeByClientMutation = useMutation({
        mutationFn: (clientId: string) => tokensAdminAPI.revokeByClient(tenantId, clientId),
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["admin-tokens", tenantId] })
            queryClient.invalidateQueries({ queryKey: ["admin-token-stats", tenantId] })
            toast({ title: "Tokens revocados", description: data.message || `${data.revoked_count} tokens revocados` })
        },
        onError: (err: any) => {
            toast({ title: "Error", description: err.message || "No se pudieron revocar los tokens", variant: "destructive" })
        },
    })

    const clientColors = ["bg-info", "bg-accent", "bg-success", "bg-warning", "bg-danger", "bg-muted-foreground"]

    const [showRevokeAllConfirm, setShowRevokeAllConfirm] = useState(false)
    const [selectedClientForRevoke, setSelectedClientForRevoke] = useState<string | null>(null)

    if (isNoDatabaseError(error)) {
        return (
            <NoDatabaseConfigured
                tenantId={tenantId}
                message="Conecta una base de datos para ver las estadísticas."
            />
        )
    }

    if (isLoading) {
        return (
            <div className="flex items-center justify-center py-20">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
        )
    }

    if (!stats) {
        return (
            <div className="flex flex-col items-center justify-center py-20">
                <AlertCircle className="h-12 w-12 text-muted-foreground/50 mb-4" />
                <p className="text-muted-foreground">No se pudieron cargar las estadísticas</p>
                <Button variant="outline" size="sm" className="mt-4" onClick={() => refetch()}>
                    <RefreshCw className="h-4 w-4 mr-2" />
                    Reintentar
                </Button>
            </div>
        )
    }

    return (
        <div className="space-y-6">
            {/* Stats Cards */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                <StatCard
                    icon={Key}
                    label="Tokens Activos"
                    value={stats.total_active.toLocaleString()}
                    variant="default"
                />
                <StatCard
                    icon={Zap}
                    label="Emitidos Hoy"
                    value={stats.issued_today}
                    variant="success"
                />
                <StatCard
                    icon={Ban}
                    label="Revocados Hoy"
                    value={stats.revoked_today}
                    variant="danger"
                />
                <StatCard
                    icon={Timer}
                    label="Vida Promedio"
                    value={`${stats.avg_lifetime_hours.toFixed(1)}h`}
                    variant="warning"
                />
            </div>

            {/* Distribution Charts */}
            <div className="grid gap-6 lg:grid-cols-2">
                {/* By Client */}
                <Card>
                    <CardHeader>
                        <CardTitle className="text-base flex items-center gap-2">
                            <Activity className="h-4 w-4 text-info" />
                            Distribución por Client
                        </CardTitle>
                        <CardDescription>
                            Tokens activos agrupados por aplicación cliente.
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        {stats.by_client && stats.by_client.length > 0 ? (
                            <DistributionBar
                                items={stats.by_client.map((c, i) => ({
                                    label: c.client_id,
                                    count: c.count,
                                    color: clientColors[i % clientColors.length],
                                }))}
                                total={stats.total_active}
                            />
                        ) : (
                            <p className="text-sm text-muted-foreground text-center py-4">
                                No hay datos de distribución por client
                            </p>
                        )}
                    </CardContent>
                </Card>

                {/* Refresh Button */}
                <Card>
                    <CardHeader>
                        <CardTitle className="text-base flex items-center gap-2">
                            <RefreshCw className="h-4 w-4 text-accent" />
                            Actualizar Estadísticas
                        </CardTitle>
                        <CardDescription>
                            Refresca las estadísticas de tokens.
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <Button variant="outline" onClick={() => refetch()} disabled={isLoading}>
                            <RefreshCw className={cn("h-4 w-4 mr-2", isLoading && "animate-spin")} />
                            Actualizar
                        </Button>
                    </CardContent>
                </Card>
            </div>

            {/* Quick Actions */}
            <Card>
                <CardHeader>
                    <CardTitle className="text-base flex items-center gap-2">
                        <ShieldAlert className="h-4 w-4 text-danger" />
                        Acciones Rápidas
                    </CardTitle>
                    <CardDescription>
                        Operaciones de revocación masiva.
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                        {stats.by_client && stats.by_client.map((client) => (
                            <Button
                                key={client.client_id}
                                variant="outline"
                                className="justify-start h-auto py-3"
                                onClick={() => setSelectedClientForRevoke(client.client_id)}
                            >
                                <div className="flex flex-col items-start gap-1">
                                    <span className="flex items-center gap-2">
                                        <Globe className="h-4 w-4" />
                                        {client.client_id}
                                    </span>
                                    <span className="text-xs text-muted-foreground font-normal">
                                        {client.count} tokens activos
                                    </span>
                                </div>
                            </Button>
                        ))}
                        <Button
                            variant="danger"
                            className="justify-start h-auto py-3"
                            onClick={() => setShowRevokeAllConfirm(true)}
                        >
                            <div className="flex flex-col items-start gap-1">
                                <span className="flex items-center gap-2">
                                    <Ban className="h-4 w-4" />
                                    Revocar Todos
                                </span>
                                <span className="text-xs font-normal opacity-80">
                                    ⚠️ Cierra todas las sesiones activas
                                </span>
                            </div>
                        </Button>
                    </div>
                </CardContent>
            </Card>

            {/* Revoke All Confirmation Dialog */}
            <Dialog open={showRevokeAllConfirm} onOpenChange={setShowRevokeAllConfirm}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2 text-destructive">
                            <AlertTriangle className="h-5 w-5" />
                            Confirmar Revocación Masiva
                        </DialogTitle>
                        <DialogDescription>
                            Esta acción revocará <strong>todos los {stats.total_active} tokens activos</strong> del tenant.
                            Todos los usuarios deberán volver a iniciar sesión.
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setShowRevokeAllConfirm(false)}>
                            Cancelar
                        </Button>
                        <Button
                            variant="danger"
                            onClick={() => {
                                revokeAllMutation.mutate()
                                setShowRevokeAllConfirm(false)
                            }}
                            disabled={revokeAllMutation.isPending}
                        >
                            {revokeAllMutation.isPending ? (
                                <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                            ) : (
                                <Ban className="h-4 w-4 mr-2" />
                            )}
                            Sí, Revocar Todos
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Revoke by Client Confirmation Dialog */}
            <Dialog open={!!selectedClientForRevoke} onOpenChange={() => setSelectedClientForRevoke(null)}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2">
                            <Globe className="h-5 w-5 text-warning" />
                            Revocar Tokens del Client
                        </DialogTitle>
                        <DialogDescription>
                            ¿Estás seguro de revocar todos los tokens del client <strong>{selectedClientForRevoke}</strong>?
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setSelectedClientForRevoke(null)}>
                            Cancelar
                        </Button>
                        <Button
                            variant="danger"
                            onClick={() => {
                                if (selectedClientForRevoke) {
                                    revokeByClientMutation.mutate(selectedClientForRevoke)
                                }
                                setSelectedClientForRevoke(null)
                            }}
                            disabled={revokeByClientMutation.isPending}
                        >
                            {revokeByClientMutation.isPending ? (
                                <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                            ) : (
                                <Ban className="h-4 w-4 mr-2" />
                            )}
                            Revocar
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}

// ----------------------------------------------------------------------
// TAB: History
// ----------------------------------------------------------------------
function HistoryTab({ tenantId }: { tenantId: string }) {
    const [filterAction, setFilterAction] = useState<string>("_all")
    const history = MOCK_HISTORY

    // Helper query to check DB status
    const { error } = useQuery({
        queryKey: ["check-db-status", tenantId],
        queryFn: () => tokensAdminAPI.getStats(tenantId),
        enabled: !!tenantId,
        retry: (failureCount, error) => {
            if (isNoDatabaseError(error)) return false
            return failureCount < 3
        },
    })



    const filteredHistory = useMemo(() => {
        if (filterAction === "_all") return history
        return history.filter((h) => h.action === filterAction)
    }, [history, filterAction])

    const getActionBadge = (action: TokenHistoryEntry["action"]) => {
        switch (action) {
            case "issued":
                return <Badge variant="default" className="bg-success/10 text-success border-success/30">Emitido</Badge>
            case "refreshed":
                return <Badge variant="default" className="bg-info/10 text-info border-info/30">Renovado</Badge>
            case "revoked":
                return <Badge variant="destructive">Revocado</Badge>
            case "expired":
                return <Badge variant="outline" className="text-muted-foreground">Expirado</Badge>
        }
    }

    const getActionIcon = (action: TokenHistoryEntry["action"]) => {
        switch (action) {
            case "issued":
                return <CheckCircle2 className="h-4 w-4 text-success" />
            case "refreshed":
                return <RefreshCw className="h-4 w-4 text-info" />
            case "revoked":
                return <Ban className="h-4 w-4 text-danger" />
            case "expired":
                return <Clock className="h-4 w-4 text-muted-foreground" />
        }
    }

    if (isNoDatabaseError(error)) {
        return (
            <NoDatabaseConfigured
                tenantId={tenantId}
                message="Conecta una base de datos para ver el historial de tokens."
            />
        )
    }

    return (
        <div className="space-y-4">
            {/* Info */}
            <InlineAlert variant="warning">
                <AlertTriangle className="h-4 w-4" />
                <span><strong>Nota:</strong> Este historial muestra datos de ejemplo. El backend necesita implementar
                    un endpoint de auditoría para tokens.</span>
            </InlineAlert>

            {/* Filters */}
            <div className="flex items-center gap-3">
                <Select value={filterAction} onValueChange={setFilterAction}>
                    <SelectTrigger className="w-[150px] h-9">
                        <SelectValue placeholder="Acción" />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="_all">Todas</SelectItem>
                        <SelectItem value="issued">Emitidos</SelectItem>
                        <SelectItem value="refreshed">Renovados</SelectItem>
                        <SelectItem value="revoked">Revocados</SelectItem>
                        <SelectItem value="expired">Expirados</SelectItem>
                    </SelectContent>
                </Select>
                <Button variant="outline" size="sm">
                    <RefreshCw className="h-4 w-4 mr-2" />
                    Actualizar
                </Button>
            </div>

            {/* Timeline */}
            <Card>
                <CardContent className="p-0">
                    <div className="divide-y">
                        {filteredHistory.map((entry, index) => (
                            <div key={entry.id} className="flex items-start gap-4 p-4 hover:bg-muted/30 transition-colors">
                                <div className="mt-0.5">
                                    {getActionIcon(entry.action)}
                                </div>
                                <div className="flex-1 min-w-0">
                                    <div className="flex items-center gap-2 flex-wrap">
                                        {getActionBadge(entry.action)}
                                        <span className="text-sm font-medium">
                                            {entry.user_email || "Sistema"}
                                        </span>
                                        <ArrowRight className="h-3 w-3 text-muted-foreground" />
                                        <span className="text-sm text-muted-foreground">
                                            {entry.client_name || entry.client_id}
                                        </span>
                                    </div>
                                    <div className="flex items-center gap-4 mt-1 text-xs text-muted-foreground">
                                        {entry.ip && (
                                            <span className="flex items-center gap-1">
                                                <Globe className="h-3 w-3" /> {entry.ip}
                                            </span>
                                        )}
                                        {entry.device && (
                                            <span className="flex items-center gap-1">
                                                <Monitor className="h-3 w-3" /> {entry.device}
                                            </span>
                                        )}
                                        {entry.reason && (
                                            <span className="flex items-center gap-1 text-warning">
                                                <Info className="h-3 w-3" /> {entry.reason}
                                            </span>
                                        )}
                                    </div>
                                </div>
                                <div className="text-xs text-muted-foreground whitespace-nowrap">
                                    {formatRelativeTime(entry.timestamp)}
                                </div>
                            </div>
                        ))}
                    </div>
                </CardContent>
            </Card>
        </div>
    )
}
