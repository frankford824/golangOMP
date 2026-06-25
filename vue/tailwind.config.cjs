/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    fontFamily: {
      sans: ['var(--yb-font-text)'],
      serif: ['var(--yb-font-text)'],
      mono: ['var(--yb-font-data)'],
      headline: ['var(--yb-font-display)'],
      body: ['var(--yb-font-text)'],
    },
    extend: {
      colors: {
        surface: 'rgb(var(--yb-legacy-surface) / <alpha-value>)',
        primary: 'rgb(var(--yb-brand) / <alpha-value>)',
        accent: 'rgb(var(--yb-brand-strong) / <alpha-value>)',
        sidebar: 'rgb(var(--yb-legacy-sidebar) / <alpha-value>)',
        'sidebar-hover': 'rgb(var(--yb-legacy-sidebar-hover) / <alpha-value>)',
        brand: 'rgb(var(--yb-brand) / <alpha-value>)',
        'brand-strong': 'rgb(var(--yb-brand-strong) / <alpha-value>)',
        'brand-soft': 'rgb(var(--yb-brand-soft) / <alpha-value>)',
        border: 'rgb(var(--yb-border) / <alpha-value>)',
        text: 'rgb(var(--yb-text) / <alpha-value>)',
        muted: 'rgb(var(--yb-text-muted) / <alpha-value>)',
        'glass-border': 'rgb(var(--yb-surface) / 0.5)',
        'glass-bg': 'rgb(var(--yb-surface) / 0.72)',
      },
      boxShadow: {
        glass: '0 4px 24px -4px rgb(var(--yb-shadow-stone) / 0.06)',
        float: '0 12px 48px -12px rgb(var(--yb-shadow-stone) / 0.08)',
        soft: '0 2px 8px -2px rgb(var(--yb-shadow-stone) / 0.04)',
      },
    },
  },
  plugins: [],
}
