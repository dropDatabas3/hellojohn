"use client"

import { useState, useEffect } from "react"
import { useParams } from "next/navigation"
import Link from "next/link"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useToast } from "@/hooks/use-toast"
import { api } from "@/lib/api"
import { useAuthStore } from "@/lib/auth-store"
import { useI18n } from "@/lib/i18n"
import {
    AlertCircle,
    CheckCircle2,
    Database,
    HardDrive,
    Play,
    Settings2,
    Zap,
    Eye,
    EyeOff,
    RefreshCw,
    Loader2,
    Shield,
    Activity,
    Server,
    Cpu,
    Layers,
    ArrowRight,
    ArrowLeft,
    Sparkles,
    XCircle,
    Power,
    PowerOff,
} from "lucide-react"

import {
    PostgresIcon,
    MySQLIcon,
    MongoDBIcon,
    RedisIcon,
    getDriverIcon,
    getCacheDriverIcon,
} from "@/components/icons/database-icons"

import {
    Button,
    Input,
    Label,
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
    Switch,
    Card,
    CardContent,
    Badge,
    InlineAlert,
    Skeleton,
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    cn,
} from "@/components/ds"

// ============================================================================
// TYPES
// ============================================================================

interface UserDBSettings {
    driver: string
    dsn: string
    schema: string
    dsnEnc?: string
}

interface CacheSettings {
    enabled: boolean
    driver: string
    host: string
    port: number
    password?: string
    db: number
    prefix: string
    passEnc?: string
}

interface TenantSettings {
    userDb?: UserDBSettings
    cache?: CacheSettings
}

// ============================================================================
// STATUS INDICATOR
// ============================================================================

function StatusDot({ status }: { status: "connected" | "configured" | "disconnected" | "error" | "loading" }) {
    const styles = {
        connected: "bg-success shadow-[0_0_8px_rgba(34,197,94,0.5)]",
        configured: "bg-warning shadow-[0_0_8px_rgba(234,179,8,0.5)]",
        disconnected: "bg-muted-foreground/30",
        error: "bg-destructive shadow-[0_0_8px_rgba(239,68,68,0.5)]",
        loading: "bg-accent animate-pulse shadow-[0_0_8px_rgba(139,92,246,0.5)]",
    }
    return <div className={cn("h-2.5 w-2.5 rounded-full transition-all duration-500", styles[status])} />
}

// ============================================================================
// INFO ROW
// ============================================================================

function InfoRow({ label, value, icon: Icon, iconColor }: {
    label: string
    value: string
    icon?: React.ElementType
    iconColor?: string
}) {
    return (
        <div className="flex items-center justify-between py-2.5 px-3 rounded-lg hover:bg-muted/30 transition-colors">
            <span className="text-xs text-muted-foreground uppercase tracking-wider flex items-center gap-2">
                {Icon && <Icon className={cn("h-3.5 w-3.5", iconColor || "text-muted-foreground")} />}
                {label}
            </span>
            <span className="text-sm font-medium text-foreground">{value}</span>
        </div>
    )
}

// ============================================================================
// DATABASE CONFIG MODAL
// ============================================================================

function DatabaseConfigModal({
    open,
    onClose,
    dbForm,
    setDbForm,
    onSave,
    isPending,
}: {
    open: boolean
    onClose: () => void
    dbForm: UserDBSettings
    setDbForm: (form: UserDBSettings) => void
    onSave: () => void
    isPending: boolean
}) {
    const [showDsn, setShowDsn] = useState(false)

    return (
        <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-3">
                        <div className="p-2 rounded-lg bg-accent/10">
                            <Database className="h-5 w-5 text-accent" />
                        </div>
                        Configurar Base de Datos
                    </DialogTitle>
                    <DialogDescription>
                        Conecta una base de datos para almacenar usuarios, sesiones y datos de autenticación.
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-5 py-2">
                    <div className="grid gap-5 sm:grid-cols-2">
                        <div className="space-y-2">
                            <Label>Motor de Base de Datos</Label>
                            <Select value={dbForm.driver} onValueChange={(val) => setDbForm({ ...dbForm, driver: val })}>
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="postgres">
                                        <div className="flex items-center gap-2">
                                            <PostgresIcon className="h-4 w-4 text-[#336791]" />
                                            PostgreSQL
                                        </div>
                                    </SelectItem>
                                    <SelectItem value="mysql">
                                        <div className="flex items-center gap-2">
                                            <MySQLIcon className="h-4 w-4 text-[#4479A1]" />
                                            MySQL
                                        </div>
                                    </SelectItem>
                                    <SelectItem value="mongo">
                                        <div className="flex items-center gap-2">
                                            <MongoDBIcon className="h-4 w-4 text-[#47A248]" />
                                            MongoDB
                                        </div>
                                    </SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="space-y-2">
                            <Label>Nombre del Schema</Label>
                            <Input
                                placeholder="public"
                                value={dbForm.schema}
                                onChange={(e) => setDbForm({ ...dbForm, schema: e.target.value })}
                            />
                        </div>
                    </div>

                    <div className="space-y-2">
                        <Label>Connection String (DSN)</Label>
                        <div className="relative">
                            <Input
                                type={showDsn ? "text" : "password"}
                                placeholder="postgresql://user:password@host:5432/database"
                                value={dbForm.dsn}
                                onChange={(e) => setDbForm({ ...dbForm, dsn: e.target.value })}
                                className="pr-12 font-mono text-sm"
                            />
                            <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                className="absolute right-1 top-1/2 -translate-y-1/2 h-8 w-8"
                                onClick={() => setShowDsn(!showDsn)}
                            >
                                {showDsn ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                            </Button>
                        </div>
                        <p className="text-xs text-muted-foreground flex items-center gap-1.5">
                            <Shield className="h-3.5 w-3.5" />
                            Tus credenciales se encriptan antes de almacenarse
                        </p>
                    </div>
                </div>

                <DialogFooter>
                    <Button variant="outline" onClick={onClose} disabled={isPending}>
                        Cancelar
                    </Button>
                    <Button onClick={onSave} disabled={isPending || !dbForm.dsn} className="shadow-clay-button">
                        {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        Guardar
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

// ============================================================================
// CACHE CONFIG MODAL
// ============================================================================

function CacheConfigModal({
    open,
    onClose,
    cacheForm,
    setCacheForm,
    onSave,
    isPending,
}: {
    open: boolean
    onClose: () => void
    cacheForm: CacheSettings
    setCacheForm: (form: CacheSettings) => void
    onSave: () => void
    isPending: boolean
}) {
    return (
        <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-3">
                        <div className="p-2 rounded-lg bg-accent/10">
                            <Server className="h-5 w-5 text-accent" />
                        </div>
                        Configurar Capa de Cache
                    </DialogTitle>
                    <DialogDescription>
                        Configura una capa de cache para acelerar los tiempos de respuesta.
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-5 py-2">
                    <div className="space-y-2">
                        <Label>Driver de Cache</Label>
                        <Select value={cacheForm.driver} onValueChange={(val) => setCacheForm({ ...cacheForm, driver: val })}>
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="memory">
                                    <div className="flex items-center gap-2">
                                        <Cpu className="h-4 w-4 text-warning" />
                                        Memory (Local)
                                    </div>
                                </SelectItem>
                                <SelectItem value="redis">
                                    <div className="flex items-center gap-2">
                                        <RedisIcon className="h-4 w-4" />
                                        Redis
                                    </div>
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    {cacheForm.driver === "redis" && (
                        <>
                            <div className="grid gap-5 sm:grid-cols-2">
                                <div className="space-y-2">
                                    <Label>Host</Label>
                                    <Input
                                        placeholder="localhost"
                                        value={cacheForm.host}
                                        onChange={(e) => setCacheForm({ ...cacheForm, host: e.target.value })}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <Label>Puerto</Label>
                                    <Input
                                        type="number"
                                        placeholder="6379"
                                        value={cacheForm.port}
                                        onChange={(e) => setCacheForm({ ...cacheForm, port: parseInt(e.target.value) || 6379 })}
                                    />
                                </div>
                            </div>
                            <div className="space-y-2">
                                <Label>Contrasena (Opcional)</Label>
                                <Input
                                    type="password"
                                    placeholder="*********"
                                    value={cacheForm.password}
                                    onChange={(e) => setCacheForm({ ...cacheForm, password: e.target.value })}
                                />
                            </div>
                            <div className="grid gap-5 sm:grid-cols-2">
                                <div className="space-y-2">
                                    <Label>Indice de Base de Datos</Label>
                                    <Input
                                        type="number"
                                        placeholder="0"
                                        value={cacheForm.db}
                                        onChange={(e) => setCacheForm({ ...cacheForm, db: parseInt(e.target.value) || 0 })}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <Label>Prefijo de Keys</Label>
                                    <Input
                                        placeholder="tenant:"
                                        value={cacheForm.prefix}
                                        onChange={(e) => setCacheForm({ ...cacheForm, prefix: e.target.value })}
                                    />
                                </div>
                            </div>
                        </>
                    )}
                </div>

                <DialogFooter>
                    <Button variant="outline" onClick={onClose} disabled={isPending}>
                        Cancelar
                    </Button>
                    <Button onClick={onSave} disabled={isPending} className="shadow-clay-button">
                        {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        Guardar
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

// ============================================================================
// MAIN COMPONENT
// ============================================================================

export default function DatabasePage() {
    const params = useParams()
    const tenantId = params.tenant_id as string
    const { token } = useAuthStore()
    const { toast } = useToast()
    const queryClient = useQueryClient()
    const { t } = useI18n()

    // State
    const [isDbModalOpen, setIsDbModalOpen] = useState(false)
    const [isCacheModalOpen, setIsCacheModalOpen] = useState(false)
    const [etag, setEtag] = useState<string>("")
    const [testingDb, setTestingDb] = useState(false)
    const [testingCache, setTestingCache] = useState(false)

    const [dbForm, setDbForm] = useState<UserDBSettings>({
        driver: "postgres",
        dsn: "",
        schema: "",
    })

    const [cacheForm, setCacheForm] = useState<CacheSettings>({
        enabled: true,
        driver: "memory",
        host: "",
        port: 6379,
        password: "",
        db: 0,
        prefix: "",
    })

    // ========================================================================
    // QUERIES
    // ========================================================================

    const {
        data: settings,
        isLoading,
        isError,
        error,
        refetch: refetchSettings,
    } = useQuery<TenantSettings>({
        queryKey: ["tenant-storage", tenantId],
        queryFn: async () => {
            if (!tenantId || !token) throw new Error("No tenant ID or token")
            const { data, headers } = await api.getWithHeaders<TenantSettings>(`/v2/admin/tenants/${tenantId}/settings`)
            const etagHeader = headers.get("ETag")
            if (etagHeader) setEtag(etagHeader)
            return data
        },
        enabled: !!tenantId && !!token,
    })

    const { data: infraStats, refetch: refetchStats } = useQuery<any>({
        queryKey: ["tenant-infra-stats", tenantId],
        queryFn: async () => {
            if (!tenantId || !token) return null
            try {
                const { data } = await api.get<any>(`/v2/admin/tenants/${tenantId}/infra-stats`)
                return data || null
            } catch (e) {
                return null
            }
        },
        enabled: !!tenantId && !!token,
        refetchInterval: 30000,
    })

    // ========================================================================
    // EFFECTS
    // ========================================================================

    useEffect(() => {
        if (settings) {
            if (settings.userDb) {
                setDbForm({
                    driver: settings.userDb.driver || "postgres",
                    dsn: "",
                    schema: settings.userDb.schema || "",
                })
            }
            if (settings.cache) {
                setCacheForm({
                    enabled: settings.cache.enabled || false,
                    driver: settings.cache.driver || "memory",
                    host: settings.cache.host || "",
                    port: settings.cache.port || 6379,
                    password: "",
                    db: settings.cache.db || 0,
                    prefix: settings.cache.prefix || "",
                })
            }
        }
    }, [settings])

    // ========================================================================
    // MUTATIONS
    // ========================================================================

    const updateSettingsMutation = useMutation({
        mutationFn: async (data: TenantSettings) => {
            if (!tenantId || !token) throw new Error("No tenant ID")
            const payload = { ...settings, ...data }
            await api.put(`/v2/admin/tenants/${tenantId}/settings`, payload, etag)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["tenant-storage", tenantId] })
            toast({ title: t("common.success"), description: t("tenants.settingsUpdatedDesc"), variant: "info" })
            setIsDbModalOpen(false)
            setIsCacheModalOpen(false)
        },
        onError: (err: any) => {
            toast({
                variant: "destructive",
                title: t("common.error"),
                description: err.response?.data?.error_description || err.message,
            })
        },
    })

    const testDbMutation = useMutation({
        mutationFn: async () => {
            if (!tenantId || !token) throw new Error("No tenant ID")
            setTestingDb(true)
            await api.post(`/v2/admin/tenants/${tenantId}/user-store/test-connection`, {})
        },
        onSuccess: () => {
            toast({ variant: "success", title: t("database.connectionSuccess"), description: t("database.testConnectionDesc") })
            setTestingDb(false)
            refetchStats()
        },
        onError: (err: any) => {
            toast({ variant: "destructive", title: t("common.error"), description: err.response?.data?.error_description || err.message })
            setTestingDb(false)
        },
    })

    const runMigrationsMutation = useMutation({
        mutationFn: async () => {
            if (!tenantId || !token) throw new Error("No tenant ID")
            return api.post<{ applied_count: number }>(`/v2/admin/tenants/${tenantId}/user-store/migrate`, {})
        },
        onSuccess: (data) => {
            toast({ title: t("database.migrationsApplied"), description: t("database.migrationsAppliedDesc", { count: data.applied_count || 0 }), variant: "success" })
        },
        onError: (err: any) => {
            toast({ variant: "destructive", title: t("common.error"), description: err.response?.data?.error_description || err.message })
        },
    })

    const testCacheMutation = useMutation({
        mutationFn: async () => {
            if (!tenantId || !token) throw new Error("No tenant ID")
            setTestingCache(true)
            await api.post(`/v2/admin/tenants/${tenantId}/cache/test-connection`, {})
        },
        onSuccess: () => {
            toast({ title: t("database.connectionSuccess"), description: "Cache connection verified.", variant: "success" })
            setTestingCache(false)
            refetchStats()
        },
        onError: (err: any) => {
            toast({ variant: "destructive", title: t("common.error"), description: err.response?.data?.error_description || err.message })
            setTestingCache(false)
        },
    })

    // ========================================================================
    // HANDLERS
    // ========================================================================

    const handleSaveDB = () => {
        updateSettingsMutation.mutate({ userDb: dbForm })
    }

    const handleSaveCache = () => {
        const cleanCacheForm = { ...cacheForm }
        if (cleanCacheForm.driver === "memory") {
            cleanCacheForm.host = ""
            cleanCacheForm.port = 0
            cleanCacheForm.password = ""
            cleanCacheForm.db = 0
            cleanCacheForm.prefix = ""
        }
        updateSettingsMutation.mutate({ cache: cleanCacheForm })
    }

    const handleToggleCache = (enabled: boolean) => {
        const updated = { ...cacheForm, enabled }
        const cleanForm = { ...updated }
        if (cleanForm.driver === "memory") {
            cleanForm.host = ""
            cleanForm.port = 0
            cleanForm.password = ""
            cleanForm.db = 0
            cleanForm.prefix = ""
        }
        updateSettingsMutation.mutate({ cache: cleanForm })
    }

    const openDbModal = () => {
        if (settings?.userDb) {
            setDbForm({
                driver: settings.userDb.driver || "postgres",
                dsn: "",
                schema: settings.userDb.schema || "",
            })
        } else {
            setDbForm({ driver: "postgres", dsn: "", schema: "" })
        }
        setIsDbModalOpen(true)
    }

    const openCacheModal = () => {
        if (settings?.cache) {
            setCacheForm({
                enabled: settings.cache.enabled ?? true,
                driver: settings.cache.driver || "memory",
                host: settings.cache.host || "",
                port: settings.cache.port || 6379,
                password: "",
                db: settings.cache.db || 0,
                prefix: settings.cache.prefix || "",
            })
        } else {
            setCacheForm({ enabled: true, driver: "memory", host: "", port: 6379, password: "", db: 0, prefix: "" })
        }
        setIsCacheModalOpen(true)
    }

    const handleRefresh = async () => {
        await Promise.all([refetchSettings(), refetchStats()])
        toast({ title: "Actualizado", description: "Datos actualizados correctamente", variant: "info" })
    }

    // ========================================================================
    // COMPUTED
    // ========================================================================

    const isDBConfigured = !!settings?.userDb?.dsnEnc || !!settings?.userDb?.dsn
    const isCacheConfigured = !!settings?.cache
    const isCacheEnabled = settings?.cache?.enabled || false

    const dbStatus = testingDb
        ? "loading" as const
        : isDBConfigured
            ? (infraStats?.db === "ok" ? "connected" as const : "configured" as const)
            : "disconnected" as const

    const cacheStatus = testingCache
        ? "loading" as const
        : isCacheEnabled
            ? (infraStats?.cache === "ok" ? "connected" as const : "configured" as const)
            : isCacheConfigured
                ? "disconnected" as const
                : "disconnected" as const

    const driverNames: Record<string, string> = {
        postgres: "PostgreSQL",
        mysql: "MySQL",
        mongo: "MongoDB",
    }

    const cacheDriverNames: Record<string, string> = {
        memory: "Memory (Local)",
        redis: "Redis",
    }

    // ========================================================================
    // ERROR STATE
    // ========================================================================

    if (isError) {
        return (
            <div className="space-y-6 animate-in fade-in duration-500">
                <InlineAlert
                    variant="danger"
                    title={t("common.error")}
                    description={(error as any)?.message || "Failed to load settings"}
                />
            </div>
        )
    }

    // ========================================================================
    // LOADING STATE
    // ========================================================================

    if (isLoading) {
        return (
            <div className="space-y-6 animate-in fade-in duration-500">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-4">
                        <Skeleton className="h-9 w-9 rounded-md" />
                        <div className="space-y-2">
                            <Skeleton className="h-7 w-48" />
                            <Skeleton className="h-4 w-72" />
                        </div>
                    </div>
                    <Skeleton className="h-10 w-32" />
                </div>
                <div className="grid md:grid-cols-2 gap-6">
                    {[1, 2].map((i) => (
                        <Card key={i} className="overflow-hidden">
                            <div className="p-6 space-y-4">
                                <div className="flex items-center gap-3">
                                    <Skeleton className="h-14 w-14 rounded-2xl" />
                                    <div className="space-y-2 flex-1">
                                        <Skeleton className="h-5 w-40" />
                                        <Skeleton className="h-4 w-56" />
                                    </div>
                                </div>
                                <Skeleton className="h-32 rounded-xl" />
                            </div>
                        </Card>
                    ))}
                </div>
            </div>
        )
    }

    // ========================================================================
    // RENDER
    // ========================================================================

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
                    <div>
                        <h1 className="text-2xl font-bold tracking-tight">Almacenamiento</h1>
                        <p className="text-sm text-muted-foreground">
                            Base de datos y cache del tenant
                        </p>
                    </div>
                </div>
                <Button onClick={handleRefresh} variant="outline" className="shadow-clay-button hover:shadow-clay-card transition-shadow">
                    <RefreshCw className="mr-2 h-4 w-4" />
                    Actualizar
                </Button>
            </div>

            {/* ─── Cards Grid (Side by Side) ─── */}
            <div className="grid md:grid-cols-2 gap-6">

                {/* ━━━━ DATABASE CARD ━━━━ */}
                <Card className={cn(
                    "group relative overflow-hidden transition-all duration-300",
                    "hover:shadow-clay-float hover:-translate-y-0.5",
                    isDBConfigured && dbStatus === "connected"
                        ? "border-success/30"
                        : isDBConfigured
                            ? "border-warning/30"
                            : "border-accent/20"
                )}>
                    {/* Subtle gradient overlay */}
                    <div className={cn(
                        "absolute inset-0 opacity-30 pointer-events-none transition-opacity duration-500",
                        isDBConfigured && dbStatus === "connected"
                            ? "bg-gradient-to-br from-success/10 via-transparent to-transparent"
                            : isDBConfigured
                                ? "bg-gradient-to-br from-warning/10 via-transparent to-transparent"
                                : "bg-gradient-to-br from-accent/10 via-transparent to-transparent"
                    )} />

                    <div className="relative p-6 space-y-5">
                        {/* Card Header */}
                        <div className="flex items-start justify-between">
                            <div className="flex items-center gap-4">
                                <div className={cn(
                                    "relative p-3.5 rounded-2xl border shadow-clay-button transition-all duration-300",
                                    isDBConfigured && dbStatus === "connected"
                                        ? "bg-gradient-to-br from-success/15 to-success/5 border-success/30"
                                        : isDBConfigured
                                            ? "bg-gradient-to-br from-warning/15 to-warning/5 border-warning/30"
                                            : "bg-gradient-to-br from-accent/15 to-accent/5 border-accent/30"
                                )}>
                                    {isDBConfigured ? (() => {
                                        const { Icon, color } = getDriverIcon(settings?.userDb?.driver || "")
                                        return <Icon className={cn("h-6 w-6", color)} />
                                    })() : (
                                        <Database className="h-6 w-6 text-accent" />
                                    )}
                                </div>
                                <div>
                                    <h3 className="font-semibold text-foreground">Base de Datos</h3>
                                    <p className="text-xs text-muted-foreground mt-0.5">
                                        {isDBConfigured
                                            ? driverNames[settings?.userDb?.driver || ""] || "Configurada"
                                            : "Sin configurar"}
                                    </p>
                                </div>
                            </div>
                            <div className="flex items-center gap-2">
                                <StatusDot status={dbStatus} />
                                <span className={cn(
                                    "text-xs font-medium",
                                    dbStatus === "connected" ? "text-success"
                                        : dbStatus === "configured" ? "text-warning"
                                            : dbStatus === "loading" ? "text-accent"
                                                : "text-muted-foreground"
                                )}>
                                    {testingDb ? "Probando..."
                                        : dbStatus === "connected" ? "Conectado"
                                            : dbStatus === "configured" ? "Pendiente"
                                                : "Desconectado"}
                                </span>
                            </div>
                        </div>

                        {/* Card Body */}
                        {isDBConfigured ? (
                            <>
                                {/* Stack Info */}
                                <div className="rounded-xl border bg-muted/20 divide-y divide-border/60">
                                    <InfoRow
                                        label="Motor"
                                        value={driverNames[settings?.userDb?.driver || ""] || "N/A"}
                                        icon={HardDrive}
                                        iconColor="text-accent"
                                    />
                                    <InfoRow
                                        label="Schema"
                                        value={settings?.userDb?.schema || "public"}
                                        icon={Layers}
                                        iconColor="text-info"
                                    />
                                    {infraStats?.db?.size && (
                                        <InfoRow
                                            label="Tamano"
                                            value={infraStats.db.size}
                                            icon={Cpu}
                                            iconColor="text-warning"
                                        />
                                    )}
                                    {infraStats?.db?.table_count !== undefined && (
                                        <InfoRow
                                            label="Tablas"
                                            value={String(infraStats.db.table_count)}
                                            icon={Layers}
                                            iconColor="text-success"
                                        />
                                    )}
                                    <InfoRow
                                        label="Credenciales"
                                        value={settings?.userDb?.dsnEnc ? "Encriptadas" : "Sin DSN"}
                                        icon={Shield}
                                        iconColor="text-accent"
                                    />
                                </div>

                                {/* Connection alert */}
                                {dbStatus === "configured" && infraStats?.db !== "ok" && infraStats?.db?.error && (
                                    <div className="flex items-center gap-2 text-xs text-destructive bg-destructive/10 rounded-lg px-3 py-2 border border-destructive/20">
                                        <XCircle className="h-3.5 w-3.5 shrink-0" />
                                        <span className="truncate">{infraStats.db.error}</span>
                                    </div>
                                )}

                                {/* Actions */}
                                <div className="flex flex-wrap items-center gap-2 pt-1">
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        onClick={() => testDbMutation.mutate()}
                                        disabled={testDbMutation.isPending}
                                        className="shadow-clay-button hover:shadow-clay-card transition-shadow flex-1 sm:flex-none"
                                    >
                                        {testDbMutation.isPending ? (
                                            <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                                        ) : (
                                            <Zap className="mr-2 h-3.5 w-3.5" />
                                        )}
                                        Test
                                    </Button>
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        onClick={() => runMigrationsMutation.mutate()}
                                        disabled={runMigrationsMutation.isPending}
                                        className="shadow-clay-button hover:shadow-clay-card transition-shadow flex-1 sm:flex-none"
                                    >
                                        {runMigrationsMutation.isPending ? (
                                            <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                                        ) : (
                                            <Play className="mr-2 h-3.5 w-3.5" />
                                        )}
                                        Migrar
                                    </Button>
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        onClick={openDbModal}
                                        className="shadow-clay-button hover:shadow-clay-card transition-shadow flex-1 sm:flex-none"
                                    >
                                        <Settings2 className="mr-2 h-3.5 w-3.5" />
                                        Editar
                                    </Button>
                                </div>
                            </>
                        ) : (
                            /* ─── DB Empty State ─── */
                            <div className="flex flex-col items-center text-center py-6">
                                <p className="text-sm text-muted-foreground max-w-xs mb-5">
                                    Conecta una base de datos para almacenar usuarios, sesiones y datos de autenticacion.
                                </p>
                                <Button onClick={openDbModal} className="shadow-clay-button hover:shadow-clay-card transition-shadow">
                                    <Sparkles className="mr-2 h-4 w-4" />
                                    Configurar
                                    <ArrowRight className="ml-2 h-4 w-4" />
                                </Button>
                            </div>
                        )}
                    </div>
                </Card>

                {/* ━━━━ CACHE CARD ━━━━ */}
                <Card className={cn(
                    "group relative overflow-hidden transition-all duration-300",
                    "hover:shadow-clay-float hover:-translate-y-0.5",
                    isCacheEnabled && cacheStatus === "connected"
                        ? "border-success/30"
                        : isCacheEnabled
                            ? "border-warning/30"
                            : "border-accent/20"
                )}>
                    {/* Subtle gradient overlay */}
                    <div className={cn(
                        "absolute inset-0 opacity-30 pointer-events-none transition-opacity duration-500",
                        isCacheEnabled && cacheStatus === "connected"
                            ? "bg-gradient-to-br from-success/10 via-transparent to-transparent"
                            : isCacheEnabled
                                ? "bg-gradient-to-br from-warning/10 via-transparent to-transparent"
                                : "bg-gradient-to-br from-accent/10 via-transparent to-transparent"
                    )} />

                    <div className="relative p-6 space-y-5">
                        {/* Card Header */}
                        <div className="flex items-start justify-between">
                            <div className="flex items-center gap-4">
                                <div className={cn(
                                    "relative p-3.5 rounded-2xl border shadow-clay-button transition-all duration-300",
                                    isCacheEnabled && cacheStatus === "connected"
                                        ? "bg-gradient-to-br from-success/15 to-success/5 border-success/30"
                                        : isCacheEnabled
                                            ? "bg-gradient-to-br from-warning/15 to-warning/5 border-warning/30"
                                            : "bg-gradient-to-br from-accent/15 to-accent/5 border-accent/30"
                                )}>
                                    {isCacheEnabled && settings?.cache?.driver === "redis" ? (
                                        <RedisIcon className="h-6 w-6 text-[#D82C20]" />
                                    ) : isCacheEnabled ? (
                                        <Cpu className="h-6 w-6 text-warning" />
                                    ) : (
                                        <Server className="h-6 w-6 text-accent" />
                                    )}
                                </div>
                                <div>
                                    <h3 className="font-semibold text-foreground">Capa de Cache</h3>
                                    <p className="text-xs text-muted-foreground mt-0.5">
                                        {isCacheEnabled
                                            ? cacheDriverNames[settings?.cache?.driver || "memory"] || "Habilitada"
                                            : isCacheConfigured
                                                ? "Deshabilitada"
                                                : "Sin configurar"}
                                    </p>
                                </div>
                            </div>
                            <div className="flex items-center gap-2">
                                <StatusDot status={isCacheEnabled ? cacheStatus : "disconnected"} />
                                <span className={cn(
                                    "text-xs font-medium",
                                    isCacheEnabled && cacheStatus === "connected" ? "text-success"
                                        : isCacheEnabled ? "text-warning"
                                            : "text-muted-foreground"
                                )}>
                                    {testingCache ? "Probando..."
                                        : isCacheEnabled && cacheStatus === "connected" ? "Conectado"
                                            : isCacheEnabled ? "Pendiente"
                                                : isCacheConfigured ? "Apagado"
                                                    : "Desconectado"}
                                </span>
                            </div>
                        </div>

                        {/* Card Body */}
                        {isCacheConfigured ? (
                            <>
                                {/* Stack Info */}
                                <div className="rounded-xl border bg-muted/20 divide-y divide-border/60">
                                    <InfoRow
                                        label="Driver"
                                        value={cacheDriverNames[settings?.cache?.driver || ""] || "N/A"}
                                        icon={Cpu}
                                        iconColor="text-accent"
                                    />
                                    <InfoRow
                                        label="Estado"
                                        value={isCacheEnabled ? "Habilitada" : "Deshabilitada"}
                                        icon={isCacheEnabled ? Power : PowerOff}
                                        iconColor={isCacheEnabled ? "text-success" : "text-muted-foreground"}
                                    />
                                    {settings?.cache?.driver === "redis" && settings.cache.host && (
                                        <InfoRow
                                            label="Host"
                                            value={`${settings.cache.host}:${settings.cache.port}`}
                                            icon={Activity}
                                            iconColor="text-info"
                                        />
                                    )}
                                    {settings?.cache?.driver === "redis" && (
                                        <InfoRow
                                            label="DB Index"
                                            value={String(settings.cache.db ?? 0)}
                                            icon={Layers}
                                            iconColor="text-warning"
                                        />
                                    )}
                                    {settings?.cache?.prefix && (
                                        <InfoRow
                                            label="Prefijo"
                                            value={settings.cache.prefix}
                                            icon={HardDrive}
                                            iconColor="text-accent"
                                        />
                                    )}
                                </div>

                                {/* Actions */}
                                <div className="flex flex-wrap items-center gap-2 pt-1">
                                    {isCacheEnabled && (
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            onClick={() => testCacheMutation.mutate()}
                                            disabled={testCacheMutation.isPending}
                                            className="shadow-clay-button hover:shadow-clay-card transition-shadow flex-1 sm:flex-none"
                                        >
                                            {testCacheMutation.isPending ? (
                                                <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                                            ) : (
                                                <Zap className="mr-2 h-3.5 w-3.5" />
                                            )}
                                            Test
                                        </Button>
                                    )}
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        onClick={openCacheModal}
                                        className="shadow-clay-button hover:shadow-clay-card transition-shadow flex-1 sm:flex-none"
                                    >
                                        <Settings2 className="mr-2 h-3.5 w-3.5" />
                                        Editar
                                    </Button>
                                    <Button
                                        variant={isCacheEnabled ? "outline" : "outline"}
                                        size="sm"
                                        onClick={() => handleToggleCache(!isCacheEnabled)}
                                        disabled={updateSettingsMutation.isPending}
                                        className={cn(
                                            "shadow-clay-button hover:shadow-clay-card transition-shadow flex-1 sm:flex-none",
                                            isCacheEnabled ? "text-destructive hover:text-destructive" : "text-success hover:text-success"
                                        )}
                                    >
                                        {updateSettingsMutation.isPending ? (
                                            <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                                        ) : isCacheEnabled ? (
                                            <PowerOff className="mr-2 h-3.5 w-3.5" />
                                        ) : (
                                            <Power className="mr-2 h-3.5 w-3.5" />
                                        )}
                                        {isCacheEnabled ? "Deshabilitar" : "Habilitar"}
                                    </Button>
                                </div>
                            </>
                        ) : (
                            /* ─── Cache Empty State ─── */
                            <div className="flex flex-col items-center text-center py-6">
                                <p className="text-sm text-muted-foreground max-w-xs mb-5">
                                    Configura una capa de cache para acelerar los tiempos de respuesta y reducir la carga.
                                </p>
                                <Button onClick={openCacheModal} className="shadow-clay-button hover:shadow-clay-card transition-shadow">
                                    <Sparkles className="mr-2 h-4 w-4" />
                                    Configurar
                                    <ArrowRight className="ml-2 h-4 w-4" />
                                </Button>
                            </div>
                        )}
                    </div>
                </Card>
            </div>

            {/* ─── Modals ─── */}
            <DatabaseConfigModal
                open={isDbModalOpen}
                onClose={() => setIsDbModalOpen(false)}
                dbForm={dbForm}
                setDbForm={setDbForm}
                onSave={handleSaveDB}
                isPending={updateSettingsMutation.isPending}
            />
            <CacheConfigModal
                open={isCacheModalOpen}
                onClose={() => setIsCacheModalOpen(false)}
                cacheForm={cacheForm}
                setCacheForm={setCacheForm}
                onSave={handleSaveCache}
                isPending={updateSettingsMutation.isPending}
            />
        </div>
    )
}
