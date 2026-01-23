"use client"

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useParams, useRouter, useSearchParams } from "next/navigation"
import { Save, Mail, FileText, RotateCcw, Play, Info, AlertCircle, CheckCircle, Copy, Check } from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle, CardFooter } from "@/components/ui/card"
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
    DialogTrigger,
} from "@/components/ui/dialog"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import type { Tenant, MailingSettings, EmailTemplate } from "@/lib/types"
import { useState, useEffect } from "react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Badge } from "@/components/ui/badge"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { DEFAULT_TEMPLATES } from "@/lib/default-templates"
import { cn } from "@/lib/utils"

// --- Constants ---

const TEMPLATE_TYPES = [
    { id: "verify_email", label: "Verificación de Email", vars: ["{{.UserEmail}}", "{{.Link}}", "{{.TTL}}", "{{.Tenant}}"] },
    { id: "reset_password", label: "Restablecer Contraseña", vars: ["{{.UserEmail}}", "{{.Link}}", "{{.TTL}}", "{{.Tenant}}"] },
    { id: "user_blocked", label: "Usuario Bloqueado", vars: ["{{.UserEmail}}", "{{.Reason}}", "{{.Until}}", "{{.Tenant}}"] },
    { id: "user_unblocked", label: "Usuario Desbloqueado", vars: ["{{.UserEmail}}", "{{.Tenant}}"] },
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

    // Sync state with fetching
    useEffect(() => {
        if (settings) {
            setSmtpData(settings.smtp || {})

            // Merge saved templates with defaults if missing
            const mergedTemplates = { ...settings.mailing?.templates }
            let needsUpdate = false
            for (const key of Object.keys(DEFAULT_TEMPLATES)) {
                if (!mergedTemplates[key] || !mergedTemplates[key].body) {
                    mergedTemplates[key] = DEFAULT_TEMPLATES[key as keyof typeof DEFAULT_TEMPLATES]
                    needsUpdate = true
                }
            }
            setTemplatesData(mergedTemplates)
        }
    }, [settings])

    // --- Mutations ---

    const updateSettingsMutation = useMutation({
        mutationFn: (data: any) => {
            const etag = settings?._etag
            if (!etag) throw new Error("Missing ETag. Please refresh.")
            return api.put<any>(`/v2/admin/tenants/${tenantId}/settings`, data, etag)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["tenant-settings", tenantId, "mailing"] })
            toast({ title: "Guardado", description: "Configuración actualizada correctamente." })
        },
        onError: (e: any) => toast({ title: "Error", description: e.message, variant: "destructive" }),
    })

    const sendTestEmailMutation = useMutation({
        mutationFn: async () => {
            await api.post(`/v2/admin/tenants/${tenantId}/mailing/test`, {
                to: testEmailTo,
            })
        },
        onSuccess: () => {
            setTestEmailOpen(false)
            toast({ title: "Email enviado", description: `Prueba enviada a ${testEmailTo}` })
        },
        onError: (e: any) => toast({ title: "Error envío", description: e.message || "Error desconocido", variant: "destructive" }),
    })

    // --- Handlers ---

    const saveSmtp = () => {
        const payload = {
            ...settings,
            smtp: smtpData,
            mailing: {
                ...settings?.mailing,
                templates: templatesData,
            },
        }
        delete payload._etag
        updateSettingsMutation.mutate(payload)
    }

    const saveTemplates = () => {
        const payload = {
            ...settings,
            smtp: smtpData,
            mailing: {
                ...settings?.mailing,
                templates: templatesData,
            },
        }
        delete payload._etag
        updateSettingsMutation.mutate(payload)
    }


    const handleResetDefault = (type: string) => {
        const def = DEFAULT_TEMPLATES[type as keyof typeof DEFAULT_TEMPLATES]
        if (!def) return
        setTemplatesData(prev => ({
            ...prev,
            [type]: { subject: def.subject, body: def.body }
        }))
        toast({ title: "Restaurado", description: "Plantilla restaurada a valor por defecto. Recuerda guardar." })
    }

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text)
        setCopiedVar(text)
        setTimeout(() => setCopiedVar(null), 2000)
    }

    // --- Logic Checks ---

    const hasSavedCredentials = settings?.smtp?.host && settings?.smtp?.fromEmail

    // Dirty Checks
    const isSmtpDirty = JSON.stringify(settings?.smtp || {}) !== JSON.stringify(smtpData)
    const isTemplatesDirty = JSON.stringify(settings?.mailing?.templates || {}) !== JSON.stringify(templatesData) && Object.keys(templatesData).length > 0;


    // --- Render Helpers ---

    const renderPreview = (type: string) => {
        const tpl = templatesData[type] || DEFAULT_TEMPLATES[type as keyof typeof DEFAULT_TEMPLATES]
        // If empty body, show placeholder
        if (!tpl || !tpl.body) return (
            <div className="flex items-center justify-center h-full text-muted-foreground bg-gray-50 dark:bg-zinc-900">
                <p>Sin contenido HTML</p>
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

        Object.entries(replacements).forEach(([k, v]) => {
            html = html.replaceAll(k, v)
        })

        return (
            <iframe
                title="preview"
                srcDoc={html}
                className="w-full h-full bg-white dark:bg-white border-none block"
            />
        )
    }

    if (isLoading) {
        return <div className="flex justify-center py-12"><div className="animate-spin rounded-full h-8 w-8 border-4 border-primary border-t-transparent" /></div>
    }

    const currentTypeIdx = TEMPLATE_TYPES.find(t => t.id === activeTemplate)

    return (
        <div className="space-y-6 w-full h-[calc(100vh-100px)] flex flex-col">
            <style jsx global>{`
                .custom-scrollbar::-webkit-scrollbar {
                    width: 8px;
                    height: 8px;
                }
                .custom-scrollbar::-webkit-scrollbar-track {
                    background: transparent;
                }
                .custom-scrollbar::-webkit-scrollbar-thumb {
                    background-color: rgba(0,0,0,0.1);
                    border-radius: 4px;
                }
                .custom-scrollbar::-webkit-scrollbar-thumb:hover {
                    background-color: rgba(0,0,0,0.2);
                }
                .dark .custom-scrollbar::-webkit-scrollbar-thumb {
                    background-color: rgba(255,255,255,0.1);
                }
                .dark .custom-scrollbar::-webkit-scrollbar-thumb:hover {
                    background-color: rgba(255,255,255,0.2);
                }
            `}</style>

            {/* Header */}
            <div className="flex flex-shrink-0 items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight">Mailing</h1>
                    <p className="text-muted-foreground">Gestiona la entrega de correos y plantillas.</p>
                </div>
            </div>

            <Tabs defaultValue="smtp" className="flex-1 flex flex-col overflow-hidden min-h-0">
                <TabsList className="bg-muted/50 p-1 w-full justify-start border-b rounded-none bg-transparent flex-shrink-0 h-auto pb-0">
                    <TabsTrigger
                        value="smtp"
                        className="gap-2 rounded-t-lg rounded-b-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:bg-background px-4 py-2"
                    >
                        <Mail className="h-4 w-4" /> Configuración SMTP
                    </TabsTrigger>
                    <TabsTrigger
                        value="templates"
                        className="gap-2 rounded-t-lg rounded-b-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:bg-background px-4 py-2"
                    >
                        <FileText className="h-4 w-4" /> Plantillas de Correo
                    </TabsTrigger>
                </TabsList>

                <TabsContent value="smtp" className="overflow-y-auto p-1 text-sm">
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-6 max-w-6xl mx-auto py-6">
                        {/* SMTP Content (Unchanged) */}
                        <div className="md:col-span-2 space-y-6">
                            <Card>
                                <CardHeader>
                                    <CardTitle>Credenciales SMTP</CardTitle>
                                    <CardDescription>Configura tu proveedor de correo.</CardDescription>
                                </CardHeader>
                                <CardContent className="space-y-6">
                                    <div className="grid gap-6 md:grid-cols-2">
                                        <div className="space-y-2">
                                            <Label>Host</Label>
                                            <Input placeholder="smtp.provider.com" value={smtpData.host || ""} onChange={e => setSmtpData({ ...smtpData, host: e.target.value })} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label>Puerto</Label>
                                            <Input type="number" placeholder="587" value={smtpData.port || ""} onChange={e => setSmtpData({ ...smtpData, port: parseInt(e.target.value) || 0 })} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label>Usuario</Label>
                                            <Input placeholder="user@domain.com" value={smtpData.username || ""} onChange={e => setSmtpData({ ...smtpData, username: e.target.value })} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label>Contraseña</Label>
                                            <Input type="password" placeholder="••••••••" value={smtpData.password || ""} onChange={e => setSmtpData({ ...smtpData, password: e.target.value })} />
                                        </div>
                                    </div>
                                    <div className="space-y-2">
                                        <Label>Remitente (From)</Label>
                                        <Input placeholder="no-reply@tudominio.com" value={smtpData.fromEmail || ""} onChange={e => setSmtpData({ ...smtpData, fromEmail: e.target.value })} />
                                    </div>
                                    <div className="flex items-center space-x-2 pt-2">
                                        <Switch id="tls" checked={smtpData.useTLS || false} onCheckedChange={c => setSmtpData({ ...smtpData, useTLS: c })} />
                                        <Label htmlFor="tls">Forzar TLS/SSL</Label>
                                    </div>
                                </CardContent>
                                <CardFooter className="bg-muted/10 flex justify-end py-4 border-t">
                                    <Button onClick={saveSmtp} disabled={!isSmtpDirty || updateSettingsMutation.isPending}>
                                        <Save className="mr-2 h-4 w-4" />
                                        {updateSettingsMutation.isPending ? "Guardando..." : "Guardar Configuración"}
                                    </Button>
                                </CardFooter>
                            </Card>
                        </div>
                        <div className="space-y-6">
                            <Card className={!hasSavedCredentials ? "opacity-75" : ""}>
                                <CardHeader>
                                    <CardTitle className="text-lg">Prueba de Envío</CardTitle>
                                    <CardDescription>Verifica que tus credenciales funcionen correctamente.</CardDescription>
                                </CardHeader>
                                <CardContent>
                                    {!hasSavedCredentials ? (
                                        <Alert variant="destructive" className="mb-4">
                                            <AlertCircle className="h-4 w-4" />
                                            <AlertTitle>No configurado</AlertTitle>
                                            <AlertDescription>
                                                Debes configurar y <strong>guardar</strong> las credenciales SMTP antes de poder realizar pruebas.
                                            </AlertDescription>
                                        </Alert>
                                    ) : isSmtpDirty ? (
                                        <Alert className="mb-4 bg-yellow-50 border-yellow-200 text-yellow-800">
                                            <AlertCircle className="h-4 w-4 text-yellow-600" />
                                            <AlertTitle>Cambios sin guardar</AlertTitle>
                                            <AlertDescription>
                                                Has modificado las credenciales. Guarda los cambios para probar con la nueva configuración.
                                            </AlertDescription>
                                        </Alert>
                                    ) : (
                                        <div className="flex flex-col items-center justify-center p-4 bg-green-50 rounded-lg border border-green-100 text-green-700">
                                            <CheckCircle className="h-8 w-8 mb-2" />
                                            <p className="text-sm font-medium">Sistema listo para enviar</p>
                                        </div>
                                    )}
                                </CardContent>
                                <CardFooter>
                                    <Dialog open={testEmailOpen} onOpenChange={setTestEmailOpen}>
                                        <DialogTrigger asChild>
                                            <Button className="w-full" variant="outline" disabled={!hasSavedCredentials || isSmtpDirty}>
                                                <Play className="mr-2 h-4 w-4" /> Probar Configuración
                                            </Button>
                                        </DialogTrigger>
                                        <DialogContent>
                                            <DialogHeader>
                                                <DialogTitle>Enviar Correo de Prueba</DialogTitle>
                                                <DialogDescription>
                                                    Se enviará un correo usando las credenciales <strong>guardadas</strong> en el servidor.
                                                </DialogDescription>
                                            </DialogHeader>
                                            <div className="py-4">
                                                <Label>Destinatario</Label>
                                                <Input
                                                    placeholder="tu@email.com"
                                                    value={testEmailTo}
                                                    onChange={e => setTestEmailTo(e.target.value)}
                                                    className="mt-2"
                                                />
                                            </div>
                                            <DialogFooter>
                                                <Button variant="ghost" onClick={() => setTestEmailOpen(false)}>Cancelar</Button>
                                                <Button onClick={() => sendTestEmailMutation.mutate()} disabled={sendTestEmailMutation.isPending || !testEmailTo}>
                                                    {sendTestEmailMutation.isPending ? "Enviando..." : "Enviar Prueba"}
                                                </Button>
                                            </DialogFooter>
                                        </DialogContent>
                                    </Dialog>
                                </CardFooter>
                            </Card>
                        </div>
                    </div>
                </TabsContent>

                <TabsContent value="templates" className="flex-1 flex flex-col overflow-hidden min-h-0 data-[state=inactive]:hidden mt-0">
                    <div className="flex-1 flex flex-col h-full border-t overflow-hidden">

                        {/* Toolbar / Header */}
                        <div className="p-4 border-b flex items-center gap-4 bg-muted/10 flex-shrink-0">
                            <div className="w-[280px]">
                                <Select value={activeTemplate} onValueChange={setActiveTemplate}>
                                    <SelectTrigger className="h-9">
                                        <SelectValue placeholder="Seleccionar plantilla" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {TEMPLATE_TYPES.map((t) => (
                                            <SelectItem key={t.id} value={t.id}>
                                                {t.label}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>
                            <div className="flex-1"></div>
                            <Button size="sm" variant="outline" onClick={() => handleResetDefault(activeTemplate)}>
                                <RotateCcw className="mr-2 h-3 w-3" /> Restaurar
                            </Button>
                            <Button size="sm" onClick={saveTemplates} disabled={!isTemplatesDirty || updateSettingsMutation.isPending}>
                                <Save className="mr-2 h-3 w-3" />
                                {updateSettingsMutation.isPending ? "Guardando..." : "Guardar"}
                            </Button>
                        </div>

                        {/* Editor Area - Split View 60/40 */}
                        <div className="flex flex-1 min-h-0 divide-x">
                            {/* Editor Column (60%) */}
                            <div className="w-[60%] flex flex-col min-h-0">
                                {/* Subject Input - Fixed Height */}
                                <div className="p-6 pb-2 space-y-2 flex-shrink-0">
                                    <Label className="text-xs font-semibold uppercase text-muted-foreground">Asunto</Label>
                                    <Input
                                        value={templatesData[activeTemplate]?.subject || ""}
                                        onChange={e => {
                                            const n = { ...templatesData }
                                            if (!n[activeTemplate]) n[activeTemplate] = { subject: "", body: "" }
                                            n[activeTemplate].subject = e.target.value
                                            setTemplatesData(n)
                                        }}
                                        className="font-medium"
                                    />
                                </div>

                                {/* Textarea - Flex 1 to fill remaining space */}
                                <div className="flex-1 flex flex-col p-6 pt-2 pb-0 min-h-0">
                                    <Label className="text-xs font-semibold uppercase text-muted-foreground mb-2">Código HTML</Label>
                                    <div className="flex-1 relative rounded-md border border-input overflow-hidden">
                                        <Textarea
                                            className="custom-scrollbar w-full h-full font-mono text-xs leading-5 p-4 resize-none border-0 rounded-none focus-visible:ring-0 bg-slate-50 text-slate-800 dark:bg-zinc-950 dark:text-zinc-100"
                                            value={templatesData[activeTemplate]?.body || ""}
                                            onChange={e => {
                                                const n = { ...templatesData }
                                                if (!n[activeTemplate]) n[activeTemplate] = { subject: "", body: "" }
                                                n[activeTemplate].body = e.target.value
                                                setTemplatesData(n)
                                            }}
                                            spellCheck={false}
                                        />
                                    </div>
                                </div>

                                {/* Variables Chips - Fixed Height at bottom */}
                                <div className="p-6 pt-4 space-y-2 flex-shrink-0">
                                    <Label className="text-xs font-semibold uppercase text-muted-foreground flex items-center gap-2">
                                        Variables Disponibles <Info className="h-3 w-3" />
                                    </Label>
                                    <div className="flex flex-wrap gap-2">
                                        {currentTypeIdx?.vars.map(v => (
                                            <Badge
                                                key={v}
                                                variant="secondary"
                                                onClick={() => copyToClipboard(v)}
                                                className={cn(
                                                    "cursor-pointer hover:bg-primary/20 transition-all font-mono text-[11px] px-2 py-1 flex items-center gap-1 border",
                                                    copiedVar === v ? "border-green-500 bg-green-50 text-green-700" : "border-transparent"
                                                )}
                                            >
                                                {v}
                                                {copiedVar === v ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3 opacity-50" />}
                                            </Badge>
                                        ))}
                                    </div>
                                    <p className="text-[10px] text-muted-foreground">Haz clic en una variable para copiarla.</p>
                                </div>
                            </div>

                            {/* Preview Column (40%) */}
                            <div className="w-[40%] flex flex-col h-full bg-gray-100 dark:bg-zinc-900 overflow-hidden">
                                <div className="flex-1 w-full h-full p-0 flex">
                                    {renderPreview(activeTemplate)}
                                </div>
                            </div>
                        </div>
                    </div>
                </TabsContent>
            </Tabs>
        </div>
    )
}
