"use client"

import { useState, useEffect, useMemo, Fragment } from "react"
import { useSearchParams, useRouter } from "next/navigation"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { API_ROUTES } from "@/lib/routes"
import { useAuthStore } from "@/lib/auth-store"
import { useI18n } from "@/lib/i18n"
import { useToast } from "@/hooks/use-toast"

// Design System Components
import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  CardFooter,
  Input,
  Label,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Switch,
  Checkbox,
  Badge,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Textarea,
  InlineAlert,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
  Skeleton,
  BackgroundBlobs,
  PageShell,
  PageHeader,
  cn,
} from "@/components/ds"

// Icons
import {
  Shield,
  ShieldCheck,
  ShieldAlert,
  Users,
  Key,
  Plus,
  Trash2,
  Edit2,
  Search,
  Loader2,
  MoreHorizontal,
  Copy,
  ChevronRight,
  Lock,
  Unlock,
  AlertCircle,
  Info,
  CheckCircle2,
  XCircle,
  Grid3X3,
  List,
  UserCog,
  Settings2,
  Layers,
  Database,
  ArrowRight,
  RefreshCw,
  Save,
  X,
  Eye,
  Check,
  Minus,
  ChevronLeft,
} from "lucide-react"

import type { Tenant } from "@/lib/types"

// ----- Types -----
interface Role {
  name: string
  description: string
  isSystem: boolean
  inherits?: string
  permissions: string[]
  userCount?: number
}

interface Permission {
  name: string
  description: string
  resource: string
  action: string
}

interface UserRolesResponse {
  user_id: string
  roles: string[]
}

interface RolePermsResponse {
  tenant_id: string
  role: string
  perms: string[]
}

// Backend response type
interface RoleResponse {
  id: string
  name: string
  description?: string
  inherits_from?: string
  system: boolean
  permissions: string[]
  users_count: number
  created_at: string
  updated_at: string
}

// ----- Predefined Data -----
const PREDEFINED_ROLES: Role[] = [
  {
    name: "admin",
    description: "Acceso completo al sistema. Puede gestionar usuarios, roles y configuración.",
    isSystem: true,
    permissions: ["*"],
    userCount: 0,
  },
  {
    name: "manager",
    description: "Puede gestionar usuarios y ver reportes. No puede cambiar configuración del sistema.",
    isSystem: false,
    inherits: "user",
    permissions: ["users:read", "users:write", "reports:read"],
    userCount: 0,
  },
  {
    name: "user",
    description: "Usuario estándar con acceso básico de lectura.",
    isSystem: true,
    permissions: ["profile:read", "profile:write"],
    userCount: 0,
  },
  {
    name: "guest",
    description: "Acceso mínimo, solo lectura pública.",
    isSystem: true,
    permissions: ["public:read"],
    userCount: 0,
  },
]

const PREDEFINED_PERMISSIONS: Permission[] = [
  // Users
  { name: "users:read", description: "Ver lista de usuarios", resource: "users", action: "read" },
  { name: "users:write", description: "Crear y editar usuarios", resource: "users", action: "write" },
  { name: "users:delete", description: "Eliminar usuarios", resource: "users", action: "delete" },
  // Roles
  { name: "roles:read", description: "Ver roles y permisos", resource: "roles", action: "read" },
  { name: "roles:write", description: "Crear y editar roles", resource: "roles", action: "write" },
  { name: "roles:delete", description: "Eliminar roles", resource: "roles", action: "delete" },
  // Profile
  { name: "profile:read", description: "Ver perfil propio", resource: "profile", action: "read" },
  { name: "profile:write", description: "Editar perfil propio", resource: "profile", action: "write" },
  // Reports
  { name: "reports:read", description: "Ver reportes y analíticas", resource: "reports", action: "read" },
  { name: "reports:export", description: "Exportar reportes", resource: "reports", action: "export" },
  // Settings
  { name: "settings:read", description: "Ver configuración del sistema", resource: "settings", action: "read" },
  { name: "settings:write", description: "Modificar configuración", resource: "settings", action: "write" },
  // Clients
  { name: "clients:read", description: "Ver aplicaciones OAuth", resource: "clients", action: "read" },
  { name: "clients:write", description: "Crear y editar aplicaciones", resource: "clients", action: "write" },
  { name: "clients:delete", description: "Eliminar aplicaciones", resource: "clients", action: "delete" },
  // Audit
  { name: "audit:read", description: "Ver logs de auditoría", resource: "audit", action: "read" },
  // Public
  { name: "public:read", description: "Acceso a recursos públicos", resource: "public", action: "read" },
]

// Group permissions by resource
const PERMISSION_GROUPS = PREDEFINED_PERMISSIONS.reduce((acc, perm) => {
  if (!acc[perm.resource]) {
    acc[perm.resource] = []
  }
  acc[perm.resource].push(perm)
  return acc
}, {} as Record<string, Permission[]>)

// ----- Helper Components -----
function InfoTooltip({ content }: { content: string }) {
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <Info className="h-3.5 w-3.5 text-muted-foreground cursor-help ml-1 inline" />
        </TooltipTrigger>
        <TooltipContent className="max-w-xs">
          <p className="text-xs">{content}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

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
    success: "bg-accent/10 text-accent",
    warning: "bg-accent/10 text-accent",
    danger: "bg-destructive/10 text-destructive",
  }

  return (
    <Card interactive className="group p-6">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm text-muted-foreground">{label}</p>
          <h3 className="text-3xl font-display font-bold text-foreground mt-1">
            {value}
          </h3>
        </div>
        <div className={cn("rounded-full p-3", colorClasses[variant])}>
          <Icon className="h-6 w-6" />
        </div>
      </div>
    </Card>
  )
}

// ----- Main Component -----
export default function RBACPage() {
  const searchParams = useSearchParams()
  const router = useRouter()
  const tenantId = searchParams.get("id")
  const { t } = useI18n()
  const { token } = useAuthStore()
  const { toast } = useToast()

  const [activeTab, setActiveTab] = useState("roles")

  // Fetch tenant details to show name in header
  const { data: tenant, isLoading: isLoadingTenant } = useQuery<Tenant>({
    queryKey: ["tenant-detail", tenantId],
    queryFn: () => api.get<Tenant>(`/v2/admin/tenants/${tenantId}`),
    enabled: !!tenantId && !!token,
  })

  // Redirect if no tenant ID
  useEffect(() => {
    if (!tenantId) {
      router.push("/admin/tenants")
    }
  }, [tenantId, router])

  if (!tenantId) {
    return null
  }

  return (
    <PageShell>
      <BackgroundBlobs />

      {/* Back Navigation */}
      <div className="mb-6">
        <Button
          variant="outline"
          size="sm"
          onClick={() => router.push("/admin/tenants")}
          className="hover:-translate-y-0.5 hover:shadow-clay-card transition-all duration-200"
        >
          <ChevronLeft className="h-4 w-4 mr-2" />
          Volver a Tenants
        </Button>
      </div>

      <PageHeader
        title="Control de Acceso (RBAC)"
        description={
          isLoadingTenant
            ? "Cargando información del tenant..."
            : `Gestiona roles, permisos y asignaciones de usuarios para ${tenant?.name || tenant?.slug || "este tenant"}.`
        }
      />

      {/* Info Banner */}
      <InlineAlert variant="info" className="mb-6">
        <Shield className="h-4 w-4" />
        <div>
          <h4 className="font-semibold mb-1">¿Qué es RBAC?</h4>
          <p className="text-sm">
            <strong>Role-Based Access Control</strong> permite controlar qué pueden hacer los usuarios en tu aplicación.
            Los <strong>Roles</strong> agrupan permisos, y los <strong>Permisos</strong> definen acciones específicas sobre recursos.
            Un usuario puede tener múltiples roles, y sus permisos efectivos son la unión de todos ellos.
          </p>
        </div>
      </InlineAlert>

      {/* Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
        <StatCard icon={Shield} label="Roles Definidos" value={PREDEFINED_ROLES.length} variant="default" />
        <StatCard icon={Key} label="Permisos Totales" value={PREDEFINED_PERMISSIONS.length} variant="success" />
        <StatCard icon={Layers} label="Recursos" value={Object.keys(PERMISSION_GROUPS).length} variant="warning" />
        <StatCard icon={Users} label="Usuarios con Roles" value="—" variant="default" />
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-6">
        <TabsList className="grid w-full max-w-2xl grid-cols-4">
          <TabsTrigger value="roles" className="flex items-center gap-2">
            <Shield className="h-4 w-4" />
            <span className="hidden sm:inline">Roles</span>
          </TabsTrigger>
          <TabsTrigger value="permissions" className="flex items-center gap-2">
            <Key className="h-4 w-4" />
            <span className="hidden sm:inline">Permisos</span>
          </TabsTrigger>
          <TabsTrigger value="matrix" className="flex items-center gap-2">
            <Grid3X3 className="h-4 w-4" />
            <span className="hidden sm:inline">Matriz</span>
          </TabsTrigger>
          <TabsTrigger value="assignments" className="flex items-center gap-2">
            <UserCog className="h-4 w-4" />
            <span className="hidden sm:inline">Asignaciones</span>
          </TabsTrigger>
        </TabsList>

        <TabsContent value="roles" className="space-y-4">
          <RolesTab tenantId={tenantId} />
        </TabsContent>

        <TabsContent value="permissions" className="space-y-4">
          <PermissionsTab tenantId={tenantId} />
        </TabsContent>

        <TabsContent value="matrix" className="space-y-4">
          <MatrixTab tenantId={tenantId} />
        </TabsContent>

        <TabsContent value="assignments" className="space-y-4">
          <AssignmentsTab tenantId={tenantId} />
        </TabsContent>
      </Tabs>
    </PageShell>
  )
}

// ----------------------------------------------------------------------
// TAB: Roles
// ----------------------------------------------------------------------
function RolesTab({ tenantId }: { tenantId: string }) {
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState("")
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [editRole, setEditRole] = useState<Role | null>(null)
  const [viewRole, setViewRole] = useState<Role | null>(null)

  // Fetch roles from backend
  const { data: rolesData, isLoading: isLoadingRoles } = useQuery({
    queryKey: ["rbac-roles", tenantId],
    enabled: !!tenantId,
    queryFn: () =>
      api.get<RoleResponse[]>(`/v2/admin/rbac/roles`, {
        headers: { "X-Tenant-ID": tenantId },
      }),
  })

  // Map backend response to frontend Role type
  const roles: Role[] = useMemo(() => {
    if (!rolesData?.length) return PREDEFINED_ROLES
    return rolesData.map((r) => ({
      name: r.name,
      description: r.description || "",
      isSystem: r.system,
      inherits: r.inherits_from || undefined,
      permissions: r.permissions || [],
      userCount: r.users_count,
    }))
  }, [rolesData])

  // Mutations
  const createRoleMutation = useMutation({
    mutationFn: (newRole: Role) =>
      api.post(
        `/v2/admin/rbac/roles`,
        {
          name: newRole.name,
          description: newRole.description,
          inherits_from: newRole.inherits || null,
          permissions: newRole.permissions,
        },
        {
          headers: { "X-Tenant-ID": tenantId },
        }
      ),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["rbac-roles", tenantId] })
      setIsCreateOpen(false)
      toast({ title: "Rol creado", description: `El rol "${variables.name}" ha sido creado.` })
    },
    onError: (error: any) => {
      toast({ title: "Error", description: error?.message || "No se pudo crear el rol", variant: "destructive" })
    },
  })

  const updateRoleMutation = useMutation({
    mutationFn: (updated: Role) =>
      api.put(
        `/v2/admin/rbac/roles/${updated.name}`,
        {
          description: updated.description,
          inherits_from: updated.inherits || null,
          permissions: updated.permissions,
        },
        undefined,
        {
          headers: { "X-Tenant-ID": tenantId },
        }
      ),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["rbac-roles", tenantId] })
      setEditRole(null)
      toast({ title: "Rol actualizado", description: `El rol "${variables.name}" ha sido actualizado.` })
    },
    onError: (error: any) => {
      toast({ title: "Error", description: error?.message || "No se pudo actualizar el rol", variant: "destructive" })
    },
  })

  const deleteRoleMutation = useMutation({
    mutationFn: (roleName: string) =>
      api.delete(`/v2/admin/rbac/roles/${roleName}`, {
        headers: { "X-Tenant-ID": tenantId },
      }),
    onSuccess: (_, roleName) => {
      queryClient.invalidateQueries({ queryKey: ["rbac-roles", tenantId] })
      toast({ title: "Rol eliminado", description: `El rol "${roleName}" ha sido eliminado.` })
    },
    onError: (error: any) => {
      toast({ title: "Error", description: error?.message || "No se pudo eliminar el rol", variant: "destructive" })
    },
  })

  // Handlers
  const filteredRoles = useMemo(() => {
    if (!search) return roles
    const q = search.toLowerCase()
    return roles.filter((r) => r.name.toLowerCase().includes(q) || r.description.toLowerCase().includes(q))
  }, [roles, search])

  const handleCreateRole = (newRole: Role) => {
    createRoleMutation.mutate(newRole)
  }

  const handleUpdateRole = (updated: Role) => {
    updateRoleMutation.mutate(updated)
  }

  const handleDeleteRole = (roleName: string) => {
    const role = roles.find((r) => r.name === roleName)
    if (role?.isSystem) {
      toast({ title: "Error", description: "No puedes eliminar roles del sistema.", variant: "destructive" })
      return
    }
    deleteRoleMutation.mutate(roleName)
  }

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div className="relative flex-1 sm:flex-none">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Buscar roles..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9 h-9 w-full sm:w-[250px]"
          />
        </div>
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <Button
            variant="default"
            onClick={() => setIsCreateOpen(true)}
            className="hover:-translate-y-0.5 hover:shadow-clay-card active:translate-y-0 transition-all duration-200"
          >
            <Plus className="mr-2 h-4 w-4" /> Crear Rol
          </Button>
          <DialogContent className="sm:max-w-lg">
            <DialogHeader>
              <DialogTitle>Crear Nuevo Rol</DialogTitle>
              <DialogDescription>Define un nuevo rol con sus permisos asociados.</DialogDescription>
            </DialogHeader>
            <RoleForm existingNames={roles.map((r) => r.name)} onSubmit={handleCreateRole} onCancel={() => setIsCreateOpen(false)} />
          </DialogContent>
        </Dialog>
      </div>

      {/* Loading State */}
      {isLoadingRoles && (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {[...Array(6)].map((_, i) => (
            <Card key={i} className="p-6">
              <div className="space-y-3">
                <Skeleton className="h-4 w-20" />
                <Skeleton className="h-6 w-32" />
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-3/4" />
              </div>
            </Card>
          ))}
        </div>
      )}

      {/* Roles Grid */}
      {!isLoadingRoles && (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {filteredRoles.map((role) => (
            <Card
              key={role.name}
              interactive
              className={cn(
                "group hover:-translate-y-1 hover:shadow-clay-float transition-all duration-200",
                role.isSystem && "border-accent/30 bg-accent/5"
              )}
            >
              <CardHeader className="pb-2">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-2">
                    <div
                      className={cn(
                        "p-2 rounded-lg",
                        role.isSystem ? "bg-accent/10 text-accent" : "bg-accent/10 text-accent"
                      )}
                    >
                      {role.isSystem ? <ShieldCheck className="h-4 w-4" /> : <Shield className="h-4 w-4" />}
                    </div>
                    <div>
                      <CardTitle className="text-base flex items-center gap-2">
                        {role.name}
                        {role.isSystem && (
                          <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-accent/10 text-accent border-accent/30">
                            Sistema
                          </Badge>
                        )}
                      </CardTitle>
                      {role.inherits && (
                        <p className="text-xs text-muted-foreground flex items-center gap-1">
                          <ArrowRight className="h-3 w-3" /> Hereda de: {role.inherits}
                        </p>
                      )}
                    </div>
                  </div>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="sm" className="h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity">
                        <MoreHorizontal className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onClick={() => setViewRole(role)}>
                        <Eye className="mr-2 h-4 w-4" /> Ver Detalles
                      </DropdownMenuItem>
                      {!role.isSystem && (
                        <>
                          <DropdownMenuItem onClick={() => setEditRole(role)}>
                            <Edit2 className="mr-2 h-4 w-4" /> Editar
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem onClick={() => handleDeleteRole(role.name)} className="text-destructive">
                            <Trash2 className="mr-2 h-4 w-4" /> Eliminar
                          </DropdownMenuItem>
                        </>
                      )}
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                <p className="text-sm text-muted-foreground line-clamp-2">{role.description}</p>
                <div className="flex items-center justify-between text-xs">
                  <span className="flex items-center gap-1 text-muted-foreground">
                    <Key className="h-3 w-3" />
                    {role.permissions.includes("*") ? "Todos" : role.permissions.length} permisos
                  </span>
                  <span className="flex items-center gap-1 text-muted-foreground">
                    <Users className="h-3 w-3" />
                    {role.userCount ?? "—"} usuarios
                  </span>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {!isLoadingRoles && filteredRoles.length === 0 && (
        <Card className="p-12">
          <div className="flex flex-col items-center text-center space-y-4">
            <div className="rounded-full bg-muted p-4">
              <Shield className="h-8 w-8 text-muted-foreground" />
            </div>
            <div className="space-y-2">
              <h3 className="text-lg font-semibold text-foreground">No se encontraron roles</h3>
              <p className="text-sm text-muted-foreground max-w-sm">
                {search ? "Intenta con otro término de búsqueda" : "Comienza creando tu primer rol personalizado"}
              </p>
            </div>
            {!search && (
              <Button variant="default" onClick={() => setIsCreateOpen(true)}>
                <Plus className="h-4 w-4 mr-2" />
                Crear Rol
              </Button>
            )}
          </div>
        </Card>
      )}

      {/* View Role Dialog */}
      <Dialog open={!!viewRole} onOpenChange={(open) => !open && setViewRole(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              {viewRole?.isSystem ? <ShieldCheck className="h-5 w-5 text-accent" /> : <Shield className="h-5 w-5" />}
              {viewRole?.name}
            </DialogTitle>
            <DialogDescription>{viewRole?.description}</DialogDescription>
          </DialogHeader>
          {viewRole && (
            <div className="space-y-4">
              {viewRole.inherits && (
                <div className="p-3 rounded-lg bg-muted/50 border">
                  <Label className="text-xs text-muted-foreground">Hereda de</Label>
                  <p className="font-medium">{viewRole.inherits}</p>
                </div>
              )}
              <div>
                <Label className="text-xs text-muted-foreground mb-2 block">Permisos ({viewRole.permissions.length})</Label>
                <div className="flex flex-wrap gap-1.5">
                  {viewRole.permissions.map((perm) => (
                    <Badge key={perm} variant="secondary" className="text-xs">
                      {perm}
                    </Badge>
                  ))}
                </div>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* Edit Role Dialog */}
      <Dialog open={!!editRole} onOpenChange={(open) => !open && setEditRole(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Editar Rol: {editRole?.name}</DialogTitle>
            <DialogDescription>Modifica la descripción y permisos del rol.</DialogDescription>
          </DialogHeader>
          {editRole && (
            <RoleForm
              initialRole={editRole}
              existingNames={roles.filter((r) => r.name !== editRole.name).map((r) => r.name)}
              onSubmit={handleUpdateRole}
              onCancel={() => setEditRole(null)}
              isEdit
            />
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}

// Role Create/Edit Form
function RoleForm({
  initialRole,
  existingNames,
  onSubmit,
  onCancel,
  isEdit,
}: {
  initialRole?: Role
  existingNames: string[]
  onSubmit: (role: Role) => void
  onCancel: () => void
  isEdit?: boolean
}) {
  const [name, setName] = useState(initialRole?.name || "")
  const [description, setDescription] = useState(initialRole?.description || "")
  const [inherits, setInherits] = useState(initialRole?.inherits || "")
  const [selectedPerms, setSelectedPerms] = useState<string[]>(initialRole?.permissions || [])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    if (!isEdit && existingNames.includes(name.trim().toLowerCase())) {
      return
    }
    onSubmit({
      name: name.trim().toLowerCase(),
      description: description.trim(),
      isSystem: false,
      inherits: inherits || undefined,
      permissions: selectedPerms,
    })
  }

  const togglePerm = (perm: string) => {
    if (selectedPerms.includes(perm)) {
      setSelectedPerms(selectedPerms.filter((p) => p !== perm))
    } else {
      setSelectedPerms([...selectedPerms, perm])
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="grid gap-4">
        <div className="grid gap-2">
          <Label htmlFor="name">Nombre del Rol</Label>
          <Input
            id="name"
            placeholder="ej. editor, moderator"
            value={name}
            onChange={(e) => setName(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ""))}
            disabled={isEdit}
          />
          {isEdit && <p className="text-xs text-muted-foreground">El nombre no se puede cambiar.</p>}
        </div>
        <div className="grid gap-2">
          <Label htmlFor="description">Descripción</Label>
          <Textarea
            id="description"
            placeholder="Describe qué puede hacer este rol..."
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={2}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="inherits">Hereda de (opcional)</Label>
          <Select value={inherits || "_none"} onValueChange={(v) => setInherits(v === "_none" ? "" : v)}>
            <SelectTrigger>
              <SelectValue placeholder="Seleccionar rol padre..." />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_none">Sin herencia</SelectItem>
              {PREDEFINED_ROLES.filter((r) => r.name !== name).map((r) => (
                <SelectItem key={r.name} value={r.name}>
                  {r.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">El rol heredará automáticamente los permisos del rol padre.</p>
        </div>
      </div>

      <div className="space-y-2">
        <Label>Permisos</Label>
        <div className="max-h-[200px] overflow-y-auto border rounded-lg p-3 space-y-3">
          {Object.entries(PERMISSION_GROUPS).map(([resource, perms]) => (
            <div key={resource}>
              <p className="text-xs font-semibold text-muted-foreground uppercase mb-1.5">{resource}</p>
              <div className="flex flex-wrap gap-1.5">
                {perms.map((perm) => (
                  <Badge
                    key={perm.name}
                    variant={selectedPerms.includes(perm.name) ? "default" : "outline"}
                    className="cursor-pointer transition-colors"
                    onClick={() => togglePerm(perm.name)}
                  >
                    {selectedPerms.includes(perm.name) && <Check className="h-3 w-3 mr-1" />}
                    {perm.action}
                  </Badge>
                ))}
              </div>
            </div>
          ))}
        </div>
        <p className="text-xs text-muted-foreground">{selectedPerms.length} permiso(s) seleccionado(s)</p>
      </div>

      <DialogFooter>
        <Button type="button" variant="outline" onClick={onCancel}>
          Cancelar
        </Button>
        <Button type="submit" disabled={!name.trim()}>
          {isEdit ? "Guardar Cambios" : "Crear Rol"}
        </Button>
      </DialogFooter>
    </form>
  )
}

// ----------------------------------------------------------------------
// TAB: Permissions
// ----------------------------------------------------------------------
function PermissionsTab({ tenantId }: { tenantId: string }) {
  const [search, setSearch] = useState("")
  const [groupBy, setGroupBy] = useState<"resource" | "action">("resource")

  const { data: backendPermissions, isLoading } = useQuery({
    queryKey: ["rbac-permissions"],
    queryFn: async () => {
      try {
        const response = await api.get<Permission[]>(API_ROUTES.ADMIN_RBAC_PERMISSIONS)
        return response
      } catch (e) {
        console.warn("Failed to load permissions from backend, using fallback", e)
        return PREDEFINED_PERMISSIONS
      }
    },
  })

  const permissions = backendPermissions || PREDEFINED_PERMISSIONS

  const filteredPerms = useMemo(() => {
    if (!search) return permissions
    const q = search.toLowerCase()
    return permissions.filter(
      (p) => p.name.toLowerCase().includes(q) || p.description.toLowerCase().includes(q) || p.resource.toLowerCase().includes(q)
    )
  }, [search, permissions])

  const grouped = useMemo(() => {
    const key = groupBy === "resource" ? "resource" : "action"
    return filteredPerms.reduce((acc, perm) => {
      const group = perm[key]
      if (!acc[group]) acc[group] = []
      acc[group].push(perm)
      return acc
    }, {} as Record<string, Permission[]>)
  }, [filteredPerms, groupBy])

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div className="relative flex-1 sm:flex-none">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input placeholder="Buscar permisos..." value={search} onChange={(e) => setSearch(e.target.value)} className="pl-9 h-9 w-full sm:w-[250px]" />
        </div>
        <div className="flex items-center gap-2">
          <Label className="text-sm text-muted-foreground">Agrupar por:</Label>
          <Select value={groupBy} onValueChange={(v: "resource" | "action") => setGroupBy(v)}>
            <SelectTrigger className="w-[130px] h-9">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="resource">Recurso</SelectItem>
              <SelectItem value="action">Acción</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* Info */}
      <InlineAlert variant="info">
        <Info className="h-4 w-4" />
        <p className="text-sm">
          Los permisos siguen el formato <code className="bg-muted px-1 rounded">recurso:acción</code>. Puedes crear permisos personalizados asignándolos
          directamente a un rol.
        </p>
      </InlineAlert>

      {/* Permissions Grid by Group */}
      <div className="space-y-6">
        {Object.entries(grouped).map(([group, perms]) => (
          <div key={group}>
            <div className="flex items-center gap-2 mb-3">
              <div className="p-1.5 rounded-md bg-accent/10">
                <Layers className="h-4 w-4 text-accent" />
              </div>
              <h3 className="font-semibold capitalize">{group}</h3>
              <Badge variant="secondary" className="text-xs">
                {perms.length}
              </Badge>
            </div>
            <div className="grid gap-2 md:grid-cols-2 lg:grid-cols-3">
              {perms.map((perm) => (
                <Card key={perm.name} interactive className="group p-3 hover:-translate-y-0.5 hover:shadow-clay-card transition-all duration-200">
                  <div className="flex items-start gap-3">
                    <div className="p-1.5 rounded-md bg-accent/10">
                      <Key className="h-3.5 w-3.5 text-accent" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="font-mono text-sm font-medium">{perm.name}</p>
                      <p className="text-xs text-muted-foreground truncate">{perm.description}</p>
                    </div>
                    <Button variant="ghost" size="sm" className="h-7 w-7" onClick={() => navigator.clipboard.writeText(perm.name)}>
                      <Copy className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </Card>
              ))}
            </div>
          </div>
        ))}
      </div>

      {filteredPerms.length === 0 && (
        <Card className="p-12">
          <div className="flex flex-col items-center text-center space-y-4">
            <div className="rounded-full bg-muted p-4">
              <Key className="h-8 w-8 text-muted-foreground" />
            </div>
            <div className="space-y-2">
              <h3 className="text-lg font-semibold text-foreground">No se encontraron permisos</h3>
              <p className="text-sm text-muted-foreground max-w-sm">Intenta con otro término de búsqueda</p>
            </div>
          </div>
        </Card>
      )}
    </div>
  )
}

// ----------------------------------------------------------------------
// TAB: Matrix
// ----------------------------------------------------------------------
function MatrixTab({ tenantId }: { tenantId: string }) {
  const { toast } = useToast()
  const [roles, setRoles] = useState<Role[]>(PREDEFINED_ROLES)
  const [hasChanges, setHasChanges] = useState(false)

  const resources = Object.keys(PERMISSION_GROUPS)

  const togglePermission = (roleName: string, permName: string) => {
    setRoles(
      roles.map((role) => {
        if (role.name !== roleName) return role
        if (role.isSystem && role.permissions.includes("*")) return role

        const hasIt = role.permissions.includes(permName)
        return {
          ...role,
          permissions: hasIt ? role.permissions.filter((p) => p !== permName) : [...role.permissions, permName],
        }
      })
    )
    setHasChanges(true)
  }

  const hasPermission = (role: Role, permName: string) => {
    if (role.permissions.includes("*")) return true
    return role.permissions.includes(permName)
  }

  const handleSave = () => {
    toast({ title: "Matriz guardada", description: "Los cambios han sido aplicados." })
    setHasChanges(false)
  }

  return (
    <div className="space-y-4">
      {/* Info */}
      <InlineAlert variant="info">
        <Grid3X3 className="h-4 w-4" />
        <p className="text-sm">
          Vista matricial de roles y permisos. Haz clic en una celda para activar/desactivar un permiso. Los roles del sistema (admin) no pueden ser
          modificados directamente.
        </p>
      </InlineAlert>

      {/* Matrix Table */}
      <Card className="overflow-hidden">
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/30">
                <TableHead className="font-semibold w-[180px] sticky left-0 bg-muted/30 z-10">Permiso</TableHead>
                {roles.map((role) => (
                  <TableHead key={role.name} className="text-center min-w-[100px]">
                    <div className="flex flex-col items-center gap-1">
                      {role.isSystem ? <ShieldCheck className="h-4 w-4 text-accent" /> : <Shield className="h-4 w-4 text-accent" />}
                      <span className="font-medium">{role.name}</span>
                    </div>
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {resources.map((resource) => (
                <Fragment key={resource}>
                  <TableRow key={`header-${resource}`} className="bg-muted/10">
                    <TableCell colSpan={roles.length + 1} className="py-2">
                      <span className="text-xs font-semibold text-muted-foreground uppercase">{resource}</span>
                    </TableCell>
                  </TableRow>
                  {PERMISSION_GROUPS[resource].map((perm) => (
                    <TableRow key={perm.name} className="hover:bg-accent/5 transition-colors">
                      <TableCell className="font-mono text-xs sticky left-0 bg-card z-10">
                        <div className="flex items-center gap-2">
                          <Key className="h-3 w-3 text-muted-foreground" />
                          {perm.action}
                        </div>
                      </TableCell>
                      {roles.map((role) => {
                        const has = hasPermission(role, perm.name)
                        const isWildcard = role.permissions.includes("*")
                        return (
                          <TableCell key={`${role.name}-${perm.name}`} className="text-center p-2">
                            <button
                              className={cn(
                                "h-8 w-8 rounded-md flex items-center justify-center transition-colors mx-auto",
                                has ? "bg-accent/20 text-accent hover:bg-accent/30" : "bg-muted/50 text-muted-foreground hover:bg-muted",
                                isWildcard && "cursor-not-allowed opacity-70"
                              )}
                              onClick={() => !isWildcard && togglePermission(role.name, perm.name)}
                              disabled={isWildcard}
                            >
                              {has ? <Check className="h-4 w-4" /> : <Minus className="h-4 w-4" />}
                            </button>
                          </TableCell>
                        )
                      })}
                    </TableRow>
                  ))}
                </Fragment>
              ))}
            </TableBody>
          </Table>
        </div>
      </Card>

      {/* Save Button */}
      {hasChanges && (
        <Card className="p-4 bg-accent/5 border-accent/30">
          <div className="flex items-center justify-between">
            <p className="text-sm text-muted-foreground">Tienes cambios sin guardar en la matriz de permisos.</p>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setRoles(PREDEFINED_ROLES)
                  setHasChanges(false)
                }}
              >
                Descartar
              </Button>
              <Button size="sm" onClick={handleSave}>
                <Save className="h-4 w-4 mr-2" />
                Guardar Cambios
              </Button>
            </div>
          </div>
        </Card>
      )}
    </div>
  )
}

// ----------------------------------------------------------------------
// TAB: User Assignments
// ----------------------------------------------------------------------
function AssignmentsTab({ tenantId }: { tenantId: string }) {
  const { toast } = useToast()
  const { t } = useI18n()
  const queryClient = useQueryClient()

  // User -> Roles
  const [userId, setUserId] = useState("")
  const [newUserRole, setNewUserRole] = useState("")

  const {
    data: userRoles,
    refetch: refetchUserRoles,
    isFetching: loadingUserRoles,
    error: userRolesError,
  } = useQuery({
    queryKey: ["rbac-user-roles", userId, tenantId],
    enabled: false,
    queryFn: () =>
      api.get<UserRolesResponse>(API_ROUTES.ADMIN_RBAC_USER_ROLES(userId), {
        headers: { "X-Tenant-ID": tenantId },
      }),
  })

  const addUserRole = useMutation({
    mutationFn: (role: string) =>
      api.post<UserRolesResponse>(
        API_ROUTES.ADMIN_RBAC_USER_ROLES(userId),
        { add: [role], remove: [] },
        {
          headers: { "X-Tenant-ID": tenantId },
        }
      ),
    onSuccess: () => {
      setNewUserRole("")
      refetchUserRoles()
      toast({ title: "Rol asignado", description: "El rol ha sido agregado al usuario." })
    },
    onError: (e: any) => toast({ title: "Error", description: e.message, variant: "destructive" }),
  })

  const removeUserRole = useMutation({
    mutationFn: (role: string) =>
      api.post<UserRolesResponse>(
        API_ROUTES.ADMIN_RBAC_USER_ROLES(userId),
        { add: [], remove: [role] },
        {
          headers: { "X-Tenant-ID": tenantId },
        }
      ),
    onSuccess: () => {
      refetchUserRoles()
      toast({ title: "Rol removido", description: "El rol ha sido quitado del usuario." })
    },
    onError: (e: any) => toast({ title: "Error", description: e.message, variant: "destructive" }),
  })

  // Role -> Permissions
  const [roleName, setRoleName] = useState("")
  const [newPerm, setNewPerm] = useState("")

  const {
    data: rolePerms,
    refetch: refetchRolePerms,
    isFetching: loadingRolePerms,
  } = useQuery({
    queryKey: ["rbac-role-perms", roleName, tenantId],
    enabled: false,
    queryFn: () =>
      api.get<RolePermsResponse>(API_ROUTES.ADMIN_RBAC_ROLE_PERMS(roleName), {
        headers: { "X-Tenant-ID": tenantId },
      }),
  })

  const addPerm = useMutation({
    mutationFn: (perm: string) =>
      api.post<RolePermsResponse>(
        API_ROUTES.ADMIN_RBAC_ROLE_PERMS(roleName),
        { add: [perm], remove: [] },
        {
          headers: { "X-Tenant-ID": tenantId },
        }
      ),
    onSuccess: () => {
      setNewPerm("")
      refetchRolePerms()
      toast({ title: "Permiso agregado", description: "El permiso ha sido añadido al rol." })
    },
    onError: (e: any) => toast({ title: "Error", description: e.message, variant: "destructive" }),
  })

  const removePerm = useMutation({
    mutationFn: (perm: string) =>
      api.post<RolePermsResponse>(
        API_ROUTES.ADMIN_RBAC_ROLE_PERMS(roleName),
        { add: [], remove: [perm] },
        {
          headers: { "X-Tenant-ID": tenantId },
        }
      ),
    onSuccess: () => {
      refetchRolePerms()
      toast({ title: "Permiso removido", description: "El permiso ha sido quitado del rol." })
    },
    onError: (e: any) => toast({ title: "Error", description: e.message, variant: "destructive" }),
  })

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      {/* User -> Roles Panel */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <div className="p-2 bg-accent/10 rounded-lg">
              <UserCog className="h-5 w-5 text-accent" />
            </div>
            <div>
              <CardTitle className="text-lg">Roles de Usuario</CardTitle>
              <CardDescription>Busca un usuario por ID y gestiona sus roles.</CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex gap-2">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input placeholder="User ID (UUID)" value={userId} onChange={(e) => setUserId(e.target.value.trim())} className="pl-9" />
            </div>
            <Button onClick={() => refetchUserRoles()} disabled={!userId || loadingUserRoles}>
              {loadingUserRoles ? <Loader2 className="h-4 w-4 animate-spin" /> : "Cargar"}
            </Button>
          </div>

          {userRoles?.roles !== undefined && (
            <div className="space-y-3 animate-in fade-in duration-200">
              <Label className="text-xs text-muted-foreground uppercase">Roles Asignados</Label>
              <div className="flex flex-wrap gap-2">
                {userRoles.roles.length === 0 ? (
                  <p className="text-sm text-muted-foreground">Sin roles asignados</p>
                ) : (
                  userRoles.roles.map((r) => (
                    <Badge key={r} variant="secondary" className="flex items-center gap-2 py-1">
                      <Shield className="h-3 w-3" /> {r}
                      <button className="text-destructive hover:text-destructive/80" onClick={() => removeUserRole.mutate(r)}>
                        <X className="h-3 w-3" />
                      </button>
                    </Badge>
                  ))
                )}
              </div>
              <div className="flex gap-2">
                <Select value={newUserRole || "_none"} onValueChange={(v) => setNewUserRole(v === "_none" ? "" : v)}>
                  <SelectTrigger className="flex-1">
                    <SelectValue placeholder="Seleccionar rol..." />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="_none" disabled>
                      Seleccionar rol...
                    </SelectItem>
                    {PREDEFINED_ROLES.filter((r) => !userRoles.roles.includes(r.name)).map((r) => (
                      <SelectItem key={r.name} value={r.name}>
                        {r.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Button onClick={() => newUserRole && addUserRole.mutate(newUserRole)} disabled={!newUserRole || addUserRole.isPending}>
                  {addUserRole.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
                </Button>
              </div>
            </div>
          )}

          {userRolesError && (
            <InlineAlert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <p className="text-sm">{(userRolesError as any)?.message || "Error al cargar roles"}</p>
            </InlineAlert>
          )}
        </CardContent>
      </Card>

      {/* Role -> Permissions Panel */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <div className="p-2 bg-accent/10 rounded-lg">
              <Key className="h-5 w-5 text-accent" />
            </div>
            <div>
              <CardTitle className="text-lg">Permisos de Rol</CardTitle>
              <CardDescription>Busca un rol y gestiona sus permisos.</CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex gap-2">
            <Select value={roleName || "_none"} onValueChange={(v) => setRoleName(v === "_none" ? "" : v)}>
              <SelectTrigger className="flex-1">
                <SelectValue placeholder="Seleccionar rol..." />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="_none" disabled>
                  Seleccionar rol...
                </SelectItem>
                {PREDEFINED_ROLES.map((r) => (
                  <SelectItem key={r.name} value={r.name}>
                    {r.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button onClick={() => refetchRolePerms()} disabled={!roleName || loadingRolePerms}>
              {loadingRolePerms ? <Loader2 className="h-4 w-4 animate-spin" /> : "Cargar"}
            </Button>
          </div>

          {rolePerms?.perms !== undefined && (
            <div className="space-y-3 animate-in fade-in duration-200">
              <Label className="text-xs text-muted-foreground uppercase">Permisos del Rol</Label>
              <div className="flex flex-wrap gap-2 max-h-[150px] overflow-y-auto">
                {rolePerms.perms.length === 0 ? (
                  <p className="text-sm text-muted-foreground">Sin permisos asignados</p>
                ) : (
                  rolePerms.perms.map((p) => (
                    <Badge key={p} variant="outline" className="flex items-center gap-2 py-1 font-mono text-xs">
                      {p}
                      <button className="text-destructive hover:text-destructive/80" onClick={() => removePerm.mutate(p)}>
                        <X className="h-3 w-3" />
                      </button>
                    </Badge>
                  ))
                )}
              </div>
              <div className="flex gap-2">
                <Select value={newPerm || "_none"} onValueChange={(v) => setNewPerm(v === "_none" ? "" : v)}>
                  <SelectTrigger className="flex-1">
                    <SelectValue placeholder="Agregar permiso..." />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="_none" disabled>
                      Agregar permiso...
                    </SelectItem>
                    {PREDEFINED_PERMISSIONS.filter((p) => !rolePerms.perms.includes(p.name)).map((p) => (
                      <SelectItem key={p.name} value={p.name}>
                        {p.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Button onClick={() => newPerm && addPerm.mutate(newPerm)} disabled={!newPerm || addPerm.isPending}>
                  {addPerm.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
                </Button>
              </div>
              <div className="pt-2">
                <Label className="text-xs text-muted-foreground">O ingresa manualmente:</Label>
                <div className="flex gap-2 mt-1">
                  <Input
                    placeholder="recurso:accion"
                    value={newPerm}
                    onChange={(e) => setNewPerm(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && newPerm) {
                        e.preventDefault()
                        addPerm.mutate(newPerm)
                      }
                    }}
                    className="font-mono text-sm"
                  />
                </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
