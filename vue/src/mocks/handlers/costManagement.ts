import type { MockHandler } from './types'
import { addMillisecondsToNowISO } from '@/utils/date'

const mockCostRules = [
  { rule_id: 1, rule_name: 'KT 板基础单价', rule_version: 3, category_code: 'KT_BOARD', product_family: 'KT 板', rule_type: 'fixed_unit_price', base_price: 24, tax_multiplier: 1, priority: 100, is_active: true },
  { rule_id: 2, rule_name: 'KT 板最低计价面积', rule_version: 2, category_code: 'KT_BOARD', product_family: 'KT 板', rule_type: 'minimum_billable_area', min_area: 0.25, priority: 90, is_active: true },
  { rule_id: 3, rule_name: '写真固定单价', rule_version: 5, category_code: 'PHOTO_PRINT', product_family: '写真', rule_type: 'fixed_unit_price', base_price: 18, priority: 100, is_active: true },
]

const mockCostRuns = [{
  id: 1,
  run_no: 'CR-MOCK-001',
  status: 'previewed',
  mode: 'all_matching',
  created_at: addMillisecondsToNowISO(-30 * 60_000),
  summary: { total_count: 3, previewed_count: 3, applied_count: 0, erp_synced_count: 0, conflict_count: 0 },
}]

function loadAuditCostRules() {
  if (import.meta.env.VITE_LARGE_SURFACE_AUDIT !== 'true') return mockCostRules
  return Array.from({ length: 100 }, (_, index) => ({
    rule_id: 1000 + index,
    rule_name: `承载审计规则 ${String(index + 1).padStart(3, '0')}`,
    rule_version: 1,
    category_code: 'LOAD_RULES',
    product_family: '承载审计规则组',
    rule_type: index % 3 === 0 ? 'minimum_billable_area' : 'fixed_unit_price',
    base_price: 10 + index,
    min_area: 0.2,
    priority: 100 - index,
    is_active: true,
  }))
}

export const costManagementHandler: MockHandler = (request) => {
  if (request.method === 'GET' && request.path === '/v1/cost-rules') {
    const data = loadAuditCostRules()
    return { status: 200, data: { data, pagination: { page: 1, page_size: data.length, total: data.length } } }
  }
  if (request.method === 'POST' && request.path === '/v1/cost-rules') return { status: 201, data: { data: request.body } }
  if (request.method === 'PATCH' && /^\/v1\/cost-rules\/\d+$/.test(request.path)) return { status: 200, data: { data: request.body } }
  if (request.method === 'POST' && request.path === '/v1/cost-rules/preview') return { status: 200, data: { data: { matched_rule_id: 1, matched_rule_version: 3, estimated_cost: 24, explanation: '按当前规则试算，未修改任何业务数据。' } } }
  if (request.method === 'GET' && request.path === '/v1/cost-rule-bindings') return { status: 200, data: { data: [{ id: 1, i_id_raw: 'STYLE-MOCK-001', normalized_i_id: 'STYLE-MOCK-001', rule_group: 'KT_BOARD', display_name: '标准 KT 板', is_active: true }], pagination: { page: 1, page_size: 500, total: 1 } } }
  if (request.method === 'POST' && request.path === '/v1/cost-rule-bindings') return { status: 201, data: { data: { id: 2, ...request.body } } }
  if (request.method === 'GET' && request.path === '/v1/cost-rule-bindings/unbound-candidates') return { status: 200, data: { data: [{ normalized_i_id: 'STYLE-MOCK-002', display_i_id: 'STYLE-MOCK-002', suggested_display_name: '待绑定款式', sku_count: 2, task_count: 1 }] } }
  if (request.method === 'GET' && request.path === '/v1/cost-management/dashboard') return { status: 200, data: { data: { total_count: 3, total_records: 3, groups: [], tags: [{ code: 'erp_mismatch', label: 'ERP 差异', count: 1 }] } } }
  if (request.method === 'GET' && request.path === '/v1/cost-management/recalculation-runs') return { status: 200, data: { data: mockCostRuns } }

  const costRunMatch = request.path.match(/^\/v1\/cost-management\/recalculation-runs\/(\d+)(?:\/(apply|sync-erp|cancel))?$/)
  if (costRunMatch && request.method === 'GET') return { status: 200, data: { data: { ...mockCostRuns[0], items: [{ id: 1, run_id: 1, product_management_record_id: 1, task_no: 'T-20260423-1001', sku_code: 'YB-MOCK-001', old_cost_price: 18.6, new_cost_price: 24, status: 'previewed' }] } } }
  if (costRunMatch && request.method === 'POST' && costRunMatch[2] === 'apply') return { status: 200, data: { run: { ...mockCostRuns[0], status: 'applied' } } }
  if (costRunMatch && request.method === 'POST' && costRunMatch[2] === 'sync-erp') return { status: 200, data: { run: { ...mockCostRuns[0], status: 'erp_syncing' } } }
  if (costRunMatch && request.method === 'POST' && costRunMatch[2] === 'cancel') return { status: 200, data: { data: { ...mockCostRuns[0], status: 'cancelled' } } }
  if (request.method === 'POST' && request.path === '/v1/cost-management/recalculation-runs') return { status: 201, data: { data: mockCostRuns[0] } }
  return null
}
