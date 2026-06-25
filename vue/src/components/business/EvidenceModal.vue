<template>
  <BaseModal
    :model-value="modelValue"
    title="资产证据信息"
    :show-confirm="false"
    cancel-text="关闭"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <div v-if="evidence" class="evidence-content">
      <p><span class="label">版本类型:</span> {{ versionTypeLabel(evidence.type) }}</p>
      <p><span class="label">上传人:</span> {{ evidence.uploaderName }}</p>
      <p><span class="label">上传时间:</span> {{ evidence.uploadedAt }}</p>
      <p v-if="evidence.note"><span class="label">备注:</span> {{ evidence.note }}</p>
      <p v-if="evidence.fileRefs?.length"><span class="label">文件引用:</span> {{ evidence.fileRefs[0] }}</p>
      <p v-if="evidence.evidenceHash"><span class="label">校验值:</span> <code>{{ evidence.evidenceHash }}</code></p>
      <p v-if="evidence.evidenceNote"><span class="label">证据说明:</span> {{ evidence.evidenceNote }}</p>
    </div>
    <p v-else class="evidence-empty">暂无证据信息</p>
  </BaseModal>
</template>

<script setup lang="ts">
import BaseModal from '@/components/base/BaseModal.vue'
import type { AssetVersionType, TaskAssetVersion } from '@/domain/types/task'

export interface EvidenceInfo extends TaskAssetVersion {
  evidenceHash?: string
  evidenceNote?: string
}

defineProps<{
  modelValue: boolean
  evidence?: EvidenceInfo | null
}>()

defineEmits<{ 'update:modelValue': [boolean] }>()

const VERSION_TYPE_LABELS: Record<AssetVersionType, string> = {
  reference: '参考图',
  draft: '初稿',
  revision: '修改稿',
  final: '终稿',
  derivative: '二创稿',
}

function versionTypeLabel(type?: string): string {
  if (!type) return '—'
  return VERSION_TYPE_LABELS[type as AssetVersionType] ?? type
}
</script>

<style scoped>
.evidence-content p {
  margin: 0.5rem 0;
  font-size: 0.875rem;
  color: rgb(var(--yb-text-slate));
}
.evidence-content .label {
  color: rgb(var(--yb-text-muted-strong));
  margin-right: 0.35rem;
}
.evidence-content code {
  font-size: 0.8125rem;
  background: rgb(var(--yb-surface-slate));
  padding: 0.15rem 0.35rem;
  border-radius: 4px;
}
.evidence-empty {
  font-size: 0.875rem;
  color: rgb(var(--yb-text-placeholder));
}
</style>
