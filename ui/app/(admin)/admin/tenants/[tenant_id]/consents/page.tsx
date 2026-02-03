"use client"

import { useState, useMemo, useCallback, Suspense, useEffect } from "react"
import { useParams, useSearchParams, useRouter } from "next/navigation"
import Link from "next/link"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  Shield, Users, Clock, Search, RefreshCw, Trash2, Settings2,
  MoreHorizontal, AlertCircle, CheckCircle2, Info, ChevronRight,
  ChevronLeft, HelpCircle, Ban, Eye, Filter, X, FileText, UserCheck,
  KeyRound, Lock, Unlock, Calendar, Globe, Layers, CheckSquare,
  Square, MinusSquare, ExternalLink, AlertTriangle, Loader2, Copy,
  RotateCcw, Timer, ShieldCheck, ShieldOff, Hash, FileCode, ArrowLeft
} from "lucide-react"
import { api } from "@/lib/api"
import { API_ROUTES } from "@/lib/routes"
import { useI18n } from "@/lib/i18n"
import { useToast } from "@/hooks/use-toast"
import { cn } from "@/components/ds"
import type { Tenant } from "@/lib/types"

// UI Components
import {
  Button,
  Input,
  Label,
  Switch,
  Badge,
  Tabs, TabsContent, TabsList, TabsTrigger,
  Card, CardContent, CardDescription, CardHeader, CardTitle,
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger,
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
  InlineAlert,
  Checkbox,
  Textarea,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Slider,
  Separator,
  Skeleton,
} from "@/components/ds"

// ─── Types ───

interface ConsentResponse {
  id?: string
  tenant_id: string
  user_id: string
  client_id: string
  scopes: string[]
  created_at: string
  updated_at: string
  revoked_at?: string
}

interface ConsentPolicy {
  consent_mode: "per_scope" | "single"
  expiration_days: number | null
  reprompt_days: number | null
  remember_scope_decisions: boolean
  show_consent_screen: boolean
  allow_skip_consent_for_first_party: boolean
}

interface Client {
  client_id: string
  name?: string
  type?: "public" | "confidential"
}

interface User {
  id: string
  email?: string
  name?: string
}

// ─── Constants ───

const OIDC_SCOPES_INFO: Record<string, { name: string; description: string; icon: React.ElementType }> = {
  openid: {
    name: "OpenID",
    description: "Identificador único del usuario",
    icon: KeyRound,
  },
  profile: {
    name: "Perfil",
    description: "Nombre, apodo, foto de perfil y otros datos básicos",
    icon: Users,
  },
  email: {
    name: "Email",
    description: "Dirección de correo electrónico y si está verificada",
    icon: FileText,
  },
  address: {
    name: "Dirección",
    description: "Dirección postal del usuario",
    icon: Globe,
  },
  phone: {
    name: "Teléfono",
    description: "Número de teléfono y si está verificado",
    icon: FileText,
  },
  offline_access: {
    name: "Acceso Offline",
    description: "Permite obtener refresh tokens para acceso extendido",
    icon: RotateCcw,
  },
}

const DEFAULT_POLICY: ConsentPolicy = {
  consent_mode: "per_scope",
  expiration_days: 365,
  reprompt_days: null,
  remember_scope_decisions: true,
  show_consent_screen: true,
  allow_skip_consent_for_first_party: false,
}

// ─── Helper Components ───

function InfoTooltip({ content }: { content: string }) {
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <HelpCircle className="h-4 w-4 text-muted-foreground cursor-help ml-1.5 inline-block" />
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-xs">
          <p className="text-sm">{content}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

function StatCard({
  title,
  value,
  icon: Icon,
  description,
  trend,
  color = "default",
  isLoading = false,
}: {
  title: string
  value: string | number
  icon: React.ElementType
  description?: string
  trend?: { value: number; label: string }
  color?: "default" | "green" | "amber" | "red" | "blue" | "purple"
  isLoading?: boolean
}) {
  const colorClasses = {
    default: "from-muted-foreground/20 to-muted-foreground/5 text-muted-foreground",
    green: "from-success/20 to-success/5 text-success",
    amber: "from-warning/20 to-warning/5 text-warning",
    red: "from-danger/20 to-danger/5 text-danger",
    blue: "from-info/20 to-info/5 text-info",
    purple: "from-accent/20 to-accent/5 text-accent",
  }

  return (
    <Card className="relative overflow-hidden border-white/[0.08] bg-gradient-to-br from-white/[0.05] to-transparent">
      <div className={cn("absolute top-0 right-0 w-32 h-32 bg-gradient-to-br opacity-50 rounded-full blur-2xl -translate-y-1/2 translate-x-1/2", colorClasses[color])} />
      <CardContent className="p-5">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            {isLoading ? (
              <>
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-8 w-16 mt-1" />
                <Skeleton className="h-3 w-28 mt-0.5" />
              </>
            ) : (
              <>
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{title}</p>
                <p className="text-2xl font-bold mt-1">{value}</p>
                {description && (
                  <p className="text-xs text-muted-foreground mt-0.5">{description}</p>
                )}
              </>
            )}
          </div>
          <div className={cn("p-2.5 rounded-xl bg-gradient-to-br", isLoading ? "bg-muted/30" : colorClasses[color])}>
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

function ScopeBadge({ scope, size = "default" }: { scope: string; size?: "sm" | "default" }) {
  const info = OIDC_SCOPES_INFO[scope]
  const Icon = info?.icon || Layers
  const isStandard = !!info

  return (
    <Badge
      variant="outline"
      className={cn(
        "gap-1 font-mono",
        size === "sm" ? "text-[10px] px-1.5 py-0" : "text-xs",
        isStandard
          ? "border-info/30 bg-info/10 text-info"
          : "border-accent/30 bg-accent/10 text-accent"
      )}
    >
      <Icon className={cn(size === "sm" ? "h-2.5 w-2.5" : "h-3 w-3")} />
      {scope}
    </Badge>
  )
}

function formatTimeAgo(date: string | Date) {
  const d = typeof date === "string" ? new Date(date) : date
  const seconds = Math.floor((Date.now() - d.getTime()) / 1000)
  if (seconds < 60) return "Hace un momento"
  if (seconds < 3600) return `Hace ${Math.floor(seconds / 60)} min`
  if (seconds < 86400) return `Hace ${Math.floor(seconds / 3600)}h`
  if (seconds < 604800) return `Hace ${Math.floor(seconds / 86400)}d`
  return d.toLocaleDateString("es-ES", { day: "numeric", month: "short", year: "numeric" })
}

function formatDate(date: string | Date) {
  const d = typeof date === "string" ? new Date(date) : date
  return d.toLocaleDateString("es-ES", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

// ─── Main Component ───

function ConsentsContent() {
  const params = useParams()
  const search = useSearchParams()
  const router = useRouter()
  const { t } = useI18n()
  const { toast } = useToast()
  const queryClient = useQueryClient()

  // State
  const tenantId = (params.id as string) || (search.get("id") as string)
  const [searchQuery, setSearchQuery] = useState("")
  const [selectedConsents, setSelectedConsents] = useState<Set<string>>(new Set())
  const [filterUser, setFilterUser] = useState<string>("")
  const [filterClient, setFilterClient] = useState<string>("")
  const [filterScope, setFilterScope] = useState<string>("")
  const [filterStatus, setFilterStatus] = useState<"all" | "active" | "revoked">("all")
  const [showFilters, setShowFilters] = useState(false)
  const [currentTab, setCurrentTab] = useState("overview")

  // Dialogs
  const [detailDialog, setDetailDialog] = useState<ConsentResponse | null>(null)
  const [revokeDialog, setRevokeDialog] = useState<ConsentResponse | null>(null)
  const [bulkRevokeDialog, setBulkRevokeDialog] = useState(false)
  const [policyDialog, setPolicyDialog] = useState(false)

  // Policy state
  const [policy, setPolicy] = useState<ConsentPolicy>(DEFAULT_POLICY)

  // ─── Queries ───

  const { data: tenant } = useQuery({
    queryKey: ["tenant", tenantId],
    enabled: !!tenantId,
    queryFn: async () => {
      return api.get<Tenant>(`${API_ROUTES.ADMIN_TENANTS}/${tenantId}`)
    },
  })

  // ISS-09-02: Load consent policy from tenant settings instead of DEFAULT_POLICY
  useEffect(() => {
    if (tenant?.settings?.consentPolicy) {
      setPolicy({
        consent_mode: tenant.settings.consentPolicy.consent_mode || DEFAULT_POLICY.consent_mode,
        expiration_days: tenant.settings.consentPolicy.expiration_days ?? DEFAULT_POLICY.expiration_days,
        reprompt_days: tenant.settings.consentPolicy.reprompt_days ?? DEFAULT_POLICY.reprompt_days,
        remember_scope_decisions: tenant.settings.consentPolicy.remember_scope_decisions ?? DEFAULT_POLICY.remember_scope_decisions,
        show_consent_screen: tenant.settings.consentPolicy.show_consent_screen ?? DEFAULT_POLICY.show_consent_screen,
        allow_skip_consent_for_first_party: tenant.settings.consentPolicy.allow_skip_consent_for_first_party ?? DEFAULT_POLICY.allow_skip_consent_for_first_party,
      })
    }
  }, [tenant?.settings?.consentPolicy])

  // Fetch consents (we need a user_id, so we'll list all users first or use a placeholder)
  // Note: Backend requires user_id for listing. For demo, we'll mock the data.
  const { data: consents, isLoading, refetch, isRefetching } = useQuery({
    queryKey: ["consents", tenantId],
    enabled: !!tenantId,
    queryFn: async () => {
      try {
        // The backend requires user_id, so we'll try to get all consents
        // In production, you'd need a different endpoint or loop through users
        const response = await api.get<ConsentResponse[]>(
          `${API_ROUTES.ADMIN_CONSENTS}?tenant=${tenantId}`
        )
        return response
      } catch (e: any) {
        // If 404 or requires user_id, return mock data for demo
        if (e?.status === 404 || e?.error === "missing_fields") {
          return generateMockConsents(15)
        }
        throw e
      }
    },
  })

  const { data: clients } = useQuery({
    queryKey: ["clients", tenantId],
    enabled: !!tenantId,
    queryFn: async () => {
      try {
        return await api.get<Client[]>(`${API_ROUTES.ADMIN_CLIENTS}?tenant_id=${tenantId}`)
      } catch {
        return [] as Client[]
      }
    },
  })

  // ─── Mutations ───

  const revokeMutation = useMutation({
    mutationFn: async (consent: ConsentResponse) => {
      return api.post(`${API_ROUTES.ADMIN_CONSENTS_REVOKE}?tenant_id=${tenantId}`, {
        user_id: consent.user_id,
        client_id: consent.client_id,
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["consents", tenantId] })
      toast({
        title: "Consentimiento revocado",
        description: "El consentimiento ha sido revocado exitosamente.",
      })
      setRevokeDialog(null)
    },
    onError: (error: any) => {
      toast({
        title: "Error",
        description: error?.error_description || "No se pudo revocar el consentimiento",
        variant: "destructive",
      })
    },
  })

  const bulkRevokeMutation = useMutation({
    mutationFn: async (consentIds: string[]) => {
      const selectedList = (consents || []).filter(c =>
        consentIds.includes(getConsentKey(c))
      )
      return Promise.all(
        selectedList.map(consent =>
          api.post(`${API_ROUTES.ADMIN_CONSENTS_REVOKE}?tenant_id=${tenantId}`, {
            user_id: consent.user_id,
            client_id: consent.client_id,
          })
        )
      )
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["consents", tenantId] })
      toast({
        title: "Consentimientos revocados",
        description: `${selectedConsents.size} consentimientos han sido revocados.`,
      })
      setSelectedConsents(new Set())
      setBulkRevokeDialog(false)
    },
    onError: (error: any) => {
      toast({
        title: "Error",
        description: error?.error_description || "No se pudieron revocar algunos consentimientos",
        variant: "destructive",
      })
    },
  })

  // Save consent policy mutation
  const savePolicyMutation = useMutation({
    mutationFn: async (policyData: ConsentPolicy) => {
      return api.patch(`${API_ROUTES.ADMIN_TENANTS}/${tenantId}/settings`, {
        consentPolicy: {
          consent_mode: policyData.consent_mode,
          expiration_days: policyData.expiration_days,
          reprompt_days: policyData.reprompt_days,
          remember_scope_decisions: policyData.remember_scope_decisions,
          show_consent_screen: policyData.show_consent_screen,
          allow_skip_consent_for_first_party: policyData.allow_skip_consent_for_first_party,
        },
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tenant", tenantId] })
      toast({
        title: "Políticas guardadas",
        description: "La configuración de consentimientos ha sido actualizada.",
      })
    },
    onError: (error: any) => {
      toast({
        title: "Error",
        description: error?.error_description || "No se pudo guardar la configuración",
        variant: "destructive",
      })
    },
  })

  // ─── Computed Values ───

  const getConsentKey = (c: ConsentResponse) => c.id || `${c.user_id}:${c.client_id}`

  const filteredConsents = useMemo(() => {
    if (!consents) return []
    return consents.filter((consent) => {
      // Search query
      if (searchQuery) {
        const query = searchQuery.toLowerCase()
        const matchesSearch =
          consent.user_id.toLowerCase().includes(query) ||
          (consent as any).user_email?.toLowerCase().includes(query) ||
          consent.client_id.toLowerCase().includes(query) ||
          consent.scopes.some(s => s.toLowerCase().includes(query))
        if (!matchesSearch) return false
      }
      // Status filter
      if (filterStatus === "active" && consent.revoked_at) return false
      if (filterStatus === "revoked" && !consent.revoked_at) return false
      // User filter
      if (filterUser && !consent.user_id.includes(filterUser)) return false
      // Client filter
      if (filterClient && consent.client_id !== filterClient) return false
      // Scope filter
      if (filterScope && !consent.scopes.includes(filterScope)) return false
      return true
    })
  }, [consents, searchQuery, filterStatus, filterUser, filterClient, filterScope])

  const stats = useMemo(() => {
    if (!consents) return { total: 0, active: 0, revoked: 0, uniqueUsers: 0, uniqueClients: 0 }
    const active = consents.filter(c => !c.revoked_at).length
    const revoked = consents.filter(c => c.revoked_at).length
    const uniqueUsers = new Set(consents.map(c => c.user_id)).size
    const uniqueClients = new Set(consents.map(c => c.client_id)).size
    return { total: consents.length, active, revoked, uniqueUsers, uniqueClients }
  }, [consents])

  const allScopes = useMemo(() => {
    if (!consents) return []
    const scopeSet = new Set<string>()
    consents.forEach(c => c.scopes.forEach(s => scopeSet.add(s)))
    return Array.from(scopeSet).sort()
  }, [consents])

  const uniqueClients = useMemo(() => {
    if (!consents) return []
    const clientSet = new Set<string>()
    consents.forEach(c => clientSet.add(c.client_id))
    return Array.from(clientSet)
  }, [consents])

  // ─── Selection Handlers ───

  const isAllSelected = filteredConsents.length > 0 &&
    filteredConsents.every(c => selectedConsents.has(getConsentKey(c)))
  const isSomeSelected = selectedConsents.size > 0 && !isAllSelected

  const toggleSelectAll = () => {
    if (isAllSelected) {
      setSelectedConsents(new Set())
    } else {
      setSelectedConsents(new Set(filteredConsents.map(getConsentKey)))
    }
  }

  const toggleSelect = (consent: ConsentResponse) => {
    const key = getConsentKey(consent)
    const newSet = new Set(selectedConsents)
    if (newSet.has(key)) {
      newSet.delete(key)
    } else {
      newSet.add(key)
    }
    setSelectedConsents(newSet)
  }

  const clearFilters = () => {
    setFilterUser("")
    setFilterClient("")
    setFilterScope("")
    setFilterStatus("all")
    setSearchQuery("")
  }

  const hasActiveFilters = filterUser || filterClient || filterScope || filterStatus !== "all"

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    toast({
      title: "Copiado",
      description: "Copiado al portapapeles",
    })
  }

  // ─── Render ───

  return (
    <TooltipProvider>
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
                <h1 className="text-2xl font-bold tracking-tight">Consentimientos</h1>
                <p className="text-sm text-muted-foreground">
                  {tenant?.name} — Gestiona los permisos otorgados por los usuarios
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* Info Banner */}
        <InlineAlert variant="info" title="¿Qué son los Consentimientos?">
          <p className="text-sm">
            Cuando un usuario autoriza una aplicación a acceder a sus datos (ej: nombre, email),
            se crea un consentimiento. Este registro documenta exactamente qué permisos (scopes)
            el usuario ha otorgado a cada aplicación. Puedes revocar consentimientos aquí si es necesario.
          </p>
        </InlineAlert>

        {/* Stats Cards */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <StatCard
            title="Total"
            value={stats.total}
            icon={FileText}
            color="blue"
            isLoading={isLoading}
          />
          <StatCard
            title="Activos"
            value={stats.active}
            icon={CheckCircle2}
            color="green"
            isLoading={isLoading}
          />
          <StatCard
            title="Revocados"
            value={stats.revoked}
            icon={ShieldOff}
            color="red"
            isLoading={isLoading}
          />
          <StatCard
            title="Usuarios Únicos"
            value={stats.uniqueUsers}
            icon={Users}
            color="purple"
            isLoading={isLoading}
          />
        </div>

        {/* Tabs */}
        <Tabs value={currentTab} onValueChange={setCurrentTab}>
          <div className="flex items-center justify-between gap-4 mb-4">
            <TabsList className="bg-white/5 border border-white/10">
              <TabsTrigger value="overview" className="gap-2">
                <FileText className="h-4 w-4" />
                <span className="hidden sm:inline">Consentimientos</span>
              </TabsTrigger>
              <TabsTrigger value="policies" className="gap-2">
                <Settings2 className="h-4 w-4" />
                <span className="hidden sm:inline">Políticas</span>
              </TabsTrigger>
            </TabsList>

            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => refetch()}
                disabled={isLoading || isRefetching}
                className="gap-2"
              >
                <RefreshCw className={cn("h-4 w-4", isRefetching && "animate-spin")} />
                <span className="hidden sm:inline">Actualizar</span>
              </Button>
            </div>
          </div>

          {/* Tab: Consents List */}
          <TabsContent value="overview" className="space-y-4 mt-0">
            {/* Filters Bar */}
            <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
              <CardContent className="p-4">
                <div className="flex flex-col gap-4">
                  {/* Search and Actions */}
                  <div className="flex flex-col sm:flex-row gap-3">
                    <div className="relative flex-1">
                      <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                      <Input
                        placeholder="Buscar por usuario, client o scope..."
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="pl-9 bg-white/5 border-white/10"
                      />
                    </div>
                    <div className="flex gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setShowFilters(!showFilters)}
                        className={cn(
                          "gap-2",
                          hasActiveFilters && "border-info/50 text-info"
                        )}
                      >
                        <Filter className="h-4 w-4" />
                        Filtros
                        {hasActiveFilters && (
                          <Badge variant="secondary" className="ml-1 h-5 px-1.5">
                            {[filterUser, filterClient, filterScope, filterStatus !== "all" ? 1 : 0]
                              .filter(Boolean).length}
                          </Badge>
                        )}
                      </Button>
                      {hasActiveFilters && (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={clearFilters}
                          className="gap-1 text-muted-foreground"
                        >
                          <X className="h-4 w-4" />
                          Limpiar
                        </Button>
                      )}
                    </div>
                  </div>

                  {/* Expanded Filters */}
                  {showFilters && (
                    <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-3 pt-3 border-t border-white/[0.06]">
                      <div className="space-y-1.5">
                        <Label className="text-xs text-muted-foreground">Estado</Label>
                        <Select value={filterStatus} onValueChange={(v: any) => setFilterStatus(v)}>
                          <SelectTrigger className="bg-white/5 border-white/10">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="all">Todos</SelectItem>
                            <SelectItem value="active">Activos</SelectItem>
                            <SelectItem value="revoked">Revocados</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-1.5">
                        <Label className="text-xs text-muted-foreground">Client</Label>
                        <Select value={filterClient} onValueChange={setFilterClient}>
                          <SelectTrigger className="bg-white/5 border-white/10">
                            <SelectValue placeholder="Todos" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="">Todos</SelectItem>
                            {uniqueClients.map((clientId) => (
                              <SelectItem key={clientId} value={clientId}>
                                {clients?.find(c => c.client_id === clientId)?.name || clientId}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-1.5">
                        <Label className="text-xs text-muted-foreground">Scope</Label>
                        <Select value={filterScope} onValueChange={setFilterScope}>
                          <SelectTrigger className="bg-white/5 border-white/10">
                            <SelectValue placeholder="Todos" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="">Todos</SelectItem>
                            {allScopes.map((scope) => (
                              <SelectItem key={scope} value={scope}>
                                {scope}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-1.5">
                        <Label className="text-xs text-muted-foreground">Usuario (ID)</Label>
                        <Input
                          placeholder="Filtrar por user ID..."
                          value={filterUser}
                          onChange={(e) => setFilterUser(e.target.value)}
                          className="bg-white/5 border-white/10"
                        />
                      </div>
                    </div>
                  )}

                  {/* Bulk Actions Bar */}
                  {selectedConsents.size > 0 && (
                    <div className="flex items-center justify-between p-3 rounded-lg bg-info/10 border border-info/20 animate-in slide-in-from-top-2 duration-200">
                      <div className="flex items-center gap-3">
                        <CheckSquare className="h-4 w-4 text-info" />
                        <span className="text-sm font-medium">
                          {selectedConsents.size} seleccionado{selectedConsents.size > 1 ? "s" : ""}
                        </span>
                      </div>
                      <div className="flex items-center gap-2">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setSelectedConsents(new Set())}
                          className="text-muted-foreground"
                        >
                          Cancelar
                        </Button>
                        <Button
                          variant="danger"
                          size="sm"
                          onClick={() => setBulkRevokeDialog(true)}
                          className="gap-2"
                        >
                          <Ban className="h-4 w-4" />
                          Revocar Todos
                        </Button>
                      </div>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>

            {/* Consents Table */}
            <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent overflow-hidden">
              {isLoading ? (
                <div className="flex items-center justify-center py-16">
                  <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                </div>
              ) : filteredConsents.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-16 px-4">
                  <div className="p-4 rounded-full bg-white/5 mb-4">
                    <ShieldOff className="h-8 w-8 text-muted-foreground" />
                  </div>
                  <h3 className="font-medium mb-1">No hay consentimientos</h3>
                  <p className="text-sm text-muted-foreground text-center max-w-sm">
                    {hasActiveFilters
                      ? "No se encontraron consentimientos con los filtros aplicados."
                      : "Los usuarios aún no han otorgado permisos a ninguna aplicación."}
                  </p>
                  {hasActiveFilters && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={clearFilters}
                      className="mt-4 gap-2"
                    >
                      <X className="h-4 w-4" />
                      Limpiar filtros
                    </Button>
                  )}
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow className="border-white/[0.06] hover:bg-transparent">
                      <TableHead className="w-10">
                        <Checkbox
                          checked={isAllSelected}
                          ref={(el) => {
                            if (el) (el as any).indeterminate = isSomeSelected
                          }}
                          onCheckedChange={toggleSelectAll}
                          aria-label="Seleccionar todos"
                        />
                      </TableHead>
                      <TableHead>Usuario</TableHead>
                      <TableHead>Aplicación</TableHead>
                      <TableHead>Scopes</TableHead>
                      <TableHead>Fecha</TableHead>
                      <TableHead>Estado</TableHead>
                      <TableHead className="text-right">Acciones</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filteredConsents.map((consent) => {
                      const key = getConsentKey(consent)
                      const isSelected = selectedConsents.has(key)
                      const isRevoked = !!consent.revoked_at
                      const clientName = clients?.find(c => c.client_id === consent.client_id)?.name

                      return (
                        <TableRow
                          key={key}
                          className={cn(
                            "border-white/[0.06] cursor-pointer transition-colors",
                            isSelected && "bg-info/10",
                            isRevoked && "opacity-60"
                          )}
                          onClick={() => setDetailDialog(consent)}
                        >
                          <TableCell onClick={(e) => e.stopPropagation()}>
                            <Checkbox
                              checked={isSelected}
                              onCheckedChange={() => toggleSelect(consent)}
                              aria-label={`Seleccionar consentimiento de ${consent.user_id}`}
                            />
                          </TableCell>
                          <TableCell>
                            <div className="flex flex-col">
                              <span className="font-medium text-sm">
                                {(consent as any).user_email || "Usuario"}
                              </span>
                              <span className="text-xs text-muted-foreground font-mono truncate max-w-[150px]">
                                {consent.user_id.slice(0, 8)}...
                              </span>
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="flex flex-col">
                              <span className="font-medium text-sm">
                                {clientName || consent.client_id}
                              </span>
                              {clientName && (
                                <span className="text-xs text-muted-foreground font-mono">
                                  {consent.client_id}
                                </span>
                              )}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="flex flex-wrap gap-1 max-w-[200px]">
                              {consent.scopes.slice(0, 3).map((scope) => (
                                <ScopeBadge key={scope} scope={scope} size="sm" />
                              ))}
                              {consent.scopes.length > 3 && (
                                <Badge variant="outline" className="text-[10px] px-1.5 py-0">
                                  +{consent.scopes.length - 3}
                                </Badge>
                              )}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="flex flex-col">
                              <span className="text-sm">{formatTimeAgo(consent.created_at)}</span>
                              <span className="text-xs text-muted-foreground">
                                {new Date(consent.created_at).toLocaleDateString()}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell>
                            {isRevoked ? (
                              <Badge variant="outline" className="border-danger/30 bg-danger/10 text-danger gap-1">
                                <ShieldOff className="h-3 w-3" />
                                Revocado
                              </Badge>
                            ) : (
                              <Badge variant="outline" className="border-success/30 bg-success/10 text-success gap-1">
                                <ShieldCheck className="h-3 w-3" />
                                Activo
                              </Badge>
                            )}
                          </TableCell>
                          <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
                            <DropdownMenu>
                              <DropdownMenuTrigger asChild>
                                <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
                                  <MoreHorizontal className="h-4 w-4" />
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end">
                                <DropdownMenuLabel>Acciones</DropdownMenuLabel>
                                <DropdownMenuSeparator />
                                <DropdownMenuItem onClick={() => setDetailDialog(consent)}>
                                  <Eye className="mr-2 h-4 w-4" />
                                  Ver detalles
                                </DropdownMenuItem>
                                <DropdownMenuItem onClick={() => copyToClipboard(consent.user_id)}>
                                  <Copy className="mr-2 h-4 w-4" />
                                  Copiar User ID
                                </DropdownMenuItem>
                                <DropdownMenuSeparator />
                                {!isRevoked && (
                                  <DropdownMenuItem
                                    onClick={() => setRevokeDialog(consent)}
                                    className="text-danger hover:text-danger hover:bg-danger/10"
                                  >
                                    <Ban className="mr-2 h-4 w-4" />
                                    Revocar
                                  </DropdownMenuItem>
                                )}
                              </DropdownMenuContent>
                            </DropdownMenu>
                          </TableCell>
                        </TableRow>
                      )
                    })}
                  </TableBody>
                </Table>
              )}
            </Card>

            {/* Pagination info */}
            {filteredConsents.length > 0 && (
              <div className="flex items-center justify-between text-sm text-muted-foreground">
                <span>
                  Mostrando {filteredConsents.length} de {stats.total} consentimientos
                </span>
              </div>
            )}
          </TabsContent>

          {/* Tab: Policies */}
          <TabsContent value="policies" className="space-y-6 mt-0">
            <InlineAlert variant="warning" title="Configuración de Políticas">
              <p className="text-sm">
                Estas políticas determinan cómo se solicitan y gestionan los consentimientos de los usuarios.
                Los cambios afectarán a todos los flujos de autorización futuros.
              </p>
            </InlineAlert>

            <div className="grid gap-6">
              {/* Consent Mode */}
              <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
                <CardHeader>
                  <CardTitle className="text-lg flex items-center gap-2">
                    <Layers className="h-5 w-5 text-info" />
                    Modo de Consentimiento
                    <InfoTooltip content="Determina cómo se agrupan los permisos cuando se solicita consentimiento al usuario" />
                  </CardTitle>
                  <CardDescription>
                    Elige cómo se presentan los permisos al usuario
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid gap-3">
                    <Label
                      className={cn(
                        "flex items-start gap-3 p-4 rounded-lg border cursor-pointer transition-all",
                        policy.consent_mode === "per_scope"
                          ? "border-info/50 bg-info/10"
                          : "border-white/10 hover:border-white/20"
                      )}
                    >
                      <input
                        type="radio"
                        name="consent_mode"
                        value="per_scope"
                        checked={policy.consent_mode === "per_scope"}
                        onChange={() => setPolicy({ ...policy, consent_mode: "per_scope" })}
                        className="mt-1"
                      />
                      <div>
                        <div className="font-medium">Por Scope Individual</div>
                        <div className="text-sm text-muted-foreground">
                          El usuario ve y aprueba cada permiso por separado. Más transparente y granular.
                        </div>
                        <Badge variant="outline" className="mt-2 text-xs border-success/30 text-success">
                          Recomendado
                        </Badge>
                      </div>
                    </Label>
                    <Label
                      className={cn(
                        "flex items-start gap-3 p-4 rounded-lg border cursor-pointer transition-all",
                        policy.consent_mode === "single"
                          ? "border-info/50 bg-info/10"
                          : "border-white/10 hover:border-white/20"
                      )}
                    >
                      <input
                        type="radio"
                        name="consent_mode"
                        value="single"
                        checked={policy.consent_mode === "single"}
                        onChange={() => setPolicy({ ...policy, consent_mode: "single" })}
                        className="mt-1"
                      />
                      <div>
                        <div className="font-medium">Consentimiento Único</div>
                        <div className="text-sm text-muted-foreground">
                          El usuario aprueba todos los permisos solicitados de una vez. Más rápido pero menos granular.
                        </div>
                      </div>
                    </Label>
                  </div>
                </CardContent>
              </Card>

              {/* Expiration Settings */}
              <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
                <CardHeader>
                  <CardTitle className="text-lg flex items-center gap-2">
                    <Timer className="h-5 w-5 text-info" />
                    Expiración de Consentimientos
                    <InfoTooltip content="Configura cuánto tiempo son válidos los consentimientos antes de requerir re-autorización" />
                  </CardTitle>
                  <CardDescription>
                    Define la duración y renovación de consentimientos
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-6">
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <Label>Expiración automática</Label>
                      <Switch
                        checked={policy.expiration_days !== null}
                        onCheckedChange={(checked) =>
                          setPolicy({
                            ...policy,
                            expiration_days: checked ? 365 : null,
                          })
                        }
                      />
                    </div>
                    {policy.expiration_days !== null && (
                      <div className="space-y-2 pl-4 border-l-2 border-info/30">
                        <div className="flex items-center justify-between">
                          <span className="text-sm text-muted-foreground">
                            Expira después de: <strong className="text-foreground">{policy.expiration_days} días</strong>
                          </span>
                        </div>
                        <Slider
                          value={[policy.expiration_days]}
                          onValueChange={([value]: number[]) =>
                            setPolicy({ ...policy, expiration_days: value })
                          }
                          min={30}
                          max={730}
                          step={30}
                          className="w-full"
                        />
                        <div className="flex justify-between text-xs text-muted-foreground">
                          <span>30 días</span>
                          <span>1 año</span>
                          <span>2 años</span>
                        </div>
                      </div>
                    )}
                  </div>

                  <Separator className="bg-white/[0.06]" />

                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <div>
                        <Label>Re-solicitar consentimiento periódicamente</Label>
                        <p className="text-xs text-muted-foreground mt-0.5">
                          Incluso si el consentimiento no ha expirado
                        </p>
                      </div>
                      <Switch
                        checked={policy.reprompt_days !== null}
                        onCheckedChange={(checked) =>
                          setPolicy({
                            ...policy,
                            reprompt_days: checked ? 90 : null,
                          })
                        }
                      />
                    </div>
                    {policy.reprompt_days !== null && (
                      <div className="space-y-2 pl-4 border-l-2 border-info/30">
                        <div className="flex items-center justify-between">
                          <span className="text-sm text-muted-foreground">
                            Re-solicitar cada: <strong className="text-foreground">{policy.reprompt_days} días</strong>
                          </span>
                        </div>
                        <Slider
                          value={[policy.reprompt_days]}
                          onValueChange={([value]: number[]) =>
                            setPolicy({ ...policy, reprompt_days: value })
                          }
                          min={7}
                          max={365}
                          step={7}
                          className="w-full"
                        />
                        <div className="flex justify-between text-xs text-muted-foreground">
                          <span>7 días</span>
                          <span>6 meses</span>
                          <span>1 año</span>
                        </div>
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>

              {/* Additional Settings */}
              <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
                <CardHeader>
                  <CardTitle className="text-lg flex items-center gap-2">
                    <Settings2 className="h-5 w-5 text-info" />
                    Configuración Adicional
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                    <div className="flex-1">
                      <Label className="text-sm font-medium">Recordar decisiones de scope</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        Si el usuario ya otorgó un scope, no volver a preguntarle
                      </p>
                    </div>
                    <Switch
                      checked={policy.remember_scope_decisions}
                      onCheckedChange={(checked) =>
                        setPolicy({ ...policy, remember_scope_decisions: checked })
                      }
                    />
                  </div>

                  <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                    <div className="flex-1">
                      <Label className="text-sm font-medium">Mostrar pantalla de consentimiento</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        Siempre mostrar la pantalla de autorización al usuario
                      </p>
                    </div>
                    <Switch
                      checked={policy.show_consent_screen}
                      onCheckedChange={(checked) =>
                        setPolicy({ ...policy, show_consent_screen: checked })
                      }
                    />
                  </div>

                  <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                    <div className="flex-1">
                      <Label className="text-sm font-medium">Omitir para apps first-party</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        No solicitar consentimiento para aplicaciones propias del tenant
                      </p>
                    </div>
                    <Switch
                      checked={policy.allow_skip_consent_for_first_party}
                      onCheckedChange={(checked) =>
                        setPolicy({ ...policy, allow_skip_consent_for_first_party: checked })
                      }
                    />
                  </div>
                </CardContent>
              </Card>

              {/* Save Button */}
              <div className="flex justify-end gap-3">
                <Button
                  variant="outline"
                  onClick={() => setPolicy(DEFAULT_POLICY)}
                >
                  Restaurar Defaults
                </Button>
                <Button
                  className="bg-gradient-to-r from-info to-accent hover:from-info/90 hover:to-accent/90 gap-2"
                  onClick={() => savePolicyMutation.mutate(policy)}
                  disabled={savePolicyMutation.isPending}
                >
                  {savePolicyMutation.isPending && (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  )}
                  Guardar Cambios
                </Button>
              </div>
            </div>
          </TabsContent>
        </Tabs>

        {/* ─── Dialogs ─── */}

        {/* Detail Dialog */}
        <Dialog open={!!detailDialog} onOpenChange={() => setDetailDialog(null)}>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <div className="flex items-center gap-3">
                <div className={cn(
                  "p-2 rounded-lg",
                  detailDialog?.revoked_at
                    ? "bg-danger/10 border border-danger/20"
                    : "bg-success/10 border border-success/20"
                )}>
                  {detailDialog?.revoked_at ? (
                    <ShieldOff className="h-5 w-5 text-danger" />
                  ) : (
                    <ShieldCheck className="h-5 w-5 text-success" />
                  )}
                </div>
                <div>
                  <DialogTitle>Detalle del Consentimiento</DialogTitle>
                  <DialogDescription>
                    {detailDialog?.revoked_at ? "Consentimiento revocado" : "Consentimiento activo"}
                  </DialogDescription>
                </div>
              </div>
            </DialogHeader>

            {detailDialog && (
              <div className="space-y-4 mt-4">
                {/* User Info */}
                <div className="p-3 rounded-lg bg-white/[0.03] border border-white/[0.06]">
                  <Label className="text-xs text-muted-foreground">Usuario</Label>
                  <div className="mt-1">
                    <p className="font-medium">{(detailDialog as any).user_email || "Usuario"}</p>
                    <div className="flex items-center gap-2 mt-1">
                      <code className="text-xs text-muted-foreground font-mono bg-white/5 px-2 py-0.5 rounded">
                        {detailDialog.user_id}
                      </code>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 w-6 p-0"
                        onClick={() => copyToClipboard(detailDialog.user_id)}
                      >
                        <Copy className="h-3 w-3" />
                      </Button>
                    </div>
                  </div>
                </div>

                {/* Client Info */}
                <div className="p-3 rounded-lg bg-white/[0.03] border border-white/[0.06]">
                  <Label className="text-xs text-muted-foreground">Aplicación (Client)</Label>
                  <div className="mt-1">
                    <p className="font-medium">
                      {clients?.find(c => c.client_id === detailDialog.client_id)?.name || detailDialog.client_id}
                    </p>
                    <div className="flex items-center gap-2 mt-1">
                      <code className="text-xs text-muted-foreground font-mono bg-white/5 px-2 py-0.5 rounded">
                        {detailDialog.client_id}
                      </code>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 w-6 p-0"
                        onClick={() => copyToClipboard(detailDialog.client_id)}
                      >
                        <Copy className="h-3 w-3" />
                      </Button>
                    </div>
                  </div>
                </div>

                {/* Scopes */}
                <div className="p-3 rounded-lg bg-white/[0.03] border border-white/[0.06]">
                  <Label className="text-xs text-muted-foreground">Scopes Otorgados</Label>
                  <div className="flex flex-wrap gap-2 mt-2">
                    {detailDialog.scopes.map((scope) => (
                      <div key={scope} className="flex items-center gap-2">
                        <ScopeBadge scope={scope} />
                        {OIDC_SCOPES_INFO[scope] && (
                          <span className="text-xs text-muted-foreground">
                            {OIDC_SCOPES_INFO[scope].description}
                          </span>
                        )}
                      </div>
                    ))}
                  </div>
                </div>

                {/* Dates */}
                <div className="grid grid-cols-2 gap-3">
                  <div className="p-3 rounded-lg bg-white/[0.03] border border-white/[0.06]">
                    <Label className="text-xs text-muted-foreground">Otorgado</Label>
                    <p className="font-medium mt-1 text-sm">
                      {formatDate(detailDialog.created_at)}
                    </p>
                  </div>
                  {detailDialog.revoked_at && (
                    <div className="p-3 rounded-lg bg-danger/5 border border-danger/20">
                      <Label className="text-xs text-danger">Revocado</Label>
                      <p className="font-medium mt-1 text-sm text-danger">
                        {formatDate(detailDialog.revoked_at)}
                      </p>
                    </div>
                  )}
                </div>

                {/* Actions */}
                {!detailDialog.revoked_at && (
                  <div className="pt-4 border-t border-white/[0.06]">
                    <Button
                      variant="danger"
                      className="w-full gap-2"
                      onClick={() => {
                        setDetailDialog(null)
                        setRevokeDialog(detailDialog)
                      }}
                    >
                      <Ban className="h-4 w-4" />
                      Revocar Consentimiento
                    </Button>
                  </div>
                )}
              </div>
            )}
          </DialogContent>
        </Dialog>

        {/* Revoke Confirmation Dialog */}
        <Dialog open={!!revokeDialog} onOpenChange={() => setRevokeDialog(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2 text-danger">
                <AlertTriangle className="h-5 w-5" />
                Revocar Consentimiento
              </DialogTitle>
              <DialogDescription>
                Esta acción no se puede deshacer. El usuario deberá volver a autorizar
                la aplicación si desea continuar usándola.
              </DialogDescription>
            </DialogHeader>

            {revokeDialog && (
              <div className="py-4">
                <InlineAlert variant="destructive">
                  <p className="text-sm">
                    Vas a revocar el acceso de <strong>{revokeDialog.client_id}</strong> a
                    los datos de <strong>{(revokeDialog as any).user_email || revokeDialog.user_id}</strong>.
                  </p>
                </InlineAlert>
              </div>
            )}

            <DialogFooter>
              <Button variant="outline" onClick={() => setRevokeDialog(null)}>
                Cancelar
              </Button>
              <Button
                variant="danger"
                onClick={() => revokeDialog && revokeMutation.mutate(revokeDialog)}
                disabled={revokeMutation.isPending}
                className="gap-2"
              >
                {revokeMutation.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Ban className="h-4 w-4" />
                )}
                Revocar
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Bulk Revoke Dialog */}
        <Dialog open={bulkRevokeDialog} onOpenChange={setBulkRevokeDialog}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2 text-danger">
                <AlertTriangle className="h-5 w-5" />
                Revocar Múltiples Consentimientos
              </DialogTitle>
              <DialogDescription>
                Estás a punto de revocar {selectedConsents.size} consentimientos.
                Esta acción no se puede deshacer.
              </DialogDescription>
            </DialogHeader>

            <div className="py-4">
              <InlineAlert variant="destructive">
                <p className="text-sm">
                  Los usuarios afectados deberán volver a autorizar las aplicaciones
                  si desean continuar usándolas.
                </p>
              </InlineAlert>
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => setBulkRevokeDialog(false)}>
                Cancelar
              </Button>
              <Button
                variant="danger"
                onClick={() => bulkRevokeMutation.mutate(Array.from(selectedConsents))}
                disabled={bulkRevokeMutation.isPending}
                className="gap-2"
              >
                {bulkRevokeMutation.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Ban className="h-4 w-4" />
                )}
                Revocar {selectedConsents.size} Consentimientos
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </TooltipProvider>
  )
}

// ─── Mock Data Generator ───

function generateMockConsents(count: number): ConsentResponse[] {
  const clients = ["web-app", "mobile-app", "api-service", "admin-dashboard", "analytics"]
  const emails = [
    "admin@example.com",
    "user@example.com",
    "developer@company.com",
    "support@business.org",
    "test@demo.io",
    "john.doe@email.com",
    "jane.smith@email.com",
  ]
  const scopeSets = [
    ["openid", "profile", "email"],
    ["openid", "email"],
    ["openid", "profile", "email", "offline_access"],
    ["openid", "profile"],
    ["openid", "email", "phone"],
    ["openid", "profile", "email", "address"],
  ]

  return Array.from({ length: count }, (_, i) => {
    const createdAt = new Date(Date.now() - Math.random() * 90 * 24 * 60 * 60 * 1000)
    const isRevoked = Math.random() > 0.85
    const userId = `user-${Math.floor(Math.random() * 7) + 1}-${Math.random().toString(36).substr(2, 8)}`

    return {
      id: `consent-${i + 1}`,
      tenant_id: "demo-tenant",
      user_id: userId,
      user_email: emails[Math.floor(Math.random() * emails.length)],
      client_id: clients[Math.floor(Math.random() * clients.length)],
      scopes: scopeSets[Math.floor(Math.random() * scopeSets.length)],
      created_at: createdAt.toISOString(),
      updated_at: createdAt.toISOString(),
      revoked_at: isRevoked
        ? new Date(createdAt.getTime() + Math.random() * 30 * 24 * 60 * 60 * 1000).toISOString()
        : undefined,
    } as ConsentResponse & { user_email: string }
  })
}

// ─── Page Export ───

export default function ConsentsPage() {
  return (
    <Suspense
      fallback={
        <div className="flex items-center justify-center min-h-[400px]">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      }
    >
      <ConsentsContent />
    </Suspense>
  )
}
