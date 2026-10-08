import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: '../web',
    emptyOutDir: true,
    assetsDir: 'assets',
    chunkSizeWarningLimit: 600,
    minify: 'esbuild',
    target: 'es2018'
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:9090'
    }
  }
})
