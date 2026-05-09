import { mockAssets } from '../db/assets'
import type { MockHandler } from './types'
import { addMillisecondsToNowISO, nowISO } from '@/utils/date'

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
    return { status: 200, data: { items: mockAssets, total: mockAssets.length } }
  }

  if (request.method === 'GET' && request.path === '/v1/assets/search') {
    const keyword = String(request.query.keyword ?? '').trim()
    const pageRaw = Number(request.query.page ?? 1)
    const sizeRaw = Number(request.query.size ?? 20)
    const page = Number.isFinite(pageRaw) && pageRaw > 0 ? Math.floor(pageRaw) : 1
    const size = Number.isFinite(sizeRaw) && sizeRaw > 0 ? Math.floor(sizeRaw) : 20
    const matched = mockAssets.filter((asset) =>
      !keyword || `${asset.id} ${asset.task_id} ${asset.file_name}`.toLowerCase().includes(keyword.toLowerCase()),
    )
    const start = (page - 1) * size
    const data = matched.slice(start, start + size)
    return { status: 200, data: { data, total: matched.length, page, size } }
  }

  if (request.method === 'POST' && request.path === '/v1/assets/upload-sessions') {
    const sessionId = `us_${Date.now()}`
    return {
      status: 201,
      data: {
        upload_session_id: sessionId,
        upload_url: 'https://mock-upload.local/session',
        expires_at: addMillisecondsToNowISO(30 * 60_000),
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
    const taskId = String(request.body?.task_id ?? 'task_1001')
    const sessionId = globalComplete?.[1] ?? taskComplete?.[2] ?? `us_${Date.now()}`
    mockAssets.unshift({
      id: `asset_${Date.now()}`,
      task_id: taskId,
      file_name: String(request.body?.file_name ?? 'uploaded-file.psd'),
      file_role: (request.body?.file_role as 'source' | 'delivery' | 'reference') ?? 'delivery',
      created_at: nowISO(),
    })
    return {
      status: 200,
      data: {
        data: {
          session: { session_id: sessionId, session_status: 'completed', upload_status: 'uploaded' },
          asset: mockAssets[0],
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
    const items = mockAssets.filter((asset) => String(asset.task_id ?? '') === taskId)
    return { status: 200, data: { items, total: items.length } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/assets\/[^/]+\/archive$/)) {
    return { status: 200, data: { success: true } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/assets\/[^/]+\/restore$/)) {
    return { status: 200, data: { success: true } }
  }

  if (request.method === 'DELETE' && request.path.match(/^\/v1\/assets\/[^/]+$/)) {
    return { status: 200, data: { success: true } }
  }

  return null
}
