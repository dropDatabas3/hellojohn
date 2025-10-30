"use client"

import { useState } from "react"
import { useMutation } from "@tanstack/react-query"
import { Database, Play, CheckCircle, XCircle, AlertCircle } from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { useToast } from "@/hooks/use-toast"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"

export default function DatabasePage() {
  const { t } = useI18n()
  const { toast } = useToast()
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null)

  const testConnectionMutation = useMutation({
    mutationFn: () => api.post<{ ok: boolean; message?: string }>("/v1/db/test", {}),
    onSuccess: (data) => {
      setTestResult({
        success: data.ok,
        message: data.message || t("database.connectionSuccess"),
      })
      toast({
        title: data.ok ? t("common.success") : t("common.error"),
        description: data.message || t("database.connectionSuccess"),
        variant: data.ok ? "default" : "destructive",
      })
    },
    onError: (error: any) => {
      setTestResult({
        success: false,
        message: error.message,
      })
      toast({
        title: t("common.error"),
        description: error.message,
        variant: "destructive",
      })
    },
  })

  const runMigrationsMutation = useMutation({
    mutationFn: () => api.post<{ applied: number; message: string }>("/v1/db/migrate", {}),
    onSuccess: (data) => {
      toast({
        title: t("database.migrationsApplied"),
        description: t("database.migrationsAppliedDesc", { count: data.applied }),
      })
    },
    onError: (error: any) => {
      toast({
        title: t("common.error"),
        description: error.message,
        variant: "destructive",
      })
    },
  })

  const seedDatabaseMutation = useMutation({
    mutationFn: () => api.post<{ message: string }>("/v1/db/seed", {}),
    onSuccess: (data) => {
      toast({
        title: t("database.seeded"),
        description: data.message,
      })
    },
    onError: (error: any) => {
      toast({
        title: t("common.error"),
        description: error.message,
        variant: "destructive",
      })
    },
  })

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">{t("database.title")}</h1>
        <p className="text-muted-foreground">{t("database.description")}</p>
      </div>

      <Tabs defaultValue="operations" className="space-y-6">
        <TabsList>
          <TabsTrigger value="operations">{t("database.operations")}</TabsTrigger>
          <TabsTrigger value="migrations">{t("database.migrations")}</TabsTrigger>
        </TabsList>

        <TabsContent value="operations" className="space-y-6">
          <Card className="p-6">
            <div className="flex items-center gap-4 mb-6">
              <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-blue-500/10">
                <Database className="h-6 w-6 text-blue-500" />
              </div>
              <div>
                <h2 className="text-xl font-semibold">{t("database.testConnection")}</h2>
                <p className="text-sm text-muted-foreground">{t("database.testConnectionDesc")}</p>
              </div>
            </div>

            {testResult && (
              <Alert className="mb-4" variant={testResult.success ? "default" : "destructive"}>
                {testResult.success ? <CheckCircle className="h-4 w-4" /> : <XCircle className="h-4 w-4" />}
                <AlertTitle>{testResult.success ? t("common.success") : t("common.error")}</AlertTitle>
                <AlertDescription>{testResult.message}</AlertDescription>
              </Alert>
            )}

            <Button onClick={() => testConnectionMutation.mutate()} disabled={testConnectionMutation.isPending}>
              <Play className="mr-2 h-4 w-4" />
              {testConnectionMutation.isPending ? t("database.testing") : t("database.testConnection")}
            </Button>
          </Card>

          <Card className="p-6">
            <div className="flex items-center gap-4 mb-6">
              <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-green-500/10">
                <Database className="h-6 w-6 text-green-500" />
              </div>
              <div>
                <h2 className="text-xl font-semibold">{t("database.seedDatabase")}</h2>
                <p className="text-sm text-muted-foreground">{t("database.seedDatabaseDesc")}</p>
              </div>
            </div>

            <Alert className="mb-4">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>{t("common.warning")}</AlertTitle>
              <AlertDescription>{t("database.seedWarning")}</AlertDescription>
            </Alert>

            <Button
              onClick={() => seedDatabaseMutation.mutate()}
              disabled={seedDatabaseMutation.isPending}
              variant="outline"
            >
              <Play className="mr-2 h-4 w-4" />
              {seedDatabaseMutation.isPending ? t("database.seeding") : t("database.seedDatabase")}
            </Button>
          </Card>
        </TabsContent>

        <TabsContent value="migrations" className="space-y-6">
          <Card className="p-6">
            <div className="flex items-center gap-4 mb-6">
              <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-purple-500/10">
                <Database className="h-6 w-6 text-purple-500" />
              </div>
              <div>
                <h2 className="text-xl font-semibold">{t("database.runMigrations")}</h2>
                <p className="text-sm text-muted-foreground">{t("database.runMigrationsDesc")}</p>
              </div>
            </div>

            <Alert className="mb-4">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>{t("common.info")}</AlertTitle>
              <AlertDescription>{t("database.migrationsInfo")}</AlertDescription>
            </Alert>

            <Button onClick={() => runMigrationsMutation.mutate()} disabled={runMigrationsMutation.isPending}>
              <Play className="mr-2 h-4 w-4" />
              {runMigrationsMutation.isPending ? t("database.running") : t("database.runMigrations")}
            </Button>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
