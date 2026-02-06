"use client"

import { useState, useMemo } from "react"
import { useParams } from "next/navigation"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
    Tag, Code2, Settings2, Eye, Plus, Trash2, Edit2,
    AlertCircle, Info,
    HelpCircle, Copy, Check, Sparkles, Shield, Database,
    Webhook, FileJson, Lock,
    ArrowLeft
} from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { useToast } from "@/hooks/use-toast"
import Link from "next/link"
import type {
    Tenant, ClaimDefinition, ClaimSource, StandardClaim,
    ClaimMapping, ClaimsConfig, ClaimsSettings
} from "@/lib/types"

// Design System Components
import {
    Button, Card, Input, Label, Badge, Skeleton, Switch,
    Select, SelectTrigger, SelectValue, SelectContent, SelectItem,
    Dialog, DialogContent, DialogHeader, DialogFooter, DialogTitle, DialogDescription,
    Tabs, TabsList, TabsTrigger, TabsContent,
    Table, TableHeader, TableBody, TableRow, TableHead, TableCell,
    Textarea,
    Tooltip, TooltipTrigger, TooltipContent, TooltipProvider,
    InlineAlert, cn,
} from "@/components/ds"

// ─── Mock Data (Replace with real API when backend is ready) ───

const STANDARD_OIDC_CLAIMS: StandardClaim[] = [
    { name: "sub", description: "Identificador único del usuario (Subject)", enabled: true, scope: "openid" },
    { name: "name", description: "Nombre completo del usuario", enabled: true, scope: "profile" },
    { name: "given_name", description: "Nombre de pila", enabled: true, scope: "profile" },
    { name: "family_name", description: "Apellido", enabled: true, scope: "profile" },
    { name: "nickname", description: "Apodo o nombre casual", enabled: false, scope: "profile" },
    { name: "preferred_username", description: "Nombre de usuario preferido", enabled: false, scope: "profile" },
    { name: "picture", description: "URL de la foto de perfil", enabled: false, scope: "profile" },
    { name: "email", description: "Dirección de correo electrónico", enabled: true, scope: "email" },
    { name: "email_verified", description: "Si el email ha sido verificado", enabled: true, scope: "email" },
    { name: "phone_number", description: "Número de teléfono", enabled: false, scope: "phone" },
    { name: "phone_number_verified", description: "Si el teléfono ha sido verificado", enabled: false, scope: "phone" },
    { name: "address", description: "Dirección física del usuario", enabled: false, scope: "address" },
    { name: "locale", description: "Configuración regional del usuario", enabled: false, scope: "profile" },
    { name: "zoneinfo", description: "Zona horaria del usuario", enabled: false, scope: "profile" },
]

const MOCK_CUSTOM_CLAIMS: ClaimDefinition[] = [
    {
        id: "claim-1",
        name: "department",
        description: "Departamento del empleado",
        source: "user_field",
        value: "metadata.department",
        always_include: false,
        scopes: ["profile"],
        enabled: true,
        system: false,
    },
    {
        id: "claim-2",
        name: "roles",
        description: "Roles asignados al usuario",
        source: "user_field",
        value: "metadata.roles",
        always_include: true,
        scopes: [],
        enabled: true,
        system: false,
    },
    {
        id: "claim-3",
        name: "company_id",
        description: "Identificador de la empresa",
        source: "static",
        value: "acme-corp-123",
        always_include: true,
        scopes: [],
        enabled: true,
        system: false,
    },
    {
        id: "claim-4",
        name: "is_premium",
        description: "Si el usuario tiene plan premium",
        source: "expression",
        value: 'user.metadata.subscription_type == "premium"',
        always_include: false,
        scopes: ["profile"],
        enabled: false,
        system: false,
    },
]

const MOCK_SCOPE_MAPPINGS: ClaimMapping[] = [
    { scope: "openid", claims: ["sub", "iss", "aud", "exp", "iat", "auth_time"] },
    { scope: "profile", claims: ["name", "given_name", "family_name", "department", "roles"] },
    { scope: "email", claims: ["email", "email_verified"] },
    { scope: "phone", claims: ["phone_number", "phone_number_verified"] },
    { scope: "address", claims: ["address"] },
]

const DEFAULT_SETTINGS: ClaimsSettings = {
    include_in_access_token: true,
    use_namespaced_claims: false,
    namespace_prefix: "",
}

// ─── Helper Functions ───

const getSourceIcon = (source: ClaimSource) => {
    switch (source) {
        case "user_field": return Database
        case "static": return Tag
        case "expression": return Code2
        case "external": return Webhook
        default: return Tag
    }
}

const getSourceLabel = (source: ClaimSource) => {
    switch (source) {
        case "user_field": return "Campo de Usuario"
        case "static": return "Valor Estático"
        case "expression": return "Expresión CEL"
        case "external": return "API Externa"
        default: return source
    }
}

// ─── Info Tooltip Component ───

function InfoTooltip({ content }: { content: string }) {
    return (
        <TooltipProvider delayDuration={200}>
            <Tooltip>
                <TooltipTrigger asChild>
                    <button className="ml-1.5 inline-flex">
                        <HelpCircle className="h-3.5 w-3.5 text-muted-foreground/60 hover:text-muted-foreground transition-colors" />
                    </button>
                </TooltipTrigger>
                <TooltipContent side="top" className="max-w-xs text-xs">
                    {content}
                </TooltipContent>
            </Tooltip>
        </TooltipProvider>
    )
}

// ─── Stats Card Component (Clay Migration) ───

function StatCard({
    icon: Icon,
    label,
    value,
    subValue,
    isLoading = false,
}: {
    icon: React.ElementType
    label: string
    value: string | number
    subValue?: string
    isLoading?: boolean
}) {
    return (
        <Card interactive className="group p-4">
            <div className="flex items-start justify-between">
                <div className="space-y-1">
                    {isLoading ? (
                        <>
                            <Skeleton className="h-4 w-24" />
                            <Skeleton className="h-8 w-16 mt-1" />
                            <Skeleton className="h-3 w-28 mt-0.5" />
                        </>
                    ) : (
                        <>
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{label}</p>
                            <p className="mt-1 text-2xl font-semibold text-foreground">{value}</p>
                            {subValue && <p className="mt-0.5 text-xs text-muted-foreground">{subValue}</p>}
                        </>
                    )}
                </div>
                <div className={cn("p-2 rounded-lg", isLoading ? "bg-muted/30" : "bg-accent/10 text-accent")}>
                    {isLoading ? (
                        <Skeleton className="h-5 w-5 rounded-full" />
                    ) : (
                        <Icon className="h-5 w-5" />
                    )}
                </div>
            </div>
        </Card>
    )
}

// ─── Empty State Component (Clay Migration) ───

function EmptyState({ title, description, icon: Icon, action }: {
    title: string
    description: string
    icon: React.ElementType
    action?: { label: string; onClick: () => void }
}) {
    return (
        <div className="flex flex-col items-center justify-center py-16 px-4 text-center">
            <div className="h-16 w-16 rounded-full bg-muted flex items-center justify-center mb-4">
                <Icon className="h-8 w-8 text-muted-foreground" />
            </div>
            <h3 className="text-lg font-medium text-foreground">{title}</h3>
            <p className="mt-1 text-sm text-muted-foreground max-w-sm">{description}</p>
            {action && (
                <Button
                    onClick={action.onClick}
                    className="mt-4 hover:-translate-y-0.5 hover:shadow-clay-card transition-all duration-200"
                    variant="outline"
                >
                    <Plus className="h-4 w-4 mr-2" />
                    {action.label}
                </Button>
            )}
        </div>
    )
}

// ─── Token Preview Component (Clay Migration) ───

function TokenPreview({
    standardClaims,
    customClaims,
    settings
}: {
    standardClaims: StandardClaim[]
    customClaims: ClaimDefinition[]
    settings: ClaimsSettings
}) {
    const [copied, setCopied] = useState(false)

    const exampleToken = useMemo(() => {
        const now = Math.floor(Date.now() / 1000)
        const token: Record<string, any> = {
            iss: "https://auth.example.com",
            sub: "user_abc123xyz",
            aud: "my-application",
            iat: now,
            exp: now + 3600,
            nbf: now,
        }

        // Add enabled standard claims
        standardClaims.filter(c => c.enabled).forEach(claim => {
            switch (claim.name) {
                case "name": token.name = "Juan García"; break
                case "given_name": token.given_name = "Juan"; break
                case "family_name": token.family_name = "García"; break
                case "email": token.email = "juan@example.com"; break
                case "email_verified": token.email_verified = true; break
                case "phone_number": token.phone_number = "+34612345678"; break
                case "phone_number_verified": token.phone_number_verified = true; break
                case "picture": token.picture = "https://example.com/photo.jpg"; break
                case "locale": token.locale = "es-ES"; break
                case "zoneinfo": token.zoneinfo = "Europe/Madrid"; break
            }
        })

        // Add custom claims
        const customSection: Record<string, any> = {}
        customClaims.filter(c => c.enabled).forEach(claim => {
            const value = claim.source === "static" ? claim.value :
                claim.source === "user_field" ? `<${claim.value}>` :
                    claim.source === "expression" ? "<evaluated>" : "<fetched>"

            if (settings.use_namespaced_claims && settings.namespace_prefix) {
                token[`${settings.namespace_prefix}/${claim.name}`] = value
            } else if (settings.include_in_access_token) {
                customSection[claim.name] = value
            }
        })

        if (Object.keys(customSection).length > 0 && !settings.use_namespaced_claims) {
            token.custom = customSection
        }

        return token
    }, [standardClaims, customClaims, settings])

    const handleCopy = () => {
        navigator.clipboard.writeText(JSON.stringify(exampleToken, null, 2))
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
    }

    return (
        <div className="space-y-4">
            <div className="flex items-center justify-between">
                <div>
                    <h3 className="text-sm font-medium flex items-center gap-2">
                        <Eye className="h-4 w-4" />
                        Vista Previa del Token
                        <InfoTooltip content="Este es un ejemplo de cómo se verá el payload del token JWT con la configuración actual de claims." />
                    </h3>
                    <p className="text-xs text-muted-foreground mt-0.5">
                        Ejemplo del payload JWT con la configuración actual
                    </p>
                </div>
                <Button
                    variant="outline"
                    size="sm"
                    onClick={handleCopy}
                    className="hover:-translate-y-0.5 transition-all duration-200"
                >
                    {copied ? <Check className="h-3.5 w-3.5 mr-1.5" /> : <Copy className="h-3.5 w-3.5 mr-1.5" />}
                    {copied ? "Copiado" : "Copiar"}
                </Button>
            </div>

            <div className="relative">
                <pre className="p-4 rounded-lg bg-card border border-border text-foreground text-xs overflow-x-auto font-mono">
                    <code>{JSON.stringify(exampleToken, null, 2)}</code>
                </pre>
                <div className="absolute top-2 right-2">
                    <Badge variant="outline" className="text-[10px] bg-accent/10 text-accent border-accent/30">
                        JWT Payload
                    </Badge>
                </div>
            </div>

            <InlineAlert variant="info">
                <Info className="h-4 w-4" />
                <div>
                    <p className="font-medium text-sm">Nota sobre valores dinámicos</p>
                    <p className="text-xs mt-0.5">
                        Los valores entre corchetes angulares (&lt;...&gt;) se resuelven en tiempo de emisión del token según la fuente configurada.
                    </p>
                </div>
            </InlineAlert>
        </div>
    )
}

// ─── Claim Editor Dialog (Clay Migration) ───

function ClaimEditorDialog({
    claim,
    open,
    onClose,
    onSave
}: {
    claim?: ClaimDefinition | null
    open: boolean
    onClose: () => void
    onSave: (claim: Omit<ClaimDefinition, "id">) => void
}) {
    const isEditing = !!claim
    const [formData, setFormData] = useState<Omit<ClaimDefinition, "id">>({
        name: claim?.name || "",
        description: claim?.description || "",
        source: claim?.source || "user_field",
        value: claim?.value || "",
        always_include: claim?.always_include || false,
        scopes: claim?.scopes || [],
        enabled: claim?.enabled ?? true,
        system: false,
    })

    const handleSubmit = () => {
        if (!formData.name || !formData.value) return
        onSave(formData)
        onClose()
    }

    const getValuePlaceholder = () => {
        switch (formData.source) {
            case "user_field": return "metadata.department"
            case "static": return "valor-fijo-123"
            case "expression": return 'user.metadata.level >= 5 ? "premium" : "basic"'
            case "external": return "https://api.example.com/claims"
            default: return ""
        }
    }

    const getValueLabel = () => {
        switch (formData.source) {
            case "user_field": return "Ruta del Campo"
            case "static": return "Valor"
            case "expression": return "Expresión CEL"
            case "external": return "URL del Webhook"
            default: return "Valor"
        }
    }

    const getValueHelp = () => {
        switch (formData.source) {
            case "user_field": return "Ruta al campo del usuario usando notación de punto (ej: metadata.department, email, custom_fields.role)"
            case "static": return "Valor literal que se incluirá en todos los tokens"
            case "expression": return "Expresión CEL que se evalúa en tiempo de emisión. Variables disponibles: user, client, scopes"
            case "external": return "URL de un webhook que devuelve el valor del claim. Recibe contexto del usuario en el body."
            default: return ""
        }
    }

    return (
        <Dialog open={open} onOpenChange={onClose}>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-3">
                        <div className="h-10 w-10 rounded-xl bg-accent/10 flex items-center justify-center text-accent">
                            {isEditing ? <Edit2 className="h-5 w-5" /> : <Plus className="h-5 w-5" />}
                        </div>
                        {isEditing ? "Editar Claim" : "Nuevo Claim Personalizado"}
                    </DialogTitle>
                    <DialogDescription>
                        {isEditing
                            ? "Modifica la configuración de este claim personalizado"
                            : "Define un nuevo claim que se incluirá en los tokens JWT"
                        }
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-4 py-4">
                    {/* Name */}
                    <div className="space-y-2">
                        <Label className="flex items-center">
                            Nombre del Claim
                            <InfoTooltip content="Nombre único que aparecerá como key en el token JWT" />
                        </Label>
                        <Input
                            placeholder="department"
                            value={formData.name}
                            onChange={(e) => setFormData({ ...formData, name: e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, "_") })}
                        />
                        <p className="text-xs text-muted-foreground">
                            Solo minúsculas, números y guiones bajos
                        </p>
                    </div>

                    {/* Description */}
                    <div className="space-y-2">
                        <Label>Descripción (opcional)</Label>
                        <Input
                            placeholder="Departamento del empleado"
                            value={formData.description}
                            onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                        />
                    </div>

                    {/* Source Type */}
                    <div className="space-y-2">
                        <Label className="flex items-center">
                            Tipo de Fuente
                            <InfoTooltip content="Determina de dónde se obtiene el valor del claim" />
                        </Label>
                        <Select
                            value={formData.source}
                            onValueChange={(v) => setFormData({ ...formData, source: v as ClaimSource, value: "" })}
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="user_field">
                                    <div className="flex items-center gap-2">
                                        <Database className="h-4 w-4" />
                                        Campo de Usuario
                                    </div>
                                </SelectItem>
                                <SelectItem value="static">
                                    <div className="flex items-center gap-2">
                                        <Tag className="h-4 w-4" />
                                        Valor Estático
                                    </div>
                                </SelectItem>
                                <SelectItem value="expression">
                                    <div className="flex items-center gap-2">
                                        <Code2 className="h-4 w-4" />
                                        Expresión CEL
                                    </div>
                                </SelectItem>
                                <SelectItem value="external">
                                    <div className="flex items-center gap-2">
                                        <Webhook className="h-4 w-4" />
                                        API Externa
                                    </div>
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    {/* Value */}
                    <div className="space-y-2">
                        <Label className="flex items-center">
                            {getValueLabel()}
                            <InfoTooltip content={getValueHelp()} />
                        </Label>
                        {formData.source === "expression" ? (
                            <Textarea
                                placeholder={getValuePlaceholder()}
                                value={formData.value}
                                onChange={(e) => setFormData({ ...formData, value: e.target.value })}
                                className="font-mono text-sm"
                                rows={3}
                            />
                        ) : (
                            <Input
                                placeholder={getValuePlaceholder()}
                                value={formData.value}
                                onChange={(e) => setFormData({ ...formData, value: e.target.value })}
                                className={formData.source === "external" ? "" : "font-mono"}
                            />
                        )}
                    </div>

                    {/* Always Include */}
                    <div className="flex items-center justify-between p-3 rounded-lg border bg-muted">
                        <div className="space-y-0.5">
                            <Label className="flex items-center">
                                Incluir Siempre
                                <InfoTooltip content="Si está habilitado, el claim se incluye en todos los tokens independientemente de los scopes solicitados" />
                            </Label>
                            <p className="text-xs text-muted-foreground">
                                Incluir en todos los tokens sin importar scopes
                            </p>
                        </div>
                        <Switch
                            checked={formData.always_include}
                            onCheckedChange={(v) => setFormData({ ...formData, always_include: v })}
                        />
                    </div>

                    {/* Enabled */}
                    <div className="flex items-center justify-between p-3 rounded-lg border bg-muted">
                        <div className="space-y-0.5">
                            <Label>Habilitado</Label>
                            <p className="text-xs text-muted-foreground">
                                Desactivar para excluir temporalmente de los tokens
                            </p>
                        </div>
                        <Switch
                            checked={formData.enabled}
                            onCheckedChange={(v) => setFormData({ ...formData, enabled: v })}
                        />
                    </div>
                </div>

                <DialogFooter>
                    <Button variant="outline" onClick={onClose}>Cancelar</Button>
                    <Button
                        onClick={handleSubmit}
                        disabled={!formData.name || !formData.value}
                        className="hover:-translate-y-0.5 hover:shadow-clay-card active:translate-y-0 transition-all duration-200"
                    >
                        {isEditing ? "Guardar Cambios" : "Crear Claim"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

// ─── Main Component ───

export default function ClaimsClientPage() {
    const params = useParams()
    const { t } = useI18n()
    const { toast } = useToast()
    const queryClient = useQueryClient()
    const tenantId = params.tenant_id as string

    // UI State
    const [activeTab, setActiveTab] = useState("standard")
    const [editorOpen, setEditorOpen] = useState(false)
    const [editingClaim, setEditingClaim] = useState<ClaimDefinition | null>(null)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [selectedClaim, setSelectedClaim] = useState<ClaimDefinition | null>(null)

    // ========================================================================
    // QUERIES
    // ========================================================================

    const { data: tenant } = useQuery({
        queryKey: ["tenant", tenantId],
        enabled: !!tenantId,
        queryFn: () => api.get<Tenant>(`/v2/admin/tenants/${tenantId}`),
    })

    // Main claims config query - loads all claims data from backend
    const { data: claimsConfig, isLoading: isLoadingClaims } = useQuery({
        queryKey: ["claims", tenantId],
        enabled: !!tenantId,
        queryFn: () => api.get<ClaimsConfig>(`/v2/admin/tenants/${tenantId}/claims`),
    })

    // Derive data from backend or use defaults
    const standardClaims = claimsConfig?.standard_claims ?? STANDARD_OIDC_CLAIMS
    const customClaims = claimsConfig?.custom_claims ?? []
    const scopeMappings = claimsConfig?.scope_mappings ?? MOCK_SCOPE_MAPPINGS
    const settings = claimsConfig?.settings ?? DEFAULT_SETTINGS

    // ========================================================================
    // MUTATIONS
    // ========================================================================

    // Toggle standard claim
    const toggleStandardMutation = useMutation({
        mutationFn: ({ name, enabled }: { name: string; enabled: boolean }) =>
            api.patch(`/v2/admin/tenants/${tenantId}/claims/standard/${name}`, { enabled }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["claims", tenantId] })
            toast({
                title: "Claim actualizado",
                description: "Los cambios se aplicarán a los nuevos tokens",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo actualizar el claim",
                variant: "destructive",
            })
        },
    })

    // Create custom claim
    const createClaimMutation = useMutation({
        mutationFn: (data: Omit<ClaimDefinition, "id">) =>
            api.post<ClaimDefinition>(`/v2/admin/tenants/${tenantId}/claims/custom`, {
                name: data.name,
                description: data.description || "",
                source: data.source,
                value: data.value,
                always_include: data.always_include,
                scopes: data.scopes || [],
                enabled: data.enabled,
            }),
        onSuccess: (_, variables) => {
            queryClient.invalidateQueries({ queryKey: ["claims", tenantId] })
            setEditorOpen(false)
            toast({
                title: "Claim creado",
                description: `El claim "${variables.name}" ha sido agregado`,
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo crear el claim",
                variant: "destructive",
            })
        },
    })

    // Update custom claim
    const updateClaimMutation = useMutation({
        mutationFn: ({ id, data }: { id: string; data: Omit<ClaimDefinition, "id"> }) =>
            api.put(`/v2/admin/tenants/${tenantId}/claims/custom/${id}`, {
                description: data.description || "",
                source: data.source,
                value: data.value,
                always_include: data.always_include,
                scopes: data.scopes || [],
                enabled: data.enabled,
            }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["claims", tenantId] })
            setEditorOpen(false)
            setEditingClaim(null)
            toast({
                title: "Claim actualizado",
                description: "Los cambios han sido guardados",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo actualizar el claim",
                variant: "destructive",
            })
        },
    })

    // Delete custom claim
    const deleteClaimMutation = useMutation({
        mutationFn: (id: string) =>
            api.delete(`/v2/admin/tenants/${tenantId}/claims/custom/${id}`),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["claims", tenantId] })
            setDeleteDialogOpen(false)
            setSelectedClaim(null)
            toast({
                title: "Claim eliminado",
                description: "El claim ha sido removido de la configuración",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo eliminar el claim",
                variant: "destructive",
            })
        },
    })

    // Update settings
    const updateSettingsMutation = useMutation({
        mutationFn: (newSettings: Partial<ClaimsSettings>) =>
            api.patch(`/v2/admin/tenants/${tenantId}/claims/settings`, newSettings),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["claims", tenantId] })
            toast({
                title: "Configuración actualizada",
                description: "Los cambios se aplicarán a los nuevos tokens",
            })
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.message || "No se pudo actualizar la configuración",
                variant: "destructive",
            })
        },
    })

    // ========================================================================
    // STATS
    // ========================================================================

    const stats = useMemo(() => ({
        standardEnabled: standardClaims.filter(c => c.enabled).length,
        standardTotal: standardClaims.length,
        customEnabled: customClaims.filter(c => c.enabled).length,
        customTotal: customClaims.length,
        scopeCount: scopeMappings.length,
    }), [standardClaims, customClaims, scopeMappings])

    // ========================================================================
    // HANDLERS
    // ========================================================================

    const handleToggleStandardClaim = (name: string) => {
        const claim = standardClaims.find(c => c.name === name)
        if (claim) {
            toggleStandardMutation.mutate({ name, enabled: !claim.enabled })
        }
    }

    const handleToggleCustomClaim = (id: string) => {
        const claim = customClaims.find(c => c.id === id)
        if (claim) {
            updateClaimMutation.mutate({
                id,
                data: { ...claim, enabled: !claim.enabled }
            })
        }
    }

    const handleCreateClaim = (claimData: Omit<ClaimDefinition, "id">) => {
        createClaimMutation.mutate(claimData)
    }

    const handleEditClaim = (claimData: Omit<ClaimDefinition, "id">) => {
        if (!editingClaim) return
        updateClaimMutation.mutate({ id: editingClaim.id, data: claimData })
    }

    const handleDeleteClaim = () => {
        if (!selectedClaim) return
        deleteClaimMutation.mutate(selectedClaim.id)
    }

    const handleSettingsChange = (key: keyof ClaimsSettings, value: any) => {
        updateSettingsMutation.mutate({ [key]: value })
    }

    return (
        <div className="animate-in fade-in duration-500">
            {/* Header */}
            <div className="flex items-center justify-between mb-6">
                <div className="flex items-center gap-4">
                    <Button variant="ghost" size="sm" asChild>
                        <Link href={`/admin/tenants/${tenantId}/detail`}>
                            <ArrowLeft className="h-4 w-4" />
                        </Link>
                    </Button>
                    <div className="flex items-center gap-3">
                        <div>
                            <h1 className="text-2xl font-bold tracking-tight">Claims & Tokens</h1>
                            <p className="text-sm text-muted-foreground">
                                {tenant?.name} — Configura qué información incluir en tokens JWT
                            </p>
                        </div>
                    </div>
                </div>
                <Button onClick={() => { setEditingClaim(null); setEditorOpen(true) }} className="shadow-clay-button hover:shadow-clay-card transition-shadow">
                    <Plus className="mr-2 h-4 w-4" />
                    Nuevo Claim
                </Button>
            </div>

            {/* Info Alert */}
            <InlineAlert variant="info" className="mb-6">
                <div>
                    <h4 className="font-semibold mb-1">¿Qué son los Claims?</h4>
                    <p className="text-sm">
                        Los claims son piezas de información incluidas en los tokens JWT que describen al usuario autenticado.
                        Puedes usar claims estándar de OIDC o definir claims personalizados para agregar información específica
                        de tu aplicación como roles, departamentos o atributos de negocio.
                    </p>
                </div>
            </InlineAlert>

            {/* Stats */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
                <StatCard
                    icon={Shield}
                    label="Claims Estándar"
                    value={`${stats.standardEnabled}/${stats.standardTotal}`}
                    subValue="Claims OIDC habilitados"
                    isLoading={isLoadingClaims}
                />
                <StatCard
                    icon={Sparkles}
                    label="Claims Personalizados"
                    value={stats.customEnabled}
                    subValue={`${stats.customTotal} definidos`}
                    isLoading={isLoadingClaims}
                />
                <StatCard
                    icon={Tag}
                    label="Scopes Mapeados"
                    value={stats.scopeCount}
                    subValue="Grupos de claims"
                    isLoading={isLoadingClaims}
                />
                <StatCard
                    icon={FileJson}
                    label="Tokens Emitidos"
                    value="—"
                    subValue="Últimas 24h"
                    isLoading={isLoadingClaims}
                />
            </div>

            {/* Tabs */}
            <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-4">
                <TabsList>
                    <TabsTrigger value="standard" className="gap-2">
                        <Shield className="h-4 w-4" />
                        Claims Estándar
                    </TabsTrigger>
                    <TabsTrigger value="custom" className="gap-2">
                        <Sparkles className="h-4 w-4" />
                        Personalizados
                    </TabsTrigger>
                    <TabsTrigger value="mappings" className="gap-2">
                        <Tag className="h-4 w-4" />
                        Mapeo de Scopes
                    </TabsTrigger>
                    <TabsTrigger value="preview" className="gap-2">
                        <Eye className="h-4 w-4" />
                        Vista Previa
                    </TabsTrigger>
                    <TabsTrigger value="settings" className="gap-2">
                        <Settings2 className="h-4 w-4" />
                        Configuración
                    </TabsTrigger>
                </TabsList>

                {/* Standard Claims Tab */}
                <TabsContent value="standard" className="space-y-4">
                    <Card className="overflow-hidden">
                        <div className="p-4 border-b">
                            <h3 className="font-medium flex items-center gap-2">
                                <Shield className="h-4 w-4 text-accent" />
                                Claims OIDC Estándar
                                <InfoTooltip content="Claims definidos por el estándar OpenID Connect. Se incluyen automáticamente según los scopes solicitados por la aplicación." />
                            </h3>
                            <p className="text-xs text-muted-foreground mt-1">
                                Habilita o deshabilita los claims estándar de OpenID Connect
                            </p>
                        </div>
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead className="w-[200px]">Claim</TableHead>
                                    <TableHead>Descripción</TableHead>
                                    <TableHead className="w-[120px]">Scope</TableHead>
                                    <TableHead className="w-[100px] text-right">Estado</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {standardClaims.map((claim) => (
                                    <TableRow
                                        key={claim.name}
                                        className="hover:bg-accent/5 transition-colors"
                                    >
                                        <TableCell>
                                            <code className="text-sm font-mono bg-muted px-2 py-1 rounded">
                                                {claim.name}
                                            </code>
                                        </TableCell>
                                        <TableCell className="text-muted-foreground text-sm">
                                            {claim.description}
                                        </TableCell>
                                        <TableCell>
                                            <Badge variant="outline" className="text-xs">
                                                {claim.scope}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="text-right">
                                            <Switch
                                                checked={claim.enabled}
                                                onCheckedChange={() => handleToggleStandardClaim(claim.name)}
                                            />
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </Card>
                </TabsContent>

                {/* Custom Claims Tab */}
                <TabsContent value="custom" className="space-y-4">
                    <div className="flex items-center justify-between">
                        <div>
                            <h3 className="font-medium">Claims Personalizados</h3>
                            <p className="text-xs text-muted-foreground">
                                Define claims adicionales para incluir información específica de tu aplicación
                            </p>
                        </div>
                        <Button
                            onClick={() => { setEditingClaim(null); setEditorOpen(true) }}
                            className="hover:-translate-y-0.5 hover:shadow-clay-card active:translate-y-0 transition-all duration-200"
                        >
                            <Plus className="h-4 w-4 mr-2" />
                            Nuevo Claim
                        </Button>
                    </div>

                    {customClaims.length === 0 ? (
                        <EmptyState
                            icon={Sparkles}
                            title="Sin claims personalizados"
                            description="Crea tu primer claim personalizado para agregar información específica a los tokens"
                            action={{
                                label: "Crear Claim",
                                onClick: () => { setEditingClaim(null); setEditorOpen(true) }
                            }}
                        />
                    ) : (
                        <Card className="overflow-hidden">
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead className="w-[180px]">Claim</TableHead>
                                        <TableHead>Fuente</TableHead>
                                        <TableHead>Valor / Expresión</TableHead>
                                        <TableHead className="w-[100px]">Inclusión</TableHead>
                                        <TableHead className="w-[80px] text-center">Estado</TableHead>
                                        <TableHead className="w-[100px] text-right">Acciones</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {customClaims.map((claim) => {
                                        const SourceIcon = getSourceIcon(claim.source)
                                        return (
                                            <TableRow
                                                key={claim.id}
                                                className={cn(
                                                    "hover:bg-accent/5 transition-colors",
                                                    !claim.enabled && "opacity-50"
                                                )}
                                            >
                                                <TableCell>
                                                    <div className="space-y-1">
                                                        <code className="text-sm font-mono bg-muted px-2 py-1 rounded">
                                                            {claim.name}
                                                        </code>
                                                        {claim.description && (
                                                            <p className="text-xs text-muted-foreground truncate max-w-[150px]">
                                                                {claim.description}
                                                            </p>
                                                        )}
                                                    </div>
                                                </TableCell>
                                                <TableCell>
                                                    <Badge variant="outline" className="text-xs bg-accent/10 text-accent border-accent/30">
                                                        <SourceIcon className="h-3 w-3 mr-1" />
                                                        {getSourceLabel(claim.source)}
                                                    </Badge>
                                                </TableCell>
                                                <TableCell>
                                                    <code className="text-xs font-mono text-muted-foreground bg-muted px-2 py-1 rounded truncate max-w-[200px] block">
                                                        {claim.value}
                                                    </code>
                                                </TableCell>
                                                <TableCell>
                                                    {claim.always_include ? (
                                                        <Badge variant="outline" className="text-xs bg-accent/10 text-accent border-accent/30">
                                                            Siempre
                                                        </Badge>
                                                    ) : (
                                                        <Badge variant="outline" className="text-xs">
                                                            Por Scope
                                                        </Badge>
                                                    )}
                                                </TableCell>
                                                <TableCell className="text-center">
                                                    <Switch
                                                        checked={claim.enabled}
                                                        onCheckedChange={() => handleToggleCustomClaim(claim.id)}
                                                    />
                                                </TableCell>
                                                <TableCell className="text-right">
                                                    <div className="flex items-center justify-end gap-1">
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            className="h-8 w-8 hover:-translate-y-0.5 transition-all duration-200"
                                                            onClick={() => { setEditingClaim(claim); setEditorOpen(true) }}
                                                        >
                                                            <Edit2 className="h-4 w-4" />
                                                        </Button>
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            className="h-8 w-8 text-destructive hover:text-destructive hover:-translate-y-0.5 transition-all duration-200"
                                                            onClick={() => { setSelectedClaim(claim); setDeleteDialogOpen(true) }}
                                                            disabled={claim.system}
                                                        >
                                                            <Trash2 className="h-4 w-4" />
                                                        </Button>
                                                    </div>
                                                </TableCell>
                                            </TableRow>
                                        )
                                    })}
                                </TableBody>
                            </Table>
                        </Card>
                    )}
                </TabsContent>

                {/* Scope Mappings Tab */}
                <TabsContent value="mappings" className="space-y-4">
                    <Card className="p-4">
                        <h3 className="font-medium flex items-center gap-2 mb-4">
                            <Tag className="h-4 w-4 text-accent" />
                            Mapeo de Scopes a Claims
                            <InfoTooltip content="Define qué claims se incluyen cuando una aplicación solicita un scope específico" />
                        </h3>

                        <div className="space-y-4">
                            {scopeMappings.map((mapping) => (
                                <div key={mapping.scope} className="p-4 rounded-lg border bg-muted">
                                    <div className="flex items-center justify-between mb-3">
                                        <Badge className="text-sm bg-accent/10 text-accent border-accent/30">
                                            {mapping.scope}
                                        </Badge>
                                        <span className="text-xs text-muted-foreground">
                                            {mapping.claims.length} claims
                                        </span>
                                    </div>
                                    <div className="flex flex-wrap gap-2">
                                        {mapping.claims.map((claim) => (
                                            <code key={claim} className="text-xs font-mono bg-card px-2 py-1 rounded">
                                                {claim}
                                            </code>
                                        ))}
                                    </div>
                                </div>
                            ))}
                        </div>

                        <InlineAlert variant="warning" className="mt-4">
                            <AlertCircle className="h-4 w-4" />
                            <div>
                                <p className="font-medium text-sm">Mapeos automáticos</p>
                                <p className="text-xs mt-0.5">
                                    Los claims estándar OIDC se mapean automáticamente a sus scopes correspondientes.
                                    Los claims personalizados marcados como "Incluir Siempre" se agregan a todos los tokens.
                                </p>
                            </div>
                        </InlineAlert>
                    </Card>
                </TabsContent>

                {/* Token Preview Tab */}
                <TabsContent value="preview" className="space-y-4">
                    <Card className="p-4">
                        <TokenPreview
                            standardClaims={standardClaims}
                            customClaims={customClaims}
                            settings={settings}
                        />
                    </Card>
                </TabsContent>

                {/* Settings Tab */}
                <TabsContent value="settings" className="space-y-4">
                    <Card className="p-4 space-y-6">
                        <div>
                            <h3 className="font-medium flex items-center gap-2">
                                <Settings2 className="h-4 w-4" />
                                Configuración de Claims
                            </h3>
                            <p className="text-xs text-muted-foreground mt-1">
                                Opciones globales para la emisión de claims en tokens
                            </p>
                        </div>

                        <div className="space-y-4">
                            {/* Include in Access Token */}
                            <div className="flex items-center justify-between p-4 rounded-lg border bg-muted">
                                <div className="space-y-1">
                                    <Label className="flex items-center">
                                        Incluir en Access Token
                                        <InfoTooltip content="Si está habilitado, los claims personalizados se incluyen tanto en el ID Token como en el Access Token. Si está deshabilitado, solo aparecen en el ID Token." />
                                    </Label>
                                    <p className="text-xs text-muted-foreground">
                                        Incluir claims personalizados en Access Token además de ID Token
                                    </p>
                                </div>
                                <Switch
                                    checked={settings.include_in_access_token}
                                    onCheckedChange={(v) => handleSettingsChange("include_in_access_token", v)}
                                />
                            </div>

                            {/* Use Namespaced Claims */}
                            <div className="flex items-center justify-between p-4 rounded-lg border bg-muted">
                                <div className="space-y-1">
                                    <Label className="flex items-center">
                                        Usar Claims con Namespace
                                        <InfoTooltip content="Los claims personalizados usarán un prefijo de URL como namespace (ej: https://tudominio.com/claims/roles) para evitar colisiones" />
                                    </Label>
                                    <p className="text-xs text-muted-foreground">
                                        Prefijar claims personalizados con URL de namespace
                                    </p>
                                </div>
                                <Switch
                                    checked={settings.use_namespaced_claims}
                                    onCheckedChange={(v) => handleSettingsChange("use_namespaced_claims", v)}
                                />
                            </div>

                            {/* Namespace Prefix */}
                            {settings.use_namespaced_claims && (
                                <div className="p-4 rounded-lg border bg-muted space-y-3">
                                    <Label className="flex items-center">
                                        Prefijo de Namespace
                                        <InfoTooltip content="URL base que se usará como prefijo para los claims personalizados" />
                                    </Label>
                                    <Input
                                        placeholder="https://auth.example.com/claims"
                                        value={settings.namespace_prefix || ""}
                                        onChange={(e) => handleSettingsChange("namespace_prefix", e.target.value)}
                                    />
                                    <p className="text-xs text-muted-foreground">
                                        Ejemplo: un claim "roles" se convertiría en "{settings.namespace_prefix || "https://..."}/roles"
                                    </p>
                                </div>
                            )}
                        </div>

                        {/* Security Notice */}
                        <InlineAlert variant="info">
                            <Lock className="h-5 w-5" />
                            <div>
                                <p className="font-medium text-sm">Consideraciones de Seguridad</p>
                                <ul className="mt-2 space-y-1 text-xs">
                                    <li>• No incluyas información sensible directamente en los tokens</li>
                                    <li>• Los tokens pueden ser decodificados por cualquiera que los tenga</li>
                                    <li>• Usa tiempos de expiración cortos para tokens con información sensible</li>
                                    <li>• Considera usar opaque tokens si necesitas máxima privacidad</li>
                                </ul>
                            </div>
                        </InlineAlert>
                    </Card>
                </TabsContent>
            </Tabs>

            {/* Claim Editor Dialog */}
            <ClaimEditorDialog
                claim={editingClaim}
                open={editorOpen}
                onClose={() => { setEditorOpen(false); setEditingClaim(null) }}
                onSave={editingClaim ? handleEditClaim : handleCreateClaim}
            />

            {/* Delete Confirmation Dialog */}
            <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
                <DialogContent className="sm:max-w-md">
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-3 text-destructive">
                            <div className="h-10 w-10 rounded-xl bg-destructive/10 flex items-center justify-center">
                                <Trash2 className="h-5 w-5" />
                            </div>
                            Eliminar Claim
                        </DialogTitle>
                        <DialogDescription>
                            ¿Estás seguro de que deseas eliminar el claim <strong>{selectedClaim?.name}</strong>?
                            Esta acción no se puede deshacer y los tokens futuros no incluirán este claim.
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>
                            Cancelar
                        </Button>
                        <Button
                            variant="danger"
                            onClick={handleDeleteClaim}
                            className="hover:-translate-y-0.5 hover:shadow-clay-card active:translate-y-0 transition-all duration-200"
                        >
                            Eliminar
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
