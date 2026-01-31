/**
 * UI Unification - Design Tokens (TypeScript Contract)
 *
 * This file provides type-safe access to design tokens.
 * The actual token values live in CSS variables (app/globals.css).
 *
 * @see docs/ui-unification/DESIGN_TOKENS.md
 * @see app/globals.css
 */

/**
 * Semantic color token names.
 * These map directly to CSS variables in globals.css.
 */
export const colorTokens = {
  // Base
  bg: '--background',
  'bg-2': '--background',
  surface: '--secondary',
  'surface-hover': '--secondary',
  card: '--card',

  // Text
  text: '--foreground',
  muted: '--muted',
  'muted-foreground': '--muted-foreground',
  subtle: '--muted-foreground',

  // Borders
  border: '--border',

  // Accents
  accent: '--accent',
  'accent-hover': '--accent-hover',
  'accent-active': '--accent-active',
  'accent-2': '--accent-2',

  // Semantic
  info: '--info',
  success: '--success',
  warning: '--warning',
  danger: '--danger',
} as const

/**
 * Shadow token names.
 */
export const shadowTokens = {
  card: '--shadow-card',
  float: '--shadow-float',
  press: '--shadow-press',
  button: '--shadow-button',
} as const

/**
 * Border radius token names.
 */
export const radiusTokens = {
  lg: '--r-lg',
  card: '--r-card',
  md: '--r-md',
  sm: '--r-sm',
  button: '--r-button',
} as const

/**
 * Spacing token names.
 */
export const spacingTokens = {
  'page-px': '--page-px',
  'page-py': '--page-py',
  'section-gap': '--section-gap',
} as const

/**
 * Motion/timing token names.
 */
export const motionTokens = {
  'ease-out': '--ease-out',
  'dur-1': '--dur-1',
  'dur-2': '--dur-2',
  'dur-3': '--dur-3',
} as const

/**
 * Font family token names.
 */
export const fontTokens = {
  body: '--font-body',
  heading: '--font-heading',
  mono: '--font-mono',
  sans: '--font-sans',
} as const

/**
 * All design tokens consolidated.
 */
export const tokens = {
  colors: colorTokens,
  shadows: shadowTokens,
  radii: radiusTokens,
  spacing: spacingTokens,
  motion: motionTokens,
  fonts: fontTokens,
} as const

/**
 * Helper to get a CSS variable value.
 * @param tokenName - The token name (e.g., '--accent')
 * @returns The CSS variable string 'var(--accent)'
 */
export function getCSSVar(tokenName: string): string {
  return `var(${tokenName})`
}

/**
 * Helper to get a token value from the DOM (runtime).
 * @param tokenName - The token name (e.g., '--accent')
 * @returns The computed value (e.g., 'hsl(258, 77%, 57%)')
 */
export function getTokenValue(tokenName: string): string {
  if (typeof window === 'undefined') return ''
  return getComputedStyle(document.documentElement).getPropertyValue(tokenName).trim()
}
