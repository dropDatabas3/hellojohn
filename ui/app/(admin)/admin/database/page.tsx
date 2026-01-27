"use client"

import { useState, useEffect } from "react"
import { useSearchParams } from "next/navigation"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
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
  Clock,
  Server,
  Cpu,
  Layers,
  ArrowRight,
  Check,
  X,
  Sparkles,
} from "lucide-react"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { cn } from "@/lib/utils"

// Types
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

// ─── Animated Status Dot ───
function StatusDot({
  status,
  size = "md"
}: {
  status: "connected" | "disconnected" | "warning" | "loading"
  size?: "sm" | "md" | "lg"
}) {
  const sizeClasses = {
    sm: "h-2 w-2",
    md: "h-3 w-3",
    lg: "h-4 w-4"
  }

  const colors = {
    connected: "bg-emerald-500",
    disconnected: "bg-zinc-400",
    warning: "bg-amber-500",
    loading: "bg-blue-500"
  }

  const shouldPulse = status === "connected" || status === "loading"

  return (
    <span className="relative flex">
      {shouldPulse && (
        <span className={cn(
          "absolute inline-flex rounded-full opacity-75 animate-ping",
          sizeClasses[size],
          colors[status]
        )} />
      )}
      <span className={cn(
        "relative inline-flex rounded-full",
        sizeClasses[size],
        colors[status],
        status === "loading" && "animate-pulse"
      )} />
    </span>
  )
}

// ─── Glass Card Component ───
function GlassCard({
  children,
  className,
  hover = true
}: {
  children: React.ReactNode
  className?: string
  hover?: boolean
}) {
  return (
    <div className={cn(
      "relative rounded-3xl overflow-hidden",
      "bg-white/80 dark:bg-zinc-900/80 backdrop-blur-xl",
      "border border-zinc-200/50 dark:border-zinc-700/50",
      "shadow-xl shadow-zinc-200/20 dark:shadow-zinc-900/30",
      hover && "transition-all duration-500 hover:shadow-2xl hover:shadow-zinc-300/30 dark:hover:shadow-zinc-800/40 hover:-translate-y-1",
      className
    )}>
      {children}
    </div>
  )
}

// ─── Metric Display ───
function MetricDisplay({
  icon: Icon,
  label,
  value,
  subtext,
  accentColor = "zinc"
}: {
  icon: any
  label: string
  value: string | number
  subtext?: string
  accentColor?: "zinc" | "emerald" | "violet" | "blue"
}) {
  const colorClasses = {
    zinc: "from-zinc-500/20 to-transparent text-zinc-600 dark:text-zinc-400",
    emerald: "from-emerald-500/20 to-transparent text-emerald-600 dark:text-emerald-400",
    violet: "from-violet-500/20 to-transparent text-violet-600 dark:text-violet-400",
    blue: "from-blue-500/20 to-transparent text-blue-600 dark:text-blue-400",
  }

  return (
    <div className="group relative p-5 rounded-2xl bg-gradient-to-br from-zinc-50 to-white dark:from-zinc-800/50 dark:to-zinc-900/50 border border-zinc-100 dark:border-zinc-800 transition-all duration-300 hover:scale-[1.02]">
      <div className={cn(
        "absolute top-0 left-0 right-0 h-1 rounded-t-2xl bg-gradient-to-r opacity-0 group-hover:opacity-100 transition-opacity duration-300",
        colorClasses[accentColor].split(" ")[0]
      )} />
      <div className="flex items-center gap-4">
        <div className={cn(
          "p-3 rounded-xl bg-gradient-to-br",
          colorClasses[accentColor]
        )}>
          <Icon className="h-5 w-5" />
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-xs font-semibold uppercase tracking-widest text-zinc-400 dark:text-zinc-500 mb-1">
            {label}
          </p>
          <p className="text-xl font-bold text-zinc-900 dark:text-white truncate">
            {value}
          </p>
          {subtext && (
            <p className="text-xs text-zinc-500 mt-1">{subtext}</p>
          )}
        </div>
      </div>
    </div>
  )
}

// ─── Section Header ───
function SectionHeader({
  icon: Icon,
  title,
  description,
  status,
  statusLabel,
  accentColor = "zinc",
  actions
}: {
  icon: any
  title: string
  description: string
  status: "connected" | "disconnected" | "warning" | "loading"
  statusLabel: string
  accentColor?: "emerald" | "violet" | "zinc"
  actions?: React.ReactNode
}) {
  const iconBgColors = {
    emerald: "bg-gradient-to-br from-emerald-400 to-emerald-600",
    violet: "bg-gradient-to-br from-violet-400 to-violet-600",
    zinc: "bg-gradient-to-br from-zinc-400 to-zinc-600",
  }

  const statusColors = {
    connected: "text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-900/30",
    disconnected: "text-zinc-500 dark:text-zinc-400 bg-zinc-100 dark:bg-zinc-800",
    warning: "text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/30",
    loading: "text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/30",
  }

  return (
    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
      <div className="flex items-center gap-4">
        <div className={cn(
          "p-3.5 rounded-2xl shadow-lg",
          iconBgColors[accentColor]
        )}>
          <Icon className="h-6 w-6 text-white" />
        </div>
        <div>
          <h2 className="text-xl font-bold text-zinc-900 dark:text-white">
            {title}
          </h2>
          <p className="text-sm text-zinc-500 dark:text-zinc-400 mt-0.5">
            {description}
          </p>
        </div>
      </div>
      <div className="flex items-center gap-3">
        <div className={cn(
          "flex items-center gap-2 px-4 py-2 rounded-full text-sm font-medium transition-all duration-300",
          statusColors[status]
        )}>
          <StatusDot status={status} size="sm" />
          <span>{statusLabel}</span>
        </div>
        {actions}
      </div>
    </div>
  )
}

// ─── Action Button ───
function ActionButton({
  icon: Icon,
  children,
  onClick,
  loading = false,
  variant = "secondary",
  disabled = false,
}: {
  icon: any
  children: React.ReactNode
  onClick: () => void
  loading?: boolean
  variant?: "primary" | "secondary"
  disabled?: boolean
}) {
  return (
    <button
      onClick={onClick}
      disabled={loading || disabled}
      className={cn(
        "group relative inline-flex items-center gap-2.5 px-5 py-2.5 rounded-xl text-sm font-semibold transition-all duration-300",
        "disabled:opacity-50 disabled:cursor-not-allowed",
        variant === "primary"
          ? "bg-zinc-900 dark:bg-white text-white dark:text-zinc-900 hover:scale-105 shadow-lg hover:shadow-xl"
          : "bg-zinc-100 dark:bg-zinc-800 text-zinc-700 dark:text-zinc-300 hover:bg-zinc-200 dark:hover:bg-zinc-700"
      )}
    >
      {loading ? (
        <Loader2 className="h-4 w-4 animate-spin" />
      ) : (
        <Icon className="h-4 w-4 group-hover:scale-110 transition-transform" />
      )}
      <span>{children}</span>
    </button>
  )
}

// ─── Empty State ───
function EmptyState({
  icon: Icon,
  title,
  description,
  action,
}: {
  icon: any
  title: string
  description: string
  action?: React.ReactNode
}) {
  return (
    <div className="relative py-16 px-8 text-center">
      <div className="absolute inset-0 bg-gradient-to-b from-zinc-50/50 to-transparent dark:from-zinc-800/20 rounded-3xl" />
      <div className="relative">
        <div className="inline-flex items-center justify-center w-20 h-20 rounded-3xl bg-gradient-to-br from-zinc-100 to-zinc-200 dark:from-zinc-800 dark:to-zinc-700 mb-6 shadow-inner">
          <Icon className="h-10 w-10 text-zinc-400 dark:text-zinc-500" />
        </div>
        <h3 className="text-xl font-bold text-zinc-900 dark:text-white mb-2">
          {title}
        </h3>
        <p className="text-sm text-zinc-500 dark:text-zinc-400 max-w-md mx-auto mb-8">
          {description}
        </p>
        {action}
      </div>
    </div>
  )
}

// ─── Main Component ───
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

  // Queries
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

  // Effects
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

  // Mutations
  const updateSettingsMutation = useMutation({
    mutationFn: async (data: TenantSettings) => {
      if (!tenantId || !token) throw new Error("No tenant ID")
      const payload = { ...settings, ...data }
      await api.put(`/v2/admin/tenants/${tenantId}/settings`, payload, etag)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tenant-storage", tenantId] })
      toast({ title: t("common.success"), description: t("tenants.settingsUpdatedDesc") })
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
      toast({ title: t("database.migrationsApplied"), description: t("database.migrationsAppliedDesc", { count: data.applied_count || 0 }) })
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
      toast({ title: t("database.connectionSuccess"), description: "Cache connection verified." })
    },
    onError: (err: any) => {
      toast({ variant: "destructive", title: t("common.error"), description: err.response?.data?.error_description || err.message })
    },
  })

  // Handlers
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
    toast({ title: "Refreshed", description: "Data updated successfully" })
  }

  // Computed
  const isDBConfigured = !!settings?.userDb?.dsnEnc || !!settings?.userDb?.dsn
  const isCacheEnabled = settings?.cache?.enabled || false
  const dbStatus = isDBConfigured ? (infraStats?.db === "ok" ? "connected" : "warning") : "disconnected"
  const cacheStatus = isCacheEnabled ? (infraStats?.cache === "ok" ? "connected" : "warning") : "disconnected"

  if (isError) {
    return (
      <div className="max-w-2xl mx-auto mt-12">
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>{t("common.error")}</AlertTitle>
          <AlertDescription>{(error as any)?.message || "Failed to load settings"}</AlertDescription>
        </Alert>
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[500px] gap-4">
        <div className="relative">
          <div className="absolute inset-0 rounded-full bg-gradient-to-r from-emerald-500 to-violet-500 animate-spin opacity-20 blur-xl" />
          <Loader2 className="h-12 w-12 animate-spin text-zinc-400" />
        </div>
        <p className="text-sm text-zinc-500 animate-pulse">Loading infrastructure...</p>
      </div>
    )
  }

  return (
    <div className="min-h-screen">
      {/* Background Gradient */}
      <div className="fixed inset-0 -z-10 overflow-hidden pointer-events-none">
        <div className="absolute top-0 left-1/4 w-96 h-96 bg-emerald-500/5 rounded-full blur-3xl" />
        <div className="absolute bottom-0 right-1/4 w-96 h-96 bg-violet-500/5 rounded-full blur-3xl" />
      </div>

      <div className="space-y-10 max-w-6xl mx-auto px-4 py-8 animate-in fade-in slide-in-from-bottom-4 duration-700">
        {/* Header */}
        <header className="flex flex-col sm:flex-row sm:items-center justify-between gap-6">
          <div>
            <div className="flex items-center gap-3 mb-2">
              <div className="p-2 rounded-xl bg-gradient-to-br from-zinc-100 to-zinc-200 dark:from-zinc-800 dark:to-zinc-700">
                <Layers className="h-5 w-5 text-zinc-600 dark:text-zinc-400" />
              </div>
              <h1 className="text-3xl font-bold bg-gradient-to-r from-zinc-900 via-zinc-700 to-zinc-600 dark:from-white dark:via-zinc-200 dark:to-zinc-400 bg-clip-text text-transparent">
                Infrastructure
              </h1>
            </div>
            <p className="text-zinc-500 dark:text-zinc-400 max-w-lg">
              Configure and monitor your database and cache settings for optimal performance.
            </p>
          </div>
          <Button
            onClick={handleRefresh}
            variant="outline"
            size="lg"
            className="gap-2.5 rounded-xl border-2 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-all duration-300"
          >
            <RefreshCw className="h-4 w-4" />
            Refresh
          </Button>
        </header>

        {/* Database Section */}
        <section className="animate-in fade-in slide-in-from-bottom-4 duration-700 delay-100">
          <GlassCard hover={false}>
            <div className="p-8">
              <SectionHeader
                icon={Database}
                title="User Database"
                description="Primary storage for user accounts, sessions, and authentication data"
                status={testingConnection ? "loading" : dbStatus as any}
                statusLabel={testingConnection ? "Testing..." : isDBConfigured ? (infraStats?.db === "ok" ? "Connected" : "Configured") : "Not configured"}
                accentColor={isDBConfigured ? "emerald" : "zinc"}
              />

              {isEditingDB ? (
                // Edit Mode
                <div className="space-y-6 mt-8">
                  <div className="grid gap-6 md:grid-cols-2">
                    <div className="space-y-2.5">
                      <Label className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">
                        Database Driver
                      </Label>
                      <Select value={dbForm.driver} onValueChange={(val) => setDbForm({ ...dbForm, driver: val })}>
                        <SelectTrigger className="h-12 rounded-xl border-2 border-zinc-200 dark:border-zinc-700 focus:border-emerald-500 transition-colors">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="postgres">
                            <div className="flex items-center gap-2">
                              <div className="h-4 w-4 rounded bg-blue-500" />
                              <span className="font-medium">PostgreSQL</span>
                            </div>
                          </SelectItem>
                          <SelectItem value="mysql">
                            <div className="flex items-center gap-2">
                              <div className="h-4 w-4 rounded bg-orange-500" />
                              <span className="font-medium">MySQL</span>
                            </div>
                          </SelectItem>
                          <SelectItem value="mongo">
                            <div className="flex items-center gap-2">
                              <div className="h-4 w-4 rounded bg-green-600" />
                              <span className="font-medium">MongoDB</span>
                            </div>
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2.5">
                      <Label className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">
                        Schema Name
                      </Label>
                      <Input
                        placeholder="public"
                        value={dbForm.schema}
                        onChange={(e) => setDbForm({ ...dbForm, schema: e.target.value })}
                        className="h-12 rounded-xl border-2 border-zinc-200 dark:border-zinc-700 focus:border-emerald-500 transition-colors"
                      />
                    </div>
                  </div>

                  <div className="space-y-2.5">
                    <Label className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">
                      Connection String (DSN)
                    </Label>
                    <div className="relative group">
                      <Input
                        type={showDsn ? "text" : "password"}
                        placeholder="postgresql://user:password@host:5432/database"
                        value={dbForm.dsn}
                        onChange={(e) => setDbForm({ ...dbForm, dsn: e.target.value })}
                        className="h-14 rounded-xl border-2 border-zinc-200 dark:border-zinc-700 focus:border-emerald-500 pr-14 font-mono text-sm transition-colors"
                      />
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="absolute right-2 top-1/2 -translate-y-1/2 h-10 w-10 rounded-lg"
                        onClick={() => setShowDsn(!showDsn)}
                      >
                        {showDsn ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                      </Button>
                    </div>
                    <p className="text-xs text-zinc-500 flex items-center gap-1.5">
                      <Shield className="h-3.5 w-3.5" />
                      Your credentials are encrypted before storage
                    </p>
                  </div>

                  <div className="flex items-center justify-end gap-3 pt-4 border-t border-zinc-200 dark:border-zinc-800">
                    <Button
                      variant="ghost"
                      onClick={() => setIsEditingDB(false)}
                      disabled={updateSettingsMutation.isPending}
                      className="rounded-xl"
                    >
                      Cancel
                    </Button>
                    <Button
                      onClick={handleSaveDB}
                      disabled={updateSettingsMutation.isPending || !dbForm.dsn}
                      className="rounded-xl gap-2 bg-emerald-600 hover:bg-emerald-700"
                    >
                      {updateSettingsMutation.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
                      Save Configuration
                    </Button>
                  </div>
                </div>
              ) : (
                // View Mode
                <>
                  {isDBConfigured ? (
                    <>
                      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mt-6">
                        <MetricDisplay
                          icon={Database}
                          label="Engine"
                          value={settings?.userDb?.driver?.toUpperCase() || "N/A"}
                          accentColor="emerald"
                        />
                        <MetricDisplay
                          icon={HardDrive}
                          label="Schema"
                          value={settings?.userDb?.schema || "public"}
                          accentColor="emerald"
                        />
                        {infraStats?.db?.size && (
                          <MetricDisplay
                            icon={Cpu}
                            label="Database Size"
                            value={infraStats.db.size}
                            accentColor="blue"
                          />
                        )}
                        {infraStats?.db?.table_count !== undefined && (
                          <MetricDisplay
                            icon={Layers}
                            label="Tables"
                            value={infraStats.db.table_count}
                            accentColor="violet"
                          />
                        )}
                      </div>

                      <div className="flex flex-wrap items-center justify-end gap-3 mt-8 pt-6 border-t border-zinc-200 dark:border-zinc-800">
                        <ActionButton
                          icon={Zap}
                          onClick={() => testDbMutation.mutate()}
                          loading={testDbMutation.isPending}
                        >
                          Test Connection
                        </ActionButton>
                        <ActionButton
                          icon={Play}
                          onClick={() => runMigrationsMutation.mutate()}
                          loading={runMigrationsMutation.isPending}
                        >
                          Run Migrations
                        </ActionButton>
                        <ActionButton
                          icon={Settings2}
                          onClick={() => setIsEditingDB(true)}
                        >
                          Edit
                        </ActionButton>
                      </div>
                    </>
                  ) : (
                    <EmptyState
                      icon={Database}
                      title="No database configured"
                      description="Connect a database to store user data, sessions, and authentication records securely."
                      action={
                        <Button
                          onClick={() => setIsEditingDB(true)}
                          size="lg"
                          className="gap-2.5 rounded-xl bg-emerald-600 hover:bg-emerald-700 transition-all duration-300 hover:scale-105"
                        >
                          <Sparkles className="h-4 w-4" />
                          Configure Database
                          <ArrowRight className="h-4 w-4" />
                        </Button>
                      }
                    />
                  )}
                </>
              )}
            </div>
          </GlassCard>
        </section>

        {/* Cache Section */}
        <section className="animate-in fade-in slide-in-from-bottom-4 duration-700 delay-200">
          <GlassCard hover={false}>
            <div className="p-8">
              <SectionHeader
                icon={Server}
                title="Cache Layer"
                description="Speed up your application with distributed caching"
                status={isCacheEnabled ? cacheStatus as any : "disconnected"}
                statusLabel={isCacheEnabled ? (infraStats?.cache === "ok" ? "Active" : "Enabled") : "Disabled"}
                accentColor={isCacheEnabled ? "violet" : "zinc"}
                actions={
                  <Switch
                    checked={cacheForm.enabled}
                    onCheckedChange={(checked) => {
                      setCacheForm((prev) => ({ ...prev, enabled: checked }))
                      setIsEditingCache(true)
                    }}
                    className="data-[state=checked]:bg-violet-600"
                  />
                }
              />

              {isEditingCache ? (
                // Edit Mode
                <div className="space-y-6 mt-8">
                  {cacheForm.enabled && (
                    <>
                      <div className="space-y-2.5">
                        <Label className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">
                          Cache Driver
                        </Label>
                        <Select value={cacheForm.driver} onValueChange={(val) => setCacheForm({ ...cacheForm, driver: val })}>
                          <SelectTrigger className="h-12 rounded-xl border-2 border-zinc-200 dark:border-zinc-700 focus:border-violet-500 transition-colors">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="memory">
                              <div className="flex items-center gap-2">
                                <Cpu className="h-4 w-4 text-amber-500" />
                                <span className="font-medium">Memory (Local)</span>
                              </div>
                            </SelectItem>
                            <SelectItem value="redis">
                              <div className="flex items-center gap-2">
                                <Server className="h-4 w-4 text-red-500" />
                                <span className="font-medium">Redis</span>
                              </div>
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      </div>

                      {cacheForm.driver === "redis" && (
                        <>
                          <div className="grid gap-6 sm:grid-cols-2">
                            <div className="space-y-2.5">
                              <Label className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">Host</Label>
                              <Input
                                placeholder="localhost"
                                value={cacheForm.host}
                                onChange={(e) => setCacheForm({ ...cacheForm, host: e.target.value })}
                                className="h-12 rounded-xl border-2 border-zinc-200 dark:border-zinc-700 focus:border-violet-500 transition-colors"
                              />
                            </div>
                            <div className="space-y-2.5">
                              <Label className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">Port</Label>
                              <Input
                                type="number"
                                placeholder="6379"
                                value={cacheForm.port}
                                onChange={(e) => setCacheForm({ ...cacheForm, port: parseInt(e.target.value) || 6379 })}
                                className="h-12 rounded-xl border-2 border-zinc-200 dark:border-zinc-700 focus:border-violet-500 transition-colors"
                              />
                            </div>
                          </div>
                          <div className="space-y-2.5">
                            <Label className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">Password (Optional)</Label>
                            <Input
                              type="password"
                              placeholder="•••••••••"
                              value={cacheForm.password}
                              onChange={(e) => setCacheForm({ ...cacheForm, password: e.target.value })}
                              className="h-12 rounded-xl border-2 border-zinc-200 dark:border-zinc-700 focus:border-violet-500 transition-colors"
                            />
                          </div>
                          <div className="grid gap-6 sm:grid-cols-2">
                            <div className="space-y-2.5">
                              <Label className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">Database Index</Label>
                              <Input
                                type="number"
                                placeholder="0"
                                value={cacheForm.db}
                                onChange={(e) => setCacheForm({ ...cacheForm, db: parseInt(e.target.value) || 0 })}
                                className="h-12 rounded-xl border-2 border-zinc-200 dark:border-zinc-700 focus:border-violet-500 transition-colors"
                              />
                            </div>
                            <div className="space-y-2.5">
                              <Label className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">Key Prefix</Label>
                              <Input
                                placeholder="tenant:"
                                value={cacheForm.prefix}
                                onChange={(e) => setCacheForm({ ...cacheForm, prefix: e.target.value })}
                                className="h-12 rounded-xl border-2 border-zinc-200 dark:border-zinc-700 focus:border-violet-500 transition-colors"
                              />
                            </div>
                          </div>
                        </>
                      )}

                      {cacheForm.driver === "memory" && (
                        <div className="p-5 rounded-2xl bg-gradient-to-br from-amber-50 to-orange-50 dark:from-amber-900/20 dark:to-orange-900/20 border border-amber-200 dark:border-amber-800/50">
                          <div className="flex gap-4">
                            <div className="p-2.5 rounded-xl bg-amber-100 dark:bg-amber-900/50 h-fit">
                              <AlertCircle className="h-5 w-5 text-amber-600 dark:text-amber-400" />
                            </div>
                            <div>
                              <p className="text-sm font-semibold text-amber-800 dark:text-amber-200">
                                Memory cache limitations
                              </p>
                              <p className="text-sm text-amber-700 dark:text-amber-300/80 mt-1">
                                In-memory cache is not shared across instances and will be cleared on restart.
                                For production workloads, consider using Redis.
                              </p>
                            </div>
                          </div>
                        </div>
                      )}
                    </>
                  )}

                  {!cacheForm.enabled && (
                    <div className="py-12 text-center">
                      <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-zinc-100 dark:bg-zinc-800 mb-4">
                        <Server className="h-8 w-8 text-zinc-400" />
                      </div>
                      <p className="text-zinc-500">Enable caching to improve performance</p>
                    </div>
                  )}

                  <div className="flex items-center justify-end gap-3 pt-4 border-t border-zinc-200 dark:border-zinc-800">
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
                      className="rounded-xl"
                    >
                      Cancel
                    </Button>
                    <Button
                      onClick={handleSaveCache}
                      disabled={updateSettingsMutation.isPending}
                      className="rounded-xl gap-2 bg-violet-600 hover:bg-violet-700"
                    >
                      {updateSettingsMutation.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
                      Save Configuration
                    </Button>
                  </div>
                </div>
              ) : (
                // View Mode
                <>
                  {isCacheEnabled ? (
                    <>
                      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mt-6">
                        <MetricDisplay
                          icon={Server}
                          label="Driver"
                          value={settings?.cache?.driver === "memory" ? "Memory" : "Redis"}
                          accentColor="violet"
                        />
                        {settings?.cache?.driver === "redis" && (
                          <MetricDisplay
                            icon={ExternalLink}
                            label="Endpoint"
                            value={`${settings?.cache?.host || "localhost"}:${settings?.cache?.port || 6379}`}
                            accentColor="violet"
                          />
                        )}
                        {infraStats?.cache?.keys !== undefined && (
                          <MetricDisplay
                            icon={Activity}
                            label="Cached Keys"
                            value={infraStats.cache.keys.toLocaleString()}
                            accentColor="blue"
                          />
                        )}
                        {infraStats?.cache?.used_memory && (
                          <MetricDisplay
                            icon={Cpu}
                            label="Memory Usage"
                            value={infraStats.cache.used_memory}
                            accentColor="emerald"
                          />
                        )}
                      </div>

                      <div className="flex flex-wrap items-center justify-end gap-3 mt-8 pt-6 border-t border-zinc-200 dark:border-zinc-800">
                        <ActionButton
                          icon={Zap}
                          onClick={() => testCacheMutation.mutate()}
                          loading={testCacheMutation.isPending}
                        >
                          Test Connection
                        </ActionButton>
                        <ActionButton
                          icon={Settings2}
                          onClick={() => setIsEditingCache(true)}
                        >
                          Edit
                        </ActionButton>
                      </div>
                    </>
                  ) : (
                    <EmptyState
                      icon={Server}
                      title="Cache is disabled"
                      description="Enable caching to dramatically improve response times and reduce database load."
                    />
                  )}
                </>
              )}
            </div>
          </GlassCard>
        </section>
      </div>
    </div>
  )
}
