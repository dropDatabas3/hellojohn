"use client"

import { useState } from "react"
import {
    Plus,
    Trash2,
    AlertTriangle,
    ChevronDown,
    Link2,
    Globe,
    LogOut,
    Lightbulb,
    Wand2,
    Info,
} from "lucide-react"
import {
    Button,
    Input,
    Label,
    Badge,
    Collapsible,
    CollapsibleContent,
    CollapsibleTrigger,
    cn,
} from "@/components/ds"
import { InfoTooltip } from "@/components/clients/shared"
import { APP_SUB_TYPES } from "./constants"
import { validateUri } from "./helpers"
import type { ClientFormState } from "./types"

interface StepUrisProps {
    form: ClientFormState
    onChange: (patch: Partial<ClientFormState>) => void
}

// Reusable URI list manager
function UriListInput({
    label,
    tooltip,
    placeholder,
    uris,
    required,
    suggestions,
    onAdd,
    onRemove,
    icon: Icon,
}: {
    label: string
    tooltip: string
    placeholder: string
    uris: string[]
    required?: boolean
    suggestions?: string[]
    onAdd: (uri: string) => void
    onRemove: (uri: string) => void
    icon: React.ElementType
}) {
    const [input, setInput] = useState("")
    const [error, setError] = useState<string | null>(null)

    const handleAdd = () => {
        const trimmed = input.trim()
        if (!trimmed) return

        // Support pasting multiple URIs separated by comma, newline, or space
        const candidates = trimmed.split(/[\s,\n]+/).map(s => s.trim()).filter(Boolean)

        if (candidates.length > 1) {
            let addedCount = 0
            let lastError: string | null = null
            for (const uri of candidates) {
                if (uris.includes(uri)) continue
                const result = validateUri(uri)
                if (result.valid) {
                    onAdd(uri)
                    addedCount++
                } else {
                    lastError = `"${uri.slice(0, 30)}..." — ${result.error}`
                }
            }
            setInput("")
            if (lastError && addedCount === 0) {
                setError(lastError)
            } else {
                setError(null)
            }
            return
        }

        // Single URI
        if (uris.includes(trimmed)) {
            setError("Esta URI ya fue agregada")
            return
        }

        const result = validateUri(trimmed)
        if (!result.valid) {
            setError(result.error || "URI invalida")
            return
        }

        onAdd(trimmed)
        setInput("")
        setError(null)
    }

    const handlePaste = (e: React.ClipboardEvent<HTMLInputElement>) => {
        const pasted = e.clipboardData.getData("text")
        const lines = pasted.split(/[\s,\n]+/).map(s => s.trim()).filter(Boolean)
        if (lines.length > 1) {
            e.preventDefault()
            let addedCount = 0
            for (const uri of lines) {
                if (uris.includes(uri)) continue
                const result = validateUri(uri)
                if (result.valid) {
                    onAdd(uri)
                    addedCount++
                }
            }
            if (addedCount > 0) {
                setInput("")
                setError(null)
            }
        }
    }

    const handleAddSuggestion = (uri: string) => {
        if (!uris.includes(uri)) {
            onAdd(uri)
        }
    }

    // Filter out suggestions already in the list
    const availableSuggestions = suggestions?.filter(s => !uris.includes(s)) || []

    return (
        <div className="space-y-2">
            <Label className="flex items-center gap-1">
                <Icon className="h-3.5 w-3.5 text-muted-foreground" />
                {label}
                {required && <span className="text-danger">*</span>}
                <InfoTooltip content={tooltip} />
            </Label>

            <div className="flex gap-2">
                <Input
                    value={input}
                    onChange={(e) => { setInput(e.target.value); setError(null) }}
                    placeholder={placeholder}
                    className="font-mono text-sm"
                    onKeyDown={(e) => {
                        if (e.key === "Enter") { e.preventDefault(); handleAdd() }
                    }}
                    onPaste={handlePaste}
                />
                <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleAdd}
                    disabled={!input.trim()}
                    className="shrink-0"
                >
                    <Plus className="h-4 w-4 mr-1" />
                    Agregar
                </Button>
            </div>

            {error && (
                <p className="text-xs text-danger flex items-center gap-1">
                    <AlertTriangle className="h-3 w-3" />
                    {error}
                </p>
            )}

            {required && uris.length === 0 && !error && (
                <p className="text-xs text-warning flex items-center gap-1">
                    <AlertTriangle className="h-3.5 w-3.5" />
                    Se requiere al menos una URI
                </p>
            )}

            {/* Suggestions */}
            {availableSuggestions.length > 0 && uris.length === 0 && (
                <div className="flex items-center gap-2 flex-wrap">
                    <span className="text-xs text-muted-foreground flex items-center gap-1">
                        <Lightbulb className="h-3 w-3" />
                        Sugerencias:
                    </span>
                    {availableSuggestions.map((s) => (
                        <button
                            key={s}
                            type="button"
                            onClick={() => handleAddSuggestion(s)}
                            className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md border border-dashed border-primary/30 text-xs font-mono text-primary/80 hover:bg-primary/5 hover:border-primary/50 transition-colors"
                        >
                            <Plus className="h-2.5 w-2.5" />
                            {s}
                        </button>
                    ))}
                </div>
            )}

            {/* Added URIs */}
            {uris.length > 0 && (
                <div className="space-y-1">
                    {uris.map((uri) => (
                        <div
                            key={uri}
                            className="flex items-center justify-between rounded-lg bg-muted/50 border px-3 py-2 group"
                        >
                            <code className="text-sm truncate flex-1 text-foreground/80">{uri}</code>
                            <Button
                                variant="ghost"
                                size="sm"
                                className="h-6 w-6 opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-danger"
                                onClick={() => onRemove(uri)}
                            >
                                <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}

export function StepUris({ form, onChange }: StepUrisProps) {
    const [corsOpen, setCorsOpen] = useState(false)
    const [logoutOpen, setLogoutOpen] = useState(false)
    const subTypeConfig = APP_SUB_TYPES[form.subType]
    const isPublic = subTypeConfig.type === "public"

    const hasSuggestedUris = subTypeConfig.suggestedRedirectUris.length > 0

    // Get contextual tooltip based on client type
    const getRedirectUriTooltip = () => {
        if (form.subType === "mobile") {
            return "URL de deep link donde tu app recibe el callback de autenticacion. Usa custom scheme (ej: myapp://callback) o universal links."
        }
        if (form.subType === "api_server") {
            return "URL en tu servidor donde HelloJohn redirige despues del login. Tu backend recibe el codigo y lo intercambia por tokens."
        }
        // Default for SPA
        return "URL donde HelloJohn redirige al usuario despues del login exitoso. Debe coincidir exactamente con la que envie tu app."
    }

    // Agregar todas las URIs sugeridas de una vez
    const handleAddDevUris = () => {
        const toAdd = subTypeConfig.suggestedRedirectUris.filter(uri => !form.redirectUris.includes(uri))
        if (toAdd.length > 0) {
            onChange({ redirectUris: [...form.redirectUris, ...toAdd] })
        }
        // Also add CORS origins for public clients
        if (isPublic && subTypeConfig.suggestedOrigins.length > 0) {
            const originsToAdd = subTypeConfig.suggestedOrigins.filter(o => !form.allowedOrigins.includes(o))
            if (originsToAdd.length > 0) {
                onChange({ allowedOrigins: [...form.allowedOrigins, ...originsToAdd] })
            }
        }
    }

    const hasDevUrisToAdd = hasSuggestedUris &&
        subTypeConfig.suggestedRedirectUris.some(uri => !form.redirectUris.includes(uri))

    // Get contextual placeholder
    const getPlaceholder = () => {
        if (form.subType === "mobile") return "myapp://callback"
        if (form.subType === "api_server") return "https://api.miempresa.com/auth/callback"
        return "https://miapp.com/callback"
    }

    return (
        <div className="space-y-6">
            {/* Contextual info for the step */}
            <div className="p-3 rounded-lg bg-primary/5 border border-primary/20 flex items-start gap-3">
                <Info className="h-4 w-4 mt-0.5 shrink-0 text-primary" />
                <div className="space-y-1">
                    <p className="text-sm font-medium text-primary">
                        Configuracion de URIs para {subTypeConfig.label}
                    </p>
                    <p className="text-xs text-muted-foreground">
                        {form.subType === "mobile"
                            ? "Configura el deep link o universal link donde tu app recibira el callback de autenticacion."
                            : form.subType === "api_server"
                                ? "Configura la URL de tu servidor donde recibiras el codigo de autorizacion para intercambiarlo por tokens."
                                : "Configura las URLs donde HelloJohn redirigira al usuario despues de completar la autenticacion."
                        }
                    </p>
                </div>
            </div>

            {/* Quick action: Add development URIs */}
            {hasDevUrisToAdd && form.redirectUris.length === 0 && (
                <Button
                    type="button"
                    variant="outline"
                    onClick={handleAddDevUris}
                    className="w-full justify-center gap-2 border-dashed border-primary/40 text-primary hover:bg-primary/5 hover:border-primary"
                >
                    <Wand2 className="h-4 w-4" />
                    Agregar URIs de desarrollo (localhost)
                </Button>
            )}

            {/* Main: Redirect URIs */}
            <UriListInput
                label="URIs de redireccion"
                tooltip={getRedirectUriTooltip()}
                placeholder={getPlaceholder()}
                uris={form.redirectUris}
                required={isPublic}
                suggestions={subTypeConfig.suggestedRedirectUris}
                onAdd={(uri) => onChange({ redirectUris: [...form.redirectUris, uri] })}
                onRemove={(uri) => onChange({ redirectUris: form.redirectUris.filter(u => u !== uri) })}
                icon={Link2}
            />

            {/* Collapsible: CORS Origins (public only) */}
            {isPublic && (
                <Collapsible open={corsOpen} onOpenChange={setCorsOpen}>
                    <CollapsibleTrigger asChild>
                        <button
                            type="button"
                            className="flex items-center justify-between w-full p-3 rounded-lg border border-border/50 hover:bg-muted/30 transition-colors group"
                        >
                            <div className="flex items-center gap-2">
                                <Globe className="h-4 w-4 text-muted-foreground" />
                                <span className="text-sm font-medium">Origenes permitidos (CORS)</span>
                                {form.allowedOrigins.length > 0 && (
                                    <Badge variant="secondary" className="text-[10px]">
                                        {form.allowedOrigins.length}
                                    </Badge>
                                )}
                            </div>
                            <ChevronDown className={cn(
                                "h-4 w-4 text-muted-foreground transition-transform duration-200",
                                corsOpen && "rotate-180"
                            )} />
                        </button>
                    </CollapsibleTrigger>
                    <CollapsibleContent className="pt-3">
                        <UriListInput
                            label="Origenes CORS"
                            tooltip="Dominio desde donde el browser puede hacer requests JS. Solo el origen, sin path. Ej: https://miapp.com"
                            placeholder="http://localhost:3000"
                            uris={form.allowedOrigins}
                            suggestions={subTypeConfig.suggestedOrigins}
                            onAdd={(uri) => onChange({ allowedOrigins: [...form.allowedOrigins, uri] })}
                            onRemove={(uri) => onChange({ allowedOrigins: form.allowedOrigins.filter(o => o !== uri) })}
                            icon={Globe}
                        />
                    </CollapsibleContent>
                </Collapsible>
            )}

            {/* Collapsible: Post-Logout URIs */}
            <Collapsible open={logoutOpen} onOpenChange={setLogoutOpen}>
                <CollapsibleTrigger asChild>
                    <button
                        type="button"
                        className="flex items-center justify-between w-full p-3 rounded-lg border border-border/50 hover:bg-muted/30 transition-colors group"
                    >
                        <div className="flex items-center gap-2">
                            <LogOut className="h-4 w-4 text-muted-foreground" />
                            <span className="text-sm font-medium">URIs post-logout</span>
                            {form.postLogoutUris.length > 0 && (
                                <Badge variant="secondary" className="text-[10px]">
                                    {form.postLogoutUris.length}
                                </Badge>
                            )}
                        </div>
                        <ChevronDown className={cn(
                            "h-4 w-4 text-muted-foreground transition-transform duration-200",
                            logoutOpen && "rotate-180"
                        )} />
                    </button>
                </CollapsibleTrigger>
                <CollapsibleContent className="pt-3">
                    <UriListInput
                        label="URIs post-logout"
                        tooltip="URL a donde redirigir al usuario despues de cerrar sesion. Ej: https://miapp.com/logged-out"
                        placeholder="https://miapp.com/logged-out"
                        uris={form.postLogoutUris}
                        onAdd={(uri) => onChange({ postLogoutUris: [...form.postLogoutUris, uri] })}
                        onRemove={(uri) => onChange({ postLogoutUris: form.postLogoutUris.filter(u => u !== uri) })}
                        icon={LogOut}
                    />
                </CollapsibleContent>
            </Collapsible>

            {/* Summary count */}
            <div className="flex items-center gap-4 text-xs text-muted-foreground pt-2 border-t border-border/50">
                <span>{form.redirectUris.length} redirect URI{form.redirectUris.length !== 1 ? "s" : ""}</span>
                {isPublic && (
                    <span>{form.allowedOrigins.length} CORS origin{form.allowedOrigins.length !== 1 ? "s" : ""}</span>
                )}
                <span>{form.postLogoutUris.length} post-logout URI{form.postLogoutUris.length !== 1 ? "s" : ""}</span>
            </div>
        </div>
    )
}
