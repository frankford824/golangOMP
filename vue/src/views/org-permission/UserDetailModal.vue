<template>
  <div class="modal-mask detail-mask" @click.self="close">
    <div ref="panel" class="modal-panel um-modal user-detail" role="dialog" aria-modal="true" aria-labelledby="user-detail-title">
      <header class="modal-header">
        <div class="modal-heading">
          <h3 id="user-detail-title" class="section-title">{{ user.display_name || user.username }}</h3>
          <div class="detail-identity">
            <span>{{ user.username }} · {{ formatEmployeeNo(user.employee_no) }}</span>
            <span class="detail-status" :class="{ 'detail-status--active': user.status === 'active' }">
              {{ user.status === 'active' ? '已启用' : formatUserStatusForDisplay(user.status) }}
            </span>
          </div>
        </div>
        <button type="button" class="modal-close" aria-label="关闭用户详情" :disabled="busy" @click="close">×</button>
      </header>

      <div v-if="loading" class="modal-body"><BaseSkeleton width="100%" height="10rem" /></div>
      <p v-else-if="unavailable" class="detail-feedback" role="alert">{{ actionMessage }}</p>
      <template v-else>
        <div class="detail-tabs" role="tablist" aria-label="用户详情" @keydown="navigateTabs">
          <button
            v-for="tab in tabs"
            :id="'user-detail-tab-' + tab.id"
            :key="tab.id"
            type="button"
            role="tab"
            :aria-selected="activeTab === tab.id"
            :aria-controls="'user-detail-panel-' + tab.id"
            :tabindex="activeTab === tab.id ? 0 : -1"
            @click="selectTab(tab.id)"
          >{{ tab.label }}</button>
        </div>

        <div v-show="activeTab === 'profile'" id="user-detail-panel-profile" class="modal-body" role="tabpanel" aria-labelledby="user-detail-tab-profile">
          <section class="detail-section">
            <h4>基本信息</h4>
            <form @submit.prevent="emit('submit-basic')">
              <div class="detail-grid">
                <label class="field-label">
                  <span>姓名</span>
                  <input v-model.trim="basicForm.display_name" class="input" :disabled="!canEditBasicInfo || busy" />
                </label>
                <label class="field-label">
                  <span>工号</span>
                  <input v-model.trim="basicForm.employee_no" class="input" inputmode="numeric" placeholder="0–9999" :disabled="!canEditBasicInfo || busy" />
                </label>
              </div>
              <div v-if="canEditBasicInfo" class="detail-actions">
                <button type="submit" class="um-btn um-btn--ghost" :disabled="busy || !isBasicDirty">{{ basicSubmitting ? '保存中…' : '保存基本信息' }}</button>
              </div>
            </form>
          </section>
          <section class="detail-section">
            <h4>组织归属</h4>
            <form v-if="canMoveTeam" @submit.prevent="emit('submit-membership')">
              <div class="detail-grid">
                <label class="field-label">
                  <span>部门</span>
                  <select v-model="membershipForm.department" class="input" :disabled="busy">
                    <option value="">选择部门</option>
                    <option v-for="d in membershipDepartmentOptions" :key="d.value" :value="d.value">{{ d.label }}</option>
                  </select>
                </label>
                <label class="field-label">
                  <span>小组</span>
                  <select v-model="membershipForm.team" class="input" :disabled="busy || !membershipForm.department">
                    <option value="">选择小组</option>
                    <option v-for="t in membershipTeamOptions" :key="t.value" :value="t.value">{{ t.label }}</option>
                  </select>
                </label>
              </div>
              <div class="detail-actions">
                <button v-if="canClearMembership" type="button" class="detail-text-button" :disabled="busy" @click="emit('clear-membership')">移到未分配池</button>
                <button type="submit" class="um-btn um-btn--primary" :disabled="busy || !isMembershipDirty">{{ membershipSubmitting ? '保存中…' : '保存归属' }}</button>
              </div>
            </form>
            <p v-else class="detail-note">{{ [user.department, user.team].filter(Boolean).join(' / ') || '未分配' }}</p>
          </section>
        </div>

        <div v-show="activeTab === 'roles'" id="user-detail-panel-roles" class="modal-body" role="tabpanel" aria-labelledby="user-detail-tab-roles">
          <section class="detail-section">
            <header class="detail-section-header">
              <h4>工作角色</h4>
              <div class="detail-section-actions">
                <button
                  v-if="canAssignRoles"
                  type="button"
                  class="um-btn um-btn--primary um-btn--sm"
                  :disabled="busy || !isRolesDirty"
                  @click="emit('submit-roles')"
                >
                  {{ roleSubmitting ? '提交中...' : '保存角色' }}
                </button>
              </div>
            </header>
            <p class="detail-note">当前角色 · {{ currentRoleNames }}</p>
            <div v-if="lockedRoleOptions.length" class="legacy-role-box">
              <span class="legacy-role-title">由部门或小组自动应用</span>
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
                      :disabled="busy || !canAssignRoles || role.code === 'member'"
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
        </div>

        <div v-if="hasSecurity" v-show="activeTab === 'security'" id="user-detail-panel-security" class="modal-body" role="tabpanel" aria-labelledby="user-detail-tab-security">
          <template v-if="!confirmDelete">
            <section v-if="canResetPassword" class="detail-section">
              <h4>重置密码</h4>
              <form class="detail-password" @submit.prevent="emit('reset-password')">
                <label class="field-label">
                  <span>新密码</span>
                  <input v-model="newPassword" class="input" type="password" autocomplete="new-password" :disabled="busy" />
                </label>
                <button type="submit" class="um-btn um-btn--ghost" :disabled="busy || !newPassword.trim()">{{ passwordSubmitting ? '重置中…' : '重置密码' }}</button>
              </form>
            </section>
            <section v-if="canDisableUser" class="detail-section detail-operation">
              <div>
                <h4>账号状态</h4>
                <p class="detail-note">{{ user.status === 'disabled' ? '启用后，该账号可以重新登录。' : '停用会阻止登录，可以随时重新启用。' }}</p>
              </div>
              <button type="button" class="um-btn um-btn--ghost" :disabled="busy" @click="emit('set-status', user.status === 'disabled' ? 'active' : 'disabled')">
                {{ statusSubmitting ? '处理中…' : user.status === 'disabled' ? '启用账号' : '停用账号' }}
              </button>
            </section>
            <section v-if="canDeleteUser" class="detail-section detail-operation">
              <div>
                <h4>删除账号</h4>
                <p class="detail-note">从用户管理中移除该账号，并结束其登录会话。</p>
              </div>
              <button ref="deleteButton" type="button" class="um-btn detail-danger-outline" :disabled="busy" @click="beginDelete">删除账号</button>
            </section>
          </template>
          <section v-else class="detail-section detail-confirm">
            <h4>确认删除账号</h4>
            <p>将删除 <strong>{{ user.display_name || user.username }}</strong>（{{ user.username }}）。</p>
            <p class="detail-note">账号将无法登录，管理员无法重新启用。历史业务记录和署名会保留。</p>
            <div class="detail-actions">
              <button ref="cancelButton" type="button" class="um-btn um-btn--ghost" :disabled="deleting" @click="cancelDelete">取消</button>
              <button type="button" class="um-btn detail-danger-solid" :disabled="deleting" @click="emit('delete-user')">{{ deleting ? '删除中…' : '确认删除' }}</button>
            </div>
          </section>
        </div>
        <p v-if="actionMessage" class="detail-feedback" role="status">{{ actionMessage }}</p>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useModalA11y } from '@/composables/useModalA11y'
import BaseSkeleton from '@/components/base/BaseSkeleton.vue'
import {
  formatEmployeeNo,
  formatUserStatusForDisplay,
  type RoleOption,
  type RoleOptionGroup,
  type SelectOptionItem,
  type UserRow,
} from './userManagementTypes'

const props = defineProps<{
  user: UserRow
  loading: boolean
  unavailable: boolean
  currentRoleNames: string
  canEditBasicInfo: boolean
  canMoveTeam: boolean
  canClearMembership: boolean
  canAssignRoles: boolean
  canResetPassword: boolean
  canDisableUser: boolean
  canDeleteUser: boolean
  isBasicDirty: boolean
  isRolesDirty: boolean
  deleting: boolean
  basicForm: { display_name: string; employee_no: string }
  membershipForm: { department: string; team: string }
  membershipDepartmentOptions: SelectOptionItem[]
  membershipTeamOptions: SelectOptionItem[]
  isMembershipDirty: boolean
  editableRoleGroups: RoleOptionGroup[]
  lockedRoleOptions: RoleOption[]
  basicSubmitting: boolean
  membershipSubmitting: boolean
  roleSubmitting: boolean
  passwordSubmitting: boolean
  statusSubmitting: boolean
  actionMessage: string
}>()

const emit = defineEmits<{
  close: []
  'delete-user': []
  'clear-message': []
  'submit-basic': []
  'submit-membership': []
  'clear-membership': []
  'submit-roles': []
  'reset-password': []
  'set-status': [next: 'active' | 'disabled']
}>()

const selectedRoles = defineModel<string[]>('selectedRoles', { required: true })
const newPassword = defineModel<string>('newPassword', { required: true })

type DetailTab = 'profile' | 'roles' | 'security'
const activeTab = ref<DetailTab>('profile')
const confirmDelete = ref(false)
const panel = ref<HTMLElement | null>(null)
const deleteButton = ref<HTMLButtonElement | null>(null)
const cancelButton = ref<HTMLButtonElement | null>(null)
const busy = computed(() => props.deleting || props.basicSubmitting || props.membershipSubmitting || props.roleSubmitting || props.passwordSubmitting || props.statusSubmitting)
const hasSecurity = computed(() => props.canResetPassword || props.canDisableUser || props.canDeleteUser)
const tabs = computed<{ id: DetailTab; label: string }[]>(() => [
  { id: 'profile', label: '资料与归属' },
  { id: 'roles', label: '角色权限' },
  ...(hasSecurity.value ? [{ id: 'security' as const, label: '账号安全' }] : []),
])

function selectTab(id: DetailTab) {
  if (id === activeTab.value || busy.value) return
  confirmDelete.value = false
  activeTab.value = id
  emit('clear-message')
}

function navigateTabs(event: KeyboardEvent) {
  const index = tabs.value.findIndex((tab) => tab.id === activeTab.value)
  const length = tabs.value.length
  const positions: Record<string, number> = {
    ArrowLeft: (index + length - 1) % length,
    ArrowRight: (index + 1) % length,
    Home: 0,
    End: length - 1,
  }
  const next = tabs.value[positions[event.key] ?? -1]
  if (!next || busy.value) return
  event.preventDefault()
  selectTab(next.id)
  panel.value?.querySelector<HTMLElement>('#user-detail-tab-' + next.id)?.focus()
}

async function beginDelete() {
  emit('clear-message')
  confirmDelete.value = true
  await nextTick()
  cancelButton.value?.focus()
}

async function cancelDelete() {
  if (busy.value) return
  confirmDelete.value = false
  emit('clear-message')
  await nextTick()
  deleteButton.value?.focus()
}

function close() {
  if (!busy.value) emit('close')
}
useModalA11y(ref(true), panel, { onEsc: () => confirmDelete.value ? void cancelDelete() : close() })
</script>

<style scoped src="./userManagementModal.css"></style>
<style scoped src="./userDetailModal.css"></style>
