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
          <aside class="org-filter-panel" aria-label="组织列表与筛选">
            <div class="org-filter-header">
              <div>
                <span>组织列表</span>
                <small v-if="canManageOrgMaster">含停用组织</small>
              </div>
              <button
                v-if="canManageOrgMaster"
                type="button"
                class="org-action-btn"
                @click="openCreateDepartment"
              >
                新增部门
              </button>
            </div>
            <div v-if="canManageOrgMaster && selectedOrgDepartment" class="org-selected-actions">
              <div class="org-selected-current">
                <span class="org-selected-label">当前组织</span>
                <strong>{{ selectedOrgTeam ? selectedOrgTeam.label : selectedOrgDepartment.label }}</strong>
                <span class="org-selected-path">
                  {{ selectedOrgTeam ? selectedOrgDepartment.label : '部门' }}
                  <span
                    class="org-state-pill"
                    :class="{ 'org-state-pill--off': selectedOrgTeam ? !selectedOrgTeam.enabled : !selectedOrgDepartment.enabled }"
                  >
                    {{ (selectedOrgTeam ? selectedOrgTeam.enabled : selectedOrgDepartment.enabled) ? '启用' : '停用' }}
                  </span>
                </span>
              </div>
              <div class="org-selected-buttons">
                <button
                  v-if="selectedOrgDepartment.enabled"
                  type="button"
                  class="org-icon-btn"
                  @click.stop="openCreateTeam(selectedOrgDepartment)"
                >
                  新增小组
                </button>
                <button type="button" class="org-icon-btn" @click.stop="openEditDepartment(selectedOrgDepartment)">部门改名</button>
                <button
                  type="button"
                  class="org-icon-btn"
                  :class="{ 'org-icon-btn--danger': selectedOrgDepartment.enabled }"
                  @click.stop="openDeleteDepartment(selectedOrgDepartment)"
                >
                  {{ selectedOrgDepartment.enabled ? '停用部门' : '恢复部门' }}
                </button>
                <button v-if="selectedOrgTeam" type="button" class="org-icon-btn" @click.stop="openEditTeam(selectedOrgTeam)">小组改名</button>
                <button
                  v-if="selectedOrgTeam"
                  type="button"
                  class="org-icon-btn"
                  :class="{ 'org-icon-btn--danger': selectedOrgTeam.enabled }"
                  :disabled="!selectedOrgDepartment.enabled && !selectedOrgTeam.enabled"
                  :title="!selectedOrgDepartment.enabled && !selectedOrgTeam.enabled ? '请先恢复部门' : undefined"
                  @click.stop="openDeleteTeam(selectedOrgTeam)"
                >
                  {{ selectedOrgTeam.enabled ? '停用小组' : '恢复小组' }}
                </button>
              </div>
            </div>
            <div class="org-tree-scroll">
              <button
                v-if="!isDeptScopedOnly"
                type="button"
                class="org-filter-item org-filter-item--all"
                :class="{ 'is-active': isAllOrgFilterActive }"
                @click="selectAllOrg"
              >
                <span class="org-item-name">全部组织</span>
              </button>
              <div v-if="enabledOrgTree.length" class="org-tree-section">
                <div class="org-tree-section-title">启用组织</div>
                <div v-for="dept in enabledOrgTree" :key="'enabled-' + dept.value" class="org-filter-dept">
                  <div class="org-filter-row">
                    <button
                      type="button"
                      class="org-filter-item org-filter-item--dept"
                      :class="{ 'is-active': isOrgDepartmentActive(dept.value) }"
                      @click="selectOrgDepartment(dept.value)"
                    >
                      <span class="org-item-name">{{ dept.label }}</span>
                    </button>
                  </div>
                  <div v-if="dept.teams.length" class="org-filter-teams">
                    <div v-for="team in dept.teams" :key="`enabled-${dept.value}-${team.value}`" class="org-filter-row">
                      <button
                        type="button"
                        class="org-filter-item org-filter-item--team"
                        :class="{ 'is-active': isOrgTeamActive(dept.value, team.value) }"
                        @click="selectOrgTeam(dept.value, team.value)"
                      >
                        <span class="org-item-name">{{ team.label }}</span>
                      </button>
                    </div>
                  </div>
                </div>
              </div>
              <div v-if="disabledOrgTree.length" class="org-tree-section org-tree-section--disabled">
                <div class="org-tree-section-title">停用组织</div>
                <div v-for="dept in disabledOrgTree" :key="'disabled-' + dept.value" class="org-filter-dept">
                  <div class="org-filter-row">
                    <button
                      type="button"
                      class="org-filter-item org-filter-item--dept"
                      :class="{ 'is-active': isOrgDepartmentActive(dept.value), 'is-disabled': !dept.enabled }"
                      @click="selectOrgDepartment(dept.value)"
                    >
                      <span class="org-item-name">{{ dept.label }}</span>
                      <span v-if="!dept.enabled" class="org-state-pill org-state-pill--off">停用</span>
                    </button>
                  </div>
                  <div v-if="dept.teams.length" class="org-filter-teams">
                    <div v-for="team in dept.teams" :key="`disabled-${dept.value}-${team.value}`" class="org-filter-row">
                      <button
                        type="button"
                        class="org-filter-item org-filter-item--team"
                        :class="{ 'is-active': isOrgTeamActive(dept.value, team.value), 'is-disabled': !team.enabled }"
                        @click="selectOrgTeam(dept.value, team.value)"
                      >
                        <span class="org-item-name">{{ team.label }}</span>
                        <span v-if="!team.enabled" class="org-state-pill org-state-pill--off">停用</span>
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </aside>
          <div class="user-list-panel">
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
              <h3 class="section-title">用户详情：{{ detailUser.display_name || detailUser.username }}</h3>
              <p class="modal-subtitle">
                {{ formatEmployeeNo(detailUser.employee_no) }} · {{ detailUser.username }} · {{ formatUserStatusForDisplay(detailUser.status) }}
              </p>
              <p class="modal-subtitle">当前角色：{{ formatWorkflowRolesForDisplay(detailUser.roles) }}</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭角色管理" @click="detailUser = null">
              ×
            </button>
          </header>
          <div class="modal-body">
            <div v-if="detailLoading" class="py-4"><BaseSkeleton width="100%" height="4rem" /></div>
            <div v-else class="modal-stack">
              <section class="detail-section">
                <header class="detail-section-header">
                  <h4>基本信息</h4>
                  <button
                    v-if="canEditBasicInfo"
                    type="button"
                    class="um-btn um-btn--primary um-btn--sm"
                    :disabled="basicSubmitting"
                    @click="submitBasicInfo"
                  >
                    {{ basicSubmitting ? '保存中...' : '保存基本信息' }}
                  </button>
                </header>
                <div class="detail-grid">
                  <label class="field-label">
                    <span>姓名</span>
                    <input v-model.trim="basicForm.display_name" class="input" :disabled="!canEditBasicInfo" />
                  </label>
                  <label class="field-label">
                    <span>工号</span>
                    <input
                      v-model.trim="basicForm.employee_no"
                      class="input"
                      inputmode="numeric"
                      placeholder="0-9999"
                      :disabled="!canEditBasicInfo"
                    />
                  </label>
                  <div class="readonly-field">
                    <span>登录账号</span>
                    <strong>{{ detailUser.username }}</strong>
                  </div>
                  <div class="readonly-field">
                    <span>账号状态</span>
                    <strong>{{ formatUserStatusForDisplay(detailUser.status) }}</strong>
                  </div>
                </div>
              </section>

              <section v-if="canMoveTeam" class="detail-section">
                <header class="detail-section-header">
                  <h4>组织归属</h4>
                </header>
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
              </section>

              <section class="detail-section">
                <header class="detail-section-header">
                  <h4>角色权限</h4>
                  <button
                    v-if="canAssignRoles"
                    type="button"
                    class="um-btn um-btn--primary um-btn--sm"
                    :disabled="roleSubmitting"
                    @click="submitRoleReplace"
                  >
                    {{ roleSubmitting ? '提交中...' : '保存角色' }}
                  </button>
                </header>
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
                          :disabled="!canAssignRoles || role.code === 'Member'"
                        />
                        <span>{{ role.display }}</span>
                        <em v-if="role.code === 'Member'">基础身份，不能移除</em>
                      </label>
                    </div>
                  </section>
                </div>
                <p v-else class="role-readonly-hint">当前账号没有可分配角色，角色信息仅可查看。</p>
              </section>

              <section v-if="canResetPassword" class="detail-section">
                <header class="detail-section-header">
                  <h4>账号安全</h4>
                </header>
                <div class="password-row">
                  <input v-model="resetPasswordValue" class="input" placeholder="输入新密码" />
                  <button type="button" class="um-btn um-btn--primary um-btn--sm" :disabled="passwordSubmitting" @click="resetPassword">
                    {{ passwordSubmitting ? '重置中...' : '重置密码' }}
                  </button>
                </div>
              </section>

              <section v-if="canDisableUser" class="detail-section detail-section--danger">
                <header class="detail-section-header">
                  <h4>账号状态</h4>
                  <button
                    v-if="detailUser.status !== 'disabled'"
                    type="button"
                    class="um-btn um-btn--ghost um-btn--sm"
                    :disabled="statusSubmitting"
                    @click="setUserStatus('disabled')"
                  >
                    {{ statusSubmitting ? '处理中...' : '停用账号' }}
                  </button>
                  <button
                    v-else
                    type="button"
                    class="um-btn um-btn--primary um-btn--sm"
                    :disabled="statusSubmitting"
                    @click="setUserStatus('active')"
                  >
                    {{ statusSubmitting ? '处理中...' : '启用账号' }}
                  </button>
                </header>
                <p class="role-readonly-hint">停用后该账号不能登录，历史任务、角色、工号和组织归属会保留。</p>
              </section>
              <p v-if="detailActionMessage" class="action-msg">{{ detailActionMessage }}</p>
            </div>
          </div>
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
              <input v-model.trim="createForm.employee_no" class="input" inputmode="numeric" placeholder="工号（0-9999）" />
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
                    <input v-model="createForm.roles" type="checkbox" :value="role.code" :disabled="role.code === 'Member'" />
                    <span>{{ role.display }}</span>
                    <em v-if="role.code === 'Member'">基础身份，不能移除</em>
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
import { usersApi } from '@/services/api/usersApi'
import {
  createOrgDepartment,
  createOrgTeam,
  departmentsAndGroupsFromOrgOptions,
  fetchOrgOwnershipOptions,
  updateOrgDepartment,
  updateOrgTeam,
  type OrgDepartmentRecord,
  type OrgOwnershipOptionsParsed,
  type OrgTeamRecord,
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
import { RoleEnum } from '@/types'

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

interface UserRow {
  id: string
  username: string
  employee_no?: number | null
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
  id?: string
  value: string
  label: string
  department: string
  enabled: boolean
}

interface OrgTreeDepartment {
  id?: string
  value: string
  label: string
  enabled: boolean
  teams: OrgTreeTeam[]
}

type OrgActionMode =
  | 'createDepartment'
  | 'editDepartment'
  | 'deleteDepartment'
  | 'createTeam'
  | 'editTeam'
  | 'deleteTeam'

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
const keyword = ref('')
const statusFilter = ref('')
const roleFilter = ref('')
const departmentFilter = ref('')
const teamFilter = ref('')
const orgMasterDepartmentFilter = ref('')
const orgMasterTeamFilter = ref('')

const departmentOptions = ref<Array<{ value: string; label: string }>>([])
const teamOptions = ref<Array<{ value: string; label: string; department?: string }>>([])
const departmentRecords = ref<OrgDepartmentRecord[]>([])
const teamRecords = ref<OrgTeamRecord[]>([])
const orgAction = ref<OrgActionState | null>(null)
const orgActionName = ref('')
const orgActionDepartmentId = ref('')
const orgActionSubmitting = ref(false)
const orgActionError = ref('')
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
      teams: teamRecords.value
        .filter((team) => team.departmentId === dept.id || team.departmentName === dept.name)
        .map((team) => ({
          id: team.id,
          value: team.name,
          label: team.name,
          department: dept.name,
          enabled: team.enabled !== false,
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
  visibleOrgTree.value.find((dept) => dept.value === orgMasterDepartmentFilter.value) ?? null,
)
const selectedOrgTeam = computed<OrgTreeTeam | null>(() => {
  if (!selectedOrgDepartment.value || !orgMasterTeamFilter.value) return null
  return selectedOrgDepartment.value.teams.find((team) => team.value === orgMasterTeamFilter.value) ?? null
})
const isAllOrgFilterActive = computed(
  () => !isDeptScopedOnly.value && !departmentFilter.value && !teamFilter.value && !orgMasterDepartmentFilter.value,
)

const showCreateModal = ref(false)
const createSubmitting = ref(false)
const createError = ref('')
const createForm = ref({
  username: '',
  employee_no: '',
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
const orgActionIsDelete = computed(() =>
  orgAction.value?.mode === 'deleteDepartment' || orgAction.value?.mode === 'deleteTeam',
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
  return ''
})
const orgActionNamePlaceholder = computed(() =>
  orgAction.value?.mode === 'createTeam' || orgAction.value?.mode === 'editTeam'
    ? '请输入小组名称'
    : '请输入部门名称',
)
const orgActionConfirmText = computed(() => {
  if (!orgActionIsDelete.value) return '保存'
  if (orgAction.value?.mode === 'deleteDepartment' && orgAction.value.department?.enabled === false) return '确认恢复'
  if (orgAction.value?.mode === 'deleteTeam' && orgAction.value.team?.enabled === false) return '确认恢复'
  return '确认停用'
})
const userTableColumns = computed<DataTableColumns<UserRow>>(() => [
  {
    title: '姓名',
    key: 'display_name',
    width: 150,
    render: (row) => row.display_name || '-',
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
        '管理',
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

function formatEmployeeNo(value: number | null | undefined): string {
  return value == null ? '待维护工号' : `工号 ${value}`
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

const invalidDepartmentScopeMessage = '当前账号部门范围已停用或不存在，请联系人事管理员或超级管理员修正组织归属。'

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
  if (department && !isAssignableDepartment(department)) {
    departmentFilter.value = ''
    teamFilter.value = ''
    return
  }
  if (team && !isAssignableTeam(department, team)) {
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

function findVisibleOrgDepartment(department: string): OrgTreeDepartment | undefined {
  return visibleOrgTree.value.find((dept) => dept.value === department)
}

function findVisibleOrgTeam(department: string, team: string): OrgTreeTeam | undefined {
  return findVisibleOrgDepartment(department)?.teams.find((item) => item.value === team)
}

function isOrgDepartmentActive(department: string): boolean {
  return orgMasterDepartmentFilter.value === department && !orgMasterTeamFilter.value
}

function isOrgTeamActive(department: string, team: string): boolean {
  return orgMasterDepartmentFilter.value === department && orgMasterTeamFilter.value === team
}

function selectAllOrg() {
  if (isDeptScopedOnly.value) return
  orgMasterDepartmentFilter.value = ''
  orgMasterTeamFilter.value = ''
  departmentFilter.value = ''
  teamFilter.value = ''
}

function selectOrgDepartment(department: string) {
  const dept = findVisibleOrgDepartment(department)
  orgMasterDepartmentFilter.value = department
  orgMasterTeamFilter.value = ''
  if (!dept || !dept.enabled) return
  departmentFilter.value = department
  teamFilter.value = ''
}

function selectOrgTeam(department: string, team: string) {
  const dept = findVisibleOrgDepartment(department)
  const orgTeam = findVisibleOrgTeam(department, team)
  orgMasterDepartmentFilter.value = department
  orgMasterTeamFilter.value = team
  if (!dept?.enabled || !orgTeam?.enabled) return
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
  const org = await fetchOrgOwnershipOptions({ includeDisabled: canManageOrgMaster.value })
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
  if (orgMasterDepartmentFilter.value && !findVisibleOrgDepartment(orgMasterDepartmentFilter.value)) {
    orgMasterDepartmentFilter.value = ''
    orgMasterTeamFilter.value = ''
  } else if (
    orgMasterDepartmentFilter.value &&
    orgMasterTeamFilter.value &&
    !findVisibleOrgTeam(orgMasterDepartmentFilter.value, orgMasterTeamFilter.value)
  ) {
    orgMasterTeamFilter.value = ''
  }
  if (isDeptScopedOnly.value) {
    if (!currentDepartmentScope.value) {
      departmentFilter.value = ''
      teamFilter.value = ''
      orgMasterDepartmentFilter.value = ''
      orgMasterTeamFilter.value = ''
      createForm.value.department = ''
      return
    }
    if (!isAssignableDepartment(currentDepartmentScope.value)) {
      departmentFilter.value = ''
      teamFilter.value = ''
      orgMasterDepartmentFilter.value = ''
      orgMasterTeamFilter.value = ''
      createForm.value.department = ''
      resetUsersForScopeError(invalidDepartmentScopeMessage)
      return
    }
    departmentFilter.value = currentDepartmentScope.value
    orgMasterDepartmentFilter.value = currentDepartmentScope.value
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
    if (isDeptScopedOnly.value && !isAssignableDepartment(currentDepartmentScope.value)) {
      resetUsersForScopeError(invalidDepartmentScopeMessage)
      return
    }
    sanitizeListOrgFilters()
    const deptScope = isDeptScopedOnly.value ? currentDepartmentScope.value : undefined
    const requestDepartment = deptScope || departmentFilter.value.trim()
    const requestTeam =
      teamFilter.value && isAssignableTeam(requestDepartment, teamFilter.value)
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

function closeOrgAction() {
  if (orgActionSubmitting.value) return
  resetOrgAction()
}

function resetOrgAction() {
  orgAction.value = null
  orgActionName.value = ''
  orgActionDepartmentId.value = ''
  orgActionError.value = ''
}

async function submitOrgAction() {
  const action = orgAction.value
  if (!action) return
  orgActionError.value = ''
  const name = orgActionName.value.trim()
  if (!orgActionIsDelete.value && !name) {
    orgActionError.value = orgActionNamePlaceholder.value
    return
  }
  orgActionSubmitting.value = true
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
    }
    const shouldResetDepartment =
      action.mode === 'deleteDepartment' &&
      action.department?.value &&
      departmentFilter.value === action.department.value
    const shouldResetTeam =
      action.mode === 'deleteTeam' &&
      action.team?.value &&
      teamFilter.value === action.team.value &&
      departmentFilter.value === action.team.department
    resetOrgAction()
    await loadOrgOptions()
    if (shouldResetDepartment || shouldResetTeam) {
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

.employee-missing {
  color: rgb(var(--yb-warning-dark));
  font-size: 0.75rem;
  font-weight: 600;
}

.management-layout {
  display: grid;
  grid-template-columns: minmax(15rem, 18rem) minmax(0, 1fr);
  gap: 1.15rem;
  align-items: start;
}

.org-filter-panel {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  max-height: min(66dvh, 44rem);
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

.org-filter-header > div {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.12rem;
}

.org-filter-header small {
  color: rgb(var(--yb-text-zinc-faint));
  font-size: 0.6875rem;
  font-weight: 500;
}

.org-selected-actions {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  margin-bottom: 0.35rem;
  padding: 0.55rem;
  border: 1px solid rgb(var(--yb-border-zinc));
  border-radius: 0.5rem;
  background: rgb(var(--yb-surface-row-even));
}

.org-selected-current {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.1rem;
}

.org-selected-label {
  color: rgb(var(--yb-text-zinc-faint));
  font-size: 0.6875rem;
  font-weight: 600;
}

.org-selected-current strong {
  min-width: 0;
  overflow: hidden;
  color: rgb(var(--yb-text-zinc-strong));
  font-size: 0.8125rem;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.org-selected-path {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.35rem;
  color: rgb(var(--yb-text-zinc-soft));
  font-size: 0.6875rem;
}

.org-selected-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.org-tree-scroll {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.45rem;
}

.org-tree-section {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.25rem;
}

.org-tree-section-title {
  margin: 0.2rem 0 0.05rem;
  color: rgb(var(--yb-text-zinc-faint));
  font-size: 0.6875rem;
  font-weight: 700;
}

.org-tree-section--disabled {
  padding-top: 0.35rem;
  border-top: 1px dashed rgb(var(--yb-border-zinc));
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

.org-filter-row {
  display: block;
}

.org-filter-item {
  display: flex;
  width: 100%;
  min-height: 2rem;
  align-items: center;
  justify-content: space-between;
  gap: 0.35rem;
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

.org-filter-item .org-item-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.org-filter-item.is-disabled {
  color: rgb(var(--yb-text-zinc-faint));
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
  background: rgb(var(--yb-surface-neutral-muted));
  color: rgb(var(--yb-text-zinc-soft));
}

.org-action-btn,
.org-icon-btn {
  display: inline-flex;
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
  white-space: nowrap;
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

  .org-filter-dept,
  .org-filter-teams,
  .org-tree-scroll,
  .org-tree-section {
    display: flex;
  }

  .org-filter-header {
    grid-column: auto;
  }

  .org-selected-actions,
  .org-tree-section-title {
    grid-column: auto;
  }

  .org-filter-row {
    display: block;
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

.um-modal .form-grid--single {
  grid-template-columns: 1fr;
}

.um-modal .detail-section {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 0.75rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
}

.um-modal .detail-section--danger {
  border-color: rgb(var(--yb-warning-border-soft));
  background: rgb(var(--yb-warning-soft));
}

.um-modal .detail-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.um-modal .detail-section-header h4 {
  margin: 0;
  color: rgb(var(--yb-text));
  font-size: 0.875rem;
  font-weight: 700;
}

.um-modal .detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.6rem;
}

.um-modal .field-label,
.um-modal .readonly-field {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.3rem;
}

.um-modal .field-label span,
.um-modal .readonly-field span {
  color: rgb(var(--yb-text-muted));
  font-size: 0.75rem;
  font-weight: 600;
}

.um-modal .readonly-field strong {
  display: flex;
  align-items: center;
  min-height: 2.5rem;
  padding: 0.45rem 0.7rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.625rem;
  background: rgb(var(--yb-surface-soft));
  color: rgb(var(--yb-text));
  font-size: 0.8125rem;
  font-weight: 600;
  overflow-wrap: anywhere;
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

.um-modal .role-check em {
  margin-left: auto;
  color: rgb(var(--yb-text-muted));
  font-size: 0.6875rem;
  font-style: normal;
  white-space: nowrap;
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

.um-modal .delete-confirm-box {
  padding: 0.8rem;
  border-radius: 0.75rem;
  border: 1px solid rgb(var(--yb-warning-border-soft));
  background: rgb(var(--yb-warning-soft));
  color: rgb(var(--yb-text));
  font-size: 0.8125rem;
  line-height: 1.6;
}

.um-modal .delete-confirm-box p {
  margin: 0;
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

  .um-modal .detail-grid {
    grid-template-columns: 1fr;
  }

  .um-modal .detail-section-header {
    align-items: stretch;
    flex-direction: column;
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
