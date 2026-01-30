"use client"

import { useState, useMemo, Suspense } from "react"
import { useParams, useSearchParams } from "next/navigation"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
    Monitor, Smartphone, Tablet, Globe, Clock, Shield, Users,
    Search, RefreshCw, LogOut, Trash2, Settings2, MoreHorizontal,
    MapPin, Activity, AlertCircle, CheckCircle2, Info, ChevronRight,
    Laptop, HelpCircle, Ban, Eye, Filter, X, Zap, Timer
} from "lucide-react"
import { api, sessionsAdminAPI } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { useToast } from "@/hooks/use-toast"
import { cn } from "@/lib/utils"
import type { Tenant, Session, SessionPolicy, SessionResponse, ListSessionsResponse, SessionStats as SessionStatsType } from "@/lib/types"

// UI Components
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import {
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
} from "@/components/ui/tooltip"

// ─── Mock Data (Replace with real API when backend is ready) ───

const generateMockSessions = (count: number): Session[] => {
    const devices = [
        { type: "desktop" as const, browser: "Chrome", os: "Windows 11" },
        { type: "desktop" as const, browser: "Firefox", os: "macOS" },
        { type: "desktop" as const, browser: "Safari", os: "macOS" },
        { type: "mobile" as const, browser: "Safari", os: "iOS 17" },
        { type: "mobile" as const, browser: "Chrome", os: "Android 14" },
        { type: "tablet" as const, browser: "Safari", os: "iPadOS" },
    ]
    const locations = [
        { city: "Buenos Aires", country: "Argentina", country_code: "AR" },
        { city: "New York", country: "United States", country_code: "US" },
        { city: "London", country: "United Kingdom", country_code: "GB" },
        { city: "Madrid", country: "Spain", country_code: "ES" },
        { city: "Berlin", country: "Germany", country_code: "DE" },
    ]
    const emails = [
        "admin@example.com",
        "user1@example.com",
        "developer@company.com",
        "support@business.org",
        "test@demo.io"
    ]

    return Array.from({ length: count }, (_, i) => {
        const device = devices[Math.floor(Math.random() * devices.length)]
        const location = locations[Math.floor(Math.random() * locations.length)]
        const email = emails[Math.floor(Math.random() * emails.length)]
        const createdAt = new Date(Date.now() - Math.random() * 7 * 24 * 60 * 60 * 1000)
        const lastActivity = new Date(Date.now() - Math.random() * 60 * 60 * 1000)
        const isIdle = Math.random() > 0.7
        const isExpired = Math.random() > 0.9

        return {
            id: `session-${i + 1}-${Math.random().toString(36).substr(2, 9)}`,
            user_id: `user-${Math.floor(Math.random() * 5) + 1}`,
            user_email: email,
            client_id: "web-app",
            ip_address: `${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}`,
            user_agent: `Mozilla/5.0 (${device.os}) ${device.browser}/120.0`,
            device_type: device.type,
            browser: device.browser,
            os: device.os,
            location,
            created_at: createdAt.toISOString(),
            last_activity: lastActivity.toISOString(),
            expires_at: new Date(createdAt.getTime() + 24 * 60 * 60 * 1000).toISOString(),
            is_current: i === 0,
            status: isExpired ? "expired" : isIdle ? "idle" : "active",
        }
    })
}

const MOCK_SESSIONS = generateMockSessions(12)

const DEFAULT_POLICY: SessionPolicy = {
    session_lifetime_seconds: 86400, // 24 hours
    inactivity_timeout_seconds: 1800, // 30 minutes
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

const getStatusColor = (status?: string) => {
    switch (status) {
        case "active": return "bg-emerald-500/10 text-emerald-600 border-emerald-500/20"
        case "idle": return "bg-amber-500/10 text-amber-600 border-amber-500/20"
        case "expired": return "bg-zinc-500/10 text-zinc-500 border-zinc-500/20"
        default: return "bg-zinc-500/10 text-zinc-500 border-zinc-500/20"
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

function StatCard({
    icon: Icon,
    label,
    value,
    subValue,
    color = "zinc"
}: {
    icon: React.ElementType
    label: string
    value: string | number
    subValue?: string
    color?: "zinc" | "emerald" | "blue" | "violet"
}) {
    const colorClasses = {
        zinc: "from-zinc-500/10 to-zinc-500/5 border-zinc-500/10",
        emerald: "from-emerald-500/10 to-emerald-500/5 border-emerald-500/10",
        blue: "from-blue-500/10 to-blue-500/5 border-blue-500/10",
        violet: "from-violet-500/10 to-violet-500/5 border-violet-500/10",
    }
    const iconColors = {
        zinc: "text-zinc-500",
        emerald: "text-emerald-500",
        blue: "text-blue-500",
        violet: "text-violet-500",
    }

    return (
        <div className={cn(
            "rounded-xl border bg-gradient-to-br p-4 transition-all hover:shadow-md",
            colorClasses[color]
        )}>
            <div className="flex items-start justify-between">
                <div>
                    <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{label}</p>
                    <p className="mt-1 text-2xl font-semibold">{value}</p>
                    {subValue && <p className="mt-0.5 text-xs text-muted-foreground">{subValue}</p>}
                </div>
                <div className={cn("p-2 rounded-lg bg-background/50", iconColors[color])}>
                    <Icon className="h-5 w-5" />
                </div>
            </div>
        </div>
    )
}

// ─── Info Tooltip Component ───

function InfoTooltip({ content }: { content: string }) {
    return (
        <TooltipProvider delayDuration={200}>
            <Tooltip>
                <TooltipTrigger asChild>
                    <button className="ml-1.5 inline-flex">
                        <HelpCircle className="h-3.5 w-3.5 text-muted-foreground/60 hover:text-muted-foreground transition-colors" />
                    </button>
                </TooltipTrigger>
                <TooltipContent side="top" className="max-w-xs text-xs">
                    {content}
                </TooltipContent>
            </Tooltip>
        </TooltipProvider>
    )
}

// ─── Empty State Component ───

function EmptyState({ title, description, icon: Icon }: { title: string; description: string; icon: React.ElementType }) {
    return (
        <div className="flex flex-col items-center justify-center py-16 px-4 text-center">
            <div className="h-16 w-16 rounded-full bg-zinc-100 dark:bg-zinc-800 flex items-center justify-center mb-4">
                <Icon className="h-8 w-8 text-zinc-400" />
            </div>
            <h3 className="text-lg font-medium text-zinc-900 dark:text-zinc-100">{title}</h3>
            <p className="mt-1 text-sm text-muted-foreground max-w-sm">{description}</p>
        </div>
    )
}

// ─── Session Detail Modal ───

function SessionDetailModal({
    session,
    open,
    onClose,
    onRevoke
}: {
    session: SessionResponse | null
    open: boolean
    onClose: () => void
    onRevoke: (id: string) => void
}) {
    if (!session) return null

    const DeviceIcon = getDeviceIcon(session.device_type)

    return (
        <Dialog open={open} onOpenChange={onClose}>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-3">
                        <div className="h-10 w-10 rounded-xl bg-gradient-to-br from-violet-500 to-purple-600 flex items-center justify-center">
                            <DeviceIcon className="h-5 w-5 text-white" />
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
                        <div className="flex items-center gap-3 p-3 rounded-lg bg-zinc-50 dark:bg-zinc-800/50">
                            <div className="h-10 w-10 rounded-full bg-gradient-to-br from-blue-500 to-cyan-500 flex items-center justify-center text-white font-medium">
                                {session.user_email?.charAt(0).toUpperCase()}
                            </div>
                            <div>
                                <p className="font-medium">{session.user_email}</p>
                                <p className="text-xs text-muted-foreground font-mono">{session.user_id}</p>
                            </div>
                        </div>
                    </div>

                    {/* Device Info */}
                    <div className="space-y-3">
                        <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Dispositivo</h4>
                        <div className="grid grid-cols-2 gap-3">
                            <div className="p-3 rounded-lg bg-zinc-50 dark:bg-zinc-800/50">
                                <p className="text-xs text-muted-foreground">Navegador</p>
                                <p className="font-medium">{session.browser || "Desconocido"}</p>
                            </div>
                            <div className="p-3 rounded-lg bg-zinc-50 dark:bg-zinc-800/50">
                                <p className="text-xs text-muted-foreground">Sistema Operativo</p>
                                <p className="font-medium">{session.os || "Desconocido"}</p>
                            </div>
                        </div>
                    </div>

                    {/* Location & Network */}
                    <div className="space-y-3">
                        <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Ubicación & Red</h4>
                        <div className="grid grid-cols-2 gap-3">
                            <div className="p-3 rounded-lg bg-zinc-50 dark:bg-zinc-800/50">
                                <p className="text-xs text-muted-foreground">Ubicación</p>
                                <p className="font-medium">
                                    {session.city || "Desconocida"}{session.country ? `, ${session.country}` : ""}
                                </p>
                            </div>
                            <div className="p-3 rounded-lg bg-zinc-50 dark:bg-zinc-800/50">
                                <p className="text-xs text-muted-foreground">Dirección IP</p>
                                <p className="font-medium font-mono text-sm">{session.ip_address}</p>
                            </div>
                        </div>
                    </div>

                    {/* Timestamps */}
                    <div className="space-y-3">
                        <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Tiempos</h4>
                        <div className="space-y-2">
                            <div className="flex items-center justify-between p-3 rounded-lg bg-zinc-50 dark:bg-zinc-800/50">
                                <div className="flex items-center gap-2">
                                    <Clock className="h-4 w-4 text-muted-foreground" />
                                    <span className="text-sm">Inicio de sesión</span>
                                </div>
                                <span className="text-sm font-medium">{new Date(session.created_at).toLocaleString()}</span>
                            </div>
                            <div className="flex items-center justify-between p-3 rounded-lg bg-zinc-50 dark:bg-zinc-800/50">
                                <div className="flex items-center gap-2">
                                    <Activity className="h-4 w-4 text-muted-foreground" />
                                    <span className="text-sm">Última actividad</span>
                                </div>
                                <span className="text-sm font-medium">{formatTimeAgo(session.last_activity || session.created_at)}</span>
                            </div>
                            <div className="flex items-center justify-between p-3 rounded-lg bg-zinc-50 dark:bg-zinc-800/50">
                                <div className="flex items-center gap-2">
                                    <Timer className="h-4 w-4 text-muted-foreground" />
                                    <span className="text-sm">Expira</span>
                                </div>
                                <span className="text-sm font-medium">{session.expires_at ? new Date(session.expires_at).toLocaleString() : "-"}</span>
                            </div>
                        </div>
                    </div>

                    {/* Status */}
                    <div className="flex items-center justify-between p-3 rounded-lg border">
                        <span className="text-sm font-medium">Estado</span>
                        <Badge variant="outline" className={cn("capitalize", getStatusColor(session.status))}>
                            {getStatusLabel(session.status)}
                        </Badge>
                    </div>
                </div>

                <DialogFooter className="gap-2 sm:gap-0">
                    <Button variant="outline" onClick={onClose}>
                        Cerrar
                    </Button>
                    <Button
                        variant="destructive"
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
}: {
    open: boolean
    onClose: () => void
    onConfirm: () => void
    title: string
    description: string
    confirmLabel?: string
    variant?: "default" | "destructive"
    loading?: boolean
}) {
    return (
        <Dialog open={open} onOpenChange={onClose}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2">
                        <AlertCircle className="h-5 w-5 text-amber-500" />
                        {title}
                    </DialogTitle>
                    <DialogDescription>{description}</DialogDescription>
                </DialogHeader>
                <DialogFooter className="gap-2 sm:gap-0">
                    <Button variant="outline" onClick={onClose} disabled={loading}>
                        Cancelar
                    </Button>
                    <Button variant={variant} onClick={onConfirm} disabled={loading} className="gap-2">
                        {loading && <RefreshCw className="h-4 w-4 animate-spin" />}
                        {confirmLabel}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

// ─── Main Component ───

function SessionsContent() {
    const params = useParams()
    const searchParams = useSearchParams()
    const tenantId = (params.id as string) || (searchParams.get("id") as string)
    const { t } = useI18n()
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
    const { data: sessionsResponse, isLoading: sessionsLoading, refetch: refetchSessions } = useQuery({
        queryKey: ["sessions", tenantId, deviceFilter, statusFilter, searchQuery],
        enabled: !!tenantId,
        queryFn: () => sessionsAdminAPI.list(tenantId!, {
            device_type: deviceFilter !== "all" ? deviceFilter : undefined,
            status: statusFilter !== "all" ? statusFilter as "active" | "expired" | "revoked" : undefined,
            search: searchQuery || undefined,
        }),
    })
    
    // Fetch stats
    const { data: statsData } = useQuery({
        queryKey: ["sessions-stats", tenantId],
        enabled: !!tenantId,
        queryFn: () => sessionsAdminAPI.getStats(tenantId!),
    })
    
    const sessions: SessionResponse[] = sessionsResponse?.sessions || []

    // Filtered sessions - filtering is now done server-side, but keep client filter for search refinement
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

    // Stats - use API data if available, fallback to computed
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
            // TODO: Implement real API call when session policies endpoint is available
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

    return (
        <div className="min-h-screen bg-gradient-to-b from-zinc-50 to-white dark:from-zinc-950 dark:to-zinc-900">
            {/* Header */}
            <div className="border-b bg-white/80 dark:bg-zinc-900/80 backdrop-blur-sm sticky top-0 z-10">
                <div className="max-w-7xl mx-auto px-6 py-5">
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-4">
                            <div className="h-10 w-10 rounded-xl bg-gradient-to-br from-cyan-500 to-blue-600 flex items-center justify-center shadow-lg shadow-cyan-500/20">
                                <Activity className="h-5 w-5 text-white" />
                            </div>
                            <div>
                                <h1 className="text-xl font-semibold text-zinc-900 dark:text-white">Gestión de Sesiones</h1>
                                <p className="text-sm text-zinc-500">
                                    Monitorea y administra las sesiones activas de {tenant?.name || "tu organización"}
                                </p>
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
                                    <Button variant="destructive" size="sm" className="gap-2">
                                        <Ban className="h-4 w-4" />
                                        <span className="hidden sm:inline">Acciones</span>
                                    </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="end" className="w-56">
                                    <DropdownMenuLabel>Acciones Masivas</DropdownMenuLabel>
                                    <DropdownMenuSeparator />
                                    <DropdownMenuItem
                                        className="text-red-600 focus:text-red-600"
                                        onClick={() => setConfirmRevoke({ type: "all" })}
                                    >
                                        <Trash2 className="mr-2 h-4 w-4" />
                                        Revocar todas las sesiones
                                    </DropdownMenuItem>
                                </DropdownMenuContent>
                            </DropdownMenu>
                        </div>
                    </div>
                </div>
            </div>

            <div className="max-w-7xl mx-auto px-6 py-8">
                <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-6">
                    <TabsList className="bg-zinc-100 dark:bg-zinc-800/50 p-1 rounded-xl">
                        <TabsTrigger value="sessions" className="gap-2 rounded-lg data-[state=active]:bg-white data-[state=active]:shadow-sm px-5 py-2.5">
                            <Users className="h-4 w-4" />
                            <span>Sesiones Activas</span>
                            <Badge variant="secondary" className="ml-1 h-5 px-1.5 text-[10px]">{stats.total}</Badge>
                        </TabsTrigger>
                        <TabsTrigger value="policies" className="gap-2 rounded-lg data-[state=active]:bg-white data-[state=active]:shadow-sm px-5 py-2.5">
                            <Settings2 className="h-4 w-4" />
                            <span>Políticas</span>
                        </TabsTrigger>
                    </TabsList>

                    {/* Sessions Tab */}
                    <TabsContent value="sessions" className="space-y-6 mt-0">
                        {/* Stats Grid */}
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                            <StatCard icon={Users} label="Total Sesiones" value={stats.total} subValue={`${stats.active} activas`} color="blue" />
                            <StatCard icon={CheckCircle2} label="Usuarios Únicos" value={stats.uniqueUsers} color="emerald" />
                            <StatCard icon={Laptop} label="Desktop" value={stats.desktopCount} subValue={`${Math.round(stats.desktopCount / stats.total * 100)}%`} color="violet" />
                            <StatCard icon={Smartphone} label="Mobile" value={stats.mobileCount} subValue={`${Math.round(stats.mobileCount / stats.total * 100)}%`} color="zinc" />
                        </div>

                        {/* Filters */}
                        <div className="bg-white dark:bg-zinc-900 rounded-xl border shadow-sm p-4">
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
                                        <Button variant="ghost" size="icon" onClick={clearFilters}>
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
                        </div>

                        {/* Sessions Table */}
                        <div className="bg-white dark:bg-zinc-900 rounded-xl border shadow-sm overflow-hidden">
                            {sessionsLoading ? (
                                <div className="flex items-center justify-center py-16">
                                    <RefreshCw className="h-6 w-6 animate-spin text-muted-foreground" />
                                </div>
                            ) : filteredSessions.length === 0 ? (
                                <EmptyState
                                    icon={Activity}
                                    title={hasActiveFilters ? "Sin resultados" : "No hay sesiones activas"}
                                    description={hasActiveFilters ? "No se encontraron sesiones con los filtros aplicados" : "Las sesiones de usuario aparecerán aquí cuando inicien sesión"}
                                />
                            ) : (
                                <Table>
                                    <TableHeader>
                                        <TableRow className="bg-zinc-50 dark:bg-zinc-800/50 hover:bg-zinc-50 dark:hover:bg-zinc-800/50">
                                            <TableHead className="pl-6">Usuario</TableHead>
                                            <TableHead>Dispositivo</TableHead>
                                            <TableHead>
                                                <div className="flex items-center">
                                                    Ubicación
                                                    <InfoTooltip content="Ubicación aproximada basada en la dirección IP" />
                                                </div>
                                            </TableHead>
                                            <TableHead>Última Actividad</TableHead>
                                            <TableHead>Estado</TableHead>
                                            <TableHead className="text-right pr-6">Acciones</TableHead>
                                        </TableRow>
                                    </TableHeader>
                                    <TableBody>
                                        {filteredSessions.map((session) => {
                                            const DeviceIcon = getDeviceIcon(session.device_type)
                                            return (
                                                <TableRow key={session.id} className="group">
                                                    <TableCell className="pl-6">
                                                        <div className="flex items-center gap-3">
                                                            <div className="h-8 w-8 rounded-full bg-gradient-to-br from-blue-500 to-cyan-500 flex items-center justify-center text-white text-xs font-medium">
                                                                {session.user_email?.charAt(0).toUpperCase()}
                                                            </div>
                                                            <div>
                                                                <div className="flex items-center gap-2">
                                                                    <span className="font-medium text-sm">{session.user_email}</span>
                                                                </div>
                                                                <span className="text-xs text-muted-foreground font-mono">{session.ip_address}</span>
                                                            </div>
                                                        </div>
                                                    </TableCell>
                                                    <TableCell>
                                                        <div className="flex items-center gap-2">
                                                            <DeviceIcon className="h-4 w-4 text-muted-foreground" />
                                                            <div>
                                                                <p className="text-sm font-medium">{session.browser}</p>
                                                                <p className="text-xs text-muted-foreground">{session.os}</p>
                                                            </div>
                                                        </div>
                                                    </TableCell>
                                                    <TableCell>
                                                        <div className="flex items-center gap-2">
                                                            <MapPin className="h-4 w-4 text-muted-foreground" />
                                                            <div>
                                                                <p className="text-sm">{session.city || "Desconocida"}</p>
                                                                <p className="text-xs text-muted-foreground">{session.country || ""}</p>
                                                            </div>
                                                        </div>
                                                    </TableCell>
                                                    <TableCell>
                                                        <div className="flex items-center gap-2">
                                                            <Clock className="h-4 w-4 text-muted-foreground" />
                                                            <span className="text-sm">{formatTimeAgo(session.last_activity || session.created_at)}</span>
                                                        </div>
                                                    </TableCell>
                                                    <TableCell>
                                                        <Badge variant="outline" className={cn("capitalize", getStatusColor(session.status))}>
                                                            {getStatusLabel(session.status)}
                                                        </Badge>
                                                    </TableCell>
                                                    <TableCell className="text-right pr-6">
                                                        <DropdownMenu>
                                                            <DropdownMenuTrigger asChild>
                                                                <Button
                                                                    variant="ghost"
                                                                    size="icon"
                                                                    className="h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity"
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
                                                                    className="text-amber-600 focus:text-amber-600"
                                                                    onClick={() => setConfirmRevoke({ type: "user", id: session.user_id, email: session.user_email })}
                                                                >
                                                                    <Users className="mr-2 h-4 w-4" />
                                                                    Cerrar todas del usuario
                                                                </DropdownMenuItem>
                                                                <DropdownMenuItem
                                                                    className="text-red-600 focus:text-red-600"
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
                        </div>

                        {/* Info Banner */}
                        <div className="flex items-start gap-3 p-4 rounded-xl bg-blue-50 dark:bg-blue-950/20 border border-blue-200 dark:border-blue-900/30">
                            <Info className="h-5 w-5 text-blue-600 dark:text-blue-400 mt-0.5 shrink-0" />
                            <div>
                                <p className="text-sm font-medium text-blue-900 dark:text-blue-100">
                                    Acerca de las sesiones
                                </p>
                                <p className="text-sm text-blue-700 dark:text-blue-300 mt-1">
                                    Las sesiones representan conexiones activas de usuarios autenticados. Revocar una sesión forzará al usuario a iniciar sesión nuevamente. La ubicación es aproximada y se basa en la dirección IP.
                                </p>
                            </div>
                        </div>
                    </TabsContent>

                    {/* Policies Tab */}
                    <TabsContent value="policies" className="space-y-6 mt-0">
                        <div className="grid lg:grid-cols-3 gap-6">
                            {/* Main Policy Form */}
                            <div className="lg:col-span-2 space-y-6">
                                {/* Session Duration */}
                                <div className="bg-white dark:bg-zinc-900 rounded-xl border shadow-sm overflow-hidden">
                                    <div className="px-6 py-4 border-b bg-zinc-50 dark:bg-zinc-800/50">
                                        <h3 className="font-medium flex items-center gap-2">
                                            <Timer className="h-4 w-4 text-muted-foreground" />
                                            Duración de Sesión
                                        </h3>
                                    </div>
                                    <div className="p-6 space-y-6">
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
                                    </div>
                                </div>

                                {/* Security Settings */}
                                <div className="bg-white dark:bg-zinc-900 rounded-xl border shadow-sm overflow-hidden">
                                    <div className="px-6 py-4 border-b bg-zinc-50 dark:bg-zinc-800/50">
                                        <h3 className="font-medium flex items-center gap-2">
                                            <Shield className="h-4 w-4 text-muted-foreground" />
                                            Seguridad
                                        </h3>
                                    </div>
                                    <div className="p-6 space-y-6">
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

                                        <div className="border-t pt-6">
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
                                    </div>
                                </div>

                                {/* Save Button */}
                                {policyDirty && (
                                    <div className="flex justify-end">
                                        <Button
                                            onClick={() => updatePolicyMutation.mutate(policyData)}
                                            disabled={updatePolicyMutation.isPending}
                                            className="gap-2 bg-zinc-900 hover:bg-zinc-800"
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
                                <div className="bg-white dark:bg-zinc-900 rounded-xl border shadow-sm p-5">
                                    <div className="flex items-start gap-3">
                                        <div className="h-8 w-8 rounded-lg bg-violet-100 dark:bg-violet-900/30 flex items-center justify-center shrink-0">
                                            <Zap className="h-4 w-4 text-violet-600 dark:text-violet-400" />
                                        </div>
                                        <div>
                                            <h4 className="font-medium text-sm">Configuración Recomendada</h4>
                                            <p className="text-xs text-muted-foreground mt-1">
                                                Para un balance entre seguridad y usabilidad, recomendamos:
                                            </p>
                                            <ul className="mt-3 space-y-2 text-xs text-muted-foreground">
                                                <li className="flex items-center gap-2">
                                                    <ChevronRight className="h-3 w-3 text-violet-500" />
                                                    Sesiones de 24 horas
                                                </li>
                                                <li className="flex items-center gap-2">
                                                    <ChevronRight className="h-3 w-3 text-violet-500" />
                                                    30 minutos de inactividad
                                                </li>
                                                <li className="flex items-center gap-2">
                                                    <ChevronRight className="h-3 w-3 text-violet-500" />
                                                    Máximo 5 sesiones concurrentes
                                                </li>
                                                <li className="flex items-center gap-2">
                                                    <ChevronRight className="h-3 w-3 text-violet-500" />
                                                    Notificaciones de nuevo dispositivo activas
                                                </li>
                                            </ul>
                                        </div>
                                    </div>
                                </div>

                                <div className="bg-amber-50 dark:bg-amber-950/20 rounded-xl border border-amber-200 dark:border-amber-900/30 p-5">
                                    <div className="flex items-start gap-3">
                                        <AlertCircle className="h-5 w-5 text-amber-600 dark:text-amber-400 shrink-0" />
                                        <div>
                                            <h4 className="font-medium text-sm text-amber-900 dark:text-amber-100">Importante</h4>
                                            <p className="text-xs text-amber-700 dark:text-amber-300 mt-1">
                                                Los cambios en las políticas solo afectan a las nuevas sesiones. Las sesiones existentes mantendrán su configuración original hasta que expiren.
                                            </p>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </TabsContent>
                </Tabs>
            </div>

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
        <Suspense fallback={
            <div className="flex items-center justify-center h-[60vh]">
                <div className="text-center">
                    <div className="h-10 w-10 mx-auto mb-4 rounded-full border-2 border-zinc-200 border-t-zinc-800 animate-spin" />
                    <p className="text-sm text-zinc-500">Cargando sesiones...</p>
                </div>
            </div>
        }>
            <SessionsContent />
        </Suspense>
    )
}
