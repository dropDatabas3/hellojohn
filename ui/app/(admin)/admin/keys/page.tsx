"use client"

import { useState, useMemo } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  Key, RotateCw, CheckCircle, Copy, Check, Info, Shield,
  Clock, AlertTriangle, HelpCircle, History, Globe, Building2,
  ExternalLink, Timer, Zap, Hash
} from "lucide-react"
import { api } from "@/lib/api"
import { API_ROUTES } from "@/lib/routes"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { useToast } from "@/hooks/use-toast"
import { Badge } from "@/components/ui/badge"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { cn } from "@/lib/utils"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import type { Tenant } from "@/lib/types"

// ─── Types ───

type KeyStatus = "active" | "retiring" | "revoked"

type KeyInfo = {
  kid: string
  alg: string
  use: string
  status: KeyStatus
  created_at: string
  retired_at?: string
  grace_seconds?: number
  tenant_id?: string
}


// ─── Helper Functions ───

const formatTimeAgo = (date: string) => {
  const seconds = Math.floor((Date.now() - new Date(date).getTime()) / 1000)
  if (seconds < 60) return "hace menos de un minuto"
  if (seconds < 3600) return `hace ${Math.floor(seconds / 60)} min`
  if (seconds < 86400) return `hace ${Math.floor(seconds / 3600)}h`
  return `hace ${Math.floor(seconds / 86400)}d`
}

const formatGracePeriodRemaining = (retiredAt: string, graceSeconds: number): string => {
  const retiredTime = new Date(retiredAt).getTime()
  const expiresAt = retiredTime + graceSeconds * 1000
  const remaining = expiresAt - Date.now()

  if (remaining <= 0) return "Expirado"

  const hours = Math.floor(remaining / (1000 * 60 * 60))
  const minutes = Math.floor((remaining % (1000 * 60 * 60)) / (1000 * 60))

  if (hours > 0) return `${hours}h ${minutes}m restantes`
  return `${minutes}m restantes`
}

const getGraceProgress = (retiredAt: string, graceSeconds: number): number => {
  const retiredTime = new Date(retiredAt).getTime()
  const expiresAt = retiredTime + graceSeconds * 1000
  const total = graceSeconds * 1000
  const elapsed = Date.now() - retiredTime
  return Math.min(100, Math.max(0, (elapsed / total) * 100))
}

const getStatusColor = (status: KeyStatus) => {
  switch (status) {
    case "active": return "bg-emerald-500/10 text-emerald-600 border-emerald-500/20"
    case "retiring": return "bg-amber-500/10 text-amber-600 border-amber-500/20"
    case "revoked": return "bg-red-500/10 text-red-500 border-red-500/20"
    default: return "bg-zinc-500/10 text-zinc-500 border-zinc-500/20"
  }
}

const getStatusLabel = (status: KeyStatus) => {
  switch (status) {
    case "active": return "Activa"
    case "retiring": return "En gracia"
    case "revoked": return "Revocada"
    default: return "Desconocido"
  }
}

// ─── Info Tooltip Component ───

function InfoTooltip({ content }: { content: string }) {
  return (
    <TooltipProvider delayDuration={200}>
      <Tooltip>
        <TooltipTrigger asChild>
          <button type="button" className="ml-1.5 inline-flex">
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
  color?: "zinc" | "emerald" | "blue" | "amber"
}) {
  const colorClasses = {
    zinc: "from-zinc-500/10 to-zinc-500/5 border-zinc-500/10",
    emerald: "from-emerald-500/10 to-emerald-500/5 border-emerald-500/10",
    blue: "from-blue-500/10 to-blue-500/5 border-blue-500/10",
    amber: "from-amber-500/10 to-amber-500/5 border-amber-500/10",
  }
  const iconColors = {
    zinc: "text-zinc-500",
    emerald: "text-emerald-500",
    blue: "text-blue-500",
    amber: "text-amber-500",
  }

  return (
    <div className={cn(
      "rounded-xl border bg-gradient-to-br p-4",
      colorClasses[color]
    )}>
      <div className="flex items-center gap-3">
        <Icon className={cn("h-5 w-5", iconColors[color])} />
        <div>
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className="text-lg font-semibold">{value}</p>
          {subValue && (
            <p className="text-xs text-muted-foreground">{subValue}</p>
          )}
        </div>
      </div>
    </div>
  )
}

// ─── Key Card Component ───

function KeyCard({
  keyInfo,
  onCopy,
  onRevoke,
  copiedKid,
  isExpanded = false
}: {
  keyInfo: KeyInfo
  onCopy: (kid: string) => void
  onRevoke: (kid: string) => void
  copiedKid: string | null
  isExpanded?: boolean
}) {
  const isRetiring = keyInfo.status === "retiring"
  const gracePeriodRemaining = isRetiring && keyInfo.retired_at && keyInfo.grace_seconds
    ? formatGracePeriodRemaining(keyInfo.retired_at, keyInfo.grace_seconds)
    : null
  const graceProgress = isRetiring && keyInfo.retired_at && keyInfo.grace_seconds
    ? getGraceProgress(keyInfo.retired_at, keyInfo.grace_seconds)
    : 0

  return (
    <div className={cn(
      "rounded-xl border transition-all",
      keyInfo.status === "active" && "border-emerald-500/30 bg-emerald-500/5",
      keyInfo.status === "retiring" && "border-amber-500/30 bg-amber-500/5",
      keyInfo.status === "revoked" && "border-red-500/30 bg-red-500/5 opacity-60"
    )}>
      <div className="p-4">
        {/* Header */}
        <div className="flex items-start justify-between mb-4">
          <div className="flex items-center gap-3">
            <div className={cn(
              "h-10 w-10 rounded-lg flex items-center justify-center",
              keyInfo.status === "active" && "bg-emerald-500/10",
              keyInfo.status === "retiring" && "bg-amber-500/10",
              keyInfo.status === "revoked" && "bg-red-500/10"
            )}>
              {keyInfo.status === "active" ? (
                <CheckCircle className="h-5 w-5 text-emerald-500" />
              ) : keyInfo.status === "retiring" ? (
                <Clock className="h-5 w-5 text-amber-500" />
              ) : (
                <AlertTriangle className="h-5 w-5 text-red-500" />
              )}
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="font-semibold">
                  {keyInfo.status === "active" ? "Clave Activa" : "Clave en Período de Gracia"}
                </span>
                <Badge variant="outline" className={getStatusColor(keyInfo.status)}>
                  {getStatusLabel(keyInfo.status)}
                </Badge>
              </div>
              <p className="text-xs text-muted-foreground">
                Creada {formatTimeAgo(keyInfo.created_at)}
              </p>
            </div>
          </div>
        </div>

        {/* Grace Period Progress */}
        {isRetiring && gracePeriodRemaining && (
          <div className="mb-4 p-3 rounded-lg bg-amber-500/10 border border-amber-500/20">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-2">
                <Timer className="h-4 w-4 text-amber-500" />
                <span className="text-sm font-medium text-amber-700 dark:text-amber-400">
                  Período de Gracia
                </span>
                <InfoTooltip content="Durante este período, los tokens firmados con esta clave siguen siendo válidos. Una vez finalizado, la clave dejará de incluirse en JWKS." />
              </div>
              <span className="text-sm font-medium text-amber-700 dark:text-amber-400">
                {gracePeriodRemaining}
              </span>
            </div>
            <div className="h-2 bg-amber-200/30 rounded-full overflow-hidden">
              <div
                className="h-full bg-amber-500 rounded-full transition-all duration-500"
                style={{ width: `${graceProgress}%` }}
              />
            </div>
          </div>
        )}

        {/* Key Details */}
        <div className="space-y-3">
          <div className="flex items-center justify-between py-2 border-b border-border/50">
            <span className="text-sm text-muted-foreground flex items-center gap-1">
              <Hash className="h-3.5 w-3.5" />
              Key ID (KID)
            </span>
            <div className="flex items-center gap-2">
              <code className="rounded bg-zinc-100 dark:bg-zinc-800 px-2 py-1 text-xs font-mono">
                {keyInfo.kid}
              </code>
              <Button
                variant="ghost"
                size="sm"
                className="h-7 w-7 p-0"
                onClick={() => onCopy(keyInfo.kid)}
              >
                {copiedKid === keyInfo.kid ? (
                  <Check className="h-3.5 w-3.5 text-emerald-500" />
                ) : (
                  <Copy className="h-3.5 w-3.5" />
                )}
              </Button>
            </div>
          </div>

          <div className="flex items-center justify-between py-2 border-b border-border/50">
            <span className="text-sm text-muted-foreground flex items-center gap-1">
              <Shield className="h-3.5 w-3.5" />
              Algoritmo
              <InfoTooltip content="EdDSA (Ed25519) es el algoritmo más moderno y seguro para firmas digitales, recomendado sobre RSA y ECDSA." />
            </span>
            <Badge className="bg-indigo-500/10 text-indigo-600 border-indigo-500/20">
              {keyInfo.alg}
            </Badge>
          </div>

          <div className="flex items-center justify-between py-2 border-b border-border/50">
            <span className="text-sm text-muted-foreground flex items-center gap-1">
              <Key className="h-3.5 w-3.5" />
              Uso
            </span>
            <Badge variant="outline">{keyInfo.use === "sig" ? "Firma" : keyInfo.use}</Badge>
          </div>

          <div className="flex items-center justify-between py-2">
            <span className="text-sm text-muted-foreground flex items-center gap-1">
              <Clock className="h-3.5 w-3.5" />
              Fecha de Creación
            </span>
            <span className="text-sm">
              {new Date(keyInfo.created_at).toLocaleDateString("es-AR", {
                year: "numeric",
                month: "short",
                day: "numeric",
                hour: "2-digit",
                minute: "2-digit"
              })}
            </span>
          </div>
        </div>

        {/* Revoke Button - Only for non-revoked keys */}
        {keyInfo.status !== "revoked" && (
          <div className="mt-4 pt-4 border-t border-border/50">
            <Button
              variant="destructive"
              size="sm"
              className="w-full gap-2"
              onClick={() => onRevoke(keyInfo.kid)}
            >
              <AlertTriangle className="h-4 w-4" />
              Revocar Clave Inmediatamente
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}

// ─── Main Component ───

export default function KeysPage() {
  const { t } = useI18n()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [rotateDialogOpen, setRotateDialogOpen] = useState(false)
  const [revokeDialogOpen, setRevokeDialogOpen] = useState(false)
  const [keyToRevoke, setKeyToRevoke] = useState<string | null>(null)
  const [copiedKid, setCopiedKid] = useState<string | null>(null)
  const [gracePeriodHours, setGracePeriodHours] = useState("24")
  const [selectedTenant, setSelectedTenant] = useState<string>("global")

  // Fetch tenants for per-tenant keys
  const { data: tenants } = useQuery({
    queryKey: ["tenants"],
    queryFn: () => api.get<{ tenants: Tenant[] }>(API_ROUTES.ADMIN_TENANTS),
    select: (data) => data.tenants || [],
  })

  // Fetch keys from admin endpoint
  const { data: keysData, isLoading: keysLoading, refetch: refetchKeys } = useQuery({
    queryKey: ["admin-keys", selectedTenant],
    queryFn: async () => {
      const url = selectedTenant === "global"
        ? API_ROUTES.ADMIN_KEYS
        : `${API_ROUTES.ADMIN_KEYS}?tenant_id=${selectedTenant}`
      return api.get<{ keys: KeyInfo[] }>(url)
    },
  })

  const keys = keysData?.keys || []

  const activeKey = keys.find(k => k.status === "active")
  const retiringKeys = keys.filter(k => k.status === "retiring")

  // Rotate mutation
  const rotateMutation = useMutation({
    mutationFn: async () => {
      const graceSeconds = parseInt(gracePeriodHours) * 3600
      return api.post<{ kid: string; grace_seconds: number; message: string }>(
        API_ROUTES.ADMIN_KEYS_ROTATE,
        {
          tenant_id: selectedTenant === "global" ? undefined : selectedTenant,
          grace_seconds: graceSeconds,
        }
      )
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ["admin-keys"] })
      refetchKeys()
      setRotateDialogOpen(false)
      toast({
        title: "Clave rotada exitosamente",
        description: `Nueva clave activa: ${data.kid.substring(0, 12)}...`,
        variant: "info",
      })
    },
    onError: (error: any) => {
      toast({
        title: "Error al rotar clave",
        description: error.message || "No se pudo rotar la clave",
        variant: "destructive",
      })
    },
  })

  // Revoke mutation
  const revokeMutation = useMutation({
    mutationFn: async (kid: string) => {
      return api.post(API_ROUTES.ADMIN_KEY_REVOKE(kid), {})
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin-keys"] })
      refetchKeys()
      setRevokeDialogOpen(false)
      setKeyToRevoke(null)
      toast({
        title: "Clave revocada",
        description: "La clave ha sido revocada inmediatamente",
        variant: "info",
      })
    },
    onError: (error: any) => {
      toast({
        title: "Error al revocar clave",
        description: error.message || "No se pudo revocar la clave",
        variant: "destructive",
      })
    },
  })

  const handleRevokeClick = (kid: string) => {
    setKeyToRevoke(kid)
    setRevokeDialogOpen(true)
  }

  const handleRevokeConfirm = () => {
    if (keyToRevoke) {
      revokeMutation.mutate(keyToRevoke)
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopiedKid(text)
    setTimeout(() => setCopiedKid(null), 2000)
    toast({
      title: "Copiado",
      description: "KID copiado al portapapeles",
      variant: "info",
    })
  }

  return (
    <div className="space-y-6 max-w-5xl">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Signing Keys</h1>
          <p className="text-muted-foreground">
            Administra las claves criptográficas para firma de JWT tokens
          </p>
        </div>
        <Button onClick={() => setRotateDialogOpen(true)} className="gap-2">
          <RotateCw className="h-4 w-4" />
          Rotar Clave
        </Button>
      </div>

      {/* Info Banner */}
      <div className="rounded-xl border border-blue-500/20 bg-gradient-to-r from-blue-500/10 via-blue-500/5 to-transparent p-4">
        <div className="flex gap-4">
          <div className="flex-shrink-0">
            <div className="h-10 w-10 rounded-lg bg-blue-500/10 flex items-center justify-center">
              <Info className="h-5 w-5 text-blue-500" />
            </div>
          </div>
          <div className="space-y-2">
            <h3 className="font-semibold text-blue-700 dark:text-blue-400 flex items-center gap-2">
              ¿Qué son las Signing Keys?
              <InfoTooltip content="Las claves de firma son pares de claves criptográficas asimétricas (pública/privada) usadas en OAuth2/OIDC" />
            </h3>
            <p className="text-sm text-muted-foreground leading-relaxed">
              Las <strong>signing keys</strong> se utilizan para <strong>firmar digitalmente</strong> los JWT tokens
              (access tokens, id tokens, refresh tokens). La clave privada firma los tokens y la clave pública
              (disponible en el endpoint <code className="text-xs bg-zinc-100 dark:bg-zinc-800 px-1 rounded">/.well-known/jwks.json</code>)
              permite a las aplicaciones <strong>verificar</strong> que los tokens son auténticos.
            </p>
            <div className="flex flex-wrap gap-4 pt-2">
              <div className="flex items-center gap-2 text-sm">
                <Shield className="h-4 w-4 text-emerald-500" />
                <span className="text-muted-foreground">Algoritmo: <strong>EdDSA (Ed25519)</strong></span>
              </div>
              <div className="flex items-center gap-2 text-sm">
                <Clock className="h-4 w-4 text-amber-500" />
                <span className="text-muted-foreground">Grace period por defecto: <strong>24 horas</strong></span>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Rotation Warning */}
      <div className="rounded-xl border border-amber-500/20 bg-amber-500/5 p-4">
        <div className="flex gap-3">
          <AlertTriangle className="h-5 w-5 text-amber-500 flex-shrink-0 mt-0.5" />
          <div>
            <h4 className="font-medium text-amber-700 dark:text-amber-400">
              ¿Qué pasa cuando roto una clave?
            </h4>
            <p className="text-sm text-muted-foreground mt-1">
              Al rotar, se genera una <strong>nueva clave activa</strong> para firmar tokens nuevos.
              La clave anterior pasa a un <strong>período de gracia</strong> durante el cual los tokens
              existentes siguen siendo válidos. Una vez finalizado el período, la clave se elimina del JWKS
              y los tokens firmados con ella <strong>dejarán de ser válidos</strong>.
            </p>
          </div>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <StatCard
          icon={Key}
          label="Claves Totales"
          value={keys.length}
          subValue="En JWKS"
          color="blue"
        />
        <StatCard
          icon={CheckCircle}
          label="Clave Activa"
          value={activeKey ? "1" : "0"}
          subValue={activeKey ? `Creada ${formatTimeAgo(activeKey.created_at)}` : "Sin clave activa"}
          color="emerald"
        />
        <StatCard
          icon={Clock}
          label="En Gracia"
          value={retiringKeys.length}
          subValue={retiringKeys.length > 0 ? "Expirando pronto" : "Ninguna"}
          color="amber"
        />
      </div>

      {/* Tenant Selector */}
      <div className="flex items-center gap-3">
        <Label className="text-sm text-muted-foreground">Ver claves de:</Label>
        <Select value={selectedTenant} onValueChange={setSelectedTenant}>
          <SelectTrigger className="w-[200px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="global">
              <div className="flex items-center gap-2">
                <Globe className="h-4 w-4" />
                Global
              </div>
            </SelectItem>
            {tenants?.map(tenant => (
              <SelectItem key={tenant.id} value={tenant.slug || tenant.id}>
                <div className="flex items-center gap-2">
                  <Building2 className="h-4 w-4" />
                  {tenant.name}
                </div>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button variant="ghost" size="sm" onClick={() => refetchKeys()}>
          <RotateCw className={cn("h-4 w-4", keysLoading && "animate-spin")} />
        </Button>
      </div>

      {/* Keys Display */}
      <Tabs defaultValue="current" className="w-full">
        <TabsList className="bg-zinc-100 dark:bg-zinc-800/50">
          <TabsTrigger value="current" className="gap-2">
            <Key className="h-4 w-4" />
            Claves Actuales
          </TabsTrigger>
          <TabsTrigger value="jwks" className="gap-2">
            <ExternalLink className="h-4 w-4" />
            JWKS Endpoint
          </TabsTrigger>
        </TabsList>

        <TabsContent value="current" className="mt-4 space-y-4">
          {keysLoading ? (
            <div className="flex items-center justify-center py-12">
              <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
            </div>
          ) : (
            <div className="grid gap-4">
              {/* Active Key */}
              {activeKey && (
                <KeyCard
                  keyInfo={activeKey}
                  onCopy={copyToClipboard}
                  onRevoke={handleRevokeClick}
                  copiedKid={copiedKid}
                  isExpanded
                />
              )}

              {/* Retiring Keys */}
              {retiringKeys.map(key => (
                <KeyCard
                  key={key.kid}
                  keyInfo={key}
                  onCopy={copyToClipboard}
                  onRevoke={handleRevokeClick}
                  copiedKid={copiedKid}
                />
              ))}

              {keys.length === 0 && (
                <div className="text-center py-12 text-muted-foreground">
                  <Key className="h-12 w-12 mx-auto mb-4 opacity-50" />
                  <p>No hay claves configuradas</p>
                  <Button
                    variant="outline"
                    className="mt-4"
                    onClick={() => setRotateDialogOpen(true)}
                  >
                    Generar primera clave
                  </Button>
                </div>
              )}
            </div>
          )}
        </TabsContent>

        <TabsContent value="jwks" className="mt-4">
          <Card className="p-4">
            <div className="flex items-center justify-between mb-4">
              <h3 className="font-medium flex items-center gap-2">
                <ExternalLink className="h-4 w-4" />
                Endpoint JWKS
                <InfoTooltip content="Este endpoint público expone las claves públicas en formato JWK Set para que las aplicaciones puedan verificar tokens" />
              </h3>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  const endpoint = selectedTenant === "global"
                    ? `${window.location.origin}${API_ROUTES.OIDC_JWKS}`
                    : `${window.location.origin}${API_ROUTES.OIDC_JWKS_TENANT(selectedTenant)}`
                  window.open(endpoint, "_blank")
                }}
              >
                <ExternalLink className="h-4 w-4 mr-1" />
                Abrir
              </Button>
            </div>
            <div className="rounded-lg bg-zinc-950 p-4 overflow-auto max-h-[400px]">
              <pre className="text-xs text-zinc-100 font-mono">
                {JSON.stringify(jwks || { keys: [] }, null, 2)}
              </pre>
            </div>
            <p className="text-xs text-muted-foreground mt-3">
              URL: <code className="bg-zinc-100 dark:bg-zinc-800 px-1 rounded">
                {selectedTenant === "global"
                  ? "/.well-known/jwks.json"
                  : `/.well-known/jwks/${selectedTenant}.json`}
              </code>
            </p>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Rotate Dialog */}
      <Dialog open={rotateDialogOpen} onOpenChange={setRotateDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <RotateCw className="h-5 w-5" />
              Rotar Signing Key
            </DialogTitle>
            <DialogDescription>
              Se generará una nueva clave activa y la actual pasará al período de gracia.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="gracePeriod" className="flex items-center gap-1">
                Período de Gracia
                <InfoTooltip content="Tiempo durante el cual los tokens firmados con la clave anterior seguirán siendo válidos" />
              </Label>
              <Select value={gracePeriodHours} onValueChange={setGracePeriodHours}>
                <SelectTrigger id="gracePeriod">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">1 hora</SelectItem>
                  <SelectItem value="6">6 horas</SelectItem>
                  <SelectItem value="12">12 horas</SelectItem>
                  <SelectItem value="24">24 horas (recomendado)</SelectItem>
                  <SelectItem value="48">48 horas</SelectItem>
                  <SelectItem value="72">72 horas</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                Recomendado: al menos el tiempo máximo de vida de tus access tokens
              </p>
            </div>

            <div className="rounded-lg border border-amber-500/20 bg-amber-500/5 p-3">
              <div className="flex gap-2">
                <AlertTriangle className="h-4 w-4 text-amber-500 flex-shrink-0 mt-0.5" />
                <div className="text-sm">
                  <p className="font-medium text-amber-700 dark:text-amber-400">
                    Impacto de la rotación
                  </p>
                  <ul className="text-muted-foreground mt-1 space-y-1 text-xs">
                    <li>• Los nuevos tokens se firmarán con la nueva clave</li>
                    <li>• Los tokens existentes serán válidos por {gracePeriodHours}h más</li>
                    <li>• Después del período de gracia, los tokens antiguos fallarán</li>
                  </ul>
                </div>
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setRotateDialogOpen(false)}>
              Cancelar
            </Button>
            <Button
              onClick={() => rotateMutation.mutate()}
              disabled={rotateMutation.isPending}
              className="gap-2"
            >
              {rotateMutation.isPending ? (
                <>
                  <RotateCw className="h-4 w-4 animate-spin" />
                  Rotando...
                </>
              ) : (
                <>
                  <Zap className="h-4 w-4" />
                  Rotar Clave
                </>
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Revoke Confirmation Dialog */}
      <Dialog open={revokeDialogOpen} onOpenChange={setRevokeDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-destructive" />
              Revocar Clave Inmediatamente
            </DialogTitle>
            <DialogDescription>
              Esta acción eliminará la clave permanentemente y no se puede deshacer.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-4">
            <div className="rounded-lg border border-red-500/20 bg-red-500/5 p-4">
              <div className="flex gap-3">
                <AlertTriangle className="h-5 w-5 text-red-500 flex-shrink-0 mt-0.5" />
                <div className="text-sm">
                  <p className="font-medium text-red-700 dark:text-red-400 mb-2">
                    ⚠️ Advertencia: Acción Destructiva
                  </p>
                  <ul className="text-muted-foreground space-y-1 text-xs">
                    <li>• Todos los tokens firmados con esta clave <strong>fallarán inmediatamente</strong></li>
                    <li>• Los usuarios con tokens activos serán <strong>desconectados</strong></li>
                    <li>• Esta acción <strong>no se puede deshacer</strong></li>
                    <li>• Se recomienda usar <strong>rotación con grace period</strong> en su lugar</li>
                  </ul>
                </div>
              </div>
            </div>

            {keyToRevoke && (
              <div className="text-sm">
                <p className="text-muted-foreground">Key ID a revocar:</p>
                <code className="block mt-1 rounded bg-zinc-100 dark:bg-zinc-800 px-2 py-1 text-xs font-mono">
                  {keyToRevoke}
                </code>
              </div>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setRevokeDialogOpen(false)}>
              Cancelar
            </Button>
            <Button
              variant="destructive"
              onClick={handleRevokeConfirm}
              disabled={revokeMutation.isPending}
              className="gap-2"
            >
              {revokeMutation.isPending ? (
                <>
                  <RotateCw className="h-4 w-4 animate-spin" />
                  Revocando...
                </>
              ) : (
                <>
                  <AlertTriangle className="h-4 w-4" />
                  Confirmar Revocación
                </>
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
