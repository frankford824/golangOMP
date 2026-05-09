/**
 * 产品分类编码选项（GET /v1/categories、GET /v1/categories/search）
 * 选项 value 为内部分类 code；提交 business-info 时请走 buildCategoryPatchFields
 * 分流到 category/category_code，而不是在页面里直接写死 category_code。
 */
import { ref, onMounted } from 'vue'
import { categoriesApi } from '@/services/api/categoriesApi'
import type { BaseSelectOption } from '@/components/base/BaseSelect.vue'
import { usePermissionsStore } from '@/stores/permissions'

/**
 * 后端 `GET /v1/categories/search` 的守卫白名单（Ops/Warehouse/Admin）。
 * 其他角色（CustomizationOperator/DepartmentAdmin/Member/Designer/Auditor 等）
 * 必定 403；前端在发起前先短路，保留空下拉即可。
 */
const CATEGORY_SEARCH_ALLOWED_ROLES = [
  'Ops',
  'Warehouse',
  'Admin',
  'HRAdmin',
  'SuperAdmin',
] as const

export type UseCategoryOptionsOpts = {
  /** false：不在挂载时请求，由调用方在首次需要时调用 load()（任务详情补全区等） */
  eager?: boolean
  /**
   * 可选更严格门禁：调用方声明「只有持有以下任一 action 才需要分类下拉」。
   * 与角色白名单是 AND 关系；未传时仅走角色短路。
   */
  requiredActions?: readonly string[]
  /**
   * 可选更严格门禁：调用方声明「只有能访问以下任一 page 才需要分类下拉」。
   * 与角色白名单是 AND 关系；未传时仅走角色短路。
   */
  requiredPages?: readonly string[]
}

export function useCategoryOptions(opts?: UseCategoryOptionsOpts) {
  const eager = opts?.eager !== false
  const requiredActions = opts?.requiredActions
  const requiredPages = opts?.requiredPages
  const permissionsStore = usePermissionsStore()
  const options = ref<BaseSelectOption[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  function canLoad(): boolean {
    if (!permissionsStore.hasAnyRole(CATEGORY_SEARCH_ALLOWED_ROLES)) return false
    if (requiredActions?.length && !requiredActions.some((a) => permissionsStore.hasAction(a))) {
      return false
    }
    if (requiredPages?.length && !requiredPages.some((p) => permissionsStore.hasPage(p))) {
      return false
    }
    return true
  }

  async function load() {
    if (!canLoad()) {
      options.value = [{ value: '', label: '请选择分类' }]
      error.value = null
      loading.value = false
      return
    }
    loading.value = true
    error.value = null
    try {
      const res = await categoriesApi.search({ is_active: true, limit: 100 })
      const body = (res as { data?: unknown })?.data ?? res
      const list = (Array.isArray(body) ? body : (body as { data?: unknown[] })?.data) ?? []
      const arr = Array.isArray(list) ? list : []
      options.value = [
        { value: '', label: '请选择分类' },
        ...arr.map((c: { id?: unknown; category_code?: string; category_name?: string; display_name?: string }) => ({
          value: c.category_code ?? String(c.id ?? ''),
          label: c.display_name ?? c.category_name ?? c.category_code ?? '-',
        })),
      ]
    } catch (e) {
      error.value = e instanceof Error ? e.message : '加载分类失败'
      options.value = [{ value: '', label: '请选择分类' }]
    } finally {
      loading.value = false
    }
  }

  if (eager) {
    onMounted(load)
  }

  return { options, loading, error, load }
}
