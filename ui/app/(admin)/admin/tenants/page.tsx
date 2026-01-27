"use client"

import { useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Search, Settings, Trash2, MoreHorizontal } from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useToast } from "@/hooks/use-toast"
import { Badge } from "@/components/ui/badge"
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

  const { data: tenants, isLoading } = useQuery({
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
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t("tenants.title")}</h1>
          <p className="text-muted-foreground">{t("tenants.description")}</p>
        </div>
        <Button onClick={() => setCreateDialogOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t("tenants.create")}
        </Button>
      </div>

      <Card>
        <div className="p-4 border-b flex items-center justify-between bg-muted/30">
          <div className="relative max-w-sm w-full">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder={t("tenants.search")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9 bg-background"
            />
          </div>
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <div className="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent" />
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="pl-6">{t("tenants.name")}</TableHead>
                <TableHead>{t("tenants.slug")}</TableHead>
                <TableHead className="text-right pr-6">{t("common.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredTenants?.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={3} className="text-center py-8 text-muted-foreground">
                    {t("tenants.noTenants")}
                  </TableCell>
                </TableRow>
              ) : (
                filteredTenants?.map((tenant) => (
                  <TableRow
                    key={tenant.id}
                    className="group cursor-pointer hover:bg-muted/40"
                    onClick={() => router.push(`/admin/tenants/detail?id=${tenant.id}`)}
                  >
                    <TableCell className="font-medium pl-6">
                      <div className="flex items-center gap-3">
                        {tenant.settings?.logoUrl ? (
                          <img
                            src={tenant.settings.logoUrl.startsWith("http") || tenant.settings.logoUrl.startsWith("data:")
                              ? tenant.settings.logoUrl
                              : `${api.getBaseUrl()}${tenant.settings.logoUrl}`}
                            alt={tenant.name}
                            className="h-10 w-10 p-1 rounded object-cover border bg-slate-100"
                          />
                        ) : (
                          <div className="h-10 w-10 rounded bg-slate-100 flex items-center justify-center text-slate-700 font-bold text-xm">
                            {tenant.name.charAt(0).toUpperCase()}
                          </div>
                        )}
                        {tenant.name}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="font-mono text-xs font-normal text-muted-foreground">
                        {tenant.slug}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right pr-6">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity data-[state=open]:opacity-100"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <MoreHorizontal className="h-4 w-4" />
                            <span className="sr-only">{t("common.actions")}</span>
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
                            className="text-destructive focus:text-destructive cursor-pointer flex w-full items-center"
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
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        )}
      </Card>

      {/* Create Wizard Dialog */}
      <CreateTenantWizard
        open={createDialogOpen}
        onOpenChange={setCreateDialogOpen}
      />

      {/* Delete Dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("tenants.deleteTitle")}</DialogTitle>
            <DialogDescription>{t("tenants.deleteDescription", { name: selectedTenant?.name })}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button variant="destructive" onClick={handleDelete} disabled={deleteMutation.isPending}>
              {deleteMutation.isPending ? t("common.deleting") : t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
