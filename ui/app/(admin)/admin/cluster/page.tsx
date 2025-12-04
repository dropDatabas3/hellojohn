"use client"

import { useQuery } from "@tanstack/react-query"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { useApi } from "@/lib/hooks/use-api"
import { useUIStore } from "@/lib/ui-store"
import { getTranslations } from "@/lib/i18n"
import type { ReadyzResponse } from "@/lib/types"
import { Server, AlertCircle, CheckCircle, Activity, RefreshCw } from "lucide-react"

export default function ClusterPage() {
  const api = useApi()
  const locale = useUIStore((state) => state.locale)
  const apiBaseOverride = useUIStore((state) => state.apiBaseOverride)
  const setApiBaseOverride = useUIStore((state) => state.setApiBaseOverride)
  const t = getTranslations(locale)

  const {
    data: health,
    isLoading,
    refetch,
  } = useQuery({
    queryKey: ["readyz"],
    queryFn: () => api.get<ReadyzResponse>("/readyz"),
    refetchInterval: 5000,
  })

  const handleResetApiBase = () => {
    setApiBaseOverride(null)
    refetch()
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t.cluster.title}</h1>
          <p className="mt-2 text-muted-foreground">Estado y configuración del clúster Raft</p>
        </div>
        <Button onClick={() => refetch()} variant="outline" size="sm">
          <RefreshCw className="mr-2 h-4 w-4" />
          Actualizar
        </Button>
      </div>

      {/* API Base Override Alert */}
      {apiBaseOverride && (
        <Alert>
          <Server className="h-4 w-4" />
          <AlertTitle>Conectado a URL personalizada</AlertTitle>
          <AlertDescription className="flex items-center justify-between">
            <span>Actualmente conectado a: {apiBaseOverride}</span>
            <Button onClick={handleResetApiBase} variant="outline" size="sm">
              Restaurar URL por defecto
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {/* Follower Warning */}
      {health && health.cluster.role === "follower" && (
        <Alert variant="default">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>Nodo Seguidor</AlertTitle>
          <AlertDescription>
            Este nodo es un seguidor. Las operaciones de escritura deben realizarse en el líder:{" "}
            {health.cluster.leader_id}
          </AlertDescription>
        </Alert>
      )}

      {isLoading ? (
        <div className="text-center text-muted-foreground">{t.common.loading}</div>
      ) : health ? (
        <>
          {/* Cluster Overview */}
          <div className="grid gap-4 md:grid-cols-3">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Rol del Nodo</CardTitle>
                <Server className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold capitalize">{health.cluster.role}</div>
                <Badge variant={health.cluster.role === "leader" ? "default" : "secondary"} className="mt-2">
                  {health.cluster.role === "leader" ? t.cluster.leader : t.cluster.follower}
                </Badge>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Peers</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {health.cluster.peers_connected} / {health.cluster.peers_configured}
                </div>
                <p className="text-xs text-muted-foreground">Conectados / Configurados</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Estado</CardTitle>
                {health.cluster.peers_connected === health.cluster.peers_configured ? (
                  <CheckCircle className="h-4 w-4 text-green-500" />
                ) : (
                  <AlertCircle className="h-4 w-4 text-yellow-500" />
                )}
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{health.cluster.mode}</div>
                <p className="text-xs text-muted-foreground">Modo de clúster</p>
              </CardContent>
            </Card>
          </div>

          {/* Cluster Details */}
          <Card>
            <CardHeader>
              <CardTitle>Detalles del Clúster</CardTitle>
              <CardDescription>Información detallada del clúster Raft</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-3">
                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <span className="text-sm font-medium">Leader ID:</span>
                    <span className="font-mono text-sm">{health.cluster.leader_id}</span>
                  </div>
                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <span className="text-sm font-medium">Modo:</span>
                    <Badge variant="outline">{health.cluster.mode}</Badge>
                  </div>
                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <span className="text-sm font-medium">Rol:</span>
                    <Badge variant={health.cluster.role === "leader" ? "default" : "secondary"}>
                      {health.cluster.role}
                    </Badge>
                  </div>
                </div>
                <div className="space-y-3">
                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <span className="text-sm font-medium">Peers configurados:</span>
                    <span className="font-mono text-sm">{health.cluster.peers_configured}</span>
                  </div>
                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <span className="text-sm font-medium">Peers conectados:</span>
                    <span className="font-mono text-sm">{health.cluster.peers_connected}</span>
                  </div>
                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <span className="text-sm font-medium">Filesystem:</span>
                    <Badge variant={health.fs_degraded ? "destructive" : "default"}>
                      {health.fs_degraded ? "Degradado" : "OK"}
                    </Badge>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Raft Details */}
          {health.cluster.raft && (
            <Card>
              <CardHeader>
                <CardTitle>Estado Raft</CardTitle>
                <CardDescription>Información del algoritmo de consenso Raft</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <span className="text-sm font-medium">State:</span>
                    <Badge variant="outline">{health.cluster.raft.state}</Badge>
                  </div>
                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <span className="text-sm font-medium">Term:</span>
                    <span className="font-mono text-sm">{health.cluster.raft.term}</span>
                  </div>
                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <span className="text-sm font-medium">Commit Index:</span>
                    <span className="font-mono text-sm">{health.cluster.raft.commit_index}</span>
                  </div>
                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <span className="text-sm font-medium">Last Applied:</span>
                    <span className="font-mono text-sm">{health.cluster.raft.last_applied}</span>
                  </div>
                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <span className="text-sm font-medium">Last Contact:</span>
                    <span className="font-mono text-sm">{health.cluster.raft.last_contact}</span>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* System Components */}
          <Card>
            <CardHeader>
              <CardTitle>Componentes del Sistema</CardTitle>
              <CardDescription>Estado de los componentes principales</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
                {Object.entries(health.components).map(([key, value]) => {
                  const status = typeof value === "string" ? value : value.status
                  return (
                    <div key={key} className="flex items-center justify-between rounded-lg border p-3">
                      <span className="text-sm font-medium capitalize">{key.replace(/_/g, " ")}</span>
                      <Badge variant={status === "ok" ? "default" : "destructive"}>{status}</Badge>
                    </div>
                  )
                })}
              </div>
            </CardContent>
          </Card>

          {/* System Info */}
          <Card>
            <CardHeader>
              <CardTitle>Información del Sistema</CardTitle>
              <CardDescription>Versión y configuración</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid gap-3 md:grid-cols-2">
                <div className="flex items-center justify-between rounded-lg border p-3">
                  <span className="text-sm font-medium">Versión:</span>
                  <span className="font-mono text-sm">{health.version}</span>
                </div>
                <div className="flex items-center justify-between rounded-lg border p-3">
                  <span className="text-sm font-medium">Commit:</span>
                  <span className="font-mono text-sm">{health.commit}</span>
                </div>
                <div className="flex items-center justify-between rounded-lg border p-3">
                  <span className="text-sm font-medium">Active Key ID:</span>
                  <span className="font-mono text-sm">{health.active_key_id}</span>
                </div>
                <div className="flex items-center justify-between rounded-lg border p-3">
                  <span className="text-sm font-medium">Estado:</span>
                  <Badge
                    variant={
                      health.status === "ready" ? "default" : health.status === "degraded" ? "secondary" : "destructive"
                    }
                  >
                    {health.status}
                  </Badge>
                </div>
              </div>
            </CardContent>
          </Card>
        </>
      ) : (
        <Card>
          <CardContent className="py-10 text-center text-muted-foreground">
            No se pudo obtener información del clúster
          </CardContent>
        </Card>
      )}
    </div>
  )
}
