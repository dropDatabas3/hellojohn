"use client"

import { useState } from "react"
import { Copy, Check } from "lucide-react"
import { Button, cn } from "@/components/ds"

interface CodeSnippetProps {
    code: string
    language: string
    filename: string
}

export function CodeSnippet({ code, language, filename }: CodeSnippetProps) {
    const [copied, setCopied] = useState(false)

    const handleCopy = () => {
        navigator.clipboard.writeText(code)
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
    }

    const lines = code.split("\n")

    return (
        <div className="rounded-xl border overflow-hidden bg-[#0d1117] shadow-lg">
            {/* Header */}
            <div className="flex items-center justify-between px-4 py-3 bg-[#161b22] border-b border-white/5">
                <div className="flex items-center gap-3">
                    {/* Traffic lights */}
                    <div className="flex items-center gap-1.5">
                        <div className="w-3 h-3 rounded-full bg-[#ff5f57]" />
                        <div className="w-3 h-3 rounded-full bg-[#febc2e]" />
                        <div className="w-3 h-3 rounded-full bg-[#28c840]" />
                    </div>
                    <span className="text-sm text-[#8b949e] font-mono">{filename}</span>
                </div>
                <Button
                    variant="ghost"
                    size="sm"
                    onClick={handleCopy}
                    className={cn(
                        "h-8 px-3 text-xs font-medium transition-all duration-200",
                        copied
                            ? "text-green-400 bg-green-400/10 hover:bg-green-400/15 hover:text-green-400"
                            : "text-[#8b949e] hover:text-[#c9d1d9] hover:bg-white/5"
                    )}
                >
                    {copied ? (
                        <>
                            <Check className="h-3.5 w-3.5 mr-1.5" />
                            Copiado
                        </>
                    ) : (
                        <>
                            <Copy className="h-3.5 w-3.5 mr-1.5" />
                            Copiar código
                        </>
                    )}
                </Button>
            </div>

            {/* Code block */}
            <div className="overflow-x-auto max-h-[350px] overflow-y-auto scrollbar-thin scrollbar-thumb-white/10 scrollbar-track-transparent">
                <pre className="p-4">
                    <code className="text-[#c9d1d9] font-mono text-[13px] leading-6">
                        {lines.map((line, i) => (
                            <div key={i} className="flex hover:bg-white/[0.02] -mx-4 px-4">
                                <span className="select-none text-[#484f58] w-10 text-right pr-4 shrink-0 text-xs leading-6">
                                    {i + 1}
                                </span>
                                <span className="flex-1 whitespace-pre">{highlightLine(line)}</span>
                            </div>
                        ))}
                    </code>
                </pre>
            </div>
        </div>
    )
}

// ============================================================================
// SYNTAX HIGHLIGHTING
// ============================================================================

function highlightLine(line: string): React.ReactNode {
    // Comments
    if (line.trimStart().startsWith("//")) {
        return <span className="text-[#8b949e] italic">{line}</span>
    }

    return highlightSyntax(line)
}

function highlightSyntax(line: string): React.ReactNode {
    // Split by strings first to preserve them
    const parts = line.split(/(["'`][^"'`]*["'`])/)

    return (
        <>
            {parts.map((part, i) => {
                // String literals - green
                if (/^["'`]/.test(part)) {
                    return <span key={i} className="text-[#a5d6ff]">{part}</span>
                }

                // Process keywords and other tokens
                return (
                    <span key={i}>
                        {part.split(/\b/).map((word, j) => {
                            // Keywords - purple/pink
                            if (/^(import|from|export|const|let|var|function|return|if|else|await|async|package|func|main|type|interface|class|new|this)$/.test(word)) {
                                return <span key={j} className="text-[#ff7b72]">{word}</span>
                            }
                            // Boolean/null values - orange
                            if (/^(true|false|null|undefined|nil|err)$/.test(word)) {
                                return <span key={j} className="text-[#ffa657]">{word}</span>
                            }
                            // Numbers
                            if (/^\d+$/.test(word)) {
                                return <span key={j} className="text-[#79c0ff]">{word}</span>
                            }
                            return word
                        })}
                    </span>
                )
            })}
        </>
    )
}
