import type { MockHandler } from './types'
import { ERP_IID_PRESETS } from '@/domain/erp-iid-presets'

export const erpBridgeHandler: MockHandler = (request) => {
  if (request.method === 'GET' && request.path === '/v1/erp/iids') {
    const q = String(request.query.q ?? request.query.keyword ?? '').trim().toLowerCase()
    const pageRaw = Number(request.query.page ?? 1)
    const pageSizeRaw = Number(request.query.page_size ?? 50)
    const page = Number.isFinite(pageRaw) && pageRaw > 0 ? Math.floor(pageRaw) : 1
    const page_size = Number.isFinite(pageSizeRaw) && pageSizeRaw > 0 ? Math.floor(pageSizeRaw) : 50
    const filtered = ERP_IID_PRESETS.filter((item) => {
      if (!q) return true
      return (
        item.i_id.toLowerCase().includes(q) ||
        item.label.toLowerCase().includes(q) ||
        item.category.toLowerCase().includes(q) ||
        item.category_name.toLowerCase().includes(q)
      )
    })
    const start = (page - 1) * page_size
    const paged = filtered.slice(start, start + page_size).map((item, idx) => ({
      ...item,
      product_count: filtered.length - idx,
    }))
    return {
      status: 200,
      data: {
        data: paged,
        pagination: {
          page,
          page_size,
          total: filtered.length,
        },
        normalized_filters: {
          q: q || undefined,
          page,
          page_size,
        },
      },
    }
  }

  if (request.method === 'GET' && request.path === '/v1/erp/products') {
    const keyword = String(request.query.keyword ?? request.query.q ?? '').trim()
    const items = ['SKU-1001', 'SKU-1002', 'NP-1001']
      .filter((code) => !keyword || code.toLowerCase().includes(keyword.toLowerCase()))
      .map((code) => ({ id: code, sku_code: code, product_name: `ERP商品 ${code}` }))
    return { status: 200, data: { items, total: items.length } }
  }

  const productDetail = request.path.match(/^\/v1\/erp\/products\/([^/]+)$/)
  if (request.method === 'GET' && productDetail) {
    const id = productDetail[1] ?? ''
    return { status: 200, data: { data: { id, sku_code: id, product_name: `ERP商品 ${id}` } } }
  }

  if (request.method === 'GET' && request.path === '/v1/erp/products/by-code') {
    const code = String(request.query.code ?? request.query.sku_code ?? '')
    return {
      status: 200,
      data: {
        items: code
          ? [
              {
                product_id: `P-${code}`,
                sku_code: code,
                product_name: `ERP商品 ${code}`,
              },
            ]
          : [],
      },
    }
  }
  if (request.method === 'GET' && request.path === '/v1/erp/categories') {
    return { status: 200, data: { items: [{ code: 'CAT-A', name: '演示类目' }] } }
  }
  if (request.method === 'GET' && request.path === '/v1/erp/warehouses') {
    return { status: 200, data: { items: [{ wms_co_id: 'WMS-1', name: '演示仓' }] } }
  }
  if (request.method === 'GET' && request.path === '/v1/erp/users') {
    return { status: 200, data: { items: [{ id: 'erp-user-1', name: 'ERP用户' }] } }
  }
  if (request.method === 'POST' && request.path === '/v1/erp/products/upsert') {
    return { status: 200, data: { data: { ...request.body, upserted: true } } }
  }
  if (request.method === 'POST' && request.path === '/v1/erp/products/style/update') {
    return { status: 200, data: { success: true, data: request.body } }
  }
  if (request.method === 'POST' && request.path === '/v1/erp/inventory/virtual-qty') {
    return { status: 200, data: { success: true, data: request.body } }
  }
  if (request.method === 'POST' && request.path === '/v1/erp/products/shelve/batch') {
    return { status: 200, data: { data: { ok: true, items: request.body } } }
  }
  if (request.method === 'POST' && request.path === '/v1/erp/products/unshelve/batch') {
    return { status: 200, data: { data: { ok: true, items: request.body } } }
  }
  if (request.method === 'GET' && request.path === '/v1/erp/sync-logs') {
    return { status: 200, data: { items: [], total: 0 } }
  }
  return null
}
