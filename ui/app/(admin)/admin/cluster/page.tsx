"use client"

import { useState, useMemo, Suspense } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  Server, AlertCircle, CheckCircle2, Activity, RefreshCw, Plus, Trash2,
  Cpu, Network, Crown, Users, Zap, Clock, Shield,
  Copy, Info, HelpCircle, Loader2, XCircle, 
  Database, Download, AlertTriangle,
  Settings2, Terminal, History,
  Play, Pause, SkipForward, Circle, Eye, EyeOff, Hash
} from "lucide-react"
import { cn } from "@/lib/utils"
import { api } from "@/lib/api"
import { API_ROUTES } from "@/lib/routes"
import { useToast } from "@/hooks/use-toast"

// UI Components
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/components/ui/alert"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

// ─── Types ───

interface ClusterInfo {
  mode: "off" | "embedded" | "external"
  role?: "leader" | "follower" | "candidate"
  node_id?: string
  leader_id?: string
  peers_configured?: number
  peers_connected?: number | string
  leader_redirects?: string[]
  raft?: {
    state?: string
    term?: string
    commit_index?: string
    applied_index?: string
    last_log_index?: string
    last_snapshot_index?: string
    num_peers?: string
    last_contact?: string
  }
}

interface HealthResponse {
  status: "ready" | "degraded" | "unavailable"
  components: Record<string, { status: string; message?: string }>
  version?: string
  commit?: string
  active_key_id?: string
  timestamp: string
  cluster: ClusterInfo
  fs_degraded?: boolean
}

interface ClusterNode {
  id: string
  address: string
  role: "leader" | "follower" | "candidate"
  state: "healthy" | "degraded" | "unreachable"
  joined_at?: string
  last_seen?: string
  latency_ms?: number
}

interface RaftLogEntry {
  index: number
  term: number
  type: string
  timestamp: string
  tenant_id?: string
  key?: string
}

interface ClusterNodesResponse {
  nodes: ClusterNode[]
}

interface ClusterStatsResponse {
  stats: {
    node_id: string
    role: string
    leader_id: string
    term: number
    commit_index: number
    applied_index: number
    num_peers: number
    healthy: boolean
  }
}

// ─── Helper Components ───

function InfoTooltip({ content }: { content: string }) {
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <HelpCircle className="h-4 w-4 text-muted-foreground cursor-help ml-1.5 inline-block" />
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-xs">
          <p className="text-sm">{content}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

function StatCard({
  title,
  value,
  icon: Icon,
  description,
  color = "default",
  badge,
}: {
  title: string
  value: string | number
  icon: React.ElementType
  description?: string
  color?: "default" | "green" | "amber" | "red" | "blue" | "purple"
  badge?: { text: string; variant: "default" | "secondary" | "destructive" | "outline" }
}) {
  const colorClasses = {
    default: "from-zinc-500/20 to-zinc-600/5 text-zinc-400",
    green: "from-emerald-500/20 to-emerald-600/5 text-emerald-400",
    amber: "from-amber-500/20 to-amber-600/5 text-amber-400",
    red: "from-red-500/20 to-red-600/5 text-red-400",
    blue: "from-blue-500/20 to-blue-600/5 text-blue-400",
    purple: "from-purple-500/20 to-purple-600/5 text-purple-400",
  }

  return (
    <Card className="relative overflow-hidden border-white/[0.08] bg-gradient-to-br from-white/[0.05] to-transparent">
      <div className={cn("absolute top-0 right-0 w-32 h-32 bg-gradient-to-br opacity-50 rounded-full blur-2xl -translate-y-1/2 translate-x-1/2", colorClasses[color])} />
      <CardContent className="p-5">
        <div className="flex items-center justify-between">
          <div>
            <div className="flex items-center gap-2">
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{title}</p>
              {badge && (
                <Badge variant={badge.variant} className="text-[10px] px-1.5 py-0">
                  {badge.text}
                </Badge>
              )}
            </div>
            <p className="text-2xl font-bold mt-1">{value}</p>
            {description && (
              <p className="text-xs text-muted-foreground mt-0.5">{description}</p>
            )}
          </div>
          <div className={cn("p-2.5 rounded-xl bg-gradient-to-br", colorClasses[color])}>
            <Icon className="h-5 w-5" />
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function NodeStatusBadge({ state }: { state: string }) {
  const config = {
    healthy: { color: "bg-emerald-500/20 text-emerald-400 border-emerald-500/30", icon: CheckCircle2 },
    degraded: { color: "bg-amber-500/20 text-amber-400 border-amber-500/30", icon: AlertCircle },
    unreachable: { color: "bg-red-500/20 text-red-400 border-red-500/30", icon: XCircle },
  }[state] || { color: "bg-zinc-500/20 text-zinc-400 border-zinc-500/30", icon: Circle }

  const IconComponent = config.icon

  return (
    <Badge variant="outline" className={cn("gap-1", config.color)}>
      <IconComponent className="h-3 w-3" />
      {state === "healthy" ? "Conectado" : state === "degraded" ? "Degradado" : "Desconectado"}
    </Badge>
  )
}

function RoleBadge({ role, isCurrentNode = false }: { role: string; isCurrentNode?: boolean }) {
  if (role === "leader") {
    return (
      <Badge className="bg-gradient-to-r from-amber-500/20 to-yellow-500/20 text-amber-300 border-amber-500/30 gap-1">
        <Crown className="h-3 w-3" />
        Leader
        {isCurrentNode && <span className="text-[10px] ml-1">(este)</span>}
      </Badge>
    )
  }
  if (role === "candidate") {
    return (
      <Badge variant="outline" className="bg-purple-500/10 text-purple-400 border-purple-500/30 gap-1">
        <Zap className="h-3 w-3" />
        Candidate
      </Badge>
    )
  }
  return (
    <Badge variant="outline" className="bg-blue-500/10 text-blue-400 border-blue-500/30 gap-1">
      <Users className="h-3 w-3" />
      Follower
      {isCurrentNode && <span className="text-[10px] ml-1">(este)</span>}
    </Badge>
  )
}

// ─── Mock Data Generators ───

function generateMockNodes(health: HealthResponse): ClusterNode[] {
  if (health.cluster.mode === "off") return []
  
  const nodes: ClusterNode[] = []
  const currentNodeId = health.cluster.node_id || "node-1"
  const leaderId = health.cluster.leader_id || currentNodeId
  const peersConfigured = health.cluster.peers_configured || 1

  // Current node
  nodes.push({
    id: currentNodeId,
    address: "127.0.0.1:7000",
    role: currentNodeId === leaderId ? "leader" : "follower",
    state: "healthy",
    joined_at: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString(),
    last_seen: new Date().toISOString(),
    latency_ms: 0,
  })

  // Additional mock nodes based on peers_configured
  for (let i = 2; i <= peersConfigured; i++) {
    const nodeId = `node-${i}`
    nodes.push({
      id: nodeId,
      address: `10.0.0.${i}:7000`,
      role: nodeId === leaderId ? "leader" : "follower",
      state: i <= Number(health.cluster.peers_connected || 1) ? "healthy" : "unreachable",
      joined_at: new Date(Date.now() - (i * 2) * 24 * 60 * 60 * 1000).toISOString(),
      last_seen: i <= Number(health.cluster.peers_connected || 1) 
        ? new Date(Date.now() - Math.random() * 5000).toISOString()
        : new Date(Date.now() - 60 * 60 * 1000).toISOString(),
      latency_ms: i <= Number(health.cluster.peers_connected || 1) ? Math.floor(Math.random() * 50) + 1 : undefined,
    })
  }

  return nodes
}

function generateMockRaftLog(): RaftLogEntry[] {
  const types = [
    "tenant.create", "tenant.update", "client.create", "client.update", 
    "scope.create", "key.rotate", "settings.update"
  ]
  const entries: RaftLogEntry[] = []
  
  for (let i = 0; i < 20; i++) {
    entries.push({
      index: 1000 - i,
      term: Math.floor((1000 - i) / 100) + 1,
      type: types[Math.floor(Math.random() * types.length)],
      timestamp: new Date(Date.now() - i * 60 * 1000).toISOString(),
      tenant_id: Math.random() > 0.3 ? `tenant-${Math.floor(Math.random() * 5) + 1}` : undefined,
      key: Math.random() > 0.5 ? `key-${Math.floor(Math.random() * 100)}` : undefined,
    })
  }
  
  return entries
}

// ─── Helpers ───

function formatTimeAgo(dateString: string): string {
  const date = new Date(dateString)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSecs = Math.floor(diffMs / 1000)
  const diffMins = Math.floor(diffSecs / 60)
  const diffHours = Math.floor(diffMins / 60)
  const diffDays = Math.floor(diffHours / 24)

  if (diffSecs < 5) return "ahora"
  if (diffSecs < 60) return `hace ${diffSecs}s`
  if (diffMins < 60) return `hace ${diffMins}m`
  if (diffHours < 24) return `hace ${diffHours}h`
  return `hace ${diffDays}d`
}

// ─── Main Component ───

function ClusterContent() {
  const { toast } = useToast()
  const queryClient = useQueryClient()
  
  // State
  const [currentTab, setCurrentTab] = useState("overview")
  const [addNodeDialog, setAddNodeDialog] = useState(false)
  const [removeNodeDialog, setRemoveNodeDialog] = useState<ClusterNode | null>(null)
  const [newNodeForm, setNewNodeForm] = useState({ id: "", address: "" })
  const [showRaftDetails, setShowRaftDetails] = useState(false)
  const [autoRefresh, setAutoRefresh] = useState(true)

  // ─── Queries ───

  const { data: health, isLoading, refetch, isRefetching } = useQuery({
    queryKey: ["cluster-health"],
    queryFn: () => api.get<HealthResponse>("/readyz"),
    refetchInterval: autoRefresh ? 5000 : false,
  })

  // Fetch cluster nodes from real API
  const { data: nodesData, isLoading: nodesLoading, refetch: refetchNodes } = useQuery({
    queryKey: ["cluster-nodes"],
    queryFn: async () => {
      const res = await api.get<ClusterNodesResponse>(API_ROUTES.ADMIN_CLUSTER_NODES)
      return res
    },
    enabled: health?.cluster?.mode !== "off",
    refetchInterval: autoRefresh ? 5000 : false,
  })
  const nodes = nodesData?.nodes || []

  // Mock raft log (until backend implements endpoint)
  const raftLog = useMemo(() => generateMockRaftLog(), [])

  // ─── Mutations ───

  const addNodeMutation = useMutation({
    mutationFn: async (data: { id: string; address: string }) => {
      return await api.post(API_ROUTES.ADMIN_CLUSTER_NODES, data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cluster-health"] })
      queryClient.invalidateQueries({ queryKey: ["cluster-nodes"] })
      toast({
        title: "Nodo agregado",
        description: "El nodo se ha agregado al cluster correctamente.",
      })
      setAddNodeDialog(false)
      setNewNodeForm({ id: "", address: "" })
    },
    onError: (error: any) => {
      toast({
        title: "Error",
        description: error?.message || "No se pudo agregar el nodo",
        variant: "destructive",
      })
    },
  })

  const removeNodeMutation = useMutation({
    mutationFn: async (nodeId: string) => {
      return await api.delete(API_ROUTES.ADMIN_CLUSTER_NODE_REMOVE(nodeId))
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cluster-health"] })
      queryClient.invalidateQueries({ queryKey: ["cluster-nodes"] })
      toast({
        title: "Nodo removido",
        description: "El nodo ha sido removido del cluster.",
      })
      setRemoveNodeDialog(null)
    },
    onError: (error: any) => {
      toast({
        title: "Error",
        description: error?.message || "No se pudo remover el nodo",
        variant: "destructive",
      })
    },
  })

  const forceSnapshotMutation = useMutation({
    mutationFn: async () => {
      // TODO: Backend endpoint needed: POST /v2/admin/cluster/snapshot
      await new Promise(resolve => setTimeout(resolve, 2000))
      return { success: true }
    },
    onSuccess: () => {
      toast({
        title: "Snapshot creado",
        description: "El snapshot del cluster se ha creado correctamente.",
      })
    },
  })

  const transferLeadershipMutation = useMutation({
    mutationFn: async () => {
      // TODO: Backend endpoint needed: POST /v2/admin/cluster/leader-transfer
      await new Promise(resolve => setTimeout(resolve, 1500))
      return { success: true }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cluster-health"] })
      toast({
        title: "Liderazgo transferido",
        description: "Se ha iniciado la transferencia de liderazgo.",
      })
    },
  })

  // ─── Computed Values ───

  const isClusterEnabled = health?.cluster.mode !== "off"
  const isLeader = health?.cluster.role === "leader"
  const currentNodeId = health?.cluster.node_id
  const healthyNodes = nodes.filter(n => n.state === "healthy").length
  const totalNodes = nodes.length

  // ─── Handlers ───

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    toast({ title: "Copiado", description: "Copiado al portapapeles" })
  }

  const handleAddNode = () => {
    if (!newNodeForm.id.trim() || !newNodeForm.address.trim()) {
      toast({
        title: "Error",
        description: "ID y dirección son requeridos",
        variant: "destructive",
      })
      return
    }
    addNodeMutation.mutate(newNodeForm)
  }

  // ─── Render ───

  return (
    <TooltipProvider>
      <div className="space-y-6 animate-in fade-in duration-500">
        {/* Header */}
        <div className="flex flex-col gap-2">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-2.5 rounded-xl bg-gradient-to-br from-purple-500/20 to-indigo-500/20 border border-purple-500/20">
                <Network className="h-6 w-6 text-purple-400" />
              </div>
              <div>
                <h1 className="text-2xl font-bold tracking-tight">Cluster Management</h1>
                <p className="text-muted-foreground text-sm">
                  Gestión del cluster Raft y nodos distribuidos
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setAutoRefresh(!autoRefresh)}
                className={cn("gap-2", autoRefresh && "border-emerald-500/30 text-emerald-400")}
              >
                {autoRefresh ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
                <span className="hidden sm:inline">Auto-refresh</span>
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => refetch()}
                disabled={isRefetching}
                className="gap-2"
              >
                <RefreshCw className={cn("h-4 w-4", isRefetching && "animate-spin")} />
                <span className="hidden sm:inline">Actualizar</span>
              </Button>
            </div>
          </div>
        </div>

        {/* Info Banner */}
        <Alert className="border-purple-500/20 bg-gradient-to-r from-purple-500/10 to-indigo-500/5">
          <Info className="h-4 w-4 text-purple-400" />
          <AlertTitle className="text-purple-300">¿Qué es el Cluster Raft?</AlertTitle>
          <AlertDescription className="text-muted-foreground">
            HelloJohn usa el algoritmo de consenso <strong>Raft</strong> para sincronizar la configuración del Control Plane 
            entre múltiples nodos. Esto garantiza alta disponibilidad y consistencia: si un nodo falla, otro toma el 
            liderazgo automáticamente. En modo single-node, el cluster tiene un solo nodo que siempre es el líder.
          </AlertDescription>
        </Alert>

        {/* Cluster Disabled Warning */}
        {!isClusterEnabled && (
          <Alert variant="default" className="border-amber-500/20 bg-amber-500/5">
            <AlertTriangle className="h-4 w-4 text-amber-400" />
            <AlertTitle className="text-amber-300">Cluster Deshabilitado</AlertTitle>
            <AlertDescription className="text-muted-foreground">
              El cluster Raft no está habilitado. HelloJohn está ejecutándose en modo single-node.
              Para habilitar el cluster, configura las variables de entorno <code className="text-amber-400">CLUSTER_*</code> y reinicia el servicio.
            </AlertDescription>
          </Alert>
        )}

        {/* Follower Warning */}
        {isClusterEnabled && !isLeader && (
          <Alert variant="default" className="border-blue-500/20 bg-blue-500/5">
            <AlertCircle className="h-4 w-4 text-blue-400" />
            <AlertTitle className="text-blue-300">Nodo Follower</AlertTitle>
            <AlertDescription className="text-muted-foreground">
              Este nodo es un <strong>follower</strong>. Las operaciones de escritura (crear tenants, clients, etc.) 
              deben realizarse en el nodo <strong>leader</strong>: <code className="text-blue-400">{health?.cluster.leader_id}</code>.
              Las lecturas funcionan en cualquier nodo.
            </AlertDescription>
          </Alert>
        )}

        {isLoading ? (
          <div className="flex items-center justify-center py-16">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : health ? (
          <>
            {/* Stats Cards */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <StatCard
                title="Rol del Nodo"
                value={health.cluster.role === "leader" ? "Leader" : health.cluster.role === "follower" ? "Follower" : "N/A"}
                icon={health.cluster.role === "leader" ? Crown : Users}
                color={health.cluster.role === "leader" ? "amber" : "blue"}
                badge={isClusterEnabled ? { 
                  text: health.cluster.node_id || "local", 
                  variant: "outline" 
                } : undefined}
              />
              <StatCard
                title="Nodos"
                value={`${healthyNodes} / ${totalNodes}`}
                icon={Server}
                description="Conectados / Total"
                color={healthyNodes === totalNodes ? "green" : "amber"}
              />
              <StatCard
                title="Term"
                value={health.cluster.raft?.term || "1"}
                icon={Hash}
                description="Época del cluster"
                color="purple"
              />
              <StatCard
                title="Estado"
                value={health.status === "ready" ? "Saludable" : health.status === "degraded" ? "Degradado" : "Error"}
                icon={health.status === "ready" ? CheckCircle2 : AlertCircle}
                color={health.status === "ready" ? "green" : health.status === "degraded" ? "amber" : "red"}
              />
            </div>

            {/* Tabs */}
            <Tabs value={currentTab} onValueChange={setCurrentTab}>
              <div className="flex items-center justify-between gap-4 mb-4">
                <TabsList className="bg-white/5 border border-white/10">
                  <TabsTrigger value="overview" className="gap-2">
                    <Activity className="h-4 w-4" />
                    <span className="hidden sm:inline">Vista General</span>
                  </TabsTrigger>
                  <TabsTrigger value="nodes" className="gap-2">
                    <Server className="h-4 w-4" />
                    <span className="hidden sm:inline">Nodos</span>
                  </TabsTrigger>
                  <TabsTrigger value="raft" className="gap-2">
                    <Database className="h-4 w-4" />
                    <span className="hidden sm:inline">Raft Log</span>
                  </TabsTrigger>
                  <TabsTrigger value="operations" className="gap-2">
                    <Settings2 className="h-4 w-4" />
                    <span className="hidden sm:inline">Operaciones</span>
                  </TabsTrigger>
                </TabsList>
              </div>

              {/* Tab: Overview */}
              <TabsContent value="overview" className="space-y-6 mt-0">
                <div className="grid md:grid-cols-2 gap-6">
                  {/* Cluster Info */}
                  <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
                    <CardHeader>
                      <CardTitle className="text-lg flex items-center gap-2">
                        <Network className="h-5 w-5 text-purple-400" />
                        Información del Cluster
                      </CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-3">
                      <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                        <span className="text-sm text-muted-foreground">Modo</span>
                        <Badge variant="outline" className="font-mono">
                          {health.cluster.mode}
                        </Badge>
                      </div>
                      <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                        <span className="text-sm text-muted-foreground">Node ID</span>
                        <div className="flex items-center gap-2">
                          <code className="text-sm font-mono">{health.cluster.node_id || "local"}</code>
                          {health.cluster.node_id && (
                            <Button 
                              variant="ghost" 
                              size="sm" 
                              className="h-6 w-6 p-0"
                              onClick={() => copyToClipboard(health.cluster.node_id!)}
                            >
                              <Copy className="h-3 w-3" />
                            </Button>
                          )}
                        </div>
                      </div>
                      <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                        <span className="text-sm text-muted-foreground">Rol</span>
                        <RoleBadge role={health.cluster.role || "leader"} />
                      </div>
                      <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                        <span className="text-sm text-muted-foreground">Leader ID</span>
                        <div className="flex items-center gap-2">
                          <code className="text-sm font-mono">{health.cluster.leader_id || "self"}</code>
                          {health.cluster.leader_id && (
                            <Button 
                              variant="ghost" 
                              size="sm" 
                              className="h-6 w-6 p-0"
                              onClick={() => copyToClipboard(health.cluster.leader_id!)}
                            >
                              <Copy className="h-3 w-3" />
                            </Button>
                          )}
                        </div>
                      </div>
                      {health.cluster.leader_redirects && health.cluster.leader_redirects.length > 0 && (
                        <div className="p-3 rounded-lg bg-amber-500/5 border border-amber-500/20">
                          <p className="text-xs text-amber-400 mb-2">Redirección a Leader:</p>
                          {health.cluster.leader_redirects.map((url, i) => (
                            <code key={i} className="text-xs text-amber-300 block">{url}</code>
                          ))}
                        </div>
                      )}
                    </CardContent>
                  </Card>

                  {/* Raft State */}
                  <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
                    <CardHeader>
                      <div className="flex items-center justify-between">
                        <CardTitle className="text-lg flex items-center gap-2">
                          <Database className="h-5 w-5 text-blue-400" />
                          Estado Raft
                        </CardTitle>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setShowRaftDetails(!showRaftDetails)}
                          className="gap-1 text-xs"
                        >
                          {showRaftDetails ? <EyeOff className="h-3 w-3" /> : <Eye className="h-3 w-3" />}
                          {showRaftDetails ? "Menos" : "Más"}
                        </Button>
                      </div>
                    </CardHeader>
                    <CardContent className="space-y-3">
                      <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                        <span className="text-sm text-muted-foreground">State</span>
                        <Badge 
                          variant="outline" 
                          className={cn(
                            health.cluster.raft?.state === "Leader" && "bg-amber-500/10 text-amber-400 border-amber-500/30",
                            health.cluster.raft?.state === "Follower" && "bg-blue-500/10 text-blue-400 border-blue-500/30",
                            health.cluster.raft?.state === "Candidate" && "bg-purple-500/10 text-purple-400 border-purple-500/30"
                          )}
                        >
                          {health.cluster.raft?.state || "N/A"}
                        </Badge>
                      </div>
                      <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                        <span className="text-sm text-muted-foreground">Term</span>
                        <code className="text-sm font-mono">{health.cluster.raft?.term || "1"}</code>
                      </div>
                      <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                        <span className="text-sm text-muted-foreground">Commit Index</span>
                        <code className="text-sm font-mono">{health.cluster.raft?.commit_index || "0"}</code>
                      </div>
                      
                      {showRaftDetails && (
                        <>
                          <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                            <span className="text-sm text-muted-foreground">Applied Index</span>
                            <code className="text-sm font-mono">{health.cluster.raft?.applied_index || "0"}</code>
                          </div>
                          <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                            <span className="text-sm text-muted-foreground">Last Log Index</span>
                            <code className="text-sm font-mono">{health.cluster.raft?.last_log_index || "0"}</code>
                          </div>
                          <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                            <span className="text-sm text-muted-foreground">Last Snapshot Index</span>
                            <code className="text-sm font-mono">{health.cluster.raft?.last_snapshot_index || "0"}</code>
                          </div>
                          <div className="flex items-center justify-between p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                            <span className="text-sm text-muted-foreground">Last Contact</span>
                            <code className="text-sm font-mono text-xs">{health.cluster.raft?.last_contact || "never"}</code>
                          </div>
                        </>
                      )}
                    </CardContent>
                  </Card>
                </div>

                {/* Components Status */}
                <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                      <Cpu className="h-5 w-5 text-emerald-400" />
                      Componentes del Sistema
                    </CardTitle>
                    <CardDescription>
                      Estado de los componentes principales del servicio
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
                      {Object.entries(health.components).map(([key, value]) => {
                        const status = typeof value === "string" ? value : value.status
                        const message = typeof value === "object" ? value.message : undefined
                        
                        return (
                          <div 
                            key={key} 
                            className={cn(
                              "p-3 rounded-lg border transition-all",
                              status === "ok" && "bg-emerald-500/5 border-emerald-500/20",
                              status === "error" && "bg-red-500/5 border-red-500/20",
                              status === "disabled" && "bg-zinc-500/5 border-zinc-500/20"
                            )}
                          >
                            <div className="flex items-center justify-between mb-1">
                              <span className="text-sm font-medium capitalize">
                                {key.replace(/_/g, " ")}
                              </span>
                              {status === "ok" && <CheckCircle2 className="h-4 w-4 text-emerald-400" />}
                              {status === "error" && <XCircle className="h-4 w-4 text-red-400" />}
                              {status === "disabled" && <Circle className="h-4 w-4 text-zinc-500" />}
                            </div>
                            {message && (
                              <p className="text-[10px] text-muted-foreground truncate">{message}</p>
                            )}
                          </div>
                        )
                      })}
                    </div>
                  </CardContent>
                </Card>

                {/* System Info */}
                <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                      <Info className="h-5 w-5 text-zinc-400" />
                      Información del Sistema
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                      <div className="p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                        <p className="text-xs text-muted-foreground mb-1">Versión</p>
                        <code className="text-sm font-mono">{health.version || "dev"}</code>
                      </div>
                      <div className="p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                        <p className="text-xs text-muted-foreground mb-1">Commit</p>
                        <code className="text-sm font-mono truncate block">{health.commit?.slice(0, 8) || "unknown"}</code>
                      </div>
                      <div className="p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                        <p className="text-xs text-muted-foreground mb-1">Active Key ID</p>
                        <code className="text-sm font-mono truncate block">{health.active_key_id?.slice(0, 12) || "N/A"}...</code>
                      </div>
                      <div className="p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                        <p className="text-xs text-muted-foreground mb-1">Filesystem</p>
                        <Badge variant={health.fs_degraded ? "destructive" : "outline"}>
                          {health.fs_degraded ? "Degradado" : "OK"}
                        </Badge>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </TabsContent>

              {/* Tab: Nodes */}
              <TabsContent value="nodes" className="space-y-4 mt-0">
                {/* Backend Note */}
                <Alert className="border-amber-500/20 bg-amber-500/5">
                  <AlertTriangle className="h-4 w-4 text-amber-400" />
                  <AlertTitle className="text-amber-300">Funcionalidad en Desarrollo</AlertTitle>
                  <AlertDescription className="text-muted-foreground text-sm">
                    La gestión de nodos requiere endpoints backend que aún no están expuestos via HTTP.
                    Los datos mostrados son simulados. Endpoints necesarios:
                    <code className="ml-2 text-amber-400">GET/POST/DELETE /v2/admin/cluster/nodes</code>
                  </AlertDescription>
                </Alert>

                {/* Actions Bar */}
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Badge variant="outline" className="gap-1">
                      <Server className="h-3 w-3" />
                      {totalNodes} nodos
                    </Badge>
                    <Badge variant="outline" className="gap-1 text-emerald-400 border-emerald-500/30">
                      <CheckCircle2 className="h-3 w-3" />
                      {healthyNodes} conectados
                    </Badge>
                  </div>
                  <Button
                    onClick={() => setAddNodeDialog(true)}
                    disabled={!isLeader || !isClusterEnabled}
                    className="gap-2 bg-gradient-to-r from-purple-500 to-indigo-500 hover:from-purple-600 hover:to-indigo-600"
                  >
                    <Plus className="h-4 w-4" />
                    Agregar Nodo
                  </Button>
                </div>

                {/* Nodes Table */}
                <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent overflow-hidden">
                  <Table>
                    <TableHeader>
                      <TableRow className="border-white/[0.06] hover:bg-transparent">
                        <TableHead className="text-xs">Node ID</TableHead>
                        <TableHead className="text-xs">Dirección</TableHead>
                        <TableHead className="text-xs">Rol</TableHead>
                        <TableHead className="text-xs">Estado</TableHead>
                        <TableHead className="text-xs">Latencia</TableHead>
                        <TableHead className="text-xs">Última Actividad</TableHead>
                        <TableHead className="text-xs text-right">Acciones</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {nodes.map((node) => (
                        <TableRow 
                          key={node.id} 
                          className={cn(
                            "border-white/[0.06]",
                            node.id === currentNodeId && "bg-purple-500/5"
                          )}
                        >
                          <TableCell>
                            <div className="flex items-center gap-2">
                              <code className="font-mono text-sm">{node.id}</code>
                              {node.id === currentNodeId && (
                                <Badge variant="outline" className="text-[10px] px-1.5 py-0">
                                  este
                                </Badge>
                              )}
                            </div>
                          </TableCell>
                          <TableCell>
                            <code className="text-sm text-muted-foreground">{node.address}</code>
                          </TableCell>
                          <TableCell>
                            <RoleBadge role={node.role} />
                          </TableCell>
                          <TableCell>
                            <NodeStatusBadge state={node.state} />
                          </TableCell>
                          <TableCell>
                            {node.latency_ms !== undefined ? (
                              <span className={cn(
                                "text-sm",
                                node.latency_ms < 10 && "text-emerald-400",
                                node.latency_ms >= 10 && node.latency_ms < 50 && "text-amber-400",
                                node.latency_ms >= 50 && "text-red-400"
                              )}>
                                {node.latency_ms}ms
                              </span>
                            ) : (
                              <span className="text-sm text-muted-foreground">-</span>
                            )}
                          </TableCell>
                          <TableCell>
                            <span className="text-sm text-muted-foreground">
                              {node.last_seen ? formatTimeAgo(node.last_seen) : "-"}
                            </span>
                          </TableCell>
                          <TableCell className="text-right">
                            {node.id !== currentNodeId && node.role !== "leader" && (
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-8 w-8 p-0 text-red-400 hover:text-red-300 hover:bg-red-500/10"
                                onClick={() => setRemoveNodeDialog(node)}
                                disabled={!isLeader}
                              >
                                <Trash2 className="h-4 w-4" />
                              </Button>
                            )}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </Card>

                {/* Legend */}
                <div className="flex items-center gap-6 text-xs text-muted-foreground">
                  <div className="flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full bg-emerald-400" />
                    <span>Conectado</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full bg-amber-400" />
                    <span>Degradado</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full bg-red-400" />
                    <span>Desconectado</span>
                  </div>
                </div>
              </TabsContent>

              {/* Tab: Raft Log */}
              <TabsContent value="raft" className="space-y-4 mt-0">
                {/* Backend Note */}
                <Alert className="border-amber-500/20 bg-amber-500/5">
                  <AlertTriangle className="h-4 w-4 text-amber-400" />
                  <AlertTitle className="text-amber-300">Datos Simulados</AlertTitle>
                  <AlertDescription className="text-muted-foreground text-sm">
                    El log de Raft requiere un endpoint backend para exponer las entradas.
                    Endpoint necesario: <code className="text-amber-400">GET /v2/admin/cluster/log</code>
                  </AlertDescription>
                </Alert>

                {/* Log Explanation */}
                <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
                  <CardHeader className="pb-3">
                    <CardTitle className="text-lg flex items-center gap-2">
                      <History className="h-5 w-5 text-blue-400" />
                      Raft Log
                      <InfoTooltip content="El log de Raft contiene todas las operaciones replicadas en el cluster. Cada entrada tiene un índice único y un term que indica en qué época del cluster fue escrita." />
                    </CardTitle>
                    <CardDescription>
                      Historial de operaciones de consenso replicadas en el cluster
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="rounded-lg border border-white/[0.06] overflow-hidden">
                      <Table>
                        <TableHeader>
                          <TableRow className="border-white/[0.06] hover:bg-transparent bg-white/[0.02]">
                            <TableHead className="text-xs w-20">Index</TableHead>
                            <TableHead className="text-xs w-16">Term</TableHead>
                            <TableHead className="text-xs">Tipo</TableHead>
                            <TableHead className="text-xs">Tenant</TableHead>
                            <TableHead className="text-xs">Key</TableHead>
                            <TableHead className="text-xs text-right">Timestamp</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {raftLog.map((entry) => (
                            <TableRow key={entry.index} className="border-white/[0.06]">
                              <TableCell>
                                <code className="font-mono text-xs text-blue-400">{entry.index}</code>
                              </TableCell>
                              <TableCell>
                                <code className="font-mono text-xs">{entry.term}</code>
                              </TableCell>
                              <TableCell>
                                <Badge variant="outline" className="font-mono text-[10px]">
                                  {entry.type}
                                </Badge>
                              </TableCell>
                              <TableCell>
                                <code className="text-xs text-muted-foreground">
                                  {entry.tenant_id || "-"}
                                </code>
                              </TableCell>
                              <TableCell>
                                <code className="text-xs text-muted-foreground truncate max-w-[100px] block">
                                  {entry.key || "-"}
                                </code>
                              </TableCell>
                              <TableCell className="text-right">
                                <span className="text-xs text-muted-foreground">
                                  {formatTimeAgo(entry.timestamp)}
                                </span>
                              </TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </div>
                  </CardContent>
                </Card>
              </TabsContent>

              {/* Tab: Operations */}
              <TabsContent value="operations" className="space-y-4 mt-0">
                {/* Backend Note */}
                <Alert className="border-amber-500/20 bg-amber-500/5">
                  <AlertTriangle className="h-4 w-4 text-amber-400" />
                  <AlertTitle className="text-amber-300">Operaciones Avanzadas</AlertTitle>
                  <AlertDescription className="text-muted-foreground text-sm">
                    Estas operaciones requieren endpoints backend adicionales. Endpoints necesarios:
                    <code className="ml-2 text-amber-400">POST /v2/admin/cluster/snapshot</code>,
                    <code className="ml-2 text-amber-400">POST /v2/admin/cluster/leader-transfer</code>
                  </AlertDescription>
                </Alert>

                {!isLeader && isClusterEnabled && (
                  <Alert variant="default" className="border-blue-500/20 bg-blue-500/5">
                    <AlertCircle className="h-4 w-4 text-blue-400" />
                    <AlertTitle className="text-blue-300">Solo el Leader puede ejecutar operaciones</AlertTitle>
                    <AlertDescription className="text-muted-foreground">
                      Las operaciones de cluster solo pueden ser ejecutadas por el nodo leader.
                      Conecta al leader para administrar el cluster.
                    </AlertDescription>
                  </Alert>
                )}

                <div className="grid md:grid-cols-2 gap-4">
                  {/* Force Snapshot */}
                  <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
                    <CardHeader>
                      <CardTitle className="text-lg flex items-center gap-2">
                        <Download className="h-5 w-5 text-emerald-400" />
                        Crear Snapshot
                      </CardTitle>
                      <CardDescription>
                        Fuerza la creación de un snapshot del estado actual del cluster.
                        Los snapshots permiten recuperación rápida y compactan el log de Raft.
                      </CardDescription>
                    </CardHeader>
                    <CardContent>
                      <div className="space-y-4">
                        <div className="p-3 rounded-lg bg-white/[0.02] border border-white/[0.06]">
                          <p className="text-xs text-muted-foreground mb-1">Último snapshot</p>
                          <code className="text-sm">Index: {health.cluster.raft?.last_snapshot_index || "0"}</code>
                        </div>
                        <Button
                          onClick={() => forceSnapshotMutation.mutate()}
                          disabled={!isLeader || forceSnapshotMutation.isPending}
                          className="w-full gap-2"
                        >
                          {forceSnapshotMutation.isPending ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            <Download className="h-4 w-4" />
                          )}
                          Crear Snapshot Ahora
                        </Button>
                      </div>
                    </CardContent>
                  </Card>

                  {/* Transfer Leadership */}
                  <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
                    <CardHeader>
                      <CardTitle className="text-lg flex items-center gap-2">
                        <SkipForward className="h-5 w-5 text-amber-400" />
                        Transferir Liderazgo
                      </CardTitle>
                      <CardDescription>
                        Transfiere el rol de leader a otro nodo del cluster.
                        Útil para mantenimiento o balanceo de carga.
                      </CardDescription>
                    </CardHeader>
                    <CardContent>
                      <div className="space-y-4">
                        <Alert className="border-amber-500/20 bg-amber-500/5">
                          <AlertTriangle className="h-4 w-4 text-amber-400" />
                          <AlertDescription className="text-xs">
                            La transferencia de liderazgo puede causar una breve interrupción
                            en las operaciones de escritura mientras se elige el nuevo leader.
                          </AlertDescription>
                        </Alert>
                        <Button
                          onClick={() => transferLeadershipMutation.mutate()}
                          disabled={!isLeader || transferLeadershipMutation.isPending || totalNodes < 2}
                          variant="outline"
                          className="w-full gap-2 border-amber-500/30 text-amber-400 hover:bg-amber-500/10"
                        >
                          {transferLeadershipMutation.isPending ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            <SkipForward className="h-4 w-4" />
                          )}
                          Transferir Liderazgo
                        </Button>
                        {totalNodes < 2 && (
                          <p className="text-xs text-muted-foreground text-center">
                            Se necesitan al menos 2 nodos para transferir liderazgo
                          </p>
                        )}
                      </div>
                    </CardContent>
                  </Card>
                </div>

                {/* Configuration Reference */}
                <Card className="border-white/[0.08] bg-gradient-to-br from-white/[0.03] to-transparent">
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                      <Terminal className="h-5 w-5 text-zinc-400" />
                      Configuración del Cluster
                    </CardTitle>
                    <CardDescription>
                      Variables de entorno para configurar el cluster Raft
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="rounded-lg bg-zinc-900/50 border border-white/[0.06] p-4 font-mono text-xs overflow-x-auto">
                      <div className="space-y-2">
                        <div><span className="text-emerald-400"># Habilitar cluster</span></div>
                        <div><span className="text-blue-400">CLUSTER_ENABLE</span>=<span className="text-amber-400">true</span></div>
                        <div><span className="text-blue-400">CLUSTER_NODE_ID</span>=<span className="text-amber-400">node-1</span></div>
                        <div><span className="text-blue-400">CLUSTER_RAFT_ADDR</span>=<span className="text-amber-400">0.0.0.0:7000</span></div>
                        <div className="pt-2"><span className="text-emerald-400"># Peers estáticos (opcional)</span></div>
                        <div><span className="text-blue-400">CLUSTER_PEERS</span>=<span className="text-amber-400">node-1=10.0.0.1:7000,node-2=10.0.0.2:7000</span></div>
                        <div className="pt-2"><span className="text-emerald-400"># TLS para comunicación entre nodos</span></div>
                        <div><span className="text-blue-400">CLUSTER_TLS_ENABLE</span>=<span className="text-amber-400">true</span></div>
                        <div><span className="text-blue-400">CLUSTER_TLS_CERT_FILE</span>=<span className="text-amber-400">/certs/raft.crt</span></div>
                        <div><span className="text-blue-400">CLUSTER_TLS_KEY_FILE</span>=<span className="text-amber-400">/certs/raft.key</span></div>
                        <div><span className="text-blue-400">CLUSTER_TLS_CA_FILE</span>=<span className="text-amber-400">/certs/ca.crt</span></div>
                      </div>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="mt-3 gap-2"
                      onClick={() => copyToClipboard(`CLUSTER_ENABLE=true
CLUSTER_NODE_ID=node-1
CLUSTER_RAFT_ADDR=0.0.0.0:7000
CLUSTER_PEERS=node-1=10.0.0.1:7000,node-2=10.0.0.2:7000`)}
                    >
                      <Copy className="h-3 w-3" />
                      Copiar configuración
                    </Button>
                  </CardContent>
                </Card>
              </TabsContent>
            </Tabs>
          </>
        ) : (
          <Card className="border-white/[0.08]">
            <CardContent className="py-16 text-center">
              <XCircle className="h-12 w-12 mx-auto text-red-400 mb-4" />
              <p className="text-muted-foreground mb-4">No se pudo obtener información del cluster</p>
              <Button onClick={() => refetch()} variant="outline" className="gap-2">
                <RefreshCw className="h-4 w-4" />
                Reintentar
              </Button>
            </CardContent>
          </Card>
        )}

        {/* ─── Add Node Dialog ─── */}
        <Dialog open={addNodeDialog} onOpenChange={setAddNodeDialog}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <Plus className="h-5 w-5 text-purple-400" />
                Agregar Nodo al Cluster
              </DialogTitle>
              <DialogDescription>
                Agrega un nuevo nodo al cluster Raft. El nodo debe estar ejecutándose y accesible.
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="nodeId" className="flex items-center">
                  Node ID
                  <InfoTooltip content="Identificador único del nodo. Debe coincidir con CLUSTER_NODE_ID del nodo remoto." />
                </Label>
                <Input
                  id="nodeId"
                  value={newNodeForm.id}
                  onChange={(e) => setNewNodeForm({ ...newNodeForm, id: e.target.value })}
                  placeholder="node-2"
                  className="font-mono"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="nodeAddress" className="flex items-center">
                  Dirección Raft
                  <InfoTooltip content="Dirección IP y puerto Raft del nodo (CLUSTER_RAFT_ADDR del nodo remoto)." />
                </Label>
                <Input
                  id="nodeAddress"
                  value={newNodeForm.address}
                  onChange={(e) => setNewNodeForm({ ...newNodeForm, address: e.target.value })}
                  placeholder="10.0.0.2:7000"
                  className="font-mono"
                />
              </div>

              <Alert className="border-blue-500/20 bg-blue-500/5">
                <Info className="h-4 w-4 text-blue-400" />
                <AlertDescription className="text-xs">
                  El nodo remoto debe estar iniciado con <code className="text-blue-400">CLUSTER_DISABLE_BOOTSTRAP=true</code> para
                  unirse a un cluster existente sin intentar hacer bootstrap propio.
                </AlertDescription>
              </Alert>
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => setAddNodeDialog(false)}>
                Cancelar
              </Button>
              <Button
                onClick={handleAddNode}
                disabled={addNodeMutation.isPending || !newNodeForm.id.trim() || !newNodeForm.address.trim()}
                className="gap-2 bg-gradient-to-r from-purple-500 to-indigo-500 hover:from-purple-600 hover:to-indigo-600"
              >
                {addNodeMutation.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Plus className="h-4 w-4" />
                )}
                Agregar Nodo
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* ─── Remove Node Dialog ─── */}
        <Dialog open={!!removeNodeDialog} onOpenChange={() => setRemoveNodeDialog(null)}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2 text-red-400">
                <Trash2 className="h-5 w-5" />
                Remover Nodo del Cluster
              </DialogTitle>
              <DialogDescription>
                ¿Estás seguro de que deseas remover este nodo del cluster?
              </DialogDescription>
            </DialogHeader>

            {removeNodeDialog && (
              <div className="py-4">
                <div className="p-4 rounded-lg bg-white/[0.02] border border-white/[0.06] space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-muted-foreground">Node ID</span>
                    <code className="font-mono">{removeNodeDialog.id}</code>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-muted-foreground">Dirección</span>
                    <code className="font-mono text-sm">{removeNodeDialog.address}</code>
                  </div>
                </div>

                <Alert className="mt-4 border-red-500/20 bg-red-500/5">
                  <AlertTriangle className="h-4 w-4 text-red-400" />
                  <AlertDescription className="text-xs">
                    Esta acción no se puede deshacer. El nodo será removido del cluster y dejará de
                    recibir actualizaciones de replicación.
                  </AlertDescription>
                </Alert>
              </div>
            )}

            <DialogFooter>
              <Button variant="outline" onClick={() => setRemoveNodeDialog(null)}>
                Cancelar
              </Button>
              <Button
                onClick={() => removeNodeDialog && removeNodeMutation.mutate(removeNodeDialog.id)}
                disabled={removeNodeMutation.isPending}
                variant="destructive"
                className="gap-2"
              >
                {removeNodeMutation.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Trash2 className="h-4 w-4" />
                )}
                Remover Nodo
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </TooltipProvider>
  )
}

// ─── Page Export ───

export default function ClusterPage() {
  return (
    <Suspense
      fallback={
        <div className="flex items-center justify-center min-h-[400px]">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      }
    >
      <ClusterContent />
    </Suspense>
  )
}
