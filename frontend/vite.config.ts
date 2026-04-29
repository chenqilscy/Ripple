import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const stagingAllowedOrigin = 'http://127.0.0.1:4173'
const mainEntry = fileURLToPath(new URL('./index.html', import.meta.url))
const subscriptionHarnessEntry = fileURLToPath(new URL('./subscription-harness.html', import.meta.url))
const summarizeGraphHarnessEntry = fileURLToPath(new URL('./summarize-graph-harness.html', import.meta.url))
const aiTriggerHarnessEntry = fileURLToPath(new URL('./ai-trigger-harness.html', import.meta.url))

export default defineConfig(({ mode }) => {
  const buildInput: Record<string, string> = { main: mainEntry }
  if (mode === 'e2e') {
    buildInput.subscriptionHarness = subscriptionHarnessEntry
    buildInput.summarizeGraphHarness = summarizeGraphHarnessEntry
    buildInput.aiTriggerHarness = aiTriggerHarnessEntry
  }

  return {
    build: {
      rollupOptions: {
        input: buildInput,
      },
    },
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
  }
})
