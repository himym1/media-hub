import { fileURLToPath } from 'node:url'
import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const envDir = fileURLToPath(new URL('.', import.meta.url))
const env = loadEnv(process.env.NODE_ENV === 'production' ? 'production' : 'development', envDir, '')
const backendTarget = env.VITE_BACKEND_URL || env.BACKEND_URL || `http://localhost:${env.VITE_BACKEND_PORT || env.BACKEND_PORT || '8088'}`

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: backendTarget,
        changeOrigin: true,
        secure: false,
      },
    },
  },
})
