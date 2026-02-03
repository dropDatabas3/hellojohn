"use client"

import { useState, useEffect, useMemo, useCallback } from "react"
import { useSearchParams, useParams, useRouter } from "next/navigation"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
    CardFooter,
} from "@/components/ds/core/card"
import { Button } from "@/components/ds/core/button"
import { Input } from "@/components/ds/core/input"
import { Label } from "@/components/ds"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ds"
import { Switch } from "@/components/ds"
import { Checkbox } from "@/components/ds"
import { useToast } from "@/hooks/use-toast"
import { api } from "@/lib/api"
import { useAuthStore } from "@/lib/auth-store"
import { useI18n } from "@/lib/i18n"
import { apiFetch, apiFetchWithTenant } from "@/lib/routes"
import {
    CheckCircle2,
    Edit2,
    Plus,
    Trash2,
    Settings2,
    AlertCircle,
    User as UserIcon,
    Search,
    Loader2,
    MoreHorizontal,
    Eye,
    Copy,
    Ban,
    Unlock,
    LayoutList,
    Sliders,
    Save,
    X,
    Database,
    ArrowRight,
    ArrowLeft,
    Users as UsersIcon,
    Mail,
    MailCheck,
    MailX,
    Key,
    Shield,
    ShieldAlert,
    ShieldCheck,
    ChevronLeft,
    ChevronRight,
    ChevronsLeft,
    ChevronsRight,
    Download,
    History,
    Clock,
    CheckCircle,
    XCircle,
    Info,
    Activity,
    FileJson,
    FileSpreadsheet,
    RefreshCw,
    Pencil,
} from "lucide-react"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ds"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ds"
import { Textarea } from "@/components/ds"
import { Alert, AlertDescription, AlertTitle } from "@/components/ds"
import { InlineAlert } from "@/components/ds"
import { Skeleton } from "@/components/ds"
import { Badge } from "@/components/ds/core/badge"
import {
    Tabs,
    TabsContent,
    TabsList,
    TabsTrigger,
} from "@/components/ds"
import { PhoneInput } from "@/components/ds/forms/phone-input"
import { CountrySelect } from "@/components/ds/forms/country-select"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ds"
import {
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
} from "@/components/ds"
import { BackgroundBlobs } from "@/components/ds/background/blobs"
import { User } from "@/lib/types"
import Link from "next/link"
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

function StatCard({ icon: Icon, label, value, variant = "default", isLoading = false }: {
    icon: any,
    label: string,
    value: string | number,
    variant?: "info" | "success" | "warning" | "accent" | "default"
    isLoading?: boolean
}) {
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
                {isLoading ? (
                    <div className="space-y-2">
                        <div className="flex items-center gap-2">
                            <Skeleton className="h-4 w-4 rounded" />
                            <Skeleton className="h-3 w-20" />
                        </div>
                        <Skeleton className="h-7 w-12 mt-1" />
                    </div>
                ) : (
                    <>
                        <div className={cn("flex items-center gap-2", iconStyles[variant])}>
                            <Icon className="h-4 w-4" />
                            <span className="text-xs font-medium uppercase tracking-wider">{label}</span>
                        </div>
                        <p className="text-2xl font-bold mt-1 text-foreground">{value}</p>
                    </>
                )}
            </CardContent>
        </Card>
    )
}

// ----- Main Component -----
export default function UsersPage() {
    const searchParams = useSearchParams()
    const tenantIdParam = searchParams.get("id")
    const params = useParams()
    const tenantId = tenantIdParam || (params?.id as string)

    const { t } = useI18n()
    const [activeTab, setActiveTab] = useState("list")
    const [isCreateUserOpen, setIsCreateUserOpen] = useState(false)

    if (!tenantId) {
        return (
            <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertTitle>{t("common.error")}</AlertTitle>
                <AlertDescription>Tenant ID missing.</AlertDescription>
            </Alert>
        )
    }

    return (
        <div className="relative space-y-6 animate-in fade-in duration-500">
            <BackgroundBlobs />

            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                    <Button variant="ghost" size="sm" asChild>
                        <Link href={`/admin/tenants/${tenantId}/detail`}>
                            <ArrowLeft className="h-4 w-4" />
                        </Link>
                    </Button>
                    <div className="flex items-center gap-3">
                        <div>
                            <h1 className="text-2xl font-bold tracking-tight">Usuarios</h1>
                            <p className="text-sm text-muted-foreground">
                                Gestión completa de usuarios, campos personalizados y actividad
                            </p>
                        </div>
                    </div>
                </div>
                <Button onClick={() => setIsCreateUserOpen(true)} className="shadow-clay-button hover:shadow-clay-card transition-shadow">
                    <Plus className="mr-2 h-4 w-4" />
                    Nuevo Usuario
                </Button>
            </div>

            {/* Info Banner */}
            <InlineAlert variant="info">
                <div>
                    <p className="font-semibold">Gestión de Identidades</p>
                    <p className="text-sm opacity-90">
                        Administra todos los usuarios registrados en este tenant. Puedes crear, editar, bloquear y eliminar usuarios.
                        Los campos personalizados se configuran en la pestaña "Campos" y aplican al formulario de registro.
                    </p>
                </div>
            </InlineAlert>

            <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-4">
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

                <TabsContent value="list" className="space-y-4">
                    <UsersList tenantId={tenantId} isCreateOpen={isCreateUserOpen} setIsCreateOpen={setIsCreateUserOpen} />
                </TabsContent>

                <TabsContent value="fields" className="space-y-4">
                    <UserFieldsSettings tenantId={tenantId} />
                </TabsContent>
            </Tabs>
        </div>
    )
}

// ----------------------------------------------------------------------
// COMPONENT: Users List with Pagination, Bulk Actions, Edit
// ----------------------------------------------------------------------

function UsersList({ tenantId, isCreateOpen, setIsCreateOpen }: { tenantId: string; isCreateOpen: boolean; setIsCreateOpen: (open: boolean) => void }) {
    const { token } = useAuthStore()
    const { toast } = useToast()
    const queryClient = useQueryClient()
    const router = useRouter()

    // State
    const [search, setSearch] = useState("")
    const [debouncedSearch, setDebouncedSearch] = useState("")
    const [selectedUser, setSelectedUser] = useState<UserType | null>(null)
    const [isDetailsOpen, setIsDetailsOpen] = useState(false)
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
    const { data: usersData, isLoading, error: usersError, refetch } = useQuery<UsersListResponse, any>({
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
    const { data: clients } = useQuery<Array<{ client_id: string, name: string, type: string }>>({
        queryKey: ["tenant-clients", tenantId],
        queryFn: async () => {
            const list = await api.get<any[]>(`/v2/admin/clients`, {
                headers: { "X-Tenant-ID": tenantId }
            })
            return (list || []).filter((c: any) => c.type !== "confidential" && c.client_id)
        },
        enabled: !!tenantId && !!token,
    })

    // Mutations
    const createMutation = useMutation({
        mutationFn: async (vars: any) => {
            return api.post(`/v2/admin/tenants/${tenantId}/users`, vars, {
                headers: { "X-Tenant-ID": tenantId }
            })
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            setIsCreateOpen(false)
            toast({ title: "Usuario creado", description: "El usuario ha sido creado exitosamente.", variant: "success" })
        },
        onError: (err: Error) => {
            toast({ title: "Error", description: err.message, variant: "destructive" })
        },
    })

    const updateMutation = useMutation({
        mutationFn: async ({ userId, data }: { userId: string, data: any }) => {
            return api.put(`/v2/admin/tenants/${tenantId}/users/${userId}`, data, undefined, {
                headers: { "X-Tenant-ID": tenantId }
            })
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            setIsEditOpen(false)
            setEditUser(null)
            toast({ title: "Usuario actualizado", description: "Los cambios han sido guardados.", variant: "success" })
        },
        onError: (err: Error) => {
            toast({ title: "Error", description: err.message, variant: "destructive" })
        },
    })

    const deleteMutation = useMutation({
        mutationFn: async (userId: string) => {
            return api.delete(`/v2/admin/tenants/${tenantId}/users/${userId}`, {
                headers: { "X-Tenant-ID": tenantId }
            })
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            toast({ title: "Usuario eliminado", description: "El usuario ha sido eliminado.", variant: "success" })
        },
        onError: (err: Error) => {
            toast({ title: "Error", description: err.message, variant: "destructive" })
        },
    })

    const blockMutation = useMutation({
        mutationFn: async ({ userId, reason, duration }: { userId: string, reason: string, duration: string }) => {
            return api.post(`/v2/admin/tenants/${tenantId}/users/${userId}/disable`,
                { reason, duration },
                { headers: { "X-Tenant-ID": tenantId } }
            )
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            setBlockUser(null)
            toast({ title: "Usuario bloqueado", description: "El usuario ha sido bloqueado exitosamente.", variant: "success" })
        },
        onError: (err: Error) => {
            toast({ title: "Error", description: err.message, variant: "destructive" })
        }
    })

    const enableMutation = useMutation({
        mutationFn: async (userId: string) => {
            return api.post(`/v2/admin/tenants/${tenantId}/users/${userId}/enable`, {}, {
                headers: { "X-Tenant-ID": tenantId }
            })
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            toast({ title: "Usuario desbloqueado", description: "El usuario ha sido habilitado nuevamente.", variant: "success" })
        },
        onError: (err: Error) => {
            toast({ title: "Error", description: err.message, variant: "destructive" })
        }
    })

    const setEmailVerifiedMutation = useMutation({
        mutationFn: async ({ userId, verified }: { userId: string, verified: boolean }) => {
            return api.post(`/v2/admin/tenants/${tenantId}/users/${userId}/set-email-verified`,
                { verified },
                { headers: { "X-Tenant-ID": tenantId } }
            )
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            toast({ title: "Email verificado", description: "El estado de verificación ha sido actualizado.", variant: "success" })
        },
        onError: (err: Error) => {
            // Fallback: If endpoint doesn't exist, try PUT update
            toast({ title: "Aviso", description: "La verificación manual de email requiere configuración adicional del backend.", variant: "default" })
        }
    })

    const changePasswordMutation = useMutation({
        mutationFn: async ({ userId, newPassword }: { userId: string, newPassword: string }) => {
            return api.post(`/v2/admin/tenants/${tenantId}/users/${userId}/set-password`,
                { password: newPassword },
                { headers: { "X-Tenant-ID": tenantId } }
            )
        },
        onSuccess: () => {
            toast({ title: "Contraseña actualizada", description: "La contraseña ha sido cambiada exitosamente.", variant: "success" })
        },
        onError: (err: Error) => {
            toast({ title: "Error", description: err.message, variant: "destructive" })
        }
    })

    // Bulk Actions
    const bulkBlockMutation = useMutation({
        mutationFn: async (userIds: string[]) => {
            const results = await Promise.allSettled(
                userIds.map(id =>
                    api.post(`/v2/admin/tenants/${tenantId}/users/${id}/disable`,
                        { reason: "Bulk action", duration: "" },
                        { headers: { "X-Tenant-ID": tenantId } }
                    )
                )
            )
            const failed = results.filter(r => r.status === "rejected").length
            if (failed > 0) throw new Error(`${failed} usuarios no pudieron ser bloqueados`)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            setSelectedIds(new Set())
            setIsBulkActionOpen(false)
            toast({ title: "Usuarios bloqueados", description: "Los usuarios seleccionados han sido bloqueados.", variant: "success" })
        },
        onError: (err: Error) => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            toast({ title: "Error parcial", description: err.message, variant: "destructive" })
        }
    })

    const bulkDeleteMutation = useMutation({
        mutationFn: async (userIds: string[]) => {
            const results = await Promise.allSettled(
                userIds.map(id =>
                    api.delete(`/v2/admin/tenants/${tenantId}/users/${id}`, {
                        headers: { "X-Tenant-ID": tenantId }
                    })
                )
            )
            const failed = results.filter(r => r.status === "rejected").length
            if (failed > 0) throw new Error(`${failed} usuarios no pudieron ser eliminados`)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            setSelectedIds(new Set())
            setIsBulkActionOpen(false)
            toast({ title: "Usuarios eliminados", description: "Los usuarios seleccionados han sido eliminados.", variant: "success" })
        },
        onError: (err: Error) => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            toast({ title: "Error parcial", description: err.message, variant: "destructive" })
        }
    })

    // Selection handlers
    const toggleSelectAll = () => {
        if (selectedIds.size === users.length) {
            setSelectedIds(new Set())
        } else {
            setSelectedIds(new Set(users.map(u => u.id)))
        }
    }

    const toggleSelect = (userId: string) => {
        const newSet = new Set(selectedIds)
        if (newSet.has(userId)) {
            newSet.delete(userId)
        } else {
            newSet.add(userId)
        }
        setSelectedIds(newSet)
    }

    // Export handlers
    const exportUsers = (format: "json" | "csv") => {
        const dataToExport = selectedIds.size > 0
            ? users.filter(u => selectedIds.has(u.id))
            : users

        if (format === "json") {
            const blob = new Blob([JSON.stringify(dataToExport, null, 2)], { type: "application/json" })
            const url = URL.createObjectURL(blob)
            const a = document.createElement("a")
            a.href = url
            a.download = `users-${tenantId}-${new Date().toISOString().split('T')[0]}.json`
            a.click()
            URL.revokeObjectURL(url)
        } else {
            const headers = ["id", "email", "email_verified", "created_at", "disabled_at"]
            const csvRows = [
                headers.join(","),
                ...dataToExport.map(u =>
                    headers.map(h => JSON.stringify((u as any)[h] ?? "")).join(",")
                )
            ]
            const blob = new Blob([csvRows.join("\n")], { type: "text/csv" })
            const url = URL.createObjectURL(blob)
            const a = document.createElement("a")
            a.href = url
            a.download = `users-${tenantId}-${new Date().toISOString().split('T')[0]}.csv`
            a.click()
            URL.revokeObjectURL(url)
        }
        toast({ title: "Exportación completada", description: `${dataToExport.length} usuarios exportados en formato ${format.toUpperCase()}.` })
    }

    // Calculate stats
    const stats = useMemo(() => {
        const active = users.filter(u => !u.disabled_at).length
        const blocked = users.filter(u => !!u.disabled_at).length
        const verified = users.filter(u => u.email_verified).length
        const unverified = users.filter(u => !u.email_verified).length
        return { active, blocked, verified, unverified }
    }, [users])

    // Check if tenant has no database configured
    const isNoDatabaseError = usersError?.error === "TENANT_NO_DATABASE" || usersError?.status === 424

    if (isNoDatabaseError) {
        return (
            <div className="flex flex-col items-center justify-center py-20 px-6">
                <div className="relative mb-8">
                    <div className="absolute inset-0 bg-gradient-to-br from-warning/20 to-warning/10 rounded-full blur-2xl scale-150" />
                    <div className="relative rounded-clay bg-gradient-to-br from-warning/10 to-warning/5 p-5 border-2 border-clay shadow-clay-float">
                        <Database className="h-8 w-8 text-warning" />
                    </div>
                </div>
                <h3 className="text-xl font-semibold text-center mb-2">Configura tu base de datos</h3>
                <p className="text-muted-foreground text-center max-w-sm mb-8 text-sm">
                    Conecta una base de datos para comenzar a gestionar los usuarios de este tenant.
                </p>
                <Button
                    onClick={() => router.push(`/admin/database?id=${tenantId}`)}
                    className="gap-2"
                    size="lg"
                >
                    Configurar
                    <ArrowRight className="h-4 w-4" />
                </Button>
            </div>
        )
    }

    return (
        <div className="space-y-4">
            {/* Stats Row */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <StatCard icon={UsersIcon} label="Total Usuarios" value={totalCount} variant="info" />
                <StatCard icon={ShieldCheck} label="Activos" value={stats.active} variant="success" />
                <StatCard icon={ShieldAlert} label="Bloqueados" value={stats.blocked} variant="warning" />
                <StatCard icon={MailCheck} label="Verificados" value={stats.verified} variant="accent" />
            </div>

            {/* Toolbar */}
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
                <div className="flex items-center gap-2 w-full sm:w-auto">
                    <div className="relative flex-1 sm:flex-none">
                        <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                        <Input
                            placeholder="Buscar por email o ID..."
                            value={search}
                            onChange={(e) => setSearch(e.target.value)}
                            className="pl-9 h-9 w-full sm:w-[250px] lg:w-[300px]"
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
                </div>
            </div>

            {/* Create User Dialog */}
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

            {/* Bulk Actions Bar */}
            {selectedIds.size > 0 && (
                <div className="flex items-center justify-between p-3 bg-accent/5 rounded-clay border-2 border-clay shadow-clay-card animate-in slide-in-from-top-2 duration-200">
                    <div className="flex items-center gap-2">
                        <Checkbox
                            checked={selectedIds.size === users.length}
                            onCheckedChange={toggleSelectAll}
                        />
                        <span className="text-sm font-medium">
                            {selectedIds.size} usuario{selectedIds.size !== 1 ? "s" : ""} seleccionado{selectedIds.size !== 1 ? "s" : ""}
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
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setSelectedIds(new Set())}
                        >
                            <X className="mr-2 h-4 w-4" />
                            Cancelar
                        </Button>
                    </div>
                </div>
            )}

            {/* Users Table */}
            <Card interactive>
                <Table>
                    <TableHeader>
                        <TableRow className="bg-muted/30">
                            <TableHead className="w-[50px]">
                                <Checkbox
                                    checked={users.length > 0 && selectedIds.size === users.length}
                                    onCheckedChange={toggleSelectAll}
                                />
                            </TableHead>
                            <TableHead className="w-[50px]"></TableHead>
                            <TableHead>Identidad</TableHead>
                            <TableHead>Estado</TableHead>
                            <TableHead className="hidden lg:table-cell">Verificación</TableHead>
                            <TableHead className="hidden md:table-cell">Creado</TableHead>
                            <TableHead className="text-right">Acciones</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {isLoading ? (
                            // Skeleton loading rows
                            [...Array(5)].map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell><Skeleton className="h-4 w-4" /></TableCell>
                                    <TableCell><Skeleton className="h-9 w-9 rounded-full" /></TableCell>
                                    <TableCell>
                                        <div className="space-y-2">
                                            <Skeleton className="h-4 w-32" />
                                            <Skeleton className="h-3 w-48" />
                                        </div>
                                    </TableCell>
                                    <TableCell><Skeleton className="h-6 w-16 rounded-full" /></TableCell>
                                    <TableCell className="hidden lg:table-cell"><Skeleton className="h-6 w-20 rounded-full" /></TableCell>
                                    <TableCell className="hidden md:table-cell"><Skeleton className="h-4 w-24" /></TableCell>
                                    <TableCell className="text-right"><Skeleton className="h-8 w-8 rounded ml-auto" /></TableCell>
                                </TableRow>
                            ))
                        ) : users.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={7} className="h-32 text-center text-muted-foreground">
                                    <div className="flex flex-col items-center justify-center gap-2">
                                        <UserIcon className="h-8 w-8 text-muted-foreground/50" />
                                        <p>No se encontraron usuarios.</p>
                                        {debouncedSearch && (
                                            <p className="text-xs">Intenta con otro término de búsqueda.</p>
                                        )}
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            users.map((user) => (
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
                                    onVerifyEmail={() => setEmailVerifiedMutation.mutate({ userId: user.id, verified: true })}
                                />
                            ))
                        )}
                    </TableBody>
                </Table>
            </Card>

            {/* Pagination */}
            {totalPages > 1 && (
                <div className="flex flex-col sm:flex-row items-center justify-between gap-4 pt-2">
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        <span>Mostrando {((page - 1) * pageSize) + 1} - {Math.min(page * pageSize, totalCount)} de {totalCount}</span>
                        <Select value={pageSize.toString()} onValueChange={(v) => { setPageSize(parseInt(v)); setPage(1); }}>
                            <SelectTrigger className="w-[100px] h-8">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="10">10 / pág</SelectItem>
                                <SelectItem value="25">25 / pág</SelectItem>
                                <SelectItem value="50">50 / pág</SelectItem>
                                <SelectItem value="100">100 / pág</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="flex items-center gap-1">
                        <Button
                            variant="outline"
                            size="sm"
                            className="h-8"
                            disabled={page === 1}
                            onClick={() => setPage(1)}
                        >
                            <ChevronsLeft className="h-4 w-4" />
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            className="h-8"
                            disabled={page === 1}
                            onClick={() => setPage(p => Math.max(1, p - 1))}
                        >
                            <ChevronLeft className="h-4 w-4" />
                        </Button>
                        <div className="flex items-center gap-1 px-2">
                            <span className="text-sm">Página</span>
                            <Input
                                type="number"
                                min={1}
                                max={totalPages}
                                value={page}
                                onChange={(e) => {
                                    const val = parseInt(e.target.value)
                                    if (val >= 1 && val <= totalPages) setPage(val)
                                }}
                                className="w-14 h-8 text-center"
                            />
                            <span className="text-sm">de {totalPages}</span>
                        </div>
                        <Button
                            variant="outline"
                            size="sm"
                            className="h-8"
                            disabled={page === totalPages}
                            onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                        >
                            <ChevronRight className="h-4 w-4" />
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            className="h-8"
                            disabled={page === totalPages}
                            onClick={() => setPage(totalPages)}
                        >
                            <ChevronsRight className="h-4 w-4" />
                        </Button>
                    </div>
                </div>
            )}

            {/* User Details Dialog */}
            <Dialog open={isDetailsOpen} onOpenChange={setIsDetailsOpen}>
                <DialogContent className="sm:max-w-2xl max-h-[85vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle>Detalles del Usuario</DialogTitle>
                        <DialogDescription>Información completa y actividad del usuario.</DialogDescription>
                    </DialogHeader>
                    {selectedUser && (
                        <UserDetails
                            user={selectedUser}
                            tenantId={tenantId}
                            token={token}
                            clients={clients || []}
                            fieldDefs={fieldDefs || []}
                            onUpdate={() => {
                                queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
                                setIsDetailsOpen(false)
                            }}
                            onEdit={() => {
                                setEditUser(selectedUser)
                                setIsDetailsOpen(false)
                                setIsEditOpen(true)
                            }}
                            onChangePassword={(newPassword) => {
                                changePasswordMutation.mutate({ userId: selectedUser.id, newPassword })
                            }}
                            onVerifyEmail={() => {
                                setEmailVerifiedMutation.mutate({ userId: selectedUser.id, verified: true })
                            }}
                        />
                    )}
                </DialogContent>
            </Dialog>

            {/* Edit User Dialog */}
            <Dialog open={isEditOpen} onOpenChange={(open) => { setIsEditOpen(open); if (!open) setEditUser(null); }}>
                <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle>Editar Usuario</DialogTitle>
                        <DialogDescription>
                            Modifica los datos del usuario. El email no puede ser cambiado.
                        </DialogDescription>
                    </DialogHeader>
                    {editUser && (
                        <EditUserForm
                            user={editUser}
                            fieldDefs={fieldDefs || []}
                            clients={clients || []}
                            onSubmit={(data) => updateMutation.mutate({ userId: editUser.id, data })}
                            isPending={updateMutation.isPending}
                        />
                    )}
                </DialogContent>
            </Dialog>

            {/* Block User Dialog */}
            {blockUser && (
                <BlockUserDialog
                    user={blockUser}
                    onClose={() => setBlockUser(null)}
                    onBlock={(userId, reason, duration) => {
                        if (!userId) {
                            toast({ title: "Error", description: "No se pudo identificar el usuario", variant: "destructive" })
                            return
                        }
                        blockMutation.mutate({ userId, reason, duration })
                    }}
                    isPending={blockMutation.isPending}
                />
            )}

            {/* Bulk Action Confirmation Dialog */}
            <Dialog open={isBulkActionOpen} onOpenChange={setIsBulkActionOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>
                            {bulkAction === "delete" ? "Eliminar usuarios" : "Bloquear usuarios"}
                        </DialogTitle>
                        <DialogDescription>
                            {bulkAction === "delete"
                                ? `¿Estás seguro de eliminar ${selectedIds.size} usuario(s)? Esta acción no se puede deshacer.`
                                : `¿Estás seguro de bloquear ${selectedIds.size} usuario(s)?`
                            }
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setIsBulkActionOpen(false)}>
                            Cancelar
                        </Button>
                        <Button
                            variant={bulkAction === "delete" ? "danger" : "default"}
                            onClick={() => {
                                const ids = Array.from(selectedIds)
                                if (bulkAction === "delete") {
                                    bulkDeleteMutation.mutate(ids)
                                } else {
                                    bulkBlockMutation.mutate(ids)
                                }
                            }}
                            disabled={bulkDeleteMutation.isPending || bulkBlockMutation.isPending}
                        >
                            {(bulkDeleteMutation.isPending || bulkBlockMutation.isPending) && (
                                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                            )}
                            {bulkAction === "delete" ? "Eliminar" : "Bloquear"}
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
    onVerifyEmail
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
    } catch (e) { }

    const initial = user.email ? user.email.slice(0, 2).toUpperCase() : "??"
    const isBlocked = !!user.disabled_at
    const isSuspended = !!user.disabled_until && new Date(user.disabled_until) > new Date()
    const displayBlocked = isBlocked || isSuspended

    return (
        <TableRow className={cn(isSelected && "bg-accent/5")}>
            <TableCell>
                <Checkbox checked={isSelected} onCheckedChange={onSelect} />
            </TableCell>
            <TableCell>
                <div className="h-9 w-9 bg-gradient-to-br from-accent-1/20 to-accent-2-clay/20 rounded-full flex items-center justify-center font-semibold text-xs text-accent-1 border-2 border-clay">
                    {initial}
                </div>
            </TableCell>
            <TableCell>
                <div className="flex flex-col">
                    <span className="font-medium truncate max-w-[200px]">{user.email}</span>
                    <span className="text-xs text-muted-foreground font-mono truncate max-w-[150px] opacity-70" title={user.id}>
                        {user.id}
                    </span>
                </div>
            </TableCell>
            <TableCell>
                {displayBlocked ? (
                    <div className="flex flex-col gap-0.5">
                        <Badge variant="destructive" className="h-5 text-[10px] px-1.5 w-fit">
                            {isSuspended ? "Suspendido" : "Deshabilitado"}
                        </Badge>
                        {isSuspended && user.disabled_until && (
                            <span className="text-[9px] text-danger font-mono">
                                Hasta: {new Date(user.disabled_until).toLocaleString("es-ES", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}
                            </span>
                        )}
                    </div>
                ) : (
                    <Badge variant="success" className="h-5 text-[10px] px-1.5 w-fit">
                        Activo
                    </Badge>
                )}
            </TableCell>
            <TableCell className="hidden lg:table-cell">
                {user.email_verified ? (
                    <div className="flex items-center gap-1 text-success">
                        <MailCheck className="h-4 w-4" />
                        <span className="text-xs font-medium">Verificado</span>
                    </div>
                ) : (
                    <div className="flex items-center gap-1 text-warning">
                        <MailX className="h-4 w-4" />
                        <span className="text-xs font-medium">Pendiente</span>
                    </div>
                )}
            </TableCell>
            <TableCell className="hidden md:table-cell text-sm text-muted-foreground">
                {dateStr}
            </TableCell>
            <TableCell className="text-right">
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
                            <span className="sr-only">Abrir menú</span>
                            <MoreHorizontal className="h-4 w-4" />
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-48">
                        <DropdownMenuLabel>Acciones</DropdownMenuLabel>
                        <DropdownMenuItem onClick={onDetails}>
                            <Eye className="mr-2 h-4 w-4" /> Ver Detalles
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={onEdit}>
                            <Pencil className="mr-2 h-4 w-4" /> Editar
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => {
                            if (user.id) navigator.clipboard.writeText(user.id)
                        }}>
                            <Copy className="mr-2 h-4 w-4" /> Copiar ID
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        {!user.email_verified && (
                            <DropdownMenuItem onClick={onVerifyEmail}>
                                <MailCheck className="mr-2 h-4 w-4 text-success" /> Marcar Verificado
                            </DropdownMenuItem>
                        )}
                        {displayBlocked ? (
                            <DropdownMenuItem onClick={onUnlock}>
                                <Unlock className="mr-2 h-4 w-4 text-success" /> Desbloquear
                            </DropdownMenuItem>
                        ) : (
                            <DropdownMenuItem onClick={onBlock}>
                                <Ban className="mr-2 h-4 w-4 text-warning" /> Bloquear
                            </DropdownMenuItem>
                        )}
                        <DropdownMenuSeparator />
                        <DropdownMenuItem onClick={onDelete} className="text-danger focus:text-danger focus:bg-danger/10">
                            <Trash2 className="mr-2 h-4 w-4" /> Eliminar
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            </TableCell>
        </TableRow>
    )
}

// ----------------------------------------------------------------------
// COMPONENT: Create User Form
// ----------------------------------------------------------------------

function CreateUserForm({ fieldDefs, clients, onSubmit, isPending }: {
    fieldDefs: UserFieldDefinition[]
    clients: Array<{ client_id: string, name: string, type: string }>
    onSubmit: (data: any) => void
    isPending: boolean
}) {
    const [email, setEmail] = useState("")
    const [password, setPassword] = useState("")
    const [sourceClientId, setSourceClientId] = useState<string>("_none")
    const [customFields, setCustomFields] = useState<Record<string, any>>({})
    const [name, setName] = useState("")

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()
        onSubmit({
            email,
            password,
            name: name || undefined,
            email_verified: true,
            custom_fields: Object.keys(customFields).length > 0 ? customFields : undefined,
            source_client_id: (!sourceClientId || sourceClientId === "_none") ? null : sourceClientId
        })
    }

    return (
        <form onSubmit={handleSubmit} className="space-y-6 py-2">
            <div className="grid gap-4">
                <div className="grid gap-2">
                    <Label htmlFor="email">Email <span className="text-danger">*</span></Label>
                    <Input
                        id="email"
                        type="email"
                        placeholder="usuario@ejemplo.com"
                        value={email}
                        onChange={e => setEmail(e.target.value)}
                        required
                    />
                </div>
                <div className="grid gap-2">
                    <Label htmlFor="name">Nombre (opcional)</Label>
                    <Input
                        id="name"
                        type="text"
                        placeholder="Juan Pérez"
                        value={name}
                        onChange={e => setName(e.target.value)}
                    />
                </div>
                <div className="grid gap-2">
                    <Label htmlFor="password">Contraseña <span className="text-danger">*</span></Label>
                    <Input
                        id="password"
                        type="password"
                        placeholder="••••••••"
                        value={password}
                        onChange={e => setPassword(e.target.value)}
                        required
                    />
                    <p className="text-xs text-muted-foreground">Mínimo 8 caracteres recomendado.</p>
                </div>
                <div className="grid gap-2">
                    <Label htmlFor="sourceClient">Cliente Origen (opcional)</Label>
                    <Select value={sourceClientId} onValueChange={setSourceClientId}>
                        <SelectTrigger>
                            <SelectValue placeholder="Seleccionar cliente..." />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="_none">Sin cliente asociado</SelectItem>
                            {clients.map((c) => (
                                <SelectItem key={c.client_id} value={c.client_id}>
                                    {c.name || c.client_id}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    <p className="text-xs text-muted-foreground">
                        Asociar a un cliente permite enviar emails de verificación con el redirect correcto.
                    </p>
                </div>
            </div>

            {fieldDefs.length > 0 && (
                <div className="space-y-4 pt-2">
                    <div className="flex items-center">
                        <div className="h-px flex-1 bg-border" />
                        <span className="px-2 text-xs font-semibold text-muted-foreground uppercase">Campos Personalizados</span>
                        <div className="h-px flex-1 bg-border" />
                    </div>

                    {fieldDefs.map((field) => (
                        <div key={field.name} className="grid gap-2">
                            <Label htmlFor={field.name}>
                                {field.name} {field.required && <span className="text-danger">*</span>}
                            </Label>
                            {field.type === "phone" ? (
                                <PhoneInput
                                    value={customFields[field.name] as string}
                                    onChange={(val) => setCustomFields({ ...customFields, [field.name]: val })}
                                />
                            ) : field.type === "country" ? (
                                <CountrySelect
                                    value={customFields[field.name] as string}
                                    onChange={(val) => setCustomFields({ ...customFields, [field.name]: val })}
                                />
                            ) : field.type === "boolean" ? (
                                <div className="flex items-center gap-2">
                                    <Switch
                                        checked={!!customFields[field.name]}
                                        onCheckedChange={(c) => setCustomFields({ ...customFields, [field.name]: c })}
                                    />
                                    <span className="text-sm text-muted-foreground">
                                        {customFields[field.name] ? "Sí" : "No"}
                                    </span>
                                </div>
                            ) : (
                                <Input
                                    id={field.name}
                                    type={field.type === "number" || field.type === "int" ? "number" : field.type === "date" ? "date" : "text"}
                                    value={customFields[field.name] || ""}
                                    onChange={(e) => setCustomFields({ ...customFields, [field.name]: e.target.value })}
                                    required={field.required}
                                    placeholder={`Ingresa ${field.name}`}
                                />
                            )}
                            {field.description && (
                                <p className="text-xs text-muted-foreground">{field.description}</p>
                            )}
                        </div>
                    ))}
                </div>
            )}

            <DialogFooter className="pt-4">
                <Button type="submit" disabled={isPending} className="w-full sm:w-auto">
                    {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                    Crear Usuario
                </Button>
            </DialogFooter>
        </form>
    )
}

// ----------------------------------------------------------------------
// COMPONENT: Edit User Form
// ----------------------------------------------------------------------

function EditUserForm({ user, fieldDefs, clients, onSubmit, isPending }: {
    user: UserType
    fieldDefs: UserFieldDefinition[]
    clients: Array<{ client_id: string, name: string, type: string }>
    onSubmit: (data: any) => void
    isPending: boolean
}) {
    const [name, setName] = useState(user.name || "")
    const [givenName, setGivenName] = useState((user as any).given_name || "")
    const [familyName, setFamilyName] = useState((user as any).family_name || "")
    const [locale, setLocale] = useState((user as any).locale || "")
    const [sourceClientId, setSourceClientId] = useState<string>(user.source_client_id || "_none")
    const [customFields, setCustomFields] = useState<Record<string, any>>(user.custom_fields || {})

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()
        onSubmit({
            name: name || undefined,
            given_name: givenName || undefined,
            family_name: familyName || undefined,
            locale: locale || undefined,
            source_client_id: (!sourceClientId || sourceClientId === "_none") ? null : sourceClientId,
            custom_fields: Object.keys(customFields).length > 0 ? customFields : undefined,
        })
    }

    return (
        <form onSubmit={handleSubmit} className="space-y-6 py-2">
            {/* Read-only Email */}
            <div className="grid gap-2">
                <Label>Email</Label>
                <div className="flex items-center gap-2 p-2 bg-muted/50 rounded-clay border border-clay">
                    <Mail className="h-4 w-4 text-muted-foreground" />
                    <span className="text-sm font-medium">{user.email}</span>
                    <Badge variant="outline" className="ml-auto text-[10px]">No editable</Badge>
                </div>
            </div>

            <div className="grid gap-4">
                <div className="grid grid-cols-2 gap-4">
                    <div className="grid gap-2">
                        <Label htmlFor="givenName">Nombre</Label>
                        <Input
                            id="givenName"
                            placeholder="Juan"
                            value={givenName}
                            onChange={e => setGivenName(e.target.value)}
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="familyName">Apellido</Label>
                        <Input
                            id="familyName"
                            placeholder="Pérez"
                            value={familyName}
                            onChange={e => setFamilyName(e.target.value)}
                        />
                    </div>
                </div>
                <div className="grid gap-2">
                    <Label htmlFor="name">Nombre Completo</Label>
                    <Input
                        id="name"
                        placeholder="Juan Pérez"
                        value={name}
                        onChange={e => setName(e.target.value)}
                    />
                </div>
                <div className="grid gap-2">
                    <Label htmlFor="locale">Idioma/Locale</Label>
                    <Select value={locale || "_unset"} onValueChange={(v) => setLocale(v === "_unset" ? "" : v)}>
                        <SelectTrigger>
                            <SelectValue placeholder="Seleccionar idioma..." />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="_unset">Sin especificar</SelectItem>
                            <SelectItem value="es">Español (es)</SelectItem>
                            <SelectItem value="en">English (en)</SelectItem>
                            <SelectItem value="pt">Português (pt)</SelectItem>
                            <SelectItem value="fr">Français (fr)</SelectItem>
                            <SelectItem value="de">Deutsch (de)</SelectItem>
                        </SelectContent>
                    </Select>
                </div>
                <div className="grid gap-2">
                    <Label htmlFor="sourceClient">Cliente Origen</Label>
                    <Select value={sourceClientId} onValueChange={setSourceClientId}>
                        <SelectTrigger>
                            <SelectValue placeholder="Seleccionar cliente..." />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="_none">Sin cliente asociado</SelectItem>
                            {clients.map((c) => (
                                <SelectItem key={c.client_id} value={c.client_id}>
                                    {c.name || c.client_id}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>
            </div>

            {fieldDefs.length > 0 && (
                <div className="space-y-4 pt-2">
                    <div className="flex items-center">
                        <div className="h-px flex-1 bg-border" />
                        <span className="px-2 text-xs font-semibold text-muted-foreground uppercase">Campos Personalizados</span>
                        <div className="h-px flex-1 bg-border" />
                    </div>

                    {fieldDefs.map((field) => (
                        <div key={field.name} className="grid gap-2">
                            <Label htmlFor={`edit-${field.name}`}>
                                {field.name} {field.required && <span className="text-danger">*</span>}
                            </Label>
                            {field.type === "phone" ? (
                                <PhoneInput
                                    value={customFields[field.name] as string}
                                    onChange={(val) => setCustomFields({ ...customFields, [field.name]: val })}
                                />
                            ) : field.type === "country" ? (
                                <CountrySelect
                                    value={customFields[field.name] as string}
                                    onChange={(val) => setCustomFields({ ...customFields, [field.name]: val })}
                                />
                            ) : field.type === "boolean" ? (
                                <div className="flex items-center gap-2">
                                    <Switch
                                        checked={!!customFields[field.name]}
                                        onCheckedChange={(c) => setCustomFields({ ...customFields, [field.name]: c })}
                                    />
                                    <span className="text-sm text-muted-foreground">
                                        {customFields[field.name] ? "Sí" : "No"}
                                    </span>
                                </div>
                            ) : (
                                <Input
                                    id={`edit-${field.name}`}
                                    type={field.type === "number" || field.type === "int" ? "number" : field.type === "date" ? "date" : "text"}
                                    value={customFields[field.name] || ""}
                                    onChange={(e) => setCustomFields({ ...customFields, [field.name]: e.target.value })}
                                    required={field.required}
                                    placeholder={`Ingresa ${field.name}`}
                                />
                            )}
                        </div>
                    ))}
                </div>
            )}

            <DialogFooter className="pt-4">
                <Button type="submit" disabled={isPending} className="w-full sm:w-auto">
                    {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                    Guardar Cambios
                </Button>
            </DialogFooter>
        </form>
    )
}

// ----------------------------------------------------------------------
// COMPONENT: User Details with Activity Tab
// ----------------------------------------------------------------------

function UserDetails({
    user,
    tenantId,
    token,
    clients,
    fieldDefs,
    onUpdate,
    onEdit,
    onChangePassword,
    onVerifyEmail
}: {
    user: UserType
    tenantId: string
    token: string | null
    clients: Array<{ client_id: string, name: string, type: string }>
    fieldDefs: UserFieldDefinition[]
    onUpdate: () => void
    onEdit: () => void
    onChangePassword: (newPassword: string) => void
    onVerifyEmail: () => void
}) {
    const { toast } = useToast()
    const [activeTab, setActiveTab] = useState("info")
    const [isResending, setIsResending] = useState(false)
    const [showPasswordChange, setShowPasswordChange] = useState(false)
    const [newPassword, setNewPassword] = useState("")
    const [confirmPassword, setConfirmPassword] = useState("")

    const formatDate = (dateStr?: string) => {
        if (!dateStr) return "-"
        try {
            const d = new Date(dateStr)
            if (isNaN(d.getTime())) return "-"
            return new Intl.DateTimeFormat("es-ES", {
                year: "numeric",
                month: "long",
                day: "numeric",
                hour: "2-digit",
                minute: "2-digit"
            }).format(d)
        } catch { return "-" }
    }

    const initial = user.email ? user.email.slice(0, 2).toUpperCase() : "??"
    const isBlocked = !!user.disabled_at

    const handleResendVerification = async () => {
        if (!user.id || !tenantId) return
        setIsResending(true)
        try {
            const res = await apiFetchWithTenant(`/v2/admin/users/resend-verification`, tenantId, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    "Authorization": `Bearer ${token}`
                },
                body: JSON.stringify({ user_id: user.id, tenant_id: tenantId })
            })
            if (!res.ok) {
                const err = await res.json().catch(() => ({}))
                throw new Error(err.error_description || err.error || "Error enviando email")
            }
            toast({ title: "Email enviado", description: "Se ha enviado un nuevo correo de verificación." })
        } catch (err: any) {
            toast({ title: "Error", description: err.message, variant: "destructive" })
        } finally {
            setIsResending(false)
        }
    }

    const handlePasswordChange = () => {
        if (newPassword !== confirmPassword) {
            toast({ title: "Error", description: "Las contraseñas no coinciden", variant: "destructive" })
            return
        }
        if (newPassword.length < 8) {
            toast({ title: "Error", description: "La contraseña debe tener al menos 8 caracteres", variant: "destructive" })
            return
        }
        onChangePassword(newPassword)
        setShowPasswordChange(false)
        setNewPassword("")
        setConfirmPassword("")
    }

    // Mock activity data (would come from backend in real implementation)
    const activityData = [
        { type: "login_success", date: new Date(Date.now() - 3600000), ip: "192.168.1.1", device: "Chrome / Windows" },
        { type: "password_changed", date: new Date(Date.now() - 86400000 * 3), ip: "192.168.1.1", device: "Firefox / MacOS" },
        { type: "login_failed", date: new Date(Date.now() - 86400000 * 5), ip: "10.0.0.1", device: "Safari / iOS" },
        { type: "account_created", date: new Date(user.created_at), ip: "-", device: "-" },
    ]

    return (
        <div className="space-y-4">
            {/* User Header */}
            <div className="flex items-center gap-4 p-4 bg-gradient-to-r from-accent-1/10 to-accent-2-clay/10 rounded-clay border-2 border-clay shadow-clay-card">
                <div className="h-16 w-16 bg-gradient-to-br from-accent-1/20 to-accent-2-clay/20 rounded-full flex items-center justify-center text-2xl font-bold text-accent-1 border-2 border-clay">
                    {initial}
                </div>
                <div className="flex-1 min-w-0">
                    <h3 className="text-lg font-semibold truncate">{user.email}</h3>
                    <div className="flex items-center gap-2 mt-1">
                        <Badge variant="outline" className="font-mono text-[10px]">{user.id}</Badge>
                        <Button variant="ghost" size="sm" className="h-5 w-5 p-0" onClick={() => navigator.clipboard.writeText(user.id)}>
                            <Copy className="h-3 w-3" />
                        </Button>
                    </div>
                    <div className="flex items-center gap-2 mt-2">
                        {isBlocked ? (
                            <Badge variant="destructive" className="text-xs">Bloqueado</Badge>
                        ) : (
                            <Badge variant="success" className="text-xs">Activo</Badge>
                        )}
                        {user.email_verified ? (
                            <Badge variant="outline" className="text-xs text-success border-success">
                                <MailCheck className="h-3 w-3 mr-1" /> Verificado
                            </Badge>
                        ) : (
                            <Badge variant="outline" className="text-xs text-warning border-warning">
                                <MailX className="h-3 w-3 mr-1" /> No verificado
                            </Badge>
                        )}
                    </div>
                </div>
                <Button variant="outline" size="sm" onClick={onEdit}>
                    <Pencil className="h-4 w-4 mr-2" />
                    Editar
                </Button>
            </div>

            {/* Tabs */}
            <Tabs value={activeTab} onValueChange={setActiveTab}>
                <TabsList className="grid w-full grid-cols-3">
                    <TabsTrigger value="info" className="text-xs">
                        <UserIcon className="h-3.5 w-3.5 mr-1" />
                        Información
                    </TabsTrigger>
                    <TabsTrigger value="security" className="text-xs">
                        <Shield className="h-3.5 w-3.5 mr-1" />
                        Seguridad
                    </TabsTrigger>
                    <TabsTrigger value="activity" className="text-xs">
                        <Activity className="h-3.5 w-3.5 mr-1" />
                        Actividad
                    </TabsTrigger>
                </TabsList>

                {/* Info Tab */}
                <TabsContent value="info" className="space-y-4 mt-4">
                    <div className="grid grid-cols-2 gap-4">
                        <div className="space-y-1">
                            <Label className="text-xs text-muted-foreground">Nombre</Label>
                            <p className="text-sm font-medium">{user.name || "-"}</p>
                        </div>
                        <div className="space-y-1">
                            <Label className="text-xs text-muted-foreground">Locale</Label>
                            <p className="text-sm font-medium">{(user as any).locale || "-"}</p>
                        </div>
                        <div className="space-y-1">
                            <Label className="text-xs text-muted-foreground">Cliente Origen</Label>
                            <p className="text-sm font-medium font-mono">
                                {clients.find(c => c.client_id === user.source_client_id)?.name || user.source_client_id || "-"}
                            </p>
                        </div>
                        <div className="space-y-1">
                            <Label className="text-xs text-muted-foreground">Creado</Label>
                            <p className="text-sm font-medium">{formatDate(user.created_at)}</p>
                        </div>
                    </div>

                    {/* Custom Fields */}
                    {user.custom_fields && Object.keys(user.custom_fields).length > 0 && (
                        <div className="space-y-2 pt-2">
                            <Label className="text-xs text-muted-foreground uppercase">Campos Personalizados</Label>
                            <div className="grid gap-2">
                                {Object.entries(user.custom_fields)
                                    .filter(([key]) => key !== 'updated_at')
                                    .map(([key, value]) => (
                                        <div key={key} className="flex items-center justify-between p-2 rounded-clay bg-muted/50 border border-clay">
                                            <span className="text-xs font-medium text-muted-foreground">{key}</span>
                                            <span className="text-sm font-medium">
                                                {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                                            </span>
                                        </div>
                                    ))}
                            </div>
                        </div>
                    )}

                    {/* Metadata */}
                    {user.metadata && Object.keys(user.metadata).length > 0 && (
                        <div className="space-y-2 pt-2">
                            <Label className="text-xs text-muted-foreground uppercase">Metadatos</Label>
                            <pre className="text-xs font-mono p-3 bg-muted/50 rounded-clay border border-clay overflow-x-auto">
                                {JSON.stringify(user.metadata, null, 2)}
                            </pre>
                        </div>
                    )}
                </TabsContent>

                {/* Security Tab */}
                <TabsContent value="security" className="space-y-4 mt-4">
                    {/* Email Verification */}
                    <div className="p-4 rounded-clay border-2 border-clay shadow-clay-card bg-card/80 backdrop-blur-sm">
                        <div className="flex items-center justify-between">
                            <div className="flex items-center gap-3">
                                {user.email_verified ? (
                                    <div className="p-2 bg-success/10 rounded-clay">
                                        <MailCheck className="h-5 w-5 text-success" />
                                    </div>
                                ) : (
                                    <div className="p-2 bg-warning/10 rounded-clay">
                                        <MailX className="h-5 w-5 text-warning" />
                                    </div>
                                )}
                                <div>
                                    <h4 className="font-medium">Verificación de Email</h4>
                                    <p className="text-sm text-muted-foreground">
                                        {user.email_verified ? "El email ha sido verificado" : "El email aún no ha sido verificado"}
                                    </p>
                                </div>
                            </div>
                            {!user.email_verified && (
                                <div className="flex items-center gap-2">
                                    <Button variant="outline" size="sm" onClick={onVerifyEmail}>
                                        <CheckCircle className="h-4 w-4 mr-2" />
                                        Marcar verificado
                                    </Button>
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        onClick={handleResendVerification}
                                        disabled={isResending || !user.source_client_id}
                                    >
                                        {isResending ? <Loader2 className="h-4 w-4 mr-2 animate-spin" /> : <Mail className="h-4 w-4 mr-2" />}
                                        Reenviar
                                    </Button>
                                </div>
                            )}
                        </div>
                    </div>

                    {/* Password Change */}
                    <div className="p-4 rounded-clay border-2 border-clay shadow-clay-card bg-card/80 backdrop-blur-sm">
                        <div className="flex items-center justify-between">
                            <div className="flex items-center gap-3">
                                <div className="p-2 bg-accent-1/10 rounded-clay">
                                    <Key className="h-5 w-5 text-accent-1" />
                                </div>
                                <div>
                                    <h4 className="font-medium">Contraseña</h4>
                                    <p className="text-sm text-muted-foreground">Cambiar la contraseña del usuario</p>
                                </div>
                            </div>
                            <Button variant="outline" size="sm" onClick={() => setShowPasswordChange(!showPasswordChange)}>
                                <Key className="h-4 w-4 mr-2" />
                                Cambiar
                            </Button>
                        </div>
                        {showPasswordChange && (
                            <div className="mt-4 pt-4 border-t space-y-3">
                                <div className="grid gap-2">
                                    <Label htmlFor="newPassword">Nueva contraseña</Label>
                                    <Input
                                        id="newPassword"
                                        type="password"
                                        placeholder="••••••••"
                                        value={newPassword}
                                        onChange={(e) => setNewPassword(e.target.value)}
                                    />
                                </div>
                                <div className="grid gap-2">
                                    <Label htmlFor="confirmPassword">Confirmar contraseña</Label>
                                    <Input
                                        id="confirmPassword"
                                        type="password"
                                        placeholder="••••••••"
                                        value={confirmPassword}
                                        onChange={(e) => setConfirmPassword(e.target.value)}
                                    />
                                </div>
                                <div className="flex justify-end gap-2">
                                    <Button variant="ghost" size="sm" onClick={() => {
                                        setShowPasswordChange(false)
                                        setNewPassword("")
                                        setConfirmPassword("")
                                    }}>
                                        Cancelar
                                    </Button>
                                    <Button size="sm" onClick={handlePasswordChange} disabled={!newPassword || !confirmPassword}>
                                        Guardar
                                    </Button>
                                </div>
                            </div>
                        )}
                    </div>

                    {/* Block Status */}
                    {isBlocked && (
                        <Alert variant="destructive">
                            <ShieldAlert className="h-4 w-4" />
                            <AlertTitle>Usuario bloqueado</AlertTitle>
                            <AlertDescription>
                                {user.disabled_reason && <p>Razón: {user.disabled_reason}</p>}
                                {user.disabled_until && (
                                    <p>Hasta: {formatDate(user.disabled_until)}</p>
                                )}
                            </AlertDescription>
                        </Alert>
                    )}
                </TabsContent>

                {/* Activity Tab */}
                <TabsContent value="activity" className="space-y-4 mt-4">
                    <div className="text-sm text-muted-foreground mb-2">
                        Historial de actividad reciente del usuario.
                    </div>
                    <div className="space-y-2">
                        {activityData.map((activity, idx) => (
                            <div key={idx} className="flex items-start gap-3 p-3 rounded-clay border-2 border-clay bg-card/80 backdrop-blur-sm hover:shadow-clay-card transition-all">
                                <div className={cn(
                                    "p-1.5 rounded-full",
                                    activity.type === "login_success" && "bg-success/10",
                                    activity.type === "login_failed" && "bg-danger/10",
                                    activity.type === "password_changed" && "bg-accent-1/10",
                                    activity.type === "account_created" && "bg-accent-2-clay/10",
                                )}>
                                    {activity.type === "login_success" && <CheckCircle className="h-4 w-4 text-success" />}
                                    {activity.type === "login_failed" && <XCircle className="h-4 w-4 text-danger" />}
                                    {activity.type === "password_changed" && <Key className="h-4 w-4 text-accent-1" />}
                                    {activity.type === "account_created" && <UserIcon className="h-4 w-4 text-accent-2-clay" />}
                                </div>
                                <div className="flex-1 min-w-0">
                                    <p className="text-sm font-medium">
                                        {activity.type === "login_success" && "Inicio de sesión exitoso"}
                                        {activity.type === "login_failed" && "Intento de inicio de sesión fallido"}
                                        {activity.type === "password_changed" && "Contraseña cambiada"}
                                        {activity.type === "account_created" && "Cuenta creada"}
                                    </p>
                                    <div className="flex items-center gap-3 text-xs text-muted-foreground mt-0.5">
                                        <span className="flex items-center gap-1">
                                            <Clock className="h-3 w-3" />
                                            {formatDate(activity.date.toISOString())}
                                        </span>
                                        {activity.ip !== "-" && (
                                            <span>IP: {activity.ip}</span>
                                        )}
                                        {activity.device !== "-" && (
                                            <span>{activity.device}</span>
                                        )}
                                    </div>
                                </div>
                            </div>
                        ))}
                    </div>
                    <p className="text-xs text-muted-foreground text-center pt-2">
                        * El historial de actividad requiere configuración adicional del backend para datos en tiempo real.
                    </p>
                </TabsContent>
            </Tabs>
        </div>
    )
}

// ----------------------------------------------------------------------
// COMPONENT: Block User Dialog
// ----------------------------------------------------------------------

function BlockUserDialog({ user, onClose, onBlock, isPending }: {
    user: UserType
    onClose: () => void
    onBlock: (userId: string, reason: string, duration: string) => void
    isPending: boolean
}) {
    const [reason, setReason] = useState("")
    const [duration, setDuration] = useState("permanent")

    return (
        <Dialog open={true} onOpenChange={onClose}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Bloquear Usuario</DialogTitle>
                    <DialogDescription>
                        Suspende el acceso de <span className="font-medium">{user.email}</span>
                    </DialogDescription>
                </DialogHeader>
                <div className="grid gap-4 py-4">
                    <div className="grid gap-2">
                        <Label>Motivo <span className="text-danger">*</span></Label>
                        <Input
                            placeholder="Ej. Violación de términos de servicio"
                            value={reason}
                            onChange={e => setReason(e.target.value)}
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label>Duración</Label>
                        <Select value={duration} onValueChange={setDuration}>
                            <SelectTrigger>
                                <SelectValue placeholder="Selecciona duración" />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="permanent">Permanente</SelectItem>
                                <SelectItem value="1h">1 hora</SelectItem>
                                <SelectItem value="6h">6 horas</SelectItem>
                                <SelectItem value="12h">12 horas</SelectItem>
                                <SelectItem value="24h">24 horas</SelectItem>
                                <SelectItem value="72h">3 días</SelectItem>
                                <SelectItem value="168h">7 días</SelectItem>
                                <SelectItem value="720h">30 días</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                </div>
                <DialogFooter>
                    <Button variant="outline" onClick={onClose} disabled={isPending}>Cancelar</Button>
                    <Button
                        variant="danger"
                        onClick={() => onBlock(user.id, reason, duration === "permanent" ? "" : duration)}
                        disabled={isPending || !reason.trim()}
                    >
                        {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        Bloquear
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

// ----------------------------------------------------------------------
// COMPONENT: User Fields Settings
// ----------------------------------------------------------------------

function UserFieldsSettings({ tenantId }: { tenantId: string }) {
    const { token } = useAuthStore()
    const { toast } = useToast()
    const queryClient = useQueryClient()
    const { t } = useI18n()

    const [isFieldsDialogOpen, setIsFieldsDialogOpen] = useState(false)
    const [editingFieldIdx, setEditingFieldIdx] = useState<number | null>(null)
    const [fieldForm, setFieldForm] = useState<UserFieldDefinition>({
        name: "",
        type: "text",
        required: false,
        unique: false,
        indexed: false,
        description: "",
    })
    const [localFields, setLocalFields] = useState<UserFieldDefinition[]>([])
    const [hasFieldChanges, setHasFieldChanges] = useState(false)

    const {
        data: settingsWithEtag,
        isLoading,
        isError,
        error,
    } = useQuery<TenantSettings & { _etag?: string }>({
        queryKey: ["tenant-users-settings", tenantId],
        queryFn: async () => {
            const { data, headers } = await api.getWithHeaders<TenantSettings>(`/v2/admin/tenants/${tenantId}/settings`)
            const etagHeader = headers.get("ETag")
            return { ...data, _etag: etagHeader || undefined }
        },
        enabled: !!tenantId && !!token,
    })

    const settings = settingsWithEtag ? (() => {
        const { _etag, ...rest } = settingsWithEtag
        return rest as TenantSettings
    })() : undefined

    useEffect(() => {
        if (settings) {
            setLocalFields(settings.userFields || [])
            setHasFieldChanges(false)
        }
    }, [settingsWithEtag])

    const updateSettingsMutation = useMutation({
        mutationFn: async (data: TenantSettings) => {
            const etag = settingsWithEtag?._etag
            if (!etag) {
                throw new Error("Missing ETag. Please refresh the page.")
            }
            const currentSettings = settings || {}
            const payload = { ...currentSettings, ...data }
            await api.put(`/v2/admin/tenants/${tenantId}/settings`, payload, etag)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["tenant-users-settings", tenantId] })
            toast({ title: t("common.success"), description: "Campos actualizados correctamente.", variant: "info" })
            setHasFieldChanges(false)
        },
        onError: (err: any) => {
            toast({ variant: "destructive", title: t("common.error"), description: err?.error_description || err?.message || "Error updating settings" })
        },
    })

    const handleOpenFieldDialog = (field?: UserFieldDefinition, idx?: number) => {
        if (field) {
            setFieldForm(field)
            setEditingFieldIdx(idx ?? null)
        } else {
            setFieldForm({ name: "", type: "text", required: false, unique: false, indexed: false, description: "" })
            setEditingFieldIdx(null)
        }
        setIsFieldsDialogOpen(true)
    }

    const handleLocalSaveField = () => {
        if (!fieldForm.name || !fieldForm.type) {
            toast({ variant: "destructive", title: t("common.error"), description: "Nombre y tipo son requeridos" })
            return
        }

        const nameExists = localFields.some((f, i) =>
            f.name.toLowerCase() === fieldForm.name.toLowerCase() && i !== editingFieldIdx
        )

        if (nameExists) {
            toast({ variant: "destructive", title: t("common.error"), description: "El nombre del campo ya existe" })
            return
        }

        let newFields = [...localFields]
        if (editingFieldIdx !== null) {
            newFields[editingFieldIdx] = fieldForm
        } else {
            newFields.push(fieldForm)
        }

        setLocalFields(newFields)
        setHasFieldChanges(true)
        setIsFieldsDialogOpen(false)
    }

    const handleLocalDeleteField = (idx: number) => {
        const newFields = localFields.filter((_, i) => i !== idx)
        setLocalFields(newFields)
        setHasFieldChanges(true)
    }

    const handleSaveAllFields = () => {
        updateSettingsMutation.mutate({ userFields: localFields } as any)
    }

    const handleCancelFieldChanges = () => {
        setLocalFields(settings?.userFields || [])
        setHasFieldChanges(false)
    }

    if (isError) {
        return (
            <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertTitle>{t("common.error")}</AlertTitle>
                <AlertDescription>
                    {t("tenants.notFound")} - {(error as any)?.message || JSON.stringify(error)}
                </AlertDescription>
            </Alert>
        )
    }

    return (
        <Card interactive>
            <CardHeader>
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <div className="p-2 bg-accent-1/10 rounded-clay">
                            <Settings2 className="h-5 w-5 text-accent-1" />
                        </div>
                        <div>
                            <CardTitle>Campos de Usuario</CardTitle>
                            <CardDescription>
                                Campos personalizados que se solicitan durante el registro.
                            </CardDescription>
                        </div>
                    </div>
                    <Button size="sm" onClick={() => handleOpenFieldDialog()}>
                        <Plus className="mr-2 h-4 w-4" />
                        Agregar Campo
                    </Button>
                </div>
            </CardHeader>
            <CardContent>
                {isLoading ? (
                    <div className="rounded-clay border-2 border-clay overflow-hidden">
                        <Table>
                            <TableHeader>
                                <TableRow className="bg-muted/30">
                                    <TableHead><Skeleton className="h-4 w-16" /></TableHead>
                                    <TableHead><Skeleton className="h-4 w-12" /></TableHead>
                                    <TableHead className="text-center"><Skeleton className="h-4 w-16 mx-auto" /></TableHead>
                                    <TableHead className="text-center"><Skeleton className="h-4 w-12 mx-auto" /></TableHead>
                                    <TableHead className="text-center"><Skeleton className="h-4 w-16 mx-auto" /></TableHead>
                                    <TableHead className="text-right"><Skeleton className="h-4 w-16 ml-auto" /></TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {[1, 2, 3].map((i) => (
                                    <TableRow key={i}>
                                        <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                        <TableCell><Skeleton className="h-6 w-16 rounded-full" /></TableCell>
                                        <TableCell className="text-center"><Skeleton className="h-4 w-4 mx-auto rounded" /></TableCell>
                                        <TableCell className="text-center"><Skeleton className="h-4 w-4 mx-auto rounded" /></TableCell>
                                        <TableCell className="text-center"><Skeleton className="h-4 w-4 mx-auto rounded" /></TableCell>
                                        <TableCell className="text-right"><Skeleton className="h-8 w-8 ml-auto rounded" /></TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </div>
                ) : (
                    <div className="rounded-clay border-2 border-clay overflow-hidden">
                        <Table>
                            <TableHeader>
                                <TableRow className="bg-muted/30">
                                    <TableHead>Nombre</TableHead>
                                    <TableHead>Tipo</TableHead>
                                    <TableHead className="text-center">Requerido</TableHead>
                                    <TableHead className="text-center">Único</TableHead>
                                    <TableHead className="text-center">Indexado</TableHead>
                                    <TableHead className="text-right">Acciones</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {localFields.length === 0 ? (
                                    <TableRow>
                                        <TableCell colSpan={6} className="h-24 text-center text-muted-foreground">
                                            <div className="flex flex-col items-center gap-2">
                                                <Sliders className="h-8 w-8 text-muted-foreground/50" />
                                                <p>No hay campos personalizados definidos.</p>
                                                <Button variant="outline" size="sm" onClick={() => handleOpenFieldDialog()}>
                                                    <Plus className="h-4 w-4 mr-2" />
                                                    Crear primer campo
                                                </Button>
                                            </div>
                                        </TableCell>
                                    </TableRow>
                                ) : (
                                    localFields.map((field, idx) => (
                                        <TableRow key={idx}>
                                            <TableCell className="font-medium">{field.name}</TableCell>
                                            <TableCell>
                                                <Badge variant="outline">{field.type}</Badge>
                                            </TableCell>
                                            <TableCell className="text-center">
                                                {field.required && <CheckCircle2 className="h-4 w-4 mx-auto text-success" />}
                                            </TableCell>
                                            <TableCell className="text-center">
                                                {field.unique && <CheckCircle2 className="h-4 w-4 mx-auto text-accent-1" />}
                                            </TableCell>
                                            <TableCell className="text-center">
                                                {field.indexed && <CheckCircle2 className="h-4 w-4 mx-auto text-muted-foreground" />}
                                            </TableCell>
                                            <TableCell className="text-right">
                                                <div className="flex justify-end gap-1">
                                                    <Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={() => handleOpenFieldDialog(field, idx)}>
                                                        <Edit2 className="h-4 w-4" />
                                                    </Button>
                                                    <Button variant="ghost" size="sm" className="h-8 w-8 p-0 text-danger hover:text-danger" onClick={() => handleLocalDeleteField(idx)}>
                                                        <Trash2 className="h-4 w-4" />
                                                    </Button>
                                                </div>
                                            </TableCell>
                                        </TableRow>
                                    ))
                                )}
                            </TableBody>
                        </Table>
                    </div>
                )}
            </CardContent>
            {hasFieldChanges && (
                <CardFooter className="flex items-center justify-between border-t bg-muted/30 py-3">
                    <p className="text-sm text-muted-foreground">
                        Tienes cambios sin guardar
                    </p>
                    <div className="flex items-center gap-2">
                        <Button variant="ghost" size="sm" onClick={handleCancelFieldChanges}>
                            Cancelar
                        </Button>
                        <Button size="sm" onClick={handleSaveAllFields} disabled={updateSettingsMutation.isPending}>
                            {updateSettingsMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                            Guardar Cambios
                        </Button>
                    </div>
                </CardFooter>
            )}

            {/* Field Dialog */}
            <Dialog open={isFieldsDialogOpen} onOpenChange={setIsFieldsDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{editingFieldIdx !== null ? "Editar Campo" : "Nuevo Campo"}</DialogTitle>
                        <DialogDescription>
                            Configura las propiedades del campo personalizado.
                        </DialogDescription>
                    </DialogHeader>
                    <div className="grid gap-4 py-4">
                        <div className="grid grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label>Nombre</Label>
                                <Input
                                    placeholder="ej. telefono"
                                    value={fieldForm.name}
                                    onChange={(e) => setFieldForm({ ...fieldForm, name: e.target.value })}
                                    disabled={editingFieldIdx !== null}
                                />
                                {editingFieldIdx !== null && (
                                    <p className="text-xs text-muted-foreground">El nombre no se puede cambiar.</p>
                                )}
                            </div>
                            <div className="space-y-2">
                                <Label>Tipo</Label>
                                <Select
                                    value={fieldForm.type}
                                    onValueChange={(val) => setFieldForm({ ...fieldForm, type: val })}
                                    disabled={editingFieldIdx !== null}
                                >
                                    <SelectTrigger>
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="text">Texto</SelectItem>
                                        <SelectItem value="int">Número Entero</SelectItem>
                                        <SelectItem value="boolean">Booleano</SelectItem>
                                        <SelectItem value="date">Fecha/Hora</SelectItem>
                                        <SelectItem value="phone">Teléfono</SelectItem>
                                        <SelectItem value="country">País</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>
                        <div className="space-y-2">
                            <Label>Descripción (opcional)</Label>
                            <Textarea
                                placeholder="Descripción del campo para mostrar al usuario..."
                                value={fieldForm.description || ""}
                                onChange={(e) => setFieldForm({ ...fieldForm, description: e.target.value })}
                            />
                        </div>
                        <div className="space-y-3 p-4 rounded-clay border-2 border-clay bg-muted/20">
                            <div className="flex items-center justify-between">
                                <div>
                                    <Label>Requerido</Label>
                                    <p className="text-xs text-muted-foreground">El campo será obligatorio</p>
                                </div>
                                <Switch
                                    checked={fieldForm.required}
                                    onCheckedChange={(c) => setFieldForm({ ...fieldForm, required: c })}
                                />
                            </div>
                            <div className="flex items-center justify-between">
                                <div>
                                    <Label>Único</Label>
                                    <p className="text-xs text-muted-foreground">No permitir valores duplicados</p>
                                </div>
                                <Switch
                                    checked={fieldForm.unique}
                                    onCheckedChange={(c) => setFieldForm({ ...fieldForm, unique: c })}
                                />
                            </div>
                            <div className="flex items-center justify-between">
                                <div>
                                    <Label>Indexado</Label>
                                    <p className="text-xs text-muted-foreground">Mejora búsquedas por este campo</p>
                                </div>
                                <Switch
                                    checked={fieldForm.indexed}
                                    onCheckedChange={(c) => setFieldForm({ ...fieldForm, indexed: c })}
                                />
                            </div>
                        </div>
                    </div>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setIsFieldsDialogOpen(false)}>Cancelar</Button>
                        <Button onClick={handleLocalSaveField}>
                            {editingFieldIdx !== null ? "Guardar" : "Agregar"}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </Card>
    )
}
