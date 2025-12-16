import { defineConfig } from 'vite'
import preact from '@preact/preset-vite'

export default defineConfig(() => {
  const useMockData = process.env.VITE_USE_MOCK_DATA === 'true'

  return {
    plugins: [preact()],
    test: {
      globals: true,
      environment: 'jsdom',
      setupFiles: './vitest.setup.js',
      env: {
        MODE: 'test',
        VITE_USE_MOCK_DATA: 'false'
      }
    },
    build: {
      outDir: 'dist',
      emptyOutDir: true,
      assetsDir: 'assets',
      rollupOptions: {
        output: {
          manualChunks: undefined,
        },
      },
    },
    // API proxy for development server (disabled when using mock data)
    server: useMockData ? {} : {
      proxy: {
        '/api': {
          target: 'http://localhost:9080',
          changeOrigin: true,
        },
      },
    },
  }
})
