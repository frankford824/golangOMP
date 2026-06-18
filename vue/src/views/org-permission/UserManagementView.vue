<template>
  <div class="user-management-view min-h-[100dvh] bg-stone-100 text-zinc-900">
    <div
      class="um-shell mx-auto w-full max-w-[min(100%,90rem)] px-3 pb-10 pt-4 sm:px-4 sm:pt-5 md:px-6 md:pt-6"
    >
      <header class="page-header">
        <div>
          <h2 class="page-title">用户与角色管理</h2>
          <p class="page-sub">维护账号、组织归属与工作流角色</p>
        </div>
        <div class="page-header-actions">
          <BaseButton
            v-if="canManageOrg"
            type="button"
            variant="secondary"
            @click="goOrgPermission"
          >
            组织主数据管理
          </BaseButton>
          <BaseButton
            v-if="canCreateUser"
            type="button"
            @click="showCreateModal = true"
          >
            新增用户
          </BaseButton>
        </div>
      </header>
    <div v-if="!canManage" class="mt-6">
      <BaseEmptyState title="无管理权限" description="需要组织管理权限才能访问本页。" />
    </div>
    <template v-else>
      <section class="content-card">
        <h3 class="section-title">用户列表</h3>
        <div class="toolbar">
          <BaseInput
            v-model="keyword"
            class="toolbar-field"
            placeholder="搜索用户名 / 姓名"
            @keyup.enter="onSearch"
          />
          <BaseSelect
            v-model="statusFilter"
            class="toolbar-field"
            placeholder="全部状态"
            :options="statusFilterOptions"
            clearable
          />
          <BaseSelect
            v-model="roleFilter"
            class="toolbar-field"
            placeholder="全部角色"
            :options="roleFilterOptions"
            clearable
          />
          <BaseSelect
            v-model="departmentFilter"
            class="toolbar-field"
            placeholder="全部部门"
            :options="departmentFilterOptions"
            clearable
          />
          <BaseSelect
            v-model="teamFilter"
            class="toolbar-field"
            placeholder="全部小组"
            :options="teamFilterOptions"
            clearable
          />
          <BaseButton type="button" variant="primary" class="toolbar-query" @click="onSearch">查询</BaseButton>
        </div>
        <div v-if="listLoading" class="space-y-2">
          <BaseSkeleton width="100%" height="2rem" />
        </div>
        <BaseErrorState v-else-if="listError" :title="listError" @retry="loadUsers" />
        <BaseEmptyState v-else-if="!users.length" title="暂无用户" description="未获取到用户列表。" />
        <div v-else class="table-wrap">
          <table class="simple-table">
            <thead>
              <tr>
                <th>用户名</th>
                <th>姓名</th>
                <th>部门</th>
                <th>组</th>
                <th>角色</th>
                <th>状态</th>
                <th class="th-action">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="u in users" :key="u.id">
                <td class="td-mono">{{ u.username }}</td>
                <td>{{ u.display_name || '-' }}</td>
                <td>{{ u.department ?? '-' }}</td>
                <td>{{ u.team ?? '-' }}</td>
                <td>{{ formatWorkflowRolesForDisplay(u.roles) }}</td>
                <td>
                  <span
                    class="status-pill"
                    :class="u.status === 'active' ? 'status-pill--on' : 'status-pill--off'"
                  >
                    {{ formatUserStatusForDisplay(u.status) }}
                  </span>
                </td>
                <td>
                  <button type="button" class="link-btn" @click="openDetail(u)">角色管理</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="pager-row">
          <span class="pager-total">共 {{ pagination.total }} 条</span>
          <div class="pager-right">
            <label class="pager-label">
              每页
              <BaseSelect
                v-model="pageSize"
                class="pager-page-size"
                :options="pageSizeOptions"
              />
            </label>
            <BaseButton variant="ghost" size="sm" :disabled="page <= 1" @click="goPage(page - 1)">
              上一页
            </BaseButton>
            <span class="pager-meta">第 {{ page }} / {{ totalPages }} 页</span>
            <BaseButton variant="ghost" size="sm" :disabled="page >= totalPages" @click="goPage(page + 1)">
              下一页
            </BaseButton>
          </div>
        </div>
      </section>

      <!-- 用户详情 / 角色管理 弹层 -->
      <div v-if="detailUser" class="modal-mask" @click.self="detailUser = null">
        <div class="modal-panel um-modal">
          <h3 class="section-title">角色管理：{{ detailUser.display_name || detailUser.username }}</h3>
          <div v-if="detailLoading" class="py-4"><BaseSkeleton width="100%" height="4rem" /></div>
          <div v-else class="space-y-3">
            <p class="text-xs text-slate-600">当前角色：{{ formatWorkflowRolesForDisplay(detailUser.roles) }}</p>
            <p class="text-xs text-slate-600">状态：{{ formatUserStatusForDisplay(detailUser.status) }}</p>
            <div class="roles-grid" :class="{ 'roles-grid-readonly': !canAssignRoles }">
              <label v-for="role in roleOptions" :key="role.code" class="role-check">
                <input
                  v-model="selectedRoleCodes"
                  type="checkbox"
                  :value="role.code"
                  :disabled="!canAssignRoles"
                />
                <span>{{ role.display }}</span>
              </label>
            </div>
            <div class="modal-actions-inline">
              <!-- 仅 `role.assign`（HRAdmin / SuperAdmin）可保存角色；DeptAdmin 只能查看。 -->
              <button
                v-if="canAssignRoles"
                type="button"
                class="um-btn um-btn--primary um-btn--sm"
                :disabled="roleSubmitting"
                @click="submitRoleReplace"
              >
                {{ roleSubmitting ? '提交中...' : '保存角色' }}
              </button>
              <p v-else class="role-readonly-hint">当前账号无角色分派权限，角色列表只读。</p>
              <button
                v-if="canDisableUser"
                type="button"
                class="um-btn um-btn--ghost um-btn--sm"
                :disabled="statusSubmitting || detailUser.status === 'disabled'"
                @click="setUserStatus('disabled')"
              >
                {{ statusSubmitting ? '处理中...' : '禁用用户' }}
              </button>
              <button
                v-if="canDisableUser"
                type="button"
                class="um-btn um-btn--ghost um-btn--sm"
                :disabled="statusSubmitting || detailUser.status === 'active'"
                @click="setUserStatus('active')"
              >
                {{ statusSubmitting ? '处理中...' : '启用用户' }}
              </button>
            </div>
            <div v-if="canResetPassword" class="password-row">
              <input v-model="resetPasswordValue" class="input" placeholder="输入新密码" />
              <button type="button" class="um-btn um-btn--primary um-btn--sm" :disabled="passwordSubmitting" @click="resetPassword">
                {{ passwordSubmitting ? '重置中...' : '重置密码' }}
              </button>
            </div>
            <div v-if="canMoveTeam" class="membership-row">
              <div class="membership-grid">
                <select v-model="membershipForm.department" class="input">
                  <option value="">选择部门</option>
                  <option v-for="d in departmentOptions" :key="d.value" :value="d.value">{{ d.label }}</option>
                </select>
                <select v-model="membershipForm.team" class="input">
                  <option value="">选择小组</option>
                  <option v-for="t in membershipTeamOptions" :key="t.value" :value="t.value">{{ t.label }}</option>
                </select>
                <button
                  type="button"
                  class="um-btn um-btn--primary um-btn--sm"
                  :disabled="membershipSubmitting || !isMembershipDirty"
                  @click="submitMembership"
                >
                  {{ membershipSubmitting ? '保存中...' : '保存归属' }}
                </button>
                <button
                  v-if="canClearMembership && (detailUser.department || detailUser.team)"
                  type="button"
                  class="um-btn um-btn--ghost um-btn--sm"
                  :disabled="membershipSubmitting"
                  @click="clearMembership"
                >
                  移出到未分组
                </button>
              </div>
            </div>
            <p v-if="detailActionMessage" class="action-msg">{{ detailActionMessage }}</p>
          </div>
          <div class="modal-actions">
            <button type="button" class="um-btn um-btn--ghost" @click="detailUser = null">关闭</button>
          </div>
        </div>
      </div>

      <!-- 新增用户 -->
      <div v-if="showCreateModal" class="modal-mask" @click.self="showCreateModal = false">
        <div class="modal-panel um-modal">
          <h3 class="section-title">新增用户</h3>
          <div class="form-grid">
            <input v-model.trim="createForm.username" class="input" placeholder="请输入用户名" />
            <input v-model.trim="createForm.display_name" class="input" placeholder="请输入姓名" />
            <select v-model="createForm.department" class="input">
              <option value="">选择部门</option>
              <option v-for="d in departmentOptions" :key="d.value" :value="d.value">{{ d.label }}</option>
            </select>
            <select v-model="createForm.team" class="input">
              <option value="">选择小组</option>
              <option v-for="t in createTeamOptions" :key="t.value" :value="t.value">{{ t.label }}</option>
            </select>
            <input v-model.trim="createForm.mobile" class="input" placeholder="手机号" />
            <input v-model.trim="createForm.email" class="input" placeholder="邮箱(可选)" />
            <input v-model="createForm.password" class="input" type="password" placeholder="初始密码" />
            <select v-model="createForm.status" class="input">
              <option value="active">启用</option>
              <option value="disabled">已禁用</option>
            </select>
          </div>
          <div class="roles-grid mt-2">
            <label v-for="role in roleOptions" :key="'create-' + role.code" class="role-check">
              <input v-model="createForm.roles" type="checkbox" :value="role.code" />
              <span>{{ role.display }}</span>
            </label>
          </div>
          <p v-if="createError" class="action-msg">{{ createError }}</p>
          <div class="modal-actions">
            <button type="button" class="um-btn um-btn--ghost" @click="showCreateModal = false">取消</button>
            <button type="button" class="um-btn um-btn--primary" :disabled="createSubmitting" @click="createUser">
              {{ createSubmitting ? '创建中...' : '创建用户' }}
            </button>
          </div>
        </div>
      </div>
    </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { usersApi } from '@/services/api/usersApi'
import {
  departmentsAndGroupsFromOrgOptions,
  fetchOrgOwnershipOptions,
} from '@/services/api/orgApi'
import { usePermissionsStore } from '@/stores/permissions'
import { usePermission } from '@/composables/usePermission'
import { patchUserMembership, clearUserMembership } from '@/composables/useOrgPermissionData'
import {
  formatWorkflowRolesForDisplay,
  workflowRoleApiToDisplay,
} from '@/domain/user-workflow-roles'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseSelect, { type BaseSelectOption } from '@/components/base/BaseSelect.vue'
import BaseSkeleton from '@/components/base/BaseSkeleton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseErrorState from '@/components/base/BaseErrorState.vue'

const router = useRouter()
const permissionsStore = usePermissionsStore()
const { can } = usePermission()

// v1.8 对齐：用户与角色页面同时向 HRAdmin / SuperAdmin 与 DepartmentAdmin 开放。
// 以下 gate 全部走 action key，不再使用 `|| isDeptAdmin` 之类角色名兜底。
const canManage = computed(() => can('user.manage') || can('department.manage'))
// 与 OrgPermissionView 一致：仅持有 org.manage 的用户可进入组织主数据页。
const canManageOrg = computed(() => can('org.manage'))
const canCreateUser = computed(() => can('user.manage') || can('department.users.create'))
const canMoveTeam = computed(() => can('user.manage') || can('department.users.move_team'))
const canDisableUser = computed(() => can('user.manage') || can('department.users.disable'))
const canResetPassword = computed(
  () => can('user.manage') || can('department.users.reset_password'),
)
// 反向流程门控（「从未分配组分配到本部门」）。该按钮由 OrgPermissionView 渲染；
// 此处保留 computed 以便后续在本视图重用反向流程，并与其他 canXxx 门控保持对称。
const canAssignFromUnassigned = computed(
  () => can('user.manage') || can('department.users.assign_from_unassigned'),
)
void canAssignFromUnassigned
// Round I.f 热修复（F-1.1）：
// 后端 `authorizeUserUpdate` 对「移出到未分配池」路径只放行 `identityActorCanManageAllUsers`
// （Admin / SuperAdmin / HRAdmin）。DepartmentAdmin 即便持有 `department.users.move_team`，
// 发起该 PATCH 也会被后端以 `deny_code=department_scope_only` 403 拒绝。
// 因此按钮可见性的唯一合法门控是全局性的 `user.manage`，不得与任何 `department.*` 键析取。
const canClearMembership = computed(() => can('user.manage'))
// 后端 `PUT/POST/DELETE /v1/users/:id/roles*` 由持有 `role.assign` capability 的角色放行。
// 按 `/v1/roles` 字典（v1.8），当前具备此 capability 的是 SuperAdmin / RoleAdmin / Admin 三者；
// HRAdmin 已改为 `user.manage + org.assign + permission_logs.read`，不再具备 `role.assign`。
// 前端只对 `role.assign` 持有者露出「保存角色」按钮；其它角色（含 DeptAdmin / Ops）看到的是
// 只读视图——复选框保留展示当前归属，但被 `:disabled` 并通过 `.roles-grid-readonly` 调灰，
// 避免"能勾但无处提交"的误导。
const canAssignRoles = computed(() => can('role.assign'))

function goOrgPermission() {
  void router.push({ name: 'OrgPermission' })
}

interface UserRow {
  id: string
  username: string
  display_name?: string
  department?: string
  team?: string
  roles: string[]
  status?: 'active' | 'disabled' | string
  frontend_access?: unknown
}

interface RoleOption {
  code: string
  display: string
}

const listLoading = ref(false)
const listError = ref('')
const users = ref<UserRow[]>([])
const detailUser = ref<UserRow | null>(null)
const detailLoading = ref(false)
const roleSubmitting = ref(false)
const statusSubmitting = ref(false)
const passwordSubmitting = ref(false)
const detailActionMessage = ref('')
const resetPasswordValue = ref('')
const selectedRoleCodes = ref<string[]>([])
const membershipForm = ref<{ department: string; team: string }>({ department: '', team: '' })
const membershipSubmitting = ref(false)
const roleOptions = ref<RoleOption[]>([])

const page = ref(1)
const pageSize = ref(20)
const pagination = ref({ total: 0, page: 1, page_size: 20 })
const keyword = ref('')
const statusFilter = ref('')
const roleFilter = ref('')
const departmentFilter = ref('')
const teamFilter = ref('')

const departmentOptions = ref<Array<{ value: string; label: string }>>([])
const teamOptions = ref<Array<{ value: string; label: string; department?: string }>>([])

const showCreateModal = ref(false)
const createSubmitting = ref(false)
const createError = ref('')
const createForm = ref({
  username: '',
  display_name: '',
  department: '',
  team: '',
  mobile: '',
  email: '',
  password: 'Init1234',
  roles: [] as string[],
  status: 'active' as 'active' | 'disabled',
})

const totalPages = computed(() => Math.max(1, Math.ceil((pagination.value.total || 0) / pageSize.value)))
const statusFilterOptions = computed<BaseSelectOption[]>(() => [
  { value: 'active', label: '启用' },
  { value: 'disabled', label: '已禁用' },
])
const roleFilterOptions = computed<BaseSelectOption[]>(() =>
  roleOptions.value.map((role) => ({
    value: role.code,
    label: role.display,
  })),
)
const departmentFilterOptions = computed<BaseSelectOption[]>(() => departmentOptions.value)
const teamOptionsFiltered = computed(() =>
  departmentFilter.value
    ? teamOptions.value.filter((t) => !t.department || t.department === departmentFilter.value)
    : teamOptions.value,
)
const teamFilterOptions = computed<BaseSelectOption[]>(() => teamOptionsFiltered.value)
const pageSizeOptions = computed<BaseSelectOption[]>(() => [
  { value: 20, label: '20' },
  { value: 50, label: '50' },
  { value: 100, label: '100' },
])
const createTeamOptions = computed(() =>
  createForm.value.department
    ? teamOptions.value.filter((t) => !t.department || t.department === createForm.value.department)
    : teamOptions.value,
)
const membershipTeamOptions = computed(() =>
  membershipForm.value.department
    ? teamOptions.value.filter(
        (t) => !t.department || t.department === membershipForm.value.department,
      )
    : teamOptions.value,
)
const isMembershipDirty = computed(() => {
  const current = detailUser.value
  if (!current) return false
  return (
    membershipForm.value.department !== (current.department ?? '') ||
    membershipForm.value.team !== (current.team ?? '')
  )
})

/** 仅展示；查询/提交仍用 active、disabled */
function formatUserStatusForDisplay(status: string | undefined): string {
  if (status == null || status === '') return '—'
  if (status === 'active') return '启用'
  if (status === 'disabled') return '已禁用'
  return status
}

function mapRawUser(raw: Record<string, unknown>): UserRow {
  return {
    id: String(raw.id ?? ''),
    username: String(raw.username ?? ''),
    display_name: typeof raw.display_name === 'string' ? raw.display_name : undefined,
    department: typeof raw.department === 'string' ? raw.department : undefined,
    team: typeof raw.team === 'string' ? raw.team : (typeof raw.group === 'string' ? raw.group : undefined),
    roles: Array.isArray(raw.roles) ? (raw.roles as string[]) : [],
    status: typeof raw.status === 'string' ? raw.status : undefined,
    frontend_access: raw.frontend_access,
  }
}

async function loadOrgOptions() {
  const org = await fetchOrgOwnershipOptions()
  departmentOptions.value = org.departmentOptions
  teamOptions.value = org.teamOptions
  const hydrated = departmentsAndGroupsFromOrgOptions(org)
  if (hydrated) {
    usePermissionsStore().hydrateOrgFromServer(hydrated.departments, hydrated.groups)
  }
}

async function loadRoleOptions() {
  const res = await usersApi.listRoles()
  const data = res?.data
  const body = data?.data ?? data
  const list = Array.isArray(body) ? body : []
  roleOptions.value = list
    .map((raw: unknown) => {
      if (typeof raw === 'string') {
        return { code: raw, display: workflowRoleApiToDisplay(raw) }
      }
      if (raw && typeof raw === 'object') {
        const o = raw as Record<string, unknown>
        const code = String(o.code ?? o.role ?? o.name ?? '').trim()
        if (!code) return null
        return { code, display: workflowRoleApiToDisplay(code) }
      }
      return null
    })
    .filter((x): x is RoleOption => x != null)
}

async function loadUsers() {
  listLoading.value = true
  listError.value = ''
  try {
    const trimmedKeyword = keyword.value.trim()
    // 部门范围过滤：当前用户仅具备 `department.manage`（DeptAdmin）而无 `user.manage`
    // （HRAdmin/SuperAdmin）时，前端主动带上本部门 scope，避免后端因 scope 计算偏差
    // 泄漏跨部门列表。`user.manage` 持有者不附带 scope，保持全局视图。
    const deptScope = !can('user.manage') && can('department.manage')
      ? permissionsStore.currentUser?.departmentId
      : undefined
    const res = await usersApi.list({
      page: page.value,
      page_size: pageSize.value,
      ...(trimmedKeyword ? { keyword: trimmedKeyword } : {}),
      ...(statusFilter.value ? { status: statusFilter.value as 'active' | 'disabled' } : {}),
      ...(roleFilter.value ? { role: roleFilter.value } : {}),
      ...(deptScope ? { department: deptScope } : departmentFilter.value ? { department: departmentFilter.value } : {}),
      ...(teamFilter.value ? { team: teamFilter.value } : {}),
    })
    const data = res?.data
    const list = Array.isArray(data?.data) ? data.data : []
    const p = (data?.pagination ?? {}) as Record<string, unknown>
    users.value = list
      .filter((x: unknown): x is Record<string, unknown> => !!x && typeof x === 'object')
      .map(mapRawUser)
    pagination.value = {
      total: typeof p.total === 'number' ? p.total : users.value.length,
      page: typeof p.page === 'number' ? p.page : page.value,
      page_size: typeof p.page_size === 'number' ? p.page_size : pageSize.value,
    }
  } catch (e) {
    listError.value = e instanceof Error ? e.message : '加载用户列表失败'
  } finally {
    listLoading.value = false
  }
}

async function openDetail(u: UserRow) {
  detailUser.value = null
  detailActionMessage.value = ''
  resetPasswordValue.value = ''
  selectedRoleCodes.value = []
  detailLoading.value = true
  try {
    const res = await usersApi.getById(u.id)
    const data = res?.data
    const body = data?.data ?? data
    const raw = body && typeof body === 'object' ? (body as Record<string, unknown>) : null
    if (!raw) throw new Error('用户详情数据异常')
    detailUser.value = mapRawUser(raw)
    selectedRoleCodes.value = [...detailUser.value.roles]
    membershipForm.value = {
      department: detailUser.value.department ?? '',
      team: detailUser.value.team ?? '',
    }
  } catch (e) {
    detailActionMessage.value = e instanceof Error ? e.message : '加载用户详情失败'
  } finally {
    detailLoading.value = false
  }
}

async function refreshDetailAndList(userId?: string) {
  if (userId) {
    const listHit = users.value.find((u) => u.id === userId)
    if (listHit) await openDetail(listHit)
  }
  await loadUsers()
}

async function submitRoleReplace() {
  if (!detailUser.value) return
  roleSubmitting.value = true
  detailActionMessage.value = ''
  try {
    const res = await usersApi.replaceRoles(detailUser.value.id, { roles: selectedRoleCodes.value })
    const data = res?.data
    const body = data?.data ?? data
    if (body && typeof body === 'object') {
      const updated = mapRawUser(body as Record<string, unknown>)
      detailUser.value = { ...detailUser.value, ...updated, roles: updated.roles }
      selectedRoleCodes.value = [...updated.roles]
    }
    await refreshDetailAndList(detailUser.value.id)
    detailActionMessage.value = '角色更新成功'
  } catch (e) {
    detailActionMessage.value = e instanceof Error ? e.message : '角色更新失败'
  } finally {
    roleSubmitting.value = false
  }
}

async function setUserStatus(next: 'active' | 'disabled') {
  if (!detailUser.value) return
  statusSubmitting.value = true
  detailActionMessage.value = ''
  try {
    if (next === 'disabled') {
      await usersApi.deactivate(detailUser.value.id)
    } else {
      await usersApi.activate(detailUser.value.id)
    }
    await refreshDetailAndList(detailUser.value.id)
    detailActionMessage.value = next === 'disabled' ? '用户已禁用' : '用户已启用'
  } catch (e) {
    detailActionMessage.value = e instanceof Error ? e.message : '更新用户状态失败'
  } finally {
    statusSubmitting.value = false
  }
}

async function submitMembership() {
  if (!detailUser.value) return
  if (!isMembershipDirty.value) return
  membershipSubmitting.value = true
  detailActionMessage.value = ''
  try {
    await patchUserMembership(
      detailUser.value.id,
      membershipForm.value.department,
      membershipForm.value.team,
    )
    await refreshDetailAndList(detailUser.value.id)
    detailActionMessage.value = '归属已更新'
  } catch (e) {
    detailActionMessage.value = e instanceof Error ? e.message : '归属更新失败'
  } finally {
    membershipSubmitting.value = false
  }
}

async function clearMembership() {
  if (!detailUser.value) return
  membershipSubmitting.value = true
  detailActionMessage.value = ''
  try {
    // Round I.f 热修复（F-9.1）：
    // 不能再发 `{department:'', team:''}`（后端 validateDepartment("") 返回 400
    // "department is required"）。统一走 `team:'ungrouped'` 别名，由后端把
    // department 覆盖为 DepartmentUnassigned 并解析未分配池 team。
    await clearUserMembership(detailUser.value.id)
    membershipForm.value = { department: '', team: '' }
    await refreshDetailAndList(detailUser.value.id)
    detailActionMessage.value = '已移出到未分组'
  } catch (e) {
    detailActionMessage.value = e instanceof Error ? e.message : '移除归属失败'
  } finally {
    membershipSubmitting.value = false
  }
}

async function resetPassword() {
  if (!detailUser.value) return
  if (!resetPasswordValue.value.trim()) {
    detailActionMessage.value = '请输入新密码'
    return
  }
  passwordSubmitting.value = true
  detailActionMessage.value = ''
  try {
    await usersApi.resetPassword(detailUser.value.id, { password: resetPasswordValue.value.trim() })
    detailActionMessage.value = '密码重置成功'
    resetPasswordValue.value = ''
  } catch (e) {
    detailActionMessage.value = e instanceof Error ? e.message : '密码重置失败'
  } finally {
    passwordSubmitting.value = false
  }
}

function validateCreateForm(): string | null {
  const f = createForm.value
  if (!f.username.trim()) return '用户名必填'
  if (!f.display_name.trim()) return '姓名必填'
  if (!f.department) return '部门必选'
  if (!f.team) return '小组必选'
  if (!f.mobile.trim()) return '手机号必填'
  if (!f.password.trim()) return '初始密码必填'
  return null
}

async function createUser() {
  const err = validateCreateForm()
  if (err) {
    createError.value = err
    return
  }
  createSubmitting.value = true
  createError.value = ''
  try {
    const f = createForm.value
    await usersApi.create({
      username: f.username.trim(),
      display_name: f.display_name.trim(),
      department: f.department,
      team: f.team,
      mobile: f.mobile.trim(),
      email: f.email.trim() || undefined,
      password: f.password,
      roles: f.roles,
      status: f.status,
    })
    showCreateModal.value = false
    createForm.value = {
      username: '',
      display_name: '',
      department: '',
      team: '',
      mobile: '',
      email: '',
      password: 'Init1234',
      roles: [],
      status: 'active',
    }
    page.value = 1
    await loadUsers()
  } catch (e) {
    createError.value = e instanceof Error ? e.message : '创建用户失败'
  } finally {
    createSubmitting.value = false
  }
}

function onSearch() {
  page.value = 1
  void loadUsers()
}

function goPage(next: number) {
  const target = Math.max(1, Math.min(totalPages.value, next))
  if (target === page.value) return
  page.value = target
  void loadUsers()
}

watch([statusFilter, roleFilter, departmentFilter, teamFilter], () => {
  page.value = 1
  void loadUsers()
})

watch(pageSize, () => {
  page.value = 1
  void loadUsers()
})

watch(
  () => createForm.value.department,
  () => {
    if (!createForm.value.department) return
    const ok = teamOptions.value.some(
      (t) => t.value === createForm.value.team && (!t.department || t.department === createForm.value.department),
    )
    if (!ok) createForm.value.team = ''
  },
)

onMounted(() => {
  if (!canManage.value) return
  void Promise.all([loadRoleOptions(), loadOrgOptions()]).then(() => {
    void loadUsers()
  })
})
</script>

<style scoped>
/* 页画布：与看板页一致的浅灰底 */
.user-management-view {
  padding: 0;
}

.page-header {
  margin-bottom: 1.25rem;
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.page-header-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}

.page-title {
  margin: 0;
  font-size: 1.375rem;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: #18181b;
  line-height: 1.25;
}

.page-sub {
  margin: 0.35rem 0 0;
  font-size: 0.8125rem;
  color: #71717a;
  font-weight: 500;
}

/* 小黑盒式主按钮：深底、圆角胶囊 */
.um-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.25rem;
  padding: 0.45rem 1.1rem;
  font-size: 0.8125rem;
  font-weight: 500;
  border-radius: 9999px;
  border: 1px solid transparent;
  cursor: pointer;
  transition:
    background-color 0.15s ease,
    border-color 0.15s ease,
    box-shadow 0.15s ease,
    opacity 0.15s ease;
  white-space: nowrap;
}

.um-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.um-btn--primary {
  color: #fafafa;
  background: #262626;
  border-color: #404040;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
}

.um-btn--primary:hover:not(:disabled) {
  background: #333333;
  border-color: #525252;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
}

.um-btn--primary:active:not(:disabled) {
  background: #1a1a1a;
}

.um-btn--ghost {
  color: #3f3f46;
  background: #fff;
  border-color: #d4d4d8;
  box-shadow: 0 1px 0 rgba(15, 23, 42, 0.04);
}

.um-btn--ghost:hover:not(:disabled) {
  background: #fafafa;
  border-color: #a1a1aa;
}

.um-btn--sm {
  min-height: 1.75rem;
  padding: 0.2rem 0.7rem;
  font-size: 0.75rem;
}

.um-btn--md {
  min-height: 2.25rem;
  padding: 0.4rem 1rem;
  font-size: 0.8125rem;
}

.um-btn--primary:not(.um-btn--sm):not(.um-btn--md) {
  min-height: 2.5rem;
  padding: 0.5rem 1.25rem;
  font-size: 0.875rem;
}

/* 与工具栏行高对齐的查询按钮，不全宽 */
.toolbar-query {
  align-self: end;
}

.content-card {
  background: #fff;
  border-radius: 0.75rem;
  border: 1px solid rgb(228 228 231 / 0.95);
  box-shadow:
    0 1px 2px rgba(15, 23, 42, 0.05),
    0 0 0 1px rgba(15, 23, 42, 0.03);
  padding: 1.25rem 1.5rem;
}

@media (min-width: 640px) {
  .content-card {
    padding: 1.5rem;
  }
}

.section-title {
  margin: 0 0 1rem;
  font-size: 0.9375rem;
  font-weight: 600;
  color: #18181b;
  letter-spacing: -0.01em;
}

.toolbar {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 0.75rem;
  margin-bottom: 1rem;
  align-items: end;
}

.toolbar-field {
  min-width: 0;
}

.input {
  width: 100%;
  min-height: 2.25rem;
  border: 1px solid #e4e4e7;
  border-radius: 0.5rem;
  padding: 0.4rem 0.65rem;
  font-size: 0.8125rem;
  color: #18181b;
  background: #fff;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.input:focus {
  outline: none;
  border-color: #a1a1aa;
  box-shadow: 0 0 0 3px rgba(63, 63, 70, 0.12);
}

.input.small {
  min-height: 1.75rem;
  width: auto;
  min-width: 4.5rem;
  padding: 0.2rem 0.45rem;
  font-size: 0.75rem;
}

/* 表格外壳：去格子线，顶栏暗色条（小黑盒常用手法） */
.table-wrap {
  overflow-x: auto;
  border-radius: 0.5rem;
  border: 1px solid #e4e4e7;
  background: #fff;
}

.simple-table {
  width: 100%;
  min-width: 56rem;
  border-collapse: collapse;
  font-size: 0.8125rem;
}

.simple-table thead {
  background: #27272a;
  color: #e4e4e7;
}

.simple-table th {
  padding: 0.65rem 0.9rem;
  text-align: left;
  font-size: 0.6875rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  white-space: nowrap;
}

.simple-table th.th-action {
  text-align: right;
  width: 6.25rem;
  min-width: 6.25rem;
}

.simple-table tbody tr {
  border-top: 1px solid #f4f4f5;
  transition: background-color 0.12s ease;
}

.simple-table tbody tr:hover {
  background: #fafafa;
}

.simple-table td {
  padding: 0.65rem 0.9rem;
  vertical-align: middle;
  color: #3f3f46;
}

.simple-table td:last-child {
  text-align: right;
  white-space: nowrap;
}

.td-mono {
  font-variant-numeric: tabular-nums;
  font-family: var(--yb-font-data);
  font-size: 0.8125rem;
  color: #27272a;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  padding: 0.12rem 0.5rem;
  border-radius: 9999px;
  font-size: 0.6875rem;
  font-weight: 600;
}

.status-pill--on {
  background: #ecfdf5;
  color: #166534;
}

.status-pill--off {
  background: #f4f4f5;
  color: #71717a;
}

.link-btn {
  padding: 0.15rem 0.35rem;
  font-size: 0.8125rem;
  font-weight: 500;
  color: #059669;
  text-decoration: none;
  background: none;
  border: none;
  border-radius: 0.25rem;
  cursor: pointer;
  white-space: nowrap;
  transition: color 0.12s ease, background-color 0.12s ease;
}

.link-btn:hover {
  color: #047857;
  background: #ecfdf5;
}

.pager-row {
  margin-top: 1.25rem;
  padding-top: 1rem;
  border-top: 1px solid #f4f4f5;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.pager-total,
.pager-meta {
  font-size: 0.75rem;
  color: #71717a;
  font-weight: 500;
}

.pager-right {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}

.pager-label {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.75rem;
  color: #71717a;
  font-weight: 500;
}

.pager-page-size {
  min-width: 4.75rem;
}

/* —— 弹窗可读性层：浅色后台风格，仅作用于新增用户 / 角色管理 —— */
.modal-mask {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  background: rgba(15, 23, 42, 0.45);
}

.modal-panel {
  min-width: 320px;
  max-width: 92vw;
  width: 640px;
  max-height: min(90dvh, 900px);
  overflow-y: auto;
}

.modal-panel.um-modal {
  padding: 1.35rem 1.5rem 1.2rem;
  border-radius: 0.875rem;
  border: 1px solid #e5e7eb !important;
  background: #ffffff !important;
  color: #111827 !important;
  box-shadow: 0 10px 40px rgba(15, 23, 42, 0.12) !important;
}

.um-modal .section-title {
  margin: 0 0 1.1rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid #e5e7eb;
  font-size: 1rem;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: #111827 !important;
  line-height: 1.3;
}

.um-modal :where(.text-slate-600) {
  color: #6b7280 !important;
}

.um-modal .form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.6rem;
}

.um-modal .roles-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.5rem;
  padding: 0.75rem;
  border-radius: 0.75rem;
  border: 1px solid #e5e7eb !important;
  background: #f9fafb !important;
  box-shadow: none;
}

.um-modal .roles-grid-readonly {
  opacity: 0.72;
}

.um-modal .roles-grid-readonly .role-check {
  cursor: not-allowed;
  background: #f3f4f6 !important;
  border-color: #e5e7eb !important;
}

.um-modal .roles-grid-readonly .role-check:hover {
  border-color: #e5e7eb !important;
  background: #f3f4f6 !important;
}

.um-modal .roles-grid-readonly .role-check input {
  cursor: not-allowed;
}

.um-modal .role-check {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-height: 2.25rem;
  padding: 0.45rem 0.65rem;
  margin: 0;
  border-radius: 0.5rem;
  border: 1px solid #e5e7eb !important;
  background: #ffffff !important;
  color: #374151 !important;
  font-size: 0.8125rem;
  font-weight: 500;
  line-height: 1.35;
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    box-shadow 0.15s ease;
}

.um-modal .role-check:hover {
  border-color: #93c5fd !important;
  background: #f9fafb !important;
}

.um-modal .role-check:has(input:checked) {
  border-color: #2563eb !important;
  background: #eff6ff !important;
  box-shadow: 0 0 0 1px rgba(37, 99, 235, 0.12);
}

.um-modal .role-check:has(input:disabled) {
  cursor: not-allowed;
  opacity: 0.55;
}

.um-modal .role-check input[type='checkbox'] {
  width: 1rem;
  height: 1rem;
  flex-shrink: 0;
  margin: 0;
  accent-color: #2563eb;
  cursor: pointer;
}

.um-modal .input {
  width: 100%;
  min-height: 2.5rem;
  border: 1px solid #d1d5db !important;
  border-radius: 0.625rem;
  padding: 0.45rem 0.7rem;
  font-size: 0.8125rem;
  color: #111827 !important;
  background: #ffffff !important;
  color-scheme: light;
  box-shadow: none !important;
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease;
}

.um-modal .input::placeholder {
  color: #9ca3af !important;
}

.um-modal .input:focus {
  outline: none;
  border-color: #2563eb !important;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12) !important;
}

.um-modal select.input {
  cursor: pointer;
  appearance: none;
  background-color: #ffffff !important;
  background-image:
    linear-gradient(45deg, transparent 50%, #6b7280 50%),
    linear-gradient(135deg, #6b7280 50%, transparent 50%);
  background-position:
    calc(100% - 1.1rem) calc(50% + 0.12rem),
    calc(100% - 0.75rem) calc(50% + 0.12rem);
  background-size:
    0.35rem 0.35rem,
    0.35rem 0.35rem;
  background-repeat: no-repeat;
  padding-right: 2rem;
}

.um-modal .modal-actions-inline {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
  margin-top: 0.35rem;
  padding-top: 0.85rem;
  border-top: 1px solid #e5e7eb;
}

.um-modal .modal-actions {
  margin-top: 1.15rem;
  padding-top: 1rem;
  border-top: 1px solid #e5e7eb;
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  align-items: center;
  gap: 0.5rem;
}

.um-modal .um-btn--ghost {
  color: #374151 !important;
  background: #ffffff !important;
  border-color: #d1d5db !important;
  box-shadow: none;
}

.um-modal .um-btn--ghost:hover:not(:disabled) {
  color: #111827 !important;
  background: #f9fafb !important;
  border-color: #9ca3af !important;
}

.um-modal .um-btn--primary {
  background: #2563eb !important;
  border-color: #2563eb !important;
  color: #fff !important;
  box-shadow: none !important;
}

.um-modal .um-btn--primary:hover:not(:disabled) {
  background: #1d4ed8 !important;
  border-color: #1d4ed8 !important;
  box-shadow: none !important;
}

.um-modal .password-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 0.5rem;
  align-items: center;
  margin-top: 0.35rem;
  padding: 0.75rem;
  border-radius: 0.625rem;
  border: 1px solid #e5e7eb;
  background: #f9fafb;
}

.um-modal .membership-row {
  margin-top: 0.5rem;
  padding: 0.75rem;
  border-radius: 0.625rem;
  border: 1px solid #e5e7eb;
  background: #f9fafb;
}

.um-modal .membership-grid {
  display: grid;
  grid-template-columns: 1fr 1fr auto auto;
  gap: 0.5rem;
  align-items: center;
}

.um-modal .role-readonly-hint {
  font-size: 0.75rem;
  color: #6b7280 !important;
  margin: 0;
}

.um-modal .action-msg {
  margin: 0.5rem 0 0;
  padding: 0.45rem 0.65rem;
  font-size: 0.75rem;
  font-weight: 500;
  color: #1d4ed8 !important;
  border-radius: 0.5rem;
  border: 1px solid #bfdbfe;
  background: #eff6ff;
}

@media (max-width: 1024px) {
  .toolbar {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 640px) {
  .toolbar {
    grid-template-columns: 1fr;
  }

  .simple-table th,
  .simple-table td {
    padding-left: 0.6rem;
    padding-right: 0.6rem;
  }
}

@media (max-width: 980px) {
  .um-modal .roles-grid,
  .um-modal .form-grid {
    grid-template-columns: 1fr;
  }

  .um-modal .membership-grid {
    grid-template-columns: 1fr;
  }
}
</style>
