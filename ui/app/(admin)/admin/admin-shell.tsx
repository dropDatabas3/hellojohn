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
import {
  Building2,
  Moon,
  Sun,
  Shield,
  Users,
  Key,
  FileText,
  Boxes,
  Globe2,
  Shapes,
  ListChecks,
  X,
  LayoutDashboard,
  LogOut,
  User,
  Settings,
  ChevronDown,
  Bell,
  Database,
  Lock,
  Fingerprint,
  ChevronsLeft,
  ChevronsRight,
  LayoutTemplate,
  Mail,
  Zap,
  Server,
  Activity
} from "lucide-react"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { Tenant } from "@/lib/types"
import { useAuthRefresh } from "@/lib/auth-refresh"
import { CommandPalette } from "@/components/command-palette"
import { cn } from "@/lib/utils"
import { CreateTenantWizard } from "@/components/tenant/CreateTenantWizard"

// Organization Selector Component
function OrganizationSelector({
  currentOrg,
  onSelect,
}: {
  currentOrg: Tenant | undefined
  onSelect: (value: string) => void
}) {
  const { data: tenants } = useQuery({
    queryKey: ["tenants"],
    queryFn: () => api.get<Tenant[]>("/v2/admin/tenants"),
  })

  return (
    <Select
      value={currentOrg?.slug || ""}
      onValueChange={onSelect}
    >
      <SelectTrigger className="w-[200px] h-9 bg-transparent border-transparent hover:bg-muted focus:ring-0 focus:ring-offset-0 px-2 font-medium">
        <SelectValue placeholder="Select Organization" />
      </SelectTrigger>
      <SelectContent>
        <div className="px-2 py-1.5 text-xs font-semibold text-muted-foreground">
          Organizations
        </div>
        {tenants?.map((t) => (
          <SelectItem key={t.slug} value={t.slug} className="cursor-pointer">
            {t.name}
          </SelectItem>
        ))}
        <DropdownMenuSeparator />
        <SelectItem value="+create" className="text-primary font-medium cursor-pointer">
          + Create New
        </SelectItem>
      </SelectContent>
    </Select>
  )
}

export default function AdminShell({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const clearAuth = useAuthStore((state) => state.clearAuth)
  const user = useAuthStore((state) => state.user)
  const hasRefresh = !!useAuthStore((s) => s.refreshToken)
  const [hideFsNotice, setHideFsNotice] = useState(false)
  const theme = useUIStore((state) => state.theme)
  const toggleTheme = useUIStore((state) => state.toggleTheme)

  // State for Create Wizard
  const [showCreateWizard, setShowCreateWizard] = useState(false)

  // Schedule token auto-refresh if available
  useAuthRefresh()

  const handleLogout = () => {
    clearAuth()
    router.push("/login")
  }

  // Tenants for selector
  const { data: tenants } = useQuery({
    queryKey: ["tenants"],
    queryFn: () => api.get<Tenant[]>("/v2/admin/tenants"),
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

  const currentTenant = tenants?.find((t) => t.id === currentTenantId || t.slug === currentTenantId)

  const handleTenantSelect = (val: string) => {
    if (val === "+create") {
      setShowCreateWizard(true)
    } else {
      const t = tenants?.find((t) => t.slug === val)
      if (t) {
        router.push(`/admin/tenants/detail?id=${t.id}`)
      }
    }
  }

  const [isCollapsed, setIsCollapsed] = useState(false)

  const NavItem = ({ href, icon: Icon, label, active }: { href: string; icon: any; label: string; active?: boolean }) => {
    const hrefPath = href.split('?')[0]
    const isActive = active || pathname === hrefPath || (hrefPath !== "/admin" && pathname?.startsWith(hrefPath))
    return (
      <Link
        href={href}
        title={isCollapsed ? label : undefined}
        className={cn(
          "relative flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-all duration-200",
          isActive
            ? "bg-sidebar-accent text-sidebar-accent-foreground shadow-sm"
            : "text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground",
          isCollapsed ? "justify-center px-2" : "",
          "group"
        )}
      >
        {isActive && !isCollapsed && (
          <div className="absolute left-0 top-1/2 -translate-y-1/2 h-5 w-1 rounded-r-full bg-primary" />
        )}
        <Icon className={cn(
          "h-5 w-5 shrink-0 transition-transform duration-200",
          isActive ? "text-sidebar-accent-foreground" : "text-sidebar-foreground/60",
          "group-hover:scale-110"
        )} />
        {!isCollapsed && <span className="truncate">{label}</span>}
      </Link>
    )
  }

  return (
    <AuthGuard>
      <CreateTenantWizard
        open={showCreateWizard}
        onOpenChange={setShowCreateWizard}
        onSuccess={(tenant) => {
          // Navegar al nuevo tenant al crearlo exitosamente
          router.push(`/admin/tenants/detail?id=${tenant.id}`)
        }}
      />

      {/* FS-admin fallback notice */}
      {!hasRefresh && !hideFsNotice && (
        <div className="fixed top-0 left-0 right-0 z-[60] bg-amber-100 text-amber-900 border-b border-amber-200 px-4 py-2 text-xs font-medium">
          <div className="container mx-auto flex items-center justify-center relative">
            <p>Sesión sin refresh (FS-admin). Tu sesión podría expirar.</p>
            <button
              onClick={() => setHideFsNotice(true)}
              className="absolute right-0 p-1 hover:bg-amber-200/50 rounded"
            >
              <X className="h-3 w-3" />
            </button>
          </div>
        </div>
      )}

      <div className="flex flex-col h-screen bg-background font-sans text-foreground">
        {/* Full Width Header - Más compacto y sin usuario */}
        <header className="h-14 border-b border-border bg-background/95 backdrop-blur-lg flex items-center justify-between px-6 sticky top-0 z-50 w-full shadow-sm">
          <div className="flex items-center gap-4">
            <Link href="/admin" className="flex items-center gap-2 font-semibold text-lg tracking-tight hover:opacity-80 transition-all duration-200">
              <div className="h-8 w-8 rounded-lg bg-gradient-to-br from-primary to-primary/80 text-primary-foreground flex items-center justify-center shadow-md">
                <span className="font-bold text-sm">A</span>
              </div>
              <span className="hidden sm:inline">Admin Panel</span>
            </Link>
            <div className="h-6 w-px bg-border mx-1" />
            <OrganizationSelector
              currentOrg={currentTenant}
              onSelect={handleTenantSelect}
            />
          </div>

          <div className="flex-1 max-w-2xl mx-8 hidden md:block">
            <CommandPalette />
          </div>

          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="icon"
              className="text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-all"
            >
              <Bell className="h-5 w-5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              onClick={toggleTheme}
              className="text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-all"
            >
              {theme === "dark" ? (
                <Sun className="h-5 w-5" />
              ) : (
                <Moon className="h-5 w-5" />
              )}
            </Button>
          </div>
        </header>

        <div className="flex flex-1 overflow-hidden">
          {/* Sidebar mejorado con nueva estructura */}
          <aside
            className={cn(
              "border-r border-sidebar-border bg-sidebar flex flex-col h-full overflow-hidden hidden md:flex transition-all duration-300",
              isCollapsed ? "w-16" : "w-64"
            )}
          >
            {/* Collapse Toggle */}
            <div className={cn(
              "flex items-center py-3 px-3 border-b border-sidebar-border",
              isCollapsed ? "justify-center" : "justify-end"
            )}>
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 hover:bg-sidebar-accent transition-colors"
                onClick={() => setIsCollapsed(!isCollapsed)}
                title={isCollapsed ? "Expandir" : "Colapsar"}
              >
                {isCollapsed ? <ChevronsRight className="h-4 w-4" /> : <ChevronsLeft className="h-4 w-4" />}
              </Button>
            </div>

            {/* Navigation */}
            <div className="flex-1 py-3 px-3 space-y-5 overflow-y-auto custom-scrollbar">
              {/* Overview */}
              <div className="space-y-1">
                {!isCollapsed && (
                  <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                    Overview
                  </h4>
                )}
                <nav className="space-y-0.5">
                  <NavItem href="/admin" icon={LayoutDashboard} label="Dashboard" />
                </nav>
              </div>

              {/* Platform */}
              <div className="space-y-1">
                {!isCollapsed && (
                  <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                    Platform
                  </h4>
                )}
                <nav className="space-y-0.5">
                  <NavItem href="/admin/tenants" icon={Building2} label="Tenants" />
                  <NavItem href="/admin/cluster" icon={Server} label="Cluster" />
                  <NavItem href="/admin/metrics" icon={Activity} label="Metrics" />
                </nav>
              </div>

              {/* Configuration */}
              <div className="space-y-1">
                {!isCollapsed && (
                  <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                    Configuration
                  </h4>
                )}
                <nav className="space-y-0.5">
                  <NavItem href="/admin/keys" icon={Key} label="Signing Keys" />
                  <NavItem href="/admin/logs" icon={FileText} label="Logs" />
                </nav>
              </div>

              {/* Developers */}
              <div className="space-y-1">
                {!isCollapsed && (
                  <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                    Developers
                  </h4>
                )}
                <nav className="space-y-0.5">
                  <NavItem href="/admin/playground" icon={Zap} label="Playground" />
                  <NavItem href="/admin/oidc" icon={Globe2} label="OIDC" />
                </nav>
              </div>

              {/* Tenant Context - Solo visible si hay tenant seleccionado */}
              {currentTenantId && (
                <>
                  {/* Divider */}
                  {!isCollapsed && (
                    <div className="relative py-2">
                      <div className="absolute inset-0 flex items-center px-2">
                        <div className="w-full border-t border-sidebar-border"></div>
                      </div>
                      <div className="relative flex justify-center text-[10px] uppercase">
                        <span className="bg-sidebar px-2 text-muted-foreground font-semibold tracking-wider">
                          {currentTenant?.name || "Tenant"}
                        </span>
                      </div>
                    </div>
                  )}

                  {/* Tenant Overview */}
                  <div className="space-y-1">
                    {!isCollapsed && (
                      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                        Overview
                      </h4>
                    )}
                    <nav className="space-y-0.5">
                      <NavItem href={`/admin/tenants/detail?id=${currentTenantId}`} icon={LayoutDashboard} label="Details" />
                      <NavItem href={`/admin/tenants/settings?id=${currentTenantId}`} icon={Settings} label="Settings" />
                    </nav>
                  </div>

                  {/* Users & Access */}
                  <div className="space-y-1">
                    {!isCollapsed && (
                      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                        Users & Access
                      </h4>
                    )}
                    <nav className="space-y-0.5">
                      <NavItem href={`/admin/users?id=${currentTenantId}`} icon={Users} label="Users" />
                      <NavItem href={`/admin/tenants/sessions?id=${currentTenantId}`} icon={Shapes} label="Sessions" />
                      <NavItem href={`/admin/rbac?id=${currentTenantId}`} icon={Shield} label="Roles & RBAC" />
                      <NavItem href={`/admin/tenants/scopes?id=${currentTenantId}`} icon={Lock} label="Scopes" />
                      <NavItem href={`/admin/tenants/claims?id=${currentTenantId}`} icon={Fingerprint} label="Claims" />
                    </nav>
                  </div>

                  {/* Applications */}
                  <div className="space-y-1">
                    {!isCollapsed && (
                      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                        Applications
                      </h4>
                    )}
                    <nav className="space-y-0.5">
                      <NavItem href={`/admin/tenants/clients?id=${currentTenantId}`} icon={Boxes} label="Clients" />
                      <NavItem href={`/admin/tenants/tokens?id=${currentTenantId}`} icon={ListChecks} label="Tokens" />
                      <NavItem href={`/admin/tenants/consents?id=${currentTenantId}`} icon={FileText} label="Consents" />
                    </nav>
                  </div>

                  {/* Integrations */}
                  <div className="space-y-1">
                    {!isCollapsed && (
                      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                        Integrations
                      </h4>
                    )}
                    <nav className="space-y-0.5">
                      <NavItem href={`/admin/providers?id=${currentTenantId}`} icon={Globe2} label="Social Providers" />
                      <NavItem href={`/admin/tenants/mailing?id=${currentTenantId}`} icon={Mail} label="Mailing" />
                    </nav>
                  </div>

                  {/* Customization */}
                  <div className="space-y-1">
                    {!isCollapsed && (
                      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                        Customization
                      </h4>
                    )}
                    <nav className="space-y-0.5">
                      <NavItem href={`/admin/tenants/forms?id=${currentTenantId}`} icon={LayoutTemplate} label="Forms" />
                      <NavItem href={`/admin/database?id=${currentTenantId}`} icon={Database} label="Storage" />
                    </nav>
                  </div>
                </>
              )}
            </div>

            {/* User Profile - Ahora en el sidebar */}
            <div className="mt-auto border-t border-sidebar-border">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button className={cn(
                    "w-full p-3 flex items-center gap-3 hover:bg-sidebar-accent transition-all group",
                    isCollapsed && "justify-center p-3"
                  )}>
                    <div className="h-9 w-9 rounded-full bg-gradient-to-br from-primary to-primary/80 flex items-center justify-center text-primary-foreground font-bold text-sm shrink-0 shadow-md group-hover:shadow-lg transition-shadow">
                      {user?.email?.charAt(0).toUpperCase() || 'A'}
                    </div>
                    {!isCollapsed && (
                      <div className="flex-1 flex flex-col items-start overflow-hidden">
                        <span className="text-sm font-semibold truncate w-full text-sidebar-foreground">
                          {user?.email?.split('@')[0] || 'Admin'}
                        </span>
                        <span className="text-xs text-muted-foreground truncate w-full">
                          Super Admin
                        </span>
                      </div>
                    )}
                    {!isCollapsed && (
                      <ChevronDown className="h-4 w-4 text-muted-foreground shrink-0" />
                    )}
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" side="top" className="w-56 mb-2">
                  <DropdownMenuLabel className="font-normal">
                    <div className="flex flex-col space-y-1">
                      <p className="text-sm font-medium">{user?.email?.split('@')[0] || 'Admin'}</p>
                      <p className="text-xs text-muted-foreground truncate">{user?.email || 'admin@system'}</p>
                    </div>
                  </DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem className="cursor-pointer">
                    <User className="mr-2 h-4 w-4" />
                    Perfil
                  </DropdownMenuItem>
                  <DropdownMenuItem className="cursor-pointer">
                    <Settings className="mr-2 h-4 w-4" />
                    Configuración
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    className="text-destructive focus:text-destructive cursor-pointer"
                    onClick={handleLogout}
                  >
                    <LogOut className="mr-2 h-4 w-4" />
                    Cerrar Sesión
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </aside>

          {/* Main Content */}
          <main className="flex-1 overflow-y-auto bg-muted/30 p-8 custom-scrollbar">
            <div className="max-w-6xl mx-auto space-y-8 animate-in fade-in duration-500">
              {children}
            </div>
          </main>
        </div>
      </div>
    </AuthGuard>
  )
}
