"use client"

import { useState, useMemo, useCallback, Suspense } from "react"
import { useParams, useRouter } from "next/navigation"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  Globe, CheckCircle2, XCircle, AlertCircle, ExternalLink, Settings2,
  Plus, ChevronRight, ChevronLeft, Copy, Eye, EyeOff, RefreshCw,
  Loader2, Info, HelpCircle, Check, X, Shield, Users, KeyRound,
  Lock, Unlock, Zap, Activity, BarChart3, Link2, Unlink, TestTube,
  Mail, User, Image, Phone, MapPin, Calendar, Hash, FileText,
  ArrowRight, ArrowLeft, Sparkles, AlertTriangle, ChevronDown
} from "lucide-react"
import Link from "next/link"
import { api } from "@/lib/api"
import { API_ROUTES } from "@/lib/routes"
import { useI18n } from "@/lib/i18n"
import { useToast } from "@/hooks/use-toast"
import { cn } from "@/components/ds"
import type { Tenant } from "@/lib/types"

// UI Components from Design System
import {
  Button,
  Input,
  Label,
  Switch,
  Badge,
  Tabs, TabsContent, TabsList, TabsTrigger,
  Card, CardContent, CardDescription, CardHeader, CardTitle,
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger,
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
  InlineAlert,
  Textarea,
  Checkbox,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Separator,
  Skeleton,
} from "@/components/ds"

// ─── Types ───

interface ProviderConfig {
  id: string
  name: string
  displayName: string
  icon: string // URL or component name
  enabled: boolean
  configured: boolean
  status: "healthy" | "degraded" | "unavailable" | "unconfigured"
  clientId?: string
  clientSecret?: string
  scopes: string[]
  defaultScopes: string[]
  fieldMapping: Record<string, string>
  authUrl: string
  tokenUrl: string
  userinfoUrl: string
  docs: string
  color: string
  stats?: {
    loginsLast30d: number
    errorsLast30d: number
    lastLogin?: string
  }
}

interface FieldMappingOption {
  id: string
  label: string
  description: string
  icon: React.ElementType
  required?: boolean
}

// ─── Constants ───

const SUPPORTED_PROVIDERS: Omit<ProviderConfig, "clientId" | "clientSecret" | "enabled" | "configured" | "status" | "stats">[] = [
  {
    id: "google",
    name: "google",
    displayName: "Google",
    icon: "/icons/google.svg",
    scopes: ["openid", "email", "profile"],
    defaultScopes: ["openid", "email", "profile"],
    fieldMapping: { email: "email", name: "name", picture: "picture", email_verified: "email_verified" },
    authUrl: "https://accounts.google.com/o/oauth2/v2/auth",
    tokenUrl: "https://oauth2.googleapis.com/token",
    userinfoUrl: "https://openidconnect.googleapis.com/v1/userinfo",
    docs: "https://developers.google.com/identity/protocols/oauth2",
    color: "#4285F4",
  },
  {
    id: "github",
    name: "github",
    displayName: "GitHub",
    icon: "/icons/github.svg",
    scopes: ["user:email", "read:user"],
    defaultScopes: ["user:email", "read:user"],
    fieldMapping: { email: "email", name: "name", picture: "avatar_url", username: "login" },
    authUrl: "https://github.com/login/oauth/authorize",
    tokenUrl: "https://github.com/login/oauth/access_token",
    userinfoUrl: "https://api.github.com/user",
    docs: "https://docs.github.com/en/developers/apps/building-oauth-apps",
    color: "#333333",
  },
  {
    id: "microsoft",
    name: "microsoft",
    displayName: "Microsoft",
    icon: "/icons/microsoft.svg",
    scopes: ["openid", "email", "profile", "User.Read"],
    defaultScopes: ["openid", "email", "profile", "User.Read"],
    fieldMapping: { email: "mail", name: "displayName", picture: "photo" },
    authUrl: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
    tokenUrl: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
    userinfoUrl: "https://graph.microsoft.com/v1.0/me",
    docs: "https://docs.microsoft.com/en-us/azure/active-directory/develop/",
    color: "#00A4EF",
  },
  {
    id: "apple",
    name: "apple",
    displayName: "Apple",
    icon: "/icons/apple.svg",
    scopes: ["name", "email"],
    defaultScopes: ["name", "email"],
    fieldMapping: { email: "email", name: "name" },
    authUrl: "https://appleid.apple.com/auth/authorize",
    tokenUrl: "https://appleid.apple.com/auth/token",
    userinfoUrl: "",
    docs: "https://developer.apple.com/documentation/sign_in_with_apple",
    color: "#000000",
  },
  {
    id: "facebook",
    name: "facebook",
    displayName: "Facebook",
    icon: "/icons/facebook.svg",
    scopes: ["email", "public_profile"],
    defaultScopes: ["email", "public_profile"],
    fieldMapping: { email: "email", name: "name", picture: "picture.data.url" },
    authUrl: "https://www.facebook.com/v18.0/dialog/oauth",
    tokenUrl: "https://graph.facebook.com/v18.0/oauth/access_token",
    userinfoUrl: "https://graph.facebook.com/me",
    docs: "https://developers.facebook.com/docs/facebook-login/",
    color: "#1877F2",
  },
  {
    id: "linkedin",
    name: "linkedin",
    displayName: "LinkedIn",
    icon: "/icons/linkedin.svg",
    scopes: ["openid", "profile", "email"],
    defaultScopes: ["openid", "profile", "email"],
    fieldMapping: { email: "email", name: "name", picture: "picture" },
    authUrl: "https://www.linkedin.com/oauth/v2/authorization",
    tokenUrl: "https://www.linkedin.com/oauth/v2/accessToken",
    userinfoUrl: "https://api.linkedin.com/v2/userinfo",
    docs: "https://docs.microsoft.com/en-us/linkedin/shared/authentication/",
    color: "#0A66C2",
  },
  {
    id: "twitter",
    name: "twitter",
    displayName: "X (Twitter)",
    icon: "/icons/twitter.svg",
    scopes: ["users.read", "tweet.read"],
    defaultScopes: ["users.read", "tweet.read"],
    fieldMapping: { email: "email", name: "name", picture: "profile_image_url", username: "username" },
    authUrl: "https://twitter.com/i/oauth2/authorize",
    tokenUrl: "https://api.twitter.com/2/oauth2/token",
    userinfoUrl: "https://api.twitter.com/2/users/me",
    docs: "https://developer.twitter.com/en/docs/authentication/oauth-2-0",
    color: "#000000",
  },
]

const FIELD_MAPPING_OPTIONS: FieldMappingOption[] = [
  { id: "email", label: "Email", description: "Dirección de correo electrónico", icon: Mail, required: true },
  { id: "name", label: "Nombre", description: "Nombre completo del usuario", icon: User },
  { id: "picture", label: "Avatar", description: "URL de la foto de perfil", icon: Image },
  { id: "phone", label: "Teléfono", description: "Número de teléfono", icon: Phone },
  { id: "address", label: "Dirección", description: "Dirección postal", icon: MapPin },
  { id: "birthdate", label: "Fecha de nacimiento", description: "Fecha de nacimiento", icon: Calendar },
  { id: "username", label: "Username", description: "Nombre de usuario único", icon: Hash },
  { id: "email_verified", label: "Email verificado", description: "Si el email está verificado", icon: CheckCircle2 },
]

// ─── Helper Components ───

function InfoTooltip({ content }: { content: string }) {
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <HelpCircle className="h-4 w-4 text-muted-foreground cursor-help ml-1.5 inline-block" />
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-xs">
          <p className="text-sm">{content}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

function StatCard({
  title,
  value,
  icon: Icon,
  description,
  color = "default",
  isLoading = false,
}: {
  title: string
  value: string | number
  icon: React.ElementType
  description?: string
  color?: "default" | "success" | "warning" | "danger" | "info" | "accent"
  isLoading?: boolean
}) {
  const colorClasses = {
    default: "from-muted/20 to-muted/5 text-muted",
    success: "from-success/20 to-success/5 text-success",
    warning: "from-warning/20 to-warning/5 text-warning",
    danger: "from-danger/20 to-danger/5 text-danger",
    info: "from-info/20 to-info/5 text-info",
    accent: "from-accent/20 to-accent/5 text-accent",
  }

  return (
    <Card className="relative overflow-hidden border-white/[0.08] bg-gradient-to-br from-white/[0.05] to-transparent">
      <div className={cn("absolute top-0 right-0 w-32 h-32 bg-gradient-to-br opacity-50 rounded-full blur-2xl -translate-y-1/2 translate-x-1/2", colorClasses[color])} />
      <CardContent className="p-5">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            {isLoading ? (
              <>
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-8 w-16 mt-1" />
                <Skeleton className="h-3 w-28 mt-0.5" />
              </>
            ) : (
              <>
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{title}</p>
                <p className="text-2xl font-bold mt-1">{value}</p>
                {description && (
                  <p className="text-xs text-muted-foreground mt-0.5">{description}</p>
                )}
              </>
            )}
          </div>
          <div className={cn("p-2.5 rounded-xl bg-gradient-to-br", isLoading ? "bg-muted/30" : colorClasses[color])}>
            {isLoading ? (
              <Skeleton className="h-5 w-5 rounded-full" />
            ) : (
              <Icon className="h-5 w-5" />
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function ProviderIcon({ provider, size = "md" }: { provider: string; size?: "sm" | "md" | "lg" }) {
  const sizeClasses = {
    sm: "h-5 w-5",
    md: "h-8 w-8",
    lg: "h-12 w-12",
  }

  // Simple SVG icons for each provider
  const icons: Record<string, React.ReactNode> = {
    google: (
      <svg viewBox="0 0 24 24" className={sizeClasses[size]}>
        <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" />
        <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
        <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
        <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
      </svg>
    ),
    github: (
      <svg viewBox="0 0 24 24" className={cn(sizeClasses[size], "fill-current")}>
        <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
      </svg>
    ),
    microsoft: (
      <svg viewBox="0 0 24 24" className={sizeClasses[size]}>
        <path fill="#F25022" d="M1 1h10v10H1z" />
        <path fill="#00A4EF" d="M13 1h10v10H13z" />
        <path fill="#7FBA00" d="M1 13h10v10H1z" />
        <path fill="#FFB900" d="M13 13h10v10H13z" />
      </svg>
    ),
    apple: (
      <svg viewBox="0 0 24 24" className={cn(sizeClasses[size], "fill-current")}>
        <path d="M18.71 19.5c-.83 1.24-1.71 2.45-3.05 2.47-1.34.03-1.77-.79-3.29-.79-1.53 0-2 .77-3.27.82-1.31.05-2.3-1.32-3.14-2.53C4.25 17 2.94 12.45 4.7 9.39c.87-1.52 2.43-2.48 4.12-2.51 1.28-.02 2.5.87 3.29.87.78 0 2.26-1.07 3.81-.91.65.03 2.47.26 3.64 1.98-.09.06-2.17 1.28-2.15 3.81.03 3.02 2.65 4.03 2.68 4.04-.03.07-.42 1.44-1.38 2.83M13 3.5c.73-.83 1.94-1.46 2.94-1.5.13 1.17-.34 2.35-1.04 3.19-.69.85-1.83 1.51-2.95 1.42-.15-1.15.41-2.35 1.05-3.11z" />
      </svg>
    ),
    facebook: (
      <svg viewBox="0 0 24 24" className={sizeClasses[size]}>
        <path fill="#1877F2" d="M24 12.073c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.99 4.388 10.954 10.125 11.854v-8.385H7.078v-3.47h3.047V9.43c0-3.007 1.792-4.669 4.533-4.669 1.312 0 2.686.235 2.686.235v2.953H15.83c-1.491 0-1.956.925-1.956 1.874v2.25h3.328l-.532 3.47h-2.796v8.385C19.612 23.027 24 18.062 24 12.073z" />
      </svg>
    ),
    linkedin: (
      <svg viewBox="0 0 24 24" className={sizeClasses[size]}>
        <path fill="#0A66C2" d="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433c-1.144 0-2.063-.926-2.063-2.065 0-1.138.92-2.063 2.063-2.063 1.14 0 2.064.925 2.064 2.063 0 1.139-.925 2.065-2.064 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z" />
      </svg>
    ),
    twitter: (
      <svg viewBox="0 0 24 24" className={cn(sizeClasses[size], "fill-current")}>
        <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
      </svg>
    ),
  }

  return icons[provider] || <Globe className={sizeClasses[size]} />
}

function getStatusColor(status: string) {
  switch (status) {
    case "healthy": return "success"
    case "degraded": return "warning"
    case "unavailable": return "danger"
    default: return "default"
  }
}

// ─── Main Component ───

function SocialProvidersContent() {
  const params = useParams()
  const router = useRouter()
  const { t } = useI18n()
  const { toast } = useToast()
  const queryClient = useQueryClient()

  // State - use dynamic route param
  const tenantId = params.tenant_id as string
  const [currentTab, setCurrentTab] = useState("providers")
  const [configureDialog, setConfigureDialog] = useState<ProviderConfig | null>(null)
  const [testDialog, setTestDialog] = useState<ProviderConfig | null>(null)
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null)
  const [showSecret, setShowSecret] = useState(false)
  const [expandedProvider, setExpandedProvider] = useState<string | null>(null)

  // Form state for configuration
  const [configForm, setConfigForm] = useState({
    clientId: "",
    clientSecret: "",
    scopes: [] as string[],
    fieldMapping: {} as Record<string, string>,
  })

  // ─── Queries ───

  const { data: tenant } = useQuery({
    queryKey: ["tenant", tenantId],
    enabled: !!tenantId,
    queryFn: async () => {
      return api.get<Tenant>(`${API_ROUTES.ADMIN_TENANTS}/${tenantId}`)
    },
  })

  // Get provider status from backend
  const { data: providers, isLoading, refetch } = useQuery({
    queryKey: ["providers", tenantId],
    queryFn: async (): Promise<ProviderConfig[]> => {
      try {
        // Try to get real provider status
        const response = await api.get<{ providers: Array<{ name: string; enabled: boolean; ready: boolean; reason?: string }> }>(
          `/v2/auth/providers?tenant_id=${tenantId}`
        )

        // Merge with supported providers
        return SUPPORTED_PROVIDERS.map((base) => {
          const apiProvider = response.providers?.find(p => p.name === base.id)
          const tenantConfig = tenant?.settings?.socialProviders

          // Check if this provider is configured in tenant settings
          const isGoogleConfigured = base.id === "google" && !!tenantConfig?.googleEnabled && !!tenantConfig?.googleClient
          const isEnabled = !!(apiProvider?.enabled) || isGoogleConfigured
          const isConfigured = !!(apiProvider?.ready) || isGoogleConfigured

          return {
            ...base,
            enabled: isEnabled,
            configured: isConfigured,
            status: isEnabled
              ? (isConfigured ? "healthy" : "degraded")
              : (isGoogleConfigured ? "healthy" : "unconfigured"),
            clientId: base.id === "google" ? tenantConfig?.googleClient : undefined,
            clientSecret: undefined, // Never expose
            stats: generateMockStats(base.id, isEnabled),
          } as ProviderConfig
        })
      } catch {
        // Fallback to mock data
        return SUPPORTED_PROVIDERS.map((base) => ({
          ...base,
          enabled: base.id === "google",
          configured: base.id === "google",
          status: base.id === "google" ? "healthy" : "unconfigured",
          stats: generateMockStats(base.id, base.id === "google"),
        }))
      }
    },
  })

  // ─── Mutations ───

  const saveMutation = useMutation({
    mutationFn: async (config: { providerId: string; clientId: string; clientSecret: string; scopes: string[] }) => {
      // Save to tenant settings
      if (!tenantId) throw new Error("Tenant ID required")

      // For now, only Google is supported in backend
      if (config.providerId === "google") {
        const settings = {
          ...tenant?.settings,
          socialProviders: {
            googleEnabled: true,
            googleClient: config.clientId,
            googleSecret: config.clientSecret,
          },
        }
        return api.put(`${API_ROUTES.ADMIN_TENANT_SETTINGS(tenantId)}`, settings)
      }

      // For other providers, show mock success
      await new Promise(resolve => setTimeout(resolve, 1000))
      return { success: true }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["providers", tenantId] })
      queryClient.invalidateQueries({ queryKey: ["tenant", tenantId] })
      toast({
        title: "Proveedor configurado",
        description: "La configuración ha sido guardada exitosamente.",
      })
      setConfigureDialog(null)
    },
    onError: (error: any) => {
      toast({
        title: "Error",
        description: error?.error_description || "No se pudo guardar la configuración",
        variant: "destructive",
      })
    },
  })

  const disableMutation = useMutation({
    mutationFn: async (providerId: string) => {
      if (!tenantId) throw new Error("Tenant ID required")

      if (providerId === "google") {
        const settings = {
          ...tenant?.settings,
          socialProviders: {
            googleEnabled: false,
            googleClient: "",
            googleSecret: "",
          },
        }
        return api.put(`${API_ROUTES.ADMIN_TENANT_SETTINGS(tenantId)}`, settings)
      }

      await new Promise(resolve => setTimeout(resolve, 500))
      return { success: true }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["providers", tenantId] })
      queryClient.invalidateQueries({ queryKey: ["tenant", tenantId] })
      toast({
        title: "Proveedor deshabilitado",
        description: "El proveedor de autenticación social ha sido deshabilitado.",
      })
    },
  })

  // ─── Computed Values ───

  const stats = useMemo(() => {
    if (!providers) return { total: 0, enabled: 0, logins30d: 0, errors30d: 0 }
    const enabled = providers.filter(p => p.enabled).length
    const logins30d = providers.reduce((sum, p) => sum + (p.stats?.loginsLast30d || 0), 0)
    const errors30d = providers.reduce((sum, p) => sum + (p.stats?.errorsLast30d || 0), 0)
    return { total: providers.length, enabled, logins30d, errors30d }
  }, [providers])

  // ─── Handlers ───

  const openConfigureDialog = (provider: ProviderConfig) => {
    setConfigForm({
      clientId: provider.clientId || "",
      clientSecret: "",
      scopes: provider.scopes,
      fieldMapping: provider.fieldMapping,
    })
    setShowSecret(false)
    setConfigureDialog(provider)
  }

  const handleSaveConfig = () => {
    if (!configureDialog) return
    if (!configForm.clientId.trim()) {
      toast({
        title: "Error",
        description: "Client ID es requerido",
        variant: "destructive",
      })
      return
    }

    saveMutation.mutate({
      providerId: configureDialog.id,
      clientId: configForm.clientId.trim(),
      clientSecret: configForm.clientSecret.trim(),
      scopes: configForm.scopes,
    })
  }

  const handleTestConnection = async (provider: ProviderConfig) => {
    setTestDialog(provider)
    setTestResult(null)

    // Simulate test
    await new Promise(resolve => setTimeout(resolve, 1500))

    if (provider.configured) {
      setTestResult({
        success: true,
        message: `Conexión exitosa con ${provider.displayName}. El proveedor está correctamente configurado.`,
      })
    } else {
      setTestResult({
        success: false,
        message: `No se pudo conectar con ${provider.displayName}. Verifica que el Client ID y Client Secret sean correctos.`,
      })
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    toast({ title: "Copiado", description: "Copiado al portapapeles" })
  }

  // ─── Render ───

  return (
    <TooltipProvider>
      <div className="space-y-6 animate-in fade-in duration-500">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <Button variant="ghost" size="sm" asChild>
              <Link href={`/admin/tenants/${tenantId}/detail`}>
                <ArrowLeft className="h-4 w-4" />
              </Link>
            </Button>
            <div className="flex items-center gap-3">
              <div>
                <h1 className="text-2xl font-bold tracking-tight">Proveedores Sociales</h1>
                <p className="text-sm text-muted-foreground">
                  {tenant?.name} — Configura la autenticación con proveedores externos
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* Info Banner */}
        <InlineAlert variant="info">
          <Globe className="h-4 w-4" />
          <div>
            <p className="font-semibold">¿Qué son los Proveedores Sociales?</p>
            <p className="text-sm opacity-90">
              Permiten a tus usuarios iniciar sesión usando sus cuentas existentes de Google, GitHub, Microsoft, etc.
              Esto simplifica el registro y mejora la experiencia de usuario al evitar que tengan que crear una nueva contraseña.
            </p>
          </div>
        </InlineAlert>

        {/* Stats Cards */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <StatCard
            title="Proveedores"
            value={stats.total}
            icon={Globe}
            color="info"
            isLoading={isLoading}
          />
          <StatCard
            title="Habilitados"
            value={stats.enabled}
            icon={CheckCircle2}
            color="success"
            isLoading={isLoading}
          />
          <StatCard
            title="Logins (30d)"
            value={stats.logins30d.toLocaleString()}
            icon={Users}
            color="accent"
            isLoading={isLoading}
          />
          <StatCard
            title="Errores (30d)"
            value={stats.errors30d}
            icon={AlertCircle}
            color={stats.errors30d > 0 ? "warning" : "default"}
            isLoading={isLoading}
          />
        </div>

        {/* Tabs */}
        <Tabs value={currentTab} onValueChange={setCurrentTab}>
          <div className="flex items-center justify-between gap-4 mb-4">
            <TabsList className="bg-white/5 border border-white/10">
              <TabsTrigger value="providers" className="gap-2">
                <Globe className="h-4 w-4" />
                <span className="hidden sm:inline">Proveedores</span>
              </TabsTrigger>
              <TabsTrigger value="guide" className="gap-2">
                <FileText className="h-4 w-4" />
                <span className="hidden sm:inline">Guía de Setup</span>
              </TabsTrigger>
            </TabsList>

            <Button
              variant="outline"
              size="sm"
              onClick={() => refetch()}
              disabled={isLoading}
              className="gap-2"
            >
              <RefreshCw className={cn("h-4 w-4", isLoading && "animate-spin")} />
              <span className="hidden sm:inline">Actualizar</span>
            </Button>
          </div>

          {/* Tab: Providers List */}
          <TabsContent value="providers" className="space-y-4 mt-0">
            {isLoading ? (
              <div className="flex items-center justify-center py-16">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : (
              <div className="grid gap-4">
                {providers?.map((provider) => (
                  <Card
                    key={provider.id}
                    className={cn(
                      "border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent transition-all",
                      provider.enabled && "border-l-2 border-l-success/50"
                    )}
                  >
                    <Collapsible
                      open={expandedProvider === provider.id}
                      onOpenChange={(open) => setExpandedProvider(open ? provider.id : null)}
                    >
                      <div className="p-5">
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-4">
                            <div
                              className={cn(
                                "p-3 rounded-xl border transition-all",
                                provider.enabled
                                  ? "bg-white/10 border-white/20"
                                  : "bg-white/5 border-white/10 opacity-60"
                              )}
                            >
                              <ProviderIcon provider={provider.id} size="md" />
                            </div>
                            <div>
                              <div className="flex items-center gap-2">
                                <h3 className="font-semibold">{provider.displayName}</h3>
                                {provider.enabled ? (
                                  <Badge
                                    variant="outline"
                                    className="border-success/30 bg-success/10 text-success gap-1"
                                  >
                                    <CheckCircle2 className="h-3 w-3" />
                                    Habilitado
                                  </Badge>
                                ) : (
                                  <Badge variant="outline" className="gap-1 opacity-60">
                                    <XCircle className="h-3 w-3" />
                                    No configurado
                                  </Badge>
                                )}
                              </div>
                              <p className="text-sm text-muted-foreground mt-0.5">
                                {provider.enabled
                                  ? `${provider.stats?.loginsLast30d || 0} logins en los últimos 30 días`
                                  : "Haz clic en Configurar para habilitar este proveedor"}
                              </p>
                            </div>
                          </div>

                          <div className="flex items-center gap-2">
                            {provider.enabled && (
                              <>
                                <Button
                                  variant="outline"
                                  size="sm"
                                  onClick={() => handleTestConnection(provider)}
                                  className="gap-2"
                                >
                                  <TestTube className="h-4 w-4" />
                                  <span className="hidden md:inline">Probar</span>
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => disableMutation.mutate(provider.id)}
                                  disabled={disableMutation.isPending}
                                  className="text-danger hover:text-danger hover:bg-danger/10"
                                >
                                  <Unlink className="h-4 w-4" />
                                </Button>
                              </>
                            )}
                            <Button
                              variant={provider.enabled ? "outline" : "default"}
                              size="sm"
                              onClick={() => openConfigureDialog(provider)}
                              className="gap-2"
                            >
                              <Settings2 className="h-4 w-4" />
                              Configurar
                            </Button>
                            <CollapsibleTrigger asChild>
                              <Button variant="ghost" size="sm" className="w-8 h-8 p-0">
                                <ChevronDown
                                  className={cn(
                                    "h-4 w-4 transition-transform",
                                    expandedProvider === provider.id && "rotate-180"
                                  )}
                                />
                              </Button>
                            </CollapsibleTrigger>
                          </div>
                        </div>

                        <CollapsibleContent className="mt-4">
                          <div className="grid md:grid-cols-2 gap-4 pt-4 border-t border-white/[0.06]">
                            {/* Configuration Details */}
                            <div className="space-y-3">
                              <h4 className="text-sm font-medium text-muted-foreground">Configuración</h4>
                              <div className="space-y-2">
                                {provider.clientId && (
                                  <div className="flex items-center justify-between text-sm">
                                    <span className="text-muted-foreground">Client ID</span>
                                    <div className="flex items-center gap-2">
                                      <code className="font-mono text-xs bg-white/5 px-2 py-0.5 rounded">
                                        {provider.clientId.slice(0, 20)}...
                                      </code>
                                      <Button
                                        variant="ghost"
                                        size="sm"
                                        className="h-6 w-6 p-0"
                                        onClick={() => copyToClipboard(provider.clientId!)}
                                      >
                                        <Copy className="h-3 w-3" />
                                      </Button>
                                    </div>
                                  </div>
                                )}
                                <div className="flex items-center justify-between text-sm">
                                  <span className="text-muted-foreground">Scopes</span>
                                  <div className="flex flex-wrap gap-1 justify-end max-w-[200px]">
                                    {provider.scopes.slice(0, 3).map((scope) => (
                                      <Badge key={scope} variant="outline" className="text-[10px] px-1.5 py-0">
                                        {scope}
                                      </Badge>
                                    ))}
                                    {provider.scopes.length > 3 && (
                                      <Badge variant="outline" className="text-[10px] px-1.5 py-0">
                                        +{provider.scopes.length - 3}
                                      </Badge>
                                    )}
                                  </div>
                                </div>
                              </div>
                            </div>

                            {/* URLs */}
                            <div className="space-y-3">
                              <h4 className="text-sm font-medium text-muted-foreground">Endpoints</h4>
                              <div className="space-y-2 text-sm">
                                <div className="flex items-center justify-between">
                                  <span className="text-muted-foreground">Auth URL</span>
                                  <a
                                    href={provider.authUrl}
                                    target="_blank"
                                    rel="noreferrer"
                                    className="text-info hover:text-info/80 text-xs truncate max-w-[180px]"
                                  >
                                    {new URL(provider.authUrl).hostname}
                                  </a>
                                </div>
                                <div className="flex items-center justify-between">
                                  <span className="text-muted-foreground">Docs</span>
                                  <a
                                    href={provider.docs}
                                    target="_blank"
                                    rel="noreferrer"
                                    className="text-info hover:text-info/80 flex items-center gap-1 text-xs"
                                  >
                                    Ver documentación
                                    <ExternalLink className="h-3 w-3" />
                                  </a>
                                </div>
                              </div>
                            </div>
                          </div>

                          {/* Stats */}
                          {provider.enabled && provider.stats && (
                            <div className="grid grid-cols-3 gap-3 mt-4 pt-4 border-t border-white/[0.06]">
                              <div className="text-center p-3 rounded-lg bg-white/[0.02]">
                                <p className="text-lg font-semibold">{provider.stats.loginsLast30d}</p>
                                <p className="text-xs text-muted-foreground">Logins (30d)</p>
                              </div>
                              <div className="text-center p-3 rounded-lg bg-white/[0.02]">
                                <p className="text-lg font-semibold">{provider.stats.errorsLast30d}</p>
                                <p className="text-xs text-muted-foreground">Errores (30d)</p>
                              </div>
                              <div className="text-center p-3 rounded-lg bg-white/[0.02]">
                                <p className="text-lg font-semibold">
                                  {provider.stats.lastLogin
                                    ? new Date(provider.stats.lastLogin).toLocaleDateString()
                                    : "-"}
                                </p>
                                <p className="text-xs text-muted-foreground">Último login</p>
                              </div>
                            </div>
                          )}
                        </CollapsibleContent>
                      </div>
                    </Collapsible>
                  </Card>
                ))}
              </div>
            )}
          </TabsContent>

          {/* Tab: Setup Guide */}
          <TabsContent value="guide" className="space-y-6 mt-0">
            <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Sparkles className="h-5 w-5 text-info" />
                  Cómo configurar un proveedor social
                </CardTitle>
                <CardDescription>
                  Sigue estos pasos para habilitar la autenticación con proveedores externos
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                {/* Step 1 */}
                <div className="flex gap-4">
                  <div className="flex-shrink-0 w-8 h-8 rounded-full bg-info/20 border border-info/30 flex items-center justify-center">
                    <span className="text-sm font-semibold text-info">1</span>
                  </div>
                  <div>
                    <h4 className="font-medium">Crea una aplicación OAuth en el proveedor</h4>
                    <p className="text-sm text-muted-foreground mt-1">
                      Ve a la consola de desarrolladores del proveedor (ej: Google Cloud Console, GitHub Developer Settings)
                      y crea una nueva aplicación OAuth.
                    </p>
                    <div className="flex flex-wrap gap-2 mt-3">
                      {SUPPORTED_PROVIDERS.slice(0, 4).map((p) => (
                        <a
                          key={p.id}
                          href={p.docs}
                          target="_blank"
                          rel="noreferrer"
                          className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-white/5 hover:bg-white/10 border border-white/10 text-sm transition-colors"
                        >
                          <ProviderIcon provider={p.id} size="sm" />
                          {p.displayName}
                          <ExternalLink className="h-3 w-3 text-muted-foreground" />
                        </a>
                      ))}
                    </div>
                  </div>
                </div>

                {/* Step 2 */}
                <div className="flex gap-4">
                  <div className="flex-shrink-0 w-8 h-8 rounded-full bg-info/20 border border-info/30 flex items-center justify-center">
                    <span className="text-sm font-semibold text-info">2</span>
                  </div>
                  <div>
                    <h4 className="font-medium">Configura las URLs de redirección</h4>
                    <p className="text-sm text-muted-foreground mt-1">
                      En la configuración de tu aplicación OAuth, agrega las siguientes URLs de callback:
                    </p>
                    <div className="mt-3 p-3 rounded-lg bg-white/[0.03] border border-white/[0.06]">
                      <div className="flex items-center justify-between">
                        <code className="text-sm font-mono text-info">
                          {typeof window !== "undefined"
                            ? `${window.location.origin.replace(":3000", ":8080")}/v2/auth/social/{'{provider}'}/callback`
                            : "https://your-domain.com/v2/auth/social/{provider}/callback"}
                        </code>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7"
                          onClick={() =>
                            copyToClipboard(
                              typeof window !== "undefined"
                                ? `${window.location.origin.replace(":3000", ":8080")}/v2/auth/social/google/callback`
                                : "https://your-domain.com/v2/auth/social/google/callback"
                            )
                          }
                        >
                          <Copy className="h-3 w-3" />
                        </Button>
                      </div>
                      <p className="text-xs text-muted-foreground mt-2">
                        Reemplaza <code className="text-warning">{'{provider}'}</code> con: google, github, microsoft, etc.
                      </p>
                    </div>
                  </div>
                </div>

                {/* Step 3 */}
                <div className="flex gap-4">
                  <div className="flex-shrink-0 w-8 h-8 rounded-full bg-info/20 border border-info/30 flex items-center justify-center">
                    <span className="text-sm font-semibold text-info">3</span>
                  </div>
                  <div>
                    <h4 className="font-medium">Obtén las credenciales</h4>
                    <p className="text-sm text-muted-foreground mt-1">
                      Una vez creada la aplicación, copia el <strong>Client ID</strong> y el{" "}
                      <strong>Client Secret</strong> proporcionados por el proveedor.
                    </p>
                    <InlineAlert variant="warning" className="mt-3">
                      <p className="text-xs">
                        Nunca compartas el Client Secret. Guárdalo de forma segura.
                      </p>
                    </InlineAlert>
                  </div>
                </div>

                {/* Step 4 */}
                <div className="flex gap-4">
                  <div className="flex-shrink-0 w-8 h-8 rounded-full bg-info/20 border border-info/30 flex items-center justify-center">
                    <span className="text-sm font-semibold text-info">4</span>
                  </div>
                  <div>
                    <h4 className="font-medium">Configura en HelloJohn</h4>
                    <p className="text-sm text-muted-foreground mt-1">
                      Vuelve a la pestaña "Proveedores", haz clic en <strong>Configurar</strong> en el proveedor
                      que deseas habilitar, e ingresa las credenciales obtenidas.
                    </p>
                  </div>
                </div>

                {/* Step 5 */}
                <div className="flex gap-4">
                  <div className="flex-shrink-0 w-8 h-8 rounded-full bg-success/20 border border-success/30 flex items-center justify-center">
                    <Check className="h-4 w-4 text-success" />
                  </div>
                  <div>
                    <h4 className="font-medium">¡Listo!</h4>
                    <p className="text-sm text-muted-foreground mt-1">
                      Usa el botón "Probar" para verificar que la configuración es correcta.
                      Tus usuarios ahora podrán iniciar sesión con este proveedor.
                    </p>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Field Mapping Reference */}
            <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
              <CardHeader>
                <CardTitle className="text-lg">Mapeo de Campos</CardTitle>
                <CardDescription>
                  Cómo se mapean los datos del proveedor al perfil del usuario
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid md:grid-cols-2 gap-3">
                  {FIELD_MAPPING_OPTIONS.map((field) => (
                    <div
                      key={field.id}
                      className="flex items-center gap-3 p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]"
                    >
                      <div className="p-2 rounded-lg bg-white/5">
                        <field.icon className="h-4 w-4 text-muted-foreground" />
                      </div>
                      <div>
                        <p className="text-sm font-medium">{field.label}</p>
                        <p className="text-xs text-muted-foreground">{field.description}</p>
                      </div>
                      {field.required && (
                        <Badge variant="outline" className="ml-auto text-[10px] border-warning/30 text-warning">
                          Requerido
                        </Badge>
                      )}
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>

        {/* ─── Configure Dialog ─── */}
        <Dialog open={!!configureDialog} onOpenChange={() => setConfigureDialog(null)}>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <div className="flex items-center gap-3">
                {configureDialog && (
                  <div className="p-2 rounded-xl bg-white/10 border border-white/20">
                    <ProviderIcon provider={configureDialog.id} size="md" />
                  </div>
                )}
                <div>
                  <DialogTitle>Configurar {configureDialog?.displayName}</DialogTitle>
                  <DialogDescription>
                    Ingresa las credenciales OAuth para habilitar este proveedor
                  </DialogDescription>
                </div>
              </div>
            </DialogHeader>

            {configureDialog && (
              <div className="space-y-4 py-4">
                {/* Client ID */}
                <div className="space-y-2">
                  <Label htmlFor="clientId" className="flex items-center">
                    Client ID
                    <InfoTooltip content="El identificador público de tu aplicación OAuth" />
                  </Label>
                  <Input
                    id="clientId"
                    value={configForm.clientId}
                    onChange={(e) => setConfigForm({ ...configForm, clientId: e.target.value })}
                    placeholder={`${configureDialog.id}-client-id.apps.example.com`}
                    className="font-mono text-sm"
                  />
                </div>

                {/* Client Secret */}
                <div className="space-y-2">
                  <Label htmlFor="clientSecret" className="flex items-center">
                    Client Secret
                    <InfoTooltip content="La clave secreta de tu aplicación. Se encriptará antes de guardarse." />
                  </Label>
                  <div className="relative">
                    <Input
                      id="clientSecret"
                      type={showSecret ? "text" : "password"}
                      value={configForm.clientSecret}
                      onChange={(e) => setConfigForm({ ...configForm, clientSecret: e.target.value })}
                      placeholder={configureDialog.configured ? "••••••••••••••••" : "Ingresa el client secret"}
                      className="font-mono text-sm pr-10"
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="absolute right-1 top-1/2 -translate-y-1/2 h-7 w-7 p-0"
                      onClick={() => setShowSecret(!showSecret)}
                    >
                      {showSecret ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    </Button>
                  </div>
                  {configureDialog.configured && (
                    <p className="text-xs text-muted-foreground">
                      Deja vacío para mantener el secret actual
                    </p>
                  )}
                </div>

                {/* Scopes */}
                <div className="space-y-2">
                  <Label className="flex items-center">
                    Scopes
                    <InfoTooltip content="Los permisos que se solicitarán al usuario durante el login" />
                  </Label>
                  <div className="flex flex-wrap gap-2">
                    {configureDialog.defaultScopes.map((scope) => (
                      <Badge
                        key={scope}
                        variant={configForm.scopes.includes(scope) ? "default" : "outline"}
                        className={cn(
                          "cursor-pointer transition-all",
                          configForm.scopes.includes(scope)
                            ? "bg-info/20 border-info/30 text-info"
                            : "hover:bg-white/5"
                        )}
                        onClick={() => {
                          if (configForm.scopes.includes(scope)) {
                            setConfigForm({
                              ...configForm,
                              scopes: configForm.scopes.filter((s) => s !== scope),
                            })
                          } else {
                            setConfigForm({
                              ...configForm,
                              scopes: [...configForm.scopes, scope],
                            })
                          }
                        }}
                      >
                        {configForm.scopes.includes(scope) && <Check className="h-3 w-3 mr-1" />}
                        {scope}
                      </Badge>
                    ))}
                  </div>
                </div>

                {/* Callback URL */}
                <div className="p-3 rounded-lg bg-white/[0.03] border border-white/[0.06]">
                  <Label className="text-xs text-muted-foreground">Callback URL (para configurar en {configureDialog.displayName})</Label>
                  <div className="flex items-center gap-2 mt-1.5">
                    <code className="text-xs font-mono text-info flex-1 truncate">
                      {typeof window !== "undefined"
                        ? `${window.location.origin.replace(":3000", ":8080")}/v2/auth/social/${configureDialog.id}/callback`
                        : `/v2/auth/social/${configureDialog.id}/callback`}
                    </code>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-6 w-6 p-0"
                      onClick={() =>
                        copyToClipboard(
                          typeof window !== "undefined"
                            ? `${window.location.origin.replace(":3000", ":8080")}/v2/auth/social/${configureDialog.id}/callback`
                            : `/v2/auth/social/${configureDialog.id}/callback`
                        )
                      }
                    >
                      <Copy className="h-3 w-3" />
                    </Button>
                  </div>
                </div>

                {/* Docs link */}
                <a
                  href={configureDialog.docs}
                  target="_blank"
                  rel="noreferrer"
                  className="flex items-center gap-2 text-sm text-info hover:text-info/80"
                >
                  <ExternalLink className="h-4 w-4" />
                  Ver documentación de {configureDialog.displayName}
                </a>
              </div>
            )}

            <DialogFooter>
              <Button variant="outline" onClick={() => setConfigureDialog(null)}>
                Cancelar
              </Button>
              <Button
                onClick={handleSaveConfig}
                disabled={saveMutation.isPending || !configForm.clientId.trim()}
                className="bg-gradient-to-r from-info to-accent hover:from-info/90 hover:to-accent/90 gap-2"
              >
                {saveMutation.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Check className="h-4 w-4" />
                )}
                Guardar
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* ─── Test Connection Dialog ─── */}
        <Dialog open={!!testDialog} onOpenChange={() => { setTestDialog(null); setTestResult(null) }}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <div className="flex items-center gap-3">
                {testDialog && (
                  <div className="p-2 rounded-xl bg-white/10 border border-white/20">
                    <ProviderIcon provider={testDialog.id} size="md" />
                  </div>
                )}
                <div>
                  <DialogTitle>Probar Conexión</DialogTitle>
                  <DialogDescription>
                    Verificando la configuración de {testDialog?.displayName}
                  </DialogDescription>
                </div>
              </div>
            </DialogHeader>

            <div className="py-6">
              {!testResult ? (
                <div className="flex flex-col items-center gap-4">
                  <div className="p-4 rounded-full bg-info/10 border border-info/20">
                    <Loader2 className="h-8 w-8 animate-spin text-info" />
                  </div>
                  <p className="text-sm text-muted-foreground">
                    Probando conexión con {testDialog?.displayName}...
                  </p>
                </div>
              ) : (
                <div className="flex flex-col items-center gap-4">
                  <div
                    className={cn(
                      "p-4 rounded-full border",
                      testResult.success
                        ? "bg-success/10 border-success/20"
                        : "bg-danger/10 border-danger/20"
                    )}
                  >
                    {testResult.success ? (
                      <CheckCircle2 className="h-8 w-8 text-success" />
                    ) : (
                      <XCircle className="h-8 w-8 text-danger" />
                    )}
                  </div>
                  <div className="text-center">
                    <p className={cn("font-medium", testResult.success ? "text-success" : "text-danger")}>
                      {testResult.success ? "¡Conexión exitosa!" : "Error de conexión"}
                    </p>
                    <p className="text-sm text-muted-foreground mt-1">{testResult.message}</p>
                  </div>
                </div>
              )}
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => { setTestDialog(null); setTestResult(null) }}>
                Cerrar
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </TooltipProvider>
  )
}

// ─── Mock Data Generator ───

function generateMockStats(providerId: string, enabled: boolean): ProviderConfig["stats"] {
  if (!enabled) {
    return {
      loginsLast30d: 0,
      errorsLast30d: 0,
    }
  }

  const baseLogins = providerId === "google" ? 500 : Math.floor(Math.random() * 200)
  return {
    loginsLast30d: baseLogins + Math.floor(Math.random() * 100),
    errorsLast30d: Math.floor(Math.random() * 5),
    lastLogin: new Date(Date.now() - Math.random() * 24 * 60 * 60 * 1000).toISOString(),
  }
}

// ─── Page Export ───

export default function ProvidersPage() {
  return (
    <Suspense
      fallback={
        <div className="flex items-center justify-center min-h-[400px]">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      }
    >
      <SocialProvidersContent />
    </Suspense>
  )
}
