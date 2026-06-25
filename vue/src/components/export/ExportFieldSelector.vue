<template>
  <div class="field-selector">
    <div class="selector-panel">
      <div class="panel-header">
        <span class="panel-title">可选字段</span>
        <button type="button" class="panel-action" @click="addAll">全部添加</button>
      </div>
      <div class="field-list">
        <button
          v-for="field in availableFields"
          :key="field.key"
          type="button"
          class="field-chip"
          @click="addField(field.key)"
        >
          {{ field.label }}
          <span class="chip-add">+</span>
        </button>
        <div v-if="!availableFields.length" class="empty-fields">全部字段已选中</div>
      </div>
    </div>

    <div class="selector-arrows">
      <button type="button" class="arrow-btn" title="添加全部" @click="addAll">&raquo;</button>
      <button type="button" class="arrow-btn" title="移除全部" @click="removeAll">&laquo;</button>
    </div>

    <div class="selector-panel">
      <div class="panel-header">
        <span class="panel-title">导出字段 ({{ selectedFields.length }})</span>
        <button type="button" class="panel-action" @click="removeAll">清空</button>
      </div>
      <component
        :is="DraggableList"
        :fields="selectedFields"
        @remove="removeField"
        @move-up="moveUp"
        @move-down="moveDown"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

export interface ExportField {
  key: string
  label: string
}

const props = defineProps<{
  allFields: ExportField[]
  modelValue: string[] // selected field keys
}>()

const emit = defineEmits<{
  'update:modelValue': [string[]]
}>()

const selectedFields = computed((): ExportField[] =>
  props.modelValue.map((k) => props.allFields.find((f) => f.key === k)!).filter(Boolean),
)

const availableFields = computed((): ExportField[] =>
  props.allFields.filter((f) => !props.modelValue.includes(f.key)),
)

function addField(key: string) {
  emit('update:modelValue', [...props.modelValue, key])
}

function removeField(key: string) {
  emit('update:modelValue', props.modelValue.filter((k) => k !== key))
}

function addAll() {
  emit('update:modelValue', props.allFields.map((f) => f.key))
}

function removeAll() {
  emit('update:modelValue', [])
}

function moveUp(key: string) {
  const arr = [...props.modelValue]
  const i = arr.indexOf(key)
  if (i > 0) {
    ;[arr[i - 1], arr[i]] = [arr[i], arr[i - 1]]
    emit('update:modelValue', arr)
  }
}

function moveDown(key: string) {
  const arr = [...props.modelValue]
  const i = arr.indexOf(key)
  if (i < arr.length - 1) {
    ;[arr[i], arr[i + 1]] = [arr[i + 1], arr[i]]
    emit('update:modelValue', arr)
  }
}

// Inline draggable-list as a local functional component
const DraggableList = {
  props: ['fields'],
  emits: ['remove', 'move-up', 'move-down'],
  template: `
    <div class="selected-list">
      <div v-if="!fields.length" class="empty-fields">请从左侧选择字段</div>
      <div v-for="(f, i) in fields" :key="f.key" class="selected-item">
        <span class="drag-handle">::</span>
        <span class="selected-label">{{ f.label }}</span>
        <div class="item-actions">
          <button type="button" class="order-btn" :disabled="i === 0" @click="$emit('move-up', f.key)">↑</button>
          <button type="button" class="order-btn" :disabled="i === fields.length - 1" @click="$emit('move-down', f.key)">↓</button>
          <button type="button" class="remove-btn" @click="$emit('remove', f.key)">×</button>
        </div>
      </div>
    </div>
  `,
}
// DraggableList is used via <component :is> — no defineOptions needed
</script>

<style scoped>
.field-selector {
  display: grid;
  grid-template-columns: 1fr 2.5rem 1fr;
  gap: 0.5rem;
  align-items: start;
}
.selector-panel {
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  overflow: hidden;
  background: #fff;
}
.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.375rem 0.625rem;
  background: #f8fafc;
  border-bottom: 1px solid #e2e8f0;
}
.panel-title { font-size: 0.75rem; font-weight: 600; color: #374151; }
.panel-action { font-size: 0.6875rem; color: #1890ff; background: none; border: none; cursor: pointer; }
.field-list, :deep(.selected-list) {
  padding: 0.375rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-height: 10rem;
  max-height: 16rem;
  overflow-y: auto;
}
.field-chip {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.25rem 0.5rem;
  border: 1px solid #e5e7eb;
  border-radius: 4px;
  background: #f9fafb;
  font-size: 0.75rem;
  color: #374151;
  cursor: pointer;
  transition: border-color 0.15s;
  text-align: left;
  width: 100%;
}
.field-chip:hover { border-color: #1890ff; color: #1d4ed8; }
.chip-add { font-size: 0.875rem; color: #9ca3af; }
.empty-fields { font-size: 0.75rem; color: #9ca3af; padding: 0.5rem; text-align: center; }
.selector-arrows {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  padding-top: 1.5rem;
}
.arrow-btn {
  width: 2rem;
  height: 2rem;
  border: 1px solid #e2e8f0;
  border-radius: 4px;
  background: #fff;
  color: #374151;
  cursor: pointer;
  font-size: 0.875rem;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: border-color 0.15s;
}
.arrow-btn:hover { border-color: #1890ff; color: #1d4ed8; }
:deep(.selected-item) {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.25rem 0.375rem;
  border: 1px solid #e5e7eb;
  border-radius: 4px;
  background: #f0fdf4;
  font-size: 0.75rem;
}
:deep(.drag-handle) { color: #9ca3af; cursor: grab; font-size: 0.625rem; }
:deep(.selected-label) { flex: 1; color: #374151; }
:deep(.item-actions) { display: flex; gap: 0.125rem; }
:deep(.order-btn) {
  width: 1.25rem;
  height: 1.25rem;
  border: none;
  background: none;
  cursor: pointer;
  font-size: 0.6875rem;
  color: #6b7280;
  padding: 0;
}
:deep(.order-btn:disabled) { opacity: 0.3; cursor: not-allowed; }
:deep(.remove-btn) {
  width: 1.25rem;
  height: 1.25rem;
  border: none;
  background: none;
  cursor: pointer;
  font-size: 0.875rem;
  color: #dc2626;
  padding: 0;
}

/* Apple Music / iOS liquid glass export field selector skin. Style-only. */
.selector-panel {
  border-radius: 0.85rem;
  border-color: rgba(255, 255, 255, 0.16);
  background: rgba(14, 15, 22, 0.78);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08);
}

.panel-header {
  border-color: rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.075);
}

.panel-title {
  color: rgba(220, 230, 255, 0.78);
}

.panel-action {
  color: var(--yb-music-cyan);
  font-weight: 700;
}

.field-list,
:deep(.selected-list) {
  scrollbar-color: rgba(220, 230, 255, 0.36) transparent;
}

.field-chip,
:deep(.selected-item) {
  border-radius: 0.65rem;
  border-color: rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.065);
  color: var(--yb-music-text-2);
}

.field-chip:hover {
  border-color: rgba(100, 210, 255, 0.36);
  background: rgba(100, 210, 255, 0.12);
  color: #fff;
}

.chip-add,
:deep(.order-btn) {
  color: var(--yb-music-cyan);
}

:deep(.selected-label),
.empty-fields {
  color: var(--yb-music-text-2);
}

.arrow-btn {
  border-radius: 0.65rem;
  border-color: rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.08);
  color: var(--yb-music-text-2);
}

.arrow-btn:hover {
  border-color: rgba(100, 210, 255, 0.36);
  background: rgba(100, 210, 255, 0.12);
  color: #fff;
}

:deep(.remove-btn) {
  color: #ff9b95;
}

@media (max-width: 700px) {
  .field-selector {
    grid-template-columns: minmax(0, 1fr);
  }

  .selector-arrows {
    flex-direction: row;
    justify-content: flex-start;
    padding-top: 0;
  }

  :deep(.selected-item) {
    align-items: flex-start;
  }

  :deep(.item-actions) {
    flex: 0 0 auto;
  }
}
</style>
