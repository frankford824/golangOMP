import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'
import { fileURLToPath } from 'node:url'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'VITE_')
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
        '@': path.join(path.dirname(fileURLToPath(import.meta.url)), 'src'),
      },
    },
    server: {
      host: '0.0.0.0',
      proxy,
    },
    build: {
      // `vite build --mode test` 输出到 `dist-test/`，避免覆盖生产 `dist/`
      outDir: mode === 'test' ? 'dist-test' : 'dist',
    },
  }
})
