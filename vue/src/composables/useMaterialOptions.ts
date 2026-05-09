/**
 * 产品材质选项
 * - GET /v1/categories?category_type=material（分页拉全量）
 * - 合并 GET /v1/category-mappings?secondary_condition_key=material（补充关联表里的名称）
 * - 若筛选结果仍为空，再拉未筛选 categories 后在客户端筛 category_type=material（兼容后端忽略 query）
 * - 最后本地 FALLBACK_MATERIALS
 */
import { ref, onMounted } from 'vue'
import { categoriesApi, type Category } from '@/services/api/categoriesApi'
import type { BaseSelectOption } from '@/components/base/BaseSelect.vue'

const FALLBACK_MATERIALS: BaseSelectOption[] = [
  { value: '', label: '请选择材质' },
  { value: 'COTTON', label: '棉' },
  { value: 'POLYESTER', label: '涤纶' },
  { value: 'LINEN', label: '麻' },
  { value: 'SILK', label: '丝' },
  { value: 'LEATHER', label: '皮革' },
  { value: 'METAL', label: '金属' },
  { value: 'PLASTIC', label: '塑料' },
  { value: 'PAPER', label: '纸' },
  { value: 'GLASS', label: '玻璃' },
  { value: 'WOOD', label: '木质' },
  { value: 'OTHER', label: '其他' },
]

const PAGE_SIZE = 200
const MAX_PAGES = 30

/** axios 成功响应 → 业务 body */
function unwrapResponseBody(res: unknown): unknown {
  if (res === null || typeof res !== 'object') return res
  const r = res as { data?: unknown; status?: number; config?: unknown }
  if (typeof r.status === 'number' && 'config' in r && 'data' in r) {
    return r.data
  }
  return res
}

/** 从常见 envelope 中取出数组 */
function extractArray(body: unknown): unknown[] {
  if (Array.isArray(body)) return body
  if (!body || typeof body !== 'object') return []
  const o = body as Record<string, unknown>
  const candidates = [o.data, o.items, o.results, o.records, o.rows, o.list, o.categories]
  for (const c of candidates) {
    if (Array.isArray(c)) return c
    if (c && typeof c === 'object' && !Array.isArray(c)) {
      const nested = c as Record<string, unknown>
      if (Array.isArray(nested.data)) return nested.data
      if (Array.isArray(nested.items)) return nested.items
    }
  }
  return []
}

function paginationMeta(body: unknown): { total?: number; page?: number; page_size?: number } | undefined {
  if (!body || typeof body !== 'object') return undefined
  const p = (body as { pagination?: { total?: number; page?: number; page_size?: number } }).pagination
  return p && typeof p === 'object' ? p : undefined
}

type CategoryRow = Category & {
  category_id?: number
  categoryId?: number
  categoryCode?: string
  categoryName?: string
  displayName?: string
}

function rowCategoryValue(c: CategoryRow): string {
  const code = String(c.category_code ?? c.categoryCode ?? '').trim()
  if (code) return code
  const id = c.category_id ?? c.categoryId ?? c.id
  if (id != null) return String(id)
  return ''
}

function rowCategoryLabel(c: CategoryRow, value: string): string {
  const label = String(
    c.display_name ?? c.displayName ?? c.category_name ?? c.categoryName ?? c.category_code ?? c.categoryCode ?? value,
  ).trim()
  return label || value
}

function mapCategoryRowsToOptions(rows: CategoryRow[]): BaseSelectOption[] {
  const seen = new Set<string>()
  const out: BaseSelectOption[] = []
  for (const c of rows) {
    const value = rowCategoryValue(c)
    if (!value || seen.has(value)) continue
    seen.add(value)
    out.push({ value, label: rowCategoryLabel(c, value) })
  }
  return out
}

function mapMappingsToOptions(rows: Array<{ secondary_condition_value?: string }>): BaseSelectOption[] {
  const seen = new Set<string>()
  const out: BaseSelectOption[] = []
  for (const m of rows) {
    const val = String(m.secondary_condition_value ?? '').trim()
    if (!val || seen.has(val)) continue
    seen.add(val)
    out.push({ value: val, label: val })
  }
  return out
}

async function fetchAllCategoryPages(
  params: { category_type?: string; is_active: boolean },
): Promise<CategoryRow[]> {
  const acc: CategoryRow[] = []
  for (let page = 1; page <= MAX_PAGES; page += 1) {
    const res = await categoriesApi.list({
      ...params,
      page,
      page_size: PAGE_SIZE,
    })
    const body = unwrapResponseBody(res)
    const chunk = extractArray(body) as CategoryRow[]
    acc.push(...chunk)
    const meta = paginationMeta(body)
    if (chunk.length < PAGE_SIZE) break
    if (meta?.total != null && acc.length >= meta.total) break
    if (chunk.length === 0) break
  }
  return acc
}

async function fetchAllMappingPages(): Promise<Array<{ secondary_condition_value?: string }>> {
  const acc: Array<{ secondary_condition_value?: string }> = []
  for (let page = 1; page <= MAX_PAGES; page += 1) {
    const res = await categoriesApi.listMappings({
      secondary_condition_key: 'material',
      is_active: true,
      page,
      page_size: PAGE_SIZE,
    })
    const body = unwrapResponseBody(res)
    const chunk = extractArray(body) as Array<{ secondary_condition_value?: string }>
    acc.push(...chunk)
    const meta = paginationMeta(body)
    if (chunk.length < PAGE_SIZE) break
    if (meta?.total != null && acc.length >= meta.total) break
    if (chunk.length === 0) break
  }
  return acc
}

function isMaterialTypeRow(c: CategoryRow): boolean {
  const t = String(c.category_type ?? (c as { categoryType?: string }).categoryType ?? '').toLowerCase()
  return t === 'material'
}

/** 合并：优先保留先出现的 label（categories 在前） */
function mergeMaterialOptions(
  primary: BaseSelectOption[],
  extra: BaseSelectOption[],
): BaseSelectOption[] {
  const seen = new Set<string>()
  const out: BaseSelectOption[] = []
  for (const o of primary) {
    const key = String(o.value ?? '')
    if (!key || seen.has(key)) continue
    seen.add(key)
    out.push({ ...o, value: key })
  }
  for (const o of extra) {
    const key = String(o.value ?? '')
    if (!key || seen.has(key)) continue
    seen.add(key)
    out.push({ ...o, value: key })
  }
  return out
}

export function useMaterialOptions() {
  const options = ref<BaseSelectOption[]>(FALLBACK_MATERIALS)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function load() {
    loading.value = true
    error.value = null
    try {
      // 1) 明确按 category_type=material 分页拉取
      let catRows = await fetchAllCategoryPages({ category_type: 'material', is_active: true })
      let catOpts = mapCategoryRowsToOptions(catRows)

      // 2) 仍为空：可能后端未识别 query，拉全部分类再在客户端筛 material
      if (catOpts.length === 0) {
        const allRows = await fetchAllCategoryPages({ is_active: true })
        catOpts = mapCategoryRowsToOptions(allRows.filter(isMaterialTypeRow))
      }

      // 3) 映射表分页合并（补充仅存在于 mappings 的材质名，如中文展示名）
      const mappingRows = await fetchAllMappingPages()
      const mappingOpts = mapMappingsToOptions(mappingRows)

      const merged = mergeMaterialOptions(catOpts, mappingOpts)

      if (merged.length > 0) {
        options.value = [{ value: '', label: '请选择材质' }, ...merged]
        return
      }

      options.value = FALLBACK_MATERIALS
    } catch (e) {
      error.value = e instanceof Error ? e.message : '加载材质失败'
      options.value = FALLBACK_MATERIALS
    } finally {
      loading.value = false
    }
  }

  onMounted(load)

  return { options, loading, error, load }
}
