"use client"

import { useParams, useRouter, useSearchParams } from "next/navigation"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import { useAuthStore } from "@/lib/auth-store"

import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
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
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import {
  Loader2,
  MoreHorizontal,
  Plus,
  Search,
  Trash2,
  Eye,
  User as UserIcon,
  Copy,
  Ban,
  Unlock,
  Clock
} from "lucide-react"

import { useToast } from "@/hooks/use-toast"
import { User, UserFieldDefinition } from "@/lib/types"
import { PhoneInput } from "@/components/ui/phone-input"
import { CountrySelect } from "@/components/ui/country-select"

export default function UsersClientPage() {
  const params = useParams()
  const searchParams = useSearchParams()
  const tenantId = (params?.id as string) || searchParams.get("id") || ""

  const router = useRouter()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const token = useAuthStore((s) => s.token)

  const [search, setSearch] = useState("")
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [isDetailsOpen, setIsDetailsOpen] = useState(false)
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [blockUser, setBlockUser] = useState<User | null>(null)

  // 1. Fetch Users
  const { data: users, isLoading } = useQuery<User[]>({
    queryKey: ["users", tenantId],
    queryFn: async () => {
      if (!tenantId) return []
      const res = await fetch(`/v1/admin/tenants/${tenantId}/users`, {
        headers: {
          "Authorization": `Bearer ${token}`
        }
      })
      if (!res.ok) throw new Error("Error fetching users")
      return res.json()
    },
    enabled: !!tenantId && !!token,
  })

  // 2. Fetch Field Definitions
  const { data: fieldDefs } = useQuery<UserFieldDefinition[]>({
    queryKey: ["user-fields", tenantId],
    queryFn: async () => {
      if (!tenantId) return []
      const res = await fetch(`/v1/admin/tenants/${tenantId}`, {
        headers: {
          "Authorization": `Bearer ${token}`
        }
      })
      if (!res.ok) return []
      const tenant = await res.json()
      return tenant.settings?.user_fields || []
    },
    enabled: !!tenantId && !!token,
  })

  // 3. Create User Mutation
  const createMutation = useMutation({
    mutationFn: async (vars: any) => {
      const res = await fetch(`/v1/admin/tenants/${tenantId}/users`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${token}`
        },
        body: JSON.stringify(vars),
      })
      if (!res.ok) {
        const err = await res.json()
        throw new Error(err.error_description || "Error creating user")
      }
      return res.json()
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
      setIsCreateOpen(false)
      toast({ title: "Usuario creado", description: "El usuario ha sido creado exitosamente." })
    },
    onError: (err: Error) => {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    },
  })

  // 4. Delete User Mutation
  const deleteMutation = useMutation({
    mutationFn: async (userId: string) => {
      const res = await fetch(`/v1/admin/tenants/${tenantId}/users/${userId}`, {
        method: "DELETE",
        headers: {
          "Authorization": `Bearer ${token}`
        },
      })
      if (!res.ok) throw new Error("Error deleting user")
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
      toast({ title: "Usuario eliminado", description: "El usuario ha sido eliminado." })
    },
    onError: (err: Error) => {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    },
  })

  // 5. Block User Mutation
  const blockMutation = useMutation({
    mutationFn: async ({ userId, reason, duration }: { userId: string, reason: string, duration: string }) => {
      const res = await fetch(`/v1/admin/users/disable`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${token}`
        },
        body: JSON.stringify({ user_id: userId, tenant_id: tenantId, reason, duration }),
      })
      if (!res.ok) {
        const err = await res.json()
        throw new Error(err.error_description || "Error blocking user")
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
      setBlockUser(null)
      toast({ title: "Usuario bloqueado", description: "El usuario ha sido bloqueado exitosamente." })
    },
    onError: (err: Error) => {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    }
  })

  // 6. Enable User Mutation
  const enableMutation = useMutation({
    mutationFn: async (userId: string) => {
      const res = await fetch(`/v1/admin/users/enable`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${token}`
        },
        body: JSON.stringify({ user_id: userId, tenant_id: tenantId }),
      })
      if (!res.ok) throw new Error("Error enabling user")
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", tenantId] })
      toast({ title: "Usuario desbloqueado", description: "El usuario ha sido habilitado nuevamente." })
    },
    onError: (err: Error) => {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    }
  })


  const filteredUsers = users?.filter(
    (user) =>
      (user.email || "").toLowerCase().includes(search.toLowerCase()) ||
      (user.id || "").toLowerCase().includes(search.toLowerCase())
  )

  const handleDetails = (user: User) => {
    setSelectedUser(user)
    setIsDetailsOpen(true)
  }

  if (!tenantId) {
    return (
      <div className="p-10 flex flex-col items-center justify-center text-center space-y-4">
        <div className="p-4 rounded-full bg-red-100 text-red-600">
          <UserIcon className="h-8 w-8" />
        </div>
        <h2 className="text-xl font-semibold">Tenant ID no encontrado</h2>
        <p className="text-muted-foreground">No se pudo identificar el tenant actual. Verifica la URL.</p>
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6 pb-20">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">Usuarios</h2>
          <p className="text-muted-foreground">Gestiona los usuarios de este tenant.</p>
        </div>
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <DialogTrigger asChild>
            <Button><Plus className="mr-2 h-4 w-4" /> Crear Usuario</Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto">
            <DialogHeader>
              <DialogTitle>Crear Nuevo Usuario</DialogTitle>
              <DialogDescription>
                Ingresa los datos básicos y campos personalizados.
              </DialogDescription>
            </DialogHeader>
            <CreateUserForm
              fieldDefs={fieldDefs || []}
              onSubmit={(data) => createMutation.mutate(data)}
              isPending={createMutation.isPending}
            />
          </DialogContent>
        </Dialog>
      </div>

      <div className="flex items-center space-x-2">
        <div className="relative">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Buscar por email o ID..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9 h-9 w-[150px] lg:w-[300px]"
          />
        </div>
      </div>

      <div className="rounded-md border bg-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[50px]"></TableHead>
              <TableHead>Identidad</TableHead>
              <TableHead>Estado</TableHead>
              <TableHead className="hidden md:table-cell">Creado</TableHead>
              <TableHead className="text-right">Acciones</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={5} className="h-24 text-center">
                  <Loader2 className="mr-2 h-4 w-4 animate-spin inline" /> Cargando...
                </TableCell>
              </TableRow>
            ) : filteredUsers?.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="h-24 text-center text-muted-foreground">
                  No se encontraron usuarios.
                </TableCell>
              </TableRow>
            ) : (
              filteredUsers?.map((user) => (
                <UserRow
                  key={user.id}
                  user={user}
                  onDelete={() => deleteMutation.mutate(user.id)}
                  onDetails={() => handleDetails(user)}
                  onBlock={() => { console.log("user", user); setBlockUser(user) }}
                  onUnlock={() => enableMutation.mutate(user.id)}
                />
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={isDetailsOpen} onOpenChange={setIsDetailsOpen}>
        <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Detalles del Usuario</DialogTitle>
            <DialogDescription>Información completa registrada.</DialogDescription>
          </DialogHeader>
          {selectedUser && <UserDetails user={selectedUser} tenantId={tenantId} token={token} />}
        </DialogContent>
      </Dialog>

      {blockUser && (
        <BlockUserDialog
          user={blockUser}
          onClose={() => setBlockUser(null)}
          onBlock={(userId, reason, duration) => {
            if (!userId) {
              toast({ title: "Error", description: "No se pudo identificar el usuario (ID faltante)", variant: "destructive" })
              return
            }
            blockMutation.mutate({ userId, reason, duration })
          }}
          isPending={blockMutation.isPending}
        />
      )}
    </div>
  )
}

function UserRow({ user, onDelete, onDetails, onBlock, onUnlock }: { user: User, onDelete: () => void, onDetails: () => void, onBlock: () => void, onUnlock: () => void }) {
  let dateStr = "-"
  try {
    if (user.created_at) {
      const d = new Date(user.created_at)
      if (!isNaN(d.getTime())) {
        dateStr = new Intl.DateTimeFormat("es-ES", { dateStyle: "medium" }).format(d)
      }
    }
  } catch (e) { }

  const initial = user.email ? user.email.slice(0, 2).toUpperCase() : "??"
  const isBlocked = !!user.disabled_at
  const isSuspended = !!user.disabled_until && new Date(user.disabled_until) > new Date()
  const displayBlocked = isBlocked || isSuspended

  return (
    <TableRow>
      <TableCell>
        <div className="h-9 w-9 bg-muted rounded-full flex items-center justify-center font-semibold text-xs text-muted-foreground">
          {initial}
        </div>
      </TableCell>
      <TableCell>
        <div className="flex flex-col">
          <span className="font-medium truncate max-w-[200px]">{user.email}</span>
          <span className="text-xs text-muted-foreground font-mono truncate max-w-[150px] opacity-70 cursor-pointer" title={user.id}>{user.id}</span>
        </div>
      </TableCell>
      <TableCell>
        <div className="flex flex-col gap-1 items-start">
          {displayBlocked ? (
            <div className="flex flex-col gap-0.5">
              <Badge variant="destructive" className="h-5 text-[10px] px-1.5" >
                {isSuspended ? "Suspendido" : "Deshabilitado"}
              </Badge>
              {isSuspended && user.disabled_until && (
                <span className="text-[9px] text-red-500 font-mono">
                  Hasta: {new Date(user.disabled_until).toLocaleString("es-ES", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}
                </span>
              )}
            </div>
          ) : (
            <Badge variant="default" className="h-5 text-[10px] px-1.5 bg-green-600 hover:bg-green-700">Activo</Badge>
          )}
          {user.email_verified ? (
            <span className="text-[10px] text-green-600 font-medium flex items-center">Ok Verificado</span>
          ) : (
            <span className="text-[10px] text-amber-600 font-medium flex items-center">No Verificado</span>
          )}
        </div>
      </TableCell>
      <TableCell className="hidden md:table-cell text-sm text-muted-foreground">
        {dateStr}
      </TableCell>
      <TableCell className="text-right">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" className="h-8 w-8 p-0">
              <span className="sr-only">Abrir menú</span>
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>Acciones</DropdownMenuLabel>
            <DropdownMenuItem onClick={onDetails}>
              <Eye className="mr-2 h-4 w-4" /> Ver Detalles
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => {
              if (user.id) navigator.clipboard.writeText(user.id)
            }}>
              <Copy className="mr-2 h-4 w-4" /> Copiar ID
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            {displayBlocked ? (
              <DropdownMenuItem onClick={onUnlock}>
                <Unlock className="mr-2 h-4 w-4 text-green-600" /> Desbloquear
              </DropdownMenuItem>
            ) : (
              <DropdownMenuItem onClick={onBlock}>
                <Ban className="mr-2 h-4 w-4 text-orange-600" /> Bloquear
              </DropdownMenuItem>
            )}
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={onDelete} className="text-red-600 focus:text-red-600 focus:bg-red-50">
              <Trash2 className="mr-2 h-4 w-4" /> Eliminar usuario
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </TableCell>
    </TableRow>
  )
}

function UserDetails({ user, tenantId, token }: { user: User, tenantId: string, token: string | null }) {
  const { toast } = useToast()
  const [isResending, setIsResending] = useState(false)

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return "-"
    try {
      const d = new Date(dateStr)
      if (isNaN(d.getTime())) return "-"
      return new Intl.DateTimeFormat("es-ES", {
        year: "numeric",
        month: "long",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit"
      }).format(d)
    } catch { return "-" }
  }

  const createdAt = formatDate(user.created_at)
  const updatedAt = formatDate(user.updated_at || user.custom_fields?.updated_at)
  const initial = user.email ? user.email.slice(0, 2).toUpperCase() : "??"

  const handleResendVerification = async () => {
    if (!user.id || !tenantId) {
      toast({ title: "Error", description: "Faltan datos del usuario o tenant", variant: "destructive" })
      return
    }
    setIsResending(true)
    try {
      const res = await fetch(`/v1/admin/users/resend-verification`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${token}`
        },
        body: JSON.stringify({ user_id: user.id, tenant_id: tenantId })
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error_description || err.error || "Error enviando email")
      }
      toast({ title: "Email enviado", description: "Se ha enviado un nuevo correo de verificación al usuario." })
    } catch (err: any) {
      toast({ title: "Error", description: err.message, variant: "destructive" })
    } finally {
      setIsResending(false)
    }
  }

  return (
    <div className="mt-4 space-y-6">
      <div className="flex flex-col items-center justify-center space-y-3 bg-muted/20 p-6 rounded-lg border border-dashed">
        <div className="h-20 w-20 bg-background border-4 border-muted rounded-full flex items-center justify-center text-3xl font-bold text-muted-foreground shadow-sm">
          {initial}
        </div>
        <div className="text-center">
          <h3 className="text-xl font-bold tracking-tight">{user.email}</h3>
          <div className="flex items-center justify-center gap-2 mt-1">
            <Badge variant="outline" className="font-mono text-xs">{user.id}</Badge>
            <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => navigator.clipboard.writeText(user.id)}>
              <Copy className="h-3 w-3" />
            </Button>
          </div>
        </div>
      </div>

      <div className="space-y-4">
        <h4 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider border-b pb-2">Información de Sistema</h4>
        <div className="grid grid-cols-2 gap-4 text-sm">
          <div className="space-y-1">
            <span className="block text-xs text-muted-foreground">Estado de Cuenta</span>
            {user.disabled_at ? (
              <span className="inline-flex items-center px-2 py-1 rounded-full bg-red-100 text-red-700 text-xs font-medium">Deshabilitado</span>
            ) : (
              <span className="inline-flex items-center px-2 py-1 rounded-full bg-green-100 text-green-700 text-xs font-medium">Activa</span>
            )}
          </div>
          <div className="space-y-1">
            <span className="block text-xs text-muted-foreground">Verificación de Email</span>
            <div className="flex items-center gap-2">
              <span className="font-medium">{user.email_verified ? "Verificado" : "Pendiente"}</span>
              {!user.email_verified && (
                <Button
                  variant="link"
                  className="h-auto p-0 text-xs text-blue-600"
                  onClick={handleResendVerification}
                  disabled={isResending}
                >
                  {isResending ? <Loader2 className="h-3 w-3 mr-1 animate-spin" /> : null}
                  Reenviar código
                </Button>
              )}
            </div>
          </div>
          <div className="space-y-1">
            <span className="block text-xs text-muted-foreground">Fecha de Creación</span>
            <span className="font-medium">{createdAt}</span>
          </div>
          <div className="space-y-1">
            <span className="block text-xs text-muted-foreground">Última Actualización</span>
            <span className="font-medium">{updatedAt}</span>
          </div>
          {user.disabled_reason && (
            <div className="space-y-1 col-span-2">
              <span className="block text-xs text-muted-foreground">Razón de Bloqueo</span>
              <span className="font-medium text-red-600">{user.disabled_reason}</span>
            </div>
          )}
        </div>
      </div>

      {user.custom_fields && Object.keys(user.custom_fields).length > 0 && (
        <div className="space-y-4">
          <h4 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider border-b pb-2">Campos Personalizados</h4>
          <div className="grid gap-3">
            {Object.entries(user.custom_fields)
              .filter(([key]) => key !== 'updated_at')
              .map(([key, value]) => (
                <div key={key} className="flex flex-col gap-1 rounded-md border p-3 bg-muted/10 hover:bg-muted/20 transition-colors">
                  <span className="text-xs font-bold uppercase text-muted-foreground/70">{key}</span>
                  <span className="text-sm font-medium break-all">
                    {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                  </span>
                </div>
              ))}
          </div>
        </div>
      )}

      {user.metadata && Object.keys(user.metadata).length > 0 && (
        <div className="space-y-4">
          <h4 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider border-b pb-2">Metadatos / Perfil</h4>
          <div className="bg-muted p-4 rounded-lg border overflow-x-auto">
            <pre className="text-xs font-mono">
              {JSON.stringify(user.metadata, null, 2)}
            </pre>
          </div>
        </div>
      )}
    </div>
  )
}


function BlockUserDialog({ user, onClose, onBlock, isPending }: { user: User, onClose: () => void, onBlock: (userId: string, reason: string, duration: string) => void, isPending: boolean }) {
  const [reason, setReason] = useState("")
  const [duration, setDuration] = useState("permanent")

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Bloquear Usuario</DialogTitle>
          <DialogDescription>
            Suspenda el acceso de {user.email}.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 py-4">
          <div className="grid gap-2">
            <Label>Motivo</Label>
            <Input
              placeholder="Ej. Violación de términos"
              value={reason}
              onChange={e => setReason(e.target.value)}
            />
          </div>
          <div className="grid gap-2">
            <Label>Duración</Label>
            <Select value={duration} onValueChange={setDuration}>
              <SelectTrigger>
                <SelectValue placeholder="Selecciona duración" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="permanent">Permanente</SelectItem>
                <SelectItem value="1h">1 hora</SelectItem>
                <SelectItem value="6h">6 horas</SelectItem>
                <SelectItem value="12h">12 horas</SelectItem>
                <SelectItem value="24h">24 horas</SelectItem>
                <SelectItem value="72h">3 días</SelectItem>
                <SelectItem value="168h">7 días</SelectItem>
                <SelectItem value="720h">30 días</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isPending}>Cancelar</Button>
          <Button
            variant="destructive"
            onClick={() => { onBlock(user.id, reason, duration === "permanent" ? "" : duration) }}
            disabled={isPending || !reason.trim()}
          >
            {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Bloquear
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}


function CreateUserForm({ fieldDefs, onSubmit, isPending }: {
  fieldDefs: UserFieldDefinition[],
  onSubmit: (data: any) => void,
  isPending: boolean
}) {
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [customFields, setCustomFields] = useState<Record<string, any>>({})

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSubmit({ email, password, email_verified: true, custom_fields: customFields })
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6 py-2">
      <div className="grid gap-4">
        <div className="grid gap-2">
          <Label htmlFor="email">Email</Label>
          <Input
            id="email"
            type="email"
            placeholder="usuario@ejemplo.com"
            value={email}
            onChange={e => setEmail(e.target.value)}
            required
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="password">Contraseña Temporal</Label>
          <Input
            id="password"
            type="password"
            placeholder="••••••••"
            value={password}
            onChange={e => setPassword(e.target.value)}
            required
          />
        </div>
      </div>

      {fieldDefs.length > 0 && (
        <div className="space-y-4 pt-2">
          <div className="flex items-center">
            <div className="h-px flex-1 bg-border" />
            <span className="px-2 text-xs font-semibold text-muted-foreground uppercase">Información Adicional</span>
            <div className="h-px flex-1 bg-border" />
          </div>

          {fieldDefs.map((field) => (
            <div key={field.name} className="grid gap-2">
              <Label htmlFor={field.name}>{field.name} {field.required && <span className="text-red-500">*</span>}</Label>
              {field.type === "phone" ? (
                <PhoneInput
                  value={customFields[field.name] as string}
                  onChange={(val) => setCustomFields({ ...customFields, [field.name]: val })}
                />
              ) : field.type === "country" ? (
                <CountrySelect
                  value={customFields[field.name] as string}
                  onChange={(val) => setCustomFields({ ...customFields, [field.name]: val })}
                />
              ) : (
                <Input
                  id={field.name}
                  type={field.type === "number" || field.type === "int" ? "number" : "text"}
                  value={customFields[field.name] || ""}
                  onChange={(e) => setCustomFields({ ...customFields, [field.name]: e.target.value })}
                  required={field.required}
                  placeholder={`Ingresa ${field.name}`}
                />
              )}
            </div>
          ))}
        </div>
      )}

      <DialogFooter className="pt-4">
        <Button type="submit" disabled={isPending} className="w-full sm:w-auto">
          {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          Crear Usuario
        </Button>
      </DialogFooter>
    </form>
  )
}
