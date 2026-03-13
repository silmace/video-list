import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) {
            return undefined
          }
          if (id.includes('vuetify')) {
            return 'vendor-vuetify'
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