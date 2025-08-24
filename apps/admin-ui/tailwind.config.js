/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './app/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
    // Scan shared UI package so Tailwind picks up classes used there
    '../../packages/ui/src/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  safelist: [
    'text-[clamp(26px,6.5vw,56px)]',
    'text-[clamp(14px,2.4vw,19px)]',
    'text-[clamp(20px,3.5vw,28px)]',
    'text-[clamp(18px,3vw,24px)]',
    'text-[15px]',
    'text-[14px]',
    'md:text-[14px]',
    'text-sm',
    'text-xs',
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
};
