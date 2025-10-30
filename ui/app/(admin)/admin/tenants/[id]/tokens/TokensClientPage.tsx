"use client"

import { useParams, useSearchParams } from "next/navigation"
import { useI18n } from "@/lib/i18n"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { useState } from "react"
import { api } from "@/lib/api"
import { useToast } from "@/hooks/use-toast"

type Introspection = Record<string, any> & { active?: boolean }

export default function TokensClientPage() {
  const params = useParams()
  const search = useSearchParams()
  const { t } = useI18n()
  const { toast } = useToast()
  const tenantId = (params.id as string) || (search.get("id") as string)

  const [clientId, setClientId] = useState("")
  const [clientSecret, setClientSecret] = useState("")
  const [token, setToken] = useState("")
  const [includeSys, setIncludeSys] = useState(true)
  const [result, setResult] = useState<Introspection | null>(null)

  const basicHeader = () => {
    const creds = btoa(`${clientId}:${clientSecret}`)
    return { Authorization: `Basic ${creds}` }
  }

  const introspect = async () => {
    try {
      const form = new URLSearchParams()
      form.set("token", token)
      if (includeSys) form.set("include_sys", "1")
      const res = await api.postForm<Introspection>(`/oauth2/introspect`, form, basicHeader())
      setResult(res)
    } catch (e: any) {
      toast({ title: t("common.error"), description: e.message || "Introspect failed", variant: "destructive" })
    }
  }

  const revoke = async () => {
    try {
      const form = new URLSearchParams()
      form.set("token", token)
      await api.postForm(`/oauth2/revoke`, form, basicHeader())
      toast({ title: t("tokens.revoked"), description: t("tokens.revokedDesc") })
      setResult(null)
    } catch (e: any) {
      toast({ title: t("common.error"), description: e.message || "Revoke failed", variant: "destructive" })
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">{t("tokens.title")}</h1>

      <Card className="p-6 space-y-4">
        <p className="text-muted-foreground">{t("tokens.description")}</p>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <Input placeholder="client_id" value={clientId} onChange={(e) => setClientId(e.target.value)} />
          <Input
            placeholder="client_secret"
            type="password"
            value={clientSecret}
            onChange={(e) => setClientSecret(e.target.value)}
          />
          <div className="md:col-span-2">
            <Input
              placeholder="paste access or refresh token here"
              value={token}
              onChange={(e) => setToken(e.target.value)}
            />
          </div>
          <label className="flex items-center gap-2 text-sm text-muted-foreground">
            <input type="checkbox" checked={includeSys} onChange={(e) => setIncludeSys(e.target.checked)} />
            include_sys (roles/perms)
          </label>
        </div>
        <div className="flex gap-2">
          <Button onClick={introspect} disabled={!clientId || !clientSecret || !token}>
            {t("tokens.introspect")}
          </Button>
          <Button variant="destructive" onClick={revoke} disabled={!clientId || !clientSecret || !token}>
            {t("tokens.revoke")}
          </Button>
        </div>
        {result && (
          <pre className="mt-4 overflow-auto rounded bg-muted p-4 text-xs">
{JSON.stringify(result, null, 2)}
          </pre>
        )}
        <p className="text-xs text-muted-foreground">
          Tenant: <code className="bg-muted px-1 py-0.5">{tenantId}</code> (not required for introspection)
        </p>
      </Card>
    </div>
  )
}
