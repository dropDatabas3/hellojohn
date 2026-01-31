'use client'

import { useToast } from '@/hooks/use-toast'
import {
  Toast,
  ToastClose,
  ToastDescription,
  ToastProvider,
  ToastTitle,
  ToastViewport,
} from './toast'

/**
 * Toaster component using DS toast styling.
 * Renders toast notifications with semantic tokens.
 *
 * @example
 * ```tsx
 * // In layout.tsx
 * <Toaster />
 *
 * // Usage with hook
 * const { toast } = useToast()
 * toast({
 *   title: "Success",
 *   description: "Your changes have been saved",
 *   variant: "success"
 * })
 * ```
 */
export function Toaster() {
  const { toasts } = useToast()

  return (
    <ToastProvider>
      {toasts.map(function ({ id, title, description, action, ...props }) {
        return (
          <Toast key={id} {...props}>
            <div className="grid gap-1">
              {title && <ToastTitle>{title}</ToastTitle>}
              {description && <ToastDescription>{description}</ToastDescription>}
            </div>
            {action}
            <ToastClose />
          </Toast>
        )
      })}
      <ToastViewport />
    </ToastProvider>
  )
}
