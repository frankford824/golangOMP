<template>
  <section class="space-y-4">
    <div class="rounded-xl border border-[var(--v1-border)] bg-white p-4">
      <div class="flex items-center justify-between gap-2">
        <div>
          <h1 class="text-base font-semibold text-[var(--v1-text-primary)]">任务草稿</h1>
          <p class="mt-1 text-xs text-[var(--v1-text-secondary)]">
            管理当前账号的未提交草稿（最多 20 条，7 天过期）。
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="rounded border border-[var(--v1-border)] bg-white px-3 py-1 text-xs text-[var(--v1-text-secondary)]"
            :disabled="loading"
            @click="loadDrafts"
          >
            {{ loading ? '刷新中...' : '刷新' }}
          </button>
          <router-link
            class="rounded bg-[var(--v1-bg-primary)] px-3 py-1 text-xs text-white"
            to="/tasks/create?create=1"
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
      <div class="space-y-3">
        <article
          v-for="draft in drafts"
          :key="draft.id"
          class="rounded-xl border border-[var(--v1-border)] bg-white p-4"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="text-sm font-semibold text-[var(--v1-text-primary)]">
                {{ taskTypeLabel(draft.task_type) }}
                <span class="ml-2 text-xs font-normal text-[var(--v1-text-secondary)]">#{{ draft.id }}</span>
              </p>
              <p class="mt-1 text-xs text-[var(--v1-text-secondary)]">
                SKU：{{ pickFormValue(draft, 'sku') || '-' }} · 产品：{{ pickFormValue(draft, 'productName') || '-' }}
              </p>
              <p class="mt-1 text-xs text-[var(--v1-text-secondary)]">
                更新于：{{ displayTime(draft.updated_at || draft.created_at) }}
                <span v-if="draft.expires_at"> · 过期：{{ displayTime(draft.expires_at) }}</span>
              </p>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <router-link
                class="rounded border border-[var(--v1-border)] px-2 py-1 text-xs text-[var(--v1-text-secondary)]"
                :to="`/tasks/create?create=1&draft_id=${encodeURIComponent(draft.id)}`"
              >
                前往创建
              </router-link>
              <button
                type="button"
                class="rounded border border-red-200 px-2 py-1 text-xs text-red-600"
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
      </div>
    </AsyncStateWrapper>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
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
const loading = ref(false)
const deletingId = ref('')
const errorMessage = ref('')

const TASK_TYPE_LABELS: Record<string, string> = {
  ORIGINAL_PRODUCT_DEV: '原款开发',
  NEW_PRODUCT_DEV: '新款开发',
  PURCHASE_TASK: '采购任务',
  RETOUCH_TASK: 'P 图任务',
}

function taskTypeLabel(taskType: string): string {
  return TASK_TYPE_LABELS[taskType] ?? taskType ?? '未命名草稿'
}

function displayTime(value?: string): string {
  if (!value) return '-'
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
