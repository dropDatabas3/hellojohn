"use client"

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useParams, useRouter, useSearchParams } from "next/navigation"
import { ArrowLeft, Settings, Users, Key, Shield, DatabaseZap, Globe2, KeyRound } from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { useToast } from "@/hooks/use-toast"
import Link from "next/link"
import type { Tenant } from "@/lib/types"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useState } from "react"

export default function TenantDetailClientPage() {
  const params = useParams()
  const searchParams = useSearchParams()
  const router = useRouter()
  const { t } = useI18n()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const tenantId = (params.id as string) || (searchParams.get("id") as string)

  const { data: tenant, isLoading } = useQuery({
    queryKey: ["tenant", tenantId],
    enabled: !!tenantId,
    queryFn: async () => {
      try {
        return await api.get<Tenant>(`/v1/admin/tenants/${tenantId}`)
      } catch (e: any) {
        if (e?.status === 404) return null as unknown as Tenant
        throw e
      }
    },
  })

  // Load related metrics for dashboard
  const { data: clients } = useQuery({
    queryKey: ["tenant-clients", tenantId],
    enabled: !!tenantId,
    queryFn: () => api.get<any[]>(`/v1/admin/clients?tenant_id=${tenantId}`),
  })
  const { data: scopes } = useQuery({
    queryKey: ["tenant-scopes", tenantId],
    enabled: !!tenantId,
    queryFn: () => api.get<any[]>(`/v1/admin/scopes?tenant_id=${tenantId}`),
  })

  // Per-tenant OIDC discovery
  const { data: oidc } = useQuery({
    queryKey: ["tenant-oidc", tenantId],
    enabled: !!tenantId,
    queryFn: async () => {
      const res = await fetch(`${api.getBaseUrl()}/v1/tenants/${tenantId}/.well-known/openid-configuration`, {
        headers: { Accept: "application/json" },
      })
      if (!res.ok) return null
      return res.json()
    },
  })

  // Per-tenant JWKS (public)
  const { data: jwks } = useQuery({
    queryKey: ["tenant-jwks", tenantId],
    enabled: !!tenantId,
    queryFn: async () => {
      const res = await fetch(`${api.getBaseUrl()}/.well-known/jwks/${tenantId}.json`, {
        headers: { Accept: "application/json" },
      })
      if (!res.ok) return null
      return res.json()
    },
  })

  // Test DB connection action
  const testDb = useMutation({
    mutationFn: async () => {
      await api.post(`/v1/admin/tenants/${tenantId}/user-store/test-connection`)
    },
    onSuccess: () => toast({ title: t("database.connectionSuccess") }),
    onError: (e: any) => toast({ title: t("common.error"), description: e?.error_description || e?.message, variant: "destructive" }),
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  if (!!tenantId && !tenant) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">{t("tenants.notFound")}</p>
      </div>
    )
  }
  if (!tenantId) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">Selecciona una organización</p>
      </div>
    )
  }

  const tnt = tenant as Tenant

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="sm" asChild>
            <Link href="/admin/tenants">
              <ArrowLeft className="h-4 w-4" />
            </Link>
          </Button>
          <div>
            <h1 className="text-3xl font-bold">{tnt.name}</h1>
            <p className="text-muted-foreground">
              {t("tenants.slug")}: <code className="rounded bg-muted px-2 py-1 text-sm">{tnt.slug}</code>
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" asChild>
            <Link href={`/admin/tenants/settings?id=${tenantId}`}>
              <Settings className="mr-2 h-4 w-4" />
              {t("common.settings")}
            </Link>
          </Button>
        </div>
      </div>

      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
        <Card className="p-6">
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
              <KeyRound className="h-6 w-6 text-primary" />
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Clientes</p>
              <Badge variant="secondary">{clients?.length ?? 0}</Badge>
            </div>
          </div>
        </Card>

        <Card className="p-6">
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-blue-500/10">
              <Shield className="h-6 w-6 text-blue-500" />
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Scopes</p>
              <Badge variant="secondary">{scopes?.length ?? 0}</Badge>
            </div>
          </div>
        </Card>

        <Card className="p-6">
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-green-500/10">
              <Users className="h-6 w-6 text-green-500" />
            </div>
            <div>
              <p className="text-sm text-muted-foreground">{t("tenants.createdAt")}</p>
              <p className="text-sm">{new Date(tnt.createdAt).toLocaleDateString()}</p>
            </div>
          </div>
        </Card>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        <Card className="p-6">
          <h2 className="mb-4 text-xl font-semibold">{t("tenants.details")}</h2>
          <dl className="space-y-3">
            <div>
              <dt className="text-sm text-muted-foreground">{t("tenants.name")}</dt>
              <dd className="font-medium">{tnt.name}</dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">{t("tenants.slug")}</dt>
              <dd>
                <code className="rounded bg-muted px-2 py-1 text-sm">{tnt.slug}</code>
              </dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">{t("tenants.displayName")}</dt>
              <dd className="font-medium">-</dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">{t("tenants.tenantId")}</dt>
              <dd className="font-mono text-sm">{tnt.id}</dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">Issuer</dt>
              <dd className="font-mono text-xs break-all">{oidc?.issuer || "-"}</dd>
            </div>
          </dl>
        </Card>

        <Card className="p-6">
          <h2 className="mb-4 text-xl font-semibold">{t("tenants.quickActions")}</h2>
          <div className="space-y-2">
            <Button variant="outline" className="w-full justify-start bg-transparent" asChild>
              <Link href={`/admin/tenants/clients?id=${tenantId}`}>
                <Key className="mr-2 h-4 w-4" />
                {t("clients.manage")}
              </Link>
            </Button>
            <Button variant="outline" className="w-full justify-start bg-transparent" asChild>
              <Link href={`/admin/tenants/scopes?id=${tenantId}`}>
                <Shield className="mr-2 h-4 w-4" />
                {t("scopes.manage")}
              </Link>
            </Button>
            <Button variant="outline" className="w-full justify-start bg-transparent" asChild>
              <Link href={`/admin/tenants/users?id=${tenantId}`}>
                <Users className="mr-2 h-4 w-4" />
                {t("users.manage")}
              </Link>
            </Button>
            <Button variant="outline" className="w-full justify-start bg-transparent" asChild>
              <Link href={`/admin/tenants/settings?id=${tenantId}`}>
                <Settings className="mr-2 h-4 w-4" />
                {t("tenants.settings")}
              </Link>
            </Button>
            <div className="pt-2" />
            <Button variant="outline" className="w-full justify-start" onClick={() => testDb.mutate()} disabled={testDb.isPending}>
              <DatabaseZap className="mr-2 h-4 w-4" /> {testDb.isPending ? t("database.testing") : t("database.testConnection")}
            </Button>
          </div>
        </Card>
      </div>

      {/* OIDC + Keys panels */}
      <div className="grid gap-6 md:grid-cols-2">
        <Card className="p-6">
          <h2 className="mb-4 text-xl font-semibold">OIDC</h2>
          {oidc ? (
            <dl className="space-y-2 text-sm">
              <div>
                <dt className="text-muted-foreground">Issuer</dt>
                <dd className="font-mono break-all">{oidc.issuer}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Auth</dt>
                <dd className="font-mono break-all">{oidc.authorization_endpoint}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Token</dt>
                <dd className="font-mono break-all">{oidc.token_endpoint}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">JWKS</dt>
                <dd className="font-mono break-all">{oidc.jwks_uri}</dd>
              </div>
            </dl>
          ) : (
            <p className="text-sm text-muted-foreground">{t("oidc.noDiscovery")}</p>
          )}
        </Card>
        <Card className="p-6">
          <h2 className="mb-4 text-xl font-semibold">JWKS</h2>
          {jwks?.keys ? (
            <div className="space-y-2 text-sm">
              <p className="text-muted-foreground">{jwks.keys.length} key(s)</p>
              <ul className="list-disc pl-5">
                {jwks.keys.slice(0, 5).map((k: any) => (
                  <li key={k.kid} className="font-mono truncate">{k.kid}</li>
                ))}
              </ul>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">No keys</p>
          )}
        </Card>
      </div>
    </div>
  )
}
