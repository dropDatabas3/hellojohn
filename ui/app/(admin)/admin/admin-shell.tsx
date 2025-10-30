// Client-only Admin Shell extracted from layout to prevent SSR hydration mismatches

"use client"

import type React from "react"
import { useState } from "react"

import { AuthGuard } from "@/components/auth/auth-guard"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { useAuthStore } from "@/lib/auth-store"
import { useUIStore } from "@/lib/ui-store"
import { api } from "@/lib/api"
import { useQuery } from "@tanstack/react-query"
import { useRouter, usePathname, useSearchParams } from "next/navigation"
import Link from "next/link"
import { Building2, Moon, Sun, Book, Shield, Users, Key, FileText, Cog, Boxes, Network, Globe2, Shapes, ListChecks, X } from "lucide-react"
import * as DropdownMenu from "@radix-ui/react-dropdown-menu"
import type { Tenant } from "@/lib/types"
import { useAuthRefresh } from "@/lib/auth-refresh"

export default function AdminShell({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const clearAuth = useAuthStore((state) => state.clearAuth)
  const user = useAuthStore((state) => state.user)
  const hasRefresh = !!useAuthStore((s) => s.refreshToken)
  const [hideFsNotice, setHideFsNotice] = useState(false)
  const theme = useUIStore((state) => state.theme)
  const toggleTheme = useUIStore((state) => state.toggleTheme)

  // Schedule token auto-refresh if available
  useAuthRefresh()

  const handleLogout = () => {
    clearAuth()
    router.push("/login")
  }

  // Tenants for selector
  const { data: tenants } = useQuery({
    queryKey: ["tenants"],
    queryFn: () => api.get<Tenant[]>("/v1/admin/tenants"),
  })
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const currentTenantId = (() => {
    // Prefer query param on static pages
    const q = searchParams.get("id") || searchParams.get("tenant")
    if (q) return q
    // Fallback: extract from legacy dynamic path /admin/tenants/{id}/...
    const m = pathname?.match(/\/admin\/tenants\/([^\/]+)/)
    return m?.[1]
  })()

  // Sidebar simplificado: un acordeón para "Organizaciones"

  return (
    <AuthGuard>
      {/* FS-admin fallback notice when there's no refresh token available */}
      {!hasRefresh && !hideFsNotice && (
        <div className="fixed top-16 left-0 right-0 z-40 bg-amber-100 text-amber-900 border-b border-amber-200 px-4 py-2 text-sm">
          <div className="container mx-auto flex items-center justify-center px-6 relative">
            <p className="text-center pr-8">
              Sesión sin refresh (FS-admin). Tu sesión podría expirar antes; vuelve a iniciar sesión si ocurre.
            </p>
            <button
              type="button"
              onClick={() => setHideFsNotice(true)}
              className="absolute right-4 inline-flex h-6 w-6 items-center justify-center rounded hover:bg-amber-200/60"
              aria-label="Cerrar aviso"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </div>
      )}
      <div className="flex h-screen bg-background">
        {/* Top fixed header */}
        <header className="fixed top-0 left-0 z-50 w-full bg-black/30 backdrop-blur-md text-white/90 border-b border-black/20">
          <div className="container mx-auto flex h-16 items-center justify-between px-6">
            <div className="flex items-center gap-4">
              <Link href="/admin" className="text-lg font-bold">
                HelloJhon
              </Link>
              <nav className="hidden md:flex items-center gap-3 text-sm text-white/90" />
            </div>
            <div className="flex items-center gap-3">
              <Button variant="ghost" size="icon" onClick={toggleTheme} className="shrink-0 text-white/90">
                {theme === "light" ? <Moon className="h-5 w-5" /> : <Sun className="h-5 w-5" />}
              </Button>
              <Link href="/docs" className="rounded px-2 py-1 text-sm hover:bg-white/5">
                <Book className="inline mr-2 h-4 w-4 align-middle" /> Docs
              </Link>
              {/* User avatar dropdown (Radix) */}
              <DropdownMenu.Root>
                <DropdownMenu.Trigger asChild>
                  <button aria-label="User menu" className="flex items-center justify-center h-9 w-9 rounded-full bg-white/10">
                    <span className="text-sm font-medium text-white/90">
                      {user?.email ? user.email.charAt(0).toUpperCase() : "U"}
                    </span>
                  </button>
                </DropdownMenu.Trigger>
                <DropdownMenu.Portal>
                  <DropdownMenu.Content align="end" className="z-50 rounded-md bg-white/95 text-black shadow-lg ring-1 ring-black/10 backdrop-blur-sm">
                    <div className="w-56 py-1">
                      <div className="px-4 py-2 text-xs text-gray-600 truncate">{user?.email}</div>
                      <DropdownMenu.Separator className="my-1 h-px bg-gray-200" />
                      <DropdownMenu.Item asChild>
                        <Link href="/admin/profile" className="block px-4 py-2 text-sm hover:bg-gray-100">
                          Perfil
                        </Link>
                      </DropdownMenu.Item>
                      <DropdownMenu.Item asChild>
                        <Link href="/admin/settings" className="block px-4 py-2 text-sm hover:bg-gray-100">
                          Configuración
                        </Link>
                      </DropdownMenu.Item>
                      <DropdownMenu.Separator className="my-1 h-px bg-gray-200" />
                      <DropdownMenu.Item asChild>
                        <button onClick={handleLogout} className="w-full text-left px-4 py-2 text-sm hover:bg-gray-100">
                          Cerrar sesión
                        </button>
                      </DropdownMenu.Item>
                    </div>
                  </DropdownMenu.Content>
                </DropdownMenu.Portal>
              </DropdownMenu.Root>
            </div>
          </div>
        </header>

        {/* Sidebar */}
        <aside className="w-64 border-r bg-card pt-16">
          <div className="flex h-full flex-col">
            {/* Spacer for header */}
            <div className="h-4" />

            {/* Navigation */}
            <nav className="flex-1 space-y-1 overflow-y-auto p-4">
              <div className="space-y-2">
                {/* Organizations selector only */}
                <div className="px-3">
                  <Select
                      value={(currentTenantId as string) || ""}
                      onValueChange={(val) => {
                        if (val === "+create") {
                          router.push("/admin/tenants")
                        } else {
                          router.push(`/admin/tenants/detail?id=${val}`)
                        }
                      }}
                    >
                    <SelectTrigger className="w-full" aria-label="Seleccionar organización">
                      <SelectValue placeholder="Selecciona una organización" />
                    </SelectTrigger>
                    <SelectContent>
                      {tenants?.map((t) => (
                        <SelectItem key={t.slug} value={t.slug}>
                          {t.name}
                        </SelectItem>
                      ))}
                      <SelectItem value={"+create"}>+ Crear una organización</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                </div>

                {/* Secondary menu when a tenant is selected */}
                {currentTenantId && (
                  <div className="mt-4 space-y-1">
                    <Link href={`/admin/tenants/settings?id=${currentTenantId}`} className="block rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground">
                      <Cog className="mr-2 inline h-4 w-4" /> General
                    </Link>
                    <Link href={`/admin/tenants/users?id=${currentTenantId}`} className="block rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground">
                      <Users className="mr-2 inline h-4 w-4" /> Usuarios
                    </Link>
                    <Link href={`/admin/tenants/clients?id=${currentTenantId}`} className="block rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground">
                      <Key className="mr-2 inline h-4 w-4" /> Clientes
                    </Link>
                    <Link href={`/admin/tenants/scopes?id=${currentTenantId}`} className="block rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground">
                      <Shield className="mr-2 inline h-4 w-4" /> Scopes
                    </Link>
                    <Link href={`/admin/tenants/consents?id=${currentTenantId}`} className="block rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground">
                      <FileText className="mr-2 inline h-4 w-4" /> Consentimientos
                    </Link>
                    <Link href={`/admin/tenants/claims?id=${currentTenantId}`} className="block rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground">
                      <FileText className="mr-2 inline h-4 w-4" /> Claims
                    </Link>
                    <Link href={`/admin/tenants/tokens?id=${currentTenantId}`} className="block rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground">
                      <ListChecks className="mr-2 inline h-4 w-4" /> Tokens
                    </Link>
                    <Link href={`/admin/tenants/sessions?id=${currentTenantId}`} className="block rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground">
                      <Shapes className="mr-2 inline h-4 w-4" /> Sesiones
                    </Link>
                    {/* Global/advanced sections (enlazadas globalmente por ahora) */}
                    <Link href={`/admin/rbac?id=${currentTenantId}`} className="block rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground" aria-label="RBAC y roles">
                      <Boxes className="mr-2 inline h-4 w-4" /> RBAC / Roles
                    </Link>
                    <Link href={`/admin/providers?id=${currentTenantId}`} className="block rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground" aria-label="Proveedores sociales">
                      <Globe2 className="mr-2 inline h-4 w-4" /> Proveedores Sociales
                    </Link>
                    <Link href={`/admin/database?id=${currentTenantId}`} className="block rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground" aria-label="Storage y cache">
                      <Network className="mr-2 inline h-4 w-4" /> Storage / Cache
                    </Link>
                  </div>
                )}

                {/* Global sections */}
                <div className="mt-6 space-y-1 px-3">
                  <Link href="/admin/logs" className="block rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground" aria-label="Ver logs">
                    <FileText className="mr-2 inline h-4 w-4" /> Logs
                  </Link>
                  <Link href="/admin/config" className="block rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground" aria-label="Configuración del sistema">
                    <Cog className="mr-2 inline h-4 w-4" /> Configuración del sistema
                  </Link>
                </div>
            </nav>

            {/* User section */}
            <div className="border-t p-4">
              <div className="mb-3">
                <p className="text-sm font-medium truncate">{user?.email}</p>
                <p className="text-xs text-muted-foreground">Admin</p>
              </div>
              {/* logout and theme moved to header */}
            </div>
          </div>
        </aside>

        {/* Main content */}
        <main className="flex-1 overflow-y-auto pt-16">
          <div className="container mx-auto p-6">{children}</div>
        </main>
      </div>
    </AuthGuard>
  )
}
