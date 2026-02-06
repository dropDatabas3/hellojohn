"use client"

import { useState, useMemo } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useParams, useRouter } from "next/navigation"
import {
    Plus,
    Search,
    Trash2,
    Copy,
    Check,
    ArrowLeft,
    Globe,
    Server,
    RefreshCw,
    AlertTriangle,
    MoreHorizontal,
    Eye,
    RotateCcw,
    Loader2,
    Monitor,
} from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import Link from "next/link"
import { useToast } from "@/hooks/use-toast"
import type { Tenant } from "@/lib/types"

// DS Components (UI Unification)
import {
    Button,
    Input,
    Card, CardContent, CardHeader,
    Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
    Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
    Badge,
    InlineAlert,
    DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger,
    cn,
} from "@/components/ds"

// Client wizard: types, constants, helpers, components
import type { ClientFormState, ClientRow, AppSubType } from "@/components/clients/wizard"
import {
    formatRelativeTime,
    ClientWizard,
} from "@/components/clients/wizard"

// Client shared components
import { StatCard } from "@/components/clients/shared"


// ============================================================================
// MAIN COMPONENT
// ============================================================================

export default function ClientsClientPage() {
    const params = useParams()
    const router = useRouter()
    const { t } = useI18n()
    const { toast } = useToast()
    const queryClient = useQueryClient()
    const tenantId = params.tenant_id as string

    // UI State
    const [search, setSearch] = useState("")
    const [createDialogOpen, setCreateDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [rotateSecretDialogOpen, setRotateSecretDialogOpen] = useState(false)
    const [selectedClient, setSelectedClient] = useState<ClientRow | null>(null)
    const [copiedField, setCopiedField] = useState<string | null>(null)

    // Track subType for navigation after creation
    const [pendingSubType, setPendingSubType] = useState<AppSubType | undefined>()

    // ========================================================================
    // QUERIES
    // ========================================================================

    const { data: tenant } = useQuery({
        queryKey: ["tenant", tenantId],
        enabled: !!tenantId,
        queryFn: () => api.get<Tenant>(`/v2/admin/tenants/${tenantId}`),
    })

    const { data: clientsRaw, isLoading, refetch } = useQuery({
        queryKey: ["clients", tenantId],
        enabled: !!tenantId,
        queryFn: () => api.get<ClientRow[]>(`/v2/admin/tenants/${tenantId}/clients`),
    })

    const clients = clientsRaw || []

    // ========================================================================
    // MUTATIONS
    // ========================================================================

    const createMutation = useMutation({
        mutationFn: (data: ClientFormState) =>
            api.post<ClientRow>(`/v2/admin/tenants/${tenantId}/clients`, {
                client_id: data.clientId,
                name: data.name,
                type: data.type,
                redirect_uris: data.redirectUris,
                allowed_origins: data.allowedOrigins || [],
                post_logout_uris: data.postLogoutUris || [],
                providers: data.providers || [],
                scopes: data.scopes || [],
                grant_types: data.grantTypes || [],
                access_token_ttl: data.accessTokenTTL,
                refresh_token_ttl: data.refreshTokenTTL,
                id_token_ttl: data.idTokenTTL,
                require_email_verification: data.requireEmailVerification || false,
                reset_password_url: data.resetPasswordUrl || "",
                verify_email_url: data.verifyEmailUrl || "",
                front_channel_logout_url: data.frontChannelLogoutUrl || "",
                back_channel_logout_url: data.backChannelLogoutUrl || "",
            }),
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
            setCreateDialogOpen(false)

            // Store secret in sessionStorage (more secure than URL)
            if (data.secret) {
                sessionStorage.setItem(`client_secret_${data.client_id}`, data.secret)
            }

            // Navigate to detail page with minimal params
            const params = new URLSearchParams({
                created: "true",
                ...(pendingSubType && { subType: pendingSubType }),
            })
            router.push(`/admin/tenants/${tenantId}/clients/${data.client_id}?${params.toString()}`)
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo crear el cliente",
                variant: "destructive",
            })
        },
    })

    const updateMutation = useMutation({
        mutationFn: ({ clientId, data }: { clientId: string; data: Partial<ClientFormState> }) =>
            api.put<ClientRow>(`/v2/admin/tenants/${tenantId}/clients/${clientId}`, {
                name: data.name,
                redirect_uris: data.redirectUris,
                allowed_origins: data.allowedOrigins || [],
                post_logout_uris: data.postLogoutUris || [],
                providers: data.providers || [],
                scopes: data.scopes || [],
                grant_types: data.grantTypes || [],
                access_token_ttl: data.accessTokenTTL,
                refresh_token_ttl: data.refreshTokenTTL,
                id_token_ttl: data.idTokenTTL,
                require_email_verification: data.requireEmailVerification || false,
                reset_password_url: data.resetPasswordUrl || "",
                verify_email_url: data.verifyEmailUrl || "",
                front_channel_logout_url: data.frontChannelLogoutUrl || "",
                back_channel_logout_url: data.backChannelLogoutUrl || "",
            }),
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
            setSelectedClient(data)
            toast({
                title: "Cliente actualizado",
                description: "Los cambios han sido guardados.",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo actualizar el cliente",
                variant: "destructive",
            })
        },
    })

    const deleteMutation = useMutation({
        mutationFn: (clientId: string) => api.delete(`/v2/admin/tenants/${tenantId}/clients/${clientId}`),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
            setDeleteDialogOpen(false)
            setSelectedClient(null)
            toast({
                title: "Cliente eliminado",
                description: "El cliente ha sido eliminado permanentemente.",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo eliminar el cliente",
                variant: "destructive",
            })
        },
    })

    const rotateSecretMutation = useMutation({
        mutationFn: (clientId: string) => api.post<{ client_id: string; new_secret: string }>(`/v2/admin/tenants/${tenantId}/clients/${clientId}/revoke`, {}),
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
            setRotateSecretDialogOpen(false)

            // Store new secret in sessionStorage (more secure than URL)
            if (data.new_secret) {
                sessionStorage.setItem(`client_secret_${data.client_id}`, data.new_secret)
            }

            // Navigate to detail page with minimal params
            const params = new URLSearchParams({
                created: "true",
            })
            router.push(`/admin/tenants/${tenantId}/clients/${data.client_id}?${params.toString()}`)

            toast({
                title: "Secret rotado",
                description: "El client_secret ha sido rotado. Guarda el nuevo secret ahora.",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo rotar el secret",
                variant: "destructive",
            })
        },
    })

    // ========================================================================
    // HANDLERS
    // ========================================================================

    const filteredClients = useMemo(() => {
        return clients.filter(
            (client) =>
                client.name.toLowerCase().includes(search.toLowerCase()) ||
                client.client_id?.toLowerCase().includes(search.toLowerCase())
        )
    }, [clients, search])

    const handleWizardSubmit = (formData: ClientFormState) => {
        setPendingSubType(formData.subType)
        createMutation.mutate(formData)
    }

    const handleDelete = () => {
        if (selectedClient) {
            deleteMutation.mutate(selectedClient.client_id)
        }
    }

    const handleRotateSecret = () => {
        if (selectedClient) {
            rotateSecretMutation.mutate(selectedClient.client_id)
        }
    }

    const openDetailDialog = (client: ClientRow) => {
        // Navigate to detail page
        router.push(`/admin/tenants/${tenantId}/clients/${client.client_id}`)
    }

    const copyToClipboard = (text: string, field: string) => {
        navigator.clipboard.writeText(text)
        setCopiedField(field)
        setTimeout(() => setCopiedField(null), 2000)
        toast({ title: "Copiado", description: `${field} copiado al portapapeles.` })
    }

    // Stats
    const stats = useMemo(() => ({
        total: clients.length,
        public: clients.filter(c => c.type === "public").length,
        confidential: clients.filter(c => c.type === "confidential").length,
    }), [clients])

    // ========================================================================
    // RENDER
    // ========================================================================

    return (
        <div className="space-y-6 animate-in fade-in duration-500">
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
                            <h1 className="text-2xl font-bold tracking-tight">Clients OAuth2</h1>
                            <p className="text-sm text-muted-foreground">
                                {tenant?.name} — Gestiona las aplicaciones que pueden autenticar usuarios
                            </p>
                        </div>
                    </div>
                </div>
                <Button onClick={() => setCreateDialogOpen(true)} className="shadow-clay-button hover:shadow-clay-card transition-shadow">
                    <Plus className="mr-2 h-4 w-4" />
                    Nuevo Client
                </Button>
            </div>

            {/* Info Banner - Premium gradient */}
            <InlineAlert variant="success">
                Un <strong>Client</strong> representa una aplicación que puede autenticar usuarios mediante HelloJohn.
                Los clients <strong>públicos</strong> (SPAs, apps móviles) usan PKCE sin secreto.
                Los clients <strong>confidenciales</strong> (backends, APIs) tienen un client_secret seguro.
            </InlineAlert>

            {/* Stats */}
            <div className="grid grid-cols-3 gap-4">
                <StatCard icon={Globe} label="Total Clients" value={stats.total} variant="default" isLoading={isLoading} />
                <StatCard icon={Monitor} label="Frontend (Públicos)" value={stats.public} variant="success" isLoading={isLoading} />
                <StatCard icon={Server} label="Backend (Confidenciales)" value={stats.confidential} variant="warning" isLoading={isLoading} />
            </div>

            {/* Table Card */}
            <Card>
                <CardHeader className="pb-3">
                    <div className="flex items-center justify-between">
                        <div className="relative flex-1 max-w-sm">
                            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                            <Input
                                placeholder="Buscar por nombre o client_id..."
                                value={search}
                                onChange={(e) => setSearch(e.target.value)}
                                className="pl-9"
                            />
                        </div>
                        <Button variant="outline" size="sm" onClick={() => refetch()}>
                            <RefreshCw className="h-4 w-4 mr-2" />
                            Actualizar
                        </Button>
                    </div>
                </CardHeader>
                <CardContent>
                    {isLoading ? (
                        <div className="flex items-center justify-center py-12">
                            <Loader2 className="h-8 w-8 animate-spin text-accent" />
                        </div>
                    ) : filteredClients.length === 0 ? (
                        <div className="flex flex-col items-center justify-center py-16">
                            <div className="relative mb-6 group">
                                <div className="absolute inset-0 bg-gradient-to-br from-success/30 to-accent/20 rounded-full blur-2xl scale-150 group-hover:scale-175 transition-transform duration-700" />
                                <div className="relative rounded-2xl bg-gradient-to-br from-success/10 to-accent/5 p-8 border border-success/20 shadow-clay-card">
                                    <Globe className="h-12 w-12 text-success" />
                                </div>
                            </div>
                            <h3 className="text-xl font-bold mb-2">No hay clients</h3>
                            <p className="text-muted-foreground text-center max-w-sm text-sm mb-6">
                                {search
                                    ? "No se encontraron clients con ese criterio de búsqueda."
                                    : "Crea tu primer client OAuth2 para permitir que aplicaciones autentiquen usuarios."}
                            </p>
                            {!search && (
                                <Button onClick={() => setCreateDialogOpen(true)} size="lg" className="shadow-clay-button">
                                    <Plus className="mr-2 h-4 w-4" />
                                    Crear primer client
                                </Button>
                            )}
                        </div>
                    ) : (
                        <Table>
                            <TableHeader>
                                <TableRow className="bg-muted/30">
                                    <TableHead>Nombre</TableHead>
                                    <TableHead>Client ID</TableHead>
                                    <TableHead>Tipo</TableHead>
                                    <TableHead>Grant Types</TableHead>
                                    <TableHead>URIs</TableHead>
                                    <TableHead className="text-right">Acciones</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {filteredClients.map((client) => (
                                    <TableRow key={client.client_id} className="cursor-pointer hover:bg-muted/50 transition-colors" onClick={() => openDetailDialog(client)}>
                                        <TableCell>
                                            <div className="flex items-center gap-3">
                                                <div className={cn(
                                                    "p-2 rounded-lg transition-all duration-200",
                                                    client.type === "public"
                                                        ? "bg-success/10 text-success"
                                                        : "bg-accent/10 text-accent"
                                                )}>
                                                    {client.type === "public" ? <Globe className="h-4 w-4" /> : <Server className="h-4 w-4" />}
                                                </div>
                                                <div>
                                                    <p className="font-medium">{client.name}</p>
                                                    <p className="text-xs text-muted-foreground">
                                                        {client.created_at ? `Creado ${formatRelativeTime(client.created_at)}` : ""}
                                                    </p>
                                                </div>
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            <div className="flex items-center gap-2">
                                                <code className="text-xs bg-muted px-2 py-1 rounded max-w-[150px] truncate" title={client.client_id}>
                                                    {client.client_id}
                                                </code>
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="h-6 w-6"
                                                    onClick={(e) => { e.stopPropagation(); copyToClipboard(client.client_id, "Client ID") }}
                                                >
                                                    {copiedField === "Client ID" ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
                                                </Button>
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            <Badge variant={client.type === "confidential" ? "default" : "secondary"}>
                                                {client.type === "confidential" ? "Backend" : "Frontend"}
                                            </Badge>
                                        </TableCell>
                                        <TableCell>
                                            <div className="flex flex-wrap gap-1">
                                                {(client.grant_types || ["authorization_code"]).slice(0, 2).map(gt => (
                                                    <Badge key={gt} variant="outline" className="text-[10px]">
                                                        {gt.replace("_", " ")}
                                                    </Badge>
                                                ))}
                                                {(client.grant_types || []).length > 2 && (
                                                    <Badge variant="outline" className="text-[10px]">+{(client.grant_types || []).length - 2}</Badge>
                                                )}
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            <span className="text-sm text-muted-foreground">
                                                {client.redirect_uris?.length || 0} redirect
                                            </span>
                                        </TableCell>
                                        <TableCell className="text-right">
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                                                    <Button variant="ghost" size="sm" className="h-8 w-8">
                                                        <MoreHorizontal className="h-4 w-4" />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    <DropdownMenuItem onClick={(e) => { e.stopPropagation(); openDetailDialog(client) }}>
                                                        <Eye className="mr-2 h-4 w-4" /> Ver detalles
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem onClick={(e) => { e.stopPropagation(); copyToClipboard(client.client_id, "Client ID") }}>
                                                        <Copy className="mr-2 h-4 w-4" /> Copiar Client ID
                                                    </DropdownMenuItem>
                                                    {client.type === "confidential" && (
                                                        <DropdownMenuItem onClick={(e) => { e.stopPropagation(); setSelectedClient(client); setRotateSecretDialogOpen(true) }}>
                                                            <RotateCcw className="mr-2 h-4 w-4" /> Rotar Secret
                                                        </DropdownMenuItem>
                                                    )}
                                                    <DropdownMenuSeparator />
                                                    <DropdownMenuItem
                                                        onClick={(e) => { e.stopPropagation(); setSelectedClient(client); setDeleteDialogOpen(true) }}
                                                        className="text-danger hover:text-danger hover:bg-danger/10"
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

            {/* ============================================================
                CREATE DIALOG - WIZARD
                ============================================================ */}
            <ClientWizard
                open={createDialogOpen}
                onOpenChange={setCreateDialogOpen}
                tenantSlug={tenant?.slug || ""}
                onSubmit={handleWizardSubmit}
                isPending={createMutation.isPending}
            />

            {/* ============================================================
                DELETE CONFIRMATION DIALOG
                ============================================================ */}
            <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2 text-danger">
                            <AlertTriangle className="h-5 w-5" />
                            Eliminar Client
                        </DialogTitle>
                        <DialogDescription>
                            ¿Estás seguro de que deseas eliminar <strong>{selectedClient?.name}</strong>?
                            Esta acción es irreversible y revocará todos los tokens activos.
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>Cancelar</Button>
                        <Button variant="danger" onClick={handleDelete} disabled={deleteMutation.isPending}>
                            {deleteMutation.isPending ? (
                                <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Eliminando...</>
                            ) : (
                                "Eliminar"
                            )}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* ============================================================
                ROTATE SECRET CONFIRMATION DIALOG
                ============================================================ */}
            <Dialog open={rotateSecretDialogOpen} onOpenChange={setRotateSecretDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2 text-warning">
                            <RotateCcw className="h-5 w-5" />
                            Rotar Client Secret
                        </DialogTitle>
                        <DialogDescription>
                            ¿Estás seguro de que deseas rotar el secret de <strong>{selectedClient?.name}</strong>?
                            El secret actual dejará de funcionar inmediatamente.
                        </DialogDescription>
                    </DialogHeader>
                    <InlineAlert
                        variant="danger"
                        className="my-4"
                        description="Todas las aplicaciones que usen el secret actual dejarán de funcionar hasta que actualices la configuración con el nuevo secret."
                    />
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setRotateSecretDialogOpen(false)}>Cancelar</Button>
                        <Button variant="danger" onClick={handleRotateSecret} disabled={rotateSecretMutation.isPending}>
                            {rotateSecretMutation.isPending ? (
                                <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Rotando...</>
                            ) : (
                                "Rotar Secret"
                            )}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
