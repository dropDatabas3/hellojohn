"use client"

import * as React from "react"
import { useRouter } from "next/navigation"
import {
    Calculator,
    Calendar,
    CreditCard,
    Settings,
    Smile,
    User,
    Users,
    Building2,
    Key,
    Shield,
    FileText,
    LogOut,
    Plus,
    Search
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

export function CommandPalette() {
    const [open, setOpen] = React.useState(false)
    const router = useRouter()
    const clearAuth = useAuthStore((state) => state.clearAuth)
    const toggleTheme = useUIStore((state) => state.toggleTheme)

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
        command()
    }, [])

    return (
        <>
            <button
                onClick={() => setOpen(true)}
                className="inline-flex items-center rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 border border-input bg-transparent shadow-sm hover:bg-accent hover:text-accent-foreground h-9 px-4 py-2 relative w-full justify-start text-sm text-muted-foreground sm:pr-12 md:w-40 lg:w-64"
            >
                <span className="hidden lg:inline-flex">Buscar...</span>
                <span className="inline-flex lg:hidden">Buscar...</span>
                <kbd className="pointer-events-none absolute right-1.5 top-1.5 hidden h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium opacity-100 sm:flex">
                    <span className="text-xs">⌘</span>K
                </kbd>
            </button>
            <CommandDialog open={open} onOpenChange={setOpen}>
                <CommandInput placeholder="Escribe un comando o busca..." />
                <CommandList>
                    <CommandEmpty>No se encontraron resultados.</CommandEmpty>
                    <CommandGroup heading="Navegación">
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/tenants"))}
                        >
                            <Building2 className="mr-2 h-4 w-4" />
                            <span>Organizaciones</span>
                        </CommandItem>
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/logs"))}
                        >
                            <FileText className="mr-2 h-4 w-4" />
                            <span>Logs del Sistema</span>
                        </CommandItem>
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/config"))}
                        >
                            <Settings className="mr-2 h-4 w-4" />
                            <span>Configuración</span>
                        </CommandItem>
                    </CommandGroup>
                    <CommandSeparator />
                    <CommandGroup heading="Acciones Rápidas">
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/tenants?action=create"))}
                        >
                            <Plus className="mr-2 h-4 w-4" />
                            <span>Crear Organización</span>
                        </CommandItem>
                        <CommandItem
                            onSelect={() => runCommand(() => toggleTheme())}
                        >
                            <Settings className="mr-2 h-4 w-4" />
                            <span>Alternar Tema (Claro/Oscuro)</span>
                        </CommandItem>
                    </CommandGroup>
                    <CommandSeparator />
                    <CommandGroup heading="Cuenta">
                        <CommandItem
                            onSelect={() => runCommand(() => router.push("/admin/profile"))}
                        >
                            <User className="mr-2 h-4 w-4" />
                            <span>Perfil</span>
                            <CommandShortcut>⌘P</CommandShortcut>
                        </CommandItem>
                        <CommandItem
                            onSelect={() => runCommand(() => {
                                clearAuth()
                                router.push("/login")
                            })}
                        >
                            <LogOut className="mr-2 h-4 w-4" />
                            <span>Cerrar Sesión</span>
                        </CommandItem>
                    </CommandGroup>
                </CommandList>
            </CommandDialog>
        </>
    )
}
