"use client"

import { useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Key, RotateCw, CheckCircle, Copy, Check } from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { useToast } from "@/hooks/use-toast"
import { Badge } from "@/components/ui/badge"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

type KeyInfo = {
  kid: string
  alg: string
  use: string
  active: boolean
  created_at: string
}

export default function KeysPage() {
  const { t } = useI18n()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [rotateDialogOpen, setRotateDialogOpen] = useState(false)
  const [copiedKid, setCopiedKid] = useState<string | null>(null)

  const { data: keys, isLoading } = useQuery({
    queryKey: ["keys"],
    queryFn: () => api.get<KeyInfo[]>("/v1/keys"),
  })

  const rotateMutation = useMutation({
    mutationFn: () => api.post<{ kid: string }>("/v1/keys/rotate", {}),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ["keys"] })
      setRotateDialogOpen(false)
      toast({
        title: t("keys.rotated"),
        description: t("keys.rotatedDesc", { kid: data.kid }),
        variant: "info",
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

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopiedKid(text)
    setTimeout(() => setCopiedKid(null), 2000)
    toast({
      title: t("common.copied"),
      description: t("keys.kidCopied"),
      variant: "info",
    })
  }

  const activeKey = keys?.find((k) => k.active)

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t("keys.title")}</h1>
          <p className="text-muted-foreground">{t("keys.description")}</p>
        </div>
        <Button onClick={() => setRotateDialogOpen(true)}>
          <RotateCw className="mr-2 h-4 w-4" />
          {t("keys.rotate")}
        </Button>
      </div>

      {activeKey && (
        <Card className="p-6">
          <div className="flex items-center gap-4 mb-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-green-500/10">
              <CheckCircle className="h-6 w-6 text-green-500" />
            </div>
            <div>
              <h2 className="text-xl font-semibold">{t("keys.activeKey")}</h2>
              <p className="text-sm text-muted-foreground">{t("keys.activeKeyDesc")}</p>
            </div>
          </div>
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">{t("keys.kid")}</span>
              <div className="flex items-center gap-2">
                <code className="rounded bg-muted px-2 py-1 text-sm">{activeKey.kid}</code>
                <Button variant="ghost" size="sm" onClick={() => copyToClipboard(activeKey.kid)}>
                  {copiedKid === activeKey.kid ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">{t("keys.algorithm")}</span>
              <Badge>{activeKey.alg}</Badge>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">{t("keys.use")}</span>
              <Badge variant="outline">{activeKey.use}</Badge>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">{t("keys.createdAt")}</span>
              <span className="text-sm">{new Date(activeKey.created_at).toLocaleString()}</span>
            </div>
          </div>
        </Card>
      )}

      <Card className="p-6">
        <h2 className="mb-4 text-xl font-semibold">{t("keys.allKeys")}</h2>
        {isLoading ? (
          <div className="flex items-center justify-center py-8">
            <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
          </div>
        ) : (
          <div className="space-y-3">
            {keys?.map((key) => (
              <div key={key.kid} className="flex items-center justify-between rounded-lg border p-4">
                <div className="flex items-center gap-4">
                  <Key className="h-5 w-5 text-muted-foreground" />
                  <div>
                    <div className="flex items-center gap-2">
                      <code className="text-sm font-medium">{key.kid}</code>
                      {key.active && <Badge variant="default">{t("keys.active")}</Badge>}
                    </div>
                    <p className="text-sm text-muted-foreground">
                      {key.alg} • {new Date(key.created_at).toLocaleDateString()}
                    </p>
                  </div>
                </div>
                <Button variant="ghost" size="sm" onClick={() => copyToClipboard(key.kid)}>
                  {copiedKid === key.kid ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>
            ))}
          </div>
        )}
      </Card>

      <Dialog open={rotateDialogOpen} onOpenChange={setRotateDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("keys.rotateTitle")}</DialogTitle>
            <DialogDescription>{t("keys.rotateDescription")}</DialogDescription>
          </DialogHeader>
          <Alert>
            <AlertTitle>{t("common.warning")}</AlertTitle>
            <AlertDescription>{t("keys.rotateWarning")}</AlertDescription>
          </Alert>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRotateDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={() => rotateMutation.mutate()} disabled={rotateMutation.isPending}>
              {rotateMutation.isPending ? t("keys.rotating") : t("keys.rotate")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
