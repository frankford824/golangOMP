<template>
  <BaseModal
    :model-value="modelValue"
    title="确认删除资源"
    :show-confirm="false"
    panel-max-width="36rem"
    @update:model-value="onVisibilityChange"
  >
    <section class="grid gap-4 pb-4">
      <div class="rounded-xl border border-[rgb(var(--yb-danger-border))] bg-[rgb(var(--yb-danger-soft))] p-4">
        <strong class="block text-[rgb(var(--yb-danger-text))]">删除后无法恢复，请谨慎操作</strong>
        <p class="mt-1 text-xs leading-5 text-[rgb(var(--yb-danger-text))]">
          当前资源及全部历史版本的存储文件都会被删除。已结单任务只删除资源，不会重新打开或改变任务状态。
        </p>
      </div>

      <dl class="grid grid-cols-[5.5rem_minmax(0,1fr)] gap-x-3 gap-y-2 text-sm">
        <dt class="text-[rgb(var(--yb-text-muted))]">任务</dt>
        <dd class="min-w-0 truncate font-mono text-[rgb(var(--yb-text))]">{{ taskNo || '—' }}</dd>
        <dt class="text-[rgb(var(--yb-text-muted))]">SKU</dt>
        <dd class="min-w-0 truncate font-mono text-[rgb(var(--yb-text))]">{{ sku || '—' }}</dd>
        <dt class="text-[rgb(var(--yb-text-muted))]">资源类型</dt>
        <dd class="min-w-0 truncate text-[rgb(var(--yb-text))]">{{ assetKind || '—' }}</dd>
        <dt class="text-[rgb(var(--yb-text-muted))]">当前文件</dt>
        <dd class="min-w-0 truncate text-[rgb(var(--yb-text))]" :title="currentFileName">
          {{ currentFileName || '—' }}
        </dd>
      </dl>

      <BaseTextarea
        :model-value="reason"
        label="删除原因"
        placeholder="请填写删除原因，便于后续审计追溯"
        :rows="3"
        :disabled="deleting"
        hint="必填；提交后会记录操作人、原因和资源信息。"
        :error="reasonError"
        @update:model-value="$emit('update:reason', $event)"
      />

      <p v-if="status" class="m-0 text-sm text-[rgb(var(--yb-brand))]" role="status" aria-live="polite">
        {{ status }}
      </p>
      <p v-if="error" class="m-0 text-sm text-[rgb(var(--yb-danger))]" role="alert">
        {{ error }}
      </p>
    </section>

    <template #footer>
      <footer class="flex flex-wrap justify-end gap-2 border-t border-[rgb(var(--yb-border))] px-4 py-3 sm:px-5 sm:py-4">
        <BaseButton variant="secondary" size="sm" :disabled="deleting" @click="$emit('cancel')">
          取消
        </BaseButton>
        <BaseButton variant="danger" size="sm" :disabled="!reason.trim() || deleting" @click="$emit('confirm')">
          {{ deleting ? '删除中…' : '确认删除' }}
        </BaseButton>
      </footer>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import BaseButton from '@/components/base/BaseButton.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'

const props = defineProps<{
  modelValue: boolean
  taskNo?: string
  sku?: string
  assetKind?: string
  currentFileName?: string
  reason: string
  reasonError?: string
  deleting?: boolean
  status?: string
  error?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [boolean]
  'update:reason': [string]
  confirm: []
  cancel: []
}>()

function onVisibilityChange(open: boolean) {
  if (!open && props.deleting) return
  emit('update:modelValue', open)
  if (!open) emit('cancel')
}
</script>
