/**
 * 从 usersApi 获取 Designer 角色用户，供创建任务、指派、筛选等场景使用。
 * v0.5 对齐：FRONTEND_ALIGNMENT_v0.5.md 第 D 节
 * 使用 GET /v1/users/designers，不要从通用用户列表过滤 Designer。
 *
 * canonical 响应 envelope：{ data: [{id, username, display_name}], pagination: {...} }
 * 本 composable 解包时对裸数组与 { data, pagination } 两种形态都兼容。
 *
 * @param includeEmpty 是否包含空选项（用于"不限"/"请选择"）
 * @param emptyLabel 空选项的文案，默认「请选择设计师」，筛选场景可传「全部」
 * @param autoLoad 是否在 onMounted 自动拉取，默认 true。按需加载场景（弹窗打开时才拉取）传 false。
 */
import { ref, onMounted, unref } from 'vue'
import type { Ref } from 'vue'
import { usersApi, type DesignersLane } from '@/services/api/usersApi'
import type { BaseSelectOption } from '@/components/base/BaseSelect.vue'
import type { Designer } from '@/mock/designers'
import { usePermissionsStore } from '@/stores/permissions'

/**
 * 后端 `GET /v1/users/designers` 的守卫白名单（v1.5/v1.6，见 docs/V0_9_MODEL_HANDOFF_MANIFEST.md）。
 * 其他角色（CustomizationOperator / DepartmentAdmin / Auditor / Warehouse / Member 等）
 * 请求必 403；前端在发起前先短路，避免 Network 面板每次路由切换都红一片。
 */
const DESIGNERS_ALLOWED_ROLES = [
  'Ops',
  'Designer',
  'Admin',
  'HRAdmin',
  'SuperAdmin',
] as const

export type UseDesignerOptionsOpts = {
  /** 空选项（「不限」/「请选择」）开关 */
  includeEmpty?: boolean
  /** 空选项文案 */
  emptyLabel?: string
  /** 是否 onMounted 自动拉取；弹窗按需加载场景传 false */
  autoLoad?: boolean
  /**
   * 可选更严格门禁：调用方声明「只有持有以下任一 action 才需要设计师下拉」。
   * 与角色白名单是 AND 关系；未传时仅走角色短路。
   */
  requiredActions?: readonly string[]
  /**
   * 可选更严格门禁：调用方声明「只有能访问以下任一 page 才需要设计师下拉」。
   * 与角色白名单是 AND 关系；未传时仅走角色短路。
   */
  requiredPages?: readonly string[]
  /**
   * 工作流泳道（N-F-1）。
   * - undefined / 'normal'：请求 `/v1/users/designers`，与迭代前完全一致。
   * - 'customization' / 'all'：追加 `?workflow_lane=<value>`，返回对应泳道人员。
   * 支持传 Ref，便于将来在同一下拉随表单 lane 切换。
   */
  workflowLane?: DesignersLane | Ref<DesignersLane | undefined>
}

export function useDesignerOptions(
  includeEmptyOrOpts: boolean | UseDesignerOptionsOpts = false,
  emptyLabel = '请选择设计师',
  autoLoad = true,
) {
  const opts: UseDesignerOptionsOpts =
    typeof includeEmptyOrOpts === 'object' && includeEmptyOrOpts !== null
      ? includeEmptyOrOpts
      : { includeEmpty: includeEmptyOrOpts, emptyLabel, autoLoad }
  const includeEmpty = opts.includeEmpty ?? false
  const resolvedEmptyLabel = opts.emptyLabel ?? '请选择设计师'
  const resolvedAutoLoad = opts.autoLoad ?? true
  const requiredActions = opts.requiredActions
  const requiredPages = opts.requiredPages
  const workflowLaneOpt = opts.workflowLane

  const permissionsStore = usePermissionsStore()
  const assigneeOptions = ref<BaseSelectOption[]>(
    includeEmpty ? [{ value: '', label: resolvedEmptyLabel }] : [],
  )
  const designers = ref<Designer[]>([])
  const loading = ref(false)
  const loadError = ref('')

  function canLoad(): boolean {
    if (!permissionsStore.hasAnyRole(DESIGNERS_ALLOWED_ROLES)) return false
    if (requiredActions?.length && !requiredActions.some((a) => permissionsStore.hasAction(a))) {
      return false
    }
    if (requiredPages?.length && !requiredPages.some((p) => permissionsStore.hasPage(p))) {
      return false
    }
    return true
  }

  async function loadDesigners() {
    // 无权访问 designers 路由的角色直接跳过请求，返回空下拉
    if (!canLoad()) {
      designers.value = []
      assigneeOptions.value = includeEmpty ? [{ value: '', label: resolvedEmptyLabel }] : []
      loadError.value = ''
      loading.value = false
      return
    }
    loading.value = true
    loadError.value = ''
    try {
      const lane = unref(workflowLaneOpt)
      const res = await usersApi.getDesigners(lane ? { workflowLane: lane } : undefined)
      const data = res?.data
      const body = data?.data ?? data
      const list = Array.isArray(body) ? body : []
      const mapped = list.map((raw: Record<string, unknown>) => {
        const id = String(raw.id ?? '')
        const name = String(raw.display_name ?? raw.username ?? '')
        return { id, name }
      })
      designers.value = mapped.map((m) => ({
        id: m.id,
        name: m.name,
        role: 'designer' as const,
      }))
      const selectOptions = mapped.map((m) => ({ value: m.id, label: m.name }))
      assigneeOptions.value = includeEmpty
        ? [{ value: '', label: resolvedEmptyLabel }, ...selectOptions]
        : selectOptions
    } catch {
      loadError.value = '加载设计师列表失败，请稍后重试'
      designers.value = []
      assigneeOptions.value = includeEmpty ? [{ value: '', label: resolvedEmptyLabel }] : []
    } finally {
      loading.value = false
    }
  }

  if (resolvedAutoLoad) onMounted(loadDesigners)

  return { assigneeOptions, designers, loadDesigners, loadError, loading }
}
