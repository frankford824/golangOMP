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
    countSelector: '.resource-card',
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
  {
    name: 'planning-sku-200',
    path: '/tasks/create?intent=planning_sku',
    ready: '.compose-page[data-compose-intent="planning_sku"]',
    countSelector: '.compose-grid__canvas-shell',
    minItems: 1,
    maxItems: 60,
    maxNodes: 25_000,
    maxReadyMs: defaultMaxReadyMs,
    viewport: { width: 1366, height: 768 },
    scenario: 'planning-sku-200',
  },
]

const startedServer = process.env.LOAD_BASE_URL ? null : startVite()

try {
  if (startedServer) {
    await waitForOwnedServer(baseUrl, startedServer)
    await Promise.race([
      runLoadAudit(),
      rejectOnUnexpectedViteExit(startedServer, 'during load audit'),
    ])
  } else {
    await waitForServer(baseUrl)
    await runLoadAudit()
  }
} finally {
  if (startedServer) {
    await stopVite(startedServer)
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
      detached: process.platform !== 'win32',
      stdio: ['ignore', 'pipe', 'pipe'],
    },
  )

  let stopping = false
  let stdout = ''
  let markReady
  const readyPromise = new Promise((resolve) => {
    markReady = resolve
  })
  const exitPromise = new Promise((resolve) => {
    child.once('error', (error) => resolve({ code: null, signal: null, error }))
    child.once('exit', (code, signal) => resolve({ code, signal, error: null }))
  })

  child.stdout.on('data', (chunk) => {
    writeViteOutput(process.stdout, chunk)
    stdout = `${stdout}${String(chunk)}`.slice(-8_192)
    if (/Local:\s+http:\/\/127\.0\.0\.1:\d+\//.test(stripAnsi(stdout))) {
      markReady()
    }
  })
  child.stderr.on('data', (chunk) => {
    writeViteOutput(process.stderr, chunk)
  })
  return {
    child,
    readyPromise,
    exitPromise,
    get stopping() {
      return stopping
    },
    beginStopping() {
      stopping = true
    },
  }
}

function stripAnsi(value) {
  return value.replace(/\u001b\[[0-9;]*m/g, '')
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

async function waitForOwnedServer(url, server) {
  await Promise.race([
    server.readyPromise,
    rejectOnUnexpectedViteExit(server, 'before ready'),
  ])
  await Promise.race([
    waitForServer(url),
    rejectOnUnexpectedViteExit(server, 'before HTTP readiness'),
  ])
  if (server.child.exitCode !== null || server.child.signalCode !== null) {
    throw new Error(
      `[load:vite] exited before readiness with code=${server.child.exitCode} signal=${server.child.signalCode}`,
    )
  }
}

async function rejectOnUnexpectedViteExit(server, phase) {
  const { code, signal, error } = await server.exitPromise
  if (server.stopping) {
    return await new Promise(() => {})
  }
  throw new Error(
    `[load:vite] exited ${phase} with code=${code} signal=${signal}` +
      (error ? ` error=${error.message}` : ''),
  )
}

async function stopVite(server) {
  server.beginStopping()
  if (server.child.exitCode !== null || server.child.signalCode !== null) {
    await server.exitPromise
    return
  }

  signalVite(server.child, 'SIGTERM')
  const exited = await Promise.race([
    server.exitPromise.then(() => true),
    sleep(5_000).then(() => false),
  ])
  if (exited) return

  signalVite(server.child, 'SIGKILL')
  const killed = await Promise.race([
    server.exitPromise.then(() => true),
    sleep(5_000).then(() => false),
  ])
  if (!killed) {
    throw new Error(`[load:vite] failed to terminate child pid=${server.child.pid}`)
  }
}

function signalVite(child, signal) {
  if (!child.pid) return
  try {
    if (process.platform === 'win32') {
      child.kill(signal)
    } else {
      process.kill(-child.pid, signal)
    }
  } catch (error) {
    if (error?.code !== 'ESRCH') throw error
  }
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
      const diagnostics = collectPageDiagnostics(page)
      await page.setViewportSize(entry.viewport ?? { width: 1440, height: 900 })
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
        const scenarioMetrics = entry.scenario === 'planning-sku-200'
          ? await preparePlanningSkuScenario(page)
          : {}
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
            maxItems: entry.maxItems ?? null,
            maxReadyMs: entry.maxReadyMs,
            maxNodes: entry.maxNodes,
            maxHorizontalOverflowPx: 2,
          },
          viewport: entry.viewport ?? { width: 1440, height: 900 },
          finalUrl: page.url(),
          diagnostics: diagnostics.snapshot(),
          ...metrics,
          ...scenarioMetrics,
        }
        reports.push(report)
        collectFailures(entry, report, failures)
      } catch (error) {
        const metrics = await page.evaluate(measurePage, entry.countSelector).catch(() => null)
        const finalUrl = page.url()
        const title = await page.title().catch(() => '')
        const bodyText = await page.locator('body').innerText({ timeout: 1_000 }).catch(() => '')
        const message = error instanceof Error ? error.message : String(error)
        reports.push({
          name: entry.name,
          path: entry.path,
          readyMs: Math.round(performance.now() - startedAt),
          error: message,
          finalUrl,
          title,
          bodyTextSnippet: bodyText.trim().slice(0, 1_000),
          diagnostics: diagnostics.snapshot(),
          ...(metrics ?? {}),
        })
        failures.push(`${entry.name} at ${finalUrl}: ${message}`)
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
    if (report.error) {
      console.error(
        `- ${report.name}: FAILED at ${report.finalUrl}; ${report.error}; ` +
          `${report.nodeCount ?? 'unknown'} nodes`,
      )
      printPageDiagnostics(report.diagnostics)
      continue
    }
    console.log(
      `- ${report.name}: ${report.itemCount} items, ${report.readyMs}ms ready, ` +
        `${report.nodeCount} nodes, overflow ${report.horizontalOverflowPx}px` +
        (report.name === 'planning-sku-200'
          ? `, order=${report.orderPreserved}, values=${report.firstValuePreserved && report.lastValuePreserved}, footer=${report.footerVisible && !report.footerOverlapsLast}`
          : ''),
    )
  }

  if (failures.length > 0) {
    console.error('Large surface audit failed:')
    for (const failure of failures) {
      console.error(`- ${failure}`)
    }
    console.error(`Report written to ${path.relative(rootDir, outputPath)}`)
    throw new Error(`Large surface audit failed with ${failures.length} failing surface(s)`)
  }

  console.log(`Large surface audit passed. Report written to ${path.relative(rootDir, outputPath)}`)
}

function collectPageDiagnostics(page) {
  const consoleMessages = []
  const pageErrors = []
  const failedRequests = []
  const httpErrors = []
  const append = (target, value) => {
    if (target.length < 50) target.push(value)
  }

  page.on('console', (message) => {
    if (!['warning', 'error'].includes(message.type())) return
    append(consoleMessages, {
      type: message.type(),
      text: message.text(),
      location: message.location(),
    })
  })
  page.on('pageerror', (error) => {
    append(pageErrors, error.stack ?? error.message)
  })
  page.on('requestfailed', (request) => {
    append(failedRequests, {
      method: request.method(),
      url: request.url(),
      errorText: request.failure()?.errorText ?? 'unknown request failure',
    })
  })
  page.on('response', (response) => {
    if (response.status() < 400) return
    append(httpErrors, {
      status: response.status(),
      method: response.request().method(),
      url: response.url(),
    })
  })

  return {
    snapshot() {
      return {
        consoleMessages: [...consoleMessages],
        pageErrors: [...pageErrors],
        failedRequests: [...failedRequests],
        httpErrors: [...httpErrors],
      }
    },
  }
}

function printPageDiagnostics(diagnostics) {
  for (const message of diagnostics?.consoleMessages ?? []) {
    console.error(`  console.${message.type}: ${message.text}`)
  }
  for (const error of diagnostics?.pageErrors ?? []) {
    console.error(`  pageerror: ${error}`)
  }
  for (const request of diagnostics?.failedRequests ?? []) {
    console.error(`  requestfailed: ${request.method} ${request.url} (${request.errorText})`)
  }
  for (const response of diagnostics?.httpErrors ?? []) {
    console.error(`  http ${response.status}: ${response.method} ${response.url}`)
  }
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

async function preparePlanningSkuScenario(page) {
  const addRow = page.getByRole('button', { name: '添加一行' })
  await addRow.waitFor({ state: 'visible' })
  await page.evaluate(() => {
    const button = [...document.querySelectorAll('button')].find((item) => item.textContent?.trim() === '添加一行')
    if (!(button instanceof HTMLButtonElement)) throw new Error('planning SKU add-row control is missing')
    for (let index = 1; index < 200; index += 1) button.click()
  })
  await page.waitForFunction(() => document.querySelector('.compose-page')?.getAttribute('data-row-count') === '200')

  await page.evaluate(() => {
    const setValue = (index, value) => {
      const control = document.querySelector(`[data-testid="compose-row"][data-row-index="${index}"] textarea`)
      if (!(control instanceof HTMLTextAreaElement)) throw new Error(`planning row ${index} is missing`)
      control.value = value
      control.dispatchEvent(new Event('input', { bubbles: true }))
    }
    setValue(0, '首行保持值 001')
    setValue(199, '末行保持值 200')
  })
  const rowOrder = await page.locator('[data-testid="compose-row"]').evaluateAll((rows) => rows.map((row) => Number(row.getAttribute('data-row-index'))))
  const firstValuePreserved = await page.locator('[data-row-index="0"] textarea').first().inputValue() === '首行保持值 001'
  const lastValuePreserved = await page.locator('[data-row-index="199"] textarea').first().inputValue() === '末行保持值 200'
  const footer = page.locator('.validation-dock')
  await footer.scrollIntoViewIfNeeded()
  const layout = await page.evaluate(() => {
    const footerElement = document.querySelector('.validation-dock')
    const gridElement = document.querySelector('.compose-grid__canvas-shell')
    if (!(footerElement instanceof HTMLElement) || !(gridElement instanceof HTMLElement)) {
      throw new Error('planning SKU validation dock or grid is missing')
    }
    const footerRect = footerElement.getBoundingClientRect()
    const gridRect = gridElement.getBoundingClientRect()
    return {
      footerVisible: footerRect.top >= 0 && footerRect.bottom <= window.innerHeight,
      footerOverlapsLast: footerRect.top < gridRect.bottom && footerRect.bottom > gridRect.top,
    }
  })

  return {
    rowCount: 200,
    visibleRowCount: await page.locator('.compose-grid__canvas-shell:visible').count(),
    firstValuePreserved,
    lastValuePreserved,
    orderPreserved: rowOrder.length === 200
      && rowOrder.at(-1) === 199
      && rowOrder.every((value, index) => value === index),
    ...layout,
  }
}

function collectFailures(entry, report, failures) {
  if (report.itemCount < entry.minItems) {
    failures.push(`${entry.name}: only ${report.itemCount} items rendered, expected at least ${entry.minItems}`)
  }
  if (entry.maxItems && report.itemCount > entry.maxItems) {
    failures.push(`${entry.name}: ${report.itemCount} visible items, limit ${entry.maxItems}`)
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
  if (entry.scenario === 'planning-sku-200') {
    if (report.rowCount !== 200) failures.push(`${entry.name}: constructed ${report.rowCount} rows, expected 200`)
    if (!report.firstValuePreserved || !report.lastValuePreserved) failures.push(`${entry.name}: first/last values changed after virtual scrolling`)
    if (!report.orderPreserved) failures.push(`${entry.name}: visible row order changed after virtual scrolling`)
    if (!report.footerVisible) failures.push(`${entry.name}: submit footer is not fully visible at 1366x768`)
    if (report.footerOverlapsLast) failures.push(`${entry.name}: submit footer overlaps the last editable row`)
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}
