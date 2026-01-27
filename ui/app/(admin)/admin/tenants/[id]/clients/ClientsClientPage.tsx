"use client"

import { useState, useMemo, useEffect } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useParams, useSearchParams } from "next/navigation"
import { Plus, Search, Trash2, Eye, Copy, Check, ArrowLeft, Ban, Globe, Server, HelpCircle, ChevronRight, ChevronLeft, Sparkles } from "lucide-react"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { useToast } from "@/hooks/use-toast"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
import Link from "next/link"
import type { Client, ClientInput, Tenant } from "@/lib/types"

// ============================================================================
// TYPES
// ============================================================================

type ClientType = "public" | "confidential"

interface ClientFormState {
	name: string
	clientId: string
	type: ClientType
	description: string
	redirectUris: string[]
	allowedOrigins: string[]
	scopes: string[]
	providers: string[]
	// Email verification & password reset
	requireEmailVerification: boolean
	resetPasswordUrl: string
	verifyEmailUrl: string
}

// ============================================================================
// HELPERS
// ============================================================================

function slugify(text: string): string {
	return text
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, "_")
		.replace(/^_|_$/g, "")
		.slice(0, 20)
}

function generateClientId(tenantSlug: string, name: string, type: ClientType): string {
	const nameSlug = slugify(name)
	const typeShort = type === "public" ? "web" : "srv"
	const rand = Math.random().toString(36).substring(2, 6)
	return `${tenantSlug}_${nameSlug}_${typeShort}_${rand}`
}

// ============================================================================
// TOOLTIP COMPONENT
// ============================================================================

function Tooltip({ text }: { text: string }) {
	return (
		<span className="inline-flex items-center ml-1 cursor-help" title={text}>
			<HelpCircle className="h-3.5 w-3.5 text-muted-foreground" />
		</span>
	)
}

// ============================================================================
// CLIENT TYPE CARD
// ============================================================================



function ClientTypeCard({
	type,
	title,
	description,
	features,
	icon: Icon,
	selected,
	onClick,
}: {
	type: ClientType
	title: string
	description: string
	features: string[]
	icon: React.ElementType
	selected: boolean
	onClick: () => void
}) {
	return (
		<button
			type="button"
			onClick={onClick}
			className={`flex flex-col items-start p-6 rounded-xl border transition-all duration-300 text-left w-full ${selected
				? "border-transparent text-white shadow-lg"
				: "border-border hover:border-primary/40 hover:bg-muted/30 hover:shadow-md"
				}`}
			style={selected ? {
				background: type == "public"
					? "linear-gradient(135deg, #22c55e 0%, #16a34a 100%)"
					: "linear-gradient(135deg, #8b5cf6 0%, #7c3aed 100%)",
				boxShadow: type == "public"
					? "0 10px 25px -5px rgba(34, 197, 94, 0.4), 0 4px 6px -2px rgba(34, 197, 94, 0.2)"
					: "0 10px 25px -5px rgba(139, 92, 246, 0.4), 0 4px 6px -2px rgba(139, 92, 246, 0.2)",
			} : undefined}
		>
			<div className={`p-3 rounded-xl mb-4 transition-colors ${selected ? "bg-white/15 backdrop-blur-sm" : "bg-muted"}`}>
				<Icon className="h-6 w-6" />
			</div>
			<h3 className="text-lg font-semibold mb-1">{title}</h3>
			<p className={`text-sm mb-4 ${selected ? "text-white/85" : "text-muted-foreground"}`}>{description}</p>
			<ul className="text-sm space-y-1.5">
				{features.map((f, i) => (
					<li key={i} className="flex items-center gap-2">
						<span className={`text-xs ${selected ? "text-white/90" : "text-primary"}`}>✓</span> {f}
					</li>
				))}
			</ul>
		</button>
	)
}

// ============================================================================
// MAIN COMPONENT
// ============================================================================

export default function ClientsClientPage() {
	const params = useParams()
	const searchParams = useSearchParams()
	const { t } = useI18n()
	const { toast } = useToast()
	const queryClient = useQueryClient()
	const tenantId = (params.id as string) || (searchParams.get("id") as string)
	const [search, setSearch] = useState("")
	const [createDialogOpen, setCreateDialogOpen] = useState(false)
	const [viewDialogOpen, setViewDialogOpen] = useState(false)
	const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
	const [selectedClient, setSelectedClient] = useState<Client | null>(null)
	const [copiedSecret, setCopiedSecret] = useState(false)

	// Wizard state
	const [step, setStep] = useState(1)
	const [form, setForm] = useState<ClientFormState>({
		name: "",
		clientId: "",
		type: "public",
		description: "",
		redirectUris: [],
		allowedOrigins: [],
		scopes: ["openid", "profile", "email"],
		providers: ["password"],
		requireEmailVerification: false,
		resetPasswordUrl: "",
		verifyEmailUrl: "",
	})
	const [redirectUriInput, setRedirectUriInput] = useState("")
	const [originInput, setOriginInput] = useState("")

	const { data: tenant } = useQuery({
		queryKey: ["tenant", tenantId],
		enabled: !!tenantId,
		queryFn: () => api.get<Tenant>(`/v2/admin/tenants/${tenantId}`),
	})

	// Auto-generate clientId when name or type changes
	useEffect(() => {
		if (form.name && tenant?.slug) {
			setForm(prev => ({
				...prev,
				clientId: generateClientId(tenant.slug, form.name, form.type)
			}))
		}
	}, [form.name, form.type, tenant?.slug])

	// Backend v2 returns snake_case
	type ClientRow = {
		id: string
		client_id: string
		name: string
		type: "public" | "confidential"
		redirect_uris: string[]
		allowed_origins?: string[]
		providers?: string[]
		scopes?: string[]
		secret?: string // Only on create response
		secret_hash?: string
	}

	const { data: clientsRaw, isLoading } = useQuery({
		queryKey: ["clients", tenantId],
		enabled: !!tenantId,
		queryFn: () => api.get<ClientRow[]>(`/v2/admin/clients`, {
			headers: { "X-Tenant-ID": tenantId }
		}),
	})

	const clients: Client[] | undefined = clientsRaw?.map((c) => ({
		id: c.id || c.client_id,
		tenantId: tenantId || "",
		name: c.name,
		clientId: c.client_id,
		type: c.type,
		redirectUris: c.redirect_uris || [],
		allowedOrigins: c.allowed_origins || [],
		providers: c.providers || [],
		scopes: c.scopes || [],
		createdAt: "",
		updatedAt: "",
	}))

	const createMutation = useMutation({
		mutationFn: (data: ClientInput) =>
			api.post<ClientRow>(`/v2/admin/clients`, {
				client_id: data.clientId,
				name: data.name,
				type: data.type,
				redirect_uris: data.redirectUris,
				allowed_origins: data.allowedOrigins || [],
				providers: data.providers || [],
				scopes: data.scopes || [],
				require_email_verification: data.requireEmailVerification || false,
				reset_password_url: data.resetPasswordUrl || "",
				verify_email_url: data.verifyEmailUrl || "",
			}, {
				headers: { "X-Tenant-ID": tenantId }
			}),
		onSuccess: (data) => {
			queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
			const d = data as ClientRow
			setSelectedClient({
				id: d.id || d.client_id,
				tenantId: tenantId!,
				name: d.name,
				clientId: d.client_id,
				type: d.type,
				redirectUris: d.redirect_uris || [],
				allowedOrigins: d.allowed_origins || [],
				providers: d.providers || [],
				scopes: d.scopes || [],
				secret: d.secret,
				createdAt: "",
				updatedAt: "",
			})
			resetForm()
			setCreateDialogOpen(false)
			setViewDialogOpen(true)
			toast({
				title: t("clients.created"),
				description: t("clients.createdDesc"),
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

	const deleteMutation = useMutation({
		mutationFn: (clientUUID: string) => api.delete(`/v2/admin/clients/${clientUUID}`, {
			headers: { "X-Tenant-ID": tenantId }
		}),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
			setDeleteDialogOpen(false)
			setSelectedClient(null)
			toast({
				title: t("clients.deleted"),
				description: t("clients.deletedDesc"),
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

	const revokeMutation = useMutation({
		mutationFn: (clientUUID: string) => api.post(`/v2/admin/clients/${clientUUID}/revoke`, {}, {
			headers: { "X-Tenant-ID": tenantId }
		}),
		onSuccess: () => {
			toast({
				title: t("clients.revoked"),
				description: t("clients.revokedDesc"),
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

	const filteredClients = clients?.filter(
		(client) =>
			client.name.toLowerCase().includes(search.toLowerCase()) ||
			client.clientId?.toLowerCase().includes(search.toLowerCase()),
	)

	const resetForm = () => {
		setForm({
			name: "",
			clientId: "",
			type: "public",
			description: "",
			redirectUris: [],
			allowedOrigins: [],
			scopes: ["openid", "profile", "email"],
			providers: ["password"],
			requireEmailVerification: false,
			resetPasswordUrl: "",
			verifyEmailUrl: "",
		})
		setRedirectUriInput("")
		setOriginInput("")
		setStep(1)
	}

	const handleCreate = () => {
		if (!form.name) {
			toast({
				title: t("common.error"),
				description: "El nombre del cliente es obligatorio",
				variant: "destructive",
			})
			return
		}
		if (form.type === "public" && form.redirectUris.length === 0) {
			toast({
				title: t("common.error"),
				description: "Las aplicaciones frontend requieren al menos una URI de redirección",
				variant: "destructive",
			})
			return
		}
		createMutation.mutate({
			clientId: form.clientId,
			name: form.name,
			type: form.type,
			redirectUris: form.redirectUris,
			allowedOrigins: form.allowedOrigins,
			scopes: form.scopes,
			providers: form.providers,
			requireEmailVerification: form.requireEmailVerification,
			resetPasswordUrl: form.resetPasswordUrl,
			verifyEmailUrl: form.verifyEmailUrl,
		})
	}

	const handleDelete = () => {
		if (selectedClient) {
			deleteMutation.mutate(selectedClient.id)
		}
	}

	const addRedirectUri = () => {
		if (redirectUriInput && !form.redirectUris.includes(redirectUriInput)) {
			setForm({ ...form, redirectUris: [...form.redirectUris, redirectUriInput] })
			setRedirectUriInput("")
		}
	}

	const removeRedirectUri = (uri: string) => {
		setForm({ ...form, redirectUris: form.redirectUris.filter((u) => u !== uri) })
	}

	const addOrigin = () => {
		if (originInput && !form.allowedOrigins.includes(originInput)) {
			setForm({ ...form, allowedOrigins: [...form.allowedOrigins, originInput] })
			setOriginInput("")
		}
	}

	const removeOrigin = (origin: string) => {
		setForm({ ...form, allowedOrigins: form.allowedOrigins.filter((o) => o !== origin) })
	}

	const copyToClipboard = (text: string) => {
		navigator.clipboard.writeText(text)
		setCopiedSecret(true)
		setTimeout(() => setCopiedSecret(false), 2000)
		toast({
			title: t("common.copied"),
			description: t("clients.secretCopied"),
		})
	}

	// ========================================================================
	// RENDER
	// ========================================================================

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
						<h1 className="text-3xl font-bold">{t("clients.title")}</h1>
						<p className="text-muted-foreground">
							{tenant?.name} - {t("clients.description")}
						</p>
					</div>
				</div>
				<Button onClick={() => { resetForm(); setCreateDialogOpen(true) }}>
					<Plus className="mr-2 h-4 w-4" />
					{t("clients.create")}
				</Button>
			</div>

			<Card className="p-6">
				<div className="mb-4">
					<div className="relative">
						<Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
						<Input
							placeholder={t("clients.search")}
							value={search}
							onChange={(e) => setSearch(e.target.value)}
							className="pl-9"
						/>
					</div>
				</div>

				{isLoading ? (
					<div className="flex items-center justify-center py-8">
						<div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
					</div>
				) : (
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead>{t("clients.name")}</TableHead>
								<TableHead>{t("clients.clientId")}</TableHead>
								<TableHead>{t("clients.type")}</TableHead>
								<TableHead>{t("clients.redirectUris")}</TableHead>
								<TableHead>{t("clients.scopes")}</TableHead>
								<TableHead className="text-right">{t("common.actions")}</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{filteredClients?.length === 0 ? (
								<TableRow>
									<TableCell colSpan={6} className="text-center py-12">
										<div className="flex flex-col items-center gap-2">
											<Globe className="h-10 w-10 text-muted-foreground/50" />
											<p className="text-muted-foreground">{t("clients.noClients")}</p>
											<Button variant="outline" size="sm" onClick={() => { resetForm(); setCreateDialogOpen(true) }}>
												<Plus className="mr-2 h-4 w-4" />
												Crear primer cliente
											</Button>
										</div>
									</TableCell>
								</TableRow>
							) : (
								filteredClients?.map((client) => (
									<TableRow key={client.id}>
										<TableCell className="font-medium">{client.name}</TableCell>
										<TableCell>
											<code className="rounded bg-muted px-2 py-1 text-xs max-w-[120px] truncate inline-block align-middle" title={client.clientId}>{client.clientId}</code>
											<Button variant="ghost" size="sm" className="h-5 w-5 p-0 ml-1" onClick={() => copyToClipboard(client.clientId || "")}><Copy className="h-3 w-3" /></Button>
										</TableCell>
										<TableCell>
											<Badge variant={client.type === "confidential" ? "default" : "secondary"}>
												{client.type === "confidential" ? "Backend/M2M" : "Frontend"}
											</Badge>
										</TableCell>
										<TableCell>
											<span className="text-sm text-muted-foreground">{client.redirectUris.length} URI(s)</span>
										</TableCell>
										<TableCell>
											<span className="text-sm text-muted-foreground">{client.scopes?.length || 0} scopes</span>
										</TableCell>
										<TableCell className="text-right">
											<div className="flex items-center justify-end gap-2">
												<Button
													variant="ghost"
													size="sm"
													onClick={() => {
														setSelectedClient(client)
														setViewDialogOpen(true)
													}}
												>
													<Eye className="h-4 w-4" />
												</Button>
												<Button
													variant="ghost"
													size="sm"
													onClick={() => {
														setSelectedClient(client)
														setDeleteDialogOpen(true)
													}}
												>
													<Trash2 className="h-4 w-4 text-destructive" />
												</Button>
												<Button
													variant="ghost"
													size="sm"
													title="Revocar todos los tokens"
													onClick={() => revokeMutation.mutate(client.id)}
												>
													<Ban className="h-4 w-4" />
												</Button>
											</div>
										</TableCell>
									</TableRow>
								))
							)}
						</TableBody>
					</Table>
				)}
			</Card>

			{/* ============================================================
			    CREATE DIALOG - WIZARD
			    ============================================================ */}
			<Dialog open={createDialogOpen} onOpenChange={(open) => { if (!open) resetForm(); setCreateDialogOpen(open) }}>
				<DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
					<DialogHeader>
						<DialogTitle className="flex items-center gap-2">
							<Sparkles className="h-5 w-5 text-primary" />
							Crear nuevo cliente OAuth
						</DialogTitle>
						<DialogDescription>
							{step === 1 && "Paso 1 de 3: Selecciona el tipo de aplicación que vas a integrar"}
							{step === 2 && "Paso 2 de 3: Información básica del cliente"}
							{step === 3 && "Paso 3 de 3: Configuración técnica"}
						</DialogDescription>
					</DialogHeader>

					{/* Step Indicator */}
					<div className="flex items-center justify-center gap-2 py-4">
						{[1, 2, 3].map((s) => (
							<div
								key={s}
								className={`h-2 w-16 rounded-full transition-colors ${s <= step ? "bg-primary" : "bg-muted"
									}`}
							/>
						))}
					</div>

					{/* Step 1: Client Type Selection */}
					{step === 1 && (
						<div className="grid md:grid-cols-2 gap-4">
							<ClientTypeCard
								type="public"
								title="Aplicación Frontend"
								description="Para aplicaciones que corren en el navegador del usuario"
								features={[
									"Single Page Apps (React, Vue, Angular)",
									"Apps móviles nativas",
									"Widgets embebidos",
									"Sin secreto (usa PKCE)",
								]}
								icon={Globe}
								selected={form.type === "public"}
								onClick={() => setForm({ ...form, type: "public" })}
							/>
							<ClientTypeCard
								type="confidential"
								title="Servidor / Backend / M2M"
								description="Para servicios que corren en un servidor seguro"
								features={[
									"APIs y microservicios",
									"Integraciones Machine-to-Machine",
									"Apps SSR (Next.js, Nuxt)",
									"Con client_secret seguro",
								]}
								icon={Server}
								selected={form.type === "confidential"}
								onClick={() => setForm({ ...form, type: "confidential" })}
							/>
						</div>
					)}

					{/* Step 2: Basic Info */}
					{step === 2 && (
						<div className="space-y-6">
							<div className="space-y-2">
								<Label htmlFor="name" className="flex items-center">
									Nombre del cliente *
									<Tooltip text="Un nombre descriptivo para identificar esta aplicación. Ej: 'Portal Web', 'App Móvil iOS'" />
								</Label>
								<Input
									id="name"
									value={form.name}
									onChange={(e) => setForm({ ...form, name: e.target.value })}
									placeholder="Mi Aplicación Web"
									className="text-lg"
								/>
							</div>

							<div className="space-y-2">
								<Label htmlFor="clientId" className="flex items-center">
									Client ID
									<Tooltip text="Identificador único generado automáticamente. Es público y se usa en las solicitudes OAuth." />
								</Label>
								<div className="flex items-center gap-2">
									<code className="flex-1 rounded-md bg-muted px-4 py-3 text-sm font-mono">
										{form.clientId || "Se generará automáticamente..."}
									</code>
									{form.clientId && (
										<Button
											variant="outline"
											size="sm"
											onClick={() => setForm({ ...form, clientId: generateClientId(tenant?.slug || "", form.name, form.type) })}
											title="Regenerar ID"
										>
											<Sparkles className="h-4 w-4" />
										</Button>
									)}
								</div>
								<p className="text-xs text-muted-foreground">
									Formato: {tenant?.slug || "tenant"}_{slugify(form.name) || "nombre"}_{form.type === "public" ? "web" : "srv"}_xxxx
								</p>
							</div>

							<div className="p-4 rounded-lg bg-muted/50 border">
								<h4 className="font-medium mb-2 flex items-center gap-2">
									<Badge variant={form.type === "public" ? "secondary" : "default"}>
										{form.type === "public" ? "Frontend" : "Backend/M2M"}
									</Badge>
									seleccionado
								</h4>
								<p className="text-sm text-muted-foreground">
									{form.type === "public"
										? "Este cliente NO tendrá un secreto. Usará PKCE para autenticación segura."
										: "Este cliente recibirá un client_secret que deberás guardar de forma segura."}
								</p>
							</div>
						</div>
					)}

					{/* Step 3: Technical Config */}
					{step === 3 && (
						<div className="space-y-6">
							{/* Redirect URIs - Required for public, optional for confidential */}
							<div className="space-y-2">
								<Label className="flex items-center">
									URIs de redirección {form.type === "public" && "*"}
									<Tooltip text="URLs a las que HelloJohn puede redirigir después del login. Debe coincidir exactamente." />
								</Label>
								<div className="flex gap-2">
									<Input
										value={redirectUriInput}
										onChange={(e) => setRedirectUriInput(e.target.value)}
										placeholder="https://miapp.com/callback"
										onKeyDown={(e) => {
											if (e.key === "Enter") {
												e.preventDefault()
												addRedirectUri()
											}
										}}
									/>
									<Button type="button" variant="outline" onClick={addRedirectUri}>
										Agregar
									</Button>
								</div>
								{form.type === "public" && form.redirectUris.length === 0 && (
									<p className="text-sm text-amber-600">⚠️ Las apps frontend requieren al menos una URI de redirección</p>
								)}
								{form.redirectUris.length > 0 && (
									<div className="mt-2 space-y-1">
										{form.redirectUris.map((uri) => (
											<div key={uri} className="flex items-center justify-between rounded bg-muted p-2">
												<code className="text-sm">{uri}</code>
												<Button variant="ghost" size="sm" onClick={() => removeRedirectUri(uri)}>
													<Trash2 className="h-4 w-4" />
												</Button>
											</div>
										))}
									</div>
								)}
							</div>

							{/* Allowed Origins - Only for public clients */}
							{form.type === "public" && (
								<div className="space-y-2">
									<Label className="flex items-center">
										Orígenes permitidos (CORS)
										<Tooltip text="Dominios desde los que se permiten requests JavaScript. Necesario para SPAs." />
									</Label>
									<div className="flex gap-2">
										<Input
											value={originInput}
											onChange={(e) => setOriginInput(e.target.value)}
											placeholder="http://localhost:3000"
											onKeyDown={(e) => {
												if (e.key === "Enter") {
													e.preventDefault()
													addOrigin()
												}
											}}
										/>
										<Button type="button" variant="outline" onClick={addOrigin}>
											Agregar
										</Button>
									</div>
									{form.allowedOrigins.length > 0 && (
										<div className="mt-2 space-y-1">
											{form.allowedOrigins.map((origin) => (
												<div key={origin} className="flex items-center justify-between rounded bg-muted p-2">
													<code className="text-sm">{origin}</code>
													<Button variant="ghost" size="sm" onClick={() => removeOrigin(origin)}>
														<Trash2 className="h-4 w-4" />
													</Button>
												</div>
											))}
										</div>
									)}
								</div>
							)}

							{/* Scopes */}
							<div className="space-y-2">
								<Label className="flex items-center">
									Scopes permitidos
									<Tooltip text="Permisos que este cliente puede solicitar. Deben existir en la configuración del tenant." />
								</Label>
								<Input
									placeholder="openid profile email"
									value={form.scopes.join(" ")}
									onChange={(e) => setForm({ ...form, scopes: e.target.value.split(/\s+/).filter(Boolean) })}
								/>
								<p className="text-xs text-muted-foreground">Separados por espacio. Valores comunes: openid, profile, email</p>
							</div>

							{/* Providers */}
							{form.type === "public" && (
								<div className="space-y-2">
									<Label className="flex items-center">
										Proveedores de autenticación
										<Tooltip text="Métodos de login habilitados." />
									</Label>
									<div className="rounded-lg border divide-y">
										{/* Password */}
										<div className="flex items-center justify-between px-3 py-2">
											<div className="flex items-center gap-2">
												<span>🔑</span>
												<span className="text-sm font-medium">Email + Contraseña</span>
											</div>
											<Switch
												checked={form.providers.includes("password")}
												onCheckedChange={(checked) => {
													if (checked) {
														setForm({ ...form, providers: [...form.providers, "password"] })
													} else {
														setForm({ ...form, providers: form.providers.filter(p => p !== "password") })
													}
												}}
											/>
										</div>
										{/* Google */}
										<div className="flex items-center justify-between px-3 py-2">
											<div className="flex items-center gap-2">
												<svg className="w-4 h-4" viewBox="0 0 24 24">
													<path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" />
													<path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
													<path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
													<path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
												</svg>
												<span className="text-sm font-medium">Google</span>
											</div>
											<Switch
												checked={form.providers.includes("google")}
												onCheckedChange={(checked) => {
													if (checked) {
														setForm({ ...form, providers: [...form.providers, "google"] })
													} else {
														setForm({ ...form, providers: form.providers.filter(p => p !== "google") })
													}
												}}
											/>
										</div>
										{/* GitHub - Coming soon */}
										<div className="flex items-center justify-between px-3 py-2 opacity-50">
											<div className="flex items-center gap-2">
												<span>🐙</span>
												<span className="text-sm font-medium">GitHub</span>
												<Badge variant="outline" className="text-[10px] px-1 py-0">Pronto</Badge>
											</div>
											<Switch disabled />
										</div>
									</div>
									{form.providers.length === 0 && (
										<p className="text-xs text-amber-600">⚠️ Selecciona al menos un proveedor</p>
									)}
								</div>
							)}

							{/* Email Verification & Password Reset */}
							{form.type === "public" && (
								<div className="space-y-4 pt-4 border-t">
									<h4 className="font-medium flex items-center gap-2">
										📧 Configuración de Email
										<Tooltip text="Opciones para verificación de email y reset de contraseña." />
									</h4>

									{/* Require Email Verification */}
									<div className="flex items-center justify-between rounded-lg border p-3">
										<div className="space-y-0.5">
											<div className="text-sm font-medium">Requerir verificación de email</div>
											<div className="text-xs text-muted-foreground">
												Los usuarios deben verificar su email antes de poder iniciar sesión
											</div>
										</div>
										<Switch
											checked={form.requireEmailVerification}
											onCheckedChange={(checked) => setForm({ ...form, requireEmailVerification: checked })}
										/>
									</div>

									{/* Reset Password URL */}
									<div className="space-y-2">
										<Label className="flex items-center">
											URL de reset de contraseña
											<Tooltip text="URL a la que redirigir cuando el usuario hace clic en 'Olvidé mi contraseña'. Ej: https://app.com/reset-password" />
										</Label>
										<Input
											placeholder="https://tu-app.com/reset-password"
											value={form.resetPasswordUrl}
											onChange={(e) => setForm({ ...form, resetPasswordUrl: e.target.value })}
										/>
									</div>

									{/* Verify Email URL */}
									<div className="space-y-2">
										<Label className="flex items-center">
											URL de verificación de email
											<Tooltip text="URL a la que redirigir cuando el usuario hace clic en el link de verificación. Ej: https://app.com/verify-email" />
										</Label>
										<Input
											placeholder="https://tu-app.com/verify-email"
											value={form.verifyEmailUrl}
											onChange={(e) => setForm({ ...form, verifyEmailUrl: e.target.value })}
										/>
									</div>
								</div>
							)}
						</div>
					)}

					<DialogFooter className="flex justify-between">
						<div>
							{step > 1 && (
								<Button variant="ghost" onClick={() => setStep(step - 1)}>
									<ChevronLeft className="mr-2 h-4 w-4" />
									Anterior
								</Button>
							)}
						</div>
						<div className="flex gap-2">
							<Button variant="outline" onClick={() => { resetForm(); setCreateDialogOpen(false) }}>
								Cancelar
							</Button>
							{step < 3 ? (
								<Button onClick={() => setStep(step + 1)} disabled={step === 2 && !form.name}>
									Siguiente
									<ChevronRight className="ml-2 h-4 w-4" />
								</Button>
							) : (
								<Button onClick={handleCreate} disabled={createMutation.isPending}>
									{createMutation.isPending ? "Creando..." : "Crear Cliente"}
								</Button>
							)}
						</div>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{/* View Dialog */}
			<Dialog open={viewDialogOpen} onOpenChange={setViewDialogOpen}>
				<DialogContent className="max-w-2xl">
					<DialogHeader>
						<DialogTitle>{t("clients.clientDetails")}</DialogTitle>
						<DialogDescription>{selectedClient?.name}</DialogDescription>
					</DialogHeader>
					{selectedClient && (
						<div className="space-y-4">
							<div className="space-y-2">
								<Label>{t("clients.clientId")}</Label>
								<div className="flex items-center gap-2">
									<code className="flex-1 rounded bg-muted p-2 text-sm">{selectedClient.clientId}</code>
									<Button variant="outline" size="sm" onClick={() => copyToClipboard(selectedClient.clientId || "")}>
										{copiedSecret ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
									</Button>
								</div>
							</div>
							{selectedClient.secret && (
								<div className="space-y-2">
									<Label>{t("clients.clientSecret")}</Label>
									<div className="flex items-center gap-2">
										<code className="flex-1 rounded bg-muted p-2 text-sm font-mono">{selectedClient.secret}</code>
										<Button variant="outline" size="sm" onClick={() => copyToClipboard(selectedClient.secret || "")}>
											{copiedSecret ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
										</Button>
									</div>
									<p className="text-sm text-destructive">{t("clients.secretWarning")}</p>
								</div>
							)}
							<div className="space-y-2">
								<Label>{t("clients.type")}</Label>
								<Badge variant={selectedClient.type === "confidential" ? "default" : "secondary"}>
									{selectedClient.type === "confidential" ? "Backend/M2M" : "Frontend"}
								</Badge>
							</div>
							<div className="space-y-2">
								<Label>{t("clients.redirectUris")}</Label>
								<div className="space-y-1">
									{selectedClient.redirectUris.length > 0 ? (
										selectedClient.redirectUris.map((uri) => (
											<code key={uri} className="block rounded bg-muted p-2 text-sm">
												{uri}
											</code>
										))
									) : (
										<p className="text-sm text-muted-foreground">Sin URIs de redirección configuradas</p>
									)}
								</div>
							</div>
						</div>
					)}
					<DialogFooter>
						<Button onClick={() => setViewDialogOpen(false)}>{t("common.close")}</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{/* Delete Dialog */}
			<Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>{t("clients.deleteTitle")}</DialogTitle>
						<DialogDescription>{t("clients.deleteDescription", { name: selectedClient?.name })}</DialogDescription>
					</DialogHeader>
					<DialogFooter>
						<Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>
							{t("common.cancel")}
						</Button>
						<Button variant="destructive" onClick={handleDelete} disabled={deleteMutation.isPending}>
							{deleteMutation.isPending ? t("common.deleting") : t("common.delete")}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	)
}
