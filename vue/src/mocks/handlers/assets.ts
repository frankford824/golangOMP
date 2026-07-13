import { mockAssets } from '../db/assets'
import type { MockHandler } from './types'
import { addMillisecondsToNowISO, nowISO } from '@/utils/date'

type MockAssetRecord = (typeof mockAssets)[number]

interface MockReplacementSession {
  assetID: string
  fileName: string
  fileRole: MockAssetRecord['file_role']
  taskID: string
}

const mockReplacementSessions = new Map<string, MockReplacementSession>()

const LARGE_SURFACE_AUDIT_TOTAL = Number(import.meta.env.VITE_LARGE_SURFACE_TOTAL ?? 5000)
const LARGE_SURFACE_AUDIT_PAGE_SIZE = Number(import.meta.env.VITE_LARGE_SURFACE_PAGE_SIZE ?? 100)

function isLargeSurfaceAuditEnabled(): boolean {
  return import.meta.env.VITE_LARGE_SURFACE_AUDIT === 'true'
}

function largeSurfaceSize(raw: unknown): number {
  const fallback = Number.isFinite(LARGE_SURFACE_AUDIT_PAGE_SIZE) ? LARGE_SURFACE_AUDIT_PAGE_SIZE : 100
  const candidate = Math.max(fallback, Number(raw ?? fallback))
  return Math.min(150, Math.max(80, Math.floor(candidate)))
}

function largeSurfaceAssets(q: Record<string, unknown>) {
  const page = Math.max(1, Number(q.page ?? 1))
  const size = largeSurfaceSize(q.size)
  const total = Number.isFinite(LARGE_SURFACE_AUDIT_TOTAL) ? LARGE_SURFACE_AUDIT_TOTAL : 5000
  const start = (page - 1) * size
  const roles: Array<'source' | 'delivery' | 'reference'> = ['delivery', 'source', 'reference']
  const data = Array.from({ length: size }, (_, index) => {
    const seq = start + index + 1
    const role = roles[seq % roles.length]
    return {
      id: `asset_load_${seq}`,
      asset_id: 700000 + seq,
      asset_version_id: 800000 + seq,
      task_id: String(900000 + seq),
      task_no: `LT-${String(seq).padStart(5, '0')}`,
      sku_code: `SKU-LOAD-${String(seq).padStart(5, '0')}`,
      product_name: `长列表承载审计素材 ${seq}`,
      title: `长列表承载审计素材 ${String(seq).padStart(5, '0')}`,
      file_name: `load-audit-${String(seq).padStart(5, '0')}.png`,
      filename: `load-audit-${String(seq).padStart(5, '0')}.png`,
      file_role: role,
      asset_kind: role,
      resource_source: 'internal',
      source: 'asset_center',
      usable_state: seq % 7 === 0 ? 'archived' : 'usable',
      mime_type: 'image/png',
      file_size: 204800 + seq,
      created_at: addMillisecondsToNowISO(-seq * 60_000),
      preview_url: '',
      download_url: '',
      versions: [],
    }
  })
  return { data, total, page, size }
}

function buildMockAssetDetail(asset: MockAssetRecord) {
  const baseFileName = asset.file_name || `${asset.id}.png`
  const numericTaskID = asset.task_id === 'task_1002' ? 1002 : asset.task_id
  const numericAssetID = asset.id === 'asset_1001' ? 1001 : asset.id
  return {
    ...asset,
    task_id: numericTaskID,
    asset_id: numericAssetID,
    task_status: 'Completed',
    asset_kind: asset.file_role,
    asset_type: asset.file_role,
    upload_status: 'uploaded',
    download_mode: 'direct',
    preview_available: true,
    source_type: 'system',
    source_label: '系统资产',
    resource_id: asset.id,
    task_no: asset.task_id === 'task_1002' ? 'T-20260423-1002' : asset.task_id,
    sku_code: 'SKU-MOCK-1002',
    primary_sku_code: 'SKU-MOCK-1002',
    scope_sku_code: 'SKU-MOCK-1002',
    product_name: '常规定制补图素材',
    workflow_lane: 'normal',
    source_department: '设计部',
    task_creator_name: 'ops_demo',
    created_by_name: 'designer_demo',
    mime_type: 'image/png',
    current_asset_id: asset.id,
    usable_state: 'usable',
    usable_label: '当前有效',
    last_access_at: addMillisecondsToNowISO(-5 * 60_000),
    versions: [
      {
        id: `${asset.id}_v2`,
        version: 2,
        file_role: asset.file_role,
        file_name: baseFileName,
        mime_type: 'image/png',
        download_mode: 'direct',
        preview_available: true,
        usable_state: 'usable',
        usable_label: '当前有效',
        created_at: asset.created_at,
        created_by: { user_id: 'u_2', username: 'designer_demo', name: '设计演示' },
      },
      {
        id: `${asset.id}_v1`,
        version: 1,
        file_role: asset.file_role,
        file_name: baseFileName.replace(/(\.[^.]+)?$/, '-history$1'),
        mime_type: 'image/png',
        download_mode: 'direct',
        preview_available: true,
        usable_state: 'superseded',
        usable_label: '历史版本',
        created_at: addMillisecondsToNowISO(-35 * 60_000),
        created_by: { user_id: 'u_1', username: 'ops_demo', name: '运营演示' },
      },
    ],
  }
}

function mockNumericAssetID(asset: MockAssetRecord): string {
  return asset.id === 'asset_1001' ? '1001' : asset.id
}

function buildMockAssetAccessMeta(asset: MockAssetRecord) {
  return {
    download_url: `/mock-assets/${asset.id}.png`,
    file_name: asset.file_name,
    filename: asset.file_name,
    mime_type: 'image/png',
    download_mode: 'direct',
    preview_available: true,
  }
}

export const assetsHandler: MockHandler = (request) => {
  if (request.method === 'POST' && request.path === '/v1/tasks/reference-upload') {
    const assetId = `asset_ref_${Date.now()}`
    return {
      status: 200,
      data: {
        data: {
          asset_id: assetId,
          ref_id: assetId,
          filename: 'reference-upload.png',
          mime_type: 'image/png',
          file_size: 1024,
          download_url: `/mock-assets/${assetId}.png`,
          status: 'uploaded',
          source: 'mock_reference_upload',
        },
      },
    }
  }

  if (request.method === 'GET' && request.path === '/v1/assets') {
    const items = mockAssets.map(buildMockAssetDetail)
    return { status: 200, data: { items, total: items.length } }
  }

  if (request.method === 'GET' && request.path === '/v1/assets/search') {
    if (isLargeSurfaceAuditEnabled()) {
      return { status: 200, data: largeSurfaceAssets(request.query as Record<string, unknown>) }
    }
    const keyword = String(request.query.keyword ?? '').trim()
    const pageRaw = Number(request.query.page ?? 1)
    const sizeRaw = Number(request.query.size ?? 20)
    const page = Number.isFinite(pageRaw) && pageRaw > 0 ? Math.floor(pageRaw) : 1
    const size = Number.isFinite(sizeRaw) && sizeRaw > 0 ? Math.floor(sizeRaw) : 20
    const matched = mockAssets.filter((asset) =>
      !keyword || `${asset.id} ${asset.task_id} ${asset.file_name}`.toLowerCase().includes(keyword.toLowerCase()),
    )
    const start = (page - 1) * size
    const data = matched.slice(start, start + size).map(buildMockAssetDetail)
    return { status: 200, data: { data, total: matched.length, page, size } }
  }

  const assetPreviewMatch = request.path.match(/^\/v1\/assets\/([^/]+)\/preview$/)
  if (request.method === 'GET' && assetPreviewMatch) {
    const asset = mockAssets.find((item) => item.id === assetPreviewMatch[1])
    if (!asset) return { status: 404, data: { message: 'asset not found' } }
    return { status: 200, data: { data: buildMockAssetAccessMeta(asset) } }
  }

  const assetDownloadMatch = request.path.match(/^\/v1\/assets\/([^/]+)\/download$/)
  if (request.method === 'GET' && assetDownloadMatch) {
    const asset = mockAssets.find((item) => item.id === assetDownloadMatch[1])
    if (!asset) return { status: 404, data: { message: 'asset not found' } }
    return { status: 200, data: { data: buildMockAssetAccessMeta(asset) } }
  }

  const assetDetailMatch = request.path.match(/^\/v1\/assets\/([^/]+)$/)
  if (request.method === 'GET' && assetDetailMatch) {
    const asset = mockAssets.find((item) => item.id === assetDetailMatch[1])
    if (!asset) return { status: 404, data: { message: 'asset not found' } }
    return { status: 200, data: { data: buildMockAssetDetail(asset) } }
  }

  if (request.method === 'POST' && request.path === '/v1/assets/upload-sessions') {
    const sessionId = `us_${Date.now()}`
    const requestedAssetID = String(request.body?.asset_id ?? '')
    const target = mockAssets.find((item) => mockNumericAssetID(item) === requestedAssetID)
    if (target) {
      mockReplacementSessions.set(sessionId, {
        assetID: target.id,
        fileName: String(request.body?.file_name ?? target.file_name),
        fileRole: (request.body?.asset_kind as MockAssetRecord['file_role']) ?? target.file_role,
        taskID: String(request.body?.task_id ?? target.task_id),
      })
    }
    return {
      status: 201,
      data: {
        data: {
          session: {
            session_id: sessionId,
            expected_size: Number(request.body?.expected_size ?? 0),
            session_status: 'created',
          },
          remote: {
            upload_url: 'https://mock-upload.local/session',
            method: 'PUT',
          },
          expires_at: addMillisecondsToNowISO(30 * 60_000),
        },
      },
    }
  }

  if (
    request.method === 'POST' &&
    (request.path === '/v1/task-create/asset-center/upload-sessions' ||
      request.path.match(/^\/v1\/tasks\/[^/]+\/asset-center\/upload-sessions(\/small|\/multipart)?$/))
  ) {
    const sessionId = `us_${Date.now()}`
    return {
      status: 201,
      data: {
        data: {
          session: {
            session_id: sessionId,
            expected_size: Number(request.body?.expected_size ?? 0),
            session_status: 'created',
          },
          remote: {
            upload_url: 'https://mock-upload.local/session',
            method: 'PUT',
          },
        },
      },
    }
  }

  const globalComplete = request.path.match(/^\/v1\/assets\/upload-sessions\/([^/]+)\/complete$/)
  const taskComplete = request.path.match(/^\/v1\/tasks\/([^/]+)\/asset-center\/upload-sessions\/([^/]+)\/complete$/)
  if (request.method === 'POST' && (globalComplete || taskComplete)) {
    const sessionId = globalComplete?.[1] ?? taskComplete?.[2] ?? `us_${Date.now()}`
    const replacement = mockReplacementSessions.get(sessionId)
    let completedAsset: MockAssetRecord | undefined
    if (replacement) {
      const target = mockAssets.find((item) => item.id === replacement.assetID)
      if (target) {
        target.file_name = replacement.fileName
        target.file_role = replacement.fileRole
        target.created_at = nowISO()
        completedAsset = target
      } else {
        completedAsset = mockAssets[0]
      }
      mockReplacementSessions.delete(sessionId)
    } else {
      const taskId = String(request.body?.task_id ?? 'task_1001')
      completedAsset = {
        id: `asset_${Date.now()}`,
        task_id: taskId,
        file_name: String(request.body?.file_name ?? 'uploaded-file.psd'),
        file_role: (request.body?.file_role as 'source' | 'delivery' | 'reference') ?? 'delivery',
        created_at: nowISO(),
      }
      mockAssets.unshift(completedAsset)
    }
    if (!completedAsset) {
      return { status: 404, data: { message: 'asset not found' } }
    }
    return {
      status: 200,
      data: {
        data: {
          session: { session_id: sessionId, session_status: 'completed', upload_status: 'uploaded' },
          asset: buildMockAssetDetail(completedAsset),
        },
      },
    }
  }

  if (
    request.method === 'POST' &&
    request.path.match(/^\/v1\/tasks\/[^/]+\/asset-center\/upload-sessions\/[^/]+\/(cancel|abort)$/)
  ) {
    return { status: 200, data: { success: true } }
  }

  if (request.method === 'GET' && request.path.match(/^\/v1\/tasks\/[^/]+\/asset-center\/assets$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const items = mockAssets
      .filter((asset) => String(asset.task_id ?? '') === taskId)
      .map(buildMockAssetDetail)
    return { status: 200, data: { items, total: items.length } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/assets\/[^/]+\/archive$/)) {
    return { status: 200, data: { success: true } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/assets\/[^/]+\/restore$/)) {
    return { status: 200, data: { success: true } }
  }

  const assetDeleteMatch = request.path.match(/^\/v1\/assets\/([^/]+)$/)
  if (request.method === 'DELETE' && assetDeleteMatch) {
    const assetIndex = mockAssets.findIndex(
      (item) => item.id === assetDeleteMatch[1] || mockNumericAssetID(item) === assetDeleteMatch[1],
    )
    if (assetIndex < 0) return { status: 404, data: { message: 'asset not found' } }
    mockAssets.splice(assetIndex, 1)
    return { status: 204, data: null }
  }

  return null
}
