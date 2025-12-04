"use client";

import { useDraggable } from "@dnd-kit/core";
import { Card } from "@/components/ui/card";
import { GripVertical } from "lucide-react";

const AVAILABLE_FIELDS = [
    { type: "email", label: "Email Address", icon: "📧" },
    { type: "password", label: "Password", icon: "🔑" },
    { type: "text", label: "Text Input", icon: "📝" },
    { type: "number", label: "Number Input", icon: "🔢" },
    { type: "phone", label: "Phone Number", icon: "📞" },
];

export function FieldPalette() {
    return (
        <div className="flex flex-col gap-2">
            {AVAILABLE_FIELDS.map((field) => (
                <DraggablePaletteItem key={field.type} field={field} />
            ))}
        </div>
    );
}

function DraggablePaletteItem({ field }: { field: any }) {
    const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
        id: `palette-${field.type}`,
        data: {
            type: "palette-item",
            field: {
                type: field.type,
                label: field.label,
                name: field.type, // default name
                required: false,
            },
        },
    });

    const style = isDragging ? { opacity: 0.5 } : undefined;

    return (
        <Card
            ref={setNodeRef}
            style={style}
            {...listeners}
            {...attributes}
            className="p-3 cursor-grab hover:bg-gray-50 flex items-center gap-2 text-sm"
        >
            <GripVertical className="h-4 w-4 text-gray-400" />
            <span>{field.icon}</span>
            <span>{field.label}</span>
        </Card>
    );
}
