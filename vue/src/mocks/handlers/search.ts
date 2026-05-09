import { mockAssets } from '../db/assets'
import { mockTasks } from '../db/tasks'
import type { MockHandler } from './types'

export const searchHandler: MockHandler = (request) => {
  if (request.method !== 'GET' || request.path !== '/v1/search') return null

  const q = String(request.query.q ?? '').trim()
  const scope = String(request.query.scope ?? 'all').trim()
  const limitRaw = Number(request.query.limit ?? 20)
  const limit = Number.isFinite(limitRaw) && limitRaw > 0 ? Math.floor(limitRaw) : 20
  const kw = q.toLowerCase()

  const taskHits = mockTasks
    .filter((task) => {
      if (!kw) return true
      return (
        task.task_no.toLowerCase().includes(kw) ||
        task.title.toLowerCase().includes(kw) ||
        task.task_type.toLowerCase().includes(kw) ||
        task.created_by.toLowerCase().includes(kw)
      )
    })
    .slice(0, limit)
    .map((task) => ({
      id: Number.parseInt(task.id, 10) || 0,
      task_no: task.task_no,
      title: task.title,
      task_status: task.status,
      priority: task.priority,
      task_type: task.task_type,
      sku_code: null,
      primary_sku_code: null,
      i_id: null,
      owner_department: null,
      owner_team: null,
      owner_org_team: null,
      creator_id: null,
      creator_name: task.created_by,
      designer_id: null,
      designer_name: null,
      created_at: task.created_at,
      deadline_at: null,
      highlight: null,
    }))

  const assetHits = mockAssets
    .filter((asset) => {
      if (!kw) return true
      return (
        asset.file_name.toLowerCase().includes(kw) ||
        asset.task_id.toLowerCase().includes(kw) ||
        asset.id.toLowerCase().includes(kw)
      )
    })
    .slice(0, limit)
    .map((asset) => ({
      asset_id: Number.parseInt(asset.id, 10) || 0,
      file_name: asset.file_name,
      source_module_key: asset.file_role,
      task_id: Number.parseInt(asset.task_id, 10) || 0,
    }))

  const productPool = [
    { erp_code: 'HSC34009', product_name: '产品名称', i_id: '常规KT板', category: '常规KT板' },
    { erp_code: 'HBJ12001', product_name: '海报模板', i_id: 'HBJ', category: 'HBJ' },
  ]
  const productHits = productPool
    .filter((item) => {
      if (!kw) return true
      return (
        item.erp_code.toLowerCase().includes(kw) ||
        item.product_name.toLowerCase().includes(kw) ||
        item.i_id.toLowerCase().includes(kw)
      )
    })
    .slice(0, limit)

  const userHits: Array<{
    user_id: number
    username: string
    department_name: string
  }> = []

  const includeTasks = scope === 'all' || scope === 'tasks'
  const includeAssets = scope === 'all' || scope === 'assets'
  const includeProducts = scope === 'all' || scope === 'products'
  const includeUsers = scope === 'all' || scope === 'users'

  return {
    status: 200,
    data: {
      query: q,
      results: {
        tasks: includeTasks ? taskHits : [],
        assets: includeAssets ? assetHits : [],
        products: includeProducts ? productHits : [],
        users: includeUsers ? userHits : [],
      },
    },
  }
}
