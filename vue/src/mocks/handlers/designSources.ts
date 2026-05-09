import type { MockHandler } from './types'

export const designSourcesHandler: MockHandler = (request) => {
  if (request.method === 'GET' && request.path === '/v1/design-sources/search') {
    const keyword = String(request.query.keyword ?? '').trim()
    const pageRaw = Number(request.query.page ?? 1)
    const sizeRaw = Number(request.query.size ?? 20)
    const page = Number.isFinite(pageRaw) && pageRaw > 0 ? Math.floor(pageRaw) : 1
    const size = Number.isFinite(sizeRaw) && sizeRaw > 0 ? Math.floor(sizeRaw) : 20
    const all = [
      {
        id: 'ds_1001',
        file_name: '基础款-v1.psd',
        owner_team_code: 'design-team-a',
        preview_url: '/mock/design-sources/ds_1001.png',
        source: 'nas',
      },
      {
        id: 'ds_1002',
        file_name: '定制参考-v2.psd',
        owner_team_code: 'design-team-b',
        preview_url: '/mock/design-sources/ds_1002.png',
        source: 'oss',
      },
    ].filter((item) => !keyword || item.file_name.includes(keyword))
    const start = (page - 1) * size
    const data = all.slice(start, start + size)
    return {
      status: 200,
      data: {
        data,
        total: all.length,
        page,
        size,
      },
    }
  }
  return null
}
