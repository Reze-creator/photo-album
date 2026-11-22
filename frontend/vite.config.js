import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 开发期模拟 Nginx 的分流，页面始终使用同源相对 URL。
const backend = process.env.ALBUM_DEV_API || 'http://127.0.0.1:8080'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '127.0.0.1',
    proxy: { '/api': backend, '/uploads': backend },
  },
})
