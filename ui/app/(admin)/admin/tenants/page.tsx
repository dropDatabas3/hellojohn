"use client"

import { useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Search, Settings, Trash2, MoreHorizontal, Building2 } from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import {
  PageShell,
  PageHeader,
  Section,
  Card,
  Button,
  Input,
  Badge,
  Skeleton,
  EmptyState,
  InlineAlert,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ds"
import { useToast } from "@/hooks/use-toast"
import Link from "next/link"
import { useRouter } from "next/navigation"
import type { Tenant } from "@/lib/types"
import { CreateTenantWizard } from "@/components/tenant/CreateTenantWizard"

export default function TenantsPage() {
  const { t } = useI18n()
  const { toast } = useToast()
  const router = useRouter()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState("")
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [selectedTenant, setSelectedTenant] = useState<Tenant | null>(null)

  const {
    data: tenants,
    isLoading,
    isError,
    error,
    refetch,
  } = useQuery({
    queryKey: ["tenants"],
    queryFn: () => api.get<Tenant[]>("/v2/admin/tenants"),
  })

  const deleteMutation = useMutation({
    mutationFn: (slug: string) => api.delete(`/v2/admin/tenants/${slug}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tenants"] })
      setDeleteDialogOpen(false)
      setSelectedTenant(null)
      toast({
        title: t("tenants.deleted"),
        description: t("tenants.deletedDesc"),
        variant: "success",
      })
    },
    onError: (error: any) => {
      toast({
        title: t("common.error"),
        description: error.message,
        variant: "destructive",
      })
    },
  })

  const filteredTenants = tenants?.filter(
    (tenant) =>
      tenant.name.toLowerCase().includes(search.toLowerCase()) ||
      tenant.slug.toLowerCase().includes(search.toLowerCase()),
  )

  const handleDelete = () => {
    if (selectedTenant) {
      deleteMutation.mutate(selectedTenant.slug)
    }
  }

  return (
    <PageShell>
      <PageHeader
        title={t("tenants.title")}
        description={t("tenants.description")}
        actions={
          <Button onClick={() => setCreateDialogOpen(true)} leftIcon={<Plus className="h-4 w-4" />}>
            {t("tenants.create")}
          </Button>
        }
      />

      {/* Error State */}
      {isError && (
        <InlineAlert
          variant="destructive"
          title={t("common.error")}
          description={(error as any)?.message || "Failed to load tenants"}
          action={
            <Button size="sm" variant="secondary" onClick={() => refetch()}>
              Retry
            </Button>
          }
        />
      )}

      <Section>
        <Card>
          {/* Search Bar */}
          <div className="p-4 border-b border-border">
            <div className="relative max-w-sm">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" aria-hidden="true" />
              <Input
                placeholder={t("tenants.search")}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-9"
                aria-label="Search tenants"
              />
            </div>
          </div>

          {/* Loading State */}
          {isLoading ? (
            <div className="divide-y divide-border">
              {[...Array(5)].map((_, i) => (
                <div key={i} className="p-6 flex items-center justify-between">
                  <div className="flex items-center gap-3 flex-1">
                    <Skeleton className="h-10 w-10 rounded" />
                    <div className="space-y-2 flex-1">
                      <Skeleton className="h-5 w-48" />
                      <Skeleton className="h-4 w-32" />
                    </div>
                  </div>
                  <Skeleton className="h-6 w-24" />
                  <Skeleton className="h-8 w-8 rounded ml-4" />
                </div>
              ))}
            </div>
          ) : filteredTenants && filteredTenants.length > 0 ? (
            /* Tenant List */
            <div className="divide-y divide-border">
              {filteredTenants.map((tenant) => (
                <div
                  key={tenant.id}
                  className="group p-6 flex items-center justify-between transition-all duration-200 hover:bg-surface cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background"
                  onClick={() => router.push(`/admin/tenants/detail?id=${tenant.id}`)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault()
                      router.push(`/admin/tenants/detail?id=${tenant.id}`)
                    }
                  }}
                >
                  <div className="flex items-center gap-3 flex-1">
                    {/* Logo/Avatar */}
                    {tenant.settings?.logoUrl ? (
                      <img
                        src={
                          tenant.settings.logoUrl.startsWith("http") ||
                          tenant.settings.logoUrl.startsWith("data:")
                            ? tenant.settings.logoUrl
                            : `${api.getBaseUrl()}${tenant.settings.logoUrl}`
                        }
                        alt={tenant.name}
                        className="h-10 w-10 p-1 rounded object-cover border border-border bg-muted"
                      />
                    ) : (
                      <div className="h-10 w-10 rounded bg-muted flex items-center justify-center text-foreground font-bold text-sm">
                        {tenant.name.charAt(0).toUpperCase()}
                      </div>
                    )}
                    {/* Name & Slug */}
                    <div>
                      <p className="font-medium text-foreground">{tenant.name}</p>
                      <Badge variant="outline" className="font-mono text-xs mt-1">
                        {tenant.slug}
                      </Badge>
                    </div>
                  </div>

                  {/* Actions Dropdown */}
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="opacity-0 group-hover:opacity-100 transition-opacity data-[state=open]:opacity-100"
                        onClick={(e) => e.stopPropagation()}
                        aria-label={`Actions for ${tenant.name}`}
                      >
                        <MoreHorizontal className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-48">
                      <DropdownMenuItem asChild>
                        <Link
                          href={`/admin/tenants/settings?id=${tenant.id}`}
                          className="flex w-full cursor-pointer items-center"
                          onClick={(e) => e.stopPropagation()}
                        >
                          <Settings className="mr-2 h-4 w-4" />
                          {t("common.edit")}
                        </Link>
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        className="text-danger focus:text-danger cursor-pointer"
                        onClick={(e) => {
                          e.stopPropagation()
                          setSelectedTenant(tenant)
                          setDeleteDialogOpen(true)
                        }}
                      >
                        <Trash2 className="mr-2 h-4 w-4" />
                        {t("common.delete")}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              ))}
            </div>
          ) : (
            /* Empty State */
            <EmptyState
              icon={<Building2 className="w-12 h-12" />}
              title={search ? t("tenants.noResults") : t("tenants.noTenants")}
              description={
                search
                  ? `No tenants found matching "${search}"`
                  : "Get started by creating your first tenant organization"
              }
              action={
                !search && (
                  <Button onClick={() => setCreateDialogOpen(true)} leftIcon={<Plus className="h-4 w-4" />}>
                    {t("tenants.create")}
                  </Button>
                )
              }
            />
          )}
        </Card>
      </Section>

      {/* Create Wizard Dialog */}
      <CreateTenantWizard open={createDialogOpen} onOpenChange={setCreateDialogOpen} />

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("tenants.deleteTitle")}</DialogTitle>
            <DialogDescription>
              {t("tenants.deleteDescription", { name: selectedTenant?.name })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDeleteDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button variant="danger" onClick={handleDelete} loading={deleteMutation.isPending}>
              {t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageShell>
  )
}
