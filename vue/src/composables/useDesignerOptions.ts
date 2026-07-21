/**
 * 从显式权限候选接口获取设计/审核人员，供创建、指派与交班使用。
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

const CANDIDATE_SELECTOR_ACTIONS = [
  'task.create',
  'task.assign',
  'task.reassign',
  'task.audit_handover',
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
   * 与候选接口能力门禁是 AND 关系。
   */
  requiredActions?: readonly string[]
  /**
   * 可选更严格门禁：调用方声明「只有能访问以下任一 page 才需要设计师下拉」。
   * 与候选接口能力门禁是 AND 关系。
   */
  requiredPages?: readonly string[]
  /**
   * 工作流泳道（N-F-1）。
   * - undefined / 'normal'：请求 `/v1/users/designers`，与迭代前完全一致。
   * - 'customization' / 'audit' / 'all'：追加 `?workflow_lane=<value>`，返回对应泳道人员。
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
    if (!CANDIDATE_SELECTOR_ACTIONS.some((action) => permissionsStore.hasAction(action))) return false
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
