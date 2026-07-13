<template>
  <BaseModal
    :model-value="modelValue"
    title="确认修改资源"
    :show-confirm="false"
    panel-class="asset-operation-dialog-panel"
    @update:model-value="onVisibilityChange"
  >
    <section class="grid gap-4 pb-4">
      <div class="rounded-xl border border-[rgb(var(--yb-warning)_/_0.28)] bg-[rgb(var(--yb-warning)_/_0.08)] p-4">
        <strong class="block text-[rgb(var(--yb-text))]">本次操作会新增版本并替换当前资源</strong>
        <p class="mt-1 text-xs leading-5 text-[rgb(var(--yb-text-muted))]">
          原版本仍保留在版本记录中；已结单任务只更新资源，不改变任务状态。请确认文件无误后再提交。
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

      <div class="rounded-xl border border-dashed border-[rgb(var(--yb-border-strong))] bg-[rgb(var(--yb-surface-soft))] p-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="min-w-0">
            <strong class="block truncate text-sm text-[rgb(var(--yb-text))]" :title="selectedFile?.name">
              {{ selectedFile ? selectedFile.name : '尚未选择新文件' }}
            </strong>
            <span class="mt-1 block text-xs text-[rgb(var(--yb-text-muted))]">
              {{ selectedFile ? formatFileSize(selectedFile.size) : '选择后仍需点击“确认替换”才会上传' }}
            </span>
          </div>
          <BaseButton variant="secondary" size="sm" :disabled="uploading" @click="$emit('choose-file')">
            {{ selectedFile ? '重新选择' : '选择文件' }}
          </BaseButton>
        </div>
      </div>

      <p v-if="status" class="m-0 text-sm text-[rgb(var(--yb-brand))]" role="status" aria-live="polite">
        {{ status }}
      </p>
      <p v-if="error" class="m-0 text-sm text-[rgb(var(--yb-danger))]" role="alert">
        {{ error }}
      </p>
    </section>

    <template #footer>
      <footer class="flex flex-wrap justify-end gap-2 border-t border-[rgb(var(--yb-border))] px-4 py-3 sm:px-5 sm:py-4">
        <BaseButton variant="secondary" size="sm" :disabled="uploading" @click="$emit('cancel')">
          取消
        </BaseButton>
        <BaseButton variant="primary" size="sm" :disabled="!selectedFile || uploading" @click="$emit('confirm')">
          {{ uploading ? '替换中…' : '确认替换' }}
        </BaseButton>
      </footer>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import BaseButton from '@/components/base/BaseButton.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import { formatFileSizeBytes } from '@/domain/formatters/file-size'

const props = defineProps<{
  modelValue: boolean
  taskNo?: string
  sku?: string
  assetKind?: string
  currentFileName?: string
  selectedFile?: File | null
  uploading?: boolean
  status?: string
  error?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [boolean]
  'choose-file': []
  confirm: []
  cancel: []
}>()

function onVisibilityChange(open: boolean) {
  if (!open && props.uploading) return
  emit('update:modelValue', open)
  if (!open) emit('cancel')
}

function formatFileSize(size: number): string {
  return size > 0 ? formatFileSizeBytes(size) : '文件大小未知'
}
</script>
