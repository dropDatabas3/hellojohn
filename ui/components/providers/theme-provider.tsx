'use client'

import * as React from 'react'
import {
  ThemeProvider as NextThemesProvider,
  type ThemeProviderProps,
} from 'next-themes'

/**
 * Canonical ThemeProvider for HelloJohn Admin.
 * Uses next-themes with class-based theme switching.
 * Supports 'dark' and 'light' modes with separate palettes.
 *
 * @see docs/ui-unification/UI_UNIFICATION_STRATEGY.md
 */
export function ThemeProvider({ children, ...props }: ThemeProviderProps) {
  return (
    <NextThemesProvider
      attribute="class"
      defaultTheme="dark"
      enableSystem={false}
      disableTransitionOnChange
      {...props}
    >
      {children}
    </NextThemesProvider>
  )
}
