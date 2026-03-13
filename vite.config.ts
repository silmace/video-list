import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src')
    }
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) {
            return undefined
          }
          if (id.includes('vue-router')) {
            return 'vendor-router'
          }
          if (id.includes('axios')) {
            return 'vendor-network'
          }
          if (id.includes('artplayer')) {
            return 'vendor-player'
          }
          if (id.includes('lucide-vue-next')) {
            return 'vendor-icons'
          }
          return 'vendor-core'
        }
      }
    }
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:3001',
        changeOrigin: true
      }
    }
  }
})