"use client"

import * as React from "react"
import { useRouter, useParams } from "next/navigation"
import {
    Building2,
    Key,
    Shield,
    FileText,
    LogOut,
    Plus,
    Search,
    LayoutDashboard,
    Users,
    Boxes,
    Settings,
    Server,
    Activity,
    Zap,
    Globe2,
    Mail,
    Database,
    Shapes,
    Lock,
    ListChecks,
    LayoutTemplate,
    User,
    Moon,
    Sun,
} from "lucide-react"

import {
    CommandDialog,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
    CommandSeparator,
    CommandShortcut,
} from "@/components/ds"
import { useAuthStore } from "@/lib/auth-store"
import { useUIStore } from "@/lib/ui-store"
import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { cn } from "@/lib/utils"
import type { Tenant } from "@/lib/types"

export function CommandPalette() {
    const [open, setOpen] = React.useState(false)
    const [search, setSearch] = React.useState("")
    const router = useRouter()
    const clearAuth = useAuthStore((state) => state.clearAuth)
    const toggleTheme = useUIStore((state) => state.toggleTheme)
    const theme = useUIStore((state) => state.theme)
    const params = useParams()
    const currentTenantId = (params?.tenant_id as string)

    // Fetch tenants para búsqueda y contexto
    const { data: tenants } = useQuery({
        queryKey: ["tenants"],
        queryFn: () => api.get<Tenant[]>("/v2/admin/tenants"),
    })

    // Filtrar tenants basándose en la búsqueda
    const listRef = React.useRef<HTMLDivElement>(null)
    const filteredTenants = React.useMemo(() => {
        if (!search || !tenants) return []
        const searchLower = search.toLowerCase()
        return tenants.filter(tenant =>
            tenant.name.toLowerCase().includes(searchLower) ||
            tenant.slug.toLowerCase().includes(searchLower)
        )
    }, [search, tenants])

    // Auto-scroll al inicio cuando se empieza a escribir
    React.useEffect(() => {
        if (search && listRef.current) {
            listRef.current.scrollTop = 0
        }
    }, [search])

    React.useEffect(() => {
        const down = (e: KeyboardEvent) => {
            if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
                e.preventDefault()
                setOpen((open) => !open)
            }
        }

        document.addEventListener("keydown", down)
        return () => document.removeEventListener("keydown", down)
    }, [])

    const runCommand = React.useCallback((command: () => unknown) => {
        setOpen(false)
        setSearch("") // Limpiar búsqueda al ejecutar comando
        command()
    }, [])

    // Limpiar búsqueda cuando se cierra el modal
    React.useEffect(() => {
        if (!open) {
            setSearch("")
        }
    }, [open])

    return (
        <>
            <button
                onClick={() => setOpen(true)}
                className={cn(
                    "group relative w-full h-10 rounded-xl px-4",
                    "inline-flex items-center justify-start gap-3",
                    "font-medium text-sm transition-all duration-300 ease-out",
                    // Solid background with subtle gradient
                    "bg-gradient-to-b from-slate-50 to-slate-100/80",
                    "dark:from-slate-800 dark:to-slate-900/90",
                    "hover:from-slate-100 hover:to-slate-150/80",
                    "dark:hover:from-slate-700 dark:hover:to-slate-800/90",
                    // Clean border
                    "border border-slate-200/80 dark:border-slate-700/50",
                    "hover:border-slate-300 dark:hover:border-slate-600",
                    // Premium 3D shadow stack
                    "shadow-[0_1px_2px_rgba(0,0,0,0.04),0_2px_4px_rgba(0,0,0,0.03),inset_0_1px_0_rgba(255,255,255,0.8)]",
                    "dark:shadow-[0_1px_2px_rgba(0,0,0,0.2),0_2px_4px_rgba(0,0,0,0.15),inset_0_1px_0_rgba(255,255,255,0.05)]",
                    "hover:shadow-[0_2px_4px_rgba(0,0,0,0.06),0_4px_8px_rgba(0,0,0,0.03),inset_0_1px_0_rgba(255,255,255,0.9)]",
                    "dark:hover:shadow-[0_2px_4px_rgba(0,0,0,0.25),0_4px_8px_rgba(0,0,0,0.2),inset_0_1px_0_rgba(255,255,255,0.08)]",
                    // Micro interaction
                    "hover:-translate-y-0.5 active:translate-y-0",
                    // Focus states
                    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/30 focus-visible:ring-offset-1",
                    "disabled:pointer-events-none disabled:opacity-50",
                    // Text color
                    "text-slate-500 dark:text-slate-400",
                    "hover:text-slate-700 dark:hover:text-slate-200"
                )}
            >
                <Search className="h-[18px] w-[18px] shrink-0 text-slate-500 dark:text-slate-300 group-hover:text-slate-600 dark:group-hover:text-white transition-colors" />
                <span className="hidden lg:inline-flex truncate">Buscar comandos...</span>
                <span className="inline-flex lg:hidden">Buscar...</span>
                <kbd className={cn(
                    "pointer-events-none absolute right-3 top-1/2 -translate-y-1/2",
                    "hidden sm:inline-flex items-center gap-1",
                    "h-6 px-2 rounded-md",
                    "bg-slate-800 dark:bg-slate-700",
                    "border border-slate-600/50 dark:border-slate-500/50",
                    "font-mono font-semibold",
                    "text-white dark:text-white",
                    "shadow-[0_1px_3px_rgba(0,0,0,0.2),inset_0_1px_0_rgba(255,255,255,0.1)]",
                    "dark:shadow-[0_1px_3px_rgba(0,0,0,0.3),inset_0_1px_0_rgba(255,255,255,0.08)]"
                )}>
                    <span className="text-[10px] opacity-80">⌘</span>
                    <span className="text-xs">K</span>
                </kbd>
            </button>
            <CommandDialog open={open} onOpenChange={setOpen}>
                <CommandInput
                    placeholder="Buscar comandos, tenants..."
                    value={search}
                    onValueChange={setSearch}
                />
                <CommandList ref={listRef} className="max-h-[450px] overflow-y-auto scrollbar-custom">
                    <CommandEmpty>No se encontraron resultados.</CommandEmpty>

                    {/* Tenants - Solo mostrar cuando hay búsqueda */}
                    {search && filteredTenants.length > 0 && (
                        <>
                            <CommandGroup heading="🏢 Tenants">
                                {filteredTenants.slice(0, 5).map((tenant) => (
                                    <CommandItem
                                        key={tenant.id}
                                        value={tenant.name}
                                        keywords={[tenant.slug, tenant.name, 'tenant', 'tenants']}
                                        onSelect={() => {
                                            runCommand(() => router.push(`/admin/tenants/${tenant.id}/detail`))
                                        }}
                                        className="cursor-pointer relative overflow-hidden group aria-selected:bg-primary/15"
                                    >
                                        <div className="absolute inset-0 pointer-events-none bg-gradient-to-r from-primary/8 to-primary/12 opacity-100 group-hover:from-primary/12 group-hover:to-primary/18 group-aria-selected:from-primary/15 group-aria-selected:to-primary/20 transition-all duration-200" />
                                        <div className="relative flex items-center gap-3 w-full">
                                            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary/15 group-hover:bg-primary/25 group-aria-selected:bg-primary/30 transition-colors">
                                                <Building2 className="h-4 w-4 text-primary" />
                                            </div>
                                            <div className="flex flex-col flex-1 min-w-0">
                                                <span className="font-semibold text-foreground truncate">
                                                    {tenant.name}
                                                </span>
                                                <span className="text-xs text-muted-foreground truncate">
                                                    {tenant.slug}
                                                </span>
                                            </div>
                                        </div>
                                    </CommandItem>
                                ))}
                            </CommandGroup>
                            <CommandSeparator />
                        </>
                    )}

                    {/* Navegación Principal */}
                    <CommandGroup heading="Navegación Principal">
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin"))}
                            className="cursor-pointer"
                        >
                            <LayoutDashboard className="mr-2 h-4 w-4" />
                            <span>Dashboard</span>
                            <CommandShortcut>⌘D</CommandShortcut>
                        </CommandItem>
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/tenants"))}
                            className="cursor-pointer"
                        >
                            <Building2 className="mr-2 h-4 w-4" />
                            <span>Tenants</span>
                            <CommandShortcut>⌘T</CommandShortcut>
                        </CommandItem>
                        {currentTenantId && (
                            <CommandItem
                                onSelect={() => runCommand(() => router.push(`/admin/tenants/${currentTenantId}/users`))}
                                className="cursor-pointer"
                            >
                                <Users className="mr-2 h-4 w-4" />
                                <span>Users</span>
                                <CommandShortcut>⌘U</CommandShortcut>
                            </CommandItem>
                        )}
                        {currentTenantId && (
                            <CommandItem
                                onSelect={() => runCommand(() => router.push(`/admin/tenants/${currentTenantId}/clients`))}
                                className="cursor-pointer"
                            >
                                <Boxes className="mr-2 h-4 w-4" />
                                <span>Clients</span>
                            </CommandItem>
                        )}
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/keys"))}
                            className="cursor-pointer"
                        >
                            <Key className="mr-2 h-4 w-4" />
                            <span>Signing Keys</span>
                        </CommandItem>
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/logs"))}
                            className="cursor-pointer"
                        >
                            <FileText className="mr-2 h-4 w-4" />
                            <span>Logs</span>
                        </CommandItem>
                    </CommandGroup>

                    <CommandSeparator />

                    {/* Platform */}
                    <CommandGroup heading="Plataforma">
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/cluster"))}
                            className="cursor-pointer"
                        >
                            <Server className="mr-2 h-4 w-4" />
                            <span>Cluster Status</span>
                        </CommandItem>
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/metrics"))}
                            className="cursor-pointer"
                        >
                            <Activity className="mr-2 h-4 w-4" />
                            <span>Metrics</span>
                        </CommandItem>
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/oidc"))}
                            className="cursor-pointer"
                        >
                            <Globe2 className="mr-2 h-4 w-4" />
                            <span>OIDC Discovery</span>
                        </CommandItem>
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/playground"))}
                            className="cursor-pointer"
                        >
                            <Zap className="mr-2 h-4 w-4" />
                            <span>API Playground</span>
                        </CommandItem>
                    </CommandGroup>

                    {/* Tenant Context - Solo si hay tenant seleccionado */}
                    {currentTenantId && (
                        <>
                            <CommandSeparator />
                            <CommandGroup heading="Tenant Actual">
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/${currentTenantId}/detail`))}
                                    className="cursor-pointer"
                                >
                                    <LayoutDashboard className="mr-2 h-4 w-4" />
                                    <span>Tenant Details</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/${currentTenantId}/settings`))}
                                    className="cursor-pointer"
                                >
                                    <Settings className="mr-2 h-4 w-4" />
                                    <span>Tenant Settings</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/${currentTenantId}/sessions`))}
                                    className="cursor-pointer"
                                >
                                    <Shapes className="mr-2 h-4 w-4" />
                                    <span>Sessions</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/${currentTenantId}/rbac`))}
                                    className="cursor-pointer"
                                >
                                    <Shield className="mr-2 h-4 w-4" />
                                    <span>Roles & RBAC</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/${currentTenantId}/scopes`))}
                                    className="cursor-pointer"
                                >
                                    <Lock className="mr-2 h-4 w-4" />
                                    <span>Scopes</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/${currentTenantId}/tokens`))}
                                    className="cursor-pointer"
                                >
                                    <ListChecks className="mr-2 h-4 w-4" />
                                    <span>Tokens</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/${currentTenantId}/providers`))}
                                    className="cursor-pointer"
                                >
                                    <Globe2 className="mr-2 h-4 w-4" />
                                    <span>Social Providers</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/${currentTenantId}/mailing`))}
                                    className="cursor-pointer"
                                >
                                    <Mail className="mr-2 h-4 w-4" />
                                    <span>Mailing</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/${currentTenantId}/database`))}
                                    className="cursor-pointer"
                                >
                                    <Database className="mr-2 h-4 w-4" />
                                    <span>Storage</span>
                                </CommandItem>
                            </CommandGroup>
                        </>
                    )}

                    <CommandSeparator />

                    {/* Acciones Rápidas */}
                    <CommandGroup heading="Acciones Rápidas">
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/tenants?action=create"))}
                            className="cursor-pointer"
                        >
                            <Plus className="mr-2 h-4 w-4" />
                            <span>Crear Tenant</span>
                            <CommandShortcut>⌘N</CommandShortcut>
                        </CommandItem>
                        <CommandItem
                            onSelect={() => runCommand(() => toggleTheme())}
                            className="cursor-pointer"
                        >
                            {theme === "dark" ? (
                                <Sun className="mr-2 h-4 w-4" />
                            ) : (
                                <Moon className="mr-2 h-4 w-4" />
                            )}
                            <span>Alternar Tema</span>
                            <CommandShortcut>⌘J</CommandShortcut>
                        </CommandItem>
                    </CommandGroup>

                    <CommandSeparator />

                    {/* Cuenta */}
                    <CommandGroup heading="Cuenta">
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/profile"))}
                            className="cursor-pointer"
                        >
                            <User className="mr-2 h-4 w-4" />
                            <span>Mi Perfil</span>
                            <CommandShortcut>⌘P</CommandShortcut>
                        </CommandItem>
                        <CommandItem
                            onSelect={() => runCommand(() => {
                                clearAuth()
                                router.push("/login")
                            })}
                            className="text-destructive cursor-pointer"
                        >
                            <LogOut className="mr-2 h-4 w-4" />
                            <span>Cerrar Sesión</span>
                            <CommandShortcut>⌘Q</CommandShortcut>
                        </CommandItem>
                    </CommandGroup>
                </CommandList>
            </CommandDialog>

            <style jsx global>{`
                .scrollbar-custom::-webkit-scrollbar {
                    width: 8px;
                }
                
                .scrollbar-custom::-webkit-scrollbar-track {
                    background: transparent;
                    margin: 8px 0;
                }
                
                .scrollbar-custom::-webkit-scrollbar-thumb {
                    background: hsl(var(--muted-foreground) / 0.2);
                    border-radius: 10px;
                    transition: background 0.2s ease;
                }
                
                .scrollbar-custom::-webkit-scrollbar-thumb:hover {
                    background: hsl(var(--muted-foreground) / 0.35);
                }
            `}</style>
        </>
    )
}
