/**
 * Round I.g · D1：任务创建「所属部门 / 所属组」前端门禁
 *
 * 目的：
 *   后端 `TaskActionCreate.role_plus_owner_scope` 规则要求提交的
 *   `owner_department` / `owner_org_team` / `owner_team` 必须落在当前 actor
 *   的 DataScope 内。若 UI 允许 DA/GroupLeader 选到其他部门/组，后端会返回
 *   403 `PERMISSION_DENIED` / `task_out_of_department_scope`。
 *
 *   本 composable 基于 `permissionsStore` 中的 `managedDepartments /
 *   managedTeams / actorDepartment / actorTeam / roles` 计算一个白名单，
 *   下拉层与 submit-guard 均以此为准。
 *
 * 约束（UI 门禁，而非第二个后端）：
 *   - 只读；不改业务状态；仅控制 UI 显隐/可选项；
 *   - 规则变动必须与后端保持对齐（见 V0_9_BACKEND_SOURCE_OF_TRUTH 与
 *     docs/api-audit/01-task.md 的 create-task 章节）。
 */
import { computed } from 'vue'
import type { BaseSelectOption } from '@/components/base/BaseSelect.vue'
import { usePermissionsStore } from '@/stores/permissions'

/** 不需要做 owner_* 范围校验的角色（Global/HR 等拥有跨部门创建任务权限）。 */
const ROLES_WITH_UNRESTRICTED_OWNER_SCOPE: readonly string[] = [
  'super_admin',
  'superadmin',
  'hr_admin',
  'hradmin',
  'admin',
  'role_admin',
  'roleadmin',
]

/** 可锁定所属部门（DA 语义）—— 当前只允许挑自己管辖的部门。 */
const ROLES_WITH_DEPARTMENT_LOCK: readonly string[] = [
  'dept_admin',
  'departmentadmin',
  'department_admin',
]

/** 可锁定所属组（组长语义）—— 当前只允许挑自己管辖的组，所属部门被隐藏/锁定。 */
const ROLES_WITH_TEAM_LOCK: readonly string[] = [
  'group_leader',
  'teamlead',
  'team_lead',
]

function lowerSet(values: readonly string[]): Set<string> {
  return new Set(values.map((v) => String(v ?? '').trim().toLowerCase()).filter(Boolean))
}

function uniqueNonEmpty(values: Array<string | undefined | null>): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const raw of values) {
    const key = String(raw ?? '').trim()
    if (!key || seen.has(key)) continue
    seen.add(key)
    out.push(key)
  }
  return out
}

export interface ActorOwnerScope {
  /** 允许在创建任务时提交的 owner_department 集合；空集合表示不限制（Global/HR）。 */
  allowedCreateOwnerDepartments: readonly string[]
  /** 允许在创建任务时提交的 owner_org_team / owner_team 集合；空集合表示不限制。 */
  allowedCreateOwnerTeams: readonly string[]
  /** 若 actor 数据范围为 Global/HR 等，任何部门/组都放行。 */
  isOwnerScopeUnrestricted: boolean
  /** 若 actor 没有组织管理角色（Ops/Member/Designer 等），这两个字段应整体隐藏，由后端根据 actor 归属派生。 */
  hideOwnerFields: boolean
  /** DA：owner_department 锁死在 actor 所管部门。 */
  lockOwnerDepartment: boolean
  /** GroupLeader：owner_department 隐藏，owner_team 锁死在 actor 所管组。 */
  lockOwnerTeam: boolean
  /** 建议的默认值（loadInitial 时使用）。 */
  defaultOwnerDepartment: string
  /** 建议的默认 owner_org_team/owner_team。 */
  defaultOwnerTeam: string
}

/**
 * 当前 actor 的组织范围白名单。
 *
 * 返回的列表直接用于：
 *   1. 下拉过滤：`filterOwnerTeamOptions(groupOptions)`；
 *   2. 提交前 guard：`validateOwnerScope(payload)`；
 *   3. 错误消息文案：`buildOwnerScopeDenyMessage()`。
 */
export function useActorOwnerScope() {
  const permissionsStore = usePermissionsStore()

  const rolesLower = computed(() =>
    (permissionsStore.roles ?? []).map((r) => String(r ?? '').trim().toLowerCase()),
  )

  const unrestrictedRoleSet = lowerSet(ROLES_WITH_UNRESTRICTED_OWNER_SCOPE)
  const departmentLockRoleSet = lowerSet(ROLES_WITH_DEPARTMENT_LOCK)
  const teamLockRoleSet = lowerSet(ROLES_WITH_TEAM_LOCK)

  const isOwnerScopeUnrestricted = computed(() => {
    if (permissionsStore.isSuperAdmin || permissionsStore.isHRAdmin) return true
    return rolesLower.value.some((r) => unrestrictedRoleSet.has(r))
  })

  const lockOwnerDepartment = computed(() => {
    if (isOwnerScopeUnrestricted.value) return false
    if (permissionsStore.isDeptAdmin) return true
    return rolesLower.value.some((r) => departmentLockRoleSet.has(r))
  })

  const lockOwnerTeam = computed(() => {
    if (isOwnerScopeUnrestricted.value) return false
    if (permissionsStore.isGroupLeader) return true
    return rolesLower.value.some((r) => teamLockRoleSet.has(r))
  })

  /**
   * 是否完全隐藏两字段：既不是无限制角色，也不是 DA/GroupLeader，则
   * 不应在表单里出现所属部门/所属组；payload 也不带这两个字段，后端按
   * actor 归属派生（见 TaskActionCreate：缺省分支）。
   */
  const hideOwnerFields = computed(
    () => !isOwnerScopeUnrestricted.value && !lockOwnerDepartment.value && !lockOwnerTeam.value,
  )

  const allowedCreateOwnerDepartments = computed<readonly string[]>(() => {
    if (isOwnerScopeUnrestricted.value) return []
    if (lockOwnerDepartment.value) {
      return uniqueNonEmpty([
        ...(permissionsStore.managedDepartments ?? []),
        permissionsStore.actorDepartment,
        permissionsStore.currentUser?.departmentId,
      ])
    }
    if (lockOwnerTeam.value) {
      return uniqueNonEmpty([
        permissionsStore.actorDepartment,
        permissionsStore.currentUser?.departmentId,
      ])
    }
    return uniqueNonEmpty([permissionsStore.actorDepartment])
  })

  const allowedCreateOwnerTeams = computed<readonly string[]>(() => {
    if (isOwnerScopeUnrestricted.value) return []
    if (lockOwnerTeam.value) {
      return uniqueNonEmpty([
        ...(permissionsStore.managedTeams ?? []),
        permissionsStore.actorTeam,
        permissionsStore.currentUser?.groupId,
      ])
    }
    if (lockOwnerDepartment.value) {
      // DA 可在本部门下任选 team（具体 team 列表由 /v1/org/options 提供，
      // 组件内结合 teamDepartmentMap 进一步按部门筛选）；此白名单主要做
      // managed_teams 严格限制（若后端下发了 managed_teams，则优先该子集）。
      const managed = permissionsStore.managedTeams ?? []
      if (managed.length) {
        return uniqueNonEmpty([
          ...managed,
          permissionsStore.actorTeam,
          permissionsStore.currentUser?.groupId,
        ])
      }
      return []
    }
    return uniqueNonEmpty([
      permissionsStore.actorTeam,
      permissionsStore.currentUser?.groupId,
    ])
  })

  const defaultOwnerDepartment = computed(
    () =>
      allowedCreateOwnerDepartments.value[0] ??
      permissionsStore.actorDepartment ??
      permissionsStore.currentUser?.departmentId ??
      '',
  )

  const defaultOwnerTeam = computed(
    () =>
      allowedCreateOwnerTeams.value[0] ??
      permissionsStore.actorTeam ??
      permissionsStore.currentUser?.groupId ??
      '',
  )

  function filterOwnerTeamOptions(
    options: ReadonlyArray<BaseSelectOption>,
    resolveDepartmentForTeam?: (team: string) => string | undefined,
  ): BaseSelectOption[] {
    if (isOwnerScopeUnrestricted.value) return [...options]
    const allowedTeams = new Set(allowedCreateOwnerTeams.value)
    const allowedDepartments = new Set(allowedCreateOwnerDepartments.value)
    return options.filter((opt) => {
      const key = String(opt.value ?? '').trim()
      if (!key) return false
      if (allowedTeams.size > 0 && allowedTeams.has(key)) return true
      if (allowedTeams.size === 0 && allowedDepartments.size > 0 && resolveDepartmentForTeam) {
        const dept = resolveDepartmentForTeam(key)
        return dept != null && allowedDepartments.has(dept)
      }
      return false
    })
  }

  function filterOwnerDepartmentOptions(
    options: ReadonlyArray<BaseSelectOption>,
  ): BaseSelectOption[] {
    if (isOwnerScopeUnrestricted.value) return [...options]
    const allowed = new Set(allowedCreateOwnerDepartments.value)
    if (!allowed.size) return []
    return options.filter((opt) => {
      const key = String(opt.value ?? '').trim()
      return allowed.has(key)
    })
  }

  /**
   * submit 前兜底校验：即便 UI 下拉被绕过、或外部脚本塞入了 payload，也不会发请求。
   *
   * @returns 若通过校验返回 null；否则返回拒绝原因（供 toast 显示）。
   */
  function validateOwnerScope(payload: {
    owner_department?: unknown
    owner_org_team?: unknown
    owner_team?: unknown
  }): string | null {
    if (isOwnerScopeUnrestricted.value) return null
    const dept = typeof payload.owner_department === 'string' ? payload.owner_department.trim() : ''
    const team = typeof payload.owner_org_team === 'string'
      ? payload.owner_org_team.trim()
      : typeof payload.owner_team === 'string'
        ? payload.owner_team.trim()
        : ''
    const allowedDept = new Set(allowedCreateOwnerDepartments.value)
    const allowedTeam = new Set(allowedCreateOwnerTeams.value)

    if (dept && allowedDept.size > 0 && !allowedDept.has(dept)) {
      return `您无权在所选部门创建任务（当前仅可选：${[...allowedDept].join('、')}）`
    }
    if (team && allowedTeam.size > 0 && !allowedTeam.has(team)) {
      return `您无权在所选组创建任务（当前仅可选：${[...allowedTeam].join('、')}）`
    }
    // hideOwnerFields 情况下，payload 中不应出现 owner_* 字段；这里不阻断，
    // 交给 submit 前的 payload-builder 主动 strip 掉。
    return null
  }

  return {
    allowedCreateOwnerDepartments,
    allowedCreateOwnerTeams,
    isOwnerScopeUnrestricted,
    hideOwnerFields,
    lockOwnerDepartment,
    lockOwnerTeam,
    defaultOwnerDepartment,
    defaultOwnerTeam,
    filterOwnerTeamOptions,
    filterOwnerDepartmentOptions,
    validateOwnerScope,
  } as const
}

/**
 * 纯函数版本：脱离 Vue 响应式，直接基于 actor snapshot 计算白名单。
 * 供单测与 CLI 脚手架使用。
 */
export function computeAllowedCreateOwnerScope(snapshot: {
  isSuperAdmin: boolean
  isHRAdmin: boolean
  isDeptAdmin: boolean
  isGroupLeader: boolean
  roles: readonly string[]
  managedDepartments: readonly string[]
  managedTeams: readonly string[]
  actorDepartment: string
  actorTeam: string
}): Pick<
  ActorOwnerScope,
  | 'allowedCreateOwnerDepartments'
  | 'allowedCreateOwnerTeams'
  | 'isOwnerScopeUnrestricted'
  | 'hideOwnerFields'
  | 'lockOwnerDepartment'
  | 'lockOwnerTeam'
> {
  const rolesLower = snapshot.roles.map((r) => String(r ?? '').trim().toLowerCase())
  const unrestrictedRoleSet = lowerSet(ROLES_WITH_UNRESTRICTED_OWNER_SCOPE)
  const departmentLockRoleSet = lowerSet(ROLES_WITH_DEPARTMENT_LOCK)
  const teamLockRoleSet = lowerSet(ROLES_WITH_TEAM_LOCK)

  const isOwnerScopeUnrestricted =
    snapshot.isSuperAdmin ||
    snapshot.isHRAdmin ||
    rolesLower.some((r) => unrestrictedRoleSet.has(r))
  const lockOwnerDepartment =
    !isOwnerScopeUnrestricted &&
    (snapshot.isDeptAdmin || rolesLower.some((r) => departmentLockRoleSet.has(r)))
  const lockOwnerTeam =
    !isOwnerScopeUnrestricted &&
    (snapshot.isGroupLeader || rolesLower.some((r) => teamLockRoleSet.has(r)))
  const hideOwnerFields = !isOwnerScopeUnrestricted && !lockOwnerDepartment && !lockOwnerTeam

  let allowedCreateOwnerDepartments: readonly string[] = []
  let allowedCreateOwnerTeams: readonly string[] = []

  if (isOwnerScopeUnrestricted) {
    allowedCreateOwnerDepartments = []
    allowedCreateOwnerTeams = []
  } else if (lockOwnerDepartment) {
    allowedCreateOwnerDepartments = uniqueNonEmpty([
      ...snapshot.managedDepartments,
      snapshot.actorDepartment,
    ])
    allowedCreateOwnerTeams = snapshot.managedTeams.length
      ? uniqueNonEmpty([...snapshot.managedTeams, snapshot.actorTeam])
      : []
  } else if (lockOwnerTeam) {
    allowedCreateOwnerDepartments = uniqueNonEmpty([snapshot.actorDepartment])
    allowedCreateOwnerTeams = uniqueNonEmpty([...snapshot.managedTeams, snapshot.actorTeam])
  } else {
    allowedCreateOwnerDepartments = uniqueNonEmpty([snapshot.actorDepartment])
    allowedCreateOwnerTeams = uniqueNonEmpty([snapshot.actorTeam])
  }

  return {
    allowedCreateOwnerDepartments,
    allowedCreateOwnerTeams,
    isOwnerScopeUnrestricted,
    hideOwnerFields,
    lockOwnerDepartment,
    lockOwnerTeam,
  }
}
