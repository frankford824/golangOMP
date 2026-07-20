<template>
  <div class="modal-mask" @click.self="emit('close')">
    <div class="modal-panel um-modal um-modal--wide">
      <header class="modal-header">
        <div class="modal-heading">
          <h3 class="section-title">用户详情：{{ user.display_name || user.username }}</h3>
          <p class="modal-subtitle">
            {{ formatEmployeeNo(user.employee_no) }} · {{ user.username }} · {{ formatUserStatusForDisplay(user.status) }}
          </p>
          <p class="modal-subtitle">当前角色：{{ currentRoleNames }}</p>
        </div>
        <button type="button" class="modal-close" aria-label="关闭角色管理" @click="emit('close')">
          ×
        </button>
      </header>
      <div class="modal-body">
        <div v-if="loading" class="py-4"><BaseSkeleton width="100%" height="4rem" /></div>
        <div v-else class="modal-stack">
          <section class="detail-section">
            <header class="detail-section-header">
              <h4>基本信息</h4>
              <button
                v-if="canEditBasicInfo"
                type="button"
                class="um-btn um-btn--primary um-btn--sm"
                :disabled="basicSubmitting"
                @click="emit('submit-basic')"
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
                <strong>{{ user.username }}</strong>
              </div>
              <div class="readonly-field">
                <span>账号状态</span>
                <strong>{{ formatUserStatusForDisplay(user.status) }}</strong>
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
                <option v-for="t in membershipTeamOptions" :key="`${t.department ?? ''}-${t.value}`" :value="t.value">{{ t.label }}</option>
              </select>
              <button
                type="button"
                class="um-btn um-btn--primary um-btn--sm"
                :disabled="membershipSubmitting || !isMembershipDirty"
                @click="emit('submit-membership')"
              >
                {{ membershipSubmitting ? '保存中...' : '保存归属' }}
              </button>
              <button
                v-if="canClearMembership && (user.department || user.team)"
                type="button"
                class="um-btn um-btn--ghost um-btn--sm"
                :disabled="membershipSubmitting"
                @click="emit('clear-membership')"
              >
                移出到未分组
              </button>
            </div>
          </section>

          <section class="detail-section">
            <header class="detail-section-header">
              <h4>角色权限</h4>
              <div class="detail-section-actions">
                <button
                  v-if="canAssignRoles"
                  type="button"
                  class="um-btn um-btn--primary um-btn--sm"
                  :disabled="roleSubmitting"
                  @click="emit('submit-roles')"
                >
                  {{ roleSubmitting ? '提交中...' : '保存角色' }}
                </button>
              </div>
            </header>
            <div v-if="lockedRoleOptions.length" class="legacy-role-box">
              <span class="legacy-role-title">{{ lockedRoleTitle }}</span>
              <span
                v-for="role in lockedRoleOptions"
                :key="'locked-' + role.code"
                class="legacy-role-tag"
                :title="role.description"
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
                      v-model="selectedRoles"
                      type="checkbox"
                      :value="role.code"
                      :disabled="!canAssignRoles || role.code === 'member'"
                    />
                    <span>{{ role.display }}</span>
                    <em v-if="role.code === 'member'">基础身份，不能移除</em>
                    <em v-else-if="role.hint">{{ role.hint }}</em>
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
              <input v-model="newPassword" class="input" placeholder="输入新密码" />
              <button type="button" class="um-btn um-btn--primary um-btn--sm" :disabled="passwordSubmitting" @click="emit('reset-password')">
                {{ passwordSubmitting ? '重置中...' : '重置密码' }}
              </button>
            </div>
          </section>

          <section v-if="canDisableUser" class="detail-section detail-section--danger">
            <header class="detail-section-header">
              <h4>账号状态</h4>
              <button
                v-if="user.status !== 'disabled'"
                type="button"
                class="um-btn um-btn--ghost um-btn--sm"
                :disabled="statusSubmitting"
                @click="emit('set-status', 'disabled')"
              >
                {{ statusSubmitting ? '处理中...' : '停用账号' }}
              </button>
              <button
                v-else
                type="button"
                class="um-btn um-btn--primary um-btn--sm"
                :disabled="statusSubmitting"
                @click="emit('set-status', 'active')"
              >
                {{ statusSubmitting ? '处理中...' : '启用账号' }}
              </button>
            </header>
            <p class="role-readonly-hint">停用后该账号不能登录，历史任务、角色、工号和组织归属会保留。</p>
          </section>
          <p v-if="actionMessage" class="action-msg">{{ actionMessage }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import BaseSkeleton from '@/components/base/BaseSkeleton.vue'
import {
  formatEmployeeNo,
  formatUserStatusForDisplay,
  type RoleOption,
  type RoleOptionGroup,
  type SelectOptionItem,
  type UserRow,
} from './userManagementTypes'

defineProps<{
  user: UserRow
  loading: boolean
  currentRoleNames: string
  canEditBasicInfo: boolean
  canMoveTeam: boolean
  canClearMembership: boolean
  canAssignRoles: boolean
  canResetPassword: boolean
  canDisableUser: boolean
  basicForm: { display_name: string; employee_no: string }
  membershipForm: { department: string; team: string }
  membershipDepartmentOptions: SelectOptionItem[]
  membershipTeamOptions: SelectOptionItem[]
  isMembershipDirty: boolean
  editableRoleGroups: RoleOptionGroup[]
  lockedRoleOptions: RoleOption[]
  lockedRoleTitle: string
  basicSubmitting: boolean
  membershipSubmitting: boolean
  roleSubmitting: boolean
  passwordSubmitting: boolean
  statusSubmitting: boolean
  actionMessage: string
}>()

const emit = defineEmits<{
  close: []
  'submit-basic': []
  'submit-membership': []
  'clear-membership': []
  'submit-roles': []
  'reset-password': []
  'set-status': [next: 'active' | 'disabled']
}>()

const selectedRoles = defineModel<string[]>('selectedRoles', { required: true })
const newPassword = defineModel<string>('newPassword', { required: true })
</script>

<style scoped src="./userManagementModal.css"></style>
