// Hook to auto-refresh access tokens using refresh_token before expiry

"use client"

import { useEffect, useRef } from "react"
import { ApiClient } from "./api"
import { useAuthStore } from "./auth-store"
import type { LoginResponse } from "./types"

export function useAuthRefresh() {
  const token = useAuthStore((s) => s.token)
  const refreshToken = useAuthStore((s) => s.refreshToken)
  const expiresAt = useAuthStore((s) => s.expiresAt)
  const user = useAuthStore((s) => s.user)
  const setAuth = useAuthStore((s) => s.setAuth)
  const clearAuth = useAuthStore((s) => s.clearAuth)

  const timerRef = useRef<NodeJS.Timeout | null>(null)

  useEffect(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }

    if (!refreshToken || !expiresAt) {
      return
    }

    const now = Date.now()
    // Refresh 60 seconds before expiry, but at least in 5 seconds
    const msUntilRefresh = Math.max(expiresAt - now - 60_000, 5_000)

    timerRef.current = setTimeout(async () => {
      try {
        const apiBase = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080"
        const client = new ApiClient(apiBase, () => token, () => {}, () => {})
        // Derive client_id and tenant from current token claims if not present in user
        let clientID: string | undefined
        let tenantID: string | undefined
        try {
          if (token) {
            const payload = token.split(".")[1]
            const json = JSON.parse(new TextDecoder().decode(base64UrlDecode(payload)))
            clientID = json["aud"] as string | undefined
            tenantID = (json["tid"] as string | undefined) || (json["tenant_id"] as string | undefined)
          }
        } catch {}
        const resp = await client.post<LoginResponse>("/v1/auth/refresh", {
          client_id: clientID,
          tenant_id: tenantID,
          refresh_token: refreshToken,
        })
        const newExpiresAt = Date.now() + (resp.expires_in ? resp.expires_in * 1000 : 0)
        setAuth(resp.access_token, user, resp.refresh_token ?? refreshToken, newExpiresAt)
      } catch (e) {
        // Refresh failed; clear auth to force re-login
        clearAuth()
        if (typeof window !== "undefined") {
          window.location.href = "/login"
        }
      }
    }, msUntilRefresh)

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current)
        timerRef.current = null
      }
    }
  }, [refreshToken, expiresAt, token, user, setAuth, clearAuth])
}

// base64url decode helper for JWT payloads
function base64UrlDecode(input: string): Uint8Array {
  // Replace URL-safe chars and pad
  let b64 = input.replace(/-/g, "+").replace(/_/g, "/")
  const pad = b64.length % 4
  if (pad) b64 += "=".repeat(4 - pad)
  if (typeof atob !== "undefined") {
    const binStr = atob(b64)
    const bytes = new Uint8Array(binStr.length)
    for (let i = 0; i < binStr.length; i++) bytes[i] = binStr.charCodeAt(i)
    return bytes
  }
  // Node fallback
  return Buffer.from(b64, "base64")
}
