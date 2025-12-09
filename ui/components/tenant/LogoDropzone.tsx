"use client"

import { useState, useCallback } from "react"
import { useDropzone } from "react-dropzone"
import { Upload, X, Image as ImageIcon, Check } from "lucide-react"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"

interface LogoDropzoneProps {
    value: string | null
    onChange: (base64: string | null) => void
    label?: string
}

export function LogoDropzone({ value, onChange, label }: LogoDropzoneProps) {
    const [bgMode, setBgMode] = useState<"checker" | "white" | "black">("checker")

    const onDrop = useCallback(
        (acceptedFiles: File[]) => {
            const file = acceptedFiles[0]
            if (file) {
                const reader = new FileReader()
                reader.onloadend = () => {
                    onChange(reader.result as string)
                }
                reader.readAsDataURL(file)
            }
        },
        [onChange],
    )

    const { getRootProps, getInputProps, isDragActive } = useDropzone({
        onDrop,
        accept: {
            "image/*": [".png", ".jpg", ".jpeg", ".svg", ".webp", ".gif"],
        },
        maxFiles: 1,
        multiple: false,
    })

    const removeLogo = (e: React.MouseEvent) => {
        e.stopPropagation()
        onChange(null)
    }

    return (
        <div className="space-y-3">
            {label && <label className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">{label}</label>}
            <div className="flex gap-6">
                {/* Dropzone Area */}
                <div
                    {...getRootProps()}
                    className={cn(
                        "flex-1 relative group cursor-pointer flex flex-col items-center justify-center w-full min-h-[160px] rounded-lg border-2 border-dashed transition-all duration-200 ease-in-out",
                        isDragActive
                            ? "border-primary bg-primary/5 scale-[1.01]"
                            : "border-muted-foreground/25 hover:border-primary/50 hover:bg-muted/50",
                        value ? "border-solid" : ""
                    )}
                >
                    <input {...getInputProps()} />

                    {value ? (
                        <div className="relative w-full h-full min-h-[160px] flex items-center justify-center overflow-hidden rounded-lg p-8">
                            {/* Background Layer */}
                            <div className={cn(
                                "absolute inset-0 z-0",
                                bgMode === "white" && "bg-white",
                                bgMode === "black" && "bg-zinc-950",
                                bgMode === "checker" && "bg-[url('https://res.cloudinary.com/demo/image/upload/v1675797387/checkers_pattern_q1i2l7.png')] bg-repeat opacity-50"
                            )} />

                            {/* Image Layer */}
                            <div className="relative z-10 w-full h-full max-h-[120px] max-w-[200px] flex items-center justify-center">
                                {/* eslint-disable-next-line @next/next/no-img-element */}
                                <img
                                    src={value}
                                    alt="Logo Preview"
                                    className="w-full h-full object-contain"
                                />
                            </div>

                            {/* Remove Button */}
                            <div className="absolute top-2 right-2 z-20 opacity-0 group-hover:opacity-100 transition-opacity">
                                <Button
                                    variant="destructive"
                                    size="icon"
                                    className="h-8 w-8 rounded-full shadow-md"
                                    onClick={removeLogo}
                                >
                                    <X className="h-4 w-4" />
                                </Button>
                            </div>
                        </div>
                    ) : (
                        <div className="flex flex-col items-center text-center p-6 space-y-2 text-muted-foreground">
                            <div className="bg-muted rounded-full p-4 mb-2 group-hover:bg-background transition-colors shadow-sm">
                                <Upload className="h-6 w-6 text-muted-foreground group-hover:text-primary transition-colors" />
                            </div>
                            <p className="text-sm font-medium text-foreground">Click to upload or drag and drop</p>
                            <p className="text-xs text-muted-foreground">SVG, PNG, JPG or GIF (max. 5MB)</p>
                        </div>
                    )}
                </div>

                {/* Controls (Only visible if value exists) */}
                {value && (
                    <div className="flex flex-col gap-2 pt-2 animate-in fade-in slide-in-from-left-2">
                        <span className="text-xs font-medium text-muted-foreground mb-1">Preview Background</span>
                        <div className="flex flex-col gap-2">
                            <Button
                                variant={bgMode === "checker" ? "default" : "outline"}
                                size="icon"
                                className="h-9 w-9"
                                onClick={() => setBgMode("checker")}
                                title="Checkerboard"
                            >
                                <div className="w-4 h-4 rounded-sm border border-current opacity-50 bg-[url('https://res.cloudinary.com/demo/image/upload/v1675797387/checkers_pattern_q1i2l7.png')] bg-cover" />
                            </Button>
                            <Button
                                variant={bgMode === "white" ? "default" : "outline"}
                                size="icon"
                                className="h-9 w-9"
                                onClick={() => setBgMode("white")}
                                title="White"
                            >
                                <div className="w-4 h-4 rounded-sm bg-white border border-gray-200" />
                            </Button>
                            <Button
                                variant={bgMode === "black" ? "default" : "outline"}
                                size="icon"
                                className="h-9 w-9"
                                onClick={() => setBgMode("black")}
                                title="Black"
                            >
                                <div className="w-4 h-4 rounded-sm bg-black" />
                            </Button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    )
}
