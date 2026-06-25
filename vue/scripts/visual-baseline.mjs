import { spawn } from 'node:child_process'
import { createHash } from 'node:crypto'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const mode = process.argv[2] ?? 'baseline'
const port = Number(process.env.VISUAL_PORT ?? 4174)
const baseUrl = process.env.VISUAL_BASE_URL ?? `http://127.0.0.1:${port}`
const outputRoot = path.join(rootDir, 'tests', 'visual')
const baselineDir = path.join(outputRoot, 'baseline')
const currentDir = path.join(outputRoot, 'current')
const manifestName = 'manifest.json'

const fixedNow = Date.parse(process.env.VISUAL_FIXED_NOW ?? '2026-06-24T12:00:00+08:00')
const pageTimeoutMs = Number(process.env.VISUAL_PAGE_TIMEOUT_MS ?? 60_000)
const maxDiffRatio = Number(process.env.VISUAL_MAX_DIFF_RATIO ?? 0.001)
const channelThreshold = Number(process.env.VISUAL_CHANNEL_THRESHOLD ?? 8)
const viewportProfiles = resolveViewportProfiles()

const pages = [
  { name: 'dashboard', path: '/', ready: '.dashboard-shell' },
  {
    name: 'avatar-dropdown-open',
    path: '/',
    ready: '.dashboard-shell',
    prepare: async (page) => {
      await page.locator('.avatar-trigger').click()
      await page.waitForSelector('.avatar-dropdown-menu', { state: 'visible' })
      await page.waitForTimeout(200)
    },
  },
  {
    name: 'global-search-overlay',
    path: '/',
    ready: '.dashboard-shell',
    prepare: async (page) => {
      await page.keyboard.press(process.platform === 'darwin' ? 'Meta+K' : 'Control+K')
      await page.waitForSelector('.global-search-panel', { state: 'visible' })
      await page.locator('.global-search-panel input').fill('task')
      await page.waitForTimeout(900)
    },
  },
  { name: 'task-list', path: '/tasks', ready: '.task-list-view' },
  { name: 'task-create-modal', path: '/tasks/create', ready: '.create-task-modal-panel' },
  {
    name: 'base-select-open',
    path: '/tasks/create',
    ready: '.create-task-modal-panel',
    prepare: async (page) => {
      const priorityTrigger = page
        .locator('.create-task-modal-panel label:has-text("优先级") + div button')
        .first()
      await priorityTrigger.click()
      await page.waitForSelector('.base-select-panel', { state: 'visible' })
      await page.waitForTimeout(200)
    },
  },
  {
    name: 'close-draft-confirm-modal',
    path: '/tasks/create',
    ready: '.create-task-modal-panel',
    prepare: async (page) => {
      await page.getByRole('button', { name: '新款单 SKU' }).click()
      await page.locator('.create-task-modal-panel .modal-close-btn').click()
      await page.waitForSelector('.close-draft-confirm-panel', { state: 'visible' })
      await page.waitForTimeout(200)
    },
  },
  { name: 'task-detail', path: '/tasks/task_1002', ready: '.task-detail-view' },
  { name: 'task-assets', path: '/tasks/task_1002/assets?asset_id=asset_1001', ready: '.task-assets-view' },
  { name: 'asset-detail', path: '/asset-center/asset_1001?task_id=task_1002', ready: '.asset-detail-view' },
  {
    name: 'task-info-edit-modal',
    path: '/tasks/task_1002',
    ready: '.task-detail-view',
    prepare: async (page) => {
      await page.getByRole('button', { name: /编辑信息|编辑母任务/ }).first().click()
      await page.waitForSelector('.task-info-edit-modal-panel', { state: 'visible' })
      await page.waitForTimeout(200)
    },
  },
  {
    name: 'reassign-designer-dialog',
    path: '/tasks/task_1002',
    ready: '.task-detail-view',
    prepare: async (page) => {
      await page.getByRole('button', { name: /重新指派/ }).first().click()
      await page.waitForSelector('.reassign-body', { state: 'visible' })
      await page.waitForTimeout(200)
    },
  },
  { name: 'assets-index', path: '/asset-center', ready: '.assets-index-view' },
  { name: 'product-management', path: '/products', ready: '.product-management-view' },
  {
    name: 'user-management-role-modal',
    path: '/users',
    ready: '.user-management-view',
    prepare: async (page) => {
      await page.waitForSelector('.link-btn', { state: 'visible' })
      await page.locator('.link-btn').first().click()
      await page.waitForSelector('.um-modal', { state: 'visible' })
      await page.waitForTimeout(200)
    },
  },
  {
    name: 'product-management-cost-tooltip',
    path: '/products',
    ready: '.product-management-view',
    prepare: async (page) => {
      if ((await page.locator('.pm-cost-help').count()) === 0) {
        await page.locator('.pm-combo-header').first().click()
      }
      await page.waitForSelector('.pm-cost-help', { state: 'visible' })
      await page.locator('.pm-cost-help').first().focus()
      await page.waitForTimeout(200)
    },
  },
]

if (!['baseline', 'compare'].includes(mode)) {
  console.error(`Unknown visual mode "${mode}". Use "baseline" or "compare".`)
  process.exit(2)
}

const startedServer = process.env.VISUAL_BASE_URL ? null : startVite()

try {
  await waitForServer(baseUrl)
  await runVisualPass()
} finally {
  if (startedServer) {
    startedServer.kill('SIGTERM')
  }
}

function startVite() {
  const viteBin = process.platform === 'win32' ? 'vite.cmd' : 'vite'
  const child = spawn(
    viteBin,
    ['--host', '127.0.0.1', '--port', String(port), '--strictPort', '--mode', 'test'],
    {
      cwd: rootDir,
      env: {
        ...process.env,
        VITE_USE_MOCK: 'true',
      },
      shell: process.platform === 'win32',
      stdio: ['ignore', 'pipe', 'pipe'],
    },
  )

  child.stdout.on('data', (chunk) => {
    process.stdout.write(`[visual:vite] ${chunk}`)
  })
  child.stderr.on('data', (chunk) => {
    process.stderr.write(`[visual:vite] ${chunk}`)
  })
  child.on('exit', (code, signal) => {
    if (code !== 0 && code !== 143 && signal !== 'SIGTERM') {
      console.error(`[visual:vite] exited with code=${code} signal=${signal}`)
    }
  })
  return child
}

async function waitForServer(url) {
  const timeoutMs = 60_000
  const startedAt = Date.now()
  let lastError
  while (Date.now() - startedAt < timeoutMs) {
    try {
      const response = await fetch(url, { method: 'GET' })
      if (response.ok) return
    } catch (error) {
      lastError = error
    }
    await sleep(500)
  }
  throw new Error(`Timed out waiting for ${url}: ${lastError?.message ?? 'no response'}`)
}

async function runVisualPass() {
  const targetDir = mode === 'baseline' ? baselineDir : currentDir
  await mkdir(targetDir, { recursive: true })

  const browser = await chromium.launch({ headless: true })
  const manifest = {
    mode,
    baseUrl,
    fixedNow: new Date(fixedNow).toISOString(),
    viewport: viewportProfiles[0],
    viewports: viewportProfiles,
    generatedAt: new Date().toISOString(),
    screenshots: [],
  }

  try {
    for (const viewport of viewportProfiles) {
      const context = await browser.newContext({
        viewport,
        deviceScaleFactor: 1,
        colorScheme: 'light',
        locale: 'zh-CN',
        timezoneId: 'Asia/Shanghai',
        reducedMotion: 'reduce',
      })

      await context.addInitScript((now) => {
        const RealDate = Date
        class FixedDate extends RealDate {
          constructor(...args) {
            if (args.length === 0) {
              super(now)
              return
            }
            super(...args)
          }
          static now() {
            return now
          }
        }
        FixedDate.UTC = RealDate.UTC
        FixedDate.parse = RealDate.parse
        FixedDate.prototype = RealDate.prototype
        window.Date = FixedDate
        window.localStorage.setItem('access_token', 'mock-token-ops-demo')
      }, fixedNow)

      try {
        for (const entry of pages) {
          const page = await context.newPage()
          page.setDefaultTimeout(pageTimeoutMs)
          await page.goto(new URL(entry.path, baseUrl).toString(), {
            waitUntil: 'domcontentloaded',
            timeout: pageTimeoutMs,
          })
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
          await page.waitForSelector(entry.ready, { state: 'visible' })
          await page.waitForTimeout(500)
          if (entry.prepare) {
            await entry.prepare(page)
          }

          const fileName = screenshotFileName(viewport, entry)
          const outputPath = path.join(targetDir, fileName)
          await page.screenshot({
            path: outputPath,
            fullPage: true,
            animations: 'disabled',
          })
          const hash = await sha256File(outputPath)
          manifest.screenshots.push({
            id: `${viewport.name}:${entry.name}`,
            name: entry.name,
            viewport: viewport.name,
            path: entry.path,
            file: fileName,
            sha256: hash,
          })
          await page.close()
        }
      } finally {
        await context.close()
      }
    }
  } finally {
    await browser.close()
  }

  await writeFile(path.join(targetDir, manifestName), `${JSON.stringify(manifest, null, 2)}\n`, 'utf8')

  if (mode === 'compare') {
    await compareAgainstBaseline(manifest)
  } else {
    console.log(`Wrote visual baseline to ${path.relative(rootDir, baselineDir)}`)
  }
}

async function compareAgainstBaseline(currentManifest) {
  const baselinePath = path.join(baselineDir, manifestName)
  const baseline = JSON.parse(await readFile(baselinePath, 'utf8'))
  const baselineByName = new Map(baseline.screenshots.map((shot) => [screenshotId(shot), shot]))
  const failures = []
  const pixelReports = []

  for (const shot of currentManifest.screenshots) {
    const expected = baselineByName.get(screenshotId(shot))
    if (!expected) {
      failures.push(`${screenshotId(shot)}: missing baseline`)
      continue
    }
    if (expected.sha256 !== shot.sha256) {
      const pixelReport = await compareImages(
        path.join(baselineDir, expected.file),
        path.join(currentDir, shot.file),
      )
      pixelReports.push(`${screenshotId(shot)}: ${formatPercent(pixelReport.diffRatio)} pixels changed`)
      if (pixelReport.diffRatio > maxDiffRatio) {
        failures.push(
          `${screenshotId(shot)}: ${formatPercent(pixelReport.diffRatio)} pixels changed ` +
            `(limit ${formatPercent(maxDiffRatio)})`,
        )
      }
    }
  }

  if (failures.length > 0) {
    console.error('Visual comparison failed:')
    for (const failure of failures) {
      console.error(`- ${failure}`)
    }
    console.error(`Current screenshots are in ${path.relative(rootDir, currentDir)}`)
    process.exit(1)
  }

  if (pixelReports.length > 0) {
    console.log('Visual comparison passed within pixel threshold:')
    for (const report of pixelReports) {
      console.log(`- ${report}`)
    }
  } else {
    console.log('Visual comparison passed with exact screenshot hashes.')
  }
}

function resolveViewportProfiles() {
  if (process.env.VISUAL_VIEWPORTS) {
    return process.env.VISUAL_VIEWPORTS.split(',')
      .map((item) => item.trim())
      .filter(Boolean)
      .map(parseViewportProfile)
  }

  if (process.env.VISUAL_VIEWPORT_WIDTH || process.env.VISUAL_VIEWPORT_HEIGHT) {
    return [
      {
        name: 'custom',
        width: Number(process.env.VISUAL_VIEWPORT_WIDTH ?? 1440),
        height: Number(process.env.VISUAL_VIEWPORT_HEIGHT ?? 900),
      },
    ]
  }

  return [
    { name: 'desktop', width: 1440, height: 900 },
    { name: 'tablet', width: 1024, height: 768 },
    { name: 'mobile', width: 390, height: 844 },
  ]
}

function parseViewportProfile(value) {
  const match = /^(?:(?<name>[a-z0-9-]+):)?(?<width>\d+)x(?<height>\d+)$/i.exec(value)
  if (!match?.groups) {
    throw new Error(`Invalid VISUAL_VIEWPORTS entry "${value}". Use "name:1440x900".`)
  }
  return {
    name: match.groups.name ?? `${match.groups.width}x${match.groups.height}`,
    width: Number(match.groups.width),
    height: Number(match.groups.height),
  }
}

function screenshotFileName(viewport, entry) {
  if (viewport.name === 'desktop') {
    return `${entry.name}.png`
  }
  return `${viewport.name}-${entry.name}.png`
}

function screenshotId(shot) {
  return shot.id ?? `${shot.viewport ?? 'desktop'}:${shot.name}`
}

async function compareImages(expectedPath, actualPath) {
  const expectedUrl = await pngDataUrl(expectedPath)
  const actualUrl = await pngDataUrl(actualPath)
  const browser = await chromium.launch({ headless: true })
  const page = await browser.newPage()
  try {
    return await page.evaluate(
      async ({ expectedUrl, actualUrl, threshold }) => {
        async function loadImage(src) {
          return new Promise((resolve, reject) => {
            const image = new Image()
            image.onload = () => resolve(image)
            image.onerror = () => reject(new Error(`Failed to load ${src}`))
            image.src = src
          })
        }

        const expected = await loadImage(expectedUrl)
        const actual = await loadImage(actualUrl)
        if (expected.width !== actual.width || expected.height !== actual.height) {
          return {
            diffPixels: Number.POSITIVE_INFINITY,
            totalPixels: Math.max(expected.width * expected.height, actual.width * actual.height),
            diffRatio: 1,
          }
        }

        const canvas = document.createElement('canvas')
        canvas.width = expected.width
        canvas.height = expected.height
        const ctx = canvas.getContext('2d', { willReadFrequently: true })
        ctx.drawImage(expected, 0, 0)
        const expectedData = ctx.getImageData(0, 0, canvas.width, canvas.height).data
        ctx.clearRect(0, 0, canvas.width, canvas.height)
        ctx.drawImage(actual, 0, 0)
        const actualData = ctx.getImageData(0, 0, canvas.width, canvas.height).data

        let diffPixels = 0
        for (let index = 0; index < expectedData.length; index += 4) {
          const dr = Math.abs(expectedData[index] - actualData[index])
          const dg = Math.abs(expectedData[index + 1] - actualData[index + 1])
          const db = Math.abs(expectedData[index + 2] - actualData[index + 2])
          const da = Math.abs(expectedData[index + 3] - actualData[index + 3])
          if (dr > threshold || dg > threshold || db > threshold || da > threshold) {
            diffPixels += 1
          }
        }

        const totalPixels = canvas.width * canvas.height
        return {
          diffPixels,
          totalPixels,
          diffRatio: totalPixels > 0 ? diffPixels / totalPixels : 0,
        }
      },
      {
        expectedUrl,
        actualUrl,
        threshold: channelThreshold,
      },
    )
  } finally {
    await browser.close()
  }
}

async function pngDataUrl(filePath) {
  const data = await readFile(filePath)
  return `data:image/png;base64,${data.toString('base64')}`
}

function formatPercent(value) {
  return `${(value * 100).toFixed(4)}%`
}

async function sha256File(filePath) {
  const data = await readFile(filePath)
  return createHash('sha256').update(data).digest('hex')
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}
