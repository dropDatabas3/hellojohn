"use client"

import { useQuery } from "@tanstack/react-query"
import { Globe, CheckCircle, XCircle, AlertCircle, ExternalLink } from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Card } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

// Canonical providers response from /v1/auth/providers
type ProvidersResponse = {
  providers: Array<{
    name: string
    enabled: boolean
    ready: boolean
    popup: boolean
    start_url?: string
    reason?: string
  }>
}

// Legacy status shape from /v1/providers/status
type LegacyProviderStatus = {
  name: string
  enabled: boolean
  configured: boolean
  status: "healthy" | "degraded" | "unavailable"
  message?: string
}

type ProviderCard = {
  name: string
  enabled: boolean
  ready: boolean
  status: "healthy" | "degraded" | "unavailable" | "unknown"
  startUrl?: string
  reason?: string
}

export default function ProvidersPage() {
  const { t } = useI18n()

  const { data: providers, isLoading } = useQuery({
    queryKey: ["providers"],
    queryFn: async (): Promise<ProviderCard[]> => {
      // Try canonical endpoint first
      try {
        const data = await api.get<ProvidersResponse>("/v1/auth/providers")
        return (data.providers || []).map((p) => ({
          name: p.name,
          enabled: p.enabled,
          ready: p.ready,
          status: p.enabled ? (p.ready ? "healthy" : "degraded") : "unavailable",
          startUrl: p.start_url,
          reason: p.reason,
        }))
      } catch (e) {
        // Fallback to legacy back-compat endpoint used by earlier UI
        const legacy = await api.get<LegacyProviderStatus[]>("/v1/providers/status")
        return legacy.map((p) => ({
          name: p.name,
          enabled: p.enabled,
          ready: !!p.configured,
          status: p.status ?? "unknown",
          reason: p.message,
        }))
      }
    },
  })

  const getStatusIcon = (status: string) => {
    switch (status) {
      case "healthy":
        return <CheckCircle className="h-5 w-5 text-green-500" />
      case "degraded":
        return <AlertCircle className="h-5 w-5 text-orange-500" />
      case "unavailable":
        return <XCircle className="h-5 w-5 text-destructive" />
      default:
        return <AlertCircle className="h-5 w-5 text-muted-foreground" />
    }
  }

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "healthy":
        return <Badge variant="default">{t("providers.healthy")}</Badge>
      case "degraded":
        return <Badge variant="secondary">{t("providers.degraded")}</Badge>
      case "unavailable":
        return <Badge variant="destructive">{t("providers.unavailable")}</Badge>
      default:
        return <Badge variant="outline">{t("providers.unknown")}</Badge>
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">{t("providers.title")}</h1>
        <p className="text-muted-foreground">{t("providers.description")}</p>
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center py-12">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
        </div>
      ) : (
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          {providers?.map((provider) => (
            <Card key={provider.name} className="p-6">
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <Globe className="h-8 w-8 text-muted-foreground" />
                  <div>
                    <h3 className="font-semibold capitalize">{provider.name}</h3>
                    <div className="mt-1 flex items-center gap-2">
                      {getStatusIcon(provider.status)}
                      {getStatusBadge(provider.status)}
                    </div>
                  </div>
                </div>
                {provider.startUrl && (
                  <Button asChild variant="outline" size="sm" title="Open start URL">
                    <a href={provider.startUrl} target="_blank" rel="noreferrer">
                      <ExternalLink className="mr-2 h-4 w-4" /> Start
                    </a>
                  </Button>
                )}
              </div>
              <div className="mt-4 space-y-2">
                <div className="flex items-center justify-between text-sm">
                  <span className="text-muted-foreground">{t("providers.enabled")}</span>
                  <Badge variant={provider.enabled ? "default" : "secondary"}>
                    {provider.enabled ? t("common.enabled") : t("common.disabled")}
                  </Badge>
                </div>
                <div className="flex items-center justify-between text-sm">
                  <span className="text-muted-foreground">{t("providers.configured")}</span>
                  <Badge variant={provider.ready ? "default" : "secondary"}>
                    {provider.ready ? t("providers.yes") : t("providers.no")}
                  </Badge>
                </div>
                {provider.reason && (
                  <p className="mt-2 text-sm text-muted-foreground">{provider.reason}</p>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
