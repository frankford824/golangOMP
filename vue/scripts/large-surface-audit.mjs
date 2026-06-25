import { spawn } from 'node:child_process'
import { mkdir, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const port = Number(process.env.LOAD_PORT ?? 4176)
const baseUrl = process.env.LOAD_BASE_URL ?? `http://127.0.0.1:${port}`
const pageTimeoutMs = Number(process.env.LOAD_PAGE_TIMEOUT_MS ?? 60_000)
const defaultMaxReadyMs = Number(process.env.LOAD_MAX_READY_MS ?? 10_000)
const outputPath = path.join(rootDir, 'tests', 'load', 'latest.json')

const pages = [
  {
    name: 'task-list',
    path: '/tasks',
    ready: '.task-list-view',
    countSelector: '.task-cards > .tc',
    minItems: 80,
    maxNodes: 22_000,
    maxReadyMs: defaultMaxReadyMs,
  },
  {
    name: 'assets-index',
    path: '/asset-center',
    ready: '.assets-index-view',
    countSelector: '.ac-card',
    minItems: 80,
    maxNodes: 24_000,
    maxReadyMs: defaultMaxReadyMs,
  },
  {
    name: 'product-management',
    path: '/products',
    ready: '.product-management-view',
    countSelector: '.pm-combo-group',
    minItems: 80,
    maxNodes: 24_000,
    maxReadyMs: defaultMaxReadyMs,
  },
]

const startedServer = process.env.LOAD_BASE_URL ? null : startVite()

try {
  await waitForServer(baseUrl)
  await runLoadAudit()
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
        VITE_LARGE_SURFACE_AUDIT: 'true',
        VITE_LARGE_SURFACE_PAGE_SIZE: process.env.VITE_LARGE_SURFACE_PAGE_SIZE ?? '100',
        VITE_LARGE_SURFACE_TOTAL: process.env.VITE_LARGE_SURFACE_TOTAL ?? '5000',
      },
      shell: process.platform === 'win32',
      stdio: ['ignore', 'pipe', 'pipe'],
    },
  )

  child.stdout.on('data', (chunk) => {
    writeViteOutput(process.stdout, chunk)
  })
  child.stderr.on('data', (chunk) => {
    writeViteOutput(process.stderr, chunk)
  })
  child.on('exit', (code, signal) => {
    if (code !== 0 && code !== 143 && signal !== 'SIGTERM') {
      console.error(`[load:vite] exited with code=${code} signal=${signal}`)
    }
  })
  return child
}

function writeViteOutput(stream, chunk) {
  const text = String(chunk)
  if (text.includes('[permissions] colon-form action key detected')) return
  stream.write(`[load:vite] ${text}`)
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

async function runLoadAudit() {
  const browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
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
  }, Date.parse(process.env.LOAD_FIXED_NOW ?? '2026-06-24T12:00:00+08:00'))

  const reports = []
  const failures = []

  try {
    for (const entry of pages) {
      const page = await context.newPage()
      page.setDefaultTimeout(pageTimeoutMs)
      const startedAt = performance.now()
      try {
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
        await page.waitForFunction(
          ({ selector, minItems }) => document.querySelectorAll(selector).length >= minItems,
          { selector: entry.countSelector, minItems: entry.minItems },
          { timeout: pageTimeoutMs },
        )
        await page.waitForTimeout(200)
        const readyMs = Math.round(performance.now() - startedAt)
        const metrics = await page.evaluate(measurePage, entry.countSelector)
        const report = {
          name: entry.name,
          path: entry.path,
          readyMs,
          thresholds: {
            minItems: entry.minItems,
            maxReadyMs: entry.maxReadyMs,
            maxNodes: entry.maxNodes,
            maxHorizontalOverflowPx: 2,
          },
          ...metrics,
        }
        reports.push(report)
        collectFailures(entry, report, failures)
      } catch (error) {
        const metrics = await page.evaluate(measurePage, entry.countSelector).catch(() => null)
        reports.push({
          name: entry.name,
          path: entry.path,
          error: error instanceof Error ? error.message : String(error),
          ...(metrics ?? {}),
        })
        failures.push(`${entry.name}: ${error instanceof Error ? error.message : String(error)}`)
      } finally {
        await page.close()
      }
    }
  } finally {
    await browser.close()
  }

  const output = {
    generatedAt: new Date().toISOString(),
    baseUrl,
    pages: reports,
  }
  await mkdir(path.dirname(outputPath), { recursive: true })
  await writeFile(outputPath, `${JSON.stringify(output, null, 2)}\n`, 'utf8')

  console.log('Large surface audit')
  for (const report of reports) {
    console.log(
      `- ${report.name}: ${report.itemCount} items, ${report.readyMs}ms ready, ` +
        `${report.nodeCount} nodes, overflow ${report.horizontalOverflowPx}px`,
    )
  }

  if (failures.length > 0) {
    console.error('Large surface audit failed:')
    for (const failure of failures) {
      console.error(`- ${failure}`)
    }
    console.error(`Report written to ${path.relative(rootDir, outputPath)}`)
    process.exit(1)
  }

  console.log(`Large surface audit passed. Report written to ${path.relative(rootDir, outputPath)}`)
}

function measurePage(countSelector) {
  const root = document.documentElement
  const body = document.body
  const scrollWidth = Math.max(root.scrollWidth, body?.scrollWidth ?? 0)
  const clientWidth = root.clientWidth
  const memory = performance.memory
    ? {
        usedJSHeapSizeMB: Number((performance.memory.usedJSHeapSize / 1024 / 1024).toFixed(1)),
        totalJSHeapSizeMB: Number((performance.memory.totalJSHeapSize / 1024 / 1024).toFixed(1)),
      }
    : null

  return {
    itemCount: document.querySelectorAll(countSelector).length,
    nodeCount: document.querySelectorAll('*').length,
    horizontalOverflowPx: Math.max(0, Math.ceil(scrollWidth - clientWidth)),
    scrollHeight: root.scrollHeight,
    clientHeight: root.clientHeight,
    memory,
  }
}

function collectFailures(entry, report, failures) {
  if (report.itemCount < entry.minItems) {
    failures.push(`${entry.name}: only ${report.itemCount} items rendered, expected at least ${entry.minItems}`)
  }
  if (report.readyMs > entry.maxReadyMs) {
    failures.push(`${entry.name}: ready in ${report.readyMs}ms, limit ${entry.maxReadyMs}ms`)
  }
  if (report.nodeCount > entry.maxNodes) {
    failures.push(`${entry.name}: ${report.nodeCount} DOM nodes, limit ${entry.maxNodes}`)
  }
  if (report.horizontalOverflowPx > 2) {
    failures.push(`${entry.name}: horizontal overflow ${report.horizontalOverflowPx}px, limit 2px`)
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}
