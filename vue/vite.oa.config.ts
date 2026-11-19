import { fileURLToPath } from 'node:url'
import { readFileSync } from 'node:fs'
import { defineConfig } from 'vite'
import tailwindcss from 'tailwindcss'
import autoprefixer from 'autoprefixer'
import type { Plugin as PostCssPlugin } from 'postcss'
import mainConfig from './vite.config'

// Resolve legacy main-ops text sizes to the OA scale when building this entry.
// Vendor spreadsheets, icon glyphs and the main/asset builds retain their own metrics.
function oaTextScale(): PostCssPlugin {
  const scale = [
    [12, '--oa-meta-size'], [13, '--oa-table-size'], [14, '--oa-body-size'],
    [15, '--oa-h2-size'], [18, '--oa-h1-size'], [24, '--oa-kpi-size'],
  ] as const
  function size(value: string) {
    const match = /^(\d*\.?\d+)(px|rem)$/.exec(value.trim())
    if (!match) return value
    const px = Number(match[1]) * (match[2] === 'rem' ? 16 : 1)
    if (!px) return value
    const closest = scale.reduce((best, item) =>
      Math.abs(item[0] - px) < Math.abs(best[0] - px) ? item : best)
    return 'var(' + closest[1] + ')'
  }
  return { postcssPlugin: 'oa-text-scale', OnceExit(root) {
    if (!root.source?.input.file?.replace(/\\/g, '/').includes('/src/')) return
    root.walkDecls(/^(font-size|font-weight|letter-spacing)$/, declaration => {
      const rule = declaration.parent
      if (!rule || rule.type !== 'rule' || /material-symbol|icon|glyph|svg|watermark/i.test(rule.selector)) return
      if (declaration.prop === 'font-size') {
        declaration.value = size(declaration.value)
      } else if (declaration.prop === 'font-weight' && /^\d+$/.test(declaration.value)) {
        const w = Number(declaration.value)
        declaration.value = String(w >= 550 ? 600 : w >= 450 ? 500 : 400)
      } else if (declaration.prop === 'letter-spacing' && /^-?[.\d]+(?:px|em|rem)$/.test(declaration.value)) {
        declaration.value = '0'
      }
    })
  } }
}
export default defineConfig(config => {
  // The candidate uses the same main-ops entry, differing only in its optional feature flag and output path.
  const main = mainConfig(config)
  const api = main.server?.proxy?.['/v1']
  return {
    ...main,
    resolve: { ...main.resolve, alias: [
      ...(config.mode === 'production' ? [{ find: /^@\/mocks$/, replacement: fileURLToPath(new URL('./src/services/noMock.ts', import.meta.url)) }] : []),
      ...Object.entries(main.resolve?.alias ?? {}).map(([find, replacement]) => ({ find, replacement: String(replacement) })),
    ] },
    plugins: [...(main.plugins ?? []), {
      name: 'oa-brand-icon',
      transformIndexHtml(html: string) {
        const icon = readFileSync(new URL('./src/oa/assets/yongbo-mark.svg', import.meta.url), 'utf8')
        return html.replace(/<link rel="icon"[^>]*>/, '<link rel="icon" type="image/svg+xml" href="data:image/svg+xml,' + encodeURIComponent(icon) + '">')
      },
    }],
    cacheDir: '.vite-oa',
    css: { ...main.css, postcss: { plugins: [tailwindcss(), autoprefixer(), oaTextScale()] } },
    define: { ...main.define, 'import.meta.env.VITE_OA_FEATURES': '"true"' },
    server: {
      ...main.server,
      proxy: api ? { ...main.server?.proxy, '/oa-api': api } : main.server?.proxy,
    },
    build: { ...main.build, outDir: 'dist-oa' },
  }
})
