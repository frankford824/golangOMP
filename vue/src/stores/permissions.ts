import { defineStore } from 'pinia'
import { ref } from 'vue'
import type {
  PermissionUser,
  PermissionEnumValue,
  RoleEnumValue,
  Department,
  Group,
} from '@/types'
import { RoleEnum, DataScopeEnum, PermissionEnum } from '@/types'
import { authApi } from '@/services/api/authApi'
import { setToken, clearToken } from '@/services/http'
import { useNotificationsStore } from '@/stores/notifications.store'
import { useRealtimeStore } from '@/stores/realtime.store'
import { useTasksStore } from '@/stores/tasks'
import { useWebPushStore } from '@/stores/webPush.store'
import type { BackendUser, FrontendAccess, LoginResponse } from '@/services/apiTypes'

export function resolveShellMenuKeys(backendMenus: string[]): string[] {
  return normalizeUniqueKeys(backendMenus)
}

type AuthMePayload =
  | BackendUser
  | { data?: BackendUser | { user?: BackendUser; frontend_access?: FrontendAccess } | { frontend_access?: FrontendAccess } }
  | { user?: BackendUser }

interface CurrentUserProfileMeta {
  account?: string
  username?: string
  avatar?: string
  avatar_url?: string
}

function normalizeUniqueKeys(keys: unknown): string[] {
  if (!Array.isArray(keys)) return []
  const out = new Set<string>()
  for (const raw of keys) {
    const key = String(raw ?? '').trim()
    if (key) out.add(key)
  }
  return Array.from(out)
}

/**
 * 语义别名（后端动作 key → 前端 PermissionEnum key）。
 *
 * 仅用于后端「命名空间」与前端 PermissionEnum 不对齐的真实漂移点。
 * 一般的 `.` ↔ `:` 分隔符差异由下方通用规则处理，这里只列需要跨命名空间
 * 的映射（例如后端放在 `task.*` 下，前端历史上归在 `design.*` 下）。
 *
 * 新增后端 action 时若前端 `can(...)` 仍沿用旧命名，向本表追加一行即可。
 */
/**
 * 规范化调用方传入的 action key：
 *
 * v1.8 起，后端 `frontend_access.actions` 全部采用点号（`task.create`、
 * `role.assign`、`department.users.disable` 等）；前端统一向点号对齐。
 *
 * 本函数做两件事：
 * 1) 冒号 → 点号：容忍历史 `can('task:create')` 调用，避免一次性全量改写时遗漏；
 * 2) 开发态警告：若在 DEV 模式捕获到冒号形式，抛出 `console.warn`，方便回归期
 *    发现残留的 colon-form token 调用点；生产态保持静默，以免打扰用户。
 */
export function normalizeActionKey(raw: unknown): string {
  return String(raw ?? '').trim()
}

/**
 * 分隔符 & 语义规范化：
 *
 * 后端 frontend_access.actions 采用点号（如 `task.create`、`department.manage`），
 * 前端 PermissionEnum / `can()` 历史上使用冒号（如 `task:create`）。两边约定不一致
 * 曾导致非 SuperAdmin 身份下发的
 * actions 在 Object.values(PermissionEnum) 过滤时被全部丢弃，`can('task:create')`
 * 永远为 false（见后端交接文档 · 问题 2）。
 *
 * 这里不改 PermissionEnum 也不改后端，而是在 `actions.value` 里对每个键做一次
 * 「冒号↔点号」双向展开，使下面两种写法恒等命中：
 * 1) 后端原始 key（点号 / 冒号 / 下划线形式按后端实际下发）；
 * 2) 其 `.` ↔ `:` 变体，用于历史 `task:create` / 新 `task.create` 两种 `can()` 调用；
 * 3) `ACTION_SEMANTIC_ALIAS` 里声明的跨命名空间等价 key（例如
 *    `task.asset_upload` → `design.upload`），别名自己同样参与双向展开，
 *    以避免 `can('design.upload')`（点号）仅因别名只给了冒号形式而丢失命中。
 */
function mergeBackendActionAliases(keys: string[]): string[] {
  return normalizeUniqueKeys(keys)
}

function normalizeFrontendAccess(access: FrontendAccess): FrontendAccess {
  return {
    ...access,
    menus: normalizeUniqueKeys([...(access.menus ?? []), ...(access.menu_keys ?? [])]),
    pages: normalizeUniqueKeys([...(access.pages ?? []), ...(access.page_keys ?? [])]),
    actions: normalizeUniqueKeys([...(access.actions ?? []), ...(access.permission_flags ?? [])]),
    modules: normalizeUniqueKeys([...(access.modules ?? []), ...(access.module_keys ?? [])]),
    scopes: normalizeUniqueKeys([...(access.scopes ?? []), ...(access.access_scopes ?? [])]),
    roles: normalizeUniqueKeys(access.roles ?? []),
  }
}

const PAGE_KEY_COMPAT_ALIASES: Record<string, string[]> = {}

/**
 * 菜单键单向兼容映射：
 *
 * 后端 frontend_access.menus 已统一使用 `user_admin / org_admin /
 * design_workspace / audit_workspace` 等规范键。
 * 前端 MENU_CONFIG 也已同步改名，后端新键是 source-of-truth。
 *
 * 此别名表只做「老前端键 → 后端新键」的单向展开，用于兜住：
 * - 仍有组件/代码按老键调用 `hasMenu('org_permission')` 等；
 * - 外部集成或 e2e 脚本里的历史键。
 *
 * 反向展开（新键 → 老键）已移除：MENU_CONFIG 现在直接使用新键，不再需要回写老键。
 */
// 菜单键只接受当前后端合同，不展开旧名称别名。
// `AppShell.vue` 侧边栏已改用规范键 `org_admin` / `user_admin` 并在 MENU_CONFIG
// 里保留 `aliases: ['org_permission' | 'user_manage']` 承担最后一层前端兜底；
// 因此 store 层不再重复做同一映射，避免 menus 数组里出现重复键。
const MENU_KEY_COMPAT_ALIASES: Record<string, string[]> = {}

function normalizeAccessKeys(
  keys: string[] | undefined,
  aliasMap: Record<string, string[]>,
): string[] {
  if (!keys?.length) return []
  const out = new Set<string>()
  for (const raw of keys) {
    const key = String(raw ?? '').trim()
    if (!key) continue
    out.add(key)
    for (const alias of aliasMap[key] ?? []) {
      const normalized = alias.trim()
      if (normalized) out.add(normalized)
    }
  }
  return Array.from(out)
}

export const usePermissionsStore = defineStore('permissions', () => {
  const currentUser = ref<PermissionUser | null>(null)
  const departments = ref<Department[]>([])
  const groups = ref<Group[]>([])
  // 后端 menus 控制侧边栏显隐（符合 v0.4 文档 §4）
  const menus = ref<string[]>([])
  const pages = ref<string[]>([])
  const actions = ref<string[]>([])
  const modules = ref<string[]>([])
  const scopes = ref<string[]>([])
  const roles = ref<string[]>([])
  // 组织名称与管理范围仅用于个人中心展示。任务数据范围由后端
  // EffectiveAccess + 稳定 department/team ID 决定，前端不再按角色或名称推断。
  const managedDepartments = ref<string[]>([])
  const managedTeams = ref<string[]>([])
  const actorDepartment = ref<string>('')
  const actorTeam = ref<string>('')

  /**
   * 真实后端登录（用于生产/联调场景）
   * 后端返回格式：{ data: { user, session: { token } } } 或 { user, token }
   */
  async function loginWithCredentials(username: string, password: string): Promise<void> {
    const res = await authApi.login({ username, password })
    const raw = res.data as (
      | LoginResponse
      | { data?: LoginResponse & { session?: { token?: string } } }
      | (LoginResponse & { session?: { token?: string } })
      | undefined
    )
    const payload = (
      raw && typeof raw === 'object' && 'data' in raw ? raw.data : raw
    ) as (LoginResponse & { session?: { token?: string } }) | undefined
    const token = payload?.session?.token ?? payload?.token
    if (!token) throw new Error('登录响应缺少 token')
    setToken(token)

    const loginUser = payload?.user
    const loginAccess = loginUser?.frontend_access ?? payload?.frontend_access
    if (loginUser && loginAccess) {
      applyAuthenticatedUser(loginUser, loginAccess)
      startAuthenticatedServices()
      return
    }

    try {
      await restoreSession()
    } catch (error) {
      // A transient 429/5xx from /auth/me must not discard the token that the
      // successful login response just issued. Only an actual 401 invalidates it.
      if (typeof error === 'object' && error !== null && 'status' in error && error.status === 401) {
        clearToken()
      }
      throw error
    }
  }

  /**
   * 从 localStorage token 恢复会话（路由守卫初始化调用）
   * 若 token 无效，后端返回 401，http 拦截器会自动清除 token 并跳转登录
   * 兼容 /me 返回 { data: user } 或直接 user
   */
  async function restoreSession(): Promise<void> {
    const res = await authApi.me()
    const snapshot = extractCurrentUserFromAuthMe(res.data as AuthMePayload | undefined)
    const user = snapshot?.user ?? null
    if (!user) throw new Error('会话恢复失败：缺少用户信息')
    const access = snapshot?.frontendAccess ?? user.frontend_access
    if (!access) throw new Error('会话恢复失败：缺少 frontend_access')
    applyAuthenticatedUser(user, access)
    startAuthenticatedServices()
  }

  function applyAuthenticatedUser(user: BackendUser, access: FrontendAccess): void {
    // /v1/auth/me canonical user fields are id + display_name; keep alias fallback only for read-compat.
    // 某些环境（如 v1.4 后端）只返回 `username`，这里兜底避免 AppShell 顶栏空名。
    const rawUser = user as BackendUser & { user_id?: string; displayName?: string }
    applyFrontendAccess(
      String(rawUser.id ?? rawUser.user_id ?? ''),
      String(rawUser.display_name ?? rawUser.displayName ?? rawUser.name ?? rawUser.username ?? ''),
      normalizeFrontendAccess(access),
      {
        account: rawUser.account,
        username: rawUser.username,
        avatar: rawUser.avatar,
        avatar_url: rawUser.avatar_url,
      },
    )
  }

  function startAuthenticatedServices(): void {
    void authApi.refreshAssetCookie().catch(() => undefined)
    void useNotificationsStore().load().catch(() => undefined)
    void useWebPushStore().initAfterLogin().catch(() => undefined)
    useRealtimeStore().start()
  }

  /**
   * 将后端 frontend_access 映射到前端 PermissionUser 结构
   * 映射规则：backend actions[] ↔ 前端 PermissionEnum values（字符串直接对应）
   */
  function applyFrontendAccess(
    userId: string,
    displayName: string,
    access: FrontendAccess,
    profile: CurrentUserProfileMeta = {},
  ): void {
    const rawActions = normalizeUniqueKeys(access.actions ?? [])
    const allActions = mergeBackendActionAliases(rawActions)
    const permissions = allActions.filter(
      (a): a is PermissionEnumValue =>
        Object.values(PermissionEnum).includes(a as PermissionEnumValue),
    )

    const normalizedRoles = normalizeUniqueKeys(access.roles ?? []).map((item) => item.toLowerCase())
    const hasRole = (code: string) => normalizedRoles.includes(code)

    // 根据 frontend_access.roles 与 actions 推断主角色
    // 优先级：super_admin > hr_admin > group_leader > dept_admin > 职位角色
    let role: RoleEnumValue = RoleEnum.MEMBER
    if (access.is_super_admin || hasRole('super_admin') || hasRole('superadmin')) {
      role = RoleEnum.SUPER_ADMIN
    } else if (hasRole('hr_admin') || hasRole('hradmin')) {
      role = RoleEnum.HR_ADMIN
    } else if (access.is_group_leader || hasRole('group_leader') || hasRole('teamlead')) {
      role = RoleEnum.GROUP_LEADER
    } else if (access.is_department_admin || hasRole('dept_admin') || hasRole('departmentadmin')) {
      role = RoleEnum.DEPT_ADMIN
    } else if (hasRole(RoleEnum.AUDITOR)) {
      role = RoleEnum.AUDITOR
    } else if (hasRole(RoleEnum.DESIGNER)) {
      role = RoleEnum.DESIGNER
    } else if (hasRole(RoleEnum.CUSTOMIZATION_OPERATOR) || hasRole('customizationoperator')) {
      role = RoleEnum.CUSTOMIZATION_OPERATOR
    } else if (hasRole(RoleEnum.OPS)) {
      role = RoleEnum.OPS
    } else if (permissions.includes(PermissionEnum.TASK_AUDIT)) {
      role = RoleEnum.AUDITOR
    } else if (permissions.includes(PermissionEnum.TASK_DESIGN_SUBMIT)) {
      role = RoleEnum.DESIGNER
    } else if (permissions.includes(PermissionEnum.TASK_ASSIGN)) {
      role = RoleEnum.OPS
    }

    const departmentId =
      access.department_codes?.find((v) => typeof v === 'string' && v.trim() !== '') ??
      access.managed_departments?.find((v) => typeof v === 'string' && v.trim() !== '') ??
      access.department ??
      ''
    const groupId =
      access.team_codes?.find((v) => typeof v === 'string' && v.trim() !== '') ??
      access.managed_teams?.find((v) => typeof v === 'string' && v.trim() !== '') ??
      access.team ??
      ''

    const scopeAliases = new Set(
      normalizeUniqueKeys(access.scopes ?? []).map((item) => item.toLowerCase()),
    )
    const scopeFromAccess =
      scopeAliases.has('global') || scopeAliases.has('all') || scopeAliases.has('all_data')
        ? DataScopeEnum.GLOBAL
        : scopeAliases.has('department') || scopeAliases.has('dept')
          ? DataScopeEnum.DEPARTMENT
          : scopeAliases.has('group') || scopeAliases.has('team')
            ? DataScopeEnum.GROUP
            : scopeAliases.has('self')
              ? DataScopeEnum.SELF
              : undefined

    currentUser.value = {
      id: userId,
      account: String(profile.account ?? profile.username ?? '').trim() || undefined,
      username: String(profile.username ?? profile.account ?? '').trim() || undefined,
      name: displayName,
      avatar: String(profile.avatar ?? profile.avatar_url ?? '').trim() || undefined,
      avatarUrl: String(profile.avatar_url ?? profile.avatar ?? '').trim() || undefined,
      role,
      departmentId,
      groupId,
      dataScope: access.view_all ? DataScopeEnum.GLOBAL : (scopeFromAccess ?? DataScopeEnum.SELF),
      permissions: access.is_super_admin ? Object.values(PermissionEnum) : permissions,
    }
    // 存储后端返回的 frontend_access 字段，并兼容新旧 key 命名。
    menus.value = normalizeAccessKeys(access.menus, MENU_KEY_COMPAT_ALIASES)
    pages.value = normalizeAccessKeys(access.pages, PAGE_KEY_COMPAT_ALIASES)
    actions.value = allActions
    modules.value = normalizeUniqueKeys(access.modules ?? [])
    scopes.value = normalizeUniqueKeys(access.scopes ?? [])
    roles.value = normalizeUniqueKeys(access.roles ?? [])
    managedDepartments.value = normalizeUniqueKeys([
      ...(access.managed_departments ?? []),
      ...(access.department_codes ?? []),
    ])
    managedTeams.value = normalizeUniqueKeys([
      ...(access.managed_teams ?? []),
      ...(access.team_codes ?? []),
    ])
    actorDepartment.value = String(access.department ?? '').trim()
    actorTeam.value = String(access.team ?? '').trim()
  }

  function setCurrentUser(user: PermissionUser) {
    currentUser.value = user
  }

  function logout() {
    void useWebPushStore().cleanupOnLogout().catch(() => undefined)
    void authApi.logout().catch(() => undefined)
    clearToken()
    useRealtimeStore().stop()
    useNotificationsStore().reset()
    currentUser.value = null
    menus.value = []
    pages.value = []
    actions.value = []
    modules.value = []
    scopes.value = []
    roles.value = []
    managedDepartments.value = []
    managedTeams.value = []
    actorDepartment.value = ''
    actorTeam.value = ''
    useTasksStore().resetToInitialState()
  }

  // 权限必须完全依赖后端下发的显式能力。
  // 权限必须完全依赖后端下发的 `frontend_access.{pages|menus|actions|modules}`，
  // 不再在前端植入角色 → 所有权限 的硬编码短路；若某个页面对 HRAdmin 不可见，
  // 则后端 frontend_access 必须显式补齐对应 key，而不是靠前端兜底。
  function hasPermission(perms: PermissionEnumValue | PermissionEnumValue[]): boolean {
    if (!currentUser.value) return false
    const userPerms = currentUser.value.permissions
    const requiredList = Array.isArray(perms) ? perms : [perms]
    return requiredList.some(
      (p) => userPerms.includes(p) || actions.value.includes(p),
    )
  }

  /**
   * 检查用户是否有某菜单权限（由后端 frontend_access.menus 控制）
   * 符合 v0.4 文档 §4：menus 控制侧边栏/顶部菜单显隐
   */
  function hasMenu(key: string): boolean {
    if (!currentUser.value) return false
    if (menus.value.includes(key)) return true
    return false
  }

  function hasPage(key: string): boolean {
    if (!currentUser.value) return false
    return pages.value.includes(key)
  }

  function hasAction(key: string): boolean {
    if (!currentUser.value) return false
    return actions.value.includes(key)
  }

  function hasModule(key: string): boolean {
    if (!currentUser.value) return false
    return modules.value.includes(key)
  }

  function addDepartment(name: string): string | null {
    const trimmed = name.trim()
    if (!trimmed) return null
    const id = `dept-${Date.now()}-${departments.value.length + 1}`
    departments.value.push({ id, name: trimmed })
    return id
  }

  function addGroup(payload: { name: string; departmentId: string }): string | null {
    const trimmed = payload.name.trim()
    if (!trimmed) return null
    const exists = departments.value.some((d) => d.id === payload.departmentId)
    if (!exists) return null
    const id = `group-${Date.now()}-${groups.value.length + 1}`
    groups.value.push({ id, name: trimmed, departmentId: payload.departmentId })
    return id
  }

  function renameDepartment(id: string, name: string) {
    const trimmed = name.trim()
    if (!trimmed) return
    const i = departments.value.findIndex((d) => d.id === id)
    if (i === -1) return
    departments.value[i] = { ...departments.value[i], name: trimmed }
  }

  function renameGroup(id: string, name: string) {
    const trimmed = name.trim()
    if (!trimmed) return
    const i = groups.value.findIndex((g) => g.id === id)
    if (i === -1) return
    groups.value[i] = { ...groups.value[i], name: trimmed }
  }

  function moveGroupToDepartment(groupId: string, newDepartmentId: string) {
    const deptOk = departments.value.some((d) => d.id === newDepartmentId)
    if (!deptOk) return
    const i = groups.value.findIndex((g) => g.id === groupId)
    if (i === -1) return
    groups.value[i] = { ...groups.value[i], departmentId: newDepartmentId }
  }

  function removeDepartment(id: string) {
    departments.value = departments.value.filter((d) => d.id !== id)
    groups.value = groups.value.filter((g) => g.departmentId !== id)
  }

  function removeGroup(id: string) {
    groups.value = groups.value.filter((g) => g.id !== id)
  }

  /** GET /v1/org/options 或组织变更后，用服务端稳定 ID/名称覆盖本地部门与团队。 */
  function hydrateOrgFromServer(depts: Department[], grps: Group[]) {
    if (depts.length) departments.value = depts.map((d) => ({ ...d }))
    if (grps.length) groups.value = grps.map((g) => ({ ...g }))
  }

  /** 根据 groupId 查找所属 departmentId（用于 DEPARTMENT 数据范围判断） */
  function getGroupDepartmentId(groupId: string): string | null {
    return groups.value.find((g) => g.id === groupId)?.departmentId ?? null
  }

  function extractCurrentUserFromAuthMe(payload: AuthMePayload | undefined): {
    user: (BackendUser & { user_id?: string; displayName?: string }) | null
    frontendAccess: FrontendAccess | null
  } | null {
    if (!payload || typeof payload !== 'object') return null
    if ('data' in payload) {
      const data = payload.data
      if (!data || typeof data !== 'object') return null
      if ('user' in data) {
        const user = data.user
        const frontendAccess =
          (data as { frontend_access?: FrontendAccess }).frontend_access ??
          ((user as BackendUser | undefined)?.frontend_access ?? null)
        return {
          user:
            user && typeof user === 'object'
              ? (user as BackendUser & { user_id?: string; displayName?: string })
              : null,
          frontendAccess: frontendAccess ?? null,
        }
      }
      const rawUser = data as BackendUser & { user_id?: string; displayName?: string }
      return {
        user: rawUser,
        frontendAccess: (data as { frontend_access?: FrontendAccess }).frontend_access ?? rawUser.frontend_access ?? null,
      }
    }
    if ('user' in payload) {
      const user = payload.user
      const rawUser =
        user && typeof user === 'object'
          ? (user as BackendUser & { user_id?: string; displayName?: string })
          : null
      return {
        user: rawUser,
        frontendAccess: rawUser?.frontend_access ?? null,
      }
    }
    const rawUser = payload as BackendUser & { user_id?: string; displayName?: string }
    return {
      user: rawUser,
      frontendAccess: rawUser.frontend_access ?? null,
    }
  }

  return {
    currentUser,
    departments,
    groups,
    menus,
    pages,
    actions,
    modules,
    scopes,
    roles,
    managedDepartments,
    managedTeams,
    actorDepartment,
    actorTeam,
    loginWithCredentials,
    restoreSession,
    setCurrentUser,
    logout,
    hasPermission,
    hasMenu,
    hasPage,
    hasAction,
    hasModule,
    addDepartment,
    addGroup,
    renameDepartment,
    renameGroup,
    moveGroupToDepartment,
    removeDepartment,
    removeGroup,
    hydrateOrgFromServer,
    getGroupDepartmentId,
  }
})
