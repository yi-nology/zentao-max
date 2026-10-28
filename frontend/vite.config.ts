import { defineConfig } from 'vite'
import type { UserConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

const config: UserConfig = {
  plugins: [vue()],
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
      // 精确匹配 /health（后端健康检查 JSON）；前缀写法会把前端路由
      // /health-check（心跳检测页）也代理到后端导致 404 白屏
      '^/health$': {
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
