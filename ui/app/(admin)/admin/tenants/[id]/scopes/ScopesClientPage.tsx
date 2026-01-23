"use client"

import { useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useParams, useSearchParams } from "next/navigation"
import { Plus, Search, Trash2, ArrowLeft, Shield } from "lucide-react"
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
import { Label } from "@/components/ui/label"
import { useToast } from "@/hooks/use-toast"
import { Badge } from "@/components/ui/badge"
import Link from "next/link"
import type { Scope, Tenant } from "@/lib/types"
import { Textarea } from "@/components/ui/textarea"

export default function ScopesClientPage() {
  const params = useParams()
  const searchParams = useSearchParams()
  const { t } = useI18n()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const tenantId = (params.id as string) || (searchParams.get("id") as string)
  const [search, setSearch] = useState("")
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [selectedScope, setSelectedScope] = useState<Scope | null>(null)
  const [newScope, setNewScope] = useState<Scope>({
    name: "",
    description: "",
  })

  const { data: tenant } = useQuery({
    queryKey: ["tenant", tenantId],
    enabled: !!tenantId,
    queryFn: () => api.get<Tenant>(`/v2/admin/tenants/${tenantId}`),
  })

  type ScopeRow = Scope & { id?: string; created_at?: string; tenant_id?: string }
  const { data: scopes, isLoading } = useQuery({
    queryKey: ["scopes", tenantId],
    enabled: !!tenantId,
    // backend expects tenant_id
    queryFn: () => api.get<ScopeRow[]>(`/v1/admin/scopes?tenant_id=${tenantId}`),
  })

  const createMutation = useMutation({
    // backend expects body with tenant_id, name, description
    mutationFn: (data: Scope) =>
      api.post<ScopeRow>(`/v1/admin/scopes`, {
        tenant_id: tenantId,
        name: data.name,
        description: data.description || "",
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["scopes", tenantId] })
      setCreateDialogOpen(false)
      setNewScope({ name: "", description: "" })
      toast({
        title: t("scopes.created"),
        description: t("scopes.createdDesc"),
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

  const deleteMutation = useMutation({
    // backend deletes by scope id in path
    mutationFn: (scopeId: string) => api.delete(`/v1/admin/scopes/${scopeId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["scopes", tenantId] })
      setDeleteDialogOpen(false)
      setSelectedScope(null)
      toast({
        title: t("scopes.deleted"),
        description: t("scopes.deletedDesc"),
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

  const filteredScopes = scopes?.filter(
    (scope) =>
      scope.name.toLowerCase().includes(search.toLowerCase()) ||
      scope.description?.toLowerCase().includes(search.toLowerCase()),
  )

  const handleCreate = () => {
    if (!newScope.name) {
      toast({
        title: t("common.error"),
        description: t("scopes.fillRequired"),
        variant: "destructive",
      })
      return
    }

    const scopeRegex = /^[a-z0-9:._-]+$/
    if (!scopeRegex.test(newScope.name)) {
      toast({
        title: t("common.error"),
        description: "El nombre del scope solo puede contener minúsculas, números, ':', '.', '_' y '-'.",
        variant: "destructive",
      })
      return
    }

    createMutation.mutate(newScope)
  }

  const handleDelete = () => {
    if (selectedScope) {
      // prefer id if present, fallback to name (will 404 if not matching)
      // backend requires ID; list returns id
      const anySel = selectedScope as any
      deleteMutation.mutate(anySel.id || selectedScope.name)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="sm" asChild>
            <Link href={`/admin/tenants/detail?id=${tenantId}`}>
              <ArrowLeft className="h-4 w-4" />
            </Link>
          </Button>
          <div>
            <h1 className="text-3xl font-bold">{t("scopes.title")}</h1>
            <p className="text-muted-foreground">
              {tenant?.name} - {t("scopes.pageDescription")}
            </p>
          </div>
        </div>
        <Button onClick={() => setCreateDialogOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t("scopes.create")}
        </Button>
      </div>

      <Card className="p-6">
        <div className="mb-4">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder={t("scopes.search")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9"
            />
          </div>
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center py-8">
            <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("scopes.name")}</TableHead>
                <TableHead>{t("scopes.description")}</TableHead>
                <TableHead>{t("scopes.type")}</TableHead>
                <TableHead className="text-right">{t("common.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredScopes?.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="text-center text-muted-foreground">
                    {t("scopes.noScopes")}
                  </TableCell>
                </TableRow>
              ) : (
                filteredScopes?.map((scope) => (
                  <TableRow key={(scope as any).id || scope.name}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <Shield className="h-4 w-4 text-muted-foreground" />
                        <code className="rounded bg-muted px-2 py-1 text-sm font-medium">{scope.name}</code>
                      </div>
                    </TableCell>
                    <TableCell>{scope.description || "-"}</TableCell>
                    <TableCell>
                      {scope.system ? (
                        <Badge variant="secondary">{t("scopes.system")}</Badge>
                      ) : (
                        <Badge>{t("scopes.custom")}</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setSelectedScope(scope)
                          setDeleteDialogOpen(true)
                        }}
                        disabled={scope.system}
                      >
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        )}
      </Card>

      {/* Create Dialog */}
      <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("scopes.createTitle")}</DialogTitle>
            <DialogDescription>{t("scopes.createDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name">{t("scopes.name")} *</Label>
              <Input
                id="name"
                value={newScope.name}
                onChange={(e) => setNewScope({ ...newScope, name: e.target.value })}
                placeholder="read:users"
              />
              <p className="text-sm text-muted-foreground">{t("scopes.nameHint")}</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="description">{t("scopes.description")}</Label>
              <Textarea
                id="description"
                value={newScope.description}
                onChange={(e) => setNewScope({ ...newScope, description: e.target.value })}
                placeholder={t("scopes.descriptionPlaceholder")}
                rows={3}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleCreate} disabled={createMutation.isPending}>
              {createMutation.isPending ? t("common.creating") : t("common.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("scopes.deleteTitle")}</DialogTitle>
            <DialogDescription>{t("scopes.deleteDescription", { name: selectedScope?.name })}</DialogDescription>
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
