/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        bg: '#0D1117',
        card: '#161B22',
        border: '#30363D',
        teal: '#00DCB4',
        green: '#39FF8E',
        dim: '#8B949E',
      },
    },
  },
  plugins: [],
}
