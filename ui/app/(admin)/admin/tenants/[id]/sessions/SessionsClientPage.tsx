"use client"

import { useParams, useSearchParams } from "next/navigation"
import { ArrowLeft, Clock3 } from "lucide-react"
import { useI18n } from "@/lib/i18n"
import { Card } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import Link from "next/link"
import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import type { Tenant } from "@/lib/types"

import { Suspense } from "react"

function SessionsContent() {
  const params = useParams()
  const searchParams = useSearchParams()
  const tenantId = (params.id as string) || (searchParams.get("id") as string)
  const { t } = useI18n()

  const { data: tenant } = useQuery({
    queryKey: ["tenant", tenantId],
    enabled: !!tenantId,
    queryFn: () => api.get<Tenant>(`/v1/admin/tenants/${tenantId}`),
  })

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="sm" asChild>
            <Link href={`/admin/tenants/detail?id=${tenantId}`}>
              <ArrowLeft className="h-4 w-4" />
            </Link>
          </Button>
          <div>
            <h1 className="text-3xl font-bold">{t("sessions.title")}</h1>
            <p className="text-muted-foreground">
              {tenant?.name} - {t("sessions.description")}
            </p>
          </div>
        </div>
      </div>

      <Card className="p-12 text-center">
        <Clock3 className="mx-auto h-12 w-12 text-muted-foreground/50" />
        <h3 className="mt-4 text-lg font-semibold">Gestión de Sesiones</h3>
        <p className="mt-2 text-muted-foreground">
          Esta funcionalidad permitirá ver y revocar sesiones activas de usuarios.
          <br />
          Actualmente en desarrollo.
        </p>
      </Card>
    </div>
  )
}

export default function SessionsClientPage() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <SessionsContent />
    </Suspense>
  )
}
