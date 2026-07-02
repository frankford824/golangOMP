import { spawn } from 'node:child_process'
import { mkdir, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const port = Number(process.env.A11Y_PORT ?? 4175)
const baseUrl = process.env.A11Y_BASE_URL ?? `http://127.0.0.1:${port}`
const pageTimeoutMs = Number(process.env.A11Y_PAGE_TIMEOUT_MS ?? 60_000)
const outputPath = path.join(rootDir, 'tests', 'a11y', 'latest.json')

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
      if ((await page.locator('.pm-detail-help').count()) === 0) {
        await page.locator('.pm-combo-header').first().click()
      }
      await page.waitForSelector('.pm-detail-help', { state: 'visible' })
      await page.locator('.pm-detail-help').first().focus()
      await page.waitForTimeout(200)
    },
  },
]

const startedServer = process.env.A11Y_BASE_URL ? null : startVite()

try {
  await waitForServer(baseUrl)
  await runAccessibilityAudit()
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
    process.stdout.write(`[a11y:vite] ${chunk}`)
  })
  child.stderr.on('data', (chunk) => {
    process.stderr.write(`[a11y:vite] ${chunk}`)
  })
  child.on('exit', (code, signal) => {
    if (code !== 0 && code !== 143 && signal !== 'SIGTERM') {
      console.error(`[a11y:vite] exited with code=${code} signal=${signal}`)
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

async function runAccessibilityAudit() {
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
  }, Date.parse(process.env.A11Y_FIXED_NOW ?? '2026-06-24T12:00:00+08:00'))

  const pageReports = []
  const failures = []

  try {
    for (const entry of pages) {
      const page = await context.newPage()
      page.setDefaultTimeout(pageTimeoutMs)
      await page.goto(new URL(entry.path, baseUrl).toString(), {
        waitUntil: 'domcontentloaded',
        timeout: pageTimeoutMs,
      })
      await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})
      await page.waitForSelector(entry.ready, { state: 'visible' })
      await page.waitForTimeout(300)
      if (entry.prepare) {
        await entry.prepare(page)
      }

      const report = await page.evaluate(runDomAudit)
      report.name = entry.name
      report.path = entry.path
      pageReports.push(report)
      for (const issue of report.issues) {
        failures.push(`${entry.name}: ${issue}`)
      }
      await page.close()
    }
  } finally {
    await browser.close()
  }

  const output = {
    generatedAt: new Date().toISOString(),
    baseUrl,
    pages: pageReports,
  }
  await mkdir(path.dirname(outputPath), { recursive: true })
  await writeFile(outputPath, `${JSON.stringify(output, null, 2)}\n`, 'utf8')

  console.log('Accessibility audit')
  for (const report of pageReports) {
    console.log(`- ${report.name}: ${report.issues.length} issues`)
  }

  if (failures.length > 0) {
    console.error('Accessibility audit failed:')
    for (const failure of failures) {
      console.error(`- ${failure}`)
    }
    console.error(`Report written to ${path.relative(rootDir, outputPath)}`)
    process.exit(1)
  }

  console.log(`Accessibility audit passed. Report written to ${path.relative(rootDir, outputPath)}`)
}

function runDomAudit() {
  const issues = []
  const idCounts = new Map()
  const idRefs = ['aria-labelledby', 'aria-describedby', 'aria-controls', 'for']
  const interactiveSelector = [
    'button',
    'a[href]',
    'input:not([type="hidden"])',
    'select',
    'textarea',
    '[role="button"]',
    '[role="link"]',
    '[role="checkbox"]',
    '[role="switch"]',
    '[role="tab"]',
    '[tabindex]:not([tabindex="-1"])',
  ].join(',')

  for (const element of document.querySelectorAll('[id]')) {
    const id = element.getAttribute('id')
    if (!id) continue
    idCounts.set(id, (idCounts.get(id) ?? 0) + 1)
  }
  for (const [id, count] of idCounts.entries()) {
    if (count > 1) {
      issues.push(`duplicate id "${id}" appears ${count} times`)
    }
  }

  for (const element of document.querySelectorAll(interactiveSelector)) {
    if (!isVisible(element) || isHiddenFromAT(element)) continue
    const name = accessibleName(element)
    if (!name) {
      issues.push(`interactive element has no accessible name: ${elementPath(element)}`)
    }
  }

  for (const input of document.querySelectorAll('input:not([type="hidden"]), select, textarea')) {
    if (!isVisible(input) || isHiddenFromAT(input)) continue
    if (!accessibleName(input)) {
      issues.push(`form control has no label: ${elementPath(input)}`)
    }
  }

  for (const image of document.querySelectorAll('img')) {
    if (!isVisible(image) || isHiddenFromAT(image)) continue
    const alt = image.getAttribute('alt')
    if (alt === null) {
      issues.push(`image is missing alt: ${elementPath(image)}`)
    }
  }

  for (const element of document.querySelectorAll(idRefs.map((attr) => `[${attr}]`).join(','))) {
    if (!isVisible(element) || isHiddenFromAT(element)) continue
    for (const attr of idRefs) {
      const raw = element.getAttribute(attr)
      if (!raw) continue
      for (const id of raw.split(/\s+/).filter(Boolean)) {
        if (!document.getElementById(id)) {
          issues.push(`${attr} references missing id "${id}": ${elementPath(element)}`)
        }
      }
    }
  }

  return {
    url: location.pathname,
    issues,
    counts: {
      interactive: [...document.querySelectorAll(interactiveSelector)].filter((element) => isVisible(element) && !isHiddenFromAT(element)).length,
      images: [...document.querySelectorAll('img')].filter((element) => isVisible(element) && !isHiddenFromAT(element)).length,
    },
  }

  function accessibleName(element) {
    const labelledBy = element.getAttribute('aria-labelledby')
    if (labelledBy) {
      const value = labelledBy
        .split(/\s+/)
        .map((id) => document.getElementById(id)?.textContent?.trim() ?? '')
        .filter(Boolean)
        .join(' ')
        .trim()
      if (value) return value
    }

    for (const attr of ['aria-label', 'alt', 'title', 'placeholder']) {
      const value = element.getAttribute(attr)?.trim()
      if (value) return value
    }

    const id = element.getAttribute('id')
    if (id) {
      const label = document.querySelector(`label[for="${cssEscape(id)}"]`)
      const text = label?.textContent?.trim()
      if (text) return text
    }

    const closestLabel = element.closest('label')
    const labelText = closestLabel?.textContent?.trim()
    if (labelText) return labelText

    return element.textContent?.trim().replace(/\s+/g, ' ') ?? ''
  }

  function isVisible(element) {
    if (element.hidden) return false
    const style = window.getComputedStyle(element)
    if (style.display === 'none' || style.visibility === 'hidden') return false
    const rect = element.getBoundingClientRect()
    return rect.width > 0 && rect.height > 0
  }

  function isHiddenFromAT(element) {
    let current = element
    while (current && current.nodeType === Node.ELEMENT_NODE) {
      if (current.getAttribute('aria-hidden') === 'true') return true
      current = current.parentElement
    }
    return false
  }

  function elementPath(element) {
    const segments = []
    let current = element
    while (current && current.nodeType === Node.ELEMENT_NODE && segments.length < 4) {
      let segment = current.tagName.toLowerCase()
      const id = current.getAttribute('id')
      if (id) {
        segment += `#${id}`
      } else {
        const className = [...current.classList].slice(0, 3).join('.')
        if (className) segment += `.${className}`
      }
      segments.unshift(segment)
      current = current.parentElement
    }
    return segments.join(' > ')
  }

  function cssEscape(value) {
    if (window.CSS?.escape) return window.CSS.escape(value)
    return value.replace(/["\\]/g, '\\$&')
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}
