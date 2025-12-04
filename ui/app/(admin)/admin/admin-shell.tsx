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
  Cog,
  Boxes,
  Network,
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
  Share2,
  ChevronsLeft,
  ChevronsRight,
  LayoutTemplate
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
    queryFn: () => api.get<Tenant[]>("/v1/admin/tenants"),
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

  const currentTenant = tenants?.find((t) => t.id === currentTenantId || t.slug === currentTenantId)

  const handleTenantSelect = (val: string) => {
    if (val === "+create") {
      router.push("/admin/tenants")
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
          "relative flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
          isActive
            ? "bg-primary/10 text-primary"
            : "text-muted-foreground hover:bg-muted hover:text-foreground",
          isCollapsed && "justify-center px-2"
        )}
      >
        {isActive && (
          <div className="absolute left-0 top-1/2 -translate-y-1/2 h-6 w-1 rounded-r-full bg-[#725deb]" />
        )}
        <Icon className="h-4 w-4 shrink-0" />
        {!isCollapsed && <span>{label}</span>}
      </Link>
    )
  }

  return (
    <AuthGuard>
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
        {/* Full Width Header */}
        <header className="h-16 border-b border-border bg-background/80 backdrop-blur-md flex items-center justify-between px-6 sticky top-0 z-50 w-full">
          <div className="flex items-center gap-4">
            <Link href="/admin" className="flex items-center gap-2 font-semibold text-lg tracking-tight hover:opacity-80 transition-opacity">
              <div className="h-8 w-8 rounded-lg bg-primary text-primary-foreground flex items-center justify-center shadow-sm">
                <span className="font-bold">A</span>
              </div>
              <span>Admin</span>
            </Link>
            <div className="h-6 w-px bg-border mx-2" />
            <OrganizationSelector
              currentOrg={currentTenant}
              onSelect={handleTenantSelect}
            />
          </div>

          <div className="flex-1 max-w-xl mx-8 hidden md:block">
            <CommandPalette />
          </div>

          <div className="flex items-center gap-3">
            <Button variant="ghost" size="icon" className="text-muted-foreground hover:text-foreground">
              <Bell className="h-5 w-5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              onClick={toggleTheme}
              className="text-muted-foreground hover:text-foreground"
            >
              {theme === "dark" ? (
                <Sun className="h-5 w-5" />
              ) : (
                <Moon className="h-5 w-5" />
              )}
            </Button>
            <div className="h-6 w-px bg-border mx-2" />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" className="gap-2 pl-2 pr-3 rounded-full hover:bg-muted h-9">
                  <div className="h-8 w-8 rounded-full bg-primary/10 flex items-center justify-center border border-border">
                    <User className="h-4 w-4" />
                  </div>
                  <div className="flex flex-col items-start text-xs hidden sm:flex">
                    <span className="font-medium max-w-[100px] truncate">{user?.email?.split('@')[0] || 'Admin'}</span>
                  </div>
                  <ChevronDown className="h-3 w-3 text-muted-foreground" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-56">
                <DropdownMenuLabel>My Account</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem>Profile</DropdownMenuItem>
                <DropdownMenuItem>Settings</DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem className="text-destructive focus:text-destructive cursor-pointer" onClick={handleLogout}>
                  <LogOut className="mr-2 h-4 w-4" />
                  Log out
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </header>

        <div className="flex flex-1 overflow-hidden">
          {/* Sidebar below header */}
          <aside
            className={cn(
              "border-r border-border bg-sidebar flex flex-col pb-4 h-full overflow-y-auto hidden md:flex transition-all duration-300 custom-scrollbar",
              isCollapsed ? "w-16" : "w-64"
            )}
          >
            <div className="flex-1 py-2 px-3 space-y-6">

              {/* Global Section */}
              <div className="space-y-1">
                <div className={cn("flex items-center mb-2", isCollapsed ? "justify-center" : "justify-between px-2")}>
                  {!isCollapsed && (
                    <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                      Global
                    </h4>
                  )}
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6"
                    onClick={() => setIsCollapsed(!isCollapsed)}
                    title={isCollapsed ? "Expand Sidebar" : "Collapse Sidebar"}
                  >
                    {isCollapsed ? <ChevronsRight className="h-4 w-4" /> : <ChevronsLeft className="h-4 w-4" />}
                  </Button>
                </div>
                <nav className="space-y-1">
                  <NavItem href="/admin" icon={LayoutDashboard} label="Dashboard" />
                  <NavItem href="/admin/tenants" icon={Building2} label="Tenants" />
                  <NavItem href="/admin/logs" icon={FileText} label="System Logs" />
                </nav>
              </div>

              {/* Tenant Context Section - Only visible if tenant selected */}
              {currentTenantId && (
                <>
                  <div className="space-y-1">
                    {!isCollapsed && (
                      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                        {currentTenant?.name || "Tenant"}
                      </h4>
                    )}
                    <nav className="space-y-1">
                      <NavItem href={`/admin/tenants/detail?id=${currentTenantId}`} icon={LayoutDashboard} label="Overview" />
                      <NavItem href={`/admin/tenants/settings?id=${currentTenantId}`} icon={Settings} label="Settings" />
                      <NavItem href={`/admin/database?id=${currentTenantId}`} icon={Database} label="Storage & Cache" />
                    </nav>
                  </div>

                  <div className="space-y-1">
                    {!isCollapsed && (
                      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                        Identity
                      </h4>
                    )}
                    <nav className="space-y-1">
                      <NavItem href={`/admin/tenants/users?id=${currentTenantId}`} icon={Users} label="Users" />
                      <NavItem href={`/admin/tenants/sessions?id=${currentTenantId}`} icon={Shapes} label="Sessions" />
                      <NavItem href={`/admin/tenants/consents?id=${currentTenantId}`} icon={FileText} label="Consents" />
                    </nav>
                  </div>

                  <div className="space-y-1">
                    {!isCollapsed && (
                      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                        Access Control
                      </h4>
                    )}
                    <nav className="space-y-1">
                      <NavItem href={`/admin/rbac?id=${currentTenantId}`} icon={Shield} label="RBAC & Roles" />
                      <NavItem href={`/admin/tenants/scopes?id=${currentTenantId}`} icon={Lock} label="Scopes" />
                      <NavItem href={`/admin/tenants/claims?id=${currentTenantId}`} icon={Fingerprint} label="Claims" />
                    </nav>
                  </div>

                  <div className="space-y-1">
                    {!isCollapsed && (
                      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                        Applications
                      </h4>
                    )}
                    <nav className="space-y-1">
                      <NavItem href={`/admin/tenants/clients?id=${currentTenantId}`} icon={Boxes} label="Clients" />
                      <NavItem href={`/admin/tenants/tokens?id=${currentTenantId}`} icon={ListChecks} label="Tokens" />
                    </nav>
                  </div>

                  <div className="space-y-1">
                    {!isCollapsed && (
                      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                        Experience
                      </h4>
                    )}
                    <nav className="space-y-1">
                      <NavItem href={`/admin/tenants/forms?id=${currentTenantId}`} icon={LayoutTemplate} label="Forms" />
                    </nav>
                  </div>

                  <div className="space-y-1">
                    {!isCollapsed && (
                      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                        Federation
                      </h4>
                    )}
                    <nav className="space-y-1">
                      <NavItem href={`/admin/providers?id=${currentTenantId}`} icon={Globe2} label="Social Providers" />
                    </nav>
                  </div>
                </>
              )}
            </div>

            <div className="px-3 mt-auto">
              <div className={cn(
                "bg-muted/50 rounded-lg p-3 flex items-center gap-3 border border-border/50",
                isCollapsed && "justify-center p-2"
              )}>
                <div className="h-8 w-8 rounded-full bg-primary flex items-center justify-center text-primary-foreground font-bold text-xs shrink-0">
                  {user?.email?.charAt(0).toUpperCase() || 'A'}
                </div>
                {!isCollapsed && (
                  <div className="flex flex-col overflow-hidden">
                    <span className="text-sm font-medium truncate">{user?.email || 'Admin User'}</span>
                    <span className="text-xs text-muted-foreground truncate">
                      Super Admin
                    </span>
                  </div>
                )}
              </div>
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
