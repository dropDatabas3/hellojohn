"use client"

import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Badge,
  Button,
  InlineAlert,
  EmptyState,
  Skeleton,
  QuickLinkCard,
  cn,
} from "@/components/ds"
import { useApi } from "@/hooks/use-api"
import { useUIStore } from "@/lib/ui-store"
import { getTranslations } from "@/lib/i18n"
import { API_ROUTES } from "@/lib/routes"
import type { ReadyzResponse, Tenant } from "@/lib/types"
import {
  Activity,
  AlertCircle,
  CheckCircle,
  Server,
  Key,
  Building2,
  RefreshCw,
  Gauge,
  Zap,
  ChevronRight,
} from "lucide-react"
import Link from "next/link"
import { CreateTenantWizard } from "@/components/tenant/CreateTenantWizard"

// Stats Card Component — Consistent with other pages
function StatsCard({
  icon: Icon,
  label,
  value,
  subValue,
  variant = "default",
}: {
  icon: React.ElementType
  label: string
  value: string | number
  subValue?: string
  variant?: "default" | "info" | "success" | "warning" | "danger" | "accent"
}) {
  const variantStyles = {
    default: "bg-muted/50 text-muted-foreground",
    info: "bg-info/10 text-info",
    success: "bg-success/10 text-success",
    warning: "bg-warning/10 text-warning",
    danger: "bg-danger/10 text-danger",
    accent: "bg-accent/10 text-accent",
  }

  return (
    <Card interactive className="group p-5">
      <div className="flex items-start justify-between">
        <div className="space-y-1">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{label}</p>
          <p className="text-2xl font-bold text-foreground">{value}</p>
          {subValue && <p className="text-xs text-muted-foreground">{subValue}</p>}
        </div>
        <div className={cn("p-2.5 rounded-xl", variantStyles[variant])}>
          <Icon className="h-5 w-5" />
        </div>
      </div>
    </Card>
  )
}

// Component Status Badge
function ComponentStatusBadge({ status }: { status: string }) {
  const isHealthy = ["ok", "ready", "healthy"].includes(status)
  const isDegraded = status === "degraded"

  return (
    <Badge
      variant={isHealthy ? "success" : isDegraded ? "warning" : "destructive"}
      className="text-xs"
    >
      {isHealthy && <CheckCircle className="h-3 w-3 mr-1 ml-0.5" />}
      {isDegraded && <AlertCircle className="h-3 w-3 mr-1 ml-0.5" />}
      {!isHealthy && !isDegraded && <AlertCircle className="h-3 w-3 mr-1 ml-0.5" />}
      {status}
    </Badge>
  )
}

export default function DashboardPage() {
  const api = useApi()
  const locale = useUIStore((state) => state.locale)
  const t = getTranslations(locale)
  const [createDialogOpen, setCreateDialogOpen] = useState(false)

  // Fetch system health
  const {
    data: health,
    isLoading: healthLoading,
    isError: healthError,
    refetch: refetchHealth,
  } = useQuery({
    queryKey: ["readyz"],
    queryFn: () => api.get<ReadyzResponse>(API_ROUTES.READYZ),
    refetchInterval: 10000,
  })

  // Fetch tenants
  const {
    data: tenants,
    isLoading: tenantsLoading,
    isError: tenantsError,
    refetch: refetchTenants,
  } = useQuery({
    queryKey: ["tenants"],
    queryFn: () => api.get<Tenant[]>(API_ROUTES.ADMIN_TENANTS),
  })

  const getStatusVariant = (status: string): "success" | "warning" | "danger" | "default" => {
    switch (status) {
      case "ready":
        return "success"
      case "degraded":
        return "warning"
      case "unavailable":
        return "danger"
      default:
        return "default"
    }
  }

  return (
    <div className="space-y-6 animate-in fade-in duration-500">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">

          <div>
            <h1 className="text-2xl font-bold tracking-tight">{t.dashboard.title}</h1>
            <p className="text-sm text-muted-foreground">
              Panel de control y monitoreo del sistema HelloJohn
            </p>
          </div>
        </div>
        <Button
          variant="outline"
          onClick={() => {
            refetchHealth()
            refetchTenants()
          }}
          className="gap-2"
        >
          <RefreshCw className="h-4 w-4" />
          Actualizar
        </Button>
      </div>

      {/* Status Alert */}
      {health && health.status !== "ready" && (
        <InlineAlert
          variant={health.status === "degraded" ? "warning" : "danger"}
        >
          <AlertCircle className="h-4 w-4" />
          <div>
            <p className="font-semibold">
              {health.status === "degraded" ? "Sistema Degradado" : "Sistema No Disponible"}
            </p>
            <p className="text-sm">
              {health.status === "degraded"
                ? "Algunos componentes pueden no estar funcionando correctamente."
                : "El sistema no está disponible. Por favor, contacte al administrador."}
            </p>
          </div>
        </InlineAlert>
      )}

      {healthError && (
        <InlineAlert variant="danger">
          <AlertCircle className="h-4 w-4" />
          <div className="flex-1">
            <p className="font-semibold">Error al cargar estado del sistema</p>
            <p className="text-sm">No se pudo conectar con el servicio de salud.</p>
          </div>
          <Button size="sm" variant="outline" onClick={() => refetchHealth()}>
            Reintentar
          </Button>
        </InlineAlert>
      )}

      {/* Stats Grid */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {healthLoading ? (
          <>
            {[1, 2, 3, 4].map((i) => (
              <Card key={i} className="p-5">
                <div className="space-y-3">
                  <Skeleton className="h-4 w-20" />
                  <Skeleton className="h-8 w-24" />
                  <Skeleton className="h-3 w-16" />
                </div>
              </Card>
            ))}
          </>
        ) : (
          <>
            <StatsCard
              icon={Activity}
              label={t.dashboard.status}
              value={health?.status || "N/A"}
              subValue={health?.fs_degraded ? "Filesystem degradado" : "Sistema operativo"}
              variant={getStatusVariant(health?.status || "")}
            />
            <StatsCard
              icon={Zap}
              label={t.dashboard.version}
              value={health?.version || "N/A"}
              subValue={`Commit: ${health?.commit?.slice(0, 7) || "N/A"}`}
              variant="info"
            />
            <StatsCard
              icon={Server}
              label={t.cluster.role}
              value={health?.cluster?.role || "N/A"}
              subValue={health?.cluster?.role === "leader" ? t.cluster.leader : t.cluster.follower}
              variant={health?.cluster?.role === "leader" ? "accent" : "default"}
            />
            <StatsCard
              icon={Key}
              label="Active Key"
              value={health?.active_key_id?.slice(0, 8) || "N/A"}
              subValue="Key ID"
              variant="warning"
            />
          </>
        )}
      </div>

      {/* Main Content Grid */}
      <div className="grid lg:grid-cols-3 gap-6">
        {/* Left Column - Cluster & Components */}
        <div className="lg:col-span-2 space-y-6">
          {/* Cluster Status */}
          <Card>
            <CardHeader className="pb-4">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-lg bg-accent/10">
                  <Server className="h-4 w-4 text-accent" />
                </div>
                <div>
                  <CardTitle className="text-base">{t.cluster.title}</CardTitle>
                  <CardDescription>Estado del clúster Raft</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              {healthLoading ? (
                <div className="grid gap-3 md:grid-cols-2">
                  {[1, 2, 3, 4, 5, 6].map((i) => (
                    <Skeleton key={i} className="h-10" />
                  ))}
                </div>
              ) : health ? (
                <div className="grid gap-3 md:grid-cols-2">
                  <div className="flex items-center justify-between p-3 rounded-lg bg-muted/30 border">
                    <span className="text-sm text-muted-foreground">Modo</span>
                    <Badge variant="outline">{health.cluster.mode}</Badge>
                  </div>
                  <div className="flex items-center justify-between p-3 rounded-lg bg-muted/30 border">
                    <span className="text-sm text-muted-foreground">Rol</span>
                    <Badge variant={health.cluster.role === "leader" ? "default" : "outline"}>
                      {health.cluster.role}
                    </Badge>
                  </div>
                  <div className="flex items-center justify-between p-3 rounded-lg bg-muted/30 border">
                    <span className="text-sm text-muted-foreground">Leader ID</span>
                    <span className="font-mono text-sm">{health.cluster.leader_id}</span>
                  </div>
                  <div className="flex items-center justify-between p-3 rounded-lg bg-muted/30 border">
                    <span className="text-sm text-muted-foreground">Peers</span>
                    <span className="font-mono text-sm">
                      {health.cluster.peers_connected}/{health.cluster.peers_configured}
                    </span>
                  </div>
                  {health.cluster.raft && (
                    <div className="flex items-center justify-between p-3 rounded-lg bg-muted/30 border md:col-span-2">
                      <span className="text-sm text-muted-foreground">Raft State</span>
                      <Badge variant="outline">{health.cluster.raft.state}</Badge>
                    </div>
                  )}
                </div>
              ) : (
                <EmptyState
                  icon={<Server className="w-10 h-10" />}
                  title="No hay datos disponibles"
                  description="No se pudo cargar la información del clúster."
                />
              )}
            </CardContent>
          </Card>

          {/* Components Status */}
          <Card>
            <CardHeader className="pb-4">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-lg bg-success/10">
                  <Activity className="h-4 w-4 text-success" />
                </div>
                <div>
                  <CardTitle className="text-base">Componentes del Sistema</CardTitle>
                  <CardDescription>Estado de los componentes principales</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              {healthLoading ? (
                <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
                  {[1, 2, 3, 4, 5, 6].map((i) => (
                    <Skeleton key={i} className="h-12" />
                  ))}
                </div>
              ) : health ? (
                <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
                  {Object.entries(health.components).map(([key, value]) => {
                    const status = typeof value === "string" ? value : (value as any)?.status ?? "unknown"
                    return (
                      <div
                        key={key}
                        className="flex items-center justify-between p-3 rounded-lg bg-muted/30 border hover:bg-muted/50 transition-colors"
                      >
                        <span className="text-sm font-medium capitalize">{key.replace(/_/g, " ")}</span>
                        <ComponentStatusBadge status={status} />
                      </div>
                    )
                  })}
                </div>
              ) : (
                <EmptyState
                  icon={<Activity className="w-10 h-10" />}
                  title="No hay datos disponibles"
                  description="No se pudo cargar el estado de los componentes."
                />
              )}
            </CardContent>
          </Card>
        </div>

        {/* Right Column - Tenants */}
        <div className="space-y-6">
          {/* Tenants Card */}
          <Card className="h-fit">
            <CardHeader className="pb-4">
              <div className="flex items-left justify-between">
                <div className="flex items-center">
                  <div className="p-2 rounded-lg bg-warning/10">
                    <Building2 className="h-4 w-4 text-warning" />
                  </div>
                  <div>
                    <CardTitle className="text-base">{t.tenants.title}</CardTitle>
                  </div>
                </div>
                {tenants && tenants.length > 0 && (
                  <Button asChild size="sm" variant="ghost">
                    <Link href="/admin/tenants">
                      Ver todos
                      <ChevronRight className="h-4 w-4" />
                    </Link>
                  </Button>
                )}
              </div>
            </CardHeader>
            <CardContent>
              {tenantsError ? (
                <InlineAlert variant="danger" className="mb-0">
                  <AlertCircle className="h-4 w-4" />
                  <div className="flex-1">
                    <p className="text-sm">Error al cargar tenants</p>
                  </div>
                  <Button size="sm" variant="outline" onClick={() => refetchTenants()}>
                    Reintentar
                  </Button>
                </InlineAlert>
              ) : tenantsLoading ? (
                <div className="space-y-3">
                  {[1, 2, 3].map((i) => (
                    <Skeleton key={i} className="h-16" />
                  ))}
                </div>
              ) : tenants && tenants.length > 0 ? (
                <div className="space-y-2">
                  {tenants.slice(0, 5).map((tenant) => (
                    <Link
                      key={tenant.id}
                      href={`/admin/tenants/${tenant.id}/detail`}
                      className="flex items-center justify-between p-3 rounded-lg border bg-muted/20 hover:bg-muted/40 hover:-translate-y-0.5 hover:shadow-md transition-all duration-200 group"
                    >
                      <div className="flex items-center gap-3">
                        <div className="h-9 w-9 rounded-lg bg-accent/20 flex items-center justify-center text-accent font-semibold text-sm">
                          {tenant.name.charAt(0).toUpperCase()}
                        </div>
                        <div>
                          <p className="font-medium text-sm group-hover:text-accent transition-colors">{tenant.name}</p>
                          <p className="text-xs text-muted-foreground font-mono">{tenant.slug}</p>
                        </div>
                      </div>
                      <ChevronRight className="h-4 w-4 text-muted-foreground group-hover:text-accent transition-colors" />
                    </Link>
                  ))}
                </div>
              ) : (
                <div className="text-center py-8">
                  <div className="h-12 w-12 rounded-full bg-muted/50 flex items-center justify-center mx-auto mb-3">
                    <Building2 className="h-6 w-6 text-muted-foreground" />
                  </div>
                  <p className="text-sm font-medium mb-1">No hay tenants</p>
                  <p className="text-xs text-muted-foreground mb-4">Crea tu primera organización</p>
                  <Button size="sm" onClick={() => setCreateDialogOpen(true)}>
                    Crear Organización
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Quick Actions */}
      <div className="space-y-4">
        <h2 className="text-lg font-semibold">{t.dashboard.quickActions}</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <QuickLinkCard
            href="/admin/tenants"
            icon={Building2}
            title="Gestionar Tenants"
            description="Crear y configurar organizaciones"
            variant="warning"
          />
          <QuickLinkCard
            href="/admin/cluster"
            icon={Server}
            title="Ver Clúster"
            description="Estado y configuración del clúster"
            variant="info"
          />
          <QuickLinkCard
            href="/admin/metrics"
            icon={Activity}
            title="Métricas"
            description="Monitoreo y estadísticas"
            variant="success"
          />
          <QuickLinkCard
            href="/admin/playground"
            icon={Key}
            title="OAuth Tools"
            description="Probar flujos de autenticación"
            variant="accent"
          />
        </div>
      </div>

      {/* Create Tenant Wizard */}
      <CreateTenantWizard open={createDialogOpen} onOpenChange={setCreateDialogOpen} />
    </div>
  )
}
