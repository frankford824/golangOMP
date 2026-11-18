import { defineConfig } from 'vite'
import mainConfig from './vite.config'

export default defineConfig(config => {
  // The candidate uses the same main-ops entry, differing only in its optional feature flag and output path.
  const main = mainConfig(config)
  const api = main.server?.proxy?.['/v1']
  return {
    ...main,
    cacheDir: '.vite-oa',
    define: { ...main.define, 'import.meta.env.VITE_OA_FEATURES': '"true"' },
    server: {
      ...main.server,
      proxy: api ? { ...main.server?.proxy, '/oa-api': api } : main.server?.proxy,
    },
    build: { ...main.build, outDir: 'dist-oa' },
  }
})
