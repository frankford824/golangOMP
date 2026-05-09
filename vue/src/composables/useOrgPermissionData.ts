import { ref } from 'vue'
import { usersApi } from '@/services/api/usersApi'
import {
  departmentsAndGroupsFromOrgOptions,
  fetchOrgOwnershipOptions,
} from '@/services/api/orgApi'
import type { OrgOwnershipOptionsParsed } from '@/services/api/orgApi'
import { usePermissionsStore } from '@/stores/permissions'
import type { OrgUser } from '@/domain/types/org-membership'
import { userDepartmentLabel, userTeamLabel } from '@/domain/org-membership'

function extractListPayload(data: unknown): unknown[] {
  const body = (data as { data?: unknown })?.data ?? data
  if (Array.isArray(body)) return body
  if (body && typeof body === 'object') {
    const o = body as Record<string, unknown>
    if (Array.isArray(o.items)) return o.items
    if (Array.isArray(o.users)) return o.users
  }
  return []
}

export function mapRawToOrgUser(raw: Record<string, unknown>): OrgUser {
  return {
    id: String(raw.id ?? ''),
    username: typeof raw.username === 'string' ? raw.username : undefined,
    display_name: (raw.display_name ?? raw.displayName) as string | undefined,
    displayName: raw.display_name as string | undefined,
    real_name: raw.name as string | undefined,
    department: raw.department as OrgUser['department'],
    team: (raw.team ?? raw.group) as string | undefined,
    group: raw.group as string | undefined,
    roles: Array.isArray(raw.roles) ? (raw.roles as string[]) : undefined,
    joined_at: raw.joined_at as string | undefined,
    created_at: raw.created_at as string | undefined,
    status: raw.status as string | undefined,
  }
}

/**
 * 更新用户部门/小组。
 *
 * v1.8 Round I I-D 审计确认：`PATCH /v1/users/{id}` 为加性更新 —— 未传字段保持原值，
 * 因此前端不再需要先 GET 详情拼 display_name / status。此函数仅提交真正要变更的字段。
 *
 * Round I.f 热修复：后端 `validateDepartment("")` 返回 400 "department is required"，
 * 因此常规归属变更必须同时携带非空 department + team。移出到未分配池的语义改由
 * 独立的 `clearUserMembership` 经由 `team: 'ungrouped'` 别名表达。
 */
export async function patchUserMembership(
  userId: string,
  department: string,
  team: string,
): Promise<void> {
  const payload: Record<string, unknown> = {
    department: department.trim(),
    team: team.trim(),
  }
  await usersApi.patch(userId, payload)
}

/**
 * 将用户移出到未分配池。
 *
 * 后端 `service/identity_service.go` 对 `team === "ungrouped"`（`userTeamUngroupedAlias`）
 * 做了特判：会把 department 覆盖为 DepartmentUnassigned 并解析未分配池 team，
 * 此路径下 department 字段在 PATCH body 中是可选的，因此前端只提交 `{ team: 'ungrouped' }`。
 *
 * 注意：后端 `authorizeUserUpdate` 仅对 `identityActorCanManageAllUsers`
 * （Admin / SuperAdmin / HRAdmin）放行此操作，DepartmentAdmin 调用会得到 403
 * `deny_code=department_scope_only`。因此按钮可见性必须用 `user.manage` 门控。
 */
export async function clearUserMembership(userId: string): Promise<void> {
  await usersApi.patch(userId, { team: 'ungrouped' })
}

export function useOrgPermissionData() {
  const users = ref<OrgUser[]>([])
  const orgOptions = ref<OrgOwnershipOptionsParsed | null>(null)
  const loading = ref(false)
  const error = ref('')

  async function load() {
    loading.value = true
    error.value = ''
    try {
      const [userRes, org] = await Promise.all([
        usersApi.list({ page: 1, page_size: 500 }),
        fetchOrgOwnershipOptions(),
      ])
      const hasOrg = org.departmentOptions.length > 0 || org.teamOptions.length > 0
      orgOptions.value = hasOrg ? org : null
      const hydrated = departmentsAndGroupsFromOrgOptions(org)
      if (hydrated) {
        usePermissionsStore().hydrateOrgFromServer(hydrated.departments, hydrated.groups)
      }
      const list = extractListPayload(userRes?.data)
      users.value = list
        .filter((x): x is Record<string, unknown> => x != null && typeof x === 'object')
        .map(mapRawToOrgUser)
    } catch (e) {
      error.value = e instanceof Error ? e.message : '加载失败'
    } finally {
      loading.value = false
    }
  }

  /** 本地更新一条用户归属（与 PATCH 成功后列表一致） */
  function applyLocalMembership(userId: string, department: string, team: string) {
    const i = users.value.findIndex((u) => u.id === userId)
    if (i === -1) return
    const next = { ...users.value[i], department, team, group: team }
    users.value = users.value.map((u, idx) => (idx === i ? next : u))
  }

  return {
    users,
    orgOptions,
    loading,
    error,
    load,
    patchUserMembership,
    clearUserMembership,
    applyLocalMembership,
    userDepartmentLabel,
    userTeamLabel,
  }
}
