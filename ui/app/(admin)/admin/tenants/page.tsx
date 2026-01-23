"use client"

import { useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Search, Settings, Trash2, Eye, MoreHorizontal, Check, X, Database, Mail, Globe, ArrowRight } from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
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
import { Label } from "@/components/ui/label"
import { useToast } from "@/hooks/use-toast"
import { Badge } from "@/components/ui/badge"
import Link from "next/link"
import { useRouter } from "next/navigation"
import type { Tenant } from "@/lib/types"

import { Switch } from "@/components/ui/switch"
import { SimpleTooltip } from "@/components/ui/simple-tooltip"
import { cn } from "@/lib/utils"
import { LogoDropzone } from "@/components/tenant/LogoDropzone"

export default function TenantsPage() {
  const { t } = useI18n()
  const { toast } = useToast()
  const router = useRouter()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState("")
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [selectedTenant, setSelectedTenant] = useState<Tenant | null>(null)
  const [currentStep, setCurrentStep] = useState(0)

  const [enableSMTP, setEnableSMTP] = useState(false)
  const [enableUserDB, setEnableUserDB] = useState(false)
  const [enableSocial, setEnableSocial] = useState(false)
  const [enableCache, setEnableCache] = useState(false)
  const [newTenant, setNewTenant] = useState<{
    name: string
    slug: string
    display_name: string
    settings?: any
  }>({
    name: "",
    slug: "",
    display_name: "",
    settings: {},
  })

  // Connection Test State
  const [connectionStatus, setConnectionStatus] = useState<"idle" | "testing" | "success" | "error">("idle")
  const [connectionMessage, setConnectionMessage] = useState("")

  const { data: tenants, isLoading } = useQuery({
    queryKey: ["tenants"],
    queryFn: () => api.get<Tenant[]>("/v2/admin/tenants"),
  })

  const createMutation = useMutation({
    mutationFn: (data: typeof newTenant) => api.post<Tenant>("/v2/admin/tenants", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tenants"] })
      setCreateDialogOpen(false)
      setNewTenant({ name: "", slug: "", display_name: "", settings: {} })
      setEnableSMTP(false)
      setEnableUserDB(false)
      setEnableSocial(false)
      setEnableCache(false)
      toast({
        title: t("tenants.created"),
        description: t("tenants.createdDesc"),
      })
    },
    onError: (error: any) => {
      toast({
        title: t("common.error"),
        description: error.message,
        variant: "destructive",
      })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (slug: string) => api.delete(`/v2/admin/tenants/${slug}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tenants"] })
      setDeleteDialogOpen(false)
      setSelectedTenant(null)
      toast({
        title: t("tenants.deleted"),
        description: t("tenants.deletedDesc"),
      })
    },
    onError: (error: any) => {
      toast({
        title: t("common.error"),
        description: error.message,
        variant: "destructive",
      })
    },
  })

  const testConnectionMutation = useMutation({
    mutationFn: (dsn: string) => api.post("/v2/admin/tenants/test-connection", { dsn }),
    onSuccess: () => {
      setConnectionStatus("success")
      setConnectionMessage("Conexión exitosa")
    },
    onError: (error: any) => {
      setConnectionStatus("error")
      setConnectionMessage(error.message || "Error al conectar")
    },
  })

  const filteredTenants = tenants?.filter(
    (tenant) =>
      tenant.name.toLowerCase().includes(search.toLowerCase()) ||
      tenant.slug.toLowerCase().includes(search.toLowerCase()),
  )

  const handleCreate = () => {
    if (!newTenant.name || !newTenant.slug) {
      toast({
        title: t("common.error"),
        description: t("tenants.fillRequired"),
        variant: "destructive",
      })
      return
    }

    const slugRegex = /^[a-z0-9\-]+$/
    if (!slugRegex.test(newTenant.slug)) {
      toast({
        title: t("common.error"),
        description: "El slug solo puede contener letras minúsculas, números y guiones.",
        variant: "destructive",
      })
      return
    }

    // Prepare payload with correct backend structure
    const payload = {
      ...newTenant,
      settings: {
        ...newTenant.settings,
        // Map toggles
        social_login_enabled: enableSocial,

        // Clear disabled sections
        userDb: enableUserDB ? newTenant.settings?.userDb : undefined,
        cache: enableCache ? newTenant.settings?.cache : undefined,
        smtp: enableSMTP ? newTenant.settings?.smtp : undefined,

        // Map Social Providers (Frontend Nested -> Backend Flat)
        socialProviders: enableSocial ? {
          googleEnabled: !!newTenant.settings?.socialProviders?.google?.clientId,
          googleClient: newTenant.settings?.socialProviders?.google?.clientId,
          googleSecretEnc: newTenant.settings?.socialProviders?.google?.clientSecret,
        } : undefined
      }
    }

    createMutation.mutate(payload as any)
  }

  const handleDelete = () => {
    if (selectedTenant) {
      deleteMutation.mutate(selectedTenant.slug)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t("tenants.title")}</h1>
          <p className="text-muted-foreground">{t("tenants.description")}</p>
        </div>
        <Button onClick={() => setCreateDialogOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t("tenants.create")}
        </Button>
      </div>

      <Card>
        <div className="p-4 border-b flex items-center justify-between bg-muted/30">
          <div className="relative max-w-sm w-full">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder={t("tenants.search")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9 bg-background"
            />
          </div>
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <div className="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent" />
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="pl-6">{t("tenants.name")}</TableHead>
                <TableHead>{t("tenants.slug")}</TableHead>
                <TableHead className="text-right pr-6">{t("common.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredTenants?.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={3} className="text-center py-8 text-muted-foreground">
                    {t("tenants.noTenants")}
                  </TableCell>
                </TableRow>
              ) : (
                filteredTenants?.map((tenant) => (
                  <TableRow
                    key={tenant.id}
                    className="group cursor-pointer hover:bg-muted/40"
                    onClick={() => router.push(`/admin/tenants/detail?id=${tenant.id}`)}
                  >
                    <TableCell className="font-medium pl-6">
                      <div className="flex items-center gap-3">
                        {tenant.settings?.logoUrl ? (
                          <img
                            src={tenant.settings.logoUrl.startsWith("http") || tenant.settings.logoUrl.startsWith("data:")
                              ? tenant.settings.logoUrl
                              : `${api.getBaseUrl()}${tenant.settings.logoUrl}`}
                            alt={tenant.name}
                            className="h-10 w-10 p-1 rounded object-cover border bg-slate-100"
                          />
                        ) : (
                          <div className="h-10 w-10 rounded bg-slate-100 flex items-center justify-center text-slate-700 font-bold text-xm">
                            {tenant.name.charAt(0).toUpperCase()}
                          </div>
                        )}
                        {tenant.name}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="font-mono text-xs font-normal text-muted-foreground">
                        {tenant.slug}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right pr-6">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity data-[state=open]:opacity-100"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <MoreHorizontal className="h-4 w-4" />
                            <span className="sr-only">{t("common.actions")}</span>
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="w-48">
                          <DropdownMenuItem asChild>
                            <Link
                              href={`/admin/tenants/settings?id=${tenant.id}`}
                              className="flex w-full cursor-pointer items-center"
                              onClick={(e) => e.stopPropagation()}
                            >
                              <Settings className="mr-2 h-4 w-4" />
                              {t("common.edit")}
                            </Link>
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            className="text-destructive focus:text-destructive cursor-pointer flex w-full items-center"
                            onClick={(e) => {
                              e.stopPropagation()
                              setSelectedTenant(tenant)
                              setDeleteDialogOpen(true)
                            }}
                          >
                            <Trash2 className="mr-2 h-4 w-4" />
                            {t("common.delete")}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table >
        )
        }
      </Card >

      {/* Create Wizard Dialog */}
      < Dialog open={createDialogOpen} onOpenChange={(open) => {
        setCreateDialogOpen(open)
        if (!open) {
          // Reset wizard on close
          setTimeout(() => {
            setCurrentStep(0)
            setNewTenant({ name: "", slug: "", display_name: "", settings: {} })
            setEnableSMTP(false)
            setEnableUserDB(false)
            setEnableSocial(false)
            setEnableCache(false)
          }, 300)
        }
      }}>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-[800px] p-0 gap-0 overflow-hidden flex flex-col bg-background/95 backdrop-blur-sm">
          {/* Wizard Header */}
          <div className="bg-muted/30 border-b p-6 pb-4">
            <DialogHeader className="mb-6">
              <DialogTitle className="text-xl">{t("tenants.createTitle")}</DialogTitle>
            </DialogHeader>

            {/* Stepper with Single Color Progress */}
            <div className="relative flex items-center justify-between px-10 mb-6 mt-4">
              {/* Background Line */}
              <div className="absolute left-10 right-10 top-4 h-[2px] bg-muted -z-10" />

              {/* Active Progress Line Container matching background line */}
              <div className="absolute left-10 right-10 top-4 h-[2px] -z-10 overflow-hidden px-10 flex justify-start pointer-events-none">
                <div
                  className="h-full bg-primary transition-all duration-500 ease-in-out"
                  style={{ width: `${(currentStep / 4) * 100}%` }}
                />
              </div>

              {[
                { id: 0, label: t("common.wizard.steps.basic") },
                { id: 1, label: t("common.wizard.steps.database") },
                { id: 2, label: t("common.wizard.steps.smtp") },
                { id: 3, label: t("common.wizard.steps.social") },
                { id: 4, label: t("common.wizard.steps.verification") }
              ].map((step, index) => {
                const isActive = currentStep === step.id
                const isCompleted = currentStep > step.id

                return (
                  <div key={step.id} className="flex flex-col items-center relative z-10 group w-24">
                    <div className={cn(
                      "w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold border-2 transition-all duration-300 bg-background",
                      isActive ? "border-primary text-primary shadow-md scale-110 ring-4 ring-primary/10" :
                        isCompleted ? "border-primary bg-primary text-primary-foreground" :
                          "border-muted text-muted-foreground"
                    )}>
                      {isCompleted ? <Check className="h-4 w-4" /> : index + 1}
                    </div>
                    <span className={cn(
                      "text-[10px] mt-2 font-medium transition-all duration-300 absolute -bottom-6 w-32 text-center uppercase tracking-wider",
                      isActive ? "text-foreground opacity-100" : "text-muted-foreground/50"
                    )}>
                      {step.label}
                    </span>
                  </div>
                )
              })}
            </div>
          </div>

          {/* Wizard Content - Custom Scrollbar */}
          <div className="p-8 flex-1 overflow-y-auto min-h-[450px] custom-scrollbar">

            {/* Step 0: Basic Info */}
            {currentStep === 0 && (
              <div className="space-y-6 animate-in slide-in-from-right-8 fade-in duration-500">
                <div className="grid grid-cols-2 gap-6">
                  <div className="col-span-2 sm:col-span-1 space-y-2">
                    <Label htmlFor="name">{t("tenants.name")} <span className="text-destructive">*</span></Label>
                    <Input
                      id="name"
                      value={newTenant.name}
                      onChange={(e) => {
                        const val = e.target.value
                        setNewTenant({ ...newTenant, name: val, slug: newTenant.slug || val.toLowerCase().replace(/[^a-z0-9-]/g, '-') })
                      }}
                      placeholder="Acme Corp"
                      autoFocus
                      className="border-primary/20 focus:border-primary"
                    />
                  </div>
                  <div className="col-span-2 sm:col-span-1 space-y-2">
                    <Label htmlFor="slug">{t("tenants.slug")} <span className="text-destructive">*</span></Label>
                    <div className="relative">
                      <Input
                        id="slug"
                        value={newTenant.slug}
                        onChange={(e) => setNewTenant({ ...newTenant, slug: e.target.value })}
                        placeholder="acme"
                        className="font-mono border-primary/20 focus:border-primary"
                      />
                      {newTenant.slug && (
                        <div className="absolute right-3 top-1/2 -translate-y-1/2">
                          {/^[a-z0-9\-]+$/.test(newTenant.slug) ? <Check className="h-3 w-3 text-emerald-500" /> : <X className="h-3 w-3 text-destructive" />}
                        </div>
                      )}
                    </div>
                    <p className="text-[10px] text-muted-foreground/80">URL friendly ID (a-z, 0-9, -)</p>
                  </div>
                  <div className="col-span-2 space-y-2">
                    <Label htmlFor="display_name">{t("tenants.displayName")}</Label>
                    <Input
                      id="display_name"
                      value={newTenant.display_name}
                      onChange={(e) => setNewTenant({ ...newTenant, display_name: e.target.value })}
                      placeholder="ACME Corporation (Optional)"
                    />
                  </div>
                </div>

                <div className="space-y-2 pt-2">
                  <Label>{t("common.logo")}</Label>
                  <LogoDropzone
                    value={newTenant.settings?.logoUrl}
                    onChange={(base64) => setNewTenant({
                      ...newTenant,
                      settings: { ...newTenant.settings, logoUrl: base64 }
                    })}
                  />
                </div>
              </div>
            )}

            {/* Step 1: Database & Cache (Data) */}
            {/* Step 1: Database & Cache (Data) */}
            {currentStep === 1 && (
              <div className="space-y-8 animate-in slide-in-from-right-8 fade-in duration-500">

                {/* User DB Section */}
                <div className="space-y-4">
                  <div className="flex items-center gap-3 mb-2">
                    <div className="p-2 rounded-lg bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400">
                      <Database className="h-5 w-5" />
                    </div>
                    <div>
                      <h3 className="font-semibold text-lg">User Database</h3>
                      <p className="text-sm text-muted-foreground">{t("common.wizard.messages.db_required")}</p>
                    </div>
                  </div>

                  <div className="flex items-center justify-between p-4 border rounded-xl bg-card hover:border-blue-500/30 transition-all shadow-sm">
                    <div className="space-y-0.5">
                      <Label htmlFor="enable_db" className="text-base font-medium cursor-pointer">Enable Custom User Store</Label>
                      <p className="text-sm text-muted-foreground">Isolate and manage users in a dedicated DB</p>
                    </div>
                    <Switch
                      id="enable_db"
                      checked={enableUserDB}
                      onCheckedChange={setEnableUserDB}
                    />
                  </div>

                  {enableUserDB && (
                    <div className={cn(
                      "p-5 border rounded-xl space-y-4 animate-in fade-in slide-in-from-top-2 transition-colors duration-500",
                      connectionStatus === "success" ? "bg-emerald-50/50 border-emerald-200 dark:bg-emerald-950/20 dark:border-emerald-800" :
                        connectionStatus === "error" ? "bg-red-50/50 border-red-200 dark:bg-red-950/20 dark:border-red-800" :
                          "bg-blue-50/50 dark:bg-blue-950/20"
                    )}>
                      {/* DB Interaction Mode */}
                      <div className="flex justify-between items-start">
                        <div className="flex bg-background/50 rounded-lg p-1 w-fit border">
                          <Button
                            size="sm"
                            variant={!newTenant.settings?.userDb?.manualMode ? "secondary" : "ghost"}
                            onClick={() => setNewTenant({ ...newTenant, settings: { ...newTenant.settings, userDb: { ...newTenant.settings?.userDb, manualMode: false } } })}
                          >
                            {t("common.wizard.actions.connection_string")}
                          </Button>
                          <Button
                            size="sm"
                            variant={newTenant.settings?.userDb?.manualMode ? "secondary" : "ghost"}
                            onClick={() => setNewTenant({ ...newTenant, settings: { ...newTenant.settings, userDb: { ...newTenant.settings?.userDb, manualMode: true } } })}
                          >
                            {t("common.wizard.actions.manual_fields")}
                          </Button>
                        </div>

                        {/* Status Indicator */}
                        {connectionStatus !== "idle" && (
                          <div className={cn(
                            "flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-medium border animate-in fade-in slide-in-from-right-4",
                            connectionStatus === "success" ? "bg-emerald-100 text-emerald-700 border-emerald-200" : "bg-red-100 text-red-700 border-red-200"
                          )}>
                            {connectionStatus === "success" ? <Check className="h-3.5 w-3.5" /> : <X className="h-3.5 w-3.5" />}
                            {connectionStatus === "testing" ? "Probando..." : connectionMessage}
                          </div>
                        )}
                      </div>

                      {!newTenant.settings?.userDb?.manualMode ? (
                        <div className="space-y-2">
                          <Label htmlFor="db_dsn">Postgres DSN <span className="text-destructive">*</span></Label>
                          <Input
                            id="db_dsn"
                            type="password"
                            value={newTenant.settings?.userDb?.dsn || ""}
                            onChange={(e) => {
                              setNewTenant({
                                ...newTenant,
                                settings: {
                                  ...newTenant.settings,
                                  userDb: { ...newTenant.settings?.userDb, dsn: e.target.value, driver: "postgres" },
                                },
                              })
                              // Reset status on change
                              if (connectionStatus !== "idle") {
                                setConnectionStatus("idle")
                                setConnectionMessage("")
                              }
                            }}
                            placeholder="postgres://user:pass@host:5432/db"
                            className={cn(
                              "font-mono text-sm bg-background transition-all",
                              connectionStatus === "success" ? "border-emerald-500 ring-emerald-500/20" :
                                connectionStatus === "error" ? "border-red-500 ring-red-500/20" : ""
                            )}
                          />
                        </div>
                      ) : (
                        <div className="grid grid-cols-2 gap-4">
                          <div className="col-span-2 text-sm text-amber-600 bg-amber-100 dark:bg-amber-900/30 p-2 rounded">
                            Manual fields parsing logic will be implemented here. For now use DSN.
                          </div>
                        </div>
                      )}

                      <div className="flex justify-end pt-2">
                        <Button
                          variant="outline"
                          size="sm"
                          className={cn(
                            "gap-2 transition-colors",
                            connectionStatus === "success" ? "text-emerald-600 border-emerald-200 hover:bg-emerald-50" :
                              connectionStatus === "error" ? "text-red-600 border-red-200 hover:bg-red-50" :
                                "text-blue-600 border-blue-200 hover:bg-blue-50"
                          )}
                          onClick={() => {
                            setConnectionStatus("testing")
                            testConnectionMutation.mutate(newTenant.settings?.userDb?.dsn)
                          }}
                          disabled={testConnectionMutation.isPending || !newTenant.settings?.userDb?.dsn}
                        >
                          {testConnectionMutation.isPending ? <div className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" /> : <Database className="h-4 w-4" />}
                          {t("common.wizard.actions.test_connection")}
                        </Button>
                      </div>
                    </div>
                  )}
                </div>

                <div className="border-t my-2" />

                {/* Cache Section */}
                <div className="space-y-4">
                  <div className="flex items-center gap-3 mb-2">
                    <div className="p-2 rounded-lg bg-orange-100 dark:bg-orange-900/30 text-orange-600 dark:text-orange-400">
                      <Database className="h-5 w-5" />
                    </div>
                    <div>
                      <h3 className="font-semibold text-lg">Cache Strategy</h3>
                      <p className="text-sm text-muted-foreground">Configure session and transient data storage.</p>
                    </div>
                  </div>

                  <div className="p-4 border rounded-xl bg-card hover:border-orange-500/30 transition-all shadow-sm">
                    <div className="flex items-center justify-between mb-2">
                      <div className="space-y-0.5">
                        <Label className="text-base font-medium">Enable Caching</Label>
                        <p className="text-sm text-muted-foreground">Improve performance with caching</p>
                      </div>
                      <Switch
                        id="enable_cache"
                        checked={enableCache}
                        onCheckedChange={(checked) => {
                          setEnableCache(checked);
                          if (!checked) {
                            setNewTenant(prev => ({ ...prev, settings: { ...prev.settings, cache: undefined } }))
                          } else {
                            if (!newTenant.settings?.cache?.driver) {
                              setNewTenant(prev => ({ ...prev, settings: { ...prev.settings, cache: { driver: "memory", enabled: true } } }))
                            }
                          }
                        }}
                      />
                    </div>

                    {enableCache ? (
                      <div className="pt-4 border-t animate-in fade-in space-y-4">
                        <div className="space-y-2">
                          <Label>Driver</Label>
                          <div className="flex items-center gap-2">
                            <Button
                              size="sm"
                              variant={newTenant.settings?.cache?.driver === "memory" ? "default" : "outline"}
                              onClick={() => setNewTenant({ ...newTenant, settings: { ...newTenant.settings, cache: { ...newTenant.settings?.cache, driver: "memory", enabled: true } } })}
                              className={newTenant.settings?.cache?.driver === "memory" ? "bg-orange-600 hover:bg-orange-700" : ""}
                            >
                              In-Memory
                            </Button>
                            <Button
                              size="sm"
                              variant={newTenant.settings?.cache?.driver === "redis" ? "default" : "outline"}
                              onClick={() => setNewTenant({ ...newTenant, settings: { ...newTenant.settings, cache: { ...newTenant.settings?.cache, driver: "redis", enabled: true } } })}
                              className={newTenant.settings?.cache?.driver === "redis" ? "bg-orange-600 hover:bg-orange-700" : ""}
                            >
                              Redis
                            </Button>
                          </div>
                        </div>

                        {newTenant.settings?.cache?.driver === "redis" ? (
                          <div className="grid grid-cols-2 gap-4 p-4 bg-orange-50/50 dark:bg-orange-950/20 rounded-lg">
                            <div className="space-y-2">
                              <Label>Host</Label>
                              <Input
                                placeholder="localhost"
                                value={newTenant.settings?.cache?.host || ""}
                                onChange={(e) => setNewTenant({ ...newTenant, settings: { ...newTenant.settings, cache: { ...newTenant.settings?.cache, host: e.target.value } } })}
                              />
                            </div>
                            <div className="space-y-2">
                              <Label>Port</Label>
                              <Input
                                type="number"
                                placeholder="6379"
                                value={newTenant.settings?.cache?.port || 6379}
                                onChange={(e) => setNewTenant({ ...newTenant, settings: { ...newTenant.settings, cache: { ...newTenant.settings?.cache, port: parseInt(e.target.value) } } })}
                              />
                            </div>
                            <div className="space-y-2">
                              <Label>Password</Label>
                              <Input
                                type="password"
                                value={newTenant.settings?.cache?.password || ""}
                                onChange={(e) => setNewTenant({ ...newTenant, settings: { ...newTenant.settings, cache: { ...newTenant.settings?.cache, password: e.target.value } } })}
                              />
                            </div>
                            <div className="space-y-2">
                              <Label>DB Index</Label>
                              <Input
                                type="number"
                                value={newTenant.settings?.cache?.db || 0}
                                onChange={(e) => setNewTenant({ ...newTenant, settings: { ...newTenant.settings, cache: { ...newTenant.settings?.cache, db: parseInt(e.target.value) } } })}
                              />
                            </div>
                            <div className="col-span-2 flex justify-end">
                              <Button variant="outline" size="sm" className="gap-2 text-orange-600 border-orange-200 hover:bg-orange-50">
                                <Database className="h-4 w-4" />
                                {t("common.wizard.actions.test_connection")}
                              </Button>
                            </div>
                          </div>
                        ) : (
                          <div className="text-sm text-yellow-600 bg-yellow-50 dark:bg-yellow-900/20 p-3 rounded-md border border-yellow-200 dark:border-yellow-800 flex items-start gap-2">
                            <div className="mt-0.5 text-lg">⚠️</div>
                            <div>
                              <p className="font-semibold">In-Memory Cache Selected</p>
                              <p className="opacity-90">Non-persistent. Data lost on restart.</p>
                            </div>
                          </div>
                        )}
                      </div>
                    ) : (
                      <div className="text-sm text-muted-foreground p-3 bg-muted/30 rounded border border-dashed text-center">
                        Caching is <b>Disabled</b>.
                      </div>
                    )}
                  </div>
                </div>
              </div>
            )}

            {/* Step 2: SMTP */}
            {currentStep === 2 && (
              <div className="space-y-8 animate-in slide-in-from-right-8 fade-in duration-500">
                <div className="flex items-center gap-3 mb-2">
                  <div className="p-2 rounded-lg bg-amber-100 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400">
                    <Mail className="h-5 w-5" />
                  </div>
                  <div>
                    <h3 className="font-semibold text-lg">SMTP Configuration</h3>
                    <p className="text-sm text-muted-foreground">Custom email server.</p>
                  </div>
                </div>

                <div className="flex items-center justify-between p-4 border rounded-xl bg-card hover:border-amber-500/30 transition-all shadow-sm">
                  <div className="space-y-0.5">
                    <Label htmlFor="enable_smtp" className="text-base font-medium cursor-pointer">Enable Custom SMTP</Label>
                    <p className="text-sm text-muted-foreground">Send emails via your own server</p>
                  </div>
                  <Switch
                    id="enable_smtp"
                    checked={enableSMTP}
                    onCheckedChange={setEnableSMTP}
                  />
                </div>

                {enableSMTP && (
                  <div className="grid grid-cols-2 gap-4 p-5 border rounded-xl bg-amber-50/50 dark:bg-amber-950/20 animate-in fade-in slide-in-from-top-2">
                    <div className="space-y-2">
                      <Label htmlFor="smtp_host">Host</Label>
                      <Input
                        id="smtp_host"
                        value={newTenant.settings?.smtp?.host || ""}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: { ...newTenant.settings, smtp: { ...newTenant.settings?.smtp, host: e.target.value } },
                          })
                        }
                        placeholder="smtp.example.com"
                        className="bg-background"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="smtp_port">Port</Label>
                      <Input
                        id="smtp_port"
                        type="number"
                        value={newTenant.settings?.smtp?.port || 587}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: { ...newTenant.settings, smtp: { ...newTenant.settings?.smtp, port: parseInt(e.target.value) } },
                          })
                        }
                        className="bg-background"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="smtp_user">Username</Label>
                      <Input
                        id="smtp_user"
                        value={newTenant.settings?.smtp?.username || ""}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: { ...newTenant.settings, smtp: { ...newTenant.settings?.smtp, username: e.target.value } },
                          })
                        }
                        className="bg-background"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="smtp_pass">Password</Label>
                      <Input
                        id="smtp_pass"
                        type="password"
                        value={newTenant.settings?.smtp?.password || ""}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: { ...newTenant.settings, smtp: { ...newTenant.settings?.smtp, password: e.target.value } },
                          })
                        }
                        className="bg-background"
                      />
                    </div>
                    <div className="col-span-2 space-y-2">
                      <Label htmlFor="smtp_from">From Email</Label>
                      <Input
                        id="smtp_from"
                        value={newTenant.settings?.smtp?.fromEmail || ""}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: { ...newTenant.settings, smtp: { ...newTenant.settings?.smtp, fromEmail: e.target.value } },
                          })
                        }
                        placeholder="noreply@example.com"
                        className="bg-background"
                      />
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Step 3: Social */}
            {currentStep === 3 && (
              <div className="space-y-8 animate-in slide-in-from-right-8 fade-in duration-500">
                <div className="flex items-center gap-3 mb-2">
                  <div className="p-2 rounded-lg bg-purple-100 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400">
                    <Globe className="h-5 w-5" />
                  </div>
                  <div>
                    <h3 className="font-semibold text-lg">Social Authentication</h3>
                    <p className="text-sm text-muted-foreground">Configure OAuth providers.</p>
                  </div>
                </div>

                <div className="flex items-center justify-between p-4 border rounded-xl bg-card hover:border-purple-500/30 transition-all shadow-sm">
                  <div className="space-y-0.5">
                    <Label htmlFor="enable_social" className="text-base font-medium cursor-pointer">Enable Social Login</Label>
                    <p className="text-sm text-muted-foreground">Google, etc.</p>
                  </div>
                  <Switch
                    id="enable_social"
                    checked={enableSocial}
                    onCheckedChange={setEnableSocial}
                  />
                </div>

                {enableSocial && (
                  <div className="space-y-4 pt-4 animate-in fade-in slide-in-from-top-2 p-5 border rounded-xl bg-purple-50/50 dark:bg-purple-950/20">
                    <div className="relative mb-6">
                      <div className="absolute inset-0 flex items-center">
                        <span className="w-full border-t border-purple-200 dark:border-purple-800" />
                      </div>
                      <div className="relative flex justify-center text-xs uppercase">
                        <span className="bg-purple-50 dark:bg-zinc-900 px-2 text-purple-600 font-bold tracking-widest">Google Provider</span>
                      </div>
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="google_id">Client ID</Label>
                      <Input
                        id="google_id"
                        value={newTenant.settings?.socialProviders?.google?.clientId || ""}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: {
                              ...newTenant.settings,
                              socialProviders: {
                                ...newTenant.settings?.socialProviders,
                                google: { ...newTenant.settings?.socialProviders?.google, clientId: e.target.value },
                              },
                            },
                          })
                        }
                        placeholder="...apps.googleusercontent.com"
                        className="bg-background"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="google_secret">Client Secret</Label>
                      <Input
                        id="google_secret"
                        type="password"
                        value={newTenant.settings?.socialProviders?.google?.clientSecret || ""}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: {
                              ...newTenant.settings,
                              socialProviders: {
                                ...newTenant.settings?.socialProviders,
                                google: { ...newTenant.settings?.socialProviders?.google, clientSecret: e.target.value },
                              },
                            },
                          })
                        }
                        className="bg-background"
                      />
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Step 4: Verification */}
            {currentStep === 4 && (
              <div className="space-y-8 animate-in slide-in-from-right-8 fade-in duration-500">
                <div className="text-center space-y-2">
                  <div className="w-16 h-16 bg-emerald-100 text-emerald-600 rounded-full flex items-center justify-center mx-auto mb-4 animate-pulse">
                    <Check className="h-8 w-8" />
                  </div>
                  <h2 className="text-2xl font-bold">{t("common.wizard.steps.verification")}</h2>
                  <p className="text-muted-foreground max-w-md mx-auto">Review your configuration before creating the tenant.</p>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <Card className="p-4 border-l-4 border-l-zinc-500 shadow-sm">
                    <h4 className="font-semibold text-sm text-zinc-600 mb-2 uppercase tracking-wide">Basic</h4>
                    <div className="space-y-1 text-sm">
                      <p><span className="text-muted-foreground">Name:</span> {newTenant.name}</p>
                      <p><span className="text-muted-foreground">Slug:</span> {newTenant.slug}</p>
                    </div>
                  </Card>

                  <Card className="p-4 border-l-4 border-l-blue-500 shadow-sm">
                    <h4 className="font-semibold text-sm text-blue-600 mb-2 uppercase tracking-wide">Data</h4>
                    <div className="space-y-1 text-sm">
                      <p><span className="text-muted-foreground">User DB:</span> {enableUserDB ? "Custom" : "Missing!"}</p>
                      <p><span className="text-muted-foreground">Cache:</span> {enableCache ? (newTenant.settings?.cache?.driver === "redis" ? "Redis" : "Memory") : "Disabled"}</p>
                      {!enableUserDB && <p className="text-destructive text-xs font-bold mt-1">WARNING: User DB Required for user management</p>}
                    </div>
                  </Card>

                  <Card className="p-4 border-l-4 border-l-amber-500 shadow-sm">
                    <h4 className="font-semibold text-sm text-amber-600 mb-2 uppercase tracking-wide">SMTP</h4>
                    <div className="space-y-1 text-sm">
                      <p><span className="text-muted-foreground">Status:</span> {enableSMTP ? "Enabled" : "Disabled"}</p>
                      {enableSMTP && <p><span className="text-muted-foreground">Host:</span> {newTenant.settings?.smtp?.host}</p>}
                    </div>
                  </Card>

                  <Card className="p-4 border-l-4 border-l-purple-500 shadow-sm">
                    <h4 className="font-semibold text-sm text-purple-600 mb-2 uppercase tracking-wide">Social</h4>
                    <div className="space-y-1 text-sm">
                      <p><span className="text-muted-foreground">Status:</span> {enableSocial ? "Enabled" : "Disabled"}</p>
                      {enableSocial && <p><span className="text-muted-foreground">Google:</span> {newTenant.settings?.socialProviders?.google?.clientId ? "Configured" : "Incomplete"}</p>}
                    </div>
                  </Card>
                </div>
              </div>
            )}

          </div>

          {/* Wizard Footer */}
          <div className="border-t p-6 bg-muted/30 flex justify-between items-center backdrop-blur-md">
            <div>
              {currentStep > 0 ? (
                <Button variant="ghost" onClick={() => setCurrentStep(prev => prev - 1)} className="hover:bg-background">
                  {t("common.wizard.back")}
                </Button>
              ) : (
                <Button variant="ghost" onClick={() => setCreateDialogOpen(false)} className="hover:bg-destructive/10 hover:text-destructive">
                  {t("common.cancel")}
                </Button>
              )}
            </div>

            <div className="flex gap-2">
              {[2, 3].includes(currentStep) && (
                <Button variant="ghost" onClick={() => setCurrentStep(prev => prev + 1)}>
                  {t("common.wizard.skip")}
                </Button>
              )}

              {currentStep < 4 ? (
                <Button
                  onClick={() => setCurrentStep(prev => prev + 1)}
                  className={cn(
                    "gap-2 transition-all duration-300",
                    currentStep === 0 ? "bg-zinc-800" :
                      currentStep === 1 ? "bg-blue-600 hover:bg-blue-700" :
                        currentStep === 2 ? "bg-amber-600 hover:bg-amber-700" :
                          "bg-purple-600 hover:bg-purple-700"
                  )}
                >
                  {t("common.wizard.next")}
                  <ArrowRight className="h-4 w-4" />
                </Button>
              ) : (
                <Button
                  onClick={handleCreate}
                  disabled={createMutation.isPending}
                  className="bg-emerald-600 hover:bg-emerald-700 gap-2 px-8"
                >
                  {createMutation.isPending ? t("common.creating") : "Configurar Tenant"}
                  {createMutation.isPending ? <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div> : <Check className="h-4 w-4" />}
                </Button>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog >

      {/* Delete Dialog */}
      < Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen} >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("tenants.deleteTitle")}</DialogTitle>
            <DialogDescription>{t("tenants.deleteDescription", { name: selectedTenant?.name })}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button variant="destructive" onClick={handleDelete} disabled={deleteMutation.isPending}>
              {deleteMutation.isPending ? t("common.deleting") : t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog >
    </div >
  )
}
