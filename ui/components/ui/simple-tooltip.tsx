import { HelpCircle } from "lucide-react"

export function SimpleTooltip({ content }: { content: string }) {
    return (
        <div className="group relative flex items-center ml-2">
            <HelpCircle className="h-4 w-4 text-muted-foreground cursor-help" />
            <div className="absolute bottom-full left-1/2 mb-2 hidden w-64 -translate-x-1/2 rounded-md bg-popover p-2 text-xs text-popover-foreground shadow-md border group-hover:block z-50">
                {content}
            </div>
        </div>
    )
}
