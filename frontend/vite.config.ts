import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': path.resolve(import.meta.dirname, './src') },
  },
  server: {
    proxy: {
      '/api': { target: process.env.VITE_API_PROXY_TARGET || 'http://localhost:8080', changeOrigin: true },
    },
  },
  // Keep `vite preview` usable for production-build browser validation with
  // the same API origin as development.
  preview: {
    proxy: {
      '/api': { target: process.env.VITE_API_PROXY_TARGET || 'http://localhost:8080', changeOrigin: true },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules/recharts') || id.includes('node_modules/d3-')) {
            return 'recharts'
          }
          if (id.includes('node_modules/react') || id.includes('node_modules/react-dom') || id.includes('node_modules/react-router')) {
            return 'react-vendor'
          }
        },
      },
    },
  },
})
