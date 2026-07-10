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

const difficultyClasses = [
  { id: 1, code: 'A', name: 'A类', description: '标准成品', enabled: true, sort_order: 10, created_by: 1001 },
  { id: 2, code: 'B', name: 'B类', description: '复杂成品', enabled: true, sort_order: 20, created_by: 1001 },
  { id: 3, code: 'C', name: 'C类', description: '文字类', enabled: true, sort_order: 30, created_by: 1001 },
]

const deductionRules = [
  { id: 21, worker_type: 'parttime', job_grade: 'P1', difficulty_class: 'A', deduction_amount: 2, effective_from: '2026-01-01', enabled: true },
  { id: 22, worker_type: 'parttime', job_grade: 'P1', difficulty_class: 'B', deduction_amount: 4, effective_from: '2026-01-01', enabled: true },
  { id: 23, worker_type: 'fulltime', job_grade: 'P2', difficulty_class: 'C', deduction_amount: 6, effective_from: '2026-01-01', enabled: true },
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
  { id: 1, name: 'A类定稿', oss_prefix: 'a/final', description: 'A类定稿', difficulty_class: 'A', allowed_file_types: ['png', 'jpg', 'jpeg'], enabled: true, sort_order: 1, created_by: 1001 },
  { id: 2, name: 'A类未定稿', oss_prefix: 'a/draft', description: 'A类未定稿', difficulty_class: 'A', allowed_file_types: [], enabled: true, sort_order: 2, created_by: 1001 },
  { id: 3, name: 'B类定稿', oss_prefix: 'b/final', description: 'B类定稿', difficulty_class: 'B', allowed_file_types: ['image/*'], enabled: true, sort_order: 3, created_by: 1001 },
  { id: 4, name: 'B类未定稿', oss_prefix: 'b/draft', description: 'B类未定稿', difficulty_class: 'B', allowed_file_types: [], enabled: true, sort_order: 4, created_by: 1001 },
  { id: 5, name: 'C类定稿', oss_prefix: 'c/final', description: '文字', difficulty_class: 'C', allowed_file_types: ['pdf'], enabled: true, sort_order: 5, created_by: 1001 },
  { id: 6, name: 'C类未定稿', oss_prefix: 'c/draft', description: '文字', difficulty_class: 'C', allowed_file_types: [], enabled: true, sort_order: 6, created_by: 1001 },
]

const driveFiles = [
  {
    id: 801,
    submission_id: 11,
    submission_item_id: 1101,
    submission_no: 'AW-202606-001',
    owner_user_id: 2001,
    upload_directory_id: 1,
    upload_directory_name: 'A类定稿',
    difficulty_class: 'A',
    order_no: 'CGP000071-001',
    original_filename: 'poster-final-a.png',
    file_type: 'png',
    mime_type: 'image/png',
    file_size: 245760,
    preview_status: 'ready',
    qc_status: 'checked',
    pricing_status: 'priced',
    settlement_status: 'unsettled',
    page_count: 1,
    business_month: '2026-06',
    created_at: '2026-06-24T10:10:00+08:00',
  },
  {
    id: 802,
    submission_id: 11,
    submission_item_id: 1101,
    submission_no: 'AW-202606-001',
    owner_user_id: 2001,
    upload_directory_id: 1,
    upload_directory_name: 'A类定稿',
    difficulty_class: 'A',
    order_no: 'CGP000071-001',
    original_filename: 'poster-final-b.jpg',
    file_type: 'jpg',
    mime_type: 'image/jpeg',
    file_size: 195072,
    preview_status: 'ready',
    qc_status: 'pending',
    pricing_status: 'priced',
    settlement_status: 'unsettled',
    page_count: 1,
    business_month: '2026-06',
    created_at: '2026-06-24T10:12:00+08:00',
  },
  {
    id: 803,
    submission_id: 12,
    submission_item_id: 1201,
    submission_no: 'AW-202606-002',
    owner_user_id: 2002,
    upload_directory_id: 3,
    upload_directory_name: 'B类定稿',
    difficulty_class: 'B',
    order_no: 'CGK000602-001',
    original_filename: 'lamp-render-preview.png',
    file_type: 'png',
    mime_type: 'image/png',
    file_size: 188743,
    preview_status: 'ready',
    qc_status: 'needs_fix',
    pricing_status: 'pending_grade',
    settlement_status: 'unsettled',
    page_count: 2,
    business_month: '2026-06',
    created_at: '2026-06-24T11:15:00+08:00',
  },
]

function driveDirectories() {
  return uploadDirectories.map((directory) => {
    const files = driveFiles.filter((file) => file.upload_directory_id === directory.id)
    return {
      directory_id: directory.id,
      name: directory.name,
      prefix: directory.oss_prefix,
      difficulty_class: directory.difficulty_class,
      file_count: files.length,
      order_count: new Set(files.map((file) => file.order_no)).size,
    }
  })
}

function driveOrders(url) {
  const dirID = Number(url.searchParams.get('dir_id') || 0)
  const rows = driveFiles.filter((file) => !dirID || file.upload_directory_id === dirID)
  const grouped = new Map()
  for (const file of rows) {
    const existing = grouped.get(file.order_no) ?? {
      order_no: file.order_no,
      submission_item_id: file.submission_item_id,
      submission_item_ids: [],
      file_count: 0,
      latest_at: file.created_at,
    }
    existing.file_count += 1
    if (!existing.submission_item_ids.includes(file.submission_item_id)) existing.submission_item_ids.push(file.submission_item_id)
    if (file.created_at > existing.latest_at) existing.latest_at = file.created_at
    grouped.set(file.order_no, existing)
  }
  return [...grouped.values()]
}

function driveFilesFor(url) {
  const dirID = Number(url.searchParams.get('dir_id') || 0)
  const orderNo = url.searchParams.get('order_no') || ''
  return driveFiles.filter((file) => (!dirID || file.upload_directory_id === dirID) && (!orderNo || file.order_no === orderNo))
}

function driveSearchRows(url) {
  const q = (url.searchParams.get('q') || '').toLowerCase()
  return driveFiles.filter((file) =>
    [file.original_filename, file.order_no, file.submission_no, file.upload_directory_name].join(' ').toLowerCase().includes(q),
  )
}

function overviewRows(url) {
  const q = (url.searchParams.get('q') || '').toLowerCase()
  const scope = url.searchParams.get('scope') || 'all'
  const rows = []
  if (scope === 'all' || scope === 'operational') {
    for (const material of clientMaterials) {
      if (q && ![material.title, material.filename_snapshot, material.resource_id].join(' ').toLowerCase().includes(q)) continue
      rows.push({
        source: 'client_material',
        scope: 'operational',
        source_label: material.source_label,
        id: material.id,
        title: material.title,
        primary_code: material.resource_id,
        created_at: material.published_at,
        route_path: `/drive?scope=operational&q=${encodeURIComponent(material.title)}`,
        locate: {
          source: 'client_material',
          material_id: material.id,
          source_type: material.source_type,
          source_ref: material.source_ref,
          resource_id: material.resource_id,
        },
        meta_json: { filename: material.filename_snapshot },
      })
    }
  }
  if (scope === 'all' || scope === 'files') {
    for (const file of driveFiles) {
      if (q && ![file.original_filename, file.order_no, file.submission_no].join(' ').toLowerCase().includes(q)) continue
      rows.push({
        source: 'submission_file',
        scope: 'files',
        source_label: '交稿文件',
        id: file.id,
        title: file.original_filename,
        primary_code: file.submission_no,
        order_no: file.order_no,
        status: file.qc_status,
        page_count: file.page_count,
        created_at: file.created_at,
        route_path: `/drive?file_id=${file.id}`,
        locate: { source: 'submission_file', file_id: file.id, submission_id: file.submission_id, item_id: file.submission_item_id },
        meta_json: { upload_directory_name: file.upload_directory_name },
      })
    }
  }
  if (scope === 'all' || scope === 'orders') {
    for (const file of driveFiles) {
      if (q && ![file.order_no, file.submission_no].join(' ').toLowerCase().includes(q)) continue
      rows.push({
        source: 'piecework_item',
        scope: 'orders',
        source_label: '订单·计件',
        id: file.submission_item_id,
        title: file.order_no,
        primary_code: file.submission_no,
        order_no: file.order_no,
        status: file.pricing_status,
        page_count: file.page_count,
        created_at: file.created_at,
        route_path: `/drive?q=${encodeURIComponent(file.order_no)}&scope=orders`,
        locate: { source: 'piecework_item', item_id: file.submission_item_id, submission_id: file.submission_id, order_no: file.order_no },
        meta_json: { upload_directory_name: file.upload_directory_name },
      })
    }
  }
  return { items: rows, total: rows.length, page: 1, size: rows.length }
}

function systemSearchResult(url) {
  const q = (url.searchParams.get('q') || '').trim().toLowerCase()
  const source = url.searchParams.get('source') || 'all'
  const items = systemAssets.filter((asset) => {
    if (source !== 'all' && asset.source_type !== source) return false
    if (!q) return true
    return [asset.product_name, asset.file_name, asset.original_filename, asset.scope_sku_code, asset.resource_id, asset.origin_path]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
      .includes(q)
  })
  return { items, total: items.length, page: 1, size: items.length }
}

function materialBrowseResult(url) {
  const path = url.searchParams.get('path') || ''
  const source = url.searchParams.get('source') || 'all'
  const systemRows = systemAssets.filter((asset) => asset.source_type === 'system')
  const externalRows = systemAssets.filter((asset) => asset.source_type === 'external')
  if (!path) {
    return {
      path: '',
      folders: [
        ...(source === 'all' || source === 'system' ? [{ path: '/系统资源', name: '系统资源', source_type: 'system', file_count: systemRows.length, direct_file_count: systemRows.length }] : []),
        ...(source === 'all' || source === 'external' ? [{ path: '/p3', name: 'p3', source_type: 'external', file_count: externalRows.length, direct_file_count: externalRows.length }] : []),
      ],
      files: [],
      total: 0,
      page: 1,
      size: 100,
    }
  }
  if (path === '/系统资源') {
    return { path, folders: [], files: systemRows, total: systemRows.length, page: 1, size: 100 }
  }
  if (path === '/p3' || path === '/quark') {
    return { path, folders: [], files: externalRows, total: externalRows.length, page: 1, size: 100 }
  }
  return { path, folders: [], files: [], total: 0, page: 1, size: 100 }
}

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
  { name: 'admin-drive', path: '/drive?q=poster&scope=all', ready: '.aw-drive', role: 'admin' },
  { name: 'admin-quality-errors', path: '/quality-errors', ready: '.aw-quality-errors-page', role: 'admin' },
  { name: 'admin-settlement', path: '/settlement', ready: '.aw-console-hero', role: 'admin' },
  { name: 'admin-notifications', path: '/notifications', ready: '.aw-compact-list', role: 'admin' },
  { name: 'simple-home', path: '/', ready: '.aw-simple-home', role: 'simple' },
  { name: 'simple-upload', path: '/upload', ready: '.aw-dropzone', role: 'simple' },
  { name: 'simple-drive', path: '/drive?scope=operational', ready: '.aw-drive', role: 'simple' },
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
  const supplementPermission = {
    id: 31,
    payee_user_id: bootstrap.profile.user_id,
    business_month: '2026-06',
    enabled: role === 'simple',
    reason: role === 'simple' ? 'fixture supplement upload' : '',
    granted_by: 1001,
    granted_at: '2026-06-24T09:00:00+08:00',
  }
  const supplementUploads = new Map()
  const fixtureSupplements = role === 'simple' ? [{
    id: 701,
    submission_item_id: 801,
    payee_user_id: bootstrap.profile.user_id,
    business_month: '2026-06',
    status: 'approved',
    order_no: '已补录海报.png',
    supplement_date: '2026-06-18',
    difficulty_class: 'A',
    finalized: true,
    page_count: 1,
    gross_amount: 120,
    files: [{
      id: 901,
      submission_id: 601,
      submission_item_id: 801,
      upload_directory_id: uploadDirectories[0].id,
      upload_directory_name: uploadDirectories[0].name,
      upload_directory_difficulty_class: uploadDirectories[0].difficulty_class,
      display_name: '已补录海报.png',
      original_filename: '已补录海报.png',
      file_type: 'image',
      mime_type: 'image/png',
      file_size: 2048,
      preview_status: 'ready',
    }],
  }] : []
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
    if (path.endsWith('/notifications/unread-count')) return json(route, { unread_count: 1 })
    if (path.endsWith('/notifications/read-all') || /\/notifications\/\d+\/read$/.test(path)) return json(route, {})
    if (path.endsWith('/notifications')) {
      return json(route, [
        {
          id: 801,
          notification_type: 'asset_workbench_profile_incomplete',
          payload: {
            source: 'asset_workbench',
            action: 'complete_profile',
            missing_fields: ['id_card'],
          },
          is_read: false,
          created_at: '2026-06-24T11:30:00+08:00',
        },
      ])
    }
    if (path.endsWith('/upload-sessions') && route.request().method() === 'POST') {
      const payload = route.request().postDataJSON()
      const sessionId = `fixture-supplement-${supplementUploads.size + 1}`
      supplementUploads.set(sessionId, payload)
      return json(route, {
        session: { session_id: sessionId, status: 'created' },
        plan: {
          mode: 'single_part',
          method: 'PUT',
          upload_url: `https://fixture-upload.invalid/${sessionId}`,
          object_key: `asset-workbench/fixture/${payload.original_filename}`,
          required_upload_content_type: payload.mime_type,
        },
      })
    }
    if (/\/upload-sessions\/[^/]+\/(complete|cancel)$/.test(path)) return json(route, {})
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
        supplement_permission: supplementPermission,
        supplements: fixtureSupplements,
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
    if (path.endsWith('/settlement/supplements') && route.request().method() === 'POST') {
      const payload = route.request().postDataJSON()
      const session = supplementUploads.get(payload.upload_session_ids?.[0]) ?? {}
      const directory = uploadDirectories.find((item) => item.id === session.upload_directory_id) ?? uploadDirectories[0]
      const created = {
        id: 701 + fixtureSupplements.length,
        submission_item_id: 801 + fixtureSupplements.length,
        ...payload,
        gross_amount: directory.difficulty_class === 'A' ? 120 : 80,
        files: [{
          id: 901 + fixtureSupplements.length,
          submission_id: 601 + fixtureSupplements.length,
          submission_item_id: 801 + fixtureSupplements.length,
          upload_directory_id: directory.id,
          upload_directory_name: directory.name,
          upload_directory_difficulty_class: directory.difficulty_class,
          display_name: payload.order_no,
          original_filename: payload.order_no,
          file_type: 'image',
          mime_type: session.mime_type || 'image/png',
          file_size: session.file_size || 1024,
          preview_status: 'ready',
        }],
      }
      fixtureSupplements.push(created)
      return json(route, created)
    }
    if (path.endsWith('/settlement/supplements')) return paginated(route, fixtureSupplements)
    if (path.endsWith('/settlement/supplement-permissions')) return paginated(route, [])
    if (path.endsWith('/difficulty-classes') || path.endsWith('/difficulty-classes/admin')) return json(route, difficultyClasses)
    if (path.endsWith('/deduction-rules')) return paginated(route, deductionRules)
    if (path.endsWith('/error-imports/excel') && route.request().method() === 'POST') {
      return json(route, {
        id: 901,
        import_no: 'ERR-202606-001',
        business_month: url.searchParams.get('business_month') || '2026-06',
        uploaded_by: 1001,
        original_filename: 'quality-errors.xlsx',
        status: 'completed',
        total_rows: 2,
        matched_rows: 2,
        unmatched_rows: 0,
        ambiguous_rows: 0,
      })
    }
    if (path.endsWith('/overview-search')) return json(route, overviewRows(url))
    if (path.endsWith('/drive/directories')) return json(route, driveDirectories())
    if (path.endsWith('/drive/orders')) return json(route, driveOrders(url))
    if (path.endsWith('/drive/files')) return paginated(route, driveFilesFor(url))
    if (path.endsWith('/drive/search')) return paginated(route, driveSearchRows(url))
    if (path.endsWith('/drive/locate')) {
      const fileID = Number(url.searchParams.get('file_id') || 0)
      return json(route, driveFiles.find((file) => file.id === fileID) ?? driveFiles[0])
    }
    if (/\/files\/\d+\/preview$/.test(path)) {
      return json(route, {
        status: 'ready',
        preparing: false,
        preview_url: previewImageUrl,
        download_url: previewImageUrl,
        mime_type: 'image/png',
        filename: 'drive-preview.png',
        preview_available: true,
      })
    }
    if (/\/files\/\d+\/download$/.test(path)) {
      return json(route, {
        download_mode: 'direct',
        download_url: previewImageUrl,
        filename: 'drive-download.png',
        file_size: 245760,
        mime_type: 'image/png',
      })
    }
    if (path.endsWith('/system-search')) return json(route, systemSearchResult(url))
    if (path.endsWith('/materials/browse')) return json(route, materialBrowseResult(url))
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

  await context.route('https://fixture-upload.invalid/**', async (route) => {
    await route.fulfill({ status: 200, headers: { ETag: 'fixture-etag' }, body: '' })
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
