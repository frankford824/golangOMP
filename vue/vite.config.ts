import { defineConfig, loadEnv, searchForWorkspaceRoot } from 'vite'
import vue from '@vitejs/plugin-vue'
import fs from 'node:fs'
import path from 'path'
import { fileURLToPath } from 'node:url'
import { sharedManualChunks } from './vite.shared'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'VITE_')
  const rootDir = path.dirname(fileURLToPath(import.meta.url))
  const dependencyDir = path.join(rootDir, 'node_modules')
  const resolvedDependencyDir = fs.existsSync(dependencyDir) ? fs.realpathSync(dependencyDir) : dependencyDir
  const devProxyTarget = env.VITE_DEV_API_PROXY_TARGET?.trim()
  const proxy = devProxyTarget
    ? {
        // 开发机：将相对 `/v1/...` 转发到后端（仅影响 `vite dev`）
        '/v1': {
          target: devProxyTarget,
          changeOrigin: true,
          secure: false,
          rewrite: (path: string) => path,
        },
      }
    : undefined

  return {
    plugins: [vue()],
    resolve: {
      alias: {
        '@': path.join(rootDir, 'src'),
        '@aw': path.join(rootDir, 'src/asset-workbench'),
      },
    },
    server: {
      host: '0.0.0.0',
      proxy,
      fs: { allow: Array.from(new Set([searchForWorkspaceRoot(rootDir), rootDir, resolvedDependencyDir])) },
    },
    build: {
      // `vite build --mode test` 输出到 `dist-test/`，避免覆盖生产 `dist/`
      outDir: mode === 'test' ? 'dist-test' : 'dist',
      chunkSizeWarningLimit: 1100,
      rollupOptions: {
        output: {
          manualChunks: sharedManualChunks,
        },
      },
    },
  }
})
