import type { MockHandler } from './types'

const largeTotal = Number(import.meta.env.VITE_LARGE_SURFACE_TOTAL ?? 5000)
const isLarge = () => import.meta.env.VITE_LARGE_SURFACE_AUDIT === 'true'
const preview = 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="160" height="120"%3E%3Crect width="160" height="120" fill="%23e8edf4"/%3E%3Ctext x="80" y="65" text-anchor="middle" fill="%235b6678" font-size="14"%3EFINAL%3C/text%3E%3C/svg%3E'

function resourceGroup(id: number) {
  return {
    id,
    task_id: 1000 + id,
    task_no: `RW-MOCK-${String(id).padStart(4, '0')}`,
    scope_kind: 'sku',
    task_sku_item_id: 2000 + id,
    sku_code: `SKU-MOCK-${String(id).padStart(4, '0')}`,
    product_name: id % 3 === 0 ? '轻奢客厅软装套系' : '北欧家居视觉组合',
    creator_id: 2,
    creator_name: '李运营',
    business_lane: id % 3 === 0 ? 'customization' : 'normal',
    sku_profile: {
      id: 3000 + id,
      task_id: 1000 + id,
      task_sku_item_id: 2000 + id,
      sku_code: `SKU-MOCK-${String(id).padStart(4, '0')}`,
      product_i_id: `STYLE-${String((id % 4) + 1).padStart(2, '0')}`,
      erp_i_id: `ERP-${1000 + id}`,
      product_name: id % 3 === 0 ? '轻奢客厅软装套系' : '北欧家居视觉组合',
      category_name: '家居视觉',
      product_family: id % 2 === 0 ? 'KT 板' : '写真画面',
      combo_sku_codes: id % 4 === 0 ? [`COMBO-${String(id).padStart(3, '0')}`] : [],
      cost_price: 18.5 + id,
      cost_trace: { rule_name: id % 2 === 0 ? 'KT 板面积计价' : '写真固定单价', matched_rule_version: 3 },
      spec_text: id % 2 === 0 ? '60 × 90 cm' : 'A2',
      size_text: id % 2 === 0 ? '宽 0.6m × 高 0.9m' : '宽 0.42m × 高 0.594m',
      area_trace: { width_m: id % 2 === 0 ? 0.6 : 0.42, height_m: id % 2 === 0 ? 0.9 : 0.594, area_m2: id % 2 === 0 ? 0.54 : 0.2495, quantity: 1, formula: '宽 × 高 × 数量', source_label: '任务规格' },
      erp_sync_status: id % 5 === 0 ? 'failed' : 'synced',
      last_erp_synced_at: '2026-07-18T08:00:00Z',
    },
    lock_version: 1,
    migration_incomplete: false,
    finalized_revision: {
      id: 5000 + id,
      group_id: id,
      revision_no: 1,
      status: 'finalized',
      mode: id % 4 === 0 ? 'set' : 'single',
      source_stage: 'audit',
      source_file: { task_asset_id: 7000 + id, file_name: `source-${id}.psd` },
      items: Array.from({ length: id % 4 === 0 ? 2 : 1 }, (_, index) => ({
        id: 8000 + id * 10 + index,
        revision_id: 5000 + id,
        task_asset_id: 9000 + id * 10 + index,
        sort_order: index + 1,
        file: { task_asset_id: 9000 + id * 10 + index, file_name: `final-${id}-${index + 1}.png`, mime_type: 'image/png', preview_url: preview, download_url: preview },
      })),
      references: [{
        id: 10000 + id,
        reference_file_ref_id: 11000 + id,
        sort_order: 0,
        ref_id: `REF-MOCK-${id}`,
        file_name: `reference-${id}.jpg`,
        mime_type: 'image/jpeg',
        file_size: 245760,
        preview_url: preview,
        download_url: preview,
      }],
    },
  }
}

const roles = [
  { id: 1, code: 'member', name: '成员', description: '基础访问', system_protected: true, version: 1, permissions: ['account.use'] },
  { id: 2, code: 'reviewer', name: '审核人员', description: '审核任务', system_protected: false, version: 1, permissions: ['task.audit'] },
]

export const v8Handler: MockHandler = (request) => {
  if (request.method === 'GET' && request.path === '/v1/resource-groups') {
    const page = Math.max(1, Number(request.query.page ?? 1))
    const requestedSize = Math.max(1, Number(request.query.page_size ?? 24))
    const pageSize = isLarge() ? Math.max(80, requestedSize) : requestedSize
    const total = isLarge() ? largeTotal : 6
    const start = (page - 1) * pageSize
    const items = Array.from({ length: Math.max(0, Math.min(pageSize, total - start)) }, (_, index) => resourceGroup(start + index + 1))
    return { status: 200, data: { data: { items, page, page_size: pageSize, total } } }
  }
  const groupMatch = request.path.match(/^\/v1\/resource-groups\/(\d+)$/)
  if (request.method === 'GET' && groupMatch) return { status: 200, data: { data: resourceGroup(Number(groupMatch[1])) } }
  if (request.method === 'POST' && request.path === '/v1/resource-groups/batch-download') return { status: 200, data: { data: { items: [] } } }

  const bundleMatch = request.path.match(/^\/v1\/tasks\/(\d+)\/resource-bundle$/)
  if (request.method === 'GET' && bundleMatch) {
    const taskID = Number(bundleMatch[1])
    return { status: 200, data: { data: { task_id: taskID, workflow_revision: 3, groups: [resourceGroup(taskID)] } } }
  }
  if (request.method === 'GET' && request.path.match(/^\/v1\/tasks\/\d+\/audit\/handovers$/)) return { status: 200, data: { data: [] } }

  if (request.method === 'GET' && request.path === '/v1/access/permissions') return { status: 200, data: { data: [{ code: 'task.audit', module: 'task', name: '审核任务', description: '审核授权范围内任务', risk_level: 'normal', enabled: true }] } }
  if (request.method === 'GET' && request.path === '/v1/access/roles') return { status: 200, data: { data: roles } }
  if (request.method === 'GET' && request.path === '/v1/access/users') return { status: 200, data: { data: { items: [{ id: 2, display_name: '李审核', username: 'reviewer', department: '设计部', department_id: 10 }], page: 1, page_size: 30, total: 1 } } }
  const effectiveMatch = request.path.match(/^\/v1\/access\/users\/(\d+)\/effective$/)
  if (request.method === 'GET' && effectiveMatch) return { status: 200, data: { data: { user_id: Number(effectiveMatch[1]), policy_revision: 4, permissions: ['task.audit'], assignments: [], sources: [] } } }
  const policyMatch = request.path.match(/^\/v1\/access\/org-policies\/(department|team)\/(\d+)$/)
  if (request.method === 'GET' && policyMatch) return { status: 200, data: { data: [] } }
  if (request.method === 'PUT' && policyMatch) return { status: 200, data: { data: { policy_revision: 5, org_policies: request.body?.policies || [] } } }
  if (request.method === 'GET' && request.path === '/v1/access/events') return { status: 200, data: { data: [{ id: 1, policy_revision: 4, actor_id: 1, action: 'assignment.replace', target_type: 'user', target_id: '2', reason: '职责调整', created_at: '2026-07-16T00:00:00Z' }] } }
  if (request.path.startsWith('/v1/access/') && ['POST', 'PUT', 'PATCH'].includes(request.method)) return { status: 200, data: { data: { policy_revision: 5 } } }
  return null
}
