"use client"

import { useParams, useSearchParams } from "next/navigation"
import { useState } from "react"
import { useMutation, useQuery } from "@tanstack/react-query"
import { Clock3, Search, Trash2 } from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { useToast } from "@/hooks/use-toast"

type ConsentRow = {
  id: string
  user_id: string
  clientIdText: string
  granted_scopes: string[]
  granted_at: string
  revoked_at?: string | null
  tenantId: string
}

export default function SessionsClientPage() {
  const params = useParams()
  const searchParams = useSearchParams()
  const { toast } = useToast()
  const tenantId = (params.id as string) || (searchParams.get("id") as string)
  const { t } = useI18n()
  const [userId, setUserId] = useState("")
  const [query, setQuery] = useState("")

  const { data, isFetching, refetch } = useQuery({
    queryKey: ["admin-consents-by-user", userId],
    enabled: false,
    queryFn: async () => {
      return api.get<ConsentRow[]>(`/v1/admin/consents/by-user?user_id=${encodeURIComponent(userId)}&active_only=false`)
    },
  })

  const revoke = useMutation({
    mutationFn: (clientID: string) => api.delete(`/v1/admin/consents/${userId}/${encodeURIComponent(clientID)}`),
    onSuccess: () => {
      toast({ title: t("common.done"), description: t("sessions.revoked") })
      refetch()
    },
    onError: (e: any) => toast({ title: t("common.error"), description: e.message, variant: "destructive" }),
  })

  const rows = (data || []).filter((c) => {
    const q = query.toLowerCase()
    return (
      c.clientIdText.toLowerCase().includes(q) ||
      c.granted_scopes.some((s) => s.toLowerCase().includes(q))
    )
  })

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Clock3 className="h-7 w-7 text-muted-foreground" />
        <div>
          <h1 className="text-3xl font-bold">{t("sessions.title")}</h1>
          <p className="text-muted-foreground">{t("sessions.description")}</p>
        </div>
      </div>

      <Card className="p-6">
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
            <Button onClick={() => refetch()} disabled={!userId || isFetching}>
              {isFetching ? t("common.loading") : t("common.load")}
            </Button>
          </div>
        </div>

        <div className="mb-4">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder={t("sessions.search")}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="pl-9"
            />
          </div>
        </div>

        {!data ? (
          <div className="py-8 text-center text-muted-foreground">{t("sessions.enterUserId")}</div>
        ) : data.length === 0 ? (
          <div className="py-8 text-center text-muted-foreground">{t("sessions.empty")}</div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("sessions.client")}</TableHead>
                <TableHead>{t("sessions.scopes")}</TableHead>
                <TableHead>{t("sessions.createdAt")}</TableHead>
                <TableHead>{t("sessions.state")}</TableHead>
                <TableHead className="text-right">{t("common.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((c) => {
                const active = !c.revoked_at
                return (
                  <TableRow key={c.id}>
                    <TableCell>
                      <code className="rounded bg-muted px-2 py-1 text-xs">{c.clientIdText}</code>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {c.granted_scopes.map((s) => (
                          <Badge key={s} variant="secondary">
                            {s}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm">{new Date(c.granted_at).toLocaleString()}</TableCell>
                    <TableCell>
                      <Badge variant={active ? "default" : "secondary"}>
                        {active ? t("sessions.active") : t("sessions.revoked")}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        title={t("sessions.revokeConsent")}
                        onClick={() => revoke.mutate(c.clientIdText)}
                        disabled={!active}
                      >
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        )}
      </Card>
    </div>
  )
}
