"use client"

import { useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useParams, useSearchParams } from "next/navigation"
import { Plus, Search, Trash2, Eye, Copy, Check, ArrowLeft, Ban } from "lucide-react"
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
import Link from "next/link"
import type { Client, ClientInput, Tenant } from "@/lib/types"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

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
	const [newClient, setNewClient] = useState<ClientInput>({
		name: "",
		type: "confidential",
		redirectUris: [],
		allowedOrigins: [],
		scopes: [],
	})
	const [redirectUriInput, setRedirectUriInput] = useState("")
	const [originInput, setOriginInput] = useState("")

	const { data: tenant } = useQuery({
			queryKey: ["tenant", tenantId],
			enabled: !!tenantId,
			queryFn: () => api.get<Tenant>(`/v1/admin/tenants/${tenantId}`),
	})

		type ClientRow = {
			id: string
			tenant_id: string
			name: string
			client_id: string
			client_type: "public" | "confidential"
			redirect_uris: string[]
			allowed_origins?: string[]
			providers?: string[]
			scopes?: string[]
			created_at?: string
		}

		const { data: clientsRaw, isLoading } = useQuery({
			queryKey: ["clients", tenantId],
			enabled: !!tenantId,
			// backend expects tenant_id
			queryFn: () => api.get<ClientRow[]>(`/v1/admin/clients?tenant_id=${tenantId}`),
		})

		// map snake_case to UI Client shape for rendering only
		const clients: Client[] | undefined = clientsRaw?.map((c) => ({
			id: c.id,
			tenantId: c.tenant_id,
			name: c.name,
			clientId: c.client_id,
			type: c.client_type,
			redirectUris: c.redirect_uris || [],
			allowedOrigins: c.allowed_origins || [],
			providers: c.providers || [],
			scopes: c.scopes || [],
			createdAt: c.created_at || "",
			updatedAt: c.created_at || "",
		}))

		const createMutation = useMutation({
			// backend requires tenant_id and snake_case fields
			mutationFn: (data: ClientInput) =>
				api.post<ClientRow>(`/v1/admin/clients`, {
					tenant_id: tenantId,
					client_id: data.clientId,
					name: data.name,
					client_type: data.type,
					redirect_uris: data.redirectUris,
					allowed_origins: data.allowedOrigins || [],
					providers: data.providers || [],
					scopes: data.scopes || [],
				}),
		onSuccess: (data) => {
			queryClient.invalidateQueries({ queryKey: ["clients", tenantId] })
				// map minimal view
				setSelectedClient({
					id: (data as any).id,
					tenantId: tenantId!,
					name: (data as any).name,
					clientId: (data as any).client_id,
					type: (data as any).client_type,
					redirectUris: (data as any).redirect_uris || [],
					allowedOrigins: (data as any).allowed_origins || [],
					providers: (data as any).providers || [],
					scopes: (data as any).scopes || [],
					createdAt: (data as any).created_at || "",
					updatedAt: (data as any).created_at || "",
				})
			setCreateDialogOpen(false)
			setViewDialogOpen(true)
			setNewClient({
				name: "",
				type: "confidential",
				redirectUris: [],
				allowedOrigins: [],
				scopes: [],
			})
			setRedirectUriInput("")
			setOriginInput("")
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
			// backend deletes by UUID id in path; no tenant param
			mutationFn: (clientUUID: string) => api.delete(`/v1/admin/clients/${clientUUID}`),
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
		mutationFn: (clientUUID: string) => api.post(`/v1/admin/clients/${clientUUID}/revoke`, {}),
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

	const handleCreate = () => {
		if (!newClient.name) {
			toast({
				title: t("common.error"),
				description: t("clients.fillRequired"),
				variant: "destructive",
			})
			return
		}
		createMutation.mutate(newClient)
	}

	const handleDelete = () => {
		if (selectedClient) {
			deleteMutation.mutate(selectedClient.id)
		}
	}

	const addRedirectUri = () => {
		if (redirectUriInput && !newClient.redirectUris.includes(redirectUriInput)) {
			setNewClient({
				...newClient,
				redirectUris: [...newClient.redirectUris, redirectUriInput],
			})
			setRedirectUriInput("")
		}
	}

	const removeRedirectUri = (uri: string) => {
		setNewClient({
			...newClient,
			redirectUris: newClient.redirectUris.filter((u) => u !== uri),
		})
	}

	const addOrigin = () => {
		if (originInput && !newClient.allowedOrigins?.includes(originInput)) {
			setNewClient({
				...newClient,
				allowedOrigins: [...(newClient.allowedOrigins || []), originInput],
			})
			setOriginInput("")
		}
	}

	const removeOrigin = (origin: string) => {
		setNewClient({
			...newClient,
			allowedOrigins: newClient.allowedOrigins?.filter((o) => o !== origin),
		})
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
						<Button onClick={() => setCreateDialogOpen(true)}>
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
								<TableHead>{t("clients.allowedOrigins")}</TableHead>
								<TableHead>{t("clients.scopes")}</TableHead>
								<TableHead className="text-right">{t("common.actions")}</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{filteredClients?.length === 0 ? (
								<TableRow>
									<TableCell colSpan={5} className="text-center text-muted-foreground">
										{t("clients.noClients")}
									</TableCell>
								</TableRow>
							) : (
								filteredClients?.map((client) => (
														<TableRow key={client.id}>
										<TableCell className="font-medium">{client.name}</TableCell>
										<TableCell>
											<code className="rounded bg-muted px-2 py-1 text-sm">{client.clientId}</code>
										</TableCell>
										<TableCell>
											<Badge variant={client.type === "confidential" ? "default" : "secondary"}>{client.type}</Badge>
										</TableCell>
										<TableCell>
																<span className="text-sm text-muted-foreground">{client.redirectUris.length} URI(s)</span>
															</TableCell>
															<TableCell>
																<span className="text-sm text-muted-foreground">{client.allowedOrigins?.length || 0} ORIG</span>
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
													title={t("clients.revokeAllTokens")}
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

			{/* Create Dialog */}
			<Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
				<DialogContent className="max-w-2xl">
								<DialogHeader>
						<DialogTitle>{t("clients.createTitle")}</DialogTitle>
						<DialogDescription>{t("clients.createDescription")}</DialogDescription>
					</DialogHeader>
					<div className="space-y-4">
						<div className="space-y-2">
							<Label htmlFor="name">{t("clients.name")} *</Label>
							<Input
								id="name"
								value={newClient.name}
								onChange={(e) => setNewClient({ ...newClient, name: e.target.value })}
								placeholder="My Application"
							/>
						</div>
									<div className="space-y-2">
										<Label htmlFor="clientId">{t("clients.clientId")} *</Label>
										<Input
											id="clientId"
											value={newClient.clientId || ""}
											onChange={(e) => setNewClient({ ...newClient, clientId: e.target.value })}
											placeholder="myapp-web"
										/>
									</div>
						<div className="space-y-2">
							<Label htmlFor="type">{t("clients.type")} *</Label>
							<Select
								value={newClient.type}
								onValueChange={(value: "public" | "confidential") => setNewClient({ ...newClient, type: value })}
							>
								<SelectTrigger>
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="confidential">Confidential</SelectItem>
									<SelectItem value="public">Public</SelectItem>
								</SelectContent>
							</Select>
							<p className="text-sm text-muted-foreground">
								Public: SPA/Native apps sin secreto. Confidential: backends/SSR con client_secret.
							</p>
						</div>
						<div className="space-y-2">
							<Label>{t("clients.redirectUris")} *</Label>
							<div className="flex gap-2">
								<Input
									value={redirectUriInput}
									onChange={(e) => setRedirectUriInput(e.target.value)}
									placeholder="https://example.com/callback"
									onKeyDown={(e) => {
										if (e.key === "Enter") {
											e.preventDefault()
											addRedirectUri()
										}
									}}
								/>
								<Button type="button" onClick={addRedirectUri}>
									{t("common.add")}
								</Button>
							</div>
							<p className="text-sm text-muted-foreground">
								Requerido para authorization_code (apps web). No se usa para M2M client_credentials.
							</p>
							{newClient.redirectUris.length > 0 && (
								<div className="mt-2 space-y-1">
									{newClient.redirectUris.map((uri) => (
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
						<div className="space-y-2">
							<Label>{t("clients.allowedOrigins")}</Label>
							<div className="flex gap-2">
								<Input
									value={originInput}
									onChange={(e) => setOriginInput(e.target.value)}
									placeholder="https://example.com"
									onKeyDown={(e) => {
										if (e.key === "Enter") {
											e.preventDefault()
											addOrigin()
										}
									}}
								/>
								<Button type="button" onClick={addOrigin}>
									{t("common.add")}
								</Button>
							</div>
							<p className="text-sm text-muted-foreground">
								CORS para apps JS. Ej: http://localhost:3000
							</p>
							{newClient.allowedOrigins && newClient.allowedOrigins.length > 0 && (
								<div className="mt-2 space-y-1">
									{newClient.allowedOrigins.map((origin) => (
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
									<div className="space-y-2">
										<Label>{t("clients.scopes")}</Label>
										<Input
											placeholder="openid profile email admin"
											value={(newClient.scopes || []).join(" ")}
											onChange={(e) => setNewClient({ ...newClient, scopes: e.target.value.split(/\s+/).filter(Boolean) })}
										/>
										<p className="text-sm text-muted-foreground">Separados por espacio. Deben existir en Scopes.</p>
									</div>
									<div className="space-y-2">
										<Label>{t("clients.providers")}</Label>
										<Input
											placeholder="password google"
											value={(newClient.providers || []).join(" ")}
											onChange={(e) => setNewClient({ ...newClient, providers: e.target.value.split(/\s+/).filter(Boolean) })}
										/>
										<p className="text-sm text-muted-foreground">Habilita proveedores: password, google, etc.</p>
									</div>
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={() => setCreateDialogOpen(false)}>
							{t("common.cancel")}
						</Button>
						<Button onClick={handleCreate} disabled={createMutation.isPending}>
							{createMutation.isPending ? t("common.creating") : t("common.create")}
						</Button>
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
										<code className="flex-1 rounded bg-muted p-2 text-sm">{selectedClient.secret}</code>
										<Button variant="outline" size="sm" onClick={() => copyToClipboard(selectedClient.secret || "")}>
											{copiedSecret ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
										</Button>
									</div>
									<p className="text-sm text-destructive">{t("clients.secretWarning")}</p>
								</div>
							)}
							<div className="space-y-2">
								<Label>{t("clients.redirectUris")}</Label>
								<div className="space-y-1">
									{selectedClient.redirectUris.map((uri) => (
										<code key={uri} className="block rounded bg-muted p-2 text-sm">
											{uri}
										</code>
									))}
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

