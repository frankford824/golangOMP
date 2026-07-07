<template>
  <div class="user-management-view min-h-[100dvh] bg-[rgb(var(--yb-bg-page))] text-[rgb(var(--yb-text))]">
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
            v-if="canCreateUser"
            type="button"
            @click="showCreateModal = true"
          >
            新增用户
          </BaseButton>
        </div>
      </header>
    <div v-if="!canManage" class="mt-6">
      <BaseEmptyState title="无管理权限" description="需要用户、组织或角色管理权限才能访问本页。" />
    </div>
    <template v-else>
      <section class="content-card">
        <div class="directory-heading">
          <h3 class="section-title">用户列表</h3>
          <span class="directory-count">共 {{ pagination.total }} 人</span>
        </div>
        <div class="management-layout">
          <aside class="org-filter-panel" aria-label="组织筛选">
            <button
              v-if="!isDeptScopedOnly"
              type="button"
              class="org-filter-item org-filter-item--all"
              :class="{ 'is-active': isAllOrgFilterActive }"
              @click="selectAllOrg"
            >
              <span>全部组织</span>
            </button>
            <div v-for="dept in visibleOrgTree" :key="dept.value" class="org-filter-dept">
              <button
                type="button"
                class="org-filter-item org-filter-item--dept"
                :class="{ 'is-active': isOrgDepartmentActive(dept.value) }"
                @click="selectOrgDepartment(dept.value)"
              >
                <span>{{ dept.label }}</span>
              </button>
              <div v-if="dept.teams.length" class="org-filter-teams">
                <button
                  v-for="team in dept.teams"
                  :key="`${dept.value}-${team.value}`"
                  type="button"
                  class="org-filter-item org-filter-item--team"
                  :class="{ 'is-active': isOrgTeamActive(dept.value, team.value) }"
                  @click="selectOrgTeam(dept.value, team.value)"
                >
                  <span>{{ team.label }}</span>
                </button>
              </div>
            </div>
          </aside>
          <div class="user-list-panel">
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
            <BaseErrorState v-if="listError" :title="listError" @retry="loadUsers" />
            <template v-else>
              <BaseDataTable
                aria-label="用户列表"
                :columns="userTableColumns"
                :data="users"
                :loading="listLoading"
                :row-key="userRowKey"
                :scroll-x="980"
                density="compact"
                empty-title="暂无用户"
                empty-description="未获取到用户列表。"
              />
              <BaseTablePager
                class="mt-4"
                :page="page"
                :page-size="pageSize"
                :total="pagination.total"
                :loading="listLoading"
                show-page-size
                :page-size-options="[20, 50, 100]"
                @update:page="goPage"
                @update:page-size="pageSize = $event"
              />
            </template>
          </div>
        </div>
      </section>

      <Teleport to="body">
      <!-- 用户详情 / 角色管理 弹层 -->
      <div v-if="detailUser" class="modal-mask" @click.self="detailUser = null">
        <div class="modal-panel um-modal um-modal--wide">
          <header class="modal-header">
            <div class="modal-heading">
              <h3 class="section-title">角色管理：{{ detailUser.display_name || detailUser.username }}</h3>
              <p class="modal-subtitle">当前角色：{{ formatWorkflowRolesForDisplay(detailUser.roles) }}</p>
              <p class="modal-subtitle">状态：{{ formatUserStatusForDisplay(detailUser.status) }}</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭角色管理" @click="detailUser = null">
              ×
            </button>
          </header>
          <div class="modal-body">
            <div v-if="detailLoading" class="py-4"><BaseSkeleton width="100%" height="4rem" /></div>
            <div v-else class="modal-stack">
              <div v-if="lockedDetailRoleOptions.length" class="legacy-role-box">
                <span class="legacy-role-title">{{ lockedDetailRoleTitle }}</span>
                <span
                  v-for="role in lockedDetailRoleOptions"
                  :key="'locked-' + role.code"
                  class="legacy-role-tag"
                  :title="lockedRoleTooltip(role)"
                >
                  {{ role.display }}
                </span>
              </div>
              <div v-if="editableRoleGroups.length" class="role-groups" :class="{ 'roles-grid-readonly': !canAssignRoles }">
                <section v-for="group in editableRoleGroups" :key="group.category" class="role-group">
                  <h4 class="role-group-title">{{ group.title }}</h4>
                  <div class="roles-grid">
                    <label v-for="role in group.roles" :key="role.code" class="role-check">
                      <input
                        v-model="selectedRoleCodes"
                        type="checkbox"
                        :value="role.code"
                        :disabled="!canAssignRoles"
                      />
                      <span>{{ role.display }}</span>
                    </label>
                  </div>
                </section>
              </div>
              <p v-else class="role-readonly-hint">当前账号没有可分配角色，角色信息仅可查看。</p>
              <div class="modal-actions-inline">
                <!-- 角色可写性同时依赖 frontend_access 与服务端角色目录 assignable_by_current_actor。 -->
                <button
                  v-if="canAssignRoles"
                  type="button"
                  class="um-btn um-btn--primary um-btn--sm"
                  :disabled="roleSubmitting"
                  @click="submitRoleReplace"
                >
                  {{ roleSubmitting ? '提交中...' : '保存角色' }}
                </button>
                <p v-else class="role-readonly-hint">当前账号无角色分配权限，角色信息仅可查看。</p>
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
                    <option v-for="d in membershipDepartmentOptions" :key="d.value" :value="d.value">{{ d.label }}</option>
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
          </div>
          <footer class="modal-footer">
            <div class="modal-footer-actions">
              <button type="button" class="um-btn um-btn--ghost" @click="detailUser = null">关闭</button>
            </div>
          </footer>
        </div>
      </div>

      <!-- 新增用户 -->
      <div v-if="showCreateModal" class="modal-mask" @click.self="showCreateModal = false">
        <div class="modal-panel um-modal um-modal--wide">
          <header class="modal-header">
            <div class="modal-heading">
              <h3 class="section-title">新增用户</h3>
              <p class="modal-subtitle">创建账号、组织归属与初始工作角色</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭新增用户" @click="showCreateModal = false">
              ×
            </button>
          </header>
          <div class="modal-body">
            <div class="form-grid">
              <input v-model.trim="createForm.username" class="input" placeholder="请输入用户名" />
              <input v-model.trim="createForm.display_name" class="input" placeholder="请输入姓名" />
              <select v-model="createForm.department" class="input">
                <option value="">选择部门</option>
                <option v-for="d in createDepartmentOptions" :key="d.value" :value="d.value">{{ d.label }}</option>
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
            <div v-if="editableRoleGroups.length" class="role-groups mt-2">
              <section v-for="group in editableRoleGroups" :key="'create-' + group.category" class="role-group">
                <h4 class="role-group-title">{{ group.title }}</h4>
                <div class="roles-grid">
                  <label v-for="role in group.roles" :key="'create-' + role.code" class="role-check">
                    <input v-model="createForm.roles" type="checkbox" :value="role.code" />
                    <span>{{ role.display }}</span>
                  </label>
                </div>
              </section>
            </div>
            <p v-else class="role-readonly-hint">当前账号没有可分配角色，新用户将使用系统默认角色。</p>
            <p v-if="createError" class="action-msg">{{ createError }}</p>
          </div>
          <footer class="modal-footer">
            <div class="modal-footer-actions">
              <button type="button" class="um-btn um-btn--ghost" @click="showCreateModal = false">取消</button>
              <button type="button" class="um-btn um-btn--primary" :disabled="createSubmitting" @click="createUser">
                {{ createSubmitting ? '创建中...' : '创建用户' }}
              </button>
            </div>
          </footer>
        </div>
      </div>
      </Teleport>
    </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, h, onBeforeUnmount, onMounted, watch } from 'vue'
import type { DataTableColumns, DataTableRowKey } from 'naive-ui'
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
import BaseDataTable from '@/components/base/BaseDataTable.vue'
import BaseTablePager from '@/components/base/BaseTablePager.vue'

const permissionsStore = usePermissionsStore()
const { can } = usePermission()

// v1.8 对齐：用户与角色页面同时向 HRAdmin / SuperAdmin 与 DepartmentAdmin 开放。
// 以下 gate 全部走 action key，不再使用 `|| isDeptAdmin` 之类角色名兜底。
const canManage = computed(
  () =>
    can('user.manage') ||
    can('department.manage') ||
    can('user.org.assign') ||
    can('role.assign') ||
    can('role.read'),
)
const canCreateUser = computed(() => can('user.manage') || can('department.users.create'))
const canMoveTeam = computed(() => can('user.manage') || can('department.users.move_team'))
const canDisableUser = computed(() => can('user.manage') || can('department.users.disable'))
const canResetPassword = computed(
  () => can('user.manage') || can('department.users.reset_password'),
)
// Round I.f 热修复（F-1.1）：
// 后端 `authorizeUserUpdate` 对「移出到未分配池」路径只放行 `identityActorCanManageAllUsers`
// （Admin / SuperAdmin / HRAdmin）。DepartmentAdmin 即便持有 `department.users.move_team`，
// 发起该 PATCH 也会被后端以 `deny_code=department_scope_only` 403 拒绝。
// 因此按钮可见性的唯一合法门控是全局性的 `user.manage`，不得与任何 `department.*` 键析取。
const canClearMembership = computed(() => can('user.manage'))
// 角色写入必须同时满足 frontend_access 与 `/v1/roles.assignable_by_current_actor`。
// Admin / RoleAdmin 即便仍是历史角色，也不能靠旧前端动作绕过服务端 assignable 合同。
const canAssignRoles = computed(() => can('role.assign') && editableRoleOptions.value.length > 0)

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
  category: 'management' | 'business' | 'asset_workbench' | 'compatibility' | string
  assignable: boolean
  assignableByCurrentActor: boolean
  deprecated: boolean
  hiddenByDefault: boolean
}

interface RoleOptionGroup {
  category: string
  title: string
  roles: RoleOption[]
}

interface OrgTreeTeam {
  value: string
  label: string
}

interface OrgTreeDepartment {
  value: string
  label: string
  teams: OrgTreeTeam[]
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
let listAbort: AbortController | null = null
let listSeq = 0

const auditPageSize = Number(import.meta.env.VITE_LARGE_SURFACE_PAGE_SIZE ?? 100)
const defaultPageSize = import.meta.env.VITE_LARGE_SURFACE_AUDIT === 'true' ? auditPageSize : 20
const page = ref(1)
const pageSize = ref(defaultPageSize)
const pagination = ref({ total: 0, page: 1, page_size: defaultPageSize })
const keyword = ref('')
const statusFilter = ref('')
const roleFilter = ref('')
const departmentFilter = ref('')
const teamFilter = ref('')

const departmentOptions = ref<Array<{ value: string; label: string }>>([])
const teamOptions = ref<Array<{ value: string; label: string; department?: string }>>([])
const currentDepartmentScope = computed(() =>
  String(
    permissionsStore.currentUser?.departmentId ||
      permissionsStore.managedDepartments[0] ||
      permissionsStore.actorDepartment ||
      '',
  ).trim(),
)
const isDeptScopedOnly = computed(
  () => !can('user.manage') && can('department.manage'),
)
const scopedDepartmentOptions = computed(() =>
  isDeptScopedOnly.value
    ? departmentOptions.value.filter((dept) => dept.value === currentDepartmentScope.value)
    : departmentOptions.value,
)
const orgTree = computed<OrgTreeDepartment[]>(() =>
  departmentOptions.value.map((dept) => ({
    value: dept.value,
    label: dept.label,
    teams: teamOptions.value
      .filter((team) => team.department === dept.value)
      .map((team) => ({ value: team.value, label: team.label })),
  })),
)
const visibleOrgTree = computed<OrgTreeDepartment[]>(() =>
  isDeptScopedOnly.value
    ? orgTree.value.filter((dept) => dept.value === currentDepartmentScope.value)
    : orgTree.value,
)
const isAllOrgFilterActive = computed(
  () => !isDeptScopedOnly.value && !departmentFilter.value && !teamFilter.value,
)

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
  roleOptions.value
    .filter((role) => !role.hiddenByDefault && role.category !== 'compatibility')
    .map((role) => ({
      value: role.code,
      label: role.display,
    })),
)
const departmentFilterOptions = computed<BaseSelectOption[]>(() => scopedDepartmentOptions.value)
const teamOptionsFiltered = computed(() =>
  departmentFilter.value
    ? teamOptions.value.filter((t) =>
        teamMatchesDepartment(t, departmentFilter.value, isDeptScopedOnly.value),
      )
    : isDeptScopedOnly.value
      ? teamOptions.value.filter((t) =>
          teamMatchesDepartment(t, currentDepartmentScope.value, true),
        )
      : teamOptions.value,
)
const teamFilterOptions = computed<BaseSelectOption[]>(() => teamOptionsFiltered.value)
const editableRoleOptions = computed(() =>
  roleOptions.value.filter(
    (role) =>
      role.assignable &&
      role.assignableByCurrentActor &&
      !role.deprecated &&
      role.category !== 'compatibility',
  ),
)
const editableRoleGroups = computed<RoleOptionGroup[]>(() => groupRoleOptions(editableRoleOptions.value))
const editableRoleCodeSet = computed(() => new Set(editableRoleOptions.value.map((role) => role.code)))
const roleOptionByCode = computed(() => {
  const out = new Map<string, RoleOption>()
  for (const role of roleOptions.value) out.set(role.code, role)
  return out
})
const lockedDetailRoleOptions = computed<RoleOption[]>(() => {
  const roles = detailUser.value?.roles ?? []
  return roles
    .filter((role) => !editableRoleCodeSet.value.has(role))
    .map((code) => roleOptionByCode.value.get(code) ?? legacyRoleOption(code))
})
const lockedDetailRoleCodes = computed(() => lockedDetailRoleOptions.value.map((role) => role.code))
const lockedDetailRoleTitle = computed(() =>
  lockedDetailRoleOptions.value.some(
    (role) => role.category === 'compatibility' || role.deprecated,
  )
    ? '历史/不可编辑角色'
    : '不可编辑角色',
)
const userTableColumns = computed<DataTableColumns<UserRow>>(() => [
  {
    title: '用户名',
    key: 'username',
    width: 150,
    render: (row) => h('span', { class: 'td-mono' }, row.username || '-'),
  },
  {
    title: '姓名',
    key: 'display_name',
    width: 150,
    render: (row) => row.display_name || '-',
  },
  {
    title: '部门',
    key: 'department',
    minWidth: 140,
    render: (row) => row.department ?? '-',
  },
  {
    title: '组',
    key: 'team',
    minWidth: 140,
    render: (row) => row.team ?? '-',
  },
  {
    title: '角色',
    key: 'roles',
    minWidth: 220,
    render: (row) => formatWorkflowRolesForDisplay(row.roles),
  },
  {
    title: '状态',
    key: 'status',
    width: 110,
    render: (row) =>
      h(
        'span',
        {
          class: ['status-pill', row.status === 'active' ? 'status-pill--on' : 'status-pill--off'],
        },
        formatUserStatusForDisplay(row.status),
      ),
  },
  {
    title: '操作',
    key: 'actions',
    width: 112,
    fixed: 'right',
    render: (row) =>
      h(
        'button',
        {
          type: 'button',
          class: 'link-btn',
          onClick: () => openDetail(row),
        },
        '角色管理',
      ),
  },
])
const createTeamOptions = computed(() =>
  createForm.value.department
    ? teamOptions.value.filter((t) =>
        teamMatchesDepartment(t, createForm.value.department, isDeptScopedOnly.value),
      )
    : isDeptScopedOnly.value
      ? teamOptions.value.filter((t) =>
          teamMatchesDepartment(t, currentDepartmentScope.value, true),
        )
      : teamOptions.value,
)
const createDepartmentOptions = computed(() => scopedDepartmentOptions.value)
const membershipDepartmentOptions = computed(() => scopedDepartmentOptions.value)
const membershipTeamOptions = computed(() =>
  membershipForm.value.department
    ? teamOptions.value.filter(
        (t) => teamMatchesDepartment(t, membershipForm.value.department, isDeptScopedOnly.value),
      )
    : isDeptScopedOnly.value
      ? teamOptions.value.filter((t) =>
          teamMatchesDepartment(t, currentDepartmentScope.value, true),
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

function readBoolean(value: unknown, fallback: boolean): boolean {
  if (typeof value === 'boolean') return value
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (normalized === 'true') return true
    if (normalized === 'false') return false
  }
  return fallback
}

function legacyRoleOption(code: string): RoleOption {
  return {
    code,
    display: workflowRoleApiToDisplay(code),
    category: 'compatibility',
    assignable: false,
    assignableByCurrentActor: false,
    deprecated: true,
    hiddenByDefault: true,
  }
}

function roleOptionFromString(code: string): RoleOption {
  return legacyRoleOption(code)
}

function roleCategoryTitle(category: string): string {
  switch (category) {
    case 'management':
      return '管理角色'
    case 'asset_workbench':
      return '素材工作台角色'
    case 'business':
      return '主业务角色'
    default:
      return '其他角色'
  }
}

function roleCategoryWeight(category: string): number {
  switch (category) {
    case 'management':
      return 10
    case 'business':
      return 20
    case 'asset_workbench':
      return 30
    default:
      return 90
  }
}

function groupRoleOptions(roles: RoleOption[]): RoleOptionGroup[] {
  const groups = new Map<string, RoleOption[]>()
  for (const role of roles) {
    const category = role.category || 'business'
    const existing = groups.get(category)
    if (existing) existing.push(role)
    else groups.set(category, [role])
  }
  return Array.from(groups.entries())
    .sort(([a], [b]) => roleCategoryWeight(a) - roleCategoryWeight(b))
    .map(([category, items]) => ({
      category,
      title: roleCategoryTitle(category),
      roles: items,
    }))
}

function roleOptionFromRecord(code: string, raw: Record<string, unknown>): RoleOption {
  const category = String(raw.category ?? '').trim() || 'business'
  const deprecated = readBoolean(raw.deprecated, category === 'compatibility')
  const hiddenByDefault = readBoolean(
    raw.hidden_by_default ?? raw.hiddenByDefault,
    deprecated || category === 'compatibility',
  )
  const displayRaw = String(
    raw.display ?? raw.display_name ?? raw.label ?? raw.title ?? '',
  ).trim()
  const nameRaw = String(raw.name ?? '').trim()
  const businessDisplay = workflowRoleApiToDisplay(code)
  return {
    code,
    display: businessDisplay && businessDisplay !== code
      ? businessDisplay
      : displayRaw || (nameRaw && nameRaw !== code ? nameRaw : code),
    category,
    assignable: readBoolean(raw.assignable, false),
    assignableByCurrentActor: readBoolean(
      raw.assignable_by_current_actor ?? raw.assignableByCurrentActor,
      false,
    ),
    deprecated,
    hiddenByDefault,
  }
}

function editableRoleCodesForUserRoles(roles: string[]): string[] {
  const editable = editableRoleCodeSet.value
  return roles.filter((role) => editable.has(role))
}

function mergeRoleCodes(...groups: string[][]): string[] {
  const out = new Set<string>()
  for (const group of groups) {
    for (const code of group) {
      const trimmed = String(code ?? '').trim()
      if (trimmed) out.add(trimmed)
    }
  }
  return Array.from(out)
}

function lockedRoleTooltip(role: RoleOption): string {
  if (role.category === 'compatibility' || role.deprecated) {
    return '当前账号已持有历史保留角色；本页不再支持新增分配。'
  }
  return '当前账号已持有该角色；本页不支持由当前登录账号调整。'
}

function teamMatchesDepartment(
  team: { department?: string },
  department: string,
  strictDepartment: boolean,
): boolean {
  if (!department) return false
  return strictDepartment ? team.department === department : !team.department || team.department === department
}

function defaultCreateRoles(): string[] {
  return editableRoleCodeSet.value.has('Member') ? ['Member'] : []
}

function emptyCreateForm() {
  return {
    username: '',
    display_name: '',
    department: isDeptScopedOnly.value ? currentDepartmentScope.value : '',
    team: '',
    mobile: '',
    email: '',
    password: 'Init1234',
    roles: defaultCreateRoles(),
    status: 'active' as 'active' | 'disabled',
  }
}

function userRowKey(row: UserRow): DataTableRowKey {
  return row.id
}

function isOrgDepartmentActive(department: string): boolean {
  return departmentFilter.value === department && !teamFilter.value
}

function isOrgTeamActive(department: string, team: string): boolean {
  return departmentFilter.value === department && teamFilter.value === team
}

function selectAllOrg() {
  if (isDeptScopedOnly.value) return
  departmentFilter.value = ''
  teamFilter.value = ''
}

function selectOrgDepartment(department: string) {
  departmentFilter.value = department
  teamFilter.value = ''
}

function selectOrgTeam(department: string, team: string) {
  departmentFilter.value = department
  teamFilter.value = team
}

async function loadOrgOptions() {
  const org = await fetchOrgOwnershipOptions()
  departmentOptions.value = org.departmentOptions
  teamOptions.value = org.teamOptions
  const hydrated = departmentsAndGroupsFromOrgOptions(org)
  if (hydrated) {
    usePermissionsStore().hydrateOrgFromServer(hydrated.departments, hydrated.groups)
  }
  if (isDeptScopedOnly.value) {
    if (!currentDepartmentScope.value) {
      departmentFilter.value = ''
      teamFilter.value = ''
      createForm.value.department = ''
      return
    }
    departmentFilter.value = currentDepartmentScope.value
    if (teamFilter.value) {
      const ok = teamOptions.value.some(
        (team) =>
          team.value === teamFilter.value &&
          teamMatchesDepartment(team, currentDepartmentScope.value, true),
      )
      if (!ok) teamFilter.value = ''
    }
    if (!createForm.value.department) createForm.value.department = currentDepartmentScope.value
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
        return roleOptionFromString(raw)
      }
      if (raw && typeof raw === 'object') {
        const o = raw as Record<string, unknown>
        const code = String(o.code ?? o.role ?? o.name ?? '').trim()
        if (!code) return null
        return roleOptionFromRecord(code, o)
      }
      return null
    })
    .filter((x): x is RoleOption => x != null)
  if (!createForm.value.roles.length) createForm.value.roles = defaultCreateRoles()
}

async function loadUsers() {
  listAbort?.abort()
  const seq = ++listSeq
  const abortController = new AbortController()
  listAbort = abortController
  listLoading.value = true
  listError.value = ''
  try {
    const trimmedKeyword = keyword.value.trim()
    // 部门范围过滤：当前用户仅具备 `department.manage`（DeptAdmin）而无 `user.manage`
    // （HRAdmin/SuperAdmin）时，前端主动带上本部门 scope，避免后端因 scope 计算偏差
    // 泄漏跨部门列表。`user.manage` 持有者不附带 scope，保持全局视图。
    if (isDeptScopedOnly.value && !currentDepartmentScope.value) {
      users.value = []
      pagination.value = { total: 0, page: page.value, page_size: pageSize.value }
      listError.value = '当前账号缺少部门管理范围，请联系人事管理员或超级管理员修正组织归属。'
      return
    }
    const deptScope = isDeptScopedOnly.value ? currentDepartmentScope.value : undefined
    const res = await usersApi.list({
      page: page.value,
      page_size: pageSize.value,
      ...(trimmedKeyword ? { keyword: trimmedKeyword } : {}),
      ...(statusFilter.value ? { status: statusFilter.value as 'active' | 'disabled' } : {}),
      ...(roleFilter.value ? { role: roleFilter.value } : {}),
      ...(deptScope ? { department: deptScope } : departmentFilter.value ? { department: departmentFilter.value } : {}),
      ...(teamFilter.value ? { team: teamFilter.value } : {}),
    }, abortController.signal)
    if (abortController.signal.aborted || seq !== listSeq) return
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
    if (abortController.signal.aborted || seq !== listSeq) return
    listError.value = e instanceof Error ? e.message : '加载用户列表失败'
  } finally {
    if (listAbort === abortController) {
      listAbort = null
    }
    if (seq === listSeq) {
      listLoading.value = false
    }
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
    selectedRoleCodes.value = editableRoleCodesForUserRoles(detailUser.value.roles)
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
    const selectedEditableRoles = selectedRoleCodes.value.filter((role) =>
      editableRoleCodeSet.value.has(role),
    )
    const roles = mergeRoleCodes(selectedEditableRoles, lockedDetailRoleCodes.value)
    const res = await usersApi.replaceRoles(detailUser.value.id, { roles })
    const data = res?.data
    const body = data?.data ?? data
    if (body && typeof body === 'object') {
      const updated = mapRawUser(body as Record<string, unknown>)
      detailUser.value = { ...detailUser.value, ...updated, roles: updated.roles }
      selectedRoleCodes.value = editableRoleCodesForUserRoles(updated.roles)
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
      roles: f.roles.filter((role) => editableRoleCodeSet.value.has(role)),
      status: f.status,
    })
    showCreateModal.value = false
    createForm.value = emptyCreateForm()
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

watch(departmentFilter, () => {
  if (!departmentFilter.value || !teamFilter.value) return
	  const ok = teamOptions.value.some(
	    (team) =>
	      team.value === teamFilter.value &&
	      teamMatchesDepartment(team, departmentFilter.value, isDeptScopedOnly.value),
	  )
  if (!ok) teamFilter.value = ''
})

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
	      (t) =>
	        t.value === createForm.value.team &&
	        teamMatchesDepartment(t, createForm.value.department, isDeptScopedOnly.value),
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

onBeforeUnmount(() => {
  listSeq += 1
  listAbort?.abort()
  listAbort = null
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
  color: rgb(var(--yb-text-zinc-strong));
  line-height: 1.25;
}

.page-sub {
  margin: 0.35rem 0 0;
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-zinc-soft));
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
  color: rgb(var(--yb-surface-row-even));
  background: rgb(var(--yb-surface-neutral-inverse));
  border-color: rgb(var(--yb-border-inverse));
  box-shadow: 0 1px 2px rgb(var(--yb-black) / 0.12);
}

.um-btn--primary:hover:not(:disabled) {
  background: rgb(var(--yb-surface-neutral-inverse-hover));
  border-color: rgb(var(--yb-border-inverse-soft));
  box-shadow: 0 2px 6px rgb(var(--yb-black) / 0.15);
}

.um-btn--primary:active:not(:disabled) {
  background: rgb(var(--yb-surface-neutral-inverse-deep));
}

.um-btn--ghost {
  color: rgb(var(--yb-text-zinc));
  background: rgb(var(--yb-surface));
  border-color: rgb(var(--yb-border-zinc-strong));
  box-shadow: 0 1px 0 rgb(var(--yb-shadow) / 0.04);
}

.um-btn--ghost:hover:not(:disabled) {
  background: rgb(var(--yb-surface-row-even));
  border-color: rgb(var(--yb-text-zinc-faint));
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
  background: rgb(var(--yb-surface));
  border-radius: 0.75rem;
  border: 1px solid rgb(var(--yb-border-zinc) / 0.95);
  box-shadow:
    0 1px 2px rgb(var(--yb-shadow) / 0.05),
    0 0 0 1px rgb(var(--yb-shadow) / 0.03);
  padding: 1.25rem 1.5rem;
}

.directory-heading {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.directory-heading .section-title {
  margin-bottom: 0;
}

.directory-count {
  font-size: 0.75rem;
  color: rgb(var(--yb-text-zinc-soft));
  font-variant-numeric: tabular-nums;
}

.management-layout {
  display: grid;
  grid-template-columns: minmax(10rem, 14rem) minmax(0, 1fr);
  gap: 1rem;
  align-items: start;
}

.org-filter-panel {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  max-height: min(66dvh, 44rem);
  overflow: auto;
  padding-right: 0.75rem;
  border-right: 1px solid rgb(var(--yb-border-zinc));
}

.org-filter-dept {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.org-filter-teams {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  margin-left: 0.65rem;
  padding-left: 0.55rem;
  border-left: 1px solid rgb(var(--yb-border-zinc));
}

.org-filter-item {
  display: flex;
  width: 100%;
  min-height: 2rem;
  align-items: center;
  justify-content: flex-start;
  border: 1px solid transparent;
  border-radius: 0.45rem;
  background: transparent;
  color: rgb(var(--yb-text-zinc));
  cursor: pointer;
  font-size: 0.8125rem;
  font-weight: 500;
  line-height: 1.35;
  padding: 0.35rem 0.55rem;
  text-align: left;
  transition:
    background-color 0.15s ease,
    border-color 0.15s ease,
    color 0.15s ease;
}

.org-filter-item span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.org-filter-item:hover {
  background: rgb(var(--yb-surface-row-even));
  border-color: rgb(var(--yb-border-zinc));
}

.org-filter-item.is-active {
  background: rgb(var(--yb-success-soft));
  border-color: rgb(var(--yb-success-border));
  color: rgb(var(--yb-success-deep));
}

.org-filter-item--team {
  min-height: 1.75rem;
  font-size: 0.75rem;
  color: rgb(var(--yb-text-zinc-soft));
}

.user-list-panel {
  min-width: 0;
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
  color: rgb(var(--yb-text-zinc-strong));
  letter-spacing: -0.01em;
}

@media (max-width: 960px) {
  .management-layout {
    grid-template-columns: 1fr;
  }

  .org-filter-panel {
    max-height: none;
    overflow: visible;
    padding-right: 0;
    padding-bottom: 0.75rem;
    border-right: 0;
    border-bottom: 1px solid rgb(var(--yb-border-zinc));
  }

  .org-filter-dept,
  .org-filter-teams {
    display: contents;
  }

  .org-filter-panel {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
    gap: 0.35rem;
  }
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
  border: 1px solid rgb(var(--yb-border-zinc));
  border-radius: 0.5rem;
  padding: 0.4rem 0.65rem;
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-zinc-strong));
  background: rgb(var(--yb-surface));
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.input:focus {
  outline: none;
  border-color: rgb(var(--yb-text-zinc-faint));
  box-shadow: 0 0 0 3px rgb(var(--yb-text-zinc) / 0.12);
}

.input.small {
  min-height: 1.75rem;
  width: auto;
  min-width: 4.5rem;
  padding: 0.2rem 0.45rem;
  font-size: 0.75rem;
}

.td-mono {
  font-variant-numeric: tabular-nums;
  font-family: var(--yb-font-data);
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-zinc-deep));
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
  background: rgb(var(--yb-success-soft));
  color: rgb(var(--yb-success-deep));
}

.status-pill--off {
  background: rgb(var(--yb-surface-neutral-muted));
  color: rgb(var(--yb-text-zinc-soft));
}

.link-btn {
  padding: 0.15rem 0.35rem;
  font-size: 0.8125rem;
  font-weight: 500;
  color: rgb(var(--yb-success-emerald));
  text-decoration: none;
  background: none;
  border: none;
  border-radius: 0.25rem;
  cursor: pointer;
  white-space: nowrap;
  transition: color 0.12s ease, background-color 0.12s ease;
}

.link-btn:hover {
  color: rgb(var(--yb-success-teal));
  background: rgb(var(--yb-success-soft));
}

/* —— 弹窗可读性层：浅色后台风格，仅作用于新增用户 / 角色管理 —— */
.modal-mask {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: clamp(0.75rem, 2vw, 1.5rem);
  background: rgb(var(--yb-shadow) / 0.45);
}

.modal-panel {
  min-width: 320px;
  width: min(760px, calc(100vw - 2rem));
  max-height: min(88dvh, 820px);
  overflow: hidden;
}

.modal-panel.um-modal {
  display: flex;
  flex-direction: column;
  border-radius: 0.875rem;
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
  box-shadow: 0 10px 40px rgb(var(--yb-shadow) / 0.12);
}

.um-modal--wide {
  width: min(780px, calc(100vw - 2rem));
}

.um-modal .modal-header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 1rem;
  align-items: start;
  padding: 0.95rem 1.35rem 0.8rem;
  border-bottom: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
}

.um-modal .modal-heading {
  min-width: 0;
}

.um-modal .section-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  letter-spacing: 0;
  color: rgb(var(--yb-text));
  line-height: 1.3;
}

.um-modal .modal-subtitle {
  margin: 0.25rem 0 0;
  font-size: 0.75rem;
  line-height: 1.35;
  color: rgb(var(--yb-text-muted));
}

.um-modal .modal-close {
  width: 2rem;
  height: 2rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 9999px;
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-muted));
  font-size: 1.25rem;
  line-height: 1;
  cursor: pointer;
  box-shadow: none;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    color 0.15s ease;
}

.um-modal .modal-close:hover {
  border-color: rgb(var(--yb-brand-border-strong));
  background: rgb(var(--yb-surface-soft));
  color: rgb(var(--yb-text));
}

.um-modal .modal-body {
  min-height: 0;
  overflow-y: auto;
  padding: 0.8rem 1.35rem;
}

.um-modal .modal-stack {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.um-modal .form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.6rem;
}

.um-modal .role-groups {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  padding: 0.65rem;
  border-radius: 0.75rem;
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
  box-shadow: none;
}

.um-modal .role-group {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.um-modal .role-group-title {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 700;
  line-height: 1.35;
  color: rgb(var(--yb-text-muted));
}

.um-modal .roles-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10.75rem, 1fr));
  gap: 0.5rem;
}

.um-modal .roles-grid-readonly {
  opacity: 0.72;
}

.um-modal .roles-grid-readonly .role-check {
  cursor: not-allowed;
  background: rgb(var(--yb-surface-muted));
  border-color: rgb(var(--yb-border));
}

.um-modal .roles-grid-readonly .role-check:hover {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface-muted));
}

.um-modal .roles-grid-readonly .role-check input {
  cursor: not-allowed;
}

.um-modal .legacy-role-box {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  align-items: center;
  padding: 0.65rem 0.75rem;
  border-radius: 0.65rem;
  border: 1px solid rgb(var(--yb-warning-border-soft));
  background: rgb(var(--yb-warning-soft));
}

.um-modal .legacy-role-title {
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(var(--yb-warning-dark));
}

.um-modal .legacy-role-tag {
  display: inline-flex;
  align-items: center;
  min-height: 1.5rem;
  padding: 0.15rem 0.55rem;
  border-radius: 9999px;
  border: 1px solid rgb(var(--yb-warning-border-soft));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-body));
  font-size: 0.75rem;
  font-weight: 500;
}

.um-modal .role-check {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
  min-height: 2.05rem;
  padding: 0.35rem 0.65rem;
  margin: 0;
  border-radius: 0.5rem;
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-body));
  font-size: 0.8125rem;
  font-weight: 500;
  line-height: 1.35;
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    box-shadow 0.15s ease;
}

.um-modal .role-check span {
  min-width: 0;
  overflow-wrap: anywhere;
}

.um-modal .role-check:hover {
  border-color: rgb(var(--yb-brand-border-strong));
  background: rgb(var(--yb-surface-soft));
}

.um-modal .role-check:has(input:checked) {
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-brand-soft));
  box-shadow: 0 0 0 1px rgb(var(--yb-brand) / 0.12);
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
  accent-color: rgb(var(--yb-brand));
  cursor: pointer;
}

.um-modal .input {
  width: 100%;
  min-height: 2.5rem;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 0.625rem;
  padding: 0.45rem 0.7rem;
  font-size: 0.8125rem;
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-surface));
  color-scheme: light;
  box-shadow: none;
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease;
}

.um-modal .input::placeholder {
  color: rgb(var(--yb-text-faint));
}

.um-modal .input:focus {
  outline: none;
  border-color: rgb(var(--yb-brand));
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.12);
}

.um-modal select.input {
  cursor: pointer;
  appearance: none;
  background-color: rgb(var(--yb-surface));
  background-image:
    linear-gradient(45deg, transparent 50%, rgb(var(--yb-text-muted)) 50%),
    linear-gradient(135deg, rgb(var(--yb-text-muted)) 50%, transparent 50%);
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
  padding-top: 0.7rem;
  border-top: 1px solid rgb(var(--yb-border));
}

.um-modal .modal-footer {
  flex-shrink: 0;
  padding: 0.7rem 1.35rem;
  border-top: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
}

.um-modal .modal-footer-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  align-items: center;
  gap: 0.5rem;
}

.um-modal .um-btn--ghost {
  color: rgb(var(--yb-text-body));
  background: rgb(var(--yb-surface));
  border-color: rgb(var(--yb-border-strong));
  box-shadow: none;
}

.um-modal .um-btn--ghost:hover:not(:disabled) {
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-surface-soft));
  border-color: rgb(var(--yb-text-faint));
}

.um-modal .um-btn--primary {
  background: rgb(var(--yb-brand));
  border-color: rgb(var(--yb-brand));
  color: rgb(var(--yb-text-inverse));
  box-shadow: none;
}

.um-modal .um-btn--primary:hover:not(:disabled) {
  background: rgb(var(--yb-brand-strong));
  border-color: rgb(var(--yb-brand-strong));
  box-shadow: none;
}

.um-modal .password-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.5rem;
  align-items: center;
  padding: 0.6rem;
  border-radius: 0.625rem;
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
}

.um-modal .membership-row {
  padding: 0.6rem;
  border-radius: 0.625rem;
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
}

.um-modal .membership-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto auto;
  gap: 0.5rem;
  align-items: center;
}

.um-modal .role-readonly-hint {
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted));
  margin: 0;
}

.um-modal .action-msg {
  margin: 0.5rem 0 0;
  padding: 0.45rem 0.65rem;
  font-size: 0.75rem;
  font-weight: 500;
  color: rgb(var(--yb-brand-strong));
  border-radius: 0.5rem;
  border: 1px solid rgb(var(--yb-brand-border));
  background: rgb(var(--yb-brand-soft));
}

:global(#app .um-modal .role-check:has(input:checked)) {
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-brand-soft));
}

:global(#app .um-modal .input) {
  border-color: rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
  box-shadow: none;
}

:global(#app .um-modal .input:focus) {
  border-color: rgb(var(--yb-brand));
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.12);
}

:global(#app .um-modal select.input) {
  background-color: rgb(var(--yb-surface));
}

:global(#app .um-modal .um-btn--ghost) {
  border-color: rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-body));
  box-shadow: none;
}

:global(#app .um-modal .um-btn--ghost:hover:not(:disabled)) {
  border-color: rgb(var(--yb-text-faint));
  background: rgb(var(--yb-surface-soft));
  color: rgb(var(--yb-text));
}

:global(#app .um-modal .um-btn--primary) {
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-brand));
  color: rgb(var(--yb-text-inverse));
  box-shadow: none;
}

:global(#app .um-modal .um-btn--primary:hover:not(:disabled)) {
  border-color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-strong));
  box-shadow: none;
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
}

@media (max-width: 980px) {
  .um-modal .form-grid {
    grid-template-columns: 1fr;
  }

  .um-modal .membership-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .modal-mask {
    align-items: stretch;
    padding: 0.75rem;
  }

  .modal-panel,
  .um-modal--wide {
    width: 100%;
    max-height: calc(100dvh - 1.5rem);
  }

  .um-modal .modal-header,
  .um-modal .modal-body,
  .um-modal .modal-footer {
    padding-left: 1rem;
    padding-right: 1rem;
  }

  .um-modal .password-row,
  .um-modal .modal-actions-inline,
  .um-modal .modal-footer-actions {
    display: grid;
    grid-template-columns: 1fr;
  }

  .um-modal .modal-actions-inline .um-btn,
  .um-modal .modal-footer-actions .um-btn,
  .um-modal .password-row .um-btn {
    width: 100%;
  }
}
</style>
