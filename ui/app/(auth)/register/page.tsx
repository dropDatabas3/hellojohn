// Register page

"use client"

import type React from "react"
import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { useMutation } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useToast } from "@/hooks/use-toast"
import { ApiClient } from "@/lib/api"
import type { LoginResponse, AuthConfigResponse } from "@/lib/types"

export default function RegisterPage() {
    const router = useRouter()
    const { toast } = useToast()

    const [email, setEmail] = useState("")
    const [password, setPassword] = useState("")
    const [confirmPassword, setConfirmPassword] = useState("")
    const [name, setName] = useState("")

    // Branding state
    const [authConfig, setAuthConfig] = useState<AuthConfigResponse | null>(null)

    // Detect return_to from URL
    const searchParams = typeof window !== "undefined" ? new URLSearchParams(window.location.search) : null
    const returnTo = searchParams?.get("return_to")
    const clientId = searchParams?.get("client_id") || (returnTo ? (decodeURIComponent(returnTo).match(/[?&]client_id=([^&]+)/)?.[1] || "") : "")


    useEffect(() => {
        // Branding Logic
        const fetchConfig = async () => {
            if (clientId) {
                try {
                    const apiBase = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080"
                    const api = new ApiClient(apiBase, () => null, () => { }, () => { })
                    const cfg = await api.get<AuthConfigResponse>(`/v1/auth/config?client_id=${clientId}`)
                    setAuthConfig(cfg)
                } catch (e) {
                    console.error("Failed to load branding", e)
                }
            }
        }
        fetchConfig()
    }, [clientId])

    const registerMutation = useMutation({
        mutationFn: async (data: any) => {
            const apiBase = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080"
            const api = new ApiClient(apiBase, () => null, () => { }, () => { })

            // Registration is always public/tenant scoped if client_id is present
            // If we have clientId, we should use it to inform backend (though /register might not check it yet, good to send)
            // Standard endpoint: /v1/auth/register
            const payload = {
                email: data.email,
                password: data.password,
                name: data.name,
                client_id: clientId || undefined
            }

            await api.post("/v1/auth/register", payload)
            return true
        },
        onSuccess: () => {
            toast({
                title: "Account created",
                description: "Please log in with your new account.",
            })
            // Redirect to login preserving params
            const params = new URLSearchParams(window.location.search)
            router.push(`/login?${params.toString()}`)
        },
        onError: (error: any) => {
            toast({
                title: "Error",
                description: error.error_description || "Registration failed",
                variant: "destructive",
            })
        },
    })

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()
        if (password !== confirmPassword) {
            toast({ title: "Error", description: "Passwords do not match", variant: "destructive" })
            return
        }
        registerMutation.mutate({ email, password, name })
    }

    // UI Calculations
    const title = authConfig?.tenant_name ? `Register at ${authConfig.tenant_name}` : "Create Account"
    const logo = authConfig?.logo_url

    return (
        <div className="flex min-h-screen items-center justify-center bg-muted/40 p-4">
            <Card className="w-full max-w-md">
                <CardHeader className="space-y-1">
                    {logo && (
                        <div className="flex justify-center mb-4">
                            <img src={logo} alt="Logo" className="h-12 object-contain" />
                        </div>
                    )}
                    <CardTitle className="text-2xl font-bold text-center">{title}</CardTitle>
                    <CardDescription className="text-center">Enter your details to create an account</CardDescription>
                </CardHeader>
                <CardContent>
                    <form onSubmit={handleSubmit} className="space-y-4">
                        <div className="space-y-2">
                            <Label htmlFor="name">Name</Label>
                            <Input
                                id="name"
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                required
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="email">Email</Label>
                            <Input
                                id="email"
                                type="email"
                                placeholder="name@example.com"
                                value={email}
                                onChange={(e) => setEmail(e.target.value)}
                                required
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="password">Password</Label>
                            <Input
                                id="password"
                                type="password"
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                required
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="confirmPassword">Confirm Password</Label>
                            <Input
                                id="confirmPassword"
                                type="password"
                                value={confirmPassword}
                                onChange={(e) => setConfirmPassword(e.target.value)}
                                required
                            />
                        </div>
                        <Button
                            type="submit"
                            className="w-full"
                            disabled={registerMutation.isPending}
                            style={authConfig?.primary_color ? { backgroundColor: authConfig.primary_color } : {}}
                        >
                            {registerMutation.isPending ? "Creating account..." : "Register"}
                        </Button>

                        <div className="text-center text-sm">
                            Already have an account?{" "}
                            <a href={`/login?${searchParams?.toString()}`} className="underline">Log in</a>
                        </div>

                        {registerMutation.isError && (
                            <div className="text-sm text-destructive">
                                {registerMutation.error?.message || "Error creating account"}
                            </div>
                        )}
                    </form>
                </CardContent>
            </Card>
        </div>
    )
}
