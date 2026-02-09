import type { Config } from 'tailwindcss';

const config: Config = {
  content: [
    './src/pages/**/*.{js,ts,jsx,tsx,mdx}',
    './src/components/**/*.{js,ts,jsx,tsx,mdx}',
    './src/app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      fontFamily: {
        sans: ['-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'sans-serif'],
        mono: ['Fira Code', 'Monaco', 'Consolas', 'monospace'],
      },
      typography: {
        DEFAULT: {
          css: {
            maxWidth: 'none',
            color: '#d1d5db',
            a: {
              color: '#22d3ee',
              '&:hover': {
                color: '#67e8f9',
              },
            },
            h1: { color: '#f9fafb' },
            h2: { color: '#f3f4f6' },
            h3: { color: '#e5e7eb' },
            code: {
              color: '#f8fafc',
              backgroundColor: 'rgba(110, 118, 129, 0.4)',
              padding: '0.2rem 0.4rem',
              borderRadius: '0.25rem',
              fontWeight: '400',
            },
            'code::before': { content: '""' },
            'code::after': { content: '""' },
            pre: {
              backgroundColor: '#0d1117',
              border: '1px solid #30363d',
            },
            blockquote: {
              borderLeftColor: '#22d3ee',
              color: '#9ca3af',
            },
            strong: { color: '#f9fafb' },
            hr: { borderColor: '#374151' },
            th: { color: '#f9fafb' },
            td: { color: '#d1d5db' },
          },
        },
      },
    },
  },
  plugins: [require('@tailwindcss/typography')],
};

export default config;
