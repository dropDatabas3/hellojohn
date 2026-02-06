"use client"

import { useState, useMemo, useEffect } from "react"
import { useQuery } from "@tanstack/react-query"
import {
    Play, Copy, Check, Code, ArrowRight, ArrowLeft, Sparkles,
    HelpCircle, ExternalLink, ChevronRight, Key, Shield, Lock,
    User, Globe, Terminal, FileCode, CheckCircle2, AlertCircle,
    RefreshCw, Eye, EyeOff, Zap, Info, Clock, Hash
} from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { apiFetch } from "@/lib/routes"
import { Button } from "@/components/ds"
import { Input } from "@/components/ds"
import { Card } from "@/components/ds"
import { Label } from "@/components/ds"
import { useToast } from "@/hooks/use-toast"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ds"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ds"
import { Badge } from "@/components/ds"
import {
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
} from "@/components/ds"
import type { Tenant, Client } from "@/lib/types"
import { Textarea } from "@/components/ds"
import { cn } from "@/components/ds/utils/cn"

// ─── Helper Functions ───

function base64UrlDecode(str: string): string {
    // Replace URL-safe chars and add padding
    let base64 = str.replace(/-/g, '+').replace(/_/g, '/')
    while (base64.length % 4) base64 += '='
    try {
        return decodeURIComponent(
            atob(base64)
                .split('')
                .map(c => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
                .join('')
        )
    } catch {
        return atob(base64)
    }
}

function decodeJwt(token: string): { header: any; payload: any; signature: string } | null {
    try {
        const parts = token.split('.')
        if (parts.length !== 3) return null

        return {
            header: JSON.parse(base64UrlDecode(parts[0])),
            payload: JSON.parse(base64UrlDecode(parts[1])),
            signature: parts[2]
        }
    } catch {
        return null
    }
}

function formatTimestamp(ts: number): string {
    return new Date(ts * 1000).toLocaleString()
}

function isTokenExpired(exp: number): boolean {
    return Date.now() > exp * 1000
}

function getTimeRemaining(exp: number): string {
    const remaining = exp * 1000 - Date.now()
    if (remaining <= 0) return "Expirado"
    const minutes = Math.floor(remaining / 60000)
    const hours = Math.floor(minutes / 60)
    if (hours > 0) return `${hours}h ${minutes % 60}m restantes`
    return `${minutes}m restantes`
}

// ─── Info Tooltip Component ───

function InfoTooltip({ content }: { content: string }) {
    return (
        <TooltipProvider delayDuration={200}>
            <Tooltip>
                <TooltipTrigger asChild>
                    <button className="ml-1.5 inline-flex">
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

// ─── Step Indicator ───

function StepIndicator({ step, currentStep, label }: { step: number; currentStep: number; label: string }) {
    const isActive = currentStep === step
    const isComplete = currentStep > step

    return (
        <div className="flex items-center gap-3">
            <div className={cn(
                "h-8 w-8 rounded-full flex items-center justify-center text-sm font-medium transition-all",
                "shadow-clay-button",
                isComplete && "bg-success text-success-foreground",
                isActive && "bg-accent text-accent-foreground ring-4 ring-accent/20 shadow-clay-float",
                !isActive && !isComplete && "bg-muted text-muted-foreground"
            )}>
                {isComplete ? <Check className="h-4 w-4" /> : step}
            </div>
            <span className={cn(
                "text-sm font-medium transition-colors",
                isActive && "text-foreground",
                !isActive && "text-muted-foreground"
            )}>
                {label}
            </span>
        </div>
    )
}

// ─── JWT Decoder Component ───

function JwtDecoder({ token, title }: { token: string; title: string }) {
    const [showRaw, setShowRaw] = useState(false)
    const decoded = useMemo(() => decodeJwt(token), [token])

    if (!decoded) {
        return (
            <div className="p-4 rounded-lg border border-amber-500/20 bg-amber-500/5">
                <p className="text-sm text-amber-600 dark:text-amber-400">Token inválido o no es un JWT</p>
            </div>
        )
    }

    const { header, payload, signature } = decoded
    const isExpired = payload.exp && isTokenExpired(payload.exp)

    return (
        <div className="space-y-4">
            <div className="flex items-center justify-between">
                <h4 className="text-sm font-medium flex items-center gap-2">
                    <Key className="h-4 w-4 text-accent" />
                    {title}
                    {isExpired ? (
                        <Badge variant="outline" className="text-[10px] bg-destructive/10 text-destructive border-destructive/20">
                            EXPIRADO
                        </Badge>
                    ) : payload.exp ? (
                        <Badge variant="outline" className="text-[10px] bg-success/10 text-success border-success/20">
                            {getTimeRemaining(payload.exp)}
                        </Badge>
                    ) : null}
                </h4>
                <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setShowRaw(!showRaw)}
                    className="text-xs hover:scale-110 active:scale-95 transition-transform"
                >
                    {showRaw ? <Eye className="h-3.5 w-3.5 mr-1" /> : <EyeOff className="h-3.5 w-3.5 mr-1" />}
                    {showRaw ? "Ver Decodificado" : "Ver Raw"}
                </Button>
            </div>

            {showRaw ? (
                <pre className="p-4 rounded-lg bg-zinc-950 text-zinc-100 text-xs overflow-x-auto font-mono">
                    <code>{token}</code>
                </pre>
            ) : (
                <div className="space-y-3">
                    {/* Header */}
                    <div className="rounded-lg border bg-muted/5 overflow-hidden shadow-clay-card">
                        <div className="px-3 py-2 bg-accent/10 border-b border-accent/20">
                            <span className="text-xs font-medium text-accent">HEADER</span>
                        </div>
                        <pre className="p-3 text-xs font-mono overflow-x-auto">
                            <code>{JSON.stringify(header, null, 2)}</code>
                        </pre>
                    </div>

                    {/* Payload */}
                    <div className="rounded-lg border bg-muted/5 overflow-hidden shadow-clay-card">
                        <div className="px-3 py-2 bg-info/10 border-b border-info/20">
                            <span className="text-xs font-medium text-info">PAYLOAD (Claims)</span>
                        </div>
                        <div className="p-3 space-y-2">
                            {/* Standard claims with better formatting */}
                            {payload.sub && (
                                <div className="flex items-center justify-between text-xs">
                                    <span className="text-muted-foreground flex items-center gap-1">
                                        <User className="h-3 w-3" /> sub (Subject)
                                    </span>
                                    <code className="bg-muted px-2 py-0.5 rounded">{payload.sub}</code>
                                </div>
                            )}
                            {payload.iss && (
                                <div className="flex items-center justify-between text-xs">
                                    <span className="text-muted-foreground flex items-center gap-1">
                                        <Globe className="h-3 w-3" /> iss (Issuer)
                                    </span>
                                    <code className="bg-muted px-2 py-0.5 rounded truncate max-w-[200px]">{payload.iss}</code>
                                </div>
                            )}
                            {payload.aud && (
                                <div className="flex items-center justify-between text-xs">
                                    <span className="text-muted-foreground flex items-center gap-1">
                                        <Shield className="h-3 w-3" /> aud (Audience)
                                    </span>
                                    <code className="bg-muted px-2 py-0.5 rounded">{payload.aud}</code>
                                </div>
                            )}
                            {payload.exp && (
                                <div className="flex items-center justify-between text-xs">
                                    <span className="text-muted-foreground flex items-center gap-1">
                                        <Clock className="h-3 w-3" /> exp (Expiration)
                                    </span>
                                    <span className={cn("font-mono", isExpired && "text-destructive")}>
                                        {formatTimestamp(payload.exp)}
                                    </span>
                                </div>
                            )}
                            {payload.iat && (
                                <div className="flex items-center justify-between text-xs">
                                    <span className="text-muted-foreground flex items-center gap-1">
                                        <Clock className="h-3 w-3" /> iat (Issued At)
                                    </span>
                                    <span className="font-mono">{formatTimestamp(payload.iat)}</span>
                                </div>
                            )}

                            {/* Other claims */}
                            {Object.entries(payload).filter(([k]) => !['sub', 'iss', 'aud', 'exp', 'iat', 'nbf'].includes(k)).length > 0 && (
                                <>
                                    <div className="border-t border-dashed pt-2 mt-2">
                                        <span className="text-[10px] text-muted-foreground uppercase tracking-wider">Otros Claims</span>
                                    </div>
                                    <pre className="text-xs font-mono overflow-x-auto bg-muted p-2 rounded">
                                        <code>
                                            {JSON.stringify(
                                                Object.fromEntries(
                                                    Object.entries(payload).filter(([k]) => !['sub', 'iss', 'aud', 'exp', 'iat', 'nbf'].includes(k))
                                                ),
                                                null, 2
                                            )}
                                        </code>
                                    </pre>
                                </>
                            )}
                        </div>
                    </div>

                    {/* Signature */}
                    <div className="rounded-lg border bg-muted/5 overflow-hidden shadow-clay-card">
                        <div className="px-3 py-2 bg-success/10 border-b border-success/20">
                            <span className="text-xs font-medium text-success">SIGNATURE</span>
                        </div>
                        <div className="p-3">
                            <code className="text-xs font-mono text-muted-foreground break-all">{signature.substring(0, 50)}...</code>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}

// ─── Code Samples Component ───

function CodeSamples({
    authUrl,
    tokenEndpoint,
    clientId,
    clientSecret,
    redirectUri,
    code
}: {
    authUrl: string
    tokenEndpoint: string
    clientId: string
    clientSecret?: string
    redirectUri: string
    code?: string
}) {
    const [copiedTab, setCopiedTab] = useState<string | null>(null)

    const copyCode = (code: string, tab: string) => {
        navigator.clipboard.writeText(code)
        setCopiedTab(tab)
        setTimeout(() => setCopiedTab(null), 2000)
    }

    const curlAuth = `# Paso 1: Abre esta URL en el navegador
open "${authUrl}"`

    const curlToken = code ? `# Paso 2: Intercambia el código por tokens
curl -X POST "${tokenEndpoint}" \\
  -H "Content-Type: application/x-www-form-urlencoded" \\
  -d "grant_type=authorization_code" \\
  -d "code=${code}" \\
  -d "redirect_uri=${redirectUri}" \\
  -d "client_id=${clientId}"${clientSecret ? ` \\
  -d "client_secret=${clientSecret}"` : ''}` : `# Paso 2: Intercambia el código por tokens (reemplaza CODE)
curl -X POST "${tokenEndpoint}" \\
  -H "Content-Type: application/x-www-form-urlencoded" \\
  -d "grant_type=authorization_code" \\
  -d "code=YOUR_AUTH_CODE" \\
  -d "redirect_uri=${redirectUri}" \\
  -d "client_id=${clientId}"${clientSecret ? ` \\
  -d "client_secret=${clientSecret}"` : ''}`

    const jsCode = `// Flujo Authorization Code con PKCE (recomendado para SPAs)

// 1. Generar code_verifier y code_challenge
function generateCodeVerifier() {
  const array = new Uint8Array(32);
  crypto.getRandomValues(array);
  return btoa(String.fromCharCode(...array))
    .replace(/\\+/g, '-').replace(/\\//g, '_').replace(/=/g, '');
}

async function generateCodeChallenge(verifier) {
  const encoder = new TextEncoder();
  const data = encoder.encode(verifier);
  const hash = await crypto.subtle.digest('SHA-256', data);
  return btoa(String.fromCharCode(...new Uint8Array(hash)))
    .replace(/\\+/g, '-').replace(/\\//g, '_').replace(/=/g, '');
}

// 2. Redirigir al usuario para autorización
const codeVerifier = generateCodeVerifier();
sessionStorage.setItem('code_verifier', codeVerifier);

const authUrl = new URL("${authUrl}");
authUrl.searchParams.set('code_challenge', await generateCodeChallenge(codeVerifier));
authUrl.searchParams.set('code_challenge_method', 'S256');

window.location.href = authUrl.toString();

// 3. En el callback, intercambiar código por tokens
async function exchangeCode(code) {
  const response = await fetch("${tokenEndpoint}", {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
      grant_type: 'authorization_code',
      code: code,
      redirect_uri: "${redirectUri}",
      client_id: "${clientId}",
      code_verifier: sessionStorage.getItem('code_verifier')
    })
  });
  return response.json();
}`

    return (
        <div className="space-y-4">
            <h4 className="text-sm font-medium flex items-center gap-2">
                <Terminal className="h-4 w-4" />
                Ejemplos de Código
                <InfoTooltip content="Copia estos ejemplos para integrar OAuth2 en tu aplicación" />
            </h4>

            <Tabs defaultValue="curl" className="w-full">
                <TabsList className="bg-zinc-100 dark:bg-zinc-800/50 w-full justify-start">
                    <TabsTrigger value="curl" className="text-xs gap-1">
                        <Terminal className="h-3 w-3" />
                        cURL
                    </TabsTrigger>
                    <TabsTrigger value="javascript" className="text-xs gap-1">
                        <FileCode className="h-3 w-3" />
                        JavaScript
                    </TabsTrigger>
                </TabsList>

                <TabsContent value="curl" className="mt-3">
                    <div className="relative">
                        <pre className="p-4 rounded-lg bg-zinc-950 text-zinc-100 text-xs overflow-x-auto font-mono">
                            <code>{curlAuth}\n\n{curlToken}</code>
                        </pre>
                        <Button
                            variant="ghost"
                            size="sm"
                            className="absolute top-2 right-2 hover:scale-110 active:scale-95 transition-transform"
                            onClick={() => copyCode(`${curlAuth}\n\n${curlToken}`, 'curl')}
                        >
                            {copiedTab === 'curl' ? <Check className="h-4 w-4 text-success" /> : <Copy className="h-4 w-4" />}
                        </Button>
                    </div>
                </TabsContent>

                <TabsContent value="javascript" className="mt-3">
                    <div className="relative">
                        <pre className="p-4 rounded-lg bg-zinc-950 text-zinc-100 text-xs overflow-x-auto font-mono max-h-[400px]">
                            <code>{jsCode}</code>
                        </pre>
                        <Button
                            variant="ghost"
                            size="sm"
                            className="absolute top-2 right-2 hover:scale-110 active:scale-95 transition-transform"
                            onClick={() => copyCode(jsCode, 'js')}
                        >
                            {copiedTab === 'js' ? <Check className="h-4 w-4 text-success" /> : <Copy className="h-4 w-4" />}
                        </Button>
                    </div>
                </TabsContent>
            </Tabs>
        </div>
    )
}

// ─── Main Page Component ───

export default function PlaygroundPage() {
    const { t } = useI18n()
    const { toast } = useToast()

    // Wizard state
    const [currentStep, setCurrentStep] = useState(1)

    // Step 1: Application selection
    const [selectedTenant, setSelectedTenant] = useState<string>("")
    const [selectedClient, setSelectedClient] = useState<string>("")

    // Step 2: Request configuration
    const [responseType, setResponseType] = useState<string>("code")
    const [selectedScopes, setSelectedScopes] = useState<string[]>(["openid", "profile", "email"])
    const [customScope, setCustomScope] = useState("")
    const [redirectUri, setRedirectUri] = useState<string>("")
    const [state, setState] = useState<string>("")
    const [nonce, setNonce] = useState<string>("")
    const [usePkce, setUsePkce] = useState(true)
    const [codeVerifier, setCodeVerifier] = useState("")
    const [codeChallenge, setCodeChallenge] = useState("")

    // Step 3 & 4: Results
    const [authUrl, setAuthUrl] = useState<string>("")
    const [tokenCode, setTokenCode] = useState<string>("")
    const [tokenResponse, setTokenResponse] = useState<any>(null)
    const [isExchanging, setIsExchanging] = useState(false)

    // UserInfo & Refresh features
    const [userInfoResponse, setUserInfoResponse] = useState<any>(null)
    const [isLoadingUserInfo, setIsLoadingUserInfo] = useState(false)
    const [isRefreshing, setIsRefreshing] = useState(false)
    const [refreshCount, setRefreshCount] = useState(0)

    // Clipboard state
    const [copiedUrl, setCopiedUrl] = useState(false)

    // Data queries
    const { data: tenants } = useQuery({
        queryKey: ["tenants"],
        queryFn: () => api.get<Tenant[]>("/v2/admin/tenants"),
    })

    const { data: clients } = useQuery({
        queryKey: ["clients", selectedTenant],
        queryFn: () => api.get<Client[]>(`/v2/admin/tenants/${selectedTenant}/clients`),
        enabled: !!selectedTenant,
    })

    // Selected client data
    const selectedClientData = useMemo(() =>
        clients?.find((c: any) => (c.client_id || c.clientId || c.id) === selectedClient),
        [clients, selectedClient]
    )

    // Selected tenant data
    const selectedTenantData = useMemo(() =>
        tenants?.find((t) => t.id === selectedTenant),
        [tenants, selectedTenant]
    )

    // Auto-populate redirect URI when client changes
    useEffect(() => {
        const uris = selectedClientData?.redirectUris || selectedClientData?.redirect_uris
        if (uris?.length) {
            setRedirectUri(uris[0])
        }
    }, [selectedClientData])

    // Generate random state/nonce
    const generateRandom = () => Math.random().toString(36).substring(2, 15)

    // Generate PKCE pair
    const generatePkce = async () => {
        const array = new Uint8Array(32)
        crypto.getRandomValues(array)
        const verifier = btoa(String.fromCharCode(...array))
            .replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')

        const encoder = new TextEncoder()
        const data = encoder.encode(verifier)
        const hash = await crypto.subtle.digest('SHA-256', data)
        const challenge = btoa(String.fromCharCode(...new Uint8Array(hash)))
            .replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')

        setCodeVerifier(verifier)
        setCodeChallenge(challenge)
    }

    // Token endpoint URL
    const tokenEndpoint = selectedTenantData?.slug
        ? `${window.location.origin}/${selectedTenantData.slug}/oauth2/token`
        : ''

    // Generate authorization URL
    const generateAuthUrl = () => {
        if (!selectedTenant || !selectedClient || !redirectUri) {
            toast({
                title: "Campos requeridos",
                description: "Selecciona un tenant, client y redirect URI",
                variant: "destructive",
            })
            return
        }

        const generatedState = state || generateRandom()
        const generatedNonce = nonce || generateRandom()

        setState(generatedState)
        setNonce(generatedNonce)

        const params = new URLSearchParams({
            client_id: selectedClientData?.clientId || "",
            response_type: responseType,
            redirect_uri: redirectUri,
            scope: selectedScopes.join(" "),
            state: generatedState,
            nonce: generatedNonce,
        })

        if (usePkce && codeChallenge) {
            params.set('code_challenge', codeChallenge)
            params.set('code_challenge_method', 'S256')
        }

        const tenantSlug = selectedTenantData?.slug || selectedTenant
        const url = `${window.location.origin}/${tenantSlug}/oauth2/authorize?${params.toString()}`
        setAuthUrl(url)
        setCurrentStep(3)
    }

    // Exchange authorization code for tokens
    const exchangeToken = async () => {
        if (!tokenCode || !selectedClient || !redirectUri) {
            toast({
                title: "Código requerido",
                description: "Pega el código de autorización de la URL de callback",
                variant: "destructive",
            })
            return
        }

        setIsExchanging(true)

        try {
            const bodyParams: Record<string, string> = {
                grant_type: "authorization_code",
                code: tokenCode,
                redirect_uri: redirectUri,
                client_id: selectedClientData?.clientId || "",
            }

            if (selectedClientData?.type === "confidential" && selectedClientData?.secret) {
                bodyParams.client_secret = selectedClientData.secret
            }

            if (usePkce && codeVerifier) {
                bodyParams.code_verifier = codeVerifier
            }

            const response = await apiFetch(tokenEndpoint, {
                method: "POST",
                headers: {
                    "Content-Type": "application/x-www-form-urlencoded",
                },
                body: new URLSearchParams(bodyParams),
            })

            const data = await response.json()
            setTokenResponse(data)

            if (response.ok) {
                toast({
                    title: "¡Tokens obtenidos!",
                    description: "El intercambio fue exitoso. Revisa los tokens decodificados.",
                    variant: "success",
                })
                setCurrentStep(4)
            } else {
                toast({
                    title: "Error en el intercambio",
                    description: data.error_description || data.error,
                    variant: "destructive",
                })
            }
        } catch (error: any) {
            toast({
                title: "Error de red",
                description: error.message,
                variant: "destructive",
            })
        } finally {
            setIsExchanging(false)
        }
    }

    // Fetch UserInfo with access token
    const fetchUserInfo = async () => {
        if (!tokenResponse?.access_token) {
            toast({
                title: "Access token requerido",
                description: "Primero debes obtener un access token",
                variant: "destructive",
            })
            return
        }

        setIsLoadingUserInfo(true)
        setUserInfoResponse(null)

        try {
            const tenantSlug = selectedTenantData?.slug || selectedTenant
            const userInfoEndpoint = `${window.location.origin}/${tenantSlug}/userinfo`

            const response = await apiFetch(userInfoEndpoint, {
                method: "GET",
                headers: {
                    "Authorization": `Bearer ${tokenResponse.access_token}`,
                },
            })

            const data = await response.json()

            if (response.ok) {
                setUserInfoResponse(data)
                toast({
                    title: "UserInfo obtenido",
                    description: "Claims del usuario recuperados exitosamente",
                    variant: "success",
                })
            } else {
                setUserInfoResponse({ error: true, ...data })
                toast({
                    title: "Error al obtener UserInfo",
                    description: data.error_description || data.error || "Token inválido o expirado",
                    variant: "destructive",
                })
            }
        } catch (error: any) {
            setUserInfoResponse({ error: true, message: error.message })
            toast({
                title: "Error de red",
                description: error.message,
                variant: "destructive",
            })
        } finally {
            setIsLoadingUserInfo(false)
        }
    }

    // Refresh tokens using refresh_token
    const refreshTokens = async () => {
        if (!tokenResponse?.refresh_token) {
            toast({
                title: "Refresh token requerido",
                description: "No hay refresh token disponible",
                variant: "destructive",
            })
            return
        }

        setIsRefreshing(true)

        try {
            const bodyParams: Record<string, string> = {
                grant_type: "refresh_token",
                refresh_token: tokenResponse.refresh_token,
                client_id: selectedClientData?.clientId || selectedClientData?.client_id || "",
            }

            if (selectedClientData?.type === "confidential" && selectedClientData?.secret) {
                bodyParams.client_secret = selectedClientData.secret
            }

            const response = await apiFetch(tokenEndpoint, {
                method: "POST",
                headers: {
                    "Content-Type": "application/x-www-form-urlencoded",
                },
                body: new URLSearchParams(bodyParams),
            })

            const data = await response.json()

            if (response.ok) {
                setTokenResponse(data)
                setRefreshCount(prev => prev + 1)
                setUserInfoResponse(null) // Reset UserInfo cuando se refrescan tokens
                toast({
                    title: "Tokens actualizados",
                    description: "Nuevos access token e ID token obtenidos",
                    variant: "success",
                })
            } else {
                toast({
                    title: "Error al refrescar",
                    description: data.error_description || data.error,
                    variant: "destructive",
                })
            }
        } catch (error: any) {
            toast({
                title: "Error de red",
                description: error.message,
                variant: "destructive",
            })
        } finally {
            setIsRefreshing(false)
        }
    }

    // Copy URL to clipboard
    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text)
        setCopiedUrl(true)
        setTimeout(() => setCopiedUrl(false), 2000)
        toast({
            title: "Copiado",
            description: "URL copiada al portapapeles",
            variant: "info",
        })
    }

    // Add custom scope
    const addScope = () => {
        if (customScope && !selectedScopes.includes(customScope)) {
            setSelectedScopes([...selectedScopes, customScope])
            setCustomScope("")
        }
    }

    // Remove scope
    const removeScope = (scope: string) => {
        setSelectedScopes(selectedScopes.filter(s => s !== scope))
    }

    // Check if can proceed to next step
    const canProceedStep1 = selectedTenant && selectedClient
    const canProceedStep2 = redirectUri && selectedScopes.length > 0

    return (
        <div className="space-y-6 max-w-4xl mx-auto">
            {/* Header */}
            <div>
                <h1 className="text-3xl font-bold flex items-center gap-3">
                    OAuth2 Playground
                </h1>
                <p className="text-muted-foreground mt-1">
                    Prueba el flujo de autenticación OAuth2/OIDC de tus aplicaciones
                </p>
            </div>

            {/* Info Banner */}
            <div className="flex items-start gap-3 p-4 rounded-xl border bg-gradient-to-r from-accent/5 to-accent/5 border-accent/10 shadow-clay-card">
                <div className="h-10 w-10 rounded-lg bg-accent/10 flex items-center justify-center shrink-0">
                    <Info className="h-5 w-5 text-accent" />
                </div>
                <div>
                    <h3 className="font-medium text-sm">¿Qué es el OAuth2 Playground?</h3>
                    <p className="text-xs text-muted-foreground mt-1 max-w-2xl">
                        Esta herramienta te permite simular el flujo OAuth2 Authorization Code paso a paso.
                        Selecciona una aplicación, configura los parámetros, y prueba el login de tus usuarios.
                        Los tokens resultantes se decodifican automáticamente para que puedas inspeccionar los claims.
                    </p>
                </div>
            </div>

            {/* Steps Indicator */}
            <div className="flex items-center justify-between p-4 rounded-xl border bg-card shadow-clay-card">
                <StepIndicator step={1} currentStep={currentStep} label="Seleccionar App" />
                <ChevronRight className="h-4 w-4 text-muted-foreground" />
                <StepIndicator step={2} currentStep={currentStep} label="Configurar" />
                <ChevronRight className="h-4 w-4 text-muted-foreground" />
                <StepIndicator step={3} currentStep={currentStep} label="Autorizar" />
                <ChevronRight className="h-4 w-4 text-muted-foreground" />
                <StepIndicator step={4} currentStep={currentStep} label="Tokens" />
            </div>

            {/* Step 1: Select Application */}
            {currentStep === 1 && (
                <Card className="p-6 shadow-clay-card">
                    <div className="space-y-6">
                        <div>
                            <h2 className="text-xl font-semibold flex items-center gap-2">
                                <span className="h-7 w-7 rounded-full bg-accent text-accent-foreground flex items-center justify-center text-sm">1</span>
                                Selecciona la Aplicación
                            </h2>
                            <p className="text-sm text-muted-foreground mt-1">
                                Elige el tenant y el client que quieres probar
                            </p>
                        </div>

                        <div className="grid gap-4 md:grid-cols-2">
                            <div className="space-y-2">
                                <Label className="flex items-center">
                                    Tenant
                                    <InfoTooltip content="El tenant es el espacio aislado donde residen los usuarios y configuración de tu aplicación" />
                                </Label>
                                <Select value={selectedTenant} onValueChange={(v) => { setSelectedTenant(v); setSelectedClient("") }}>
                                    <SelectTrigger>
                                        <SelectValue placeholder="Selecciona un tenant" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {tenants?.filter((t) => t.id).map((tenant) => (
                                            <SelectItem key={tenant.id} value={tenant.id}>
                                                <div className="flex items-center gap-2">
                                                    <div className="h-6 w-6 rounded bg-gradient-to-br from-accent to-accent-foreground flex items-center justify-center text-white text-xs">
                                                        {tenant.name.charAt(0).toUpperCase()}
                                                    </div>
                                                    {tenant.name}
                                                </div>
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>

                            <div className="space-y-2">
                                <Label className="flex items-center">
                                    Client (Aplicación)
                                    <InfoTooltip content="El client representa tu aplicación que solicita acceso a los recursos del usuario" />
                                </Label>
                                {clients && clients.length === 0 && selectedTenant ? (
                                    <div className="flex flex-col gap-2">
                                        <div className="relative">
                                            <Select disabled>
                                                <SelectTrigger>
                                                    <SelectValue placeholder="No hay clientes creados" />
                                                </SelectTrigger>
                                            </Select>
                                        </div>
                                        <Button
                                            variant="outline"
                                            className="w-full border-dashed"
                                            onClick={() => window.location.href = `/admin/tenants/${selectedTenant}/clients`}
                                        >
                                            <Plus className="mr-2 h-4 w-4" />
                                            Crear Cliente
                                        </Button>
                                    </div>
                                ) : (
                                    <Select
                                        value={selectedClient}
                                        onValueChange={setSelectedClient}
                                        disabled={!selectedTenant}
                                    >
                                        <SelectTrigger>
                                            <SelectValue placeholder={selectedTenant ? "Selecciona un client" : "Primero selecciona un tenant"} />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {clients?.map((client: any) => {
                                                const id = client.client_id || client.clientId || client.id
                                                if (!id) return null

                                                return (
                                                    <SelectItem key={id} value={id}>
                                                        <div className="flex items-center gap-2">
                                                            <Badge variant="outline" className="text-[10px]">
                                                                {client.type === "public" ? "Público" : "Confidencial"}
                                                            </Badge>
                                                            {client.name}
                                                        </div>
                                                    </SelectItem>
                                                )
                                            })}
                                        </SelectContent>
                                    </Select>
                                )}
                            </div>
                        </div>

                        {/* Client Info Preview */}
                        {selectedClientData && (
                            <div className="p-4 rounded-lg border bg-muted/5 space-y-3 shadow-clay-card">
                                <h4 className="text-sm font-medium flex items-center gap-2">
                                    <Shield className="h-4 w-4 text-info" />
                                    Información del Client
                                </h4>
                                <div className="grid gap-2 text-sm">
                                    <div className="flex items-center justify-between">
                                        <span className="text-muted-foreground">Client ID</span>
                                        <code className="bg-muted px-2 py-0.5 rounded text-xs">
                                            {selectedClientData.clientId || selectedClientData.client_id}
                                        </code>
                                    </div>
                                    <div className="flex items-center justify-between">
                                        <span className="text-muted-foreground">Tipo</span>
                                        <Badge variant={selectedClientData.type === "public" ? "outline" : "default"}>
                                            {selectedClientData.type === "public" ? "Público" : "Confidencial"}
                                        </Badge>
                                    </div>
                                    {(selectedClientData.redirectUris?.length > 0 || selectedClientData.redirect_uris?.length > 0) && (
                                        <div className="flex items-center justify-between">
                                            <span className="text-muted-foreground">Redirect URIs</span>
                                            <span className="text-xs">{(selectedClientData.redirectUris || selectedClientData.redirect_uris).length} configurados</span>
                                        </div>
                                    )}
                                    {selectedClientData.scopes && selectedClientData.scopes.length > 0 && (
                                        <div className="flex items-center justify-between">
                                            <span className="text-muted-foreground">Scopes permitidos</span>
                                            <div className="flex gap-1">
                                                {selectedClientData.scopes.slice(0, 3).map(s => (
                                                    <Badge key={s} variant="outline" className="text-[10px]">{s}</Badge>
                                                ))}
                                                {selectedClientData.scopes.length > 3 && (
                                                    <Badge variant="outline" className="text-[10px]">+{selectedClientData.scopes.length - 3}</Badge>
                                                )}
                                            </div>
                                        </div>
                                    )}
                                </div>
                            </div>
                        )}

                        <div className="flex justify-end">
                            <Button
                                onClick={() => setCurrentStep(2)}
                                disabled={!canProceedStep1}
                                className="shadow-clay-button hover:shadow-clay-float hover:-translate-y-0.5 active:translate-y-0 transition-all"
                            >
                                Continuar
                                <ArrowRight className="ml-2 h-4 w-4" />
                            </Button>
                        </div>
                    </div>
                </Card>
            )}

            {/* Step 2: Configure Request */}
            {currentStep === 2 && (
                <Card className="p-6 shadow-clay-card">
                    <div className="space-y-6">
                        <div className="flex items-center justify-between">
                            <div>
                                <h2 className="text-xl font-semibold flex items-center gap-2">
                                    <span className="h-7 w-7 rounded-full bg-accent text-accent-foreground flex items-center justify-center text-sm">2</span>
                                    Configura el Request
                                </h2>
                                <p className="text-sm text-muted-foreground mt-1">
                                    Define los parámetros de la solicitud de autorización
                                </p>
                            </div>
                            <Button variant="ghost" size="sm" onClick={() => setCurrentStep(1)}>
                                <ArrowLeft className="mr-2 h-4 w-4" />
                                Volver
                            </Button>
                        </div>

                        <div className="space-y-4">
                            {/* Redirect URI */}
                            <div className="space-y-2">
                                <Label className="flex items-center">
                                    Redirect URI
                                    <InfoTooltip content="URL donde el usuario será redirigido después de autenticarse. Debe estar registrada en el client." />
                                </Label>
                                <Select value={redirectUri} onValueChange={setRedirectUri}>
                                    <SelectTrigger>
                                        <SelectValue placeholder="Selecciona o escribe una URI" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {(selectedClientData?.redirectUris || selectedClientData?.redirect_uris)?.map((uri: string) => (
                                            <SelectItem key={uri} value={uri}>
                                                {uri}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                {(selectedClientData?.redirectUris?.length === 0 && selectedClientData?.redirect_uris?.length === 0) && (
                                    <p className="text-xs text-amber-600">
                                        ⚠️ Este client no tiene redirect URIs configuradas
                                    </p>
                                )}
                            </div>

                            {/* Response Type */}
                            <div className="space-y-2">
                                <Label className="flex items-center">
                                    Response Type
                                    <InfoTooltip content="Tipo de respuesta esperada. 'code' es el más seguro y recomendado." />
                                </Label>
                                <Select value={responseType} onValueChange={setResponseType}>
                                    <SelectTrigger>
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="code">
                                            <div className="flex items-center gap-2">
                                                <Badge className="text-[10px] bg-success/10 text-success border-success/20">Recomendado</Badge>
                                                code
                                            </div>
                                        </SelectItem>
                                        <SelectItem value="token">token (Implicit - No recomendado)</SelectItem>
                                        <SelectItem value="id_token">id_token</SelectItem>
                                        <SelectItem value="code id_token">code id_token (Hybrid)</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>

                            {/* Scopes */}
                            <div className="space-y-2">
                                <Label className="flex items-center">
                                    Scopes
                                    <InfoTooltip content="Permisos que solicitas al usuario. 'openid' es obligatorio para OIDC." />
                                </Label>
                                <div className="flex flex-wrap gap-2 p-3 rounded-lg border bg-muted/5 min-h-[48px]">
                                    {selectedScopes.map((scope) => (
                                        <Badge
                                            key={scope}
                                            variant="secondary"
                                            className="cursor-pointer hover:bg-destructive/20"
                                            onClick={() => removeScope(scope)}
                                        >
                                            {scope}
                                            <span className="ml-1 text-muted-foreground">×</span>
                                        </Badge>
                                    ))}
                                    <div className="flex items-center gap-1">
                                        <Input
                                            value={customScope}
                                            onChange={(e) => setCustomScope(e.target.value)}
                                            onKeyDown={(e) => e.key === 'Enter' && (e.preventDefault(), addScope())}
                                            placeholder="Agregar scope..."
                                            className="h-6 w-24 text-xs border-0 bg-transparent p-0 focus-visible:ring-0"
                                        />
                                        {customScope && (
                                            <Button variant="ghost" size="sm" className="h-6 px-2" onClick={addScope}>
                                                <Plus className="h-3 w-3" />
                                            </Button>
                                        )}
                                    </div>
                                </div>
                                <div className="flex gap-2">
                                    {['openid', 'profile', 'email', 'offline_access'].filter(s => !selectedScopes.includes(s)).map(s => (
                                        <Button
                                            key={s}
                                            variant="outline"
                                            size="sm"
                                            className="text-xs h-6"
                                            onClick={() => setSelectedScopes([...selectedScopes, s])}
                                        >
                                            + {s}
                                        </Button>
                                    ))}
                                </div>
                            </div>

                            {/* PKCE */}
                            {responseType === "code" && (
                                <div className="p-4 rounded-lg border bg-success/5 border-success/20 space-y-3 shadow-clay-card">
                                    <div className="flex items-center justify-between">
                                        <div className="flex items-center gap-2">
                                            <Lock className="h-4 w-4 text-success" />
                                            <Label className="cursor-pointer">
                                                Usar PKCE
                                                <InfoTooltip content="Proof Key for Code Exchange - Agrega seguridad adicional al flujo de autorización. Recomendado para todas las aplicaciones." />
                                            </Label>
                                        </div>
                                        <div className="flex items-center gap-2">
                                            <Badge className="text-[10px] bg-success/10 text-success border-success/20">
                                                Recomendado
                                            </Badge>
                                            <input
                                                type="checkbox"
                                                checked={usePkce}
                                                onChange={(e) => setUsePkce(e.target.checked)}
                                                className="h-4 w-4"
                                            />
                                        </div>
                                    </div>
                                    {usePkce && (
                                        <div className="space-y-2">
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={generatePkce}
                                                className="w-full text-xs"
                                            >
                                                <RefreshCw className="mr-2 h-3 w-3" />
                                                Generar Code Verifier & Challenge
                                            </Button>
                                            {codeChallenge && (
                                                <div className="text-xs space-y-1 font-mono bg-muted p-2 rounded">
                                                    <div><span className="text-muted-foreground">verifier:</span> {codeVerifier.substring(0, 20)}...</div>
                                                    <div><span className="text-muted-foreground">challenge:</span> {codeChallenge.substring(0, 20)}...</div>
                                                </div>
                                            )}
                                        </div>
                                    )}
                                </div>
                            )}

                            {/* State & Nonce */}
                            <div className="grid gap-4 md:grid-cols-2">
                                <div className="space-y-2">
                                    <Label className="flex items-center">
                                        State
                                        <InfoTooltip content="Valor aleatorio para prevenir CSRF. Se genera automáticamente si no lo especificas." />
                                    </Label>
                                    <div className="flex gap-2">
                                        <Input
                                            value={state}
                                            onChange={(e) => setState(e.target.value)}
                                            placeholder="Se genera automáticamente"
                                        />
                                        <Button variant="outline" size="icon" onClick={() => setState(generateRandom())}>
                                            <RefreshCw className="h-4 w-4" />
                                        </Button>
                                    </div>
                                </div>
                                <div className="space-y-2">
                                    <Label className="flex items-center">
                                        Nonce
                                        <InfoTooltip content="Valor aleatorio para prevenir replay attacks en ID tokens." />
                                    </Label>
                                    <div className="flex gap-2">
                                        <Input
                                            value={nonce}
                                            onChange={(e) => setNonce(e.target.value)}
                                            placeholder="Se genera automáticamente"
                                        />
                                        <Button variant="outline" size="icon" onClick={() => setNonce(generateRandom())}>
                                            <RefreshCw className="h-4 w-4" />
                                        </Button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div className="flex justify-end">
                            <Button
                                onClick={generateAuthUrl}
                                disabled={!canProceedStep2 || (usePkce && !codeChallenge && responseType === "code")}
                                className="shadow-clay-button hover:shadow-clay-float hover:-translate-y-0.5 active:translate-y-0 transition-all"
                            >
                                Generar URL de Autorización
                                <ArrowRight className="ml-2 h-4 w-4" />
                            </Button>
                        </div>
                    </div>
                </Card>
            )}

            {/* Step 3: Authorization */}
            {currentStep === 3 && (
                <Card className="p-6 shadow-clay-card">
                    <div className="space-y-6">
                        <div className="flex items-center justify-between">
                            <div>
                                <h2 className="text-xl font-semibold flex items-center gap-2">
                                    <span className="h-7 w-7 rounded-full bg-accent text-accent-foreground flex items-center justify-center text-sm">3</span>
                                    Inicia el Flujo de Autorización
                                </h2>
                                <p className="text-sm text-muted-foreground mt-1">
                                    Abre la URL en el navegador, autentícate, y copia el código del callback
                                </p>
                            </div>
                            <Button variant="ghost" size="sm" onClick={() => setCurrentStep(2)}>
                                <ArrowLeft className="mr-2 h-4 w-4" />
                                Volver
                            </Button>
                        </div>

                        {/* Authorization URL */}
                        <div className="space-y-3">
                            <Label className="flex items-center">
                                URL de Autorización
                                <InfoTooltip content="Esta URL redirige al usuario a la página de login. Después de autenticarse, será redirigido a tu redirect_uri con un código." />
                            </Label>
                            <div className="relative">
                                <Textarea
                                    value={authUrl}
                                    readOnly
                                    rows={3}
                                    className="font-mono text-sm pr-24"
                                />
                                <div className="absolute top-2 right-2 flex gap-1">
                                    <Button variant="outline" size="sm" onClick={() => copyToClipboard(authUrl)}>
                                        {copiedUrl ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                                    </Button>
                                </div>
                            </div>
                            <div className="flex gap-2">
                                <Button
                                    onClick={() => window.open(authUrl, "_blank")}
                                    className="flex-1 shadow-clay-button hover:shadow-clay-float hover:-translate-y-0.5 active:translate-y-0 transition-all"
                                >
                                    <ExternalLink className="mr-2 h-4 w-4" />
                                    Abrir en Nueva Pestaña
                                </Button>
                                <Button
                                    variant="outline"
                                    onClick={() => copyToClipboard(authUrl)}
                                    className="hover:scale-110 active:scale-95 transition-transform"
                                >
                                    <Copy className="mr-2 h-4 w-4" />
                                    Copiar URL
                                </Button>
                            </div>
                        </div>

                        {/* Instructions */}
                        <div className="p-4 rounded-lg border bg-amber-500/5 border-amber-500/20">
                            <h4 className="text-sm font-medium flex items-center gap-2 text-amber-700 dark:text-amber-300">
                                <AlertCircle className="h-4 w-4" />
                                Siguiente paso
                            </h4>
                            <ol className="mt-2 text-xs text-amber-600 dark:text-amber-400 space-y-1 list-decimal list-inside">
                                <li>Haz clic en "Abrir en Nueva Pestaña" para iniciar el login</li>
                                <li>Autentícate con las credenciales del usuario de prueba</li>
                                <li>Después del login, serás redirigido a tu redirect_uri</li>
                                <li>Copia el parámetro <code className="bg-amber-200 dark:bg-amber-900 px-1 rounded">code</code> de la URL</li>
                                <li>Pégalo abajo para intercambiarlo por tokens</li>
                            </ol>
                        </div>

                        {/* Code Input */}
                        <div className="space-y-3">
                            <Label className="flex items-center">
                                Código de Autorización
                                <InfoTooltip content="Después de autenticarte, copia el valor del parámetro 'code' de la URL de callback" />
                            </Label>
                            <div className="flex gap-2">
                                <Input
                                    value={tokenCode}
                                    onChange={(e) => setTokenCode(e.target.value)}
                                    placeholder="Pega aquí el código de la URL (ej: abc123...)"
                                    className="font-mono"
                                />
                                <Button
                                    onClick={exchangeToken}
                                    disabled={!tokenCode || isExchanging}
                                    className="shadow-clay-button hover:shadow-clay-float hover:-translate-y-0.5 active:translate-y-0 transition-all"
                                >
                                    {isExchanging ? (
                                        <>
                                            <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                                            Intercambiando...
                                        </>
                                    ) : (
                                        <>
                                            <Play className="mr-2 h-4 w-4" />
                                            Intercambiar
                                        </>
                                    )}
                                </Button>
                            </div>
                        </div>

                        {/* Code Samples */}
                        <CodeSamples
                            authUrl={authUrl}
                            tokenEndpoint={tokenEndpoint}
                            clientId={selectedClientData?.clientId || selectedClientData?.client_id || ""}
                            clientSecret={selectedClientData?.type === "confidential" ? selectedClientData.secret : undefined}
                            redirectUri={redirectUri}
                            code={tokenCode}
                        />
                    </div>
                </Card>
            )}

            {/* Step 4: Tokens */}
            {currentStep === 4 && tokenResponse && (
                <Card className="p-6 shadow-clay-card">
                    <div className="space-y-6">
                        <div className="flex items-center justify-between">
                            <div>
                                <h2 className="text-xl font-semibold flex items-center gap-2">
                                    <CheckCircle2 className="h-6 w-6 text-success" />
                                    ¡Tokens Obtenidos!
                                </h2>
                                <p className="text-sm text-muted-foreground mt-1">
                                    El flujo OAuth2 se completó exitosamente
                                </p>
                            </div>
                            <Button
                                variant="outline"
                                size="sm"
                                className="shadow-clay-button hover:shadow-clay-float hover:scale-105 active:scale-95 transition-all"
                                onClick={() => {
                                    setCurrentStep(1)
                                    setTokenResponse(null)
                                    setTokenCode("")
                                    setAuthUrl("")
                                    setUserInfoResponse(null)
                                    setRefreshCount(0)
                                }}>
                                <RefreshCw className="mr-2 h-4 w-4" />
                                Nuevo Test
                            </Button>
                        </div>

                        {/* Token Response Raw */}
                        {tokenResponse.error ? (
                            <div className="p-4 rounded-lg border border-destructive/20 bg-destructive/5 shadow-clay-card">
                                <h4 className="text-sm font-medium text-destructive flex items-center gap-2">
                                    <AlertCircle className="h-4 w-4" />
                                    Error en la respuesta
                                </h4>
                                <pre className="mt-2 text-xs font-mono text-destructive">
                                    {JSON.stringify(tokenResponse, null, 2)}
                                </pre>
                            </div>
                        ) : (
                            <div className="space-y-6">
                                {/* Access Token */}
                                {tokenResponse.access_token && (
                                    <JwtDecoder
                                        token={tokenResponse.access_token}
                                        title="Access Token"
                                    />
                                )}

                                {/* ID Token */}
                                {tokenResponse.id_token && (
                                    <JwtDecoder
                                        token={tokenResponse.id_token}
                                        title="ID Token"
                                    />
                                )}

                                {/* Refresh Token */}
                                {tokenResponse.refresh_token && (
                                    <div className="p-4 rounded-lg border bg-muted/5 shadow-clay-card">
                                        <div className="flex items-center justify-between mb-2">
                                            <h4 className="text-sm font-medium flex items-center gap-2">
                                                <RefreshCw className="h-4 w-4 text-info" />
                                                Refresh Token
                                                {refreshCount > 0 && (
                                                    <Badge variant="outline" className="text-[10px] bg-info/10 text-info border-info/20">
                                                        Refreshed {refreshCount}x
                                                    </Badge>
                                                )}
                                            </h4>
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={refreshTokens}
                                                disabled={isRefreshing}
                                                className="text-xs shadow-clay-button hover:scale-110 active:scale-95 transition-transform"
                                            >
                                                {isRefreshing ? (
                                                    <>
                                                        <RefreshCw className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                                                        Refrescando...
                                                    </>
                                                ) : (
                                                    <>
                                                        <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
                                                        Refresh Tokens
                                                    </>
                                                )}
                                            </Button>
                                        </div>
                                        <code className="text-xs font-mono text-muted-foreground break-all">
                                            {tokenResponse.refresh_token}
                                        </code>
                                    </div>
                                )}

                                {/* UserInfo Testing */}
                                {tokenResponse.access_token && (
                                    <div className="p-4 rounded-lg border bg-accent/5 shadow-clay-card">
                                        <div className="flex items-center justify-between mb-3">
                                            <h4 className="text-sm font-medium flex items-center gap-2">
                                                <User className="h-4 w-4 text-accent" />
                                                Probar UserInfo Endpoint
                                            </h4>
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={fetchUserInfo}
                                                disabled={isLoadingUserInfo}
                                                className="text-xs shadow-clay-button hover:scale-110 active:scale-95 transition-transform"
                                            >
                                                {isLoadingUserInfo ? (
                                                    <>
                                                        <RefreshCw className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                                                        Obteniendo...
                                                    </>
                                                ) : (
                                                    <>
                                                        <Play className="mr-1.5 h-3.5 w-3.5" />
                                                        GET /userinfo
                                                    </>
                                                )}
                                            </Button>
                                        </div>

                                        {userInfoResponse && (
                                            <div className="rounded-lg border bg-white dark:bg-zinc-800 overflow-hidden shadow-clay-card">
                                                {userInfoResponse.error ? (
                                                    <div className="p-3 bg-destructive/5 border-destructive/20">
                                                        <p className="text-xs text-destructive font-medium flex items-center gap-1.5">
                                                            <AlertCircle className="h-3.5 w-3.5" />
                                                            {userInfoResponse.error_description || userInfoResponse.message || "Error al obtener claims"}
                                                        </p>
                                                    </div>
                                                ) : (
                                                    <div className="divide-y">
                                                        <div className="px-3 py-2 bg-accent/10">
                                                            <span className="text-xs font-medium text-accent">
                                                                OIDC User Claims
                                                            </span>
                                                        </div>
                                                        <div className="p-3 space-y-2">
                                                            {Object.entries(userInfoResponse).map(([key, value]) => (
                                                                <div key={key} className="flex items-start justify-between text-xs gap-3">
                                                                    <span className="text-muted-foreground font-mono">{key}</span>
                                                                    <code className="bg-muted px-2 py-0.5 rounded text-right flex-1 break-all">
                                                                        {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                                                                    </code>
                                                                </div>
                                                            ))}
                                                        </div>
                                                    </div>
                                                )}
                                            </div>
                                        )}

                                        {!userInfoResponse && !isLoadingUserInfo && (
                                            <p className="text-xs text-muted-foreground mt-2">
                                                Haz clic en el botón para verificar que el access token funciona contra el endpoint OIDC /userinfo.
                                            </p>
                                        )}
                                    </div>
                                )}

                                {/* Additional Info */}
                                <div className="grid grid-cols-3 gap-4">
                                    {tokenResponse.token_type && (
                                        <div className="p-3 rounded-lg border bg-muted/5 shadow-clay-card">
                                            <p className="text-xs text-muted-foreground">Token Type</p>
                                            <p className="font-medium">{tokenResponse.token_type}</p>
                                        </div>
                                    )}
                                    {tokenResponse.expires_in && (
                                        <div className="p-3 rounded-lg border bg-muted/5 shadow-clay-card">
                                            <p className="text-xs text-muted-foreground">Expira en</p>
                                            <p className="font-medium">{tokenResponse.expires_in}s</p>
                                        </div>
                                    )}
                                    {tokenResponse.scope && (
                                        <div className="p-3 rounded-lg border bg-muted/5 shadow-clay-card">
                                            <p className="text-xs text-muted-foreground">Scopes</p>
                                            <p className="font-medium text-sm">{tokenResponse.scope}</p>
                                        </div>
                                    )}
                                </div>

                                {/* Raw Response */}
                                <details className="group">
                                    <summary className="text-sm text-muted-foreground cursor-pointer hover:text-foreground transition-colors">
                                        Ver respuesta JSON completa
                                    </summary>
                                    <pre className="mt-2 p-4 rounded-lg bg-zinc-950 text-zinc-100 text-xs overflow-x-auto font-mono">
                                        <code>{JSON.stringify(tokenResponse, null, 2)}</code>
                                    </pre>
                                </details>
                            </div>
                        )}
                    </div>
                </Card>
            )}
        </div>
    )
}

// Plus icon for scope adding
function Plus({ className }: { className?: string }) {
    return (
        <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
        </svg>
    )
}
