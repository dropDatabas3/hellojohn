// Zustand store for authentication state

import { create } from "zustand"
import { persist } from "zustand/middleware"

type AuthState = {
  token: string | null
  refreshToken: string | null
  expiresAt: number | null // epoch ms when access token expires
  user: {
    sub: string
    email: string
    scopes: string[]
  } | null
  isDemoMode: boolean
  setAuth: (token: string, user: any, refreshToken?: string | null, expiresAt?: number | null) => void
  clearAuth: () => void
  isAuthenticated: () => boolean
  setDemoMode: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      refreshToken: null,
      expiresAt: null,
      user: null,
      isDemoMode: false,
      setAuth: (token, user, refreshToken = null, expiresAt = null) =>
        set({ token, user, refreshToken, expiresAt, isDemoMode: false }),
      clearAuth: () => set({ token: null, refreshToken: null, expiresAt: null, user: null, isDemoMode: false }),
      isAuthenticated: () => {
        const { token, expiresAt, isDemoMode } = get()
        return (!!token && (!expiresAt || expiresAt > Date.now() - 5000)) || isDemoMode
      },
      setDemoMode: () =>
        set({
          isDemoMode: true,
          token: "demo-token",
          refreshToken: null,
          expiresAt: null,
          user: {
            sub: "demo-user",
            email: "demo@hellojohn.local",
            scopes: ["admin:*"],
          },
        }),
    }),
    {
      name: "hellojohn-auth",
      partialize: (state) => ({ token: state.token, refreshToken: state.refreshToken, expiresAt: state.expiresAt, user: state.user, isDemoMode: state.isDemoMode }),
    },
  ),
)
