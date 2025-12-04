import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { Trash2 } from "lucide-react";
import { Separator } from "@/components/ui/separator";

interface FieldPropertiesProps {
    field: any;
    onChange: (updates: any) => void;
    onDelete: () => void;
}

export function FieldProperties({ field, onChange, onDelete }: FieldPropertiesProps) {
    return (
        <div className="space-y-6">
            <div className="space-y-4">
                <div className="space-y-2">
                    <Label>Label</Label>
                    <Input
                        value={field.label}
                        onChange={(e) => onChange({ label: e.target.value })}
                    />
                </div>

                <div className="space-y-2">
                    <Label>Field Name (API Key)</Label>
                    <Input
                        value={field.name}
                        onChange={(e) => onChange({ name: e.target.value })}
                        className="font-mono text-xs"
                    />
                </div>

                <div className="space-y-2">
                    <Label>Placeholder</Label>
                    <Input
                        value={field.placeholder || ""}
                        onChange={(e) => onChange({ placeholder: e.target.value })}
                    />
                </div>

                <div className="space-y-2">
                    <Label>Help Text</Label>
                    <Input
                        value={field.helpText || ""}
                        onChange={(e) => onChange({ helpText: e.target.value })}
                        placeholder="Instructions for the user"
                    />
                </div>

                <div className="flex items-center justify-between">
                    <Label>Required</Label>
                    <Switch
                        checked={field.required}
                        onCheckedChange={(checked) => onChange({ required: checked })}
                    />
                </div>
            </div>

            <Separator />

            <div className="space-y-4">
                <h4 className="text-sm font-medium">Validation</h4>

                <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-2">
                        <Label className="text-xs">Min Length</Label>
                        <Input
                            type="number"
                            value={field.minLength || ""}
                            onChange={(e) => onChange({ minLength: parseInt(e.target.value) || undefined })}
                        />
                    </div>
                    <div className="space-y-2">
                        <Label className="text-xs">Max Length</Label>
                        <Input
                            type="number"
                            value={field.maxLength || ""}
                            onChange={(e) => onChange({ maxLength: parseInt(e.target.value) || undefined })}
                        />
                    </div>
                </div>

                <div className="space-y-2">
                    <Label className="text-xs">Regex Pattern</Label>
                    <Input
                        value={field.pattern || ""}
                        onChange={(e) => onChange({ pattern: e.target.value })}
                        placeholder="e.g. ^[A-Z]+$"
                        className="font-mono text-xs"
                    />
                </div>
            </div>

            <Separator />

            <Button
                variant="destructive"
                className="w-full"
                onClick={onDelete}
            >
                <Trash2 className="mr-2 h-4 w-4" />
                Delete Field
            </Button>
        </div>
    );
}
