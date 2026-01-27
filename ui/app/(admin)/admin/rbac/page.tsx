"use client"

import { useState } from "react"
import { useQuery, useMutation } from "@tanstack/react-query"
import { Search, Shield, Trash2, Plus } from "lucide-react"
import { api } from "@/lib/api"
import { API_ROUTES } from "@/lib/routes"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { useToast } from "@/hooks/use-toast"

type UserRolesResponse = { user_id: string; roles: string[] }
type RolePermsResponse = { tenant_id: string; role: string; perms: string[] }

export default function RBACPage() {
  const { t } = useI18n()
  const { toast } = useToast()

  // User -> Roles panel
  const [userId, setUserId] = useState("")
  const [newUserRole, setNewUserRole] = useState("")
  const {
    data: userRoles,
    refetch: refetchUserRoles,
    isFetching: loadingUserRoles,
  } = useQuery({
    queryKey: ["rbac-user-roles", userId],
    enabled: false,
    queryFn: () => api.get<UserRolesResponse>(API_ROUTES.ADMIN_RBAC_USER_ROLES(userId)),
  })

  const addUserRole = useMutation({
    mutationFn: (role: string) =>
      api.post<UserRolesResponse>(API_ROUTES.ADMIN_RBAC_USER_ROLES(userId), { add: [role], remove: [] }),
    onSuccess: (res) => {
      setNewUserRole("")
      toast({ title: t("common.saved"), description: t("rbac.userRolesUpdated") })
    },
    onError: (e: any) => toast({ title: t("common.error"), description: e.message, variant: "destructive" }),
  })

  const removeUserRole = useMutation({
    mutationFn: (role: string) =>
      api.post<UserRolesResponse>(API_ROUTES.ADMIN_RBAC_USER_ROLES(userId), { add: [], remove: [role] }),
    onSuccess: () => toast({ title: t("common.saved"), description: t("rbac.userRolesUpdated") }),
    onError: (e: any) => toast({ title: t("common.error"), description: e.message, variant: "destructive" }),
  })

  // Role -> Perms panel
  const [roleName, setRoleName] = useState("")
  const [newPerm, setNewPerm] = useState("")
  const {
    data: rolePerms,
    refetch: refetchRolePerms,
    isFetching: loadingRolePerms,
  } = useQuery({
    queryKey: ["rbac-role-perms", roleName],
    enabled: false,
    queryFn: () => api.get<RolePermsResponse>(API_ROUTES.ADMIN_RBAC_ROLE_PERMS(roleName)),
  })

  const addPerm = useMutation({
    mutationFn: (perm: string) =>
      api.post<RolePermsResponse>(API_ROUTES.ADMIN_RBAC_ROLE_PERMS(roleName), {
        add: [perm],
        remove: [],
      }),
    onSuccess: () => toast({ title: t("common.saved"), description: t("rbac.rolePermsUpdated") }),
    onError: (e: any) => toast({ title: t("common.error"), description: e.message, variant: "destructive" }),
  })

  const removePerm = useMutation({
    mutationFn: (perm: string) =>
      api.post<RolePermsResponse>(API_ROUTES.ADMIN_RBAC_ROLE_PERMS(roleName), {
        add: [],
        remove: [perm],
      }),
    onSuccess: () => toast({ title: t("common.saved"), description: t("rbac.rolePermsUpdated") }),
    onError: (e: any) => toast({ title: t("common.error"), description: e.message, variant: "destructive" }),
  })

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold">{t("rbac.title")}</h1>
        <p className="text-muted-foreground">{t("rbac.description")}</p>
      </div>

      {/* User -> Roles */}
      <Card className="p-6">
        <h2 className="mb-4 text-xl font-semibold">User roles</h2>
        <div className="mb-4 grid grid-cols-1 gap-3 md:grid-cols-3">
          <div className="col-span-2">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="user_id (UUID)"
                value={userId}
                onChange={(e) => setUserId(e.target.value.trim())}
                className="pl-9"
              />
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button onClick={() => refetchUserRoles()} disabled={!userId || loadingUserRoles}>
              {loadingUserRoles ? t("common.loading") : t("common.load")}
            </Button>
          </div>
        </div>

        {userRoles?.roles && (
          <div className="space-y-3">
            <div className="flex flex-wrap gap-2">
              {userRoles.roles.map((r) => (
                <Badge key={r} variant="secondary" className="flex items-center gap-2">
                  <Shield className="h-3 w-3" /> {r}
                  <button
                    className="text-destructive"
                    onClick={() => removeUserRole.mutate(r)}
                    title={t("common.remove")}
                  >
                    <Trash2 className="h-3 w-3" />
                  </button>
                </Badge>
              ))}
              {userRoles.roles.length === 0 && (
                <span className="text-sm text-muted-foreground">No roles</span>
              )}
            </div>
            <div className="flex gap-2">
              <Input
                placeholder="role to add (e.g. admin)"
                value={newUserRole}
                onChange={(e) => setNewUserRole(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && newUserRole) {
                    e.preventDefault()
                    addUserRole.mutate(newUserRole)
                  }
                }}
              />
              <Button onClick={() => newUserRole && addUserRole.mutate(newUserRole)} disabled={!newUserRole}>
                <Plus className="mr-2 h-4 w-4" /> {t("common.add")}
              </Button>
            </div>
          </div>
        )}
      </Card>

      {/* Role -> Perms */}
      <Card className="p-6">
        <h2 className="mb-4 text-xl font-semibold">Role permissions</h2>
        <div className="mb-4 grid grid-cols-1 gap-3 md:grid-cols-3">
          <div className="col-span-2">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="role (e.g. admin)"
                value={roleName}
                onChange={(e) => setRoleName(e.target.value.trim())}
                className="pl-9"
              />
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button onClick={() => refetchRolePerms()} disabled={!roleName || loadingRolePerms}>
              {loadingRolePerms ? t("common.loading") : t("common.load")}
            </Button>
          </div>
        </div>

        {rolePerms?.perms && (
          <div className="space-y-3">
            <div className="flex flex-wrap gap-2">
              {rolePerms.perms.map((p) => (
                <Badge key={p} variant="secondary" className="flex items-center gap-2">
                  {p}
                  <button className="text-destructive" onClick={() => removePerm.mutate(p)} title={t("common.remove")}>
                    <Trash2 className="h-3 w-3" />
                  </button>
                </Badge>
              ))}
              {rolePerms.perms.length === 0 && (
                <span className="text-sm text-muted-foreground">No permissions</span>
              )}
            </div>
            <div className="flex gap-2">
              <Input
                placeholder="permission to add (e.g. rbac:write)"
                value={newPerm}
                onChange={(e) => setNewPerm(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && newPerm) {
                    e.preventDefault()
                    addPerm.mutate(newPerm)
                  }
                }}
              />
              <Button onClick={() => newPerm && addPerm.mutate(newPerm)} disabled={!newPerm}>
                <Plus className="mr-2 h-4 w-4" /> {t("common.add")}
              </Button>
            </div>
          </div>
        )}
      </Card>
    </div>
  )
}
