"use client"

import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import {
  PageShell,
  PageHeader,
  Section,
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
} from "@/components/ds"
import { useApi } from "@/lib/hooks/use-api"
import { useUIStore } from "@/lib/ui-store"
import { getTranslations } from "@/lib/i18n"
import { API_ROUTES } from "@/lib/routes"
import type { ReadyzResponse, Tenant } from "@/lib/types"
import { Activity, AlertCircle, CheckCircle, Server, Key, Building2, RefreshCw } from "lucide-react"
import Link from "next/link"
import { CreateTenantWizard } from "@/components/tenant/CreateTenantWizard"

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
    refetchInterval: 10000, // Refresh every 10 seconds
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

  const getStatusVariant = (status: string): "default" | "success" | "warning" | "danger" => {
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

  const getStatusIcon = (status: string) => {
    switch (status) {
      case "ready":
        return <CheckCircle className="h-5 w-5" aria-hidden="true" />
      case "degraded":
        return <AlertCircle className="h-5 w-5" aria-hidden="true" />
      case "unavailable":
        return <AlertCircle className="h-5 w-5" aria-hidden="true" />
      default:
        return <Activity className="h-5 w-5" aria-hidden="true" />
    }
  }

  return (
    <PageShell>
      <PageHeader
        title={t.dashboard.title}
        description="Panel de control de HelloJohn Admin"
        actions={
          <Button
            variant="secondary"
            leftIcon={<RefreshCw className="h-4 w-4" />}
            onClick={() => {
              refetchHealth()
              refetchTenants()
            }}
          >
            Actualizar
          </Button>
        }
      />

      {/* Status Alert */}
      {health && health.status !== "ready" && (
        <InlineAlert
          variant={health.status === "degraded" ? "warning" : "destructive"}
          title={health.status === "degraded" ? "Sistema Degradado" : "Sistema No Disponible"}
          description={
            health.status === "degraded"
              ? "Algunos componentes pueden no estar funcionando correctamente."
              : "El sistema no está disponible. Por favor, contacte al administrador."
          }
        />
      )}

      {/* Error States */}
      {healthError && (
        <InlineAlert
          variant="destructive"
          title="Error al cargar estado del sistema"
          description="No se pudo conectar con el servicio de salud."
          action={
            <Button size="sm" variant="secondary" onClick={() => refetchHealth()}>
              Reintentar
            </Button>
          }
        />
      )}

      {/* System Status Cards */}
      <Section>
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t.dashboard.status}</CardTitle>
              {health && (
                <Badge variant={getStatusVariant(health.status)}>
                  {getStatusIcon(health.status)}
                </Badge>
              )}
            </CardHeader>
            <CardContent>
              {healthLoading ? (
                <>
                  <Skeleton className="h-8 w-24 mb-2" />
                  <Skeleton className="h-4 w-32" />
                </>
              ) : health ? (
                <>
                  <div className="text-2xl font-bold capitalize">{health.status}</div>
                  <p className="text-xs text-muted">
                    {health.fs_degraded ? "Filesystem degradado" : "Sistema operativo"}
                  </p>
                </>
              ) : (
                <p className="text-sm text-muted">No disponible</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t.dashboard.version}</CardTitle>
              <Activity className="h-4 w-4 text-muted" aria-hidden="true" />
            </CardHeader>
            <CardContent>
              {healthLoading ? (
                <>
                  <Skeleton className="h-8 w-20 mb-2" />
                  <Skeleton className="h-4 w-28" />
                </>
              ) : health ? (
                <>
                  <div className="text-2xl font-bold">{health.version || "N/A"}</div>
                  <p className="text-xs text-muted">Commit: {health.commit?.slice(0, 7) || "N/A"}</p>
                </>
              ) : (
                <p className="text-sm text-muted">No disponible</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t.cluster.role}</CardTitle>
              <Server className="h-4 w-4 text-muted" aria-hidden="true" />
            </CardHeader>
            <CardContent>
              {healthLoading ? (
                <>
                  <Skeleton className="h-8 w-24 mb-2" />
                  <Skeleton className="h-4 w-20" />
                </>
              ) : health ? (
                <>
                  <div className="text-2xl font-bold capitalize">{health.cluster.role || "N/A"}</div>
                  <p className="text-xs text-muted">
                    {health.cluster.role === "leader" ? t.cluster.leader : t.cluster.follower}
                  </p>
                </>
              ) : (
                <p className="text-sm text-muted">No disponible</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Active Key</CardTitle>
              <Key className="h-4 w-4 text-muted" aria-hidden="true" />
            </CardHeader>
            <CardContent>
              {healthLoading ? (
                <>
                  <Skeleton className="h-8 w-20 mb-2" />
                  <Skeleton className="h-4 w-16" />
                </>
              ) : health ? (
                <>
                  <div className="text-xl font-mono font-bold">{health.active_key_id?.slice(0, 8) || "N/A"}</div>
                  <p className="text-xs text-muted">Key ID</p>
                </>
              ) : (
                <p className="text-sm text-muted">No disponible</p>
              )}
            </CardContent>
          </Card>
        </div>
      </Section>

      {/* Cluster Details */}
      <Section>
        <Card>
          <CardHeader>
            <CardTitle>{t.cluster.title}</CardTitle>
            <CardDescription>Estado del clúster Raft</CardDescription>
          </CardHeader>
          <CardContent>
            {healthLoading ? (
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-3">
                  <Skeleton className="h-5 w-full" />
                  <Skeleton className="h-5 w-full" />
                  <Skeleton className="h-5 w-full" />
                </div>
                <div className="space-y-3">
                  <Skeleton className="h-5 w-full" />
                  <Skeleton className="h-5 w-full" />
                  <Skeleton className="h-5 w-full" />
                </div>
              </div>
            ) : health ? (
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium">Modo:</span>
                    <Badge variant="outline">{health.cluster.mode}</Badge>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium">Rol:</span>
                    <Badge variant={health.cluster.role === "leader" ? "default" : "outline"}>
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
              <EmptyState
                icon={<Server className="w-12 h-12" />}
                title="No hay datos de clúster disponibles"
                description="No se pudo cargar la información del clúster."
              />
            )}
          </CardContent>
        </Card>
      </Section>

      {/* Components Status */}
      <Section>
        <Card>
          <CardHeader>
            <CardTitle>Componentes del Sistema</CardTitle>
            <CardDescription>Estado de los componentes principales</CardDescription>
          </CardHeader>
          <CardContent>
            {healthLoading ? (
              <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
                {[...Array(6)].map((_, i) => (
                  <Skeleton key={i} className="h-14 w-full" />
                ))}
              </div>
            ) : health ? (
              <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
                {Object.entries(health.components).map(([key, value]) => {
                  const status = typeof value === "string" ? value : (value as any)?.status ?? "unknown"
                  const variant: "default" | "success" | "warning" | "danger" = ["ok", "ready", "healthy"].includes(status)
                    ? "success"
                    : status === "degraded"
                      ? "warning"
                      : "danger"
                  return (
                    <div key={key} className="flex items-center justify-between rounded-card border border-border p-3">
                      <span className="text-sm font-medium capitalize">{key.replace(/_/g, " ")}</span>
                      <Badge variant={variant}>{status}</Badge>
                    </div>
                  )
                })}
              </div>
            ) : (
              <EmptyState
                icon={<Activity className="w-12 h-12" />}
                title="No hay datos de componentes disponibles"
                description="No se pudo cargar el estado de los componentes."
              />
            )}
          </CardContent>
        </Card>
      </Section>

      {/* Tenants Overview */}
      <Section>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0">
            <div>
              <CardTitle>{t.tenants.title}</CardTitle>
              <CardDescription>Tenants configurados en el sistema</CardDescription>
            </div>
            {tenants && tenants.length > 0 ? (
              <Button asChild>
                <Link href="/admin/tenants">Ver todos</Link>
              </Button>
            ) : (
              <Button onClick={() => setCreateDialogOpen(true)}>Crear Organización</Button>
            )}
          </CardHeader>
          <CardContent>
            {tenantsError ? (
              <InlineAlert
                variant="destructive"
                title="Error al cargar tenants"
                description="No se pudo conectar con el servicio de tenants."
                action={
                  <Button size="sm" variant="secondary" onClick={() => refetchTenants()}>
                    Reintentar
                  </Button>
                }
              />
            ) : tenantsLoading ? (
              <div className="space-y-2">
                {[...Array(3)].map((_, i) => (
                  <Skeleton key={i} className="h-16 w-full" />
                ))}
              </div>
            ) : tenants && tenants.length > 0 ? (
              <div className="space-y-2">
                {tenants.slice(0, 5).map((tenant) => (
                  <Link
                    key={tenant.id}
                    href={`/admin/tenants/detail?id=${tenant.id}`}
                    className="flex items-center justify-between rounded-card border border-border p-3 transition-all duration-200 hover:bg-surface hover:shadow-card focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background"
                  >
                    <div className="flex items-center gap-3">
                      <Building2 className="h-5 w-5 text-muted" aria-hidden="true" />
                      <div>
                        <p className="font-medium">{tenant.name}</p>
                        <p className="text-sm text-muted">{tenant.slug}</p>
                      </div>
                    </div>
                    <Badge variant="outline">{tenant.settings.issuerMode || "global"}</Badge>
                  </Link>
                ))}
              </div>
            ) : (
              <EmptyState
                icon={<Building2 className="w-12 h-12" />}
                title="No hay tenants configurados"
                description="Comienza creando tu primera organización para gestionar usuarios y aplicaciones."
                action={
                  <Button onClick={() => setCreateDialogOpen(true)}>Crear Organización</Button>
                }
              />
            )}
          </CardContent>
        </Card>
      </Section>

      {/* Quick Actions */}
      <Section>
        <Card>
          <CardHeader>
            <CardTitle>{t.dashboard.quickActions}</CardTitle>
            <CardDescription>Acciones rápidas de administración</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-4">
              <Link
                href="/admin/tenants"
                className="flex flex-col items-start gap-2 rounded-card border border-border p-4 transition-all duration-200 hover:bg-surface hover:shadow-card focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background"
              >
                <Building2 className="h-5 w-5 text-accent" aria-hidden="true" />
                <div className="text-left">
                  <div className="font-medium">Gestionar Tenants</div>
                  <div className="text-xs text-muted">Crear y configurar tenants</div>
                </div>
              </Link>
              <Link
                href="/admin/cluster"
                className="flex flex-col items-start gap-2 rounded-card border border-border p-4 transition-all duration-200 hover:bg-surface hover:shadow-card focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background"
              >
                <Server className="h-5 w-5 text-accent" aria-hidden="true" />
                <div className="text-left">
                  <div className="font-medium">Ver Clúster</div>
                  <div className="text-xs text-muted">Estado y configuración</div>
                </div>
              </Link>
              <Link
                href="/admin/metrics"
                className="flex flex-col items-start gap-2 rounded-card border border-border p-4 transition-all duration-200 hover:bg-surface hover:shadow-card focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background"
              >
                <Activity className="h-5 w-5 text-accent" aria-hidden="true" />
                <div className="text-left">
                  <div className="font-medium">Métricas</div>
                  <div className="text-xs text-muted">Monitoreo del sistema</div>
                </div>
              </Link>
              <Link
                href="/admin/tools/oauth"
                className="flex flex-col items-start gap-2 rounded-card border border-border p-4 transition-all duration-200 hover:bg-surface hover:shadow-card focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background"
              >
                <Key className="h-5 w-5 text-accent" aria-hidden="true" />
                <div className="text-left">
                  <div className="font-medium">OAuth Tools</div>
                  <div className="text-xs text-muted">Probar flujos OAuth</div>
                </div>
              </Link>
            </div>
          </CardContent>
        </Card>
      </Section>

      {/* Create Tenant Wizard */}
      <CreateTenantWizard open={createDialogOpen} onOpenChange={setCreateDialogOpen} />
    </PageShell>
  )
}
