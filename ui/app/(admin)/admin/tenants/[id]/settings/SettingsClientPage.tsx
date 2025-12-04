"use client"

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useParams, useRouter, useSearchParams } from "next/navigation"
import { ArrowLeft, Save, Download } from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { useToast } from "@/hooks/use-toast"
import Link from "next/link"
import type { Tenant } from "@/lib/types"
import { useState, useEffect } from "react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"

export default function SettingsClientPage() {
  const params = useParams()
  const search = useSearchParams()
  const router = useRouter()
  const { t } = useI18n()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const tenantId = (params.id as string) || (search.get("id") as string)

  const { data: tenant } = useQuery({
    queryKey: ["tenant", tenantId],
    enabled: !!tenantId,
    queryFn: () => api.get<Tenant>(`/v1/admin/tenants/${tenantId}`),
  })
  const { data: settings, isLoading } = useQuery({
    queryKey: ["tenant-settings", tenantId, "v2"],
    enabled: !!tenantId,
    queryFn: async () => {
      const token = (await import("@/lib/auth-store")).useAuthStore.getState().token
      const resp = await fetch(`${api.getBaseUrl()}/v1/admin/tenants/${tenantId}/settings`, {
        headers: {
          Authorization: token ? `Bearer ${token}` : "",
        },
      })
      const et = resp.headers.get("ETag") || undefined
      const data = await resp.json()
      // Return data with ETag embedded so it persists in cache
      return { ...data, _etag: et }
    },
  })

  const [formData, setFormData] = useState<any>({})
  const [settingsData, setSettingsData] = useState<any>({})

  useEffect(() => {
    if (tenant) {
      setFormData({
        name: tenant.name,
        slug: tenant.slug,
        display_name: tenant.display_name,
      })
    }
  }, [tenant])

  useEffect(() => {
    if (settings) {
      const { _etag, ...rest } = settings
      setSettingsData(rest)
    }
  }, [settings])

  const updateTenantMutation = useMutation({
    mutationFn: (data: Partial<Tenant>) => api.put<Tenant>(`/v1/admin/tenants/${tenantId}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tenant", tenantId] })
      queryClient.invalidateQueries({ queryKey: ["tenants"] })
      toast({
        title: t("tenants.updated"),
        description: t("tenants.updatedDesc"),
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

  const updateSettingsMutation = useMutation({
    mutationFn: (data: any) => {
      const etag = settings?._etag
      if (!etag) {
        throw new Error("Missing ETag. Please refresh the page.")
      }
      return api.put<any>(`/v1/admin/tenants/${tenantId}/settings`, data, etag)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tenant-settings", tenantId, "v2"] })
      toast({
        title: t("tenants.settingsUpdated"),
        description: t("tenants.settingsUpdatedDesc"),
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

  const handleSaveBasic = () => {
    updateTenantMutation.mutate(formData)
  }

  const handleSaveSettings = () => {
    updateSettingsMutation.mutate(settingsData)
  }

  const handleExport = async () => {
    try {
      const [clients, scopes, users] = await Promise.all([
        api.get(`/v1/admin/tenants/${tenantId}/clients`),
        api.get(`/v1/admin/tenants/${tenantId}/scopes`),
        api.get(`/v1/admin/tenants/${tenantId}/users`),
      ])

      const exportData = {
        tenant: tenant,
        settings: settingsData,
        clients,
        scopes,
        users,
        exportedAt: new Date().toISOString(),
      }

      const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: "application/json" })
      const url = URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = `tenant-${tenant?.slug}-config.json`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)

      toast({
        title: "Exportación exitosa",
        description: "La configuración se ha descargado correctamente.",
      })
    } catch (e) {
      toast({
        title: "Error al exportar",
        description: "No se pudo generar el archivo de configuración.",
        variant: "destructive",
      })
    }
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="sm" asChild>
            <Link href={`/admin/tenants/detail?id=${tenantId}`}>
              <ArrowLeft className="h-4 w-4" />
            </Link>
          </Button>
          <div>
            <h1 className="text-3xl font-bold">{t("tenants.settings")}</h1>
            <p className="text-muted-foreground">{tenant?.name}</p>
          </div>
        </div>
        <Button variant="outline" onClick={handleExport}>
          <Download className="mr-2 h-4 w-4" />
          Exportar Configuración
        </Button>
      </div>

      <Tabs defaultValue="basic" className="space-y-6">
        <TabsList>
          <TabsTrigger value="basic">{t("tenants.basicSettings")}</TabsTrigger>
          <TabsTrigger value="advanced">{t("tenants.advancedSettings")}</TabsTrigger>
        </TabsList>

        <TabsContent value="basic" className="space-y-6">
          <Card className="p-6">
            <h2 className="mb-4 text-xl font-semibold">{t("tenants.basicInfo")}</h2>
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="name">{t("tenants.name")}</Label>
                <Input
                  id="name"
                  value={formData.name || ""}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="slug">{t("tenants.slug")}</Label>
                <Input
                  id="slug"
                  value={formData.slug || ""}
                  onChange={(e) => setFormData({ ...formData, slug: e.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="display_name">{t("tenants.displayName")}</Label>
                <Input
                  id="display_name"
                  value={formData.display_name || ""}
                  onChange={(e) => setFormData({ ...formData, display_name: e.target.value })}
                />
              </div>
              <div className="flex items-center justify-between rounded-lg border p-4">
                <div className="space-y-0.5">
                  <Label>{t("tenants.enabled")}</Label>
                  <p className="text-sm text-muted-foreground">{t("tenants.enabledDesc")}</p>
                </div>
                <Switch
                  checked={formData.enabled || false}
                  onCheckedChange={(checked) => setFormData({ ...formData, enabled: checked })}
                />
              </div>
              <Button onClick={handleSaveBasic} disabled={updateTenantMutation.isPending}>
                <Save className="mr-2 h-4 w-4" />
                {updateTenantMutation.isPending ? t("common.saving") : t("common.save")}
              </Button>
            </div>
          </Card>
        </TabsContent>

        <TabsContent value="advanced" className="space-y-6">
          <Card className="p-6">
            <h2 className="mb-4 text-xl font-semibold">{t("tenants.authSettings")}</h2>
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="session_lifetime">{t("tenants.sessionLifetime")}</Label>
                <Input
                  id="session_lifetime"
                  type="number"
                  value={settingsData.session_lifetime_seconds || 3600}
                  onChange={(e) =>
                    setSettingsData({
                      ...settingsData,
                      session_lifetime_seconds: Number.parseInt(e.target.value),
                    })
                  }
                />
                <p className="text-sm text-muted-foreground">{t("tenants.sessionLifetimeDesc")}</p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="refresh_token_lifetime">{t("tenants.refreshTokenLifetime")}</Label>
                <Input
                  id="refresh_token_lifetime"
                  type="number"
                  value={settingsData.refresh_token_lifetime_seconds || 86400}
                  onChange={(e) =>
                    setSettingsData({
                      ...settingsData,
                      refresh_token_lifetime_seconds: Number.parseInt(e.target.value),
                    })
                  }
                />
                <p className="text-sm text-muted-foreground">{t("tenants.refreshTokenLifetimeDesc")}</p>
              </div>
              <div className="flex items-center justify-between rounded-lg border p-4">
                <div className="space-y-0.5">
                  <Label>{t("tenants.mfaEnabled")}</Label>
                  <p className="text-sm text-muted-foreground">{t("tenants.mfaEnabledDesc")}</p>
                </div>
                <Switch
                  checked={settingsData.mfa_enabled || false}
                  onCheckedChange={(checked) => setSettingsData({ ...settingsData, mfa_enabled: checked })}
                />
              </div>
              <div className="flex items-center justify-between rounded-lg border p-4">
                <div className="space-y-0.5">
                  <Label>{t("tenants.socialLoginEnabled")}</Label>
                  <p className="text-sm text-muted-foreground">{t("tenants.socialLoginEnabledDesc")}</p>
                </div>
                <Switch
                  checked={settingsData.social_login_enabled || false}
                  onCheckedChange={(checked) =>
                    setSettingsData({
                      ...settingsData,
                      social_login_enabled: checked,
                    })
                  }
                />
              </div>
              <Button onClick={handleSaveSettings} disabled={updateSettingsMutation.isPending}>
                <Save className="mr-2 h-4 w-4" />
                {updateSettingsMutation.isPending ? t("common.saving") : t("common.save")}
              </Button>
            </div>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
