import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import vue from '@vitejs/plugin-vue'
import { searchForWorkspaceRoot, type UserConfig } from 'vite'

const rootDir = path.dirname(fileURLToPath(import.meta.url))

function dependencyAllowList(): string[] {
  const dependencyDir = path.join(rootDir, 'node_modules')
  const resolvedDependencyDir = fs.existsSync(dependencyDir) ? fs.realpathSync(dependencyDir) : dependencyDir
  return Array.from(new Set([searchForWorkspaceRoot(rootDir), rootDir, resolvedDependencyDir]))
}

export const sharedAlias = {
  '@': path.resolve(rootDir, 'src'),
  '@aw': path.resolve(rootDir, 'src/asset-workbench'),
}

export function sharedPlugins() {
  return [vue()]
}

export function sharedManualChunks(id: string): string | undefined {
  const normalizedId = id.replace(/\\/g, '/')
  if (normalizedId.includes('vite/preload-helper') || normalizedId.includes('importAnalysisBuild')) {
    return 'asset-runtime'
  }
  if (!normalizedId.includes('node_modules')) {
    return undefined
  }
  if (normalizedId.includes('@formatjs/intl-segmenter')) {
    return 'univer-intl'
  }
  if (normalizedId.includes('@univerjs/core') || normalizedId.includes('@wendellhu')) {
    return 'univer-core'
  }
  if (normalizedId.includes('@univerjs/preset-sheets-core')) {
    return 'univer-sheets-core'
  }
  if (normalizedId.includes('@univerjs/engine-formula') || normalizedId.includes('@univerjs/sheets-formula')) {
    return 'univer-formula'
  }
  if (normalizedId.includes('@univerjs/engine-render') || normalizedId.includes('@univerjs/design')) {
    return 'univer-render'
  }
  if (
    normalizedId.includes('@univerjs/ui') ||
    normalizedId.includes('@univerjs/docs') ||
    normalizedId.includes('@univerjs/sheets-ui') ||
    normalizedId.includes('@univerjs/sheets-numfmt-ui')
  ) {
    return 'univer-ui'
  }
  if (normalizedId.includes('@univerjs/sheets') || normalizedId.includes('@univerjs/sheets-numfmt')) {
    return 'univer-sheets'
  }
  if (normalizedId.includes('@univerjs') || normalizedId.includes('node_modules/rxjs')) {
    return 'univer-runtime'
  }
  if (
    normalizedId.includes('node_modules/react') ||
    normalizedId.includes('node_modules/react-dom') ||
    normalizedId.includes('node_modules/scheduler')
  ) {
    return 'react-runtime'
  }
  if (normalizedId.includes('echarts')) {
    return 'echarts'
  }
  if (normalizedId.includes('exceljs') || normalizedId.includes('jszip')) {
    return 'office-export'
  }
  if (normalizedId.includes('naive-ui') || normalizedId.includes('@css-render')) {
    return 'ui-vendor'
  }
  if (/\/node_modules\/(?:@vue\/[^/]+|vue|vue-router|pinia)\//.test(normalizedId)) {
    return 'vue-vendor'
  }
  return 'vendor'
}

export function devServerProxy(): UserConfig['server'] {
  const target = process.env.VITE_DEV_API_PROXY_TARGET || 'http://127.0.0.1:8080'
  return {
    host: '0.0.0.0',
    fs: { allow: dependencyAllowList() },
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
