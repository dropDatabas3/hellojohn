import { Button } from "@/components/ui/button";
import { Plus, Trash2, GripVertical } from "lucide-react";
import { cn } from "@/lib/utils";

interface Step {
    id: string;
    title: string;
    description?: string;
}

interface StepManagerProps {
    steps: Step[];
    activeStepId: string;
    onStepSelect: (id: string) => void;
    onAddStep: () => void;
    onDeleteStep: (id: string) => void;
    onUpdateStep: (id: string, updates: Partial<Step>) => void;
}

export function StepManager({
    steps,
    activeStepId,
    onStepSelect,
    onAddStep,
    onDeleteStep,
    onUpdateStep,
}: StepManagerProps) {
    return (
        <div className="space-y-4">
            <div className="flex items-center justify-between">
                <h3 className="text-sm font-medium">Steps</h3>
                <Button variant="outline" size="sm" onClick={onAddStep}>
                    <Plus className="h-4 w-4 mr-2" />
                    Add Step
                </Button>
            </div>

            <div className="space-y-2">
                {steps.map((step, index) => (
                    <div
                        key={step.id}
                        className={cn(
                            "group flex items-center gap-2 rounded-md border p-2 text-sm transition-colors hover:bg-muted/50",
                            activeStepId === step.id && "border-primary bg-muted"
                        )}
                        onClick={() => onStepSelect(step.id)}
                    >
                        <div className="flex h-6 w-6 items-center justify-center rounded-full bg-muted text-xs font-medium text-muted-foreground group-hover:bg-background">
                            {index + 1}
                        </div>
                        <div className="flex-1 truncate">
                            <input
                                type="text"
                                value={step.title}
                                onChange={(e) => onUpdateStep(step.id, { title: e.target.value })}
                                className="w-full bg-transparent font-medium focus:outline-none"
                                placeholder="Step Title"
                            />
                        </div>
                        {steps.length > 1 && (
                            <Button
                                variant="ghost"
                                size="icon"
                                className="h-6 w-6 opacity-0 group-hover:opacity-100"
                                onClick={(e) => {
                                    e.stopPropagation();
                                    onDeleteStep(step.id);
                                }}
                            >
                                <Trash2 className="h-3 w-3 text-destructive" />
                            </Button>
                        )}
                    </div>
                ))}
            </div>
        </div>
    );
}
