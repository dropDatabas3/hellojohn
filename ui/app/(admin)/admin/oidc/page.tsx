"use client"

import { useState, useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import {
  Copy, Check, ExternalLink, Globe, Shield, Key, Lock,
  Info, HelpCircle, Building2, RefreshCw, Terminal, FileCode,
  CheckCircle2, Link, User, FileJson, Fingerprint, Mail
} from "lucide-react"
import { api } from "@/lib/api"
import { API_ROUTES } from "@/lib/routes"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { useToast } from "@/hooks/use-toast"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import type { OIDCDiscovery, Tenant } from "@/lib/types"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"

// ─── Helper Components ───

function InfoTooltip({ content }: { content: string }) {
  return (
    <TooltipProvider delayDuration={200}>
      <Tooltip>
        <TooltipTrigger asChild>
          <button type="button" className="ml-1.5 inline-flex">
            <HelpCircle className="h-3.5 w-3.5 text-muted-foreground/60 hover:text-muted-foreground transition-colors" />
          </button>
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-xs text-xs">
          {content}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

function StatCard({
  icon: Icon,
  label,
  value,
  color = "zinc"
}: {
  icon: React.ElementType
  label: string
  value: string | number
  color?: "zinc" | "emerald" | "blue" | "indigo"
}) {
  const colorClasses = {
    zinc: "from-zinc-500/10 to-zinc-500/5 border-zinc-500/10",
    emerald: "from-emerald-500/10 to-emerald-500/5 border-emerald-500/10",
    blue: "from-blue-500/10 to-blue-500/5 border-blue-500/10",
    indigo: "from-indigo-500/10 to-indigo-500/5 border-indigo-500/10",
  }
  const iconColors = {
    zinc: "text-zinc-500",
    emerald: "text-emerald-500",
    blue: "text-blue-500",
    indigo: "text-indigo-500",
  }

  return (
    <div className={cn(
      "rounded-xl border bg-gradient-to-br p-4",
      colorClasses[color]
    )}>
      <div className="flex items-center gap-3">
        <Icon className={cn("h-5 w-5", iconColors[color])} />
        <div>
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className="text-lg font-semibold">{value}</p>
        </div>
      </div>
    </div>
  )
}

// ─── Endpoint Descriptions ───

const ENDPOINT_INFO: Record<string, { icon: React.ElementType; description: string; method: string }> = {
  issuer: {
    icon: Globe,
    description: "Identificador único del servidor de autorización. Se usa para verificar el 'iss' claim en los tokens.",
    method: "Identifier",
  },
  authorization_endpoint: {
    icon: Shield,
    description: "URL donde los usuarios inician sesión. Redirigí aquí para comenzar el flujo OAuth2/OIDC.",
    method: "GET",
  },
  token_endpoint: {
    icon: Key,
    description: "URL para intercambiar códigos de autorización por tokens o refrescar tokens existentes.",
    method: "POST",
  },
  userinfo_endpoint: {
    icon: User,
    description: "URL para obtener información del usuario autenticado usando el access token.",
    method: "GET/POST",
  },
  jwks_uri: {
    icon: Fingerprint,
    description: "URL con las claves públicas (JWK Set) para verificar firmas de tokens JWT.",
    method: "GET",
  },
  revocation_endpoint: {
    icon: Lock,
    description: "URL para revocar tokens de acceso o refresh tokens.",
    method: "POST",
  },
  introspection_endpoint: {
    icon: FileJson,
    description: "URL para verificar si un token es válido y obtener sus claims.",
    method: "POST",
  },
  end_session_endpoint: {
    icon: Shield,
    description: "URL para cerrar sesión del usuario (logout). Soporta redirect post-logout.",
    method: "GET",
  },
}

// ─── Scope Descriptions ───

const SCOPE_INFO: Record<string, { description: string; claims: string[] }> = {
  openid: {
    description: "Requerido para OIDC. Incluye el ID token con el claim 'sub'.",
    claims: ["sub", "iss", "aud", "exp", "iat"],
  },
  profile: {
    description: "Información básica del perfil del usuario.",
    claims: ["name", "family_name", "given_name", "nickname", "picture", "updated_at"],
  },
  email: {
    description: "Dirección de email del usuario.",
    claims: ["email", "email_verified"],
  },
  phone: {
    description: "Número de teléfono del usuario.",
    claims: ["phone_number", "phone_number_verified"],
  },
  address: {
    description: "Dirección postal del usuario.",
    claims: ["address"],
  },
  offline_access: {
    description: "Permite obtener refresh tokens para acceso prolongado.",
    claims: [],
  },
}

export default function OIDCPage() {
  const { t } = useI18n()
  const { toast } = useToast()
  const [selectedTenant, setSelectedTenant] = useState<string>("global")
  const [copiedField, setCopiedField] = useState<string | null>(null)

  // Fetch tenants
  const { data: tenants } = useQuery({
    queryKey: ["tenants"],
    queryFn: () => api.get<{ tenants: Tenant[] }>(API_ROUTES.ADMIN_TENANTS),
    select: (data) => data.tenants || [],
  })

  // Fetch OIDC Discovery
  const { data: discovery, isLoading, refetch } = useQuery({
    queryKey: ["oidc-discovery", selectedTenant],
    queryFn: () => {
      const endpoint = selectedTenant === "global"
        ? API_ROUTES.OIDC_DISCOVERY
        : API_ROUTES.OIDC_DISCOVERY_TENANT(selectedTenant)
      return api.get<OIDCDiscovery>(endpoint)
    },
  })

  // Calculate stats
  const stats = useMemo(() => {
    if (!discovery) return null
    return {
      endpointsCount: Object.keys(discovery).filter(k => k.includes("endpoint") || k.includes("uri")).length,
      scopesCount: discovery.scopes_supported?.length || 0,
      claimsCount: discovery.claims_supported?.length || 0,
      responseTypesCount: discovery.response_types_supported?.length || 0,
    }
  }, [discovery])

  const copyToClipboard = (text: string, field: string) => {
    navigator.clipboard.writeText(text)
    setCopiedField(field)
    setTimeout(() => setCopiedField(null), 2000)
    toast({
      title: "Copiado",
      description: "URL copiada al portapapeles",
      variant: "info",
    })
  }

  const openInNewTab = (url: string) => {
    window.open(url, "_blank")
  }

  // Generate cURL examples
  const curlExamples = useMemo(() => {
    if (!discovery) return null

    const authUrl = new URL(discovery.authorization_endpoint)
    authUrl.searchParams.set("response_type", "code")
    authUrl.searchParams.set("client_id", "YOUR_CLIENT_ID")
    authUrl.searchParams.set("redirect_uri", "YOUR_REDIRECT_URI")
    authUrl.searchParams.set("scope", "openid profile email")
    authUrl.searchParams.set("state", "random_state_value")

    return {
      discovery: `# Obtener configuración OIDC Discovery
curl -X GET "${discovery.issuer}/.well-known/openid-configuration" \\
  -H "Accept: application/json"`,

      jwks: `# Obtener claves públicas (JWKS)
curl -X GET "${discovery.jwks_uri}" \\
  -H "Accept: application/json"`,

      token: `# Intercambiar código por tokens
curl -X POST "${discovery.token_endpoint}" \\
  -H "Content-Type: application/x-www-form-urlencoded" \\
  -d "grant_type=authorization_code" \\
  -d "code=YOUR_AUTH_CODE" \\
  -d "redirect_uri=YOUR_REDIRECT_URI" \\
  -d "client_id=YOUR_CLIENT_ID" \\
  -d "client_secret=YOUR_CLIENT_SECRET"  # Solo para clients confidenciales`,

      userinfo: `# Obtener información del usuario
curl -X GET "${discovery.userinfo_endpoint}" \\
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"`,

      introspect: discovery.introspection_endpoint ? `# Introspección de token
curl -X POST "${discovery.introspection_endpoint}" \\
  -H "Content-Type: application/x-www-form-urlencoded" \\
  -d "token=YOUR_TOKEN" \\
  -d "client_id=YOUR_CLIENT_ID" \\
  -d "client_secret=YOUR_CLIENT_SECRET"` : null,
    }
  }, [discovery])

  const selectedTenantName = useMemo(() => {
    if (selectedTenant === "global") return "Global"
    return tenants?.find(t => t.slug === selectedTenant)?.name || selectedTenant
  }, [selectedTenant, tenants])

  return (
    <div className="space-y-6 max-w-5xl">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">OIDC Discovery</h1>
          <p className="text-muted-foreground">
            Configuración OpenID Connect de tu servidor de autorización
          </p>
        </div>
        <Button
          variant="outline"
          onClick={() => {
            const endpoint = selectedTenant === "global"
              ? `${window.location.origin}${API_ROUTES.OIDC_DISCOVERY}`
              : `${window.location.origin}${API_ROUTES.OIDC_DISCOVERY_TENANT(selectedTenant)}`
            openInNewTab(endpoint)
          }}
        >
          <ExternalLink className="h-4 w-4 mr-2" />
          Abrir JSON
        </Button>
      </div>

      {/* Info Banner */}
      <div className="rounded-xl border border-indigo-500/20 bg-gradient-to-r from-indigo-500/10 via-indigo-500/5 to-transparent p-4">
        <div className="flex gap-4">
          <div className="flex-shrink-0">
            <div className="h-10 w-10 rounded-lg bg-indigo-500/10 flex items-center justify-center">
              <Info className="h-5 w-5 text-indigo-500" />
            </div>
          </div>
          <div className="space-y-2">
            <h3 className="font-semibold text-indigo-700 dark:text-indigo-400 flex items-center gap-2">
              ¿Qué es OIDC Discovery?
              <InfoTooltip content="OpenID Connect Discovery es una especificación que permite a los clientes obtener automáticamente la configuración del servidor" />
            </h3>
            <p className="text-sm text-muted-foreground leading-relaxed">
              El endpoint <code className="text-xs bg-zinc-100 dark:bg-zinc-800 px-1 rounded">/.well-known/openid-configuration</code> expone
              la <strong>configuración pública</strong> de tu servidor OAuth2/OIDC. Los clientes lo usan para descubrir
              automáticamente todos los endpoints, algoritmos soportados, y capacidades del servidor sin configuración manual.
            </p>
            <div className="flex flex-wrap gap-3 pt-2">
              <Badge variant="outline" className="gap-1">
                <CheckCircle2 className="h-3 w-3 text-emerald-500" />
                Estándar RFC 8414
              </Badge>
              <Badge variant="outline" className="gap-1">
                <Globe className="h-3 w-3 text-blue-500" />
                Endpoint público
              </Badge>
              <Badge variant="outline" className="gap-1">
                <Shield className="h-3 w-3 text-indigo-500" />
                Sin autenticación
              </Badge>
            </div>
          </div>
        </div>
      </div>

      {/* Tenant Selector */}
      <div className="flex items-center gap-3 p-4 rounded-xl border bg-zinc-50/50 dark:bg-zinc-900/50">
        <Globe className="h-5 w-5 text-muted-foreground" />
        <div className="flex-1">
          <label className="text-sm font-medium">Seleccionar Tenant</label>
          <p className="text-xs text-muted-foreground">
            Cada tenant puede tener su propia configuración OIDC con issuer único
          </p>
        </div>
        <Select value={selectedTenant} onValueChange={setSelectedTenant}>
          <SelectTrigger className="w-[220px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="global">
              <div className="flex items-center gap-2">
                <Globe className="h-4 w-4" />
                Global (Default)
              </div>
            </SelectItem>
            {tenants?.map(tenant => (
              <SelectItem key={tenant.id} value={tenant.slug || tenant.id}>
                <div className="flex items-center gap-2">
                  <Building2 className="h-4 w-4" />
                  {tenant.name}
                </div>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button variant="ghost" size="icon" onClick={() => refetch()}>
          <RefreshCw className={cn("h-4 w-4", isLoading && "animate-spin")} />
        </Button>
      </div>

      {/* Stats */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <StatCard icon={Link} label="Endpoints" value={stats.endpointsCount} color="blue" />
          <StatCard icon={Shield} label="Scopes" value={stats.scopesCount} color="emerald" />
          <StatCard icon={Mail} label="Claims" value={stats.claimsCount} color="indigo" />
          <StatCard icon={FileJson} label="Response Types" value={stats.responseTypesCount} color="zinc" />
        </div>
      )}

      {/* Content */}
      {isLoading ? (
        <div className="flex items-center justify-center py-16">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
        </div>
      ) : discovery ? (
        <Tabs defaultValue="endpoints" className="w-full">
          <TabsList className="bg-zinc-100 dark:bg-zinc-800/50 w-full justify-start">
            <TabsTrigger value="endpoints" className="gap-2">
              <Link className="h-4 w-4" />
              Endpoints
            </TabsTrigger>
            <TabsTrigger value="capabilities" className="gap-2">
              <Shield className="h-4 w-4" />
              Capacidades
            </TabsTrigger>
            <TabsTrigger value="scopes" className="gap-2">
              <Key className="h-4 w-4" />
              Scopes & Claims
            </TabsTrigger>
            <TabsTrigger value="examples" className="gap-2">
              <Terminal className="h-4 w-4" />
              Ejemplos cURL
            </TabsTrigger>
            <TabsTrigger value="raw" className="gap-2">
              <FileCode className="h-4 w-4" />
              JSON Raw
            </TabsTrigger>
          </TabsList>

          {/* Endpoints Tab */}
          <TabsContent value="endpoints" className="mt-4 space-y-3">
            <p className="text-sm text-muted-foreground mb-4">
              URLs que tu aplicación necesita para interactuar con el servidor OAuth2/OIDC.
            </p>

            {/* Issuer */}
            <EndpointCard
              name="Issuer"
              value={discovery.issuer}
              info={ENDPOINT_INFO.issuer}
              copiedField={copiedField}
              onCopy={copyToClipboard}
              onOpen={openInNewTab}
              canOpen={true}
            />

            {/* Authorization Endpoint */}
            <EndpointCard
              name="Authorization Endpoint"
              value={discovery.authorization_endpoint}
              info={ENDPOINT_INFO.authorization_endpoint}
              copiedField={copiedField}
              onCopy={copyToClipboard}
            />

            {/* Token Endpoint */}
            <EndpointCard
              name="Token Endpoint"
              value={discovery.token_endpoint}
              info={ENDPOINT_INFO.token_endpoint}
              copiedField={copiedField}
              onCopy={copyToClipboard}
            />

            {/* Userinfo Endpoint */}
            <EndpointCard
              name="Userinfo Endpoint"
              value={discovery.userinfo_endpoint}
              info={ENDPOINT_INFO.userinfo_endpoint}
              copiedField={copiedField}
              onCopy={copyToClipboard}
            />

            {/* JWKS URI */}
            <EndpointCard
              name="JWKS URI"
              value={discovery.jwks_uri}
              info={ENDPOINT_INFO.jwks_uri}
              copiedField={copiedField}
              onCopy={copyToClipboard}
              onOpen={openInNewTab}
              canOpen={true}
            />

            {/* Revocation Endpoint */}
            {discovery.revocation_endpoint && (
              <EndpointCard
                name="Revocation Endpoint"
                value={discovery.revocation_endpoint}
                info={ENDPOINT_INFO.revocation_endpoint}
                copiedField={copiedField}
                onCopy={copyToClipboard}
              />
            )}

            {/* Introspection Endpoint */}
            {discovery.introspection_endpoint && (
              <EndpointCard
                name="Introspection Endpoint"
                value={discovery.introspection_endpoint}
                info={ENDPOINT_INFO.introspection_endpoint}
                copiedField={copiedField}
                onCopy={copyToClipboard}
              />
            )}

            {/* End Session Endpoint */}
            {discovery.end_session_endpoint && (
              <EndpointCard
                name="End Session Endpoint"
                value={discovery.end_session_endpoint}
                info={ENDPOINT_INFO.end_session_endpoint}
                copiedField={copiedField}
                onCopy={copyToClipboard}
              />
            )}
          </TabsContent>

          {/* Capabilities Tab */}
          <TabsContent value="capabilities" className="mt-4 space-y-4">
            <p className="text-sm text-muted-foreground mb-4">
              Funcionalidades y algoritmos soportados por el servidor.
            </p>

            {/* Response Types */}
            <CapabilityCard
              title="Response Types Soportados"
              description="Tipos de respuesta que el servidor puede generar en el endpoint de autorización"
              items={discovery.response_types_supported}
              icon={FileJson}
            />

            {/* Grant Types */}
            {discovery.grant_types_supported && (
              <CapabilityCard
                title="Grant Types Soportados"
                description="Flujos OAuth2 disponibles para obtener tokens"
                items={discovery.grant_types_supported}
                icon={Shield}
                highlights={["authorization_code", "refresh_token"]}
              />
            )}

            {/* Signing Algorithms */}
            <CapabilityCard
              title="Algoritmos de Firma (ID Token)"
              description="Algoritmos criptográficos usados para firmar ID tokens"
              items={discovery.id_token_signing_alg_values_supported}
              icon={Key}
              highlights={["EdDSA", "ES256"]}
            />

            {/* Token Auth Methods */}
            <CapabilityCard
              title="Métodos de Autenticación (Token Endpoint)"
              description="Cómo los clientes se autentican al solicitar tokens"
              items={discovery.token_endpoint_auth_methods_supported}
              icon={Lock}
            />

            {/* Code Challenge Methods */}
            {discovery.code_challenge_methods_supported && (
              <CapabilityCard
                title="Métodos PKCE"
                description="Proof Key for Code Exchange - mejora seguridad para clientes públicos"
                items={discovery.code_challenge_methods_supported}
                icon={Shield}
                highlights={["S256"]}
              />
            )}

            {/* Subject Types */}
            {discovery.subject_types_supported && (
              <CapabilityCard
                title="Subject Types"
                description="Cómo se genera el identificador del usuario (sub claim)"
                items={discovery.subject_types_supported}
                icon={User}
              />
            )}
          </TabsContent>

          {/* Scopes & Claims Tab */}
          <TabsContent value="scopes" className="mt-4 space-y-4">
            <p className="text-sm text-muted-foreground mb-4">
              Scopes que puedes solicitar y los claims que incluirán en los tokens.
            </p>

            {/* Scopes */}
            <Card className="p-4">
              <h3 className="font-medium mb-4 flex items-center gap-2">
                <Key className="h-4 w-4 text-emerald-500" />
                Scopes Disponibles
              </h3>
              <div className="space-y-3">
                {discovery.scopes_supported.map(scope => {
                  const info = SCOPE_INFO[scope]
                  return (
                    <div
                      key={scope}
                      className="flex items-start gap-3 p-3 rounded-lg border bg-zinc-50/50 dark:bg-zinc-900/50"
                    >
                      <Badge variant="outline" className="font-mono mt-0.5">
                        {scope}
                      </Badge>
                      <div className="flex-1">
                        <p className="text-sm text-muted-foreground">
                          {info?.description || "Scope personalizado"}
                        </p>
                        {info?.claims && info.claims.length > 0 && (
                          <div className="flex flex-wrap gap-1 mt-2">
                            {info.claims.map(claim => (
                              <code
                                key={claim}
                                className="text-xs bg-zinc-200 dark:bg-zinc-800 px-1.5 py-0.5 rounded"
                              >
                                {claim}
                              </code>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                  )
                })}
              </div>
            </Card>

            {/* Claims */}
            <Card className="p-4">
              <h3 className="font-medium mb-4 flex items-center gap-2">
                <Mail className="h-4 w-4 text-indigo-500" />
                Claims Soportados
                <InfoTooltip content="Claims son piezas de información sobre el usuario que se incluyen en los tokens" />
              </h3>
              <div className="flex flex-wrap gap-2">
                {discovery.claims_supported.map(claim => (
                  <Badge key={claim} variant="secondary" className="font-mono">
                    {claim}
                  </Badge>
                ))}
              </div>
            </Card>
          </TabsContent>

          {/* Examples Tab */}
          <TabsContent value="examples" className="mt-4 space-y-4">
            <p className="text-sm text-muted-foreground mb-4">
              Ejemplos de comandos cURL para interactuar con los endpoints OIDC.
            </p>

            {curlExamples && (
              <div className="space-y-4">
                <CodeBlock
                  title="Discovery"
                  code={curlExamples.discovery}
                  copiedField={copiedField}
                  onCopy={(code) => copyToClipboard(code, "curl-discovery")}
                  fieldKey="curl-discovery"
                />
                <CodeBlock
                  title="JWKS"
                  code={curlExamples.jwks}
                  copiedField={copiedField}
                  onCopy={(code) => copyToClipboard(code, "curl-jwks")}
                  fieldKey="curl-jwks"
                />
                <CodeBlock
                  title="Token Exchange"
                  code={curlExamples.token}
                  copiedField={copiedField}
                  onCopy={(code) => copyToClipboard(code, "curl-token")}
                  fieldKey="curl-token"
                />
                <CodeBlock
                  title="Userinfo"
                  code={curlExamples.userinfo}
                  copiedField={copiedField}
                  onCopy={(code) => copyToClipboard(code, "curl-userinfo")}
                  fieldKey="curl-userinfo"
                />
                {curlExamples.introspect && (
                  <CodeBlock
                    title="Introspección"
                    code={curlExamples.introspect}
                    copiedField={copiedField}
                    onCopy={(code) => copyToClipboard(code, "curl-introspect")}
                    fieldKey="curl-introspect"
                  />
                )}
              </div>
            )}
          </TabsContent>

          {/* Raw JSON Tab */}
          <TabsContent value="raw" className="mt-4">
            <Card className="p-4">
              <div className="flex items-center justify-between mb-4">
                <h3 className="font-medium flex items-center gap-2">
                  <FileCode className="h-4 w-4" />
                  OpenID Configuration (JSON)
                </h3>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => copyToClipboard(JSON.stringify(discovery, null, 2), "raw-json")}
                >
                  {copiedField === "raw-json" ? (
                    <Check className="h-4 w-4 mr-1" />
                  ) : (
                    <Copy className="h-4 w-4 mr-1" />
                  )}
                  Copiar
                </Button>
              </div>
              <div className="rounded-lg bg-zinc-950 p-4 overflow-auto max-h-[500px]">
                <pre className="text-xs text-zinc-100 font-mono">
                  {JSON.stringify(discovery, null, 2)}
                </pre>
              </div>
            </Card>
          </TabsContent>
        </Tabs>
      ) : (
        <Card className="p-12 text-center">
          <Globe className="h-12 w-12 mx-auto mb-4 text-muted-foreground/50" />
          <h3 className="text-lg font-medium mb-2">No se pudo cargar la configuración</h3>
          <p className="text-sm text-muted-foreground mb-4">
            Verifica que el servidor esté funcionando y que el tenant exista.
          </p>
          <Button onClick={() => refetch()}>
            <RefreshCw className="h-4 w-4 mr-2" />
            Reintentar
          </Button>
        </Card>
      )}
    </div>
  )
}

// ─── Sub-components ───

function EndpointCard({
  name,
  value,
  info,
  copiedField,
  onCopy,
  onOpen,
  canOpen = false,
}: {
  name: string
  value: string
  info: { icon: React.ElementType; description: string; method: string }
  copiedField: string | null
  onCopy: (text: string, field: string) => void
  onOpen?: (url: string) => void
  canOpen?: boolean
}) {
  const Icon = info.icon
  const fieldKey = name.toLowerCase().replace(/\s+/g, "-")

  return (
    <div className="rounded-xl border p-4 hover:border-zinc-300 dark:hover:border-zinc-700 transition-colors">
      <div className="flex items-start gap-4">
        <div className="h-10 w-10 rounded-lg bg-zinc-100 dark:bg-zinc-800 flex items-center justify-center flex-shrink-0">
          <Icon className="h-5 w-5 text-zinc-600 dark:text-zinc-400" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <span className="font-medium">{name}</span>
            <Badge variant="outline" className="text-xs font-mono">
              {info.method}
            </Badge>
          </div>
          <p className="text-xs text-muted-foreground mb-2">{info.description}</p>
          <div className="flex items-center gap-2">
            <code className="text-xs bg-zinc-100 dark:bg-zinc-800 px-2 py-1 rounded flex-1 truncate">
              {value}
            </code>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0 flex-shrink-0"
              onClick={() => onCopy(value, fieldKey)}
            >
              {copiedField === fieldKey ? (
                <Check className="h-3.5 w-3.5 text-emerald-500" />
              ) : (
                <Copy className="h-3.5 w-3.5" />
              )}
            </Button>
            {canOpen && onOpen && (
              <Button
                variant="ghost"
                size="sm"
                className="h-7 w-7 p-0 flex-shrink-0"
                onClick={() => onOpen(value)}
              >
                <ExternalLink className="h-3.5 w-3.5" />
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function CapabilityCard({
  title,
  description,
  items,
  icon: Icon,
  highlights = [],
}: {
  title: string
  description: string
  items: string[]
  icon: React.ElementType
  highlights?: string[]
}) {
  return (
    <Card className="p-4">
      <div className="flex items-start gap-3 mb-3">
        <Icon className="h-5 w-5 text-muted-foreground mt-0.5" />
        <div>
          <h3 className="font-medium">{title}</h3>
          <p className="text-xs text-muted-foreground">{description}</p>
        </div>
      </div>
      <div className="flex flex-wrap gap-2">
        {items.map(item => (
          <Badge
            key={item}
            variant={highlights.includes(item) ? "default" : "secondary"}
            className={cn(
              "font-mono",
              highlights.includes(item) && "bg-emerald-500/10 text-emerald-600 border-emerald-500/20"
            )}
          >
            {item}
            {highlights.includes(item) && (
              <CheckCircle2 className="h-3 w-3 ml-1" />
            )}
          </Badge>
        ))}
      </div>
    </Card>
  )
}

function CodeBlock({
  title,
  code,
  copiedField,
  onCopy,
  fieldKey,
}: {
  title: string
  code: string
  copiedField: string | null
  onCopy: (code: string) => void
  fieldKey: string
}) {
  return (
    <Card className="p-4">
      <div className="flex items-center justify-between mb-3">
        <h4 className="text-sm font-medium flex items-center gap-2">
          <Terminal className="h-4 w-4" />
          {title}
        </h4>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onCopy(code)}
        >
          {copiedField === fieldKey ? (
            <Check className="h-4 w-4 mr-1 text-emerald-500" />
          ) : (
            <Copy className="h-4 w-4 mr-1" />
          )}
          Copiar
        </Button>
      </div>
      <div className="rounded-lg bg-zinc-950 p-4 overflow-x-auto">
        <pre className="text-xs text-zinc-100 font-mono whitespace-pre-wrap">
          {code}
        </pre>
      </div>
    </Card>
  )
}
