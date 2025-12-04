import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";

interface ThemeEditorProps {
    theme: any;
    onChange: (theme: any) => void;
}

export function ThemeEditor({ theme, onChange }: ThemeEditorProps) {
    const handleChange = (key: string, value: any) => {
        onChange({ ...theme, [key]: value });
    };

    const handleNestedChange = (parent: string, key: string, value: any) => {
        onChange({
            ...theme,
            [parent]: { ...theme[parent], [key]: value }
        });
    };

    return (
        <Accordion type="single" collapsible className="w-full" defaultValue="colors">
            <AccordionItem value="colors">
                <AccordionTrigger>Colors</AccordionTrigger>
                <AccordionContent className="space-y-4">
                    <div className="space-y-2">
                        <Label>Primary Color</Label>
                        <div className="flex gap-2">
                            <Input
                                type="color"
                                value={theme.primaryColor}
                                onChange={(e) => handleChange("primaryColor", e.target.value)}
                                className="w-12 h-8 p-1"
                            />
                            <Input
                                value={theme.primaryColor}
                                onChange={(e) => handleChange("primaryColor", e.target.value)}
                                className="flex-1 h-8"
                            />
                        </div>
                    </div>
                    <div className="space-y-2">
                        <Label>Background Color</Label>
                        <div className="flex gap-2">
                            <Input
                                type="color"
                                value={theme.backgroundColor}
                                onChange={(e) => handleChange("backgroundColor", e.target.value)}
                                className="w-12 h-8 p-1"
                            />
                            <Input
                                value={theme.backgroundColor}
                                onChange={(e) => handleChange("backgroundColor", e.target.value)}
                                className="flex-1 h-8"
                            />
                        </div>
                    </div>
                    <div className="space-y-2">
                        <Label>Text Color</Label>
                        <div className="flex gap-2">
                            <Input
                                type="color"
                                value={theme.textColor || "#334155"}
                                onChange={(e) => handleChange("textColor", e.target.value)}
                                className="w-12 h-8 p-1"
                            />
                            <Input
                                value={theme.textColor || "#334155"}
                                onChange={(e) => handleChange("textColor", e.target.value)}
                                className="flex-1 h-8"
                            />
                        </div>
                    </div>
                </AccordionContent>
            </AccordionItem>

            <AccordionItem value="typography">
                <AccordionTrigger>Typography</AccordionTrigger>
                <AccordionContent className="space-y-4">
                    <div className="space-y-2">
                        <Label>Font Family</Label>
                        <Select
                            value={theme.fontFamily || "inter"}
                            onValueChange={(val) => handleChange("fontFamily", val)}
                        >
                            <SelectTrigger>
                                <SelectValue placeholder="Select font" />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="inter">Inter (Default)</SelectItem>
                                <SelectItem value="roboto">Roboto</SelectItem>
                                <SelectItem value="open-sans">Open Sans</SelectItem>
                                <SelectItem value="lato">Lato</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                </AccordionContent>
            </AccordionItem>

            <AccordionItem value="inputs">
                <AccordionTrigger>Inputs</AccordionTrigger>
                <AccordionContent className="space-y-4">
                    <div className="space-y-2">
                        <Label>Style Variant</Label>
                        <Select
                            value={theme.inputStyle?.variant || "outlined"}
                            onValueChange={(val) => handleNestedChange("inputStyle", "variant", val)}
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="outlined">Outlined</SelectItem>
                                <SelectItem value="filled">Filled</SelectItem>
                                <SelectItem value="underlined">Underlined</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="space-y-2">
                        <Label>Border Radius</Label>
                        <Select
                            value={theme.borderRadius}
                            onValueChange={(val) => handleChange("borderRadius", val)}
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="0">Square (0px)</SelectItem>
                                <SelectItem value="0.25rem">Small (4px)</SelectItem>
                                <SelectItem value="0.5rem">Medium (8px)</SelectItem>
                                <SelectItem value="1rem">Large (16px)</SelectItem>
                                <SelectItem value="9999px">Pill</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                </AccordionContent>
            </AccordionItem>

            <AccordionItem value="buttons">
                <AccordionTrigger>Buttons</AccordionTrigger>
                <AccordionContent className="space-y-4">
                    <div className="space-y-2">
                        <Label>Style Variant</Label>
                        <Select
                            value={theme.buttonStyle?.variant || "solid"}
                            onValueChange={(val) => handleNestedChange("buttonStyle", "variant", val)}
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="solid">Solid</SelectItem>
                                <SelectItem value="outline">Outline</SelectItem>
                                <SelectItem value="ghost">Ghost</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="flex items-center justify-between">
                        <Label>Full Width</Label>
                        <Switch
                            checked={theme.buttonStyle?.fullWidth ?? true}
                            onCheckedChange={(val) => handleNestedChange("buttonStyle", "fullWidth", val)}
                        />
                    </div>
                </AccordionContent>
            </AccordionItem>

            <AccordionItem value="layout">
                <AccordionTrigger>Layout</AccordionTrigger>
                <AccordionContent className="space-y-4">
                    <div className="space-y-2">
                        <Label>Spacing</Label>
                        <Select
                            value={theme.spacing || "normal"}
                            onValueChange={(val) => handleChange("spacing", val)}
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="compact">Compact</SelectItem>
                                <SelectItem value="normal">Normal</SelectItem>
                                <SelectItem value="relaxed">Relaxed</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="flex items-center justify-between">
                        <Label>Show Labels</Label>
                        <Switch
                            checked={theme.showLabels ?? true}
                            onCheckedChange={(val) => handleChange("showLabels", val)}
                        />
                    </div>
                    <div className="space-y-2">
                        <Label>Logo URL</Label>
                        <Input
                            value={theme.logoUrl || ""}
                            onChange={(e) => handleChange("logoUrl", e.target.value)}
                            placeholder="https://..."
                        />
                    </div>
                </AccordionContent>
            </AccordionItem>
        </Accordion>
    );
}
