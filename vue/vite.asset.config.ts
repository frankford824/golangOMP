import { defineConfig } from 'vite'
import fs from 'node:fs'
import path from 'node:path'
import type { Plugin } from 'vite'

import { devServerProxy, sharedAlias, sharedManualChunks, sharedPlugins } from './vite.shared'

function assetWorkbenchDevFallback(): Plugin {
  return {
    name: 'asset-workbench-dev-fallback',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const urlPath = (req.url || '/').split('?')[0] || '/'
        const isAssetWorkbenchRoute =
          !urlPath.startsWith('/@') &&
          !urlPath.startsWith('/src/') &&
          !urlPath.startsWith('/node_modules/') &&
          !urlPath.startsWith('/v1') &&
          !urlPath.startsWith('/ws') &&
          !path.extname(urlPath)

        if (!isAssetWorkbenchRoute) {
          next()
          return
        }

        try {
          const htmlPath = path.resolve(process.cwd(), 'asset.html')
          const html = fs.readFileSync(htmlPath, 'utf8')
          const transformed = await server.transformIndexHtml('/asset.html', html)
          res.statusCode = 200
          res.setHeader('Content-Type', 'text/html')
          res.end(transformed)
        } catch (error) {
          next(error)
        }
      })
    },
  }
}

export default defineConfig({
  plugins: [assetWorkbenchDevFallback(), ...sharedPlugins()],
  resolve: {
    alias: sharedAlias,
  },
  server: devServerProxy(),
  build: {
    outDir: 'dist-asset',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1100,
    sourcemap: false,
    rolldownOptions: {
      input: 'asset.html',
      checks: {
        pluginTimings: false,
      },
      output: {
        manualChunks: sharedManualChunks,
      },
    },
  },
})
