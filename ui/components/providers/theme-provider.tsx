// Theme provider

"use client"

import type React from "react"

import { useEffect } from "react"
import { useUIStore } from "@/lib/ui-store"

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const theme = useUIStore((state) => state.theme)

  useEffect(() => {
    const root = window.document.documentElement
    root.classList.remove("light", "dark")
    root.classList.add(theme)
  }, [theme])

  return <>{children}</>
}
