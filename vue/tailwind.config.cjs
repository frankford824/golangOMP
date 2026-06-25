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
        surface: '#FAFAF9',
        primary: '#44403c',
        accent: '#57534e',
        sidebar: '#1C1917',
        'sidebar-hover': '#292524',
        'glass-border': 'rgba(255, 255, 255, 0.5)',
        'glass-bg': 'rgba(255, 255, 255, 0.72)',
      },
      boxShadow: {
        glass: '0 4px 24px -4px rgba(28, 25, 23, 0.06)',
        float: '0 12px 48px -12px rgba(28, 25, 23, 0.08)',
        soft: '0 2px 8px -2px rgba(28, 25, 23, 0.04)',
      },
    },
  },
  plugins: [],
}
