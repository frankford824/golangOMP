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
      <nav v-if="canManage" class="management-tabs" aria-label="用户与权限管理内容">
        <button v-if="canManageDirectory" type="button" :class="{ active: workspaceTab === 'directory' }" @click="openWorkspace('directory')">人员与组织</button>
        <button v-if="canManageAccess" type="button" :class="{ active: workspaceTab === 'access' }" @click="openWorkspace('access')">角色与权限</button>
      </nav>
    <div v-if="!canManage" class="mt-6">
      <BaseEmptyState title="无管理权限" description="需要用户、组织或角色管理权限才能访问本页。" />
    </div>
    <template v-else>
      <section v-if="workspaceTab === 'directory'" class="content-card">
        <div class="management-layout">
          <aside class="org-filter-panel" aria-label="组织列表与筛选">
            <div class="org-filter-header">
              <span>组织</span>
              <button
                v-if="canManageOrgMaster"
                type="button"
                class="org-action-btn"
                @click="openCreateDepartment"
              >
                新增部门
              </button>
            </div>
            <div v-if="orgOptionsError" class="org-load-error" role="alert">
              <p>{{ orgOptionsError }}</p>
              <button type="button" class="org-action-btn" @click="retryLoadOrgOptions">重试</button>
            </div>
            <div v-if="canManageOrgMaster && selectedOrgDepartment" class="org-selected-actions">
              <div class="org-selected-current">
                <strong>
                  {{ selectedOrgTeam ? `${selectedOrgDepartment.label} / ${selectedOrgTeam.label}` : selectedOrgDepartment.label }}
                </strong>
                <span
                  class="org-state-pill"
                  :class="{ 'org-state-pill--off': selectedOrgTeam ? !selectedOrgTeam.enabled : !selectedOrgDepartment.enabled }"
                >
                  {{ (selectedOrgTeam ? selectedOrgTeam.enabled : selectedOrgDepartment.enabled) ? '已启用' : '已停用' }}
                </span>
                <span v-if="selectedOrgMemberCount != null" class="org-selected-member-count">
                  {{ selectedOrgMemberCount }} 人
                </span>
              </div>
              <!-- 选中部门只显示部门操作,选中小组只显示小组操作,避免按钮堆叠 -->
              <div v-if="!selectedOrgTeam" class="org-selected-buttons">
                <button
                  v-if="selectedOrgDepartment.enabled"
                  type="button"
                  class="org-icon-btn"
                  @click.stop="openCreateTeam(selectedOrgDepartment)"
                >
                  新增小组
                </button>
                <button type="button" class="org-icon-btn" @click.stop="openEditDepartment(selectedOrgDepartment)">改名</button>
                <button
                  type="button"
                  class="org-icon-btn"
                  :class="{ 'org-icon-btn--danger': selectedOrgDepartment.enabled }"
                  @click.stop="openDeleteDepartment(selectedOrgDepartment)"
                >
                  {{ selectedOrgDepartment.enabled ? '停用' : '恢复' }}
                </button>
                <button
                  v-if="!isSystemDepartment(selectedOrgDepartment)"
                  type="button"
                  class="org-icon-btn"
                  @click.stop="openMergeDepartment(selectedOrgDepartment)"
                >
                  合并到…
                </button>
                <button
                  v-if="!isSystemDepartment(selectedOrgDepartment)"
                  type="button"
                  class="org-icon-btn org-icon-btn--danger"
                  @click.stop="openPurgeDepartment(selectedOrgDepartment)"
                >
                  彻底删除
                </button>
                <button v-if="canManageAccess" type="button" class="org-icon-btn org-icon-btn--policy" @click.stop="openSelectedOrgAccess">应用权限策略</button>
              </div>
              <div v-else class="org-selected-buttons">
                <button type="button" class="org-icon-btn" @click.stop="openEditTeam(selectedOrgTeam)">改名</button>
                <button
                  type="button"
                  class="org-icon-btn"
                  :class="{ 'org-icon-btn--danger': selectedOrgTeam.enabled }"
                  :disabled="!selectedOrgDepartment.enabled && !selectedOrgTeam.enabled"
                  :title="!selectedOrgDepartment.enabled && !selectedOrgTeam.enabled ? '请先恢复部门' : undefined"
                  @click.stop="openDeleteTeam(selectedOrgTeam)"
                >
                  {{ selectedOrgTeam.enabled ? '停用' : '恢复' }}
                </button>
                <button
                  v-if="!isSystemDepartment(selectedOrgDepartment)"
                  type="button"
                  class="org-icon-btn"
                  @click.stop="openMergeTeam(selectedOrgTeam)"
                >
                  合并到…
                </button>
                <button
                  v-if="!isSystemDepartment(selectedOrgDepartment)"
                  type="button"
                  class="org-icon-btn org-icon-btn--danger"
                  @click.stop="openPurgeTeam(selectedOrgTeam)"
                >
                  彻底删除
                </button>
                <button v-if="canManageAccess" type="button" class="org-icon-btn org-icon-btn--policy" @click.stop="openSelectedOrgAccess">应用权限策略</button>
              </div>
            </div>
            <OrgTreePanel
              :enabled-tree="enabledOrgTree"
              :disabled-tree="disabledOrgTree"
              :selected-department="departmentFilter"
              :selected-team="teamFilter"
              :show-all-entry="!isDeptScopedOnly"
              :all-active="isAllOrgFilterActive"
              :can-manage-policy="canManageAccess"
              @select-all="selectAllOrg"
              @select-department="selectOrgDepartment"
              @select-team="selectOrgTeam"
              @manage-policy="openOrgAccess"
            />
            <button
              v-if="canManageOrgMaster && removableDisabledOrgCount > 0"
              type="button"
              class="org-cleanup-btn"
              @click="openPurgeAllEmpty"
            >
              一键清理停用组织（{{ removableDisabledOrgCount }} 项）
            </button>
          </aside>
          <div class="user-list-panel">
            <div class="list-heading">
              <div class="list-heading-scope">
                <h3 class="section-title">{{ orgFilterBreadcrumb || '全部用户' }}</h3>
                <button
                  v-if="orgFilterBreadcrumb && !isDeptScopedOnly"
                  type="button"
                  class="org-scope-clear"
                  @click="selectAllOrg"
                >
                  清除组织筛选
                </button>
              </div>
              <span class="directory-count">共 {{ pagination.total }} 人</span>
            </div>
            <div class="toolbar">
              <BaseInput
                v-model="keyword"
                class="toolbar-field"
                placeholder="搜索姓名 / 工号 / 登录账号"
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
                :row-class-name="userRowClassName"
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

      <section v-else class="access-workspace-card">
        <AccessPolicyView
          :key="accessContextKey"
          embedded
          :initial-tab="accessSubtab"
          :initial-user="accessInitialUser"
          :initial-org="accessInitialOrg"
        />
      </section>

      <Teleport to="body">
      <!-- 用户详情 / 角色管理 弹层 -->
      <UserDetailModal
        v-if="detailUser"
        :user="detailUser"
        :loading="detailLoading"
        :can-edit-basic-info="canEditBasicInfo"
        :can-move-team="canMoveTeam"
        :can-clear-membership="canClearMembership"
        :can-assign-roles="canAssignRoles"
        :can-reset-password="canResetPassword"
        :can-disable-user="canDisableUser"
        :can-manage-access="canManageAccess"
        :basic-form="basicForm"
        :membership-form="membershipForm"
        :membership-department-options="membershipDepartmentOptions"
        :membership-team-options="membershipTeamOptions"
        :is-membership-dirty="isMembershipDirty"
        :editable-role-groups="editableRoleGroups"
        :locked-role-options="lockedDetailRoleOptions"
        :locked-role-title="lockedDetailRoleTitle"
        :basic-submitting="basicSubmitting"
        :membership-submitting="membershipSubmitting"
        :role-submitting="roleSubmitting"
        :password-submitting="passwordSubmitting"
        :status-submitting="statusSubmitting"
        :action-message="detailActionMessage"
        v-model:selected-roles="selectedRoleCodes"
        v-model:new-password="resetPasswordValue"
        @close="detailUser = null"
        @submit-basic="submitBasicInfo"
        @submit-membership="submitMembership"
        @clear-membership="clearMembership"
        @submit-roles="submitRoleReplace"
        @reset-password="resetPassword"
        @set-status="setUserStatus"
        @manage-access="openUserAccess(detailUser)"
      />

      <!-- 新增用户 -->
      <UserCreateModal
        v-if="showCreateModal"
        :form="createForm"
        :department-options="createDepartmentOptions"
        :team-options="createTeamOptions"
        :editable-role-groups="editableRoleGroups"
        :error="createError"
        :submitting="createSubmitting"
        @close="showCreateModal = false"
        @submit="createUser"
      />

      <!-- 组织维护 -->
      <div v-if="orgAction" class="modal-mask" @click.self="closeOrgAction">
        <div class="modal-panel um-modal org-action-modal">
          <header class="modal-header">
            <div class="modal-heading">
              <h3 class="section-title">{{ orgActionTitle }}</h3>
              <p class="modal-subtitle">{{ orgActionSubtitle }}</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭组织维护" @click="closeOrgAction">
              ×
            </button>
          </header>
          <div class="modal-body">
            <div v-if="orgActionIsDelete" class="delete-confirm-box">
              <p>{{ orgActionDeleteCopy }}</p>
              <ul v-if="orgAction?.mode === 'purgeAllEmpty'" class="purge-list">
                <li v-for="name in removableDisabledOrgNames" :key="name">{{ name }}</li>
              </ul>
            </div>
            <div v-else-if="orgActionIsMerge" class="form-grid form-grid--single">
              <p class="merge-hint">{{ orgActionMergeCopy }}</p>
              <select v-model="orgActionTargetId" class="input">
                <option value="">选择合并目标</option>
                <option v-for="target in orgMergeTargetOptions" :key="'merge-' + target.id" :value="target.id">
                  {{ target.label }}
                </option>
              </select>
            </div>
            <div v-else class="form-grid form-grid--single">
              <select v-if="orgAction?.mode === 'createTeam'" v-model="orgActionDepartmentId" class="input">
                <option value="">选择部门</option>
                <option v-for="dept in enabledOrgTree" :key="'org-action-' + dept.value" :value="dept.id">{{ dept.label }}</option>
              </select>
              <input v-model.trim="orgActionName" class="input" :placeholder="orgActionNamePlaceholder" />
            </div>
            <p v-if="orgActionError" class="action-msg">{{ orgActionError }}</p>
          </div>
          <footer class="modal-footer">
            <div class="modal-footer-actions">
              <button type="button" class="um-btn um-btn--ghost" @click="closeOrgAction">取消</button>
              <button
                type="button"
                class="um-btn"
                :class="orgActionIsDelete ? 'um-btn--ghost' : 'um-btn--primary'"
                :disabled="orgActionSubmitting"
                @click="submitOrgAction"
              >
                {{ orgActionSubmitting ? '处理中...' : orgActionConfirmText }}
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
import { useRoute, useRouter } from 'vue-router'
import { usersApi } from '@/services/api/usersApi'
import {
  createOrgDepartment,
  createOrgTeam,
  deleteOrgDepartment,
  deleteOrgTeam,
  departmentsAndGroupsFromOrgOptions,
  fetchOrgOwnershipOptions,
  mergeOrgDepartment,
  mergeOrgTeam,
  updateOrgDepartment,
  updateOrgTeam,
  type OrgDepartmentRecord,
  type OrgOwnershipOptionsParsed,
  type OrgTeamRecord,
} from '@/services/api/orgApi'
import OrgTreePanel from './OrgTreePanel.vue'
import UserDetailModal from './UserDetailModal.vue'
import UserCreateModal from './UserCreateModal.vue'
import type { OrgTreeDepartment, OrgTreeTeam } from './orgTreeTypes'
import {
  formatUserStatusForDisplay,
  type CreateUserForm,
  type RoleOption,
  type RoleOptionGroup,
  type UserRow,
} from './userManagementTypes'
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
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseErrorState from '@/components/base/BaseErrorState.vue'
import BaseDataTable from '@/components/base/BaseDataTable.vue'
import BaseTablePager from '@/components/base/BaseTablePager.vue'
import { RoleEnum } from '@/types'
import AccessPolicyView from '@/views/AccessPolicyView.vue'
import type { AccessUserOption, ScopeSubjectType } from '@/services/api/accessPolicyApi'

const permissionsStore = usePermissionsStore()
const { can } = usePermission()
const route = useRoute()
const router = useRouter()

// v1.8 对齐：用户与角色页面同时向 HRAdmin / SuperAdmin 与 DepartmentAdmin 开放。
// 以下 gate 全部走 action key，不再使用 `|| isDeptAdmin` 之类角色名兜底。
const canManageDirectory = computed(
  () =>
    can('user.manage') ||
    can('department.manage') ||
    can('user.org.assign') ||
    can('role.assign') ||
    can('role.read'),
)
const canManageAccess = computed(() => can('access.manage') || can('access.view'))
const canManage = computed(() => canManageDirectory.value || canManageAccess.value)
type WorkspaceTab = 'directory' | 'access'
type AccessTab = 'roles' | 'people' | 'org' | 'events'
const routeTab = String(route.query.tab || '')
const workspaceTab = ref<WorkspaceTab>(routeTab === 'access' || ['roles', 'people', 'org', 'events'].includes(routeTab) ? 'access' : 'directory')
const accessSubtab = ref<AccessTab>(['roles', 'people', 'org', 'events'].includes(routeTab) ? routeTab as AccessTab : 'roles')
const accessInitialUser = ref<AccessUserOption | null>(null)
const accessInitialOrg = ref<{ subject_type: ScopeSubjectType; subject_id: number } | null>(null)
const accessContextKey = computed(() => [accessSubtab.value, accessInitialUser.value?.id || 0, accessInitialOrg.value?.subject_type || '', accessInitialOrg.value?.subject_id || 0].join(':'))
if (!canManageDirectory.value && canManageAccess.value) workspaceTab.value = 'access'

function openWorkspace(tab: WorkspaceTab) {
  workspaceTab.value = tab
  accessInitialUser.value = null
  accessInitialOrg.value = null
  const query = tab === 'access' ? { ...route.query, tab: accessSubtab.value } : { ...route.query, tab: undefined }
  void router.replace({ query })
}

function openUserAccess(user: UserRow | null) {
  if (!user || !canManageAccess.value) return
  detailUser.value = null
  accessInitialOrg.value = null
  accessInitialUser.value = {
    id: Number(user.id),
    username: user.username,
    display_name: user.display_name,
    department: user.department,
    team: user.team,
  }
  accessSubtab.value = 'people'
  workspaceTab.value = 'access'
  void router.replace({ query: { ...route.query, tab: 'people', user_id: user.id } })
}

function openOrgAccess(subjectType: ScopeSubjectType, subjectId: number) {
  if (!canManageAccess.value || subjectId <= 0) return
  accessInitialUser.value = null
  accessInitialOrg.value = { subject_type: subjectType, subject_id: subjectId }
  accessSubtab.value = 'org'
  workspaceTab.value = 'access'
  void router.replace({ query: { ...route.query, tab: 'org', subject_type: subjectType, subject_id: String(subjectId) } })
}

function openSelectedOrgAccess() {
  const teamID = Number(selectedOrgTeam.value?.id || 0)
  if (teamID > 0) { openOrgAccess('team', teamID); return }
  const departmentID = Number(selectedOrgDepartment.value?.id || 0)
  if (departmentID > 0) openOrgAccess('department', departmentID)
}
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
const hasOrgMasterRole = computed(
  () =>
    permissionsStore.hasAnyRole(['SuperAdmin', 'HRAdmin']) ||
    permissionsStore.currentUser?.role === RoleEnum.SUPER_ADMIN ||
    permissionsStore.currentUser?.role === RoleEnum.HR_ADMIN,
)
const canManageOrgMaster = computed(
  () => can('org.manage') && hasOrgMasterRole.value,
)
const canEditBasicInfo = computed(() => can('user.manage'))

type OrgActionMode =
  | 'createDepartment'
  | 'editDepartment'
  | 'deleteDepartment'
  | 'createTeam'
  | 'editTeam'
  | 'deleteTeam'
  | 'mergeDepartment'
  | 'mergeTeam'
  | 'purgeDepartment'
  | 'purgeTeam'
  | 'purgeAllEmpty'

interface OrgActionState {
  mode: OrgActionMode
  department?: OrgTreeDepartment
  team?: OrgTreeTeam
}

const listLoading = ref(false)
const listError = ref('')
const users = ref<UserRow[]>([])
const detailUser = ref<UserRow | null>(null)
const detailLoading = ref(false)
const roleSubmitting = ref(false)
const statusSubmitting = ref(false)
const passwordSubmitting = ref(false)
const basicSubmitting = ref(false)
const detailActionMessage = ref('')
const resetPasswordValue = ref('')
const basicForm = ref<{ display_name: string; employee_no: string }>({ display_name: '', employee_no: '' })
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
// keyword 是输入框实时值;appliedKeyword 才是列表实际生效的关键词。
// 分开维护是为了避免"输入到一半、还没点查询,切换组织/翻页时半截关键词
// 被悄悄带进请求"的联动怪象。
const keyword = ref('')
const appliedKeyword = ref('')
const statusFilter = ref('')
const roleFilter = ref('')
// 单一组织选中态:左侧树的选中即列表筛选,不再维护"树选中"与"下拉筛选"两套状态。
const departmentFilter = ref('')
const teamFilter = ref('')

const departmentOptions = ref<Array<{ value: string; label: string }>>([])
const teamOptions = ref<Array<{ value: string; label: string; department?: string }>>([])
const departmentRecords = ref<OrgDepartmentRecord[]>([])
const teamRecords = ref<OrgTeamRecord[]>([])
const orgAction = ref<OrgActionState | null>(null)
const orgActionName = ref('')
const orgActionDepartmentId = ref('')
const orgActionTargetId = ref('')
const orgActionSubmitting = ref(false)
const orgActionError = ref('')
const orgOptionsError = ref('')
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
const orgTree = computed<OrgTreeDepartment[]>(() => {
  if (departmentRecords.value.length) {
    return departmentRecords.value.map((dept) => ({
      id: dept.id,
      value: dept.name,
      label: dept.name,
      enabled: dept.enabled !== false,
      memberCount: dept.memberCount,
      teams: teamRecords.value
        .filter((team) => team.departmentId === dept.id || team.departmentName === dept.name)
        .map((team) => ({
          id: team.id,
          value: team.name,
          label: team.name,
          department: dept.name,
          enabled: team.enabled !== false,
          memberCount: team.memberCount,
        })),
    }))
  }
  return departmentOptions.value.map((dept) => ({
    id: undefined,
    value: dept.value,
    label: dept.label,
    enabled: true,
    teams: teamOptions.value
      .filter((team) => team.department === dept.value)
      .map((team) => ({
        id: undefined,
        value: team.value,
        label: team.label,
        department: dept.value,
        enabled: true,
      })),
  }))
})
const visibleOrgTree = computed<OrgTreeDepartment[]>(() =>
  isDeptScopedOnly.value
    ? orgTree.value.filter((dept) => dept.value === currentDepartmentScope.value)
    : canManageOrgMaster.value
      ? orgTree.value
      : orgTree.value
          .filter((dept) => dept.enabled)
          .map((dept) => ({ ...dept, teams: dept.teams.filter((team) => team.enabled) })),
)
const enabledOrgTree = computed<OrgTreeDepartment[]>(() =>
  visibleOrgTree.value
    .filter((dept) => dept.enabled)
    .map((dept) => ({ ...dept, teams: dept.teams.filter((team) => team.enabled) })),
)
const disabledOrgTree = computed<OrgTreeDepartment[]>(() => {
  if (!canManageOrgMaster.value) return []
  const out: OrgTreeDepartment[] = []
  for (const dept of visibleOrgTree.value) {
    if (!dept.enabled) {
      out.push(dept)
      continue
    }
    const disabledTeams = dept.teams.filter((team) => !team.enabled)
    if (disabledTeams.length) out.push({ ...dept, teams: disabledTeams })
  }
  return out
})
const selectedOrgDepartment = computed<OrgTreeDepartment | null>(() =>
  visibleOrgTree.value.find((dept) => dept.value === departmentFilter.value) ?? null,
)
const selectedOrgTeam = computed<OrgTreeTeam | null>(() => {
  if (!selectedOrgDepartment.value || !teamFilter.value) return null
  return selectedOrgDepartment.value.teams.find((team) => team.value === teamFilter.value) ?? null
})
const hasNonOrgListFilters = computed(
  () => appliedKeyword.value.trim() !== '' || statusFilter.value !== '' || roleFilter.value !== '',
)
const selectedOrgMemberCount = computed<number | undefined>(() => {
  const known = selectedOrgTeam.value
    ? selectedOrgTeam.value.memberCount
    : selectedOrgDepartment.value?.memberCount
  if (known != null) return known
  // 兼容不返回 member_count 的旧后端:选中组织时,用户列表就是按该组织过滤的,
  // 没有其它筛选条件时 pagination.total 即成员数。
  if (!hasNonOrgListFilters.value && !listLoading.value && !listError.value) {
    return pagination.value.total
  }
  return undefined
})
const isAllOrgFilterActive = computed(
  () => !isDeptScopedOnly.value && !departmentFilter.value && !teamFilter.value,
)
const orgFilterBreadcrumb = computed(() => {
  if (!departmentFilter.value) return ''
  return teamFilter.value ? `${departmentFilter.value} / ${teamFilter.value}` : departmentFilter.value
})
// 一键治理候选:所有已停用的非系统部门与小组。删除时后端会把仍在该
// 组织内的账号归入未分配池,因此不再用 member_count 阻断治理。
// 部门删除会级联其小组,因此小组列表排除待删部门下的小组。
const removableDisabledOrgCleanup = computed<{ departments: OrgTreeDepartment[]; teams: OrgTreeTeam[] }>(() => {
  if (!canManageOrgMaster.value) return { departments: [], teams: [] }
  const departments = orgTree.value.filter(
    (dept) => !dept.enabled && !!dept.id && !isSystemDepartment(dept),
  )
  const purgedDeptIds = new Set(departments.map((dept) => dept.id))
  const teams: OrgTreeTeam[] = []
  for (const dept of orgTree.value) {
    if (isSystemDepartment(dept) || purgedDeptIds.has(dept.id)) continue
    for (const team of dept.teams) {
      if (!team.enabled && !!team.id) teams.push(team)
    }
  }
  return { departments, teams }
})
const removableDisabledOrgCount = computed(
  () => removableDisabledOrgCleanup.value.departments.length + removableDisabledOrgCleanup.value.teams.length,
)
const removableDisabledOrgNames = computed(() => {
  const parts: string[] = []
  for (const dept of removableDisabledOrgCleanup.value.departments) parts.push(`部门「${dept.label}」`)
  for (const team of removableDisabledOrgCleanup.value.teams) parts.push(`小组「${team.department} / ${team.label}」`)
  return parts
})

const showCreateModal = ref(false)
const createSubmitting = ref(false)
const createError = ref('')
const createForm = ref<CreateUserForm>({
  username: '',
  employee_no: '',
  display_name: '',
  department: '',
  team: '',
  mobile: '',
  email: '',
  password: 'Init1234',
  roles: [],
  status: 'active',
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
const orgActionIsDelete = computed(() =>
  orgAction.value?.mode === 'deleteDepartment' ||
  orgAction.value?.mode === 'deleteTeam' ||
  orgAction.value?.mode === 'purgeDepartment' ||
  orgAction.value?.mode === 'purgeTeam' ||
  orgAction.value?.mode === 'purgeAllEmpty',
)
const orgActionIsMerge = computed(
  () => orgAction.value?.mode === 'mergeDepartment' || orgAction.value?.mode === 'mergeTeam',
)
const orgActionTitle = computed(() => {
  switch (orgAction.value?.mode) {
    case 'createDepartment':
      return '新增部门'
    case 'editDepartment':
      return '编辑部门名称'
    case 'deleteDepartment':
      return orgAction.value.department?.enabled === false ? '恢复部门' : '停用部门'
    case 'createTeam':
      return '新增小组'
    case 'editTeam':
      return '编辑小组名称'
    case 'deleteTeam':
      return orgAction.value.team?.enabled === false ? '恢复小组' : '停用小组'
    case 'mergeDepartment':
      return '合并部门'
    case 'mergeTeam':
      return '合并小组'
    case 'purgeDepartment':
      return '彻底删除部门'
    case 'purgeTeam':
      return '彻底删除小组'
    case 'purgeAllEmpty':
      return '清理停用组织'
    default:
      return '组织维护'
  }
})
const orgActionSubtitle = computed(() => {
  if (!orgAction.value) return ''
  if (orgAction.value.mode === 'deleteDepartment') {
    return orgAction.value.department?.enabled === false
      ? '恢复后该部门会重新出现在组织归属选择中。'
      : '停用后该部门不再可选，部门内人员会自动进入未分配池。'
  }
  if (orgAction.value.mode === 'deleteTeam') {
    return orgAction.value.team?.enabled === false
      ? '恢复后该小组会重新出现在组织归属选择中。'
      : '停用后该小组不再可选，组内人员会自动进入未分配池。'
  }
  if (orgAction.value.mode === 'mergeDepartment') {
    return '成员与管理范围会迁入目标部门，本部门随后自动停用。'
  }
  if (orgAction.value.mode === 'mergeTeam') {
    return '组内成员会迁入目标小组，本小组随后自动停用。'
  }
  if (orgAction.value.mode === 'purgeDepartment' || orgAction.value.mode === 'purgeTeam') {
    return '彻底删除会从组织树中永久移除该记录；当前仍在该组织内的账号会自动进入未分配池。'
  }
  if (orgAction.value.mode === 'purgeAllEmpty') {
    return '批量彻底删除所有停用部门与小组；相关账号会自动进入未分配池。'
  }
  return '组织名称会立即用于用户归属选择。'
})
const orgActionDeleteCopy = computed(() => {
  if (orgAction.value?.mode === 'deleteDepartment') {
    return orgAction.value.department?.enabled === false
      ? `确认恢复部门「${orgAction.value.department?.label ?? ''}」？`
      : `确认停用部门「${orgAction.value.department?.label ?? ''}」？该部门及其小组会停止使用，原有人员会自动进入未分配池。`
  }
  if (orgAction.value?.mode === 'deleteTeam') {
    return orgAction.value.team?.enabled === false
      ? `确认恢复小组「${orgAction.value.team?.label ?? ''}」？`
      : `确认停用小组「${orgAction.value.team?.label ?? ''}」？原有人员会自动进入未分配池。`
  }
  if (orgAction.value?.mode === 'purgeDepartment') {
    return `确认彻底删除部门「${orgAction.value.department?.label ?? ''}」？该操作不可恢复，该部门下账号会自动进入未分配池。`
  }
  if (orgAction.value?.mode === 'purgeTeam') {
    return `确认彻底删除小组「${orgAction.value.team?.label ?? ''}」？该操作不可恢复，该小组下账号会自动进入未分配池。`
  }
  if (orgAction.value?.mode === 'purgeAllEmpty') {
    return `确认批量彻底删除以下 ${removableDisabledOrgCount.value} 项停用组织？该操作不可恢复，相关账号会自动进入未分配池。`
  }
  return ''
})
const orgActionMergeCopy = computed(() => {
  if (orgAction.value?.mode === 'mergeDepartment') {
    return `把部门「${orgAction.value.department?.label ?? ''}」的成员与小组并入以下目标部门：`
  }
  if (orgAction.value?.mode === 'mergeTeam') {
    return `把小组「${orgAction.value.team?.label ?? ''}」的成员并入以下目标小组：`
  }
  return ''
})
const orgMergeTargetOptions = computed<Array<{ id: string; label: string }>>(() => {
  if (orgAction.value?.mode === 'mergeDepartment') {
    const sourceId = orgAction.value.department?.id
    return orgTree.value
      .filter((dept) => dept.enabled && dept.id && dept.id !== sourceId && !isSystemDepartment(dept))
      .map((dept) => ({ id: dept.id as string, label: dept.label }))
  }
  if (orgAction.value?.mode === 'mergeTeam') {
    const sourceId = orgAction.value.team?.id
    const out: Array<{ id: string; label: string }> = []
    for (const dept of orgTree.value) {
      if (!dept.enabled || isSystemDepartment(dept)) continue
      for (const team of dept.teams) {
        if (!team.enabled || !team.id || team.id === sourceId) continue
        out.push({ id: team.id, label: `${dept.label} / ${team.label}` })
      }
    }
    return out
  }
  return []
})
const orgActionNamePlaceholder = computed(() =>
  orgAction.value?.mode === 'createTeam' || orgAction.value?.mode === 'editTeam'
    ? '请输入小组名称'
    : '请输入部门名称',
)
const orgActionConfirmText = computed(() => {
  if (orgActionIsMerge.value) return '确认合并'
  if (!orgActionIsDelete.value) return '保存'
  if (orgAction.value?.mode === 'purgeAllEmpty') return '确认清理'
  if (orgAction.value?.mode === 'purgeDepartment' || orgAction.value?.mode === 'purgeTeam') return '确认删除'
  if (orgAction.value?.mode === 'deleteDepartment' && orgAction.value.department?.enabled === false) return '确认恢复'
  if (orgAction.value?.mode === 'deleteTeam' && orgAction.value.team?.enabled === false) return '确认恢复'
  return '确认停用'
})
const userTableColumns = computed<DataTableColumns<UserRow>>(() => [
  {
    title: '姓名',
    key: 'display_name',
    minWidth: 170,
    render: (row) =>
      h('div', { class: 'user-name-cell' }, [
        h('span', { class: 'user-name-text' }, row.display_name || '-'),
        renderUserStatusPill(row.status, 'status-pill--inline'),
      ]),
  },
  {
    title: '工号',
    key: 'employee_no',
    width: 120,
    render: (row) =>
      row.employee_no == null
        ? h('span', { class: 'employee-missing' }, '待维护工号')
        : h('span', { class: 'td-mono' }, String(row.employee_no)),
  },
  {
    title: '登录账号',
    key: 'username',
    width: 150,
    render: (row) => h('span', { class: 'td-mono' }, row.username || '-'),
  },
  {
    title: '部门',
    key: 'department',
    minWidth: 140,
    // 列表 → 树的反向联动:点部门名即选中左侧树对应部门并过滤列表。
    render: (row) => {
      const department = row.department ?? ''
      if (!department) return '-'
      if (!isFilterableDepartment(department)) return department
      return h(
        'button',
        {
          type: 'button',
          class: 'org-link-btn',
          title: `按部门「${department}」筛选`,
          onClick: () => selectOrgDepartment(department),
        },
        department,
      )
    },
  },
  {
    title: '组',
    key: 'team',
    minWidth: 140,
    render: (row) => {
      const team = row.team ?? ''
      const department = row.department ?? ''
      if (!team) return '-'
      if (!department || !isFilterableTeam(department, team)) return team
      return h(
        'button',
        {
          type: 'button',
          class: 'org-link-btn',
          title: `按小组「${department} / ${team}」筛选`,
          onClick: () => selectOrgTeam(department, team),
        },
        team,
      )
    },
  },
  {
    title: '角色',
    key: 'roles',
    minWidth: 220,
    render: (row) => formatWorkflowRolesForDisplay(row.roles),
  },
  {
    title: '账号状态',
    key: 'status',
    width: 116,
    render: (row) => renderUserStatusPill(row.status),
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
        '管理',
      ),
  },
])
const createTeamOptions = computed(() =>
  createForm.value.department
    ? teamOptions.value.filter((t) =>
        teamMatchesDepartment(t, createForm.value.department, true),
      )
    : isDeptScopedOnly.value
      ? teamOptions.value.filter((t) =>
          teamMatchesDepartment(t, currentDepartmentScope.value, true),
        )
      : [],
)
const createDepartmentOptions = computed(() => scopedDepartmentOptions.value)
const membershipDepartmentOptions = computed(() => scopedDepartmentOptions.value)
const membershipTeamOptions = computed(() =>
  membershipForm.value.department
    ? teamOptions.value.filter(
        (t) => teamMatchesDepartment(t, membershipForm.value.department, true),
      )
    : isDeptScopedOnly.value
      ? teamOptions.value.filter((t) =>
          teamMatchesDepartment(t, currentDepartmentScope.value, true),
        )
      : [],
)
const isMembershipDirty = computed(() => {
  const current = detailUser.value
  if (!current) return false
  return (
    membershipForm.value.department !== (current.department ?? '') ||
    membershipForm.value.team !== (current.team ?? '')
  )
})

function mapRawUser(raw: Record<string, unknown>): UserRow {
  const rawEmployeeNo = raw.employee_no ?? raw.employeeNo
  let employeeNo: number | null | undefined
  if (typeof rawEmployeeNo === 'number' && Number.isInteger(rawEmployeeNo)) employeeNo = rawEmployeeNo
  else if (typeof rawEmployeeNo === 'string' && /^\d+$/.test(rawEmployeeNo.trim())) employeeNo = Number(rawEmployeeNo.trim())
  else if (rawEmployeeNo === null) employeeNo = null
  return {
    id: String(raw.id ?? ''),
    username: String(raw.username ?? ''),
    employee_no: employeeNo,
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

function ensureMemberRoleCodes(roles: string[]): string[] {
  return mergeRoleCodes(['Member'], roles)
}

function parseEmployeeNoInput(raw: string): number | null {
  const value = raw.trim()
  if (!value) return null
  if (!/^\d+$/.test(value)) return null
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed < 0 || parsed > 9999) return null
  return parsed
}

function validateEmployeeNoInput(raw: string, required: boolean): string | null {
  const value = raw.trim()
  if (!value) return required ? '工号必填。' : null
  if (!/^\d+$/.test(value)) return '工号必须是 0 到 9999 之间的纯数字。'
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed < 0 || parsed > 9999) return '工号必须是 0 到 9999 之间的纯数字。'
  return null
}

function teamMatchesDepartment(
  team: { department?: string },
  department: string,
  strictDepartment: boolean,
): boolean {
  if (!department) return false
  return strictDepartment ? team.department === department : !team.department || team.department === department
}

const invalidDepartmentScopeMessage = '当前账号部门范围已停用或不存在，请联系人事管理员或超级管理员修正组织归属。'

const systemDepartmentName = '未分配'

function isSystemDepartment(dept: Pick<OrgTreeDepartment, 'value'> | null | undefined): boolean {
  return dept?.value === systemDepartmentName
}

function isAssignableDepartment(department: string): boolean {
  const value = department.trim()
  return value !== '' && departmentOptions.value.some((item) => item.value === value)
}

function isAssignableTeam(department: string, team: string): boolean {
  const value = team.trim()
  if (!value) return true
  return teamOptions.value.some(
    (item) =>
      item.value === value &&
      (department ? teamMatchesDepartment(item, department, true) : true),
  )
}

// 列表筛选比"可分配"更宽:组织管理员可以点选停用组织,查看其中残留成员
// 以便治理(合并/删除前迁移人员)。
function isFilterableDepartment(department: string): boolean {
  if (isAssignableDepartment(department)) return true
  return canManageOrgMaster.value && !!findVisibleOrgDepartment(department)
}

function isFilterableTeam(department: string, team: string): boolean {
  if (isAssignableTeam(department, team)) return true
  return canManageOrgMaster.value && !!findVisibleOrgTeam(department, team)
}

function resetUsersForScopeError(message: string) {
  users.value = []
  pagination.value = { total: 0, page: page.value, page_size: pageSize.value }
  listError.value = message
}

function sanitizeListOrgFilters() {
  const department = departmentFilter.value.trim()
  const team = teamFilter.value.trim()
  if (department !== departmentFilter.value) departmentFilter.value = department
  if (team !== teamFilter.value) teamFilter.value = team
  if (department && !isFilterableDepartment(department)) {
    departmentFilter.value = ''
    teamFilter.value = ''
    return
  }
  if (team && !isFilterableTeam(department, team)) {
    teamFilter.value = ''
  }
}

function defaultCreateRoles(): string[] {
  return editableRoleCodeSet.value.has('Member') ? ['Member'] : []
}

function emptyCreateForm() {
  return {
    username: '',
    employee_no: '',
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

function userRowClassName(row: UserRow): string {
  return row.status === 'disabled' ? 'user-row--disabled' : ''
}

function renderUserStatusPill(status: string | undefined, extraClass = '') {
  return h(
    'span',
    {
      class: [
        'status-pill',
        status === 'active' ? 'status-pill--on' : 'status-pill--off',
        extraClass,
      ].filter(Boolean),
    },
    formatUserStatusForDisplay(status),
  )
}

function findVisibleOrgDepartment(department: string): OrgTreeDepartment | undefined {
  return visibleOrgTree.value.find((dept) => dept.value === department)
}

function findVisibleOrgTeam(department: string, team: string): OrgTreeTeam | undefined {
  return findVisibleOrgDepartment(department)?.teams.find((item) => item.value === team)
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

function activeOnlyOrgOptions(org: OrgOwnershipOptionsParsed): OrgOwnershipOptionsParsed {
  const departmentRecordsActive = (org.departmentRecords ?? []).filter((dept) => dept.enabled !== false)
  const activeDepartmentNames = new Set(departmentRecordsActive.map((dept) => dept.name))
  const activeDepartmentIds = new Set(departmentRecordsActive.map((dept) => dept.id))
  const teamRecordsActive = (org.teamRecords ?? []).filter(
    (team) => team.enabled !== false && activeDepartmentIds.has(team.departmentId),
  )
  const activeTeamKeys = new Set(
    teamRecordsActive.map((team) => `${team.departmentName ?? ''}@@${team.name}`),
  )
  const hasStructuredRecords = departmentRecordsActive.length > 0 || (org.departmentRecords ?? []).length > 0
  if (!hasStructuredRecords) return org
  return {
    departmentOptions: (org.departmentOptions ?? []).filter((dept) => activeDepartmentNames.has(dept.value)),
    teamOptions: (org.teamOptions ?? []).filter((team) => {
      if (!team.department) return false
      return activeTeamKeys.has(`${team.department}@@${team.value}`)
    }),
    departmentRecords: departmentRecordsActive,
    teamRecords: teamRecordsActive,
  }
}

async function loadOrgOptions() {
  let org: OrgOwnershipOptionsParsed
  try {
    org = await fetchOrgOwnershipOptions({
      includeDisabled: canManageOrgMaster.value,
      throwOnError: true,
    })
    orgOptionsError.value = ''
  } catch (e) {
    // 组织目录加载失败必须显式可见:静默空列表会让树和筛选凭空消失,
    // 管理员误以为组织被删除。
    orgOptionsError.value = e instanceof Error && e.message ? `组织列表加载失败：${e.message}` : '组织列表加载失败，请重试。'
    return
  }
  const activeOrg = activeOnlyOrgOptions(org)
  departmentOptions.value = activeOrg.departmentOptions ?? []
  teamOptions.value = activeOrg.teamOptions ?? []
  departmentRecords.value = org.departmentRecords ?? []
  teamRecords.value = org.teamRecords ?? []
  const hydrated = departmentsAndGroupsFromOrgOptions(activeOrg)
  if (hydrated) {
    usePermissionsStore().hydrateOrgFromServer(hydrated.departments, hydrated.groups)
  }
  sanitizeListOrgFilters()
  if (isDeptScopedOnly.value) {
    if (!currentDepartmentScope.value) {
      departmentFilter.value = ''
      teamFilter.value = ''
      createForm.value.department = ''
      return
    }
    if (!isAssignableDepartment(currentDepartmentScope.value)) {
      departmentFilter.value = ''
      teamFilter.value = ''
      createForm.value.department = ''
      resetUsersForScopeError(invalidDepartmentScopeMessage)
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

async function retryLoadOrgOptions() {
  await loadOrgOptions()
  if (!orgOptionsError.value) await loadUsers()
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
    const trimmedKeyword = appliedKeyword.value.trim()
    // 部门范围过滤：当前用户仅具备 `department.manage`（DeptAdmin）而无 `user.manage`
    // （HRAdmin/SuperAdmin）时，前端主动带上本部门 scope，避免后端因 scope 计算偏差
    // 泄漏跨部门列表。`user.manage` 持有者不附带 scope，保持全局视图。
    if (isDeptScopedOnly.value && !currentDepartmentScope.value) {
      users.value = []
      pagination.value = { total: 0, page: page.value, page_size: pageSize.value }
      listError.value = '当前账号缺少部门管理范围，请联系人事管理员或超级管理员修正组织归属。'
      return
    }
    if (isDeptScopedOnly.value && !isAssignableDepartment(currentDepartmentScope.value)) {
      resetUsersForScopeError(invalidDepartmentScopeMessage)
      return
    }
    sanitizeListOrgFilters()
    const deptScope = isDeptScopedOnly.value ? currentDepartmentScope.value : undefined
    const requestDepartment = deptScope || departmentFilter.value.trim()
    const requestTeam =
      teamFilter.value && isFilterableTeam(requestDepartment, teamFilter.value)
        ? teamFilter.value.trim()
        : ''
    const res = await usersApi.list({
      page: page.value,
      page_size: pageSize.value,
      ...(trimmedKeyword ? { keyword: trimmedKeyword } : {}),
      ...(statusFilter.value ? { status: statusFilter.value as 'active' | 'disabled' } : {}),
      ...(roleFilter.value ? { role: roleFilter.value } : {}),
      ...(requestDepartment ? { department: requestDepartment } : {}),
      ...(requestTeam ? { team: requestTeam } : {}),
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
    basicForm.value = {
      display_name: detailUser.value.display_name ?? '',
      employee_no: detailUser.value.employee_no == null ? '' : String(detailUser.value.employee_no),
    }
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
    const roles = ensureMemberRoleCodes(mergeRoleCodes(selectedEditableRoles, lockedDetailRoleCodes.value))
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

async function submitBasicInfo() {
  if (!detailUser.value) return
  const employeeNoError = validateEmployeeNoInput(basicForm.value.employee_no, true)
  if (employeeNoError) {
    detailActionMessage.value = employeeNoError
    return
  }
  const employeeNo = parseEmployeeNoInput(basicForm.value.employee_no)
  if (employeeNo == null) {
    detailActionMessage.value = '工号必须是 0 到 9999 之间的纯数字。'
    return
  }
  const displayName = basicForm.value.display_name.trim()
  if (!displayName) {
    detailActionMessage.value = '姓名必填。'
    return
  }
  basicSubmitting.value = true
  detailActionMessage.value = ''
  try {
    await usersApi.patch(detailUser.value.id, {
      display_name: displayName,
      employee_no: employeeNo,
    })
    await refreshDetailAndList(detailUser.value.id)
    detailActionMessage.value = '基本信息已保存'
  } catch (e) {
    detailActionMessage.value = e instanceof Error ? e.message : '基本信息保存失败'
  } finally {
    basicSubmitting.value = false
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
    // 归属变化会改变组织人数,同步刷新树上的人数徽标。
    void loadOrgOptions()
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
    void loadOrgOptions()
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
  const employeeNoError = validateEmployeeNoInput(f.employee_no, true)
  if (employeeNoError) return employeeNoError
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
    const employeeNo = parseEmployeeNoInput(f.employee_no)
    if (employeeNo == null) throw new Error('工号必须是 0 到 9999 之间的纯数字。')
    await usersApi.create({
      username: f.username.trim(),
      employee_no: employeeNo,
      display_name: f.display_name.trim(),
      department: f.department,
      team: f.team,
      mobile: f.mobile.trim(),
      email: f.email.trim() || undefined,
      password: f.password,
      roles: ensureMemberRoleCodes(f.roles.filter((role) => editableRoleCodeSet.value.has(role))),
      status: f.status,
    })
    showCreateModal.value = false
    createForm.value = emptyCreateForm()
    page.value = 1
    await loadUsers()
    // 新用户入组后同步刷新树上的人数徽标。
    void loadOrgOptions()
  } catch (e) {
    createError.value = e instanceof Error ? e.message : '创建用户失败'
  } finally {
    createSubmitting.value = false
  }
}

function openCreateDepartment() {
  orgAction.value = { mode: 'createDepartment' }
  orgActionName.value = ''
  orgActionDepartmentId.value = ''
  orgActionError.value = ''
}

function openEditDepartment(department: OrgTreeDepartment) {
  orgAction.value = { mode: 'editDepartment', department }
  orgActionName.value = department.label
  orgActionDepartmentId.value = ''
  orgActionError.value = ''
}

function openDeleteDepartment(department: OrgTreeDepartment) {
  orgAction.value = { mode: 'deleteDepartment', department }
  orgActionName.value = ''
  orgActionDepartmentId.value = ''
  orgActionError.value = ''
}

function openCreateTeam(department?: OrgTreeDepartment) {
  orgAction.value = { mode: 'createTeam', department }
  orgActionName.value = ''
  orgActionDepartmentId.value = department?.id ?? ''
  orgActionError.value = ''
}

function openEditTeam(team: OrgTreeTeam) {
  orgAction.value = { mode: 'editTeam', department: selectedOrgDepartment.value ?? undefined, team }
  orgActionName.value = team.label
  orgActionDepartmentId.value = ''
  orgActionError.value = ''
}

function openDeleteTeam(team: OrgTreeTeam) {
  orgAction.value = { mode: 'deleteTeam', department: selectedOrgDepartment.value ?? undefined, team }
  orgActionName.value = ''
  orgActionDepartmentId.value = ''
  orgActionError.value = ''
}

function openMergeDepartment(department: OrgTreeDepartment) {
  orgAction.value = { mode: 'mergeDepartment', department }
  orgActionName.value = ''
  orgActionDepartmentId.value = ''
  orgActionTargetId.value = ''
  orgActionError.value = ''
}

function openMergeTeam(team: OrgTreeTeam) {
  orgAction.value = { mode: 'mergeTeam', department: selectedOrgDepartment.value ?? undefined, team }
  orgActionName.value = ''
  orgActionDepartmentId.value = ''
  orgActionTargetId.value = ''
  orgActionError.value = ''
}

function openPurgeDepartment(department: OrgTreeDepartment) {
  orgAction.value = { mode: 'purgeDepartment', department }
  orgActionName.value = ''
  orgActionDepartmentId.value = ''
  orgActionError.value = ''
}

function openPurgeTeam(team: OrgTreeTeam) {
  orgAction.value = { mode: 'purgeTeam', department: selectedOrgDepartment.value ?? undefined, team }
  orgActionName.value = ''
  orgActionDepartmentId.value = ''
  orgActionError.value = ''
}

function openPurgeAllEmpty() {
  orgAction.value = { mode: 'purgeAllEmpty' }
  orgActionName.value = ''
  orgActionDepartmentId.value = ''
  orgActionTargetId.value = ''
  orgActionError.value = ''
}

function closeOrgAction() {
  if (orgActionSubmitting.value) return
  resetOrgAction()
}

function resetOrgAction() {
  orgAction.value = null
  orgActionName.value = ''
  orgActionDepartmentId.value = ''
  orgActionTargetId.value = ''
  orgActionError.value = ''
}

async function submitOrgAction() {
  const action = orgAction.value
  if (!action) return
  orgActionError.value = ''
  const name = orgActionName.value.trim()
  if (!orgActionIsDelete.value && !orgActionIsMerge.value && !name) {
    orgActionError.value = orgActionNamePlaceholder.value
    return
  }
  if (orgActionIsMerge.value && !orgActionTargetId.value) {
    orgActionError.value = '请选择合并目标。'
    return
  }
  orgActionSubmitting.value = true
  if (action.mode === 'purgeAllEmpty') {
    // 批量清理:逐项删除并收集失败原因;无论成败都刷新组织树,
    // 已删除的项立即从树中消失,失败项保留并在弹窗中给出原因。
    const { departments, teams } = removableDisabledOrgCleanup.value
    const failures: string[] = []
    try {
      for (const team of teams) {
        try {
          await deleteOrgTeam(team.id as string)
        } catch (e) {
          failures.push(`小组「${team.department} / ${team.label}」：${e instanceof Error ? e.message : '删除失败'}`)
        }
      }
      for (const dept of departments) {
        try {
          await deleteOrgDepartment(dept.id as string)
        } catch (e) {
          failures.push(`部门「${dept.label}」：${e instanceof Error ? e.message : '删除失败'}`)
        }
      }
      await loadOrgOptions()
      sanitizeListOrgFilters()
      await loadUsers()
      if (failures.length) {
        orgActionError.value = `部分组织未能删除：${failures.join('；')}`
        return
      }
      resetOrgAction()
    } finally {
      orgActionSubmitting.value = false
    }
    return
  }
  try {
    switch (action.mode) {
      case 'createDepartment':
        await createOrgDepartment({ name })
        break
      case 'editDepartment':
        if (!action.department?.id) throw new Error('未找到部门编号，无法保存。')
        await updateOrgDepartment(action.department.id, { name })
        break
      case 'deleteDepartment':
        if (!action.department?.id) throw new Error('未找到部门编号，无法处理。')
        await updateOrgDepartment(action.department.id, { enabled: action.department.enabled === false })
        break
      case 'createTeam': {
        const departmentId = orgActionDepartmentId.value || action.department?.id
        if (!departmentId) throw new Error('请选择部门。')
        await createOrgTeam({ department_id: departmentId, name })
        break
      }
      case 'editTeam':
        if (!action.team?.id) throw new Error('未找到小组编号，无法保存。')
        await updateOrgTeam(action.team.id, { name })
        break
      case 'deleteTeam':
        if (!action.team?.id) throw new Error('未找到小组编号，无法处理。')
        if (action.team.enabled === false && action.department?.enabled === false) {
          throw new Error('请先恢复部门，再恢复小组。')
        }
        await updateOrgTeam(action.team.id, { enabled: action.team.enabled === false })
        break
      case 'mergeDepartment':
        if (!action.department?.id) throw new Error('未找到部门编号，无法处理。')
        await mergeOrgDepartment(action.department.id, orgActionTargetId.value)
        break
      case 'mergeTeam':
        if (!action.team?.id) throw new Error('未找到小组编号，无法处理。')
        await mergeOrgTeam(action.team.id, orgActionTargetId.value)
        break
      case 'purgeDepartment':
        if (!action.department?.id) throw new Error('未找到部门编号，无法处理。')
        await deleteOrgDepartment(action.department.id)
        break
      case 'purgeTeam':
        if (!action.team?.id) throw new Error('未找到小组编号，无法处理。')
        await deleteOrgTeam(action.team.id)
        break
    }
    const departmentGone =
      (action.mode === 'deleteDepartment' || action.mode === 'mergeDepartment' || action.mode === 'purgeDepartment') &&
      action.department?.value &&
      departmentFilter.value === action.department.value
    const teamGone =
      (action.mode === 'deleteTeam' || action.mode === 'mergeTeam' || action.mode === 'purgeTeam') &&
      action.team?.value &&
      teamFilter.value === action.team.value &&
      departmentFilter.value === action.team.department
    resetOrgAction()
    await loadOrgOptions()
    if (departmentGone || teamGone) {
      departmentFilter.value = isDeptScopedOnly.value ? currentDepartmentScope.value : ''
      teamFilter.value = ''
    }
    await loadUsers()
  } catch (e) {
    orgActionError.value = e instanceof Error ? e.message : '组织维护失败'
  } finally {
    orgActionSubmitting.value = false
  }
}

function onSearch() {
  appliedKeyword.value = keyword.value.trim()
  page.value = 1
  void loadUsers()
}

// 清空输入框即恢复全量:不必再点一次"查询"才能取消关键词过滤。
watch(keyword, () => {
  if (keyword.value.trim() === '' && appliedKeyword.value !== '') {
    appliedKeyword.value = ''
    page.value = 1
    void loadUsers()
  }
})

function goPage(next: number) {
  const target = Math.max(1, Math.min(totalPages.value, next))
  if (target === page.value) return
  page.value = target
  void loadUsers()
}

watch(departmentFilter, () => {
  if (!departmentFilter.value || !teamFilter.value) return
  // 树中的停用小组也允许保留(治理场景);仅当小组既不在活跃选项、
  // 也不在树里时才清空。
  if (findVisibleOrgTeam(departmentFilter.value, teamFilter.value)) return
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
    if (!createForm.value.department) {
      createForm.value.team = ''
      return
    }
    const ok = teamOptions.value.some(
      (t) =>
        t.value === createForm.value.team &&
        teamMatchesDepartment(t, createForm.value.department, true),
    )
    if (!ok) createForm.value.team = ''
  },
)

watch(
  () => membershipForm.value.department,
  () => {
    if (!membershipForm.value.department) {
      membershipForm.value.team = ''
      return
    }
    const ok = teamOptions.value.some(
      (t) =>
        t.value === membershipForm.value.team &&
        teamMatchesDepartment(t, membershipForm.value.department, true),
    )
    if (!ok) membershipForm.value.team = ''
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

<!-- 弹窗共用样式:与 UserDetailModal / UserCreateModal 共享同一份 scoped 规则 -->
<style scoped src="./userManagementModal.css"></style>

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

.list-heading {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem 0.75rem;
  margin-bottom: 0.75rem;
}

.list-heading-scope {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.6rem;
}

.list-heading .section-title {
  margin: 0;
  overflow-wrap: anywhere;
}

.org-scope-clear {
  padding: 0.15rem 0.5rem;
  border: 1px solid rgb(var(--yb-border-zinc));
  border-radius: 9999px;
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-zinc-soft));
  cursor: pointer;
  font-size: 0.6875rem;
  font-weight: 600;
  white-space: nowrap;
  transition: color 0.12s ease, border-color 0.12s ease, background-color 0.12s ease;
}

.org-scope-clear:hover {
  color: rgb(var(--yb-text-zinc));
  border-color: rgb(var(--yb-text-zinc-faint));
  background: rgb(var(--yb-surface-row-even));
}

.directory-count {
  font-size: 0.75rem;
  color: rgb(var(--yb-text-zinc-soft));
  font-variant-numeric: tabular-nums;
}

.employee-missing {
  color: rgb(var(--yb-warning-dark));
  font-size: 0.75rem;
  font-weight: 600;
}

.management-layout {
  display: grid;
  grid-template-columns: minmax(16.5rem, clamp(18rem, 23vw, 21.5rem)) minmax(0, 1fr);
  gap: 1.25rem;
  align-items: start;
}

.org-filter-panel {
  /* 跟随页面滚动吸顶:表格再长,组织树也一直在视野内 */
  position: sticky;
  top: 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  max-height: calc(100dvh - 1.5rem);
  overflow: auto;
  padding-right: 0.9rem;
  border-right: 1px solid rgb(var(--yb-border-zinc));
}

.org-filter-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.25rem;
  color: rgb(var(--yb-text-zinc-strong));
  font-size: 0.75rem;
  font-weight: 700;
}

.org-selected-actions {
  /* UX2:操作条吸顶,树滚到任何位置都能直接编辑当前选中组织 */
  position: sticky;
  top: 0;
  z-index: 5;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  margin-bottom: 0.35rem;
  padding: 0.55rem;
  border: 1px solid rgb(var(--yb-border-zinc));
  border-radius: 0.5rem;
  background: rgb(var(--yb-surface-row-even));
  box-shadow: 0 2px 6px rgb(var(--yb-shadow) / 0.08);
}

.org-load-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.5rem 0.55rem;
  border: 1px solid rgb(var(--yb-danger-border-soft));
  border-radius: 0.5rem;
  background: rgb(var(--yb-danger) / 0.06);
}

.org-load-error p {
  margin: 0;
  color: rgb(var(--yb-danger));
  font-size: 0.75rem;
}

.org-cleanup-btn {
  width: 100%;
  padding: 0.4rem 0.55rem;
  border: 1px dashed rgb(var(--yb-danger-border-soft));
  border-radius: 0.5rem;
  background: rgb(var(--yb-danger) / 0.04);
  color: rgb(var(--yb-danger));
  font-size: 0.75rem;
  font-weight: 600;
  cursor: pointer;
  text-align: left;
  transition: background-color 0.12s ease, border-color 0.12s ease;
}

.org-cleanup-btn:hover {
  background: rgb(var(--yb-danger) / 0.1);
  border-color: rgb(var(--yb-danger));
}

.org-selected-member-count {
  color: rgb(var(--yb-text-zinc-faint));
  font-variant-numeric: tabular-nums;
}

.org-selected-current {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
}

.org-selected-current strong {
  min-width: 0;
  color: rgb(var(--yb-text-zinc-strong));
  font-size: 0.8125rem;
  font-weight: 700;
  overflow-wrap: anywhere;
}

.org-selected-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.org-state-pill {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  min-height: 1rem;
  padding: 0.05rem 0.35rem;
  border-radius: 9999px;
  background: rgb(var(--yb-success-soft));
  color: rgb(var(--yb-success-deep));
  font-size: 0.625rem;
  font-weight: 700;
}

.org-state-pill--off {
  background: rgb(var(--yb-danger) / 0.1);
  color: rgb(var(--yb-danger));
}

.org-action-btn,
.org-icon-btn {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  justify-content: center;
  min-height: 1.55rem;
  padding: 0.18rem 0.5rem;
  border-radius: 0.35rem;
  border: 1px solid rgb(var(--yb-border-zinc));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-zinc));
  cursor: pointer;
  font-size: 0.6875rem;
  font-weight: 600;
  text-align: center;
  white-space: normal;
}

.org-action-btn:hover,
.org-icon-btn:hover {
  background: rgb(var(--yb-surface-row-even));
  border-color: rgb(var(--yb-text-zinc-faint));
}

.org-icon-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.org-icon-btn--danger {
  color: rgb(var(--yb-danger));
  border-color: rgb(var(--yb-danger-border-soft));
}

.user-list-panel {
  min-width: 0;
}

@media (min-width: 640px) {
  .content-card {
    padding: 1.5rem;
  }
}

@media (max-width: 960px) {
  .management-layout {
    grid-template-columns: 1fr;
  }

  .org-filter-panel {
    position: static;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    max-height: none;
    overflow: visible;
    padding-right: 0;
    padding-bottom: 0.75rem;
    border-right: 0;
    border-bottom: 1px solid rgb(var(--yb-border-zinc));
  }

  .org-filter-header {
    grid-column: auto;
  }

  .org-selected-actions {
    grid-column: auto;
  }
}

.toolbar {
  display: grid;
  grid-template-columns: minmax(0, 2fr) repeat(2, minmax(0, 1fr)) auto;
  gap: 0.75rem;
  margin-bottom: 0.6rem;
  align-items: end;
}

.toolbar-field {
  min-width: 0;
}

.merge-hint {
  margin: 0;
  color: rgb(var(--yb-text-zinc-soft));
  font-size: 0.8125rem;
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

.user-name-cell {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
}

.user-name-text {
  min-width: 0;
  overflow-wrap: anywhere;
  color: rgb(var(--yb-text-zinc-strong));
  font-weight: 650;
}

.status-pill {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  min-width: 3.2rem;
  padding: 0.12rem 0.5rem;
  border: 1px solid transparent;
  border-radius: 9999px;
  font-size: 0.6875rem;
  font-weight: 750;
  line-height: 1.2;
}

.status-pill--inline {
  min-width: 0;
  padding-inline: 0.42rem;
}

.status-pill--on {
  background: rgb(var(--yb-success-soft));
  border-color: rgb(var(--yb-success-border));
  color: rgb(var(--yb-success-deep));
}

.status-pill--off {
  background: rgb(var(--yb-danger) / 0.14);
  border-color: rgb(var(--yb-danger-border-soft));
  color: rgb(var(--yb-danger));
}

:deep(.user-row--disabled .n-data-table-td) {
  background: rgb(var(--yb-danger) / 0.025);
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

/* 表格内部门/组名:可点击反向联动左侧组织树 */
.org-link-btn {
  padding: 0;
  border: none;
  background: none;
  color: inherit;
  font: inherit;
  cursor: pointer;
  text-decoration: underline;
  text-decoration-color: transparent;
  text-underline-offset: 0.2em;
  transition: color 0.12s ease, text-decoration-color 0.12s ease;
}

.org-link-btn:hover {
  color: rgb(var(--yb-success-emerald));
  text-decoration-color: currentColor;
}

/* 批量清理弹窗内的组织清单 */
.um-modal .purge-list {
  margin: 0.5rem 0 0;
  padding-left: 1.1rem;
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.management-tabs {
  display: flex;
  gap: 0.4rem;
  margin: 0.9rem 0;
  padding: 0.25rem;
  width: max-content;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-soft));
}

.management-tabs button {
  min-height: 2.25rem;
  padding: 0 0.85rem;
  border: 0;
  border-radius: 0.55rem;
  background: transparent;
  color: rgb(var(--yb-text-muted));
  cursor: pointer;
  font-weight: 700;
}

.management-tabs button.active {
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-brand));
  box-shadow: 0 1px 3px rgb(var(--yb-shadow) / 0.08);
}

.access-workspace-card {
  padding: 1rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 1rem;
  background: rgb(var(--yb-surface));
}

.org-icon-btn--policy {
  border-color: rgb(var(--yb-brand-border));
  color: rgb(var(--yb-brand));
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
</style>
