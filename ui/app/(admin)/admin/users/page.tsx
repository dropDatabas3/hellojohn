"use client"

import { useState, useEffect } from "react"
import { useSearchParams, useParams, useRouter } from "next/navigation"
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
import { useToast } from "@/hooks/use-toast"
import { api } from "@/lib/api"
import { useAuthStore } from "@/lib/auth-store"
import { useI18n } from "@/lib/i18n"
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
    Sliders
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
import { PhoneInput } from "@/components/ui/phone-input"
import { CountrySelect } from "@/components/ui/country-select"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { User } from "@/lib/types" // Import User type if available, otherwise define local

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
    user_fields?: UserFieldDefinition[]
}

// Ensure User type matches if not importing
// (Using explicit type here to match previous file)
interface UserType extends User {
    // Extend if needed
}

export default function UsersPage() {
    const searchParams = useSearchParams()
    const tenantIdParam = searchParams.get("id")
    const params = useParams()
    // Support both query param and router param if applicable, though strictly sidebar uses query param
    const tenantId = tenantIdParam || (params?.id as string)

    const { t } = useI18n()

    // Tab State
    const [activeTab, setActiveTab] = useState("list")

    if (!tenantId) {
        return (
            <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertTitle>{t("common.error")}</AlertTitle>
                <AlertDescription>
                    Tenant ID missing.
                </AlertDescription>
            </Alert>
        )
    }

    return (
        <div className="space-y-6 animate-in fade-in duration-500">
            <div className="flex items-center justify-between">
                <div>
                    <h2 className="text-3xl font-bold tracking-tight">
                        Usuarios
                    </h2>
                    <p className="text-muted-foreground">Gestión de usuarios y campos personalizados.</p>
                </div>
            </div>

            <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-4">
                <TabsList>
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
                    <UsersList tenantId={tenantId} />
                </TabsContent>

                <TabsContent value="fields" className="space-y-4">
                    <UserFieldsSettings tenantId={tenantId} />
                </TabsContent>
            </Tabs>
        </div>
    )
}

// ----------------------------------------------------------------------
// COMPONENT: Users List (Migrated from UsersClientPage)
// ----------------------------------------------------------------------

function UsersList({ tenantId }: { tenantId: string }) {
    const { token } = useAuthStore()
    const { toast } = useToast()
    const queryClient = useQueryClient()

    const [search, setSearch] = useState("")
    const [selectedUser, setSelectedUser] = useState<UserType | null>(null)
    const [isDetailsOpen, setIsDetailsOpen] = useState(false)
    const [isCreateOpen, setIsCreateOpen] = useState(false)
    const [blockUser, setBlockUser] = useState<UserType | null>(null)

    // 1. Fetch Users
    const { data: users, isLoading } = useQuery<UserType[]>({
        queryKey: ["users", tenantId],
        queryFn: async () => {
            const res = await fetch(`/v1/admin/tenants/${tenantId}/users`, {
                headers: {
                    "Authorization": `Bearer ${token}`
                }
            })
            if (!res.ok) throw new Error("Error fetching users")
            return res.json()
        },
        enabled: !!tenantId && !!token,
    })

    // 2. Fetch Field Definitions (Read-only for Create Form)
    const { data: fieldDefs } = useQuery<UserFieldDefinition[]>({
        queryKey: ["user-fields", tenantId],
        queryFn: async () => {
            const res = await fetch(`/v1/admin/tenants/${tenantId}`, {
                headers: {
                    "Authorization": `Bearer ${token}`
                }
            })
            if (!res.ok) return []
            const tenant = await res.json()
            return tenant.settings?.user_fields || []
        },
        enabled: !!tenantId && !!token,
    })

    // 3. Create User Mutation
    const createMutation = useMutation({
        mutationFn: async (vars: any) => {
            const res = await fetch(`/v1/admin/tenants/${tenantId}/users`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    "Authorization": `Bearer ${token}`
                },
                body: JSON.stringify(vars),
            })
            if (!res.ok) {
                const err = await res.json()
                throw new Error(err.error_description || "Error creating user")
            }
            return res.json()
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            setIsCreateOpen(false)
            toast({ title: "Usuario creado", description: "El usuario ha sido creado exitosamente." })
        },
        onError: (err: Error) => {
            toast({ title: "Error", description: err.message, variant: "destructive" })
        },
    })

    // 4. Delete User Mutation
    const deleteMutation = useMutation({
        mutationFn: async (userId: string) => {
            const res = await fetch(`/v1/admin/tenants/${tenantId}/users/${userId}`, {
                method: "DELETE",
                headers: {
                    "Authorization": `Bearer ${token}`
                },
            })
            if (!res.ok) throw new Error("Error deleting user")
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            toast({ title: "Usuario eliminado", description: "El usuario ha sido eliminado." })
        },
        onError: (err: Error) => {
            toast({ title: "Error", description: err.message, variant: "destructive" })
        },
    })

    // 5. Block User Mutation
    const blockMutation = useMutation({
        mutationFn: async ({ userId, reason, duration }: { userId: string, reason: string, duration: string }) => {
            const res = await fetch(`/v1/admin/users/disable`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    "Authorization": `Bearer ${token}`
                },
                body: JSON.stringify({ user_id: userId, tenant_id: tenantId, reason, duration }),
            })
            if (!res.ok) {
                const err = await res.json()
                throw new Error(err.error_description || "Error blocking user")
            }
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            setBlockUser(null)
            toast({ title: "Usuario bloqueado", description: "El usuario ha sido bloqueado exitosamente." })
        },
        onError: (err: Error) => {
            toast({ title: "Error", description: err.message, variant: "destructive" })
        }
    })

    // 6. Enable User Mutation
    const enableMutation = useMutation({
        mutationFn: async (userId: string) => {
            const res = await fetch(`/v1/admin/users/enable`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    "Authorization": `Bearer ${token}`
                },
                body: JSON.stringify({ user_id: userId, tenant_id: tenantId }),
            })
            if (!res.ok) throw new Error("Error enabling user")
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
            toast({ title: "Usuario desbloqueado", description: "El usuario ha sido habilitado nuevamente." })
        },
        onError: (err: Error) => {
            toast({ title: "Error", description: err.message, variant: "destructive" })
        }
    })

    const filteredUsers = users?.filter(
        (user) =>
            (user.email || "").toLowerCase().includes(search.toLowerCase()) ||
            (user.id || "").toLowerCase().includes(search.toLowerCase())
    )

    const handleDetails = (user: UserType) => {
        setSelectedUser(user)
        setIsDetailsOpen(true)
    }

    return (
        <div className="space-y-4">
            <div className="flex items-center justify-between">
                <div className="relative">
                    <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input
                        placeholder="Buscar por email o ID..."
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        className="pl-9 h-9 w-[150px] lg:w-[300px]"
                    />
                </div>
                <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
                    <DialogTrigger asChild>
                        <Button><Plus className="mr-2 h-4 w-4" /> Crear Usuario</Button>
                    </DialogTrigger>
                    <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto">
                        <DialogHeader>
                            <DialogTitle>Crear Nuevo Usuario</DialogTitle>
                            <DialogDescription>
                                Ingresa los datos básicos y campos personalizados.
                            </DialogDescription>
                        </DialogHeader>
                        <CreateUserForm
                            fieldDefs={fieldDefs || []}
                            onSubmit={(data) => createMutation.mutate(data)}
                            isPending={createMutation.isPending}
                        />
                    </DialogContent>
                </Dialog>
            </div>

            <div className="rounded-md border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead className="w-[50px]"></TableHead>
                            <TableHead>Identidad</TableHead>
                            <TableHead>Estado</TableHead>
                            <TableHead className="hidden md:table-cell">Creado</TableHead>
                            <TableHead className="text-right">Acciones</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {isLoading ? (
                            <TableRow>
                                <TableCell colSpan={5} className="h-24 text-center">
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin inline" /> Cargando...
                                </TableCell>
                            </TableRow>
                        ) : (!filteredUsers || filteredUsers.length === 0) ? (
                            <TableRow>
                                <TableCell colSpan={5} className="h-32 text-center text-muted-foreground">
                                    <div className="flex flex-col items-center justify-center gap-2">
                                        <UserIcon className="h-8 w-8 text-muted-foreground/50" />
                                        <p>No se encontraron usuarios.</p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            filteredUsers.map((user) => (
                                <UserRow
                                    key={user.id}
                                    user={user}
                                    onDelete={() => deleteMutation.mutate(user.id)}
                                    onDetails={() => handleDetails(user)}
                                    onBlock={() => { setBlockUser(user) }}
                                    onUnlock={() => enableMutation.mutate(user.id)}
                                />
                            ))
                        )}
                    </TableBody>
                </Table>
            </div>

            <Dialog open={isDetailsOpen} onOpenChange={setIsDetailsOpen}>
                <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle>Detalles del Usuario</DialogTitle>
                        <DialogDescription>Información completa registrada.</DialogDescription>
                    </DialogHeader>
                    {selectedUser && <UserDetails user={selectedUser} />}
                </DialogContent>
            </Dialog>

            {blockUser && (
                <BlockUserDialog
                    user={blockUser}
                    onClose={() => setBlockUser(null)}
                    onBlock={(userId, reason, duration) => {
                        if (!userId) {
                            toast({ title: "Error", description: "No se pudo identificar el usuario (ID faltante)", variant: "destructive" })
                            return
                        }
                        blockMutation.mutate({ userId, reason, duration })
                    }}
                    isPending={blockMutation.isPending}
                />
            )}
        </div>
    )
}

function UserRow({ user, onDelete, onDetails, onBlock, onUnlock }: { user: UserType, onDelete: () => void, onDetails: () => void, onBlock: () => void, onUnlock: () => void }) {
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
        <TableRow>
            <TableCell>
                <div className="h-9 w-9 bg-muted rounded-full flex items-center justify-center font-semibold text-xs text-muted-foreground">
                    {initial}
                </div>
            </TableCell>
            <TableCell>
                <div className="flex flex-col">
                    <span className="font-medium truncate max-w-[200px]">{user.email}</span>
                    <span className="text-xs text-muted-foreground font-mono truncate max-w-[150px] opacity-70 cursor-pointer" title={user.id}>{user.id}</span>
                </div>
            </TableCell>
            <TableCell>
                <div className="flex flex-col gap-1 items-start">
                    {displayBlocked ? (
                        <div className="flex flex-col gap-0.5">
                            <Badge variant="destructive" className="h-5 text-[10px] px-1.5" >
                                {isSuspended ? "Suspendido" : "Deshabilitado"}
                            </Badge>
                            {isSuspended && user.disabled_until && (
                                <span className="text-[9px] text-red-500 font-mono">
                                    Hasta: {new Date(user.disabled_until).toLocaleString("es-ES", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}
                                </span>
                            )}
                        </div>
                    ) : (
                        <Badge variant="default" className="h-5 text-[10px] px-1.5 bg-green-600 hover:bg-green-700">Activo</Badge>
                    )}
                    {user.email_verified ? (
                        <span className="text-[10px] text-green-600 font-medium flex items-center">Ok Verificado</span>
                    ) : (
                        <span className="text-[10px] text-amber-600 font-medium flex items-center">No Verificado</span>
                    )}
                </div>
            </TableCell>
            <TableCell className="hidden md:table-cell text-sm text-muted-foreground">
                {dateStr}
            </TableCell>
            <TableCell className="text-right">
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button variant="ghost" className="h-8 w-8 p-0">
                            <span className="sr-only">Abrir menú</span>
                            <MoreHorizontal className="h-4 w-4" />
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                        <DropdownMenuLabel>Acciones</DropdownMenuLabel>
                        <DropdownMenuItem onClick={onDetails}>
                            <Eye className="mr-2 h-4 w-4" /> Ver Detalles
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => {
                            if (user.id) navigator.clipboard.writeText(user.id)
                        }}>
                            <Copy className="mr-2 h-4 w-4" /> Copiar ID
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        {displayBlocked ? (
                            <DropdownMenuItem onClick={onUnlock}>
                                <Unlock className="mr-2 h-4 w-4 text-green-600" /> Desbloquear
                            </DropdownMenuItem>
                        ) : (
                            <DropdownMenuItem onClick={onBlock}>
                                <Ban className="mr-2 h-4 w-4 text-orange-600" /> Bloquear
                            </DropdownMenuItem>
                        )}
                        <DropdownMenuSeparator />
                        <DropdownMenuItem onClick={onDelete} className="text-red-600 focus:text-red-600 focus:bg-red-50">
                            <Trash2 className="mr-2 h-4 w-4" /> Eliminar usuario
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            </TableCell>
        </TableRow>
    )
}

function CreateUserForm({ fieldDefs, onSubmit, isPending }: {
    fieldDefs: UserFieldDefinition[],
    onSubmit: (data: any) => void,
    isPending: boolean
}) {
    const [email, setEmail] = useState("")
    const [password, setPassword] = useState("")
    const [customFields, setCustomFields] = useState<Record<string, any>>({})

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()
        onSubmit({ email, password, email_verified: true, custom_fields: customFields })
    }

    return (
        <form onSubmit={handleSubmit} className="space-y-6 py-2">
            <div className="grid gap-4">
                <div className="grid gap-2">
                    <Label htmlFor="email">Email</Label>
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
                    <Label htmlFor="password">Contraseña Temporal</Label>
                    <Input
                        id="password"
                        type="password"
                        placeholder="••••••••"
                        value={password}
                        onChange={e => setPassword(e.target.value)}
                        required
                    />
                </div>
            </div>

            {fieldDefs.length > 0 && (
                <div className="space-y-4 pt-2">
                    <div className="flex items-center">
                        <div className="h-px flex-1 bg-border" />
                        <span className="px-2 text-xs font-semibold text-muted-foreground uppercase">Información Adicional</span>
                        <div className="h-px flex-1 bg-border" />
                    </div>

                    {fieldDefs.map((field) => (
                        <div key={field.name} className="grid gap-2">
                            <Label htmlFor={field.name}>{field.name} {field.required && <span className="text-red-500">*</span>}</Label>
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
                            ) : (
                                <Input
                                    id={field.name}
                                    type={field.type === "number" || field.type === "int" ? "number" : "text"}
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
                    Crear Usuario
                </Button>
            </DialogFooter>
        </form>
    )
}

// ----------------------------------------------------------------------
// COMPONENT: Fields Settings (Migrated from original UsersPage)
// ----------------------------------------------------------------------

function UserFieldsSettings({ tenantId }: { tenantId: string }) {
    const { token } = useAuthStore()
    const { toast } = useToast()
    const queryClient = useQueryClient()
    const { t } = useI18n()

    const [etag, setEtag] = useState<string>("")
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
        data: settings,
        isLoading,
        isError,
        error,
    } = useQuery<TenantSettings>({
        queryKey: ["tenant-users-settings", tenantId],
        queryFn: async () => {
            const { data, headers } = await api.getWithHeaders<TenantSettings>(`/v1/admin/tenants/${tenantId}/settings`)
            const etagHeader = headers.get("ETag")
            if (etagHeader) {
                setEtag(etagHeader)
            }
            return data
        },
        enabled: !!tenantId && !!token,
    })

    useEffect(() => {
        if (settings) {
            setLocalFields(settings.user_fields || [])
            setHasFieldChanges(false)
        }
    }, [settings])

    const updateSettingsMutation = useMutation({
        mutationFn: async (data: TenantSettings) => {
            const currentSettings = settings || {}
            const payload = {
                ...currentSettings,
                ...data,
            }
            await api.put(`/v1/admin/tenants/${tenantId}/settings`, payload, etag)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["tenant-users-settings", tenantId] })
            toast({
                title: t("common.success"),
                description: t("tenants.settingsUpdatedDesc"),
            })
            setHasFieldChanges(false)
        },
        onError: (err: any) => {
            toast({
                variant: "destructive",
                title: t("common.error"),
                description: err.response?.data?.error_description || err.message,
            })
        },
    })

    const handleOpenFieldDialog = (field?: UserFieldDefinition, idx?: number) => {
        if (field) {
            setFieldForm(field)
            setEditingFieldIdx(idx ?? null)
        } else {
            setFieldForm({
                name: "",
                type: "text",
                required: false,
                unique: false,
                indexed: false,
                description: "",
            })
            setEditingFieldIdx(null)
        }
        setIsFieldsDialogOpen(true)
    }

    const handleLocalSaveField = () => {
        // Validate
        if (!fieldForm.name || !fieldForm.type) {
            toast({
                variant: "destructive",
                title: t("common.error"),
                description: "Name and Type are required",
            })
            return
        }

        const nameExists = localFields.some((f, i) =>
            f.name.toLowerCase() === fieldForm.name.toLowerCase() && i !== editingFieldIdx
        )

        if (nameExists) {
            toast({
                variant: "destructive",
                title: t("common.error"),
                description: "Field name already exists",
            })
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
        updateSettingsMutation.mutate({
            user_fields: localFields,
        })
    }

    const handleCancelFieldChanges = () => {
        setLocalFields(settings?.user_fields || [])
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
        <Card className="flex flex-col h-full">
            <CardHeader>
                <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-2">
                        <div className="p-2 bg-purple-500/10 rounded-lg">
                            <Settings2 className="h-5 w-5 text-purple-600" />
                        </div>
                        <CardTitle>Campos de Usuario</CardTitle>
                    </div>
                    <Button size="sm" onClick={() => handleOpenFieldDialog()}>
                        <Plus className="mr-2 h-4 w-4" />
                        Agregar Campo
                    </Button>
                </div>
                <CardDescription>
                    Campos personalizados para solicitar al user al momento del registro.
                </CardDescription>
            </CardHeader>
            <CardContent>
                <div className="rounded-md border">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>Nombre</TableHead>
                                <TableHead>Tipo</TableHead>
                                <TableHead className="text-center">Requerido</TableHead>
                                <TableHead className="text-center">Único</TableHead>
                                <TableHead className="text-center">Indexado</TableHead>
                                <TableHead className="text-right">Acciones</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {(!localFields || localFields.length === 0) ? (
                                <TableRow>
                                    <TableCell colSpan={6} className="h-24 text-center">
                                        No hay campos personalizados definidos.
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
                                            {field.required && <CheckCircle2 className="h-4 w-4 mx-auto text-green-500" />}
                                        </TableCell>
                                        <TableCell className="text-center">
                                            {field.unique && <CheckCircle2 className="h-4 w-4 mx-auto text-blue-500" />}
                                        </TableCell>
                                        <TableCell className="text-center">
                                            {field.indexed && <CheckCircle2 className="h-4 w-4 mx-auto text-gray-500" />}
                                        </TableCell>
                                        <TableCell className="text-right">
                                            <div className="flex justify-end gap-2">
                                                <Button variant="ghost" size="icon" onClick={() => handleOpenFieldDialog(field, idx)}>
                                                    <Edit2 className="h-4 w-4" />
                                                </Button>
                                                <Button variant="ghost" size="icon" className="text-red-500 hover:text-red-600" onClick={() => handleLocalDeleteField(idx)}>
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
            </CardContent>
            <CardFooter className="mt-auto flex items-center justify-end gap-2 px-4 py-4">
                {hasFieldChanges && (
                    <>
                        <Button variant="ghost" onClick={handleCancelFieldChanges}>
                            Cancelar
                        </Button>
                        <Button onClick={handleSaveAllFields} disabled={updateSettingsMutation.isPending}>
                            {updateSettingsMutation.isPending ? "Guardando..." : "Guardar Cambios"}
                        </Button>
                    </>
                )}
            </CardFooter>

            <Dialog open={isFieldsDialogOpen} onOpenChange={setIsFieldsDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{editingFieldIdx !== null ? "Editar Campo" : "Agregar Campo"}</DialogTitle>
                        <DialogDescription>
                            Configura las propiedades del campo. Los cambios se aplicarán al guardar la lista completa.
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
                                    disabled={editingFieldIdx !== null} // Prevent renaming for now
                                />
                                {editingFieldIdx !== null && <p className="text-xs text-muted-foreground">El nombre no se puede cambiar una vez creado.</p>}
                            </div>
                            <div className="space-y-2">
                                <Label>Tipo</Label>
                                <Select
                                    value={fieldForm.type}
                                    onValueChange={(val) => setFieldForm({ ...fieldForm, type: val })}
                                    disabled={editingFieldIdx !== null} // Prevent type change for now
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
                            <Label>Descripción</Label>
                            <Textarea
                                placeholder="Descripción opcional..."
                                value={fieldForm.description || ""}
                                onChange={(e) => setFieldForm({ ...fieldForm, description: e.target.value })}
                            />
                        </div>
                        <div className="flex flex-col gap-4 border p-4 rounded-lg">
                            <div className="flex items-center justify-between">
                                <div className="space-y-0.5">
                                    <Label>Requerido (Not Null)</Label>
                                    <p className="text-xs text-muted-foreground">Si se activa en una tabla con datos, fallará si hay nulos.</p>
                                </div>
                                <Switch
                                    checked={fieldForm.required}
                                    onCheckedChange={(c) => setFieldForm({ ...fieldForm, required: c })}
                                />
                            </div>
                            <div className="flex items-center justify-between">
                                <div className="space-y-0.5">
                                    <Label>Único (Unique)</Label>
                                    <p className="text-xs text-muted-foreground">No permitir valores duplicados.</p>
                                </div>
                                <Switch
                                    checked={fieldForm.unique}
                                    onCheckedChange={(c) => setFieldForm({ ...fieldForm, unique: c })}
                                />
                            </div>
                            <div className="flex items-center justify-between">
                                <div className="space-y-0.5">
                                    <Label>Indexado</Label>
                                    <p className="text-xs text-muted-foreground">Mejora la velocidad de búsqueda.</p>
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
                            Agregar a Lista
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </Card>
    )
}

function UserDetails({ user }: { user: UserType }) {
    const { toast } = useToast()

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

    const createdAt = formatDate(user.created_at)
    const updatedAt = formatDate(user.updated_at || user.custom_fields?.updated_at)
    const initial = user.email ? user.email.slice(0, 2).toUpperCase() : "??"

    const handleResendVerification = () => {
        toast({
            title: "Funcionalidad en desarrollo",
            description: "El reenvío de códigos de verificación estará disponible pronto.",
        })
    }

    return (
        <div className="mt-4 space-y-6">
            <div className="flex flex-col items-center justify-center space-y-3 bg-muted/20 p-6 rounded-lg border border-dashed">
                <div className="h-20 w-20 bg-background border-4 border-muted rounded-full flex items-center justify-center text-3xl font-bold text-muted-foreground shadow-sm">
                    {initial}
                </div>
                <div className="text-center">
                    <h3 className="text-xl font-bold tracking-tight">{user.email}</h3>
                    <div className="flex items-center justify-center gap-2 mt-1">
                        <Badge variant="outline" className="font-mono text-xs">{user.id}</Badge>
                        <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => navigator.clipboard.writeText(user.id)}>
                            <Copy className="h-3 w-3" />
                        </Button>
                    </div>
                </div>
            </div>

            <div className="space-y-4">
                <h4 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider border-b pb-2">Información de Sistema</h4>
                <div className="grid grid-cols-2 gap-4 text-sm">
                    <div className="space-y-1">
                        <span className="block text-xs text-muted-foreground">Estado de Cuenta</span>
                        {user.disabled_at ? (
                            <span className="inline-flex items-center px-2 py-1 rounded-full bg-red-100 text-red-700 text-xs font-medium">Deshabilitado</span>
                        ) : (
                            <span className="inline-flex items-center px-2 py-1 rounded-full bg-green-100 text-green-700 text-xs font-medium">Activa</span>
                        )}
                    </div>
                    <div className="space-y-1">
                        <span className="block text-xs text-muted-foreground">Verificación de Email</span>
                        <div className="flex items-center gap-2">
                            <span className="font-medium">{user.email_verified ? "Verificado" : "Pendiente"}</span>
                            {!user.email_verified && (
                                <Button variant="link" className="h-auto p-0 text-xs text-blue-600" onClick={handleResendVerification}>
                                    Reenviar código
                                </Button>
                            )}
                        </div>
                    </div>
                    <div className="space-y-1">
                        <span className="block text-xs text-muted-foreground">Fecha de Creación</span>
                        <span className="font-medium">{createdAt}</span>
                    </div>
                    <div className="space-y-1">
                        <span className="block text-xs text-muted-foreground">Última Actualización</span>
                        <span className="font-medium">{updatedAt}</span>
                    </div>
                    {user.disabled_reason && (
                        <div className="space-y-1 col-span-2">
                            <span className="block text-xs text-muted-foreground">Razón de Bloqueo</span>
                            <span className="font-medium text-red-600">{user.disabled_reason}</span>
                        </div>
                    )}
                </div>
            </div>

            {user.custom_fields && Object.keys(user.custom_fields).length > 0 && (
                <div className="space-y-4">
                    <h4 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider border-b pb-2">Campos Personalizados</h4>
                    <div className="grid gap-3">
                        {Object.entries(user.custom_fields)
                            .filter(([key]) => key !== 'updated_at')
                            .map(([key, value]) => (
                                <div key={key} className="flex flex-col gap-1 rounded-md border p-3 bg-muted/10 hover:bg-muted/20 transition-colors">
                                    <span className="text-xs font-bold uppercase text-muted-foreground/70">{key}</span>
                                    <span className="text-sm font-medium break-all">
                                        {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                                    </span>
                                </div>
                            ))}
                    </div>
                </div>
            )}

            {user.metadata && Object.keys(user.metadata).length > 0 && (
                <div className="space-y-4">
                    <h4 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider border-b pb-2">Metadatos / Perfil</h4>
                    <div className="bg-muted p-4 rounded-lg border overflow-x-auto">
                        <pre className="text-xs font-mono">
                            {JSON.stringify(user.metadata, null, 2)}
                        </pre>
                    </div>
                </div>
            )}
        </div>
    )
}

function BlockUserDialog({ user, onClose, onBlock, isPending }: { user: UserType, onClose: () => void, onBlock: (userId: string, reason: string, duration: string) => void, isPending: boolean }) {
    const [reason, setReason] = useState("")
    const [duration, setDuration] = useState("permanent")

    return (
        <Dialog open={true} onOpenChange={onClose}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Bloquear Usuario</DialogTitle>
                    <DialogDescription>
                        Suspenda el acceso de {user.email}.
                    </DialogDescription>
                </DialogHeader>
                <div className="grid gap-4 py-4">
                    <div className="grid gap-2">
                        <Label>Motivo</Label>
                        <Input
                            placeholder="Ej. Violación de términos"
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
                        variant="destructive"
                        onClick={() => { onBlock(user.id, reason, duration === "permanent" ? "" : duration) }}
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
