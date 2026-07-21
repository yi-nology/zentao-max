import { defineConfig } from 'vite'
import type { UserConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import wails from '@wailsio/runtime/plugins/vite'

const config: UserConfig = {
  plugins: [vue(), wails('./bindings')],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  base: './',
  server: {
    port: 6100,
    proxy: {
      '/api': {
        target: 'http://localhost:12345',
        changeOrigin: true,
        secure: false
      },
      '/mcp': {
        target: 'http://localhost:12345',
        changeOrigin: true,
        secure: false
      },
      '/health': {
        target: 'http://localhost:12345',
        changeOrigin: true,
        secure: false
      },
      '/metrics': {
        target: 'http://localhost:12345',
        changeOrigin: true,
        secure: false
      }
    }
  }
}

export default defineConfig(config)
