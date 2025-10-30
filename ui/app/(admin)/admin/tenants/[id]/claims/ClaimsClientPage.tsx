"use client"

import { useParams, useSearchParams } from "next/navigation"
import { Card } from "@/components/ui/card"
import { useI18n } from "@/lib/i18n"

export default function ClaimsClientPage() {
  const params = useParams()
  const searchParams = useSearchParams()
  const tenantId = (params.id as string) || (searchParams.get("id") as string)
  const { t } = useI18n()
  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">{t("claims.title")}</h1>
      <Card className="p-6">
        <p className="text-muted-foreground">
          {t("claims.description")} ({tenantId})
        </p>
        <p className="text-sm mt-2">Coming soon…</p>
      </Card>
    </div>
  )
}
