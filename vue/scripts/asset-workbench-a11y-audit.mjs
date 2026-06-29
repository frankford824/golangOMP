import { mkdir, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

import {
  assetAuditPages,
  installAssetWorkbenchFixture,
  runDomAudit,
  startAssetVite,
  waitForServer,
} from './asset-workbench-fixture.mjs'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const port = Number(process.env.ASSET_A11Y_PORT ?? 4185)
const baseUrl = process.env.ASSET_A11Y_BASE_URL ?? `http://127.0.0.1:${port}`
const outputPath = path.join(rootDir, 'tests', 'asset-workbench', 'a11y', 'latest.json')
const startedServer = process.env.ASSET_A11Y_BASE_URL ? null : startAssetVite(rootDir, port, 'asset-a11y')

try {
  await waitForServer(baseUrl)
  await runAudit()
} finally {
  startedServer?.kill('SIGTERM')
}

async function runAudit() {
  const browser = await chromium.launch({ headless: true })
  const reports = []
  const failures = []

  try {
    for (const entry of assetAuditPages) {
      const context = await browser.newContext({
        viewport: { width: 1440, height: 900 },
        deviceScaleFactor: 1,
        colorScheme: 'light',
        locale: 'zh-CN',
        timezoneId: 'Asia/Shanghai',
        reducedMotion: 'reduce',
      })
      await installAssetWorkbenchFixture(context, entry.role)
      const page = await context.newPage()
      page.setDefaultTimeout(Number(process.env.ASSET_A11Y_PAGE_TIMEOUT_MS ?? 45_000))
      await page.goto(new URL(entry.path, baseUrl).toString(), { waitUntil: 'domcontentloaded' })
      await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})
      await waitForReady(page, entry)
      await entry.prepare?.(page)
      const report = await page.evaluate(runDomAudit)
      report.name = entry.name
      report.path = entry.path
      report.role = entry.role
      reports.push(report)
      for (const issue of report.issues) failures.push(`${entry.name}: ${issue}`)
      await context.close()
    }
  } finally {
    await browser.close()
  }

  await mkdir(path.dirname(outputPath), { recursive: true })
  await writeFile(outputPath, `${JSON.stringify({ generatedAt: new Date().toISOString(), baseUrl, pages: reports }, null, 2)}\n`, 'utf8')

  console.log('Asset workbench accessibility audit')
  for (const report of reports) {
    console.log(`- ${report.name}: ${report.issues.length} issues, ${report.interactiveCount} interactive elements`)
  }
  if (failures.length > 0) {
    console.error('Asset workbench accessibility audit failed:')
    for (const failure of failures) console.error(`- ${failure}`)
    console.error(`Report written to ${path.relative(rootDir, outputPath)}`)
    process.exit(1)
  }
  console.log(`Asset workbench accessibility audit passed. Report written to ${path.relative(rootDir, outputPath)}`)
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
