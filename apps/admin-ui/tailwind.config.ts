import type { Config } from 'tailwindcss'

export default {
  content: [
    './app/**/*.{js,ts,jsx,tsx}',
    './components/**/*.{js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        bg: '#0b0b12',
        card: '#11111a',
        stroke: '#1f1f2a',
        fg: '#e6e6f2',
        muted: '#a5a5be',
        brand: '#8c7bff',
      },
      boxShadow: {
        glow: '0 0 40px rgba(140, 123, 255, 0.35)'
      }
    },
  },
  plugins: [],
} satisfies Config
