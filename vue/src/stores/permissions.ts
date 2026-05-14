import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type {
  PermissionUser,
  PermissionEnumValue,
  RoleEnumValue,
  DataScopeEnumValue,
  Department,
  Group,
} from '@/types'
import { RoleEnum, DataScopeEnum, PermissionEnum } from '@/types'
import { authApi } from '@/services/api/authApi'
import { setToken, clearToken } from '@/services/http'
import { useNotificationsStore } from '@/stores/notifications.store'
import { useTasksStore } from '@/stores/tasks'
import type { BackendUser, FrontendAccess, LoginResponse } from '@/services/apiTypes'

const BACKEND_MENU_TO_SHELL_KEY: Record<string, string> = {
  user_admin: 'user_manage',
  org_admin: 'org_permission',
}

export function resolveShellMenuKeys(backendMenus: string[]): string[] {
  return backendMenus.map((menu) => BACKEND_MENU_TO_SHELL_KEY[menu] ?? menu)
}

type AuthMePayload =
  | BackendUser
  | { data?: BackendUser | { user?: BackendUser; frontend_access?: FrontendAccess } | { frontend_access?: FrontendAccess } }
  | { user?: BackendUser }

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
 * 的映射（例如后端放在 `task.*` 下，前端历史上归在 `design:*` 下）。
 *
 * 新增后端 action 时若前端 `can(...)` 仍沿用旧命名，向本表追加一行即可。
 */
const ACTION_SEMANTIC_ALIAS: Record<string, string[]> = {
  'task.asset_upload': ['design:upload'],
  'task.design_submit': ['design:submit'],
}

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
  const key = String(raw ?? '').trim()
  if (!key) return ''
  if (!key.includes(':')) return key
  const dotted = key.replace(/:/g, '.')
  if (import.meta.env.DEV) {
    // eslint-disable-next-line no-console
    console.warn(
      `[permissions] colon-form action key detected: "${key}" → "${dotted}". ` +
        `Please migrate to dot-notation (see V1.8 frontend alignment).`,
    )
  }
  return dotted
}

/**
 * 分隔符 & 语义规范化：
 *
 * 后端 frontend_access.actions 采用点号（如 `task.create`、`department.manage`），
 * 前端 PermissionEnum / `can()` 历史上使用冒号（如 `task:create`）。两边约定不一致
 * 曾导致 DepartmentAdmin / Ops / Designer / Warehouse 等非 SuperAdmin 角色下发的
 * actions 在 Object.values(PermissionEnum) 过滤时被全部丢弃，`can('task:create')`
 * 永远为 false（见后端交接文档 · 问题 2）。
 *
 * 这里不改 PermissionEnum 也不改后端，而是在 `actions.value` 里对每个键做一次
 * 「冒号↔点号」双向展开，使下面两种写法恒等命中：
 * 1) 后端原始 key（点号 / 冒号 / 下划线形式按后端实际下发）；
 * 2) 其 `.` ↔ `:` 变体，用于历史 `task:create` / 新 `task.create` 两种 `can()` 调用；
 * 3) `ACTION_SEMANTIC_ALIAS` 里声明的跨命名空间等价 key（例如
 *    `task.asset_upload` → `design:upload`），别名自己同样参与双向展开，
 *    以避免 `can('design.upload')`（点号）仅因别名只给了冒号形式而丢失命中。
 */
function mergeBackendActionAliases(keys: string[]): string[] {
  const out = new Set<string>()
  const addBothForms = (key: string) => {
    if (!key) return
    out.add(key)
    if (key.includes('.')) {
      const colon = key.replace(/\./g, ':')
      if (colon !== key) out.add(colon)
    }
    if (key.includes(':')) {
      const dotted = key.replace(/:/g, '.')
      if (dotted !== key) out.add(dotted)
    }
  }
  for (const raw of keys) {
    const key = String(raw ?? '').trim()
    if (!key) continue
    addBothForms(key)
    for (const alias of ACTION_SEMANTIC_ALIAS[key] ?? []) {
      addBothForms(alias.trim())
    }
  }
  return Array.from(out)
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

const PAGE_KEY_COMPAT_ALIASES: Record<string, string[]> = {
  dashboard: ['dashboard_home', 'task_board'],
  dashboard_home: ['dashboard', 'task_board'],
  task_board: ['dashboard', 'dashboard_home'],
  org_permission: ['org_options'],
  org_options: ['org_permission'],
  user_manage: ['department_users', 'team_users', 'admin_users', 'admin_roles'],
  department_users: ['user_manage', 'team_users', 'admin_users'],
  team_users: ['user_manage', 'department_users'],
  admin_users: ['user_manage'],
  admin_roles: ['user_manage'],
  warehouse: ['warehouse_receive', 'warehouse_processing'],
  warehouse_receive: ['warehouse', 'warehouse_processing'],
  warehouse_processing: ['warehouse', 'warehouse_receive'],
  task_list: ['my_tasks'],
  my_tasks: ['task_list'],
  design_workbench: ['design_submit', 'design_rework', 'design_workspace'],
  design_submit: ['design_workbench'],
  design_rework: ['design_workbench'],
  design_workspace: ['design_workbench'],
  audit_workbench: ['audit_workspace'],
  audit_workspace: ['audit_workbench'],
  logs_manage: ['admin_permission_logs', 'admin_operation_logs'],
  admin_permission_logs: ['logs_manage'],
  admin_operation_logs: ['logs_manage'],
}

/**
 * 菜单键单向兼容映射：
 *
 * 后端 frontend_access.menus 已统一使用 `user_admin / org_admin / logs_center /
 * design_workspace / audit_workspace` 等规范键（见后端交接文档 · 问题 3）。
 * 前端 MENU_CONFIG 也已同步改名，后端新键是 source-of-truth。
 *
 * 此别名表只做「老前端键 → 后端新键」的单向展开，用于兜住：
 * - 仍有组件/代码按老键调用 `hasMenu('org_permission')` 等；
 * - 外部集成或 e2e 脚本里的历史键。
 *
 * 反向展开（新键 → 老键）已移除：MENU_CONFIG 现在直接使用新键，不再需要回写老键。
 * 对仍有多种拆分的菜单（warehouse 的收货/加工子页）保留本来的同级别名。
 */
// v1.8 Round I（F-10）：删除 `org_permission` / `user_manage` 菜单键别名。
// `AppShell.vue` 侧边栏已改用规范键 `org_admin` / `user_admin` 并在 MENU_CONFIG
// 里保留 `aliases: ['org_permission' | 'user_manage']` 承担最后一层前端兜底；
// 因此 store 层不再重复做同一映射，避免 menus 数组里出现重复键。
const MENU_KEY_COMPAT_ALIASES: Record<string, string[]> = {
  audit_workbench: ['audit_workspace'],
  design_workbench: ['design_workspace'],
  logs_manage: ['logs_center'],
  audit_queue: ['audit_workbench', 'audit_workspace'],
  warehouse: ['warehouse_receive', 'warehouse_processing'],
  warehouse_receive: ['warehouse', 'warehouse_processing'],
  warehouse_processing: ['warehouse', 'warehouse_receive'],
  dashboard: ['task_board'],
  task_board: ['dashboard'],
}

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
  // Round I.g：任务创建「所属部门/所属组」下拉前端门禁需要的 actor scope。
  // 来源为 frontend_access.managed_departments / managed_teams，
  // 用于 `useActorOwnerScope` 计算 `allowedCreateOwnerDepartments` /
  // `allowedCreateOwnerTeams`，以防 DA 把 owner_department 指向 DataScope 外的部门
  // 触发 403 PERMISSION_DENIED / task_out_of_department_scope。
  const managedDepartments = ref<string[]>([])
  const managedTeams = ref<string[]>([])
  const actorDepartment = ref<string>('')
  const actorTeam = ref<string>('')

  // display-only, NOT an authorization gate —— 仅用于角色徽章、列表摘要等 UI 展示；
  // 鉴权必须走 hasPermission / hasAction / frontend_access。
  const isSuperAdmin = computed(() => currentUser.value?.role === RoleEnum.SUPER_ADMIN)
  const isHRAdmin = computed(() => currentUser.value?.role === RoleEnum.HR_ADMIN)
  const isDeptAdmin = computed(() => currentUser.value?.role === RoleEnum.DEPT_ADMIN)
  const isGroupLeader = computed(() => currentUser.value?.role === RoleEnum.GROUP_LEADER)
  const isCustomizationOperator = computed(() => currentUser.value?.role === RoleEnum.CUSTOMIZATION_OPERATOR)
  // 职位角色便捷判断（基于权限集，而非角色字面量，使层级角色也能透明兼容）。
  // display-only, NOT an authorization gate —— 这些派生 computed 供角色徽章/文案分流。
  const isOps = computed(() => hasPermission(PermissionEnum.TASK_ASSIGN) && !isSuperAdmin.value && !isHRAdmin.value)
  const isDesigner = computed(
    () =>
      !!currentUser.value?.permissions.includes(PermissionEnum.DESIGN_WORK) &&
      !hasPermission(PermissionEnum.TASK_AUDIT),
  )
  const isAuditA = computed(() => currentUser.value?.role === RoleEnum.AUDIT_A)
  const isAuditB = computed(() => currentUser.value?.role === RoleEnum.AUDIT_B)
  const isAuditor = computed(
    () => isAuditA.value || isAuditB.value ||
      (hasPermission(PermissionEnum.TASK_AUDIT) && !isSuperAdmin.value && !isHRAdmin.value && !isDeptAdmin.value),
  )
  const isWarehouseUser = computed(
    () =>
      hasPermission(PermissionEnum.TASK_WAREHOUSE) &&
      !isSuperAdmin.value &&
      !isHRAdmin.value &&
      !isDeptAdmin.value &&
      !isGroupLeader.value,
  )
  const dataScope = computed<DataScopeEnumValue | null>(
    () => currentUser.value?.dataScope ?? null,
  )

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

    try {
      await restoreSession()
    } catch (error) {
      clearToken()
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
    // /v1/auth/me canonical user fields are id + display_name; keep alias fallback only for read-compat.
    // 某些环境（如 v1.4 后端）只返回 `username`，这里兜底避免 AppShell 顶栏空名。
    applyFrontendAccess(
      String(user.id ?? user.user_id ?? ''),
      String(user.display_name ?? user.displayName ?? user.username ?? ''),
      normalizeFrontendAccess(access),
    )
    void useNotificationsStore().load().catch(() => undefined)
  }

  /**
   * 将后端 frontend_access 映射到前端 PermissionUser 结构
   * 映射规则：backend actions[] ↔ 前端 PermissionEnum values（字符串直接对应）
   */
  function applyFrontendAccess(
    userId: string,
    displayName: string,
    access: FrontendAccess,
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
    } else if (hasRole(RoleEnum.WAREHOUSE)) {
      role = RoleEnum.WAREHOUSE
    } else if (hasRole(RoleEnum.AUDIT_A) || hasRole('auditor')) {
      role = RoleEnum.AUDIT_A
    } else if (hasRole(RoleEnum.AUDIT_B) || hasRole('customizationreviewer')) {
      role = RoleEnum.AUDIT_B
    } else if (hasRole(RoleEnum.DESIGNER)) {
      role = RoleEnum.DESIGNER
    } else if (hasRole(RoleEnum.CUSTOMIZATION_OPERATOR) || hasRole('customizationoperator')) {
      role = RoleEnum.CUSTOMIZATION_OPERATOR
    } else if (hasRole(RoleEnum.OPS)) {
      role = RoleEnum.OPS
    } else if (permissions.includes(PermissionEnum.TASK_WAREHOUSE)) {
      role = RoleEnum.WAREHOUSE
    } else if (permissions.includes(PermissionEnum.TASK_AUDIT)) {
      role = RoleEnum.AUDIT_A
    } else if (permissions.includes(PermissionEnum.DESIGN_WORK)) {
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

    // 根据角色推断数据范围；若后端明确下发 view_all，则以前端运行时字段优先。
    const scopeMap: Partial<Record<string, DataScopeEnumValue>> & Record<RoleEnumValue, DataScopeEnumValue> = {
      [RoleEnum.SUPER_ADMIN]: DataScopeEnum.GLOBAL,
      [RoleEnum.HR_ADMIN]: DataScopeEnum.GLOBAL,
      [RoleEnum.DEPT_ADMIN]: DataScopeEnum.DEPARTMENT,
      [RoleEnum.GROUP_LEADER]: DataScopeEnum.DEPARTMENT,
      [RoleEnum.WAREHOUSE]: DataScopeEnum.GLOBAL,
      [RoleEnum.AUDIT_A]: DataScopeEnum.DEPARTMENT,
      [RoleEnum.AUDIT_B]: DataScopeEnum.DEPARTMENT,
      [RoleEnum.AUDITOR]: DataScopeEnum.DEPARTMENT,
      [RoleEnum.OPS]: DataScopeEnum.GROUP,
      [RoleEnum.DESIGNER]: DataScopeEnum.SELF,
      [RoleEnum.CUSTOMIZATION_OPERATOR]: DataScopeEnum.SELF,
      [RoleEnum.MEMBER]: DataScopeEnum.SELF,
    }

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
      name: displayName,
      role,
      departmentId,
      groupId,
      dataScope: access.view_all ? DataScopeEnum.GLOBAL : (scopeFromAccess ?? scopeMap[role]),
      permissions: (access.is_super_admin || role === RoleEnum.HR_ADMIN) ? Object.values(PermissionEnum) : permissions,
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
    clearToken()
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

  // v1.8 Round I：删除 `SuperAdmin / HRAdmin` 角色名兜底。
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
    // 与 AppShell 侧栏一致：超级管理员在未单独下发 `report_center` 时仍可进入报表路由
    if (currentUser.value.role === RoleEnum.SUPER_ADMIN && key === 'report_center') {
      return true
    }
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

  /**
   * 角色白名单判断（大小写不敏感）。
   *
   * 专用于「后端路由直接按 PascalCase 角色名做守卫」的场景，前端在发请求前先短路，
   * 避免 CustomizationOperator / DepartmentAdmin / Member 等角色刷新页面时必然触发
   * 403（如 `/v1/users/designers`、`/v1/categories/search`）。
   *
   * 不替代 `hasPermission` / `hasAction`——那两个面向后端下发的 actions 动作键，
   * 本函数面向 `frontend_access.roles` 原始列表里的角色 code。
   */
  function hasAnyRole(codes: readonly string[]): boolean {
    if (!currentUser.value) return false
    if (!codes.length) return false
    const lowered = codes.map((c) => String(c).toLowerCase())
    return roles.value.some((r) => lowered.includes(String(r).toLowerCase()))
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

  /** v0.9：GET /v1/org/options 或组织变更后，用服务端 id/名称覆盖本地部门与小组（非空才写入） */
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
    isSuperAdmin,
    isHRAdmin,
    isDeptAdmin,
    isGroupLeader,
    isCustomizationOperator,
    isOps,
    isDesigner,
    isAuditA,
    isAuditB,
    isAuditor,
    isWarehouseUser,
    dataScope,
    loginWithCredentials,
    restoreSession,
    setCurrentUser,
    logout,
    hasPermission,
    hasMenu,
    hasPage,
    hasAction,
    hasModule,
    hasAnyRole,
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
