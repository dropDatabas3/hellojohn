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

    console.log('[AUTH-REFRESH] Scheduling refresh in', Math.round(msUntilRefresh / 1000), 'seconds')
    console.log('[AUTH-REFRESH] Current time:', new Date(now).toISOString())
    console.log('[AUTH-REFRESH] Token expires at:', new Date(expiresAt).toISOString())

    timerRef.current = setTimeout(async () => {
      const refreshStartTime = Date.now()
      console.log('[AUTH-REFRESH] Starting refresh at:', new Date(refreshStartTime).toISOString())

      try {
        const apiBase = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080"
        const client = new ApiClient(apiBase, () => token, () => { }, () => { })

        // Simplified: Backend handles admin refresh tokens automatically via JWT
        // No need to extract client_id/tenant_id for admin tokens
        console.log('[AUTH-REFRESH] Calling /v2/auth/refresh endpoint')
        const resp = await client.post<LoginResponse>("/v2/auth/refresh", {
          refresh_token: refreshToken,
        })

        console.log('[AUTH-REFRESH] Refresh successful')
        console.log('[AUTH-REFRESH] Response expires_in:', resp.expires_in, 'seconds')

        const newExpiresAt = Date.now() + (resp.expires_in ? resp.expires_in * 1000 : 0)
        console.log('[AUTH-REFRESH] New expiration:', new Date(newExpiresAt).toISOString())

        setAuth(resp.access_token, user, resp.refresh_token ?? refreshToken, newExpiresAt)
        console.log('[AUTH-REFRESH] Auth state updated successfully')
      } catch (e: any) {
        // Log detailed error information for debugging
        console.error('[AUTH-REFRESH] Refresh failed:', e)
        console.error('[AUTH-REFRESH] Error details:', {
          message: e?.message,
          status: e?.status,
          statusText: e?.statusText,
          error: e?.error,
          error_description: e?.error_description,
        })

        // Refresh failed; clear auth to force re-login
        console.warn('[AUTH-REFRESH] Clearing auth and redirecting to login')
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
