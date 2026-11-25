import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// Unique per deployment so an old build's failed-chunk guard cannot suppress
// recovery after the user receives a newer entry point.
const buildId =
  process.env.VERCEL_DEPLOYMENT_ID ??
  process.env.GITHUB_SHA ??
  Date.now().toString(36)

export default defineConfig({
  plugins: [react(), tailwindcss()],
  define: {
    __KWIK_BUILD_ID__: JSON.stringify(buildId),
  },
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
    // Used by scripts/check-build-assets.mjs to verify every emitted chunk and
    // stylesheet actually made it into the deployment artifact.
    manifest: true,
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
