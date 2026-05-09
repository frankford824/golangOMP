/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
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
      fontFamily: {
        headline: ['Manrope', 'sans-serif'],
        body: ['Inter', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
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
