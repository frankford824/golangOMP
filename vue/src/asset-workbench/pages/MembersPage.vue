<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RefreshCw, Search, ShieldCheck, UserRound } from 'lucide-vue-next'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { assetWorkbenchApi, type AccountMergePreview, type WorkbenchMemberRow } from '@aw/shared/api/assetWorkbenchApi'
import { chipClass, workerTypeMeta } from '@aw/shared/format/status'
import { managedAssetRoles, roleDisplayList, roleDisplayName } from '@aw/shared/format/roleDisplay'

const loading = ref(false)
const savingUserId = ref<number | null>(null)
const error = ref('')
const notice = ref('')
const query = ref('')
const statusFilter = ref('active')
const members = ref<WorkbenchMemberRow[]>([])
const lookupQuery = ref('')
const lookupResults = ref<WorkbenchMemberRow[]>([])
const selectedLookupUser = ref<WorkbenchMemberRow | null>(null)
const openRoles = ref<string[]>(['AssetSubmitter'])
const openReason = ref('开通资产工作台')
const disableTarget = ref<WorkbenchMemberRow | null>(null)
const disableReason = ref('')
const mergeSourceId = ref('')
const mergeCanonicalId = ref('')
const mergeReason = ref('合并资产工作台账号')
const mergePreview = ref<AccountMergePreview | null>(null)
const mergeChoices = ref<Record<string, string>>({})
const { bootstrap, refresh: refreshBootstrap } = useAssetWorkbenchBootstrap()
const roleOptions = managedAssetRoles.filter((role) => role !== 'AssetSubmitter')
const statusOptions = [
  { value: 'active', label: '已开通' },
  { value: 'pending', label: '待处理' },
  { value: 'disabled', label: '已停用' },
  { value: 'merged', label: '已合并' },
  { value: 'all', label: '全部' },
]

const total = computed(() => members.value.length)
const canChangeRoles = computed(() =>
  (bootstrap.value?.capabilities ?? []).includes('asset.workbench.member.identity'),
)
const openableRoles = computed(() => ['AssetSubmitter', ...roleOptions])

function memberName(member: WorkbenchMemberRow) {
  return member.real_name || member.display_name || member.username || `用户 ${member.user_id}`
}

function statusMeta(status?: string) {
  const value = status || ''
  const labels: Record<string, { label: string; tone: 'success' | 'warn' | 'danger' | 'info' | 'neutral' }> = {
    active: { label: '已开通', tone: 'success' },
    pending: { label: '待处理', tone: 'warn' },
    disabled: { label: '已停用', tone: 'danger' },
    merged: { label: '已合并', tone: 'neutral' },
  }
  return labels[value] ?? { label: value || '未知', tone: 'neutral' }
}

async function loadMembers() {
  loading.value = true
  error.value = ''
  try {
    const params: Record<string, unknown> = {
      q: query.value,
      page_size: 200,
    }
    if (statusFilter.value !== 'all') params.status = statusFilter.value
    const result = await assetWorkbenchApi.listMembers(params)
    members.value = result.items
  } catch (err) {
    error.value = err instanceof Error ? err.message : '成员加载失败'
  } finally {
    loading.value = false
  }
}

async function loadPage() {
  await Promise.allSettled([refreshBootstrap(), loadMembers()])
}

function memberRoles(member: WorkbenchMemberRow) {
  return new Set(member.roles ?? [])
}

function memberLabels(member: WorkbenchMemberRow) {
  return member.role_labels?.length ? member.role_labels : roleDisplayList(member.roles ?? [])
}

async function toggleRole(member: WorkbenchMemberRow, role: string, checked: boolean) {
  if (!canChangeRoles.value || member.status !== 'active') {
    error.value = '仅超级管理员可以调整成员可用功能'
    return
  }
  const roles = memberRoles(member)
  roles.add('AssetSubmitter')
  if (checked) roles.add(role)
  else roles.delete(role)
  savingUserId.value = member.user_id
  error.value = ''
  notice.value = ''
  try {
    const updated = await assetWorkbenchApi.updateMemberRoles(member.user_id, Array.from(roles), 'workbench role update')
    Object.assign(member, updated)
    notice.value = `${memberName(member)} 的可用功能已更新`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '功能更新失败'
  } finally {
    savingUserId.value = null
  }
}

async function searchAllUsers() {
  if (!canChangeRoles.value) return
  error.value = ''
  try {
    const result = await assetWorkbenchApi.searchPeople({
      q: lookupQuery.value,
      scope: 'all_users',
      page_size: 20,
    })
    lookupResults.value = result.items
  } catch (err) {
    error.value = err instanceof Error ? err.message : '账号搜索失败'
  }
}

function selectLookupUser(user: WorkbenchMemberRow) {
  selectedLookupUser.value = user
}

function toggleOpenRole(role: string, checked: boolean) {
  const roles = new Set(openRoles.value)
  roles.add('AssetSubmitter')
  if (checked) roles.add(role)
  else if (role !== 'AssetSubmitter') roles.delete(role)
  openRoles.value = Array.from(roles)
}

async function openSelectedAccess() {
  const user = selectedLookupUser.value
  if (!user || !canChangeRoles.value) return
  savingUserId.value = user.user_id
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.openAccess({
      user_id: user.user_id,
      roles: openRoles.value,
      identity_type: 'staff',
      reason: openReason.value,
    })
    notice.value = `已为 ${memberName(user)} 开通资产工作台`
    lookupResults.value = []
    selectedLookupUser.value = null
    await loadMembers()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '开通失败'
  } finally {
    savingUserId.value = null
  }
}

function startDisable(member: WorkbenchMemberRow) {
  disableTarget.value = member
  disableReason.value = ''
}

async function disableSelectedAccess() {
  const member = disableTarget.value
  if (!member || !disableReason.value.trim()) {
    error.value = '停用必须填写原因'
    return
  }
  savingUserId.value = member.user_id
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.disableAccess({ user_id: member.user_id, reason: disableReason.value })
    notice.value = `已停用 ${memberName(member)} 的资产工作台访问`
    disableTarget.value = null
    await loadMembers()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '停用失败'
  } finally {
    savingUserId.value = null
  }
}

async function restoreMember(member: WorkbenchMemberRow) {
  if (!canChangeRoles.value) return
  savingUserId.value = member.user_id
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.openAccess({
      user_id: member.user_id,
      roles: ['AssetSubmitter'],
      identity_type: 'staff',
      reason: '恢复资产工作台访问',
    })
    notice.value = `已恢复 ${memberName(member)} 的资产工作台访问`
    await loadMembers()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '恢复失败'
  } finally {
    savingUserId.value = null
  }
}

async function previewMerge() {
  const source = Number(mergeSourceId.value)
  const canonical = Number(mergeCanonicalId.value)
  if (!source || !canonical) {
    error.value = '请输入来源账号编号和主账号编号'
    return
  }
  error.value = ''
  notice.value = ''
  try {
    const result = await assetWorkbenchApi.previewAccountMerge({ source_user_id: source, canonical_user_id: canonical })
    mergePreview.value = result
    const choices: Record<string, string> = {}
    for (const key of Object.keys(result.conflicts ?? {})) choices[key] = 'canonical'
    mergeChoices.value = choices
  } catch (err) {
    error.value = err instanceof Error ? err.message : '合并预览失败'
  }
}

async function confirmMerge() {
  if (!mergePreview.value) return
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.mergeAccounts({
      source_user_id: mergePreview.value.source_user_id,
      canonical_user_id: mergePreview.value.canonical_user_id,
      profile_choices: mergeChoices.value,
      reason: mergeReason.value,
    })
    notice.value = '账号已合并，历史真实收款人保持不变，工作台归属已迁移到主账号'
    mergePreview.value = null
    mergeSourceId.value = ''
    mergeCanonicalId.value = ''
    await loadMembers()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '账号合并失败'
  }
}

onMounted(() => {
  void loadPage()
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">成员</p>
        <h2>成员管理</h2>
        <p>{{ canChangeRoles ? '开通、停用、恢复和配置成员可用功能。' : '你可以查看成员，开通和调整仅超级管理员可用。' }}</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" :disabled="loading" @click="loadMembers">
          <RefreshCw :size="16" aria-hidden="true" />
          刷新
        </button>
      </div>
    </div>

    <p v-if="error" class="aw-inline-alert">{{ error }}</p>
    <p v-else-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <p v-if="!canChangeRoles" class="aw-inline-alert">当前账号只能查看成员，不会显示开通、停用、恢复或合并操作。</p>

    <div class="aw-two-column">
      <section class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">检索</p>
            <h3>工作台成员</h3>
          </div>
          <span class="aw-chip aw-chip--neutral">{{ total }} 人</span>
        </div>
        <div class="aw-form-grid">
          <label>
            成员状态
            <select v-model="statusFilter" @change="loadMembers">
              <option v-for="option in statusOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </label>
          <label>
            搜索成员
            <input v-model="query" placeholder="输入姓名、账号或手机号" @keydown.enter="loadMembers" />
          </label>
          <button class="aw-primary-button aw-form-grid__full" type="button" :disabled="loading" @click="loadMembers">
            <Search :size="16" aria-hidden="true" />
            搜索
          </button>
        </div>
      </section>

      <section v-if="canChangeRoles" class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">主动开通</p>
            <h3>开通主站账号</h3>
          </div>
          <ShieldCheck :size="18" aria-hidden="true" />
        </div>
        <div class="aw-form-grid">
          <label class="aw-form-grid__full">
            按姓名或账号搜索
            <input v-model="lookupQuery" placeholder="搜索主站账号" @keydown.enter="searchAllUsers" />
          </label>
          <button class="aw-secondary-button" type="button" @click="searchAllUsers">查找账号</button>
          <label class="aw-form-grid__full">
            开通原因
            <input v-model="openReason" />
          </label>
        </div>
        <div v-if="lookupResults.length" class="aw-compact-list">
          <button
            v-for="user in lookupResults"
            :key="user.user_id"
            class="aw-compact-list__item"
            type="button"
            @click="selectLookupUser(user)"
          >
            <span class="aw-template-option__icon">
              <UserRound :size="16" aria-hidden="true" />
            </span>
            <span>{{ memberName(user) }} · {{ user.username || user.user_id }}</span>
          </button>
        </div>
        <div v-if="selectedLookupUser" class="aw-panel">
          <p class="aw-copy">将为 {{ memberName(selectedLookupUser) }} 开通：</p>
          <label v-for="role in openableRoles" :key="role" class="aw-inline-check">
            <input
              type="checkbox"
              :checked="openRoles.includes(role)"
              :disabled="role === 'AssetSubmitter'"
              @change="toggleOpenRole(role, ($event.target as HTMLInputElement).checked)"
            />
            <span>{{ roleDisplayName(role) }}</span>
          </label>
          <button class="aw-primary-button" type="button" :disabled="savingUserId === selectedLookupUser.user_id" @click="openSelectedAccess">
            确认开通
          </button>
        </div>
      </section>
    </div>

    <section class="aw-data-surface">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">列表</p>
          <h3>成员维护</h3>
        </div>
        <ShieldCheck :size="18" aria-hidden="true" />
      </div>
      <div class="aw-compact-list">
        <article v-for="member in members" :key="member.user_id" class="aw-member-row">
          <div class="aw-member-row__identity">
            <span class="aw-template-option__icon">
              <UserRound :size="16" aria-hidden="true" />
            </span>
            <div>
              <strong>{{ memberName(member) }}</strong>
              <span>{{ member.username }} · {{ workerTypeMeta(member.worker_type).label }} · {{ member.job_grade || '未定级' }}</span>
            </div>
          </div>
          <div class="aw-member-row__badges">
            <span :class="chipClass(statusMeta(member.status).tone)">
              {{ statusMeta(member.status).label }}
            </span>
            <span v-for="label in memberLabels(member)" :key="`${member.user_id}-${label}`" class="aw-chip aw-chip--info">{{ label }}</span>
          </div>
          <div class="aw-member-row__controls">
            <label v-for="role in roleOptions" :key="`${member.user_id}-${role}`" class="aw-inline-check">
              <input
                type="checkbox"
                :checked="memberRoles(member).has(role)"
                :disabled="!canChangeRoles || member.status !== 'active' || savingUserId === member.user_id"
                @change="toggleRole(member, role, ($event.target as HTMLInputElement).checked)"
              />
              <span>{{ roleDisplayName(role) }}</span>
            </label>
            <button
              v-if="canChangeRoles && member.status === 'active'"
              class="aw-secondary-button"
              type="button"
              @click="startDisable(member)"
            >
              停用
            </button>
            <button
              v-if="canChangeRoles && member.status === 'disabled'"
              class="aw-primary-button"
              type="button"
              @click="restoreMember(member)"
            >
              恢复
            </button>
          </div>
        </article>
        <p v-if="!loading && members.length === 0" class="aw-inline-alert">没有找到成员</p>
      </div>
    </section>

    <section v-if="disableTarget" class="aw-panel">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">停用访问</p>
          <h3>{{ memberName(disableTarget) }}</h3>
        </div>
        <span class="aw-chip aw-chip--danger">需要原因</span>
      </div>
      <div class="aw-form-grid">
        <label class="aw-form-grid__full">
          停用原因
          <input v-model="disableReason" placeholder="离职、账号误开、暂停合作" />
        </label>
      </div>
      <div class="aw-inline-actions">
        <button class="aw-primary-button" type="button" @click="disableSelectedAccess">确认停用</button>
        <button class="aw-secondary-button" type="button" @click="disableTarget = null">取消</button>
      </div>
    </section>

    <section v-if="canChangeRoles" class="aw-panel">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">账号合并</p>
          <h3>外部账号合并到主账号</h3>
        </div>
        <span class="aw-chip aw-chip--warn">先预览再确认</span>
      </div>
      <p class="aw-copy">合并后工作台归属迁到主账号；历史已发放记录的真实收款人不会被改写。</p>
      <div class="aw-form-grid">
        <label>
          来源账号编号
          <input v-model="mergeSourceId" inputmode="numeric" placeholder="外部资产账号" />
        </label>
        <label>
          主账号编号
          <input v-model="mergeCanonicalId" inputmode="numeric" placeholder="运营主账号" />
        </label>
        <label class="aw-form-grid__full">
          合并原因
          <input v-model="mergeReason" />
        </label>
        <button class="aw-secondary-button" type="button" @click="previewMerge">预览影响</button>
      </div>
      <div v-if="mergePreview" class="aw-panel">
        <p class="aw-copy">{{ mergePreview.settlement_note }}</p>
        <div class="aw-material-related">
          <span v-for="(count, key) in mergePreview.counts" :key="key" class="aw-chip aw-chip--neutral">{{ key }}：{{ count }}</span>
        </div>
        <div v-if="Object.keys(mergePreview.conflicts ?? {}).length" class="aw-form-grid">
          <label v-for="(conflict, key) in mergePreview.conflicts" :key="key">
            {{ conflict.field }}
            <select v-model="mergeChoices[key]">
              <option value="canonical">保留主账号：{{ conflict.canonical_value || '空' }}</option>
              <option value="source">采用来源账号：{{ conflict.source_value || '空' }}</option>
            </select>
          </label>
        </div>
        <button class="aw-primary-button" type="button" @click="confirmMerge">确认合并</button>
      </div>
    </section>
  </section>
</template>
