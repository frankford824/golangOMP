import { erpApi } from '@/services/api/erpApi'
import type { ErpIidOption } from '@/domain/erp-iid-presets'

const PAGE_SIZE = 200

function iidKey(value: string): string {
  return value.trim().toLowerCase()
}

/** Both selection and submission use the ERP replica, never the mock/local presets. */
export async function fetchErpIidOptions(keyword = '', signal?: AbortSignal): Promise<ErpIidOption[]> {
  const items = new Map<string, ErpIidOption>()
  for (let page = 1; ; page += 1) {
    const response = await erpApi.getIids({ q: keyword.trim(), page, page_size: PAGE_SIZE }, signal)
    if (signal?.aborted) throw new Error('ERP 款式查询已取消')
    const payload = response.data
    if (!payload || !Array.isArray(payload.data)) throw new Error('ERP 款式查询返回格式异常')
    const rows: Array<Record<string, unknown>> = payload.data
    for (const row of rows) {
      if (!row || typeof row.i_id !== 'string' || !row.i_id.trim()) continue
      const i_id = row.i_id.trim()
      const category = String(row.category ?? '').trim()
      const category_name = String(row.category_name ?? category).trim()
      items.set(iidKey(i_id), { i_id, label: i_id, category, category_name })
    }
    const total = Number(payload.pagination?.total)
    const pageSize = Number(payload.pagination?.page_size ?? PAGE_SIZE)
    if (!Number.isFinite(total)) {
      // A full page without pagination is not proof of completeness.
      if (rows.length >= PAGE_SIZE) throw new Error('ERP 款式查询缺少分页信息')
      return [...items.values()]
    }
    if (!Number.isInteger(total) || total < 0 || !Number.isInteger(pageSize) || pageSize <= 0) {
      throw new Error('ERP 款式查询分页信息异常')
    }
    const expectedRows = Math.min(pageSize, Math.max(0, total - (page - 1) * pageSize))
    if (rows.length < expectedRows) throw new Error('ERP 款式查询结果不完整，请重试')
    if (page * pageSize >= total) return [...items.values()]
  }
}

export async function isValidErpIid(iid: string): Promise<boolean> {
  if (!iid.trim()) return false
  const options = await fetchErpIidOptions(iid)
  return options.some((item) => iidKey(item.i_id) === iidKey(iid))
}
