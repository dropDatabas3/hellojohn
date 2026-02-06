"use client"

import { useState, useMemo, Suspense } from "react"
import { useParams, useRouter } from "next/navigation"
import Link from "next/link"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
    Monitor, Smartphone, Tablet, Globe, Clock, Shield, Users,
    Search, RefreshCw, LogOut, Trash2, Settings2, MoreHorizontal,
    MapPin, Activity, AlertCircle, CheckCircle2, ChevronRight,
    Laptop, HelpCircle, Ban, Eye, Filter, X, Zap, Timer, ArrowLeft
} from "lucide-react"
import { api, sessionsAdminAPI } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { useToast } from "@/hooks/use-toast"
import type { Tenant, SessionPolicy, SessionResponse, SessionStats as SessionStatsType } from "@/lib/types"

// Design System Components
import {
    PageShell,
    Card,
    CardContent,
    Button,
    Input,
    Label,
    Switch,
    Badge,
    Tabs,
    TabsContent,
    TabsList,
    TabsTrigger,
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
    EmptyState,
    Skeleton,
    InlineAlert,
    NoDatabaseConfigured,
    isNoDatabaseError,
    cn,
} from "@/components/ds"

// ─── Default Policy ───

const DEFAULT_POLICY: SessionPolicy = {
    session_lifetime_seconds: 86400,
    inactivity_timeout_seconds: 1800,
    max_concurrent_sessions: 5,
    notify_on_new_device: true,
    require_2fa_new_device: false,
}

// ─── Helper Functions ───

const getDeviceIcon = (type?: string) => {
    switch (type) {
        case "mobile": return Smartphone
        case "tablet": return Tablet
        case "desktop": return Laptop
        default: return Monitor
    }
}

const formatTimeAgo = (date: string) => {
    const seconds = Math.floor((Date.now() - new Date(date).getTime()) / 1000)
    if (seconds < 60) return "Hace menos de un minuto"
    if (seconds < 3600) return `Hace ${Math.floor(seconds / 60)} min`
    if (seconds < 86400) return `Hace ${Math.floor(seconds / 3600)}h`
    return `Hace ${Math.floor(seconds / 86400)}d`
}

const formatDuration = (seconds: number) => {
    if (seconds < 60) return `${seconds} segundos`
    if (seconds < 3600) return `${Math.floor(seconds / 60)} minutos`
    if (seconds < 86400) return `${Math.floor(seconds / 3600)} horas`
    return `${Math.floor(seconds / 86400)} días`
}

const getStatusStyles = (status?: string) => {
    switch (status) {
        case "active": return "bg-success/10 text-success border-success/20"
        case "idle": return "bg-warning/10 text-warning border-warning/20"
        case "expired": return "bg-muted/50 text-muted-foreground border-border"
        default: return "bg-muted/50 text-muted-foreground border-border"
    }
}

const getStatusLabel = (status?: string) => {
    switch (status) {
        case "active": return "Activa"
        case "idle": return "Inactiva"
        case "expired": return "Expirada"
        default: return "Desconocido"
    }
}

// ─── Stats Card Component ───

function StatsCard({
    icon: Icon,
    label,
    value,
    subValue,
    variant = "default",
    isLoading = false
}: Readonly<{
    icon: React.ElementType
    label: string
    value: string | number
    subValue?: string
    variant?: "info" | "success" | "warning" | "accent" | "default"
    isLoading?: boolean
}>) {
    const variantStyles = {
        default: "from-muted/30 to-muted/10 border-border/50",
        info: "from-info/15 to-info/5 border-info/30",
        success: "from-success/15 to-success/5 border-success/30",
        warning: "from-warning/15 to-warning/5 border-warning/30",
        accent: "from-accent/15 to-accent/5 border-accent/30",
    }
    const iconStyles = {
        default: "text-muted-foreground bg-muted/50",
        info: "text-info bg-info/10",
        success: "text-success bg-success/10",
        warning: "text-warning bg-warning/10",
        accent: "text-accent bg-accent/10",
    }

    return (
        <Card className={cn(
            "bg-gradient-to-br border transition-all duration-200",
            "hover:-translate-y-0.5 hover:shadow-float",
            variantStyles[variant]
        )}>
            <CardContent className="p-4">
                <div className="flex items-start justify-between">
                    <div className="space-y-1">
                        {isLoading ? (
                            <>
                                <Skeleton className="h-4 w-24" />
                                <Skeleton className="h-8 w-16 mt-1" />
                                <Skeleton className="h-3 w-20 mt-0.5" />
                            </>
                        ) : (
                            <>
                                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{label}</p>
                                <p className="mt-1 text-2xl font-semibold text-foreground">{value}</p>
                                {subValue && <p className="mt-0.5 text-xs text-muted-foreground">{subValue}</p>}
                            </>
                        )}
                    </div>
                    <div className={cn("p-2 rounded-lg", isLoading ? "bg-muted/30" : iconStyles[variant])}>
                        {isLoading ? (
                            <Skeleton className="h-5 w-5 rounded-full" />
                        ) : (
                            <Icon className="h-5 w-5" />
                        )}
                    </div>
                </div>
            </CardContent>
        </Card>
    )
}

// ─── Info Tooltip Component ───

function InfoTooltip({ content }: Readonly<{ content: string }>) {
    return (
        <TooltipProvider delayDuration={200}>
            <Tooltip>
                <TooltipTrigger asChild>
                    <button className="ml-1.5 inline-flex" type="button">
                        <HelpCircle className="h-3.5 w-3.5 text-muted-foreground/60 hover:text-muted-foreground transition-colors" />
                    </button>
                </TooltipTrigger>
                <TooltipContent side="top" className="max-w-xs">
                    {content}
                </TooltipContent>
            </Tooltip>
        </TooltipProvider>
    )
}

// ─── Session Detail Modal ───

function SessionDetailModal({
    session,
    open,
    onClose,
    onRevoke
}: Readonly<{
    session: SessionResponse | null
    open: boolean
    onClose: () => void
    onRevoke: (id: string) => void
}>) {
    if (!session) return null

    const DeviceIcon = getDeviceIcon(session.device_type)

    return (
        <Dialog open={open} onOpenChange={onClose}>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-3">
                        <div className="h-10 w-10 rounded-xl bg-gradient-to-br from-accent to-accent/70 flex items-center justify-center">
                            <DeviceIcon className="h-5 w-5 text-accent-foreground" />
                        </div>
                        <div>
                            <span>Detalle de Sesión</span>
                        </div>
                    </DialogTitle>
                    <DialogDescription>
                        Información completa de la sesión activa
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-6 py-4">
                    {/* User Info */}
                    <div className="space-y-3">
                        <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Usuario</h4>
                        <div className="flex items-center gap-3 p-3 rounded-lg bg-muted/30">
                            <div className="h-10 w-10 rounded-full bg-gradient-to-br from-info to-info/70 flex items-center justify-center text-background font-medium">
                                {session.user_email?.charAt(0).toUpperCase()}
                            </div>
                            <div>
                                <p className="font-medium text-foreground">{session.user_email}</p>
                                <p className="text-xs text-muted-foreground font-mono">{session.user_id}</p>
                            </div>
                        </div>
                    </div>

                    {/* Device Info */}
                    <div className="space-y-3">
                        <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Dispositivo</h4>
                        <div className="grid grid-cols-2 gap-3">
                            <div className="p-3 rounded-lg bg-muted/30">
                                <p className="text-xs text-muted-foreground">Navegador</p>
                                <p className="font-medium text-foreground">{session.browser || "Desconocido"}</p>
                            </div>
                            <div className="p-3 rounded-lg bg-muted/30">
                                <p className="text-xs text-muted-foreground">Sistema Operativo</p>
                                <p className="font-medium text-foreground">{session.os || "Desconocido"}</p>
                            </div>
                        </div>
                    </div>

                    {/* Location & Network */}
                    <div className="space-y-3">
                        <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Ubicación & Red</h4>
                        <div className="grid grid-cols-2 gap-3">
                            <div className="p-3 rounded-lg bg-muted/30">
                                <p className="text-xs text-muted-foreground">Ubicación</p>
                                <p className="font-medium text-foreground">
                                    {session.city || "Desconocida"}{session.country ? `, ${session.country}` : ""}
                                </p>
                            </div>
                            <div className="p-3 rounded-lg bg-muted/30">
                                <p className="text-xs text-muted-foreground">Dirección IP</p>
                                <p className="font-medium font-mono text-sm text-foreground">{session.ip_address}</p>
                            </div>
                        </div>
                    </div>

                    {/* Timestamps */}
                    <div className="space-y-3">
                        <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Tiempos</h4>
                        <div className="space-y-2">
                            <div className="flex items-center justify-between p-3 rounded-lg bg-muted/30">
                                <div className="flex items-center gap-2">
                                    <Clock className="h-4 w-4 text-muted-foreground" />
                                    <span className="text-sm text-foreground">Inicio de sesión</span>
                                </div>
                                <span className="text-sm font-medium text-foreground">{new Date(session.created_at).toLocaleString()}</span>
                            </div>
                            <div className="flex items-center justify-between p-3 rounded-lg bg-muted/30">
                                <div className="flex items-center gap-2">
                                    <Activity className="h-4 w-4 text-muted-foreground" />
                                    <span className="text-sm text-foreground">Última actividad</span>
                                </div>
                                <span className="text-sm font-medium text-foreground">{formatTimeAgo(session.last_activity || session.created_at)}</span>
                            </div>
                            <div className="flex items-center justify-between p-3 rounded-lg bg-muted/30">
                                <div className="flex items-center gap-2">
                                    <Timer className="h-4 w-4 text-muted-foreground" />
                                    <span className="text-sm text-foreground">Expira</span>
                                </div>
                                <span className="text-sm font-medium text-foreground">{session.expires_at ? new Date(session.expires_at).toLocaleString() : "-"}</span>
                            </div>
                        </div>
                    </div>

                    {/* Status */}
                    <div className="flex items-center justify-between p-3 rounded-lg border border-border">
                        <span className="text-sm font-medium text-foreground">Estado</span>
                        <Badge variant="outline" className={cn("capitalize", getStatusStyles(session.status))}>
                            {getStatusLabel(session.status)}
                        </Badge>
                    </div>
                </div>

                <DialogFooter className="gap-2 sm:gap-0">
                    <Button variant="outline" onClick={onClose}>
                        Cerrar
                    </Button>
                    <Button
                        variant="danger"
                        onClick={() => {
                            onRevoke(session.id)
                            onClose()
                        }}
                        className="gap-2"
                    >
                        <LogOut className="h-4 w-4" />
                        Revocar Sesión
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

// ─── Confirm Dialog Component ───

function ConfirmDialog({
    open,
    onClose,
    onConfirm,
    title,
    description,
    confirmLabel = "Confirmar",
    variant = "destructive",
    loading = false
}: Readonly<{
    open: boolean
    onClose: () => void
    onConfirm: () => void
    title: string
    description: string
    confirmLabel?: string
    variant?: "default" | "destructive"
    loading?: boolean
}>) {
    return (
        <Dialog open={open} onOpenChange={onClose}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2">
                        <AlertCircle className="h-5 w-5 text-warning" />
                        {title}
                    </DialogTitle>
                    <DialogDescription>{description}</DialogDescription>
                </DialogHeader>
                <DialogFooter className="gap-2 sm:gap-0">
                    <Button variant="outline" onClick={onClose} disabled={loading}>
                        Cancelar
                    </Button>
                    <Button variant={variant === "destructive" ? "danger" : variant} onClick={onConfirm} disabled={loading} className="gap-2">
                        {loading && <RefreshCw className="h-4 w-4 animate-spin" />}
                        {confirmLabel}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

// ─── Loading Skeleton ───

function SessionsSkeleton() {
    return (
        <div className="space-y-6 animate-in fade-in duration-500">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                    <Skeleton className="h-10 w-10 rounded-xl" />
                    <div className="space-y-2">
                        <Skeleton className="h-6 w-48" />
                        <Skeleton className="h-4 w-64" />
                    </div>
                </div>
                <div className="flex gap-2">
                    <Skeleton className="h-9 w-24" />
                    <Skeleton className="h-9 w-24" />
                </div>
            </div>

            {/* Tabs */}
            <Skeleton className="h-10 w-72 rounded-xl" />

            {/* Stats */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                {[1, 2, 3, 4].map((i) => (
                    <Skeleton key={`stat-${i}`} className="h-24 rounded-xl" />
                ))}
            </div>

            {/* Filters */}
            <Skeleton className="h-16 rounded-xl" />

            {/* Table */}
            <Skeleton className="h-96 rounded-xl" />
        </div>
    )
}

// ─── Main Component ───

function SessionsContent() {
    const params = useParams()
    const tenantId = params.tenant_id as string
    const router = useRouter()
    const { toast } = useToast()
    const queryClient = useQueryClient()

    // State
    const [activeTab, setActiveTab] = useState("sessions")
    const [searchQuery, setSearchQuery] = useState("")
    const [deviceFilter, setDeviceFilter] = useState<string>("all")
    const [statusFilter, setStatusFilter] = useState<string>("all")
    const [selectedSession, setSelectedSession] = useState<SessionResponse | null>(null)
    const [detailOpen, setDetailOpen] = useState(false)
    const [confirmRevoke, setConfirmRevoke] = useState<{ type: "single" | "user" | "all"; id?: string; email?: string } | null>(null)
    const [policyData, setPolicyData] = useState<SessionPolicy>(DEFAULT_POLICY)
    const [policyDirty, setPolicyDirty] = useState(false)

    // Fetch tenant
    const { data: tenant } = useQuery({
        queryKey: ["tenant", tenantId],
        enabled: !!tenantId,
        queryFn: () => api.get<Tenant>(`/v2/admin/tenants/${tenantId}`),
    })

    // Fetch sessions from API
    const { data: sessionsResponse, isLoading: sessionsLoading, error: sessionsError } = useQuery({
        queryKey: ["sessions", tenantId, deviceFilter, statusFilter, searchQuery],
        enabled: !!tenantId,
        queryFn: () => sessionsAdminAPI.list(tenantId!, {
            device_type: deviceFilter !== "all" ? deviceFilter : undefined,
            status: statusFilter !== "all" ? statusFilter as "active" | "expired" | "revoked" : undefined,
            search: searchQuery || undefined,
        }),
        retry: (failureCount, error) => {
            if ((error as any)?.error === "TENANT_NO_DATABASE" || (error as any)?.status === 424) return false
            return failureCount < 3
        },
    })

    // Fetch stats
    const { data: statsData } = useQuery({
        queryKey: ["sessions-stats", tenantId],
        enabled: !!tenantId,
        queryFn: () => sessionsAdminAPI.getStats(tenantId!),
    })

    const sessions: SessionResponse[] = sessionsResponse?.sessions || []

    // Filtered sessions
    const filteredSessions = useMemo(() => {
        return sessions.filter(session => {
            const matchesSearch = searchQuery === "" ||
                session.user_email?.toLowerCase().includes(searchQuery.toLowerCase()) ||
                session.ip_address?.includes(searchQuery) ||
                session.city?.toLowerCase().includes(searchQuery.toLowerCase()) ||
                session.country?.toLowerCase().includes(searchQuery.toLowerCase())

            const matchesDevice = deviceFilter === "all" || session.device_type === deviceFilter
            const matchesStatus = statusFilter === "all" || session.status === statusFilter

            return matchesSearch && matchesDevice && matchesStatus
        })
    }, [sessions, searchQuery, deviceFilter, statusFilter])

    // Stats
    const stats = useMemo(() => {
        if (statsData) {
            return {
                total: sessionsResponse?.total || 0,
                active: statsData.total_active,
                uniqueUsers: new Set(sessions.map(s => s.user_id)).size,
                desktopCount: statsData.by_device?.find(d => d.device_type === "desktop")?.count || 0,
                mobileCount: (statsData.by_device?.find(d => d.device_type === "mobile")?.count || 0) +
                    (statsData.by_device?.find(d => d.device_type === "tablet")?.count || 0),
            }
        }
        const active = sessions.filter(s => s.status === "active").length
        const uniqueUsers = new Set(sessions.map(s => s.user_id)).size
        const desktopCount = sessions.filter(s => s.device_type === "desktop").length
        const mobileCount = sessions.filter(s => s.device_type === "mobile" || s.device_type === "tablet").length

        return { total: sessions.length, active, uniqueUsers, desktopCount, mobileCount }
    }, [sessions, statsData, sessionsResponse?.total])

    // Mutations
    const revokeSessionMutation = useMutation({
        mutationFn: async (sessionId: string) => {
            return sessionsAdminAPI.revoke(tenantId!, sessionId, "Revocado por administrador")
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["sessions", tenantId] })
            queryClient.invalidateQueries({ queryKey: ["sessions-stats", tenantId] })
            toast({ title: "✓ Sesión revocada", description: "La sesión ha sido terminada correctamente.", variant: "default" })
        },
        onError: (e: Error) => toast({ title: "Error", description: e.message, variant: "destructive" }),
    })

    const revokeUserSessionsMutation = useMutation({
        mutationFn: async (userId: string) => {
            return sessionsAdminAPI.revokeByUser(tenantId!, userId, "Revocado por administrador")
        },
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["sessions", tenantId] })
            queryClient.invalidateQueries({ queryKey: ["sessions-stats", tenantId] })
            toast({ title: "✓ Sesiones revocadas", description: `${data.revoked_count} sesiones del usuario han sido terminadas.`, variant: "default" })
        },
        onError: (e: Error) => toast({ title: "Error", description: e.message, variant: "destructive" }),
    })

    const revokeAllSessionsMutation = useMutation({
        mutationFn: async () => {
            return sessionsAdminAPI.revokeAll(tenantId!, "Revocación masiva por administrador")
        },
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["sessions", tenantId] })
            queryClient.invalidateQueries({ queryKey: ["sessions-stats", tenantId] })
            toast({ title: "✓ Todas las sesiones revocadas", description: `${data.revoked_count} sesiones terminadas. Todos los usuarios deberán iniciar sesión nuevamente.`, variant: "default" })
        },
        onError: (e: Error) => toast({ title: "Error", description: e.message, variant: "destructive" }),
    })

    const updatePolicyMutation = useMutation({
        mutationFn: async (_policy: SessionPolicy) => {
            await new Promise(resolve => setTimeout(resolve, 500))
        },
        onSuccess: () => {
            setPolicyDirty(false)
            toast({ title: "✓ Políticas actualizadas", description: "Los cambios se aplicarán a nuevas sesiones.", variant: "default" })
        },
        onError: (e: Error) => toast({ title: "Error", description: e.message, variant: "destructive" }),
    })

    // Handlers
    const handleRevoke = () => {
        if (!confirmRevoke) return
        if (confirmRevoke.type === "single" && confirmRevoke.id) {
            revokeSessionMutation.mutate(confirmRevoke.id)
        } else if (confirmRevoke.type === "user" && confirmRevoke.id) {
            revokeUserSessionsMutation.mutate(confirmRevoke.id)
        } else if (confirmRevoke.type === "all") {
            revokeAllSessionsMutation.mutate()
        }
        setConfirmRevoke(null)
    }

    const handlePolicyChange = (key: keyof SessionPolicy, value: number | boolean) => {
        setPolicyData(prev => ({ ...prev, [key]: value }))
        setPolicyDirty(true)
    }

    const clearFilters = () => {
        setSearchQuery("")
        setDeviceFilter("all")
        setStatusFilter("all")
    }

    const hasActiveFilters = searchQuery !== "" || deviceFilter !== "all" || statusFilter !== "all"

    const hasNoDatabaseError = isNoDatabaseError(sessionsError)

    if (hasNoDatabaseError) {
        return (
            <div className="space-y-6 animate-in fade-in duration-500">
                {/* Header */}
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-4">
                        <Button variant="ghost" size="sm" asChild>
                            <Link href={`/admin/tenants/${tenantId}/detail`}>
                                <ArrowLeft className="h-4 w-4" />
                            </Link>
                        </Button>
                        <div className="flex items-center gap-3">
                            <div>
                                <h1 className="text-2xl font-bold tracking-tight">Gestión de Sesiones</h1>
                                <p className="text-sm text-muted-foreground">
                                    {tenant?.name} — Monitorea y administra las sesiones activas
                                </p>
                            </div>
                        </div>
                    </div>
                </div>

                {/* Info Banner */}
                <InlineAlert variant="info">
                    <div>
                        <p className="font-semibold">Sesiones Activas</p>
                        <p className="text-sm opacity-90">
                            Monitorea las sesiones activas de los usuarios, revoca accesos y configura políticas de sesión.
                        </p>
                    </div>
                </InlineAlert>

                <NoDatabaseConfigured
                    tenantId={tenantId}
                    message="Conecta una base de datos para comenzar a gestionar las sesiones de este tenant."
                />
            </div>
        )
    }

    return (
        <div className="space-y-6 animate-in fade-in duration-500">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                    <Button variant="ghost" size="sm" asChild>
                        <Link href={`/admin/tenants/${tenantId}/detail`}>
                            <ArrowLeft className="h-4 w-4" />
                        </Link>
                    </Button>
                    <div className="flex items-center gap-3">
                        <div>
                            <h1 className="text-2xl font-bold tracking-tight">Gestión de Sesiones</h1>
                            <p className="text-sm text-muted-foreground">
                                {tenant?.name} — Monitorea y administra las sesiones activas
                            </p>
                        </div>
                    </div>
                </div>
                <div className="flex items-center gap-3">
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={() => queryClient.invalidateQueries({ queryKey: ["sessions", tenantId] })}
                        className="gap-2"
                    >
                        <RefreshCw className="h-4 w-4" />
                        <span className="hidden sm:inline">Actualizar</span>
                    </Button>
                    <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                            <Button variant="danger" size="sm" className="gap-2 shadow-clay-button hover:shadow-clay-card transition-shadow">
                                <Ban className="h-4 w-4" />
                                <span className="hidden sm:inline">Acciones</span>
                            </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="w-56">
                            <DropdownMenuLabel>Acciones Masivas</DropdownMenuLabel>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                                className="text-danger focus:text-danger"
                                onClick={() => setConfirmRevoke({ type: "all" })}
                            >
                                <Trash2 className="mr-2 h-4 w-4" />
                                Revocar todas las sesiones
                            </DropdownMenuItem>
                        </DropdownMenuContent>
                    </DropdownMenu>
                </div>
            </div>
            {/* Info Banner */}
            <InlineAlert variant="info">
                <strong>Acerca de las sesiones:</strong> Las sesiones representan conexiones activas de usuarios autenticados. Revocar una sesión forzará al usuario a iniciar sesión nuevamente. La ubicación es aproximada y se basa en la dirección IP.
            </InlineAlert>

            <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-6">
                <TabsList>
                    <TabsTrigger value="sessions" className="gap-2">
                        <Users className="h-4 w-4" />
                        <span>Sesiones Activas</span>
                        <Badge variant="secondary" className="ml-1 h-5 px-1.5 text-[10px]">{stats.total}</Badge>
                    </TabsTrigger>
                    <TabsTrigger value="policies" className="gap-2">
                        <Settings2 className="h-4 w-4" />
                        <span>Políticas</span>
                    </TabsTrigger>
                </TabsList>

                {/* Sessions Tab */}
                <TabsContent value="sessions" className="space-y-6 mt-0">
                    {/* Stats Grid */}
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                        <StatsCard icon={Users} label="Total Sesiones" value={stats.total} subValue={`${stats.active} activas`} variant="info" isLoading={sessionsLoading} />
                        <StatsCard icon={CheckCircle2} label="Usuarios Únicos" value={stats.uniqueUsers} variant="success" isLoading={sessionsLoading} />
                        <StatsCard icon={Laptop} label="Desktop" value={stats.desktopCount} subValue={stats.total > 0 ? `${Math.round(stats.desktopCount / stats.total * 100)}%` : "0%"} variant="accent" isLoading={sessionsLoading} />
                        <StatsCard icon={Smartphone} label="Mobile" value={stats.mobileCount} subValue={stats.total > 0 ? `${Math.round(stats.mobileCount / stats.total * 100)}%` : "0%"} variant="default" isLoading={sessionsLoading} />
                    </div>

                    {/* Filters */}
                    <Card className="p-4">
                        <div className="flex flex-col md:flex-row gap-4">
                            <div className="flex-1 relative">
                                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                                <Input
                                    placeholder="Buscar por email, IP o ubicación..."
                                    value={searchQuery}
                                    onChange={(e) => setSearchQuery(e.target.value)}
                                    className="pl-10"
                                />
                            </div>
                            <div className="flex gap-3">
                                <Select value={deviceFilter} onValueChange={setDeviceFilter}>
                                    <SelectTrigger className="w-[140px]">
                                        <Monitor className="h-4 w-4 mr-2 text-muted-foreground" />
                                        <SelectValue placeholder="Dispositivo" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="all">Todos</SelectItem>
                                        <SelectItem value="desktop">Desktop</SelectItem>
                                        <SelectItem value="mobile">Mobile</SelectItem>
                                        <SelectItem value="tablet">Tablet</SelectItem>
                                    </SelectContent>
                                </Select>
                                <Select value={statusFilter} onValueChange={setStatusFilter}>
                                    <SelectTrigger className="w-[140px]">
                                        <Activity className="h-4 w-4 mr-2 text-muted-foreground" />
                                        <SelectValue placeholder="Estado" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="all">Todos</SelectItem>
                                        <SelectItem value="active">Activa</SelectItem>
                                        <SelectItem value="idle">Inactiva</SelectItem>
                                        <SelectItem value="expired">Expirada</SelectItem>
                                    </SelectContent>
                                </Select>
                                {hasActiveFilters && (
                                    <Button variant="ghost" className="h-8 w-8 p-0" onClick={clearFilters}>
                                        <X className="h-4 w-4" />
                                    </Button>
                                )}
                            </div>
                        </div>
                        {hasActiveFilters && (
                            <div className="mt-3 flex items-center gap-2 text-sm text-muted-foreground">
                                <Filter className="h-4 w-4" />
                                Mostrando {filteredSessions.length} de {sessions.length} sesiones
                            </div>
                        )}
                    </Card>

                    {/* Sessions Table */}
                    {sessionsLoading ? (
                        <Card>
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead>Usuario</TableHead>
                                        <TableHead>Dispositivo</TableHead>
                                        <TableHead>Ubicación</TableHead>
                                        <TableHead>Última Actividad</TableHead>
                                        <TableHead>Estado</TableHead>
                                        <TableHead className="text-right">Acciones</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {[1, 2, 3, 4, 5].map((i) => (
                                        <TableRow key={`skeleton-${i}`}>
                                            <TableCell>
                                                <div className="flex items-center gap-3">
                                                    <Skeleton className="h-8 w-8 rounded-full" />
                                                    <div className="space-y-1">
                                                        <Skeleton className="h-4 w-32" />
                                                        <Skeleton className="h-3 w-24" />
                                                    </div>
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                <div className="space-y-1">
                                                    <Skeleton className="h-4 w-20" />
                                                    <Skeleton className="h-3 w-16" />
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                <div className="space-y-1">
                                                    <Skeleton className="h-4 w-24" />
                                                    <Skeleton className="h-3 w-16" />
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                <Skeleton className="h-4 w-20" />
                                            </TableCell>
                                            <TableCell>
                                                <Skeleton className="h-6 w-16 rounded-full" />
                                            </TableCell>
                                            <TableCell className="text-right">
                                                <Skeleton className="h-8 w-8 rounded-md ml-auto" />
                                            </TableCell>
                                        </TableRow>
                                    ))}
                                </TableBody>
                            </Table>
                        </Card>
                    ) : filteredSessions.length === 0 ? (
                        <Card>
                            <EmptyState
                                icon={<Activity className="h-12 w-12" />}
                                title={hasActiveFilters ? "Sin resultados" : "No hay sesiones activas"}
                                description={hasActiveFilters ? "No se encontraron sesiones con los filtros aplicados" : "Las sesiones de usuario aparecerán aquí cuando inicien sesión"}
                            />
                        </Card>
                    ) : (
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>Usuario</TableHead>
                                    <TableHead>Dispositivo</TableHead>
                                    <TableHead>
                                        <div className="flex items-center">
                                            Ubicación
                                            <InfoTooltip content="Ubicación aproximada basada en la dirección IP" />
                                        </div>
                                    </TableHead>
                                    <TableHead>Última Actividad</TableHead>
                                    <TableHead>Estado</TableHead>
                                    <TableHead className="text-right">Acciones</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {filteredSessions.map((session) => {
                                    const DeviceIcon = getDeviceIcon(session.device_type)
                                    return (
                                        <TableRow key={session.id} className="group">
                                            <TableCell>
                                                <div className="flex items-center gap-3">
                                                    <div className="h-8 w-8 rounded-full bg-gradient-to-br from-info to-info/70 flex items-center justify-center text-background text-xs font-medium">
                                                        {session.user_email?.charAt(0).toUpperCase()}
                                                    </div>
                                                    <div>
                                                        <div className="flex items-center gap-2">
                                                            <span className="font-medium text-sm text-foreground">{session.user_email}</span>
                                                        </div>
                                                        <span className="text-xs text-muted-foreground font-mono">{session.ip_address}</span>
                                                    </div>
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                <div className="flex items-center gap-2">
                                                    <DeviceIcon className="h-4 w-4 text-muted-foreground" />
                                                    <div>
                                                        <p className="text-sm font-medium text-foreground">{session.browser}</p>
                                                        <p className="text-xs text-muted-foreground">{session.os}</p>
                                                    </div>
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                <div className="flex items-center gap-2">
                                                    <MapPin className="h-4 w-4 text-muted-foreground" />
                                                    <div>
                                                        <p className="text-sm text-foreground">{session.city || "Desconocida"}</p>
                                                        <p className="text-xs text-muted-foreground">{session.country || ""}</p>
                                                    </div>
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                <div className="flex items-center gap-2">
                                                    <Clock className="h-4 w-4 text-muted-foreground" />
                                                    <span className="text-sm text-foreground">{formatTimeAgo(session.last_activity || session.created_at)}</span>
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                <Badge variant="outline" className={cn("capitalize", getStatusStyles(session.status))}>
                                                    {getStatusLabel(session.status)}
                                                </Badge>
                                            </TableCell>
                                            <TableCell className="text-right">
                                                <DropdownMenu>
                                                    <DropdownMenuTrigger asChild>
                                                        <Button
                                                            variant="ghost"
                                                            className="h-8 w-8 p-0 opacity-0 group-hover:opacity-100 transition-opacity"
                                                        >
                                                            <MoreHorizontal className="h-4 w-4" />
                                                        </Button>
                                                    </DropdownMenuTrigger>
                                                    <DropdownMenuContent align="end">
                                                        <DropdownMenuItem
                                                            onClick={() => {
                                                                setSelectedSession(session)
                                                                setDetailOpen(true)
                                                            }}
                                                        >
                                                            <Eye className="mr-2 h-4 w-4" />
                                                            Ver detalles
                                                        </DropdownMenuItem>
                                                        <DropdownMenuSeparator />
                                                        <DropdownMenuItem
                                                            className="text-warning focus:text-warning"
                                                            onClick={() => setConfirmRevoke({ type: "user", id: session.user_id, email: session.user_email })}
                                                        >
                                                            <Users className="mr-2 h-4 w-4" />
                                                            Cerrar todas del usuario
                                                        </DropdownMenuItem>
                                                        <DropdownMenuItem
                                                            className="text-danger focus:text-danger"
                                                            onClick={() => setConfirmRevoke({ type: "single", id: session.id })}
                                                        >
                                                            <LogOut className="mr-2 h-4 w-4" />
                                                            Revocar sesión
                                                        </DropdownMenuItem>
                                                    </DropdownMenuContent>
                                                </DropdownMenu>
                                            </TableCell>
                                        </TableRow>
                                    )
                                })}
                            </TableBody>
                        </Table>
                    )}
                </TabsContent>

                {/* Policies Tab */}
                <TabsContent value="policies" className="space-y-6 mt-0">
                    <div className="grid lg:grid-cols-3 gap-6">
                        {/* Main Policy Form */}
                        <div className="lg:col-span-2 space-y-6">
                            {/* Session Duration */}
                            <Card>
                                <div className="px-6 py-4 border-b border-border bg-muted/30">
                                    <h3 className="font-medium flex items-center gap-2 text-foreground">
                                        <Timer className="h-4 w-4 text-muted-foreground" />
                                        Duración de Sesión
                                    </h3>
                                </div>
                                <CardContent className="p-6 space-y-6">
                                    <div className="space-y-3">
                                        <Label className="flex items-center">
                                            Tiempo máximo de sesión
                                            <InfoTooltip content="Tiempo máximo que una sesión puede permanecer activa antes de expirar automáticamente" />
                                        </Label>
                                        <div className="flex items-center gap-3">
                                            <Input
                                                type="number"
                                                value={Math.floor(policyData.session_lifetime_seconds / 3600)}
                                                onChange={(e) => handlePolicyChange("session_lifetime_seconds", parseInt(e.target.value) * 3600)}
                                                className="w-24"
                                                min={1}
                                                max={720}
                                            />
                                            <span className="text-sm text-muted-foreground">horas</span>
                                            <span className="text-xs text-muted-foreground">
                                                ({formatDuration(policyData.session_lifetime_seconds)})
                                            </span>
                                        </div>
                                    </div>

                                    <div className="space-y-3">
                                        <Label className="flex items-center">
                                            Timeout por inactividad
                                            <InfoTooltip content="Tiempo de inactividad después del cual la sesión se considera inactiva" />
                                        </Label>
                                        <div className="flex items-center gap-3">
                                            <Input
                                                type="number"
                                                value={Math.floor(policyData.inactivity_timeout_seconds / 60)}
                                                onChange={(e) => handlePolicyChange("inactivity_timeout_seconds", parseInt(e.target.value) * 60)}
                                                className="w-24"
                                                min={5}
                                                max={480}
                                            />
                                            <span className="text-sm text-muted-foreground">minutos</span>
                                        </div>
                                    </div>

                                    <div className="space-y-3">
                                        <Label className="flex items-center">
                                            Máximo de sesiones concurrentes
                                            <InfoTooltip content="Número máximo de sesiones activas permitidas por usuario. Las sesiones más antiguas se cerrarán automáticamente al exceder el límite." />
                                        </Label>
                                        <div className="flex items-center gap-3">
                                            <Input
                                                type="number"
                                                value={policyData.max_concurrent_sessions}
                                                onChange={(e) => handlePolicyChange("max_concurrent_sessions", parseInt(e.target.value))}
                                                className="w-24"
                                                min={1}
                                                max={20}
                                            />
                                            <span className="text-sm text-muted-foreground">por usuario</span>
                                        </div>
                                    </div>
                                </CardContent>
                            </Card>

                            {/* Security Settings */}
                            <Card>
                                <div className="px-6 py-4 border-b border-border bg-muted/30">
                                    <h3 className="font-medium flex items-center gap-2 text-foreground">
                                        <Shield className="h-4 w-4 text-muted-foreground" />
                                        Seguridad
                                    </h3>
                                </div>
                                <CardContent className="p-6 space-y-6">
                                    <div className="flex items-center justify-between">
                                        <div className="space-y-0.5">
                                            <Label className="flex items-center">
                                                Notificar en nuevo dispositivo
                                                <InfoTooltip content="Enviar un email al usuario cuando se inicia sesión desde un nuevo dispositivo" />
                                            </Label>
                                            <p className="text-sm text-muted-foreground">
                                                Enviar email de alerta al iniciar sesión desde un dispositivo no reconocido
                                            </p>
                                        </div>
                                        <Switch
                                            checked={policyData.notify_on_new_device}
                                            onCheckedChange={(checked) => handlePolicyChange("notify_on_new_device", checked)}
                                        />
                                    </div>

                                    <div className="border-t border-border pt-6">
                                        <div className="flex items-center justify-between">
                                            <div className="space-y-0.5">
                                                <Label className="flex items-center">
                                                    Requerir 2FA en nuevos dispositivos
                                                    <InfoTooltip content="Forzar verificación de dos factores cuando el usuario inicia sesión desde un dispositivo nuevo" />
                                                </Label>
                                                <p className="text-sm text-muted-foreground">
                                                    Solicitar verificación adicional para dispositivos no confiables
                                                </p>
                                            </div>
                                            <Switch
                                                checked={policyData.require_2fa_new_device}
                                                onCheckedChange={(checked) => handlePolicyChange("require_2fa_new_device", checked)}
                                            />
                                        </div>
                                    </div>
                                </CardContent>
                            </Card>

                            {/* Save Button */}
                            {policyDirty && (
                                <div className="flex justify-end">
                                    <Button
                                        onClick={() => updatePolicyMutation.mutate(policyData)}
                                        disabled={updatePolicyMutation.isPending}
                                        className="gap-2"
                                    >
                                        {updatePolicyMutation.isPending ? (
                                            <RefreshCw className="h-4 w-4 animate-spin" />
                                        ) : (
                                            <CheckCircle2 className="h-4 w-4" />
                                        )}
                                        Guardar Políticas
                                    </Button>
                                </div>
                            )}
                        </div>

                        {/* Info Sidebar */}
                        <div className="space-y-4">
                            <Card className="p-5">
                                <div className="flex items-start gap-3">
                                    <div className="h-8 w-8 rounded-lg bg-accent/20 flex items-center justify-center shrink-0">
                                        <Zap className="h-4 w-4 text-accent" />
                                    </div>
                                    <div>
                                        <h4 className="font-medium text-sm text-foreground">Configuración Recomendada</h4>
                                        <p className="text-xs text-muted-foreground mt-1">
                                            Para un balance entre seguridad y usabilidad, recomendamos:
                                        </p>
                                        <ul className="mt-3 space-y-2 text-xs text-muted-foreground">
                                            <li className="flex items-center gap-2">
                                                <ChevronRight className="h-3 w-3 text-accent" />
                                                Sesiones de 24 horas
                                            </li>
                                            <li className="flex items-center gap-2">
                                                <ChevronRight className="h-3 w-3 text-accent" />
                                                30 minutos de inactividad
                                            </li>
                                            <li className="flex items-center gap-2">
                                                <ChevronRight className="h-3 w-3 text-accent" />
                                                Máximo 5 sesiones concurrentes
                                            </li>
                                            <li className="flex items-center gap-2">
                                                <ChevronRight className="h-3 w-3 text-accent" />
                                                Notificaciones de nuevo dispositivo activas
                                            </li>
                                        </ul>
                                    </div>
                                </div>
                            </Card>

                            <InlineAlert variant="warning">
                                <strong>Importante:</strong> Los cambios en las políticas solo afectan a las nuevas sesiones. Las sesiones existentes mantendrán su configuración original hasta que expiren.
                            </InlineAlert>
                        </div>
                    </div>
                </TabsContent>
            </Tabs>

            {/* Modals */}
            <SessionDetailModal
                session={selectedSession}
                open={detailOpen}
                onClose={() => setDetailOpen(false)}
                onRevoke={(id) => setConfirmRevoke({ type: "single", id })}
            />

            <ConfirmDialog
                open={confirmRevoke !== null}
                onClose={() => setConfirmRevoke(null)}
                onConfirm={handleRevoke}
                title={
                    confirmRevoke?.type === "all" ? "Revocar todas las sesiones" :
                        confirmRevoke?.type === "user" ? "Revocar sesiones del usuario" :
                            "Revocar sesión"
                }
                description={
                    confirmRevoke?.type === "all"
                        ? "¿Estás seguro de que deseas cerrar todas las sesiones activas? Todos los usuarios deberán iniciar sesión nuevamente."
                        : confirmRevoke?.type === "user"
                            ? `¿Estás seguro de que deseas cerrar todas las sesiones de ${confirmRevoke.email}? El usuario deberá iniciar sesión nuevamente.`
                            : "¿Estás seguro de que deseas cerrar esta sesión? El usuario deberá iniciar sesión nuevamente."
                }
                confirmLabel={
                    confirmRevoke?.type === "all" ? "Revocar todas" :
                        confirmRevoke?.type === "user" ? "Revocar del usuario" :
                            "Revocar"
                }
                loading={revokeSessionMutation.isPending || revokeUserSessionsMutation.isPending || revokeAllSessionsMutation.isPending}
            />
        </div>
    )
}

// ─── Export ───

export default function SessionsClientPage() {
    return (
        <Suspense fallback={<SessionsSkeleton />}>
            <SessionsContent />
        </Suspense>
    )
}
