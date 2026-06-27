import path from 'node:path'
import { fileURLToPath } from 'node:url'

import vue from '@vitejs/plugin-vue'
import type { UserConfig } from 'vite'

const rootDir = path.dirname(fileURLToPath(import.meta.url))

export const sharedAlias = {
  '@': path.resolve(rootDir, 'src'),
  '@aw': path.resolve(rootDir, 'src/asset-workbench'),
}

export function sharedPlugins() {
  return [vue()]
}

export function sharedManualChunks(id: string): string | undefined {
  if (!id.includes('node_modules')) {
    return undefined
  }
  if (id.includes('echarts')) {
    return 'echarts'
  }
  if (id.includes('exceljs') || id.includes('jszip')) {
    return 'office-export'
  }
  if (id.includes('naive-ui') || id.includes('@css-render')) {
    return 'ui-vendor'
  }
  if (id.includes('vue') || id.includes('pinia')) {
    return 'vue-vendor'
  }
  return 'vendor'
}

export function devServerProxy(): UserConfig['server'] {
  const target = process.env.VITE_DEV_API_PROXY_TARGET || 'http://127.0.0.1:8080'
  return {
    host: '0.0.0.0',
    proxy: {
      '/v1': {
        target,
        changeOrigin: true,
      },
      '/ws': {
        target,
        changeOrigin: true,
        ws: true,
      },
    },
  }
}
