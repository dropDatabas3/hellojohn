// Client layout wrapper: gate rendering until mounted to avoid hydration mismatches

"use client"

import type React from "react"
import { useEffect, useState, Suspense } from "react"

import AdminShell from "./admin-shell"

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const [mounted, setMounted] = useState(false)
  useEffect(() => setMounted(true), [])

  if (!mounted) {
    // Ensure server and first client render match (both null) to prevent hydration mismatch
    return null
  }

  return (
    <Suspense fallback={null}>
      <AdminShell>{children}</AdminShell>
    </Suspense>
  )
}
