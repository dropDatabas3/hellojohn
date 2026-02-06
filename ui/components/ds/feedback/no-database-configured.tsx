"use client"

import { Database, ArrowRight } from "lucide-react"
import { useRouter } from "next/navigation"
import { Button } from "../core/button"

interface NoDatabaseConfiguredProps {
  tenantId: string
  /** Contextual message describing what requires a database */
  message?: string
}

export function NoDatabaseConfigured({
  tenantId,
  message = "Conecta una base de datos para comenzar a gestionar este tenant.",
}: NoDatabaseConfiguredProps) {
  const router = useRouter()

  return (
    <div className="flex flex-col items-center justify-center py-20 px-6">
      <div className="relative mb-8">
        <div className="absolute inset-0 bg-gradient-to-br from-warning/20 to-warning/10 rounded-full blur-2xl scale-150" />
        <div className="relative rounded-clay bg-gradient-to-br from-warning/10 to-warning/5 p-5 border-2 border-clay shadow-clay-float">
          <Database className="h-8 w-8 text-warning" />
        </div>
      </div>
      <h3 className="text-xl font-semibold text-center mb-2">Configura tu base de datos</h3>
      <p className="text-muted-foreground text-center max-w-sm mb-8 text-sm">
        {message}
      </p>
      <Button
        onClick={() => router.push(`/admin/tenants/${tenantId}/database`)}
        className="gap-2"
        size="lg"
      >
        Configurar
        <ArrowRight className="h-4 w-4" />
      </Button>
    </div>
  )
}

/**
 * Helper to detect if an API error indicates a missing tenant database.
 */
export function isNoDatabaseError(error: unknown): boolean {
  if (!error || typeof error !== "object") return false
  const err = error as Record<string, unknown>
  return err.error === "TENANT_NO_DATABASE" || err.status === 424
}
