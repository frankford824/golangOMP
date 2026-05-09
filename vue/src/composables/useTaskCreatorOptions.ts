/**
 * 任务中心「创建人」筛选项。
 * - GET /v1/users 仅限具备用户/组织菜单的管理角色（与后端路由层一致），避免 Designer 等非管理账号触发 403。
 * - 其他角色仅从 GET /v1/tasks 列表推导已出现的创建人，足够筛选项需求。
 */
import { onMounted, ref } from 'vue'
import { usersApi } from '@/services/api/usersApi'
import { tasksApi } from '@/services/api/tasksApi'
import type { BaseSelectOption } from '@/components/base/BaseSelect.vue'
import { usePermission } from '@/composables/usePermission'

function mapUserOptions(list: unknown[]): BaseSelectOption[] {
  const selectOptions: BaseSelectOption[] = []
  for (const raw of list as Record<string, unknown>[]) {
    const id = String(raw.id ?? raw.user_id ?? raw.uid ?? '').trim()
    if (!id) continue
    const label = String(
      raw.display_name ??
      raw.displayName ??
      raw.real_name ??
      raw.realName ??
      raw.full_name ??
      raw.fullName ??
      raw.nickname ??
      raw.name ??
      raw.username ??
      id,
    ).trim()
    selectOptions.push({ value: id, label: label || id })
  }
  return selectOptions
}

function mapCreatorOptionsFromTasks(list: unknown[]): BaseSelectOption[] {
  const seen = new Set<string>()
  const out: BaseSelectOption[] = []
  for (const raw of list as Record<string, unknown>[]) {
    const creatorObj =
      raw.creator && typeof raw.creator === 'object'
        ? (raw.creator as Record<string, unknown>)
        : null
    const id = String(
      raw.creator_id ??
      raw.creatorId ??
      raw.created_by ??
      raw.createdBy ??
      creatorObj?.id ??
      creatorObj?.user_id ??
      '',
    ).trim()
    if (!id || seen.has(id)) continue
    seen.add(id)
    const label = String(
      raw.creator_name ??
      raw.creatorName ??
      raw.created_by_name ??
      raw.createdByName ??
      creatorObj?.display_name ??
      creatorObj?.displayName ??
      creatorObj?.real_name ??
      creatorObj?.realName ??
      creatorObj?.name ??
      raw.requester_name ??
      raw.requesterName ??
      id,
    ).trim()
    out.push({ value: id, label: label || id })
  }
  return out
}

function extractListFromEnvelope(payload: unknown): unknown[] {
  const body =
    payload && typeof payload === 'object' && 'data' in payload
      ? (payload as { data?: unknown }).data
      : payload
  if (Array.isArray(body)) return body
  if (body && typeof body === 'object') {
    const obj = body as {
      items?: unknown[]
      list?: unknown[]
      rows?: unknown[]
      results?: unknown[]
      tasks?: unknown[]
      users?: unknown[]
      data?: unknown[]
    }
    if (Array.isArray(obj.items)) return obj.items
    if (Array.isArray(obj.list)) return obj.list
    if (Array.isArray(obj.rows)) return obj.rows
    if (Array.isArray(obj.results)) return obj.results
    if (Array.isArray(obj.tasks)) return obj.tasks
    if (Array.isArray(obj.users)) return obj.users
    if (Array.isArray(obj.data)) return obj.data
  }
  return []
}

function dedupeOptions(options: BaseSelectOption[]): BaseSelectOption[] {
  const seen = new Set<string>()
  const out: BaseSelectOption[] = []
  for (const option of options) {
    const value = String(option.value ?? '').trim()
    if (!value || seen.has(value)) continue
    seen.add(value)
    out.push({ value, label: String(option.label ?? value).trim() || value })
  }
  return out
}

export function useTaskCreatorOptions(includeEmpty = true, emptyLabel = '全部') {
  const { canAccessMenu } = usePermission()

  const creatorOptions = ref<BaseSelectOption[]>(
    includeEmpty ? [{ value: '', label: emptyLabel }] : [],
  )
  const loadError = ref('')
  const loading = ref(false)

  function setOptions(options: BaseSelectOption[]) {
    creatorOptions.value = includeEmpty ? [{ value: '', label: emptyLabel }, ...options] : options
  }

  async function loadCreators() {
    loading.value = true
    loadError.value = ''
    try {
      const merged: BaseSelectOption[] = []

      const mayCallUserAdminList =
        canAccessMenu('org_admin') ||
        canAccessMenu('user_admin')
      // 与用户管理视图一致：仅管理侧拉全量用户目录；Designer 等非管理账号只靠任务列表推导
      if (mayCallUserAdminList) {
        try {
          const usersRes = await usersApi.list({
            page: 1,
            page_size: 1000,
            status: 'active',
          })
          merged.push(...mapUserOptions(extractListFromEnvelope(usersRes?.data)))
        } catch {
          // 管理列表失败时仍可依赖下方任务推导
        }
      }

      // 全员可用：任务列表中的真实创建人，保证「卡片上出现过的人」均可筛且不访问 /v1/users
      try {
        const tasksRes = await tasksApi.list({ page: 1, page_size: 1000, sort: '-updated_at' })
        merged.push(...mapCreatorOptionsFromTasks(extractListFromEnvelope(tasksRes?.data)))
      } catch {
        // ignore, 交给统一空结果处理
      }

      const options = dedupeOptions(merged)
      if (options.length === 0) {
        throw new Error('empty creator options')
      }
      setOptions(options)
    } catch {
      loadError.value = '加载用户候选失败，请稍后重试'
      creatorOptions.value = includeEmpty ? [{ value: '', label: emptyLabel }] : []
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    void loadCreators()
  })

  return { creatorOptions, loadCreators, loadError, loading }
}
