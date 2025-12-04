import { useDroppable } from "@dnd-kit/core";
import { SortableContext, verticalListSortingStrategy, useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { cn } from "@/lib/utils";
import { GripVertical, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";

interface FormCanvasProps {
    fields: any[];
    theme: any;
    selectedFieldId: string | null;
    onSelectField: (id: string) => void;
    activeStep: any;
    totalSteps: number;
    currentStepIndex: number;
}

export function FormCanvas({
    fields,
    theme,
    selectedFieldId,
    onSelectField,
    activeStep,
    totalSteps,
    currentStepIndex
}: FormCanvasProps) {
    const { setNodeRef } = useDroppable({
        id: "canvas-droppable",
    });

    // Theme Styles
    const containerStyle = {
        backgroundColor: theme.backgroundColor,
        borderRadius: theme.borderRadius,
        color: theme.textColor,
        fontFamily: theme.fontFamily === "inter" ? "inherit" : theme.fontFamily,
    };

    const inputClasses = cn(
        "w-full px-3 py-2 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-primary/20",
        theme.inputStyle?.variant === "outlined" && "border rounded-md bg-transparent",
        theme.inputStyle?.variant === "filled" && "border-0 bg-muted/50 rounded-md",
        theme.inputStyle?.variant === "underlined" && "border-b rounded-none bg-transparent px-0",
        theme.borderRadius === "9999px" && theme.inputStyle?.variant !== "underlined" && "rounded-full"
    );

    const buttonClasses = cn(
        "inline-flex items-center justify-center text-sm font-medium transition-colors focus-visible:outline-none disabled:opacity-50",
        theme.buttonStyle?.variant === "solid" && "bg-primary text-primary-foreground hover:bg-primary/90",
        theme.buttonStyle?.variant === "outline" && "border border-input hover:bg-accent hover:text-accent-foreground",
        theme.buttonStyle?.variant === "ghost" && "hover:bg-accent hover:text-accent-foreground",
        theme.buttonStyle?.fullWidth ? "w-full" : "w-auto",
        theme.spacing === "compact" ? "h-8 px-3" : theme.spacing === "relaxed" ? "h-12 px-6" : "h-10 px-4",
        theme.borderRadius === "9999px" ? "rounded-full" : "rounded-md"
    );

    const spacingClass = theme.spacing === "compact" ? "space-y-3" : theme.spacing === "relaxed" ? "space-y-6" : "space-y-4";

    return (
        <div
            ref={setNodeRef}
            className="w-full min-h-[500px] shadow-xl border transition-all duration-200"
            style={containerStyle}
        >
            {/* Header / Logo */}
            <div className="p-8 pb-0 text-center">
                {theme.logoUrl && (
                    <img src={theme.logoUrl} alt="Logo" className="h-12 mx-auto mb-4 object-contain" />
                )}
                {totalSteps > 1 && (
                    <div className="mb-6">
                        <div className="flex justify-center gap-2 mb-2">
                            {Array.from({ length: totalSteps }).map((_, i) => (
                                <div
                                    key={i}
                                    className={cn(
                                        "h-1.5 flex-1 rounded-full transition-colors",
                                        i <= currentStepIndex ? "bg-primary" : "bg-muted"
                                    )}
                                />
                            ))}
                        </div>
                        <h2 className="text-lg font-semibold" style={{ color: theme.headingColor }}>
                            {activeStep.title}
                        </h2>
                        {activeStep.description && (
                            <p className="text-sm text-muted-foreground mt-1">{activeStep.description}</p>
                        )}
                    </div>
                )}
            </div>

            {/* Form Fields */}
            <div className={cn("p-8", spacingClass)}>
                <SortableContext items={fields.map((f) => f.id)} strategy={verticalListSortingStrategy}>
                    {fields.length === 0 ? (
                        <div className="border-2 border-dashed border-muted-foreground/25 rounded-lg p-8 text-center text-muted-foreground">
                            Drag fields here
                        </div>
                    ) : (
                        fields.map((field) => (
                            <SortableField
                                key={field.id}
                                field={field}
                                isSelected={selectedFieldId === field.id}
                                onClick={() => onSelectField(field.id)}
                                theme={theme}
                                inputClasses={inputClasses}
                            />
                        ))
                    )}
                </SortableContext>

                {/* Submit / Navigation Buttons */}
                <div className="pt-4 flex gap-3">
                    {currentStepIndex > 0 && (
                        <button
                            className={cn(buttonClasses, "bg-muted text-muted-foreground hover:bg-muted/80")}
                            style={{ width: theme.buttonStyle?.fullWidth ? "50%" : "auto" }}
                        >
                            Back
                        </button>
                    )}
                    <button
                        className={buttonClasses}
                        style={{
                            backgroundColor: theme.buttonStyle?.variant === "solid" ? theme.primaryColor : undefined,
                            borderColor: theme.buttonStyle?.variant === "outline" ? theme.primaryColor : undefined,
                            color: theme.buttonStyle?.variant === "outline" ? theme.primaryColor : undefined,
                            width: theme.buttonStyle?.fullWidth ? (currentStepIndex > 0 ? "50%" : "100%") : "auto"
                        }}
                    >
                        {currentStepIndex === totalSteps - 1 ? "Submit" : "Next"}
                    </button>
                </div>
            </div>
        </div>
    );
}

function SortableField({ field, isSelected, onClick, theme, inputClasses }: any) {
    const { attributes, listeners, setNodeRef, transform, transition } = useSortable({ id: field.id });

    const style = {
        transform: CSS.Transform.toString(transform),
        transition,
    };

    return (
        <div
            ref={setNodeRef}
            style={style}
            {...attributes}
            {...listeners}
            onClick={(e) => {
                e.stopPropagation(); // Prevent drag from triggering select immediately if handled poorly
                onClick();
            }}
            className={cn(
                "group relative rounded-lg border-2 border-transparent p-2 transition-colors hover:bg-muted/30 cursor-pointer",
                isSelected && "border-primary bg-muted/30"
            )}
        >
            {theme.showLabels && (
                <label className="mb-1.5 block text-sm font-medium">
                    {field.label}
                    {field.required && <span className="text-red-500 ml-1">*</span>}
                </label>
            )}

            <div className="pointer-events-none">
                {field.type === "textarea" ? (
                    <textarea
                        className={inputClasses}
                        placeholder={field.placeholder}
                        rows={3}
                        disabled
                    />
                ) : (
                    <input
                        type={field.type}
                        className={inputClasses}
                        placeholder={field.placeholder}
                        disabled
                    />
                )}
            </div>

            {field.helpText && (
                <p className="mt-1 text-xs text-muted-foreground">{field.helpText}</p>
            )}
        </div>
    );
}
