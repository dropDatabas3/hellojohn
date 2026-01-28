"use client"

import { useQuery } from "@tanstack/react-query"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { useApi } from "@/lib/hooks/use-api"
import { useUIStore } from "@/lib/ui-store"
import { getTranslations } from "@/lib/i18n"
import type { ReadyzResponse } from "@/lib/types"
import { Activity, RefreshCw, ExternalLink } from "lucide-react"

export default function MetricsPage() {
  const api = useApi()
  const locale = useUIStore((state) => state.locale)
  const t = getTranslations(locale)

  const {
    data: health,
    isLoading,
    refetch,
  } = useQuery({
    queryKey: ["readyz"],
    queryFn: () => api.get<ReadyzResponse>("/readyz"),
    refetchInterval: 10000,
  })

  const apiBase = api.getBaseUrl()

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t.dashboard.title} - Métricas</h1>
          <p className="mt-2 text-muted-foreground">Monitoreo y métricas del sistema</p>
        </div>
        <div className="flex gap-2">
          <Button onClick={() => refetch()} variant="outline" size="sm">
            <RefreshCw className="mr-2 h-4 w-4" />
            Actualizar
          </Button>
          <Button asChild variant="outline" size="sm">
            <a href={`${apiBase}/metrics`} target="_blank" rel="noopener noreferrer">
              <ExternalLink className="mr-2 h-4 w-4" />
              Ver /metrics
            </a>
          </Button>
        </div>
      </div>

      {isLoading ? (
        <div className="text-center text-muted-foreground">{t.common.loading}</div>
      ) : health ? (
        <>
          {/* System Status */}
          <div className="grid gap-4 md:grid-cols-4">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Estado</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold capitalize">{health.status}</div>
                <Badge
                  variant={
                    health.status === "ready" ? "default" : health.status === "degraded" ? "secondary" : "destructive"
                  }
                  className="mt-2"
                >
                  {health.status}
                </Badge>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Versión</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{health.version}</div>
                <p className="text-xs text-muted-foreground">Commit: {health.commit?.slice(0, 7) || "Unknown"}</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Rol</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold capitalize">{health.cluster.role}</div>
                <p className="text-xs text-muted-foreground">{health.cluster.mode}</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Peers</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {health.cluster.peers_connected}/{health.cluster.peers_configured}
                </div>
                <p className="text-xs text-muted-foreground">Conectados</p>
              </CardContent>
            </Card>
          </div>

          {/* Components Health */}
          <Card>
            <CardHeader>
              <CardTitle>Estado de Componentes</CardTitle>
              <CardDescription>Salud de los componentes del sistema</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
                {Object.entries(health.components).map(([key, value]) => {
                  const status = typeof value === "string" ? value : value.status
                  return (
                    <div key={key} className="flex items-center justify-between rounded-lg border p-4">
                      <div>
                        <p className="font-medium capitalize">{key.replace(/_/g, " ")}</p>
                        <p className="text-sm text-muted-foreground">{key}</p>
                      </div>
                      <Badge variant={status === "ok" ? "default" : "destructive"}>{status}</Badge>
                    </div>
                  )
                })}
              </div>
            </CardContent>
          </Card>

          {/* Raft Metrics */}
          {health.cluster.raft && (
            <Card>
              <CardHeader>
                <CardTitle>Métricas Raft</CardTitle>
                <CardDescription>Información del consenso distribuido</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
                  <div className="rounded-lg border p-4">
                    <p className="text-sm font-medium text-muted-foreground">State</p>
                    <p className="mt-2 text-2xl font-bold">{health.cluster.raft.state}</p>
                  </div>
                  <div className="rounded-lg border p-4">
                    <p className="text-sm font-medium text-muted-foreground">Term</p>
                    <p className="mt-2 font-mono text-2xl font-bold">{health.cluster.raft.term}</p>
                  </div>
                  <div className="rounded-lg border p-4">
                    <p className="text-sm font-medium text-muted-foreground">Commit Index</p>
                    <p className="mt-2 font-mono text-2xl font-bold">{health.cluster.raft.commit_index}</p>
                  </div>
                  <div className="rounded-lg border p-4">
                    <p className="text-sm font-medium text-muted-foreground">Last Applied</p>
                    <p className="mt-2 font-mono text-2xl font-bold">{health.cluster.raft.last_applied}</p>
                  </div>
                </div>
                <div className="mt-4 rounded-lg border p-4">
                  <p className="text-sm font-medium text-muted-foreground">Last Contact</p>
                  <p className="mt-2 font-mono text-lg">{health.cluster.raft.last_contact}</p>
                </div>
              </CardContent>
            </Card>
          )}

          {/* System Info */}
          <Card>
            <CardHeader>
              <CardTitle>Información del Sistema</CardTitle>
              <CardDescription>Detalles de configuración y estado</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-3">
                <div className="flex items-center justify-between rounded-lg border p-3">
                  <span className="text-sm font-medium">Active Key ID:</span>
                  <span className="font-mono text-sm">{health.active_key_id}</span>
                </div>
                <div className="flex items-center justify-between rounded-lg border p-3">
                  <span className="text-sm font-medium">Leader ID:</span>
                  <span className="font-mono text-sm">{health.cluster.leader_id}</span>
                </div>
                <div className="flex items-center justify-between rounded-lg border p-3">
                  <span className="text-sm font-medium">Filesystem:</span>
                  <Badge variant={health.fs_degraded ? "destructive" : "default"}>
                    {health.fs_degraded ? "Degradado" : "OK"}
                  </Badge>
                </div>
              </div>
            </CardContent>
          </Card>
        </>
      ) : (
        <Card>
          <CardContent className="py-10 text-center text-muted-foreground">
            No se pudieron cargar las métricas
          </CardContent>
        </Card>
      )}
    </div>
  )
}
