"use client"

import { useState, useEffect, useMemo, useCallback } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useSearchParams, useParams, useRouter } from "next/navigation"
import {
  Plus,
  Search,
  Settings,
  Trash2,
  MoreHorizontal,
  UsersIcon,
  UserIcon,
  RefreshCw,
  Download,
  FileJson,
  FileSpreadsheet,
  Ban,
  X,
  ShieldCheck,
  ShieldAlert,
  MailCheck,
  MailX,
  Database,
  ArrowRight,
  LayoutList,
  Sliders,
  Info as InfoIcon,
  Lock,
  Unlock,
  Eye,
  Edit,
  CheckCircle2,
  XCircle,
  Calendar,
  MapPin,
  Loader2,
} from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { useAuthStore } from "@/lib/auth-store"
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
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  Label,
  Switch,
  Checkbox,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Textarea,
} from "@/components/ds"
import { useToast } from "@/hooks/use-toast"
import { cn } from "@/lib/utils"

// ----- Types -----
interface UserFieldDefinition {
  name: string
  type: string // text, int, boolean, date, phone, country
  required?: boolean
  unique?: boolean
  indexed?: boolean
  description?: string
}

interface TenantSettings {
  userFields?: UserFieldDefinition[]
}

interface UserType {
  id: string
  tenant_id?: string
  email: string
  email_verified: boolean
  name?: string
  given_name?: string
  family_name?: string
  picture?: string
  locale?: string
  metadata?: Record<string, any>
  custom_fields?: Record<string, any>
  created_at: string
  updated_at?: string
  disabled_at?: string
  disabled_until?: string
  disabled_reason?: string
  last_login_at?: string
  login_count?: number
  failed_login_count?: number
  source_client_id?: string
}

interface UsersListResponse {
  users: UserType[]
  total_count: number
  page: number
  page_size: number
}

// ----- Main Component -----
export default function UsersPage() {
  const searchParams = useSearchParams()
  const tenantIdParam = searchParams.get("id")
  const params = useParams()
  const tenantId = tenantIdParam || (params?.id as string)

  const { t } = useI18n()
  const [activeTab, setActiveTab] = useState("list")

  if (!tenantId) {
    return (
      <PageShell>
        <InlineAlert
          variant="destructive"
          title={t("common.error")}
          description="Tenant ID missing."
        />
      </PageShell>
    )
  }

  return (
    <PageShell>
      <PageHeader
        title="Usuarios"
        description="Gestión completa de usuarios, campos personalizados y actividad."
      />

      <InlineAlert
        variant="info"
        title="Gestión de Identidades"
        description="Administra todos los usuarios registrados en este tenant. Puedes crear, editar, bloquear y eliminar usuarios. Los campos personalizados se configuran en la pestaña 'Campos' y aplican al formulario de registro."
      />

      <Section>
        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList className="grid w-full max-w-md grid-cols-2">
            <TabsTrigger value="list" className="flex items-center gap-2">
              <LayoutList className="h-4 w-4" />
              Lista de Usuarios
            </TabsTrigger>
            <TabsTrigger value="fields" className="flex items-center gap-2">
              <Sliders className="h-4 w-4" />
              Campos Personalizados
            </TabsTrigger>
          </TabsList>

          <TabsContent value="list">
            <UsersList tenantId={tenantId} />
          </TabsContent>

          <TabsContent value="fields">
            <UserFieldsSettings tenantId={tenantId} />
          </TabsContent>
        </Tabs>
      </Section>
    </PageShell>
  )
}

// ----------------------------------------------------------------------
// COMPONENT: Users List with Pagination, Bulk Actions, Edit
// ----------------------------------------------------------------------

function UsersList({ tenantId }: { tenantId: string }) {
  const { token } = useAuthStore()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const router = useRouter()

  // State
  const [search, setSearch] = useState("")
  const [debouncedSearch, setDebouncedSearch] = useState("")
  const [selectedUser, setSelectedUser] = useState<UserType | null>(null)
  const [isDetailsOpen, setIsDetailsOpen] = useState(false)
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [isEditOpen, setIsEditOpen] = useState(false)
  const [editUser, setEditUser] = useState<UserType | null>(null)
  const [blockUser, setBlockUser] = useState<UserType | null>(null)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [isBulkActionOpen, setIsBulkActionOpen] = useState(false)
  const [bulkAction, setBulkAction] = useState<"block" | "delete" | null>(null)

  // Pagination
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(25)

  // Debounce search
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(search)
      setPage(1) // Reset to first page on search
    }, 300)
    return () => clearTimeout(timer)
  }, [search])

  // Fetch Users with Pagination
  const {
    data: usersData,
    isLoading,
    error: usersError,
    refetch,
  } = useQuery<UsersListResponse, any>({
    queryKey: ["users", tenantId, page, pageSize, debouncedSearch],
    queryFn: async () => {
      const params = new URLSearchParams({
        page: page.toString(),
        page_size: pageSize.toString(),
      })
      if (debouncedSearch) {
        params.append("search", debouncedSearch)
      }
      const response = await api.get<UsersListResponse>(
        `/v2/admin/tenants/${tenantId}/users?${params.toString()}`,
        { headers: { "X-Tenant-ID": tenantId } }
      )
      return response || { users: [], total_count: 0, page: 1, page_size: pageSize }
    },
    enabled: !!tenantId && !!token,
    retry: (failureCount, error) => {
      if (error?.error === "TENANT_NO_DATABASE" || error?.status === 424) return false
      return failureCount < 3
    },
  })

  const users = usersData?.users || []
  const totalCount = usersData?.total_count || 0
  const totalPages = Math.ceil(totalCount / pageSize)

  // Fetch Field Definitions
  const { data: fieldDefs } = useQuery<UserFieldDefinition[]>({
    queryKey: ["user-fields", tenantId],
    queryFn: async () => {
      const tenant = await api.get<any>(`/v2/admin/tenants/${tenantId}`)
      return tenant?.settings?.userFields || []
    },
    enabled: !!tenantId && !!token,
  })

  // Fetch Clients
  const { data: clients } = useQuery<Array<{ client_id: string; name: string; type: string }>>({
    queryKey: ["tenant-clients", tenantId],
    queryFn: async () => {
      const list = await api.get<any[]>(`/v2/admin/clients`, {
        headers: { "X-Tenant-ID": tenantId },
      })
      return (list || []).filter((c: any) => c.type !== "confidential" && c.client_id)
    },
    enabled: !!tenantId && !!token,
  })

  // Mutations
  const createMutation = useMutation({
    mutationFn: async (vars: any) => {
      return api.post(`/v2/admin/tenants/${tenantId}/users`, vars, {
        headers: { "X-Tenant-ID": tenantId },
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
      setIsCreateOpen(false)
      toast({ title: "Usuario creado", variant: "default" })
    },
    onError: (err: Error) => {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    },
  })

  const updateMutation = useMutation({
    mutationFn: async ({ userId, data }: { userId: string; data: any }) => {
      return api.put(`/v2/admin/tenants/${tenantId}/users/${userId}`, data, undefined, {
        headers: { "X-Tenant-ID": tenantId },
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
      setIsEditOpen(false)
      setEditUser(null)
      toast({ title: "Usuario actualizado", variant: "default" })
    },
    onError: (err: Error) => {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (userId: string) => {
      return api.delete(`/v2/admin/tenants/${tenantId}/users/${userId}`, {
        headers: { "X-Tenant-ID": tenantId },
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
      toast({ title: "Usuario eliminado", variant: "default" })
    },
    onError: (err: Error) => {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    },
  })

  const blockMutation = useMutation({
    mutationFn: async ({
      userId,
      reason,
      duration,
    }: {
      userId: string
      reason: string
      duration: string
    }) => {
      return api.post(
        `/v2/admin/tenants/${tenantId}/users/${userId}/disable`,
        { reason, duration },
        { headers: { "X-Tenant-ID": tenantId } }
      )
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
      setBlockUser(null)
      toast({ title: "Usuario bloqueado", variant: "default" })
    },
    onError: (err: Error) => {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    },
  })

  const enableMutation = useMutation({
    mutationFn: async (userId: string) => {
      return api.post(
        `/v2/admin/tenants/${tenantId}/users/${userId}/enable`,
        {},
        {
          headers: { "X-Tenant-ID": tenantId },
        }
      )
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
      toast({ title: "Usuario desbloqueado", variant: "default" })
    },
    onError: (err: Error) => {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    },
  })

  const setEmailVerifiedMutation = useMutation({
    mutationFn: async ({ userId, verified }: { userId: string; verified: boolean }) => {
      return api.post(
        `/v2/admin/tenants/${tenantId}/users/${userId}/set-email-verified`,
        { verified },
        { headers: { "X-Tenant-ID": tenantId } }
      )
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
      toast({ title: "Email verificado", variant: "default" })
    },
    onError: (err: Error) => {
      toast({
        title: "Aviso",
        description: "La verificación manual de email requiere configuración adicional del backend.",
        variant: "default",
      })
    },
  })

  const changePasswordMutation = useMutation({
    mutationFn: async ({ userId, newPassword }: { userId: string; newPassword: string }) => {
      return api.post(
        `/v2/admin/tenants/${tenantId}/users/${userId}/set-password`,
        { password: newPassword },
        { headers: { "X-Tenant-ID": tenantId } }
      )
    },
    onSuccess: () => {
      toast({ title: "Contraseña actualizada", variant: "default" })
    },
    onError: (err: Error) => {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    },
  })

  // Bulk Actions
  const bulkBlockMutation = useMutation({
    mutationFn: async (userIds: string[]) => {
      const results = await Promise.allSettled(
        userIds.map((id) =>
          api.post(
            `/v2/admin/tenants/${tenantId}/users/${id}/disable`,
            { reason: "Bulk action", duration: "" },
            { headers: { "X-Tenant-ID": tenantId } }
          )
        )
      )
      const failed = results.filter((r) => r.status === "rejected").length
      if (failed > 0) throw new Error(`${failed} usuarios no pudieron ser bloqueados`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
      setSelectedIds(new Set())
      setIsBulkActionOpen(false)
      toast({ title: "Usuarios bloqueados", variant: "default" })
    },
    onError: (err: Error) => {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    },
  })

  const bulkDeleteMutation = useMutation({
    mutationFn: async (userIds: string[]) => {
      const results = await Promise.allSettled(
        userIds.map((id) =>
          api.delete(`/v2/admin/tenants/${tenantId}/users/${id}`, {
            headers: { "X-Tenant-ID": tenantId },
          })
        )
      )
      const failed = results.filter((r) => r.status === "rejected").length
      if (failed > 0) throw new Error(`${failed} usuarios no pudieron ser eliminados`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
      setSelectedIds(new Set())
      setIsBulkActionOpen(false)
      toast({ title: "Usuarios eliminados", variant: "default" })
    },
    onError: (err: Error) => {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    },
  })

  // Stats
  const stats = useMemo(() => {
    const active = users.filter((u) => !u.disabled_at && !u.disabled_until).length
    const blocked = users.filter((u) => u.disabled_at || u.disabled_until).length
    const verified = users.filter((u) => u.email_verified).length
    return { active, blocked, verified }
  }, [users])

  // Selection handlers
  const toggleSelect = (id: string) => {
    const newSet = new Set(selectedIds)
    if (newSet.has(id)) {
      newSet.delete(id)
    } else {
      newSet.add(id)
    }
    setSelectedIds(newSet)
  }

  const toggleSelectAll = () => {
    if (selectedIds.size === users.length) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(users.map((u) => u.id)))
    }
  }

  // Export handlers
  const exportUsers = async (format: "json" | "csv") => {
    try {
      const allUsers = await api.get<UsersListResponse>(
        `/v2/admin/tenants/${tenantId}/users?page_size=10000`,
        { headers: { "X-Tenant-ID": tenantId } }
      )
      const data = allUsers?.users || []

      if (format === "json") {
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" })
        const url = URL.createObjectURL(blob)
        const a = document.createElement("a")
        a.href = url
        a.download = `users-${tenantId}-${Date.now()}.json`
        a.click()
        URL.revokeObjectURL(url)
      } else {
        // CSV export (basic implementation)
        const headers = ["id", "email", "email_verified", "created_at"]
        const rows = data.map((u) => [u.id, u.email, u.email_verified, u.created_at])
        const csv = [headers.join(","), ...rows.map((r) => r.join(","))].join("\n")
        const blob = new Blob([csv], { type: "text/csv" })
        const url = URL.createObjectURL(blob)
        const a = document.createElement("a")
        a.href = url
        a.download = `users-${tenantId}-${Date.now()}.csv`
        a.click()
        URL.revokeObjectURL(url)
      }
      toast({ title: "Exportación completa", variant: "default" })
    } catch (err: any) {
      toast({ title: "Error al exportar", description: err.message, variant: "destructive" })
    }
  }

  // Check if tenant has no database configured
  const isNoDatabaseError =
    usersError?.error === "TENANT_NO_DATABASE" || usersError?.status === 424

  if (isNoDatabaseError) {
    return (
      <EmptyState
        icon={<Database className="w-12 h-12" />}
        title="Configura tu base de datos"
        description="Conecta una base de datos para comenzar a gestionar los usuarios de este tenant."
        action={
          <Button onClick={() => router.push(`/admin/database?id=${tenantId}`)}>
            <ArrowRight className="mr-2 h-4 w-4" />
            Configurar
          </Button>
        }
      />
    )
  }

  return (
    <div className="space-y-4">
      {/* Stats Row */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <StatCard icon={UsersIcon} label="Total Usuarios" value={totalCount} variant="default" />
        <StatCard icon={ShieldCheck} label="Activos" value={stats.active} variant="success" />
        <StatCard icon={ShieldAlert} label="Bloqueados" value={stats.blocked} variant="danger" />
        <StatCard icon={MailCheck} label="Verificados" value={stats.verified} variant="success" />
      </div>

      {/* Toolbar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div className="flex items-center gap-2 w-full sm:w-auto">
          <div className="relative flex-1 sm:flex-none">
            <Search
              className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground"
              aria-hidden="true"
            />
            <Input
              placeholder="Buscar por email o ID..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9 h-9 w-full sm:w-[250px] lg:w-[300px]"
              aria-label="Buscar usuarios"
            />
          </div>
          <Button variant="outline" size="sm" className="h-9" onClick={() => refetch()}>
            <RefreshCw className="h-4 w-4" />
          </Button>
        </div>
        <div className="flex items-center gap-2 w-full sm:w-auto justify-end">
          {/* Export Dropdown */}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="sm" className="h-9">
                <Download className="mr-2 h-4 w-4" />
                Exportar
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuLabel>Formato de exportación</DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => exportUsers("json")}>
                <FileJson className="mr-2 h-4 w-4" />
                JSON
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => exportUsers("csv")}>
                <FileSpreadsheet className="mr-2 h-4 w-4" />
                CSV
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <Button className="h-9" onClick={() => setIsCreateOpen(true)}>
            <Plus className="mr-2 h-4 w-4" /> Crear Usuario
          </Button>
        </div>
      </div>

      {/* Bulk Actions Bar */}
      {selectedIds.size > 0 && (
        <div className="flex items-center justify-between p-3 bg-muted/50 rounded-lg border border-border">
          <div className="flex items-center gap-2">
            <Checkbox
              checked={selectedIds.size === users.length}
              onCheckedChange={toggleSelectAll}
              aria-label="Seleccionar todos"
            />
            <span className="text-sm font-medium text-foreground">
              {selectedIds.size} usuario{selectedIds.size !== 1 ? "s" : ""} seleccionado
              {selectedIds.size !== 1 ? "s" : ""}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setBulkAction("block")
                setIsBulkActionOpen(true)
              }}
            >
              <Ban className="mr-2 h-4 w-4" />
              Bloquear
            </Button>
            <Button
              variant="danger"
              size="sm"
              onClick={() => {
                setBulkAction("delete")
                setIsBulkActionOpen(true)
              }}
            >
              <Trash2 className="mr-2 h-4 w-4" />
              Eliminar
            </Button>
            <Button variant="ghost" size="sm" onClick={() => setSelectedIds(new Set())}>
              <X className="mr-2 h-4 w-4" />
              Cancelar
            </Button>
          </div>
        </div>
      )}

      {/* Users List */}
      <Card>
        {isLoading ? (
          <div className="divide-y divide-border">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="p-6 flex items-center justify-between">
                <div className="flex items-center gap-3 flex-1">
                  <Skeleton className="h-10 w-10 rounded-full" />
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
        ) : users.length === 0 ? (
          <EmptyState
            icon={<UserIcon className="w-12 h-12" />}
            title={debouncedSearch ? "No se encontraron usuarios" : "No hay usuarios"}
            description={
              debouncedSearch
                ? `No se encontraron usuarios que coincidan con "${debouncedSearch}"`
                : "Comienza creando tu primer usuario"
            }
            action={
              !debouncedSearch && (
                <Button onClick={() => setIsCreateOpen(true)}>
                  <Plus className="mr-2 h-4 w-4" />
                  Crear Usuario
                </Button>
              )
            }
          />
        ) : (
          <div className="divide-y divide-border">
            {users.map((user) => (
              <UserRow
                key={user.id}
                user={user}
                isSelected={selectedIds.has(user.id)}
                onSelect={() => toggleSelect(user.id)}
                onDelete={() => deleteMutation.mutate(user.id)}
                onDetails={() => {
                  setSelectedUser(user)
                  setIsDetailsOpen(true)
                }}
                onEdit={() => {
                  setEditUser(user)
                  setIsEditOpen(true)
                }}
                onBlock={() => setBlockUser(user)}
                onUnlock={() => enableMutation.mutate(user.id)}
                onVerifyEmail={() =>
                  setEmailVerifiedMutation.mutate({
                    userId: user.id,
                    verified: !user.email_verified,
                  })
                }
              />
            ))}
          </div>
        )}
      </Card>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex flex-col sm:flex-row items-center justify-between gap-4 px-2">
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">
              Mostrando {(page - 1) * pageSize + 1} a {Math.min(page * pageSize, totalCount)} de{" "}
              {totalCount} usuarios
            </span>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage(1)}
              disabled={page === 1}
            >
              Primera
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage(page - 1)}
              disabled={page === 1}
            >
              Anterior
            </Button>
            <span className="text-sm font-medium text-foreground px-2">
              Página {page} de {totalPages}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage(page + 1)}
              disabled={page === totalPages}
            >
              Siguiente
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage(totalPages)}
              disabled={page === totalPages}
            >
              Última
            </Button>
          </div>
        </div>
      )}

      {/* Create Dialog */}
      <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
        <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Crear Nuevo Usuario</DialogTitle>
            <DialogDescription>
              Ingresa los datos básicos y campos personalizados.
            </DialogDescription>
          </DialogHeader>
          <CreateUserForm
            fieldDefs={fieldDefs || []}
            clients={clients || []}
            onSubmit={(data) => createMutation.mutate(data)}
            isPending={createMutation.isPending}
          />
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      {editUser && (
        <Dialog open={isEditOpen} onOpenChange={setIsEditOpen}>
          <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto">
            <DialogHeader>
              <DialogTitle>Editar Usuario</DialogTitle>
              <DialogDescription>Actualiza los datos del usuario.</DialogDescription>
            </DialogHeader>
            <EditUserForm
              user={editUser}
              fieldDefs={fieldDefs || []}
              clients={clients || []}
              onSubmit={(data) =>
                updateMutation.mutate({ userId: editUser.id, data })
              }
              isPending={updateMutation.isPending}
            />
          </DialogContent>
        </Dialog>
      )}

      {/* Details Dialog */}
      {selectedUser && (
        <UserDetails
          user={selectedUser}
          open={isDetailsOpen}
          onClose={() => {
            setIsDetailsOpen(false)
            setSelectedUser(null)
          }}
        />
      )}

      {/* Block Dialog */}
      {blockUser && (
        <BlockUserDialog
          user={blockUser}
          onClose={() => setBlockUser(null)}
          onBlock={(reason, duration) =>
            blockMutation.mutate({ userId: blockUser.id, reason, duration })
          }
          isPending={blockMutation.isPending}
        />
      )}

      {/* Bulk Action Confirmation */}
      <Dialog open={isBulkActionOpen} onOpenChange={setIsBulkActionOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {bulkAction === "block" ? "Bloquear usuarios" : "Eliminar usuarios"}
            </DialogTitle>
            <DialogDescription>
              ¿Estás seguro de que quieres{" "}
              {bulkAction === "block" ? "bloquear" : "eliminar permanentemente"}{" "}
              {selectedIds.size} usuario{selectedIds.size !== 1 ? "s" : ""}?
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setIsBulkActionOpen(false)}>
              Cancelar
            </Button>
            <Button
              variant="danger"
              onClick={() => {
                if (bulkAction === "block") {
                  bulkBlockMutation.mutate(Array.from(selectedIds))
                } else {
                  bulkDeleteMutation.mutate(Array.from(selectedIds))
                }
              }}
              loading={bulkBlockMutation.isPending || bulkDeleteMutation.isPending}
            >
              {bulkAction === "block" ? "Bloquear" : "Eliminar"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

// ----------------------------------------------------------------------
// COMPONENT: User Row
// ----------------------------------------------------------------------

function UserRow({
  user,
  isSelected,
  onSelect,
  onDelete,
  onDetails,
  onEdit,
  onBlock,
  onUnlock,
  onVerifyEmail,
}: {
  user: UserType
  isSelected: boolean
  onSelect: () => void
  onDelete: () => void
  onDetails: () => void
  onEdit: () => void
  onBlock: () => void
  onUnlock: () => void
  onVerifyEmail: () => void
}) {
  let dateStr = "-"
  try {
    if (user.created_at) {
      const d = new Date(user.created_at)
      if (!isNaN(d.getTime())) {
        dateStr = new Intl.DateTimeFormat("es-ES", { dateStyle: "medium" }).format(d)
      }
    }
  } catch (e) {}

  const initial = user.email ? user.email.slice(0, 2).toUpperCase() : "??"
  const isBlocked = !!user.disabled_at
  const isSuspended = !!user.disabled_until && new Date(user.disabled_until) > new Date()
  const displayBlocked = isBlocked || isSuspended

  return (
    <div
      className={cn(
        "group p-6 flex items-center justify-between transition-all duration-200 hover:bg-surface",
        isSelected && "bg-muted/50"
      )}
    >
      <div className="flex items-center gap-3 flex-1">
        <Checkbox checked={isSelected} onCheckedChange={onSelect} aria-label={`Seleccionar ${user.email}`} />
        <div className="h-10 w-10 bg-muted rounded-full flex items-center justify-center font-semibold text-sm text-foreground">
          {initial}
        </div>
        <div className="flex flex-col flex-1">
          <span className="font-medium truncate max-w-[200px] text-foreground">{user.email}</span>
          <div className="flex items-center gap-2 mt-1">
            <span className="text-xs text-muted-foreground font-mono truncate max-w-[150px]">
              {user.id}
            </span>
            {displayBlocked ? (
              <Badge variant="danger" className="h-5 text-[10px] px-1.5">
                {isSuspended ? "Suspendido" : "Bloqueado"}
              </Badge>
            ) : (
              <Badge variant="success" className="h-5 text-[10px] px-1.5">
                Activo
              </Badge>
            )}
          </div>
        </div>
      </div>

      <div className="hidden lg:flex items-center gap-1 text-muted-foreground">
        {user.email_verified ? (
          <>
            <MailCheck className="h-4 w-4 text-success" aria-hidden="true" />
            <span className="text-xs font-medium">Verificado</span>
          </>
        ) : (
          <>
            <MailX className="h-4 w-4 text-warning" aria-hidden="true" />
            <span className="text-xs font-medium">Pendiente</span>
          </>
        )}
      </div>

      <span className="hidden md:block text-sm text-muted-foreground ml-4">{dateStr}</span>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" className="h-8 w-8 p-0 ml-4 opacity-0 group-hover:opacity-100 transition-opacity" aria-label={`Acciones para ${user.email}`}>
            <MoreHorizontal className="h-4 w-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-48">
          <DropdownMenuLabel>Acciones</DropdownMenuLabel>
          <DropdownMenuItem onClick={onDetails}>
            <Eye className="mr-2 h-4 w-4" />
            Ver detalles
          </DropdownMenuItem>
          <DropdownMenuItem onClick={onEdit}>
            <Edit className="mr-2 h-4 w-4" />
            Editar
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          {displayBlocked ? (
            <DropdownMenuItem onClick={onUnlock}>
              <Unlock className="mr-2 h-4 w-4" />
              Desbloquear
            </DropdownMenuItem>
          ) : (
            <DropdownMenuItem onClick={onBlock}>
              <Ban className="mr-2 h-4 w-4" />
              Bloquear
            </DropdownMenuItem>
          )}
          <DropdownMenuItem onClick={onVerifyEmail}>
            {user.email_verified ? (
              <>
                <XCircle className="mr-2 h-4 w-4" />
                Marcar no verificado
              </>
            ) : (
              <>
                <CheckCircle2 className="mr-2 h-4 w-4" />
                Verificar email
              </>
            )}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem className="text-danger focus:text-danger" onClick={onDelete}>
            <Trash2 className="mr-2 h-4 w-4" />
            Eliminar
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}

// ----------------------------------------------------------------------
// COMPONENT: Create User Form
// ----------------------------------------------------------------------

function CreateUserForm({
  fieldDefs,
  clients,
  onSubmit,
  isPending,
}: {
  fieldDefs: UserFieldDefinition[]
  clients: Array<{ client_id: string; name: string; type: string }>
  onSubmit: (data: any) => void
  isPending: boolean
}) {
  const [formData, setFormData] = useState<any>({
    email: "",
    password: "",
    email_verified: false,
    disabled: false,
    name: "",
    given_name: "",
    family_name: "",
    source_client_id: "",
    custom_fields: {},
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSubmit(formData)
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="email" required>
          Email
        </Label>
        <Input
          id="email"
          type="email"
          value={formData.email}
          onChange={(e) => setFormData({ ...formData, email: e.target.value })}
          required
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="password" required>
          Contraseña
        </Label>
        <Input
          id="password"
          type="password"
          value={formData.password}
          onChange={(e) => setFormData({ ...formData, password: e.target.value })}
          required
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="name">Nombre completo</Label>
        <Input
          id="name"
          value={formData.name}
          onChange={(e) => setFormData({ ...formData, name: e.target.value })}
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-2">
          <Label htmlFor="given_name">Nombre</Label>
          <Input
            id="given_name"
            value={formData.given_name}
            onChange={(e) => setFormData({ ...formData, given_name: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="family_name">Apellido</Label>
          <Input
            id="family_name"
            value={formData.family_name}
            onChange={(e) => setFormData({ ...formData, family_name: e.target.value })}
          />
        </div>
      </div>

      {clients && clients.length > 0 && (
        <div className="space-y-2">
          <Label htmlFor="source_client_id">Cliente origen</Label>
          <Select
            value={formData.source_client_id}
            onValueChange={(value) => setFormData({ ...formData, source_client_id: value })}
          >
            <SelectTrigger id="source_client_id">
              <SelectValue placeholder="Seleccionar cliente" />
            </SelectTrigger>
            <SelectContent>
              {clients.map((client) => (
                <SelectItem key={client.client_id} value={client.client_id}>
                  {client.name || client.client_id}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      <div className="flex items-center justify-between p-3 border border-border rounded-md">
        <Label htmlFor="email_verified">Email verificado</Label>
        <Switch
          id="email_verified"
          checked={formData.email_verified}
          onCheckedChange={(checked) => setFormData({ ...formData, email_verified: checked })}
        />
      </div>

      <div className="flex items-center justify-between p-3 border border-border rounded-md">
        <Label htmlFor="disabled">Usuario deshabilitado</Label>
        <Switch
          id="disabled"
          checked={formData.disabled}
          onCheckedChange={(checked) => setFormData({ ...formData, disabled: checked })}
        />
      </div>

      {/* Custom Fields */}
      {fieldDefs.map((field) => (
        <div key={field.name} className="space-y-2">
          <Label htmlFor={field.name} required={field.required}>
            {field.name}
          </Label>
          {field.type === "boolean" ? (
            <Switch
              id={field.name}
              checked={formData.custom_fields[field.name] || false}
              onCheckedChange={(checked) =>
                setFormData({
                  ...formData,
                  custom_fields: { ...formData.custom_fields, [field.name]: checked },
                })
              }
            />
          ) : (
            <Input
              id={field.name}
              type={field.type === "int" ? "number" : "text"}
              value={formData.custom_fields[field.name] || ""}
              onChange={(e) =>
                setFormData({
                  ...formData,
                  custom_fields: { ...formData.custom_fields, [field.name]: e.target.value },
                })
              }
              required={field.required}
            />
          )}
          {field.description && (
            <p className="text-xs text-muted-foreground">{field.description}</p>
          )}
        </div>
      ))}

      <DialogFooter>
        <Button type="submit" loading={isPending}>
          Crear Usuario
        </Button>
      </DialogFooter>
    </form>
  )
}

// ----------------------------------------------------------------------
// COMPONENT: Edit User Form
// ----------------------------------------------------------------------

function EditUserForm({
  user,
  fieldDefs,
  clients,
  onSubmit,
  isPending,
}: {
  user: UserType
  fieldDefs: UserFieldDefinition[]
  clients: Array<{ client_id: string; name: string; type: string }>
  onSubmit: (data: any) => void
  isPending: boolean
}) {
  const [formData, setFormData] = useState<any>({
    email: user.email,
    email_verified: user.email_verified,
    name: user.name || "",
    given_name: user.given_name || "",
    family_name: user.family_name || "",
    custom_fields: user.custom_fields || {},
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSubmit(formData)
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="edit-email" required>
          Email
        </Label>
        <Input
          id="edit-email"
          type="email"
          value={formData.email}
          onChange={(e) => setFormData({ ...formData, email: e.target.value })}
          required
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="edit-name">Nombre completo</Label>
        <Input
          id="edit-name"
          value={formData.name}
          onChange={(e) => setFormData({ ...formData, name: e.target.value })}
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-2">
          <Label htmlFor="edit-given_name">Nombre</Label>
          <Input
            id="edit-given_name"
            value={formData.given_name}
            onChange={(e) => setFormData({ ...formData, given_name: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="edit-family_name">Apellido</Label>
          <Input
            id="edit-family_name"
            value={formData.family_name}
            onChange={(e) => setFormData({ ...formData, family_name: e.target.value })}
          />
        </div>
      </div>

      <div className="flex items-center justify-between p-3 border border-border rounded-md">
        <Label htmlFor="edit-email_verified">Email verificado</Label>
        <Switch
          id="edit-email_verified"
          checked={formData.email_verified}
          onCheckedChange={(checked) => setFormData({ ...formData, email_verified: checked })}
        />
      </div>

      {/* Custom Fields */}
      {fieldDefs.map((field) => (
        <div key={field.name} className="space-y-2">
          <Label htmlFor={`edit-${field.name}`} required={field.required}>
            {field.name}
          </Label>
          {field.type === "boolean" ? (
            <Switch
              id={`edit-${field.name}`}
              checked={formData.custom_fields[field.name] || false}
              onCheckedChange={(checked) =>
                setFormData({
                  ...formData,
                  custom_fields: { ...formData.custom_fields, [field.name]: checked },
                })
              }
            />
          ) : (
            <Input
              id={`edit-${field.name}`}
              type={field.type === "int" ? "number" : "text"}
              value={formData.custom_fields[field.name] || ""}
              onChange={(e) =>
                setFormData({
                  ...formData,
                  custom_fields: { ...formData.custom_fields, [field.name]: e.target.value },
                })
              }
              required={field.required}
            />
          )}
        </div>
      ))}

      <DialogFooter>
        <Button type="submit" loading={isPending}>
          Guardar Cambios
        </Button>
      </DialogFooter>
    </form>
  )
}

// ----------------------------------------------------------------------
// COMPONENT: Block User Dialog
// ----------------------------------------------------------------------

function BlockUserDialog({
  user,
  onClose,
  onBlock,
  isPending,
}: {
  user: UserType
  onClose: () => void
  onBlock: (reason: string, duration: string) => void
  isPending: boolean
}) {
  const [reason, setReason] = useState("")
  const [duration, setDuration] = useState("")

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onBlock(reason, duration)
  }

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Bloquear Usuario</DialogTitle>
          <DialogDescription>
            Bloquear a {user.email}. Opcionalmente especifica un motivo y duración.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="block-reason">Motivo (opcional)</Label>
            <Textarea
              id="block-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="Ej: Actividad sospechosa"
              rows={3}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="block-duration">Duración (opcional)</Label>
            <Input
              id="block-duration"
              type="datetime-local"
              value={duration}
              onChange={(e) => setDuration(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">
              Dejar vacío para bloqueo permanente
            </p>
          </div>

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" variant="danger" loading={isPending}>
              Bloquear
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ----------------------------------------------------------------------
// COMPONENT: User Details Dialog
// ----------------------------------------------------------------------

function UserDetails({
  user,
  open,
  onClose,
}: {
  user: UserType
  open: boolean
  onClose: () => void
}) {
  const [activeTab, setActiveTab] = useState("info")

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Detalles del Usuario</DialogTitle>
          <DialogDescription>{user.email}</DialogDescription>
        </DialogHeader>

        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList className="grid w-full grid-cols-3">
            <TabsTrigger value="info">Información</TabsTrigger>
            <TabsTrigger value="activity">Actividad</TabsTrigger>
            <TabsTrigger value="raw">JSON Raw</TabsTrigger>
          </TabsList>

          <TabsContent value="info" className="space-y-4 mt-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <p className="text-sm font-medium text-muted-foreground">ID</p>
                <p className="text-sm font-mono text-foreground">{user.id}</p>
              </div>
              <div>
                <p className="text-sm font-medium text-muted-foreground">Email</p>
                <p className="text-sm text-foreground">{user.email}</p>
              </div>
              <div>
                <p className="text-sm font-medium text-muted-foreground">Nombre</p>
                <p className="text-sm text-foreground">{user.name || "-"}</p>
              </div>
              <div>
                <p className="text-sm font-medium text-muted-foreground">Locale</p>
                <p className="text-sm text-foreground">{user.locale || "-"}</p>
              </div>
              <div>
                <p className="text-sm font-medium text-muted-foreground">Email Verificado</p>
                <p className="text-sm text-foreground">
                  {user.email_verified ? "Sí" : "No"}
                </p>
              </div>
              <div>
                <p className="text-sm font-medium text-muted-foreground">Creado</p>
                <p className="text-sm text-foreground">
                  {new Date(user.created_at).toLocaleString()}
                </p>
              </div>
            </div>

            {user.custom_fields && Object.keys(user.custom_fields).length > 0 && (
              <div className="border-t border-border pt-4">
                <h4 className="text-sm font-medium text-foreground mb-3">Campos Personalizados</h4>
                <div className="grid grid-cols-2 gap-4">
                  {Object.entries(user.custom_fields).map(([key, value]) => (
                    <div key={key}>
                      <p className="text-sm font-medium text-muted-foreground">{key}</p>
                      <p className="text-sm text-foreground">{String(value)}</p>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </TabsContent>

          <TabsContent value="activity" className="space-y-4 mt-4">
            <div className="space-y-3">
              <div className="flex items-center justify-between p-3 border border-border rounded-md">
                <div className="flex items-center gap-2">
                  <Calendar className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-medium text-foreground">Último login</span>
                </div>
                <span className="text-sm text-muted-foreground">
                  {user.last_login_at
                    ? new Date(user.last_login_at).toLocaleString()
                    : "Nunca"}
                </span>
              </div>

              <div className="flex items-center justify-between p-3 border border-border rounded-md">
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-medium text-foreground">Total logins</span>
                </div>
                <span className="text-sm text-muted-foreground">{user.login_count || 0}</span>
              </div>

              <div className="flex items-center justify-between p-3 border border-border rounded-md">
                <div className="flex items-center gap-2">
                  <XCircle className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-medium text-foreground">Intentos fallidos</span>
                </div>
                <span className="text-sm text-muted-foreground">
                  {user.failed_login_count || 0}
                </span>
              </div>
            </div>
          </TabsContent>

          <TabsContent value="raw" className="mt-4">
            <pre className="p-4 bg-muted rounded-md text-xs font-mono overflow-auto max-h-[400px] text-foreground">
              {JSON.stringify(user, null, 2)}
            </pre>
          </TabsContent>
        </Tabs>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cerrar
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ----------------------------------------------------------------------
// COMPONENT: User Fields Settings (Custom Fields Tab)
// ----------------------------------------------------------------------

function UserFieldsSettings({ tenantId }: { tenantId: string }) {
  const { toast } = useToast()
  const queryClient = useQueryClient()

  const [fields, setFields] = useState<UserFieldDefinition[]>([])
  const [isAddOpen, setIsAddOpen] = useState(false)

  const { data: tenant, isLoading } = useQuery({
    queryKey: ["tenant", tenantId],
    queryFn: async () => {
      const data = await api.get<any>(`/v2/admin/tenants/${tenantId}`)
      setFields(data?.settings?.userFields || [])
      return data
    },
    enabled: !!tenantId,
  })

  const saveFieldsMutation = useMutation({
    mutationFn: async (newFields: UserFieldDefinition[]) => {
      const updatedSettings = {
        ...tenant?.settings,
        userFields: newFields,
      }
      return api.put(
        `/v2/admin/tenants/${tenantId}`,
        { settings: updatedSettings },
        undefined,
        {}
      )
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tenant", tenantId] })
      queryClient.invalidateQueries({ queryKey: ["user-fields", tenantId] })
      toast({ title: "Campos actualizados", variant: "default" })
    },
    onError: (err: Error) => {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    },
  })

  const addField = (field: UserFieldDefinition) => {
    const updated = [...fields, field]
    setFields(updated)
    saveFieldsMutation.mutate(updated)
    setIsAddOpen(false)
  }

  const removeField = (name: string) => {
    const updated = fields.filter((f) => f.name !== name)
    setFields(updated)
    saveFieldsMutation.mutate(updated)
  }

  if (isLoading) {
    return (
      <Card className="p-6">
        <Skeleton className="h-8 w-48 mb-4" />
        <Skeleton className="h-12 w-full mb-2" />
        <Skeleton className="h-12 w-full mb-2" />
        <Skeleton className="h-12 w-full" />
      </Card>
    )
  }

  return (
    <div className="space-y-4">
      <Card>
        <div className="p-6">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h3 className="text-lg font-semibold text-foreground">Campos Personalizados</h3>
              <p className="text-sm text-muted-foreground">
                Define campos adicionales para el registro de usuarios
              </p>
            </div>
            <Button onClick={() => setIsAddOpen(true)}>
              <Plus className="mr-2 h-4 w-4" />
              Agregar Campo
            </Button>
          </div>

          {fields.length === 0 ? (
            <EmptyState
              icon={<Sliders className="w-12 h-12" />}
              title="No hay campos personalizados"
              description="Comienza agregando campos adicionales para capturar más información de tus usuarios"
              action={
                <Button onClick={() => setIsAddOpen(true)}>
                  <Plus className="mr-2 h-4 w-4" />
                  Agregar Campo
                </Button>
              }
            />
          ) : (
            <div className="divide-y divide-border border border-border rounded-lg">
              {fields.map((field) => (
                <div key={field.name} className="p-4 flex items-center justify-between hover:bg-surface transition-colors">
                  <div className="flex-1">
                    <p className="font-medium text-foreground">{field.name}</p>
                    <p className="text-sm text-muted-foreground">
                      Tipo: {field.type}
                      {field.required && " • Requerido"}
                      {field.unique && " • Único"}
                    </p>
                    {field.description && (
                      <p className="text-xs text-muted-foreground mt-1">{field.description}</p>
                    )}
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => removeField(field.name)}
                    className="text-danger"
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>
      </Card>

      {/* Add Field Dialog */}
      <Dialog open={isAddOpen} onOpenChange={setIsAddOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Agregar Campo Personalizado</DialogTitle>
            <DialogDescription>
              Define un nuevo campo para capturar información adicional
            </DialogDescription>
          </DialogHeader>
          <AddFieldForm onAdd={addField} onCancel={() => setIsAddOpen(false)} />
        </DialogContent>
      </Dialog>
    </div>
  )
}

// ----------------------------------------------------------------------
// COMPONENT: Add Field Form
// ----------------------------------------------------------------------

function AddFieldForm({
  onAdd,
  onCancel,
}: {
  onAdd: (field: UserFieldDefinition) => void
  onCancel: () => void
}) {
  const [formData, setFormData] = useState<UserFieldDefinition>({
    name: "",
    type: "text",
    required: false,
    unique: false,
    indexed: false,
    description: "",
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onAdd(formData)
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="field-name" required>
          Nombre del campo
        </Label>
        <Input
          id="field-name"
          value={formData.name}
          onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          required
          placeholder="ej: phone_number"
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="field-type" required>
          Tipo
        </Label>
        <Select
          value={formData.type}
          onValueChange={(value) => setFormData({ ...formData, type: value })}
        >
          <SelectTrigger id="field-type">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="text">Texto</SelectItem>
            <SelectItem value="int">Número</SelectItem>
            <SelectItem value="boolean">Boolean</SelectItem>
            <SelectItem value="date">Fecha</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-2">
        <Label htmlFor="field-description">Descripción (opcional)</Label>
        <Textarea
          id="field-description"
          value={formData.description}
          onChange={(e) => setFormData({ ...formData, description: e.target.value })}
          placeholder="Información adicional sobre este campo"
          rows={2}
        />
      </div>

      <div className="space-y-3">
        <div className="flex items-center justify-between p-3 border border-border rounded-md">
          <Label htmlFor="field-required">Campo requerido</Label>
          <Switch
            id="field-required"
            checked={formData.required}
            onCheckedChange={(checked) => setFormData({ ...formData, required: checked })}
          />
        </div>

        <div className="flex items-center justify-between p-3 border border-border rounded-md">
          <Label htmlFor="field-unique">Valor único</Label>
          <Switch
            id="field-unique"
            checked={formData.unique}
            onCheckedChange={(checked) => setFormData({ ...formData, unique: checked })}
          />
        </div>

        <div className="flex items-center justify-between p-3 border border-border rounded-md">
          <Label htmlFor="field-indexed">Indexado</Label>
          <Switch
            id="field-indexed"
            checked={formData.indexed}
            onCheckedChange={(checked) => setFormData({ ...formData, indexed: checked })}
          />
        </div>
      </div>

      <DialogFooter>
        <Button type="button" variant="ghost" onClick={onCancel}>
          Cancelar
        </Button>
        <Button type="submit">Agregar Campo</Button>
      </DialogFooter>
    </form>
  )
}

// ----------------------------------------------------------------------
// COMPONENT: Stat Card
// ----------------------------------------------------------------------

function StatCard({
  icon: Icon,
  label,
  value,
  variant = "default",
}: {
  icon: any
  label: string
  value: string | number
  variant?: "default" | "success" | "warning" | "danger"
}) {
  const colorClasses = {
    default: "bg-accent/10 text-accent",
    success: "bg-success/10 text-success",
    warning: "bg-warning/10 text-warning",
    danger: "bg-danger/10 text-danger",
  }

  return (
    <div className="flex items-center gap-3 p-3 rounded-lg bg-muted/30 border border-border">
      <div className={cn("p-2 rounded-lg", colorClasses[variant])}>
        <Icon className="h-4 w-4" />
      </div>
      <div>
        <p className="text-xs text-muted-foreground">{label}</p>
        <p className="text-lg font-semibold text-foreground">{value}</p>
      </div>
    </div>
  )
}
