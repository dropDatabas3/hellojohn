"use client"

import { useParams, useSearchParams } from "next/navigation"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useI18n } from "@/lib/i18n"
import { api } from "@/lib/api"
import { API_ROUTES } from "@/lib/routes"
import { Card } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from "@/components/ui/table"
import { Trash2, RefreshCw } from "lucide-react"
import { Input } from "@/components/ui/input"
import { useState, useMemo } from "react"

type ConsentRow = {
  id?: string
  user_id?: string
  user_email?: string
  client_id?: string
  client_name?: string
  scopes?: string[]
  created_at?: string
}

import { Suspense } from "react"

function ConsentsContent() {
  const params = useParams()
  const search = useSearchParams()
  const { t } = useI18n()
  const queryClient = useQueryClient()
  const tenantId = (params.id as string) || (search.get("id") as string)
  const [q, setQ] = useState("")

  const { data: rows, isLoading, refetch, isRefetching } = useQuery({
    queryKey: ["consents", tenantId],
    enabled: !!tenantId,
    queryFn: async () => {
      try {
        return await api.get<ConsentRow[]>(`${API_ROUTES.ADMIN_CONSENTS}?tenant=${tenantId}`)
      } catch (e: any) {
        if (e?.status === 404) return [] as ConsentRow[]
        throw e
      }
    },
  })

  const revokeMutation = useMutation({
    mutationFn: async (c: ConsentRow) => {
      const qs = new URLSearchParams()
      if (tenantId) qs.set("tenant", tenantId)
      if (c.user_id) qs.set("user", c.user_id)
      if (c.client_id) qs.set("client", c.client_id)
      // Prefer a DELETE composed by keys; fallback to POST revoke if backend expects it
      try {
        return await api.delete(`${API_ROUTES.ADMIN_CONSENTS}?${qs.toString()}`)
      } catch {
        return await api.post(`${API_ROUTES.ADMIN_CONSENTS_REVOKE}?${qs.toString()}`)
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["consents", tenantId] })
    },
  })

  const filtered = useMemo(() => {
    const list = rows || []
    if (!q) return list
    const s = q.toLowerCase()
    return list.filter((r) =>
      [r.user_email, r.user_id, r.client_id, r.client_name, (r.scopes || []).join(" ")]
        .filter(Boolean)
        .some((v) => String(v).toLowerCase().includes(s)),
    )
  }, [rows, q])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-3">
        <h1 className="text-3xl font-bold">{t("consents.title")}</h1>
        <div className="flex items-center gap-2">
          <Input
            placeholder={t("common.search")}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            className="w-64"
          />
          <Button variant="outline" onClick={() => refetch()} disabled={isLoading || isRefetching}>
            <RefreshCw className="mr-2 h-4 w-4" /> {t("common.refresh")}
          </Button>
        </div>
      </div>

      <Card className="p-6">
        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("consents.user")}</TableHead>
                <TableHead>{t("consents.client")}</TableHead>
                <TableHead>{t("consents.scopes")}</TableHead>
                <TableHead>{t("consents.createdAt")}</TableHead>
                <TableHead className="text-right">{t("common.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground">
                    {t("consents.empty")}
                  </TableCell>
                </TableRow>
              ) : (
                filtered.map((c, idx) => (
                  <TableRow key={c.id || `${c.user_id}:${c.client_id}:${idx}`}>
                    <TableCell className="font-medium">{c.user_email || c.user_id}</TableCell>
                    <TableCell>
                      <code className="rounded bg-muted px-2 py-1 text-xs">{c.client_id}</code>
                    </TableCell>
                    <TableCell>
                      <span className="text-xs text-muted-foreground">{(c.scopes || []).join(" ") || "-"}</span>
                    </TableCell>
                    <TableCell>
                      <span className="text-xs">{c.created_at ? new Date(c.created_at).toLocaleString() : "-"}</span>
                    </TableCell>
                    <TableCell className="text-right">
                      {c.user_id && c.client_id ? (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => revokeMutation.mutate(c)}
                          disabled={revokeMutation.isPending}
                          aria-label={t("consents.revoke")}
                        >
                          <Trash2 className="h-4 w-4 text-destructive" />
                        </Button>
                      ) : null}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        )}
      </Card>
    </div>
  )
}

export default function ConsentsPage() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <ConsentsContent />
    </Suspense>
  )
}
