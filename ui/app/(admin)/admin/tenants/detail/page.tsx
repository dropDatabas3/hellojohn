"use client"

import { useSearchParams } from "next/navigation"
import { useQuery } from "@tanstack/react-query"
import Link from "next/link"
import {
  Users, Key, Shield, Settings, Mail, Boxes, FileText,
  Lock, Fingerprint, LayoutTemplate, Globe2, Activity, Clock,
  CheckCircle2, AlertCircle
} from "lucide-react"
import { api } from "@/lib/api"
import type { Tenant } from "@/lib/types"

// Design System Components
import {
  PageShell,
  Card,
  CardContent,
  Badge,
  Button,
  Skeleton,
  EmptyState,
  QuickLinkCard,
  cn,
} from "@/components/ds"

// Stats Card Component — Clay themed with semantic colors
function StatsCard({
  icon: Icon,
  label,
  value,
  variant = "default",
}: Readonly<{
  icon: React.ElementType
  label: string
  value: string | number
  variant?: "info" | "success" | "warning" | "accent" | "default"
}>) {
  const variantStyles = {
    default: "from-muted/30 to-muted/10 border-border/50",
    info: "from-info/15 to-info/5 border-info/30",
    success: "from-success/15 to-success/5 border-success/30",
    warning: "from-warning/15 to-warning/5 border-warning/30",
    accent: "from-accent/15 to-accent/5 border-accent/30",
  }
  const iconStyles = {
    default: "text-muted-foreground",
    info: "text-info",
    success: "text-success",
    warning: "text-warning",
    accent: "text-accent",
  }

  return (
    <Card className={cn(
      "bg-gradient-to-br border transition-all duration-200",
      "hover:-translate-y-0.5 hover:shadow-float",
      variantStyles[variant]
    )}>
      <CardContent className="p-4">
        <div className={cn("flex items-center gap-2", iconStyles[variant])}>
          <Icon className="h-4 w-4" />
          <span className="text-xs font-medium uppercase tracking-wider">{label}</span>
        </div>
        <p className="text-2xl font-bold mt-1 text-foreground">{value}</p>
      </CardContent>
    </Card>
  )
}

// Skeleton Loading State
function TenantDetailSkeleton() {
  return (
    <PageShell>
      {/* Header Skeleton */}
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-4">
          <Skeleton className="h-16 w-16 rounded-xl" />
          <div className="space-y-2">
            <Skeleton className="h-7 w-48" />
            <div className="flex gap-2">
              <Skeleton className="h-5 w-20" />
              <Skeleton className="h-5 w-16" />
            </div>
          </div>
        </div>
        <Skeleton className="h-9 w-24" />
      </div>

      {/* Stats Skeleton */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mt-6">
        {[1, 2, 3, 4].map((i) => (
          <Skeleton key={`stat-skeleton-${i}`} className="h-24 rounded-xl" />
        ))}
      </div>

      {/* Quick Links Skeleton */}
      <div className="space-y-4 mt-6">
        <Skeleton className="h-6 w-32" />
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          {[1, 2, 3, 4, 5, 6].map((i) => (
            <Skeleton key={`link-skeleton-${i}`} className="h-20 rounded-xl" />
          ))}
        </div>
      </div>
    </PageShell>
  )
}

export default function TenantDetailPage() {
  const searchParams = useSearchParams()
  const tenantId = searchParams.get("id")

  const { data: tenant, isLoading, isError } = useQuery({
    queryKey: ["tenant", tenantId],
    queryFn: () => api.get<Tenant>(`/v2/admin/tenants/${tenantId}`),
    enabled: !!tenantId,
  })

  // Loading State
  if (isLoading) {
    return <TenantDetailSkeleton />
  }

  // Error or Not Found State
  if (!tenant || isError) {
    return (
      <PageShell>
        <EmptyState
          icon={<AlertCircle className="h-12 w-12" />}
          title="Tenant not found"
          description="The tenant you're looking for doesn't exist or you don't have access to it."
          action={
            <Button asChild variant="outline">
              <Link href="/admin/tenants">Back to Tenants</Link>
            </Button>
          }
        />
      </PageShell>
    )
  }

  return (
    <PageShell className="animate-in fade-in duration-500">
      {/* Custom Header with Logo */}
      <header className="flex items-start justify-between pb-8">
        <div className="flex items-center gap-4">
          {/* Logo/Avatar */}
          {tenant.settings?.logoUrl ? (
            <img
              src={tenant.settings.logoUrl.startsWith("http") || tenant.settings.logoUrl.startsWith("data:")
                ? tenant.settings.logoUrl
                : `${api.getBaseUrl()}${tenant.settings.logoUrl}`}
              alt={tenant.name}
              className="h-16 w-16 rounded-xl object-cover border bg-background shadow-card"
            />
          ) : (
            <div className="h-16 w-16 rounded-xl bg-accent/20 flex items-center justify-center text-accent font-bold text-2xl border border-accent/30 shadow-card">
              {tenant.name.charAt(0).toUpperCase()}
            </div>
          )}
          <div>
            <h1 className="text-2xl font-bold text-foreground">{tenant.name}</h1>
            <div className="flex items-center gap-2 mt-1">
              <Badge variant="outline" className="font-mono text-xs">
                {tenant.slug}
              </Badge>
              <Badge variant="success" className="text-xs gap-1">
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
      </header>

      {/* Quick Stats Grid */}
      <section>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <StatsCard icon={Users} label="Users" value="-" variant="info" />
          <StatsCard icon={Boxes} label="Clients" value="-" variant="success" />
          <StatsCard icon={Activity} label="Sessions" value="-" variant="warning" />
          <StatsCard
            icon={Clock}
            label="Created"
            value={tenant.createdAt ? new Date(tenant.createdAt).toLocaleDateString() : "-"}
            variant="accent"
          />
        </div>
      </section>

      {/* Quick Access Links */}
      <section className="mt-8 space-y-6">
        <h2 className="text-lg font-semibold text-foreground">Quick Access</h2>

        {/* Identity Section */}
        <div className="space-y-3">
          <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
            Identity
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <QuickLinkCard
              href={`/admin/users?id=${tenantId}`}
              icon={Users}
              title="Users"
              description="Manage user accounts and profiles"
              variant="info"
            />
            <QuickLinkCard
              href={`/admin/tenants/sessions?id=${tenantId}`}
              icon={Activity}
              title="Sessions"
              description="View and manage active sessions"
              variant="success"
            />
            <QuickLinkCard
              href={`/admin/tenants/consents?id=${tenantId}`}
              icon={FileText}
              title="Consents"
              description="User consent management"
              variant="warning"
            />
          </div>
        </div>

        {/* Access Control Section */}
        <div className="space-y-3">
          <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
            Access Control
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <QuickLinkCard
              href={`/admin/rbac?id=${tenantId}`}
              icon={Shield}
              title="RBAC & Roles"
              description="Role-based access control"
              variant="accent"
            />
            <QuickLinkCard
              href={`/admin/tenants/scopes?id=${tenantId}`}
              icon={Lock}
              title="Scopes"
              description="OAuth2 scope definitions"
              variant="info"
            />
            <QuickLinkCard
              href={`/admin/tenants/claims?id=${tenantId}`}
              icon={Fingerprint}
              title="Claims"
              description="Token claims configuration"
              variant="success"
            />
          </div>
        </div>

        {/* Applications Section */}
        <div className="space-y-3">
          <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
            Applications
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <QuickLinkCard
              href={`/admin/tenants/clients?id=${tenantId}`}
              icon={Boxes}
              title="Clients"
              description="OAuth2 client applications"
              variant="warning"
            />
            <QuickLinkCard
              href={`/admin/tenants/tokens?id=${tenantId}`}
              icon={Key}
              title="Tokens"
              description="Token policies and active tokens"
              variant="accent"
            />
            <QuickLinkCard
              href={`/admin/providers?id=${tenantId}`}
              icon={Globe2}
              title="Social Providers"
              description="External identity providers"
              variant="danger"
            />
          </div>
        </div>

        {/* Configuration Section */}
        <div className="space-y-3">
          <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
            Configuration
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <QuickLinkCard
              href={`/admin/tenants/settings?id=${tenantId}`}
              icon={Settings}
              title="Settings"
              description="General tenant configuration"
              variant="default"
            />
            <QuickLinkCard
              href={`/admin/tenants/mailing?id=${tenantId}`}
              icon={Mail}
              title="Mailing"
              description="Email templates and SMTP"
              variant="info"
            />
            <QuickLinkCard
              href={`/admin/tenants/forms?id=${tenantId}`}
              icon={LayoutTemplate}
              title="Forms"
              description="Login and registration forms"
              variant="success"
            />
          </div>
        </div>
      </section>
    </PageShell>
  )
}
