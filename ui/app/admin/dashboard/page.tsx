"use client"

import { Card } from "@/components/ui/card"
import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { useI18n } from "@/lib/i18n"

export default function AdminDashboardPage() {
	const { t } = useI18n()
	const { data } = useQuery({
		queryKey: ["admin-stats"],
		queryFn: async () => {
			try {
				return await api.get<any>("/v2/admin/stats")
			} catch (e: any) {
				return null
			}
		},
	})
	return (
		<main className="p-6 space-y-6">
			<div>
				<h1 className="text-3xl font-bold">{t("dashboard.title")}</h1>
				<p className="text-muted-foreground mt-2">{t("dashboard.subtitle")}</p>
			</div>
			<div className="grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-4">
				<Card className="p-4">
					<p className="text-xs text-muted-foreground">{t("dashboard.usersTotal")}</p>
					<p className="text-2xl font-bold">{data?.users_total ?? "—"}</p>
				</Card>
				<Card className="p-4">
					<p className="text-xs text-muted-foreground">{t("dashboard.clients")}</p>
					<p className="text-2xl font-bold">{data?.clients_total ?? "—"}</p>
				</Card>
				<Card className="p-4">
					<p className="text-xs text-muted-foreground">{t("dashboard.loginsToday")}</p>
					<p className="text-2xl font-bold">{data?.logins_today ?? "—"}</p>
				</Card>
				<Card className="p-4">
					<p className="text-xs text-muted-foreground">{t("dashboard.activeSessions")}</p>
					<p className="text-2xl font-bold">{data?.sessions_active ?? "—"}</p>
				</Card>
			</div>
		</main>
	)
}
