import { spawn } from 'node:child_process'
import { chromium } from 'playwright'

const rootDir = new URL('..', import.meta.url)
const port = Number(process.env.SURFACE_PORT ?? 4177)
const baseUrl = process.env.SURFACE_BASE_URL ?? `http://127.0.0.1:${port}`
const maxReadyMs = Number(process.env.SURFACE_MAX_READY_MS ?? 5_000)
const vite = process.platform === 'win32' ? 'vite.cmd' : 'vite'

const surfaces = [
  {
    name: 'normal-task',
    path: '/tasks/1001',
    expectedBadges: ['原品开发', '常规任务', '单 SKU'],
  },
  {
    name: 'customization-task',
    path: '/tasks/1002',
    expectedBadges: ['常规定制 · 定制', '单 SKU'],
  },
]

const server = process.env.SURFACE_BASE_URL ? null : startVite()

try {
  if (server) await waitForServer(server)
  await auditSurfaces()
} finally {
  if (server) await stopVite(server)
}

function startVite() {
  const child = spawn(vite, ['--host', '127.0.0.1', '--port', String(port), '--strictPort', '--mode', 'test'], {
    cwd: rootDir,
    env: { ...process.env, VITE_USE_MOCK: 'true' },
    shell: process.platform === 'win32',
    detached: process.platform !== 'win32',
    stdio: 'ignore',
  })
  return child
}

async function waitForServer(child) {
  const deadline = Date.now() + 30_000
  let lastError
  while (Date.now() < deadline) {
    if (child.exitCode !== null) throw new Error(`Vite exited before readiness (code ${child.exitCode})`)
    try {
      if ((await fetch(baseUrl)).ok) return
    } catch (error) {
      lastError = error
    }
    await new Promise((resolve) => setTimeout(resolve, 200))
  }
  throw new Error(`Timed out waiting for ${baseUrl}: ${lastError?.message ?? 'no response'}`)
}

async function stopVite(child) {
  if (child.exitCode !== null) return
  if (process.platform === 'win32') child.kill('SIGTERM')
  else process.kill(-child.pid, 'SIGTERM')
  await Promise.race([
    new Promise((resolve) => child.once('exit', resolve)),
    new Promise((resolve) => setTimeout(resolve, 5_000)),
  ])
}

async function auditSurfaces() {
  const browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN', timezoneId: 'Asia/Shanghai' })
  await context.addInitScript(() => localStorage.setItem('access_token', 'mock-token-ops-demo'))

  try {
    for (const surface of surfaces) {
      const page = await context.newPage()
      const startedAt = performance.now()
      try {
        await page.goto(new URL(surface.path, baseUrl).toString(), { waitUntil: 'domcontentloaded' })
        await page.locator('.task-detail-view .identity-badges').waitFor({ state: 'visible' })
        const readyMs = Math.round(performance.now() - startedAt)
        const badges = await page.locator('.task-detail-view .identity-badge').allTextContents()
        const normalizedBadges = badges.map((badge) => badge.replace(/\s+/g, ' ').trim())
        if (readyMs > maxReadyMs) throw new Error(`${surface.name}: ready in ${readyMs}ms, limit ${maxReadyMs}ms`)
        if (JSON.stringify(normalizedBadges) !== JSON.stringify(surface.expectedBadges)) {
          throw new Error(`${surface.name}: identity/kind badges ${JSON.stringify(normalizedBadges)} do not match ${JSON.stringify(surface.expectedBadges)}`)
        }
        console.log(`- ${surface.name}: ${readyMs}ms ready; ${normalizedBadges.join(' | ')}`)
      } finally {
        await page.close()
      }
    }
  } finally {
    await browser.close()
  }
  console.log('Task detail surface regression passed.')
}
