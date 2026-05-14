<template>
  <div class="type-section">
    <section class="form-row">
      <div class="form-card">
        <IIdSelector v-model="categoryModel" />
      </div>
      <div class="form-card">
        <BaseInput
          v-model="localForm.productName"
          label="产品名称"
          placeholder="新品产品名称"
        />
        <p v-if="localForm.sku" class="form-hint">
          SKU：{{ localForm.sku }}（后端生成，预展示可用）
        </p>
        <p v-else class="form-hint danger">SKU 将由后端在创建任务时自动生成</p>
      </div>
    </section>

    <section class="form-card requirement-card">
      <BaseTextarea
        v-model="localForm.designRequirement"
        label="设计需求说明"
        :rows="4"
        placeholder="设计需求"
      />
    </section>

    <section class="form-row">
      <div class="form-card">
        <BaseTextarea
          v-model="localForm.prefillSpecText"
          label="规格尺寸"
          :rows="2"
          placeholder="请输入规格尺寸"
        />
      </div>
      <div class="form-card upload-card">
        <label class="field-label">参考图（可选）</label>
        <ReferenceUploadPanel v-model="referenceRefsModel" compact />
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { TaskCreateFormModel } from '@/domain/types'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import ReferenceUploadPanel from '@/components/task/ReferenceUploadPanel.vue'
import IIdSelector from '@/components/task-create/IIdSelector.vue'

const props = defineProps<{
  form: TaskCreateFormModel
}>()

const emit = defineEmits<{
  'update:form': [TaskCreateFormModel]
}>()

const localForm = computed({
  get: () => props.form,
  set: (value: TaskCreateFormModel) => emit('update:form', value),
})
const referenceRefsModel = computed({
  get: () => localForm.value.referenceFileRefs,
  set: (value: (string | Record<string, unknown>)[]) => {
    localForm.value.referenceFileRefs = value
  },
})

const categoryModel = computed({
  get: () => localForm.value.category ?? '',
  set: (v: string) => {
    localForm.value.category = v === '' ? undefined : v
  },
})
</script>

<style scoped>
.type-section {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}
.form-row {
  display: contents;
}
.form-card {
  border: 1px solid #e6eaf0;
  border-radius: 0.875rem;
  padding: 0.75rem;
  background: #fff;
  min-height: 5.25rem;
}
.upload-card {
  background: #eef5ff;
}
.field-label {
  font-size: 0.8125rem;
  font-weight: 500;
  color: #334155;
}
.required {
  color: #dc2626;
}
.form-hint {
  margin: 0.25rem 0 0;
  font-size: 0.8125rem;
  color: #64748b;
}
.form-hint.danger {
  color: #dc2626;
}
.form-card :deep(.flex.flex-col.gap-1) {
  gap: 0.4rem;
}
.form-card :deep(input),
.form-card :deep(.relative > div) {
  height: 2.75rem;
  border-radius: 0.75rem;
  background: #f8fafc;
}
.form-card :deep(textarea) {
  border-radius: 0.75rem;
  background: #f8fafc;
  box-shadow: none;
  resize: vertical;
}

/* Apple Music / iOS liquid glass create-task embedded form skin. Style-only. */
.form-card {
  border-color: rgba(148, 163, 184, 0.20);
  background:
    linear-gradient(145deg, rgba(22, 31, 47, 0.92), rgba(9, 14, 23, 0.96));
  color: #dce7f7;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

.upload-card {
  background:
    radial-gradient(circle at 0% 0%, rgba(100, 210, 255, 0.10), transparent 12rem),
    linear-gradient(145deg, rgba(22, 31, 47, 0.94), rgba(9, 14, 23, 0.96));
}

.field-label {
  color: #cbd8ec;
}

.form-hint {
  color: #8fa0b8;
}

.form-hint.danger,
.required {
  color: #ffb4ad;
}

.form-card :deep(input),
.form-card :deep(.relative > div),
.form-card :deep(textarea) {
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(7, 12, 20, 0.82);
  color: #f8fbff;
}

.form-card :deep(input::placeholder),
.form-card :deep(textarea::placeholder) {
  color: #64748b;
}

.form-card :deep(input:focus),
.form-card :deep(textarea:focus) {
  border-color: rgba(125, 211, 252, 0.62);
  box-shadow: 0 0 0 3px rgba(100, 210, 255, 0.12);
}
@media (max-width: 760px) {
  .type-section {
    grid-template-columns: 1fr;
  }
}
</style>
