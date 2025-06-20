import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const isDev = mode === 'development'
  const backendUrl = process.env.VITE_BACKEND_URL || 'http://localhost:8080'
  
  return {
    plugins: [react()],
    server: {
      host: '0.0.0.0',
      port: 5173,
      proxy: isDev ? {
        '/api': {
          target: backendUrl,
          changeOrigin: true,
        },
      } : undefined,
    },
  }
})
