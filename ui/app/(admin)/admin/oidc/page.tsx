"use client"

import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Copy, Check, ExternalLink } from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { useToast } from "@/hooks/use-toast"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import type { OIDCDiscovery } from "@/lib/types"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"

export default function OIDCPage() {
  const { t } = useI18n()
  const { toast } = useToast()
  const [tenantSlug, setTenantSlug] = useState("")
  const [copiedField, setCopiedField] = useState<string | null>(null)

  const { data: discovery, isLoading } = useQuery({
    queryKey: ["oidc-discovery", tenantSlug],
    queryFn: () => {
      const path = tenantSlug
        ? `/v1/tenants/${tenantSlug}/.well-known/openid-configuration`
        : "/.well-known/openid-configuration"
      return api.get<OIDCDiscovery>(path)
    },
    enabled: true,
  })

  const copyToClipboard = (text: string, field: string) => {
    navigator.clipboard.writeText(text)
    setCopiedField(field)
    setTimeout(() => setCopiedField(null), 2000)
    toast({
      title: t("common.copied"),
      description: t("oidc.fieldCopied"),
    })
  }

  const openInNewTab = (url: string) => {
    window.open(url, "_blank")
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">{t("oidc.title")}</h1>
        <p className="text-muted-foreground">{t("oidc.description")}</p>
      </div>

      <Card className="p-6">
        <div className="mb-6 space-y-2">
          <Label htmlFor="tenant">{t("oidc.tenantSlug")}</Label>
          <Input
            id="tenant"
            value={tenantSlug}
            onChange={(e) => setTenantSlug(e.target.value)}
            placeholder={t("oidc.tenantSlugPlaceholder")}
          />
          <p className="text-sm text-muted-foreground">{t("oidc.tenantSlugHint")}</p>
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center py-8">
            <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
          </div>
        ) : discovery ? (
          <Tabs defaultValue="endpoints" className="space-y-6">
            <TabsList>
              <TabsTrigger value="endpoints">{t("oidc.endpoints")}</TabsTrigger>
              <TabsTrigger value="supported">{t("oidc.supported")}</TabsTrigger>
            </TabsList>

            <TabsContent value="endpoints" className="space-y-3">
              <div className="space-y-3">
                <div className="flex items-center justify-between rounded-lg border p-4">
                  <div className="flex-1">
                    <p className="mb-1 text-sm font-medium">{t("oidc.issuer")}</p>
                    <code className="text-sm text-muted-foreground">{discovery.issuer}</code>
                  </div>
                  <div className="flex gap-2">
                    <Button variant="ghost" size="sm" onClick={() => copyToClipboard(discovery.issuer, "issuer")}>
                      {copiedField === "issuer" ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => openInNewTab(discovery.issuer)}>
                      <ExternalLink className="h-4 w-4" />
                    </Button>
                  </div>
                </div>

                <div className="flex items-center justify-between rounded-lg border p-4">
                  <div className="flex-1">
                    <p className="mb-1 text-sm font-medium">{t("oidc.authorizationEndpoint")}</p>
                    <code className="text-sm text-muted-foreground">{discovery.authorization_endpoint}</code>
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => copyToClipboard(discovery.authorization_endpoint, "auth")}
                  >
                    {copiedField === "auth" ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  </Button>
                </div>

                <div className="flex items-center justify-between rounded-lg border p-4">
                  <div className="flex-1">
                    <p className="mb-1 text-sm font-medium">{t("oidc.tokenEndpoint")}</p>
                    <code className="text-sm text-muted-foreground">{discovery.token_endpoint}</code>
                  </div>
                  <Button variant="ghost" size="sm" onClick={() => copyToClipboard(discovery.token_endpoint, "token")}>
                    {copiedField === "token" ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  </Button>
                </div>

                <div className="flex items-center justify-between rounded-lg border p-4">
                  <div className="flex-1">
                    <p className="mb-1 text-sm font-medium">{t("oidc.userinfoEndpoint")}</p>
                    <code className="text-sm text-muted-foreground">{discovery.userinfo_endpoint}</code>
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => copyToClipboard(discovery.userinfo_endpoint, "userinfo")}
                  >
                    {copiedField === "userinfo" ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  </Button>
                </div>

                <div className="flex items-center justify-between rounded-lg border p-4">
                  <div className="flex-1">
                    <p className="mb-1 text-sm font-medium">{t("oidc.jwksUri")}</p>
                    <code className="text-sm text-muted-foreground">{discovery.jwks_uri}</code>
                  </div>
                  <div className="flex gap-2">
                    <Button variant="ghost" size="sm" onClick={() => copyToClipboard(discovery.jwks_uri, "jwks")}>
                      {copiedField === "jwks" ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => openInNewTab(discovery.jwks_uri)}>
                      <ExternalLink className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </div>
            </TabsContent>

            <TabsContent value="supported" className="space-y-4">
              <div className="space-y-3">
                <div className="rounded-lg border p-4">
                  <p className="mb-2 text-sm font-medium">{t("oidc.responseTypes")}</p>
                  <div className="flex flex-wrap gap-2">
                    {discovery.response_types_supported.map((type) => (
                      <code key={type} className="rounded bg-muted px-2 py-1 text-sm">
                        {type}
                      </code>
                    ))}
                  </div>
                </div>

                <div className="rounded-lg border p-4">
                  <p className="mb-2 text-sm font-medium">{t("oidc.scopes")}</p>
                  <div className="flex flex-wrap gap-2">
                    {discovery.scopes_supported.map((scope) => (
                      <code key={scope} className="rounded bg-muted px-2 py-1 text-sm">
                        {scope}
                      </code>
                    ))}
                  </div>
                </div>

                <div className="rounded-lg border p-4">
                  <p className="mb-2 text-sm font-medium">{t("oidc.signingAlgorithms")}</p>
                  <div className="flex flex-wrap gap-2">
                    {discovery.id_token_signing_alg_values_supported.map((alg) => (
                      <code key={alg} className="rounded bg-muted px-2 py-1 text-sm">
                        {alg}
                      </code>
                    ))}
                  </div>
                </div>

                <div className="rounded-lg border p-4">
                  <p className="mb-2 text-sm font-medium">{t("oidc.tokenAuthMethods")}</p>
                  <div className="flex flex-wrap gap-2">
                    {discovery.token_endpoint_auth_methods_supported.map((method) => (
                      <code key={method} className="rounded bg-muted px-2 py-1 text-sm">
                        {method}
                      </code>
                    ))}
                  </div>
                </div>

                <div className="rounded-lg border p-4">
                  <p className="mb-2 text-sm font-medium">{t("oidc.claims")}</p>
                  <div className="flex flex-wrap gap-2">
                    {discovery.claims_supported.map((claim) => (
                      <code key={claim} className="rounded bg-muted px-2 py-1 text-sm">
                        {claim}
                      </code>
                    ))}
                  </div>
                </div>
              </div>
            </TabsContent>
          </Tabs>
        ) : (
          <p className="text-center text-muted-foreground">{t("oidc.noDiscovery")}</p>
        )}
      </Card>
    </div>
  )
}
