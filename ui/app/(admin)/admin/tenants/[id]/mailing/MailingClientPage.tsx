"use client"

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useParams, useSearchParams } from "next/navigation"
import {
    Save, Mail, FileText, RotateCcw, Send, Server, Lock,
    CheckCircle2, AlertCircle, Eye, Code2, Sparkles, Shield,
    ChevronRight, Zap
} from "lucide-react"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { useToast } from "@/hooks/use-toast"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import type { Tenant, EmailTemplate } from "@/lib/types"
import { useState, useEffect } from "react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Badge } from "@/components/ui/badge"
import { DEFAULT_TEMPLATES } from "@/lib/default-templates"
import { cn } from "@/lib/utils"

// --- Template Types ---
const TEMPLATE_TYPES = [
    { id: "verify_email", label: "Verificación de Email", icon: Mail, description: "Confirmar dirección de correo", vars: ["{{.UserEmail}}", "{{.Link}}", "{{.TTL}}", "{{.Tenant}}"] },
    { id: "reset_password", label: "Restablecer Contraseña", icon: Lock, description: "Recuperación de cuenta", vars: ["{{.UserEmail}}", "{{.Link}}", "{{.TTL}}", "{{.Tenant}}"] },
    { id: "user_blocked", label: "Usuario Bloqueado", icon: Shield, description: "Notificación de bloqueo", vars: ["{{.UserEmail}}", "{{.Reason}}", "{{.Until}}", "{{.Tenant}}"] },
    { id: "user_unblocked", label: "Usuario Desbloqueado", icon: CheckCircle2, description: "Notificación de desbloqueo", vars: ["{{.UserEmail}}", "{{.Tenant}}"] },
]

export default function MailingClientPage() {
    const params = useParams()
    const search = useSearchParams()
    const { toast } = useToast()
    const queryClient = useQueryClient()
    const tenantId = (params.id as string) || (search.get("id") as string)

    // Data Fetching
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

    // Local State
    const [smtpData, setSmtpData] = useState<any>({})
    const [templatesData, setTemplatesData] = useState<Record<string, EmailTemplate>>({})
    const [activeTemplate, setActiveTemplate] = useState("verify_email")
    const [testEmailOpen, setTestEmailOpen] = useState(false)
    const [testEmailTo, setTestEmailTo] = useState("")
    const [copiedVar, setCopiedVar] = useState<string | null>(null)
    const [previewMode, setPreviewMode] = useState<"code" | "preview">("preview")
    const [activeTip, setActiveTip] = useState(0)

    // Map snake_case from backend to camelCase for frontend
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

    // Rotate tips every 5 seconds
    const SMTP_TIPS = [
        {
            title: "Gmail & Google Workspace",
            content: <>Usa <strong>smtp.gmail.com</strong> puerto <strong>587</strong> con una contraseña de aplicación. Activa "Verificación en 2 pasos" primero.</>
        },
        {
            title: "Servidores SMTP Compatibles",
            content: <>Amazon SES, Mailgun, SendGrid, Office 365 y Gmail. Todos usan puerto <strong>587</strong> con STARTTLS.</>
        },
    ]

    useEffect(() => {
        const timer = setInterval(() => {
            setActiveTip(prev => (prev + 1) % SMTP_TIPS.length)
        }, 7000)
        return () => clearInterval(timer)
    }, [SMTP_TIPS.length])

    // Mutations
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

    // Map frontend camelCase to backend snake_case
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

    // Computed
    const hasSavedCredentials = hasBackendSavedCredentials
    const mappedSavedSmtp = mapSmtpFromBackend(settings?.smtp)
    const isSmtpDirty = JSON.stringify(mappedSavedSmtp) !== JSON.stringify(smtpData)
    const isTemplatesDirty = JSON.stringify(settings?.mailing?.templates || {}) !== JSON.stringify(templatesData) && Object.keys(templatesData).length > 0
    const isDirty = isSmtpDirty || isTemplatesDirty
    const currentTemplate = TEMPLATE_TYPES.find(t => t.id === activeTemplate)

    const renderPreview = (type: string) => {
        const tpl = templatesData[type] || DEFAULT_TEMPLATES[type as keyof typeof DEFAULT_TEMPLATES]
        if (!tpl?.body) return (
            <div className="flex items-center justify-center h-full text-zinc-400">
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

    if (isLoading) {
        return (
            <div className="flex items-center justify-center h-[60vh]">
                <div className="text-center">
                    <div className="h-10 w-10 mx-auto mb-4 rounded-full border-2 border-zinc-200 border-t-zinc-800 animate-spin" />
                    <p className="text-sm text-zinc-500">Cargando configuración...</p>
                </div>
            </div>
        )
    }

    return (
        <div className="h-auto  bg-gradient-to-b from-zinc-50 to-white dark:from-zinc-950 dark:to-zinc-900">
            {/* Header */}
            <div className="border-b bg-white/80 dark:bg-zinc-900/80 backdrop-blur-sm  top-0 z-10">
                <div className="max-w-7xl mx-auto px-6 py-5">
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-4">
                            <div className="h-10 w-10 rounded-xl bg-gradient-to-br from-violet-500 to-purple-600 flex items-center justify-center shadow-lg shadow-violet-500/20">
                                <Mail className="h-5 w-5 text-white" />
                            </div>
                            <div>
                                <h1 className="text-xl font-semibold text-zinc-900 dark:text-white">Email & Notificaciones</h1>
                                <p className="text-sm text-zinc-500">Configura el envío de correos para {tenant?.name || "tu organización"}</p>
                            </div>
                        </div>
                        {isDirty && (
                            <Button onClick={saveSettings} disabled={updateSettingsMutation.isPending} className="gap-2 bg-zinc-900 hover:bg-zinc-800 text-white shadow-lg">
                                {updateSettingsMutation.isPending ? (
                                    <div className="h-4 w-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                                ) : (
                                    <Save className="h-4 w-4" />
                                )}
                                Guardar Cambios
                            </Button>
                        )}
                    </div>
                </div>
            </div>

            <div className="max-w-7xl mx-auto px-6 py-8">
                <Tabs defaultValue="smtp" className="space-y-8">
                    <TabsList className="bg-zinc-100 dark:bg-zinc-800/50 p-1 rounded-xl">
                        <TabsTrigger value="smtp" className="gap-2 rounded-lg data-[state=active]:bg-white data-[state=active]:shadow-sm px-5 py-2.5">
                            <Server className="h-4 w-4" />
                            <span>Configuración SMTP</span>
                        </TabsTrigger>
                        <TabsTrigger value="templates" className="gap-2 rounded-lg data-[state=active]:bg-white data-[state=active]:shadow-sm px-5 py-2.5">
                            <FileText className="h-4 w-4" />
                            <span>Plantillas</span>
                        </TabsTrigger>
                    </TabsList>

                    {/* SMTP Tab */}
                    <TabsContent value="smtp" className="space-y-6 mt-0">
                        <div className="grid lg:grid-cols-3 gap-6">
                            {/* Main Form */}
                            <div className="lg:col-span-2">
                                <div className="bg-white dark:bg-zinc-900 rounded-2xl border border-zinc-200 dark:border-zinc-800 shadow-sm overflow-hidden">
                                    {/* Status Header */}
                                    <div className={cn(
                                        "px-6 py-4 border-b flex items-center justify-between",
                                        hasSavedCredentials && !isSmtpDirty
                                            ? "bg-emerald-50 dark:bg-emerald-950/20 border-emerald-100 dark:border-emerald-900/30"
                                            : "bg-zinc-50 dark:bg-zinc-800/50"
                                    )}>
                                        <div className="flex items-center gap-3">
                                            {hasSavedCredentials && !isSmtpDirty ? (
                                                <>
                                                    <div className="h-8 w-8 rounded-full bg-emerald-100 dark:bg-emerald-900/50 flex items-center justify-center">
                                                        <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                                                    </div>
                                                    <div>
                                                        <p className="font-medium text-emerald-900 dark:text-emerald-100">Servidor configurado</p>
                                                        <p className="text-xs text-emerald-600 dark:text-emerald-400">{smtpData.host}:{smtpData.port}</p>
                                                    </div>
                                                </>
                                            ) : (
                                                <>
                                                    <div className="h-8 w-8 rounded-full bg-zinc-200 dark:bg-zinc-700 flex items-center justify-center">
                                                        <Server className="h-4 w-4 text-zinc-500" />
                                                    </div>
                                                    <div>
                                                        <p className="font-medium text-zinc-700 dark:text-zinc-200">Servidor SMTP</p>
                                                        <p className="text-xs text-zinc-500">Configura las credenciales de tu proveedor</p>
                                                    </div>
                                                </>
                                            )}
                                        </div>
                                        {isSmtpDirty && (
                                            <Badge className="bg-amber-100 text-amber-700 border-amber-200 dark:bg-amber-900/30 dark:text-amber-400 dark:border-amber-800">
                                                Sin guardar
                                            </Badge>
                                        )}
                                    </div>

                                    {/* Form Fields */}
                                    <div className="p-6 space-y-6">
                                        <div className="grid sm:grid-cols-2 gap-5">
                                            <div className="space-y-2">
                                                <Label className="text-xs font-medium text-zinc-600 dark:text-zinc-400 uppercase tracking-wide">Servidor SMTP</Label>
                                                <Input
                                                    placeholder="smtp.gmail.com"
                                                    value={smtpData.host || ""}
                                                    onChange={e => setSmtpData({ ...smtpData, host: e.target.value })}
                                                    className="h-11 bg-zinc-50 dark:bg-zinc-800/50 border-zinc-200 dark:border-zinc-700 focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500"
                                                />
                                            </div>
                                            <div className="space-y-2">
                                                <Label className="text-xs font-medium text-zinc-600 dark:text-zinc-400 uppercase tracking-wide">Puerto</Label>
                                                <Input
                                                    type="number"
                                                    placeholder="587"
                                                    value={smtpData.port || ""}
                                                    onChange={e => setSmtpData({ ...smtpData, port: parseInt(e.target.value) || 0 })}
                                                    className="h-11 bg-zinc-50 dark:bg-zinc-800/50 border-zinc-200 dark:border-zinc-700 focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500"
                                                />
                                            </div>
                                        </div>

                                        <div className="grid sm:grid-cols-2 gap-5">
                                            <div className="space-y-2">
                                                <Label className="text-xs font-medium text-zinc-600 dark:text-zinc-400 uppercase tracking-wide">Usuario</Label>
                                                <Input
                                                    placeholder="tu@email.com"
                                                    value={smtpData.username || ""}
                                                    onChange={e => setSmtpData({ ...smtpData, username: e.target.value })}
                                                    className="h-11 bg-zinc-50 dark:bg-zinc-800/50 border-zinc-200 dark:border-zinc-700 focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500"
                                                />
                                            </div>
                                            <div className="space-y-2">
                                                <Label className="text-xs font-medium text-zinc-600 dark:text-zinc-400 uppercase tracking-wide">
                                                    Contraseña
                                                    {hasSavedCredentials && <span className="ml-1 text-zinc-400 normal-case">(opcional)</span>}
                                                </Label>
                                                <Input
                                                    type="password"
                                                    placeholder={hasSavedCredentials ? "••••••••" : "Tu contraseña"}
                                                    value={smtpData.password || ""}
                                                    onChange={e => setSmtpData({ ...smtpData, password: e.target.value })}
                                                    className="h-11 bg-zinc-50 dark:bg-zinc-800/50 border-zinc-200 dark:border-zinc-700 focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500"
                                                />
                                            </div>
                                        </div>

                                        <div className="space-y-2">
                                            <Label className="text-xs font-medium text-zinc-600 dark:text-zinc-400 uppercase tracking-wide">Remitente (From)</Label>
                                            <Input
                                                placeholder="noreply@tudominio.com"
                                                value={smtpData.fromEmail || ""}
                                                onChange={e => setSmtpData({ ...smtpData, fromEmail: e.target.value })}
                                                className="h-11 bg-zinc-50 dark:bg-zinc-800/50 border-zinc-200 dark:border-zinc-700 focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500"
                                            />
                                        </div>

                                        <div className="flex items-center justify-between p-4 rounded-xl bg-zinc-50 dark:bg-zinc-800/30 border border-zinc-100 dark:border-zinc-700/50">
                                            <div className="flex items-center gap-3">
                                                <Shield className="h-5 w-5 text-zinc-400" />
                                                <div>
                                                    <p className="text-sm font-medium text-zinc-700 dark:text-zinc-200">Conexión segura TLS/SSL</p>
                                                    <p className="text-xs text-zinc-500">Encripta la comunicación con el servidor</p>
                                                </div>
                                            </div>
                                            <Switch checked={smtpData.useTLS || false} onCheckedChange={c => setSmtpData({ ...smtpData, useTLS: c })} />
                                        </div>
                                    </div>
                                </div>
                            </div>

                            {/* Test Panel */}
                            <div className="space-y-4">
                                <div className="bg-white dark:bg-zinc-900 rounded-2xl border border-zinc-200 dark:border-zinc-800 shadow-sm overflow-hidden">
                                    <div className="p-6 text-center">
                                        <div className={cn(
                                            "h-16 w-16 mx-auto rounded-2xl flex items-center justify-center mb-4",
                                            hasSavedCredentials && !isSmtpDirty
                                                ? "bg-gradient-to-br from-emerald-400 to-teal-500 shadow-lg shadow-emerald-500/20"
                                                : "bg-zinc-100 dark:bg-zinc-800"
                                        )}>
                                            {hasSavedCredentials && !isSmtpDirty ? (
                                                <Zap className="h-7 w-7 text-white" />
                                            ) : (
                                                <AlertCircle className="h-7 w-7 text-zinc-400" />
                                            )}
                                        </div>

                                        <h3 className="font-semibold text-zinc-900 dark:text-white mb-1">
                                            {hasSavedCredentials && !isSmtpDirty ? "Listo para enviar" : "Configuración requerida"}
                                        </h3>
                                        <p className="text-sm text-zinc-500 mb-5">
                                            {hasSavedCredentials && !isSmtpDirty
                                                ? "Puedes probar el envío de emails"
                                                : "Guarda las credenciales primero"}
                                        </p>

                                        <Button
                                            className="w-full h-11 gap-2"
                                            variant={hasSavedCredentials && !isSmtpDirty ? "default" : "outline"}
                                            disabled={!hasSavedCredentials || isSmtpDirty}
                                            onClick={() => setTestEmailOpen(true)}
                                        >
                                            <Send className="h-4 w-4" />
                                            Enviar Prueba
                                        </Button>
                                    </div>
                                </div>

                                {/* Rotating Tips */}
                                <div className="bg-gradient-to-br from-violet-50 to-purple-50 dark:from-violet-950/30 dark:to-purple-950/30 rounded-2xl border border-violet-100 dark:border-violet-900/30 p-4">
                                    <div className="flex items-start gap-3">
                                        <Sparkles className="h-5 w-5 text-violet-500 mt-0.5 flex-shrink-0" />
                                        <div className="flex-1 min-w-0">
                                            <p className="text-sm font-medium text-violet-900 dark:text-violet-100 mb-1">Tip: {SMTP_TIPS[activeTip].title}</p>
                                            <p className="text-xs text-violet-700 dark:text-violet-300 leading-relaxed">
                                                {SMTP_TIPS[activeTip].content}
                                            </p>
                                        </div>
                                    </div>
                                    {/* Dot indicators */}
                                    <div className="flex justify-center gap-1.5 mt-3">
                                        {SMTP_TIPS.map((_, idx) => (
                                            <button
                                                key={idx}
                                                onClick={() => setActiveTip(idx)}
                                                className={cn(
                                                    "h-1.5 rounded-full transition-all duration-300",
                                                    activeTip === idx
                                                        ? "w-4 bg-violet-500"
                                                        : "w-1.5 bg-violet-300 dark:bg-violet-700 hover:bg-violet-400"
                                                )}
                                            />
                                        ))}
                                    </div>
                                </div>
                            </div>
                        </div>

                        {/* Gmail Note - Footer */}
                        <div className="mt-6 bg-amber-50 dark:bg-amber-950/20 rounded-xl border border-amber-200/50 dark:border-amber-800/30 p-4">
                            <div className="flex items-start gap-3">
                                <AlertCircle className="h-5 w-5 text-amber-500 mt-0.5 flex-shrink-0" />
                                <div>
                                    <p className="text-sm font-medium text-amber-700 dark:text-amber-300">Nota sobre Gmail</p>
                                    <p className="text-xs text-amber-600 dark:text-amber-400 leading-relaxed">
                                        Gmail sobrescribe el remitente &quot;From&quot; con la cuenta autenticada.
                                        Para usar un remitente personalizado, usa SES, Mailgun o SendGrid con tu dominio verificado.
                                    </p>
                                </div>
                            </div>
                        </div>
                    </TabsContent>

                    {/* Templates Tab */}
                    <TabsContent value="templates" className="mt-0">
                        <div className="grid lg:grid-cols-12 gap-6 h-[calc(100vh-280px)] min-h-[500px]">
                            {/* Template List */}
                            <div className="lg:col-span-3 bg-white dark:bg-zinc-900 rounded-2xl border border-zinc-200 dark:border-zinc-800 p-4 space-y-2 overflow-auto">
                                <p className="text-xs font-medium text-zinc-500 uppercase tracking-wide px-2 mb-3">Plantillas</p>
                                {TEMPLATE_TYPES.map(t => {
                                    const Icon = t.icon
                                    return (
                                        <button
                                            key={t.id}
                                            onClick={() => setActiveTemplate(t.id)}
                                            className={cn(
                                                "w-full flex items-center gap-3 p-3 rounded-xl text-left transition-all",
                                                activeTemplate === t.id
                                                    ? "bg-zinc-900 dark:bg-white text-white dark:text-zinc-900 shadow-lg"
                                                    : "hover:bg-zinc-100 dark:hover:bg-zinc-800 text-zinc-700 dark:text-zinc-300"
                                            )}
                                        >
                                            <Icon className="h-5 w-5 flex-shrink-0" />
                                            <div className="flex-1 min-w-0">
                                                <p className="text-sm font-medium truncate">{t.label}</p>
                                                <p className={cn(
                                                    "text-xs truncate",
                                                    activeTemplate === t.id ? "text-zinc-400 dark:text-zinc-500" : "text-zinc-500"
                                                )}>{t.description}</p>
                                            </div>
                                            <ChevronRight className={cn(
                                                "h-4 w-4 flex-shrink-0 transition-transform",
                                                activeTemplate === t.id ? "rotate-90" : ""
                                            )} />
                                        </button>
                                    )
                                })}
                            </div>

                            {/* Editor */}
                            <div className="lg:col-span-5 bg-white dark:bg-zinc-900 rounded-2xl border border-zinc-200 dark:border-zinc-800 overflow-hidden flex flex-col">
                                {/* Editor Header */}
                                <div className="px-5 py-4 border-b border-zinc-100 dark:border-zinc-800 flex items-center justify-between">
                                    <div>
                                        <h3 className="font-medium text-zinc-900 dark:text-white">{currentTemplate?.label}</h3>
                                        <p className="text-xs text-zinc-500">Edita el asunto y contenido HTML</p>
                                    </div>
                                    <div className="flex gap-2">
                                        <Button size="sm" variant="ghost" onClick={() => handleResetDefault(activeTemplate)} className="text-zinc-500 hover:text-zinc-900">
                                            <RotateCcw className="h-4 w-4 mr-1" /> Reset
                                        </Button>
                                        <Button size="sm" onClick={saveSettings} disabled={!isTemplatesDirty || updateSettingsMutation.isPending}>
                                            <Save className="h-4 w-4 mr-1" /> Guardar
                                        </Button>
                                    </div>
                                </div>

                                {/* Subject */}
                                <div className="px-5 py-4 border-b border-zinc-100 dark:border-zinc-800">
                                    <Label className="text-xs font-medium text-zinc-500 uppercase tracking-wide mb-2 block">Asunto</Label>
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
                                        className="flex-1 font-mono text-sm leading-relaxed p-5 resize-none border-0 rounded-none focus-visible:ring-0 bg-zinc-50 dark:bg-zinc-950"
                                        spellCheck={false}
                                    />
                                </div>

                                {/* Variables */}
                                <div className="p-4 border-t border-zinc-100 dark:border-zinc-800 bg-zinc-50 dark:bg-zinc-950">
                                    <p className="text-xs text-zinc-500 mb-2">Variables disponibles · Click para copiar</p>
                                    <div className="flex flex-wrap gap-1.5">
                                        {currentTemplate?.vars.map(v => (
                                            <button
                                                key={v}
                                                onClick={() => copyToClipboard(v)}
                                                className={cn(
                                                    "px-2 py-1 rounded-md text-xs font-mono transition-all",
                                                    copiedVar === v
                                                        ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/50 dark:text-emerald-300"
                                                        : "bg-zinc-200 dark:bg-zinc-800 text-zinc-700 dark:text-zinc-300 hover:bg-violet-100 hover:text-violet-700 dark:hover:bg-violet-900/30"
                                                )}
                                            >
                                                {copiedVar === v ? "✓ Copiado" : v}
                                            </button>
                                        ))}
                                    </div>
                                </div>
                            </div>

                            {/* Preview */}
                            <div className="lg:col-span-4 bg-white dark:bg-zinc-900 rounded-2xl border border-zinc-200 dark:border-zinc-800 overflow-hidden flex flex-col">
                                <div className="px-5 py-4 border-b border-zinc-100 dark:border-zinc-800 flex items-center justify-between">
                                    <div className="flex items-center gap-2">
                                        <Eye className="h-4 w-4 text-zinc-400" />
                                        <span className="font-medium text-zinc-700 dark:text-zinc-200 text-sm">Vista Previa</span>
                                    </div>
                                    <div className="flex bg-zinc-100 dark:bg-zinc-800 rounded-lg p-0.5">
                                        <button
                                            onClick={() => setPreviewMode("preview")}
                                            className={cn("px-3 py-1 text-xs rounded-md transition-all", previewMode === "preview" ? "bg-white dark:bg-zinc-700 shadow-sm" : "text-zinc-500")}
                                        >
                                            Preview
                                        </button>
                                        <button
                                            onClick={() => setPreviewMode("code")}
                                            className={cn("px-3 py-1 text-xs rounded-md transition-all", previewMode === "code" ? "bg-white dark:bg-zinc-700 shadow-sm" : "text-zinc-500")}
                                        >
                                            <Code2 className="h-3 w-3 inline mr-1" />HTML
                                        </button>
                                    </div>
                                </div>
                                <div className="flex-1 bg-zinc-100 dark:bg-zinc-950">
                                    {previewMode === "preview" ? (
                                        renderPreview(activeTemplate)
                                    ) : (
                                        <pre className="p-4 text-xs font-mono text-zinc-600 dark:text-zinc-400 overflow-auto h-full">
                                            {templatesData[activeTemplate]?.body || "Sin contenido"}
                                        </pre>
                                    )}
                                </div>
                            </div>
                        </div>
                    </TabsContent>
                </Tabs>
            </div>

            {/* Test Email Dialog */}
            <Dialog open={testEmailOpen} onOpenChange={setTestEmailOpen}>
                <DialogContent className="sm:max-w-md">
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2">
                            <Send className="h-5 w-5 text-violet-500" />
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
                            className="h-11"
                            autoFocus
                        />
                    </div>
                    <DialogFooter className="gap-2">
                        <Button variant="ghost" onClick={() => setTestEmailOpen(false)}>Cancelar</Button>
                        <Button
                            onClick={() => sendTestEmailMutation.mutate()}
                            disabled={sendTestEmailMutation.isPending || !testEmailTo}
                            className="gap-2"
                        >
                            {sendTestEmailMutation.isPending ? (
                                <div className="h-4 w-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                            ) : (
                                <Send className="h-4 w-4" />
                            )}
                            Enviar Prueba
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
