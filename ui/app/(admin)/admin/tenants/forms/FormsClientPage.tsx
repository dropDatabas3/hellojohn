"use client";

import { useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { FormBuilder } from "@/components/form-builder/FormBuilder";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useToast } from "@/hooks/use-toast";

export default function FormsClientPage() {
    const params = useParams();
    const searchParams = useSearchParams();
    const tenantId = (params.id as string) || (searchParams.get("id") as string);
    const { toast } = useToast();
    const queryClient = useQueryClient();

    // Fetch tenant settings to get current form config
    const { data: tenant, isLoading } = useQuery({
        queryKey: ["tenant", tenantId],
        queryFn: async () => {
            if (!tenantId) return null;
            const res = await fetch(`http://localhost:8081/v1/admin/tenants/${tenantId}`, {
                headers: { Authorization: "Bearer dev-token" }, // TODO: Real auth
            });
            if (!res.ok) throw new Error("Failed to fetch tenant");
            return res.json();
        },
        enabled: !!tenantId,
    });

    const updateMutation = useMutation({
        mutationFn: async (newSettings: any) => {
            const res = await fetch(`http://localhost:8081/v1/admin/tenants/${tenantId}/settings`, {
                method: "PUT",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: "Bearer dev-token",
                },
                body: JSON.stringify(newSettings),
            });
            if (!res.ok) throw new Error("Failed to update settings");
            return res.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["tenant", tenantId] });
            toast({ title: "Saved", description: "Form configuration saved successfully." });
        },
        onError: () => {
            toast({ title: "Error", description: "Failed to save configuration.", variant: "destructive" });
        },
    });

    if (isLoading) return <div>Loading...</div>;
    if (!tenantId) return <div>No tenant selected</div>;

    const forms = tenant?.settings?.forms || { login: null, register: null };

    const handleSave = (type: "login" | "register", config: any) => {
        const newForms = { ...forms, [type]: config };
        const newSettings = { ...tenant.settings, forms: newForms };
        updateMutation.mutate(newSettings);
    };

    return (
        <div className="space-y-6">
            <div className="flex flex-col justify-between">
                <h2 className="text-3xl font-bold tracking-tight">Form Builder</h2>
                <p className="text-muted-foreground">En desarrollo</p>
            </div>

            <Tabs defaultValue="login" className="w-full">
                <TabsList>
                    <TabsTrigger value="login">Login Form</TabsTrigger>
                    <TabsTrigger value="register">Register Form</TabsTrigger>
                </TabsList>

                <TabsContent value="login">
                    <Card>
                        <CardHeader>
                            <CardTitle>Customize Login Form</CardTitle>
                        </CardHeader>
                        <CardContent>
                            <FormBuilder
                                initialConfig={forms.login}
                                onSave={(config) => handleSave("login", config)}
                            />
                        </CardContent>
                    </Card>
                </TabsContent>

                <TabsContent value="register">
                    <Card>
                        <CardHeader>
                            <CardTitle>Customize Register Form</CardTitle>
                        </CardHeader>
                        <CardContent>
                            <FormBuilder
                                initialConfig={forms.register}
                                onSave={(config) => handleSave("register", config)}
                            />
                        </CardContent>
                    </Card>
                </TabsContent>
            </Tabs>
        </div>
    );
}
