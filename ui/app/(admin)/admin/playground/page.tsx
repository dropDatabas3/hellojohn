"use client"

import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Play, Copy, Check, Code } from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { useToast } from "@/hooks/use-toast"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import type { Tenant, Client } from "@/lib/types"
import { Textarea } from "@/components/ui/textarea"

export default function PlaygroundPage() {
  const { t } = useI18n()
  const { toast } = useToast()
  const [selectedTenant, setSelectedTenant] = useState<string>("")
  const [selectedClient, setSelectedClient] = useState<string>("")
  const [responseType, setResponseType] = useState<string>("code")
  const [scope, setScope] = useState<string>("openid profile email")
  const [redirectUri, setRedirectUri] = useState<string>("")
  const [state, setState] = useState<string>("")
  const [nonce, setNonce] = useState<string>("")
  const [authUrl, setAuthUrl] = useState<string>("")
  const [tokenCode, setTokenCode] = useState<string>("")
  const [tokenResponse, setTokenResponse] = useState<any>(null)
  const [copiedUrl, setCopiedUrl] = useState(false)

  const { data: tenants } = useQuery({
    queryKey: ["tenants"],
    queryFn: () => api.get<Tenant[]>("/v1/tenants"),
  })

  const { data: clients } = useQuery({
    queryKey: ["clients", selectedTenant],
    queryFn: () => api.get<Client[]>(`/v1/tenants/${selectedTenant}/clients`),
    enabled: !!selectedTenant,
  })

  const selectedClientData = clients?.find((c) => c.id === selectedClient)

  const generateAuthUrl = () => {
    if (!selectedTenant || !selectedClient || !redirectUri) {
      toast({
        title: t("common.error"),
        description: t("playground.fillRequired"),
        variant: "destructive",
      })
      return
    }

    const params = new URLSearchParams({
      client_id: selectedClientData?.clientId || "",
      response_type: responseType,
      redirect_uri: redirectUri,
      scope: scope,
      state: state || Math.random().toString(36).substring(7),
      nonce: nonce || Math.random().toString(36).substring(7),
    })

    const url = `${window.location.origin}/v1/tenants/${selectedTenant}/authorize?${params.toString()}`
    setAuthUrl(url)
  }

  const exchangeToken = async () => {
    if (!tokenCode || !selectedClient || !redirectUri) {
      toast({
        title: t("common.error"),
        description: t("playground.fillTokenRequired"),
        variant: "destructive",
      })
      return
    }

    try {
      const response = await fetch(`/v1/tenants/${selectedTenant}/token`, {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
        },
        body: new URLSearchParams({
          grant_type: "authorization_code",
          code: tokenCode,
          redirect_uri: redirectUri,
          client_id: selectedClientData?.clientId || "",
          client_secret: selectedClientData?.secret || "",
        }),
      })

      const data = await response.json()
      setTokenResponse(data)

      if (response.ok) {
        toast({
          title: t("common.success"),
          description: t("playground.tokenExchanged"),
        })
      } else {
        toast({
          title: t("common.error"),
          description: data.error_description || data.error,
          variant: "destructive",
        })
      }
    } catch (error: any) {
      toast({
        title: t("common.error"),
        description: error.message,
        variant: "destructive",
      })
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopiedUrl(true)
    setTimeout(() => setCopiedUrl(false), 2000)
    toast({
      title: t("common.copied"),
      description: t("playground.urlCopied"),
    })
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">{t("playground.title")}</h1>
        <p className="text-muted-foreground">{t("playground.description")}</p>
      </div>

      <Tabs defaultValue="authorize" className="space-y-6">
        <TabsList>
          <TabsTrigger value="authorize">{t("playground.authorize")}</TabsTrigger>
          <TabsTrigger value="token">{t("playground.token")}</TabsTrigger>
        </TabsList>

        <TabsContent value="authorize" className="space-y-6">
          <Card className="p-6">
            <h2 className="mb-4 text-xl font-semibold">{t("playground.authorizationRequest")}</h2>
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="tenant">{t("playground.tenant")} *</Label>
                <Select value={selectedTenant} onValueChange={setSelectedTenant}>
                  <SelectTrigger>
                    <SelectValue placeholder={t("playground.selectTenant")} />
                  </SelectTrigger>
                  <SelectContent>
                    {tenants?.map((tenant) => (
                      <SelectItem key={tenant.id} value={tenant.id}>
                        {tenant.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="client">{t("playground.client")} *</Label>
                <Select value={selectedClient} onValueChange={setSelectedClient} disabled={!selectedTenant}>
                  <SelectTrigger>
                    <SelectValue placeholder={t("playground.selectClient")} />
                  </SelectTrigger>
                  <SelectContent>
                    {clients?.map((client) => (
                      <SelectItem key={client.id} value={client.id}>
                        {client.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="response_type">{t("playground.responseType")} *</Label>
                <Select value={responseType} onValueChange={setResponseType}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="code">code</SelectItem>
                    <SelectItem value="token">token</SelectItem>
                    <SelectItem value="id_token">id_token</SelectItem>
                    <SelectItem value="code id_token">code id_token</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="redirect_uri">{t("playground.redirectUri")} *</Label>
                <Input
                  id="redirect_uri"
                  value={redirectUri}
                  onChange={(e) => setRedirectUri(e.target.value)}
                  placeholder="https://example.com/callback"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="scope">{t("playground.scope")}</Label>
                <Input
                  id="scope"
                  value={scope}
                  onChange={(e) => setScope(e.target.value)}
                  placeholder="openid profile email"
                />
              </div>

              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="state">{t("playground.state")}</Label>
                  <Input
                    id="state"
                    value={state}
                    onChange={(e) => setState(e.target.value)}
                    placeholder={t("playground.autoGenerated")}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="nonce">{t("playground.nonce")}</Label>
                  <Input
                    id="nonce"
                    value={nonce}
                    onChange={(e) => setNonce(e.target.value)}
                    placeholder={t("playground.autoGenerated")}
                  />
                </div>
              </div>

              <Button onClick={generateAuthUrl} className="w-full">
                <Code className="mr-2 h-4 w-4" />
                {t("playground.generateUrl")}
              </Button>

              {authUrl && (
                <div className="space-y-2">
                  <Label>{t("playground.authorizationUrl")}</Label>
                  <div className="flex gap-2">
                    <Textarea value={authUrl} readOnly rows={4} className="font-mono text-sm" />
                    <Button variant="outline" size="sm" onClick={() => copyToClipboard(authUrl)}>
                      {copiedUrl ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                    </Button>
                  </div>
                  <Button onClick={() => window.open(authUrl, "_blank")} variant="secondary" className="w-full">
                    <Play className="mr-2 h-4 w-4" />
                    {t("playground.openInNewTab")}
                  </Button>
                </div>
              )}
            </div>
          </Card>
        </TabsContent>

        <TabsContent value="token" className="space-y-6">
          <Card className="p-6">
            <h2 className="mb-4 text-xl font-semibold">{t("playground.tokenExchange")}</h2>
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="code">{t("playground.authorizationCode")} *</Label>
                <Input
                  id="code"
                  value={tokenCode}
                  onChange={(e) => setTokenCode(e.target.value)}
                  placeholder={t("playground.pasteCode")}
                />
              </div>

              <Button onClick={exchangeToken} className="w-full">
                <Play className="mr-2 h-4 w-4" />
                {t("playground.exchangeToken")}
              </Button>

              {tokenResponse && (
                <div className="space-y-2">
                  <Label>{t("playground.tokenResponse")}</Label>
                  <Textarea
                    value={JSON.stringify(tokenResponse, null, 2)}
                    readOnly
                    rows={12}
                    className="font-mono text-sm"
                  />
                </div>
              )}
            </div>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
