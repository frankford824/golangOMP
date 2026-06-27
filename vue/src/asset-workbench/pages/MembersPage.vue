<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RefreshCw, Search, ShieldCheck, UserRound } from 'lucide-vue-next'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { assetWorkbenchApi, type WorkbenchMemberRow } from '@aw/shared/api/assetWorkbenchApi'
import { chipClass, workerTypeMeta } from '@aw/shared/format/status'

const loading = ref(false)
const savingUserId = ref<number | null>(null)
const error = ref('')
const notice = ref('')
const query = ref('')
const members = ref<WorkbenchMemberRow[]>([])
const { bootstrap, refresh: refreshBootstrap } = useAssetWorkbenchBootstrap()

const total = computed(() => members.value.length)
const canChangeIdentity = computed(() =>
  (bootstrap.value?.capabilities ?? []).includes('asset.workbench.member.identity'),
)

function memberName(member: WorkbenchMemberRow) {
  return member.real_name || member.display_name || member.username || `用户 ${member.user_id}`
}

async function loadMembers() {
  loading.value = true
  error.value = ''
  try {
    const result = await assetWorkbenchApi.listMembers({
      q: query.value,
      page_size: 200,
    })
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

async function setIdentity(member: WorkbenchMemberRow, identity: 'admin' | 'normal') {
  if (member.identity === identity) return
  if (!canChangeIdentity.value) {
    error.value = '仅超级管理员可以切换成员身份'
    return
  }
  savingUserId.value = member.user_id
  error.value = ''
  notice.value = ''
  try {
    const updated = await assetWorkbenchApi.updateMemberIdentity(member.user_id, identity, 'workbench identity update')
    member.identity = updated.identity
    notice.value = `${memberName(member)} 已切换为${identity === 'admin' ? '管理员' : '普通用户'}`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '身份切换失败'
  } finally {
    savingUserId.value = null
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
        <p class="aw-eyebrow">权限</p>
        <h2>成员与权限</h2>
        <p>{{ canChangeIdentity ? '按姓名找到账号，只切换资产工作台里的管理员或普通用户身份。' : '你可以查看成员，身份切换仅超级管理员可用。' }}</p>
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
    <p v-if="!canChangeIdentity" class="aw-inline-alert">仅超级管理员可以切换成员身份。</p>

    <section class="aw-panel">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">搜索</p>
          <h3>按姓名或账号搜索</h3>
        </div>
        <span class="aw-chip aw-chip--neutral">{{ total }} 人</span>
      </div>
      <div class="aw-form-grid">
        <label class="aw-form-grid__full">
          <span>搜索成员</span>
          <input v-model="query" placeholder="输入姓名、账号或手机号" @keydown.enter="loadMembers" />
        </label>
        <button class="aw-primary-button" type="button" :disabled="loading" @click="loadMembers">
          <Search :size="16" aria-hidden="true" />
          搜索
        </button>
      </div>
    </section>

    <section class="aw-data-surface">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">列表</p>
          <h3>工作台成员</h3>
        </div>
        <ShieldCheck :size="18" aria-hidden="true" />
      </div>
      <div class="aw-compact-list">
        <article v-for="member in members" :key="member.user_id" class="aw-compact-list__item">
          <div>
            <span class="aw-template-option__icon">
              <UserRound :size="16" aria-hidden="true" />
            </span>
            <div>
              <strong>{{ memberName(member) }}</strong>
              <span>{{ member.username }} · {{ workerTypeMeta(member.worker_type).label }} · {{ member.job_grade || '未定级' }}</span>
            </div>
          </div>
          <div class="aw-page-bar__actions">
            <span :class="chipClass(member.identity === 'admin' ? 'info' : 'neutral')">
              {{ member.identity === 'admin' ? '管理员' : '普通用户' }}
            </span>
            <select
              :value="member.identity"
              :disabled="!canChangeIdentity || savingUserId === member.user_id"
              @change="setIdentity(member, ($event.target as HTMLSelectElement).value as 'admin' | 'normal')"
            >
              <option value="normal">普通用户</option>
              <option value="admin">管理员</option>
            </select>
          </div>
        </article>
        <p v-if="!loading && members.length === 0" class="aw-inline-alert">没有找到成员</p>
      </div>
    </section>
  </section>
</template>
