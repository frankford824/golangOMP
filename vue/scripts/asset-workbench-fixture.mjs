import { spawn } from 'node:child_process'

export const fixedNow = Date.parse(process.env.ASSET_AUDIT_FIXED_NOW ?? '2026-06-24T12:00:00+08:00')

const adminCapabilities = [
  'asset.workbench.submit',
  'asset.workbench.manage',
  'asset.workbench.settlement',
  'asset.workbench.system_search',
  'asset.workbench.cost_center.manage',
  'asset.workbench.profile.manage',
  'asset.workbench.member.identity',
]

const adminProfile = {
  id: 1,
  user_id: 1001,
  worker_type: 'fulltime',
  job_grade: 'P2',
  real_name: '暗房主管',
  phone: '13800000000',
  province: '浙江',
  city: '杭州',
  id_card: '330100199001010000',
  alipay_account: 'darkroom@example.com',
  status: 'active',
  pii_completed: true,
}

const simpleProfile = {
  id: 2,
  user_id: 2001,
  worker_type: 'parttime',
  job_grade: 'P1',
  real_name: '交付同学',
  phone: '13900000000',
  province: '浙江',
  city: '宁波',
  id_card: '',
  alipay_account: 'worker@example.com',
  status: 'active',
  pii_completed: true,
}

export const submissions = [
  {
    id: 11,
    submission_no: 'AW-202606-001',
    submitter_user_id: 2001,
    business_month: '2026-06',
    submitted_at: '2026-06-24T10:00:00+08:00',
    status: 'submitted',
    item_count: 8,
    file_count: 12,
    page_count: 36,
    gross_total: 1280,
  },
  {
    id: 12,
    submission_no: 'AW-202606-002',
    submitter_user_id: 2002,
    business_month: '2026-06',
    submitted_at: '2026-06-24T11:00:00+08:00',
    status: 'checked',
    item_count: 6,
    file_count: 6,
    page_count: 18,
    gross_total: 720,
  },
]

const payrollRows = [
  {
    payee_user_id: 2001,
    business_month: '2026-06',
    row_type: 'normal_piecework',
    item_count: 14,
    page_count: 54,
    gross_amount: 2000,
    error_count: 1,
    deduction_amount: 20,
    welfare_amount: 80,
    supplement_amount: 0,
    adjustment_amount: 0,
    net_amount: 2060,
  },
  {
    payee_user_id: 2001,
    business_month: '2026-06',
    row_type: 'supplement_piecework',
    item_count: 2,
    page_count: 8,
    gross_amount: 0,
    error_count: 0,
    deduction_amount: 0,
    welfare_amount: 0,
    supplement_amount: 240,
    adjustment_amount: 0,
    net_amount: 240,
  },
]

const settlementPreview = {
  business_month: '2026-06',
  rows: [
    {
      payee_user_id: 2001,
      item_count: 16,
      page_count: 62,
      gross_amount: 2000,
      error_count: 1,
      deduction_amount: 20,
      welfare_amount: 80,
      supplement_amount: 240,
      net_amount: 2300,
    },
  ],
  totals: {
    payee_user_id: 0,
    item_count: 16,
    page_count: 62,
    gross_amount: 2000,
    error_count: 1,
    deduction_amount: 20,
    welfare_amount: 80,
    supplement_amount: 240,
    net_amount: 2300,
  },
  payroll_rows: payrollRows,
}

const previewImageUrl =
  'data:image/svg+xml,%3Csvg%20xmlns%3D%22http%3A//www.w3.org/2000/svg%22%20viewBox%3D%220%200%20320%20200%22%3E%3Crect%20width%3D%22320%22%20height%3D%22200%22%20fill%3D%22%23f4efe8%22/%3E%3Cpath%20d%3D%22M48%20148L124%2072l48%2044%2032-30%2068%2062z%22%20fill%3D%22%234b6f5d%22/%3E%3Ccircle%20cx%3D%22242%22%20cy%3D%2258%22%20r%3D%2224%22%20fill%3D%22%23d99a48%22/%3E%3C/svg%3E'

const systemAssets = [
  {
    id: 501,
    resource_id: 'res_darkroom_501',
    source_type: 'system',
    source_label: '系统资源',
    asset_no: 'MAT-501',
    scope_sku_code: 'CGP000071',
    file_name: 'poster-key-visual.png',
    original_filename: 'poster-key-visual.png',
    mime_type: 'image/png',
    product_name: '夏季海报主视觉',
    task_no: 'TASK-20260624',
    preview_available: true,
    download_url: previewImageUrl,
  },
  {
    id: 502,
    resource_id: 'res_darkroom_502',
    source_type: 'system',
    source_label: '系统资源',
    asset_no: 'MAT-502',
    scope_sku_code: 'CGK000602',
    file_name: 'lamp-render.psd',
    original_filename: 'lamp-render.psd',
    mime_type: 'application/vnd.adobe.photoshop',
    product_name: '小夜灯渲染',
    task_no: 'TASK-20260618',
    preview_available: false,
    download_url: 'https://example.invalid/lamp-render.psd',
  },
  {
    id: 900,
    resource_id: 'ext-900',
    source_type: 'external',
    source_label: '外部资源',
    file_name: 'external-reference.png',
    original_filename: 'external-reference.png',
    mime_type: 'image/png',
    product_name: '/p3/reference/external-reference.png',
    task_no: '',
    preview_available: true,
    preview_url: previewImageUrl,
    download_url: previewImageUrl,
    origin_path: '/p3/reference/external-reference.png',
  },
]

const clientMaterials = [
  {
    id: 1,
    asset_id: 501,
    source_type: 'system',
    source_ref: '501',
    resource_id: '501',
    source_label: '系统资源',
    title: '陈管常规海报 / 寿宴 / 红底黑字福如东海',
    description: '客户端可直接下载的已发布素材',
    filename_snapshot: 'poster-key-visual.png',
    mime_type_snapshot: 'image/png',
    file_size_snapshot: 245760,
    scope_sku_code: 'CGP000071',
    preview_available: true,
    enabled: true,
    sort_order: 1,
    published_by: 1001,
    published_at: '2026-06-24T10:00:00+08:00',
  },
  {
    id: 2,
    asset_id: 900,
    source_type: 'external',
    source_ref: 'ext-900',
    resource_id: 'ext-900',
    source_label: '外部资源',
    title: '外部参考素材',
    description: '从外部资源发布给客户端的素材',
    filename_snapshot: 'external-reference.png',
    mime_type_snapshot: 'image/png',
    file_size_snapshot: 188743,
    preview_available: true,
    enabled: true,
    sort_order: 2,
    published_by: 1001,
    published_at: '2026-06-24T10:20:00+08:00',
  },
  {
    id: 3,
    asset_id: 502,
    source_type: 'system',
    source_ref: '502',
    resource_id: '502',
    source_label: '系统资源',
    title: '菲瑶常规 KT 板 / 毕业典礼迎宾牌',
    description: 'PSD 源文件',
    filename_snapshot: 'lamp-render.psd',
    mime_type_snapshot: 'application/vnd.adobe.photoshop',
    file_size_snapshot: 10485760,
    scope_sku_code: 'CGK000602',
    preview_available: false,
    enabled: true,
    sort_order: 3,
    published_by: 1001,
    published_at: '2026-06-24T10:05:00+08:00',
  },
]

const uploadDirectories = [
  { id: 1, name: 'A类定稿', oss_prefix: 'a/final', description: 'A类定稿', enabled: true, sort_order: 1, created_by: 1001 },
  { id: 2, name: 'A类未定稿', oss_prefix: 'a/draft', description: 'A类未定稿', enabled: true, sort_order: 2, created_by: 1001 },
  { id: 3, name: 'B类定稿', oss_prefix: 'b/final', description: 'B类定稿', enabled: true, sort_order: 3, created_by: 1001 },
  { id: 4, name: 'B类未定稿', oss_prefix: 'b/draft', description: 'B类未定稿', enabled: true, sort_order: 4, created_by: 1001 },
  { id: 5, name: 'C类定稿', oss_prefix: 'c/final', description: '文字', enabled: true, sort_order: 5, created_by: 1001 },
  { id: 6, name: 'C类未定稿', oss_prefix: 'c/draft', description: '文字', enabled: true, sort_order: 6, created_by: 1001 },
]

function bootstrapFor(role) {
  const simple = role === 'simple'
  const profile = simple ? simpleProfile : adminProfile
  const capabilities = simple ? ['asset.workbench.submit', 'asset.workbench.material.download'] : adminCapabilities
  return {
    app: 'asset-workbench',
    version: 'fixture',
    timezone: 'Asia/Shanghai',
    oss_prefix: 'asset-workbench-fixture',
    upload_session_ttl_seconds: 3600,
    is_admin: !simple,
    access: {
      membership_status: 'active',
      is_enabled: true,
      is_admin_shell: !simple,
      asset_roles: simple ? ['AssetSubmitter'] : ['AssetAdmin'],
      role_labels: simple ? ['提交人'] : ['管理员'],
      capabilities,
    },
    role_labels: simple ? ['提交人'] : ['管理员'],
    user: {
      id: profile.user_id,
      username: simple ? 'asset-worker' : 'asset-admin',
      name: profile.real_name,
      account: simple ? 'asset-worker' : 'asset-admin',
    },
    profile,
    capabilities,
    settlement_item_types: ['normal_piecework', 'supplement_piecework'],
    deferred_business_items: [],
    architecture_guardrails: ['asset-workbench-fixture'],
  }
}

export const assetAuditPages = [
  { name: 'admin-dashboard', path: '/', ready: '.aw-dashboard-page', role: 'admin' },
  {
    name: 'admin-ledger-sheet',
    path: '/',
    ready: '.aw-dashboard-page',
    role: 'admin',
    prepare: async (page) => {
      await page.getByRole('button', { name: /本月预估净额|应结净额/ }).first().click()
      await page.waitForSelector('.aw-ledger-sheet', { state: 'visible' })
    },
  },
  { name: 'admin-submissions', path: '/submissions', ready: '.aw-data-surface', role: 'admin' },
  { name: 'admin-settlement', path: '/settlement', ready: '.aw-console-hero', role: 'admin' },
  { name: 'admin-materials', path: '/materials', ready: '.aw-material-browser', role: 'admin' },
  { name: 'simple-home', path: '/', ready: '.aw-simple-home', role: 'simple' },
  { name: 'simple-upload', path: '/upload', ready: '.aw-dropzone', role: 'simple' },
  { name: 'simple-materials', path: '/materials', ready: '.aw-material-client-list', role: 'simple' },
  { name: 'simple-income', path: '/my-settlement', ready: '.aw-simple-income', role: 'simple' },
]

export function startAssetVite(rootDir, port, label) {
  const viteBin = process.platform === 'win32' ? 'vite.cmd' : 'vite'
  const child = spawn(
    viteBin,
    ['--config', 'vite.asset.config.ts', '--host', '127.0.0.1', '--port', String(port), '--strictPort'],
    {
      cwd: rootDir,
      env: { ...process.env, VITE_USE_MOCK: 'false' },
      shell: process.platform === 'win32',
      stdio: ['ignore', 'pipe', 'pipe'],
    },
  )
  child.stdout.on('data', (chunk) => process.stdout.write(`[${label}:vite] ${chunk}`))
  child.stderr.on('data', (chunk) => process.stderr.write(`[${label}:vite] ${chunk}`))
  return child
}

export async function waitForServer(url) {
  const startedAt = Date.now()
  let lastError
  while (Date.now() - startedAt < 60_000) {
    try {
      const response = await fetch(url)
      if (response.ok) return
    } catch (error) {
      lastError = error
    }
    await sleep(500)
  }
  throw new Error(`Timed out waiting for ${url}: ${lastError?.message ?? 'no response'}`)
}

export async function installAssetWorkbenchFixture(context, role = 'admin') {
  const bootstrap = bootstrapFor(role)
  await context.addInitScript(
    ({ now }) => {
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
      window.localStorage.setItem('access_token', 'asset-fixture-token')
    },
    { now: fixedNow },
  )

  await context.route('**/v1/asset-workbench/**', async (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname
    if (path.endsWith('/entry')) return json(route, { state: 'ready', message: '', bootstrap })
    if (path.endsWith('/bootstrap')) return json(route, bootstrap)
    if (path.endsWith('/submissions')) return paginated(route, submissions)
    if (path.endsWith('/settlement/preview')) return json(route, settlementPreview)
    if (path.endsWith('/settlement/my')) {
      return json(route, {
        current_month: '2026-06',
        estimated_net_amount: 2300,
        months: [
          {
            business_month: '2026-06',
            item_count: 16,
            page_count: 62,
            gross_amount: 2000,
            deduction_amount: 20,
            welfare_amount: 80,
            supplement_amount: 240,
            adjustment_amount: 0,
            net_amount: 2300,
            confirmed: false,
          },
        ],
      })
    }
    if (path.endsWith('/settlement/batches')) {
      return paginated(route, [
        {
          id: 301,
          batch_no: 'SET-202606-001',
          business_month: '2026-06',
          status: 'generated',
          item_count: 16,
          gross_amount: 2000,
          deduction_amount: 20,
          welfare_amount: 80,
          supplement_amount: 240,
          adjustment_amount: 0,
          net_amount: 2300,
        },
      ])
    }
    if (path.endsWith('/settlement/supplements')) return paginated(route, [])
    if (path.endsWith('/settlement/supplement-permissions')) return paginated(route, [])
    if (path.endsWith('/system-search')) return json(route, { items: systemAssets, total: systemAssets.length, page: 1, size: systemAssets.length })
    if (path.endsWith('/upload-directories') || path.endsWith('/upload-directories/admin')) return json(route, uploadDirectories)
    if (path.endsWith('/client-materials')) return json(route, clientMaterials)
    if (path.endsWith('/client-materials/1/preview')) {
      return json(route, {
        asset_id: 501,
        status: 'ready',
        preparing: false,
        preview_url: previewImageUrl,
        download_url: previewImageUrl,
        mime_type: 'image/png',
        filename: 'poster-key-visual.png',
        preview_available: true,
      })
    }
    if (path.endsWith('/client-materials/2/preview')) {
      return json(route, {
        asset_id: 900,
        source_type: 'external',
        source_ref: 'ext-900',
        status: 'ready',
        preparing: false,
        preview_url: previewImageUrl,
        download_url: previewImageUrl,
        mime_type: 'image/png',
        filename: 'external-reference.png',
        preview_available: true,
      })
    }
    if (path.endsWith('/client-materials/3/preview')) {
      return json(route, {
        asset_id: 502,
        source_type: 'system',
        source_ref: '502',
        status: 'not_applicable',
        preparing: false,
        mime_type: 'application/vnd.adobe.photoshop',
        filename: 'lamp-render.psd',
        preview_available: false,
      })
    }
    if (path.endsWith('/saved-views')) return json(route, [])
    return json(route, {})
  })

  await context.route('**/v1/assets/**', async (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname
    if (path.endsWith('/ext-900/preview') || path.endsWith('/ext-900/download')) {
      return json(route, {
        download_mode: 'direct',
        download_url: previewImageUrl,
        access_hint: 'external_fixture',
        preview_available: path.endsWith('/preview'),
        filename: 'external-reference.png',
        file_size: 188743,
        mime_type: 'image/png',
      })
    }
    return json(route, {})
  })
}

export function runDomAudit() {
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
    if (count > 1) issues.push(`duplicate id "${id}" appears ${count} times`)
  }

  for (const element of document.querySelectorAll(interactiveSelector)) {
    if (!isVisible(element) || isHiddenFromAT(element)) continue
    if (!accessibleName(element)) issues.push(`interactive element has no accessible name: ${elementPath(element)}`)
  }

  for (const image of document.querySelectorAll('img')) {
    if (!isVisible(image) || isHiddenFromAT(image)) continue
    if (image.getAttribute('alt') === null) issues.push(`image is missing alt: ${elementPath(image)}`)
  }

  for (const element of document.querySelectorAll(idRefs.map((attr) => `[${attr}]`).join(','))) {
    if (!isVisible(element) || isHiddenFromAT(element)) continue
    for (const attr of idRefs) {
      const raw = element.getAttribute(attr)
      if (!raw) continue
      for (const id of raw.split(/\s+/).filter(Boolean)) {
        if (!document.getElementById(id)) issues.push(`${attr} references missing id "${id}": ${elementPath(element)}`)
      }
    }
  }

  return { issues, interactiveCount: [...document.querySelectorAll(interactiveSelector)].filter((item) => isVisible(item) && !isHiddenFromAT(item)).length }

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
    return closestLabel?.textContent?.trim() || element.textContent?.trim().replace(/\s+/g, ' ') || ''
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
      if (id) segment += `#${id}`
      else {
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

export function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function json(route, data) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ data }),
  })
}

async function paginated(route, items) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ data: items, pagination: { total: items.length, page: 1, page_size: items.length } }),
  })
}
