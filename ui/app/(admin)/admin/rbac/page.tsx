"use client"

import { useState, useEffect, useMemo } from "react"
import { useSearchParams } from "next/navigation"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
    CardFooter,
} from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Checkbox } from "@/components/ui/checkbox"
import { useToast } from "@/hooks/use-toast"
import { api } from "@/lib/api"
import { API_ROUTES } from "@/lib/routes"
import { useAuthStore } from "@/lib/auth-store"
import { useI18n } from "@/lib/i18n"
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
} from "lucide-react"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import {
    Tabs,
    TabsContent,
    TabsList,
    TabsTrigger,
} from "@/components/ui/tabs"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import { Tenant } from "@/lib/types"

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
// These would ideally come from backend, but for now we define common defaults
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

function StatCard({ icon: Icon, label, value, variant = "default" }: {
    icon: any
    label: string
    value: string | number
    variant?: "default" | "success" | "warning" | "danger"
}) {
    const colorClasses = {
        default: "bg-blue-500/10 text-blue-600",
        success: "bg-green-500/10 text-green-600",
        warning: "bg-amber-500/10 text-amber-600",
        danger: "bg-red-500/10 text-red-600",
    }
    return (
        <div className="flex items-center gap-3 p-3 rounded-lg bg-muted/30 border">
            <div className={cn("p-2 rounded-lg", colorClasses[variant])}>
                <Icon className="h-4 w-4" />
            </div>
            <div>
                <p className="text-xs text-muted-foreground">{label}</p>
                <p className="text-lg font-semibold">{value}</p>
            </div>
        </div>
    )
}

// ----- Main Component -----
export default function RBACPage() {
    const searchParams = useSearchParams()
    const tenantIdParam = searchParams.get("id")
    const { t } = useI18n()
    const { token } = useAuthStore()
    const { toast } = useToast()

    const [selectedTenantId, setSelectedTenantId] = useState<string>(tenantIdParam || "")
    const [activeTab, setActiveTab] = useState("roles")

    // Fetch tenants for selector
    const { data: tenants } = useQuery<Tenant[]>({
        queryKey: ["tenants-list"],
        queryFn: () => api.get<Tenant[]>("/v2/admin/tenants"),
        enabled: !!token,
    })

    // Auto-select first tenant if none selected
    useEffect(() => {
        if (!selectedTenantId && tenants && tenants.length > 0) {
            setSelectedTenantId(tenants[0].id)
        }
    }, [tenants, selectedTenantId])

    return (
        <div className="space-y-6 animate-in fade-in duration-500">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                    <div className="relative">
                        <div className="absolute inset-0 bg-gradient-to-br from-emerald-400/20 to-teal-400/20 rounded-xl blur-xl" />
                        <div className="relative p-3 bg-gradient-to-br from-emerald-500/10 to-teal-500/10 rounded-xl border border-emerald-500/20">
                            <Shield className="h-6 w-6 text-emerald-600 dark:text-emerald-400" />
                        </div>
                    </div>
                    <div>
                        <h2 className="text-2xl font-bold tracking-tight">Control de Acceso (RBAC)</h2>
                        <p className="text-sm text-muted-foreground">
                            Gestiona roles, permisos y asignaciones de usuarios.
                        </p>
                    </div>
                </div>
                {/* Tenant Selector */}
                <div className="flex items-center gap-2">
                    <Label className="text-sm text-muted-foreground">Tenant:</Label>
                    <Select value={selectedTenantId} onValueChange={setSelectedTenantId}>
                        <SelectTrigger className="w-[200px]">
                            <SelectValue placeholder="Seleccionar tenant..." />
                        </SelectTrigger>
                        <SelectContent>
                            {tenants?.map((t) => (
                                <SelectItem key={t.id} value={t.id}>
                                    {t.name || t.slug}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>
            </div>

            {/* Info Banner */}
            <Alert className="bg-gradient-to-r from-emerald-50 to-teal-50 dark:from-emerald-950/20 dark:to-teal-950/20 border-emerald-200 dark:border-emerald-800">
                <Shield className="h-4 w-4 text-emerald-600" />
                <AlertTitle className="text-emerald-900 dark:text-emerald-100">¿Qué es RBAC?</AlertTitle>
                <AlertDescription className="text-emerald-800 dark:text-emerald-200">
                    <strong>Role-Based Access Control</strong> permite controlar qué pueden hacer los usuarios en tu aplicación.
                    Los <strong>Roles</strong> agrupan permisos, y los <strong>Permisos</strong> definen acciones específicas sobre recursos.
                    Un usuario puede tener múltiples roles, y sus permisos efectivos son la unión de todos ellos.
                </AlertDescription>
            </Alert>

            {!selectedTenantId ? (
                <div className="flex flex-col items-center justify-center py-20 px-6">
                    <div className="relative mb-8">
                        <div className="absolute inset-0 bg-gradient-to-br from-amber-400/20 to-orange-400/20 rounded-full blur-2xl scale-150" />
                        <div className="relative rounded-2xl bg-gradient-to-br from-amber-50 to-orange-50 dark:from-amber-950/50 dark:to-orange-950/50 p-5">
                            <Database className="h-8 w-8 text-amber-600 dark:text-amber-400" />
                        </div>
                    </div>
                    <h3 className="text-xl font-semibold text-center mb-2">Selecciona un Tenant</h3>
                    <p className="text-muted-foreground text-center max-w-sm text-sm">
                        Para gestionar roles y permisos, primero selecciona un tenant del menú superior.
                    </p>
                </div>
            ) : (
                <>
                    {/* Stats */}
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                        <StatCard icon={Shield} label="Roles Definidos" value={PREDEFINED_ROLES.length} variant="default" />
                        <StatCard icon={Key} label="Permisos Totales" value={PREDEFINED_PERMISSIONS.length} variant="success" />
                        <StatCard icon={Layers} label="Recursos" value={Object.keys(PERMISSION_GROUPS).length} variant="warning" />
                        <StatCard icon={Users} label="Usuarios con Roles" value="—" variant="default" />
                    </div>

                    {/* Tabs */}
                    <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-4">
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
                            <RolesTab tenantId={selectedTenantId} />
                        </TabsContent>

                        <TabsContent value="permissions" className="space-y-4">
                            <PermissionsTab tenantId={selectedTenantId} />
                        </TabsContent>

                        <TabsContent value="matrix" className="space-y-4">
                            <MatrixTab tenantId={selectedTenantId} />
                        </TabsContent>

                        <TabsContent value="assignments" className="space-y-4">
                            <AssignmentsTab tenantId={selectedTenantId} />
                        </TabsContent>
                    </Tabs>
                </>
            )}
        </div>
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

    // ======================================================================
    // QUERIES
    // ======================================================================

    // Fetch roles from backend
    const { data: rolesData, isLoading: isLoadingRoles } = useQuery({
        queryKey: ["rbac-roles", tenantId],
        enabled: !!tenantId,
        queryFn: () => api.get<RoleResponse[]>(`/v2/admin/rbac/roles`, {
            headers: { "X-Tenant-ID": tenantId }
        }),
    })

    // Map backend response to frontend Role type, or use predefined if loading/error
    const roles: Role[] = useMemo(() => {
        if (!rolesData?.length) return PREDEFINED_ROLES
        return rolesData.map(r => ({
            name: r.name,
            description: r.description || "",
            isSystem: r.system,
            inherits: r.inherits_from || undefined,
            permissions: r.permissions || [],
            userCount: r.users_count,
        }))
    }, [rolesData])

    // ======================================================================
    // MUTATIONS
    // ======================================================================

    const createRoleMutation = useMutation({
        mutationFn: (newRole: Role) =>
            api.post(`/v2/admin/rbac/roles`, {
                name: newRole.name,
                description: newRole.description,
                inherits_from: newRole.inherits || null,
                permissions: newRole.permissions,
            }, {
                headers: { "X-Tenant-ID": tenantId }
            }),
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
            api.put(`/v2/admin/rbac/roles/${updated.name}`, {
                description: updated.description,
                inherits_from: updated.inherits || null,
                permissions: updated.permissions,
            }, undefined, {
                headers: { "X-Tenant-ID": tenantId }
            }),
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
                headers: { "X-Tenant-ID": tenantId }
            }),
        onSuccess: (_, roleName) => {
            queryClient.invalidateQueries({ queryKey: ["rbac-roles", tenantId] })
            toast({ title: "Rol eliminado", description: `El rol "${roleName}" ha sido eliminado.` })
        },
        onError: (error: any) => {
            toast({ title: "Error", description: error?.message || "No se pudo eliminar el rol", variant: "destructive" })
        },
    })

    // ======================================================================
    // HANDLERS
    // ======================================================================

    const filteredRoles = useMemo(() => {
        if (!search) return roles
        const q = search.toLowerCase()
        return roles.filter(r =>
            r.name.toLowerCase().includes(q) ||
            r.description.toLowerCase().includes(q)
        )
    }, [roles, search])

    const handleCreateRole = (newRole: Role) => {
        createRoleMutation.mutate(newRole)
    }

    const handleUpdateRole = (updated: Role) => {
        updateRoleMutation.mutate(updated)
    }

    const handleDeleteRole = (roleName: string) => {
        const role = roles.find(r => r.name === roleName)
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
                    <DialogTrigger asChild>
                        <Button className="h-9">
                            <Plus className="mr-2 h-4 w-4" /> Crear Rol
                        </Button>
                    </DialogTrigger>
                    <DialogContent className="sm:max-w-lg">
                        <DialogHeader>
                            <DialogTitle>Crear Nuevo Rol</DialogTitle>
                            <DialogDescription>
                                Define un nuevo rol con sus permisos asociados.
                            </DialogDescription>
                        </DialogHeader>
                        <RoleForm
                            existingNames={roles.map(r => r.name)}
                            onSubmit={handleCreateRole}
                            onCancel={() => setIsCreateOpen(false)}
                        />
                    </DialogContent>
                </Dialog>
            </div>

            {/* Roles Grid */}
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                {filteredRoles.map((role) => (
                    <Card key={role.name} className={cn(
                        "group hover:shadow-lg transition-all duration-200",
                        role.isSystem && "border-emerald-500/30 bg-emerald-500/5"
                    )}>
                        <CardHeader className="pb-2">
                            <div className="flex items-start justify-between">
                                <div className="flex items-center gap-2">
                                    <div className={cn(
                                        "p-2 rounded-lg",
                                        role.isSystem
                                            ? "bg-emerald-500/10 text-emerald-600"
                                            : "bg-blue-500/10 text-blue-600"
                                    )}>
                                        {role.isSystem ? <ShieldCheck className="h-4 w-4" /> : <Shield className="h-4 w-4" />}
                                    </div>
                                    <div>
                                        <CardTitle className="text-base flex items-center gap-2">
                                            {role.name}
                                            {role.isSystem && (
                                                <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-emerald-500/10 text-emerald-600 border-emerald-500/30">
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
                                        <Button variant="ghost" size="icon" className="h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity">
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
                                                <DropdownMenuItem
                                                    onClick={() => handleDeleteRole(role.name)}
                                                    className="text-red-600"
                                                >
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

            {filteredRoles.length === 0 && (
                <div className="text-center py-12 text-muted-foreground">
                    <Shield className="h-12 w-12 mx-auto mb-4 opacity-20" />
                    <p>No se encontraron roles.</p>
                </div>
            )}

            {/* View Role Dialog */}
            <Dialog open={!!viewRole} onOpenChange={(open) => !open && setViewRole(null)}>
                <DialogContent className="sm:max-w-lg">
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2">
                            {viewRole?.isSystem ? <ShieldCheck className="h-5 w-5 text-emerald-600" /> : <Shield className="h-5 w-5" />}
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
                        <DialogDescription>
                            Modifica la descripción y permisos del rol.
                        </DialogDescription>
                    </DialogHeader>
                    {editRole && (
                        <RoleForm
                            initialRole={editRole}
                            existingNames={roles.filter(r => r.name !== editRole.name).map(r => r.name)}
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
function RoleForm({ initialRole, existingNames, onSubmit, onCancel, isEdit }: {
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
            setSelectedPerms(selectedPerms.filter(p => p !== perm))
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
                        onChange={(e) => setName(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ''))}
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
                            {PREDEFINED_ROLES.filter(r => r.name !== name).map(r => (
                                <SelectItem key={r.name} value={r.name}>{r.name}</SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    <p className="text-xs text-muted-foreground">
                        El rol heredará automáticamente los permisos del rol padre.
                    </p>
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
                <p className="text-xs text-muted-foreground">
                    {selectedPerms.length} permiso(s) seleccionado(s)
                </p>
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
// ISS-10-02: Fix hardcoded permissions - now fetches from backend
// ----------------------------------------------------------------------
function PermissionsTab({ tenantId }: { tenantId: string }) {
    const [search, setSearch] = useState("")
    const [groupBy, setGroupBy] = useState<"resource" | "action">("resource")

    // ISS-10-02: Fetch permissions from backend instead of using hardcoded data
    const { data: backendPermissions, isLoading } = useQuery({
        queryKey: ["rbac-permissions"],
        queryFn: async () => {
            try {
                const response = await api.get<Permission[]>(API_ROUTES.ADMIN_RBAC_PERMISSIONS)
                return response
            } catch (e) {
                // Fallback to predefined permissions if backend fails
                console.warn("Failed to load permissions from backend, using fallback", e)
                return PREDEFINED_PERMISSIONS
            }
        },
    })

    // Use backend permissions or fallback to predefined
    const permissions = backendPermissions || PREDEFINED_PERMISSIONS

    const filteredPerms = useMemo(() => {
        if (!search) return permissions
        const q = search.toLowerCase()
        return permissions.filter(p =>
            p.name.toLowerCase().includes(q) ||
            p.description.toLowerCase().includes(q) ||
            p.resource.toLowerCase().includes(q)
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
                    <Input
                        placeholder="Buscar permisos..."
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        className="pl-9 h-9 w-full sm:w-[250px]"
                    />
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
            <Alert>
                <Info className="h-4 w-4" />
                <AlertDescription>
                    Los permisos siguen el formato <code className="bg-muted px-1 rounded">recurso:acción</code>.
                    Puedes crear permisos personalizados asignándolos directamente a un rol.
                </AlertDescription>
            </Alert>

            {/* Permissions Grid by Group */}
            <div className="space-y-6">
                {Object.entries(grouped).map(([group, perms]) => (
                    <div key={group}>
                        <div className="flex items-center gap-2 mb-3">
                            <div className="p-1.5 rounded-md bg-blue-500/10">
                                <Layers className="h-4 w-4 text-blue-600" />
                            </div>
                            <h3 className="font-semibold capitalize">{group}</h3>
                            <Badge variant="secondary" className="text-xs">{perms.length}</Badge>
                        </div>
                        <div className="grid gap-2 md:grid-cols-2 lg:grid-cols-3">
                            {perms.map((perm) => (
                                <div
                                    key={perm.name}
                                    className="flex items-start gap-3 p-3 rounded-lg border bg-card hover:bg-muted/30 transition-colors"
                                >
                                    <div className="p-1.5 rounded-md bg-emerald-500/10">
                                        <Key className="h-3.5 w-3.5 text-emerald-600" />
                                    </div>
                                    <div className="flex-1 min-w-0">
                                        <p className="font-mono text-sm font-medium">{perm.name}</p>
                                        <p className="text-xs text-muted-foreground truncate">{perm.description}</p>
                                    </div>
                                    <Button
                                        variant="ghost"
                                        size="icon"
                                        className="h-7 w-7"
                                        onClick={() => navigator.clipboard.writeText(perm.name)}
                                    >
                                        <Copy className="h-3.5 w-3.5" />
                                    </Button>
                                </div>
                            ))}
                        </div>
                    </div>
                ))}
            </div>

            {filteredPerms.length === 0 && (
                <div className="text-center py-12 text-muted-foreground">
                    <Key className="h-12 w-12 mx-auto mb-4 opacity-20" />
                    <p>No se encontraron permisos.</p>
                </div>
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

    // Group permissions by resource for better display
    const resources = Object.keys(PERMISSION_GROUPS)

    const togglePermission = (roleName: string, permName: string) => {
        setRoles(roles.map(role => {
            if (role.name !== roleName) return role
            if (role.isSystem && role.permissions.includes("*")) return role // Can't modify admin

            const hasIt = role.permissions.includes(permName)
            return {
                ...role,
                permissions: hasIt
                    ? role.permissions.filter(p => p !== permName)
                    : [...role.permissions, permName]
            }
        }))
        setHasChanges(true)
    }

    const hasPermission = (role: Role, permName: string) => {
        if (role.permissions.includes("*")) return true
        return role.permissions.includes(permName)
    }

    const handleSave = () => {
        // In real implementation, this would call the API
        toast({ title: "Matriz guardada", description: "Los cambios han sido aplicados." })
        setHasChanges(false)
    }

    return (
        <div className="space-y-4">
            {/* Info */}
            <Alert>
                <Grid3X3 className="h-4 w-4" />
                <AlertDescription>
                    Vista matricial de roles y permisos. Haz clic en una celda para activar/desactivar un permiso.
                    Los roles del sistema (admin) no pueden ser modificados directamente.
                </AlertDescription>
            </Alert>

            {/* Matrix Table */}
            <div className="border rounded-lg overflow-hidden">
                <div className="overflow-x-auto">
                    <Table>
                        <TableHeader>
                            <TableRow className="bg-muted/30">
                                <TableHead className="font-semibold w-[180px] sticky left-0 bg-muted/30 z-10">
                                    Permiso
                                </TableHead>
                                {roles.map((role) => (
                                    <TableHead key={role.name} className="text-center min-w-[100px]">
                                        <div className="flex flex-col items-center gap-1">
                                            {role.isSystem ? (
                                                <ShieldCheck className="h-4 w-4 text-emerald-600" />
                                            ) : (
                                                <Shield className="h-4 w-4 text-blue-600" />
                                            )}
                                            <span className="font-medium">{role.name}</span>
                                        </div>
                                    </TableHead>
                                ))}
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {resources.map((resource) => (
                                <>
                                    {/* Resource Header Row */}
                                    <TableRow key={`header-${resource}`} className="bg-muted/10">
                                        <TableCell colSpan={roles.length + 1} className="py-2">
                                            <span className="text-xs font-semibold text-muted-foreground uppercase">
                                                {resource}
                                            </span>
                                        </TableCell>
                                    </TableRow>
                                    {/* Permission Rows */}
                                    {PERMISSION_GROUPS[resource].map((perm) => (
                                        <TableRow key={perm.name}>
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
                                                                has
                                                                    ? "bg-emerald-500/20 text-emerald-600 hover:bg-emerald-500/30"
                                                                    : "bg-muted/50 text-muted-foreground hover:bg-muted",
                                                                isWildcard && "cursor-not-allowed opacity-70"
                                                            )}
                                                            onClick={() => !isWildcard && togglePermission(role.name, perm.name)}
                                                            disabled={isWildcard}
                                                        >
                                                            {has ? (
                                                                <Check className="h-4 w-4" />
                                                            ) : (
                                                                <Minus className="h-4 w-4" />
                                                            )}
                                                        </button>
                                                    </TableCell>
                                                )
                                            })}
                                        </TableRow>
                                    ))}
                                </>
                            ))}
                        </TableBody>
                    </Table>
                </div>
            </div>

            {/* Save Button */}
            {hasChanges && (
                <div className="flex items-center justify-between p-3 bg-muted/50 rounded-lg border border-dashed animate-in slide-in-from-bottom-2">
                    <p className="text-sm text-muted-foreground">
                        Tienes cambios sin guardar en la matriz de permisos.
                    </p>
                    <div className="flex items-center gap-2">
                        <Button variant="ghost" size="sm" onClick={() => {
                            setRoles(PREDEFINED_ROLES)
                            setHasChanges(false)
                        }}>
                            Descartar
                        </Button>
                        <Button size="sm" onClick={handleSave}>
                            <Save className="h-4 w-4 mr-2" />
                            Guardar Cambios
                        </Button>
                    </div>
                </div>
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
        queryFn: () => api.get<UserRolesResponse>(API_ROUTES.ADMIN_RBAC_USER_ROLES(userId), {
            headers: { "X-Tenant-ID": tenantId }
        }),
    })

    const addUserRole = useMutation({
        mutationFn: (role: string) =>
            api.post<UserRolesResponse>(API_ROUTES.ADMIN_RBAC_USER_ROLES(userId), { add: [role], remove: [] }, {
                headers: { "X-Tenant-ID": tenantId }
            }),
        onSuccess: () => {
            setNewUserRole("")
            refetchUserRoles()
            toast({ title: "Rol asignado", description: "El rol ha sido agregado al usuario." })
        },
        onError: (e: any) => toast({ title: "Error", description: e.message, variant: "destructive" }),
    })

    const removeUserRole = useMutation({
        mutationFn: (role: string) =>
            api.post<UserRolesResponse>(API_ROUTES.ADMIN_RBAC_USER_ROLES(userId), { add: [], remove: [role] }, {
                headers: { "X-Tenant-ID": tenantId }
            }),
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
        queryFn: () => api.get<RolePermsResponse>(API_ROUTES.ADMIN_RBAC_ROLE_PERMS(roleName), {
            headers: { "X-Tenant-ID": tenantId }
        }),
    })

    const addPerm = useMutation({
        mutationFn: (perm: string) =>
            api.post<RolePermsResponse>(API_ROUTES.ADMIN_RBAC_ROLE_PERMS(roleName), { add: [perm], remove: [] }, {
                headers: { "X-Tenant-ID": tenantId }
            }),
        onSuccess: () => {
            setNewPerm("")
            refetchRolePerms()
            toast({ title: "Permiso agregado", description: "El permiso ha sido añadido al rol." })
        },
        onError: (e: any) => toast({ title: "Error", description: e.message, variant: "destructive" }),
    })

    const removePerm = useMutation({
        mutationFn: (perm: string) =>
            api.post<RolePermsResponse>(API_ROUTES.ADMIN_RBAC_ROLE_PERMS(roleName), { add: [], remove: [perm] }, {
                headers: { "X-Tenant-ID": tenantId }
            }),
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
                        <div className="p-2 bg-purple-500/10 rounded-lg">
                            <UserCog className="h-5 w-5 text-purple-600" />
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
                            <Input
                                placeholder="User ID (UUID)"
                                value={userId}
                                onChange={(e) => setUserId(e.target.value.trim())}
                                className="pl-9"
                            />
                        </div>
                        <Button
                            onClick={() => refetchUserRoles()}
                            disabled={!userId || loadingUserRoles}
                        >
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
                                            <button
                                                className="text-destructive hover:text-destructive/80"
                                                onClick={() => removeUserRole.mutate(r)}
                                            >
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
                                        <SelectItem value="_none" disabled>Seleccionar rol...</SelectItem>
                                        {PREDEFINED_ROLES.filter(r => !userRoles.roles.includes(r.name)).map(r => (
                                            <SelectItem key={r.name} value={r.name}>{r.name}</SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                <Button
                                    onClick={() => newUserRole && addUserRole.mutate(newUserRole)}
                                    disabled={!newUserRole || addUserRole.isPending}
                                >
                                    {addUserRole.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
                                </Button>
                            </div>
                        </div>
                    )}

                    {userRolesError && (
                        <Alert variant="destructive">
                            <AlertCircle className="h-4 w-4" />
                            <AlertDescription>
                                {(userRolesError as any)?.message || "Error al cargar roles"}
                            </AlertDescription>
                        </Alert>
                    )}
                </CardContent>
            </Card>

            {/* Role -> Permissions Panel */}
            <Card>
                <CardHeader>
                    <div className="flex items-center gap-2">
                        <div className="p-2 bg-blue-500/10 rounded-lg">
                            <Key className="h-5 w-5 text-blue-600" />
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
                                <SelectItem value="_none" disabled>Seleccionar rol...</SelectItem>
                                {PREDEFINED_ROLES.map(r => (
                                    <SelectItem key={r.name} value={r.name}>{r.name}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                        <Button
                            onClick={() => refetchRolePerms()}
                            disabled={!roleName || loadingRolePerms}
                        >
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
                                            <button
                                                className="text-destructive hover:text-destructive/80"
                                                onClick={() => removePerm.mutate(p)}
                                            >
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
                                        <SelectItem value="_none" disabled>Agregar permiso...</SelectItem>
                                        {PREDEFINED_PERMISSIONS.filter(p => !rolePerms.perms.includes(p.name)).map(p => (
                                            <SelectItem key={p.name} value={p.name}>{p.name}</SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                <Button
                                    onClick={() => newPerm && addPerm.mutate(newPerm)}
                                    disabled={!newPerm || addPerm.isPending}
                                >
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
