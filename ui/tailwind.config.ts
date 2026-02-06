import type { Config } from 'tailwindcss'

/**
 * Tailwind CSS Configuration — UI Unification Phase 1
 * Maps to CSS variables defined in app/globals.css
 *
 * @see docs/ui-unification/UI_UNIFICATION_STRATEGY.md
 */
const config: Config = {
  darkMode: 'class',
  content: [
    './app/**/*.{ts,tsx,js,jsx}',
    './components/**/*.{ts,tsx,js,jsx}',
    './lib/**/*.{ts,tsx,js,jsx}',
  ],
  theme: {
    extend: {
      colors: {
        // Base colors
        bg: 'hsl(var(--background))',
        'bg-2': 'hsl(var(--background))',
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',

        // Surface & card
        surface: 'hsl(var(--secondary))',
        'surface-hover': 'hsl(var(--secondary))',
        card: 'hsl(var(--card))',
        'card-foreground': 'hsl(var(--card-foreground))',

        // Popover
        popover: 'hsl(var(--popover))',
        'popover-foreground': 'hsl(var(--popover-foreground))',

        // Primary & secondary
        primary: 'hsl(var(--primary))',
        'primary-foreground': 'hsl(var(--primary-foreground))',
        secondary: 'hsl(var(--secondary))',
        'secondary-foreground': 'hsl(var(--secondary-foreground))',

        // Text & muted
        text: 'hsl(var(--foreground))',
        muted: 'hsl(var(--muted))',
        'muted-foreground': 'hsl(var(--muted-foreground))',
        subtle: 'hsl(var(--muted-foreground))',

        // Borders & input
        border: 'hsl(var(--border))',
        input: 'hsl(var(--input))',
        ring: 'hsl(var(--ring))',

        // Accent colors (alpha-friendly using HSL components)
        accent: 'hsl(var(--accent-h) var(--accent-s) var(--accent-l) / <alpha-value>)',
        'accent-hover': 'hsl(var(--accent-h) var(--accent-s) calc(var(--accent-l) + 5%) / <alpha-value>)',
        'accent-active': 'hsl(var(--accent-h) var(--accent-s) calc(var(--accent-l) - 5%) / <alpha-value>)',
        'accent-foreground': 'hsl(var(--accent-foreground))',
        'accent-2': 'hsl(var(--accent-2) / <alpha-value>)',

        // Semantic colors - using <alpha-value> for opacity modifier support
        // Usage: bg-success, text-success, border-success, bg-success/50
        info: 'hsl(199 89% 48% / <alpha-value>)',
        'info-foreground': 'hsl(0 0% 100%)',
        success: 'hsl(158 64% 52% / <alpha-value>)',
        'success-foreground': 'hsl(0 0% 100%)',
        warning: 'hsl(38 92% 50% / <alpha-value>)',
        'warning-foreground': 'hsl(0 0% 100%)',
        danger: 'hsl(351 95% 71% / <alpha-value>)',
        'danger-foreground': 'hsl(0 0% 100%)',

        // Destructive
        destructive: 'hsl(var(--destructive))',
        'destructive-foreground': 'hsl(var(--destructive-foreground))',

        // Sidebar
        sidebar: 'hsl(var(--sidebar))',
        'sidebar-foreground': 'hsl(var(--sidebar-foreground))',
        'sidebar-primary': 'hsl(var(--sidebar-primary))',
        'sidebar-primary-foreground': 'hsl(var(--sidebar-primary-foreground))',
        'sidebar-accent': 'hsl(var(--sidebar-accent))',
        'sidebar-accent-foreground': 'hsl(var(--sidebar-accent-foreground))',
        'sidebar-border': 'hsl(var(--sidebar-border))',
        'sidebar-ring': 'hsl(var(--sidebar-ring))',
      },
      boxShadow: {
        card: 'var(--shadow-card)',
        float: 'var(--shadow-float)',
        press: 'var(--shadow-press)',
        button: 'var(--shadow-button)',
        // Clay shadows (4-layer stacking)
        'clay-button': '0 1px 2px rgba(0,0,0,0.04), 0 2px 4px rgba(0,0,0,0.04), 0 4px 8px rgba(0,0,0,0.04), 0 6px 12px rgba(0,0,0,0.02)',
        'clay-card': '0 2px 4px rgba(0,0,0,0.04), 0 4px 8px rgba(0,0,0,0.04), 0 8px 16px rgba(0,0,0,0.04), 0 12px 24px rgba(0,0,0,0.02)',
        'clay-float': '0 4px 8px rgba(0,0,0,0.06), 0 8px 16px rgba(0,0,0,0.06), 0 16px 32px rgba(0,0,0,0.06), 0 24px 48px rgba(0,0,0,0.04)',
        'clay-modal': '0 8px 16px rgba(0,0,0,0.08), 0 16px 32px rgba(0,0,0,0.08), 0 32px 64px rgba(0,0,0,0.08), 0 48px 96px rgba(0,0,0,0.06)',
      },
      borderRadius: {
        lg: 'var(--r-lg)',
        card: 'var(--r-card)',
        md: 'var(--r-md)',
        sm: 'var(--r-sm)',
        button: 'var(--r-button)',
      },
      fontFamily: {
        body: 'var(--font-body)',
        heading: 'var(--font-heading)',
        mono: 'var(--font-mono)',
        sans: 'var(--font-sans)',
        display: ['Nunito', 'var(--font-sans)', 'sans-serif'],
      },
      transitionTimingFunction: {
        out: 'var(--ease-out)',
      },
      transitionDuration: {
        '120': 'var(--dur-1)',
        '200': 'var(--dur-2)',
        '320': 'var(--dur-3)',
      },
      spacing: {
        'page-px': 'var(--page-px)',
        'page-py': 'var(--page-py)',
        'section-gap': 'var(--section-gap)',
      },
      keyframes: {
        'blob-float': {
          '0%, 100%': { transform: 'translate(0, 0) scale(1)' },
          '33%': { transform: 'translate(30px, -50px) scale(1.1)' },
          '66%': { transform: 'translate(-20px, 20px) scale(0.9)' },
        },
        'gentle-pulse': {
          '0%, 100%': { opacity: '0.6', transform: 'scale(1)' },
          '50%': { opacity: '0.8', transform: 'scale(1.05)' },
        },
        // Progress component animations
        'shimmer': {
          '0%': { transform: 'translateX(-100%)' },
          '100%': { transform: 'translateX(200%)' },
        },
        'indeterminate': {
          '0%': { transform: 'translateX(-100%)' },
          '50%': { transform: 'translateX(100%)' },
          '100%': { transform: 'translateX(-100%)' },
        },
        'progress-glow': {
          '0%, 100%': { opacity: '0.4', filter: 'blur(8px)' },
          '50%': { opacity: '0.6', filter: 'blur(12px)' },
        },
      },
      animation: {
        'blob-float': 'blob-float 20s ease-in-out infinite',
        'gentle-pulse': 'gentle-pulse 4s ease-in-out infinite',
        'shimmer': 'shimmer 2s ease-in-out infinite',
        'indeterminate': 'indeterminate 1.5s ease-in-out infinite',
        'progress-glow': 'progress-glow 2s ease-in-out infinite',
      },
    },
  },
  plugins: [require('tailwindcss-animate')],
}

export default config
