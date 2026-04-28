import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const stagingAllowedOrigin = 'http://127.0.0.1:4173'

export default defineConfig({
  plugins: [react()],
  resolve: {
    extensions: ['.tsx', '.ts', '.jsx', '.js', '.mjs', '.json'],
  },
  server: {
    port: 5234,
    hmr: false,
    proxy: {
      '/api': {
        target: 'http://fn.cky:18000',
        changeOrigin: false,
        ws: true,
        headers: { Origin: stagingAllowedOrigin },
        configure(proxy) {
          proxy.on('proxyReqWs', proxyReq => {
            proxyReq.setHeader('Origin', stagingAllowedOrigin)
          })
        },
      },
      '/yjs': {
        target: 'http://fn.cky:17790',
        changeOrigin: false,
        ws: true,
        headers: { Origin: stagingAllowedOrigin },
        configure(proxy) {
          proxy.on('proxyReqWs', proxyReq => {
            proxyReq.setHeader('Origin', stagingAllowedOrigin)
          })
        },
      },
      '/ws': { target: 'ws://fn.cky:18000', ws: true },
    },
  },
  test: {
    exclude: ['e2e/**', '**/node_modules/**'],
  },
})
