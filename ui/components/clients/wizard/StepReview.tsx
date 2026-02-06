"use client"

import { useState, useMemo } from "react"
import {
    Globe,
    Server,
    Smartphone,
    Cpu,
    ChevronDown,
    Clock,
    Shield,
    Key,
    Link2,
    Zap,
    AlertTriangle,
    Lock,
} from "lucide-react"
import {
    Badge,
    Checkbox,
    Label,
    Switch,
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
    Collapsible,
    CollapsibleContent,
    CollapsibleTrigger,
    cn,
} from "@/components/ds"
import {
    APP_SUB_TYPES,
    GRANT_TYPES,
    AVAILABLE_PROVIDERS,
    TOKEN_TTL_OPTIONS,
    hasInteractiveUsers as checkHasInteractiveUsers,
    requiresRedirectUris as checkRequiresRedirectUris,
} from "./constants"
import { formatTTL } from "./helpers"
import type { ClientFormState } from "./types"

const REVIEW_ICONS: Record<string, React.ElementType> = {
    Globe,
    Smartphone,
    Server,
    Cpu,
}

interface StepReviewProps {
    form: ClientFormState
    onChange: (patch: Partial<ClientFormState>) => void
}

export function StepReview({ form, onChange }: StepReviewProps) {
    const [advancedOpen, setAdvancedOpen] = useState(false)
    const subTypeConfig = APP_SUB_TYPES[form.subType]
    const isPublic = subTypeConfig.type === "public"
    const SubTypeIcon = REVIEW_ICONS[subTypeConfig.icon] || Globe

    // Use config metadata for dynamic behavior
    const isM2M = form.subType === "m2m"
    const needsRedirectUris = useMemo(() => checkRequiresRedirectUris(form.subType), [form.subType])
    const hasUsers = useMemo(() => checkHasInteractiveUsers(form.subType), [form.subType])

    // Filter grant types to only show relevant ones for this subType
    const relevantGrantTypes = useMemo(() => {
        const relevantIds = subTypeConfig.relevantGrantTypes || []
        return GRANT_TYPES.filter(gt => relevantIds.includes(gt.id))
    }, [subTypeConfig.relevantGrantTypes])

    // Get scopes label based on type  
    const getScopesLabel = () => isM2M ? "Scopes de API" : "Scopes"

    return (
        <div className="space-y-5">
            {/* M2M Security Banner */}
            {isM2M && (
                <div className="p-3 rounded-lg bg-amber-500/10 border border-amber-500/30 flex items-start gap-3">
                    <Lock className="h-4 w-4 mt-0.5 shrink-0 text-amber-500" />
                    <div className="space-y-1">
                        <p className="text-sm font-medium text-amber-600 dark:text-amber-400">
                            Cliente Machine-to-Machine (M2M)
                        </p>
                        <p className="text-xs text-muted-foreground">
                            Este cliente recibira un <code className="bg-muted px-1 rounded">client_secret</code> que
                            tiene acceso completo a los scopes configurados. Guardalo de forma segura y nunca lo
                            expongas en codigo del lado del cliente.
                        </p>
                    </div>
                </div>
            )}

            {/* Summary card */}
            <div className="rounded-xl border bg-gradient-to-br from-muted/30 to-muted/10 p-5 space-y-4">
                {/* Header */}
                <div className="flex items-start gap-4">
                    <div className={cn(
                        "p-2.5 rounded-xl shrink-0",
                        isM2M
                            ? "bg-amber-500/10 text-amber-500"
                            : isPublic
                                ? "bg-success/10 text-success"
                                : "bg-accent/10 text-accent"
                    )}>
                        <SubTypeIcon className="h-5 w-5" />
                    </div>
                    <div className="flex-1 min-w-0">
                        <h3 className="text-base font-semibold truncate">{form.name || "Sin nombre"}</h3>
                        {form.description && (
                            <p className="text-sm text-muted-foreground mt-0.5 line-clamp-2">{form.description}</p>
                        )}
                        <div className="flex items-center gap-2 mt-2">
                            <Badge variant={isPublic ? "success" : "default"}>
                                {subTypeConfig.label}
                            </Badge>
                            <Badge variant="outline" className="text-[10px]">
                                {isM2M
                                    ? "Confidential (client_credentials)"
                                    : isPublic
                                        ? "Public (PKCE)"
                                        : "Confidential (Secret)"
                                }
                            </Badge>
                        </div>
                    </div>
                </div>

                {/* Divider */}
                <div className="border-t border-border/50" />

                {/* Key details grid */}
                <div className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
                    {/* Client ID */}
                    <div>
                        <span className="text-xs text-muted-foreground">Client ID</span>
                        <p className="font-mono text-xs mt-0.5 truncate">{form.clientId}</p>
                    </div>

                    {/* Scopes */}
                    <div>
                        <span className="text-xs text-muted-foreground">{getScopesLabel()}</span>
                        <div className="flex flex-wrap gap-1 mt-0.5">
                            {form.scopes.slice(0, 4).map(s => (
                                <Badge key={s} variant="outline" className="text-[10px]">{s}</Badge>
                            ))}
                            {form.scopes.length > 4 && (
                                <Badge variant="outline" className="text-[10px]">+{form.scopes.length - 4}</Badge>
                            )}
                            {form.scopes.length === 0 && (
                                <span className="text-xs text-muted-foreground italic">Ninguno</span>
                            )}
                        </div>
                    </div>

                    {/* Redirect URIs - Only show if relevant */}
                    {needsRedirectUris && (
                        <div>
                            <span className="text-xs text-muted-foreground flex items-center gap-1">
                                <Link2 className="h-3 w-3" /> Redirect URIs
                            </span>
                            {form.redirectUris.length > 0 ? (
                                <ul className="mt-0.5 space-y-0.5">
                                    {form.redirectUris.slice(0, 2).map(u => (
                                        <li key={u} className="font-mono text-[11px] truncate">{u}</li>
                                    ))}
                                    {form.redirectUris.length > 2 && (
                                        <li className="text-[11px] text-muted-foreground">+{form.redirectUris.length - 2} mas</li>
                                    )}
                                </ul>
                            ) : (
                                <p className="text-xs text-muted-foreground mt-0.5 italic">Ninguna</p>
                            )}
                        </div>
                    )}

                    {/* Audience for M2M (instead of URIs) */}
                    {isM2M && (
                        <div>
                            <span className="text-xs text-muted-foreground flex items-center gap-1">
                                <Shield className="h-3 w-3" /> Tipo de Acceso
                            </span>
                            <p className="text-xs mt-0.5">
                                Servicio backend a APIs protegidas
                            </p>
                        </div>
                    )}

                    {/* Grant Types */}
                    <div>
                        <span className="text-xs text-muted-foreground flex items-center gap-1">
                            <Key className="h-3 w-3" /> Grant Types
                        </span>
                        <div className="flex flex-wrap gap-1 mt-0.5">
                            {form.grantTypes.map(gt => (
                                <Badge key={gt} variant="outline" className="text-[10px]">
                                    {gt.replace(/_/g, " ")}
                                </Badge>
                            ))}
                        </div>
                    </div>

                    {/* Token TTLs */}
                    <div className="col-span-2">
                        <span className="text-xs text-muted-foreground flex items-center gap-1">
                            <Clock className="h-3 w-3" /> Tokens
                        </span>
                        <div className="flex items-center gap-4 mt-1">
                            <span className="text-xs">
                                Access: <strong>{formatTTL(form.accessTokenTTL)}</strong>
                            </span>
                            {/* Only show Refresh Token for clients that can use it */}
                            {form.grantTypes.includes("refresh_token") && (
                                <span className="text-xs">
                                    Refresh: <strong>{formatTTL(form.refreshTokenTTL)}</strong>
                                </span>
                            )}
                            {/* Only show ID Token for clients with users */}
                            {hasUsers && (
                                <span className="text-xs">
                                    ID: <strong>{formatTTL(form.idTokenTTL)}</strong>
                                </span>
                            )}
                        </div>
                    </div>
                </div>
            </div>

            {/* Advanced config - Collapsible */}
            <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
                <CollapsibleTrigger asChild>
                    <button
                        type="button"
                        className="flex items-center justify-between w-full p-3 rounded-lg border border-border/50 hover:bg-muted/30 transition-colors"
                    >
                        <div className="flex items-center gap-2">
                            <Settings2Icon className="h-4 w-4 text-muted-foreground" />
                            <span className="text-sm font-medium">Configuracion avanzada</span>
                            <Badge variant="secondary" className="text-[10px]">Opcional</Badge>
                        </div>
                        <ChevronDown className={cn(
                            "h-4 w-4 text-muted-foreground transition-transform duration-200",
                            advancedOpen && "rotate-180"
                        )} />
                    </button>
                </CollapsibleTrigger>
                <CollapsibleContent className="pt-4 space-y-6">
                    {/* Grant Types config - filtered by relevance */}
                    <div className="space-y-3">
                        <Label className="text-sm font-medium flex items-center gap-1">
                            <Shield className="h-3.5 w-3.5 text-muted-foreground" />
                            Grant Types
                            {isM2M && (
                                <Badge variant="outline" className="text-[10px] ml-1">Solo client_credentials</Badge>
                            )}
                        </Label>
                        <div className="space-y-2">
                            {relevantGrantTypes.map((gt) => {
                                const disabled = gt.confidentialOnly && isPublic
                                const checked = form.grantTypes.includes(gt.id)
                                // For M2M, client_credentials should be always on and disabled
                                const isRequired = isM2M && gt.id === "client_credentials"
                                return (
                                    <label
                                        key={gt.id}
                                        className={cn(
                                            "flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors",
                                            (disabled || isRequired) && "opacity-60 cursor-not-allowed",
                                            checked && !disabled && "border-primary/30 bg-primary/5",
                                            !checked && !disabled && "hover:bg-muted/50"
                                        )}
                                    >
                                        <Checkbox
                                            checked={checked || isRequired}
                                            disabled={disabled || isRequired}
                                            onCheckedChange={(c) => {
                                                if (isRequired) return // Can't uncheck required grants
                                                if (c) {
                                                    onChange({ grantTypes: [...form.grantTypes, gt.id] })
                                                } else {
                                                    onChange({ grantTypes: form.grantTypes.filter(g => g !== gt.id) })
                                                }
                                            }}
                                            className="mt-0.5"
                                        />
                                        <div className="flex-1">
                                            <div className="flex items-center gap-2">
                                                <span className="font-medium text-sm">{gt.label}</span>
                                                {gt.recommended && <Badge variant="secondary" className="text-[10px]">Recomendado</Badge>}
                                                {gt.deprecated && <Badge variant="destructive" className="text-[10px]">Deprecado</Badge>}
                                                {gt.confidentialOnly && <Badge variant="outline" className="text-[10px]">Solo Backend</Badge>}
                                                {isRequired && <Badge variant="outline" className="text-[10px]">Requerido</Badge>}
                                            </div>
                                            <p className="text-xs text-muted-foreground mt-0.5">{gt.description}</p>
                                        </div>
                                    </label>
                                )
                            })}
                        </div>
                    </div>

                    {/* Token TTLs */}
                    <div className="space-y-3">
                        <Label className="text-sm font-medium flex items-center gap-1">
                            <Clock className="h-3.5 w-3.5 text-muted-foreground" />
                            Duracion de Tokens
                        </Label>
                        <div className={cn(
                            "grid gap-3",
                            hasUsers ? "grid-cols-1 sm:grid-cols-3" : "grid-cols-1 sm:grid-cols-2"
                        )}>
                            <div className="space-y-1.5">
                                <Label className="text-xs text-muted-foreground">Access Token</Label>
                                <Select
                                    value={String(form.accessTokenTTL)}
                                    onValueChange={(v) => onChange({ accessTokenTTL: Number(v) })}
                                >
                                    <SelectTrigger className="h-9">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {TOKEN_TTL_OPTIONS.access.map((opt) => (
                                            <SelectItem key={opt.value} value={String(opt.value)}>
                                                {opt.label} — {opt.description}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>
                            {/* Only show Refresh if grant type is enabled */}
                            {form.grantTypes.includes("refresh_token") && (
                                <div className="space-y-1.5">
                                    <Label className="text-xs text-muted-foreground">Refresh Token</Label>
                                    <Select
                                        value={String(form.refreshTokenTTL)}
                                        onValueChange={(v) => onChange({ refreshTokenTTL: Number(v) })}
                                    >
                                        <SelectTrigger className="h-9">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {TOKEN_TTL_OPTIONS.refresh.map((opt) => (
                                                <SelectItem key={opt.value} value={String(opt.value)}>
                                                    {opt.label} — {opt.description}
                                                </SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                </div>
                            )}
                            {/* Only show ID Token for clients with users */}
                            {hasUsers && (
                                <div className="space-y-1.5">
                                    <Label className="text-xs text-muted-foreground">ID Token</Label>
                                    <Select
                                        value={String(form.idTokenTTL)}
                                        onValueChange={(v) => onChange({ idTokenTTL: Number(v) })}
                                    >
                                        <SelectTrigger className="h-9">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {TOKEN_TTL_OPTIONS.id.map((opt) => (
                                                <SelectItem key={opt.value} value={String(opt.value)}>
                                                    {opt.label} — {opt.description}
                                                </SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                </div>
                            )}
                        </div>
                        {isM2M && (
                            <p className="text-xs text-muted-foreground flex items-center gap-1">
                                <AlertTriangle className="h-3 w-3" />
                                M2M no usa Refresh ni ID tokens. Solo Access tokens via client_credentials.
                            </p>
                        )}
                    </div>

                    {/* Providers (for clients with interactive users) */}
                    {hasUsers && (
                        <div className="space-y-3">
                            <Label className="text-sm font-medium flex items-center gap-1">
                                <Zap className="h-3.5 w-3.5 text-muted-foreground" />
                                Proveedores de autenticacion
                            </Label>
                            <div className="rounded-lg border divide-y">
                                {AVAILABLE_PROVIDERS.map((p) => (
                                    <div key={p.id} className={cn("flex items-center justify-between px-4 py-3", !p.enabled && "opacity-50")}>
                                        <div className="flex items-center gap-3">
                                            <span className="text-lg">{p.icon}</span>
                                            <span className="text-sm font-medium">{p.label}</span>
                                            {p.comingSoon && <Badge variant="outline" className="text-[10px]">Proximamente</Badge>}
                                        </div>
                                        <Switch
                                            disabled={!p.enabled}
                                            checked={form.providers.includes(p.id)}
                                            onCheckedChange={(c) => {
                                                if (c) {
                                                    onChange({ providers: [...form.providers, p.id] })
                                                } else {
                                                    onChange({ providers: form.providers.filter(pr => pr !== p.id) })
                                                }
                                            }}
                                        />
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                </CollapsibleContent>
            </Collapsible>
        </div>
    )
}

// Small wrapper to avoid importing Settings2 at top level (already using many icons)
function Settings2Icon(props: React.SVGProps<SVGSVGElement>) {
    return (
        <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" {...props}>
            <path d="M20 7h-9" /><path d="M14 17H5" /><circle cx="17" cy="17" r="3" /><circle cx="7" cy="7" r="3" />
        </svg>
    )
}
