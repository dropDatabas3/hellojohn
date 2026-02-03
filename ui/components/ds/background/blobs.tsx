/**
 * Background Blobs — Clay Design System
 *
 * Animated gradient blobs for background decoration.
 */

import { cn } from "../utils/cn"

export interface BackgroundBlobsProps {
  className?: string
}

export function BackgroundBlobs({ className }: BackgroundBlobsProps) {
  return (
    <div className={cn("pointer-events-none fixed inset-0 -z-10 overflow-hidden", className)}>
      {/* Blob 1 - Top Left */}
      <div
        className="absolute -left-20 -top-20 h-96 w-96 rounded-full bg-accent-1/20 blur-3xl animate-blob-float"
        style={{ animationDelay: '0s' }}
      />

      {/* Blob 2 - Top Right */}
      <div
        className="absolute -right-20 top-40 h-80 w-80 rounded-full bg-accent-2-clay/15 blur-3xl animate-blob-float animation-delay-200"
        style={{ animationDelay: '2s' }}
      />

      {/* Blob 3 - Bottom */}
      <div
        className="absolute -bottom-20 left-1/2 h-96 w-96 -translate-x-1/2 rounded-full bg-accent-1/10 blur-3xl animate-blob-float animation-delay-400"
        style={{ animationDelay: '4s' }}
      />
    </div>
  )
}
