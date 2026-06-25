import { computed } from 'vue'
import type { Task } from '@/domain/types/task'
import {
  DataScopeEnum,
  PermissionEnum,
  type DataScopeEnumValue,
  type PermissionEnumValue,
} from '@/types'
import { normalizeActionKey, usePermissionsStore } from '@/stores/permissions'

export function usePermission() {
  const store = usePermissionsStore()

  const currentUser = computed(() => store.currentUser)
  const dataScope = computed<DataScopeEnumValue | null>(() => store.dataScope)
  const frontendScopes = computed(() => store.scopes)
  const frontendRoles = computed(() => store.roles)

  /**
   * 能力判断：v1.8 起调用方统一使用点号（`task.create`、`role.assign` 等）。
   *
   * - `PermissionEnum` 里仍有历史冒号形式，走 `hasPermission`（其 `actions` 别名
   *   层已同时塞入点号与冒号两种 key，命中结果一致）；已迁移的点号枚举也同路处理；
   * - 其它字符串走 `hasAction`（包含 v1.8 新增的细粒度 action）；
   * - 冒号形式调用会被规范化为点号并在 DEV 态 `console.warn`，便于清理残留。
   */
  function can(
    perm: PermissionEnumValue | PermissionEnumValue[] | string | string[],
  ): boolean {
    const list = (Array.isArray(perm) ? perm : [perm]) as string[]
    const enumSet = new Set<string>(Object.values(PermissionEnum) as string[])
    return list.some((raw) => {
      const dotted = normalizeActionKey(raw)
      if (!dotted) return false
      // PermissionEnum 历史值使用冒号形式（如 'task:audit'），而 normalizeActionKey
      // 统一把调用方的 key 规整为点号。此处同时按冒号/点号两种形态查 PermissionEnum，
      // 保证带有 SuperAdmin/HRAdmin 全量兜底的 hasPermission 分支不会被短路遮住。
      const colon = dotted.replace(/\./g, ':')
      if (enumSet.has(colon)) {
        return store.hasPermission(colon as PermissionEnumValue)
      }
      if (enumSet.has(dotted)) {
        return store.hasPermission(dotted as PermissionEnumValue)
      }
      return store.hasAction(dotted)
    })
  }

  function canAccessMenu(key: string): boolean {
    return store.hasMenu(key)
  }

  function canAccessPage(key: string): boolean {
    return store.hasPage(key)
  }

  function canAccessAction(key: string): boolean {
    return store.hasAction(key)
  }

  function canAccessModule(key: string): boolean {
    return store.hasModule(key)
  }

  /**
   * 当后端未下发 `modules`（空数组）时不拦截；下发后须命中模块键。
   * 用于页面内模块块显隐（与侧栏 `menus` 解耦时的补充开关）。
   */
  // v1.8 Round I：删除 SuperAdmin / HRAdmin 角色名兜底；是否放行完全由后端
  // `frontend_access.modules` 决定。后端未下发 modules 数组（空列表）的存量场景继续
  // 保持“开放通行”的兼容行为，直到所有页面都显式声明模块键为止。
  function canAccessModuleWhenDeclared(moduleKey: string): boolean {
    if (!currentUser.value) return false
    const mods = store.modules
    if (!mods.length) return true
    return mods.includes(moduleKey)
  }

  /**
   * 判断当前用户是否有权访问某条任务（数据范围守卫）。
   *
   * GLOBAL  : 全数据，无限制。
   * SELF    : 仅自己创建或被指派的任务。
   * GROUP   : 本小组内的任务（含自己）。
   * DEPARTMENT : 本部门内所有小组的任务。通过 groups 表做真实 join，不模拟。
   */
  function canAccessTask(task: Task): boolean {
    const user = currentUser.value
    const scope = dataScope.value
    if (!user || !scope) return false

    if (scope === DataScopeEnum.GLOBAL) return true

    const isOwner =
      task.requesterId === user.id ||
      task.creatorId === user.id ||
      (task.designerId ?? task.assigneeId) === user.id ||
      task.currentHandlerId === user.id
    if (scope === DataScopeEnum.SELF) return isOwner

    const taskGroupId = String(task.ownerOrgTeam ?? task.groupId ?? '').trim()
    const taskDepartmentId = String(
      task.ownerDepartment ?? store.getGroupDepartmentId(task.groupId) ?? '',
    ).trim()
    const sameGroup = taskGroupId !== '' && taskGroupId === String(user.groupId ?? '').trim()
    if (scope === DataScopeEnum.GROUP) return isOwner || sameGroup

    // DEPARTMENT：优先使用任务读模型 ownerDepartment，缺失时回退 groups 映射。
    // 仍保留 isOwner 兜底，避免跨部门协作链路（如运营发起、设计承接）里
    // 任务责任人可进入详情但被前端范围守卫误判为不可访问，导致动作按钮被隐藏。
    if (scope === DataScopeEnum.DEPARTMENT) {
      const userDeptId = String(user.departmentId ?? '').trim()
      const sameDepartment = taskDepartmentId !== '' && taskDepartmentId === userDeptId
      return isOwner || sameDepartment
    }

    return false
  }

  /**
   * 判断当前用户是否有权操作某条任务（写操作守卫）。
   * TeamLead (GROUP_LEADER) 可查看整个部门，但仅能操作本组任务。
   * 其他角色的操作范围与查看范围一致。
   */
  function canOperateTask(task: Task): boolean {
    const user = currentUser.value
    if (!user) return false
    if (!canAccessTask(task)) return false

    if (store.isGroupLeader) {
      const taskGroupId = String(task.ownerOrgTeam ?? task.groupId ?? '').trim()
      const userGroupId = String(user.groupId ?? '').trim()
      const isOwner =
        task.requesterId === user.id ||
        task.creatorId === user.id ||
        (task.designerId ?? task.assigneeId) === user.id ||
        task.currentHandlerId === user.id
      return isOwner || (taskGroupId !== '' && taskGroupId === userGroupId)
    }

    return true
  }

  /**
   * 判断当前用户是否有权访问某个部门的数据。
   * GLOBAL / DEPARTMENT 可跨部门；SELF / GROUP 仅限本部门。
   */
  function canAccessDepartment(departmentId: string): boolean {
    const user = currentUser.value
    const scope = dataScope.value
    if (!user || !scope) return false

    if (scope === DataScopeEnum.GLOBAL || scope === DataScopeEnum.DEPARTMENT) return true

    return user.departmentId === departmentId
  }

  /**
   * 过滤任务列表，保留当前用户在数据范围内可见的任务。
   */
  function filterTasksByScope(tasks: Task[]): Task[] {
    if (!currentUser.value) return []
    if (dataScope.value === DataScopeEnum.GLOBAL) return tasks
    return tasks.filter((t) => canAccessTask(t))
  }

  return {
    currentUser,
    dataScope,
    frontendScopes,
    frontendRoles,
    can,
    canAccessMenu,
    canAccessPage,
    canAccessAction,
    canAccessModule,
    canAccessModuleWhenDeclared,
    canAccessTask,
    canOperateTask,
    canAccessDepartment,
    filterTasksByScope,
  }
}
