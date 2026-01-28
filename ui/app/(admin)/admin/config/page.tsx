"use client"

import { useEffect, useMemo, useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Card } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { useToast } from "@/hooks/use-toast"
import { useI18n } from "@/lib/i18n"
import { api } from "@/lib/api"
import { Save, RefreshCw } from "lucide-react"

export default function SystemConfigPage() {
  const { t } = useI18n()
  const { toast } = useToast()
  const qc = useQueryClient()
  const [etag, setEtag] = useState<string | undefined>(undefined)
  const [raw, setRaw] = useState<string>("{}")

  const cfgQuery = useQuery({
    queryKey: ["system-config"],
    queryFn: async () => {
      const token = (await import("@/lib/auth-store")).useAuthStore.getState().token
      const resp = await fetch(`${api.getBaseUrl()}/v1/admin/config`, {
        headers: {
          Authorization: token ? `Bearer ${token}` : "",
        },
      })
      setEtag(resp.headers.get("ETag") || undefined)
      const json = await resp.json()
      return json
    },
  })

  useEffect(() => {
    if (cfgQuery.data) {
      setRaw(JSON.stringify(cfgQuery.data, null, 2))
    }
  }, [cfgQuery.data])

  const saveMutation = useMutation({
    mutationFn: async (payload: any) => api.put(`/v1/admin/config`, payload, etag),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["system-config"] })
      toast({ title: t("config.saved"), description: t("config.savedDesc"), variant: "info" })
    },
    onError: (err: any) => {
      toast({ title: t("common.error"), description: err.message, variant: "destructive" })
    },
  })

  const handleSave = () => {
    try {
      const parsed = JSON.parse(raw)
      saveMutation.mutate(parsed)
    } catch (e: any) {
      toast({ title: t("common.error"), description: t("config.invalidJson"), variant: "destructive" })
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">{t("config.title")}</h1>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={() => cfgQuery.refetch()} disabled={cfgQuery.isFetching}>
            <RefreshCw className="mr-2 h-4 w-4" /> {t("common.refresh")}
          </Button>
          <Button onClick={handleSave} disabled={saveMutation.isPending}>
            <Save className="mr-2 h-4 w-4" /> {saveMutation.isPending ? t("common.saving") : t("common.save")}
          </Button>
        </div>
      </div>

      <Card className="p-4">
        {cfgQuery.isLoading ? (
          <div className="flex items-center justify-center py-12">
            <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
          </div>
        ) : (
          <Textarea value={raw} onChange={(e) => setRaw(e.target.value)} rows={28} className="font-mono" />
        )}
        <p className="mt-2 text-xs text-muted-foreground">
          {t("config.warning")}
        </p>
      </Card>
    </div>
  )
}
