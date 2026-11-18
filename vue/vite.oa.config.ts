import { defineConfig, type Plugin } from 'vite'
import { devServerProxy, sharedAlias, sharedManualChunks, sharedPlugins } from './vite.shared'

const server = devServerProxy()
const api = process.env.VITE_DEV_OA_API_PROXY_TARGET || process.env.VITE_DEV_API_PROXY_TARGET || 'http://127.0.0.1:8080'

function oaEntries(): Plugin {
  const workbenchRouters = ['/src/router/index.ts', '/src/asset-workbench/router.ts']
  return {
    name: 'oa-independent-workbenches',
    enforce: 'pre',
    configureServer(vite) {
      vite.middlewares.use((req, _res, next) => {
        const url = req.url?.split('?')[0]
        if (url === '/') req.url = '/oa.html'
        next()
      })
    },
    transform(code, id) {
      const path = id.replaceAll('\\', '/').split('?')[0]
      if (workbenchRouters.some(router => path.endsWith(router))) {
        if (!code.includes('createWebHistory()')) throw new Error('OA router integration must be updated after changing history')
        return code.replaceAll('createWebHistory', 'createWebHashHistory')
      }
      if (!id.includes('?') && path.endsWith('/src/views/UnifiedTaskCreateView.vue')) {
        const line = '  if (currentDraftId.value) await restoreDraft(currentDraftId.value)'
        const marker = "import { computed, markRaw, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'"
        if (!code.includes(line) || !code.includes(marker)) throw new Error('OA task source prefill must be updated after changing the task form')
        return code.replace(marker, marker + "\nimport { getOrderTaskContext } from '@/oa/orderTask'")
          .replace(line, line + "\n  else { const source = getOrderTaskContext(); if (source && intent.value === 'retouch' && rows.value[0]) rows.value[0].erp_sku = source.sku }")
      }
      if (path.endsWith('/src/services/api/tasksApi.ts')) {
        const create = /create: \(payload: Record<string, unknown>, signal\?: AbortSignal, idempotencyKey\?: string\) =>\s*http\.post\('\/v1\/tasks', payload, \{\s*signal,\s*headers: idempotencyKey \? \{ 'Idempotency-Key': idempotencyKey \} : undefined,\s*\}\),/
        if (!create.test(code)) throw new Error('OA task creation integration must be updated after changing tasksApi.create')
        return "import { createOAOrderTask } from '@/oa/orderTask'\n" + code.replace(create, 'create: createOAOrderTask,')
      }
      if (path.endsWith('/src/services/mediaDelivery.ts')) {
        // This candidate never probes the production LAN media gateway.
        return code
          .replace("const GATEWAY = 'https://media-cache.yongbo.cloud'", "const GATEWAY = window.location.origin")
          .replace('const capability = capabilities.data.data', 'const capability = { ...capabilities.data.data, lan_available: false }')
      }
    },
    transformIndexHtml(_html, context) {
      if (context.server) return []
      return [{
        tag: 'meta',
        attrs: {
          'http-equiv': 'Content-Security-Policy',
          content: "default-src 'self'; connect-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; font-src 'self' data:; worker-src 'self' blob:; frame-src 'self' blob:; form-action 'self'; base-uri 'none'; object-src 'none'",
        },
        injectTo: 'head-prepend',
      }]
    },
  }
}

export default defineConfig({
  plugins: [oaEntries(), ...sharedPlugins()],
  cacheDir: '.vite-oa',
  resolve: { alias: sharedAlias },
  define: { 'import.meta.env.VITE_WS_BASE_URL': '""', 'import.meta.env.VITE_USE_MOCK': '"false"' },
  server: { ...server, proxy: { ...server?.proxy, '/oa-api': { target: api, changeOrigin: true } } },
  build: {
    outDir: 'dist-oa',
    chunkSizeWarningLimit: 5300,
    rolldownOptions: {
      input: ['oa.html', 'oa-work.html', 'oa-assets.html'],
      output: { manualChunks: sharedManualChunks },
    },
  },
})
