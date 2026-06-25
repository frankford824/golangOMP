<template>
  <BaseModal
    :model-value="modelValue"
    :title="title"
    :show-confirm="false"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <template #default>
      <div class="space-y-3">
        <p class="text-sm text-[rgb(var(--yb-text-secondary))]">
          {{ description }}
        </p>
        <div v-if="loading" class="designer-loading text-sm text-[rgb(var(--yb-text-muted))] py-4">
          {{ loadingLabel }}
        </div>
        <div v-else-if="!designers.length" class="designer-empty text-sm text-[rgb(var(--yb-text-muted))] py-4">
          {{ emptyHint }}
        </div>
        <div v-else class="designer-list">
          <label
            v-for="designer in designers"
            :key="designer.id"
            class="designer-item"
          >
            <input
              v-model="selectedId"
              type="radio"
              class="designer-radio"
              :value="designer.id"
            />
            <span class="designer-name">{{ designer.name }}</span>
            <span class="designer-role">
              {{ designer.role === 'lead' ? leadRoleLabel : assigneeRoleLabel }}
            </span>
          </label>
        </div>
      </div>
    </template>
    <template #footer>
      <footer class="flex-shrink-0 flex justify-end gap-2 px-5 py-4 border-t border-[rgb(var(--yb-border-quiet))]">
        <BaseButton size="sm" variant="secondary" @click="$emit('update:modelValue', false)">
          取消
        </BaseButton>
        <BaseButton
          size="sm"
          variant="primary"
          :disabled="!selectedDesigner || loading"
          @click="onConfirm"
        >
          {{ confirmLabel }}
        </BaseButton>
      </footer>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import type { Designer } from '@/mock/designers'

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    designers: Designer[]
    loading?: boolean
    currentAssigneeId?: string | null
    title?: string
    description?: string
    loadingLabel?: string
    emptyHint?: string
    confirmLabel?: string
    assigneeRoleLabel?: string
    leadRoleLabel?: string
  }>(),
  {
    loading: false,
    title: '指派设计师',
    description: '请选择本次任务的负责设计师，后续审核与交班将以此为基础。',
    loadingLabel: '加载设计师列表...',
    emptyHint: '暂无可指派的设计师，请先在用户管理中配置设计师角色',
    confirmLabel: '确认指派',
    assigneeRoleLabel: '设计师',
    leadRoleLabel: '主设计',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  confirm: [{ assigneeId: string; assigneeName: string }]
}>()

const selectedId = ref<string | null>(null)

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      // 打开时根据当前指派人预选中
      selectedId.value = props.currentAssigneeId ?? null
    }
  }
)

const selectedDesigner = computed(() =>
  props.designers.find((d) => d.id === selectedId.value) ?? null
)

function onConfirm() {
  if (!selectedDesigner.value) return
  emit('confirm', {
    assigneeId: selectedDesigner.value.id,
    assigneeName: selectedDesigner.value.name,
  })
  emit('update:modelValue', false)
}
</script>

<style scoped>
.designer-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.designer-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid rgb(var(--yb-border));
  background-color: rgb(var(--yb-surface));
  cursor: pointer;
  transition: background-color 0.15s ease, border-color 0.15s ease;
}

.designer-item:hover {
  background-color: rgb(var(--yb-surface-soft));
  border-color: rgb(var(--yb-border-strong));
}

.designer-item:has(.designer-radio:checked) {
  border-color: rgb(var(--yb-brand));
  background-color: rgb(var(--yb-brand-soft));
}

.designer-item:has(.designer-radio:disabled) {
  opacity: 0.55;
  cursor: not-allowed;
}

.designer-radio {
  margin: 0;
  accent-color: rgb(var(--yb-brand));
}

.designer-name {
  font-size: 0.875rem;
  font-weight: 500;
  color: rgb(var(--yb-text));
}

.designer-role {
  margin-left: auto;
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted));
}

.designer-item:has(.designer-radio:checked) .designer-name {
  color: rgb(var(--yb-brand-strong));
}

.designer-item:has(.designer-radio:checked) .designer-role {
  color: rgb(var(--yb-brand));
}
</style>

