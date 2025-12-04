"use client"

import { useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Search, Settings, Trash2, Eye, MoreHorizontal } from "lucide-react"
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
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Label } from "@/components/ui/label"
import { useToast } from "@/hooks/use-toast"
import { Badge } from "@/components/ui/badge"
import Link from "next/link"
import type { Tenant } from "@/lib/types"

import { Switch } from "@/components/ui/switch"
import { SimpleTooltip } from "@/components/ui/simple-tooltip"

export default function TenantsPage() {
  const { t } = useI18n()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState("")
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [selectedTenant, setSelectedTenant] = useState<Tenant | null>(null)

  const [enableSMTP, setEnableSMTP] = useState(false)
  const [enableUserDB, setEnableUserDB] = useState(false)
  const [enableSocial, setEnableSocial] = useState(false)
  const [newTenant, setNewTenant] = useState<{
    name: string
    slug: string
    display_name: string
    settings?: any
  }>({
    name: "",
    slug: "",
    display_name: "",
    settings: {},
  })

  const { data: tenants, isLoading } = useQuery({
    queryKey: ["tenants"],
    queryFn: () => api.get<Tenant[]>("/v1/admin/tenants"),
  })

  const createMutation = useMutation({
    mutationFn: (data: typeof newTenant) => api.post<Tenant>("/v1/admin/tenants", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tenants"] })
      setCreateDialogOpen(false)
      setNewTenant({ name: "", slug: "", display_name: "", settings: {} })
      setEnableSMTP(false)
      setEnableUserDB(false)
      setEnableSocial(false)
      toast({
        title: t("tenants.created"),
        description: t("tenants.createdDesc"),
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
    mutationFn: (slug: string) => api.delete(`/v1/admin/tenants/${slug}`),
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

  const handleCreate = () => {
    if (!newTenant.name || !newTenant.slug) {
      toast({
        title: t("common.error"),
        description: t("tenants.fillRequired"),
        variant: "destructive",
      })
      return
    }

    const slugRegex = /^[a-z0-9\-]+$/
    if (!slugRegex.test(newTenant.slug)) {
      toast({
        title: t("common.error"),
        description: "El slug solo puede contener letras minúsculas, números y guiones.",
        variant: "destructive",
      })
      return
    }

    createMutation.mutate(newTenant)
  }

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
                  <TableRow key={tenant.id} className="group cursor-pointer hover:bg-muted/40">
                    <TableCell className="font-medium pl-6">
                      <div className="flex items-center gap-3">
                        <div className="h-8 w-8 rounded bg-primary/10 flex items-center justify-center text-primary font-bold text-xs">
                          {tenant.name.charAt(0).toUpperCase()}
                        </div>
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
                          <Button variant="ghost" size="icon" className="h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity data-[state=open]:opacity-100">
                            <MoreHorizontal className="h-4 w-4" />
                            <span className="sr-only">{t("common.actions")}</span>
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="w-48">
                          <DropdownMenuItem asChild>
                            <Link href={`/admin/tenants/detail?id=${tenant.id}`} className="flex w-full cursor-pointer items-center">
                              <Eye className="mr-2 h-4 w-4" />
                              {t("common.details")}
                            </Link>
                          </DropdownMenuItem>
                          <DropdownMenuItem asChild>
                            <Link href={`/admin/tenants/settings?id=${tenant.id}`} className="flex w-full cursor-pointer items-center">
                              <Settings className="mr-2 h-4 w-4" />
                              {t("common.edit")}
                            </Link>
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            className="text-destructive focus:text-destructive cursor-pointer flex w-full items-center"
                            onClick={() => {
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

      {/* Create Dialog */}
      <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-[600px] custom-scrollbar">
          <DialogHeader>
            <DialogTitle>{t("tenants.createTitle")}</DialogTitle>
            <DialogDescription>{t("tenants.createDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name">{t("tenants.name")} *</Label>
              <Input
                id="name"
                value={newTenant.name}
                onChange={(e) => setNewTenant({ ...newTenant, name: e.target.value })}
                placeholder="acme-corp"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="slug">{t("tenants.slug")} *</Label>
              <Input
                id="slug"
                value={newTenant.slug}
                onChange={(e) => setNewTenant({ ...newTenant, slug: e.target.value })}
                placeholder="acme"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="display_name">{t("tenants.displayName")}</Label>
              <Input
                id="display_name"
                value={newTenant.display_name}
                onChange={(e) => setNewTenant({ ...newTenant, display_name: e.target.value })}
                placeholder="ACME Corporation"
              />
            </div>

            <div className="space-y-4 pt-4 border-t">
              <h3 className="font-medium">Advanced Configuration</h3>

              {/* SMTP Toggle */}
              <div className="space-y-4 rounded-md border p-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center">
                    <Label htmlFor="enable_smtp" className="font-medium">SMTP Settings</Label>
                    <SimpleTooltip content="Configure custom SMTP server for sending emails from this tenant." />
                  </div>
                  <Switch
                    id="enable_smtp"
                    checked={enableSMTP}
                    onCheckedChange={setEnableSMTP}
                  />
                </div>

                {enableSMTP && (
                  <div className="grid grid-cols-2 gap-4 pt-2 animate-in fade-in slide-in-from-top-2">
                    <div className="space-y-2">
                      <Label htmlFor="smtp_host">Host</Label>
                      <Input
                        id="smtp_host"
                        value={newTenant.settings?.smtp?.host || ""}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: {
                              ...newTenant.settings,
                              smtp: { ...newTenant.settings?.smtp, host: e.target.value },
                            },
                          })
                        }
                        placeholder="smtp.example.com"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="smtp_port">Port</Label>
                      <Input
                        id="smtp_port"
                        type="number"
                        value={newTenant.settings?.smtp?.port || 587}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: {
                              ...newTenant.settings,
                              smtp: { ...newTenant.settings?.smtp, port: parseInt(e.target.value) },
                            },
                          })
                        }
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="smtp_user">Username</Label>
                      <Input
                        id="smtp_user"
                        value={newTenant.settings?.smtp?.username || ""}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: {
                              ...newTenant.settings,
                              smtp: { ...newTenant.settings?.smtp, username: e.target.value },
                            },
                          })
                        }
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="smtp_pass">Password</Label>
                      <Input
                        id="smtp_pass"
                        type="password"
                        value={newTenant.settings?.smtp?.password || ""}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: {
                              ...newTenant.settings,
                              smtp: { ...newTenant.settings?.smtp, password: e.target.value },
                            },
                          })
                        }
                      />
                    </div>
                    <div className="col-span-2 space-y-2">
                      <Label htmlFor="smtp_from">From Email</Label>
                      <Input
                        id="smtp_from"
                        value={newTenant.settings?.smtp?.fromEmail || ""}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: {
                              ...newTenant.settings,
                              smtp: { ...newTenant.settings?.smtp, fromEmail: e.target.value },
                            },
                          })
                        }
                        placeholder="noreply@example.com"
                      />
                    </div>
                  </div>
                )}
              </div>

              {/* Database Toggle */}
              <div className="space-y-4 rounded-md border p-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center">
                    <Label htmlFor="enable_db" className="font-medium">Database (User Store)</Label>
                    <SimpleTooltip content="Configure a dedicated database for this tenant's users." />
                  </div>
                  <Switch
                    id="enable_db"
                    checked={enableUserDB}
                    onCheckedChange={setEnableUserDB}
                  />
                </div>

                {enableUserDB && (
                  <div className="space-y-2 pt-2 animate-in fade-in slide-in-from-top-2">
                    <Label htmlFor="db_dsn">DSN (Connection String)</Label>
                    <Input
                      id="db_dsn"
                      type="password"
                      value={newTenant.settings?.userDb?.dsn || ""}
                      onChange={(e) =>
                        setNewTenant({
                          ...newTenant,
                          settings: {
                            ...newTenant.settings,
                            userDb: { ...newTenant.settings?.userDb, dsn: e.target.value, driver: "postgres" },
                          },
                        })
                      }
                      placeholder="postgres://user:pass@host:5432/db"
                    />
                  </div>
                )}
              </div>

              {/* Social Providers Toggle */}
              <div className="space-y-4 rounded-md border p-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center">
                    <Label htmlFor="enable_social" className="font-medium">Social Providers</Label>
                    <SimpleTooltip content="Configure social login providers like Google." />
                  </div>
                  <Switch
                    id="enable_social"
                    checked={enableSocial}
                    onCheckedChange={setEnableSocial}
                  />
                </div>

                {enableSocial && (
                  <div className="grid grid-cols-2 gap-4 pt-2 animate-in fade-in slide-in-from-top-2">
                    <div className="col-span-2 font-medium text-sm text-muted-foreground">Google</div>
                    <div className="space-y-2">
                      <Label htmlFor="google_id">Client ID</Label>
                      <Input
                        id="google_id"
                        value={newTenant.settings?.socialProviders?.google?.clientId || ""}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: {
                              ...newTenant.settings,
                              socialProviders: {
                                ...newTenant.settings?.socialProviders,
                                google: { ...newTenant.settings?.socialProviders?.google, clientId: e.target.value },
                              },
                            },
                          })
                        }
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="google_secret">Client Secret</Label>
                      <Input
                        id="google_secret"
                        type="password"
                        value={newTenant.settings?.socialProviders?.google?.clientSecret || ""}
                        onChange={(e) =>
                          setNewTenant({
                            ...newTenant,
                            settings: {
                              ...newTenant.settings,
                              socialProviders: {
                                ...newTenant.settings?.socialProviders,
                                google: { ...newTenant.settings?.socialProviders?.google, clientSecret: e.target.value },
                              },
                            },
                          })
                        }
                      />
                    </div>
                  </div>
                )}
              </div>
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
