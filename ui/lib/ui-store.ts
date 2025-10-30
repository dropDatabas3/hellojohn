// Zustand store for UI state (theme, locale, selected tenant)

import { create } from "zustand"
import { persist } from "zustand/middleware"

type UIState = {
  theme: "light" | "dark"
  locale: "es" | "en"
  selectedTenantSlug: string | null
  apiBaseOverride: string | null
  toggleTheme: () => void
  setLocale: (locale: "es" | "en") => void
  setSelectedTenant: (slug: string | null) => void
  setApiBaseOverride: (url: string | null) => void
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      theme: "light",
      locale: "es",
      selectedTenantSlug: null,
      apiBaseOverride: null,
      toggleTheme: () => set((state) => ({ theme: state.theme === "light" ? "dark" : "light" })),
      setLocale: (locale) => set({ locale }),
      setSelectedTenant: (slug) => set({ selectedTenantSlug: slug }),
      setApiBaseOverride: (url) => set({ apiBaseOverride: url }),
    }),
    {
      name: "hellojohn-ui",
    },
  ),
)
