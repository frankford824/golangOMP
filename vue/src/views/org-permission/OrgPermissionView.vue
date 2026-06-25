<template>
  <div class="org-page min-h-[100dvh]">
    <header class="page-top">
      <div class="titles">
        <h1 class="page-title">组织与权限</h1>
        <p class="page-sub">按部门和小组管理真实成员归属</p>
      </div>
      <div class="top-actions">
        <BaseInput
          v-model="headerSearch"
          class="search-field"
          placeholder="搜索用户名 / 姓名"
          label=""
        />
        <div class="add-wrap" @click.stop>
          <BaseButton size="sm" variant="secondary" @click="addMenuOpen = !addMenuOpen">
            新增
          </BaseButton>
          <div v-if="addMenuOpen" class="add-dropdown">
            <button type="button" class="dd-item" @click="openNewDept">新增部门</button>
            <button type="button" class="dd-item" @click="openNewGroupFromHeader">新增小组</button>
          </div>
        </div>
      </div>
    </header>

    <p v-if="actionBanner" class="banner">{{ actionBanner }}</p>

    <div v-if="!canManage" class="mt-6">
      <BaseEmptyState title="无管理权限" description="需要组织管理权限才能访问本页。" />
    </div>
    <div v-else-if="loading" class="loading-skel">
      <BaseSkeleton width="100%" height="24rem" />
    </div>
    <BaseErrorState v-else-if="error" :title="error" @retry="load" />
    <div v-else class="three-cols">
      <aside class="col col-left">
        <OrgDeptTreePanel
          :tree="tree"
          :selected-group-key="selectedGroupKey"
          :expanded-names="expandedDeptNames"
          :group-counts="groupCounts"
          @select-group="selectedGroupKey = $event"
          @toggle-dept="toggleDept"
          @create-group="onCreateGroupFromTree"
          @rename-dept="openRenameDept"
          @delete-dept="confirmDeleteDept"
          @rename-group="openRenameGroup"
          @move-group-dept="openMoveGroupDept"
          @delete-group="confirmDeleteGroup"
        />
      </aside>
      <main class="col col-center">
        <OrgMembersCenterPanel
          :selected-group-key="selectedGroupKey"
          :path-label="currentPathLabel"
          :members="centerMembers"
          @add-member="showAddMemberModal = true"
          @move="openMoveUser"
          @remove="openRemoveUser"
        />
      </main>
      <aside class="col col-right">
        <OrgCandidatesPanel
          :users="users"
          :selected-group-key="selectedGroupKey"
          :header-search="headerSearch"
          :can-act="!!selectedGroupKey"
          @join="(u) => joinCurrentGroup(u)"
          @move="(u) => joinCurrentGroup(u)"
          @locate="locateUserGroup"
        />
      </aside>
    </div>

    <!-- 新增部门 -->
    <BaseModal
      v-model="showNewDeptModal"
      title="新增部门"
      panel-class="max-w-md"
      :show-confirm="false"
      cancel-text="关闭"
      @update:model-value="(v) => !v && (newDeptInput = '')"
    >
      <BaseInput v-model="newDeptInput" label="部门名称" placeholder="请输入部门名称" />
      <template #footer>
        <div class="flex justify-end gap-2 px-5 py-4 border-t border-[rgb(var(--yb-border-quiet))]">
          <BaseButton variant="secondary" size="sm" @click="showNewDeptModal = false">取消</BaseButton>
          <BaseButton variant="primary" size="sm" :disabled="!newDeptInput.trim()" @click="submitNewDept">
            确认
          </BaseButton>
        </div>
      </template>
    </BaseModal>

    <!-- 创建小组 -->
    <BaseModal
      v-model="showCreateGroupModal"
      :title="createGroupCtx?.deptName ? `创建小组（${createGroupCtx.deptName}）` : '创建小组'"
      panel-class="max-w-md"
      :show-confirm="false"
      cancel-text="关闭"
    >
      <BaseInput v-model="createGroupName" label="小组名称" placeholder="小组名称" />
      <BaseSelect
        v-if="!createGroupCtx?.deptName"
        v-model="createGroupDeptId"
        label="所属部门"
        placeholder="选择部门"
        :options="deptSelectOptions"
      />
      <template #footer>
        <div class="flex justify-end gap-2 px-5 py-4 border-t border-[rgb(var(--yb-border-quiet))]">
          <BaseButton variant="secondary" size="sm" @click="showCreateGroupModal = false">取消</BaseButton>
          <BaseButton
            variant="primary"
            size="sm"
            :disabled="!createGroupName.trim() || (!createGroupCtx?.deptName && !createGroupDeptId)"
            @click="submitCreateGroup"
          >
            确认创建
          </BaseButton>
        </div>
      </template>
    </BaseModal>

    <!-- 重命名部门 -->
    <BaseModal v-model="showRenameDeptModal" title="重命名部门" :show-confirm="false">
      <BaseInput v-model="renameDeptInput" label="新名称" />
      <template #footer>
        <BaseButton variant="secondary" size="sm" @click="showRenameDeptModal = false">取消</BaseButton>
        <BaseButton variant="primary" size="sm" @click="submitRenameDept">确认</BaseButton>
      </template>
    </BaseModal>

    <!-- 重命名小组 -->
    <BaseModal v-model="showRenameGroupModal" title="重命名小组" :show-confirm="false">
      <BaseInput v-model="renameGroupInput" label="新名称" />
      <template #footer>
        <BaseButton variant="secondary" size="sm" @click="showRenameGroupModal = false">取消</BaseButton>
        <BaseButton variant="primary" size="sm" @click="submitRenameGroup">确认</BaseButton>
      </template>
    </BaseModal>

    <!-- 移动小组到其他部门 -->
    <BaseModal v-model="showMoveGroupModal" title="移动小组到其他部门" :show-confirm="false">
      <BaseSelect
        v-model="moveGroupTargetDeptId"
        label="目标部门"
        placeholder="选择部门"
        :options="deptSelectOptions"
      />
      <template #footer>
        <BaseButton variant="secondary" size="sm" @click="showMoveGroupModal = false">取消</BaseButton>
        <BaseButton
          variant="primary"
          size="sm"
          :disabled="!moveGroupTargetDeptId"
          @click="submitMoveGroupDept"
        >
          确认移动
        </BaseButton>
      </template>
    </BaseModal>

    <!-- 添加成员 -->
    <BaseModal
      v-model="showAddMemberModal"
      :title="addMemberTitle"
      panel-class="max-w-lg"
      :show-confirm="false"
    >
      <BaseInput v-model="addMemberSearch" label="搜索" placeholder="用户名 / 姓名" />
      <div class="modal-user-list">
        <p v-if="!addMemberFiltered.length" class="muted">未找到相关成员</p>
        <div v-for="u in addMemberFiltered" :key="u.id" class="modal-user-row">
          <div class="modal-user-text">
            <div class="strong modal-user-display-name" :title="displayName(u)">
              {{ displayName(u) }}
            </div>
            <div
              class="muted small modal-user-meta"
              :title="addMemberModalMetaLine(u)"
            >
              {{ u.username }} · {{ formatUserGroupDisplay(u) }} · {{ rolesLine(u) }}
            </div>
          </div>
          <div class="modal-user-gutter" aria-hidden="true" />
          <BaseButton
            class="modal-user-action"
            size="sm"
            variant="secondary"
            :disabled="!selectedGroupKey || addMemberActionDisabled(u)"
            @click="addMemberFromModal(u)"
          >
            {{ addMemberActionLabel(u) }}
          </BaseButton>
        </div>
      </div>
    </BaseModal>

    <!-- 移动成员 -->
    <BaseModal v-model="showMoveUserModal" title="移动成员" :show-confirm="false">
      <p v-if="moveUserTarget" class="muted small">
        {{ displayName(moveUserTarget) }}（{{ moveUserTarget.username }}）
      </p>
      <p class="muted small">当前组：{{ moveUserCurrentLocation }}</p>
      <BaseSelect
        v-model="moveUserTargetGroupKey"
        label="目标组"
        placeholder="选择目标小组"
        :options="allGroupSelectOptions"
      />
      <template #footer>
        <BaseButton variant="secondary" size="sm" @click="showMoveUserModal = false">取消</BaseButton>
        <BaseButton
          variant="primary"
          size="sm"
          :disabled="!moveUserTargetGroupKey || !moveUserTarget"
          @click="submitMoveUser"
        >
          确认移动
        </BaseButton>
      </template>
    </BaseModal>

    <!-- 停用部门 -->
    <BaseModal v-model="showDisableDeptModal" title="确认停用部门？" :show-confirm="false">
      <p v-if="disableDeptTarget" class="remove-copy">
        即将停用部门「{{ disableDeptTarget.name }}」。停用后该部门将从组织选项中隐藏，已有用户归属不会被动迁移。
      </p>
      <p class="remove-copy muted small">
        停用为可逆操作，后续可在运营后台重新启用。
      </p>
      <template #footer>
        <BaseButton variant="secondary" size="sm" @click="showDisableDeptModal = false">取消</BaseButton>
        <BaseButton
          variant="primary"
          size="sm"
          :disabled="disableDeptSubmitting"
          @click="submitDisableDept"
        >
          {{ disableDeptSubmitting ? '处理中…' : '确认停用' }}
        </BaseButton>
      </template>
    </BaseModal>

    <!-- 停用小组 -->
    <BaseModal v-model="showDisableGroupModal" title="确认停用小组？" :show-confirm="false">
      <p v-if="disableGroupTarget" class="remove-copy">
        即将停用小组「{{ disableGroupTarget.teamName }}」。停用后该小组将从组织选项中隐藏，已有用户归属不会被动迁移。
      </p>
      <p class="remove-copy muted small">
        停用为可逆操作，后续可在运营后台重新启用。
      </p>
      <template #footer>
        <BaseButton variant="secondary" size="sm" @click="showDisableGroupModal = false">取消</BaseButton>
        <BaseButton
          variant="primary"
          size="sm"
          :disabled="disableGroupSubmitting"
          @click="submitDisableGroup"
        >
          {{ disableGroupSubmitting ? '处理中…' : '确认停用' }}
        </BaseButton>
      </template>
    </BaseModal>

    <!-- 移除成员 -->
    <BaseModal v-model="showRemoveConfirm" title="确认移除成员？" :show-confirm="false">
      <p class="remove-copy">
        移除后，该成员将不再属于当前小组，并进入未分组状态。
      </p>
      <template #footer>
        <BaseButton variant="secondary" size="sm" @click="showRemoveConfirm = false">取消</BaseButton>
        <BaseButton variant="primary" size="sm" @click="submitRemoveUser">确认移除</BaseButton>
      </template>
    </BaseModal>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { usePermissionsStore } from '@/stores/permissions'
import { usePermission } from '@/composables/usePermission'
import { useOrgPermissionData, patchUserMembership } from '@/composables/useOrgPermissionData'
import {
  createOrgDepartment,
  createOrgTeam,
  updateOrgDepartment,
  updateOrgTeam,
} from '@/services/api/orgApi'
import {
  buildOrgTreeDepartments,
  parseGroupKey,
  groupKey,
  getUsersByGroupKey,
  getGroupMemberCount,
  filterCandidateUsers,
  formatUserGroupDisplay,
  userDisplayName,
  userDepartmentLabel,
  userTeamLabel,
} from '@/domain/org-membership'
import { formatWorkflowRolesForDisplay } from '@/domain/user-workflow-roles'
import type { OrgTreeDepartmentNode, OrgTreeGroupNode, OrgUser } from '@/domain/types/org-membership'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import BaseSkeleton from '@/components/base/BaseSkeleton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseErrorState from '@/components/base/BaseErrorState.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import OrgDeptTreePanel from '@/components/org-permission/OrgDeptTreePanel.vue'
import OrgMembersCenterPanel from '@/components/org-permission/OrgMembersCenterPanel.vue'
import OrgCandidatesPanel from '@/components/org-permission/OrgCandidatesPanel.vue'

const { can } = usePermission()
// /org-permission 依然是组织主数据页面，仅 HRAdmin / SuperAdmin 可达。
// DepartmentAdmin 的细粒度成员管理走 /users。
const canManage = computed(() => can('org.manage'))

const permissionsStore = usePermissionsStore()
const { departments, groups } = storeToRefs(permissionsStore)

const { users, orgOptions, loading, error, load, applyLocalMembership } = useOrgPermissionData()

const headerSearch = ref('')
const addMenuOpen = ref(false)
const actionBanner = ref('')
const expandedDeptNames = ref<Set<string>>(new Set())
const selectedGroupKey = ref<string | null>(null)

function toggleDept(name: string) {
  const next = new Set(expandedDeptNames.value)
  if (next.has(name)) next.delete(name)
  else next.add(name)
  expandedDeptNames.value = next
}

const visibleDepartments = computed(() => departments.value)
const visibleGroups = computed(() => groups.value)

const deptIdToName = computed(() => {
  const m = new Map<string, string>()
  for (const d of visibleDepartments.value) m.set(d.id, d.name)
  return m
})

const departmentNameToStoreId = computed(() => {
  const m = new Map<string, string>()
  for (const d of visibleDepartments.value) m.set(d.name.trim(), d.id)
  return m
})

const tree = computed(() =>
  buildOrgTreeDepartments(
    orgOptions.value,
    visibleDepartments.value,
    visibleGroups.value,
    deptIdToName.value,
    departmentNameToStoreId.value,
    users.value,
  ),
)

watch(
  () => tree.value.map((d) => d.name),
  (names) => {
    const s = new Set(expandedDeptNames.value)
    for (const n of names) s.add(n)
    expandedDeptNames.value = s
  },
  { immediate: true },
)

const allGroupKeys = computed(() =>
  new Set(tree.value.flatMap((d) => d.groups.map((g: OrgTreeGroupNode) => g.key))),
)

watch(
  [tree, users],
  () => {
    if (selectedGroupKey.value && allGroupKeys.value.has(selectedGroupKey.value)) return
    const first = tree.value.flatMap((d) => d.groups)[0]
    selectedGroupKey.value = first?.key ?? null
  },
  { immediate: true, deep: true },
)

const groupCounts = computed(() => {
  const counts: Record<string, number> = {}
  for (const d of tree.value) {
    for (const g of d.groups) {
      counts[g.key] = getGroupMemberCount(g.key, users.value)
    }
  }
  return counts
})

const currentPathLabel = computed(() => {
  if (!selectedGroupKey.value) return ''
  const { departmentName, teamName } = parseGroupKey(selectedGroupKey.value)
  if (!teamName) return ''
  return `${departmentName} / ${teamName}`
})

const centerMembers = computed(() =>
  selectedGroupKey.value ? getUsersByGroupKey(users.value, selectedGroupKey.value) : [],
)

const DEPT_NAME_OPTION_PREFIX = 'name:'

function deptOptionValueByName(name: string): string {
  const trimmed = name.trim()
  const id = departmentNameToStoreId.value.get(trimmed)
  return id ?? `${DEPT_NAME_OPTION_PREFIX}${trimmed}`
}

function decodeDeptSelectionToStoreId(selection: string): string {
  const raw = selection.trim()
  if (!raw) return ''
  if (!raw.startsWith(DEPT_NAME_OPTION_PREFIX)) return raw
  const deptName = raw.slice(DEPT_NAME_OPTION_PREFIX.length).trim()
  return ensureStoreDepartmentId(deptName) ?? ''
}

const deptSelectOptions = computed(() =>
  tree.value.map((d) => ({ value: deptOptionValueByName(d.name), label: d.name })),
)

const allGroupSelectOptions = computed(() =>
  tree.value.flatMap((d) =>
    d.groups.map((g: OrgTreeGroupNode) => ({
      value: g.key,
      label: `${d.name} / ${g.teamName}`,
    })),
  ),
)

function displayName(u: OrgUser) {
  return userDisplayName(u)
}

function rolesLine(u: OrgUser) {
  return formatWorkflowRolesForDisplay(u.roles)
}

function addMemberModalMetaLine(u: OrgUser): string {
  return `${u.username ?? '—'} · ${formatUserGroupDisplay(u)} · ${rolesLine(u)}`
}

/** 确保权限 store 中存在该名称的部门，返回 departmentId */
function ensureStoreDepartmentId(deptName: string): string | null {
  const n = deptName.trim()
  if (!n) return null
  const found = visibleDepartments.value.find((d) => d.name.trim() === n)
  if (found) return found.id
  return permissionsStore.addDepartment(n)
}

// ─── 新增部门 / 小组 ───────────────────────────────────────
const showNewDeptModal = ref(false)
const newDeptInput = ref('')

const showCreateGroupModal = ref(false)
const createGroupName = ref('')
const createGroupDeptId = ref('')
const createGroupCtx = ref<{ deptName: string; storeDepartmentId?: string } | null>(null)

function openNewDept() {
  addMenuOpen.value = false
  newDeptInput.value = ''
  showNewDeptModal.value = true
}

function openNewGroupFromHeader() {
  addMenuOpen.value = false
  createGroupCtx.value = null
  createGroupName.value = ''
  createGroupDeptId.value = ''
  showCreateGroupModal.value = true
}

async function submitNewDept() {
  const name = newDeptInput.value.trim()
  if (!name) return
  actionBanner.value = ''
  try {
    await createOrgDepartment({ name, enabled: true })
    await load()
    actionBanner.value = '部门已创建，组织信息已刷新。'
    showNewDeptModal.value = false
    newDeptInput.value = ''
  } catch (e) {
    actionBanner.value = e instanceof Error ? e.message : '创建部门失败'
  }
}

function onCreateGroupFromTree(dept: OrgTreeDepartmentNode) {
  createGroupCtx.value = {
    deptName: dept.name,
    storeDepartmentId: dept.storeDepartmentId,
  }
  createGroupName.value = ''
  createGroupDeptId.value = deptOptionValueByName(dept.name)
  showCreateGroupModal.value = true
}

async function submitCreateGroup() {
  const name = createGroupName.value.trim()
  if (!name) return
  let deptId =
    createGroupCtx.value?.storeDepartmentId ??
    decodeDeptSelectionToStoreId(createGroupDeptId.value)
  if (!deptId && createGroupCtx.value?.deptName) {
    deptId = ensureStoreDepartmentId(createGroupCtx.value.deptName) ?? ''
  }
  if (!deptId) return
  actionBanner.value = ''
  try {
    await createOrgTeam({ name, department_id: deptId, enabled: true })
    await load()
    actionBanner.value = '小组已创建，组织信息已刷新。'
    showCreateGroupModal.value = false
    createGroupCtx.value = null
  } catch (e) {
    actionBanner.value = e instanceof Error ? e.message : '创建小组失败'
  }
}

// ─── 重命名 / 删除 / 移动小组 ──────────────────────────────
const showRenameDeptModal = ref(false)
const renameDeptInput = ref('')
const renameDeptTarget = ref<OrgTreeDepartmentNode | null>(null)

const showRenameGroupModal = ref(false)
const renameGroupInput = ref('')
const renameGroupTarget = ref<OrgTreeGroupNode | null>(null)

const showMoveGroupModal = ref(false)
const moveGroupTarget = ref<OrgTreeGroupNode | null>(null)
const moveGroupTargetDeptId = ref('')

function openRenameDept(dept: OrgTreeDepartmentNode) {
  if (!dept.storeDepartmentId) return
  renameDeptTarget.value = dept
  renameDeptInput.value = dept.name
  showRenameDeptModal.value = true
}

function submitRenameDept() {
  showRenameDeptModal.value = false
  actionBanner.value =
    '当前系统暂不支持在此页重命名部门，请联系运营 / 管理员走组织主数据流程。'
}

const showDisableDeptModal = ref(false)
const disableDeptTarget = ref<OrgTreeDepartmentNode | null>(null)
const disableDeptSubmitting = ref(false)

function confirmDeleteDept(dept: OrgTreeDepartmentNode) {
  if (!canManage.value) return
  if (!dept.storeDepartmentId) {
    actionBanner.value = '该部门未完成主数据同步，暂无法停用；请先通过「新增」登记。'
    return
  }
  disableDeptTarget.value = dept
  showDisableDeptModal.value = true
}

async function submitDisableDept() {
  const dept = disableDeptTarget.value
  if (!dept?.storeDepartmentId) return
  disableDeptSubmitting.value = true
  actionBanner.value = ''
  try {
    await updateOrgDepartment(dept.storeDepartmentId, false)
    await load()
    actionBanner.value = `部门「${dept.name}」已停用，组织信息已刷新。`
    showDisableDeptModal.value = false
    disableDeptTarget.value = null
  } catch (e) {
    actionBanner.value = e instanceof Error ? e.message : '停用部门失败'
  } finally {
    disableDeptSubmitting.value = false
  }
}

function openRenameGroup(g: OrgTreeGroupNode) {
  if (!g.storeGroupId) return
  renameGroupTarget.value = g
  renameGroupInput.value = g.teamName
  showRenameGroupModal.value = true
}

function submitRenameGroup() {
  showRenameGroupModal.value = false
  actionBanner.value =
    '当前系统暂不支持在此页重命名小组，请联系运营 / 管理员走组织主数据流程。'
}

function openMoveGroupDept(g: OrgTreeGroupNode) {
  if (!g.storeGroupId) return
  moveGroupTarget.value = g
  moveGroupTargetDeptId.value = ''
  showMoveGroupModal.value = true
}

function submitMoveGroupDept() {
  showMoveGroupModal.value = false
  actionBanner.value =
    '当前系统暂不支持将小组移动到其他部门，请联系运营 / 管理员在后台处理。'
}

const showDisableGroupModal = ref(false)
const disableGroupTarget = ref<OrgTreeGroupNode | null>(null)
const disableGroupSubmitting = ref(false)

function confirmDeleteGroup(g: OrgTreeGroupNode) {
  if (!canManage.value) return
  if (!g.storeGroupId) {
    actionBanner.value = '该小组未完成主数据同步，暂无法停用；请先通过「创建小组」登记。'
    return
  }
  disableGroupTarget.value = g
  showDisableGroupModal.value = true
}

async function submitDisableGroup() {
  const g = disableGroupTarget.value
  if (!g?.storeGroupId) return
  disableGroupSubmitting.value = true
  actionBanner.value = ''
  try {
    await updateOrgTeam(g.storeGroupId, false)
    await load()
    actionBanner.value = `小组「${g.teamName}」已停用，组织信息已刷新。`
    showDisableGroupModal.value = false
    disableGroupTarget.value = null
  } catch (e) {
    actionBanner.value = e instanceof Error ? e.message : '停用小组失败'
  } finally {
    disableGroupSubmitting.value = false
  }
}

// ─── 成员：添加 / 移动 / 移除 ─────────────────────────────
const showAddMemberModal = ref(false)
const addMemberSearch = ref('')

const addMemberTitle = computed(() => {
  const { teamName } = selectedGroupKey.value ? parseGroupKey(selectedGroupKey.value) : { teamName: '' }
  return teamName ? `添加成员到【${teamName}】` : '添加成员'
})

const addMemberFiltered = computed(() =>
  filterCandidateUsers(addMemberSearch.value, users.value),
)

function addMemberActionLabel(u: OrgUser): string {
  if (!selectedGroupKey.value) return '—'
  const { departmentName, teamName } = parseGroupKey(selectedGroupKey.value)
  if (userDepartmentLabel(u) === departmentName && userTeamLabel(u) === teamName) return '已在本组'
  return userTeamLabel(u) ? '移动到当前组' : '加入当前组'
}

function addMemberActionDisabled(u: OrgUser): boolean {
  if (!selectedGroupKey.value) return true
  const { departmentName, teamName } = parseGroupKey(selectedGroupKey.value)
  return userDepartmentLabel(u) === departmentName && userTeamLabel(u) === teamName
}

async function addMemberFromModal(u: OrgUser) {
  if (!selectedGroupKey.value) return
  const { departmentName, teamName } = parseGroupKey(selectedGroupKey.value)
  await runMembershipPatch(u.id, departmentName, teamName, '已更新成员归属')
  showAddMemberModal.value = false
}

async function joinCurrentGroup(u: OrgUser) {
  if (!selectedGroupKey.value) return
  const { departmentName, teamName } = parseGroupKey(selectedGroupKey.value)
  await runMembershipPatch(u.id, departmentName, teamName, '已加入当前组')
}

async function runMembershipPatch(userId: string, department: string, team: string, okMsg: string) {
  actionBanner.value = ''
  try {
    await patchUserMembership(userId, department, team)
    applyLocalMembership(userId, department, team)
    actionBanner.value = okMsg
  } catch (e) {
    actionBanner.value = e instanceof Error ? e.message : '操作失败（请确认后端 PATCH 支持 department/team）'
  }
}

const showMoveUserModal = ref(false)
const moveUserTarget = ref<OrgUser | null>(null)
const moveUserTargetGroupKey = ref('')

const moveUserCurrentLocation = computed(() =>
  moveUserTarget.value ? formatUserGroupDisplay(moveUserTarget.value) : '—',
)

function openMoveUser(u: OrgUser) {
  moveUserTarget.value = u
  moveUserTargetGroupKey.value = ''
  showMoveUserModal.value = true
}

async function submitMoveUser() {
  const u = moveUserTarget.value
  const key = moveUserTargetGroupKey.value
  if (!u || !key) return
  const { departmentName, teamName } = parseGroupKey(key)
  await runMembershipPatch(u.id, departmentName, teamName, '已移动成员')
  showMoveUserModal.value = false
}

const showRemoveConfirm = ref(false)
const removeUserTarget = ref<OrgUser | null>(null)

function openRemoveUser(u: OrgUser) {
  removeUserTarget.value = u
  showRemoveConfirm.value = true
}

async function submitRemoveUser() {
  const u = removeUserTarget.value
  if (!u) return
  await runMembershipPatch(u.id, '', '', '已移出当前组')
  showRemoveConfirm.value = false
}

function locateUserGroup(u: OrgUser) {
  const d = userDepartmentLabel(u)
  const t = userTeamLabel(u)
  if (!d || !t) return
  const key = groupKey(d, t)
  if (!allGroupKeys.value.has(key)) {
    actionBanner.value = '左侧树中暂无该组节点，可能组织配置与用户归属不一致。'
    return
  }
  selectedGroupKey.value = key
  const next = new Set(expandedDeptNames.value)
  next.add(d)
  expandedDeptNames.value = next
  actionBanner.value = `已定位到：${d} / ${t}`
}

onMounted(() => {
  document.addEventListener('click', () => {
    addMenuOpen.value = false
  })
  if (canManage.value) load()
})

watch(canManage, (ok) => {
  if (ok) load()
})
</script>

<style scoped>
.org-page {
  padding: 0 0 1.5rem;
  max-width: 100%;
}
.page-top {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}
.titles {
  flex: 1;
  min-width: 200px;
}
.page-title {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 700;
  color: rgb(var(--yb-text-navy));
}
.page-sub {
  margin: 0.25rem 0 0;
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-muted-strong));
}
.top-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.5rem;
}
.search-field {
  width: 14rem;
  min-width: 10rem;
}
.add-wrap {
  position: relative;
}
.add-dropdown {
  position: absolute;
  right: 0;
  top: 100%;
  margin-top: 4px;
  z-index: 30;
  background: rgb(var(--yb-surface));
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 0.5rem;
  box-shadow: 0 8px 24px rgb(var(--yb-shadow) / 0.08);
  min-width: 8rem;
  padding: 0.25rem;
}
.dd-item {
  display: block;
  width: 100%;
  text-align: left;
  padding: 0.4rem 0.5rem;
  font-size: 0.75rem;
  border: none;
  background: none;
  border-radius: 0.25rem;
  cursor: pointer;
}
.dd-item:hover {
  background: rgb(var(--yb-surface-subtle));
}
.banner {
  margin: 0 0 0.75rem;
  padding: 0.5rem 0.75rem;
  font-size: 0.75rem;
  border-radius: 0.5rem;
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-deep));
}
.loading-skel {
  margin-top: 0.5rem;
}
.three-cols {
  display: grid;
  grid-template-columns: minmax(220px, 260px) minmax(0, 1fr) minmax(220px, 280px);
  gap: 0.75rem;
  align-items: stretch;
  min-height: calc(100dvh - 8rem);
}
.col {
  background: rgb(var(--yb-surface));
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 0.75rem;
  padding: 0.75rem;
  min-height: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.col-center {
  box-shadow: 0 1px 3px rgb(var(--yb-shadow) / 0.04);
}
.modal-user-list {
  max-height: 50vh;
  overflow: auto;
  margin-top: 0.75rem;
}
/* 左 50% · 中 20% 留白 · 右 30% 操作区 */
.modal-user-row {
  display: grid;
  grid-template-columns: 5fr 2fr 3fr;
  align-items: center;
  column-gap: 0;
  padding: 0.5rem 0;
  border-bottom: 1px solid rgb(var(--yb-surface-slate));
  min-width: 0;
}
.modal-user-text {
  min-width: 0;
}
.modal-user-gutter {
  min-width: 0;
  pointer-events: none;
}
.modal-user-action {
  width: 100%;
  min-width: 0;
  justify-self: stretch;
}
.modal-user-display-name {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.modal-user-meta {
  margin-top: 0.125rem;
  max-width: 100%;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  overflow: hidden;
  overflow-wrap: anywhere;
  word-break: break-word;
  line-height: 1.35;
}
.strong {
  font-weight: 600;
  font-size: 0.8125rem;
}
.muted {
  color: rgb(var(--yb-text-muted-strong));
}
.small {
  font-size: 0.6875rem;
}
.remove-copy {
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-slate));
  line-height: 1.5;
}
@media (max-width: 1100px) {
  .three-cols {
    grid-template-columns: 1fr;
  }
}
</style>
