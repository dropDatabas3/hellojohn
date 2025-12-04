"use client";

import { useState, useEffect } from "react";
import { DndContext, DragOverlay, useSensor, useSensors, PointerSensor, DragStartEvent, DragEndEvent } from "@dnd-kit/core";
import { arrayMove } from "@dnd-kit/sortable";
import { FieldPalette } from "./FieldPalette";
import { FormCanvas } from "./FormCanvas";
import { FieldProperties } from "./FieldProperties";
import { ThemeEditor } from "./ThemeEditor";
import { StepManager } from "./StepManager";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

interface FormConfig {
    theme: any;
    steps: any[];
    socialLayout?: any;
}

interface FormBuilderProps {
    initialConfig?: FormConfig;
    onSave: (config: FormConfig) => void;
}

export function FormBuilder({ initialConfig, onSave }: FormBuilderProps) {
    // Initialize steps. If legacy config (no steps), wrap fields in a default step.
    const [steps, setSteps] = useState<any[]>(() => {
        if (initialConfig?.steps && initialConfig.steps.length > 0) {
            return initialConfig.steps;
        }
        // Migration for legacy config
        return [{
            id: "step-1",
            title: "Step 1",
            fields: (initialConfig as any)?.fields || []
        }];
    });

    const [activeStepId, setActiveStepId] = useState<string>(steps[0]?.id || "step-1");

    const [theme, setTheme] = useState<any>(initialConfig?.theme || {
        primaryColor: "#0f172a",
        backgroundColor: "#ffffff",
        textColor: "#334155",
        borderRadius: "0.5rem",
        inputStyle: { variant: "outlined" },
        buttonStyle: { variant: "solid", fullWidth: true },
        spacing: "normal",
        showLabels: true
    });

    const [selectedFieldId, setSelectedFieldId] = useState<string | null>(null);
    const [activeDragItem, setActiveDragItem] = useState<any>(null);

    const sensors = useSensors(useSensor(PointerSensor));

    const activeStep = steps.find(s => s.id === activeStepId) || steps[0];
    const activeFields = activeStep?.fields || [];

    // --- Step Management ---

    const handleAddStep = () => {
        const newStep = {
            id: `step-${Date.now()}`,
            title: `Step ${steps.length + 1}`,
            fields: []
        };
        setSteps([...steps, newStep]);
        setActiveStepId(newStep.id);
    };

    const handleDeleteStep = (id: string) => {
        if (steps.length <= 1) return;
        const newSteps = steps.filter(s => s.id !== id);
        setSteps(newSteps);
        if (activeStepId === id) {
            setActiveStepId(newSteps[0].id);
        }
    };

    const handleUpdateStep = (id: string, updates: any) => {
        setSteps(steps.map(s => s.id === id ? { ...s, ...updates } : s));
    };

    // --- Field Management (within active step) ---

    const updateActiveStepFields = (newFields: any[]) => {
        setSteps(steps.map(s => s.id === activeStepId ? { ...s, fields: newFields } : s));
    };

    const handleDragStart = (event: DragStartEvent) => {
        const { active } = event;
        if (active.data.current?.type === "palette-item") {
            setActiveDragItem({ ...active.data.current.field, id: `field-${Date.now()}` });
        } else {
            const field = activeFields.find((f: any) => f.id === active.id);
            setActiveDragItem(field);
        }
    };

    const handleDragEnd = (event: DragEndEvent) => {
        const { active, over } = event;
        setActiveDragItem(null);

        if (!over) return;

        // Dropping new item from palette
        if (active.data.current?.type === "palette-item") {
            const newField = {
                ...active.data.current.field,
                id: `field-${Date.now()}`,
            };
            updateActiveStepFields([...activeFields, newField]);
            setSelectedFieldId(newField.id);
            return;
        }

        // Reordering existing items
        if (active.id !== over.id) {
            const oldIndex = activeFields.findIndex((i: any) => i.id === active.id);
            const newIndex = activeFields.findIndex((i: any) => i.id === over.id);
            updateActiveStepFields(arrayMove(activeFields, oldIndex, newIndex));
        }
    };

    const handleFieldUpdate = (id: string, updates: any) => {
        const newFields = activeFields.map((f: any) => f.id === id ? { ...f, ...updates } : f);
        updateActiveStepFields(newFields);
    };

    const handleDeleteField = (id: string) => {
        const newFields = activeFields.filter((f: any) => f.id !== id);
        updateActiveStepFields(newFields);
        if (selectedFieldId === id) setSelectedFieldId(null);
    };

    const selectedField = activeFields.find((f: any) => f.id === selectedFieldId);

    return (
        <div className="flex h-[800px] gap-4">
            <DndContext sensors={sensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
                {/* Left Sidebar: Steps & Palette */}
                <div className="w-72 flex flex-col gap-4">
                    <Card className="p-4">
                        <StepManager
                            steps={steps}
                            activeStepId={activeStepId}
                            onStepSelect={setActiveStepId}
                            onAddStep={handleAddStep}
                            onDeleteStep={handleDeleteStep}
                            onUpdateStep={handleUpdateStep}
                        />
                    </Card>
                    <Card className="p-4 flex-1 overflow-y-auto">
                        <h3 className="font-semibold mb-4 text-sm">Fields</h3>
                        <FieldPalette />
                    </Card>
                </div>

                {/* Center: Canvas & Theme */}
                <div className="flex-1 flex flex-col gap-4">
                    {/* Theme Editor (Top Bar or Tabs) */}
                    <Card className="p-2">
                        <ThemeEditor theme={theme} onChange={setTheme} />
                    </Card>

                    <div className="flex-1 bg-gray-50/50 border rounded-lg p-8 overflow-y-auto flex justify-center relative">
                        {/* Canvas Area */}
                        <div className="w-full max-w-md">
                            <FormCanvas
                                fields={activeFields}
                                theme={theme}
                                selectedFieldId={selectedFieldId}
                                onSelectField={setSelectedFieldId}
                                activeStep={activeStep}
                                totalSteps={steps.length}
                                currentStepIndex={steps.findIndex(s => s.id === activeStepId)}
                            />
                        </div>
                    </div>
                </div>

                {/* Right Sidebar: Properties */}
                <div className="w-72">
                    <Card className="p-4 h-full overflow-y-auto">
                        <h3 className="font-semibold mb-4 text-sm">Properties</h3>
                        {selectedField ? (
                            <FieldProperties
                                field={selectedField}
                                onChange={(updates) => handleFieldUpdate(selectedField.id, updates)}
                                onDelete={() => handleDeleteField(selectedField.id)}
                            />
                        ) : (
                            <div className="text-center py-8 text-muted-foreground text-sm">
                                <p>Select a field to edit properties.</p>
                            </div>
                        )}
                    </Card>
                </div>

                <DragOverlay>
                    {activeDragItem ? (
                        <div className="bg-white p-3 border rounded shadow opacity-80 w-48 font-medium text-sm">
                            {activeDragItem.label}
                        </div>
                    ) : null}
                </DragOverlay>
            </DndContext>

            <div className="absolute bottom-8 right-8 z-50">
                <Button size="lg" onClick={() => onSave({ theme, steps })}>
                    Save Changes
                </Button>
            </div>
        </div>
    );
}
