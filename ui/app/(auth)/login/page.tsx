// Login page

"use client"

import type React from "react"
import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { useMutation } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useToast } from "@/hooks/use-toast"
import { useAuthStore } from "@/lib/auth-store"
import { useUIStore } from "@/lib/ui-store"
import { ApiClient } from "@/lib/api"
import type { LoginRequest, LoginResponse, MeResponse, AuthConfigResponse } from "@/lib/types"
import { getTranslations } from "@/lib/i18n"

export default function LoginPage() {
  const router = useRouter()
  const { toast } = useToast()
  const setAuth = useAuthStore((state) => state.setAuth)
  const setDemoMode = useAuthStore((state) => state.setDemoMode)
  const locale = useUIStore((state) => state.locale)
  const t = getTranslations(locale)

  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")

  // Branding state
  const [authConfig, setAuthConfig] = useState<AuthConfigResponse | null>(null)


  // Detect return_to from URL
  const searchParams = typeof window !== "undefined" ? new URLSearchParams(window.location.search) : null
  const returnTo = searchParams?.get("return_to")

  useEffect(() => {
    // Branding Logic
    const fetchConfig = async () => {
      let clientId = ""
      if (returnTo) {
        try {
          const decoded = decodeURIComponent(returnTo)
          const match = decoded.match(/[?&]client_id=([^&]+)/)
          if (match) clientId = match[1]
        } catch (e) { console.error(e) }
      }
      if (!clientId && searchParams?.get("client_id")) {
        clientId = searchParams.get("client_id")!
      }

      if (clientId) {
        try {
          const apiBase = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080"
          const api = new ApiClient(apiBase, () => null, () => { }, () => { })
          const cfg = await api.get<AuthConfigResponse>(`/v1/auth/config?client_id=${clientId}`)
          setAuthConfig(cfg)
        } catch (e) {
          console.error("Failed to load branding", e)
        }
      }
    }

    fetchConfig()
  }, [returnTo]) // Run once or when returnTo changes

  const loginMutation = useMutation({
    mutationFn: async (data: LoginRequest) => {
      const apiBase = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080"

      // If authenticating for an OAuth flow (return_to present), use Session Login (Cookies)
      if (returnTo) {
        // Need to extract the tenant from the return_to URL if possible, or fallback to "local" / user input
        const api = new ApiClient(apiBase, () => null, () => { }, () => { })

        let targetTenant = "default" // fallback
        let targetClient = "admin-cli" // fallback

        if (returnTo) {
          try {
            const decoded = decodeURIComponent(returnTo)
            const matchTempParams = new URL("http://dummy" + decoded).searchParams
            if (matchTempParams.get("tenant")) targetTenant = matchTempParams.get("tenant")!
            if (matchTempParams.get("client_id")) targetClient = matchTempParams.get("client_id")!
          } catch (e) { console.error("Error parsing return_to", e) }
        }

        // Use a specialized session login call
        await api.post("/v1/session/login", {
          email: data.email,
          password: data.password,
          tenant_id: targetTenant,
          client_id: targetClient
        })
        return { returnToUrl: returnTo }
      }

      // Default Admin Flow (Stateless Token)
      const api = new ApiClient(
        apiBase,
        () => null,
        () => { },
        () => { },
      )

      const loginResponse = await api.post<LoginResponse>("/v1/auth/login", {
        ...data,
        tenant_id: data.tenant_id ?? "", // Send empty if undefined, let backend handle global admin
        client_id: data.client_id ?? ""
      })

      // Get user info
      const tempApi = new ApiClient(
        apiBase,
        () => loginResponse.access_token,
        () => { },
        () => { },
      )
      const userInfo = await tempApi.get<MeResponse>("/v1/me")

      const expiresAt = Date.now() + (loginResponse.expires_in ? loginResponse.expires_in * 1000 : 0)
      return { token: loginResponse.access_token, user: userInfo, refreshToken: loginResponse.refresh_token, expiresAt, returnToUrl: null }
    },
    onSuccess: ({ token, user, refreshToken, expiresAt, returnToUrl }) => {
      // If we have a return url, blindly redirect there (Session established)
      if (returnToUrl) {
        window.location.href = returnToUrl
        return
      }

      // Otherwise, store tokens for Admin Panel
      if (token && user) {
        setAuth(token, user, refreshToken ?? null, expiresAt || null)
        toast({
          title: t.common.success,
          description: "Inicio de sesión exitoso",
        })
        router.push("/admin")
      }
    },
    onError: (error: any) => {
      toast({
        title: t.auth.loginError,
        description: error.error_description || t.auth.invalidCredentials,
        variant: "destructive",
      })
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    // Send only email/password; backend supports FS-admin login without tenant/client.
    loginMutation.mutate({
      email,
      password,
    })
  }

  // UI Calculations
  // If we have authConfig (Tenant context), show ONLY tenant info.
  // If no authConfig (Admin context), show HelloJohn Admin.
  const isTenantContext = !!authConfig
  const title = isTenantContext ? authConfig?.tenant_name : "HelloJohn Admin"
  const subtitle = isTenantContext
    ? (authConfig?.client_name ? `Log in to ${authConfig.client_name}` : "Log in")
    : t.auth.login

  const primaryColor = authConfig?.primary_color || ""
  // We can use primaryColor in inline styles or Tailwind config if setup allows dynamic values.
  // For now we stick to default styling but maybe apply it to button.

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/40 p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          {authConfig?.logo_url && (
            <div className="flex justify-center mb-4">
              <img src={authConfig.logo_url} alt="Logo" className="h-12 object-contain" />
            </div>
          )}
          <CardTitle className="text-2xl font-bold text-center">{title}</CardTitle>
          <CardDescription className="text-center">{subtitle}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="email">{t.auth.email}</Label>
              <Input
                id="email"
                type="email"
                placeholder="name@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">{t.auth.password}</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
            <Button
              type="submit"
              className="w-full"
              disabled={loginMutation.isPending}
              style={primaryColor ? { backgroundColor: primaryColor } : {}}
            >
              {loginMutation.isPending ? t.common.loading : t.auth.loginButton}
            </Button>

            {/* Show Register link if in Tenant Context (assuming public registration allowed or wanted) */}
            {authConfig && (
              <div className="text-center text-sm mt-2">
                Don't have an account?{" "}
                <a href={`/register?${searchParams?.toString()}`} className="underline">Sign up</a>
              </div>
            )}

            {loginMutation.isError && (
              <div className="text-sm text-destructive">
                {loginMutation.error?.message || "Error al iniciar sesión"}
              </div>
            )}
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
