// Hook to get configured API client

"use client"

import { useCallback } from "react"
import { useRouter } from "next/navigation"
import { ApiClient } from "../api"
import { useAuthStore } from "../auth-store"
import { useUIStore } from "../ui-store"
import { useToast } from "@/hooks/use-toast"

export function useApi() {
  const router = useRouter()
  const { toast } = useToast()
  const token = useAuthStore((state) => state.token)
  const clearAuth = useAuthStore((state) => state.clearAuth)
  const apiBaseOverride = useUIStore((state) => state.apiBaseOverride)
  const setApiBaseOverride = useUIStore((state) => state.setApiBaseOverride)

  const getToken = useCallback(() => token, [token])

  const onUnauthorized = useCallback(() => {
    clearAuth()
    router.push("/login")
    toast({
      title: "Sesión expirada",
      description: "Por favor, inicia sesión nuevamente",
      variant: "destructive",
    })
  }, [clearAuth, router, toast])

  const onLeaderRedirect = useCallback(
    (leaderUrl: string) => {
      toast({
        title: "Nodo seguidor detectado",
        description: `Este nodo es un seguidor. ¿Cambiar al líder? ${leaderUrl}`,
        action: {
          label: "Cambiar",
          onClick: () => setApiBaseOverride(leaderUrl),
        },
      })
    },
    [toast, setApiBaseOverride],
  )

  const baseUrl = apiBaseOverride || process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080"

  return new ApiClient(baseUrl, getToken, onUnauthorized, onLeaderRedirect)
}
