"use client"

import { useState, useEffect } from "react"
import { useSearchParams } from "next/navigation"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  CardFooter,
} from "@/components/ui/card"
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
  Server,
  Play,
  Edit2,
  X,
  Power,
  Eye,
  EyeOff,
  Plus,
  Trash2,
  Settings2,
} from "lucide-react"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

import { Textarea } from "@/components/ui/textarea"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"


interface UserDBSettings {
  driver: string
  dsn: string
  schema: string
  dsnEnc?: string // To check if configured
}

interface CacheSettings {
  enabled: boolean
  driver: string
  host: string
  port: number
  password?: string
  db: number
  prefix: string
  passEnc?: string // To check if configured
}

interface UserFieldDefinition {
  name: string
  type: string // text, int, boolean, date
  required?: boolean
  unique?: boolean
  indexed?: boolean
  description?: string
}

interface TenantSettings {
  userDb?: UserDBSettings
  cache?: CacheSettings
  user_fields?: UserFieldDefinition[]
}

export default function DatabasePage() {
  const searchParams = useSearchParams()
  const tenantId = searchParams.get("id")
  const { token } = useAuthStore()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const { t } = useI18n()

  // --- State ---
  const [isEditingDB, setIsEditingDB] = useState(false)
  const [isEditingCache, setIsEditingCache] = useState(false)
  const [showDsn, setShowDsn] = useState(false)
  const [etag, setEtag] = useState<string>("")

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

  // Fields State
  const [isFieldsDialogOpen, setIsFieldsDialogOpen] = useState(false)
  const [editingFieldIdx, setEditingFieldIdx] = useState<number | null>(null)
  const [fieldForm, setFieldForm] = useState<UserFieldDefinition>({
    name: "",
    type: "text",
    required: false,
    unique: false,
    indexed: false,
    description: "",
  })
  const [localFields, setLocalFields] = useState<UserFieldDefinition[]>([])
  const [hasFieldChanges, setHasFieldChanges] = useState(false)

  // --- Queries ---
  // ... (existing queries)


  const {
    data: settings,
    isLoading,
    isError,
    error,
  } = useQuery<TenantSettings>({
    queryKey: ["tenant-storage", tenantId],
    queryFn: async () => {
      if (!tenantId || !token) throw new Error("No tenant ID or token")
      const { data, headers } = await api.getWithHeaders<TenantSettings>(`/v1/admin/tenants/${tenantId}/settings`)
      const etagHeader = headers.get("ETag")
      if (etagHeader) {
        setEtag(etagHeader)
      }
      return data
    },
    enabled: !!tenantId && !!token,
  })

  const { data: infraStats } = useQuery<any>({
    queryKey: ["tenant-infra-stats", tenantId],
    queryFn: async () => {
      if (!tenantId || !token) return null
      try {
        const { data } = await api.get<any>(`/v1/admin/tenants/${tenantId}/infra-stats`)
        return data || null
      } catch (e) {
        return null
      }
    },
    enabled: !!tenantId && !!token,
    refetchInterval: 10000,
  })

  // --- Effects ---
  useEffect(() => {
    if (settings) {
      // DB Init
      if (settings.userDb) {
        setDbForm({
          driver: settings.userDb.driver || "postgres",
          dsn: "", // Always empty for security
          schema: settings.userDb.schema || "",
        })

        // Logic: If dsnEnc or dsn is present, it is configured.
        if (settings.userDb.dsnEnc || settings.userDb.dsn) {
          setIsEditingDB(false)
        } else {
          // If not configured, default to edit mode
          setIsEditingDB(true)
        }
      } else {
        setIsEditingDB(true)
      }

      // Cache Init
      if (settings.cache) {
        setCacheForm({
          enabled: settings.cache.enabled || false,
          driver: settings.cache.driver || "memory",
          host: settings.cache.host || "",
          port: settings.cache.port || 6379,
          password: "", // Always empty
          db: settings.cache.db || 0,
          prefix: settings.cache.prefix || "",
        })
        setIsEditingCache(false)
      } else {
        setCacheForm((prev) => ({ ...prev, enabled: false }))
        setIsEditingCache(false)
      }
      // Fields Init
      setLocalFields(settings.user_fields || [])
      setHasFieldChanges(false)
    }
  }, [settings])

  // --- Mutations ---
  const updateSettingsMutation = useMutation({
    mutationFn: async (data: TenantSettings) => {
      if (!tenantId || !token) throw new Error("No tenant ID")
      const currentSettings = settings || {}
      const payload = {
        ...currentSettings,
        ...data,
      }
      await api.put(`/v1/admin/tenants/${tenantId}/settings`, payload, etag)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tenant-storage", tenantId] })
      toast({
        title: t("common.success"),
        description: t("tenants.settingsUpdatedDesc"),
      })
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
      await api.post(
        `/v1/admin/tenants/${tenantId}/user-store/test-connection`,
        {}
      )
    },
    onSuccess: () => {
      toast({
        variant: "success",
        title: t("database.connectionSuccess"),
        description: t("database.testConnectionDesc"),
      })
    },
    onError: (err: any) => {
      toast({
        variant: "destructive",
        title: t("common.error"),
        description: err.response?.data?.error_description || err.message,
      })
    },
  })

  const runMigrationsMutation = useMutation({
    mutationFn: async () => {
      if (!tenantId || !token) throw new Error("No tenant ID")
      return api.post<{ applied_count: number }>(
        `/v1/admin/tenants/${tenantId}/user-store/migrate`,
        {}
      )
    },
    onSuccess: (data) => {
      toast({
        title: t("database.migrationsApplied"),
        description: t("database.migrationsAppliedDesc", {
          count: data.applied_count || 0,
        }),
      })
    },
    onError: (err: any) => {
      toast({
        variant: "destructive",
        title: t("common.error"),
        description: err.response?.data?.error_description || err.message,
      })
    },
  })

  const testCacheMutation = useMutation({
    mutationFn: async () => {
      if (!tenantId || !token) throw new Error("No tenant ID")
      await api.post(
        `/v1/admin/tenants/${tenantId}/cache/test-connection`,
        {}
      )
    },
    onSuccess: () => {
      toast({
        title: t("database.connectionSuccess"),
        description: "Cache connection verified.",
      })
    },
    onError: (err: any) => {
      toast({
        variant: "destructive",
        title: t("common.error"),
        description: err.response?.data?.error_description || err.message,
      })
    },
  })

  // --- Handlers ---

  const handleSaveDB = () => {
    updateSettingsMutation.mutate({
      userDb: dbForm,
    })
  }

  const handleSaveCache = () => {
    // Clean up cache form based on driver to avoid residual data
    const cleanCacheForm = { ...cacheForm }
    if (cleanCacheForm.driver === "memory") {
      cleanCacheForm.host = ""
      cleanCacheForm.port = 0
      cleanCacheForm.password = ""
      cleanCacheForm.db = 0
      cleanCacheForm.prefix = ""
    }

    // If driver is memory, ensure enabled is true if it was set to true
    // Logic: if user toggled enabled=true, and driver=memory, we save enabled=true.
    // However, if the user toggled enabled=false, we save enabled=false.

    // We trust cacheForm.enabled state which is updated by the switch.

    updateSettingsMutation.mutate({
      cache: cleanCacheForm,
    })
  }

  const handleCacheToggle = (checked: boolean) => {
    setCacheForm((prev) => ({ ...prev, enabled: checked }))
    setIsEditingCache(true)
  }

  const handleCancelCache = () => {
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
    } else {
      setCacheForm((prev) => ({ ...prev, enabled: false }))
    }
    setIsEditingCache(false)
  }

  // --- Fields Handlers ---
  const handleOpenFieldDialog = (field?: UserFieldDefinition, idx?: number) => {
    if (field) {
      setFieldForm(field)
      setEditingFieldIdx(idx ?? null)
    } else {
      setFieldForm({
        name: "",
        type: "text",
        required: false,
        unique: false,
        indexed: false,
        description: "",
      })
      setEditingFieldIdx(null)
    }
    setIsFieldsDialogOpen(true)
  }

  const handleLocalSaveField = () => {
    // Validate
    if (!fieldForm.name || !fieldForm.type) {
      toast({
        variant: "destructive",
        title: t("common.error"),
        description: "Name and Type are required",
      })
      return
    }

    const nameExists = localFields.some((f, i) =>
      f.name.toLowerCase() === fieldForm.name.toLowerCase() && i !== editingFieldIdx
    )

    if (nameExists) {
      toast({
        variant: "destructive",
        title: t("common.error"),
        description: "Field name already exists",
      })
      return
    }

    let newFields = [...localFields]
    if (editingFieldIdx !== null) {
      // Edit
      newFields[editingFieldIdx] = fieldForm
    } else {
      // Add
      newFields.push(fieldForm)
    }

    setLocalFields(newFields)
    setHasFieldChanges(true)
    setIsFieldsDialogOpen(false)
  }

  const handleLocalDeleteField = (idx: number) => {
    const newFields = localFields.filter((_, i) => i !== idx)
    setLocalFields(newFields)
    setHasFieldChanges(true)
  }

  const handleSaveAllFields = () => {
    updateSettingsMutation.mutate({
      user_fields: localFields,
    })
    setHasFieldChanges(false)
  }

  const handleCancelFieldChanges = () => {
    setLocalFields(settings?.user_fields || [])
    setHasFieldChanges(false)
  }



  if (isError) {
    return (
      <Alert variant="destructive">
        <AlertCircle className="h-4 w-4" />
        <AlertTitle>{t("common.error")}</AlertTitle>
        <AlertDescription>
          {t("tenants.notFound")} - {(error as any)?.message || JSON.stringify(error)}
        </AlertDescription>
      </Alert>
    )
  }

  const isDBConfigured = !!settings?.userDb?.dsnEnc || !!settings?.userDb?.dsn
  const isCacheConfigured = !!settings?.cache?.enabled

  return (
    <div className="space-y-6 animate-in fade-in duration-500">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">
            {t("database.title")}
          </h2>
          <p className="text-muted-foreground">{t("database.description")}</p>
        </div>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        {/* DATABASE CARD */}
        <Card className={`flex flex-col h-full transition-all duration-200 ${isDBConfigured ? "border-l-4 border-l-green-500" : ""}`}>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-2">
                <div className="p-2 bg-blue-500/10 rounded-lg">
                  <Database className="h-5 w-5 text-blue-600" />
                </div>
                <CardTitle>{t("database.title")}</CardTitle>
              </div>
              {isDBConfigured && !isEditingDB && (
                <Badge variant="secondary" className="bg-blue-50 text-blue-700 hover:bg-blue-100 border-blue-200">
                  <CheckCircle2 className="mr-1 h-3 w-3" />
                  {t("database.configured")}
                </Badge>
              )}
            </div>
            <CardDescription>{t("database.connectionDesc")}</CardDescription>
            <div className="mt-4 flex items-center gap-2 text-sm">
              <div className={`h-2 w-2 rounded-full ${isDBConfigured ? "bg-green-500" : "bg-gray-300"}`} />
              <span className="text-muted-foreground">
                {isDBConfigured ? t("database.statusConnected") : t("database.statusNotConfigured")}
              </span>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            {isEditingDB ? (
              // EDIT MODE
              <div className="space-y-4 animate-in slide-in-from-top-2 duration-300">
                <div className="space-y-2">
                  <Label>{t("database.driver")}</Label>
                  <Select
                    value={dbForm.driver}
                    onValueChange={(val) =>
                      setDbForm({ ...dbForm, driver: val })
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="postgres">PostgreSQL</SelectItem>
                      <SelectItem value="mysql">MySQL</SelectItem>
                      <SelectItem value="mongo">MongoDB</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-2">
                  <Label>{t("database.dsn")}</Label>
                  <div className="relative">
                    <Input
                      type={showDsn ? "text" : "password"}
                      placeholder={t("database.dsnPlaceholder")}
                      value={dbForm.dsn}
                      onChange={(e) =>
                        setDbForm({ ...dbForm, dsn: e.target.value })
                      }
                      className="pr-10"
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="absolute right-0 top-0 h-full px-3 py-2 hover:bg-transparent"
                      onClick={() => setShowDsn(!showDsn)}
                    >
                      {showDsn ? (
                        <EyeOff className="h-4 w-4 text-muted-foreground" />
                      ) : (
                        <Eye className="h-4 w-4 text-muted-foreground" />
                      )}
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {t("database.dsnHint")}
                  </p>
                </div>

                <div className="space-y-2">
                  <Label>{t("database.schema")}</Label>
                  <Input
                    placeholder="public"
                    value={dbForm.schema}
                    onChange={(e) =>
                      setDbForm({ ...dbForm, schema: e.target.value })
                    }
                  />
                  <p className="text-xs text-muted-foreground">
                    {t("database.schemaHint")}
                  </p>
                </div>
              </div>
            ) : (
              // VIEW MODE
              <div className="space-y-4 py-2">
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div className="p-3 rounded-lg bg-muted/50">
                    <span className="font-medium text-muted-foreground block mb-1 text-xs uppercase tracking-wider">
                      {t("database.driver")}
                    </span>
                    <p className="font-medium capitalize">{settings?.userDb?.driver}</p>
                  </div>
                  <div className="p-3 rounded-lg bg-muted/50">
                    <span className="font-medium text-muted-foreground block mb-1 text-xs uppercase tracking-wider">
                      {t("database.schema")}
                    </span>
                    <p className="font-medium">{settings?.userDb?.schema || "public"}</p>
                  </div>
                  {infraStats?.db && (
                    <>
                      <div className="p-3 rounded-lg bg-muted/50">
                        <span className="font-medium text-muted-foreground block mb-1 text-xs uppercase tracking-wider">
                          Tamaño
                        </span>
                        <p className="font-medium">{infraStats.db.size}</p>
                      </div>
                      <div className="p-3 rounded-lg bg-muted/50">
                        <span className="font-medium text-muted-foreground block mb-1 text-xs uppercase tracking-wider">
                          Tablas
                        </span>
                        <p className="font-medium">{infraStats.db.table_count}</p>
                      </div>
                    </>
                  )}
                </div>
              </div>
            )}
          </CardContent>
          <CardFooter className="mt-auto flex items-center justify-end gap-2 px-4 py-4">
            {isEditingDB ? (
              <>
                <Button
                  variant="ghost"
                  onClick={() => setIsEditingDB(false)}
                  disabled={updateSettingsMutation.isPending}
                >
                  {t("common.cancel")}
                </Button>
                <Button
                  onClick={handleSaveDB}
                  disabled={updateSettingsMutation.isPending}
                >
                  {updateSettingsMutation.isPending
                    ? t("common.saving")
                    : t("database.saveChanges")}
                </Button>
              </>
            ) : (
              <>
                <div className="flex items-center gap-2 w-full justify-end" >
                  {isDBConfigured && (
                    <>
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => runMigrationsMutation.mutate()}
                        disabled={runMigrationsMutation.isPending}
                      >
                        <Play className="mr-2 h-4 w-4" />
                        {runMigrationsMutation.isPending
                          ? t("database.running")
                          : t("database.runMigrations")}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => testDbMutation.mutate()}
                        disabled={testDbMutation.isPending}
                      >
                        {testDbMutation.isPending
                          ? t("database.testing")
                          : t("database.testConnection")}
                      </Button>
                    </>
                  )}
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setIsEditingDB(true)}
                  >
                    <Edit2 className="mr-2 h-4 w-4" />
                    {t("common.edit")}
                  </Button>
                </div>
              </>
            )}
          </CardFooter>
        </Card>

        {/* CACHE CARD */}
        <Card className={`flex flex-col h-full transition-all duration-200 ${cacheForm.enabled ? "border-l-4 border-l-red-500" : ""}`}>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-2">
                <div className={`p-2 rounded-lg ${cacheForm.enabled ? "bg-green-500/10" : "bg-muted"}`}>
                  <Server
                    className={`h-5 w-5 ${cacheForm.enabled ? "text-green-600" : "text-muted-foreground"}`}
                  />
                </div>
                <CardTitle>{t("database.cacheTitle")}</CardTitle>
              </div>
              <div className="flex items-center space-x-2">
                <Label htmlFor="cache-mode" className="text-sm font-normal text-muted-foreground">
                  {cacheForm.enabled
                    ? t("common.enabled")
                    : t("common.disabled")}
                </Label>
                <Switch
                  id="cache-mode"
                  checked={cacheForm.enabled}
                  onCheckedChange={handleCacheToggle}
                />
              </div>
            </div>
            <CardDescription>{t("database.cacheDesc")}</CardDescription>
            <div className="mt-4 flex items-center gap-2 text-sm">
              <div className={`h-2 w-2 rounded-full ${cacheForm.enabled ? "bg-green-500" : "bg-gray-300"}`} />
              <span className="text-muted-foreground">
                {cacheForm.enabled ? t("database.statusActive") : t("database.statusInactive")}
              </span>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            {isEditingCache ? (
              // EDIT MODE
              <div className="space-y-4 animate-in slide-in-from-top-2 duration-300">
                {cacheForm.enabled && (
                  <>
                    <div className="space-y-2">
                      <Label>{t("database.cacheDriver")}</Label>
                      <Select
                        value={cacheForm.driver}
                        onValueChange={(val) =>
                          setCacheForm({ ...cacheForm, driver: val })
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="memory">Memory (Local)</SelectItem>
                          <SelectItem value="redis">Redis</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>

                    {cacheForm.driver === "redis" && (
                      <>
                        <div className="grid grid-cols-2 gap-4">
                          <div className="space-y-2">
                            <Label>{t("database.host")}</Label>
                            <Input
                              placeholder="localhost"
                              value={cacheForm.host}
                              onChange={(e) =>
                                setCacheForm({ ...cacheForm, host: e.target.value })
                              }
                            />
                          </div>
                          <div className="space-y-2">
                            <Label>{t("database.port")}</Label>
                            <Input
                              type="number"
                              placeholder="6379"
                              value={cacheForm.port}
                              onChange={(e) =>
                                setCacheForm({
                                  ...cacheForm,
                                  port: parseInt(e.target.value) || 6379,
                                })
                              }
                            />
                          </div>
                        </div>
                        <div className="space-y-2">
                          <Label>{t("database.password")}</Label>
                          <Input
                            type="password"
                            placeholder="******"
                            value={cacheForm.password}
                            onChange={(e) =>
                              setCacheForm({
                                ...cacheForm,
                                password: e.target.value,
                              })
                            }
                          />
                        </div>
                        <div className="grid grid-cols-2 gap-4">
                          <div className="space-y-2">
                            <Label>{t("database.dbIndex")}</Label>
                            <Input
                              type="number"
                              placeholder="0"
                              value={cacheForm.db}
                              onChange={(e) =>
                                setCacheForm({
                                  ...cacheForm,
                                  db: parseInt(e.target.value) || 0,
                                })
                              }
                            />
                          </div>
                          <div className="space-y-2">
                            <Label>{t("database.prefix")}</Label>
                            <Input
                              placeholder="tenant:"
                              value={cacheForm.prefix}
                              onChange={(e) =>
                                setCacheForm({
                                  ...cacheForm,
                                  prefix: e.target.value,
                                })
                              }
                            />
                          </div>
                        </div>
                      </>
                    )}
                  </>
                )}
              </div>
            ) : (
              // VIEW MODE
              <div className="space-y-4 py-2">
                {cacheForm.enabled ? (
                  <div className="grid grid-cols-2 gap-4 text-sm">
                    <div className="p-3 rounded-lg bg-muted/50">
                      <span className="font-medium text-muted-foreground block mb-1 text-xs uppercase tracking-wider">
                        {t("database.cacheDriver")}
                      </span>
                      <p className="capitalize font-medium">{settings?.cache?.driver}</p>
                    </div>
                    {settings?.cache?.driver === "redis" && (
                      <div className="p-3 rounded-lg bg-muted/50">
                        <span className="font-medium text-muted-foreground block mb-1 text-xs uppercase tracking-wider">
                          {t("database.host")}
                        </span>
                        <p className="font-medium">
                          {settings?.cache?.host}:{settings?.cache?.port}
                        </p>
                      </div>
                    )}
                    {infraStats?.cache && (
                      <>
                        <div className="p-3 rounded-lg bg-muted/50">
                          <span className="font-medium text-muted-foreground block mb-1 text-xs uppercase tracking-wider">
                            Keys
                          </span>
                          <p className="font-medium">{infraStats.cache.keys}</p>
                        </div>
                        {infraStats.cache.used_memory && (
                          <div className="p-3 rounded-lg bg-muted/50">
                            <span className="font-medium text-muted-foreground block mb-1 text-xs uppercase tracking-wider">
                              Memoria
                            </span>
                            <p className="font-medium">{infraStats.cache.used_memory}</p>
                          </div>
                        )}
                      </>
                    )}
                  </div>
                ) : (
                  <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
                    <div className="p-4 rounded-full bg-muted mb-3">
                      <Power className="h-6 w-6 opacity-40" />
                    </div>
                    <p className="text-sm">{t("database.cacheDesc")}</p>
                  </div>
                )}
              </div>
            )}
          </CardContent>
          <CardFooter className="mt-auto flex items-center justify-end gap-2 px-4 py-4">
            {isEditingCache ? (
              <>
                <Button
                  variant="ghost"
                  onClick={handleCancelCache}
                  disabled={updateSettingsMutation.isPending}
                >
                  {t("common.cancel")}
                </Button>
                <Button
                  onClick={handleSaveCache}
                  disabled={updateSettingsMutation.isPending}
                >
                  {updateSettingsMutation.isPending
                    ? t("common.saving")
                    : t("database.saveChanges")}
                </Button>
              </>
            ) : (
              <>
                {isCacheConfigured && (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => testCacheMutation.mutate()}
                    disabled={testCacheMutation.isPending}
                  >
                    {testCacheMutation.isPending
                      ? t("database.testing")
                      : t("database.testCache")}
                  </Button>
                )}
                {cacheForm.enabled && (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setIsEditingCache(true)}
                  >
                    <Edit2 className="mr-2 h-4 w-4" />
                    {t("common.edit")}
                  </Button>
                )}
              </>
            )}
          </CardFooter>
        </Card>
      </div>

      {/* USER FIELDS CARD */}
      <Card className="flex flex-col h-full">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <div className="p-2 bg-purple-500/10 rounded-lg">
                <Settings2 className="h-5 w-5 text-purple-600" />
              </div>
              <CardTitle>Campos de Usuario</CardTitle>
            </div>
            <Button size="sm" onClick={() => handleOpenFieldDialog()}>
              <Plus className="mr-2 h-4 w-4" />
              Agregar Campo
            </Button>
          </div>
          <CardDescription>
            Define campos personalizados para el perfil de usuario. Estos se agregarán como columnas en la base de datos.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Nombre</TableHead>
                  <TableHead>Tipo</TableHead>
                  <TableHead className="text-center">Requerido</TableHead>
                  <TableHead className="text-center">Único</TableHead>
                  <TableHead className="text-center">Indexado</TableHead>
                  <TableHead className="text-right">Acciones</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(!localFields || localFields.length === 0) ? (
                  <TableRow>
                    <TableCell colSpan={6} className="h-24 text-center">
                      No hay campos personalizados definidos.
                    </TableCell>
                  </TableRow>
                ) : (
                  localFields.map((field, idx) => (
                    <TableRow key={idx}>
                      <TableCell className="font-medium">{field.name}</TableCell>
                      <TableCell>
                        <Badge variant="outline">{field.type}</Badge>
                      </TableCell>
                      <TableCell className="text-center">
                        {field.required && <CheckCircle2 className="h-4 w-4 mx-auto text-green-500" />}
                      </TableCell>
                      <TableCell className="text-center">
                        {field.unique && <CheckCircle2 className="h-4 w-4 mx-auto text-blue-500" />}
                      </TableCell>
                      <TableCell className="text-center">
                        {field.indexed && <CheckCircle2 className="h-4 w-4 mx-auto text-gray-500" />}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-2">
                          <Button variant="ghost" size="icon" onClick={() => handleOpenFieldDialog(field, idx)}>
                            <Edit2 className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" size="icon" className="text-red-500 hover:text-red-600" onClick={() => handleLocalDeleteField(idx)}>
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
        <CardFooter className="mt-auto flex items-center justify-end gap-2 px-4 py-4">
          {hasFieldChanges && (
            <>
              <Button variant="ghost" onClick={handleCancelFieldChanges}>
                Cancelar
              </Button>
              <Button onClick={handleSaveAllFields} disabled={updateSettingsMutation.isPending}>
                {updateSettingsMutation.isPending ? "Guardando..." : "Guardar Cambios"}
              </Button>
            </>
          )}
        </CardFooter>
      </Card>

      {/* FIELD DIALOG */}
      <Dialog open={isFieldsDialogOpen} onOpenChange={setIsFieldsDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingFieldIdx !== null ? "Editar Campo" : "Agregar Campo"}</DialogTitle>
            <DialogDescription>
              Configura las propiedades del campo. Los cambios se aplicarán al guardar la lista completa.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Nombre</Label>
                <Input
                  placeholder="ej. telefono"
                  value={fieldForm.name}
                  onChange={(e) => setFieldForm({ ...fieldForm, name: e.target.value })}
                  disabled={editingFieldIdx !== null} // Prevent renaming for now
                />
                {editingFieldIdx !== null && <p className="text-xs text-muted-foreground">El nombre no se puede cambiar una vez creado.</p>}
              </div>
              <div className="space-y-2">
                <Label>Tipo</Label>
                <Select
                  value={fieldForm.type}
                  onValueChange={(val) => setFieldForm({ ...fieldForm, type: val })}
                  disabled={editingFieldIdx !== null} // Prevent type change for now
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="text">Texto</SelectItem>
                    <SelectItem value="int">Número Entero</SelectItem>
                    <SelectItem value="boolean">Booleano</SelectItem>
                    <SelectItem value="date">Fecha/Hora</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="space-y-2">
              <Label>Descripción</Label>
              <Textarea
                placeholder="Descripción opcional..."
                value={fieldForm.description || ""}
                onChange={(e) => setFieldForm({ ...fieldForm, description: e.target.value })}
              />
            </div>
            <div className="flex flex-col gap-4 border p-4 rounded-lg">
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label>Requerido (Not Null)</Label>
                  <p className="text-xs text-muted-foreground">Si se activa en una tabla con datos, fallará si hay nulos.</p>
                </div>
                <Switch
                  checked={fieldForm.required}
                  onCheckedChange={(c) => setFieldForm({ ...fieldForm, required: c })}
                />
              </div>
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label>Único (Unique)</Label>
                  <p className="text-xs text-muted-foreground">No permitir valores duplicados.</p>
                </div>
                <Switch
                  checked={fieldForm.unique}
                  onCheckedChange={(c) => setFieldForm({ ...fieldForm, unique: c })}
                />
              </div>
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label>Indexado</Label>
                  <p className="text-xs text-muted-foreground">Mejora la velocidad de búsqueda.</p>
                </div>
                <Switch
                  checked={fieldForm.indexed}
                  onCheckedChange={(c) => setFieldForm({ ...fieldForm, indexed: c })}
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsFieldsDialogOpen(false)}>Cancelar</Button>
            <Button onClick={handleLocalSaveField}>
              Agregar a Lista
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>


    </div>
  )
}
