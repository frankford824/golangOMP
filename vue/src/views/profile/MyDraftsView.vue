<template>
  <section class="space-y-4">
    <div class="rounded-xl border border-[var(--v1-border)] bg-[rgb(var(--yb-surface))] p-4">
      <div class="flex items-center justify-between gap-2">
        <div>
          <h1 :class="{ 'oa-draft-title': oaEnabled }" class="text-base font-semibold text-[var(--v1-text-primary)]">任务草稿</h1>
          <p class="mt-1 text-xs text-[var(--v1-text-secondary)]">
            管理当前账号的未提交草稿（最多 20 条，7 天过期）。
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="inline-flex min-h-9 items-center justify-center rounded border border-[var(--v1-border)] bg-[rgb(var(--yb-surface))] px-3 text-xs text-[var(--v1-text-secondary)]"
            :disabled="loading"
            @click="loadDrafts"
          >
            {{ loading ? '刷新中...' : '刷新' }}
          </button>
          <router-link
            class="inline-flex min-h-9 items-center justify-center rounded bg-[var(--v1-bg-primary)] px-3 text-xs text-[rgb(var(--yb-text-inverse))]"
            :to="{ name: 'TaskCreate', query: { create: '1', taskFrom: '/me/task-drafts' } }"
          >
            创建任务
          </router-link>
        </div>
      </div>
    </div>

    <AsyncStateWrapper
      :loading="loading"
      :error="errorMessage"
      :empty="!loading && !errorMessage && drafts.length === 0"
      empty-title="暂无草稿"
      empty-description="点击右上角「创建任务」开始填写，关闭前可保存为草稿。"
      error-title="草稿加载失败"
      @retry="loadDrafts"
    >
      <component v-for="group in draftGroups" :key="group.expired ? 'expired' : 'active'" :is="group.expired ? 'details' : 'div'" class="space-y-3">
        <summary v-if="group.expired" class="expired-summary">已过期（{{ group.items.length }}）</summary>
        <article
          v-for="draft in group.items"
          :key="draft.id"
          :class="{ 'expired-draft': group.expired }"
          class="rounded-xl border border-[var(--v1-border)] bg-[rgb(var(--yb-surface))] p-4"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="text-sm font-semibold text-[var(--v1-text-primary)]">
                {{ taskTypeLabel(draft.task_type) }}
                <span class="ml-2 text-xs font-normal text-[var(--v1-text-secondary)]">#{{ draft.id }}</span>
              </p>
              <p class="mt-1 text-xs text-[var(--v1-text-secondary)]">
                SKU：{{ pickFormValue(draft, 'sku') || (oaEnabled ? '—' : '-') }} · 产品：{{ pickFormValue(draft, 'productName') || (oaEnabled ? '—' : '-') }}
              </p>
              <p class="mt-1 text-xs text-[var(--v1-text-secondary)]">
                更新于：{{ displayTime(draft.updated_at || draft.created_at) }}
                <span v-if="draft.expires_at"> · 过期：{{ displayTime(draft.expires_at) }}</span>
              </p>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <router-link
                v-if="!group.expired"
                class="inline-flex min-h-8 items-center justify-center rounded border border-[var(--v1-border)] px-2 text-xs text-[var(--v1-text-secondary)]"
                :to="{ name: 'TaskCreate', query: { create: '1', draft_id: draft.id, taskFrom: '/me/task-drafts' } }"
              >
                继续编辑
              </router-link>
              <button
                type="button"
                class="inline-flex min-h-8 items-center justify-center rounded border border-[rgb(var(--yb-danger-border))] px-2 text-xs text-[rgb(var(--yb-danger))]"
                :disabled="deletingId === draft.id"
                @click="removeDraft(draft.id)"
              >
                {{ deletingId === draft.id ? '删除中...' : '删除草稿' }}
              </button>
            </div>
          </div>
          <p class="mt-2 line-clamp-2 text-xs text-[var(--v1-text-secondary)]">
            备注：{{ pickFormValue(draft, 'note') || pickFormValue(draft, 'designRequirement') || '—' }}
          </p>
        </article>
      </component>
    </AsyncStateWrapper>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { oaEnabled } from '@/oa/access'
import AsyncStateWrapper from '@/components/base/AsyncStateWrapper.vue'
import { useTaskDraft } from '@/composables/useTaskDraft'
import { resolveApiUserMessage } from '@/utils/api-message-zh'
import { formatDateTimeBeijing } from '@/utils/date'

interface DraftLike {
  id: string
  task_type: string
  payload: Record<string, unknown>
  created_at?: string
  updated_at?: string
  expires_at?: string
}

const { drafts, load, remove } = useTaskDraft()
const draftGroups = computed(() => {
  if (!oaEnabled) return [{ expired: false, items: drafts.value }]
  const current = []
  const expired = []
  const now = Date.now()
  for (const draft of drafts.value) {
    if (draft.expires_at && Date.parse(draft.expires_at) <= now) expired.push(draft)
    else current.push(draft)
  }
  return [{ expired: false, items: current }, { expired: true, items: expired }].filter((group) => group.items.length)
})
const oaDate = new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })
const loading = ref(false)
const deletingId = ref('')
const errorMessage = ref('')

const TASK_TYPE_LABELS: Record<string, string> = {
  ORIGINAL_PRODUCT_DEV: '原款开发',
  NEW_PRODUCT_DEV: '新款开发',
  SKU_PLANNING: '策划 SKU',
  RETOUCH_TASK: 'P 图任务',
}

function taskTypeLabel(taskType: string): string {
  return TASK_TYPE_LABELS[taskType] ?? taskType ?? '未命名草稿'
}

function displayTime(value?: string): string {
  if (!value) return oaEnabled ? '—' : '-'
  if (oaEnabled && Number.isFinite(Date.parse(value))) return oaDate.format(new Date(value))
  return formatDateTimeBeijing(value) || value
}

function pickFormValue(draft: DraftLike, key: string): string {
  const form = draft.payload?.form
  if (!form || typeof form !== 'object') return ''
  const raw = (form as Record<string, unknown>)[key]
  if (raw == null) return ''
  return String(raw)
}

async function loadDrafts(): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  try {
    await load()
  } catch (error) {
    errorMessage.value = resolveApiUserMessage(error, { fallback: '草稿列表加载失败，请稍后重试' })
  } finally {
    loading.value = false
  }
}

async function removeDraft(id: string): Promise<void> {
  if (!id) return
  if (!window.confirm('确认删除该草稿？删除后不可恢复。')) return
  deletingId.value = id
  try {
    await remove(id)
  } catch (error) {
    errorMessage.value = resolveApiUserMessage(error, { fallback: '删除草稿失败，请稍后重试' })
  } finally {
    deletingId.value = ''
  }
}

onMounted(loadDrafts)
</script>

<style scoped>
.oa-draft-title { font-size: 18px; font-weight: 600; }
.expired-summary { cursor: pointer; padding: 12px; color: rgb(var(--yb-text-muted-strong)); }
.expired-draft { background: rgb(var(--yb-bg-page)); }
</style>
