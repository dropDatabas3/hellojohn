"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { LayoutDashboard, Building2, Shield, Database, Key, Globe, Play, Server, Activity, Layers } from "lucide-react"
import { useI18n } from "@/lib/i18n"
import { cn } from "@/lib/utils"

export function AdminSidebar() {
  const { t } = useI18n()
  const pathname = usePathname()

  const navigation = [
    { name: t("dashboard.title"), href: "/admin", icon: LayoutDashboard },
    { name: t("tenants.title"), href: "/admin/tenants", icon: Building2 },
    { name: t("rbac.title"), href: "/admin/rbac", icon: Shield },
    { name: t("database.title"), href: "/admin/database", icon: Database },
    { name: t("keys.title"), href: "/admin/keys", icon: Key },
    { name: t("oidc.title"), href: "/admin/oidc", icon: Globe },
    { name: t("playground.title"), href: "/admin/playground", icon: Play },
    { name: t("providers.title"), href: "/admin/providers", icon: Layers },
    { name: t("cluster.title"), href: "/admin/cluster", icon: Server },
    { name: t("metrics.title"), href: "/admin/metrics", icon: Activity },
  ]

  return (
    <nav className="space-y-1">
      {navigation.map((item) => {
        const isActive = pathname === item.href || pathname?.startsWith(item.href + "/")
        const Icon = item.icon
        return (
          <Link
            key={item.name}
            href={item.href}
            className={cn(
              "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
              isActive
                ? "bg-primary text-primary-foreground"
                : "text-muted-foreground hover:bg-muted hover:text-foreground",
            )}
          >
            <Icon className="h-5 w-5" />
            {item.name}
          </Link>
        )
      })}
    </nav>
  )
}
