// Client-only Admin Shell extracted from layout to prevent SSR hydration mismatches

"use client"

import type React from "react"
import { useState, useRef } from "react"
import { useTheme } from "next-themes"

import { AuthGuard } from "@/components/auth/auth-guard"
import { Button } from "@/components/ds"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ds"
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
} from "@/components/ds"
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
      <SelectTrigger className={cn(
        "group h-10 min-w-[180px] px-4 gap-2 font-medium rounded-xl",
        "transition-all duration-300 ease-out",
        // Clean light background
        "bg-white dark:bg-slate-800",
        "hover:bg-slate-50 dark:hover:bg-slate-750",
        // Subtle border
        "border border-slate-200 dark:border-slate-700",
        "hover:border-slate-300 dark:hover:border-slate-600",
        "focus:ring-2 focus:ring-primary/20 focus:ring-offset-0 focus:border-primary/40",
        // Refined shadow
        "shadow-sm hover:shadow-md",
        "dark:shadow-[0_1px_3px_rgba(0,0,0,0.3)]",
        "dark:hover:shadow-[0_4px_12px_rgba(0,0,0,0.4)]",
        // Text
        "text-slate-800 dark:text-slate-100",
        // Micro interaction
        "hover:-translate-y-[0.5px] active:translate-y-0",
        "[&>svg:last-child]:text-slate-400 [&>svg:last-child]:h-4 [&>svg:last-child]:w-4"
      )}>
        <span className="truncate text-sm">{currentOrg?.name || "Select organization"}</span>
      </SelectTrigger>
      <SelectContent className={cn(
        "min-w-[220px] overflow-hidden rounded-xl p-1",
        // Solid premium background
        "bg-slate-900 dark:bg-slate-900",
        // Border
        "border border-slate-700/50 dark:border-slate-700",
        // Premium shadow
        "shadow-[0_4px_6px_rgba(0,0,0,0.15),0_10px_20px_rgba(0,0,0,0.2),0_20px_40px_rgba(0,0,0,0.25)]",
        "dark:shadow-[0_4px_6px_rgba(0,0,0,0.3),0_10px_20px_rgba(0,0,0,0.4),0_20px_40px_rgba(0,0,0,0.4)]",
        // Animation
        "animate-in fade-in-0 zoom-in-95 duration-200"
      )}>
        {/* Header */}
        <div className="px-3 py-1.5">
          <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider">
            Organizations
          </p>
        </div>

        {/* Tenant list - scrollable, max 3 visible */}
        <div className={cn(
          "space-y-0.5 max-h-[148px] overflow-y-auto",
          // Modern dark scrollbar
          "[&::-webkit-scrollbar]:w-1.5",
          "[&::-webkit-scrollbar-track]:bg-slate-800/50 [&::-webkit-scrollbar-track]:rounded-full",
          "[&::-webkit-scrollbar-thumb]:bg-slate-600 [&::-webkit-scrollbar-thumb]:rounded-full",
          "[&::-webkit-scrollbar-thumb]:hover:bg-slate-500"
        )}>
          {tenants
            ?.slice()
            .sort((a, b) => a.name.localeCompare(b.name))
            .map((t) => (
              <SelectItem
                key={t.slug}
                value={t.slug}
                className={cn(
                  "relative cursor-pointer rounded-lg pl-7 pr-3 py-1.5",
                  "text-sm font-medium",
                  "transition-all duration-200 ease-out",
                  // Base text - white with shadow
                  "text-white",
                  // Hover/focus states - subtle violet bg
                  "hover:bg-violet-500/30",
                  "focus:bg-violet-500/30",
                  "data-[highlighted]:bg-violet-500/30",
                  // Hover text enhancement
                  "hover:text-white hover:[text-shadow:0_1px_8px_rgba(139,92,246,0.6),0_0_20px_rgba(139,92,246,0.4)]",
                  "data-[highlighted]:text-white data-[highlighted]:[text-shadow:0_1px_8px_rgba(139,92,246,0.6),0_0_20px_rgba(139,92,246,0.4)]",
                  // Selected state - stronger violet gradient
                  "data-[state=checked]:bg-gradient-to-r data-[state=checked]:from-violet-600 data-[state=checked]:to-purple-600",
                  "data-[state=checked]:text-white data-[state=checked]:[text-shadow:0_1px_3px_rgba(0,0,0,0.3)]",
                  // Fix indicator position - left side
                  "[&>span:first-child]:absolute [&>span:first-child]:left-2 [&>span:first-child]:top-1/2 [&>span:first-child]:-translate-y-1/2",
                  "[&>span:first-child]:text-white [&>span:first-child]:drop-shadow-sm"
                )}
              >
                <div className="flex flex-col min-w-0">
                  <span className="truncate">{t.name}</span>
                  <span className="text-[10px] text-slate-400 truncate">{t.slug}</span>
                </div>
              </SelectItem>
            ))}
        </div>

        {/* Divider */}
        <div className="my-1.5 mx-2 h-px bg-slate-700/50" />

        {/* Create new */}
        <SelectItem
          value="+create"
          className={cn(
            "cursor-pointer rounded-lg px-3 py-1.5",
            "text-sm font-semibold",
            "text-emerald-400 [text-shadow:0_1px_2px_rgba(0,0,0,0.3)]",
            "hover:bg-emerald-500/20 hover:text-emerald-300",
            "focus:bg-emerald-500/20 focus:text-emerald-300",
            "data-[highlighted]:bg-emerald-500/20 data-[highlighted]:text-emerald-300",
            "transition-colors duration-150",
            "[&>span:first-child]:hidden"
          )}
        >
          <div className="flex items-center gap-2">
            <span className="text-base leading-none">+</span>
            <span>Create Organization</span>
          </div>
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

  // Use next-themes for theme switching (canonical provider per UI_UNIFICATION_STRATEGY)
  const { theme, setTheme } = useTheme()
  const toggleTheme = () => setTheme(theme === "dark" ? "light" : "dark")

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

    // Known sub-routes that are NOT tenant IDs
    const knownSubRoutes = [
      "detail", "settings", "sessions", "rbac", "scopes", "claims",
      "clients", "tokens", "consents", "providers", "mailing", "database"
    ]

    // Fallback: extract from legacy dynamic path /admin/tenants/{id}/...
    // but exclude known sub-routes
    const m = pathname?.match(/\/admin\/tenants\/([^\/]+)/)
    if (m?.[1] && !knownSubRoutes.includes(m[1])) {
      return m[1]
    }

    return undefined
  })()

  const currentTenant = tenants?.find((t) => t.id === currentTenantId || t.slug === currentTenantId)

  const handleTenantSelect = (val: string) => {
    if (val === "+create") {
      setShowCreateWizard(true)
    } else {
      const t = tenants?.find((t) => t.slug === val)
      if (t) {
        router.push(`/admin/tenants/${t.id}/detail`)
      }
    }
  }

  const [isCollapsed, setIsCollapsed] = useState(true)
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false)
  const hoverTimeoutRef = useRef<NodeJS.Timeout | null>(null)

  // Sidebar hover handlers with delay to prevent accidental triggers
  const handleSidebarMouseEnter = () => {
    // Clear any pending collapse timeout
    if (hoverTimeoutRef.current) {
      clearTimeout(hoverTimeoutRef.current)
    }
    // Delay expansion by 250ms to avoid accidental triggers
    hoverTimeoutRef.current = setTimeout(() => {
      setIsCollapsed(false)
    }, 300)
  }

  const handleSidebarMouseLeave = () => {
    // Clear any pending expansion timeout
    if (hoverTimeoutRef.current) {
      clearTimeout(hoverTimeoutRef.current)
    }
    // Collapse immediately if user menu is not open
    if (!isUserMenuOpen) {
      setIsCollapsed(true)
    }
  }

  const handleUserMenuChange = (open: boolean) => {
    setIsUserMenuOpen(open)
    // Cuando el menú se cierra, también colapsar el sidebar
    if (!open) {
      setIsCollapsed(true)
    }
  }

  const NavItem = ({ href, icon: Icon, label, active }: { href: string; icon: any; label: string; active?: boolean }) => {
    const hrefPath = href.split('?')[0]
    const isExactMatch = pathname === hrefPath
    const isActive = active || isExactMatch

    return (
      <Link
        href={href}
        title={isCollapsed ? label : undefined}
        className={cn(
          "relative flex items-center rounded-xl text-[13px] font-medium",
          "transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)]",
          "group min-h-[40px] min-w-[42px] justify-start px-3 mb-1 py-2.5 gap-3",
          // Active state - soft violet highlight
          isActive && [
            "text-foreground",
            "bg-gradient-to-r from-violet-500/12 via-purple-500/8 to-violet-400/3",
            "shadow-[0_1px_3px_rgba(139,92,246,0.06),0_2px_6px_rgba(139,92,246,0.04),inset_0_1px_0_rgba(255,255,255,0.3)]",
            "dark:shadow-[0_1px_3px_rgba(139,92,246,0.1),0_2px_6px_rgba(139,92,246,0.06),inset_0_1px_0_rgba(255,255,255,0.05)]",
            "ring-1 ring-violet-500/10 dark:ring-violet-400/12",
          ],
          // Non-active base state
          !isActive && "text-muted-foreground",
          // Hover effects (non-active only)
          !isActive && [
            "hover:text-foreground",
            // Violet background gradient on hover - extends more to the right
            "hover:bg-gradient-to-r hover:from-violet-500/12 hover:via-purple-500/8 hover:to-violet-400/3",
            // Subtle 3D lift and shadow
            "hover:-translate-y-[1px]",
            "hover:shadow-[0_2px_4px_rgba(139,92,246,0.08),0_4px_8px_rgba(139,92,246,0.04),inset_0_1px_0_rgba(255,255,255,0.3)]",
            "dark:hover:shadow-[0_2px_4px_rgba(139,92,246,0.12),0_4px_8px_rgba(139,92,246,0.06),inset_0_1px_0_rgba(255,255,255,0.05)]",
            // Violet border hint
            "hover:ring-1 hover:ring-violet-500/10 dark:hover:ring-violet-500/15",
          ]
        )}
      >
        {isActive && (
          <div className={cn(
            "absolute left-0 top-1/2 -translate-y-1/2 h-5 w-[3px] rounded-full bg-gradient-to-b from-violet-500 to-purple-500 transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)]",
            "shadow-[0_0_4px_rgba(139,92,246,0.25)]",
            isCollapsed ? "opacity-0" : "opacity-100"
          )} />
        )}
        <div className={cn(
          "flex items-center justify-center shrink-0 w-5",
          "transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)]",
          // Collapsed + Active state - white icon with subtle 3D effect
          isCollapsed && isActive && [
            "drop-shadow-[0_1px_2px_rgba(0,0,0,0.25)]",
            "[filter:drop-shadow(0_0_3px_rgba(255,255,255,0.3))_drop-shadow(0_1px_2px_rgba(0,0,0,0.15))]",
          ]
        )}>
          <Icon className={cn(
            "h-[20px] w-[20px] transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)]",
            // Collapsed + Active - white icon
            isCollapsed && isActive
              ? "text-white dark:text-white"
              : isActive
                ? "text-violet-600 dark:text-violet-400"
                : "text-muted-foreground/70 group-hover:text-violet-600 dark:group-hover:text-violet-400",
            // Subtle scale and rotate on hover (non-active, expanded only)
            !isActive && !isCollapsed && "group-hover:scale-110 group-hover:-rotate-3"
          )} />
        </div>
        <span className={cn(
          "truncate overflow-hidden whitespace-nowrap transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)]",
          isCollapsed ? "opacity-0 w-0" : "opacity-100 w-auto"
        )}>
          {label}
        </span>
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
          router.push(`/admin/tenants/${tenant.id}/detail`)
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

      <div className="relative flex h-screen bg-background font-sans text-foreground p-4 pr-0 overflow-hidden">
        {/* ═══════════════════════════════════════════════════════════════════
            DECORATIVE BACKGROUND LAYER - Premium ambient effects with diagonal flow
        ═══════════════════════════════════════════════════════════════════ */}
        <div className="pointer-events-none absolute inset-0 z-0 overflow-hidden">
          {/* ══════ TOP-LEFT CORNER EMPHASIS (Sidebar + Header intersection) ══════ */}
          {/* Primary hero orb - intense top left */}
          <div className="absolute -top-32 -left-32 h-[600px] w-[600px] rounded-full bg-gradient-to-br from-primary/10 via-primary/5 to-transparent blur-[100px] dark:from-primary/8 dark:via-primary/4" />

          {/* Secondary accent layer - top left reinforcement */}
          <div className="absolute top-0 left-0 h-[400px] w-[400px] rounded-full bg-gradient-to-br from-accent/8 via-info/4 to-transparent blur-[80px] dark:from-accent/5 dark:via-info/3" />

          {/* Soft highlight - top left corner glow */}
          <div className="absolute -top-10 left-20 h-[300px] w-[500px] rounded-full bg-gradient-to-r from-white/5 via-primary/4 to-transparent blur-[60px] dark:from-white/2 dark:via-primary/3" />

          {/* ══════ DIAGONAL DARK BAND (Top-left to Bottom-right center) ══════ */}
          {/* Diagonal shadow/depth band */}
          <div
            className="absolute inset-0 bg-gradient-to-br from-transparent via-slate-950/[0.08] to-transparent dark:via-black/25"
            style={{
              maskImage: 'linear-gradient(135deg, transparent 15%, black 35%, black 65%, transparent 85%)',
              WebkitMaskImage: 'linear-gradient(135deg, transparent 15%, black 35%, black 65%, transparent 85%)'
            }}
          />

          {/* Center depth - darker diagonal stripe */}
          <div className="absolute top-1/4 left-1/4 right-1/4 bottom-1/4 bg-gradient-to-br from-slate-900/[0.04] via-slate-950/[0.08] to-slate-900/[0.04] blur-[40px] dark:from-black/10 dark:via-black/20 dark:to-black/10 rotate-[-10deg] scale-150" />

          {/* ══════ BOTTOM-RIGHT CORNER EMPHASIS (Violet/Purple + Grid) ══════ */}
          {/* Primary violet orb - bottom right */}
          <div className="absolute -bottom-20 -right-20 h-[550px] w-[550px] rounded-full bg-gradient-to-tl from-violet-200/15 via-purple-100/8 to-transparent blur-[100px] dark:from-violet-500/6 dark:via-purple-500/3" />

          {/* Secondary purple layer */}
          <div className="absolute bottom-10 right-10 h-[350px] w-[350px] rounded-full bg-gradient-to-tl from-purple-100/10 via-indigo-100/5 to-transparent blur-[70px] dark:from-purple-500/5 dark:via-indigo-500/3" />

          {/* Soft violet/lavender highlight - bottom right */}
          <div className="absolute bottom-0 right-0 h-[400px] w-[600px] rounded-full bg-gradient-to-tl from-violet-50/10 via-fuchsia-50/5 to-transparent blur-[80px] dark:from-violet-400/4 dark:via-fuchsia-500/2" />

          {/* ══════ SUPPORTING ELEMENTS ══════ */}
          {/* Subtle top-right accent (balance) */}
          <div className="absolute -top-20 right-20 h-[300px] w-[300px] rounded-full bg-gradient-to-bl from-info/5 via-transparent to-transparent blur-3xl dark:from-info/4" />

          {/* Subtle bottom-left (balance) */}
          <div className="absolute bottom-20 -left-10 h-[250px] w-[250px] rounded-full bg-gradient-to-tr from-success/4 via-transparent to-transparent blur-3xl dark:from-success/3" />

          {/* ══════ TEXTURE OVERLAYS ══════ */}
          {/* Noise texture overlay for depth */}
          <div className="absolute inset-0 opacity-[0.02] dark:opacity-[0.04]" style={{ backgroundImage: 'url("data:image/svg+xml,%3Csvg viewBox=\'0 0 256 256\' xmlns=\'http://www.w3.org/2000/svg\'%3E%3Cfilter id=\'noise\'%3E%3CfeTurbulence type=\'fractalNoise\' baseFrequency=\'0.65\' numOctaves=\'3\' stitchTiles=\'stitch\'/%3E%3C/filter%3E%3Crect width=\'100%25\' height=\'100%25\' filter=\'url(%23noise)\'/%3E%3C/svg%3E")' }} />

          {/* Grid pattern - violet colored, enhanced visibility in bottom-right */}
          <div
            className="absolute inset-0"
            style={{
              backgroundImage: 'linear-gradient(to right, rgba(139,92,246,0.10) 1px, transparent 1px), linear-gradient(to bottom, rgba(139,92,246,0.10) 1px, transparent 1px)',
              backgroundSize: '48px 48px',
              maskImage: 'linear-gradient(135deg, transparent 0%, transparent 30%, rgba(0,0,0,0.3) 50%, rgba(0,0,0,0.8) 70%, black 100%)',
              WebkitMaskImage: 'linear-gradient(135deg, transparent 0%, transparent 30%, rgba(0,0,0,0.3) 50%, rgba(0,0,0,0.8) 70%, black 100%)'
            }}
          />

          {/* Secondary finer grid - violet, bottom right only */}
          <div
            className="absolute inset-0"
            style={{
              backgroundImage: 'linear-gradient(to right, rgba(167,139,250,0.08) 1px, transparent 1px), linear-gradient(to bottom, rgba(167,139,250,0.08) 1px, transparent 1px)',
              backgroundSize: '16px 16px',
              maskImage: 'linear-gradient(135deg, transparent 0%, transparent 60%, rgba(0,0,0,0.5) 80%, black 100%)',
              WebkitMaskImage: 'linear-gradient(135deg, transparent 0%, transparent 60%, rgba(0,0,0,0.5) 80%, black 100%)'
            }}
          />
        </div>

        {/* Sidebar - Auto-expand con hover (con delay para evitar disparos accidentales) */}
        <aside
          onMouseEnter={handleSidebarMouseEnter}
          onMouseLeave={handleSidebarMouseLeave}
          className={cn(
            "relative z-10 flex flex-col overflow-hidden hidden md:flex transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)]",
            "rounded-r-2xl",
            // Glass background - allows decorative layer to show through
            "bg-white/40 dark:bg-slate-900/50",
            "backdrop-blur-2xl backdrop-saturate-150",
            "border border-l-0 border-white/60 dark:border-white/[0.08]",
            "shadow-[0_2px_4px_rgba(0,0,0,0.04),0_4px_8px_rgba(0,0,0,0.04),0_8px_16px_rgba(0,0,0,0.04),0_16px_32px_rgba(0,0,0,0.04)]",
            "dark:shadow-[0_2px_4px_rgba(0,0,0,0.2),0_4px_8px_rgba(0,0,0,0.2),0_8px_16px_rgba(0,0,0,0.15),0_16px_32px_rgba(0,0,0,0.1)]",
            isCollapsed ? "w-16" : "w-64"
          )}
        >
          <div className="relative z-10 flex flex-col h-full">
            {/* Logo */}
            <div className="flex items-center h-16 gap-3 transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)] justify-start px-4">
              <Link
                href="/admin"
                className="flex items-center gap-3 font-semibold text-base tracking-tight hover:opacity-70 transition-all duration-500"
              >
                <div className="h-9 w-9 rounded-xl bg-gradient-to-br from-primary to-primary/80 text-primary-foreground flex items-center justify-center shadow-lg shadow-primary/20 shrink-0 transition-transform duration-200 hover:scale-105">
                  <span className="font-bold text-sm">HJ</span>
                </div>
                <span className={cn(
                  "text-foreground/90 transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)] whitespace-nowrap overflow-hidden",
                  isCollapsed ? "opacity-0 w-0" : "opacity-100 w-auto"
                )}>
                  HelloJohn
                </span>
              </Link>
            </div>

            {/* Navigation */}
            <div className={cn(
              "flex-1 py-4 px-3 space-y-6 transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)]",
              isCollapsed ? "overflow-hidden" : "overflow-y-auto custom-scrollbar"
            )}>
              {/* Overview */}
              <div className="space-y-1">
                <h4 className={cn(
                  "text-[10px] font-semibold text-muted-foreground/60 uppercase tracking-widest px-3",
                  "transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)] overflow-hidden whitespace-nowrap",
                  isCollapsed ? "opacity-0 max-h-0 mb-0" : "opacity-100 max-h-6 mb-3"
                )}>
                  Overview
                </h4>
                <nav className="space-y-0.5">
                  <NavItem href="/admin" icon={LayoutDashboard} label="Dashboard" />
                  <NavItem href="/admin/tenants" icon={Building2} label="Organizaciones" />
                </nav>
              </div>

              {/* Configuration */}
              <div className="space-y-1">
                <h4 className={cn(
                  "text-[10px] font-semibold text-muted-foreground/60 uppercase tracking-widest px-3",
                  "transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)] overflow-hidden whitespace-nowrap",
                  isCollapsed ? "opacity-0 max-h-0 mb-0" : "opacity-100 max-h-6 mb-3"
                )}>
                  Configuration
                </h4>
                <nav className="space-y-0.5">
                  <NavItem href="/admin/cluster" icon={Server} label="Cluster" />
                  <NavItem href="/admin/keys" icon={Key} label="Signing Keys" />
                </nav>
              </div>

              {/* Developers */}
              <div className="space-y-1">
                <h4 className={cn(
                  "text-[10px] font-semibold text-muted-foreground/60 uppercase tracking-widest px-3",
                  "transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)] overflow-hidden whitespace-nowrap",
                  isCollapsed ? "opacity-0 max-h-0 mb-0" : "opacity-100 max-h-6 mb-3"
                )}>
                  Developers
                </h4>
                <nav className="space-y-0.5">
                  <NavItem href="/admin/playground" icon={Zap} label="Playground" />
                  <NavItem href="/admin/oidc" icon={Globe2} label="OIDC" />
                </nav>
              </div>

              {/* Tenant Context - Solo visible si hay tenant seleccionado */}
              {currentTenantId && (
                <>
                  {/* Divider */}
                  <div className={cn(
                    "relative py-2 transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)] overflow-hidden",
                    isCollapsed ? "opacity-0 max-h-0 py-0" : "opacity-100 max-h-12"
                  )}>
                    <div className="absolute inset-0 flex items-center px-2">
                      <div className="w-full border-t border-sidebar-border"></div>
                    </div>
                    <div className="relative flex justify-center text-[10px] uppercase">
                      <span className="bg-sidebar px-2 text-muted-foreground font-semibold tracking-wider">
                        {currentTenant?.name || "Tenant"}
                      </span>
                    </div>
                  </div>

                  {/* Tenant Overview */}
                  <div className="space-y-1">
                    <h4 className={cn(
                      "text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2",
                      "transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)] overflow-hidden whitespace-nowrap",
                      isCollapsed ? "opacity-0 max-h-0 mb-0" : "opacity-100 max-h-6 mb-2"
                    )}>
                      Overview
                    </h4>
                    <nav className="space-y-0.5">
                      <NavItem href={`/admin/tenants/${currentTenantId}/detail`} icon={LayoutDashboard} label="Details" />
                      <NavItem href={`/admin/tenants/${currentTenantId}/settings`} icon={Settings} label="Settings" />
                    </nav>
                  </div>

                  {/* Users & Access */}
                  <div className="space-y-1">
                    <h4 className={cn(
                      "text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2",
                      "transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)] overflow-hidden whitespace-nowrap",
                      isCollapsed ? "opacity-0 max-h-0 mb-0" : "opacity-100 max-h-6 mb-2"
                    )}>
                      Users & Access
                    </h4>
                    <nav className="space-y-0.5">
                      <NavItem href={`/admin/tenants/${currentTenantId}/users`} icon={Users} label="Users" />
                      <NavItem href={`/admin/tenants/${currentTenantId}/sessions`} icon={Shapes} label="Sessions" />
                      <NavItem href={`/admin/tenants/${currentTenantId}/rbac`} icon={Shield} label="Roles & RBAC" />
                      <NavItem href={`/admin/tenants/${currentTenantId}/scopes`} icon={Lock} label="Scopes" />
                      <NavItem href={`/admin/tenants/${currentTenantId}/claims`} icon={Fingerprint} label="Claims" />
                    </nav>
                  </div>

                  {/* Applications */}
                  <div className="space-y-1">
                    <h4 className={cn(
                      "text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2",
                      "transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)] overflow-hidden whitespace-nowrap",
                      isCollapsed ? "opacity-0 max-h-0 mb-0" : "opacity-100 max-h-6 mb-2"
                    )}>
                      Applications
                    </h4>
                    <nav className="space-y-0.5">
                      <NavItem href={`/admin/tenants/${currentTenantId}/clients`} icon={Boxes} label="Clients" />
                      <NavItem href={`/admin/tenants/${currentTenantId}/tokens`} icon={ListChecks} label="Tokens" />
                      <NavItem href={`/admin/tenants/${currentTenantId}/consents`} icon={FileText} label="Consents" />
                    </nav>
                  </div>

                  {/* Integrations */}
                  <div className="space-y-1">
                    <h4 className={cn(
                      "text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2",
                      "transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)] overflow-hidden whitespace-nowrap",
                      isCollapsed ? "opacity-0 max-h-0 mb-0" : "opacity-100 max-h-6 mb-2"
                    )}>
                      Integrations
                    </h4>
                    <nav className="space-y-0.5">
                      <NavItem href={`/admin/tenants/${currentTenantId}/providers`} icon={Globe2} label="Social Providers" />
                      <NavItem href={`/admin/tenants/${currentTenantId}/mailing`} icon={Mail} label="Mailing" />
                      <NavItem href={`/admin/tenants/${currentTenantId}/database`} icon={Database} label="Storage" />
                    </nav>
                  </div>
                </>
              )}
            </div>

            {/* User Profile - Ahora en el sidebar */}
            <div className="mt-auto border-t border-sidebar-border">
              <DropdownMenu onOpenChange={handleUserMenuChange}>
                <DropdownMenuTrigger asChild>
                  <button className="w-full flex items-center gap-3 p-3 hover:bg-sidebar-accent transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)] group justify-start">
                    <div className="h-9 w-9 rounded-full bg-gradient-to-br from-primary to-primary/80 flex items-center justify-center text-primary-foreground font-bold text-sm shrink-0 shadow-md group-hover:shadow-lg transition-shadow">
                      {user?.email?.charAt(0).toUpperCase() || 'A'}
                    </div>
                    <div className={cn(
                      "flex-1 flex flex-col items-start overflow-hidden transition-all duration-500",
                      isCollapsed ? "w-0 opacity-0" : "w-auto opacity-100"
                    )}>
                      <span className="text-sm font-semibold truncate w-full text-sidebar-foreground whitespace-nowrap">
                        {user?.email?.split('@')[0] || 'Admin'}
                      </span>
                      <span className="text-xs text-muted-foreground truncate w-full whitespace-nowrap">
                        Super Admin
                      </span>
                    </div>
                    <ChevronDown className={cn(
                      "h-4 w-4 text-muted-foreground shrink-0 transition-all duration-500",
                      isCollapsed ? "w-0 opacity-0" : "w-4 opacity-100"
                    )} />
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
                  <DropdownMenuItem className="cursor-pointer" onClick={toggleTheme}>
                    {theme === "dark" ? (
                      <>
                        <Sun className="mr-2 h-4 w-4" />
                        Tema Claro
                      </>
                    ) : (
                      <>
                        <Moon className="mr-2 h-4 w-4" />
                        Tema Oscuro
                      </>
                    )}
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
          </div>
        </aside>

        {/* Main Content Area con Header */}
        <div className="flex-1 flex flex-col overflow-hidden bg-transparent pr-4">
          {/* Header - Modern elevated card */}
          <header
            className={cn(
              "relative z-10 ml-4 h-[64px] rounded-2xl px-5",
              "flex items-center gap-5",
              // Solid elevated background
              "bg-gradient-to-b from-white to-slate-50/90",
              "dark:from-slate-800/95 dark:to-slate-900/90",
              // Clean border
              "border border-slate-200/60 dark:border-slate-700/40",
              // Premium 3D shadow stack - more pronounced
              "shadow-[0_1px_2px_rgba(0,0,0,0.03),0_2px_4px_rgba(0,0,0,0.03),0_4px_8px_rgba(0,0,0,0.03),0_8px_16px_rgba(0,0,0,0.03),0_16px_32px_rgba(0,0,0,0.02)]",
              "dark:shadow-[0_2px_4px_rgba(0,0,0,0.2),0_4px_8px_rgba(0,0,0,0.15),0_8px_16px_rgba(0,0,0,0.1),0_16px_32px_rgba(0,0,0,0.1)]",
              // Micro interaction
              "transform-gpu transition-all duration-300 ease-out",
              "hover:shadow-[0_2px_4px_rgba(0,0,0,0.04),0_4px_8px_rgba(0,0,0,0.04),0_8px_16px_rgba(0,0,0,0.04),0_16px_32px_rgba(0,0,0,0.03),0_24px_48px_rgba(0,0,0,0.02)]",
              "dark:hover:shadow-[0_4px_8px_rgba(0,0,0,0.25),0_8px_16px_rgba(0,0,0,0.2),0_16px_32px_rgba(0,0,0,0.15),0_32px_64px_rgba(0,0,0,0.1)]"
            )}
          >
            {/* Subtle inner highlight */}
            <div className="pointer-events-none absolute inset-x-0 top-0 h-px rounded-t-2xl bg-gradient-to-r from-transparent via-white/80 to-transparent dark:via-white/10" />

            {/* Content */}
            <div className="relative flex w-full items-center justify-between">
              {/* Organization Selector - Left */}
              <div className="flex items-center gap-3 shrink-0">
                <OrganizationSelector
                  currentOrg={currentTenant}
                  onSelect={handleTenantSelect}
                />
              </div>

              {/* Command Palette - Absolutely centered on screen */}
              <div className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 w-full max-w-[280px] pointer-events-auto">
                <CommandPalette />
              </div>

              {/* Actions - Right */}
              <div className="flex items-center gap-2 shrink-0">
                <Button
                  variant="ghost"
                  size="sm"
                  className={cn(
                    "h-9 w-9 rounded-xl",
                    // Solid background with gradient
                    "bg-gradient-to-b from-slate-50 to-slate-100/80",
                    "dark:from-slate-700 dark:to-slate-800/90",
                    "hover:from-slate-100 hover:to-slate-150/80",
                    "dark:hover:from-slate-600 dark:hover:to-slate-700/90",
                    // Border
                    "border border-slate-200/80 dark:border-slate-600/50",
                    "hover:border-slate-300 dark:hover:border-slate-500",
                    // Text
                    "text-slate-500 dark:text-slate-400",
                    "hover:text-slate-700 dark:hover:text-slate-200",
                    // 3D shadows
                    "shadow-[0_1px_2px_rgba(0,0,0,0.04),0_2px_4px_rgba(0,0,0,0.03),inset_0_1px_0_rgba(255,255,255,0.8)]",
                    "dark:shadow-[0_1px_2px_rgba(0,0,0,0.2),inset_0_1px_0_rgba(255,255,255,0.05)]",
                    "hover:shadow-[0_2px_4px_rgba(0,0,0,0.06),0_4px_8px_rgba(0,0,0,0.03),inset_0_1px_0_rgba(255,255,255,0.9)]",
                    "dark:hover:shadow-[0_2px_4px_rgba(0,0,0,0.3),inset_0_1px_0_rgba(255,255,255,0.08)]",
                    // Animation
                    "transition-all duration-200",
                    "hover:-translate-y-0.5 active:translate-y-0 active:scale-95"
                  )}
                >
                  <Bell className="h-[17px] w-[17px]" />
                </Button>
              </div>
            </div>
          </header>


          {/* Main Content - Glass panel with 3D depth */}
          <main
            className={cn(
              "flex-1 overflow-y-auto no-scrollbar",
              "relative m-6 ml-8 mt-8 rounded-2xl",
              // Glass background - semi-transparent with blur
              "bg-white/40 dark:bg-slate-900/30",
              "backdrop-blur-md backdrop-saturate-150",
              // Border system for definition
              "border border-white/60 dark:border-white/[0.08]",
              "ring-1 ring-black/[0.03] dark:ring-white/[0.03]",
              // 3D shadow stack for depth/volume
              "shadow-[0_2px_4px_rgba(0,0,0,0.02),0_4px_8px_rgba(0,0,0,0.03),0_8px_16px_rgba(0,0,0,0.04),0_16px_32px_rgba(0,0,0,0.05),0_32px_64px_rgba(0,0,0,0.03)]",
              "dark:shadow-[0_2px_4px_rgba(0,0,0,0.1),0_4px_8px_rgba(0,0,0,0.15),0_8px_16px_rgba(0,0,0,0.15),0_16px_32px_rgba(0,0,0,0.1),0_32px_64px_rgba(0,0,0,0.1)]",
              // Inner highlight for raised effect
              "before:absolute before:inset-0 before:rounded-2xl before:pointer-events-none",
              "before:bg-gradient-to-b before:from-white/30 before:via-transparent before:to-transparent",
              "dark:before:from-white/[0.05] dark:before:via-transparent dark:before:to-black/10"
            )}
          >
            {/* Top edge highlight */}
            <div className="pointer-events-none absolute inset-x-0 top-0 h-px rounded-t-2xl bg-gradient-to-r from-transparent via-white/70 to-transparent dark:via-white/15" />

            {/* Content wrapper */}
            <div className="relative p-8 pb-4 space-y-8 animate-in fade-in duration-500">
              {children}
            </div>
          </main>
        </div>
      </div>
    </AuthGuard>
  )
}
