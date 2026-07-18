<template>
  <main class="access-page" :class="{ 'access-page--embedded': embedded }">
    <header v-if="!embedded" class="page-head">
      <div>
        <p class="eyebrow">权限配置</p>
        <h1>权限管理</h1>
        <p>按业务角色配置可执行操作；人员与组织只需选择角色和可管理范围。</p>
      </div>
      <button class="secondary" :disabled="loading" @click="load">{{ loading ? '刷新中…' : '刷新配置' }}</button>
    </header>
    <div v-if="error" class="error" role="alert">{{ error }}</div>

    <nav class="tabs" aria-label="权限管理内容">
      <button v-for="item in tabs" :key="item.id" :class="{ active: activeTab === item.id }" @click="activateTab(item.id)">{{ item.label }}</button>
    </nav>

    <section v-if="activeTab === 'roles'" class="layout">
      <aside class="role-list">
        <header><h2>业务角色</h2><button class="link-button" @click="beginCreateRole">新建</button></header>
        <button v-for="role in roles" :key="role.id" :class="{ active: selectedRole?.id === role.id }" @click="selectRole(role)">
          <span><strong>{{ role.name }}</strong><small>{{ role.description || '未填写说明' }}</small></span>
          <em v-if="role.system_protected">系统保护</em>
        </button>
      </aside>
      <section class="matrix-panel">
        <template v-if="selectedRole">
          <header class="matrix-head">
            <div><h2>{{ selectedRole.name }}</h2><p>{{ selectedRole.description || '未填写说明' }}</p></div>
            <div class="role-actions"><button class="secondary" :disabled="selectedRole.system_protected" @click="beginEditRole">编辑角色</button><button class="danger-button" :disabled="selectedRole.system_protected" @click="archiveSelectedRole">停用角色</button></div>
          </header>
          <div class="permission-groups">
            <section v-for="group in permissionGroups" :key="group.module">
              <h3>{{ moduleLabel(group.module) }}</h3>
              <label v-for="permission in group.items" :key="permission.code" class="permission-row">
                <input type="checkbox" :checked="isOperationChecked(permission.code)" :disabled="selectedRole.system_protected" @change="toggleOperation(permission.code, ($event.target as HTMLInputElement).checked)" />
                <span>
                  <strong>{{ permission.name }}</strong>
                  <small>{{ permission.description }}</small>
                  <div v-if="isOperationChecked(permission.code) && supportsTaskTypes(permission.code)" class="task-type-picker">
                    <span class="hint">允许的任务类型（不选=全部）</span>
                    <label v-for="option in TASK_TYPE_OPTIONS" :key="option.value" class="task-type-chip">
                      <input type="checkbox" :checked="hasTaskType(permission.code, option.value)" :disabled="selectedRole.system_protected" @change="toggleTaskType(permission.code, option.value, ($event.target as HTMLInputElement).checked)" />
                      {{ option.label }}
                    </label>
                  </div>
                </span>
                <em :class="permission.risk_level">{{ permission.risk_level === 'high' ? '需谨慎授权' : '常规' }}</em>
              </label>
            </section>
          </div>
          <footer class="save-bar"><input v-model.trim="reason" :disabled="selectedRole.system_protected" placeholder="填写调整原因" /><button class="primary" :disabled="selectedRole.system_protected || !reason || saving" @click="saveRolePermissions">{{ saving ? '保存中…' : '保存角色权限' }}</button></footer>
        </template>
        <div v-else class="empty">选择一个角色查看可执行操作。</div>
      </section>
    </section>

    <section v-else-if="activeTab === 'people'" class="panel people-panel">
      <header class="panel-head">
        <div><h2>人员授权</h2><p>选择人员后，为其配置角色与可管理范围。</p></div>
        <form @submit.prevent="searchPeople"><input v-model.trim="userKeyword" placeholder="姓名或账号" /><button class="secondary">搜索</button></form>
      </header>
      <div class="people-layout">
        <div class="user-results">
          <button v-for="user in userOptions" :key="user.id" :class="{ active: selectedUser?.id === user.id }" @click="selectUser(user)">
            <strong>{{ user.display_name || user.username || '人员 ' + user.id }}</strong>
            <small>{{ [user.department, user.team].filter(Boolean).join(' · ') || '未设置组织' }}</small>
          </button>
          <p v-if="!userOptions.length">请先搜索并选择人员。</p>
        </div>
        <form v-if="selectedUser" class="assignment-editor" @submit.prevent="saveAssignments">
          <header class="editor-head"><div><h3>直接授权</h3><p>保存时提交下列完整集合。</p></div><button type="button" class="secondary" @click="addAssignment">添加角色</button></header>
          <article v-for="(item, index) in directAssignments" :key="item.client_key" class="assignment-card">
            <label>业务角色<select v-model.number="item.role_id" required><option :value="0" disabled>请选择</option><option v-for="role in activeRoles" :key="role.id" :value="role.id">{{ role.name }}</option></select></label>
            <label>可管理范围<select v-model="item.scope_mode"><option value="self">仅本人相关</option><option value="own_department">本人所在部门</option><option value="own_team">本人所在团队</option><option value="selected_org">指定部门或团队</option><option value="global">全部数据</option></select></label>
            <section v-if="item.scope_mode === 'selected_org'" class="subject-editor">
              <header><strong>指定组织</strong><button type="button" class="link-button" @click="addAssignmentSubject(item)">添加组织</button></header>
              <div v-for="(subject, subjectIndex) in item.subjects" :key="subjectIndex" class="subject-row">
                <label>组织类型<select v-model="subject.subject_type" @change="subject.subject_id = 0"><option value="department">部门</option><option value="team">团队</option></select></label>
                <label>组织
                  <select v-model.number="subject.subject_id" required>
                    <option :value="0" disabled>请选择</option>
                    <option v-for="option in orgOptionsFor(subject.subject_type)" :key="option.id" :value="option.id">{{ option.name }}</option>
                  </select>
                </label>
                <button type="button" class="remove-button" @click="removeAssignmentSubject(item, subjectIndex)">删除组织</button>
              </div>
            </section>
            <button type="button" class="remove-button" @click="removeAssignment(index)">删除</button>
            <p v-if="assignmentError(item, index)" class="row-error" role="alert">{{ assignmentError(item, index) }}</p>
          </article>
          <p v-if="!directAssignments.length" class="empty-line">此人员没有直接授权。</p>
          <section v-if="inheritedAssignments.length" class="inherited">
            <h3>组织策略带来的权限（只读）</h3>
            <article v-for="item in inheritedAssignments" :key="'inherited-' + item.id">
              <strong>{{ item.role_name || roleName(item.role_id) }}</strong>
              <span>{{ scopeLabel(item.scope_mode) }}</span>
            </article>
          </section>
          <label class="wide">调整原因<input v-model.trim="assignmentReason" required /></label>
          <button class="primary" :disabled="saving || !assignmentReason || !assignmentsValid">保存全部直接授权</button>
        </form>
      </div>
      <div v-if="effective" class="effective-result"><strong>{{ selectedUser?.display_name || selectedUser?.username }} 当前可用操作</strong><div class="chips"><span v-for="permission in effective.permissions" :key="permission">{{ permissionName(permission) }}</span></div></div>
    </section>

    <section v-else-if="activeTab === 'org'" class="panel">
      <header class="panel-head"><div><h2>组织默认策略</h2><p>为部门或团队配置默认角色。</p></div></header>
      <form class="org-selector" @submit.prevent="loadOrgPolicies">
        <label>组织类型<select v-model="orgSubjectType" @change="resetOrgPolicies"><option value="department">部门</option><option value="team">团队</option></select></label>
        <label>组织
          <select v-model.number="orgSubjectId" required @change="resetOrgPolicies">
            <option :value="0" disabled>请选择</option>
            <option v-for="option in orgOptionsFor(orgSubjectType)" :key="option.id" :value="option.id">{{ option.name }}</option>
          </select>
        </label>
        <button class="secondary">读取当前策略</button>
      </form>
      <form v-if="orgPoliciesLoaded" class="org-editor" @submit.prevent="saveOrgPolicies">
        <header class="editor-head"><div><h3>当前策略</h3><p>保存时会提交下列完整集合。</p></div><button type="button" class="secondary" @click="addOrgPolicy">添加默认角色</button></header>
        <article v-for="(item, index) in orgPolicyDrafts" :key="item.client_key" class="org-policy-card">
          <label>默认角色<select v-model.number="item.role_id" required><option :value="0" disabled>请选择</option><option v-for="role in activeRoles" :key="role.id" :value="role.id">{{ role.name }}</option></select></label>
          <label>生效范围<select v-model="item.scope_mode"><option value="own_department">本部门</option><option value="own_team">本团队</option><option value="selected_org">当前组织</option><option value="global">全部数据</option></select></label>
          <label class="switch"><input v-model="item.enabled" type="checkbox" /><span>{{ item.enabled ? '已启用' : '已停用' }}</span></label>
          <button type="button" class="remove-button" @click="removeOrgPolicy(index)">删除</button>
        </article>
        <p v-if="!orgPolicyDrafts.length" class="empty-line">当前组织没有默认策略。</p>
        <label class="wide">调整原因<input v-model.trim="orgReason" required /></label>
        <button class="primary" :disabled="saving || !orgReason || !orgPoliciesValid">保存全部组织策略</button>
      </form>
    </section>

    <section v-else class="panel">
      <header class="panel-head">
        <div><h2>权限变更记录</h2><p>记录谁在何时调整了角色、人员和组织策略。</p></div>
        <div class="event-actions"><input v-model.trim="eventFilter" placeholder="筛选动作或对象" /><button class="secondary" @click="loadEvents">刷新记录</button></div>
      </header>
      <div class="event-list"><article v-for="item in filteredEvents" :key="item.id"><strong>{{ item.action }}</strong><span>{{ item.target_type }} · {{ item.target_id }}</span><p>{{ item.reason || '未填写原因' }}</p><time>{{ item.created_at }}</time></article><p v-if="!filteredEvents.length">暂无匹配的变更记录。</p></div>
    </section>

    <div v-if="showRoleEditor" class="dialog-mask" @click.self="showRoleEditor = false">
      <form class="dialog" role="dialog" aria-modal="true" @submit.prevent="saveRole">
        <h2>{{ editingRole ? '编辑角色' : '新建角色' }}</h2>
        <label v-if="!editingRole">角色代码<input v-model.trim="roleDraft.code" pattern="[a-z0-9_]+" required /></label>
        <label>角色名称<input v-model.trim="roleDraft.name" required /></label>
        <label>角色说明<textarea v-model.trim="roleDraft.description" rows="3" /></label>
        <label>调整原因<input v-model.trim="roleDraft.reason" required /></label>
        <div><button type="button" class="secondary" @click="showRoleEditor = false">取消</button><button class="primary" :disabled="saving">保存</button></div>
      </form>
    </div>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import {
  accessPolicyApi,
  TASK_TYPE_OPTIONS,
  TASK_TYPE_SCOPED_OPERATIONS,
  type AccessAssignment,
  type AccessPermission,
  type AccessPolicyEvent,
  type AccessRole,
  type AccessRolePermission,
  type AccessUserOption,
  type EffectiveAccess,
  type OrgPolicy,
  type ScopeMode,
  type ScopeSubjectType,
} from '@/services/api/accessPolicyApi'
import { fetchOrgOwnershipOptions } from '@/services/api/orgApi'
import { usePermissionsStore } from '@/stores/permissions'

type AssignmentDraft = AccessAssignment & { client_key: string }
type OrgPolicyDraft = OrgPolicy & { client_key: string }
type OrgOption = { id: number; name: string }

const props = withDefaults(defineProps<{
  embedded?: boolean
  initialTab?: 'roles' | 'people' | 'org' | 'events'
  initialUser?: AccessUserOption | null
  initialOrg?: { subject_type: ScopeSubjectType; subject_id: number } | null
}>(), { embedded: false, initialTab: 'roles', initialUser: null, initialOrg: null })

const embedded = computed(() => props.embedded)

const store = usePermissionsStore()
const tabs = [{ id: 'roles', label: '业务角色' }, { id: 'people', label: '人员授权' }, { id: 'org', label: '组织策略' }, { id: 'events', label: '变更记录' }] as const
const activeTab = ref<(typeof tabs)[number]['id']>(props.initialTab)
const permissions = ref<AccessPermission[]>([])
const roles = ref<AccessRole[]>([])
const selectedRole = ref<AccessRole | null>(null)
const draftPermissions = ref<AccessRolePermission[]>([])
const policyRevision = ref(0)
const reason = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const showRoleEditor = ref(false)
const editingRole = ref(false)
const roleDraft = reactive({ code: '', name: '', description: '', reason: '' })
const userKeyword = ref('')
const userOptions = ref<AccessUserOption[]>([])
const selectedUser = ref<AccessUserOption | null>(null)
const effective = ref<EffectiveAccess | null>(null)
const directAssignments = ref<AssignmentDraft[]>([])
const inheritedAssignments = ref<AccessAssignment[]>([])
const assignmentReason = ref('')
const orgSubjectType = ref<ScopeSubjectType>('department')
const orgSubjectId = ref(0)
const orgPoliciesLoaded = ref(false)
const orgPolicyDrafts = ref<OrgPolicyDraft[]>([])
const orgReason = ref('')
const events = ref<AccessPolicyEvent[]>([])
const eventFilter = ref('')
const departmentOptions = ref<OrgOption[]>([])
const teamOptions = ref<OrgOption[]>([])
let clientKey = 0

const activeRoles = computed(() => roles.value.filter((item) => !item.archived_at))
const permissionGroups = computed(() => {
  const groups = new Map<string, AccessPermission[]>()
  permissions.value.forEach((item) => groups.set(item.module, [...(groups.get(item.module) || []), item]))
  return [...groups].map(([module, items]) => ({ module, items }))
})
const assignmentsValid = computed(() => directAssignments.value.every((item, index) => !assignmentError(item, index)))
const orgPoliciesValid = computed(() => orgPolicyDrafts.value.every((item) => item.role_id > 0))
const filteredEvents = computed(() => {
  const keyword = eventFilter.value.toLowerCase()
  if (!keyword) return events.value
  return events.value.filter((item) => [item.action, item.target_type, item.target_id, item.reason].some((value) => String(value || '').toLowerCase().includes(keyword)))
})

function nextKey(prefix: string) { clientKey += 1; return prefix + '-' + clientKey }
function activateTab(tab: (typeof tabs)[number]['id']) { activeTab.value = tab; if (tab === 'events' && !events.value.length) void loadEvents() }
function supportsTaskTypes(code: string) { return TASK_TYPE_SCOPED_OPERATIONS.has(code) }
function isOperationChecked(code: string) { return draftPermissions.value.some((item) => item.code === code) }
function hasTaskType(code: string, taskType: string) {
  return (draftPermissions.value.find((item) => item.code === code)?.task_types || []).includes(taskType)
}
function toggleOperation(code: string, checked: boolean) {
  if (checked) {
    if (!isOperationChecked(code)) draftPermissions.value = [...draftPermissions.value, { code, task_types: [] }]
    return
  }
  draftPermissions.value = draftPermissions.value.filter((item) => item.code !== code)
}
function toggleTaskType(code: string, taskType: string, checked: boolean) {
  draftPermissions.value = draftPermissions.value.map((item) => {
    if (item.code !== code) return item
    const current = new Set(item.task_types || [])
    if (checked) current.add(taskType)
    else current.delete(taskType)
    return { ...item, task_types: [...current] }
  })
}
function selectRole(role: AccessRole) {
  selectedRole.value = role
  draftPermissions.value = role.permissions.map((item) => ({ code: item.code, task_types: [...(item.task_types || [])] }))
  reason.value = ''
}
async function loadOrgOptions() {
  try {
    const parsed = await fetchOrgOwnershipOptions({ throwOnError: false })
    departmentOptions.value = (parsed.departmentRecords || [])
      .map((item) => ({ id: Number(item.id), name: item.name }))
      .filter((item) => item.id > 0 && item.name)
    teamOptions.value = (parsed.teamRecords || [])
      .map((item) => ({ id: Number(item.id), name: item.departmentName ? `${item.name}（${item.departmentName}）` : item.name }))
      .filter((item) => item.id > 0 && item.name)
  } catch {
    departmentOptions.value = []
    teamOptions.value = []
  }
}
function orgOptionsFor(type: ScopeSubjectType) {
  return type === 'department' ? departmentOptions.value : teamOptions.value
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    const [permissionRows, roleRows] = await Promise.all([accessPolicyApi.permissions(), accessPolicyApi.roles(true), loadOrgOptions()])
    permissions.value = permissionRows
    roles.value = roleRows
    const previousId = selectedRole.value?.id
    const next = roleRows.find((item) => item.id === previousId) || roleRows.find((item) => !item.archived_at) || null
    if (next) selectRole(next)
    const selfId = Number(store.currentUser?.id || 0)
    if (selfId > 0) policyRevision.value = (await accessPolicyApi.effective(selfId)).policy_revision
  } catch (cause) { handleError(cause, '权限配置加载失败。') } finally { loading.value = false }
}
function handleError(cause: unknown, fallback: string) {
  const status = Number((cause as { response?: { status?: number } })?.response?.status || 0)
  error.value = status === 409 ? '配置已被其他管理员更新，已重新读取，请确认当前内容后再保存。' : cause instanceof Error ? cause.message : fallback
  if (status === 409) {
    void load()
    if (selectedUser.value) void selectUser(selectedUser.value)
    if (orgPoliciesLoaded.value) void loadOrgPolicies()
  }
}
async function saveRolePermissions() {
  if (!selectedRole.value || !reason.value) return
  saving.value = true
  error.value = ''
  try {
    const result = await accessPolicyApi.replaceRolePermissions(selectedRole.value, draftPermissions.value, policyRevision.value, reason.value)
    policyRevision.value = Number((result as { policy_revision?: number })?.policy_revision || policyRevision.value)
    await load()
  } catch (cause) { handleError(cause, '角色权限保存失败。') } finally { saving.value = false }
}
function beginCreateRole() { editingRole.value = false; Object.assign(roleDraft, { code: '', name: '', description: '', reason: '' }); showRoleEditor.value = true }
function beginEditRole() { if (!selectedRole.value) return; editingRole.value = true; Object.assign(roleDraft, { code: selectedRole.value.code, name: selectedRole.value.name, description: selectedRole.value.description, reason: '' }); showRoleEditor.value = true }
async function saveRole() {
  saving.value = true
  try {
    if (editingRole.value && selectedRole.value) await accessPolicyApi.updateRole(selectedRole.value, roleDraft.name, roleDraft.description, policyRevision.value, roleDraft.reason)
    else await accessPolicyApi.createRole({ code: roleDraft.code, name: roleDraft.name, description: roleDraft.description, permissions: [], reason: roleDraft.reason, expected_policy_revision: policyRevision.value })
    showRoleEditor.value = false
    await load()
  } catch (cause) { handleError(cause, '角色保存失败。') } finally { saving.value = false }
}
async function archiveSelectedRole() {
  if (!selectedRole.value || !reason.value) { error.value = '请先在下方填写停用原因。'; return }
  saving.value = true
  try { await accessPolicyApi.archiveRole(selectedRole.value, policyRevision.value, reason.value); await load() } catch (cause) { handleError(cause, '角色停用失败。') } finally { saving.value = false }
}
async function searchPeople() {
  if (!userKeyword.value) return
  try { userOptions.value = await accessPolicyApi.searchUsers(userKeyword.value) } catch (cause) { handleError(cause, '人员搜索失败。') }
}
function toAssignmentDraft(item?: AccessAssignment): AssignmentDraft {
  return {
    ...(item || { role_id: 0, scope_mode: 'self' as ScopeMode, subjects: [] }),
    subjects: item?.subjects ? item.subjects.map((subject) => ({ ...subject })) : [],
    client_key: nextKey('assignment'),
  }
}
async function selectUser(user: AccessUserOption) {
  selectedUser.value = user
  effective.value = await accessPolicyApi.effective(user.id)
  policyRevision.value = effective.value.policy_revision
  directAssignments.value = effective.value.assignments.filter((item) => item.source_type !== 'org_policy').map(toAssignmentDraft)
  inheritedAssignments.value = effective.value.assignments.filter((item) => item.source_type === 'org_policy')
  assignmentReason.value = ''
}
function addAssignment() { directAssignments.value.push(toAssignmentDraft()) }
function removeAssignment(index: number) { directAssignments.value.splice(index, 1) }
function addAssignmentSubject(item: AssignmentDraft) { item.subjects.push({ subject_type: 'department', subject_id: 0 }) }
function removeAssignmentSubject(item: AssignmentDraft, index: number) { item.subjects.splice(index, 1) }
function assignmentError(item: AssignmentDraft, index: number): string {
  if (item.role_id <= 0) return '请选择业务角色。'
  if (directAssignments.value.some((candidate, candidateIndex) => candidateIndex !== index && candidate.role_id === item.role_id)) return '同一个业务角色只能配置一次。'
  if (item.scope_mode === 'selected_org') {
    if (!item.subjects.length) return '指定组织范围至少需要一个部门或团队。'
    if (item.subjects.some((subject) => Number(subject.subject_id) <= 0)) return '请选择有效的组织。'
    const keys = item.subjects.map((subject) => subject.subject_type + ':' + subject.subject_id)
    if (new Set(keys).size !== keys.length) return '同一部门或团队不能重复添加。'
  }
  return ''
}
async function saveAssignments() {
  if (!selectedUser.value || !assignmentReason.value || !assignmentsValid.value) return
  const assignments: AccessAssignment[] = directAssignments.value.map((item) => ({
    role_id: item.role_id,
    scope_mode: item.scope_mode,
    subjects: item.scope_mode === 'selected_org' ? item.subjects.map((subject) => ({ subject_type: subject.subject_type, subject_id: Number(subject.subject_id) })) : [],
  }))
  saving.value = true
  try {
    await accessPolicyApi.replaceUserAssignments(selectedUser.value.id, assignments, policyRevision.value, assignmentReason.value)
    await selectUser(selectedUser.value)
  } catch (cause) { handleError(cause, '人员授权保存失败。') } finally { saving.value = false }
}
function resetOrgPolicies() { orgPoliciesLoaded.value = false; orgPolicyDrafts.value = []; orgReason.value = '' }
function toOrgPolicyDraft(item?: OrgPolicy): OrgPolicyDraft {
  return {
    ...(item || { subject_type: orgSubjectType.value, subject_id: Number(orgSubjectId.value), role_id: 0, scope_mode: 'selected_org' as ScopeMode, enabled: true }),
    subject_type: orgSubjectType.value,
    subject_id: Number(orgSubjectId.value),
    client_key: nextKey('org-policy'),
  }
}
async function loadOrgPolicies() {
  if (!orgSubjectId.value) return
  try {
    orgPolicyDrafts.value = (await accessPolicyApi.orgPolicies(orgSubjectType.value, orgSubjectId.value)).map(toOrgPolicyDraft)
    orgPoliciesLoaded.value = true
    orgReason.value = ''
  } catch (cause) { handleError(cause, '组织策略读取失败。') }
}
function addOrgPolicy() { orgPolicyDrafts.value.push(toOrgPolicyDraft()) }
function removeOrgPolicy(index: number) { orgPolicyDrafts.value.splice(index, 1) }
async function saveOrgPolicies() {
  if (!orgPoliciesLoaded.value || !orgSubjectId.value || !orgReason.value || !orgPoliciesValid.value) return
  const policies = orgPolicyDrafts.value.map(({ client_key: _clientKey, ...item }) => ({ ...item, subject_type: orgSubjectType.value, subject_id: Number(orgSubjectId.value) }))
  saving.value = true
  try {
    const result = await accessPolicyApi.replaceOrgPolicies(orgSubjectType.value, orgSubjectId.value, policies, policyRevision.value, orgReason.value)
    policyRevision.value = result.policy_revision
    orgPolicyDrafts.value = result.org_policies.map(toOrgPolicyDraft)
    orgReason.value = ''
  } catch (cause) { handleError(cause, '组织策略保存失败。') } finally { saving.value = false }
}
async function loadEvents() { try { events.value = await accessPolicyApi.events(200) } catch (cause) { handleError(cause, '变更记录加载失败。') } }
const moduleLabel = (module: string) => ({ task: '任务操作', planning_sku: '策划 SKU', asset: '资产', workbench: '素材工作台', catalog: '产品资料', erp: 'ERP', account: '个人工作台', report: '经营报表', system: '系统管理', access: '权限' }[module] || module)
const permissionName = (code: string) => permissions.value.find((item) => item.code === code)?.name || code
const roleName = (roleId: number) => roles.value.find((item) => item.id === roleId)?.name || '角色 ' + roleId
const scopeLabel = (scope: ScopeMode) => ({ self: '仅本人相关', own_department: '本人所在部门', own_team: '本人所在团队', selected_org: '指定组织', global: '全部数据' }[scope])
async function applyInitialTarget() {
  activeTab.value = props.initialTab
  if (props.initialUser) {
    activeTab.value = 'people'
    userOptions.value = [props.initialUser]
    await selectUser(props.initialUser)
    return
  }
  if (props.initialOrg?.subject_id) {
    activeTab.value = 'org'
    orgSubjectType.value = props.initialOrg.subject_type
    orgSubjectId.value = Number(props.initialOrg.subject_id)
    await loadOrgPolicies()
  }
}
watch(() => [props.initialUser?.id, props.initialOrg?.subject_type, props.initialOrg?.subject_id, props.initialTab] as const, () => { void applyInitialTarget() })
onMounted(async () => { await load(); await applyInitialTarget() })
</script>

<style scoped>
.access-page{max-width:1320px;margin:0 auto;padding:30px;display:grid;gap:20px}.access-page--embedded{max-width:none;padding:0}.page-head,.matrix-head,.panel-head,.editor-head{display:flex;align-items:center;justify-content:space-between;gap:20px}.page-head h1{margin:4px 0;font-size:32px}.page-head p,.matrix-head p,.panel-head p,.editor-head p{margin:0;color:rgb(var(--yb-text-muted))}.eyebrow{color:rgb(var(--yb-brand));font-size:11px;letter-spacing:.14em;font-weight:900}.primary,.secondary,.danger-button,.link-button,.tabs button,.remove-button{min-height:40px;padding:0 16px;border-radius:11px;font-weight:750;cursor:pointer}.primary{border:0;background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse))}.secondary,.tabs button{border:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.danger-button,.remove-button{border:1px solid rgb(var(--yb-danger-border));background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.link-button{border:0;background:transparent;color:rgb(var(--yb-brand))}.tabs{display:flex;gap:8px}.tabs button.active{background:rgb(var(--yb-brand-soft));border-color:rgb(var(--yb-brand-border))}.layout{display:grid;grid-template-columns:280px 1fr;border:1px solid rgb(var(--yb-border));border-radius:18px;background:rgb(var(--yb-surface));overflow:hidden}.role-list{border-right:1px solid rgb(var(--yb-border));padding:14px}.role-list header{display:flex;justify-content:space-between;align-items:center}.role-list h2{margin:0;font-size:16px}.role-list>button{width:100%;display:flex;justify-content:space-between;align-items:center;padding:12px;border:0;border-radius:11px;background:transparent;color:rgb(var(--yb-text));text-align:left;cursor:pointer}.role-list>button.active{background:rgb(var(--yb-brand-soft))}.role-list span{display:grid;gap:3px}.role-list small,.role-list em{color:rgb(var(--yb-text-muted));font-style:normal}.matrix-panel{min-height:560px;display:flex;flex-direction:column}.matrix-head{padding:20px 22px;border-bottom:1px solid rgb(var(--yb-border))}.matrix-head h2{margin:0}.role-actions,.event-actions{display:flex;gap:8px}.permission-groups{padding:18px 22px;display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:22px}.permission-groups h3{font-size:13px;color:rgb(var(--yb-text-muted))}.permission-row{display:grid;grid-template-columns:20px 1fr auto;gap:10px;align-items:start;padding:10px 0;border-bottom:1px solid rgb(var(--yb-border))}.permission-row span{display:grid;gap:3px}.permission-row small{color:rgb(var(--yb-text-muted))}.permission-row em{font-size:11px;font-style:normal}.permission-row em.high{color:rgb(var(--yb-danger-text))}.task-type-picker{display:flex;flex-wrap:wrap;gap:8px;margin-top:8px}.task-type-picker .hint{width:100%;font-size:12px;color:rgb(var(--yb-text-muted))}.task-type-chip{display:inline-flex;align-items:center;gap:4px;padding:4px 8px;border-radius:999px;background:rgb(var(--yb-surface-muted));font-size:12px}.save-bar{margin-top:auto;display:flex;gap:10px;padding:16px 22px;border-top:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface-soft))}.save-bar input,.panel input,.panel select,.dialog input,.dialog textarea{min-height:40px;border:1px solid rgb(var(--yb-border));border-radius:10px;padding:8px 12px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.save-bar input{flex:1}.panel{padding:20px;border:1px solid rgb(var(--yb-border));border-radius:18px;background:rgb(var(--yb-surface));display:grid;gap:18px}.people-layout{display:grid;grid-template-columns:300px 1fr;gap:18px}.user-results{display:grid;align-content:start;gap:6px}.user-results button{display:grid;gap:3px;padding:11px;border:1px solid rgb(var(--yb-border));border-radius:10px;background:rgb(var(--yb-surface));text-align:left}.user-results button.active{background:rgb(var(--yb-brand-soft))}.user-results small{color:rgb(var(--yb-text-muted))}.assignment-editor,.org-editor{display:grid;gap:12px}.assignment-card,.org-policy-card{display:grid;grid-template-columns:repeat(2,minmax(0,1fr)) auto;gap:12px;align-items:end;padding:14px;border:1px solid rgb(var(--yb-border));border-radius:12px;background:rgb(var(--yb-surface-soft))}.assignment-editor label,.org-editor label,.org-selector label,.dialog label{display:grid;gap:6px}.org-selector{display:flex;align-items:end;gap:12px}.org-selector label{min-width:220px}.wide{grid-column:1/-1}.switch{display:flex;align-items:center;gap:8px;min-height:40px}.switch input{width:auto}.inherited{display:grid;gap:8px;padding:14px;border-radius:12px;background:rgb(var(--yb-surface-muted))}.inherited h3{margin:0}.inherited article{display:flex;justify-content:space-between}.empty-line{color:rgb(var(--yb-text-muted))}.effective-result{display:grid;gap:10px;padding-top:14px;border-top:1px solid rgb(var(--yb-border))}.chips{display:flex;gap:6px;flex-wrap:wrap}.chips span{padding:5px 8px;border-radius:999px;background:rgb(var(--yb-surface-muted));font-size:12px}.event-list{display:grid;gap:8px}.event-list article{display:grid;grid-template-columns:1fr auto;gap:4px;padding:12px;border-radius:11px;background:rgb(var(--yb-surface-muted))}.event-list p,.event-list time{margin:0;color:rgb(var(--yb-text-muted))}.event-list time{grid-column:2;grid-row:2}.dialog-mask{position:fixed;inset:0;z-index:90;display:grid;place-items:center;padding:20px;background:rgb(var(--yb-overlay-night) / .48)}.dialog{width:min(500px,100%);display:grid;gap:14px;padding:22px;border-radius:18px;background:rgb(var(--yb-surface));box-shadow:0 20px 55px rgb(var(--yb-shadow) / .22)}.dialog h2{margin:0}.dialog>div{display:flex;justify-content:flex-end;gap:8px}.error{padding:12px 14px;border-radius:12px;background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.empty{display:grid;place-items:center;min-height:400px;color:rgb(var(--yb-text-muted))}
@media(max-width:850px){.access-page{padding:16px}.layout,.people-layout{grid-template-columns:1fr}.role-list{border-right:0;border-bottom:1px solid rgb(var(--yb-border));max-height:240px;overflow:auto}.permission-groups,.assignment-card,.org-policy-card{grid-template-columns:1fr}.wide{grid-column:auto}.page-head,.matrix-head,.panel-head,.editor-head{align-items:flex-start;flex-direction:column}.tabs,.event-actions{overflow:auto}.role-actions{flex-wrap:wrap}.org-selector{align-items:stretch;flex-direction:column}.org-selector label{min-width:0}.remove-button{justify-self:start}}
.subject-editor{grid-column:1/-1;display:grid;gap:8px}.subject-editor header,.subject-row{display:flex;align-items:end;gap:10px}.subject-editor header{align-items:center;justify-content:space-between}.subject-row label{flex:1}.row-error{grid-column:1/-1;margin:0;color:rgb(var(--yb-danger-text));font-size:13px}
@media(max-width:850px){.subject-row{align-items:stretch;flex-direction:column}.subject-row label{width:100%}}
</style>
