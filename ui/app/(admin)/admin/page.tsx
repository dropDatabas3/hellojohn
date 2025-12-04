"use client"

import { useQuery } from "@tanstack/react-query"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { useApi } from "@/lib/hooks/use-api"
import { useUIStore } from "@/lib/ui-store"
import { getTranslations } from "@/lib/i18n"
import type { ReadyzResponse, Tenant } from "@/lib/types"
import { Activity, AlertCircle, CheckCircle, Server, Key, Building2 } from "lucide-react"
import Link from "next/link"

export default function DashboardPage() {
  const api = useApi()
  const locale = useUIStore((state) => state.locale)
  const t = getTranslations(locale)

  // Fetch system health
  const { data: health, isLoading: healthLoading } = useQuery({
    queryKey: ["readyz"],
    queryFn: () => api.get<ReadyzResponse>("/readyz"),
    refetchInterval: 10000, // Refresh every 10 seconds
  })

  // Fetch tenants
  const { data: tenants, isLoading: tenantsLoading } = useQuery({
    queryKey: ["tenants"],
    queryFn: () => api.get<Tenant[]>("/v1/admin/tenants"),
  })

  const getStatusColor = (status: string) => {
    switch (status) {
      case "ready":
        return "bg-green-500"
      case "degraded":
        return "bg-yellow-500"
      case "unavailable":
        return "bg-red-500"
      default:
        return "bg-gray-500"
    }
  }

  const getStatusIcon = (status: string) => {
    switch (status) {
      case "ready":
        return <CheckCircle className="h-5 w-5 text-green-500" />
      case "degraded":
        return <AlertCircle className="h-5 w-5 text-yellow-500" />
      case "unavailable":
        return <AlertCircle className="h-5 w-5 text-red-500" />
      default:
        return <Activity className="h-5 w-5 text-gray-500" />
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">{t.dashboard.title}</h1>
        <p className="mt-2 text-muted-foreground">Panel de control de HelloJohn Admin</p>
      </div>

      {/* Status Alert */}
      {health && health.status !== "ready" && (
        <Alert variant={health.status === "degraded" ? "default" : "destructive"}>
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>
            {health.status === "degraded"
              ? "El sistema está degradado. Algunos componentes pueden no estar funcionando correctamente."
              : "El sistema no está disponible. Por favor, contacte al administrador."}
          </AlertDescription>
        </Alert>
      )}

      {/* System Status Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">{t.dashboard.status}</CardTitle>
            {health && getStatusIcon(health.status)}
          </CardHeader>
          <CardContent>
            {healthLoading ? (
              <div className="text-2xl font-bold text-muted-foreground">{t.common.loading}</div>
            ) : (
              <>
                <div className="text-2xl font-bold capitalize">{health?.status || "Unknown"}</div>
                <p className="text-xs text-muted-foreground">
                  {health?.fs_degraded ? "Filesystem degradado" : "Sistema operativo"}
                </p>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">{t.dashboard.version}</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            {healthLoading ? (
              <div className="text-2xl font-bold text-muted-foreground">{t.common.loading}</div>
            ) : (
              <>
                <div className="text-2xl font-bold">{health?.version || "N/A"}</div>
                <p className="text-xs text-muted-foreground">Commit: {health?.commit?.slice(0, 7) || "N/A"}</p>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">{t.cluster.role}</CardTitle>
            <Server className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            {healthLoading ? (
              <div className="text-2xl font-bold text-muted-foreground">{t.common.loading}</div>
            ) : (
              <>
                <div className="text-2xl font-bold capitalize">{health?.cluster.role || "N/A"}</div>
                <p className="text-xs text-muted-foreground">
                  {health?.cluster.role === "leader" ? t.cluster.leader : t.cluster.follower}
                </p>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Key</CardTitle>
            <Key className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            {healthLoading ? (
              <div className="text-2xl font-bold text-muted-foreground">{t.common.loading}</div>
            ) : (
              <>
                <div className="text-xl font-mono font-bold">{health?.active_key_id?.slice(0, 8) || "N/A"}</div>
                <p className="text-xs text-muted-foreground">Key ID</p>
              </>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Cluster Details */}
      <Card>
        <CardHeader>
          <CardTitle>{t.cluster.title}</CardTitle>
          <CardDescription>Estado del clúster Raft</CardDescription>
        </CardHeader>
        <CardContent>
          {healthLoading ? (
            <div className="text-muted-foreground">{t.common.loading}</div>
          ) : health ? (
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">Modo:</span>
                  <Badge variant="outline">{health.cluster.mode}</Badge>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">Rol:</span>
                  <Badge variant={health.cluster.role === "leader" ? "default" : "secondary"}>
                    {health.cluster.role}
                  </Badge>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">Leader ID:</span>
                  <span className="font-mono text-sm">{health.cluster.leader_id}</span>
                </div>
              </div>
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">Peers configurados:</span>
                  <span className="font-mono text-sm">{health.cluster.peers_configured}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">Peers conectados:</span>
                  <span className="font-mono text-sm">{health.cluster.peers_connected}</span>
                </div>
                {health.cluster.raft && (
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium">Raft State:</span>
                    <Badge variant="outline">{health.cluster.raft.state}</Badge>
                  </div>
                )}
              </div>
            </div>
          ) : (
            <div className="text-muted-foreground">No hay datos disponibles</div>
          )}
        </CardContent>
      </Card>

      {/* Components Status */}
      <Card>
        <CardHeader>
          <CardTitle>Componentes del Sistema</CardTitle>
          <CardDescription>Estado de los componentes principales</CardDescription>
        </CardHeader>
        <CardContent>
          {healthLoading ? (
            <div className="text-muted-foreground">{t.common.loading}</div>
          ) : health ? (
            <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
              {Object.entries(health.components).map(([key, value]) => {
                const status = typeof value === "string" ? value : (value as any)?.status ?? "unknown"
                const variant = ["ok", "ready", "healthy"].includes(status)
                  ? "default"
                  : status === "degraded"
                    ? "secondary"
                    : "destructive"
                return (
                  <div key={key} className="flex items-center justify-between rounded-lg border p-3">
                    <span className="text-sm font-medium capitalize">{key.replace(/_/g, " ")}</span>
                    <Badge variant={variant}>{status}</Badge>
                  </div>
                )
              })}
            </div>
          ) : (
            <div className="text-muted-foreground">No hay datos disponibles</div>
          )}
        </CardContent>
      </Card>

      {/* Tenants Overview */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>{t.tenants.title}</CardTitle>
            <CardDescription>Tenants configurados en el sistema</CardDescription>
          </div>
          <Button asChild>
            <Link href="/admin/tenants">Ver todos</Link>
          </Button>
        </CardHeader>
        <CardContent>
          {tenantsLoading ? (
            <div className="text-muted-foreground">{t.common.loading}</div>
          ) : tenants && tenants.length > 0 ? (
            <div className="space-y-2">
              {tenants.slice(0, 5).map((tenant) => (
                <Link
                  key={tenant.id}
                  href={`/admin/tenants/detail?id=${tenant.id}`}
                  className="flex items-center justify-between rounded-lg border p-3 transition-colors hover:bg-accent"
                >
                  <div className="flex items-center gap-3">
                    <Building2 className="h-5 w-5 text-muted-foreground" />
                    <div>
                      <p className="font-medium">{tenant.name}</p>
                      <p className="text-sm text-muted-foreground">{tenant.slug}</p>
                    </div>
                  </div>
                  <Badge variant="outline">{tenant.settings.issuerMode || "global"}</Badge>
                </Link>
              ))}
            </div>
          ) : (
            <div className="text-center text-muted-foreground">No hay tenants configurados</div>
          )}
        </CardContent>
      </Card>

      {/* Quick Actions */}
      <Card>
        <CardHeader>
          <CardTitle>{t.dashboard.quickActions}</CardTitle>
          <CardDescription>Acciones rápidas de administración</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-4">
            <Button asChild variant="outline" className="h-auto flex-col items-start gap-2 p-4 bg-transparent">
              <Link href="/admin/tenants">
                <Building2 className="h-5 w-5" />
                <div className="text-left">
                  <div className="font-medium">Gestionar Tenants</div>
                  <div className="text-xs text-muted-foreground">Crear y configurar tenants</div>
                </div>
              </Link>
            </Button>
            <Button asChild variant="outline" className="h-auto flex-col items-start gap-2 p-4 bg-transparent">
              <Link href="/admin/cluster">
                <Server className="h-5 w-5" />
                <div className="text-left">
                  <div className="font-medium">Ver Clúster</div>
                  <div className="text-xs text-muted-foreground">Estado y configuración</div>
                </div>
              </Link>
            </Button>
            <Button asChild variant="outline" className="h-auto flex-col items-start gap-2 p-4 bg-transparent">
              <Link href="/admin/metrics">
                <Activity className="h-5 w-5" />
                <div className="text-left">
                  <div className="font-medium">Métricas</div>
                  <div className="text-xs text-muted-foreground">Monitoreo del sistema</div>
                </div>
              </Link>
            </Button>
            <Button asChild variant="outline" className="h-auto flex-col items-start gap-2 p-4 bg-transparent">
              <Link href="/admin/tools/oauth">
                <Key className="h-5 w-5" />
                <div className="text-left">
                  <div className="font-medium">OAuth Tools</div>
                  <div className="text-xs text-muted-foreground">Probar flujos OAuth</div>
                </div>
              </Link>
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
