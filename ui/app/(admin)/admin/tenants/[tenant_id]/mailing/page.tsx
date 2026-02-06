"use client"

import { useState, useEffect } from "react"
import { useSearchParams } from "next/navigation"
import Link from "next/link"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
    Save, Mail, FileText, RotateCcw, Send, Server, Lock,
    CheckCircle2, AlertCircle, Eye, Code2, Sparkles, Shield,
    ChevronRight, Zap, ArrowLeft, RefreshCw, Activity
} from "lucide-react"
import { api } from "@/lib/api"
import { useToast } from "@/hooks/use-toast"
import { DEFAULT_TEMPLATES } from "@/lib/default-templates"
import type { Tenant, EmailTemplate } from "@/lib/types"

// Design System Components
import {
    Button,
    Input,
    Label,
    Switch,
    Textarea,
    Badge,
    Card,
    CardContent,
    CardHeader,
    CardTitle,
    CardDescription,
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    Tabs,
    TabsContent,
    TabsList,
    TabsTrigger,
    InlineAlert,
    Skeleton,
    cn,
} from "@/components/ds"

// ─── Template Types ───

const TEMPLATE_TYPES = [
    { id: "verify_email", label: "Verificación de Email", icon: Mail, description: "Confirmar dirección de correo", vars: ["{{.UserEmail}}", "{{.Link}}", "{{.TTL}}", "{{.Tenant}}"] },
    { id: "reset_password", label: "Restablecer Contraseña", icon: Lock, description: "Recuperación de cuenta", vars: ["{{.UserEmail}}", "{{.Link}}", "{{.TTL}}", "{{.Tenant}}"] },
    { id: "user_blocked", label: "Usuario Bloqueado", icon: Shield, description: "Notificación de bloqueo", vars: ["{{.UserEmail}}", "{{.Reason}}", "{{.Until}}", "{{.Tenant}}"] },
    { id: "user_unblocked", label: "Usuario Desbloqueado", icon: CheckCircle2, description: "Notificación de desbloqueo", vars: ["{{.UserEmail}}", "{{.Tenant}}"] },
]

const SMTP_TIPS = [
    {
        title: "Gmail & Google Workspace",
        content: <>Usa <strong>smtp.gmail.com</strong> puerto <strong>587</strong> con una contraseña de aplicación. Activa &quot;Verificación en 2 pasos&quot; primero.</>
    },
    {
        title: "Servidores SMTP Compatibles",
        content: <>Amazon SES, Mailgun, SendGrid, Office 365 y Gmail. Todos usan puerto <strong>587</strong> con STARTTLS.</>
    },
]

// ─── Stats Card Component ───

function StatCard({
    icon: Icon,
    label,
    value,
    variant = "default",
    isLoading = false,
}: {
    icon: React.ElementType
    label: string
    value: string | number
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

// ─── Main Component ───

export default function MailingPage() {
    const search = useSearchParams()
    const tenantId = search.get("id") as string
    const { toast } = useToast()
    const queryClient = useQueryClient()

    // ─── Data Fetching ───

    const { data: tenant } = useQuery({
        queryKey: ["tenant", tenantId],
        enabled: !!tenantId,
        queryFn: () => api.get<Tenant>(`/v2/admin/tenants/${tenantId}`),
    })

    const { data: settings, isLoading } = useQuery({
        queryKey: ["tenant-settings", tenantId, "mailing"],
        enabled: !!tenantId,
        queryFn: async () => {
            const token = (await import("@/lib/auth-store")).useAuthStore.getState().token
            const resp = await fetch(`${api.getBaseUrl()}/v2/admin/tenants/${tenantId}/settings`, {
                headers: { Authorization: token ? `Bearer ${token}` : "" },
            })
            const etag = resp.headers.get("ETag") || undefined
            const data = await resp.json()
            return { ...data, _etag: etag }
        },
    })

    // ─── Local State ───

    const [smtpData, setSmtpData] = useState<any>({})
    const [templatesData, setTemplatesData] = useState<Record<string, EmailTemplate>>({})
    const [activeTemplate, setActiveTemplate] = useState("verify_email")
    const [testEmailOpen, setTestEmailOpen] = useState(false)
    const [testEmailTo, setTestEmailTo] = useState("")
    const [copiedVar, setCopiedVar] = useState<string | null>(null)
    const [previewMode, setPreviewMode] = useState<"code" | "preview">("preview")
    const [activeTip, setActiveTip] = useState(0)

    // ─── Helper Functions ───

    const mapSmtpFromBackend = (smtp: any) => {
        if (!smtp) return {}
        return {
            host: smtp.host || "",
            port: smtp.port || 0,
            username: smtp.username || "",
            fromEmail: smtp.from_email || smtp.fromEmail || "",
            useTLS: smtp.use_tls ?? smtp.useTLS ?? false,
        }
    }

    const hasBackendSavedCredentials = Boolean(
        settings?.smtp?.host &&
        (settings.smtp.from_email || settings.smtp.fromEmail) &&
        (settings.smtp.password_enc || settings.smtp.passwordEnc || settings.smtp.username)
    )

    // ─── Effects ───

    useEffect(() => {
        if (settings) {
            setSmtpData(mapSmtpFromBackend(settings.smtp))
            const mergedTemplates = { ...settings.mailing?.templates }
            for (const key of Object.keys(DEFAULT_TEMPLATES)) {
                if (!mergedTemplates[key]?.body) {
                    mergedTemplates[key] = DEFAULT_TEMPLATES[key as keyof typeof DEFAULT_TEMPLATES]
                }
            }
            setTemplatesData(mergedTemplates)
        }
    }, [settings])

    useEffect(() => {
        const timer = setInterval(() => {
            setActiveTip(prev => (prev + 1) % SMTP_TIPS.length)
        }, 7000)
        return () => clearInterval(timer)
    }, [])

    // ─── Mutations ───

    const updateSettingsMutation = useMutation({
        mutationFn: (data: any) => {
            if (!settings?._etag) throw new Error("Missing ETag. Please refresh.")
            return api.put<any>(`/v2/admin/tenants/${tenantId}/settings`, data, settings._etag)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["tenant-settings", tenantId, "mailing"] })
            toast({ title: "✓ Guardado", description: "Configuración actualizada correctamente.", variant: "info" })
        },
        onError: (e: any) => toast({ title: "Error", description: e.message, variant: "destructive" }),
    })

    const sendTestEmailMutation = useMutation({
        mutationFn: () => api.post(`/v2/admin/tenants/${tenantId}/mailing/test`, { to: testEmailTo }),
        onSuccess: () => {
            setTestEmailOpen(false)
            setTestEmailTo("")
            toast({ title: "✓ Enviado", description: `Email de prueba enviado a ${testEmailTo}`, variant: "success" })
        },
        onError: (e: any) => toast({ title: "Error", description: e.message || "No se pudo enviar", variant: "destructive" }),
    })

    // ─── Handlers ───

    const mapSmtpToBackend = (smtp: any) => ({
        host: smtp.host || "",
        port: smtp.port || 0,
        username: smtp.username || "",
        from_email: smtp.fromEmail || "",
        use_tls: smtp.useTLS ?? false,
        ...(smtp.password ? { password: smtp.password } : {}),
    })

    const saveSettings = () => {
        const payload = {
            ...settings,
            smtp: mapSmtpToBackend(smtpData),
            mailing: { ...settings?.mailing, templates: templatesData },
        }
        delete payload._etag
        updateSettingsMutation.mutate(payload)
    }

    const handleResetDefault = (type: string) => {
        const def = DEFAULT_TEMPLATES[type as keyof typeof DEFAULT_TEMPLATES]
        if (!def) return
        setTemplatesData(prev => ({ ...prev, [type]: { subject: def.subject, body: def.body } }))
        toast({ title: "Restaurado", description: "Plantilla restaurada. Recuerda guardar.", variant: "info" })
    }

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text)
        setCopiedVar(text)
        setTimeout(() => setCopiedVar(null), 1500)
    }

    // ─── Computed ───

    const hasSavedCredentials = hasBackendSavedCredentials
    const mappedSavedSmtp = mapSmtpFromBackend(settings?.smtp)
    const isSmtpDirty = JSON.stringify(mappedSavedSmtp) !== JSON.stringify(smtpData)
    const isTemplatesDirty = JSON.stringify(settings?.mailing?.templates || {}) !== JSON.stringify(templatesData) && Object.keys(templatesData).length > 0
    const isDirty = isSmtpDirty || isTemplatesDirty
    const currentTemplate = TEMPLATE_TYPES.find(t => t.id === activeTemplate)

    const renderPreview = (type: string) => {
        const tpl = templatesData[type] || DEFAULT_TEMPLATES[type as keyof typeof DEFAULT_TEMPLATES]
        if (!tpl?.body) return (
            <div className="flex items-center justify-center h-full text-muted-foreground">
                <div className="text-center">
                    <FileText className="h-12 w-12 mx-auto mb-3 opacity-30" />
                    <p className="text-sm">Sin contenido</p>
                </div>
            </div>
        )
        let html = tpl.body
        const replacements: Record<string, string> = {
            "{{.UserEmail}}": "usuario@ejemplo.com",
            "{{.Link}}": "#",
            "{{.TTL}}": "24 horas",
            "{{.Tenant}}": tenant?.name || "Mi Organización",
            "{{.Reason}}": "Violación de términos",
            "{{.Until}}": "Mañana a las 10:00 AM"
        }
        Object.entries(replacements).forEach(([k, v]) => { html = html.replaceAll(k, v) })
        return <iframe title="preview" srcDoc={html} className="w-full h-full bg-white border-0" />
    }

    // ─── Render ───

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
                            <h1 className="text-2xl font-bold tracking-tight">Email & Notificaciones</h1>
                            <p className="text-sm text-muted-foreground">
                                {tenant?.name} — Configura el envío de correos
                            </p>
                        </div>
                    </div>
                </div>
                <div className="flex items-center gap-3">
                    {isDirty && (
                        <Button onClick={saveSettings} disabled={updateSettingsMutation.isPending} className="shadow-clay-button hover:shadow-clay-card transition-shadow">
                            {updateSettingsMutation.isPending ? (
                                <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                            ) : (
                                <Save className="h-4 w-4 mr-2" />
                            )}
                            Guardar Cambios
                        </Button>
                    )}
                </div>
            </div>

            {/* Info Banner */}
            <InlineAlert variant="info">
                <div>
                    <p className="font-semibold">Configuración de Email</p>
                    <p className="text-sm opacity-90">
                        Configura tu servidor SMTP para enviar correos de verificación, recuperación de contraseña y notificaciones a los usuarios de tu tenant.
                    </p>
                </div>
            </InlineAlert>

            {/* Stats Cards */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <StatCard
                    icon={Server}
                    label="Estado SMTP"
                    value={hasSavedCredentials ? "Configurado" : "Sin configurar"}
                    variant={hasSavedCredentials ? "success" : "warning"}
                    isLoading={isLoading}
                />
                <StatCard
                    icon={FileText}
                    label="Plantillas"
                    value={TEMPLATE_TYPES.length}
                    variant="info"
                    isLoading={isLoading}
                />
                <StatCard
                    icon={Activity}
                    label="Conexión TLS"
                    value={smtpData.useTLS ? "Activa" : "Inactiva"}
                    variant={smtpData.useTLS ? "success" : "default"}
                    isLoading={isLoading}
                />
                <StatCard
                    icon={Mail}
                    label="Remitente"
                    value={smtpData.fromEmail ? "Configurado" : "—"}
                    variant={smtpData.fromEmail ? "accent" : "default"}
                    isLoading={isLoading}
                />
            </div>

            {/* Tabs */}
            <Tabs defaultValue="smtp" className="space-y-6">
                <TabsList>
                    <TabsTrigger value="smtp" className="gap-2">
                        <Server className="h-4 w-4" />
                        Configuración SMTP
                    </TabsTrigger>
                    <TabsTrigger value="templates" className="gap-2">
                        <FileText className="h-4 w-4" />
                        Plantillas
                    </TabsTrigger>
                </TabsList>

                {/* SMTP Tab */}
                <TabsContent value="smtp" className="space-y-6 mt-0">
                    <div className="grid lg:grid-cols-3 gap-6">
                        {/* Main Form */}
                        <div className="lg:col-span-2">
                            <Card>
                                <CardHeader className={cn(
                                    "border-b",
                                    hasSavedCredentials && !isSmtpDirty
                                        ? "bg-success/5 border-success/20"
                                        : ""
                                )}>
                                    <div className="flex items-center justify-between">
                                        <div className="flex items-center gap-3">
                                            {hasSavedCredentials && !isSmtpDirty ? (
                                                <>
                                                    <div className="h-8 w-8 rounded-full bg-success/20 flex items-center justify-center">
                                                        <CheckCircle2 className="h-4 w-4 text-success" />
                                                    </div>
                                                    <div>
                                                        <CardTitle className="text-success">Servidor configurado</CardTitle>
                                                        <CardDescription className="text-success/80">{smtpData.host}:{smtpData.port}</CardDescription>
                                                    </div>
                                                </>
                                            ) : (
                                                <>
                                                    <div className="h-8 w-8 rounded-full bg-muted flex items-center justify-center">
                                                        <Server className="h-4 w-4 text-muted-foreground" />
                                                    </div>
                                                    <div>
                                                        <CardTitle>Servidor SMTP</CardTitle>
                                                        <CardDescription>Configura las credenciales de tu proveedor</CardDescription>
                                                    </div>
                                                </>
                                            )}
                                        </div>
                                        {isSmtpDirty && (
                                            <Badge variant="warning">Sin guardar</Badge>
                                        )}
                                    </div>
                                </CardHeader>
                                <CardContent className="p-6 space-y-6">
                                    {isLoading ? (
                                        <div className="space-y-6">
                                            <div className="grid sm:grid-cols-2 gap-5">
                                                <div className="space-y-2">
                                                    <Skeleton className="h-4 w-24" />
                                                    <Skeleton className="h-11 w-full" />
                                                </div>
                                                <div className="space-y-2">
                                                    <Skeleton className="h-4 w-16" />
                                                    <Skeleton className="h-11 w-full" />
                                                </div>
                                            </div>
                                            <div className="grid sm:grid-cols-2 gap-5">
                                                <div className="space-y-2">
                                                    <Skeleton className="h-4 w-20" />
                                                    <Skeleton className="h-11 w-full" />
                                                </div>
                                                <div className="space-y-2">
                                                    <Skeleton className="h-4 w-24" />
                                                    <Skeleton className="h-11 w-full" />
                                                </div>
                                            </div>
                                            <div className="space-y-2">
                                                <Skeleton className="h-4 w-32" />
                                                <Skeleton className="h-11 w-full" />
                                            </div>
                                            <Skeleton className="h-16 w-full rounded-xl" />
                                        </div>
                                    ) : (
                                        <>
                                            <div className="grid sm:grid-cols-2 gap-5">
                                                <div className="space-y-2">
                                                    <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Servidor SMTP</Label>
                                                    <Input
                                                        placeholder="smtp.gmail.com"
                                                        value={smtpData.host || ""}
                                                        onChange={e => setSmtpData({ ...smtpData, host: e.target.value })}
                                                    />
                                                </div>
                                                <div className="space-y-2">
                                                    <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Puerto</Label>
                                                    <Input
                                                        type="number"
                                                        placeholder="587"
                                                        value={smtpData.port || ""}
                                                        onChange={e => setSmtpData({ ...smtpData, port: parseInt(e.target.value) || 0 })}
                                                    />
                                                </div>
                                            </div>

                                            <div className="grid sm:grid-cols-2 gap-5">
                                                <div className="space-y-2">
                                                    <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Usuario</Label>
                                                    <Input
                                                        placeholder="tu@email.com"
                                                        value={smtpData.username || ""}
                                                        onChange={e => setSmtpData({ ...smtpData, username: e.target.value })}
                                                    />
                                                </div>
                                                <div className="space-y-2">
                                                    <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                                                        Contraseña
                                                        {hasSavedCredentials && <span className="ml-1 text-muted-foreground/60 normal-case">(opcional)</span>}
                                                    </Label>
                                                    <Input
                                                        type="password"
                                                        placeholder={hasSavedCredentials ? "••••••••" : "Tu contraseña"}
                                                        value={smtpData.password || ""}
                                                        onChange={e => setSmtpData({ ...smtpData, password: e.target.value })}
                                                    />
                                                </div>
                                            </div>

                                            <div className="space-y-2">
                                                <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Remitente (From)</Label>
                                                <Input
                                                    placeholder="noreply@tudominio.com"
                                                    value={smtpData.fromEmail || ""}
                                                    onChange={e => setSmtpData({ ...smtpData, fromEmail: e.target.value })}
                                                />
                                            </div>

                                            <div className="flex items-center justify-between p-4 rounded-xl bg-muted/50 border">
                                                <div className="flex items-center gap-3">
                                                    <Shield className="h-5 w-5 text-muted-foreground" />
                                                    <div>
                                                        <p className="text-sm font-medium text-foreground">Conexión segura TLS/SSL</p>
                                                        <p className="text-xs text-muted-foreground">Encripta la comunicación con el servidor</p>
                                                    </div>
                                                </div>
                                                <Switch checked={smtpData.useTLS || false} onCheckedChange={c => setSmtpData({ ...smtpData, useTLS: c })} />
                                            </div>
                                        </>
                                    )}
                                </CardContent>
                            </Card>
                        </div>

                        {/* Test Panel */}
                        <div className="space-y-4">
                            <Card>
                                <CardContent className="p-6 text-center">
                                    {isLoading ? (
                                        <div className="space-y-4">
                                            <Skeleton className="h-16 w-16 mx-auto rounded-2xl" />
                                            <Skeleton className="h-5 w-32 mx-auto" />
                                            <Skeleton className="h-4 w-48 mx-auto" />
                                            <Skeleton className="h-11 w-full" />
                                        </div>
                                    ) : (
                                        <>
                                            <div className={cn(
                                                "h-16 w-16 mx-auto rounded-2xl flex items-center justify-center mb-4",
                                                hasSavedCredentials && !isSmtpDirty
                                                    ? "bg-gradient-to-br from-success to-success/80 shadow-lg shadow-success/20"
                                                    : "bg-muted"
                                            )}>
                                                {hasSavedCredentials && !isSmtpDirty ? (
                                                    <Zap className="h-7 w-7 text-white" />
                                                ) : (
                                                    <AlertCircle className="h-7 w-7 text-muted-foreground" />
                                                )}
                                            </div>

                                            <h3 className="font-semibold text-foreground mb-1">
                                                {hasSavedCredentials && !isSmtpDirty ? "Listo para enviar" : "Configuración requerida"}
                                            </h3>
                                            <p className="text-sm text-muted-foreground mb-5">
                                                {hasSavedCredentials && !isSmtpDirty
                                                    ? "Puedes probar el envío de emails"
                                                    : "Guarda las credenciales primero"}
                                            </p>

                                            <Button
                                                className="w-full"
                                                variant={hasSavedCredentials && !isSmtpDirty ? "default" : "outline"}
                                                disabled={!hasSavedCredentials || isSmtpDirty}
                                                onClick={() => setTestEmailOpen(true)}
                                            >
                                                <Send className="h-4 w-4 mr-2" />
                                                Enviar Prueba
                                            </Button>
                                        </>
                                    )}
                                </CardContent>
                            </Card>

                            {/* Tips Card */}
                            <Card className="bg-gradient-to-br from-accent/10 to-accent/5 border-accent/20">
                                <CardContent className="p-4">
                                    <div className="flex items-start gap-3">
                                        <Sparkles className="h-5 w-5 text-accent mt-0.5 flex-shrink-0" />
                                        <div className="flex-1 min-w-0">
                                            <p className="text-sm font-medium text-accent mb-1">Tip: {SMTP_TIPS[activeTip].title}</p>
                                            <p className="text-xs text-accent/80 leading-relaxed">
                                                {SMTP_TIPS[activeTip].content}
                                            </p>
                                        </div>
                                    </div>
                                    <div className="flex justify-center gap-1.5 mt-3">
                                        {SMTP_TIPS.map((_, idx) => (
                                            <button
                                                key={idx}
                                                onClick={() => setActiveTip(idx)}
                                                className={cn(
                                                    "h-1.5 rounded-full transition-all duration-300",
                                                    activeTip === idx
                                                        ? "w-4 bg-accent"
                                                        : "w-1.5 bg-accent/40 hover:bg-accent/60"
                                                )}
                                            />
                                        ))}
                                    </div>
                                </CardContent>
                            </Card>
                        </div>
                    </div>

                    {/* Gmail Warning */}
                    <InlineAlert variant="warning" title="Nota sobre Gmail">
                        Gmail sobrescribe el remitente &quot;From&quot; con la cuenta autenticada.
                        Para usar un remitente personalizado, usa SES, Mailgun o SendGrid con tu dominio verificado.
                    </InlineAlert>
                </TabsContent>

                {/* Templates Tab */}
                <TabsContent value="templates" className="mt-0">
                    <div className="grid lg:grid-cols-12 gap-6 min-h-[500px]">
                        {/* Template List */}
                        <Card className="lg:col-span-3">
                            <CardContent className="p-4 space-y-2">
                                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide px-2 mb-3">Plantillas</p>
                                {isLoading ? (
                                    <div className="space-y-2">
                                        {[1, 2, 3, 4].map((i) => (
                                            <Skeleton key={i} className="h-16 w-full rounded-xl" />
                                        ))}
                                    </div>
                                ) : (
                                    TEMPLATE_TYPES.map(t => {
                                        const Icon = t.icon
                                        return (
                                            <button
                                                key={t.id}
                                                onClick={() => setActiveTemplate(t.id)}
                                                className={cn(
                                                    "w-full flex items-center gap-3 p-3 rounded-xl text-left transition-all",
                                                    activeTemplate === t.id
                                                        ? "bg-accent text-accent-foreground shadow-lg"
                                                        : "hover:bg-muted text-foreground"
                                                )}
                                            >
                                                <Icon className="h-5 w-5 flex-shrink-0" />
                                                <div className="flex-1 min-w-0">
                                                    <p className="text-sm font-medium truncate">{t.label}</p>
                                                    <p className={cn(
                                                        "text-xs truncate",
                                                        activeTemplate === t.id ? "text-accent-foreground/70" : "text-muted-foreground"
                                                    )}>{t.description}</p>
                                                </div>
                                                <ChevronRight className={cn(
                                                    "h-4 w-4 flex-shrink-0 transition-transform",
                                                    activeTemplate === t.id ? "rotate-90" : ""
                                                )} />
                                            </button>
                                        )
                                    })
                                )}
                            </CardContent>
                        </Card>

                        {/* Editor */}
                        <Card className="lg:col-span-5 flex flex-col overflow-hidden">
                            <CardHeader className="border-b flex-row items-center justify-between space-y-0 py-4">
                                <div>
                                    <CardTitle className="text-base">{currentTemplate?.label}</CardTitle>
                                    <CardDescription>Edita el asunto y contenido HTML</CardDescription>
                                </div>
                                <div className="flex gap-2">
                                    <Button size="sm" variant="ghost" onClick={() => handleResetDefault(activeTemplate)}>
                                        <RotateCcw className="h-4 w-4 mr-1" /> Reset
                                    </Button>
                                    <Button size="sm" onClick={saveSettings} disabled={!isTemplatesDirty || updateSettingsMutation.isPending}>
                                        <Save className="h-4 w-4 mr-1" /> Guardar
                                    </Button>
                                </div>
                            </CardHeader>

                            {isLoading ? (
                                <CardContent className="flex-1 p-6 space-y-4">
                                    <div className="space-y-2">
                                        <Skeleton className="h-4 w-16" />
                                        <Skeleton className="h-8 w-full" />
                                    </div>
                                    <Skeleton className="h-64 w-full" />
                                </CardContent>
                            ) : (
                                <>
                                    {/* Subject */}
                                    <div className="px-5 py-4 border-b">
                                        <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2 block">Asunto</Label>
                                        <Input
                                            value={templatesData[activeTemplate]?.subject || ""}
                                            onChange={e => {
                                                setTemplatesData(prev => ({
                                                    ...prev,
                                                    [activeTemplate]: { ...prev[activeTemplate], subject: e.target.value }
                                                }))
                                            }}
                                            className="border-0 px-0 text-lg font-medium bg-transparent focus-visible:ring-0 h-auto py-1"
                                            placeholder="Asunto del correo..."
                                        />
                                    </div>

                                    {/* Body */}
                                    <div className="flex-1 flex flex-col min-h-0">
                                        <Textarea
                                            value={templatesData[activeTemplate]?.body || ""}
                                            onChange={e => {
                                                setTemplatesData(prev => ({
                                                    ...prev,
                                                    [activeTemplate]: { ...prev[activeTemplate], body: e.target.value }
                                                }))
                                            }}
                                            placeholder="Contenido HTML del email..."
                                            className="flex-1 font-mono text-sm leading-relaxed p-5 resize-none border-0 rounded-none focus-visible:ring-0 bg-muted/50"
                                            spellCheck={false}
                                        />
                                    </div>

                                    {/* Variables */}
                                    <div className="p-4 border-t bg-muted/30">
                                        <p className="text-xs text-muted-foreground mb-2">Variables disponibles · Click para copiar</p>
                                        <div className="flex flex-wrap gap-1.5">
                                            {currentTemplate?.vars.map(v => (
                                                <button
                                                    key={v}
                                                    onClick={() => copyToClipboard(v)}
                                                    className={cn(
                                                        "px-2 py-1 rounded-md text-xs font-mono transition-all",
                                                        copiedVar === v
                                                            ? "bg-success/10 text-success"
                                                            : "bg-background border text-foreground hover:bg-accent/10 hover:text-accent"
                                                    )}
                                                >
                                                    {copiedVar === v ? "✓ Copiado" : v}
                                                </button>
                                            ))}
                                        </div>
                                    </div>
                                </>
                            )}
                        </Card>

                        {/* Preview */}
                        <Card className="lg:col-span-4 flex flex-col overflow-hidden">
                            <CardHeader className="border-b flex-row items-center justify-between space-y-0 py-4">
                                <div className="flex items-center gap-2">
                                    <Eye className="h-4 w-4 text-muted-foreground" />
                                    <CardTitle className="text-base">Vista Previa</CardTitle>
                                </div>
                                <div className="flex bg-muted rounded-lg p-0.5">
                                    <button
                                        onClick={() => setPreviewMode("preview")}
                                        className={cn("px-3 py-1 text-xs rounded-md transition-all", previewMode === "preview" ? "bg-background shadow-sm" : "text-muted-foreground")}
                                    >
                                        Preview
                                    </button>
                                    <button
                                        onClick={() => setPreviewMode("code")}
                                        className={cn("px-3 py-1 text-xs rounded-md transition-all", previewMode === "code" ? "bg-background shadow-sm" : "text-muted-foreground")}
                                    >
                                        <Code2 className="h-3 w-3 inline mr-1" />HTML
                                    </button>
                                </div>
                            </CardHeader>
                            <CardContent className="flex-1 p-0 bg-muted/30">
                                {isLoading ? (
                                    <div className="p-6">
                                        <Skeleton className="h-full min-h-[300px] w-full" />
                                    </div>
                                ) : previewMode === "preview" ? (
                                    renderPreview(activeTemplate)
                                ) : (
                                    <pre className="p-4 text-xs font-mono text-muted-foreground overflow-auto h-full">
                                        {templatesData[activeTemplate]?.body || "Sin contenido"}
                                    </pre>
                                )}
                            </CardContent>
                        </Card>
                    </div>
                </TabsContent>
            </Tabs>

            {/* Test Email Dialog */}
            <Dialog open={testEmailOpen} onOpenChange={setTestEmailOpen}>
                <DialogContent className="sm:max-w-md">
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2">
                            <Send className="h-5 w-5 text-accent" />
                            Enviar Email de Prueba
                        </DialogTitle>
                        <DialogDescription>
                            Se enviará un correo de prueba usando la configuración guardada.
                        </DialogDescription>
                    </DialogHeader>
                    <div className="py-4">
                        <Label className="text-sm mb-2 block">Enviar a:</Label>
                        <Input
                            placeholder="tu@email.com"
                            type="email"
                            value={testEmailTo}
                            onChange={e => setTestEmailTo(e.target.value)}
                            autoFocus
                        />
                    </div>
                    <DialogFooter className="gap-2">
                        <Button variant="ghost" onClick={() => setTestEmailOpen(false)}>Cancelar</Button>
                        <Button
                            onClick={() => sendTestEmailMutation.mutate()}
                            disabled={sendTestEmailMutation.isPending || !testEmailTo}
                        >
                            {sendTestEmailMutation.isPending ? (
                                <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                            ) : (
                                <Send className="h-4 w-4 mr-2" />
                            )}
                            Enviar Prueba
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
