"use client"

import { useState, useEffect, useCallback, useRef } from "react"
import { useSearchParams, useRouter } from "next/navigation"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { API_ROUTES } from "@/lib/routes"
import { useI18n } from "@/lib/i18n"
import { useToast } from "@/hooks/use-toast"
import Link from "next/link"
import type { Tenant, TenantSettings } from "@/lib/types"

// DS Components (UI Unification)
import {
  Button,
  Card, CardContent, CardDescription, CardHeader, CardTitle,
  Input,
  Label,
  Switch,
  Tabs, TabsContent, TabsList, TabsTrigger,
  Badge,
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
  InlineAlert,
  Progress,
} from "@/components/ds"

// Icons
import {
  ChevronLeft,
  ArrowLeft,
  Save,
  Download,
  Upload,
  Settings,
  Palette,
  Shield,
  Globe,
  FileJson,
  Check,
  Copy,
  Eye,
  RefreshCw,
  AlertTriangle,
  Loader2,
  Building2,
  Image as ImageIcon,
  Paintbrush,
  Clock,
  Key,
  Lock,
  Timer,
  Link2,
  ExternalLink,
  HelpCircle,
  Sparkles,
  FileUp,
  FileDown,
  Package,
  CheckCircle2,
  XCircle,
  AlertCircle,
  Trash2
} from "lucide-react"

// ============================================================================
// TYPES
// ============================================================================

interface SettingsFormData {
  // General
  name: string
  slug: string
  display_name: string
  language: string
  
  // Branding
  logoUrl: string
  brandColor: string
  secondaryColor?: string  // Future: not in backend yet
  favicon?: string         // Future: not in backend yet
  
  // Security
  sessionLifetimeSeconds: number
  refreshTokenLifetimeSeconds: number
  mfaEnabled: boolean
  socialLoginEnabled: boolean
  security?: {
    passwordMinLength?: number
    mfaRequired?: boolean
  }
  
  // Issuer
  issuerMode: "path" | "subdomain" | "global"
  issuerOverride?: string
}

interface ExportData {
  version: string
  exportedAt: string
  tenant: {
    name: string
    slug: string
    display_name?: string
    language?: string
  }
  settings: TenantSettings
  clients?: any[]
  scopes?: any[]
  claims?: any[]
  users?: any[]
  roles?: any[]
}

// ============================================================================
// CONSTANTS
// ============================================================================

const DEFAULT_BRAND_COLORS = [
  { name: "Índigo", value: "#5E6AD2" },
  { name: "Azul", value: "#3B82F6" },
  { name: "Verde", value: "#10B981" },
  { name: "Rojo", value: "#EF4444" },
  { name: "Naranja", value: "#F97316" },
  { name: "Púrpura", value: "#8B5CF6" },
  { name: "Rosa", value: "#EC4899" },
  { name: "Cian", value: "#06B6D4" },
]

const LANGUAGES = [
  { code: "es", name: "Español" },
  { code: "en", name: "English" },
  { code: "pt", name: "Português" },
]

const ISSUER_MODES = [
  { 
    value: "path", 
    label: "Path-based", 
    description: "https://auth.example.com/tenant-slug",
    example: "/acme"
  },
  { 
    value: "subdomain", 
    label: "Subdomain", 
    description: "https://tenant-slug.auth.example.com",
    example: "acme.auth.example.com"
  },
  { 
    value: "global", 
    label: "Global (Custom)", 
    description: "URL personalizada definida en issuerOverride",
    example: "https://auth.acme.com"
  },
]

const SESSION_PRESETS = [
  { label: "15 minutos", seconds: 900, description: "Alta seguridad" },
  { label: "1 hora", seconds: 3600, description: "Recomendado" },
  { label: "8 horas", seconds: 28800, description: "Jornada laboral" },
  { label: "24 horas", seconds: 86400, description: "Uso diario" },
  { label: "7 días", seconds: 604800, description: "Recordar sesión" },
]

const REFRESH_TOKEN_PRESETS = [
  { label: "1 hora", seconds: 3600, description: "Corta duración" },
  { label: "24 horas", seconds: 86400, description: "Estándar" },
  { label: "7 días", seconds: 604800, description: "Recomendado" },
  { label: "30 días", seconds: 2592000, description: "Larga duración" },
  { label: "90 días", seconds: 7776000, description: "Extendido" },
]

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds} segundos`
  if (seconds < 3600) return `${Math.floor(seconds / 60)} minutos`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} horas`
  return `${Math.floor(seconds / 86400)} días`
}

function isValidColor(color: string): boolean {
  return /^#([0-9A-Fa-f]{3}){1,2}$/.test(color)
}

function isValidSlug(slug: string): boolean {
  return /^[a-z0-9]+(-[a-z0-9]+)*$/.test(slug)
}

// ============================================================================
// SUB-COMPONENTS
// ============================================================================

// Color Picker with presets
function ColorPicker({ 
  value, 
  onChange, 
  label,
  description 
}: { 
  value: string
  onChange: (color: string) => void
  label: string
  description?: string
}) {
  const [customColor, setCustomColor] = useState(value || "#5E6AD2")
  
  return (
    <div className="space-y-3">
      <div>
        <Label className="text-sm font-medium">{label}</Label>
        {description && (
          <p className="text-xs text-muted-foreground mt-0.5">{description}</p>
        )}
      </div>
      
      {/* Preset Colors */}
      <div className="flex flex-wrap gap-2">
        {DEFAULT_BRAND_COLORS.map((color) => (
          <Tooltip key={color.value}>
            <TooltipTrigger asChild>
              <button
                type="button"
                onClick={() => {
                  onChange(color.value)
                  setCustomColor(color.value)
                }}
                className={`
                  w-8 h-8 rounded-lg border-2 transition-all duration-200
                  hover:scale-110 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-background
                  ${value === color.value 
                    ? "border-foreground ring-2 ring-foreground/20 scale-110" 
                    : "border-transparent hover:border-muted-foreground/30"
                  }
                `}
                style={{ backgroundColor: color.value }}
              />
            </TooltipTrigger>
            <TooltipContent>
              <p>{color.name}</p>
              <p className="text-xs text-muted-foreground font-mono">{color.value}</p>
            </TooltipContent>
          </Tooltip>
        ))}
      </div>
      
      {/* Custom Color Input */}
      <div className="flex items-center gap-3">
        <div className="relative">
          <input
            type="color"
            value={customColor}
            onChange={(e) => {
              setCustomColor(e.target.value)
              onChange(e.target.value)
            }}
            className="w-10 h-10 rounded-lg cursor-pointer border border-input"
          />
        </div>
        <Input
          value={customColor}
          onChange={(e) => {
            setCustomColor(e.target.value)
            if (isValidColor(e.target.value)) {
              onChange(e.target.value)
            }
          }}
          placeholder="#5E6AD2"
          className="w-28 font-mono text-sm"
        />
        <span className="text-xs text-muted-foreground">Color personalizado</span>
      </div>
    </div>
  )
}

// Logo Uploader with preview
function LogoUploader({
  value,
  onChange,
  brandColor
}: {
  value: string | null
  onChange: (logo: string | null) => void
  brandColor: string
}) {
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [previewBg, setPreviewBg] = useState<"light" | "dark" | "brand">("light")
  const [isDragging, setIsDragging] = useState(false)
  
  const handleFile = useCallback((file: File) => {
    if (file.size > 5 * 1024 * 1024) {
      alert("El archivo es muy grande. Máximo 5MB.")
      return
    }
    
    const reader = new FileReader()
    reader.onloadend = () => {
      onChange(reader.result as string)
    }
    reader.readAsDataURL(file)
  }, [onChange])
  
  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setIsDragging(false)
    
    const file = e.dataTransfer.files[0]
    if (file && file.type.startsWith("image/")) {
      handleFile(file)
    }
  }, [handleFile])
  
  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setIsDragging(true)
  }, [])
  
  const handleDragLeave = useCallback(() => {
    setIsDragging(false)
  }, [])
  
  const bgStyles = {
    light: "bg-white",
    dark: "bg-zinc-900",
    brand: ""
  }
  
  return (
    <div className="space-y-4">
      <div className="flex items-start gap-6">
        {/* Upload Area */}
        <div
          onDrop={handleDrop}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onClick={() => fileInputRef.current?.click()}
          className={`
            relative flex-1 min-h-[160px] rounded-xl border-2 border-dashed 
            transition-all duration-200 cursor-pointer group
            flex flex-col items-center justify-center gap-3 p-6
            ${isDragging 
              ? "border-primary bg-primary/5 scale-[1.02]" 
              : "border-muted-foreground/25 hover:border-primary/50 hover:bg-muted/30"
            }
            ${value ? "border-solid" : ""}
          `}
        >
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            onChange={(e) => {
              const file = e.target.files?.[0]
              if (file) handleFile(file)
            }}
            className="hidden"
          />
          
          {value ? (
            <div className="relative w-full h-full min-h-[120px] flex items-center justify-center">
              <div 
                className={`
                  absolute inset-0 rounded-lg transition-colors
                  ${bgStyles[previewBg]}
                `}
                style={previewBg === "brand" ? { backgroundColor: brandColor } : undefined}
              />
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={value}
                alt="Logo preview"
                className="relative z-10 max-h-[100px] max-w-full object-contain"
              />
              <Button
                variant="danger"
                size="sm"
                className="absolute top-2 right-2 z-20 h-7 w-7 opacity-0 group-hover:opacity-100 transition-opacity"
                onClick={(e) => {
                  e.stopPropagation()
                  onChange(null)
                }}
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </div>
          ) : (
            <>
              <div className="p-3 rounded-full bg-muted">
                <Upload className="h-6 w-6 text-muted-foreground" />
              </div>
              <div className="text-center">
                <p className="text-sm font-medium">Click o arrastra tu logo aquí</p>
                <p className="text-xs text-muted-foreground mt-1">
                  SVG, PNG, JPG o GIF (máx. 5MB)
                </p>
              </div>
            </>
          )}
        </div>
        
        {/* Preview Controls */}
        {value && (
          <div className="flex flex-col gap-2 animate-in slide-in-from-left-2">
            <span className="text-xs font-medium text-muted-foreground">Preview</span>
            <div className="flex flex-col gap-1.5">
              <Button
                variant={previewBg === "light" ? "default" : "outline"}
                size="sm"
                className="h-8 w-8 p-0"
                onClick={() => setPreviewBg("light")}
              >
                <div className="w-4 h-4 rounded bg-white border" />
              </Button>
              <Button
                variant={previewBg === "dark" ? "default" : "outline"}
                size="sm"
                className="h-8 w-8 p-0"
                onClick={() => setPreviewBg("dark")}
              >
                <div className="w-4 h-4 rounded bg-zinc-900 border" />
              </Button>
              <Button
                variant={previewBg === "brand" ? "default" : "outline"}
                size="sm"
                className="h-8 w-8 p-0"
                onClick={() => setPreviewBg("brand")}
              >
                <div 
                  className="w-4 h-4 rounded border"
                  style={{ backgroundColor: brandColor }}
                />
              </Button>
            </div>
          </div>
        )}
      </div>
      
      {/* URL Input alternative */}
      <div className="flex items-center gap-2">
        <div className="flex-1 h-px bg-border" />
        <span className="text-xs text-muted-foreground">o pega una URL</span>
        <div className="flex-1 h-px bg-border" />
      </div>
      
      <Input
        value={value?.startsWith("http") ? value : ""}
        onChange={(e) => onChange(e.target.value || null)}
        placeholder="https://example.com/logo.png"
        className="text-sm"
      />
    </div>
  )
}

// Duration Selector with presets
function DurationSelector({
  value,
  onChange,
  presets,
  label,
  description,
  min = 60,
  max = 31536000
}: {
  value: number
  onChange: (seconds: number) => void
  presets: { label: string; seconds: number; description: string }[]
  label: string
  description?: string
  min?: number
  max?: number
}) {
  const [isCustom, setIsCustom] = useState(
    !presets.some(p => p.seconds === value)
  )
  
  return (
    <div className="space-y-3">
      <div>
        <Label className="text-sm font-medium">{label}</Label>
        {description && (
          <p className="text-xs text-muted-foreground mt-0.5">{description}</p>
        )}
      </div>
      
      <div className="grid grid-cols-3 gap-2">
        {presets.map((preset) => (
          <button
            key={preset.seconds}
            type="button"
            onClick={() => {
              onChange(preset.seconds)
              setIsCustom(false)
            }}
            className={`
              p-3 rounded-lg border text-left transition-all duration-200
              hover:border-primary/50 hover:bg-muted/50
              ${value === preset.seconds && !isCustom
                ? "border-primary bg-primary/10 ring-1 ring-primary/20"
                : "border-border"
              }
            `}
          >
            <div className="text-sm font-medium">{preset.label}</div>
            <div className="text-xs text-muted-foreground">{preset.description}</div>
          </button>
        ))}
      </div>
      
      <div className="flex items-center gap-3">
        <Switch
          checked={isCustom}
          onCheckedChange={(checked) => setIsCustom(checked)}
        />
        <span className="text-sm">Valor personalizado</span>
      </div>
      
      {isCustom && (
        <div className="flex items-center gap-3 animate-in slide-in-from-top-2">
          <Input
            type="number"
            value={value}
            onChange={(e) => onChange(Math.max(min, Math.min(max, parseInt(e.target.value) || min)))}
            min={min}
            max={max}
            className="w-32"
          />
          <span className="text-sm text-muted-foreground">
            segundos = {formatDuration(value)}
          </span>
        </div>
      )}
    </div>
  )
}

// Branding Preview Card
function BrandingPreview({
  logo,
  brandColor,
  tenantName
}: {
  logo: string | null
  brandColor: string
  tenantName: string
}) {
  return (
    <Card className="overflow-hidden">
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-medium flex items-center gap-2">
          <Eye className="h-4 w-4" />
          Vista previa
        </CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        {/* Mock Login Page */}
        <div 
          className="p-6 min-h-[280px] flex flex-col items-center justify-center gap-6"
          style={{ backgroundColor: `${brandColor}15` }}
        >
          {/* Logo / Initial */}
          <div className="flex flex-col items-center gap-3">
            {logo ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img 
                src={logo} 
                alt="Logo" 
                className="h-12 w-auto object-contain"
              />
            ) : (
              <div 
                className="h-14 w-14 rounded-xl flex items-center justify-center text-white font-bold text-xl"
                style={{ backgroundColor: brandColor }}
              >
                {tenantName.charAt(0).toUpperCase()}
              </div>
            )}
            <span className="text-lg font-semibold">{tenantName || "Mi Organización"}</span>
          </div>
          
          {/* Mock Form */}
          <div className="w-full max-w-[260px] space-y-3">
            <div className="h-10 rounded-lg bg-background border" />
            <div className="h-10 rounded-lg bg-background border" />
            <div 
              className="h-10 rounded-lg flex items-center justify-center text-white text-sm font-medium"
              style={{ backgroundColor: brandColor }}
            >
              Iniciar Sesión
            </div>
          </div>
          
          {/* Footer */}
          <p className="text-xs text-muted-foreground">
            Así verán tus usuarios la página de login
          </p>
        </div>
      </CardContent>
    </Card>
  )
}

// Export Progress Dialog
// ISS-11-03: Export Dialog - Now uses single backend endpoint instead of 5 separate calls
function ExportDialog({
  open,
  onOpenChange,
  tenantId,
  tenantSlug,
  settings
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  tenantId: string
  tenantSlug: string
  settings: any
}) {
  const [progress, setProgress] = useState(0)
  const [status, setStatus] = useState<"idle" | "exporting" | "done" | "error">("idle")
  const [exportOptions, setExportOptions] = useState({
    includeClients: true,
    includeScopes: true,
    includeUsers: false,
    includeRoles: true,
    includeClaims: true
  })
  const { toast } = useToast()
  
  const handleExport = async () => {
    setStatus("exporting")
    setProgress(30)
    
    try {
      // ISS-11-03: Use single backend endpoint instead of 5 separate API calls
      const exportUrl = API_ROUTES.ADMIN_TENANT_EXPORT(tenantId, {
        clients: exportOptions.includeClients,
        scopes: exportOptions.includeScopes,
        users: exportOptions.includeUsers,
        roles: exportOptions.includeRoles,
      })
      
      setProgress(60)
      const exportData = await api.get<ExportData>(exportUrl)
      
      setProgress(90)
      
      // Generate and download file
      const blob = new Blob([JSON.stringify(exportData, null, 2)], { 
        type: "application/json" 
      })
      const url = URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = `hellojohn-tenant-${tenantSlug}-${new Date().toISOString().split('T')[0]}.json`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      
      setProgress(100)
      setStatus("done")
      
      toast({
        title: "Exportación completada",
        description: "El archivo se ha descargado correctamente.",
      })
      
      setTimeout(() => {
        onOpenChange(false)
        setStatus("idle")
        setProgress(0)
      }, 1500)
      
    } catch (error: any) {
      setStatus("error")
      toast({
        title: "Error al exportar",
        description: error?.error_description || "No se pudo generar el archivo de configuración.",
        variant: "destructive"
      })
    }
  }
  
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <FileDown className="h-5 w-5" />
            Exportar Configuración
          </DialogTitle>
          <DialogDescription>
            Descarga un archivo JSON con la configuración completa del tenant.
          </DialogDescription>
        </DialogHeader>
        
        {status === "idle" && (
          <>
            <div className="space-y-4 py-4">
              <p className="text-sm text-muted-foreground">
                Selecciona qué datos incluir en la exportación:
              </p>
              
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Package className="h-4 w-4 text-muted-foreground" />
                    <span className="text-sm">Configuración del tenant</span>
                  </div>
                  <Badge variant="secondary">Siempre incluido</Badge>
                </div>
                
                <hr className="border-border" />
                
                {[
                  { key: "includeClients", label: "Clientes OAuth2", icon: Key },
                  { key: "includeScopes", label: "Scopes", icon: Lock },
                  { key: "includeClaims", label: "Claims personalizados", icon: FileJson },
                  { key: "includeRoles", label: "Roles y permisos", icon: Shield },
                  { key: "includeUsers", label: "Usuarios", icon: Building2, warning: true },
                ].map(({ key, label, icon: Icon, warning }) => (
                  <div key={key} className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Icon className="h-4 w-4 text-muted-foreground" />
                      <span className="text-sm">{label}</span>
                      {warning && (
                        <Tooltip>
                          <TooltipTrigger>
                            <AlertTriangle className="h-3.5 w-3.5 text-warning" />
                          </TooltipTrigger>
                          <TooltipContent>
                            <p className="max-w-xs text-xs">
                              Incluir usuarios puede generar un archivo muy grande 
                              y contener información sensible.
                            </p>
                          </TooltipContent>
                        </Tooltip>
                      )}
                    </div>
                    <Switch
                      checked={exportOptions[key as keyof typeof exportOptions]}
                      onCheckedChange={(checked) => 
                        setExportOptions(prev => ({ ...prev, [key]: checked }))
                      }
                    />
                  </div>
                ))}
              </div>
            </div>
            
            <DialogFooter>
              <Button variant="outline" onClick={() => onOpenChange(false)}>
                Cancelar
              </Button>
              <Button onClick={handleExport}>
                <Download className="mr-2 h-4 w-4" />
                Exportar
              </Button>
            </DialogFooter>
          </>
        )}
        
        {status === "exporting" && (
          <div className="py-8 space-y-4">
            <div className="flex flex-col items-center gap-3">
              <Loader2 className="h-8 w-8 animate-spin text-primary" />
              <p className="text-sm">Exportando configuración...</p>
            </div>
            <Progress value={progress} className="h-2" />
          </div>
        )}
        
        {status === "done" && (
          <div className="py-8 flex flex-col items-center gap-3">
            <div className="p-3 rounded-full bg-success/10">
              <CheckCircle2 className="h-8 w-8 text-success" />
            </div>
            <p className="text-sm font-medium">¡Exportación completada!</p>
          </div>
        )}
        
        {status === "error" && (
          <div className="py-8 flex flex-col items-center gap-3">
            <div className="p-3 rounded-full bg-danger/10">
              <XCircle className="h-8 w-8 text-danger" />
            </div>
            <p className="text-sm font-medium">Error al exportar</p>
            <Button variant="outline" size="sm" onClick={() => setStatus("idle")}>
              Reintentar
            </Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

// ISS-11-02: Import Dialog - Now enabled with backend endpoints
function ImportDialog({
  open,
  onOpenChange,
  tenantId,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  tenantId: string
}) {
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<ExportData | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [validationResult, setValidationResult] = useState<{ valid: boolean; warnings?: string[]; errors?: string[] } | null>(null)
  const [isValidating, setIsValidating] = useState(false)
  const [isImporting, setIsImporting] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  
  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const selectedFile = e.target.files?.[0]
    if (!selectedFile) return
    
    setFile(selectedFile)
    setError(null)
    setValidationResult(null)
    
    const reader = new FileReader()
    reader.onload = (event) => {
      try {
        const data = JSON.parse(event.target?.result as string)
        if (!data.version || !data.tenant || !data.settings) {
          setError("El archivo no tiene el formato correcto de exportación HelloJohn.")
          setPreview(null)
          return
        }
        setPreview(data)
      } catch {
        setError("El archivo no es un JSON válido.")
        setPreview(null)
      }
    }
    reader.readAsText(selectedFile)
  }

  // ISS-11-02: Validate import before applying
  const handleValidate = async () => {
    if (!preview || !tenantId) return
    
    setIsValidating(true)
    setError(null)
    
    try {
      const result = await api.post<{ valid: boolean; warnings?: string[]; errors?: string[] }>(
        API_ROUTES.ADMIN_TENANT_IMPORT_VALIDATE(tenantId),
        preview
      )
      setValidationResult(result)
      
      if (result.valid) {
        toast({
          title: "Validación exitosa",
          description: "El archivo es válido y puede ser importado.",
        })
      } else {
        toast({
          title: "Validación fallida",
          description: "Hay errores que deben corregirse antes de importar.",
          variant: "destructive",
        })
      }
    } catch (err: any) {
      setError(err?.error_description || err?.message || "Error al validar el archivo")
    } finally {
      setIsValidating(false)
    }
  }

  // ISS-11-02: Perform the actual import
  const handleImport = async () => {
    if (!preview || !tenantId) return
    
    setIsImporting(true)
    setError(null)
    
    try {
      await api.put(API_ROUTES.ADMIN_TENANT_IMPORT(tenantId), preview)
      
      toast({
        title: "Importación exitosa",
        description: "La configuración ha sido importada correctamente.",
      })
      
      // Invalidate queries to refresh data
      queryClient.invalidateQueries({ queryKey: ["tenant", tenantId] })
      queryClient.invalidateQueries({ queryKey: ["tenants"] })
      
      onOpenChange(false)
    } catch (err: any) {
      setError(err?.error_description || err?.message || "Error al importar la configuración")
      toast({
        title: "Error de importación",
        description: err?.error_description || "No se pudo importar la configuración",
        variant: "destructive",
      })
    } finally {
      setIsImporting(false)
    }
  }
  
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <FileUp className="h-5 w-5" />
            Importar Configuración
          </DialogTitle>
          <DialogDescription>
            Restaura la configuración de un tenant desde un archivo de exportación.
          </DialogDescription>
        </DialogHeader>
        
        <InlineAlert variant="warning">
            <strong>Precaución:</strong> La importación sobrescribirá la configuración existente.
            Se recomienda hacer un backup antes de continuar.
        </InlineAlert>
        
        <div className="space-y-4 py-4">
          {/* File Drop Zone */}
          <div
            onClick={() => fileInputRef.current?.click()}
            className={`
              border-2 border-dashed rounded-lg p-8 text-center cursor-pointer
              transition-colors hover:border-primary/50 hover:bg-muted/30
              ${file ? "border-primary bg-primary/5" : "border-muted-foreground/25"}
            `}
          >
            <input
              ref={fileInputRef}
              type="file"
              accept=".json"
              onChange={handleFileChange}
              className="hidden"
            />
            {file ? (
              <div className="flex items-center justify-center gap-3">
                <FileJson className="h-8 w-8 text-primary" />
                <div className="text-left">
                  <p className="text-sm font-medium">{file.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {(file.size / 1024).toFixed(1)} KB
                  </p>
                </div>
              </div>
            ) : (
              <>
                <Upload className="h-8 w-8 mx-auto text-muted-foreground mb-2" />
                <p className="text-sm">Arrastra un archivo JSON o click para seleccionar</p>
              </>
            )}
          </div>
          
          {error && (
            <InlineAlert variant="destructive">{error}</InlineAlert>
          )}

          {/* Validation Result */}
          {validationResult && (
            <InlineAlert variant={validationResult.valid ? "success" : "destructive"}>
                {validationResult.valid ? (
                  "El archivo es válido y puede ser importado."
                ) : (
                  <ul className="list-disc list-inside">
                    {validationResult.errors?.map((err, i) => (
                      <li key={i}>{err}</li>
                    ))}
                  </ul>
                )}
                {validationResult.warnings && validationResult.warnings.length > 0 && (
                  <div className="mt-2 text-warning">
                    <strong>Advertencias:</strong>
                    <ul className="list-disc list-inside">
                      {validationResult.warnings.map((warn, i) => (
                        <li key={i}>{warn}</li>
                      ))}
                    </ul>
                  </div>
                )}
            </InlineAlert>
          )}
          
          {/* Preview */}
          {preview && (
            <div className="rounded-lg border bg-muted/30 p-4 space-y-3">
              <h4 className="text-sm font-medium flex items-center gap-2">
                <Eye className="h-4 w-4" />
                Vista previa del archivo
              </h4>
              <div className="grid grid-cols-2 gap-2 text-sm">
                <div>
                  <span className="text-muted-foreground">Tenant:</span>
                  <span className="ml-2 font-medium">{preview.tenant.name}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Slug:</span>
                  <span className="ml-2 font-mono text-xs">{preview.tenant.slug}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Exportado:</span>
                  <span className="ml-2">{new Date(preview.exportedAt).toLocaleDateString()}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Versión:</span>
                  <span className="ml-2">{preview.version}</span>
                </div>
              </div>
              
              <hr className="border-border" />
              
              <div className="flex flex-wrap gap-2">
                {preview.clients && preview.clients.length > 0 && (
                  <Badge variant="secondary">{preview.clients.length} clients</Badge>
                )}
                {preview.scopes && preview.scopes.length > 0 && (
                  <Badge variant="secondary">{preview.scopes.length} scopes</Badge>
                )}
                {preview.roles && preview.roles.length > 0 && (
                  <Badge variant="secondary">{preview.roles.length} roles</Badge>
                )}
                {preview.users && preview.users.length > 0 && (
                  <Badge variant="secondary">{preview.users.length} users</Badge>
                )}
              </div>
            </div>
          )}
        </div>
        
        <DialogFooter className="gap-2">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancelar
          </Button>
          {preview && !validationResult?.valid && (
            <Button variant="secondary" onClick={handleValidate} disabled={isValidating}>
              {isValidating ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Validando...
                </>
              ) : (
                <>
                  <CheckCircle2 className="mr-2 h-4 w-4" />
                  Validar
                </>
              )}
            </Button>
          )}
          <Button 
            onClick={handleImport} 
            disabled={!preview || isImporting || (validationResult !== null && !validationResult.valid)}
          >
            {isImporting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Importando...
              </>
            ) : (
              <>
                <Upload className="mr-2 h-4 w-4" />
                Importar
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ============================================================================
// MAIN COMPONENT
// ============================================================================

export default function TenantSettingsPage() {
  const search = useSearchParams()
  const router = useRouter()
  const { t } = useI18n()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const tenantId = search.get("id") as string
  
  // State
  const [activeTab, setActiveTab] = useState("general")
  const [hasChanges, setHasChanges] = useState(false)
  const [showExportDialog, setShowExportDialog] = useState(false)
  const [showImportDialog, setShowImportDialog] = useState(false)
  
  // Form State
  const [formData, setFormData] = useState<SettingsFormData>({
    name: "",
    slug: "",
    display_name: "",
    language: "es",
    logoUrl: "",
    brandColor: "#5E6AD2",
    sessionLifetimeSeconds: 3600,
    refreshTokenLifetimeSeconds: 604800,
    mfaEnabled: false,
    socialLoginEnabled: false,
    issuerMode: "path",
    issuerOverride: ""
  })
  
  // Queries
  const { data: tenant, isLoading: loadingTenant } = useQuery({
    queryKey: ["tenant", tenantId],
    enabled: !!tenantId,
    queryFn: () => api.get<Tenant>(`/v2/admin/tenants/${tenantId}`),
  })
  
  const { data: settings, isLoading: loadingSettings } = useQuery({
    queryKey: ["tenant-settings", tenantId, "v2"],
    enabled: !!tenantId,
    queryFn: async () => {
      const token = (await import("@/lib/auth-store")).useAuthStore.getState().token
      const resp = await fetch(`${api.getBaseUrl()}/v2/admin/tenants/${tenantId}/settings`, {
        headers: {
          Authorization: token ? `Bearer ${token}` : "",
        },
      })
      const etag = resp.headers.get("ETag") || undefined
      const data = await resp.json()
      return { ...data, _etag: etag }
    },
  })
  
  // Initialize form data
  useEffect(() => {
    if (tenant && settings) {
      setFormData({
        name: tenant.name || "",
        slug: tenant.slug || "",
        display_name: tenant.display_name || "",
        language: "es", // Not in current API
        logoUrl: settings.logoUrl || "",
        brandColor: settings.brandColor || "#5E6AD2",
        sessionLifetimeSeconds: settings.sessionLifetimeSeconds || 3600,
        refreshTokenLifetimeSeconds: settings.refreshTokenLifetimeSeconds || 604800,
        mfaEnabled: settings.mfaEnabled || false,
        socialLoginEnabled: settings.socialLoginEnabled || false,
        issuerMode: (settings.issuerMode as any) || "path",
        issuerOverride: settings.issuerOverride || "",
        security: settings.security
      })
      setHasChanges(false)
    }
  }, [tenant, settings])
  
  // Track changes
  const updateForm = useCallback((updates: Partial<SettingsFormData>) => {
    setFormData(prev => ({ ...prev, ...updates }))
    setHasChanges(true)
  }, [])
  
  // Mutations
  const updateTenantMutation = useMutation({
    mutationFn: (data: Partial<Tenant>) => 
      api.put<Tenant>(`/v2/admin/tenants/${tenantId}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tenant", tenantId] })
      queryClient.invalidateQueries({ queryKey: ["tenants"] })
    },
  })
  
  const updateSettingsMutation = useMutation({
    mutationFn: (data: any) => {
      const etag = settings?._etag
      if (!etag) throw new Error("Missing ETag. Please refresh.")
      return api.put<any>(`/v2/admin/tenants/${tenantId}/settings`, data, etag)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tenant-settings", tenantId, "v2"] })
    },
  })
  
  // Save handler
  const handleSave = async () => {
    try {
      // Update tenant basic info
      await updateTenantMutation.mutateAsync({
        name: formData.name,
        slug: formData.slug,
        display_name: formData.display_name,
      })
      
      // Update settings
      await updateSettingsMutation.mutateAsync({
        logoUrl: formData.logoUrl,
        brandColor: formData.brandColor,
        sessionLifetimeSeconds: formData.sessionLifetimeSeconds,
        refreshTokenLifetimeSeconds: formData.refreshTokenLifetimeSeconds,
        mfaEnabled: formData.mfaEnabled,
        socialLoginEnabled: formData.socialLoginEnabled,
        issuerMode: formData.issuerMode,
        issuerOverride: formData.issuerOverride || undefined,
        security: formData.security
      })
      
      setHasChanges(false)
      toast({
        title: "Configuración guardada",
        description: "Los cambios se aplicaron correctamente.",
      })
    } catch (error: any) {
      toast({
        title: "Error al guardar",
        description: error.message || "No se pudieron guardar los cambios.",
        variant: "destructive"
      })
    }
  }
  
  // Loading state
  if (loadingTenant || loadingSettings) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="flex flex-col items-center gap-3">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
          <p className="text-sm text-muted-foreground">Cargando configuración...</p>
        </div>
      </div>
    )
  }
  
  // No tenant selected
  if (!tenantId) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center space-y-3">
          <Settings className="h-12 w-12 mx-auto text-muted-foreground" />
          <h2 className="text-lg font-semibold">No hay tenant seleccionado</h2>
          <p className="text-sm text-muted-foreground">
            Selecciona un tenant para ver su configuración.
          </p>
          <Button asChild>
            <Link href="/admin/tenants">Ver Tenants</Link>
          </Button>
        </div>
      </div>
    )
  }
  
  const isSaving = updateTenantMutation.isPending || updateSettingsMutation.isPending
  
  return (
    <TooltipProvider>
      <div className="space-y-6 animate-in fade-in duration-500">
        {/* Header */}
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-4">
            <Button variant="ghost" size="sm" asChild>
              <Link href={`/admin/tenants/detail?id=${tenantId}`}>
                <ArrowLeft className="h-4 w-4" />
              </Link>
            </Button>
            <div className="flex items-center gap-3">
              <div className="relative group">
                <div className="absolute inset-0 bg-gradient-to-br from-muted-foreground/30 to-accent/20 rounded-xl blur-xl group-hover:blur-2xl transition-all duration-500" />
              </div>
              <div>
                <h1 className="text-2xl font-bold tracking-tight">Configuración</h1>
                <p className="text-sm text-muted-foreground">
                  {tenant?.name} — Ajustes generales del tenant
                </p>
              </div>
            </div>
          </div>
          
          <div className="flex items-center gap-2">
            {hasChanges && (
              <Badge variant="outline" className="text-warning border-warning/30">
                Cambios sin guardar
              </Badge>
            )}
            <Button 
              variant="outline" 
              size="sm"
              onClick={() => setShowImportDialog(true)}
            >
              <Upload className="mr-2 h-4 w-4" />
              Importar
            </Button>
            <Button 
              variant="outline" 
              size="sm"
              onClick={() => setShowExportDialog(true)}
            >
              <Download className="mr-2 h-4 w-4" />
              Exportar
            </Button>
            <Button 
              onClick={handleSave}
              disabled={!hasChanges || isSaving}
              className="min-w-[100px]"
            >
              {isSaving ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <Save className="mr-2 h-4 w-4" />
              )}
              {isSaving ? "Guardando..." : "Guardar"}
            </Button>
          </div>
        </div>
        
        {/* Info Banner */}
        <InlineAlert variant="info">
            Configura la identidad visual, políticas de seguridad y comportamiento de tu organización.
            Los cambios se aplican a todas las aplicaciones y usuarios del tenant.
        </InlineAlert>
        
        {/* Main Content */}
        <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-6">
          <TabsList className="bg-muted/50 p-1">
            <TabsTrigger value="general" className="gap-2">
              <Building2 className="h-4 w-4" />
              General
            </TabsTrigger>
            <TabsTrigger value="branding" className="gap-2">
              <Palette className="h-4 w-4" />
              Branding
            </TabsTrigger>
            <TabsTrigger value="security" className="gap-2">
              <Shield className="h-4 w-4" />
              Seguridad
            </TabsTrigger>
            <TabsTrigger value="issuer" className="gap-2">
              <Globe className="h-4 w-4" />
              Issuer
            </TabsTrigger>
            <TabsTrigger value="export" className="gap-2">
              <FileJson className="h-4 w-4" />
              Export/Import
            </TabsTrigger>
          </TabsList>
          
          {/* ============================================================== */}
          {/* TAB: GENERAL */}
          {/* ============================================================== */}
          <TabsContent value="general" className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Building2 className="h-5 w-5" />
                  Información General
                </CardTitle>
                <CardDescription>
                  Datos básicos de identificación del tenant.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="grid grid-cols-2 gap-6">
                  <div className="space-y-2">
                    <Label htmlFor="name">Nombre *</Label>
                    <Input
                      id="name"
                      value={formData.name}
                      onChange={(e) => updateForm({ name: e.target.value })}
                      placeholder="Mi Organización"
                    />
                    <p className="text-xs text-muted-foreground">
                      Nombre visible para usuarios y administradores.
                    </p>
                  </div>
                  
                  <div className="space-y-2">
                    <Label htmlFor="display_name">Nombre para mostrar</Label>
                    <Input
                      id="display_name"
                      value={formData.display_name}
                      onChange={(e) => updateForm({ display_name: e.target.value })}
                      placeholder="Mi Org S.A. de C.V."
                    />
                    <p className="text-xs text-muted-foreground">
                      Nombre legal o formal (opcional).
                    </p>
                  </div>
                </div>
                
                <div className="grid grid-cols-2 gap-6">
                  <div className="space-y-2">
                    <Label htmlFor="slug">
                      Slug (identificador único) *
                      <Tooltip>
                        <TooltipTrigger>
                          <HelpCircle className="h-3.5 w-3.5 ml-1 inline text-muted-foreground" />
                        </TooltipTrigger>
                        <TooltipContent className="max-w-xs">
                          <p>
                            El slug se usa en URLs y no puede cambirse después de crear el tenant.
                            Solo minúsculas, números y guiones.
                          </p>
                        </TooltipContent>
                      </Tooltip>
                    </Label>
                    <div className="relative">
                      <Input
                        id="slug"
                        value={formData.slug}
                        onChange={(e) => updateForm({ slug: e.target.value.toLowerCase() })}
                        placeholder="mi-organizacion"
                        className="font-mono"
                        disabled // Usually can't change slug
                      />
                      {formData.slug && (
                        <div className="absolute right-3 top-1/2 -translate-y-1/2">
                          {isValidSlug(formData.slug) ? (
                            <Check className="h-4 w-4 text-success" />
                          ) : (
                            <XCircle className="h-4 w-4 text-danger" />
                          )}
                        </div>
                      )}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      URL: auth.example.com/<span className="font-mono">{formData.slug || "slug"}</span>
                    </p>
                  </div>
                  
                  <div className="space-y-2">
                    <Label htmlFor="language">Idioma predeterminado</Label>
                    <Select
                      value={formData.language}
                      onValueChange={(value) => updateForm({ language: value })}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="Selecciona idioma" />
                      </SelectTrigger>
                      <SelectContent>
                        {LANGUAGES.map((lang) => (
                          <SelectItem key={lang.code} value={lang.code}>
                            {lang.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <p className="text-xs text-muted-foreground">
                      Idioma para emails y páginas de login.
                    </p>
                  </div>
                </div>
                
                <hr className="border-border" />
                
                <div className="flex items-center justify-between p-4 rounded-lg border bg-muted/30">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium">ID del Tenant</span>
                      <Badge variant="secondary" className="font-mono text-xs">
                        {tenantId}
                      </Badge>
                    </div>
                    <p className="text-xs text-muted-foreground">
                      Identificador único interno. Usado en integraciones API.
                    </p>
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      navigator.clipboard.writeText(tenantId)
                      toast({ title: "ID copiado al portapapeles" })
                    }}
                  >
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
          
          {/* ============================================================== */}
          {/* TAB: BRANDING */}
          {/* ============================================================== */}
          <TabsContent value="branding" className="space-y-6">
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
              {/* Main branding settings */}
              <div className="lg:col-span-2 space-y-6">
                <Card>
                  <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                      <ImageIcon className="h-5 w-5" />
                      Logo
                    </CardTitle>
                    <CardDescription>
                      El logo aparece en la página de login, emails y otros lugares.
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <LogoUploader
                      value={formData.logoUrl || null}
                      onChange={(logo) => updateForm({ logoUrl: logo || "" })}
                      brandColor={formData.brandColor}
                    />
                  </CardContent>
                </Card>
                
                <Card>
                  <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                      <Paintbrush className="h-5 w-5" />
                      Colores de Marca
                    </CardTitle>
                    <CardDescription>
                      Define los colores que representan tu organización.
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-6">
                    <ColorPicker
                      value={formData.brandColor}
                      onChange={(color) => updateForm({ brandColor: color })}
                      label="Color Principal"
                      description="Usado en botones, links y elementos destacados."
                    />
                    
                    <hr className="border-border" />
                    
                    {/* Future: Secondary color - not in backend yet */}
                    <div className="opacity-50 pointer-events-none">
                      <div className="flex items-center gap-2 mb-3">
                        <Label className="text-sm font-medium">Color Secundario</Label>
                        <Badge variant="outline" className="text-xs">Próximamente</Badge>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        {DEFAULT_BRAND_COLORS.slice(0, 4).map((color) => (
                          <div
                            key={color.value}
                            className="w-8 h-8 rounded-lg border-2 border-transparent"
                            style={{ backgroundColor: color.value }}
                          />
                        ))}
                      </div>
                      <p className="text-xs text-muted-foreground mt-2">
                        Color para elementos secundarios y acentos.
                      </p>
                    </div>
                    
                    {/* Future: Favicon */}
                    <div className="opacity-50 pointer-events-none">
                      <div className="flex items-center gap-2 mb-3">
                        <Label className="text-sm font-medium">Favicon</Label>
                        <Badge variant="outline" className="text-xs">Próximamente</Badge>
                      </div>
                      <div className="flex items-center gap-3 p-4 rounded-lg border border-dashed">
                        <div className="w-8 h-8 rounded bg-muted flex items-center justify-center">
                          <ImageIcon className="h-4 w-4 text-muted-foreground" />
                        </div>
                        <span className="text-sm text-muted-foreground">
                          Sube un favicon personalizado (32x32 px)
                        </span>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </div>
              
              {/* Preview sidebar */}
              <div className="space-y-6">
                <BrandingPreview
                  logo={formData.logoUrl || null}
                  brandColor={formData.brandColor}
                  tenantName={formData.name}
                />
                
                <Card>
                  <CardHeader className="pb-3">
                    <CardTitle className="text-sm font-medium flex items-center gap-2">
                      <Sparkles className="h-4 w-4" />
                      Próximas funciones
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-2 text-sm text-muted-foreground">
                    <div className="flex items-center gap-2">
                      <div className="w-1.5 h-1.5 rounded-full bg-warning" />
                      Tema claro/oscuro personalizado
                    </div>
                    <div className="flex items-center gap-2">
                      <div className="w-1.5 h-1.5 rounded-full bg-warning" />
                      Fuente personalizada
                    </div>
                    <div className="flex items-center gap-2">
                      <div className="w-1.5 h-1.5 rounded-full bg-warning" />
                      CSS personalizado
                    </div>
                    <div className="flex items-center gap-2">
                      <div className="w-1.5 h-1.5 rounded-full bg-warning" />
                      Preview de emails
                    </div>
                  </CardContent>
                </Card>
              </div>
            </div>
          </TabsContent>
          
          {/* ============================================================== */}
          {/* TAB: SECURITY */}
          {/* ============================================================== */}
          <TabsContent value="security" className="space-y-6">
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Timer className="h-5 w-5" />
                    Duración de Sesión
                  </CardTitle>
                  <CardDescription>
                    Tiempo máximo que un usuario permanece autenticado sin actividad.
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <DurationSelector
                    value={formData.sessionLifetimeSeconds}
                    onChange={(seconds) => updateForm({ sessionLifetimeSeconds: seconds })}
                    presets={SESSION_PRESETS}
                    label="Tiempo de sesión"
                    description="Después de este tiempo, el usuario debe volver a iniciar sesión."
                  />
                </CardContent>
              </Card>
              
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <RefreshCw className="h-5 w-5" />
                    Refresh Token
                  </CardTitle>
                  <CardDescription>
                    Duración del token de refresco para mantener sesiones activas.
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <DurationSelector
                    value={formData.refreshTokenLifetimeSeconds}
                    onChange={(seconds) => updateForm({ refreshTokenLifetimeSeconds: seconds })}
                    presets={REFRESH_TOKEN_PRESETS}
                    label="Tiempo de refresh token"
                    description="Los clientes pueden obtener nuevos access tokens durante este período."
                  />
                </CardContent>
              </Card>
            </div>
            
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Lock className="h-5 w-5" />
                  Autenticación Multi-Factor (MFA)
                </CardTitle>
                <CardDescription>
                  Añade una capa extra de seguridad requiriendo un segundo factor de autenticación.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between p-4 rounded-lg border">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">Habilitar MFA</span>
                      {formData.mfaEnabled && (
                        <Badge className="bg-success/10 text-success border-success/20">
                          Activo
                        </Badge>
                      )}
                    </div>
                    <p className="text-sm text-muted-foreground">
                      Permitir que los usuarios configuren autenticación de dos factores.
                    </p>
                  </div>
                  <Switch
                    checked={formData.mfaEnabled}
                    onCheckedChange={(checked) => updateForm({ mfaEnabled: checked })}
                  />
                </div>
                
                {formData.mfaEnabled && (
                  <div className="ml-4 pl-4 border-l space-y-4 animate-in slide-in-from-top-2">
                    <div className="flex items-center justify-between p-4 rounded-lg border bg-muted/30">
                      <div className="space-y-1">
                        <span className="text-sm font-medium">MFA Obligatorio</span>
                        <p className="text-xs text-muted-foreground">
                          Forzar a todos los usuarios a configurar MFA.
                        </p>
                      </div>
                      <Switch
                        checked={formData.security?.mfaRequired || false}
                        onCheckedChange={(checked) => 
                          updateForm({ 
                            security: { ...formData.security, mfaRequired: checked } 
                          })
                        }
                      />
                    </div>
                    
                    <InlineAlert variant="info">
                        Los métodos de MFA disponibles son: TOTP (Google Authenticator, Authy) 
                        y códigos de respaldo. WebAuthn/passkeys estará disponible próximamente.
                    </InlineAlert>
                  </div>
                )}
              </CardContent>
            </Card>
            
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Globe className="h-5 w-5" />
                  Login Social
                </CardTitle>
                <CardDescription>
                  Permite que los usuarios inicien sesión con proveedores externos.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between p-4 rounded-lg border">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">Habilitar Login Social</span>
                      {formData.socialLoginEnabled && (
                        <Badge className="bg-info/10 text-info border-info/20">
                          Activo
                        </Badge>
                      )}
                    </div>
                    <p className="text-sm text-muted-foreground">
                      Permitir login con Google, GitHub, etc.
                    </p>
                  </div>
                  <Switch
                    checked={formData.socialLoginEnabled}
                    onCheckedChange={(checked) => updateForm({ socialLoginEnabled: checked })}
                  />
                </div>
                
                {formData.socialLoginEnabled && (
                  <InlineAlert variant="info">
                      Configura los proveedores sociales en{" "}
                      <Link 
                        href={`/admin/tenants/providers?id=${tenantId}`}
                        className="text-info hover:underline font-medium"
                      >
                        Social Providers →
                      </Link>
                  </InlineAlert>
                )}
              </CardContent>
            </Card>
            
            {/* Future: Password policies */}
            <Card className="opacity-60">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Key className="h-5 w-5" />
                  Políticas de Contraseña
                  <Badge variant="outline">Próximamente</Badge>
                </CardTitle>
                <CardDescription>
                  Requisitos mínimos para contraseñas de usuarios.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4 pointer-events-none">
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label>Longitud mínima</Label>
                    <Input type="number" value={8} disabled />
                  </div>
                  <div className="space-y-2">
                    <Label>Requiere mayúscula</Label>
                    <Switch disabled />
                  </div>
                  <div className="space-y-2">
                    <Label>Requiere número</Label>
                    <Switch disabled />
                  </div>
                  <div className="space-y-2">
                    <Label>Requiere símbolo</Label>
                    <Switch disabled />
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
          
          {/* ============================================================== */}
          {/* TAB: ISSUER */}
          {/* ============================================================== */}
          <TabsContent value="issuer" className="space-y-6">
            <InlineAlert variant="info">
                <strong>¿Qué es el Issuer?</strong> El issuer es la URL que identifica a tu servidor 
                de autorización en tokens JWT. Aparece en el claim &quot;iss&quot; y debe coincidir con 
                la configuración de tus aplicaciones cliente.
            </InlineAlert>
            
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Link2 className="h-5 w-5" />
                  Modo de Issuer
                </CardTitle>
                <CardDescription>
                  Define cómo se construye la URL del issuer para este tenant.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  {ISSUER_MODES.map((mode) => (
                    <button
                      key={mode.value}
                      type="button"
                      onClick={() => updateForm({ issuerMode: mode.value as any })}
                      className={`
                        p-4 rounded-xl border text-left transition-all duration-200
                        hover:border-primary/50 hover:bg-muted/50
                        ${formData.issuerMode === mode.value
                          ? "border-primary bg-primary/10 ring-1 ring-primary/20"
                          : "border-border"
                        }
                      `}
                    >
                      <div className="flex items-center gap-2 mb-2">
                        {formData.issuerMode === mode.value && (
                          <Check className="h-4 w-4 text-primary" />
                        )}
                        <span className="font-medium">{mode.label}</span>
                      </div>
                      <p className="text-xs text-muted-foreground mb-2">
                        {mode.description}
                      </p>
                      <code className="text-xs bg-muted px-2 py-1 rounded font-mono">
                        {mode.example}
                      </code>
                    </button>
                  ))}
                </div>
                
                {formData.issuerMode === "global" && (
                  <div className="space-y-3 animate-in slide-in-from-top-2">
                    <Label htmlFor="issuerOverride">URL del Issuer Personalizado</Label>
                    <Input
                      id="issuerOverride"
                      value={formData.issuerOverride}
                      onChange={(e) => updateForm({ issuerOverride: e.target.value })}
                      placeholder="https://auth.tudominio.com"
                      className="font-mono"
                    />
                    <p className="text-xs text-muted-foreground">
                      Asegúrate de que este dominio apunte a tu instancia de HelloJohn 
                      y tenga certificado SSL válido.
                    </p>
                  </div>
                )}
                
                <hr className="border-border" />
                
                <div className="p-4 rounded-lg border bg-muted/30">
                  <h4 className="text-sm font-medium mb-2">Issuer URL actual</h4>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 text-sm bg-background px-3 py-2 rounded border font-mono">
                      {formData.issuerMode === "global" && formData.issuerOverride
                        ? formData.issuerOverride
                        : formData.issuerMode === "subdomain"
                        ? `https://${formData.slug}.auth.example.com`
                        : `https://auth.example.com/${formData.slug}`}
                    </code>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => {
                        const url = formData.issuerMode === "global" && formData.issuerOverride
                          ? formData.issuerOverride
                          : formData.issuerMode === "subdomain"
                          ? `https://${formData.slug}.auth.example.com`
                          : `https://auth.example.com/${formData.slug}`
                        navigator.clipboard.writeText(url)
                        toast({ title: "URL copiada" })
                      }}
                    >
                      <Copy className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
                
                <InlineAlert variant="warning">
                    <strong>Precaución:</strong> Cambiar el modo de issuer puede invalidar 
                    tokens existentes y romper integraciones. Asegúrate de actualizar la 
                    configuración en todas tus aplicaciones cliente.
                </InlineAlert>
              </CardContent>
            </Card>
            
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Globe className="h-5 w-5" />
                  OIDC Discovery
                </CardTitle>
                <CardDescription>
                  Endpoints estándar de OpenID Connect para este tenant.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  {[
                    { label: "Well-Known", path: "/.well-known/openid-configuration" },
                    { label: "JWKS", path: "/.well-known/jwks.json" },
                    { label: "Authorize", path: "/oauth2/authorize" },
                    { label: "Token", path: "/oauth2/token" },
                    { label: "UserInfo", path: "/oauth2/userinfo" },
                  ].map((endpoint) => (
                    <div 
                      key={endpoint.label}
                      className="flex items-center justify-between p-3 rounded-lg border bg-muted/20"
                    >
                      <span className="text-sm font-medium">{endpoint.label}</span>
                      <code className="text-xs font-mono text-muted-foreground">
                        {endpoint.path}
                      </code>
                    </div>
                  ))}
                </div>
                
                <div className="mt-4">
                  <Button variant="outline" size="sm" asChild>
                    <Link href={`/admin/oidc?tenant=${formData.slug}`}>
                      <ExternalLink className="mr-2 h-4 w-4" />
                      Ver Discovery Completo
                    </Link>
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
          
          {/* ============================================================== */}
          {/* TAB: EXPORT/IMPORT */}
          {/* ============================================================== */}
          <TabsContent value="export" className="space-y-6">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <Card className="border-success/20">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <FileDown className="h-5 w-5 text-success" />
                    Exportar Configuración
                  </CardTitle>
                  <CardDescription>
                    Descarga un archivo JSON con toda la configuración del tenant.
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <ul className="space-y-2 text-sm">
                    <li className="flex items-center gap-2">
                      <CheckCircle2 className="h-4 w-4 text-success" />
                      Configuración general y branding
                    </li>
                    <li className="flex items-center gap-2">
                      <CheckCircle2 className="h-4 w-4 text-success" />
                      Políticas de seguridad
                    </li>
                    <li className="flex items-center gap-2">
                      <CheckCircle2 className="h-4 w-4 text-success" />
                      Clientes OAuth2 y scopes
                    </li>
                    <li className="flex items-center gap-2">
                      <CheckCircle2 className="h-4 w-4 text-success" />
                      Claims personalizados
                    </li>
                    <li className="flex items-center gap-2">
                      <CheckCircle2 className="h-4 w-4 text-success" />
                      Roles y permisos (opcional)
                    </li>
                    <li className="flex items-center gap-2 text-muted-foreground">
                      <AlertCircle className="h-4 w-4" />
                      Usuarios (opcional, datos sensibles)
                    </li>
                  </ul>
                  
                  <hr className="border-border" />
                  
                  <InlineAlert variant="info">
                      El archivo exportado <strong>no incluye</strong> secrets 
                      (client_secret, contraseñas SMTP, etc.) por seguridad.
                  </InlineAlert>
                  
                  <Button 
                    className="w-full"
                    onClick={() => setShowExportDialog(true)}
                  >
                    <Download className="mr-2 h-4 w-4" />
                    Exportar Configuración
                  </Button>
                </CardContent>
              </Card>
              
              <Card className="border-info/20">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <FileUp className="h-5 w-5 text-info" />
                    Importar Configuración
                    <Badge variant="outline">Próximamente</Badge>
                  </CardTitle>
                  <CardDescription>
                    Restaura la configuración desde un archivo de exportación.
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <ul className="space-y-2 text-sm text-muted-foreground">
                    <li className="flex items-center gap-2">
                      <div className="w-4 h-4 rounded-full border-2 border-dashed border-muted-foreground/50" />
                      Crear nuevo tenant desde backup
                    </li>
                    <li className="flex items-center gap-2">
                      <div className="w-4 h-4 rounded-full border-2 border-dashed border-muted-foreground/50" />
                      Merge configuración en tenant existente
                    </li>
                    <li className="flex items-center gap-2">
                      <div className="w-4 h-4 rounded-full border-2 border-dashed border-muted-foreground/50" />
                      Preview antes de aplicar cambios
                    </li>
                    <li className="flex items-center gap-2">
                      <div className="w-4 h-4 rounded-full border-2 border-dashed border-muted-foreground/50" />
                      Detección de conflictos
                    </li>
                  </ul>
                  
                  <hr className="border-border" />
                  
                  <InlineAlert variant="warning">
                      Esta funcionalidad requiere endpoints adicionales en el backend.
                      Ver requisitos en el diálogo de importación.
                  </InlineAlert>
                  
                  <Button 
                    variant="outline" 
                    className="w-full"
                    onClick={() => setShowImportDialog(true)}
                  >
                    <Upload className="mr-2 h-4 w-4" />
                    Importar Configuración
                  </Button>
                </CardContent>
              </Card>
            </div>
            
            {/* Backup History - Future */}
            <Card className="opacity-60">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Clock className="h-5 w-5" />
                  Historial de Backups
                  <Badge variant="outline">Próximamente</Badge>
                </CardTitle>
                <CardDescription>
                  Backups automáticos de configuración.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="text-center py-8 text-muted-foreground">
                  <FileJson className="h-12 w-12 mx-auto mb-3 opacity-50" />
                  <p className="text-sm">
                    Los backups automáticos estarán disponibles próximamente.
                  </p>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
        
        {/* Dialogs */}
        <ExportDialog
          open={showExportDialog}
          onOpenChange={setShowExportDialog}
          tenantId={tenantId}
          tenantSlug={formData.slug}
          settings={settings}
        />
        
        <ImportDialog
          open={showImportDialog}
          onOpenChange={setShowImportDialog}
          tenantId={tenantId}
        />
      </div>
    </TooltipProvider>
  )
}
