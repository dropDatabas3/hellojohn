// Login page

"use client"

import type React from "react"

import { useState } from "react"
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
import type { LoginRequest, LoginResponse, MeResponse } from "@/lib/types"
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
  // Per nueva UX: no pedir tenant/cliente en login

  const loginMutation = useMutation({
    mutationFn: async (data: LoginRequest) => {
      const apiBase = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080"
      const api = new ApiClient(
        apiBase,
        () => null,
        () => {},
        () => {},
      )

      const loginResponse = await api.post<LoginResponse>("/v1/auth/login", data)

      // Get user info
      const tempApi = new ApiClient(
        apiBase,
        () => loginResponse.access_token,
        () => {},
        () => {},
      )
      const userInfo = await tempApi.get<MeResponse>("/v1/me")

      const expiresAt = Date.now() + (loginResponse.expires_in ? loginResponse.expires_in * 1000 : 0)
      return { token: loginResponse.access_token, user: userInfo, refreshToken: loginResponse.refresh_token, expiresAt }
    },
    onSuccess: ({ token, user, refreshToken, expiresAt }) => {
      setAuth(token, user, refreshToken ?? null, expiresAt || null)
      toast({
        title: t.common.success,
        description: "Inicio de sesión exitoso",
      })
      router.push("/admin")
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
    console.log("[v0] Login form submitted")
    console.log("[v0] Email:", email)
    console.log("[v0] Password length:", password.length)
  // Tenant/Client ya no se piden en login

    // Send only email/password; backend supports FS-admin login without tenant/client.
    loginMutation.mutate({
      email,
      password,
    })
  }

  const handleDemoMode = () => {
    setDemoMode()
    toast({
      title: "Modo Demo",
      description: "Accediendo al portal en modo demostración",
    })
    router.push("/admin")
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/40 p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <CardTitle className="text-2xl font-bold">HelloJohn Admin</CardTitle>
          <CardDescription>{t.auth.login}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="email">{t.auth.email}</Label>
              <Input
                id="email"
                type="email"
                placeholder="admin@example.com"
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
            {/* Tenant/Client removidos del formulario por nueva UX */}
            <Button type="submit" className="w-full" disabled={loginMutation.isPending}>
              {loginMutation.isPending ? t.common.loading : t.auth.loginButton}
            </Button>
            {loginMutation.isError && (
              <div className="text-sm text-destructive">
                {loginMutation.error?.message || "Error al iniciar sesión"}
              </div>
            )}
          </form>
          <div className="mt-4 pt-4 border-t">
            <Button type="button" variant="outline" className="w-full bg-transparent" onClick={handleDemoMode}>
              🎭 Modo Demo (Sin Backend)
            </Button>
            <p className="text-xs text-muted-foreground text-center mt-2">
              Accede al portal sin credenciales para ver la interfaz
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
