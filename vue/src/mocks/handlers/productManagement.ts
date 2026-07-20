import type { MockHandler } from './types'
import { addMillisecondsToNowISO } from '@/utils/date'

type ProductSyncStatus =
  | 'pending_sync'
  | 'queued'
  | 'syncing'
  | 'synced'
  | 'failed'
  | 'cooling_down'
  | 'waiting_image'

type ProductImageSource =
  | 'manual'
  | 'erp_product_image'
  | 'delivery'
  | 'derived_preview'
  | 'task_reference'
  | 'auto_on_close'
  | 'missing'

interface MockProductRecord {
  id: number
  record_key: string
  task_id: number
  task_sku_item_id?: number
  task_no: string
  task_type?: string
  source_mode?: string
  sku_code: string
  product_i_id: string
  erp_i_id?: string
  category_name?: string
  product_family?: string
  product_name: string
  cost_price?: number | null
  cost_trace?: Record<string, unknown> | null
  creator_id: number
  creator_name: string
  task_created_at: string
  image_source: ProductImageSource
  image_source_label: string
  image_selection_mode: 'auto' | 'manual'
  image_asset_id?: number
  image_asset_version_id?: number
  image_preview_url?: string
  image_filename?: string
  image_mime_type?: string
  image_missing_reason?: string
  image_sync_source?: ProductImageSource
  erp_sync_status: ProductSyncStatus
  base_sync_status?: ProductSyncStatus
  image_sync_status?: ProductSyncStatus
  last_erp_synced_at?: string
  last_erp_checked_at?: string
  last_base_synced_at?: string
  last_image_synced_at?: string
  sync_cooldown_until?: string
  last_sync_error?: string
  base_sync_error?: string
  image_sync_error?: string
  image_required?: boolean
  can_maintain_image: boolean
  can_cross_task_select: boolean
  can_sync_erp: boolean
  can_force_override: boolean
  created_at: string
  updated_at: string
}

function productImage(label: string, fill: string): string {
  const svg = [
    `<svg xmlns="http://www.w3.org/2000/svg" width="320" height="220" viewBox="0 0 320 220">`,
    `<rect width="320" height="220" rx="28" fill="${fill}"/>`,
    `<circle cx="90" cy="82" r="34" fill="white" opacity=".72"/>`,
    `<path d="M36 178c45-52 72-64 112-28 25 22 48 18 70-4 21-21 38-18 66 32z" fill="white" opacity=".82"/>`,
    `<text x="160" y="198" text-anchor="middle" font-family="Arial" font-size="24" font-weight="700" fill="midnightblue">${label}</text>`,
    `</svg>`,
  ].join('')
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
}

function baseRecord(overrides: Partial<MockProductRecord>): MockProductRecord {
  const now = addMillisecondsToNowISO(-20 * 60_000)
  return {
    id: 1,
    record_key: 'mock:record:1',
    task_id: 1001,
    task_no: 'T-20260423-1001',
    task_type: 'original_product_development',
    source_mode: 'task',
    sku_code: 'YB-MOCK-001',
    product_i_id: 'STYLE-MOCK-001',
    erp_i_id: 'ERP-I-001',
    category_name: '装饰画',
    product_family: '画布',
    product_name: '莫兰迪蓝装饰画',
    cost_price: 18.6,
    cost_trace: {
      rule_name: '画布面积成本规则',
      rule_source: 'mock_rule',
      matched_rule_version: 3,
      input_snapshot: {
        category_name: '装饰画',
        width: 40,
        height: 60,
        area: 0.24,
        quantity: 1,
      },
      calculation_snapshot: {
        formula: '面积 x 单价 + 包装费',
        estimated_cost: 18.6,
        cost_price: 18.6,
      },
      snapshot_at: now,
    },
    creator_id: 1,
    creator_name: '演示账号',
    task_created_at: addMillisecondsToNowISO(-90 * 60_000),
    image_source: 'erp_product_image',
    image_source_label: '专项 ERP 商品图',
    image_selection_mode: 'auto',
    image_preview_url: productImage('SKU 001', 'hsl(214 95% 93%)'),
    image_filename: 'mock-product-001.png',
    image_mime_type: 'image/png',
    erp_sync_status: 'synced',
    base_sync_status: 'synced',
    image_sync_status: 'synced',
    last_erp_synced_at: now,
    last_base_synced_at: now,
    last_image_synced_at: now,
    image_required: true,
    can_maintain_image: true,
    can_cross_task_select: true,
    can_sync_erp: true,
    can_force_override: false,
    created_at: addMillisecondsToNowISO(-90 * 60_000),
    updated_at: now,
    ...overrides,
  }
}

const records: MockProductRecord[] = [
  baseRecord({}),
  baseRecord({
    id: 2,
    record_key: 'mock:record:2',
    sku_code: 'YB-MOCK-002',
    product_i_id: 'STYLE-MOCK-002',
    erp_i_id: 'ERP-I-002',
    product_name: '奶油白组合装子 SKU',
    cost_price: 23.8,
    image_source: 'delivery',
    image_source_label: 'SKU 成品图',
    image_preview_url: productImage('SKU 002', 'hsl(141 84% 93%)'),
    base_sync_status: 'pending_sync',
    image_sync_status: 'waiting_image',
    erp_sync_status: 'pending_sync',
    last_base_synced_at: undefined,
    last_image_synced_at: undefined,
  }),
  baseRecord({
    id: 3,
    record_key: 'mock:record:3',
    task_id: 1002,
    task_no: 'T-20260423-1002',
    sku_code: 'YB-MOCK-003',
    product_i_id: 'STYLE-MOCK-003',
    erp_i_id: 'ERP-I-003',
    product_name: '缺图待处理 SKU',
    cost_price: null,
    cost_trace: null,
    image_source: 'missing',
    image_source_label: '缺图',
    image_preview_url: '',
    image_missing_reason: 'ERP 图片待补充',
    base_sync_status: 'failed',
    image_sync_status: 'waiting_image',
    erp_sync_status: 'failed',
    base_sync_error: 'mock: ERP 基础资料校验失败',
    image_sync_error: undefined,
  }),
]

const groups = [
  {
    group_key: 'combo:MOCK-COMBO-001',
    group_type: 'combo',
    combo_sku_code: 'MOCK-COMBO-001',
    combo_name: 'Mock 组合装展示',
    combo_short_name: '组合装',
    erp_i_id: 'ERP-COMBO-001',
    entity_sku_id: 'ENTITY-MOCK-001',
    pic_url: productImage('COMBO', 'hsl(214 100% 97%)'),
    brand: 'YB',
    vc_name: '装饰画',
    properties_value: '40x60cm / 双联装 / 浅色外框',
    enabled: true,
    cost_price: 42.4,
    sale_price: 129,
    weight: 1.2,
    sku_qty: 2,
    erp_created_at: addMillisecondsToNowISO(-5 * 24 * 60 * 60_000),
    modified_at: addMillisecondsToNowISO(-30 * 60_000),
    last_synced_at: addMillisecondsToNowISO(-20 * 60_000),
    children: [
      { record: records[0], quantity: 1 },
      { record: records[1], quantity: 1 },
    ],
  },
  {
    group_key: 'single:3',
    group_type: 'single',
    children: [{ record: records[2], quantity: 1 }],
  },
]

const LARGE_SURFACE_AUDIT_TOTAL = Number(import.meta.env.VITE_LARGE_SURFACE_TOTAL ?? 5000)
const LARGE_SURFACE_AUDIT_PAGE_SIZE = Number(import.meta.env.VITE_LARGE_SURFACE_PAGE_SIZE ?? 100)

function isLargeSurfaceAuditEnabled(): boolean {
  return import.meta.env.VITE_LARGE_SURFACE_AUDIT === 'true'
}

function largeSurfacePageSize(raw: unknown): number {
  const fallback = Number.isFinite(LARGE_SURFACE_AUDIT_PAGE_SIZE) ? LARGE_SURFACE_AUDIT_PAGE_SIZE : 100
  const candidate = Math.max(fallback, Number(raw ?? fallback))
  return Math.min(150, Math.max(80, Math.floor(candidate)))
}

function largeSurfaceProductRecords(q: Record<string, unknown>) {
  const page = Math.max(1, Number(q.page ?? 1))
  const pageSize = largeSurfacePageSize(q.page_size)
  const total = Number.isFinite(LARGE_SURFACE_AUDIT_TOTAL) ? LARGE_SURFACE_AUDIT_TOTAL : 5000
  const start = (page - 1) * pageSize
  const data = Array.from({ length: pageSize }, (_, index) => {
    const seq = start + index + 1
    return baseRecord({
      id: 700000 + seq,
      record_key: `load:record:${seq}`,
      task_id: 900000 + seq,
      task_no: `LT-${String(seq).padStart(5, '0')}`,
      sku_code: `SKU-LOAD-${String(seq).padStart(5, '0')}`,
      product_i_id: `STYLE-LOAD-${String(seq).padStart(5, '0')}`,
      erp_i_id: `ERP-LOAD-${String(seq).padStart(5, '0')}`,
      category_name: seq % 2 === 0 ? '装饰画' : '节日织物',
      product_family: seq % 2 === 0 ? '画布' : '挂旗',
      product_name: `承载审计产品 ${String(seq).padStart(5, '0')}`,
      cost_price: seq % 9 === 0 ? null : Number((18 + (seq % 37) * 0.6).toFixed(2)),
      cost_trace: seq % 9 === 0 ? null : {
        rule_name: '承载审计成本规则',
        rule_source: 'load_audit',
        matched_rule_version: 1,
        input_snapshot: { width: 40 + (seq % 5), height: 60 + (seq % 7), quantity: 1 },
        calculation_snapshot: { formula: 'mock', cost_price: Number((18 + (seq % 37) * 0.6).toFixed(2)) },
        snapshot_at: addMillisecondsToNowISO(-seq * 30_000),
      },
      creator_id: (seq % 5) + 1,
      creator_name: `承载用户 ${((seq % 5) + 1)}`,
      image_source: seq % 8 === 0 ? 'missing' : 'erp_product_image',
      image_source_label: seq % 8 === 0 ? '缺图' : '专项 ERP 商品图',
      image_preview_url: '',
      image_missing_reason: '承载审计不加载图片',
      image_asset_id: undefined,
      image_asset_version_id: undefined,
      base_sync_status: seq % 11 === 0 ? 'failed' : 'synced',
      image_sync_status: seq % 8 === 0 ? 'waiting_image' : 'synced',
      erp_sync_status: seq % 11 === 0 ? 'failed' : 'synced',
      base_sync_error: seq % 11 === 0 ? '承载审计同步异常样本' : undefined,
      last_base_synced_at: seq % 11 === 0 ? undefined : addMillisecondsToNowISO(-seq * 40_000),
      last_image_synced_at: seq % 8 === 0 ? undefined : addMillisecondsToNowISO(-seq * 40_000),
    })
  })
  const groups = data.map((record) => ({
    group_key: `combo:LOAD-${record.id}`,
    group_type: 'combo' as const,
    combo_sku_code: `COMBO-LOAD-${record.id}`,
    combo_name: `承载组合装 ${record.id}`,
    combo_short_name: '承载组合',
    erp_i_id: `ERP-COMBO-${record.id}`,
    entity_sku_id: `ENTITY-LOAD-${record.id}`,
    pic_url: '',
    brand: 'YB',
    vc_name: record.category_name,
    properties_value: '40x60cm / 承载审计 / 无图片加载',
    enabled: true,
    cost_price: record.cost_price,
    sale_price: 129,
    weight: 1.2,
    sku_qty: 1,
    erp_created_at: addMillisecondsToNowISO(-5 * 24 * 60 * 60_000),
    modified_at: addMillisecondsToNowISO(-seqSafe(record.id) * 30_000),
    last_synced_at: addMillisecondsToNowISO(-seqSafe(record.id) * 20_000),
    children: [{ record, quantity: 1 }],
  }))
  return { data, groups, page, pageSize, total }
}

function seqSafe(value: number): number {
  return Math.max(1, value - 700000)
}

const mockCostRules = [
  { rule_id: 1, rule_name: 'KT 板基础单价', rule_version: 3, category_code: 'KT_BOARD', product_family: 'KT 板', rule_type: 'fixed_unit_price', base_price: 24, tax_multiplier: 1, priority: 100, is_active: true },
  { rule_id: 2, rule_name: 'KT 板最低计价面积', rule_version: 2, category_code: 'KT_BOARD', product_family: 'KT 板', rule_type: 'minimum_billable_area', min_area: 0.25, priority: 90, is_active: true },
  { rule_id: 3, rule_name: '写真固定单价', rule_version: 5, category_code: 'PHOTO_PRINT', product_family: '写真', rule_type: 'fixed_unit_price', base_price: 18, priority: 100, is_active: true },
]
const mockCostRuns = [{ id: 1, run_no: 'CR-MOCK-001', status: 'previewed', mode: 'all_matching', created_at: addMillisecondsToNowISO(-30 * 60_000), summary: { total_count: 3, previewed_count: 3, applied_count: 0, erp_synced_count: 0, conflict_count: 0 } }]

function loadAuditCostRules() {
  if (!isLargeSurfaceAuditEnabled()) return mockCostRules
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

export const productManagementHandler: MockHandler = (request) => {
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
  if (request.method === 'GET' && request.path === '/v1/product-management/cost-dashboard') return { status: 200, data: { data: { total_count: records.length, total_records: records.length, groups: [], tags: [{ code: 'erp_mismatch', label: 'ERP 差异', count: 1 }] } } }
  if (request.method === 'GET' && request.path === '/v1/product-management/cost-recalculation-runs') return { status: 200, data: { data: mockCostRuns } }
  const costRunMatch = request.path.match(/^\/v1\/product-management\/cost-recalculation-runs\/(\d+)(?:\/(apply|sync-erp))?$/)
  if (costRunMatch && request.method === 'GET') return { status: 200, data: { data: { ...mockCostRuns[0], items: [{ id: 1, run_id: 1, product_management_record_id: 1, task_no: 'T-20260423-1001', sku_code: 'YB-MOCK-001', old_cost_price: 18.6, new_cost_price: 24, status: 'previewed' }] } } }
  if (costRunMatch && request.method === 'POST' && costRunMatch[2] === 'apply') return { status: 200, data: { run: { ...mockCostRuns[0], status: 'applied' } } }
  if (costRunMatch && request.method === 'POST' && costRunMatch[2] === 'sync-erp') return { status: 200, data: { run: { ...mockCostRuns[0], status: 'erp_syncing' } } }
  if (request.method === 'POST' && request.path === '/v1/product-management/cost-recalculation-runs') return { status: 201, data: { data: mockCostRuns[0] } }
  if (request.method === 'GET' && request.path === '/v1/product-management/combo-tree') {
    if (isLargeSurfaceAuditEnabled()) {
      const audit = largeSurfaceProductRecords(request.query as Record<string, unknown>)
      return {
        status: 200,
        data: {
          data: audit.data,
          pagination: {
            page: audit.page,
            page_size: audit.pageSize,
            total: audit.total,
          },
          groups: audit.groups,
          combo_sync_summary: {
            id: 700001,
            window_begin: addMillisecondsToNowISO(-60 * 60_000),
            window_end: addMillisecondsToNowISO(-10 * 60_000),
            page_index: audit.page,
            page_size: audit.pageSize,
            status: 'success',
            last_success_at: addMillisecondsToNowISO(-10 * 60_000),
            processed_items: audit.data.length,
          },
        },
      }
    }
    return {
      status: 200,
      data: {
        data: records,
        pagination: {
          page: Number(request.query.page ?? 1),
          page_size: Number(request.query.page_size ?? 20),
          total: records.length,
        },
        groups,
        combo_sync_summary: {
          id: 1,
          window_begin: addMillisecondsToNowISO(-60 * 60_000),
          window_end: addMillisecondsToNowISO(-10 * 60_000),
          page_index: 1,
          page_size: 100,
          status: 'success',
          last_success_at: addMillisecondsToNowISO(-10 * 60_000),
          processed_items: 3,
        },
      },
    }
  }

  if (request.method === 'GET' && request.path === '/v1/product-management') {
    return {
      status: 200,
      data: {
        data: records,
        pagination: {
          page: Number(request.query.page ?? 1),
          page_size: Number(request.query.page_size ?? 20),
          total: records.length,
        },
      },
    }
  }

  const candidatesMatch = request.path.match(/^\/v1\/product-management\/(\d+)\/image-candidates$/)
  if (request.method === 'GET' && candidatesMatch) {
    const record = records.find((item) => item.id === Number(candidatesMatch[1]))
    return {
      status: 200,
      data: {
        data: record
          ? [
              {
                asset_id: 101,
                asset_version_id: 1001,
                task_id: record.task_id,
                task_no: record.task_no,
                sku_code: record.sku_code,
                source: 'task_reference',
                source_label: '任务参考图',
                preview_url: productImage('ALT', 'hsl(48 96% 89%)'),
                file_name: 'mock-candidate.png',
                mime_type: 'image/png',
                created_at: addMillisecondsToNowISO(-40 * 60_000),
              },
            ]
          : [],
      },
    }
  }

  const updateMatch = request.path.match(/^\/v1\/product-management\/(\d+)\/(reparse-image|image|sync-request|base-sync-request|image-sync-request)$/)
  if ((request.method === 'POST' || request.method === 'PATCH') && updateMatch) {
    const record = records.find((item) => item.id === Number(updateMatch[1]))
    if (!record) return { status: 404, data: { message: 'product record not found' } }
    return { status: 200, data: { data: record } }
  }

  return null
}
