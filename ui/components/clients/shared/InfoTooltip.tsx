"use client"

import { HelpCircle } from "lucide-react"
import {
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
} from "@/components/ds"

interface InfoTooltipProps {
    content: string
    title?: string
    example?: string
}

export function InfoTooltip({ content, title, example }: InfoTooltipProps) {
    return (
        <TooltipProvider delayDuration={250}>
            <Tooltip>
                <TooltipTrigger asChild>
                    <HelpCircle className="h-3.5 w-3.5 text-muted-foreground/60 hover:text-muted-foreground cursor-help ml-1 inline transition-colors" />
                </TooltipTrigger>
                <TooltipContent className="max-w-sm" side="top">
                    <div className="space-y-1">
                        {title && (
                            <p className="text-xs font-medium">{title}</p>
                        )}
                        <p className="text-xs text-muted-foreground">{content}</p>
                        {example && (
                            <p className="text-xs text-muted-foreground/70 italic">
                                Ej: {example}
                            </p>
                        )}
                    </div>
                </TooltipContent>
            </Tooltip>
        </TooltipProvider>
    )
}
