"use client"

import { useEffect, useMemo, useRef, useState } from "react"
import { Card } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { useI18n } from "@/lib/i18n"
import { api } from "@/lib/api"
import { RefreshCw, Pause, Play } from "lucide-react"

type LogLine = string

export default function LogsPage() {
  const { t } = useI18n()
  const [lines, setLines] = useState<LogLine[]>([])
  const [autoScroll, setAutoScroll] = useState(true)
  const [paused, setPaused] = useState(false)
  const [filter, setFilter] = useState("")
  const viewportRef = useRef<HTMLDivElement>(null)
  const esRef = useRef<EventSource | null>(null)

  useEffect(() => {
    const token = (window as any).__HJ_TOKEN__ || (require("@/lib/auth-store").useAuthStore.getState().token as string)

    // Try SSE first
    const url = new URL(`${api.getBaseUrl()}/v1/admin/logs/stream`)
    if (token) url.searchParams.set("bearer", token)

    try {
      const es = new EventSource(url.toString(), { withCredentials: false })
      esRef.current = es
      es.onmessage = (ev) => {
        if (paused) return
        setLines((prev) => [...prev, ev.data].slice(-2000))
      }
      es.onerror = () => {
        es.close()
        esRef.current = null
      }
      return () => es.close()
    } catch {
      // Fallback to polling below
    }
  }, [paused])

  // Polling fallback if SSE is not connected
  useEffect(() => {
    if (esRef.current) return
    let cancelled = false
    const tick = async () => {
      try {
        const data = await api.get<LogLine[]>(`/v1/admin/logs?tail=200`)
        if (!cancelled && !paused) setLines(data)
      } catch {
        // ignore
      }
      if (!cancelled) setTimeout(tick, 3000)
    }
    tick()
    return () => {
      cancelled = true
    }
  }, [paused])

  useEffect(() => {
    if (!autoScroll) return
    const el = viewportRef.current
    if (!el) return
    el.scrollTop = el.scrollHeight
  }, [lines, autoScroll])

  const filtered = useMemo(() => {
    const s = filter.trim().toLowerCase()
    if (!s) return lines
    return lines.filter((l) => l.toLowerCase().includes(s))
  }, [lines, filter])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">{t("logs.title")}</h1>
        <div className="flex items-center gap-2">
          <Input
            placeholder={t("common.search")}
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="w-64"
          />
          <Button variant="outline" onClick={() => setPaused((p) => !p)} aria-label={paused ? t("logs.resume") : t("logs.pause")}>
            {paused ? <Play className="mr-2 h-4 w-4" /> : <Pause className="mr-2 h-4 w-4" />}
            {paused ? t("logs.resume") : t("logs.pause")}
          </Button>
          <Button variant="outline" onClick={() => setLines([])}>
            <RefreshCw className="mr-2 h-4 w-4" /> {t("common.clear")}
          </Button>
        </div>
      </div>

      <Card className="p-0 overflow-hidden">
        <div ref={viewportRef} className="h-[60vh] w-full overflow-auto bg-black text-white font-mono text-xs p-3">
          {filtered.map((l, i) => (
            <div key={i} className="whitespace-pre-wrap">
              {l}
            </div>
          ))}
        </div>
        <div className="flex items-center justify-between border-t p-2 text-xs text-muted-foreground">
          <label className="flex items-center gap-2">
            <input type="checkbox" checked={autoScroll} onChange={(e) => setAutoScroll(e.target.checked)} />
            {t("logs.autoScroll")}
          </label>
          <div>{t("logs.lines")}: {filtered.length}</div>
        </div>
      </Card>
    </div>
  )
}
