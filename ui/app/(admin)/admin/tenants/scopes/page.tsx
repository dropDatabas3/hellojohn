"use client"

import { useState, useMemo, useEffect } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useParams, useSearchParams } from "next/navigation"
import {
    Plus,
    Search,
    Trash2,
    Eye,
    Copy,
    Check,
    ArrowLeft,
    Shield,
    ShieldCheck,
    ShieldAlert,
    ChevronRight,
    Sparkles,
    Lock,
    Unlock,
    Settings2,
    Info,
    AlertTriangle,
    CheckCircle2,
    XCircle,
    Edit,
    MoreHorizontal,
    Link2,
    Loader2,
    ChevronDown,
    ChevronUp,
    Globe,
    User,
    Mail,
    Phone,
    MapPin,
    Tag,
    FileCode2,
    Layers,
    RefreshCw,
    BookOpen,
    Zap,
    Key,
    Users,
    Database,
    ExternalLink,
} from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card, CardContent, CardDescription, CardHeader, CardTitle, CardFooter } from "@/components/ui/card"
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
import { Switch } from "@/components/ui/switch"
import { Checkbox } from "@/components/ui/checkbox"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
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
import {
    Collapsible,
    CollapsibleContent,
    CollapsibleTrigger,
} from "@/components/ui/collapsible"
import Link from "next/link"
import { cn } from "@/lib/utils"
import type { Scope, Tenant } from "@/lib/types"

// ============================================================================
// TYPES
// ============================================================================

interface ScopeRow {
    name: string
    description?: string
    system?: boolean
    claims?: string[]
    depends_on?: string
    display_name?: string
    created_at?: string
    updated_at?: string
}

interface ScopeFormState {
    name: string
    displayName: string
    description: string
    claims: string[]
    dependsOn: string
}

// ============================================================================
// CONSTANTS
// ============================================================================

const OIDC_STANDARD_SCOPES: ScopeRow[] = [
    {
        name: "openid",
        display_name: "OpenID",
        description: "Obligatorio para flujos OIDC. Indica que la aplicación solicita autenticación.",
        system: true,
        claims: ["sub"],
        depends_on: "",
    },
    {
        name: "profile",
        display_name: "Profile",
        description: "Información básica del perfil del usuario.",
        system: true,
        claims: ["name", "family_name", "given_name", "middle_name", "nickname", "preferred_username", "picture", "website", "gender", "birthdate", "zoneinfo", "locale", "updated_at"],
        depends_on: "openid",
    },
    {
        name: "email",
        display_name: "Email",
        description: "Dirección de correo electrónico y estado de verificación.",
        system: true,
        claims: ["email", "email_verified"],
        depends_on: "openid",
    },
    {
        name: "address",
        display_name: "Address",
        description: "Dirección postal del usuario.",
        system: true,
        claims: ["address"],
        depends_on: "openid",
    },
    {
        name: "phone",
        display_name: "Phone",
        description: "Número de teléfono y estado de verificación.",
        system: true,
        claims: ["phone_number", "phone_number_verified"],
        depends_on: "openid",
    },
    {
        name: "offline_access",
        display_name: "Offline Access",
        description: "Permite obtener refresh tokens para acceso prolongado.",
        system: true,
        claims: [],
        depends_on: "openid",
    },
]

const COMMON_CLAIMS = [
    { name: "sub", description: "Identificador único del usuario" },
    { name: "name", description: "Nombre completo" },
    { name: "given_name", description: "Nombre de pila" },
    { name: "family_name", description: "Apellido" },
    { name: "email", description: "Correo electrónico" },
    { name: "email_verified", description: "Email verificado" },
    { name: "phone_number", description: "Teléfono" },
    { name: "picture", description: "URL de avatar" },
    { name: "locale", description: "Idioma preferido" },
    { name: "zoneinfo", description: "Zona horaria" },
]

const SCOPE_ICON_MAP: Record<string, React.ElementType> = {
    openid: Key,
    profile: User,
    email: Mail,
    address: MapPin,
    phone: Phone,
    offline_access: RefreshCw,
}

const DEFAULT_FORM: ScopeFormState = {
    name: "",
    displayName: "",
    description: "",
    claims: [],
    dependsOn: "",
}

// ============================================================================
// HELPERS
// ============================================================================

function getScopeIcon(scopeName: string) {
    return SCOPE_ICON_MAP[scopeName] || Tag
}

function formatScopeName(name: string): string {
    return name
        .toLowerCase()
        .replace(/[^a-z0-9:._-]/g, "_")
        .replace(/_+/g, "_")
        .replace(/^_|_$/g, "")
}

// ============================================================================
// HELPER COMPONENTS
// ============================================================================

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

function ScopeCard({
    scope,
    isStandard,
    onView,
    onEdit,
    onDelete,
}: {
    scope: ScopeRow
    isStandard: boolean
    onView: () => void
    onEdit?: () => void
    onDelete?: () => void
}) {
    const Icon = getScopeIcon(scope.name)
    const [expanded, setExpanded] = useState(false)

    return (
        <Card className={cn(
            "transition-all duration-200 hover:shadow-md",
            isStandard
                ? "border-blue-500/20 bg-gradient-to-br from-blue-500/5 to-indigo-500/5"
                : "hover:border-primary/30"
        )}>
            <CardHeader className="pb-2">
                <div className="flex items-start justify-between">
                    <div className="flex items-center gap-3">
                        <div className={cn(
                            "p-2.5 rounded-xl",
                            isStandard
                                ? "bg-blue-500/10 text-blue-600"
                                : "bg-purple-500/10 text-purple-600"
                        )}>
                            <Icon className="h-5 w-5" />
                        </div>
                        <div>
                            <div className="flex items-center gap-2">
                                <code className="font-mono font-semibold text-base">{scope.name}</code>
                                {isStandard && (
                                    <Badge variant="secondary" className="text-[10px]">
                                        <Lock className="h-2.5 w-2.5 mr-1" />
                                        OIDC Standard
                                    </Badge>
                                )}
                            </div>
                            {scope.display_name && scope.display_name !== scope.name && (
                                <p className="text-sm text-muted-foreground">{scope.display_name}</p>
                            )}
                        </div>
                    </div>
                    {!isStandard && (
                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <Button variant="ghost" size="icon" className="h-8 w-8">
                                    <MoreHorizontal className="h-4 w-4" />
                                </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                                <DropdownMenuItem onClick={onView}>
                                    <Eye className="mr-2 h-4 w-4" /> Ver detalles
                                </DropdownMenuItem>
                                <DropdownMenuItem onClick={onEdit}>
                                    <Edit className="mr-2 h-4 w-4" /> Editar
                                </DropdownMenuItem>
                                <DropdownMenuSeparator />
                                <DropdownMenuItem onClick={onDelete} className="text-red-600">
                                    <Trash2 className="mr-2 h-4 w-4" /> Eliminar
                                </DropdownMenuItem>
                            </DropdownMenuContent>
                        </DropdownMenu>
                    )}
                </div>
            </CardHeader>
            <CardContent className="space-y-3">
                <p className="text-sm text-muted-foreground line-clamp-2">
                    {scope.description || "Sin descripción"}
                </p>

                {scope.depends_on && (
                    <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                        <Link2 className="h-3 w-3" />
                        <span>Requiere: <code className="bg-muted px-1 rounded">{scope.depends_on}</code></span>
                    </div>
                )}

                {scope.claims && scope.claims.length > 0 && (
                    <Collapsible open={expanded} onOpenChange={setExpanded}>
                        <CollapsibleTrigger asChild>
                            <Button variant="ghost" size="sm" className="w-full justify-between px-2 h-8">
                                <span className="text-xs text-muted-foreground">
                                    Claims incluidos ({scope.claims.length})
                                </span>
                                {expanded ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
                            </Button>
                        </CollapsibleTrigger>
                        <CollapsibleContent className="pt-2">
                            <div className="flex flex-wrap gap-1">
                                {scope.claims.map(claim => (
                                    <Badge key={claim} variant="outline" className="text-[10px] font-mono">
                                        {claim}
                                    </Badge>
                                ))}
                            </div>
                        </CollapsibleContent>
                    </Collapsible>
                )}
            </CardContent>
            {isStandard && (
                <CardFooter className="pt-0">
                    <Button variant="ghost" size="sm" className="w-full" onClick={onView}>
                        <Eye className="h-3.5 w-3.5 mr-2" />
                        Ver detalles
                    </Button>
                </CardFooter>
            )}
        </Card>
    )
}

// ============================================================================
// MAIN COMPONENT
// ============================================================================

export default function ScopesClientPage() {
    const params = useParams()
    const searchParams = useSearchParams()
    const { t } = useI18n()
    const { toast } = useToast()
    const queryClient = useQueryClient()
    const tenantId = (params.id as string) || (searchParams.get("id") as string)

    // UI State
    const [search, setSearch] = useState("")
    const [activeTab, setActiveTab] = useState("overview")
    const [createDialogOpen, setCreateDialogOpen] = useState(false)
    const [detailDialogOpen, setDetailDialogOpen] = useState(false)
    const [editDialogOpen, setEditDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [selectedScope, setSelectedScope] = useState<ScopeRow | null>(null)
    const [copiedField, setCopiedField] = useState<string | null>(null)

    // Form state
    const [form, setForm] = useState<ScopeFormState>({ ...DEFAULT_FORM })

    // ========================================================================
    // QUERIES
    // ========================================================================

    const { data: tenant } = useQuery({
        queryKey: ["tenant", tenantId],
        enabled: !!tenantId,
        queryFn: () => api.get<Tenant>(`/v2/admin/tenants/${tenantId}`),
    })

    const { data: scopesRaw, isLoading, refetch } = useQuery({
        queryKey: ["scopes", tenantId],
        enabled: !!tenantId,
        queryFn: () => api.get<ScopeRow[]>(`/v2/admin/scopes`, {
            headers: { "X-Tenant-ID": tenantId }
        }),
    })

    // Merge standard scopes with backend scopes
    const allScopes = useMemo(() => {
        const backendScopes = scopesRaw || []
        const standardNames = OIDC_STANDARD_SCOPES.map(s => s.name)
        const customScopes = backendScopes.filter(s => !standardNames.includes(s.name) && !s.system)

        // Check which standard scopes exist in backend
        const mergedStandard = OIDC_STANDARD_SCOPES.map(std => {
            const fromBackend = backendScopes.find(b => b.name === std.name)
            return fromBackend ? { ...std, ...fromBackend } : std
        })

        return {
            standard: mergedStandard,
            custom: customScopes,
            all: [...mergedStandard, ...customScopes],
        }
    }, [scopesRaw])

    // ========================================================================
    // MUTATIONS
    // ========================================================================

    const createMutation = useMutation({
        mutationFn: (data: ScopeFormState) =>
            api.post<ScopeRow>(`/v2/admin/scopes`, {
                name: data.name,
                description: data.description || "",
                display_name: data.displayName || "",
                claims: data.claims || [],
                depends_on: data.dependsOn || "",
            }, {
                headers: { "X-Tenant-ID": tenantId }
            }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["scopes", tenantId] })
            setCreateDialogOpen(false)
            resetForm()
            toast({
                title: "Scope creado",
                description: "El scope ha sido creado exitosamente.",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo crear el scope",
                variant: "destructive",
            })
        },
    })

    const updateMutation = useMutation({
        mutationFn: (data: ScopeFormState) =>
            api.post<ScopeRow>(`/v2/admin/scopes`, {
                name: data.name,
                description: data.description || "",
                display_name: data.displayName || "",
                claims: data.claims || [],
                depends_on: data.dependsOn || "",
            }, {
                headers: { "X-Tenant-ID": tenantId }
            }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["scopes", tenantId] })
            setEditDialogOpen(false)
            setSelectedScope(null)
            resetForm()
            toast({
                title: "Scope actualizado",
                description: "Los cambios han sido guardados.",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo actualizar el scope",
                variant: "destructive",
            })
        },
    })

    const deleteMutation = useMutation({
        mutationFn: (scopeName: string) => api.delete(`/v2/admin/scopes/${encodeURIComponent(scopeName)}`, {
            headers: { "X-Tenant-ID": tenantId }
        }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["scopes", tenantId] })
            setDeleteDialogOpen(false)
            setDetailDialogOpen(false)
            setSelectedScope(null)
            toast({
                title: "Scope eliminado",
                description: "El scope ha sido eliminado permanentemente.",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo eliminar el scope",
                variant: "destructive",
            })
        },
    })

    // ========================================================================
    // HANDLERS
    // ========================================================================

    const resetForm = () => {
        setForm({ ...DEFAULT_FORM })
    }

    const filteredCustomScopes = useMemo(() => {
        return allScopes.custom.filter(
            (scope) =>
                scope.name.toLowerCase().includes(search.toLowerCase()) ||
                scope.description?.toLowerCase().includes(search.toLowerCase())
        )
    }, [allScopes.custom, search])

    const handleCreate = () => {
        if (!form.name) {
            toast({ title: "Error", description: "El nombre es obligatorio", variant: "destructive" })
            return
        }

        const scopeRegex = /^[a-z0-9:._-]+$/
        if (!scopeRegex.test(form.name)) {
            toast({
                title: "Error",
                description: "El nombre solo puede contener minúsculas, números, ':', '.', '_' y '-'.",
                variant: "destructive",
            })
            return
        }

        // Check if name conflicts with standard
        if (OIDC_STANDARD_SCOPES.some(s => s.name === form.name)) {
            toast({
                title: "Error",
                description: "No puedes crear un scope con nombre reservado OIDC.",
                variant: "destructive",
            })
            return
        }

        createMutation.mutate(form)
    }

    const handleUpdate = () => {
        if (!form.name) {
            toast({ title: "Error", description: "El nombre es obligatorio", variant: "destructive" })
            return
        }
        updateMutation.mutate(form)
    }

    const handleDelete = () => {
        if (selectedScope) {
            deleteMutation.mutate(selectedScope.name)
        }
    }

    const openDetailDialog = (scope: ScopeRow) => {
        setSelectedScope(scope)
        setDetailDialogOpen(true)
    }

    const openEditDialog = (scope: ScopeRow) => {
        setSelectedScope(scope)
        setForm({
            name: scope.name,
            displayName: scope.display_name || "",
            description: scope.description || "",
            claims: scope.claims || [],
            dependsOn: scope.depends_on || "",
        })
        setEditDialogOpen(true)
    }

    const copyToClipboard = (text: string, field: string) => {
        navigator.clipboard.writeText(text)
        setCopiedField(field)
        setTimeout(() => setCopiedField(null), 2000)
        toast({ title: "Copiado", description: `${field} copiado al portapapeles.` })
    }

    // Stats
    const stats = useMemo(() => ({
        total: allScopes.all.length,
        standard: allScopes.standard.length,
        custom: allScopes.custom.length,
    }), [allScopes])

    // ========================================================================
    // RENDER
    // ========================================================================

    return (
        <div className="space-y-6 animate-in fade-in duration-500">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                    <Button variant="ghost" size="icon" asChild>
                        <Link href={`/admin/tenants/detail?id=${tenantId}`}>
                            <ArrowLeft className="h-4 w-4" />
                        </Link>
                    </Button>
                    <div className="flex items-center gap-3">
                        <div className="relative">
                            <div className="absolute inset-0 bg-gradient-to-br from-indigo-400/20 to-purple-400/20 rounded-xl blur-xl" />
                            <div className="relative p-3 bg-gradient-to-br from-indigo-500/10 to-purple-500/10 rounded-xl border border-indigo-500/20">
                                <Shield className="h-6 w-6 text-indigo-600 dark:text-indigo-400" />
                            </div>
                        </div>
                        <div>
                            <h1 className="text-2xl font-bold tracking-tight">Scopes OAuth2</h1>
                            <p className="text-sm text-muted-foreground">
                                {tenant?.name} — Define qué información y permisos pueden solicitar las aplicaciones
                            </p>
                        </div>
                    </div>
                </div>
                <Button onClick={() => { resetForm(); setCreateDialogOpen(true) }}>
                    <Plus className="mr-2 h-4 w-4" />
                    Nuevo Scope
                </Button>
            </div>

            {/* Info Banner */}
            <Alert className="bg-gradient-to-r from-indigo-50 to-purple-50 dark:from-indigo-950/20 dark:to-purple-950/20 border-indigo-200 dark:border-indigo-800">
                <Shield className="h-4 w-4 text-indigo-600" />
                <AlertTitle className="text-indigo-900 dark:text-indigo-100">¿Qué son los Scopes?</AlertTitle>
                <AlertDescription className="text-indigo-800 dark:text-indigo-200">
                    Los <strong>scopes</strong> definen qué información o permisos una aplicación puede solicitar.
                    OIDC define scopes estándar (<code className="bg-indigo-100 dark:bg-indigo-900 px-1 rounded">openid</code>,{" "}
                    <code className="bg-indigo-100 dark:bg-indigo-900 px-1 rounded">profile</code>,{" "}
                    <code className="bg-indigo-100 dark:bg-indigo-900 px-1 rounded">email</code>).
                    Puedes crear <strong>scopes personalizados</strong> para controlar acceso granular a tu API.
                </AlertDescription>
            </Alert>

            {/* Stats */}
            <div className="grid grid-cols-3 gap-4">
                <StatCard icon={Layers} label="Total Scopes" value={stats.total} variant="default" />
                <StatCard icon={ShieldCheck} label="OIDC Standard" value={stats.standard} variant="success" />
                <StatCard icon={Tag} label="Personalizados" value={stats.custom} variant="warning" />
            </div>

            {/* Tabs */}
            <Tabs value={activeTab} onValueChange={setActiveTab}>
                <TabsList className="grid w-full grid-cols-3">
                    <TabsTrigger value="overview" className="flex items-center gap-2">
                        <Layers className="h-4 w-4" />
                        <span className="hidden sm:inline">Vista General</span>
                    </TabsTrigger>
                    <TabsTrigger value="standard" className="flex items-center gap-2">
                        <ShieldCheck className="h-4 w-4" />
                        <span className="hidden sm:inline">OIDC Standard</span>
                    </TabsTrigger>
                    <TabsTrigger value="custom" className="flex items-center gap-2">
                        <Tag className="h-4 w-4" />
                        <span className="hidden sm:inline">Personalizados</span>
                    </TabsTrigger>
                </TabsList>

                {/* Tab: Overview */}
                <TabsContent value="overview" className="space-y-6 mt-6">
                    {/* Standard Scopes Section */}
                    <div className="space-y-4">
                        <div className="flex items-center justify-between">
                            <div>
                                <h2 className="text-lg font-semibold flex items-center gap-2">
                                    <ShieldCheck className="h-5 w-5 text-blue-600" />
                                    Scopes OIDC Standard
                                </h2>
                                <p className="text-sm text-muted-foreground">
                                    Scopes definidos por el estándar OpenID Connect. No pueden ser eliminados.
                                </p>
                            </div>
                        </div>
                        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-4">
                            {allScopes.standard.map(scope => (
                                <ScopeCard
                                    key={scope.name}
                                    scope={scope}
                                    isStandard={true}
                                    onView={() => openDetailDialog(scope)}
                                />
                            ))}
                        </div>
                    </div>

                    {/* Custom Scopes Section */}
                    <div className="space-y-4">
                        <div className="flex items-center justify-between">
                            <div>
                                <h2 className="text-lg font-semibold flex items-center gap-2">
                                    <Tag className="h-5 w-5 text-purple-600" />
                                    Scopes Personalizados
                                </h2>
                                <p className="text-sm text-muted-foreground">
                                    Scopes definidos por ti para controlar acceso a tu API.
                                </p>
                            </div>
                            <Button variant="outline" size="sm" onClick={() => { resetForm(); setCreateDialogOpen(true) }}>
                                <Plus className="h-4 w-4 mr-2" />
                                Crear scope
                            </Button>
                        </div>
                        {allScopes.custom.length === 0 ? (
                            <Card className="border-dashed">
                                <CardContent className="flex flex-col items-center justify-center py-12">
                                    <div className="relative mb-4">
                                        <div className="absolute inset-0 bg-gradient-to-br from-purple-400/20 to-pink-400/20 rounded-full blur-2xl scale-150" />
                                        <div className="relative rounded-2xl bg-gradient-to-br from-purple-50 to-pink-50 dark:from-purple-950/50 dark:to-pink-950/50 p-6">
                                            <Tag className="h-8 w-8 text-purple-600 dark:text-purple-400" />
                                        </div>
                                    </div>
                                    <h3 className="text-lg font-semibold mb-2">Sin scopes personalizados</h3>
                                    <p className="text-muted-foreground text-center max-w-sm text-sm mb-4">
                                        Crea scopes personalizados para controlar qué permisos pueden solicitar tus aplicaciones.
                                    </p>
                                    <Button onClick={() => { resetForm(); setCreateDialogOpen(true) }}>
                                        <Plus className="mr-2 h-4 w-4" />
                                        Crear primer scope
                                    </Button>
                                </CardContent>
                            </Card>
                        ) : (
                            <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-4">
                                {allScopes.custom.map(scope => (
                                    <ScopeCard
                                        key={scope.name}
                                        scope={scope}
                                        isStandard={false}
                                        onView={() => openDetailDialog(scope)}
                                        onEdit={() => openEditDialog(scope)}
                                        onDelete={() => { setSelectedScope(scope); setDeleteDialogOpen(true) }}
                                    />
                                ))}
                            </div>
                        )}
                    </div>
                </TabsContent>

                {/* Tab: Standard Scopes */}
                <TabsContent value="standard" className="space-y-6 mt-6">
                    <Alert>
                        <Lock className="h-4 w-4" />
                        <AlertDescription>
                            Los scopes OIDC estándar están predefinidos y no pueden ser modificados ni eliminados.
                            Cada scope incluye un conjunto específico de claims que se incluyen en los tokens.
                        </AlertDescription>
                    </Alert>

                    <Card>
                        <Table>
                            <TableHeader>
                                <TableRow className="bg-muted/30">
                                    <TableHead>Scope</TableHead>
                                    <TableHead>Descripción</TableHead>
                                    <TableHead>Claims incluidos</TableHead>
                                    <TableHead>Dependencia</TableHead>
                                    <TableHead className="text-right">Acciones</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {allScopes.standard.map(scope => {
                                    const Icon = getScopeIcon(scope.name)
                                    return (
                                        <TableRow key={scope.name}>
                                            <TableCell>
                                                <div className="flex items-center gap-3">
                                                    <div className="p-2 rounded-lg bg-blue-500/10">
                                                        <Icon className="h-4 w-4 text-blue-600" />
                                                    </div>
                                                    <div>
                                                        <code className="font-mono font-medium">{scope.name}</code>
                                                        <Badge variant="secondary" className="ml-2 text-[10px]">
                                                            Standard
                                                        </Badge>
                                                    </div>
                                                </div>
                                            </TableCell>
                                            <TableCell className="max-w-xs">
                                                <p className="text-sm text-muted-foreground truncate">
                                                    {scope.description}
                                                </p>
                                            </TableCell>
                                            <TableCell>
                                                <div className="flex flex-wrap gap-1 max-w-xs">
                                                    {scope.claims?.slice(0, 3).map(claim => (
                                                        <Badge key={claim} variant="outline" className="text-[10px] font-mono">
                                                            {claim}
                                                        </Badge>
                                                    ))}
                                                    {(scope.claims?.length || 0) > 3 && (
                                                        <Badge variant="outline" className="text-[10px]">
                                                            +{(scope.claims?.length || 0) - 3}
                                                        </Badge>
                                                    )}
                                                    {(!scope.claims || scope.claims.length === 0) && (
                                                        <span className="text-xs text-muted-foreground">—</span>
                                                    )}
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                {scope.depends_on ? (
                                                    <code className="text-xs bg-muted px-2 py-0.5 rounded">{scope.depends_on}</code>
                                                ) : (
                                                    <span className="text-xs text-muted-foreground">—</span>
                                                )}
                                            </TableCell>
                                            <TableCell className="text-right">
                                                <Button variant="ghost" size="sm" onClick={() => openDetailDialog(scope)}>
                                                    <Eye className="h-4 w-4" />
                                                </Button>
                                            </TableCell>
                                        </TableRow>
                                    )
                                })}
                            </TableBody>
                        </Table>
                    </Card>
                </TabsContent>

                {/* Tab: Custom Scopes */}
                <TabsContent value="custom" className="space-y-6 mt-6">
                    <Card>
                        <CardHeader className="pb-3">
                            <div className="flex items-center justify-between">
                                <div className="relative flex-1 max-w-sm">
                                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                                    <Input
                                        placeholder="Buscar por nombre o descripción..."
                                        value={search}
                                        onChange={(e) => setSearch(e.target.value)}
                                        className="pl-9"
                                    />
                                </div>
                                <div className="flex items-center gap-2">
                                    <Button variant="outline" size="sm" onClick={() => refetch()}>
                                        <RefreshCw className="h-4 w-4 mr-2" />
                                        Actualizar
                                    </Button>
                                    <Button size="sm" onClick={() => { resetForm(); setCreateDialogOpen(true) }}>
                                        <Plus className="h-4 w-4 mr-2" />
                                        Nuevo
                                    </Button>
                                </div>
                            </div>
                        </CardHeader>
                        <CardContent>
                            {isLoading ? (
                                <div className="flex items-center justify-center py-12">
                                    <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                                </div>
                            ) : filteredCustomScopes.length === 0 ? (
                                <div className="flex flex-col items-center justify-center py-12">
                                    <div className="relative mb-6">
                                        <div className="absolute inset-0 bg-gradient-to-br from-purple-400/20 to-pink-400/20 rounded-full blur-2xl scale-150" />
                                        <div className="relative rounded-2xl bg-gradient-to-br from-purple-50 to-pink-50 dark:from-purple-950/50 dark:to-pink-950/50 p-6">
                                            <Tag className="h-10 w-10 text-purple-600 dark:text-purple-400" />
                                        </div>
                                    </div>
                                    <h3 className="text-lg font-semibold mb-2">
                                        {search ? "Sin resultados" : "Sin scopes personalizados"}
                                    </h3>
                                    <p className="text-muted-foreground text-center max-w-sm text-sm mb-4">
                                        {search
                                            ? "No se encontraron scopes con ese criterio de búsqueda."
                                            : "Crea tu primer scope personalizado para controlar permisos de tu API."}
                                    </p>
                                    {!search && (
                                        <Button onClick={() => { resetForm(); setCreateDialogOpen(true) }}>
                                            <Plus className="mr-2 h-4 w-4" />
                                            Crear primer scope
                                        </Button>
                                    )}
                                </div>
                            ) : (
                                <Table>
                                    <TableHeader>
                                        <TableRow className="bg-muted/30">
                                            <TableHead>Nombre</TableHead>
                                            <TableHead>Descripción</TableHead>
                                            <TableHead>Claims</TableHead>
                                            <TableHead className="text-right">Acciones</TableHead>
                                        </TableRow>
                                    </TableHeader>
                                    <TableBody>
                                        {filteredCustomScopes.map((scope) => (
                                            <TableRow key={scope.name} className="cursor-pointer hover:bg-muted/50" onClick={() => openDetailDialog(scope)}>
                                                <TableCell>
                                                    <div className="flex items-center gap-3">
                                                        <div className="p-2 rounded-lg bg-purple-500/10 text-purple-600">
                                                            <Tag className="h-4 w-4" />
                                                        </div>
                                                        <div>
                                                            <code className="font-mono font-medium">{scope.name}</code>
                                                            <Badge variant="default" className="ml-2 text-[10px]">
                                                                Custom
                                                            </Badge>
                                                        </div>
                                                    </div>
                                                </TableCell>
                                                <TableCell className="max-w-xs">
                                                    <p className="text-sm text-muted-foreground truncate">
                                                        {scope.description || "Sin descripción"}
                                                    </p>
                                                </TableCell>
                                                <TableCell>
                                                    {scope.claims && scope.claims.length > 0 ? (
                                                        <div className="flex flex-wrap gap-1">
                                                            {scope.claims.slice(0, 2).map(claim => (
                                                                <Badge key={claim} variant="outline" className="text-[10px] font-mono">
                                                                    {claim}
                                                                </Badge>
                                                            ))}
                                                            {scope.claims.length > 2 && (
                                                                <Badge variant="outline" className="text-[10px]">
                                                                    +{scope.claims.length - 2}
                                                                </Badge>
                                                            )}
                                                        </div>
                                                    ) : (
                                                        <span className="text-xs text-muted-foreground">—</span>
                                                    )}
                                                </TableCell>
                                                <TableCell className="text-right">
                                                    <DropdownMenu>
                                                        <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                                                            <Button variant="ghost" size="icon" className="h-8 w-8">
                                                                <MoreHorizontal className="h-4 w-4" />
                                                            </Button>
                                                        </DropdownMenuTrigger>
                                                        <DropdownMenuContent align="end">
                                                            <DropdownMenuItem onClick={(e) => { e.stopPropagation(); openDetailDialog(scope) }}>
                                                                <Eye className="mr-2 h-4 w-4" /> Ver detalles
                                                            </DropdownMenuItem>
                                                            <DropdownMenuItem onClick={(e) => { e.stopPropagation(); openEditDialog(scope) }}>
                                                                <Edit className="mr-2 h-4 w-4" /> Editar
                                                            </DropdownMenuItem>
                                                            <DropdownMenuItem onClick={(e) => { e.stopPropagation(); copyToClipboard(scope.name, "Scope") }}>
                                                                <Copy className="mr-2 h-4 w-4" /> Copiar nombre
                                                            </DropdownMenuItem>
                                                            <DropdownMenuSeparator />
                                                            <DropdownMenuItem
                                                                onClick={(e) => { e.stopPropagation(); setSelectedScope(scope); setDeleteDialogOpen(true) }}
                                                                className="text-red-600"
                                                            >
                                                                <Trash2 className="mr-2 h-4 w-4" /> Eliminar
                                                            </DropdownMenuItem>
                                                        </DropdownMenuContent>
                                                    </DropdownMenu>
                                                </TableCell>
                                            </TableRow>
                                        ))}
                                    </TableBody>
                                </Table>
                            )}
                        </CardContent>
                    </Card>
                </TabsContent>
            </Tabs>

            {/* ============================================================
                CREATE DIALOG
                ============================================================ */}
            <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
                <DialogContent className="max-w-lg">
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2">
                            <Tag className="h-5 w-5 text-primary" />
                            Crear nuevo scope
                        </DialogTitle>
                        <DialogDescription>
                            Define un scope personalizado para controlar permisos en tu API.
                        </DialogDescription>
                    </DialogHeader>

                    <div className="space-y-6 py-4">
                        {/* Name */}
                        <div className="space-y-2">
                            <Label className="flex items-center">
                                Nombre del scope *
                                <InfoTooltip content="Identificador único. Solo minúsculas, números, ':', '.', '_' y '-'. Ejemplo: read:users" />
                            </Label>
                            <Input
                                value={form.name}
                                onChange={(e) => setForm({ ...form, name: formatScopeName(e.target.value) })}
                                placeholder="read:orders"
                                className="font-mono"
                            />
                            <p className="text-xs text-muted-foreground">
                                Usa convención de namespaces: <code>recurso:acción</code> (ej: <code>orders:read</code>, <code>users:write</code>)
                            </p>
                        </div>

                        {/* Display Name */}
                        <div className="space-y-2">
                            <Label className="flex items-center">
                                Nombre para mostrar
                                <InfoTooltip content="Nombre amigable que verán los usuarios en la pantalla de consentimiento." />
                            </Label>
                            <Input
                                value={form.displayName}
                                onChange={(e) => setForm({ ...form, displayName: e.target.value })}
                                placeholder="Leer órdenes"
                            />
                        </div>

                        {/* Description */}
                        <div className="space-y-2">
                            <Label>Descripción</Label>
                            <Textarea
                                value={form.description}
                                onChange={(e) => setForm({ ...form, description: e.target.value })}
                                placeholder="Permite leer el historial de órdenes del usuario"
                                rows={3}
                            />
                        </div>

                        {/* Depends on */}
                        <div className="space-y-2">
                            <Label className="flex items-center">
                                Depende de
                                <InfoTooltip content="Si este scope requiere otro scope para funcionar." />
                            </Label>
                            <Select value={form.dependsOn || "_none"} onValueChange={(v) => setForm({ ...form, dependsOn: v === "_none" ? "" : v })}>
                                <SelectTrigger>
                                    <SelectValue placeholder="Sin dependencia" />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="_none">Sin dependencia</SelectItem>
                                    <SelectItem value="openid">openid</SelectItem>
                                    {allScopes.custom.filter(s => s.name !== form.name).map(scope => (
                                        <SelectItem key={scope.name} value={scope.name}>{scope.name}</SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>

                        {/* Claims hint */}
                        <Alert>
                            <Info className="h-4 w-4" />
                            <AlertDescription className="text-sm">
                                El mapeo de claims a scopes se configura en la sección de <strong>Claims</strong>.
                                Primero crea el scope, luego asócialo a los claims deseados.
                            </AlertDescription>
                        </Alert>
                    </div>

                    <DialogFooter>
                        <Button variant="outline" onClick={() => setCreateDialogOpen(false)}>
                            Cancelar
                        </Button>
                        <Button onClick={handleCreate} disabled={createMutation.isPending}>
                            {createMutation.isPending ? (
                                <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Creando...</>
                            ) : (
                                "Crear Scope"
                            )}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* ============================================================
                EDIT DIALOG
                ============================================================ */}
            <Dialog open={editDialogOpen} onOpenChange={setEditDialogOpen}>
                <DialogContent className="max-w-lg">
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2">
                            <Edit className="h-5 w-5 text-primary" />
                            Editar scope
                        </DialogTitle>
                        <DialogDescription>
                            Modifica la configuración del scope <code>{selectedScope?.name}</code>
                        </DialogDescription>
                    </DialogHeader>

                    <div className="space-y-6 py-4">
                        {/* Name (read-only) */}
                        <div className="space-y-2">
                            <Label className="flex items-center">
                                Nombre del scope
                                <InfoTooltip content="El nombre no puede modificarse después de crear el scope." />
                            </Label>
                            <Input
                                value={form.name}
                                disabled
                                className="font-mono bg-muted"
                            />
                        </div>

                        {/* Display Name */}
                        <div className="space-y-2">
                            <Label>Nombre para mostrar</Label>
                            <Input
                                value={form.displayName}
                                onChange={(e) => setForm({ ...form, displayName: e.target.value })}
                                placeholder="Nombre amigable"
                            />
                        </div>

                        {/* Description */}
                        <div className="space-y-2">
                            <Label>Descripción</Label>
                            <Textarea
                                value={form.description}
                                onChange={(e) => setForm({ ...form, description: e.target.value })}
                                placeholder="Descripción del scope"
                                rows={3}
                            />
                        </div>
                    </div>

                    <DialogFooter>
                        <Button variant="outline" onClick={() => setEditDialogOpen(false)}>
                            Cancelar
                        </Button>
                        <Button onClick={handleUpdate} disabled={updateMutation.isPending}>
                            {updateMutation.isPending ? (
                                <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Guardando...</>
                            ) : (
                                "Guardar cambios"
                            )}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* ============================================================
                DETAIL DIALOG
                ============================================================ */}
            <Dialog open={detailDialogOpen} onOpenChange={setDetailDialogOpen}>
                <DialogContent className="max-w-2xl">
                    <DialogHeader>
                        <div className="flex items-center justify-between">
                            <div className="flex items-center gap-3">
                                {selectedScope && (
                                    <>
                                        <div className={cn(
                                            "p-2.5 rounded-xl",
                                            selectedScope.system || OIDC_STANDARD_SCOPES.some(s => s.name === selectedScope.name)
                                                ? "bg-blue-500/10 text-blue-600"
                                                : "bg-purple-500/10 text-purple-600"
                                        )}>
                                            {(() => { const Icon = getScopeIcon(selectedScope.name); return <Icon className="h-5 w-5" /> })()}
                                        </div>
                                        <div>
                                            <DialogTitle className="flex items-center gap-2">
                                                <code className="font-mono">{selectedScope.name}</code>
                                                {(selectedScope.system || OIDC_STANDARD_SCOPES.some(s => s.name === selectedScope.name)) && (
                                                    <Badge variant="secondary" className="text-[10px]">
                                                        <Lock className="h-2.5 w-2.5 mr-1" />
                                                        OIDC Standard
                                                    </Badge>
                                                )}
                                            </DialogTitle>
                                            {selectedScope.display_name && selectedScope.display_name !== selectedScope.name && (
                                                <DialogDescription>{selectedScope.display_name}</DialogDescription>
                                            )}
                                        </div>
                                    </>
                                )}
                            </div>
                            {selectedScope && !selectedScope.system && !OIDC_STANDARD_SCOPES.some(s => s.name === selectedScope.name) && (
                                <div className="flex items-center gap-2">
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        onClick={() => { setDetailDialogOpen(false); openEditDialog(selectedScope) }}
                                    >
                                        <Edit className="h-4 w-4 mr-2" />
                                        Editar
                                    </Button>
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        onClick={() => { setDeleteDialogOpen(true) }}
                                    >
                                        <Trash2 className="h-4 w-4 mr-2 text-red-500" />
                                        Eliminar
                                    </Button>
                                </div>
                            )}
                        </div>
                    </DialogHeader>

                    {selectedScope && (
                        <div className="space-y-6 py-4">
                            {/* Description */}
                            <div className="space-y-2">
                                <Label className="text-muted-foreground text-xs uppercase tracking-wide">Descripción</Label>
                                <p className="text-sm">
                                    {selectedScope.description || "Sin descripción"}
                                </p>
                            </div>

                            {/* Dependency */}
                            {selectedScope.depends_on && (
                                <div className="space-y-2">
                                    <Label className="text-muted-foreground text-xs uppercase tracking-wide">Depende de</Label>
                                    <div className="flex items-center gap-2">
                                        <Link2 className="h-4 w-4 text-muted-foreground" />
                                        <code className="bg-muted px-2 py-1 rounded text-sm">{selectedScope.depends_on}</code>
                                    </div>
                                </div>
                            )}

                            {/* Claims */}
                            {selectedScope.claims && selectedScope.claims.length > 0 && (
                                <div className="space-y-3">
                                    <Label className="text-muted-foreground text-xs uppercase tracking-wide">
                                        Claims incluidos ({selectedScope.claims.length})
                                    </Label>
                                    <div className="grid grid-cols-2 md:grid-cols-3 gap-2">
                                        {selectedScope.claims.map(claim => {
                                            const claimInfo = COMMON_CLAIMS.find(c => c.name === claim)
                                            return (
                                                <div
                                                    key={claim}
                                                    className="flex items-center gap-2 p-2 rounded-lg bg-muted/50 border"
                                                >
                                                    <code className="text-xs font-mono flex-1">{claim}</code>
                                                    {claimInfo && (
                                                        <InfoTooltip content={claimInfo.description} />
                                                    )}
                                                </div>
                                            )
                                        })}
                                    </div>
                                </div>
                            )}

                            {/* Usage example */}
                            <div className="space-y-3">
                                <Label className="text-muted-foreground text-xs uppercase tracking-wide">Uso en solicitud OAuth2</Label>
                                <div className="rounded-lg bg-muted p-4 font-mono text-sm">
                                    <p className="text-muted-foreground mb-2"># Incluir en el parámetro scope:</p>
                                    <code className="text-primary">scope=openid {selectedScope.name}</code>
                                </div>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    className="w-full"
                                    onClick={() => copyToClipboard(`openid ${selectedScope.name}`, "Scope string")}
                                >
                                    {copiedField === "Scope string" ? <Check className="h-4 w-4 mr-2" /> : <Copy className="h-4 w-4 mr-2" />}
                                    Copiar string de scope
                                </Button>
                            </div>
                        </div>
                    )}
                </DialogContent>
            </Dialog>

            {/* ============================================================
                DELETE CONFIRMATION DIALOG
                ============================================================ */}
            <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2 text-red-600">
                            <AlertTriangle className="h-5 w-5" />
                            Eliminar Scope
                        </DialogTitle>
                        <DialogDescription>
                            ¿Estás seguro de que deseas eliminar el scope <strong>{selectedScope?.name}</strong>?
                            Esta acción es irreversible.
                        </DialogDescription>
                    </DialogHeader>

                    <Alert variant="destructive" className="my-4">
                        <AlertTriangle className="h-4 w-4" />
                        <AlertDescription>
                            Las aplicaciones que usen este scope dejarán de funcionar correctamente.
                            Revisa las dependencias antes de eliminar.
                        </AlertDescription>
                    </Alert>

                    <DialogFooter>
                        <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>Cancelar</Button>
                        <Button variant="destructive" onClick={handleDelete} disabled={deleteMutation.isPending}>
                            {deleteMutation.isPending ? (
                                <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Eliminando...</>
                            ) : (
                                "Eliminar"
                            )}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
