/// <reference types="vitest/config" />
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
      proxy: isDev
        ? {
            '/api': {
              target: backendUrl,
              changeOrigin: true,
            },
            // 认证接口也走后端，保证本地开发登录行为与生产一致
            '/auth': {
              target: backendUrl,
              changeOrigin: true,
            },
          }
        : undefined,
    },
    test: {
      environment: 'node',
      include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
    },
  }
})
