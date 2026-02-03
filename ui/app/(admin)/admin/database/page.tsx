"use client"

import { useState, useEffect } from "react"
import { useSearchParams } from "next/navigation"
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
    ExternalLink,
    Shield,
    Activity,
    Server,
    Cpu,
    Layers,
    ArrowRight,
    ArrowLeft,
    Sparkles,
} from "lucide-react"

// ============================================================================
// DATABASE ICONS - Imported from shared components
// ============================================================================

import {
    PostgresIcon,
    MySQLIcon,
    MongoDBIcon,
    RedisIcon,
    getDriverIcon,
    getCacheDriverIcon,
} from "@/components/icons/database-icons"

// DS Components (UI Unification)
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
    CardDescription,
    CardHeader,
    CardTitle,
    Badge,
    InlineAlert,
    Skeleton,
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
// HELPER COMPONENTS
// ============================================================================

function StatusBadge({ 
    status, 
    label 
}: { 
    status: "connected" | "configured" | "disconnected" | "warning" | "loading"
    label: string 
}) {
    const variants: Record<string, "success" | "outline" | "warning" | "default"> = {
        connected: "success",
        configured: "warning",
        disconnected: "outline",
        warning: "warning",
        loading: "default",
    }

    const icons: Record<string, React.ReactNode> = {
        connected: <CheckCircle2 className="h-3 w-3" />,
        configured: <AlertCircle className="h-3 w-3" />,
        disconnected: null,
        warning: <AlertCircle className="h-3 w-3" />,
        loading: <Loader2 className="h-3 w-3 animate-spin" />,
    }

    return (
        <Badge variant={variants[status]} className="gap-1.5">
            {icons[status]}
            {label}
        </Badge>
    )
}

// ============================================================================
// MAIN COMPONENT
// ============================================================================

export default function DatabasePage() {
    const searchParams = useSearchParams()
    const tenantId = searchParams.get("id")
    const { token } = useAuthStore()
    const { toast } = useToast()
    const queryClient = useQueryClient()
    const { t } = useI18n()

    // State
    const [isEditingDB, setIsEditingDB] = useState(false)
    const [isEditingCache, setIsEditingCache] = useState(false)
    const [showDsn, setShowDsn] = useState(false)
    const [etag, setEtag] = useState<string>("")
    const [testingConnection, setTestingConnection] = useState(false)

    const [dbForm, setDbForm] = useState<UserDBSettings>({
        driver: "postgres",
        dsn: "",
        schema: "",
    })

    const [cacheForm, setCacheForm] = useState<CacheSettings>({
        enabled: false,
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
                setIsEditingDB(!(settings.userDb.dsnEnc || settings.userDb.dsn))
            } else {
                setIsEditingDB(true)
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
                setIsEditingCache(false)
            } else {
                setCacheForm((prev) => ({ ...prev, enabled: false }))
                setIsEditingCache(false)
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
            setIsEditingDB(false)
            setIsEditingCache(false)
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
            setTestingConnection(true)
            await api.post(`/v2/admin/tenants/${tenantId}/user-store/test-connection`, {})
        },
        onSuccess: () => {
            toast({ variant: "success", title: t("database.connectionSuccess"), description: t("database.testConnectionDesc") })
            setTestingConnection(false)
        },
        onError: (err: any) => {
            toast({ variant: "destructive", title: t("common.error"), description: err.response?.data?.error_description || err.message })
            setTestingConnection(false)
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
            await api.post(`/v2/admin/tenants/${tenantId}/cache/test-connection`, {})
        },
        onSuccess: () => {
            toast({ title: t("database.connectionSuccess"), description: "Cache connection verified.", variant: "success" })
        },
        onError: (err: any) => {
            toast({ variant: "destructive", title: t("common.error"), description: err.response?.data?.error_description || err.message })
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

    const handleRefresh = async () => {
        await Promise.all([refetchSettings(), refetchStats()])
        toast({ title: "Actualizado", description: "Datos actualizados correctamente", variant: "info" })
    }

    // ========================================================================
    // COMPUTED
    // ========================================================================

    const isDBConfigured = !!settings?.userDb?.dsnEnc || !!settings?.userDb?.dsn
    const isCacheEnabled = settings?.cache?.enabled || false
    const dbStatus: "connected" | "configured" | "disconnected" | "warning" | "loading" = testingConnection 
        ? "loading" 
        : isDBConfigured 
            ? (infraStats?.db === "ok" ? "connected" : "configured") 
            : "disconnected"
    const cacheStatus: "connected" | "configured" | "disconnected" | "warning" = isCacheEnabled 
        ? (infraStats?.cache === "ok" ? "connected" : "configured") 
        : "disconnected"

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
                {/* Header Skeleton */}
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-4">
                        <Skeleton className="h-9 w-9 rounded-md" />
                        <div className="flex items-center gap-3">
                            <Skeleton className="h-12 w-12 rounded-xl" />
                            <div className="space-y-2">
                                <Skeleton className="h-7 w-48" />
                                <Skeleton className="h-4 w-72" />
                            </div>
                        </div>
                    </div>
                    <Skeleton className="h-10 w-32" />
                </div>
                
                {/* Database Card Skeleton */}
                <Card>
                    <CardHeader>
                        <div className="flex items-center justify-between">
                            <div className="flex items-center gap-3">
                                <Skeleton className="h-10 w-10 rounded-lg" />
                                <div className="space-y-2">
                                    <Skeleton className="h-5 w-48" />
                                    <Skeleton className="h-4 w-72" />
                                </div>
                            </div>
                            <Skeleton className="h-6 w-24 rounded-full" />
                        </div>
                    </CardHeader>
                    <CardContent className="space-y-6">
                        <div className="grid md:grid-cols-3 gap-4">
                            {[1, 2, 3].map((i) => (
                                <div key={i} className="p-4 rounded-lg bg-muted/30 space-y-2">
                                    <Skeleton className="h-4 w-16" />
                                    <Skeleton className="h-5 w-32" />
                                </div>
                            ))}
                        </div>
                    </CardContent>
                </Card>

                {/* Cache Card Skeleton */}
                <Card>
                    <CardHeader>
                        <div className="flex items-center justify-between">
                            <div className="flex items-center gap-3">
                                <Skeleton className="h-10 w-10 rounded-lg" />
                                <div className="space-y-2">
                                    <Skeleton className="h-5 w-40" />
                                    <Skeleton className="h-4 w-64" />
                                </div>
                            </div>
                            <Skeleton className="h-6 w-24 rounded-full" />
                        </div>
                    </CardHeader>
                    <CardContent className="space-y-6">
                        <div className="grid md:grid-cols-4 gap-4">
                            {[1, 2, 3, 4].map((i) => (
                                <div key={i} className="p-4 rounded-lg bg-muted/30 space-y-2">
                                    <Skeleton className="h-4 w-16" />
                                    <Skeleton className="h-5 w-24" />
                                </div>
                            ))}
                        </div>
                    </CardContent>
                </Card>
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
                        <Link href="/admin">
                            <ArrowLeft className="h-4 w-4" />
                        </Link>
                    </Button>
                    <div className="flex items-center gap-3">
                        <div>
                            <h1 className="text-2xl font-bold tracking-tight">Almacenamiento</h1>
                            <p className="text-sm text-muted-foreground">
                                Configura y monitorea la base de datos y caché del tenant
                            </p>
                        </div>
                    </div>
                </div>
                <Button onClick={handleRefresh} variant="outline" className="shadow-clay-button hover:shadow-clay-card transition-shadow">
                    <RefreshCw className="mr-2 h-4 w-4" />
                    Actualizar
                </Button>
            </div>

            {/* Database Section */}
            <Card>
                <CardHeader>
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                            <div className="p-2 rounded-lg bg-success/10">
                                <Database className="h-4 w-4 text-success" />
                            </div>
                            <div>
                                <CardTitle className="text-base">Base de Datos de Usuarios</CardTitle>
                                <CardDescription>
                                    Almacenamiento principal para cuentas, sesiones y datos de autenticación
                                </CardDescription>
                            </div>
                        </div>
                        <StatusBadge 
                            status={dbStatus} 
                            label={testingConnection ? "Probando..." : isDBConfigured ? (infraStats?.db === "ok" ? "Conectado" : "Pendiente") : "Sin configurar"} 
                        />
                    </div>
                </CardHeader>
                <CardContent>
                    {isEditingDB ? (
                        /* ─── Database Edit Mode ─── */
                        <div className="space-y-6">
                            <div className="grid gap-6 md:grid-cols-2">
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

                            <div className="flex items-center justify-end gap-3 pt-4 border-t">
                                <Button
                                    variant="ghost"
                                    onClick={() => setIsEditingDB(false)}
                                    disabled={updateSettingsMutation.isPending}
                                >
                                    Cancelar
                                </Button>
                                <Button
                                    onClick={handleSaveDB}
                                    disabled={updateSettingsMutation.isPending || !dbForm.dsn}
                                    className="shadow-clay-button"
                                >
                                    {updateSettingsMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                    Guardar Configuración
                                </Button>
                            </div>
                        </div>
                    ) : (
                        /* ─── Database View Mode ─── */
                        <>
                            {isDBConfigured ? (
                                <div className="space-y-6">
                                    {/* Metrics Grid */}
                                    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                                        {(() => {
                                            const { Icon, color } = getDriverIcon(settings?.userDb?.driver || "")
                                            const driverNames: Record<string, string> = {
                                                postgres: "PostgreSQL",
                                                mysql: "MySQL",
                                                mongo: "MongoDB",
                                            }
                                            return (
                                                <div className="flex items-center gap-3 p-4 rounded-lg bg-muted/30 border">
                                                    <div className="p-2.5 rounded-lg bg-muted">
                                                        <Icon className={cn("h-5 w-5", color)} />
                                                    </div>
                                                    <div>
                                                        <p className="text-xs text-muted-foreground uppercase tracking-wider">Motor</p>
                                                        <p className="font-semibold">{driverNames[settings?.userDb?.driver || ""] || "N/A"}</p>
                                                    </div>
                                                </div>
                                            )
                                        })()}
                                        <div className="flex items-center gap-3 p-4 rounded-lg bg-muted/30 border">
                                            <div className="p-2 rounded-lg bg-info/10">
                                                <HardDrive className="h-4 w-4 text-info" />
                                            </div>
                                            <div>
                                                <p className="text-xs text-muted-foreground uppercase tracking-wider">Schema</p>
                                                <p className="font-semibold">{settings?.userDb?.schema || "public"}</p>
                                            </div>
                                        </div>
                                        {infraStats?.db?.size && (
                                            <div className="flex items-center gap-3 p-4 rounded-lg bg-muted/30 border">
                                                <div className="p-2 rounded-lg bg-accent/10">
                                                    <Cpu className="h-4 w-4 text-accent" />
                                                </div>
                                                <div>
                                                    <p className="text-xs text-muted-foreground uppercase tracking-wider">Tamaño</p>
                                                    <p className="font-semibold">{infraStats.db.size}</p>
                                                </div>
                                            </div>
                                        )}
                                        {infraStats?.db?.table_count !== undefined && (
                                            <div className="flex items-center gap-3 p-4 rounded-lg bg-muted/30 border">
                                                <div className="p-2 rounded-lg bg-warning/10">
                                                    <Layers className="h-4 w-4 text-warning" />
                                                </div>
                                                <div>
                                                    <p className="text-xs text-muted-foreground uppercase tracking-wider">Tablas</p>
                                                    <p className="font-semibold">{infraStats.db.table_count}</p>
                                                </div>
                                            </div>
                                        )}
                                    </div>

                                    {/* Actions */}
                                    <div className="flex flex-wrap items-center justify-end gap-3 pt-4 border-t">
                                        <Button
                                            variant="outline"
                                            onClick={() => testDbMutation.mutate()}
                                            disabled={testDbMutation.isPending}
                                            className="shadow-clay-button"
                                        >
                                            {testDbMutation.isPending ? (
                                                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                            ) : (
                                                <Zap className="mr-2 h-4 w-4" />
                                            )}
                                            Probar Conexión
                                        </Button>
                                        <Button
                                            variant="outline"
                                            onClick={() => runMigrationsMutation.mutate()}
                                            disabled={runMigrationsMutation.isPending}
                                            className="shadow-clay-button"
                                        >
                                            {runMigrationsMutation.isPending ? (
                                                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                            ) : (
                                                <Play className="mr-2 h-4 w-4" />
                                            )}
                                            Ejecutar Migraciones
                                        </Button>
                                        <Button
                                            variant="outline"
                                            onClick={() => setIsEditingDB(true)}
                                            className="shadow-clay-button"
                                        >
                                            <Settings2 className="mr-2 h-4 w-4" />
                                            Editar
                                        </Button>
                                    </div>
                                </div>
                            ) : (
                                /* ─── Database Empty State ─── */
                                <div className="flex flex-col items-center justify-center py-12 text-center">
                                    <div className="relative mb-6 group">
                                        <div className="absolute inset-0 bg-gradient-to-br from-success/30 to-accent/20 rounded-full blur-2xl scale-150 group-hover:scale-175 transition-transform duration-700" />
                                        <div className="relative rounded-2xl bg-gradient-to-br from-success/10 to-accent/5 p-8 border border-success/20 shadow-clay-card">
                                            <Database className="h-12 w-12 text-success" />
                                        </div>
                                    </div>
                                    <h3 className="text-xl font-bold mb-2">Sin base de datos configurada</h3>
                                    <p className="text-muted-foreground text-sm max-w-md mb-6">
                                        Conecta una base de datos para almacenar usuarios, sesiones y registros de autenticación de forma segura.
                                    </p>
                                    <Button onClick={() => setIsEditingDB(true)} size="lg" className="shadow-clay-button">
                                        <Sparkles className="mr-2 h-4 w-4" />
                                        Configurar Base de Datos
                                        <ArrowRight className="ml-2 h-4 w-4" />
                                    </Button>
                                </div>
                            )}
                        </>
                    )}
                </CardContent>
            </Card>

            {/* Cache Section */}
            <Card>
                <CardHeader>
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                            <div className="p-2 rounded-lg bg-accent/10">
                                <Server className="h-4 w-4 text-accent" />
                            </div>
                            <div>
                                <CardTitle className="text-base">Capa de Caché</CardTitle>
                                <CardDescription>
                                    Acelera tu aplicación con caché distribuido
                                </CardDescription>
                            </div>
                        </div>
                        <div className="flex items-center gap-3">
                            <StatusBadge 
                                status={isCacheEnabled ? cacheStatus : "disconnected"} 
                                label={isCacheEnabled ? (infraStats?.cache === "ok" ? "Conectado" : "Pendiente") : "Deshabilitado"} 
                            />
                            <Switch
                                checked={cacheForm.enabled}
                                onCheckedChange={(checked) => {
                                    setCacheForm((prev) => ({ ...prev, enabled: checked }))
                                    setIsEditingCache(true)
                                }}
                            />
                        </div>
                    </div>
                </CardHeader>
                <CardContent>
                    {isEditingCache ? (
                        /* ─── Cache Edit Mode ─── */
                        <div className="space-y-6">
                            {cacheForm.enabled ? (
                                <>
                                    <div className="space-y-2">
                                        <Label>Driver de Caché</Label>
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
                                            <div className="grid gap-6 sm:grid-cols-2">
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
                                                <Label>Contraseña (Opcional)</Label>
                                                <Input
                                                    type="password"
                                                    placeholder="•••••••••"
                                                    value={cacheForm.password}
                                                    onChange={(e) => setCacheForm({ ...cacheForm, password: e.target.value })}
                                                />
                                            </div>
                                            <div className="grid gap-6 sm:grid-cols-2">
                                                <div className="space-y-2">
                                                    <Label>Índice de Base de Datos</Label>
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

                                    {cacheForm.driver === "memory" && (
                                        <InlineAlert 
                                            variant="warning"
                                            title="Limitaciones del caché en memoria"
                                            description="El caché en memoria no se comparte entre instancias y se borra al reiniciar. Para cargas de producción, considera usar Redis."
                                        />
                                    )}
                                </>
                            ) : (
                                <div className="py-12 text-center">
                                    <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-muted mb-4">
                                        <Server className="h-8 w-8 text-muted-foreground" />
                                    </div>
                                    <p className="text-muted-foreground">Habilita el caché para mejorar el rendimiento</p>
                                </div>
                            )}

                            <div className="flex items-center justify-end gap-3 pt-4 border-t">
                                <Button
                                    variant="ghost"
                                    onClick={() => {
                                        if (settings?.cache) {
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
                                        setIsEditingCache(false)
                                    }}
                                    disabled={updateSettingsMutation.isPending}
                                >
                                    Cancelar
                                </Button>
                                <Button
                                    onClick={handleSaveCache}
                                    disabled={updateSettingsMutation.isPending}
                                    className="shadow-clay-button"
                                >
                                    {updateSettingsMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                    Guardar Configuración
                                </Button>
                            </div>
                        </div>
                    ) : (
                        /* ─── Cache View Mode ─── */
                        <>
                            {isCacheEnabled ? (
                                <div className="space-y-6">
                                    {/* Metrics Grid */}
                                    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                                        <div className="flex items-center gap-3 p-4 rounded-lg bg-muted/30 border">
                                            <div className="p-2 rounded-lg bg-accent/10">
                                                {settings?.cache?.driver === "redis" ? (
                                                    <RedisIcon className="h-4 w-4" />
                                                ) : (
                                                    <Cpu className="h-4 w-4 text-warning" />
                                                )}
                                            </div>
                                            <div>
                                                <p className="text-xs text-muted-foreground uppercase tracking-wider">Driver</p>
                                                <p className="font-semibold">{settings?.cache?.driver === "memory" ? "Memory" : "Redis"}</p>
                                            </div>
                                        </div>
                                        {settings?.cache?.driver === "redis" && (
                                            <div className="flex items-center gap-3 p-4 rounded-lg bg-muted/30 border">
                                                <div className="p-2 rounded-lg bg-info/10">
                                                    <ExternalLink className="h-4 w-4 text-info" />
                                                </div>
                                                <div>
                                                    <p className="text-xs text-muted-foreground uppercase tracking-wider">Endpoint</p>
                                                    <p className="font-semibold text-sm">{`${settings?.cache?.host || "localhost"}:${settings?.cache?.port || 6379}`}</p>
                                                </div>
                                            </div>
                                        )}
                                        {infraStats?.cache?.keys !== undefined && (
                                            <div className="flex items-center gap-3 p-4 rounded-lg bg-muted/30 border">
                                                <div className="p-2 rounded-lg bg-warning/10">
                                                    <Activity className="h-4 w-4 text-warning" />
                                                </div>
                                                <div>
                                                    <p className="text-xs text-muted-foreground uppercase tracking-wider">Keys</p>
                                                    <p className="font-semibold">{infraStats.cache.keys.toLocaleString()}</p>
                                                </div>
                                            </div>
                                        )}
                                        {infraStats?.cache?.used_memory && (
                                            <div className="flex items-center gap-3 p-4 rounded-lg bg-muted/30 border">
                                                <div className="p-2 rounded-lg bg-success/10">
                                                    <Cpu className="h-4 w-4 text-success" />
                                                </div>
                                                <div>
                                                    <p className="text-xs text-muted-foreground uppercase tracking-wider">Memoria</p>
                                                    <p className="font-semibold">{infraStats.cache.used_memory}</p>
                                                </div>
                                            </div>
                                        )}
                                    </div>

                                    {/* Actions */}
                                    <div className="flex flex-wrap items-center justify-end gap-3 pt-4 border-t">
                                        <Button
                                            variant="outline"
                                            onClick={() => testCacheMutation.mutate()}
                                            disabled={testCacheMutation.isPending}
                                            className="shadow-clay-button"
                                        >
                                            {testCacheMutation.isPending ? (
                                                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                            ) : (
                                                <Zap className="mr-2 h-4 w-4" />
                                            )}
                                            Probar Conexión
                                        </Button>
                                        <Button
                                            variant="outline"
                                            onClick={() => setIsEditingCache(true)}
                                            className="shadow-clay-button"
                                        >
                                            <Settings2 className="mr-2 h-4 w-4" />
                                            Editar
                                        </Button>
                                    </div>
                                </div>
                            ) : (
                                /* ─── Cache Empty State ─── */
                                <div className="flex flex-col items-center justify-center py-12 text-center">
                                    <div className="relative mb-6 group">
                                        <div className="absolute inset-0 bg-gradient-to-br from-accent/30 to-info/20 rounded-full blur-2xl scale-150 group-hover:scale-175 transition-transform duration-700" />
                                        <div className="relative rounded-2xl bg-gradient-to-br from-accent/10 to-info/5 p-8 border border-accent/20 shadow-clay-card">
                                            <Server className="h-12 w-12 text-accent" />
                                        </div>
                                    </div>
                                    <h3 className="text-xl font-bold mb-2">Caché deshabilitado</h3>
                                    <p className="text-muted-foreground text-sm max-w-md mb-6">
                                        Habilita el caché para mejorar dramáticamente los tiempos de respuesta y reducir la carga en la base de datos.
                                    </p>
                                </div>
                            )}
                        </>
                    )}
                </CardContent>
            </Card>
        </div>
    )
}
