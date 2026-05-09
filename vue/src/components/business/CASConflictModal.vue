<template>
  <BaseModal
    :model-value="modelValue"
    title="版本冲突"
    confirm-text="使用服务端版本"
    show-confirm
    @update:model-value="$emit('update:modelValue', $event)"
    @confirm="$emit('useServer')"
    @cancel="$emit('cancel')"
  >
    <div class="conflict-content">
      <p class="conflict-hint">当前数据与服务端不一致，请选择保留的版本。</p>
      <div class="version-cards">
        <div class="version-card client">
          <h4>当前客户端</h4>
          <p><span class="label">校验值:</span> {{ clientVersion?.hash ?? '-' }}</p>
          <p><span class="label">更新时间:</span> {{ clientVersion?.updatedAt ?? '-' }}</p>
        </div>
        <div class="version-card server">
          <h4>服务端</h4>
          <p><span class="label">校验值:</span> {{ serverVersion?.hash ?? '-' }}</p>
          <p><span class="label">更新时间:</span> {{ serverVersion?.updatedAt ?? '-' }}</p>
        </div>
      </div>
    </div>
  </BaseModal>
</template>

<script setup lang="ts">
import BaseModal from '@/components/base/BaseModal.vue'

export interface VersionSummary {
  hash?: string
  updatedAt?: string
}

withDefaults(
  defineProps<{
    modelValue: boolean
    clientVersion?: VersionSummary | null
    serverVersion?: VersionSummary | null
  }>(),
  {
    clientVersion: () => ({}),
    serverVersion: () => ({}),
  }
)

defineEmits<{ 'update:modelValue': [boolean]; useServer: []; cancel: [] }>()
</script>

<style scoped>
.conflict-content {
  font-size: 0.875rem;
}
.conflict-hint {
  margin: 0 0 1rem;
  color: #475569;
}
.version-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
}
.version-card {
  padding: 0.75rem;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
}
.version-card h4 {
  margin: 0 0 0.5rem;
  font-size: 0.8125rem;
  font-weight: 600;
  color: #0f172a;
}
.version-card p {
  margin: 0.25rem 0;
  font-size: 0.8125rem;
  color: #475569;
}
.version-card .label {
  color: #64748b;
  margin-right: 0.25rem;
}
</style>
