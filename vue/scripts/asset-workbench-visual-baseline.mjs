import { createHash } from 'node:crypto'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

import {
  assetAuditPages,
  installAssetWorkbenchFixture,
  startAssetVite,
  waitForServer,
} from './asset-workbench-fixture.mjs'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const mode = process.argv[2] ?? 'baseline'
const port = Number(process.env.ASSET_VISUAL_PORT ?? 4184)
const baseUrl = process.env.ASSET_VISUAL_BASE_URL ?? `http://127.0.0.1:${port}`
const outputRoot = path.join(rootDir, 'tests', 'asset-workbench', 'visual')
const baselineDir = path.join(outputRoot, 'baseline')
const currentDir = path.join(outputRoot, 'current')
const manifestName = 'manifest.json'
const enforceCompare = process.env.ASSET_VISUAL_ENFORCE === '1'
const viewports = [
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'mobile', width: 390, height: 844 },
]

if (!['baseline', 'compare'].includes(mode)) {
  console.error(`Unknown asset visual mode "${mode}". Use "baseline" or "compare".`)
  process.exit(2)
}

const startedServer = process.env.ASSET_VISUAL_BASE_URL ? null : startAssetVite(rootDir, port, 'asset-visual')

try {
  await waitForServer(baseUrl)
  await runVisualPass()
} finally {
  startedServer?.kill('SIGTERM')
}

async function runVisualPass() {
  const targetDir = mode === 'baseline' ? baselineDir : currentDir
  await mkdir(targetDir, { recursive: true })
  const browser = await chromium.launch({ headless: true })
  const manifest = {
    mode,
    baseUrl,
    generatedAt: new Date().toISOString(),
    reportOnly: mode === 'compare' && !enforceCompare,
    viewports,
    screenshots: [],
  }

  try {
    for (const viewport of viewports) {
      for (const entry of assetAuditPages) {
        const context = await browser.newContext({
          viewport,
          deviceScaleFactor: 1,
          colorScheme: 'light',
          locale: 'zh-CN',
          timezoneId: 'Asia/Shanghai',
          reducedMotion: 'reduce',
        })
        await installAssetWorkbenchFixture(context, entry.role)
        const page = await context.newPage()
        page.setDefaultTimeout(Number(process.env.ASSET_VISUAL_PAGE_TIMEOUT_MS ?? 45_000))
        await page.goto(new URL(entry.path, baseUrl).toString(), { waitUntil: 'domcontentloaded' })
        await page.addStyleTag({
          content: `
            *, *::before, *::after {
              animation-duration: 0s !important;
              animation-delay: 0s !important;
              transition-duration: 0s !important;
              transition-delay: 0s !important;
              scroll-behavior: auto !important;
              caret-color: transparent !important;
            }
          `,
        })
        await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})
        await waitForReady(page, entry)
        await entry.prepare?.(page)
        await page.waitForTimeout(250)
        const fileName = screenshotFileName(viewport, entry)
        const outputPath = path.join(targetDir, fileName)
        await page.screenshot({ path: outputPath, fullPage: true, animations: 'disabled' })
        manifest.screenshots.push({
          id: `${viewport.name}:${entry.name}`,
          name: entry.name,
          role: entry.role,
          viewport: viewport.name,
          path: entry.path,
          file: fileName,
          sha256: await sha256File(outputPath),
        })
        await context.close()
      }
    }
  } finally {
    await browser.close()
  }

  await writeFile(path.join(targetDir, manifestName), `${JSON.stringify(manifest, null, 2)}\n`, 'utf8')
  if (mode === 'baseline') {
    console.log(`Wrote asset-workbench visual baseline to ${path.relative(rootDir, baselineDir)}`)
    return
  }
  await compareAgainstBaseline(manifest)
}

async function compareAgainstBaseline(currentManifest) {
  const baselinePath = path.join(baselineDir, manifestName)
  let baseline
  try {
    baseline = JSON.parse(await readFile(baselinePath, 'utf8'))
  } catch {
    console.error(`Asset visual baseline missing: ${path.relative(rootDir, baselinePath)}`)
    process.exit(enforceCompare ? 1 : 0)
  }

  const baselineById = new Map(baseline.screenshots.map((shot) => [shot.id, shot]))
  const diffs = []
  for (const shot of currentManifest.screenshots) {
    const expected = baselineById.get(shot.id)
    if (!expected) {
      diffs.push(`${shot.id}: missing baseline`)
      continue
    }
    if (expected.sha256 !== shot.sha256) diffs.push(`${shot.id}: screenshot hash changed`)
  }

  if (diffs.length === 0) {
    console.log('Asset visual comparison passed with exact screenshot hashes.')
    return
  }

  console.error(`Asset visual comparison found ${diffs.length} change(s):`)
  for (const diff of diffs) console.error(`- ${diff}`)
  console.error(`Current screenshots are in ${path.relative(rootDir, currentDir)}`)
  if (enforceCompare) process.exit(1)
  console.error('Report-only mode: set ASSET_VISUAL_ENFORCE=1 to fail on diffs.')
}

async function waitForReady(page, entry) {
  try {
    await page.waitForSelector(entry.ready, { state: 'visible' })
  } catch (error) {
    await page.reload({ waitUntil: 'domcontentloaded' })
    await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})
    await page.waitForSelector(entry.ready, { state: 'visible' })
  }
}

function screenshotFileName(viewport, entry) {
  return `${viewport.name}-${entry.name}.png`
}

async function sha256File(filePath) {
  const data = await readFile(filePath)
  return createHash('sha256').update(data).digest('hex')
}
