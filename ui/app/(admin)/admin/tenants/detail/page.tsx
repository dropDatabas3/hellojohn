"use client"

import { useSearchParams } from "next/navigation"
import { useQuery } from "@tanstack/react-query"
import Link from "next/link"
import {
  Building2, Users, Key, Shield, Settings, Mail, Boxes, FileText,
  Lock, Fingerprint, LayoutTemplate, Globe2, ArrowRight, Loader2,
  Activity, Clock, CheckCircle2, AlertCircle
} from "lucide-react"
import { api } from "@/lib/api"
import { cn } from "@/lib/utils"
import type { Tenant } from "@/lib/types"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

// Quick link card component
function QuickLinkCard({
  href,
  icon: Icon,
  title,
  description,
  color = "zinc"
}: {
  href: string
  icon: React.ElementType
  title: string
  description: string
  color?: "zinc" | "blue" | "emerald" | "amber" | "purple" | "rose"
}) {
  const colorClasses = {
    zinc: "from-zinc-500/10 to-zinc-500/5 border-zinc-500/20 group-hover:border-zinc-500/40",
    blue: "from-blue-500/10 to-blue-500/5 border-blue-500/20 group-hover:border-blue-500/40",
    emerald: "from-emerald-500/10 to-emerald-500/5 border-emerald-500/20 group-hover:border-emerald-500/40",
    amber: "from-amber-500/10 to-amber-500/5 border-amber-500/20 group-hover:border-amber-500/40",
    purple: "from-purple-500/10 to-purple-500/5 border-purple-500/20 group-hover:border-purple-500/40",
    rose: "from-rose-500/10 to-rose-500/5 border-rose-500/20 group-hover:border-rose-500/40",
  }
  const iconColors = {
    zinc: "text-zinc-500",
    blue: "text-blue-500",
    emerald: "text-emerald-500",
    amber: "text-amber-500",
    purple: "text-purple-500",
    rose: "text-rose-500",
  }

  return (
    <Link href={href} className="group">
      <Card className={cn(
        "h-full transition-all duration-200 bg-gradient-to-br border",
        colorClasses[color]
      )}>
        <CardContent className="p-4 flex items-start gap-3">
          <div className={cn("p-2 rounded-lg bg-background/50", iconColors[color])}>
            <Icon className="h-5 w-5" />
          </div>
          <div className="flex-1 min-w-0">
            <h3 className="font-medium text-sm flex items-center gap-2">
              {title}
              <ArrowRight className="h-3 w-3 opacity-0 -translate-x-2 group-hover:opacity-100 group-hover:translate-x-0 transition-all" />
            </h3>
            <p className="text-xs text-muted-foreground mt-0.5 line-clamp-2">{description}</p>
          </div>
        </CardContent>
      </Card>
    </Link>
  )
}

export default function TenantDetailPage() {
  const searchParams = useSearchParams()
  const tenantId = searchParams.get("id")

  const { data: tenant, isLoading } = useQuery({
    queryKey: ["tenant", tenantId],
    queryFn: () => api.get<Tenant>(`/v2/admin/tenants/${tenantId}`),
    enabled: !!tenantId,
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (!tenant) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[400px] gap-4">
        <AlertCircle className="h-12 w-12 text-muted-foreground" />
        <p className="text-muted-foreground">Tenant not found</p>
        <Button asChild variant="outline">
          <Link href="/admin/tenants">Back to Tenants</Link>
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-in fade-in duration-500">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-4">
          {tenant.settings?.logoUrl ? (
            <img
              src={tenant.settings.logoUrl.startsWith("http") || tenant.settings.logoUrl.startsWith("data:")
                ? tenant.settings.logoUrl
                : `${api.getBaseUrl()}${tenant.settings.logoUrl}`}
              alt={tenant.name}
              className="h-16 w-16 rounded-xl object-cover border bg-background"
            />
          ) : (
            <div className="h-16 w-16 rounded-xl bg-primary/10 flex items-center justify-center text-primary font-bold text-2xl border">
              {tenant.name.charAt(0).toUpperCase()}
            </div>
          )}
          <div>
            <h1 className="text-2xl font-bold">{tenant.name}</h1>
            <div className="flex items-center gap-2 mt-1">
              <Badge variant="outline" className="font-mono text-xs">
                {tenant.slug}
              </Badge>
              <Badge variant="secondary" className="text-xs gap-1">
                <CheckCircle2 className="h-3 w-3" />
                Active
              </Badge>
            </div>
          </div>
        </div>
        <Button asChild variant="outline" size="sm">
          <Link href={`/admin/tenants/settings?id=${tenantId}`}>
            <Settings className="h-4 w-4 mr-2" />
            Settings
          </Link>
        </Button>
      </div>

      {/* Quick Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <Card className="bg-gradient-to-br from-blue-500/10 to-blue-500/5 border-blue-500/20">
          <CardContent className="p-4">
            <div className="flex items-center gap-2 text-blue-500">
              <Users className="h-4 w-4" />
              <span className="text-xs font-medium uppercase tracking-wider">Users</span>
            </div>
            <p className="text-2xl font-bold mt-1">-</p>
          </CardContent>
        </Card>
        <Card className="bg-gradient-to-br from-emerald-500/10 to-emerald-500/5 border-emerald-500/20">
          <CardContent className="p-4">
            <div className="flex items-center gap-2 text-emerald-500">
              <Boxes className="h-4 w-4" />
              <span className="text-xs font-medium uppercase tracking-wider">Clients</span>
            </div>
            <p className="text-2xl font-bold mt-1">-</p>
          </CardContent>
        </Card>
        <Card className="bg-gradient-to-br from-amber-500/10 to-amber-500/5 border-amber-500/20">
          <CardContent className="p-4">
            <div className="flex items-center gap-2 text-amber-500">
              <Activity className="h-4 w-4" />
              <span className="text-xs font-medium uppercase tracking-wider">Sessions</span>
            </div>
            <p className="text-2xl font-bold mt-1">-</p>
          </CardContent>
        </Card>
        <Card className="bg-gradient-to-br from-purple-500/10 to-purple-500/5 border-purple-500/20">
          <CardContent className="p-4">
            <div className="flex items-center gap-2 text-purple-500">
              <Clock className="h-4 w-4" />
              <span className="text-xs font-medium uppercase tracking-wider">Created</span>
            </div>
            <p className="text-sm font-medium mt-1">
              {tenant.createdAt ? new Date(tenant.createdAt).toLocaleDateString() : "-"}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Quick Links */}
      <div className="space-y-4">
        <h2 className="text-lg font-semibold">Quick Access</h2>
        
        <div className="space-y-2">
          <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Identity</h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <QuickLinkCard
              href={`/admin/users?id=${tenantId}`}
              icon={Users}
              title="Users"
              description="Manage user accounts and profiles"
              color="blue"
            />
            <QuickLinkCard
              href={`/admin/tenants/sessions?id=${tenantId}`}
              icon={Activity}
              title="Sessions"
              description="View and manage active sessions"
              color="emerald"
            />
            <QuickLinkCard
              href={`/admin/tenants/consents?id=${tenantId}`}
              icon={FileText}
              title="Consents"
              description="User consent management"
              color="amber"
            />
          </div>
        </div>

        <div className="space-y-2">
          <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Access Control</h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <QuickLinkCard
              href={`/admin/rbac?id=${tenantId}`}
              icon={Shield}
              title="RBAC & Roles"
              description="Role-based access control"
              color="purple"
            />
            <QuickLinkCard
              href={`/admin/tenants/scopes?id=${tenantId}`}
              icon={Lock}
              title="Scopes"
              description="OAuth2 scope definitions"
              color="blue"
            />
            <QuickLinkCard
              href={`/admin/tenants/claims?id=${tenantId}`}
              icon={Fingerprint}
              title="Claims"
              description="Token claims configuration"
              color="emerald"
            />
          </div>
        </div>

        <div className="space-y-2">
          <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Applications</h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <QuickLinkCard
              href={`/admin/tenants/clients?id=${tenantId}`}
              icon={Boxes}
              title="Clients"
              description="OAuth2 client applications"
              color="amber"
            />
            <QuickLinkCard
              href={`/admin/tenants/tokens?id=${tenantId}`}
              icon={Key}
              title="Tokens"
              description="Token policies and active tokens"
              color="purple"
            />
            <QuickLinkCard
              href={`/admin/providers?id=${tenantId}`}
              icon={Globe2}
              title="Social Providers"
              description="External identity providers"
              color="rose"
            />
          </div>
        </div>

        <div className="space-y-2">
          <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Configuration</h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <QuickLinkCard
              href={`/admin/tenants/settings?id=${tenantId}`}
              icon={Settings}
              title="Settings"
              description="General tenant configuration"
              color="zinc"
            />
            <QuickLinkCard
              href={`/admin/tenants/mailing?id=${tenantId}`}
              icon={Mail}
              title="Mailing"
              description="Email templates and SMTP"
              color="blue"
            />
            <QuickLinkCard
              href={`/admin/tenants/forms?id=${tenantId}`}
              icon={LayoutTemplate}
              title="Forms"
              description="Login and registration forms"
              color="emerald"
            />
          </div>
        </div>
      </div>
    </div>
  )
}
