<template>
  <div class="detail-card h-full flex flex-col rounded-lg border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface))] shadow-sm p-6">
    <div class="sidebar-header">ERP 建档状态</div>
    <template v-if="hasFilingInfo">
      <div class="filing-status-row">
        <span class="filing-label">当前状态</span>
        <FilingStatusBadge
          :status="props.task?.filing_status"
          :task-type="props.task?.businessType ?? props.task?.taskType"
          :missing-fields-summary="props.task?.missing_fields_summary_cn"
          :error-message="props.task?.filing_error_message"
        />
      </div>
      <div v-if="props.task?.missing_fields_summary_cn" class="filing-detail-row">
        <span class="filing-label">缺失字段</span>
        <span class="filing-value text-[rgb(var(--yb-warning-text))]">{{ props.task.missing_fields_summary_cn }}</span>
      </div>
      <div v-if="businessErrorMessage" class="filing-detail-row">
        <span class="filing-label">失败原因</span>
        <span class="filing-value text-[rgb(var(--yb-danger-text))]">{{ businessErrorMessage }}</span>
      </div>
      <div v-if="props.task?.last_filed_at" class="filing-detail-row">
        <span class="filing-label">最近同步时间</span>
        <span class="filing-value">{{ formatDate(props.task.last_filed_at) }}</span>
      </div>
      <div v-if="props.task?.last_filing_attempt_at && props.task.last_filing_attempt_at !== props.task?.last_filed_at" class="filing-detail-row">
        <span class="filing-label">最近尝试时间</span>
        <span class="filing-value">{{ formatDate(props.task.last_filing_attempt_at) }}</span>
      </div>
      <div v-if="props.task?.erp_sync_required != null" class="filing-detail-row">
        <span class="filing-label">仍需同步</span>
        <span class="filing-value">{{ props.task.erp_sync_required ? '是' : '否' }}</span>
      </div>
      <div v-if="showRetryButton" class="filing-retry-row mt-2">
        <BaseButton
          variant="secondary"
          size="sm"
          :loading="retrying"
          :disabled="retrying"
          @click="onRetry"
        >
          重试同步
        </BaseButton>
      </div>
    </template>
    <p v-else class="filing-empty text-[rgb(var(--yb-text-muted))] text-sm">暂无建档状态信息</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Task } from '@/domain/types/task'
import { taskNeedsErpFilingRetry } from '@/domain/erp-filing-retry'
import { tasksApi } from '@/services/api/tasksApi'
import FilingStatusBadge from '@/components/business/FilingStatusBadge.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import { formatDateBeijing } from '@/utils/date'
import { formatErpSyncFailureMessage } from '@/utils/business-copy'

const props = defineProps<{ task: Task | null }>()

const hasFilingInfo = computed(() => {
  const t = props.task
  if (!t) return false
  return (
    t.filing_status != null ||
    t.missing_fields_summary_cn != null ||
    t.filing_error_message != null ||
    t.last_filed_at != null ||
    t.last_filing_attempt_at != null ||
    t.erp_sync_required != null
  )
})

const showRetryButton = computed(() => taskNeedsErpFilingRetry(props.task))
const businessErrorMessage = computed(() => formatErpSyncFailureMessage(props.task?.filing_error_message ?? ''))

const retrying = ref(false)
const emit = defineEmits<{ refreshed: [] }>()

function formatDate(iso: string): string {
  return formatDateBeijing(iso)
}

async function onRetry() {
  const t = props.task
  if (!t?.id || retrying.value) return
  retrying.value = true
  try {
    await tasksApi.retryFiling(t.id)
    emit('refreshed')
  } catch {
    // 错误由调用方通过 actionError 展示，或可在此加 toast
  } finally {
    retrying.value = false
  }
}
</script>

<style scoped>
.filing-status-row,
.filing-detail-row {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  margin-bottom: 0.5rem;
}
.filing-label {
  font-size: 0.6875rem;
  font-weight: 500;
  color: rgb(var(--yb-text-muted-strong));
}
.filing-value {
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-slate));
}
.filing-empty {
  margin: 0;
}
.filing-retry-row {
  padding-top: 0.5rem;
  border-top: 1px solid rgb(var(--yb-surface-slate));
}
</style>
