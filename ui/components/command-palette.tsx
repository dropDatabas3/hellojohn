"use client"

import * as React from "react"
import { useRouter, useSearchParams } from "next/navigation"
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
} from "@/components/ui/command"
import { useAuthStore } from "@/lib/auth-store"
import { useUIStore } from "@/lib/ui-store"
import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import type { Tenant } from "@/lib/types"

export function CommandPalette() {
    const [open, setOpen] = React.useState(false)
    const [search, setSearch] = React.useState("")
    const router = useRouter()
    const clearAuth = useAuthStore((state) => state.clearAuth)
    const toggleTheme = useUIStore((state) => state.toggleTheme)
    const theme = useUIStore((state) => state.theme)
    const searchParams = useSearchParams()
    const currentTenantId = searchParams.get("id") || searchParams.get("tenant")

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
                className="inline-flex items-center rounded-lg font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 border border-input bg-background/50 backdrop-blur-sm shadow-sm hover:bg-accent hover:text-accent-foreground hover:shadow-md h-9 px-4 py-2 relative w-full justify-start text-sm text-muted-foreground sm:pr-12 md:w-40 lg:w-80"
            >
                <Search className="mr-2 h-4 w-4 shrink-0" />
                <span className="hidden lg:inline-flex">Buscar comandos...</span>
                <span className="inline-flex lg:hidden">Buscar...</span>
                <kbd className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 hidden h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium opacity-100 sm:flex">
                    <span className="text-xs">⌘</span>K
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
                                            runCommand(() => router.push(`/admin/tenants/detail?id=${tenant.id}`))
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
                                onSelect={() => runCommand(() => router.push(`/admin/users?id=${currentTenantId}`))}
                                className="cursor-pointer"
                            >
                                <Users className="mr-2 h-4 w-4" />
                                <span>Users</span>
                                <CommandShortcut>⌘U</CommandShortcut>
                            </CommandItem>
                        )}
                        {currentTenantId && (
                            <CommandItem
                                onSelect={() => runCommand(() => router.push(`/admin/tenants/clients?id=${currentTenantId}`))}
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
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/detail?id=${currentTenantId}`))}
                                    className="cursor-pointer"
                                >
                                    <LayoutDashboard className="mr-2 h-4 w-4" />
                                    <span>Tenant Details</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/settings?id=${currentTenantId}`))}
                                    className="cursor-pointer"
                                >
                                    <Settings className="mr-2 h-4 w-4" />
                                    <span>Tenant Settings</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/sessions?id=${currentTenantId}`))}
                                    className="cursor-pointer"
                                >
                                    <Shapes className="mr-2 h-4 w-4" />
                                    <span>Sessions</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/rbac?id=${currentTenantId}`))}
                                    className="cursor-pointer"
                                >
                                    <Shield className="mr-2 h-4 w-4" />
                                    <span>Roles & RBAC</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/scopes?id=${currentTenantId}`))}
                                    className="cursor-pointer"
                                >
                                    <Lock className="mr-2 h-4 w-4" />
                                    <span>Scopes</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/tokens?id=${currentTenantId}`))}
                                    className="cursor-pointer"
                                >
                                    <ListChecks className="mr-2 h-4 w-4" />
                                    <span>Tokens</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/providers?id=${currentTenantId}`))}
                                    className="cursor-pointer"
                                >
                                    <Globe2 className="mr-2 h-4 w-4" />
                                    <span>Social Providers</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/mailing?id=${currentTenantId}`))}
                                    className="cursor-pointer"
                                >
                                    <Mail className="mr-2 h-4 w-4" />
                                    <span>Mailing</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/tenants/forms?id=${currentTenantId}`))}
                                    className="cursor-pointer"
                                >
                                    <LayoutTemplate className="mr-2 h-4 w-4" />
                                    <span>Forms</span>
                                </CommandItem>
                                <CommandItem
                                    onSelect={() => runCommand(() => router.push(`/admin/database?id=${currentTenantId}`))}
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
