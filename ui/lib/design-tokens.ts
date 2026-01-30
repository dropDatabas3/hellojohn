/**
 * Design Tokens - Sistema de diseño centralizado
 * Inspirado en los principios de diseño de Google, Apple y Amazon
 */

export const designTokens = {
    // Paleta de colores Light Mode
    light: {
        background: {
            primary: '#FFFFFF',
            secondary: '#F8F9FC',
            sidebar: '#F8F9FC',
            muted: '#F3F4F6',
            elevated: '#FFFFFF',
        },
        text: {
            primary: '#111827',
            secondary: '#6B7280',
            muted: '#9CA3AF',
            inverse: '#FFFFFF',
        },
        accent: {
            primary: '#725DEB',
            primaryHover: '#8B7AFF',
            primaryActive: '#5F4DD1',
            light: 'rgba(114, 93, 235, 0.1)',
            lighter: 'rgba(114, 93, 235, 0.05)',
        },
        border: {
            default: '#E5E7EB',
            light: '#F3F4F6',
            dark: '#D1D5DB',
        },
        status: {
            success: '#10B981',
            warning: '#F59E0B',
            error: '#EF4444',
            info: '#3B82F6',
        },
    },

    // Paleta de colores Dark Mode
    dark: {
        background: {
            primary: '#0F1117',
            secondary: '#13151D',
            sidebar: '#13151D',
            muted: '#1A1D28',
            elevated: '#1F2937',
        },
        text: {
            primary: '#F9FAFB',
            secondary: '#D1D5DB',
            muted: '#9CA3AF',
            inverse: '#111827',
        },
        accent: {
            primary: '#8B7AFF',
            primaryHover: '#A195FF',
            primaryActive: '#725DEB',
            light: 'rgba(139, 122, 255, 0.15)',
            lighter: 'rgba(139, 122, 255, 0.08)',
        },
        border: {
            default: '#374151',
            light: '#2D3748',
            dark: '#4B5563',
        },
        status: {
            success: '#34D399',
            warning: '#FBBF24',
            error: '#F87171',
            info: '#60A5FA',
        },
    },

    // Sistema de sombras
    shadows: {
        sm: '0 1px 2px 0 rgba(0, 0, 0, 0.05)',
        md: '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)',
        lg: '0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05)',
        xl: '0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04)',
        colored: '0 4px 12px rgba(114, 93, 235, 0.2)',
        coloredHover: '0 8px 16px rgba(114, 93, 235, 0.25)',
        inner: 'inset 0 2px 4px 0 rgba(0, 0, 0, 0.06)',
    },

    // Sombras para dark mode
    shadowsDark: {
        sm: '0 1px 2px 0 rgba(0, 0, 0, 0.3)',
        md: '0 4px 6px -1px rgba(0, 0, 0, 0.4), 0 2px 4px -1px rgba(0, 0, 0, 0.3)',
        lg: '0 10px 15px -3px rgba(0, 0, 0, 0.5), 0 4px 6px -2px rgba(0, 0, 0, 0.4)',
        xl: '0 20px 25px -5px rgba(0, 0, 0, 0.6), 0 10px 10px -5px rgba(0, 0, 0, 0.5)',
        colored: '0 4px 12px rgba(139, 122, 255, 0.3)',
        coloredHover: '0 8px 16px rgba(139, 122, 255, 0.4)',
        inner: 'inset 0 2px 4px 0 rgba(0, 0, 0, 0.3)',
    },

    // Sistema de animaciones
    animations: {
        transition: {
            base: 'all 0.2s cubic-bezier(0.4, 0, 0.2, 1)',
            fast: 'all 0.15s cubic-bezier(0.4, 0, 0.2, 1)',
            slow: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
            spring: 'all 0.3s cubic-bezier(0.68, -0.55, 0.265, 1.55)',
        },
        duration: {
            fast: '150ms',
            base: '200ms',
            slow: '300ms',
        },
        easing: {
            default: 'cubic-bezier(0.4, 0, 0.2, 1)',
            in: 'cubic-bezier(0.4, 0, 1, 1)',
            out: 'cubic-bezier(0, 0, 0.2, 1)',
            inOut: 'cubic-bezier(0.4, 0, 0.2, 1)',
        },
    },

    // Espaciado
    spacing: {
        xs: '0.25rem',  // 4px
        sm: '0.5rem',   // 8px
        md: '1rem',     // 16px
        lg: '1.5rem',   // 24px
        xl: '2rem',     // 32px
        '2xl': '3rem',  // 48px
    },

    // Radios de borde
    borderRadius: {
        sm: '0.25rem',  // 4px
        md: '0.375rem', // 6px
        lg: '0.5rem',   // 8px
        xl: '0.75rem',  // 12px
        '2xl': '1rem',  // 16px
        full: '9999px',
    },

    // Tipografía
    typography: {
        fontFamily: {
            sans: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
            mono: 'ui-monospace, SFMono-Regular, "SF Mono", Consolas, "Liberation Mono", Menlo, monospace',
        },
        fontSize: {
            xs: '0.75rem',   // 12px
            sm: '0.875rem',  // 14px
            base: '1rem',    // 16px
            lg: '1.125rem',  // 18px
            xl: '1.25rem',   // 20px
            '2xl': '1.5rem', // 24px
        },
        fontWeight: {
            normal: '400',
            medium: '500',
            semibold: '600',
            bold: '700',
        },
    },
}

// Tipos para TypeScript
export type DesignTokens = typeof designTokens
export type ColorMode = 'light' | 'dark'
